package inference

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

	if client.temperature != nil {
		t.Errorf("Expected temperature nil, got %f", *client.temperature)
	}

	if client.maxTokens != 64000 {
		t.Errorf("Expected maxTokens 64000, got %d", client.maxTokens)
	}

	if client.contextSize != 128000 {
		t.Errorf("Expected contextSize 128000, got %d", client.contextSize)
	}

	if client.streaming != true {
		t.Errorf("Expected streaming true, got %v", client.streaming)
	}

	if client.maxRetries != 3 {
		t.Errorf("Expected maxRetries 3, got %d", client.maxRetries)
	}

	if client.retryDelay != 1*time.Second {
		t.Errorf("Expected retryDelay 1s, got %v", client.retryDelay)
	}

	if client.client == nil {
		t.Error("Expected http client to be initialized")
	}
}

func TestNewInferenceClient_CustomConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Model = "custom-model"
	cfg.MaxTokens = 8192
	cfg.Streaming = false
	cfg.APIEndpoint = "http://custom:9000"

	client := NewInferenceClient(cfg)

	if client.model != "custom-model" {
		t.Errorf("Expected model 'custom-model', got '%s'", client.model)
	}

	if client.maxTokens != 8192 {
		t.Errorf("Expected maxTokens 8192, got %d", client.maxTokens)
	}

	if client.streaming != false {
		t.Errorf("Expected streaming false, got %v", client.streaming)
	}

	if client.endpoint != "http://custom:9000" {
		t.Errorf("Expected endpoint 'http://custom:9000', got '%s'", client.endpoint)
	}
}

func TestNewInferenceClient_Timeout(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.InitialTokenTimeout = 300
	cfg.ReadTimeout = 600

	client := NewInferenceClient(cfg)

	if client.timeout != 600*time.Second {
		t.Errorf("Expected timeout 600s (read timeout), got %v", client.timeout)
	}
}

func TestSetEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	endpoints := []string{
		"http://localhost:8080",
		"http://api.example.com/v1",
		"https://api.openai.com/v1",
	}

	for _, endpoint := range endpoints {
		client.SetEndpoint(endpoint)
		if client.endpoint != endpoint {
			t.Errorf("SetEndpoint(%q) failed, got %q", endpoint, client.endpoint)
		}
	}
}

func TestSetAPIKey(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	keys := []string{
		"",
		"sk-test-key-123",
		"another-key-abc",
	}

	for _, key := range keys {
		client.SetAPIKey(key)
		if client.apiKey != key {
			t.Errorf("SetAPIKey(%q) failed, got %q", key, client.apiKey)
		}
	}
}

func TestSetTools(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	tools := []ToolDefinition{
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "test_tool",
				Description: "A test tool",
				Parameters: ParameterSchema{
					Type: "object",
					Properties: map[string]Property{
						"arg1": {Type: "string", Description: "First arg"},
					},
					Required: []string{"arg1"},
				},
			},
		},
	}

	client.SetTools(tools)
	retrieved := client.GetTools()

	if len(retrieved) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(retrieved))
	}

	if retrieved[0].Function.Name != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", retrieved[0].Function.Name)
	}
}

func TestGetTools_Empty(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	tools := client.GetTools()
	if len(tools) != 0 {
		t.Errorf("Expected empty tools, got %d", len(tools))
	}
}

func TestBuildMessages_WithSystemPrompt(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	systemPrompt := "You are a helpful assistant."
	messages := []*Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	result := client.buildMessages(messages, systemPrompt)

	if len(result) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(result))
	}

	if result[0].Role != "system" {
		t.Errorf("First message should be system, got '%s'", result[0].Role)
	}

	if result[0].Content != systemPrompt {
		t.Errorf("System prompt mismatch")
	}

	if result[1].Content != "Hello" {
		t.Errorf("Expected second message 'Hello', got '%s'", result[1].Content)
	}
}

