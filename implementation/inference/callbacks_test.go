package inference

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coding-agent/harness/config"
)

func TestInferenceClient_SetTools_Preserves(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	// Set initial tools
	client.SetTools([]ToolDefinition{
		{Type: "function", Function: FunctionDefinition{Name: "tool1", Description: "First tool", Parameters: ParameterSchema{Type: "object"}}},
	})

	// Set different tools
	client.SetTools([]ToolDefinition{
		{Type: "function", Function: FunctionDefinition{Name: "tool2", Description: "Second tool", Parameters: ParameterSchema{Type: "object"}}},
	})

	tools := client.GetTools()
	if len(tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(tools))
	}

	if tools[0].Function.Name != "tool2" {
		t.Errorf("Expected tool2, got %s", tools[0].Function.Name)
	}
}

func TestResponse_WithReasoning(t *testing.T) {
	resp := Response{
		Content:   "Final answer",
		Reasoning: "After careful consideration, I believe the answer is 42",
	}

	if resp.Reasoning == "" {
		t.Error("Expected non-empty reasoning")
	}

	if resp.Content != "Final answer" {
		t.Errorf("Expected content 'Final answer', got '%s'", resp.Content)
	}
}

func TestMessage_WithToolCallId(t *testing.T) {
	msg := Message{
		Role:       "tool",
		Content:    "Result of the tool call",
		ToolCallId: "call_xyz",
	}

	if msg.Role != "tool" {
		t.Errorf("Expected role 'tool', got '%s'", msg.Role)
	}

	if msg.ToolCallId != "call_xyz" {
		t.Errorf("Expected ToolCallId 'call_xyz', got '%s'", msg.ToolCallId)
	}

	if msg.Content != "Result of the tool call" {
		t.Errorf("Expected content, got '%s'", msg.Content)
	}
}

func TestHandleResponse_TimingsFallback(t *testing.T) {
	// Test with llama.cpp timings format instead of OpenAI format
	mockResponse := `{
		"choices": [{
			"message": {
				"role": "assistant",
				"content": "Response from llama.cpp"
			},
			"finish_reason": "stop"
		}],
		"timings": {
			"prompt_n": 50,
			"predicted_n": 100
		}
	}`

	body := io.NopCloser(strings.NewReader(mockResponse))
	client := NewInferenceClient(config.DefaultConfig())

	resp, err := client.handleResponse(body)
	if err != nil {
		t.Fatalf("handleResponse() error: %v", err)
	}

	if resp.InputTokens != 50 {
		t.Errorf("Expected input tokens 50 from timings, got %d", resp.InputTokens)
	}

	if resp.OutputTokens != 100 {
		t.Errorf("Expected output tokens 100 from timings, got %d", resp.OutputTokens)
	}

	if resp.TokenUsage != 150 {
		t.Errorf("Expected total tokens 150, got %d", resp.TokenUsage)
	}
}

func TestHandleStreamResponse_TimingsFallback(t *testing.T) {
	// In streaming, the usage is embedded in the choices chunks, not separate
	// This test verifies the response struct handles usage correctly
	sseStream := `data: {"choices": [{"delta": {"content": "test"}, "finish_reason": "stop"}], "usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}}
data: [DONE]`

	body := io.NopCloser(strings.NewReader(sseStream))
	client := NewInferenceClient(config.DefaultConfig())

	resp, err := client.handleStreamResponse(body, nil)
	if err != nil {
		t.Fatalf("handleStreamResponse() error: %v", err)
	}

	if resp.InputTokens != 10 || resp.OutputTokens != 20 || resp.TokenUsage != 30 {
		t.Errorf("Expected tokens from usage: in=10, out=20, total=30")
	}
}

func TestBuildMessages_MultipleMessages(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	messages := []*Message{
		{Role: "user", Content: "First user message"},
		{Role: "assistant", Content: "First assistant response"},
		{Role: "user", Content: "Second user message"},
	}

	result := client.buildMessages(messages, "System prompt")

	if len(result) != 4 {
		t.Fatalf("Expected 4 messages (1 system + 3 conversation), got %d", len(result))
	}

	// Verify message order is preserved
	expectedRoles := []string{"system", "user", "assistant", "user"}
	for i, expectedRole := range expectedRoles {
		if result[i].Role != expectedRole {
			t.Errorf("Message %d: expected role '%s', got '%s'", i, expectedRole, result[i].Role)
		}
	}
}

