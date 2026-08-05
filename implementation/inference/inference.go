// Package inference handles communication with the LLM backend.
package inference

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/tools"
)

// StreamingContentType represents the type of content being streamed.
type StreamingContentType int

const (
	StreamingContentTypeNormal StreamingContentType = iota
	StreamingContentTypeReasoning
	StreamingContentTypeGoal
	StreamingContentTypeCompression
)

// StreamingChunk represents a streaming chunk with content type.
type StreamingChunk struct {
	Text        string
	ContentType StreamingContentType
}

// StreamingCallbackWithType is a function type for handling streaming chunks with content type.
type StreamingCallbackWithType func(chunk StreamingChunk)

// InferenceClient handles communication with the LLM backend.
type InferenceClient struct {
	endpoint            string
	apiKey              string
	model               string
	temperature         *float64
	maxTokens           int
	contextSize         int
	streaming           bool
	timeout             time.Duration
	client              *http.Client
	maxRetries          int
	retryDelay          time.Duration
	tools               []ToolDefinition
	debugVerbose        bool
	debugVerboseVerbose bool
	maxDisplayWidth     int // Maximum display width for tool call arguments (0 = no limit)
}

// Message represents a chat message.
type Message struct {
	Role             string         `json:"role"`
	Content          string         `json:"content"`                     // Plain text content
	ContentParts     []ContentPart  `json:"-"`                           // Multi-modal content parts (images + text)
	Reasoning        string         `json:"reasoning,omitempty"`         // OpenAI standard field for reasoning models (o1, o3-mini, etc.)
	ReasoningContent string         `json:"reasoning_content,omitempty"` // llama.cpp
	ToolCallId       string         `json:"tool_call_id,omitempty"`      // For tool call output messages
	ToolCalls        []*APIToolCall `json:"tool_calls,omitempty"`        // For assistant messages with tool calls
}

// ContentPart represents a single part of multi-modal message content.
type ContentPart struct {
	Type     string        `json:"type"`      // "text" or "image_url"
	Text     string        `json:"text"`      // Text content (when type is "text")
	ImageURL *ImageURLPart `json:"image_url"` // Image content (when type is "image_url")
}

// ImageURLPart represents an image in a multi-modal message.
type ImageURLPart struct {
	URL    string `json:"url"`    // Public URL or base64 data URI (data:[mime_type];base64,[base64_string])
	Detail string `json:"detail"` // "auto", "low", or "high" (default: "auto")
}

// HasContentParts returns true if the message has multi-modal content parts.
func (m *Message) HasContentParts() bool {
	return len(m.ContentParts) > 0
}

// SetImageContent creates a message with image content.
func (m *Message) SetImageContent(text string, imageDataURI string, detail string) {
	m.ContentParts = []ContentPart{
		{Type: "text", Text: text},
		{Type: "image_url", ImageURL: &ImageURLPart{URL: imageDataURI, Detail: detail}},
	}
}

