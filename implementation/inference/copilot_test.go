package inference

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/coding-agent/harness/config"
)

func TestIsCopilotEndpoint_True(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.APIEndpoint = "https://api.githubcopilot.com"
	client := NewInferenceClient(cfg)

	if !client.isCopilotEndpoint() {
		t.Error("Expected isCopilotEndpoint() to return true for githubcopilot.com URL")
	}
}

func TestIsCopilotEndpoint_False(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.APIEndpoint = "http://localhost:8080"
	client := NewInferenceClient(cfg)

	if client.isCopilotEndpoint() {
		t.Error("Expected isCopilotEndpoint() to return false for localhost URL")
	}
}

func TestIsCopilotEndpoint_WithSubdomain(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.APIEndpoint = "https://githubcopilot.com/something"
	client := NewInferenceClient(cfg)

	if !client.isCopilotEndpoint() {
		t.Error("Expected isCopilotEndpoint() to return true for any URL containing githubcopilot.com")
	}
}

func TestIsGitHubModelsEndpoint_True(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.APIEndpoint = "https://models.github.ai"
	client := NewInferenceClient(cfg)

	if !client.isGitHubModelsEndpoint() {
		t.Error("Expected isGitHubModelsEndpoint() to return true for models.github.ai URL")
	}
}

func TestIsGitHubModelsEndpoint_False(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.APIEndpoint = "http://localhost:8080"
	client := NewInferenceClient(cfg)

	if client.isGitHubModelsEndpoint() {
		t.Error("Expected isGitHubModelsEndpoint() to return false for localhost URL")
	}
}

func TestBuildURL_CopilotEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.APIEndpoint = "https://api.githubcopilot.com"
	client := NewInferenceClient(cfg)

	url := client.buildURL()
	expected := "https://api.githubcopilot.com/chat/completions"
	if url != expected {
		t.Errorf("Expected buildURL() to return %q for Copilot endpoint, got %q", expected, url)
	}
}

func TestBuildURL_GitHubModelsEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.APIEndpoint = "https://models.github.ai"
	client := NewInferenceClient(cfg)

	url := client.buildURL()
	expected := "https://models.github.ai/inference/chat/completions"
	if url != expected {
		t.Errorf("Expected buildURL() to return %q for GitHub Models endpoint, got %q", expected, url)
	}
}

func TestBuildURL_DefaultEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.APIEndpoint = "http://localhost:8080"
	client := NewInferenceClient(cfg)

	url := client.buildURL()
	expected := "http://localhost:8080/v1/chat/completions"
	if url != expected {
		t.Errorf("Expected buildURL() to return %q for default endpoint, got %q", expected, url)
	}
}

func TestBuildURL_CustomEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.APIEndpoint = "https://api.openai.com/v1"
	client := NewInferenceClient(cfg)

	url := client.buildURL()
	expected := "https://api.openai.com/v1/v1/chat/completions"
	if url != expected {
		t.Errorf("Expected buildURL() to return %q for custom endpoint, got %q", expected, url)
	}
}

