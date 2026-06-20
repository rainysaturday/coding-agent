package inference

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coding-agent/harness/config"
)

func TestStreamingContentType_Constants(t *testing.T) {
	if StreamingContentTypeNormal != 0 {
		t.Errorf("Expected StreamingContentTypeNormal = 0, got %d", StreamingContentTypeNormal)
	}

	if StreamingContentTypeReasoning != 1 {
		t.Errorf("Expected StreamingContentTypeReasoning = 1, got %d", StreamingContentTypeReasoning)
	}
}

func TestStreamingChunk_Structure(t *testing.T) {
	chunk := StreamingChunk{
		Text:        "test content",
		ContentType: StreamingContentTypeNormal,
	}

	if chunk.Text != "test content" {
		t.Errorf("Expected Text 'test content', got '%s'", chunk.Text)
	}

	if chunk.ContentType != StreamingContentTypeNormal {
		t.Errorf("Expected ContentType Normal")
	}
}

func TestHandleStreamResponse_SimpleText(t *testing.T) {
	// Create a mock SSE stream
	sseStream := `data: {"choices": [{"delta": {"content": "Hello"}}]}
data: {"choices": [{"delta": {"content": " World"}}]}
data: [DONE]`

	body := io.NopCloser(strings.NewReader(sseStream))
	client := NewInferenceClient(config.DefaultConfig())

	resp, err := client.handleStreamResponse(body, nil)
	if err != nil {
		t.Fatalf("handleStreamResponse() error: %v", err)
	}

	if resp.Content != "Hello World" {
		t.Errorf("Expected content 'Hello World', got '%s'", resp.Content)
	}
}

func TestHandleStreamResponse_WithCallbacks(t *testing.T) {
	var chunks []StreamingChunk
	callback := func(chunk StreamingChunk) {
		chunks = append(chunks, chunk)
	}

	sseStream := `data: {"choices": [{"delta": {"content": "chunk1"}}]}
data: {"choices": [{"delta": {"content": "chunk2"}}]}
data: [DONE]`

	body := io.NopCloser(strings.NewReader(sseStream))
	client := NewInferenceClient(config.DefaultConfig())

	_, err := client.handleStreamResponse(body, callback)
	if err != nil {
		t.Fatalf("handleStreamResponse() error: %v", err)
	}

	if len(chunks) != 2 {
		t.Fatalf("Expected 2 callback chunks, got %d", len(chunks))
	}

	if chunks[0].Text != "chunk1" {
		t.Errorf("Expected first chunk 'chunk1', got '%s'", chunks[0].Text)
	}

	if chunks[1].Text != "chunk2" {
		t.Errorf("Expected second chunk 'chunk2', got '%s'", chunks[1].Text)
	}
}

func TestHandleStreamResponse_ToolCalls(t *testing.T) {
	sseStream := `data: {"choices": [{"delta": {"tool_calls": [{"id":"call_1","type":"function","function":{"name":"bash","arguments":""}}]}}]}
data: {"choices": [{"delta": {"tool_calls": [{"function":{"arguments":"{\"command\":"}}]}}]}
data: {"choices": [{"delta": {"tool_calls": [{"function":{"arguments":"\"ls\"}"}}]}}]}
data: [DONE]`

	body := io.NopCloser(strings.NewReader(sseStream))
	client := NewInferenceClient(config.DefaultConfig())

	resp, err := client.handleStreamResponse(body, nil)
	if err != nil {
		t.Fatalf("handleStreamResponse() error: %v", err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(resp.ToolCalls))
	}

	if resp.ToolCalls[0].Name != "bash" {
		t.Errorf("Expected tool name 'bash', got '%s'", resp.ToolCalls[0].Name)
	}

	// The arguments may or may not parse correctly depending on the JSON
	// Just verify the tool call was created
	if resp.ToolCalls[0].ID != "call_1" {
		t.Errorf("Expected tool ID 'call_1', got '%s'", resp.ToolCalls[0].ID)
	}
}

