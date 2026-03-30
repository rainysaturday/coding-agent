package inference

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"coding-agent/context"
	"coding-agent/stats"
)

// Message represents a chat message for the API
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolDefinition represents a tool definition for the API
type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction represents the function definition for a tool
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// Request represents a chat completion request
type Request struct {
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	Stream      bool             `json:"stream"`
	MaxTokens   int              `json:"max_tokens"`
	Temperature float64          `json:"temperature"`
	ToolChoice  string           `json:"tool_choice,omitempty"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
}

// Response represents a chat completion response
type Response struct {
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a choice in the response
type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamResponse represents a streaming response chunk
type StreamResponse struct {
	Choices []StreamChoice `json:"choices"`
}

// StreamChoice represents a streaming choice
type StreamChoice struct {
	Delta        Message `json:"delta"`
	FinishReason string  `json:"finish_reason"`
}

// InferenceClient handles communication with the inference backend
type InferenceClient struct {
	endpoint            string
	apiKey              string
	model               string
	maxTokens           int
	initialTokenTimeout int
	streamingEnabled    bool
	connectionTimeout   int
	readTimeout         int
	httpClient          *http.Client
	stats               *stats.Stats
}

// NewInferenceClient creates a new inference client
func NewInferenceClient(endpoint, apiKey, model string, maxTokens, initialTokenTimeout, connectionTimeout, readTimeout int, streamingEnabled bool, stats *stats.Stats) *InferenceClient {
	return &InferenceClient{
		endpoint:            endpoint,
		apiKey:              apiKey,
		model:               model,
		maxTokens:           maxTokens,
		initialTokenTimeout: initialTokenTimeout,
		streamingEnabled:    streamingEnabled,
		connectionTimeout:   connectionTimeout,
		readTimeout:         readTimeout,
		httpClient: &http.Client{
			Timeout: time.Duration(connectionTimeout) * time.Second,
		},
		stats: stats,
	}
}

// ChatCompletionRequest represents a request for chat completion
type ChatCompletionRequest struct {
	Context    *context.Context
	OnToken    func(token string)
	OnComplete func()
	OnError    func(error)
}

// ChatCompletion executes a chat completion request
func (c *InferenceClient) ChatCompletion(req ChatCompletionRequest) error {
	// Prepare messages - convert context.Message to inference.Message
	ctxMessages := req.Context.FormatForAPI()
	messages := make([]Message, len(ctxMessages))
	for i, m := range ctxMessages {
		messages[i] = Message{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	// Create request body
	request := Request{
		Model:       c.model,
		Messages:    messages,
		Stream:      c.streamingEnabled,
		MaxTokens:   c.maxTokens,
		Temperature: 0.7,
	}

	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", c.endpoint+"/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" && c.apiKey != "not-needed" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Execute request
	if c.streamingEnabled {
		return c.streamCompletion(httpReq, req)
	}
	return c.nonStreamCompletion(httpReq, req)
}

func (c *InferenceClient) nonStreamCompletion(req *http.Request, chatReq ChatCompletionRequest) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Choices) == 0 {
		return fmt.Errorf("no choices in response")
	}

	// Update stats
	c.stats.AddInputTokens(int64(response.Usage.PromptTokens))
	c.stats.AddOutputTokens(int64(response.Usage.CompletionTokens))

	// Add assistant message to context
	chatReq.Context.AddAssistantMessage(response.Choices[0].Message.Content)

	if chatReq.OnToken != nil {
		chatReq.OnToken(response.Choices[0].Message.Content)
	}
	if chatReq.OnComplete != nil {
		chatReq.OnComplete()
	}

	return nil
}

func (c *InferenceClient) streamCompletion(req *http.Request, chatReq ChatCompletionRequest) error {
	// Create a new client with streaming timeout
	client := &http.Client{
		Timeout: time.Duration(c.initialTokenTimeout) * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse streaming response
	scanner := bufio.NewScanner(resp.Body)
	var fullContent strings.Builder
	var totalTokens int

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var streamResp StreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue // Skip invalid JSON
			}

			if len(streamResp.Choices) > 0 {
				delta := streamResp.Choices[0].Delta
				if delta.Content != "" {
					fullContent.WriteString(delta.Content)
					totalTokens++

					if chatReq.OnToken != nil {
						chatReq.OnToken(delta.Content)
					}
				}

				if streamResp.Choices[0].FinishReason != "" {
					break
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stream: %w", err)
	}

	// Update stats (estimate tokens)
	c.stats.AddOutputTokens(int64(totalTokens))

	// Add assistant message to context
	chatReq.Context.AddAssistantMessage(fullContent.String())

	if chatReq.OnComplete != nil {
		chatReq.OnComplete()
	}

	return nil
}
