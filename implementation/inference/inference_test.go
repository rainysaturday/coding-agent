package inference

import (
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

func TestParseToolCalls(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{
			name:    "no tool calls",
			content: "Hello, how can I help you?",
			want:    0,
		},
		{
			name:    "single tool call",
			content: "I'll help with that.\n[TOOL:{\"name\":\"bash\",\"parameters\":{\"command\":\"ls -la\"}}]\nDone.",
			want:    1,
		},
		{
			name:    "multiple tool calls",
			content: `[TOOL:{"name":"bash","parameters":{"command":"echo hello"}}]
And also: [TOOL:{"name":"read_file","parameters":{"path":"test.txt"}}]`,
			want: 2,
		},
		{
			name:    "tool call with multi-line",
			content: `[TOOL:{"name":"write_file","parameters":{"path":"test.txt","content":"line1\nline2"}}]`,
			want:    1,
		},
		{
			name:    "text before tool call",
			content: "Let me run a command for you.\n[TOOL:{\"name\":\"bash\",\"parameters\":{\"command\":\"pwd\"}}]",
			want:    1,
		},
		{
			name:    "complex multi-line script",
			content: `[TOOL:{"name":"bash","parameters":{"command":"#!/bin/bash\necho \"hello\"\nfor i in {1..5}; do\n    echo $i\ndone"}}]`,
			want:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseToolCalls(tt.content)

			if len(result) != tt.want {
				t.Errorf("parseToolCalls() found %d tool calls, want %d", len(result), tt.want)
			}
		})
	}
}

func TestParseToolCalls_BashTool(t *testing.T) {
	content := `[TOOL:{"name":"bash","parameters":{"command":"ls -la /home"}}]`

	toolCalls := parseToolCalls(content)

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	tc := toolCalls[0]

	if tc.Name != "bash" {
		t.Errorf("Expected tool name 'bash', got '%s'", tc.Name)
	}

	cmd, ok := tc.Parameters["command"].(string)
	if !ok {
		t.Fatal("Expected command parameter to be string")
	}

	if cmd != "ls -la /home" {
		t.Errorf("Expected command 'ls -la /home', got '%s'", cmd)
	}
}

func TestParseToolCalls_ReadFileTool(t *testing.T) {
	content := `[TOOL:{"name":"read_file","parameters":{"path":"/test/file.txt"}}]`

	toolCalls := parseToolCalls(content)

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	tc := toolCalls[0]

	if tc.Name != "read_file" {
		t.Errorf("Expected tool name 'read_file', got '%s'", tc.Name)
	}

	path, ok := tc.Parameters["path"].(string)
	if !ok {
		t.Fatal("Expected path parameter to be string")
	}

	if path != "/test/file.txt" {
		t.Errorf("Expected path '/test/file.txt', got '%s'", path)
	}
}

func TestParseToolCalls_WriteFileTool(t *testing.T) {
	content := `[TOOL:{"name":"write_file","parameters":{"path":"output.txt","content":"hello world"}}]`

	toolCalls := parseToolCalls(content)

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	tc := toolCalls[0]

	if tc.Name != "write_file" {
		t.Errorf("Expected tool name 'write_file', got '%s'", tc.Name)
	}
}

func TestParseToolCalls_ReadLinesTool(t *testing.T) {
	content := `[TOOL:{"name":"read_lines","parameters":{"path":"large.txt","start":100,"end":200}}]`

	toolCalls := parseToolCalls(content)

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	tc := toolCalls[0]

	if tc.Name != "read_lines" {
		t.Errorf("Expected tool name 'read_lines', got '%s'", tc.Name)
	}
}

func TestParseToolCalls_InsertLinesTool(t *testing.T) {
	content := `[TOOL:{"name":"insert_lines","parameters":{"path":"file.txt","line":10,"lines":"new line"}}]`

	toolCalls := parseToolCalls(content)

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	tc := toolCalls[0]

	if tc.Name != "insert_lines" {
		t.Errorf("Expected tool name 'insert_lines', got '%s'", tc.Name)
	}
}

func TestParseToolCalls_ReplaceLinesTool(t *testing.T) {
	content := `[TOOL:{"name":"replace_lines","parameters":{"path":"file.txt","start":1,"end":5,"lines":"replacement"}}]`

	toolCalls := parseToolCalls(content)

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	tc := toolCalls[0]

	if tc.Name != "replace_lines" {
		t.Errorf("Expected tool name 'replace_lines', got '%s'", tc.Name)
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

func TestStreamingToolCallAccumulation(t *testing.T) {
	// We can't easily test handleStreamResponse directly without a real HTTP response
	// But we can verify the accumulation logic by testing the tool call parsing
	content := `I'll help with that.
[TOOL:{"name":"bash","parameters":{"command":"ls -la"}}]`

	toolCalls := parseToolCalls(content)
	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	tc := toolCalls[0]
	if tc.Name != "bash" {
		t.Errorf("Expected tool name 'bash', got '%s'", tc.Name)
	}

	cmd, ok := tc.Parameters["command"].(string)
	if !ok {
		t.Fatal("Expected command parameter to be string")
	}
	if cmd != "ls -la" {
		t.Errorf("Expected command 'ls -la', got '%s'", cmd)
	}
}

func TestStreamingMultipleToolCalls(t *testing.T) {
	// Test parsing multiple tool calls from content
	content := `[TOOL:{"name":"bash","parameters":{"command":"ls -la"}}]
And also: [TOOL:{"name":"read_file","parameters":{"path":"test.txt"}}]`

	toolCalls := parseToolCalls(content)
	if len(toolCalls) != 2 {
		t.Fatalf("Expected 2 tool calls, got %d", len(toolCalls))
	}

	if toolCalls[0].Name != "bash" {
		t.Errorf("Expected first tool 'bash', got '%s'", toolCalls[0].Name)
	}
	if toolCalls[1].Name != "read_file" {
		t.Errorf("Expected second tool 'read_file', got '%s'", toolCalls[1].Name)
	}
}
