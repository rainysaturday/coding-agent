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
	"strconv"
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
	temperature *float64
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
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCallId string         `json:"tool_call_id,omitempty"` // For tool call output messages
	ToolCalls  []*APIToolCall `json:"tool_calls,omitempty"`   // For assistant messages with tool calls
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
	Type        string     `json:"type"`
	Description string     `json:"description"`
	Items       *Property  `json:"items,omitempty"`
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
	Content      string
	Reasoning    string            // Reasoning content from the model (OpenAI standard "reasoning" field)
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

// isCopilotEndpoint checks if the endpoint is a GitHub Copilot URL.
func (ic *InferenceClient) isCopilotEndpoint() bool {
	return strings.Contains(ic.endpoint, "githubcopilot.com")
}

// isGitHubModelsEndpoint checks if the endpoint is a GitHub Models URL.
func (ic *InferenceClient) isGitHubModelsEndpoint() bool {
	return strings.Contains(ic.endpoint, "models.github.ai")
}

// buildURL constructs the full API URL based on the endpoint type.
// Copilot uses /chat/completions, GitHub Models uses /inference/chat/completions,
// and all other endpoints use the default /v1/chat/completions.
func (ic *InferenceClient) buildURL() string {
	if ic.isCopilotEndpoint() {
		return ic.endpoint + "/chat/completions"
	}
	if ic.isGitHubModelsEndpoint() {
		return ic.endpoint + "/inference/chat/completions"
	}
	return ic.endpoint + "/v1/chat/completions"
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

		// Make request
		resp, err := ic.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to make request (attempt %d): %w", attempt+1, err)
			continue
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
				return nil, fmt.Errorf(errorMsg)
			}

			// Handle bad request errors with Copilot-specific guidance
			if resp.StatusCode == http.StatusBadRequest {
				errorMsg := fmt.Sprintf("API error (HTTP %d) - %s", resp.StatusCode, string(body))
				if ic.isCopilotEndpoint() {
					if strings.Contains(string(body), "third-party user token") || strings.Contains(string(body), "Personal Access Token") {
						errorMsg += "\nhint: api.githubcopilot.com does not accept Personal Access Tokens (github_pat). Use a Copilot user token (ghu_) for this endpoint.\nAlternatively switch to CODING_AGENT_API_ENDPOINT=https://models.github.ai when using PAT/OAuth tokens"
					}
				}
				return nil, fmt.Errorf(errorMsg)
			}

			// Retry on server errors (5xx), not client errors (4xx)
			if resp.StatusCode >= 500 {
				lastErr = fmt.Errorf("API error (attempt %d): %d - %s", attempt+1, resp.StatusCode, string(body))
				continue
			}

			// For other non-OK status codes, return error without retry
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
				Reasoning string        `json:"reasoning"` // OpenAI standard field for reasoning models (o1, o3-mini, etc.)
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
			CacheN       int `json:"cache_n"`
			PromptN      int `json:"prompt_n"`
			PredictedN   int `json:"predicted_n"`
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
		Content:      content,
		Reasoning:    message.Reasoning, // OpenAI standard "reasoning" field
		ToolCalls:    toolCalls,
		APIToolCalls: apiToolCalls,
		TokenUsage:   tokenUsage,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
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
	// Track the last active tool call index for continuation deltas
	lastActiveToolCallIndex := -1

	scanner := bufio.NewScanner(body)
	var jsonBuffer strings.Builder
	inJSON := false

	// Declare chunk for JSON parsing - used for both single-line and multi-line SSE
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content   string        `json:"content"`
				Reasoning string        `json:"reasoning"` // OpenAI standard field for reasoning models (o1, o3-mini, etc.)
				ToolCalls []APIToolCall `json:"tool_calls,omitempty"`
			} `json:"delta"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	Timings struct {
			CacheN       int `json:"cache_n"`
			PromptN      int `json:"prompt_n"`
			PredictedN   int `json:"predicted_n"`
		} `json:"timings"`
	}

	for scanner.Scan() {
		line := scanner.Text()

		// Handle SSE data lines
		if strings.HasPrefix(line, "data: ") {
			// If we were accumulating multi-line JSON, flush and process it first
			if inJSON && jsonBuffer.Len() > 0 {
				inJSON = false
				var bufferChunk struct {
					Choices []struct {
						Delta struct {
							Content   string        `json:"content"`
							Reasoning string        `json:"reasoning"`
							ToolCalls []APIToolCall `json:"tool_calls,omitempty"`
						} `json:"delta"`
						FinishReason string `json:"finish_reason"`
					} `json:"choices"`
					Usage struct {
						PromptTokens     int `json:"prompt_tokens"`
						CompletionTokens int `json:"completion_tokens"`
						TotalTokens      int `json:"total_tokens"`
					} `json:"usage"`
					Timings struct {
CacheN       int `json:"cache_n"`
						PromptN    int `json:"prompt_n"`
						PredictedN int `json:"predicted_n"`
					} `json:"timings"`
				}
				if err := json.Unmarshal([]byte(jsonBuffer.String()), &bufferChunk); err != nil {
					jsonBuffer.Reset()
				} else {
					// Process the buffered chunk data immediately
					if len(bufferChunk.Choices) > 0 {
						if bufferChunk.Choices[0].Delta.Content != "" {
							fullContent.WriteString(bufferChunk.Choices[0].Delta.Content)
						}
						if bufferChunk.Choices[0].Delta.Reasoning != "" {
							reasoningContent.WriteString(bufferChunk.Choices[0].Delta.Reasoning)
						}
						if len(bufferChunk.Choices[0].Delta.ToolCalls) > 0 {
							for _, deltaTC := range bufferChunk.Choices[0].Delta.ToolCalls {
								targetIndex := -1
								if deltaTC.Index != nil && *deltaTC.Index >= 0 {
									targetIndex = *deltaTC.Index
								}
								if targetIndex == -1 && deltaTC.ID != "" {
									for i, tc := range toolCallsList {
										if tc.ID == deltaTC.ID {
											targetIndex = i
											break
										}
									}
								}
								if targetIndex == -1 && deltaTC.ID == "" && deltaTC.Function.Name == "" && len(toolCallsList) > 0 {
									if deltaTC.Function.Arguments != "" {
										found := false
										for i, tc := range toolCallsList {
											if tc.Arguments == "" {
												targetIndex = i
												found = true
												break
											}
										}
										if !found {
											targetIndex = lastActiveToolCallIndex
											if targetIndex < 0 {
												targetIndex = len(toolCallsList) - 1
											}
										}
									}
								}
								if targetIndex == -1 {
									targetIndex = len(toolCallsList)
								}
								for len(toolCallsList) <= targetIndex {
									toolCallsList = append(toolCallsList, &accumulatedToolCall{})
								}
								existing := toolCallsList[targetIndex]
								if deltaTC.ID != "" || deltaTC.Function.Name != "" {
									lastActiveToolCallIndex = targetIndex
								}
								if deltaTC.ID != "" {
									existing.ID = deltaTC.ID
								}
								if deltaTC.Type != "" {
									existing.Type = deltaTC.Type
								} else if existing.Type == "" {
									existing.Type = "function"
								}
								if deltaTC.Function.Name != "" {
									existing.Name = deltaTC.Function.Name
								}
								if deltaTC.Function.Arguments != "" {
									existing.Arguments += deltaTC.Function.Arguments
								}
								if callback != nil && deltaTC.Function.Name != "" && !notifiedToolCalls[targetIndex] {
									toolName := deltaTC.Function.Name
									notification := fmt.Sprintf("\n[Tool Call] %s\n", toolName)
									callback(StreamingChunk{
										Text:        notification,
										ContentType: StreamingContentTypeNormal,
									})
									notifiedToolCalls[targetIndex] = true
								}
							}
						}
						if bufferChunk.Usage.TotalTokens > 0 {
							totalTokens = bufferChunk.Usage.TotalTokens
							inputTokens = bufferChunk.Usage.PromptTokens
							outputTokens = bufferChunk.Usage.CompletionTokens
						} else if bufferChunk.Timings.CacheN+bufferChunk.Timings.PromptN > 0 {
							// llama.cpp timings format - cache_n is NOT included in prompt_n
							inputTokens = bufferChunk.Timings.CacheN + bufferChunk.Timings.PromptN
							outputTokens = bufferChunk.Timings.PredictedN
							totalTokens = inputTokens + outputTokens
						}
					}
					jsonBuffer.Reset()
				}
			} else if inJSON {
				jsonBuffer.Reset()
			}

			// Reset inJSON since we're now handling a new data line
			inJSON = false

			data := strings.TrimPrefix(line, "data: ")

			// Check for end of stream
			if data == "[DONE]" {
				break
			}

			// Reset chunk before unmarshaling to prevent stale data from persisting
			// json.Unmarshal does not clear existing slice values, so we must reset manually
			chunk = struct {
				Choices []struct {
					Delta struct {
						Content   string        `json:"content"`
						Reasoning string        `json:"reasoning"`
						ToolCalls []APIToolCall `json:"tool_calls,omitempty"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
				Usage struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
					TotalTokens      int `json:"total_tokens"`
				} `json:"usage"`
				Timings struct {
					CacheN       int `json:"cache_n"`
					PromptN      int `json:"prompt_n"`
					PredictedN   int `json:"predicted_n"`
				} `json:"timings"`
			}{}

			// Try to parse as complete JSON first
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				// If it failed, check if this is a multi-line JSON blob
				// by seeing if the data looks like it's incomplete
				jsonBuffer.WriteString(data)
				inJSON = true
				continue
			}
		} else if inJSON {
			// We are in the middle of accumulating multi-line JSON,
			// append this line (without SSE prefix) to the buffer
			jsonBuffer.WriteString(line)
			continue
		} else {
			// Skip empty lines and non-SSE data when not accumulating JSON
			continue
		}

		if len(chunk.Choices) > 0 {
			// Get content from delta
			chunkText := ""
			if chunk.Choices[0].Delta.Content != "" {
				chunkText = chunk.Choices[0].Delta.Content
				fullContent.WriteString(chunkText)
			}
			if chunk.Choices[0].Delta.Reasoning != "" {
				reasoningContent.WriteString(chunk.Choices[0].Delta.Reasoning)
				if chunkText == "" {
					chunkText = chunk.Choices[0].Delta.Reasoning
				}
			}

			// Accumulate tool calls from streaming delta
			// Tool calls come in partial chunks that need to be merged
			// Deltas may arrive with an index field indicating which tool call they belong to
			// or without an index (continuation of the current tool call being built)
			if len(chunk.Choices[0].Delta.ToolCalls) > 0 {
				for _, deltaTC := range chunk.Choices[0].Delta.ToolCalls {
					// Determine which tool call this delta belongs to
					// Priority: index field > ID lookup > continuation of last active tool call

					targetIndex := -1

					// 1. Try to use the index field if present
					if deltaTC.Index != nil && *deltaTC.Index >= 0 {
						targetIndex = *deltaTC.Index
					}

					// 2. If no index, try to find by ID
					if targetIndex == -1 && deltaTC.ID != "" {
						for i, tc := range toolCallsList {
							if tc.ID == deltaTC.ID {
								targetIndex = i
								break
							}
						}
					}

					// 3. If still not found and this is a continuation delta (no ID, no name),
					// find the first tool call that has empty arguments to match it correctly
					if targetIndex == -1 && deltaTC.ID == "" && deltaTC.Function.Name == "" && len(toolCallsList) > 0 {
						if deltaTC.Function.Arguments != "" {
							// Find the first tool call with empty arguments to match continuation correctly
							found := false
							for i, tc := range toolCallsList {
								if tc.Arguments == "" {
									targetIndex = i
									found = true
									break
								}
							}
							if !found {
								targetIndex = lastActiveToolCallIndex
								if targetIndex < 0 {
									targetIndex = len(toolCallsList) - 1
								}
							}
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

					// Update last active tool call index when we see a new tool call start
					if deltaTC.ID != "" || deltaTC.Function.Name != "" {
						lastActiveToolCallIndex = targetIndex
					}

					// Merge with existing tool call - accumulate fields
					// ID
					if deltaTC.ID != "" {
						existing.ID = deltaTC.ID
					}
					// Type - normalize empty values to "function" for Copilot compatibility
					// This prevents errors like: Invalid value: ''. Supported values are: 'function', 'allowed_tools', and 'custom'.
					if deltaTC.Type != "" {
						existing.Type = deltaTC.Type
					} else if existing.Type == "" {
						existing.Type = "function"
					}
					// Name typically comes first and doesn't change
					if deltaTC.Function.Name != "" {
						existing.Name = deltaTC.Function.Name
					}
					// Arguments are streamed as incremental JSON string fragments
					if deltaTC.Function.Arguments != "" {
						existing.Arguments += deltaTC.Function.Arguments
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
			if chunk.Choices[0].Delta.Reasoning != "" && callback != nil {
				callback(StreamingChunk{
					Text:        chunk.Choices[0].Delta.Reasoning,
					ContentType: StreamingContentTypeReasoning,
				})
			}

			// Stream normal content with appropriate type
			if chunk.Choices[0].Delta.Content != "" && callback != nil {
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
			} else if chunk.Timings.CacheN+chunk.Timings.PromptN > 0 {
				// llama.cpp timings format - cache_n is NOT included in prompt_n
				inputTokens = chunk.Timings.CacheN + chunk.Timings.PromptN
				outputTokens = chunk.Timings.PredictedN
				totalTokens = inputTokens + outputTokens
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

		// Ensure type is set for compatibility - normalize empty values to "function"
		// This prevents errors from models that don't include type in streaming deltas
		if accTC.Type == "" {
			accTC.Type = "function"
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
		Reasoning:    reasoningContent.String(), // OpenAI standard "reasoning" field
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
