package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
)

func TestGetStats_Complete(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	stats := agent.GetStats()

	if stats == nil {
		t.Fatal("GetStats() returned nil")
	}

	// Verify all fields exist and are accessible
	if stats.InputTokens < 0 {
		t.Error("InputTokens should be non-negative")
	}
	if stats.OutputTokens < 0 {
		t.Error("OutputTokens should be non-negative")
	}
	if stats.ToolCalls < 0 {
		t.Error("ToolCalls should be non-negative")
	}
	if stats.FailedToolCalls < 0 {
		t.Error("FailedToolCalls should be non-negative")
	}
	if stats.Iterations < 0 {
		t.Error("Iterations should be non-negative")
	}

	// StartTime should be set
	if stats.StartTime.IsZero() {
		t.Error("Expected non-zero StartTime")
	}
}

func TestGetStats_TimeElapsed(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Wait a bit to ensure time passes
	time.Sleep(10 * time.Millisecond)

	stats := agent.GetStats()

	// After some time has passed, the function should still work without errors
	// Verify Stats struct is populated
	if stats == nil {
		t.Fatal("GetStats() returned nil after time elapsed")
	}
}

func TestGetStats_TokensPerSecond(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	stats := agent.GetStats()

	// TokensPerSecond should be 0 initially (no time elapsed or no tokens)
	if stats.TokensPerSecond < 0 {
		t.Errorf("Expected non-negative TokensPerSecond, got %f", stats.TokensPerSecond)
	}
}

func TestRun_FinalResponse(t *testing.T) {
	// Create a mock inference server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that this is a chat completions request
		if r.URL.Path != "/v1/chat/completions" && !strings.HasSuffix(r.URL.Path, "chat/completions") {
			// Could also be the root path depending on the endpoint setup
		}
		w.Header().Set("Content-Type", "application/json")
		// Return a response with no tool calls - final response
		w.Write([]byte(`{
			"id": "test-1",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "test-model",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "This is the final answer"
				},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 20,
				"total_tokens": 30
			}
		}`))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.APIEndpoint = server.URL + "/v1"
	cfg.Streaming = false
	ag := NewAgent(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := ag.Run(ctx, "test prompt")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.FinalOutput != "This is the final answer" {
		t.Errorf("Expected 'This is the final answer', got '%s'", result.FinalOutput)
	}

	if len(result.Steps) != 0 {
		t.Errorf("Expected 0 steps (no tool calls), got %d", len(result.Steps))
	}
}

func TestRun_ToolCalls(t *testing.T) {
	// Track the number of API calls made
	callCount := 0
	var receivedMessages [][]inference.Message

	// Create a mock inference server that returns tool calls on first call, then final response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		// Read the request body to see what messages were sent
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			receivedMessages = append(receivedMessages, nil) // track calls
		}

		w.Header().Set("Content-Type", "application/json")

		if callCount == 1 {
			// First call: return a tool call
			w.Write([]byte(`{
				"id": "test-1",
				"object": "chat.completion",
				"created": 1234567890,
				"model": "test-model",
				"choices": [{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "",
						"tool_calls": [{
							"id": "call-1",
							"type": "function",
							"function": {
								"name": "read_file",
								"arguments": "{\"path\": \"test.txt\"}"
							}
						}]
					},
					"finish_reason": "tool_calls"
				}],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 15,
					"total_tokens": 25
				}
			}`))
		} else {
			// Subsequent calls: return final response (no tool calls)
			w.Write([]byte(`{
				"id": "test-2",
				"object": "chat.completion",
				"created": 1234567890,
				"model": "test-model",
				"choices": [{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "After reading the file, here is the answer"
					},
					"finish_reason": "stop"
				}],
				"usage": {
					"prompt_tokens": 20,
					"completion_tokens": 25,
					"total_tokens": 45
				}
			}`))
		}
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.APIEndpoint = server.URL + "/v1"
	cfg.Streaming = false
	cfg.MaxTokens = 10000
	cfg.ContextSize = 32000
	ag := NewAgent(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := ag.Run(ctx, "What is in test.txt?")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestRun_MaxIterationsExceeded(t *testing.T) {
	// Create a mock server that always returns tool calls to force max iterations
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "test-1",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "test-model",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "",
					"tool_calls": [{
						"id": "call-1",
						"type": "function",
						"function": {
							"name": "bash",
							"arguments": "{\"command\": \"echo test\"}"
						}
					}]
				},
				"finish_reason": "tool_calls"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 15,
				"total_tokens": 25
			}
		}`))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.APIEndpoint = server.URL + "/v1"
	cfg.Streaming = false
	cfg.MaxTokens = 10000
	cfg.ContextSize = 32000
	cfg.MaxIterations = 5 // Low max iterations to test the limit
	ag := NewAgent(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := ag.Run(ctx, "test prompt")
	if err == nil {
		t.Error("Expected error for max iterations exceeded")
	}
	if !strings.Contains(err.Error(), "maximum iterations") {
		t.Errorf("Expected 'maximum iterations' in error, got: %v", err)
	}
}

func TestRun_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block to simulate slow response
		time.Sleep(2 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "test-1",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "test-model",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "response"
				},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 10,
				"total_tokens": 20
			}
		}`))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.APIEndpoint = server.URL + "/v1"
	cfg.Streaming = false
	ag := NewAgent(cfg)

	// Create context that will be cancelled after 100ms
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := ag.Run(ctx, "test prompt")
	if err == nil {
		t.Error("Expected context cancellation error")
	}
	if err != context.DeadlineExceeded && !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "context") {
		t.Errorf("Expected deadline/cancellation error, got: %v", err)
	}
}

// ===== Tests for NewAgent with different config options =====

