// Package inference handles communication with the LLM backend.
package inference

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/tools"
)

// StreamingCallback is a function type for handling streaming chunks.
type StreamingCallback func(chunk string)

// StreamingContentType represents the type of content being streamed.
type StreamingContentType int

const (
	StreamingContentTypeNormal StreamingContentType = iota
	StreamingContentTypeReasoning
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
	endpoint    string
	apiKey      string
	model       string
	temperature float64
	maxTokens   int
	contextSize int
	streaming   bool
	timeout     time.Duration
	client      *http.Client
	maxRetries  int
	retryDelay  time.Duration
	tools       []ToolDefinition
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ToolCall represents a tool call from the OpenAI API response.
type APIToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall represents the function part of a tool call.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Response represents an inference response.
type Response struct {
	Content      string
	ToolCalls    []*tools.ToolCall // Parsed tool calls compatible with tool executor
	APIToolCalls []*APIToolCall    // Raw tool calls from API for reference
	TokenUsage   int
	StreamUsage  int
	InputTokens  int // Prompt tokens from API (actual input to LLM)
	OutputTokens int // Completion tokens from API (actual output from LLM)
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
		maxRetries: 3,
		retryDelay: 1 * time.Second,
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

// GetTools returns the registered tools.
func (ic *InferenceClient) GetTools() []ToolDefinition {
	return ic.tools
}

// InferenceRequest sends a request to the inference backend.
func (ic *InferenceClient) InferenceRequest(ctx context.Context, messages []*Message, systemPrompt string) (*Response, error) {
	return ic.InferenceRequestWithCallback(ctx, messages, systemPrompt, nil)
}

// InferenceRequestStream sends a request with a streaming callback.
func (ic *InferenceClient) InferenceRequestStream(ctx context.Context, messages []*Message, systemPrompt string, callback StreamingCallbackWithType) (*Response, error) {
	return ic.InferenceRequestWithCallbackTyped(ctx, messages, systemPrompt, callback)
}

// InferenceRequestWithCallback sends a request with a streaming callback (legacy, converts to typed).
func (ic *InferenceClient) InferenceRequestWithCallback(ctx context.Context, messages []*Message, systemPrompt string, callback StreamingCallback) (*Response, error) {
	// Convert old callback to typed callback for backwards compatibility
	typedCallback := func(chunk StreamingChunk) {
		if callback != nil {
			callback(chunk.Text)
		}
	}
	return ic.InferenceRequestWithCallbackTyped(ctx, messages, systemPrompt, typedCallback)
}

// InferenceRequestWithCallbackTyped sends a request with a typed streaming callback that supports reasoning content.
func (ic *InferenceClient) InferenceRequestWithCallbackTyped(ctx context.Context, messages []*Message, systemPrompt string, callback StreamingCallbackWithType) (*Response, error) {
	// Build the request
	reqBody := &RequestBody{
		Model:       ic.model,
		Messages:    ic.buildMessages(messages, systemPrompt),
		Stream:      ic.streaming,
		Temperature: ic.temperature,
		MaxTokens:   ic.maxTokens,
	}

	// Add tools if registered
	if len(ic.tools) > 0 {
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

		// Create HTTP request
		req, err := http.NewRequestWithContext(ctx, "POST", ic.endpoint+"/v1/chat/completions", bytes.NewReader(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		if ic.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+ic.apiKey)
		}

		// Make request
		resp, err := ic.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to make request (attempt %d): %w", attempt+1, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			// Only retry on server errors (5xx), not client errors (4xx)
			if resp.StatusCode >= 500 {
				lastErr = fmt.Errorf("API error (attempt %d): %d - %s", attempt+1, resp.StatusCode, string(body))
				continue
			}
			return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
		}

		// Success - handle response
		defer resp.Body.Close()

		// Handle streaming or non-streaming response
		if ic.streaming {
			return ic.handleStreamResponse(resp.Body, callback)
		}
		return ic.handleResponse(resp.Body)
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

	// Add conversation messages
	result = append(result, messages...)

	return result
}

// handleResponse handles a non-streaming response.
func (ic *InferenceClient) handleResponse(body io.Reader) (*Response, error) {
	var respBody struct {
		Choices []struct {
			Message struct {
				Role      string        `json:"role"`
				Content   string        `json:"content"`
				ToolCalls []APIToolCall `json:"tool_calls,omitempty"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
		Timings struct {
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

	// Parse OpenAI tool calls
	var toolCalls []*tools.ToolCall
	var apiToolCalls []*APIToolCall

	if len(message.ToolCalls) > 0 {
		// Convert API tool calls to internal tool calls
		for _, apiTC := range message.ToolCalls {
			apiToolCalls = append(apiToolCalls, &apiTC)

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

	// Get token usage from either usage or timings
	tokenUsage := respBody.Usage.TotalTokens
	if tokenUsage == 0 {
		tokenUsage = respBody.Timings.PredictedN
	}

	return &Response{
		Content:      content,
		ToolCalls:    toolCalls,
		APIToolCalls: apiToolCalls,
		TokenUsage:   tokenUsage,
		InputTokens:  respBody.Usage.PromptTokens,
		OutputTokens: respBody.Usage.CompletionTokens,
	}, nil
}

// handleStreamResponse handles a streaming response.
func (ic *InferenceClient) handleStreamResponse(body io.Reader, callback StreamingCallbackWithType) (*Response, error) {
	var fullContent strings.Builder
	var reasoningContent strings.Builder
	var totalTokens int
	var inputTokens int
	var outputTokens int

	// Use a slice to accumulate tool calls in order
	// Each entry accumulates partial data from streaming deltas
	type accumulatedToolCall struct {
		ID        string
		Type      string
		Name      string
		Arguments string
	}
	var toolCallsList []*accumulatedToolCall
	// Track which tool calls we've already notified about
	notifiedToolCalls := make(map[int]bool)

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and non-SSE data
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// Check for end of stream
		if data == "[DONE]" {
			break
		}

		// Parse SSE data
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string        `json:"content"`
					ReasoningContent string        `json:"reasoning_content"`
					ToolCalls        []APIToolCall `json:"tool_calls,omitempty"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
			Timings struct {
				PredictedN int `json:"predicted_n"`
			} `json:"timings"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 {
			// Get content from delta
			chunkText := ""
			if chunk.Choices[0].Delta.Content != "" {
				chunkText = chunk.Choices[0].Delta.Content
				fullContent.WriteString(chunkText)
			}
			if chunk.Choices[0].Delta.ReasoningContent != "" {
				reasoningContent.WriteString(chunk.Choices[0].Delta.ReasoningContent)
				if chunkText == "" {
					chunkText = chunk.Choices[0].Delta.ReasoningContent
				}
			}

			// Accumulate tool calls from streaming delta
			// Tool calls come in partial chunks that need to be merged
			if len(chunk.Choices[0].Delta.ToolCalls) > 0 {
				for _, deltaTC := range chunk.Choices[0].Delta.ToolCalls {
					// Determine which tool call this delta belongs to
					// If the tool call has an ID, look for an existing one with that ID
					// If no ID or not found, check if we should merge with the last tool call

					targetIndex := -1

					// First, try to find by ID
					if deltaTC.ID != "" {
						for i, tc := range toolCallsList {
							if tc.ID == deltaTC.ID {
								targetIndex = i
								break
							}
						}
					}

					// If not found by ID and this chunk has no ID or no name,
					// it might be a continuation of the last tool call
					if targetIndex == -1 && (deltaTC.ID == "" && deltaTC.Function.Name == "") && len(toolCallsList) > 0 {
						// Check if this chunk has arguments (indicating it's a continuation)
						if deltaTC.Function.Arguments != "" {
							targetIndex = len(toolCallsList) - 1
						}
					}

					if targetIndex == -1 {
						// New tool call - create a new entry at the next index
						targetIndex = len(toolCallsList)
					}

					// Ensure the slice is large enough
					for len(toolCallsList) <= targetIndex {
						toolCallsList = append(toolCallsList, &accumulatedToolCall{})
					}

					existing := toolCallsList[targetIndex]

					// Merge with existing tool call - accumulate fields
					// ID
					if deltaTC.ID != "" {
						existing.ID = deltaTC.ID
					}
					// Name typically comes first and doesn't change
					if deltaTC.Function.Name != "" {
						existing.Name = deltaTC.Function.Name
					}
					// Arguments are streamed as incremental JSON string fragments
					if deltaTC.Function.Arguments != "" {
						existing.Arguments += deltaTC.Function.Arguments
					}
					// Type should be consistent
					if deltaTC.Type != "" {
						existing.Type = deltaTC.Type
					}

					// Notify about new tool call if callback is available
					// Only notify once per tool call (when we first see the name)
					if callback != nil && deltaTC.Function.Name != "" && !notifiedToolCalls[targetIndex] {
						toolName := deltaTC.Function.Name
						// Stream a notification that a tool call is being made
						notification := fmt.Sprintf("\n[Tool Call] %s\n", toolName)
						callback(StreamingChunk{
							Text:        notification,
							ContentType: StreamingContentTypeNormal,
						})
						notifiedToolCalls[targetIndex] = true
					}
				}
			}

			// Stream reasoning content with appropriate type
			if chunk.Choices[0].Delta.ReasoningContent != "" {
				callback(StreamingChunk{
					Text:        chunk.Choices[0].Delta.ReasoningContent,
					ContentType: StreamingContentTypeReasoning,
				})
			}

			// Stream normal content with appropriate type
			if chunk.Choices[0].Delta.Content != "" {
				callback(StreamingChunk{
					Text:        chunk.Choices[0].Delta.Content,
					ContentType: StreamingContentTypeNormal,
				})
			}

			// Get token usage - also track input/output separately
			if chunk.Usage.TotalTokens > 0 {
				totalTokens = chunk.Usage.TotalTokens
				inputTokens = chunk.Usage.PromptTokens
				outputTokens = chunk.Usage.CompletionTokens
			} else if chunk.Timings.PredictedN > 0 {
				totalTokens = chunk.Timings.PredictedN
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("stream error: %w", err)
	}

	// Combine content and reasoning content
	content := reasoningContent.String() + fullContent.String()

	// Convert accumulated API tool calls to internal format
	var apiToolCalls []*APIToolCall
	var toolCalls []*tools.ToolCall

	// Process tool calls in order
	for _, accTC := range toolCallsList {
		// Skip empty tool calls (those that were only created for merging)
		if accTC.Name == "" && accTC.Arguments == "" {
			continue
		}

		// Create API tool call for reference
		apiTC := &APIToolCall{
			ID:   accTC.ID,
			Type: accTC.Type,
			Function: FunctionCall{
				Name:      accTC.Name,
				Arguments: accTC.Arguments,
			},
		}
		apiToolCalls = append(apiToolCalls, apiTC)

		// Parse the accumulated arguments JSON string
		var params map[string]interface{}
		if accTC.Arguments != "" {
			if err := json.Unmarshal([]byte(accTC.Arguments), &params); err != nil {
				// If parsing fails, log the error but don't fail entirely
				params = map[string]interface{}{
					"_raw_arguments": accTC.Arguments,
					"_parse_error":   err.Error(),
				}
			}
		}

		toolCall := &tools.ToolCall{
			ID:         accTC.ID,
			Name:       accTC.Name,
			Parameters: params,
			Raw:        accTC.Arguments,
		}
		toolCalls = append(toolCalls, toolCall)
	}

	return &Response{
		Content:      content,
		ToolCalls:    toolCalls,
		APIToolCalls: apiToolCalls,
		TokenUsage:   totalTokens,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}, nil
}

// RequestBody represents the request body for the inference API.
type RequestBody struct {
	Model       string           `json:"model"`
	Messages    []*Message       `json:"messages"`
	Stream      bool             `json:"stream"`
	Temperature float64          `json:"temperature"`
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