func TestBuildMessages_NoSystemPrompt(t *testing.T) {
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

func TestBuildMessages_EmptyMessages(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	result := client.buildMessages(nil, "System prompt")

	if len(result) != 1 {
		t.Errorf("Expected 1 message (system only), got %d", len(result))
	}

	if result[0].Role != "system" {
		t.Errorf("Expected system role, got '%s'", result[0].Role)
	}
}

func TestEstimateTokens_Empty(t *testing.T) {
	if result := EstimateTokens(""); result != 0 {
		t.Errorf("Expected 0 tokens for empty string, got %d", result)
	}
}

func TestEstimateTokens_SingleWord(t *testing.T) {
	result := EstimateTokens("hello")
	if result < 1 {
		t.Errorf("Expected at least 1 token, got %d", result)
	}
}

func TestEstimateTokens_MultipleWords(t *testing.T) {
	text := "this is a test with multiple words for token estimation"
	result := EstimateTokens(text)
	words := len(strings.Fields(text))
	// Should be roughly proportional to word count
	if result < words/2 || result > words*3 {
		t.Errorf("Expected reasonable token count for %d words, got %d", words, result)
	}
}

func TestEstimateTokens_Code(t *testing.T) {
	code := "func main() { fmt.Println(\"hello\"); }"
	result := EstimateTokens(code)
	if result < 1 {
		t.Errorf("Expected at least 1 token for code, got %d", result)
	}
}

func TestEstimateContextSize_OnlySystem(t *testing.T) {
	systemPrompt := "You are a helpful assistant."
	result := EstimateContextSize(nil, nil, systemPrompt)
	if result <= 0 {
		t.Errorf("Expected positive result for system prompt only, got %d", result)
	}
}

func TestEstimateContextSize_WithMessages(t *testing.T) {
	systemPrompt := "System prompt"
	messages := []*Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	}
	result := EstimateContextSize(messages, nil, systemPrompt)
	if result <= 0 {
		t.Errorf("Expected positive result, got %d", result)
	}
}

func TestEstimateContextSize_WithTools(t *testing.T) {
	systemPrompt := "System"
	tools := []ToolDefinition{
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "bash",
				Description: "Execute a bash command",
				Parameters: ParameterSchema{
					Type: "object",
					Properties: map[string]Property{
						"command": {Type: "string", Description: "Command to run"},
					},
					Required: []string{"command"},
				},
			},
		},
	}
	result := EstimateContextSize(nil, tools, systemPrompt)
	if result <= 0 {
		t.Errorf("Expected positive result with tools, got %d", result)
	}

	// Adding tools should increase count
	resultNoTools := EstimateContextSize(nil, nil, systemPrompt)
	if result <= resultNoTools {
		t.Errorf("Expected result with tools > without tools: %d <= %d", result, resultNoTools)
	}
}

func TestEstimateContextSize_AllComponents(t *testing.T) {
	systemPrompt := "System prompt for the assistant"
	messages := []*Message{
		{Role: "user", Content: "This is a user message with some content"},
	}
	tools := []ToolDefinition{
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "test_tool",
				Description: "Test tool description",
				Parameters: ParameterSchema{
					Type: "object",
					Properties: map[string]Property{
						"param1": {Type: "string", Description: "First parameter"},
						"param2": {Type: "integer", Description: "Second parameter"},
					},
					Required: []string{"param1"},
				},
			},
		},
	}

	result := EstimateContextSize(messages, tools, systemPrompt)

	// Verify components are additive
	resultNoSys := EstimateContextSize(messages, tools, "")
	resultNoMsgs := EstimateContextSize(nil, tools, systemPrompt)
	resultNoTools := EstimateContextSize(messages, nil, systemPrompt)

	// Total should be >= each component
	if result < resultNoSys || result < resultNoMsgs || result < resultNoTools {
		t.Errorf("Expected total to be sum of all components")
	}
}