// MarshalJSON implements custom JSON marshaling for Message to support multi-modal content.
// When ContentParts is set, content is serialized as an array; otherwise as a plain string.
func (m *Message) MarshalJSON() ([]byte, error) {
	// Build the JSON manually to handle the conditional content field
	type Alias Message // Avoid infinite recursion
	aux := &struct {
		Content interface{} `json:"content,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	// Set content based on whether we have multi-modal parts
	if len(m.ContentParts) > 0 {
		aux.Content = m.ContentParts
	} else if m.Content == "" {
		// Emit null (omitted via omitempty) rather than an empty string. Several
		// OpenAI-compatible servers expect content:null for assistant messages
		// that only carry tool_calls, and reject content:"".
		aux.Content = nil
	} else {
		aux.Content = m.Content
	}

	return json.Marshal(aux)
}

// ToolDefinition represents a tool definition for the LLM (OpenAI format).
type ToolDefinition struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition defines a function tool (OpenAI format).
type FunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  ParameterSchema `json:"parameters"`
}

// ParameterSchema defines the schema for tool parameters (OpenAI format).
type ParameterSchema struct {
	Type       string              `json:"type"`
	Required   []string            `json:"required,omitempty"`
	Properties map[string]Property `json:"properties"`
}

// Property defines a single property in the schema.
type Property struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Items       *Property `json:"items,omitempty"`
}

// ToolCall represents a tool call from the OpenAI API response.
type APIToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
	Index    *int         `json:"index,omitempty"` // Index in streaming deltas
}

// FunctionCall represents the function part of a tool call.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Response represents an inference response.
type Response struct {
	Content              string            // The actual response text (does NOT include reasoning)
	Reasoning            string            // Reasoning content from the model (from whichever field the server used)
	ReasoningContentType string            // Which reasoning field the server used: "reasoning" or "reasoning_content"
	ToolCalls            []*tools.ToolCall // Parsed tool calls compatible with tool executor
	APIToolCalls         []*APIToolCall    // Raw tool calls from API for reference
	TokenUsage           int
	StreamUsage          int
	InputTokens          int // Prompt tokens from API (actual input to LLM)
	OutputTokens         int // Completion tokens from API (actual output from LLM)
}

// NewInferenceClient creates a new inference client.
func NewInferenceClient(cfg *config.Config) *InferenceClient {
	totalTimeout := time.Duration(cfg.InitialTokenTimeout) * time.Second
	if cfg.ReadTimeout > 0 {
		totalTimeout = time.Duration(cfg.ReadTimeout) * time.Second
	}

	return &InferenceClient{
		endpoint:    cfg.APIEndpoint,
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		temperature: cfg.Temperature,
		maxTokens:   cfg.MaxTokens,
		contextSize: cfg.ContextSize,
		streaming:   cfg.Streaming,
		timeout:     totalTimeout,
		client: &http.Client{
			Timeout: totalTimeout,
		},
		maxRetries:          3,
		retryDelay:          1 * time.Second,
		debugVerbose:        cfg.DebugVerbose,
		debugVerboseVerbose: cfg.DebugVerboseVerbose,
	}
}

// SetEndpoint sets the API endpoint.
func (ic *InferenceClient) SetEndpoint(endpoint string) {
	ic.endpoint = endpoint
}

// SetAPIKey sets the API key.
func (ic *InferenceClient) SetAPIKey(key string) {
	ic.apiKey = key
}

// SetTools sets the available tools for tool calling.
func (ic *InferenceClient) SetTools(tools []ToolDefinition) {
	ic.tools = tools
}

// SetMaxDisplayWidth sets the maximum display width for tool call arguments.
// This is used to truncate long parameter values to prevent terminal line wrapping.
func (ic *InferenceClient) SetMaxDisplayWidth(width int) {
	ic.maxDisplayWidth = width
}

// GetTools returns the registered tools.
func (ic *InferenceClient) GetTools() []ToolDefinition {
	return ic.tools
}

// isCopilotEndpoint checks if the endpoint is a GitHub Copilot URL.
func (ic *InferenceClient) isCopilotEndpoint() bool {
	return config.IsGitHubCopilotEndpoint(ic.endpoint)
}

// isGitHubModelsEndpoint checks if the endpoint is a GitHub Models URL.
func (ic *InferenceClient) isGitHubModelsEndpoint() bool {
	return config.IsGitHubModelsEndpoint(ic.endpoint)
}

// buildURL constructs the full API URL based on the endpoint type.
// Copilot uses /chat/completions, GitHub Models uses /inference/chat/completions,
// and all other endpoints use the default /v1/chat/completions.
// A trailing slash (and, for the default path, a trailing /v1) is stripped from
// the configured endpoint first so callers may provide either
// "https://api.openai.com" or "https://api.openai.com/v1" without producing a
// doubled "/v1/v1" path segment.
func (ic *InferenceClient) buildURL() string {
	base := strings.TrimRight(ic.endpoint, "/")
	if ic.isCopilotEndpoint() {
		return base + "/chat/completions"
	}
	if ic.isGitHubModelsEndpoint() {
		return base + "/inference/chat/completions"
	}
	base = strings.TrimSuffix(base, "/v1")
	return base + "/v1/chat/completions"
}

// InferenceRequest sends a request to the inference backend (non-streaming).
func (ic *InferenceClient) InferenceRequest(ctx context.Context, messages []*Message, systemPrompt string) (*Response, error) {
	return ic.request(ctx, messages, systemPrompt, nil, true)
}

// InferenceRequestNoTools is like InferenceRequest but omits the registered tool
// definitions and tool_choice from the request body. It is used for vision/image
// analysis calls that should not advertise function calling (some endpoints/models
// reject requests with a tools array when they do not support tools).
func (ic *InferenceClient) InferenceRequestNoTools(ctx context.Context, messages []*Message, systemPrompt string) (*Response, error) {
	return ic.request(ctx, messages, systemPrompt, nil, false)
}

// InferenceRequestStream sends a request with a streaming callback.
func (ic *InferenceClient) InferenceRequestStream(ctx context.Context, messages []*Message, systemPrompt string, callback StreamingCallbackWithType) (*Response, error) {
	return ic.request(ctx, messages, systemPrompt, callback, true)
}

// InferenceRequestStreamNoTools is like InferenceRequestStream but omits the
// registered tool definitions and tool_choice from the request body.
func (ic *InferenceClient) InferenceRequestStreamNoTools(ctx context.Context, messages []*Message, systemPrompt string, callback StreamingCallbackWithType) (*Response, error) {
	return ic.request(ctx, messages, systemPrompt, callback, false)
}

// InferenceRequestWithCallbackTyped sends a request with a typed streaming callback that supports reasoning content.
func (ic *InferenceClient) InferenceRequestWithCallbackTyped(ctx context.Context, messages []*Message, systemPrompt string, callback StreamingCallbackWithType) (*Response, error) {
	return ic.request(ctx, messages, systemPrompt, callback, true)
}

// request is the shared implementation for all inference requests. When
// includeTools is false, the registered tool definitions and tool_choice are
// omitted from the request body.
func (ic *InferenceClient) request(ctx context.Context, messages []*Message, systemPrompt string, callback StreamingCallbackWithType, includeTools bool) (*Response, error) {
	// Build the request
	reqBody := &RequestBody{
		Model:       ic.model,
		Messages:    ic.buildMessages(messages, systemPrompt),
		Stream:      ic.streaming,
		Temperature: ic.temperature,
		MaxTokens:   ic.maxTokens,
	}

	// Add tools if registered
	if includeTools && len(ic.tools) > 0 {
		reqBody.Tools = ic.tools
		reqBody.ToolChoice = "auto"
	}

	// Serialize request
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Retry logic
	var lastErr error
	for attempt := 0; attempt <= ic.maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(ic.retryDelay):
			}
		}

		// Create HTTP request using buildURL() for endpoint-aware path construction
		req, err := http.NewRequestWithContext(ctx, "POST", ic.buildURL(), bytes.NewReader(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		if ic.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+ic.apiKey)
		}

		// Add Copilot-specific headers when needed
		if ic.isCopilotEndpoint() {
			req.Header.Set("Copilot-Integration-Id", "coding-agent")
			req.Header.Set("Editor-Version", "coding-agent/1.0")
		}

		// Debug verbose: log the request
		if ic.debugVerbose {
			fmt.Fprintf(os.Stderr, "[DEBUG-VERBOSE] >>> Request: POST %s\n", ic.buildURL())
			fmt.Fprintf(os.Stderr, "[DEBUG-VERBOSE] >>> Headers: ")
			first := true
			for k, v := range req.Header {
				if !first {
					fmt.Fprintf(os.Stderr, ", ")
				}
				fmt.Fprintf(os.Stderr, "%s=%s", k, strings.Join(v, ";"))
				first = false
			}
			fmt.Fprintln(os.Stderr)
			// Log body (redact authorization)
			bodyStr := string(jsonData)
			fmt.Fprintf(os.Stderr, "[DEBUG-VERBOSE] >>> Body (%d bytes): %s\n", len(jsonData), truncateJSON(bodyStr, 1000))
		}

		// Make request
		resp, err := ic.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to make request (attempt %d): %w", attempt+1, err)
			if ic.debugVerbose {
				fmt.Fprintf(os.Stderr, "[DEBUG-VERBOSE] <<< Request failed: %v\n", err)
			}
			continue
		}

		// Debug verbose: log response status
		if ic.debugVerbose {
			fmt.Fprintf(os.Stderr, "[DEBUG-VERBOSE] <<< Response: %d %s\n", resp.StatusCode, resp.Status)
		}

		// Handle 429 rate limiting - retry with backoff
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := resp.Header.Get("Retry-After")
			var retryDelay time.Duration
			if retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
					retryDelay = time.Duration(seconds) * time.Second
				} else {
					retryDelay = ic.retryDelay
				}
			} else {
				retryDelay = ic.retryDelay
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if attempt < ic.maxRetries {
				// Log retry attempt for debugging
				if len(body) > 0 {
					lastErr = fmt.Errorf("API rate limited (429, attempt %d): %s - retrying in %v", attempt+1, string(body), retryDelay)
				} else {
					lastErr = fmt.Errorf("API rate limited (429, attempt %d) - retrying in %v", attempt+1, retryDelay)
				}
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(retryDelay):
				}
				continue
			}
			return nil, fmt.Errorf("API rate limited (429) after %d retries", ic.maxRetries)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			// Handle authentication errors with specific guidance
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				errorMsg := fmt.Sprintf("API authentication failed (HTTP %d)", resp.StatusCode)
				if ic.isCopilotEndpoint() {
					errorMsg += "\nEnsure your GITHUB_TOKEN or --api-key is a valid GitHub Copilot token.\nGenerate one at: https://github.com/settings/tokens"
				}
				return nil, fmt.Errorf("%s", errorMsg)
			}

			// Handle bad request errors with Copilot-specific guidance
			if resp.StatusCode == http.StatusBadRequest {
				errorMsg := fmt.Sprintf("API error (HTTP %d) - %s", resp.StatusCode, string(body))
				if ic.isCopilotEndpoint() {
					if strings.Contains(string(body), "third-party user token") || strings.Contains(string(body), "Personal Access Token") {
						errorMsg += "\nhint: api.githubcopilot.com does not accept Personal Access Tokens (github_pat). Use a Copilot user token (ghu_) for this endpoint.\nAlternatively switch to CODING_AGENT_API_ENDPOINT=https://models.github.ai when using PAT/OAuth tokens"
					}
				}
				return nil, fmt.Errorf("%s", errorMsg)
			}

			// Retry on server errors (5xx), not client errors (4xx)
			if resp.StatusCode >= 500 {
				lastErr = fmt.Errorf("API error (attempt %d): %d - %s", attempt+1, resp.StatusCode, string(body))
				continue
			}

			// For other non-OK status codes, return error without retry
			return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
		}

		// Success - handle response. Close the body explicitly on every return
		// path rather than deferring inside the loop (a function-scoped defer
		// inside a retry loop is fragile and easy to leak).
		var respOut *Response
		var respErr error
		if ic.streaming {
			// Set read deadline to prevent hanging on slow/stalled streams.
			// http.Response.Body implements SetReadDeadline for network connections,
			// so this always succeeds in normal operation.
			if rc, ok := resp.Body.(interface{ SetReadDeadline(time.Time) error }); ok {
				_ = rc.SetReadDeadline(time.Now().Add(ic.timeout))
			}
			respOut, respErr = ic.handleStreamResponse(resp.Body, callback)
		} else {
			respOut, respErr = ic.handleResponse(resp.Body)
		}
		resp.Body.Close()
		return respOut, respErr
	}

	return nil, lastErr
}

// buildMessages builds the message list with system prompt.
func (ic *InferenceClient) buildMessages(messages []*Message, systemPrompt string) []*Message {
	result := make([]*Message, 0, len(messages)+1)

	// Add system prompt first
	if systemPrompt != "" {
		result = append(result, &Message{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	// Add conversation messages with normalized tool call types
	for _, msg := range messages {
		normalized := *msg
		// Deep-copy ToolCalls so we can normalize types without mutating the original.
		// IMPORTANT: if new pointer/slice fields are added to Message in the future,
		// they must also be deep-copied here to maintain the no-mutation guarantee.
		if len(msg.ToolCalls) > 0 {
			normalized.ToolCalls = make([]*APIToolCall, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				cpy := *tc
				normalized.ToolCalls[i] = &cpy
			}
		}
		for _, tc := range normalized.ToolCalls {
			if tc.Type == "" {
				tc.Type = "function"
			}
		}
		result = append(result, &normalized)
	}

	return result
}

// handleResponse handles a non-streaming response.
func (ic *InferenceClient) handleResponse(body io.Reader) (*Response, error) {
	var respBody struct {
		Choices []struct {
			Message      Message `json:"message"`
			FinishReason string  `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
		Timings struct {
			CacheN     int `json:"cache_n"`
			PromptN    int `json:"prompt_n"`
			PredictedN int `json:"predicted_n"`
		} `json:"timings"`
	}

	if err := json.NewDecoder(body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(respBody.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	message := respBody.Choices[0].Message
	content := message.Content

	// Determine which reasoning field the server used and set ReasoningContentType accordingly
	var reasoning string
	var reasoningContentType string
	if message.Reasoning != "" {
		reasoning = message.Reasoning
		reasoningContentType = "reasoning"
	} else if message.ReasoningContent != "" {
		reasoning = message.ReasoningContent
		reasoningContentType = "reasoning_content"
	}

	// Parse OpenAI tool calls
	var toolCalls []*tools.ToolCall
	var apiToolCalls []*APIToolCall

	if len(message.ToolCalls) > 0 {
		// Convert API tool calls to internal tool calls
		for _, apiTC := range message.ToolCalls {
			apiToolCalls = append(apiToolCalls, apiTC)

			// Parse the arguments JSON string
			var params map[string]interface{}
			if apiTC.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(apiTC.Function.Arguments), &params); err != nil {
					// If parsing fails, store as raw string in parameters
					params = map[string]interface{}{
						"_raw_arguments": apiTC.Function.Arguments,
						"_parse_error":   err.Error(),
					}
				}
			}

			toolCall := &tools.ToolCall{
				ID:         apiTC.ID,
				Name:       apiTC.Function.Name,
				Parameters: params,
				Raw:        apiTC.Function.Arguments,
			}
			toolCalls = append(toolCalls, toolCall)
		}
	}

	// Get token usage from API - prefer OpenAI-style fields, fall back to timings
	inputTokens := respBody.Usage.PromptTokens
	outputTokens := respBody.Usage.CompletionTokens
	tokenUsage := respBody.Usage.TotalTokens

	if inputTokens == 0 && outputTokens == 0 && tokenUsage == 0 {
		// Fall back to llama.cpp timings format - cache_n is NOT included in prompt_n
		inputTokens = respBody.Timings.CacheN + respBody.Timings.PromptN
		outputTokens = respBody.Timings.PredictedN
		if outputTokens > 0 {
			tokenUsage = inputTokens + outputTokens
		}
	}

	return &Response{
		Content:              content,
		Reasoning:            reasoning,
		ReasoningContentType: reasoningContentType,
		ToolCalls:            toolCalls,
		APIToolCalls:         apiToolCalls,
		TokenUsage:           tokenUsage,
		InputTokens:          inputTokens,
		OutputTokens:         outputTokens,
	}, nil
}

