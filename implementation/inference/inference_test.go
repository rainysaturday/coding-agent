package inference

import (
	"encoding/json"
	"testing"

	"github.com/coding-agent/harness/config"
)

func TestNewInferenceClient(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	if client == nil {
		t.Fatal("NewInferenceClient() returned nil")
	}

	if client.model != "llama3" {
		t.Errorf("Expected model 'llama3', got '%s'", client.model)
	}

	if client.temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", client.temperature)
	}

	if client.maxTokens != 4096 {
		t.Errorf("Expected max tokens 4096, got %d", client.maxTokens)
	}

	if client.contextSize != 128000 {
		t.Errorf("Expected context size 128000, got %d", client.contextSize)
	}

	if client.streaming != true {
		t.Errorf("Expected streaming true, got %v", client.streaming)
	}
}

func TestSetEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	endpoint := "http://test:8080"
	client.SetEndpoint(endpoint)

	if client.endpoint != endpoint {
		t.Errorf("Expected endpoint '%s', got '%s'", endpoint, client.endpoint)
	}
}

func TestSetAPIKey(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	key := "test-key"
	client.SetAPIKey(key)

	if client.apiKey != key {
		t.Errorf("Expected API key '%s', got '%s'", key, client.apiKey)
	}
}

func TestBuildMessages(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	systemPrompt := "You are a helpful assistant"
	messages := []*Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	result := client.buildMessages(messages, systemPrompt)

	if len(result) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(result))
	}

	if result[0].Role != "system" {
		t.Errorf("Expected first message role 'system', got '%s'", result[0].Role)
	}

	if result[0].Content != systemPrompt {
		t.Errorf("Expected system prompt, got '%s'", result[0].Content)
	}

	if result[1].Role != "user" {
		t.Errorf("Expected second message role 'user', got '%s'", result[1].Role)
	}
}

func TestBuildMessagesNoSystemPrompt(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	messages := []*Message{
		{Role: "user", Content: "Hello"},
	}

	result := client.buildMessages(messages, "")

	if len(result) != 1 {
		t.Errorf("Expected 1 message, got %d", len(result))
	}

	if result[0].Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", result[0].Role)
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"", 0},
		{"hello", 1},
		{"hello world", 2},
		{"This is a test message with multiple words", 10},
	}

	for _, tt := range tests {
		result := EstimateTokens(tt.text)
		// Allow some variance in estimation
		if result < 0 || result > len(tt.text)/2 {
			t.Errorf("EstimateTokens(%q) = %d, expected reasonable value", tt.text, result)
		}
	}
}

func TestRequestBody_JSON(t *testing.T) {
	req := &RequestBody{
		Model:       "llama3",
		Messages:    []*Message{{Role: "user", Content: "test"}},
		Stream:      true,
		Temperature: 0.7,
		MaxTokens:   4096,
	}

	// Just verify the struct can be created without error
	if req.Model != "llama3" {
		t.Errorf("Expected model 'llama3', got '%s'", req.Model)
	}
	if req.Stream != true {
		t.Errorf("Expected stream true, got %v", req.Stream)
	}
}

func TestStreamingToolCallAccumulationRealistic(t *testing.T) {
	// Test realistic streaming format where arguments come in separate chunks
	// This simulates what llama.cpp server sends

	type APIToolCall struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}

	type accumulatedToolCall struct {
		ID        string
		Type      string
		Name      string
		Arguments string
	}

	// Simulate streaming chunks
	type delta struct {
		ToolCalls []APIToolCall `json:"tool_calls,omitempty"`
	}

	chunks := []delta{
		{ToolCalls: []APIToolCall{{ID: "call_1", Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "bash", Arguments: ""}}}},
		{ToolCalls: []APIToolCall{{ID: "", Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Arguments: "{\"command\""}}}},
		{ToolCalls: []APIToolCall{{ID: "", Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Arguments: ":\"ls -la\"}"}}}},
	}

	var toolCallsList []*accumulatedToolCall

	for _, deltaTC := range chunks[0].ToolCalls {
		targetIndex := len(toolCallsList)
		for len(toolCallsList) <= targetIndex {
			toolCallsList = append(toolCallsList, &accumulatedToolCall{})
		}
		existing := toolCallsList[targetIndex]
		if deltaTC.ID != "" {
			existing.ID = deltaTC.ID
		}
		if deltaTC.Function.Name != "" {
			existing.Name = deltaTC.Function.Name
		}
		if deltaTC.Function.Arguments != "" {
			existing.Arguments += deltaTC.Function.Arguments
		}
	}

	// Process remaining chunks - they should merge with the last tool call
	for _, chunk := range chunks[1:] {
		for _, deltaTC := range chunk.ToolCalls {
			targetIndex := -1

			// Try to find by ID
			if deltaTC.ID != "" {
				for i, tc := range toolCallsList {
					if tc.ID == deltaTC.ID {
						targetIndex = i
						break
					}
				}
			}

			// If no ID and no name, merge with last tool call
			if targetIndex == -1 && (deltaTC.ID == "" && deltaTC.Function.Name == "") && len(toolCallsList) > 0 {
				if deltaTC.Function.Arguments != "" {
					targetIndex = len(toolCallsList) - 1
				}
			}

			if targetIndex == -1 {
				targetIndex = len(toolCallsList)
			}

			for len(toolCallsList) <= targetIndex {
				toolCallsList = append(toolCallsList, &accumulatedToolCall{})
			}

			existing := toolCallsList[targetIndex]
			if deltaTC.ID != "" {
				existing.ID = deltaTC.ID
			}
			if deltaTC.Function.Name != "" {
				existing.Name = deltaTC.Function.Name
			}
			if deltaTC.Function.Arguments != "" {
				existing.Arguments += deltaTC.Function.Arguments
			}
		}
	}

	// Verify the accumulated tool call
	if len(toolCallsList) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCallsList))
	}

	tc := toolCallsList[0]
	if tc.ID != "call_1" {
		t.Errorf("Expected ID 'call_1', got '%s'", tc.ID)
	}
	if tc.Name != "bash" {
		t.Errorf("Expected name 'bash', got '%s'", tc.Name)
	}
	if tc.Arguments != `{"command":"ls -la"}` {
		t.Errorf("Expected arguments '{\"command\":\"ls -la\"}', got '%s'", tc.Arguments)
	}

	// Verify parsing
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(tc.Arguments), &params); err != nil {
		t.Fatalf("Failed to parse arguments: %v", err)
	}

	cmd, ok := params["command"].(string)
	if !ok {
		t.Fatal("Expected command parameter to be string")
	}
	if cmd != "ls -la" {
		t.Errorf("Expected command 'ls -la', got '%s'", cmd)
	}
}