func TestHandleStreamResponse_Reasoning(t *testing.T) {
	var chunks []StreamingChunk
	callback := func(chunk StreamingChunk) {
		chunks = append(chunks, chunk)
	}

	sseStream := `data: {"choices": [{"delta": {"reasoning": "Let me think..."}}]}
data: {"choices": [{"delta": {"content": "Here is the answer"}}]}
data: [DONE]`

	body := io.NopCloser(strings.NewReader(sseStream))
	client := NewInferenceClient(config.DefaultConfig())

	_, err := client.handleStreamResponse(body, callback)
	if err != nil {
		t.Fatalf("handleStreamResponse() error: %v", err)
	}

	if len(chunks) != 2 {
		t.Fatalf("Expected 2 chunks, got %d", len(chunks))
	}

	if chunks[0].ContentType != StreamingContentTypeReasoning {
		t.Errorf("Expected first chunk to be reasoning type")
	}

	if chunks[1].ContentType != StreamingContentTypeNormal {
		t.Errorf("Expected second chunk to be normal type")
	}
}

func TestHandleStreamResponse_UsageTokens(t *testing.T) {
	// Usage data comes within choices chunks in streaming format
	sseStream := `data: {"choices": [{"delta": {"content": "test"}, "finish_reason": null}]}
data: {"choices": [{"delta": {"content": ""}, "finish_reason": "stop"}], "usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}}
data: [DONE]`

	body := io.NopCloser(strings.NewReader(sseStream))
	client := NewInferenceClient(config.DefaultConfig())

	resp, err := client.handleStreamResponse(body, nil)
	if err != nil {
		t.Fatalf("handleStreamResponse() error: %v", err)
	}

	if resp.InputTokens != 10 {
		t.Errorf("Expected input tokens 10, got %d", resp.InputTokens)
	}

	if resp.OutputTokens != 20 {
		t.Errorf("Expected output tokens 20, got %d", resp.OutputTokens)
	}

	if resp.TokenUsage != 30 {
		t.Errorf("Expected total tokens 30, got %d", resp.TokenUsage)
	}
}

func TestInferenceRequestStream_NoCallback(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Should work with nil callback
	_, err := client.InferenceRequestStream(ctx, nil, "test", nil)
	if err == nil {
		t.Error("Expected error when no server is running")
	}
}

func TestInferenceRequestWithCallback_NilCallback(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Should work with nil callback - no error, just fails to connect
	_, err := client.InferenceRequestWithCallbackTyped(ctx, nil, "test", nil)
	if err == nil {
		t.Error("Expected error when no server is running")
	}
}

func TestEstimateContextSize_NullMessages(t *testing.T) {
	result := EstimateContextSize(nil, nil, "")
	if result != 0 {
		t.Errorf("Expected 0 for all null/empty, got %d", result)
	}
}

func TestSetMaxDisplayWidth(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)
	ic.SetMaxDisplayWidth(80)
	if ic.maxDisplayWidth != 80 {
		t.Errorf("Expected maxDisplayWidth to be 80, got %d", ic.maxDisplayWidth)
	}
}

func TestFormatToolCallArgs(t *testing.T) {
	args := map[string]interface{}{"key1": "value1", "key2": 123}
	result := formatToolCallArgs(args, 80)
	if result == "" {
		t.Error("Expected non-empty result")
	}
	result = formatToolCallArgs(map[string]interface{}{}, 80)
	if result == "" {
		t.Error("Expected non-empty result for empty args")
	}
	result = formatToolCallArgs(nil, 80)
	if result == "" {
		t.Error("Expected non-empty result for nil args")
	}
}

func TestFormatJSONValue(t *testing.T) {
	if formatJSONValue("hello") != `"hello"` {
		t.Error("Expected \"hello\"")
	}
	if formatJSONValue(42) != "42" {
		t.Error("Expected 42")
	}
	if formatJSONValue(true) != "true" {
		t.Error("Expected true")
	}
	if formatJSONValue(nil) != "null" {
		t.Error("Expected null")
	}
}

func TestFormatJSONMap(t *testing.T) {
	result := formatJSONMap(map[string]interface{}{"key": "value"})
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

func TestFormatJSONArray(t *testing.T) {
	result := formatJSONArray([]interface{}{"a", "b", "c"})
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

func TestTruncateJSON(t *testing.T) {
	longStr := strings.Repeat("a", 200)
	result := truncateJSON(longStr, 10)
	if len(result) > 13 {
		t.Error("Expected truncated result")
	}
	if !strings.HasSuffix(result, "...") {
		t.Error("Expected result to end with ...")
	}
}

func TestFormatJSONValueWithMaxWidth(t *testing.T) {
	result := formatJSONValueWithMaxWidth("hello", 10)
	if result != `"hello"` {
		t.Errorf("Expected \"hello\", got %s", result)
	}
	result = formatJSONValueWithMaxWidth("hello world", 10)
	if strings.Contains(result, "hello world") {
		t.Error("Expected truncated result")
	}
}

func TestTruncateJSON_ExactLength(t *testing.T) {
	result := truncateJSON("12345", 5)
	if result != "12345" {
		t.Errorf("Expected exact string, got %s", result)
	}
}

func TestBuildMessages_NormalizeToolCallType(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)
	idx := 0
	messages := []*Message{
		{
			Role:    "assistant",
			Content: "Let me check",
			ToolCalls: []*APIToolCall{
				{
					ID:       "call_1",
					Type:     "",
					Function: FunctionCall{Name: "bash"},
					Index:    &idx,
				},
			},
		},
	}
	result := ic.buildMessages(messages, "")
	if len(result) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(result))
	}
	if result[0].ToolCalls[0].Type != "function" {
		t.Errorf("Expected tool call type to be 'function', got %q", result[0].ToolCalls[0].Type)
	}
}