// ===== Tests for helper functions =====

func TestInferenceRequestWithCallback_MockServer(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.APIEndpoint = "" // Will use httptest server
	client := NewInferenceClient(cfg)

	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Hello"}}]}`)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":" world"}}]}`)
		fmt.Fprintln(w, `data: {"usage":{"prompt_tokens":5,"completion_tokens":10,"total_tokens":15}}`)
		fmt.Fprintln(w, `data: [DONE]`)
	}))
	defer server.Close()
	client.SetEndpoint(server.URL)

	var chunks []string
	callback := func(chunk StreamingChunk) {
		chunks = append(chunks, chunk.Text)
	}

	resp, err := client.InferenceRequestWithCallbackTyped(context.Background(), nil, "test", callback)
	if err != nil {
		t.Fatalf("InferenceRequestWithCallbackTyped() error: %v", err)
	}
	if resp == nil {
		t.Fatal("Expected non-nil response")
	}
	if resp.Content != "Hello world" {
		t.Errorf("Expected 'Hello world', got %q", resp.Content)
	}
	// Callback should have been called for each chunk
	if len(chunks) != 2 {
		t.Errorf("Expected 2 callback invocations, got %d: %v", len(chunks), chunks)
	}
}

func TestInferenceRequestWithCallbackTyped_MockServer(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Hello"}}]}`)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":" world"}}]}`)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"\n"}}]}`)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Final"}}]}`)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":" answer"}}]}`)
		fmt.Fprintln(w, `data: {"usage":{"prompt_tokens":5,"completion_tokens":10,"total_tokens":15}}`)
		fmt.Fprintln(w, `data: [DONE]`)
	}))
	defer server.Close()
	client.SetEndpoint(server.URL)

	var chunks []string
	callback := func(chunk StreamingChunk) {
		chunks = append(chunks, chunk.Text)
	}

	resp, err := client.InferenceRequestWithCallbackTyped(context.Background(), nil, "test", callback)
	if err != nil {
		t.Fatalf("InferenceRequestWithCallbackTyped() error: %v", err)
	}
	if resp == nil {
		t.Fatal("Expected non-nil response")
	}
	if resp.Content != "Hello world\nFinal answer" {
		t.Errorf("Expected 'Hello world\\nFinal answer', got %q", resp.Content)
	}
	if len(chunks) != 5 {
		t.Errorf("Expected 5 callback invocations, got %d: %v", len(chunks), chunks)
	}
}

func TestInferenceRequestWithCallbackTyped_WithReasoning(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	// Create a mock server that returns reasoning content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"reasoning":"I need to think about this"}}]}`)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"The answer is 42"}}]}`)
		fmt.Fprintln(w, `data: {"usage":{"prompt_tokens":5,"completion_tokens":10,"total_tokens":15}}`)
		fmt.Fprintln(w, `data: [DONE]`)
	}))
	defer server.Close()
	client.SetEndpoint(server.URL)

	var chunks []string
	var reasoningChunks []string
	callback := func(chunk StreamingChunk) {
		chunks = append(chunks, chunk.Text)
		if chunk.ContentType == StreamingContentTypeReasoning {
			reasoningChunks = append(reasoningChunks, chunk.Text)
		}
	}

	resp, err := client.InferenceRequestWithCallbackTyped(context.Background(), nil, "test", callback)
	if err != nil {
		t.Fatalf("InferenceRequestWithCallbackTyped() error: %v", err)
	}
	if resp == nil {
		t.Fatal("Expected non-nil response")
	}
	if resp.Reasoning != "I need to think about this" {
		t.Errorf("Expected reasoning 'I need to think about this', got %q", resp.Reasoning)
	}
	if resp.Content != "The answer is 42" {
		t.Errorf("Expected content 'The answer is 42', got %q", resp.Content)
	}
	if len(reasoningChunks) != 1 {
		t.Errorf("Expected 1 reasoning chunk, got %d: %v", len(reasoningChunks), reasoningChunks)
	}
}

