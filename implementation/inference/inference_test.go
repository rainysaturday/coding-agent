package inference

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coding-agent/coding-agent/config"
)

func testCfg(base string) *config.Config {
	c := config.NewDefault()
	c.APIEndpoint = base
	c.Stream = false
	c.Model = "test-model"
	return c
}

func TestChatURLVariants(t *testing.T) {
	c := New(testCfg("http://localhost:8080"))
	if got := c.ChatURL(); got != "http://localhost:8080/v1/chat/completions" {
		t.Fatalf("default chat url: %s", got)
	}
	c = New(testCfg("https://api.githubcopilot.com"))
	if got := c.ChatURL(); got != "https://api.githubcopilot.com/chat/completions" {
		t.Fatalf("copilot chat url: %s", got)
	}
	c = New(testCfg("https://models.github.ai/inference"))
	if got := c.ChatURL(); got != "https://models.github.ai/inference/chat/completions" {
		t.Fatalf("github models chat url: %s", got)
	}
	// Explicit /v1 suffix should not be doubled.
	c = New(testCfg("http://localhost:8080/v1"))
	if got := c.ChatURL(); got != "http://localhost:8080/v1/chat/completions" {
		t.Fatalf("explicit v1 chat url: %s", got)
	}
}

func TestAuthHeadersCopilot(t *testing.T) {
	c := New(testCfg("https://api.githubcopilot.com"))
	c.Config().APIKey = "ghu_token"
	req, _ := http.NewRequest(http.MethodPost, "https://api.githubcopilot.com/chat/completions", nil)
	c.AuthHeaders(req)
	if got := req.Header.Get("Authorization"); got != "Bearer ghu_token" {
		t.Fatalf("authorization: %q", got)
	}
	if req.Header.Get("Copilot-Integration-Id") == "" {
		t.Fatal("missing Copilot-Integration-Id")
	}
	if req.Header.Get("Editor-Version") == "" {
		t.Fatal("missing Editor-Version")
	}
}

func TestNonStreamParsing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{{
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "hello",
					"tool_calls": []map[string]interface{}{{
						"id":   "call_1",
						"type": "function",
						"function": map[string]string{
							"name":      "bash",
							"arguments": `{"command":"ls"}`,
						},
					}},
				},
				"finish_reason": "tool_calls",
			}},
			"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := New(testCfg(srv.URL))
	resp, err := c.ChatCompletion(context.Background(), &Request{Model: "m", Messages: []*Message{NewTextMessage("user", "hi")}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello" {
		t.Fatalf("content: %q", resp.Content)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Function.Name != "bash" {
		t.Fatalf("tool calls: %+v", resp.ToolCalls)
	}
	if resp.Usage.PromptTokens != 10 || resp.Usage.CompletionTokens != 5 {
		t.Fatalf("usage: %+v", resp.Usage)
	}
}

func TestLlamaCppTimingsUsage(t *testing.T) {
	// llama.cpp reports cache_n + prompt_n; total prompt tokens must be the sum.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{{"message": map[string]interface{}{"role": "assistant", "content": "x"}}},
			"timings": map[string]int{"prompt_n": 100, "predicted_n": 20, "cache_n": 900},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	c := New(testCfg(srv.URL))
	resp, err := c.ChatCompletion(context.Background(), &Request{Model: "m"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Usage.PromptTokens != 1000 {
		t.Fatalf("expected cache_n+prompt_n=1000, got %d", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 20 {
		t.Fatalf("predicted_n: %d", resp.Usage.CompletionTokens)
	}
}

func TestStreamToolCallMergeByIndex(t *testing.T) {
	chunks := []string{
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_a","function":{"name":"bash","arguments":""}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"command\":"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"ls\"}"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_b","function":{"name":"read_file","arguments":"{\"path\":\"x\"}"}}]}}]}`,
		`{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, ch := range chunks {
			fmt.Fprintf(w, "data: %s\n\n", ch)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	cfg := testCfg(srv.URL)
	cfg.Stream = true
	c := New(cfg)

	var deltas int
	cb := &StreamCallbacks{ToolCallDelta: func(index int, d ToolCallDelta) { deltas++ }}
	resp, err := c.ChatCompletion(context.Background(), &Request{Model: "m"}, cb)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ToolCalls) != 2 {
		t.Fatalf("expected 2 merged tool calls, got %d: %+v", len(resp.ToolCalls), resp.ToolCalls)
	}
	if resp.ToolCalls[0].ID != "call_a" || resp.ToolCalls[0].Function.Name != "bash" {
		t.Fatalf("tool call 0: %+v", resp.ToolCalls[0])
	}
	if resp.ToolCalls[0].Function.Arguments != `{"command":"ls"}` {
		t.Fatalf("merged args: %q", resp.ToolCalls[0].Function.Arguments)
	}
	if resp.ToolCalls[1].Function.Name != "read_file" {
		t.Fatalf("tool call 1: %+v", resp.ToolCalls[1])
	}
	if deltas != 4 {
		t.Fatalf("expected 4 delta callbacks, got %d", deltas)
	}
}

func TestStreamReasoningFieldDetection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"think \"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"hard\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()
	cfg := testCfg(srv.URL)
	cfg.Stream = true
	c := New(cfg)
	var gotReasoning, gotContent strings.Builder
	cb := &StreamCallbacks{
		Reasoning: func(s string) { gotReasoning.WriteString(s) },
		Content:   func(s string) { gotContent.WriteString(s) },
	}
	resp, err := c.ChatCompletion(context.Background(), &Request{Model: "m"}, cb)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Reasoning != "think hard" {
		t.Fatalf("reasoning: %q", resp.Reasoning)
	}
	if gotContent.String() != "answer" {
		t.Fatalf("content callback: %q", gotContent.String())
	}
	if c.ReasoningField() != ReasoningFieldLlamaCPP {
		t.Fatalf("reasoning field: %s", c.ReasoningField())
	}
}