// RequestBody represents the request body for the inference API.
type RequestBody struct {
	Model       string           `json:"model"`
	Messages    []*Message       `json:"messages"`
	Stream      bool             `json:"stream"`
	Temperature *float64         `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	ToolChoice  string           `json:"tool_choice,omitempty"`
}

// EstimateTokens estimates the number of tokens in text.
// Uses a more sophisticated heuristic based on content type.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	// Count words as a better proxy for tokens
	words := strings.Fields(text)
	wordCount := len(words)

	// Rough estimate: 1 word ≈ 1.3 tokens (common heuristic)
	// Add extra for special characters and formatting
	estimatedTokens := int(float64(wordCount) * 1.3)

	// Adjust for code-like content (more tokens per word due to special chars)
	if strings.Contains(text, "{") || strings.Contains(text, "}") ||
		strings.Contains(text, "func") || strings.Contains(text, "import") {
		estimatedTokens = int(float64(estimatedTokens) * 1.2)
	}

	// Ensure minimum of 1 token for non-empty text
	if estimatedTokens < 1 && len(text) > 0 {
		estimatedTokens = 1
	}

	return estimatedTokens
}

// EstimateContextSize estimates the total context size including messages and tool definitions.
func EstimateContextSize(messages []*Message, toolDefinitions []ToolDefinition, systemPrompt string) int {
	total := 0

	// Add system prompt tokens
	if systemPrompt != "" {
		total += EstimateTokens(systemPrompt)
	}

	// Add message tokens
	for _, msg := range messages {
		// Add role prefix tokens (system: ~2, user: ~2, assistant: ~3)
		switch msg.Role {
		case "system":
			total += 2
		case "user":
			total += 2
		case "assistant":
			total += 3
		}
		total += EstimateTokens(msg.Content)
		// Also estimate tokens for reasoning content if present
		if msg.Reasoning != "" {
			total += EstimateTokens(msg.Reasoning)
		}
		if msg.ReasoningContent != "" {
			total += EstimateTokens(msg.ReasoningContent)
		}
	}

	// Add tool definition tokens (rough estimate)
	for _, tool := range toolDefinitions {
		total += EstimateTokens(tool.Function.Name)
		total += EstimateTokens(tool.Function.Description)
		// Estimate parameter tokens
		for _, prop := range tool.Function.Parameters.Properties {
			total += EstimateTokens(prop.Type)
			total += EstimateTokens(prop.Description)
		}
	}

	return total
}

// formatToolCallArgs formats tool call arguments as a flat single-line string
// for display in the TUI during streaming.
// The maxArgWidth parameter limits the total width of the argument display.
// This produces output like: key1: "value1", key2: "value2"
// Instead of pretty-printed multi-line JSON which causes visual noise.
func formatToolCallArgs(params map[string]interface{}, maxArgWidth int) string {
	if len(params) == 0 {
		return "{}"
	}

	numParams := len(params)

	// Reserve space for separators (", " between params = 2 chars each)
	separatorWidth := (numParams - 1) * 2
	availableWidth := maxArgWidth - separatorWidth
	if availableWidth < 20 {
		availableWidth = 20
	}

	// Calculate per-parameter budget
	// Reserve ~6 chars for "key: " prefix (key name + ": ")
	perParamWidth := availableWidth / numParams
	valueWidth := perParamWidth - 6
	if valueWidth < 8 {
		valueWidth = 8
	}

	// Build flat single-line output for streaming
	var parts []string
	for key, value := range params {
		formattedValue := formatJSONValueWithMaxWidth(value, valueWidth)
		parts = append(parts, fmt.Sprintf("%s: %s", key, formattedValue))
	}

	result := strings.Join(parts, ", ")

	// Safety: if result still exceeds max width (can happen with many params),
	// truncate and indicate truncation
	if maxArgWidth > 0 && len(result) > maxArgWidth {
		// Truncate to max width minus space for "..." suffix
		// Use rune-aware slicing to avoid splitting multi-byte characters
		truncLimit := maxArgWidth - 4
		if truncLimit < 10 {
			truncLimit = 10
		}
		runes := []rune(result)
		if truncLimit > len(runes) {
			truncLimit = len(runes)
		}
		return string(runes[:truncLimit]) + "..."
	}

	return result
}

// formatJSONValue formats a single JSON value for display with a default max width.
func formatJSONValue(value interface{}) string {
	return formatJSONValueWithMaxWidth(value, 100)
}

// formatJSONValueWithMaxWidth formats a single JSON value for display with a maximum width limit.
func formatJSONValueWithMaxWidth(value interface{}, maxWidth int) string {
	switch v := value.(type) {
	case string:
		// Truncate long strings for display
		// Leave room for quotes and potential " (N chars)" suffix
		quoteWidth := 2   // opening and closing quotes
		suffixWidth := 10 // " (...) " minimum for suffix
		availableWidth := maxWidth - quoteWidth - suffixWidth
		if availableWidth < 10 {
			availableWidth = 10
		}
		if len(v) > availableWidth {
			return fmt.Sprintf("%q (%d chars)", v[:availableWidth], len(v))
		}
		return fmt.Sprintf("%q", v)
	case float64:
		// JSON numbers are unmarshaled as float64
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%.1f", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case nil:
		return "null"
	case map[string]interface{}:
		return formatJSONMapWithMaxWidth(v, maxWidth)
	case []interface{}:
		return formatJSONArrayWithMaxWidth(v, maxWidth)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// formatJSONMap formats a nested map for display as a flat single-line string.
// This keeps the streaming display clean by avoiding newlines.
func formatJSONMap(m map[string]interface{}) string {
	return formatJSONMapWithMaxWidth(m, 100)
}

// formatJSONMapWithMaxWidth formats a nested map for display with a maximum width limit.
func formatJSONMapWithMaxWidth(m map[string]interface{}, maxWidth int) string {
	if len(m) == 0 {
		return "{}"
	}

	// Reserve space for braces and separators
	bracesWidth := 2 // { }
	numEntries := len(m)
	separatorWidth := (numEntries - 1) * 2 // ", "
	availableWidth := maxWidth - bracesWidth - separatorWidth
	if availableWidth < 10 {
		return "{...}"
	}

	perEntryWidth := availableWidth / numEntries

	var parts []string
	for key, value := range m {
		// Reserve space for "key: "
		valueWidth := perEntryWidth - 5
		if valueWidth < 5 {
			valueWidth = 5
		}
		formattedValue := formatJSONValueWithMaxWidth(value, valueWidth)
		parts = append(parts, fmt.Sprintf("%s: %s", key, formattedValue))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// formatJSONArray formats a nested array for display.
func formatJSONArray(arr []interface{}) string {
	return formatJSONArrayWithMaxWidth(arr, 100)
}

// formatJSONArrayWithMaxWidth formats a nested array for display with a maximum width limit.
func formatJSONArrayWithMaxWidth(arr []interface{}, maxWidth int) string {
	if len(arr) == 0 {
		return "[]"
	}
	// For arrays, just show the length if it's long
	if len(arr) > 10 {
		return fmt.Sprintf("[%d items]", len(arr))
	}

	// Reserve space for brackets and separators
	bracketsWidth := 2 // [ ]
	numItems := len(arr)
	separatorWidth := (numItems - 1) * 2 // ", "
	availableWidth := maxWidth - bracketsWidth - separatorWidth
	if availableWidth < 10 {
		return fmt.Sprintf("[%d items]", len(arr))
	}

	perItemWidth := availableWidth / numItems

	var parts []string
	for _, item := range arr {
		formattedItem := formatJSONValueWithMaxWidth(item, perItemWidth)
		parts = append(parts, formattedItem)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// truncateJSON truncates a JSON string to a maximum length, appending "..." if truncated.
func truncateJSON(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
