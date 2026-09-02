package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coding-agent/harness/config"
)

// TestRun_CancellationLeavesWellFormedConversation verifies that cancelling a
// run mid-tool-loop synthesizes tool results for every pending tool call so the
// assistant turn stays well-formed (one tool message per tool_call_id), as
// required by OpenAI-compatible servers.
func TestRun_CancellationLeavesWellFormedConversation(t *testing.T) {
	// Mock server returns one assistant message with two bash tool calls.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id":"test-1","object":"chat.completion","created":1234567890,"model":"test-model",
			"choices":[{
				"index":0,
				"message":{
					"role":"assistant","content":"","tool_calls":[
						{"id":"call_1","type":"function","function":{"name":"bash","arguments":"{\"command\":\"sleep 2\"}"}},
						{"id":"call_2","type":"function","function":{"name":"bash","arguments":"{\"command\":\"sleep 2\"}"}}
					]
				},
				"finish_reason":"tool_calls"
			}],
			"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}
		}`))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.APIEndpoint = server.URL + "/v1"
	cfg.Streaming = false
	ag := NewAgent(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	_, err := ag.Run(ctx, "do something")
	if err == nil {
		t.Fatal("expected a cancellation error")
	}

	ag.mu.Lock()
	defer ag.mu.Unlock()

	// Count assistant tool calls and tool result messages.
	var assistantToolCalls int
	var toolResults int
	for _, m := range ag.context {
		if m.Role == "assistant" {
			assistantToolCalls += len(m.ToolCalls)
		}
		if m.Role == "tool" {
			toolResults++
		}
	}

	if assistantToolCalls != toolResults {
		t.Errorf("conversation is malformed: assistant emitted %d tool_calls but there are %d tool result messages",
			assistantToolCalls, toolResults)
	}
	if assistantToolCalls != 2 {
		t.Errorf("expected 2 assistant tool calls, got %d", assistantToolCalls)
	}
}