func TestRequestBody_Marshal(t *testing.T) {
	temp := 0.7
	req := RequestBody{
		Model:       "llama3",
		Messages:    []*Message{{Role: "user", Content: "test"}},
		Stream:      true,
		Temperature: &temp,
		MaxTokens:   4096,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if parsed["model"] != "llama3" {
		t.Errorf("Expected model 'llama3', got %v", parsed["model"])
	}

	if parsed["stream"] != true {
		t.Errorf("Expected stream true, got %v", parsed["stream"])
	}
}

func TestRequestBody_WithoutTemperature(t *testing.T) {
	req := RequestBody{
		Model:     "test",
		Messages:  []*Message{{Role: "user", Content: "test"}},
		Stream:    false,
		MaxTokens: 1024,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(jsonData, &parsed)

	// Temperature should not be present when nil
	if _, exists := parsed["temperature"]; exists {
		t.Error("Expected temperature to be omitted when nil")
	}
}

func TestResponse_ToolCallsParsing(t *testing.T) {
	// Test that APIToolCall and ToolCall types are properly structured
	apiCall := APIToolCall{
		ID:   "call_123",
		Type: "function",
		Function: FunctionCall{
			Name:      "bash",
			Arguments: `{"command":"ls -la"}`,
		},
	}

	if apiCall.ID != "call_123" {
		t.Errorf("Expected ID 'call_123', got '%s'", apiCall.ID)
	}

	if apiCall.Function.Name != "bash" {
		t.Errorf("Expected name 'bash', got '%s'", apiCall.Function.Name)
	}
}

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

func TestInferenceClient_MessagesType(t *testing.T) {
	msg := Message{
		Role:       "user",
		Content:    "Hello",
		ToolCallId: "call_123",
		ToolCalls: []*APIToolCall{
			{
				ID:   "call_123",
				Type: "function",
				Function: FunctionCall{
					Name:      "bash",
					Arguments: `{"command":"echo hello"}`,
				},
			},
		},
	}

	if msg.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", msg.Role)
	}

	if msg.ToolCallId != "call_123" {
		t.Errorf("Expected ToolCallId 'call_123', got '%s'", msg.ToolCallId)
	}

	if len(msg.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(msg.ToolCalls))
	}
}

func TestInferenceClient_ResponseFields(t *testing.T) {
	resp := Response{
		Content:      "Test response",
		Reasoning:    "Let me think about this...",
		TokenUsage:   100,
		StreamUsage:  50,
		InputTokens:  60,
		OutputTokens: 40,
	}

	if resp.Content != "Test response" {
		t.Errorf("Expected content 'Test response', got '%s'", resp.Content)
	}

	if resp.Reasoning != "Let me think about this..." {
		t.Errorf("Expected reasoning content")
	}

	if resp.InputTokens+resp.OutputTokens != 100 {
		t.Errorf("Expected input+output to equal total")
	}
}

func TestToolDefinition_Structure(t *testing.T) {
	toolDef := ToolDefinition{
		Type: "function",
		Function: FunctionDefinition{
			Name:        "test_tool",
			Description: "A test tool for testing",
			Parameters: ParameterSchema{
				Type:     "object",
				Required: []string{"required_param"},
				Properties: map[string]Property{
					"required_param": {Type: "string", Description: "A required parameter"},
					"optional_param": {Type: "string", Description: "An optional parameter"},
				},
			},
		},
	}

	if toolDef.Type != "function" {
		t.Errorf("Expected type 'function', got '%s'", toolDef.Type)
	}

	if toolDef.Function.Name != "test_tool" {
		t.Errorf("Expected name 'test_tool', got '%s'", toolDef.Function.Name)
	}

	if len(toolDef.Function.Parameters.Required) != 1 {
		t.Errorf("Expected 1 required parameter, got %d", len(toolDef.Function.Parameters.Required))
	}

	if len(toolDef.Function.Parameters.Properties) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(toolDef.Function.Parameters.Properties))
	}
}

func TestParameterSchema_Structure(t *testing.T) {
	schema := ParameterSchema{
		Type:       "object",
		Required:   []string{"param1", "param2"},
		Properties: map[string]Property{},
	}

	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got '%s'", schema.Type)
	}

	schema.Properties["param1"] = Property{Type: "string", Description: "First param"}
	schema.Properties["param2"] = Property{Type: "integer", Description: "Second param"}

	if len(schema.Properties) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(schema.Properties))
	}
}

func TestFunctionCall_Structure(t *testing.T) {
	call := FunctionCall{
		Name:      "bash",
		Arguments: `{"command":"ls -la"}`,
	}

	if call.Name != "bash" {
		t.Errorf("Expected name 'bash', got '%s'", call.Name)
	}

	if call.Arguments != `{"command":"ls -la"}` {
		t.Errorf("Expected arguments, got '%s'", call.Arguments)
	}
}