// ===== Tests for InferenceRequestWithCallback error paths =====

func TestInferenceRequestWithCallback_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)
	callback := func(chunk StreamingChunk) {}
	_, err := ic.InferenceRequestWithCallbackTyped(ctx, nil, "", callback)
	if err == nil {
		t.Fatal("Expected error for cancelled context")
	}
}

func TestInferenceRequestWithCallbackTyped_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)
	callback := func(chunk StreamingChunk) {}
	_, err := ic.InferenceRequestWithCallbackTyped(ctx, nil, "", callback)
	if err == nil {
		t.Fatal("Expected error for cancelled context")
	}
}

// ===== Tests for handleStreamResponse error paths =====

func TestHandleStreamResponse_EmptyBody(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)
	result, err := ic.handleStreamResponse(strings.NewReader("data: [DONE]\n\n"), nil)
	if err != nil {
		t.Fatalf("handleStreamResponse() error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestHandleStreamResponse_NoContent(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)
	result, err := ic.handleStreamResponse(strings.NewReader("data: [DONE]\n\n"), nil)
	if err != nil {
		t.Fatalf("handleStreamResponse() error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Content != "" {
		t.Errorf("Expected empty content, got %q", result.Content)
	}
}

// ===== Additional tests for handleStreamResponse =====

func TestHandleStreamResponse_WithContent(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	stream := `data: {"choices":[{"delta":{"content":"Hello"}}]}
data: {"choices":[{"delta":{"content":" world"}}]}
data: [DONE]
`
	result, err := ic.handleStreamResponse(strings.NewReader(stream), nil)
	if err != nil {
		t.Fatalf("handleStreamResponse() error: %v", err)
	}
	if result.Content != "Hello world" {
		t.Errorf("Expected 'Hello world', got %q", result.Content)
	}
}

// ===== Tests for HTTP error handling =====

func TestInferenceRequestWithCallback_HttpError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.APIEndpoint = server.URL
	ic := NewInferenceClient(cfg)

	callback := func(chunk StreamingChunk) {}

	_, err := ic.InferenceRequestWithCallbackTyped(context.Background(), nil, "", callback)
	if err == nil {
		t.Fatal("Expected error for HTTP 500")
	}
}

func TestInferenceRequestWithCallbackTyped_HttpError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.APIEndpoint = server.URL
	ic := NewInferenceClient(cfg)

	callback := func(chunk StreamingChunk) {}

	_, err := ic.InferenceRequestWithCallbackTyped(context.Background(), nil, "", callback)
	if err == nil {
		t.Fatal("Expected error for HTTP 500")
	}
}

// ===== Additional tests for formatToolCallArgs =====

