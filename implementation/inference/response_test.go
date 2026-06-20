package inference

import (
	"io"
	"strings"
	"testing"

	"github.com/coding-agent/harness/config"
)

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