func TestHandleStreamResponse_ToolCallTypeNormalization(t *testing.T) {
	// Test that empty type values in streaming deltas are normalized to "function"
	sseStream := `data: {"choices": [{"delta": {"tool_calls": [{"id":"call_1","type":"","function":{"name":"bash","arguments":""}}]}}]}
data: {"choices": [{"delta": {"tool_calls": [{"function":{"arguments":"{\"command\":"}}]}}]}
data: {"choices": [{"delta": {"tool_calls": [{"function":{"arguments":"\"ls -la\"}"}}]}}]}
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

	if resp.ToolCalls[0].ID != "call_1" {
		t.Errorf("Expected tool ID 'call_1', got '%s'", resp.ToolCalls[0].ID)
	}

	if resp.ToolCalls[0].Name != "bash" {
		t.Errorf("Expected tool name 'bash', got '%s'", resp.ToolCalls[0].Name)
	}

	// Verify APIToolCall has type set to "function" even though it was empty in the stream
	if len(resp.APIToolCalls) != 1 {
		t.Fatalf("Expected 1 APIToolCall, got %d", len(resp.APIToolCalls))
	}

	if resp.APIToolCalls[0].Type != "function" {
		t.Errorf("Expected APIToolCall type 'function', got '%s'", resp.APIToolCalls[0].Type)
	}
}

func TestHandleStreamResponse_ToolCallTypeWithExplicitValue(t *testing.T) {
	// Test that explicit type values are preserved
	sseStream := `data: {"choices": [{"delta": {"tool_calls": [{"id":"call_1","type":"function","function":{"name":"bash","arguments":""}}]}}]}
data: {"choices": [{"delta": {"tool_calls": [{"function":{"arguments":"{\"command\":\"ls\"}"}}]}}]}
data: [DONE]`

	body := io.NopCloser(strings.NewReader(sseStream))
	client := NewInferenceClient(config.DefaultConfig())

	resp, err := client.handleStreamResponse(body, nil)
	if err != nil {
		t.Fatalf("handleStreamResponse() error: %v", err)
	}

	if len(resp.APIToolCalls) != 1 {
		t.Fatalf("Expected 1 APIToolCall, got %d", len(resp.APIToolCalls))
	}

	if resp.APIToolCalls[0].Type != "function" {
		t.Errorf("Expected APIToolCall type 'function', got '%s'", resp.APIToolCalls[0].Type)
	}
}

func TestHandleStreamResponse_MultipleToolCallsTypeNormalization(t *testing.T) {
	// Test type normalization for multiple tool calls in a single response
	sseStream := `data: {"choices": [{"delta": {"tool_calls": [
		{"id":"call_1","type":"","function":{"name":"bash","arguments":""}},
		{"id":"call_2","type":"","function":{"name":"read_file","arguments":""}}
	]}}]}
data: {"choices": [{"delta": {"tool_calls": [{"function":{"arguments":"{\"command\":\"ls\"}"}}]}}]}
data: {"choices": [{"delta": {"tool_calls": [{"function":{"arguments":"{\"path\":\"test.txt\"}"}}]}}]}
data: [DONE]`

	body := io.NopCloser(strings.NewReader(sseStream))
	client := NewInferenceClient(config.DefaultConfig())

	resp, err := client.handleStreamResponse(body, nil)
	if err != nil {
		t.Fatalf("handleStreamResponse() error: %v", err)
	}

	if len(resp.APIToolCalls) != 2 {
		t.Fatalf("Expected 2 APIToolCalls, got %d", len(resp.APIToolCalls))
	}

	for i, apiTC := range resp.APIToolCalls {
		if apiTC.Type != "function" {
			t.Errorf("Expected APIToolCall[%d] type 'function', got '%s'", i, apiTC.Type)
		}
	}
}

func TestHandleResponse_WithEmptyTypeToolCall(t *testing.T) {
	// Test that empty type in non-streaming responses is handled correctly
	mockResponse := `{
		"choices": [{
			"message": {
				"role": "assistant",
				"content": "",
				"tool_calls": [{
					"id": "call_123",
					"type": "",
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

	if len(resp.APIToolCalls) != 1 {
		t.Fatalf("Expected 1 APIToolCall, got %d", len(resp.APIToolCalls))
	}

	// The APIToolCall preserves whatever type was in the API response
	// The normalization happens at the streaming level
	if resp.APIToolCalls[0].ID != "call_123" {
		t.Errorf("Expected tool ID 'call_123', got '%s'", resp.APIToolCalls[0].ID)
	}
}

func TestNewInferenceClient_CopilotConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.APIEndpoint = "https://api.githubcopilot.com"
	cfg.Model = "gpt-4o"
	cfg.APIKey = "ghu_test_token"

	client := NewInferenceClient(cfg)

	if client.endpoint != "https://api.githubcopilot.com" {
		t.Errorf("Expected endpoint 'https://api.githubcopilot.com', got '%s'", client.endpoint)
	}

	if client.model != "gpt-4o" {
		t.Errorf("Expected model 'gpt-4o', got '%s'", client.model)
	}

	if client.apiKey != "ghu_test_token" {
		t.Errorf("Expected API key 'ghu_test_token', got '%s'", client.apiKey)
	}
}

func TestInferenceRequest_CopilotEndpointURL(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.APIEndpoint = "https://api.githubcopilot.com"
	cfg.Model = "gpt-4o"
	cfg.APIKey = "ghu_test_token"
	cfg.Streaming = false
	client := NewInferenceClient(cfg)

	// Verify the URL path is correct for Copilot
	url := client.buildURL()
	expectedURL := "https://api.githubcopilot.com/chat/completions"
	if url != expectedURL {
		t.Errorf("Expected URL %q for Copilot endpoint, got %q", expectedURL, url)
	}
}

func TestInferenceRequest_GitHubModelsEndpointURL(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.APIEndpoint = "https://models.github.ai"
	cfg.Model = "openai/gpt-4.1"
	cfg.APIKey = "github_pat_test_token"
	cfg.Streaming = false
	client := NewInferenceClient(cfg)

	// Verify the URL path is correct for GitHub Models
	url := client.buildURL()
	expectedURL := "https://models.github.ai/inference/chat/completions"
	if url != expectedURL {
		t.Errorf("Expected URL %q for GitHub Models endpoint, got %q", expectedURL, url)
	}
}

func TestStreamingDeltaMerge_ByIndex(t *testing.T) {
	// Test that tool call deltas are merged by index correctly
	sseStream := `data: {"choices": [{"delta": {"tool_calls": [{"index":0,"id":"call_1","type":"function","function":{"name":"bash"}}]}}]}
data: {"choices": [{"delta": {"tool_calls": [{"index":0,"function":{"arguments":"{\"command\":"}}]}}]}
data: {"choices": [{"delta": {"tool_calls": [{"index":0,"function":{"arguments":"\"ls\"}"}}]}}]}
data: {"choices": [{"delta": {"tool_calls": [{"index":1,"id":"call_2","type":"function","function":{"name":"read_file"}}]}}]}
data: {"choices": [{"delta": {"tool_calls": [{"index":1,"function":{"arguments":"{\"path\":\"test.txt\"}"}}]}}]}
data: [DONE]`

	body := io.NopCloser(strings.NewReader(sseStream))
	client := NewInferenceClient(config.DefaultConfig())

	resp, err := client.handleStreamResponse(body, nil)
	if err != nil {
		t.Fatalf("handleStreamResponse() error: %v", err)
	}

	if len(resp.ToolCalls) != 2 {
		t.Fatalf("Expected 2 tool calls, got %d", len(resp.ToolCalls))
	}

	if resp.ToolCalls[0].Name != "bash" {
		t.Errorf("Expected first tool call name 'bash', got '%s'", resp.ToolCalls[0].Name)
	}

	if resp.ToolCalls[1].Name != "read_file" {
		t.Errorf("Expected second tool call name 'read_file', got '%s'", resp.ToolCalls[1].Name)
	}
}

func TestInferenceClient_WithCopilotHeaders(t *testing.T) {
	// This test verifies that the InferenceClient has the correct methods for Copilot detection
	cfg := config.DefaultConfig()
	cfg.APIEndpoint = "https://api.githubcopilot.com"
	client := NewInferenceClient(cfg)

	// Verify methods exist and work correctly
	if !client.isCopilotEndpoint() {
		t.Error("Expected isCopilotEndpoint() to return true")
	}

	if client.isGitHubModelsEndpoint() {
		t.Error("Expected isGitHubModelsEndpoint() to return false")
	}
}

func TestInferenceClient_WithGitHubModelsHeaders(t *testing.T) {
	// This test verifies that the InferenceClient has the correct methods for GitHub Models detection
	cfg := config.DefaultConfig()
	cfg.APIEndpoint = "https://models.github.ai"
	client := NewInferenceClient(cfg)

	// Verify methods exist and work correctly
	if client.isCopilotEndpoint() {
		t.Error("Expected isCopilotEndpoint() to return false")
	}

	if !client.isGitHubModelsEndpoint() {
		t.Error("Expected isGitHubModelsEndpoint() to return true")
	}
}

func TestInferenceRequest_WithCancellation(t *testing.T) {
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

func TestInferenceRequestStream_WithCancellation(t *testing.T) {
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