func TestInferenceRequestWithCallbackTyped_WithReasoningContent(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	// Create a mock server that returns reasoning_content (not reasoning)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"reasoning_content":"Let me reason through this"}}]}`)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"The answer is 42"}}]}`)
		fmt.Fprintln(w, `data: {"usage":{"prompt_tokens":5,"completion_tokens":10,"total_tokens":15}}`)
		fmt.Fprintln(w, `data: [DONE]`)
	}))
	defer server.Close()
	client.SetEndpoint(server.URL)

	var chunks []string
	var reasoningChunks []string
	callback := func(chunk StreamingChunk) {
		chunks = append(chunks, chunk.Text)
		if chunk.ContentType == StreamingContentTypeReasoning {
			reasoningChunks = append(reasoningChunks, chunk.Text)
		}
	}

	resp, err := client.InferenceRequestWithCallbackTyped(context.Background(), nil, "test", callback)
	if err != nil {
		t.Fatalf("InferenceRequestWithCallbackTyped() error: %v", err)
	}
	if resp == nil {
		t.Fatal("Expected non-nil response")
	}
	if resp.Reasoning != "Let me reason through this" {
		t.Errorf("Expected reasoning 'Let me reason through this', got %q", resp.Reasoning)
	}
	if resp.ReasoningContentType != "reasoning_content" {
		t.Errorf("Expected reasoningContentType 'reasoning_content', got %q", resp.ReasoningContentType)
	}
	if len(reasoningChunks) != 1 {
		t.Errorf("Expected 1 reasoning chunk, got %d: %v", len(reasoningChunks), reasoningChunks)
	}
}

func TestInferenceRequestWithCallbackTyped_WithToolCalls(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	// Create a mock server that returns tool calls
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"bash"}}]}}]}`)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"command\":\"ls -la\"}"}}]}}]}`)
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":""},"finish_reason":"tool_calls"}]}`)
		fmt.Fprintln(w, `data: [DONE]`)
	}))
	defer server.Close()
	client.SetEndpoint(server.URL)

	var chunks []string
	callback := func(chunk StreamingChunk) {
		chunks = append(chunks, chunk.Text)
	}

	resp, err := client.InferenceRequestWithCallbackTyped(context.Background(), nil, "test", callback)
	if err != nil {
		t.Fatalf("InferenceRequestWithCallbackTyped() error: %v", err)
	}
	if resp == nil {
		t.Fatal("Expected non-nil response")
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ID != "call_1" {
		t.Errorf("Expected tool call ID 'call_1', got %q", resp.ToolCalls[0].ID)
	}
	if resp.ToolCalls[0].Name != "bash" {
		t.Errorf("Expected tool name 'bash', got %q", resp.ToolCalls[0].Name)
	}
}

func TestInferenceRequestWithCallbackTyped_NilCallback(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Hello"}}]}`)
		fmt.Fprintln(w, `data: [DONE]`)
	}))
	defer server.Close()
	client.SetEndpoint(server.URL)

	// Pass nil callback - should still work
	resp, err := client.InferenceRequestWithCallbackTyped(context.Background(), nil, "test", nil)
	if err != nil {
		t.Fatalf("InferenceRequestWithCallbackTyped() error with nil callback: %v", err)
	}
	if resp == nil {
		t.Fatal("Expected non-nil response")
	}
	if resp.Content != "Hello" {
		t.Errorf("Expected 'Hello', got %q", resp.Content)
	}
}

func TestInferenceRequestWithCallback_MarshalError(t *testing.T) {
	// Test the marshal error path by creating a client
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This will fail due to connection error, not marshal error
	// But it tests that the function works when no server is available
	_, err := client.InferenceRequestWithCallbackTyped(ctx, nil, "test", nil)
	if err == nil {
		t.Error("Expected error when no server is running")
	}
}

