package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
	"github.com/coding-agent/harness/tools"
)

func TestRunStream(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// This tests the method exists and doesn't panic without actual LLM
	// The call will fail because there's no LLM server, but we test the method structure
	_, err := agent.RunStream(ctx, "test prompt", func(chunk inference.StreamingChunk) {
		// noop
	})

	// Expect error since no LLM server is running (timeout or connection error)
	if err == nil {
		t.Error("Expected error when no LLM server is available")
	}
}

// TestRun_NonStreamingWithCallback_DoesNotForceStreamingInference verifies
// that a web session which installs a notification callback (for tool cards)
// while running in --no-stream mode still uses non-streaming inference and
// does not leak status lines to stdout (I-10). The mock serves a single,
// non-SSE JSON completion on each request, which the streaming parser would
// reject.
func TestRun_NonStreamingWithCallback_DoesNotForceStreamingInference(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		calls++
		if calls == 1 {
			w.Write([]byte(`{
				"id":"test-1","object":"chat.completion","created":1234567890,"model":"test-model",
				"choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[
					{"id":"call_1","type":"function","function":{"name":"bash","arguments":"{\"command\":\"echo hi\"}"}}
				]},"finish_reason":"tool_calls"}],
				"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}
			}`))
			return
		}
		w.Write([]byte(`{
			"id":"test-2","object":"chat.completion","created":1234567891,"model":"test-model",
			"choices":[{"index":0,"message":{"role":"assistant","content":"Final answer here"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}
		}`))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.APIEndpoint = server.URL + "/v1"
	cfg.Streaming = false
	ag := NewAgent(cfg)

	var chunks []inference.StreamingChunk
	ag.SetStreamCallback(func(chunk inference.StreamingChunk) {
		chunks = append(chunks, chunk)
	})

	result, err := ag.Run(context.Background(), "hello")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if result.FinalOutput != "Final answer here" {
		t.Errorf("unexpected final output %q", result.FinalOutput)
	}
	if len(chunks) == 0 {
		t.Error("expected tool notification chunks via callback")
	}
}


func TestStreamResult_Callback(t *testing.T) {
	var received []inference.StreamingChunk

	cb := func(chunk inference.StreamingChunk) {
		received = append(received, chunk)
	}

	successResult := &tools.ToolResult{
		Success: true,
		Output:  "success output",
	}
	streamResult("bash", successResult, cb)
	if len(received) == 0 {
		t.Error("Expected callback to be called for success result")
	}

	failureResult := &tools.ToolResult{
		Success: false,
		Error:   "some error",
	}
	streamResult("bash", failureResult, cb)
	if len(received) == 0 {
		t.Error("Expected callback to be called for failure result")
	}

	// Test nil callback
	streamResult("bash", successResult, nil)
}

func TestStreamToolCallWithFullParams_NilCallback(t *testing.T) {
	tc := &tools.ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo test",
		},
	}

	// Should not panic with nil callback
	streamToolCallWithFullParams(tc, nil)
}

func TestRunStream_ContextCancellation(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := agent.RunStream(ctx, "test prompt", nil)
	if err == nil {
		t.Error("Expected error when context is cancelled")
	}
}