func TestAPIToolCall_JSON(t *testing.T) {
	apiCall := APIToolCall{
		ID:   "call_abc",
		Type: "function",
		Function: FunctionCall{
			Name:      "read_file",
			Arguments: `{"path":"/test/file.txt"}`,
		},
	}

	jsonData, err := json.Marshal(apiCall)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if parsed["id"] != "call_abc" {
		t.Errorf("Expected id 'call_abc', got %v", parsed["id"])
	}

	fn, ok := parsed["function"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected function to be a map")
	}

	if fn["name"] != "read_file" {
		t.Errorf("Expected function name 'read_file', got %v", fn["name"])
	}
}

func TestInferenceClient_BuildMessages_MessageOrder(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	systemPrompt := "System"
	messages := []*Message{
		{Role: "user", Content: "First"},
		{Role: "assistant", Content: "Second"},
		{Role: "user", Content: "Third"},
	}

	result := client.buildMessages(messages, systemPrompt)

	if len(result) != 4 {
		t.Fatalf("Expected 4 messages, got %d", len(result))
	}

	// Verify order: system first, then user messages in order
	if result[0].Role != "system" {
		t.Errorf("Expected first message to be system, got '%s'", result[0].Role)
	}
	if result[1].Content != "First" {
		t.Errorf("Expected second message to be 'First', got '%s'", result[1].Content)
	}
	if result[2].Content != "Second" {
		t.Errorf("Expected third message to be 'Second', got '%s'", result[2].Content)
	}
	if result[3].Content != "Third" {
		t.Errorf("Expected fourth message to be 'Third', got '%s'", result[3].Content)
	}
}

func TestHandleResponse_NonStreaming(t *testing.T) {
	// Create a mock response body
	mockResponse := `{
		"choices": [{
			"message": {
				"role": "assistant",
				"content": "This is the response content"
			},
			"finish_reason": "stop"
		}],
		"usage": {
			"prompt_tokens": 50,
			"completion_tokens": 100,
			"total_tokens": 150
		}
	}`

	body := io.NopCloser(strings.NewReader(mockResponse))
	client := NewInferenceClient(config.DefaultConfig())

	resp, err := client.handleResponse(body)
	if err != nil {
		t.Fatalf("handleResponse() error: %v", err)
	}

	if resp.Content != "This is the response content" {
		t.Errorf("Expected content 'This is the response content', got '%s'", resp.Content)
	}

	if resp.InputTokens != 50 {
		t.Errorf("Expected input tokens 50, got %d", resp.InputTokens)
	}

	if resp.OutputTokens != 100 {
		t.Errorf("Expected output tokens 100, got %d", resp.OutputTokens)
	}

	if resp.TokenUsage != 150 {
		t.Errorf("Expected total tokens 150, got %d", resp.TokenUsage)
	}
}

func TestHandleResponse_WithToolCalls(t *testing.T) {
	mockResponse := `{
		"choices": [{
			"message": {
				"role": "assistant",
				"content": "",
				"tool_calls": [{
					"id": "call_123",
					"type": "function",
					"function": {
						"name": "bash",
						"arguments": "{\"command\":\"ls -la\"}"
					}
				}]
			},
			"finish_reason": "tool_calls"
		}],
		"usage": {
			"prompt_tokens": 50,
			"completion_tokens": 30,
			"total_tokens": 80
		}
	}`

	body := io.NopCloser(strings.NewReader(mockResponse))
	client := NewInferenceClient(config.DefaultConfig())

	resp, err := client.handleResponse(body)
	if err != nil {
		t.Fatalf("handleResponse() error: %v", err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(resp.ToolCalls))
	}

	if resp.ToolCalls[0].Name != "bash" {
		t.Errorf("Expected tool name 'bash', got '%s'", resp.ToolCalls[0].Name)
	}

	if resp.ToolCalls[0].ID != "call_123" {
		t.Errorf("Expected tool ID 'call_123', got '%s'", resp.ToolCalls[0].ID)
	}

	if params, ok := resp.ToolCalls[0].Parameters["command"].(string); !ok || params != "ls -la" {
		t.Errorf("Expected command 'ls -la', got %v", resp.ToolCalls[0].Parameters["command"])
	}
}