func TestRetryOn500(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"error":{"message":"server boom"}}`)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{{"message": map[string]interface{}{"role": "assistant", "content": "ok"}}},
		})
	}))
	defer srv.Close()
	c := New(testCfg(srv.URL))
	resp, err := c.ChatCompletion(context.Background(), &Request{Model: "m"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "ok" || atomic.LoadInt32(&hits) != 2 {
		t.Fatalf("retry did not happen: hits=%d content=%q", hits, resp.Content)
	}
}

func TestAuthErrorShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"Unauthorized"}}`)
	}))
	defer srv.Close()
	c := New(testCfg(srv.URL))
	_, err := c.ChatCompletion(context.Background(), &Request{Model: "m"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !asAPIError(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != 401 {
		t.Fatalf("status: %d", apiErr.StatusCode)
	}
	if apiErr.Hint == "" {
		t.Fatal("expected actionable hint for auth failure")
	}
}

func asAPIError(err error, target **APIError) bool {
	e, ok := err.(*APIError)
	if ok {
		*target = e
	}
	return ok
}

func TestFirstTokenTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush() // send headers, then never send a token
		<-r.Context().Done()
	}))
	defer srv.Close()
	cfg := testCfg(srv.URL)
	cfg.Stream = true
	cfg.InitialTokenTimeout = 1
	c := New(cfg)
	start := time.Now()
	_, err := c.ChatCompletion(context.Background(), &Request{Model: "m"}, nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("unexpected error: %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("timeout took too long")
	}
}

func TestStreamCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		f := w.(http.Flusher)
		for i := 0; i < 100; i++ {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\n")
			f.Flush()
			time.Sleep(20 * time.Millisecond)
		}
	}))
	defer srv.Close()
	cfg := testCfg(srv.URL)
	cfg.Stream = true
	c := New(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()
	_, err := c.ChatCompletion(ctx, &Request{Model: "m"}, nil)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestParseRetryAfter(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "7")
	secs, ok := ParseRetryAfterSeconds(resp)
	if !ok || secs != 7 {
		t.Fatalf("retry-after: %d %v", secs, ok)
	}
}

func TestTemperatureOmittedWhenUnset(t *testing.T) {
	var body map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{{"message": map[string]interface{}{"role": "assistant", "content": "ok"}}},
		})
	}))
	defer srv.Close()
	c := New(testCfg(srv.URL))
	if _, err := c.ChatCompletion(context.Background(), &Request{Model: "m"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["temperature"]; ok {
		t.Fatal("temperature must not be sent when not explicitly configured")
	}
}