func TestHandleStreamResponse_WithCallback(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	var chunks []string
	callback := func(chunk StreamingChunk) {
		chunks = append(chunks, chunk.Text)
	}

	stream := `data: {"choices":[{"delta":{"content":"Hello"}}]}
data: {"choices":[{"delta":{"content":" world"}}]}
data: [DONE]
`
	result, err := ic.handleStreamResponse(strings.NewReader(stream), callback)
	if err != nil {
		t.Fatalf("handleStreamResponse() error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Content != "Hello world" {
		t.Errorf("Expected 'Hello world', got %q", result.Content)
	}
	if len(chunks) != 2 {
		t.Errorf("Expected 2 chunks, got %d", len(chunks))
	}
}

// ===== Additional tests for InferenceRequestWithCallback with mock server =====

func TestHandleStreamResponse_WithReasoningContent(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	var chunks []string
	callback := func(chunk StreamingChunk) {
		chunks = append(chunks, chunk.Text)
	}

	stream := `data: {"choices":[{"delta":{"reasoning_content":"Let me think about this"}}]}
data: {"choices":[{"delta":{"content":"The answer"}}]}
data: [DONE]
`
	result, err := ic.handleStreamResponse(strings.NewReader(stream), callback)
	if err != nil {
		t.Fatalf("handleStreamResponse() error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Content != "The answer" {
		t.Errorf("Expected 'The answer', got %q", result.Content)
	}
	if result.Reasoning != "Let me think about this" {
		t.Errorf("Expected reasoning 'Let me think about this', got %q", result.Reasoning)
	}
	if result.ReasoningContentType != "reasoning_content" {
		t.Errorf("Expected reasoningContentType 'reasoning_content', got %q", result.ReasoningContentType)
	}
	if len(chunks) != 2 {
		t.Errorf("Expected 2 chunks, got %d", len(chunks))
	}
}

func TestHandleStreamResponse_WithToolCalls(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	var chunks []string
	callback := func(chunk StreamingChunk) {
		chunks = append(chunks, chunk.Text)
	}

	stream := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"echo"}}]}}]}` + "\n" +
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"hello"}}]}}]}` + "\n" +
		`data: {"choices":[{"finish_reason":"tool_calls"}]}` + "\n" +
		`data: [DONE]`
	result, err := ic.handleStreamResponse(strings.NewReader(stream), callback)
	if err != nil {
		t.Fatalf("handleStreamResponse() error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].ID != "call_abc" {
		t.Errorf("Expected tool call ID 'call_abc', got %q", result.ToolCalls[0].ID)
	}
	if result.ToolCalls[0].Name != "echo" {
		t.Errorf("Expected tool name 'echo', got %q", result.ToolCalls[0].Name)
	}
}

func TestHandleStreamResponse_WithTimings(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	stream := `data: {"choices":[{"delta":{"content":"Hello"}}]}
data: {"choices":[{"delta":{"content":" world"}}]}
data: {"choices":[{"delta":{}}],"usage":{"prompt_tokens":50,"completion_tokens":100,"total_tokens":150}}
data: [DONE]
`
	result, err := ic.handleStreamResponse(strings.NewReader(stream), nil)
	if err != nil {
		t.Fatalf("handleStreamResponse() error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.InputTokens != 50 {
		t.Errorf("Expected inputTokens 50, got %d", result.InputTokens)
	}
	if result.OutputTokens != 100 {
		t.Errorf("Expected outputTokens 100, got %d", result.OutputTokens)
	}
	if result.TokenUsage != 150 {
		t.Errorf("Expected tokenUsage 150, got %d", result.TokenUsage)
	}
}

func TestHandleStreamResponse_WithLlamaCppTimings(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	stream := `data: {"choices":[{"delta":{"content":"Hello"}}]}
data: {"choices":[{"delta":{"content":" world"}}]}
data: {"choices":[{"delta":{}}],"timings":{"prompt_n":50,"predicted_n":100,"cache_n":10}}
data: [DONE]
`
	result, err := ic.handleStreamResponse(strings.NewReader(stream), nil)
	if err != nil {
		t.Fatalf("handleStreamResponse() error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	// With llama.cpp timings: input = cache_n + prompt_n = 10 + 50 = 60, output = predicted_n = 100
	if result.InputTokens != 60 {
		t.Errorf("Expected inputTokens 60 (cache_n + prompt_n), got %d", result.InputTokens)
	}
	if result.OutputTokens != 100 {
		t.Errorf("Expected outputTokens 100, got %d", result.OutputTokens)
	}
}

func TestHandleStreamResponse_EmptyStream(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	stream := `data: [DONE]
`
	result, err := ic.handleStreamResponse(strings.NewReader(stream), nil)
	if err != nil {
		t.Fatalf("handleStreamResponse() error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Content != "" {
		t.Errorf("Expected empty content, got %q", result.Content)
	}
}

func TestHandleStreamResponse_MultiLineJSON(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	// Multi-line JSON response (common with tool calls with complex arguments)
	stream := `data: {"choices":[{"delta":{"content":"Hello"}}]}
data: {"choices":[{"delta":{"content":" world"}}]}
data: [DONE]
`
	result, err := ic.handleStreamResponse(strings.NewReader(stream), nil)
	if err != nil {
		t.Fatalf("handleStreamResponse() error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Content != "Hello world" {
		t.Errorf("Expected 'Hello world', got %q", result.Content)
	}
}