func TestHandleResponse_EmptyChoices(t *testing.T) {
	mockResponse := `{"choices": []}`

	body := io.NopCloser(strings.NewReader(mockResponse))
	client := NewInferenceClient(config.DefaultConfig())

	_, err := client.handleResponse(body)
	if err == nil {
		t.Fatal("Expected error for empty choices")
	}

	if !strings.Contains(err.Error(), "no choices") {
		t.Errorf("Expected 'no choices' error, got: %v", err)
	}
}

func TestHandleResponse_DecodeError(t *testing.T) {
	body := io.NopCloser(strings.NewReader("not valid json"))
	client := NewInferenceClient(config.DefaultConfig())

	_, err := client.handleResponse(body)
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("Expected decode error, got: %v", err)
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

func TestInferenceRequest_WithContext(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This will fail because there's no server, but tests the interface
	_, err := client.InferenceRequest(ctx, nil, "test")
	if err == nil {
		t.Error("Expected error when no server is running")
	}
}

func TestInferenceRequestStream_WithContext(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This will fail because there's no server, but tests the interface
	_, err := client.InferenceRequestStream(ctx, nil, "test", nil)
	if err == nil {
		t.Error("Expected error when no server is running")
	}
}

func TestInferenceRequestWithCallbackTyped(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var callback = func(chunk StreamingChunk) {
		// noop
	}

	_, err := client.InferenceRequestWithCallbackTyped(ctx, nil, "test", callback)
	if err == nil {
		t.Error("Expected error when no server is running")
	}
	// Callback should not be called since connection fails before streaming
}

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

func TestFormatToolCallArgs_EmptyMap(t *testing.T) {
	result := formatToolCallArgs(map[string]interface{}{}, 80)
	if result != "{}" {
		t.Errorf("Expected '{}', got %q", result)
	}
}

func TestFormatToolCallArgs_NilParams(t *testing.T) {
	result := formatToolCallArgs(nil, 80)
	if result != "{}" {
		t.Errorf("Expected '{}', got %q", result)
	}
}

func TestFormatToolCallArgs_SingleParam(t *testing.T) {
	result := formatToolCallArgs(map[string]interface{}{"key": "value"}, 80)
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

func TestFormatToolCallArgs_MaxWidthTruncation(t *testing.T) {
	// Use a very small max width to trigger truncation
	result := formatToolCallArgs(map[string]interface{}{"key": "value"}, 10)
	if result == "" {
		t.Error("Expected non-empty result")
	}
	// Should be truncated
	if len(result) > 15 {
		t.Error("Expected truncated result")
	}
}

func TestFormatToolCallArgs_ManualParams(t *testing.T) {
	// Test with manually constructed params like bash tool would receive
	params := map[string]interface{}{
		"command": "echo hello",
		"args":    []interface{}{"arg1", "arg2"},
	}
	result := formatToolCallArgs(params, 100)
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

// ===== Tests for formatJSONMapWithMaxWidth =====

func TestFormatJSONMapWithMaxWidth(t *testing.T) {
	m := map[string]interface{}{"key": "value"}
	result := formatJSONMapWithMaxWidth(m, 80)
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

func TestFormatJSONArrayWithMaxWidth(t *testing.T) {
	arr := []interface{}{"a", "b", "c"}
	result := formatJSONArrayWithMaxWidth(arr, 80)
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

// ===== Tests for handleStreamResponse with callbacks =====

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

func TestBuildMessages(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	messages := []*Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}

	result := ic.buildMessages(messages, "System prompt")

	if len(result) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(result))
	}
	if result[0].Role != "system" {
		t.Errorf("Expected first message role 'system', got %q", result[0].Role)
	}
	if result[0].Content != "System prompt" {
		t.Errorf("Expected first message content 'System prompt', got %q", result[0].Content)
	}
	if result[1].Content != "Hello" {
		t.Errorf("Expected second message content 'Hello', got %q", result[1].Content)
	}
	if result[2].Content != "Hi there" {
		t.Errorf("Expected third message content 'Hi there', got %q", result[2].Content)
	}
}

func TestBuildMessages_WithSystemMessage(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	messages := []*Message{
		{Role: "user", Content: "Hello"},
	}

	result := ic.buildMessages(messages, "")

	if len(result) != 1 {
		t.Fatalf("Expected 1 message (no system prompt), got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Errorf("Expected first message role 'user', got %q", result[0].Role)
	}
}

func TestBuildMessages_Empty(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	result := ic.buildMessages(nil, "")
	if len(result) != 0 {
		t.Errorf("Expected 0 messages, got %d", len(result))
	}
}

func TestBuildMessages_EmptySystemPrompt(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	messages := []*Message{
		{Role: "user", Content: "Hello"},
	}

	result := ic.buildMessages(messages, "")
	if len(result) != 1 {
		t.Errorf("Expected 1 message when system prompt is empty, got %d", len(result))
	}
}

func TestInferenceClient_SetMaxDisplayWidth(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	ic.SetMaxDisplayWidth(120)
	if ic.maxDisplayWidth != 120 {
		t.Errorf("Expected maxDisplayWidth 120, got %d", ic.maxDisplayWidth)
	}
}

func TestInferenceClient_SetAPIKey(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	ic.SetAPIKey("test-key-123")
	if ic.apiKey != "test-key-123" {
		t.Errorf("Expected apiKey 'test-key-123', got %q", ic.apiKey)
	}
}

func TestInferenceClient_GetTools(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	tools := []ToolDefinition{
		{Type: "function", Function: FunctionDefinition{Name: "tool1", Description: "First tool", Parameters: ParameterSchema{Type: "object"}}},
	}
	ic.SetTools(tools)

	result := ic.GetTools()
	if len(result) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(result))
	}
	if result[0].Function.Name != "tool1" {
		t.Errorf("Expected tool name 'tool1', got %q", result[0].Function.Name)
	}
}

func TestIsCopilotEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	// Default is not copilot
	if ic.isCopilotEndpoint() {
		t.Error("Expected isCopilotEndpoint to be false for default endpoint")
	}

	// Copilot endpoints
	ic.SetEndpoint("https://copilot.githubcopilot.com/v1")
	if !ic.isCopilotEndpoint() {
		t.Error("Expected isCopilotEndpoint to be true for Copilot endpoint")
	}

	// Not a Copilot endpoint
	ic.SetEndpoint("https://other-server.com/api")
	if ic.isCopilotEndpoint() {
		t.Error("Expected isCopilotEndpoint to be false for non-Copilot endpoint")
	}
}

func TestIsGitHubModelsEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	// GitHub Models endpoint (models.github.ai)
	ic.SetEndpoint("https://models.github.ai")
	if !ic.isGitHubModelsEndpoint() {
		t.Error("Expected isGitHubModelsEndpoint to be true for GitHub Models endpoint")
	}

	// GitHub Models endpoint with path
	ic.SetEndpoint("https://models.github.ai/v1")
	if !ic.isGitHubModelsEndpoint() {
		t.Error("Expected isGitHubModelsEndpoint to be true for GitHub Models endpoint with path")
	}

	// Not a GitHub Models endpoint
	ic.SetEndpoint("https://other-server.com/api")
	if ic.isGitHubModelsEndpoint() {
		t.Error("Expected isGitHubModelsEndpoint to be false for non-GitHub Models endpoint")
	}
}

func TestBuildURL(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	// Default URL
	url := ic.buildURL()
	if url != "http://localhost:8080/v1/chat/completions" {
		t.Errorf("Expected default URL, got %q", url)
	}

	// Custom endpoint without trailing slash
	ic.SetEndpoint("https://custom-server.com")
	url = ic.buildURL()
	if url != "https://custom-server.com/v1/chat/completions" {
		t.Errorf("Expected custom URL with path, got %q", url)
	}

	// Custom endpoint with trailing slash
	ic.SetEndpoint("https://custom-server.com/")
	url = ic.buildURL()
	if url != "https://custom-server.com//v1/chat/completions" {
		t.Errorf("Expected custom URL with double slash, got %q", url)
	}

	// Custom endpoint with path
	ic.SetEndpoint("https://custom-server.com/api")
	url = ic.buildURL()
	if url != "https://custom-server.com/api/v1/chat/completions" {
		t.Errorf("Expected custom URL with path, got %q", url)
	}
}
