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

// InferenceClient handles communication with the LLM backend.
type InferenceClient struct {
	endpoint       string
	apiKey         string
	model          string
	temperature    float64
	maxTokens      int
	contextSize    int
	streaming      bool
	timeout        time.Duration
	client         *http.Client
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolDefinition represents a tool definition for the LLM.
type ToolDefinition struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Parameters  ParameterSchema  `json:"parameters"`
}

// ParameterSchema defines the schema for tool parameters.
type ParameterSchema struct {
	Type       string   `json:"type"`
	Required   []string `json:"required"`
	Properties map[string]Property `json:"properties"`
}

// Property defines a single property in the schema.
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// Response represents an inference response.
type Response struct {
	Content     string
	ToolCalls   []*tools.ToolCall
	TokenUsage  int
	StreamUsage int
}

// NewInferenceClient creates a new inference client.
func NewInferenceClient(cfg *config.Config) *InferenceClient {
	return &InferenceClient{
		endpoint:    cfg.APIEndpoint,
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		temperature: cfg.Temperature,
		maxTokens:   cfg.MaxTokens,
		contextSize: cfg.ContextSize,
		streaming:   cfg.Streaming,
		timeout:     time.Duration(cfg.InitialTokenTimeout) * time.Second,
		client: &http.Client{
			Timeout: time.Duration(cfg.InitialTokenTimeout) * time.Second,
		},
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

// InferenceRequest sends a request to the inference backend.
func (ic *InferenceClient) InferenceRequest(ctx context.Context, messages []*Message, systemPrompt string) (*Response, error) {
	// Build the request
	reqBody := &RequestBody{
		Model:       ic.model,
		Messages:    ic.buildMessages(messages, systemPrompt),
		Stream:      ic.streaming,
		Temperature: ic.temperature,
		MaxTokens:   ic.maxTokens,
	}

	// Serialize request
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
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
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	// Handle streaming or non-streaming response
	if ic.streaming {
		return ic.handleStreamResponse(resp.Body)
	}
	return ic.handleResponse(resp.Body)
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
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(respBody.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	content := respBody.Choices[0].Message.Content

	// Parse tool calls from content
	toolCalls := parseToolCalls(content)

	return &Response{
		Content:    content,
		ToolCalls:  toolCalls,
		TokenUsage: respBody.Usage.TotalTokens,
	}, nil
}

// handleStreamResponse handles a streaming response.
func (ic *InferenceClient) handleStreamResponse(body io.Reader) (*Response, error) {
	var fullContent strings.Builder
	var totalTokens int

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
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 {
			fullContent.WriteString(chunk.Choices[0].Delta.Content)
			totalTokens = chunk.Usage.TotalTokens
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("stream error: %w", err)
	}

	content := fullContent.String()
	toolCalls := parseToolCalls(content)

	return &Response{
		Content:    content,
		ToolCalls:  toolCalls,
		TokenUsage: totalTokens,
	}, nil
}

// parseToolCalls parses tool calls from the response content.
func parseToolCalls(content string) []*tools.ToolCall {
	var toolCalls []*tools.ToolCall

	// Find all tool call patterns
	start := 0
	for {
		idx := strings.Index(content[start:], "[TOOL:")
		if idx == -1 {
			break
		}

		// Find the closing bracket
		endIdx := start + idx + 7 // Skip "[TOOL:"
		bracketCount := 1
		inString := false
		escapeNext := false

		for i := endIdx; i < len(content); i++ {
			char := content[i]

			if escapeNext {
				escapeNext = false
				continue
			}

			if char == '\\' {
				escapeNext = true
				continue
			}

			if char == '"' && !escapeNext {
				inString = !inString
				continue
			}

			if !inString {
				if char == '{' {
					bracketCount++
				} else if char == '}' {
					bracketCount--
					if bracketCount == 0 {
						// Found the end of the JSON object
						jsonStr := content[start+idx : i+1]
						fullCall := "[" + jsonStr + "]"

						tc, err := tools.ParseToolCall(fullCall)
						if err == nil {
							toolCalls = append(toolCalls, tc)
						}
						break
					}
				} else if char == ']' && bracketCount == 1 {
					// Edge case: ] closes the wrapper
					jsonStr := content[start+idx : i]
					tc, err := tools.ParseToolCall(jsonStr)
					if err == nil {
						toolCalls = append(toolCalls, tc)
					}
					break
				}
			}
		}

		start = start + idx + 1
	}

	return toolCalls
}

// RequestBody represents the request body for the inference API.
type RequestBody struct {
	Model       string    `json:"model"`
	Messages    []*Message `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	ToolChoice  string    `json:"tool_choice,omitempty"`
}

// EstimateTokens estimates the number of tokens in text.
func EstimateTokens(text string) int {
	// Rough estimate: 1 token ≈ 4 characters
	return len(text) / 4
}
