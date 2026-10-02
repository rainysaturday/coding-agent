package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coding-agent/coding-agent/config"
	"github.com/coding-agent/coding-agent/inference"
)

// mockBackend serves scripted chat completion responses in order.
type mockBackend struct {
	mu        sync.Mutex
	responses []func(req inference.Request, n int) map[string]interface{}
	requests  []inference.Request
	srv       *httptest.Server
}

func newMockBackend(t *testing.T, responses ...func(req inference.Request, n int) map[string]interface{}) *mockBackend {
	t.Helper()
	m := &mockBackend{responses: responses}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req inference.Request
		_ = json.NewDecoder(r.Body).Decode(&req)
		m.mu.Lock()
		n := len(m.requests)
		m.requests = append(m.requests, req)
		var fn func(inference.Request, int) map[string]interface{}
		if n < len(m.responses) {
			fn = m.responses[n]
		} else if len(m.responses) > 0 {
			fn = m.responses[len(m.responses)-1] // repeat last
		}
		m.mu.Unlock()
		resp := map[string]interface{}{"choices": []map[string]interface{}{{"message": map[string]interface{}{"role": "assistant", "content": "default"}}}}
		if fn != nil {
			resp = fn(req, n)
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(m.srv.Close)
	return m
}

func (m *mockBackend) requestCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.requests)
}

func textResp(content string) func(inference.Request, int) map[string]interface{} {
	return func(inference.Request, int) map[string]interface{} {
		return map[string]interface{}{
			"choices": []map[string]interface{}{{
				"message":       map[string]interface{}{"role": "assistant", "content": content},
				"finish_reason": "stop",
			}},
			"usage": map[string]int{"prompt_tokens": 50, "completion_tokens": 10, "total_tokens": 60},
		}
	}
}

func toolResp(id, name, args string) func(inference.Request, int) map[string]interface{} {
	return func(inference.Request, int) map[string]interface{} {
		return map[string]interface{}{
			"choices": []map[string]interface{}{{
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "",
					"tool_calls": []map[string]interface{}{{
						"id":       id,
						"type":     "function",
						"function": map[string]string{"name": name, "arguments": args},
					}},
				},
				"finish_reason": "tool_calls",
			}},
		}
	}
}

func testAgentCfg(t *testing.T, endpoint string) *config.Config {
	t.Helper()
	c := config.NewDefault()
	c.APIEndpoint = endpoint
	c.APIKey = "test"
	c.Stream = false
	c.SkillsEnabled = false
	c.WorkingDir = t.TempDir()
	c.AutoCompress = false
	return c
}

func collectEvents(a *Agent) *[]Event {
	var mu sync.Mutex
	var evs []Event
	a.SetEventCallback(func(e Event) {
		mu.Lock()
		evs = append(evs, e)
		mu.Unlock()
	})
	return &evs
}

func TestSubmitSimpleConversation(t *testing.T) {
	mb := newMockBackend(t, textResp("Hello there!"))
	a := New(testAgentCfg(t, mb.srv.URL))
	evs := collectEvents(a)

	out, err := a.Submit(context.Background(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	if out != "Hello there!" {
		t.Fatalf("answer: %q", out)
	}
	ctx := a.Context()
	if len(ctx) != 2 || ctx[0].Role != "user" || ctx[1].Role != "assistant" {
		t.Fatalf("context: %+v", ctx)
	}
	if ctx[0].Text() != "hi" {
		t.Fatalf("user message: %q", ctx[0].Text())
	}
	// The system prompt is injected per request, not stored.
	req := mb.requests[0]
	if req.Messages[0].Role != "system" || req.Messages[1].Role != "user" {
		t.Fatalf("request messages: %+v", req.Messages)
	}
	// Events: iteration, content, done.
	var sawIter, sawContent, sawDone bool
	for _, e := range *evs {
		switch e.Type {
		case EventIteration:
			sawIter = true
		case EventContent:
			sawContent = true
		case EventDone:
			sawDone = true
		}
	}
	if !sawIter || !sawContent || !sawDone {
		t.Fatalf("events: %+v", *evs)
	}
	st := a.Stats().Snapshot()
	if st.Iterations != 1 || st.InputTokens != 50 || st.OutputTokens != 10 {
		t.Fatalf("stats: %+v", st)
	}
}

func TestToolCallRoundTrip(t *testing.T) {
	mb := newMockBackend(t,
		toolResp("call_1", "bash", `{"command":"echo toolworks"}`),
		textResp("The command printed toolworks."),
	)
	a := New(testAgentCfg(t, mb.srv.URL))
	evs := collectEvents(a)

	out, err := a.Submit(context.Background(), "run echo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "toolworks") {
		t.Fatalf("final: %q", out)
	}
	ctx := a.Context()
	if len(ctx) != 4 {
		t.Fatalf("context len: %d", len(ctx))
	}
	toolMsg := ctx[2]
	if toolMsg.Role != "tool" || toolMsg.ToolCallID != "call_1" {
		t.Fatalf("tool message: %+v", toolMsg)
	}
	// Requirement 016: success wrapper format.
	if !strings.HasPrefix(toolMsg.Text(), "Tool 'bash' executed successfully:\n") || !strings.Contains(toolMsg.Text(), "toolworks") {
		t.Fatalf("tool message wrapper: %q", toolMsg.Text())
	}
	if len(mb.requests) != 2 {
		t.Fatalf("requests: %d", len(mb.requests))
	}
	// Second request must include the tool result with tool_call_id.
	second := mb.requests[1]
	var found bool
	for _, m := range second.Messages {
		if m.Role == "tool" && m.ToolCallID == "call_1" {
			found = true
		}
	}
	if !found {
		t.Fatal("second request missing tool result message")
	}
	var sawStart, sawResult bool
	for _, e := range *evs {
		if e.Type == EventToolStart && e.Tool == "bash" {
			sawStart = true
		}
		if e.Type == EventToolResult && strings.Contains(e.Result, "toolworks") {
			sawResult = true
		}
	}
	if !sawStart || !sawResult {
		t.Fatalf("tool events missing: %+v", *evs)
	}
}

// Requirement 016 + 018: failed tool calls are wrapped as
// "Tool 'name' failed: ..." in the context and counted in statistics.
func TestToolFailureWrapperAndStats(t *testing.T) {
	mb := newMockBackend(t,
		toolResp("call_1", "bash", `{"command":"exit 7"}`),
		textResp("that failed"),
	)
	a := New(testAgentCfg(t, mb.srv.URL))
	if _, err := a.Submit(context.Background(), "fail"); err != nil {
		t.Fatal(err)
	}
	var toolMsg *inference.Message
	for _, m := range a.Context() {
		if m.Role == "tool" {
			toolMsg = m
		}
	}
	if toolMsg == nil {
		t.Fatal("no tool message in context")
	}
	if !strings.HasPrefix(toolMsg.Text(), "Tool 'bash' failed: ") || !strings.Contains(toolMsg.Text(), "exit status 7") {
		t.Fatalf("failure wrapper: %q", toolMsg.Text())
	}
	st := a.Stats().Snapshot()
	if st.FailedToolCalls != 1 || st.ToolCalls != 1 {
		t.Fatalf("stats: %+v", st)
	}
}

func TestMaxIterationsStops(t *testing.T) {
	mb := newMockBackend(t, toolResp("c", "bash", `{"command":"true"}`))
	cfg := testAgentCfg(t, mb.srv.URL)
	cfg.MaxIterations = 3
	a := New(cfg)
	collectEvents(a)

	_, err := a.Submit(context.Background(), "loop forever")
	if err == nil {
		t.Fatal("expected max iterations error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "maximum number of iterations") {
		t.Fatalf("error: %v", err)
	}
	if mb.requestCount() != 3 {
		t.Fatalf("requests: %d", mb.requestCount())
	}
	if code := ExitCodeFor(err); code != 1 {
		t.Fatalf("exit code: %d", code)
	}
}

func TestGoalModeAchieved(t *testing.T) {
	mb := newMockBackend(t,
		textResp("I fixed the bug and ran the tests."),
		textResp("Verified: goal achieved."),
	)
	a := New(testAgentCfg(t, mb.srv.URL))
	evs := collectEvents(a)
	a.SetGoal("fix the bug")

	out, err := a.Submit(context.Background(), "fix the bug")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "goal achieved") {
		t.Fatalf("final: %q", out)
	}
	goal, _ := a.Goal()
	if goal != "" {
		t.Fatalf("goal should be cleared, got %q", goal)
	}
	// A goal-check user message was injected between the two assistant turns.
	ctx := a.Context()
	var goalCheckInjected bool
	for _, m := range ctx {
		if m.Role == "user" && strings.Contains(m.Text(), "goal achieved") {
			goalCheckInjected = true
		}
	}
	if !goalCheckInjected {
		t.Fatal("goal check prompt not injected")
	}
	var sawGoal bool
	for _, e := range *evs {
		if e.Type == EventGoal {
			sawGoal = true
		}
	}
	if !sawGoal {
		t.Fatal("expected goal event")
	}
}

func TestGoalNotAchievedKeepsLooping(t *testing.T) {
	// First stop: no "goal achieved" phrase -> agent injects check; second
	// response still lacks the phrase -> inject again; third achieves it.
	mb := newMockBackend(t,
		textResp("made progress"),
		textResp("more progress"),
		textResp("all done, goal achieved"),
	)
	cfg := testAgentCfg(t, mb.srv.URL)
	cfg.MaxIterations = 10
	a := New(cfg)
	a.SetGoal("ship it")
	if _, err := a.Submit(context.Background(), "ship it"); err != nil {
		t.Fatal(err)
	}
	if mb.requestCount() != 3 {
		t.Fatalf("requests: %d", mb.requestCount())
	}
	goal, _ := a.Goal()
	if goal != "" {
		t.Fatal("goal should be cleared after achievement")
	}
}

func TestCompressionPreservesEnds(t *testing.T) {
	mb := newMockBackend(t, textResp("SUMMARY OF MIDDLE"))
	a := New(testAgentCfg(t, mb.srv.URL))
	a.AddMessage(inference.NewTextMessage("user", "FIRST PROMPT"))
	for i := 0; i < 10; i++ {
		a.AddMessage(inference.NewTextMessage("assistant", fmt.Sprintf("middle %d", i)))
		a.AddMessage(inference.NewTextMessage("user", fmt.Sprintf("reply %d", i)))
	}
	a.AddMessage(inference.NewTextMessage("user", "LAST PROMPT"))

	if err := a.Compress(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx := a.Context()
	if len(ctx) >= 13 {
		t.Fatalf("context not shrunk: %d", len(ctx))
	}
	if ctx[0].Role != "user" || ctx[0].Text() != "FIRST PROMPT" {
		t.Fatalf("first user message not preserved: %+v", ctx[0])
	}
	var sawSummary bool
	for _, m := range ctx {
		if strings.Contains(m.Text(), "SUMMARY OF MIDDLE") {
			sawSummary = true
		}
	}
	if !sawSummary {
		t.Fatal("summary message missing")
	}
	last := ctx[len(ctx)-1]
	if last.Text() != "LAST PROMPT" {
		t.Fatalf("tail not preserved: %+v", last)
	}
	if st := a.Stats().Snapshot(); st.CompressionCount != 1 {
		t.Fatalf("compression count: %d", st.CompressionCount)
	}
}

func TestShouldCompressThreshold(t *testing.T) {
	cfg := testAgentCfg(t, "http://127.0.0.1:1")
	cfg.AutoCompress = true
	cfg.ContextSize = 1000
	cfg.ContextThreshold = 0.8
	a := New(cfg)
	if a.ShouldCompress() {
		t.Fatal("empty context should not compress")
	}
	a.AddMessage(inference.NewTextMessage("user", strings.Repeat("x", 4*850)))
	if !a.ShouldCompress() {
		t.Fatalf("should compress at %.1f%%", a.ContextPercent())
	}
}

func TestDumpAndLoadContext(t *testing.T) {
	mb := newMockBackend(t, textResp("answer one"))
	a := New(testAgentCfg(t, mb.srv.URL))
	if _, err := a.Submit(context.Background(), "question one"); err != nil {
		t.Fatal(err)
	}
	path, err := a.DumpContext()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	if !strings.Contains(path, "coding-agent-context") {
		t.Fatalf("dump path: %q", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var dump ContextDump
	if err := json.Unmarshal(data, &dump); err != nil {
		t.Fatal(err)
	}
	if dump.Version != DumpVersion {
		t.Fatalf("version: %d", dump.Version)
	}
	if len(dump.Iterations) == 0 {
		t.Fatal("no iterations recorded")
	}
	last := dump.Iterations[len(dump.Iterations)-1]
	if len(last.Messages) < 2 || last.Messages[0].Role != "system" {
		t.Fatalf("iteration messages: %+v", last.Messages)
	}

	// A fresh agent restores the conversation (minus system prompt).
	a2 := New(testAgentCfg(t, mb.srv.URL))
	n, lastUser, err := a2.LoadContext(path)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("restored messages: %d", n)
	}
	if lastUser != "question one" {
		t.Fatalf("last user prompt: %q", lastUser)
	}
	if a2.Context()[0].Role != "user" {
		t.Fatal("system prompt should be stripped on load")
	}

	if _, _, err := a2.LoadContext(filepath.Join(path, "missing.json")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestUniqueDumpPaths(t *testing.T) {
	dir := t.TempDir()
	old := os.Getenv("TMPDIR")
	os.Setenv("TMPDIR", dir)
	defer os.Setenv("TMPDIR", old)
	p1 := uniqueDumpPath()
	if err := os.WriteFile(p1, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	p2 := uniqueDumpPath()
	if p1 == p2 {
		t.Fatalf("paths must differ: %s", p1)
	}
	if !strings.Contains(p2, "-2") {
		t.Fatalf("second path: %q", p2)
	}
}

func TestExitCodeMapping(t *testing.T) {
	if got := ExitCodeFor(nil); got != 0 {
		t.Fatalf("nil: %d", got)
	}
	if got := ExitCodeFor(context.Canceled); got != 1 {
		t.Fatalf("canceled: %d", got)
	}
	if got := ExitCodeFor(&inference.APIError{StatusCode: 401, Message: "Unauthorized"}); got != 3 {
		t.Fatalf("401: %d", got)
	}
	if got := ExitCodeFor(&inference.APIError{StatusCode: 400, Message: "context length exceeded"}); got != 4 {
		t.Fatalf("context 400: %d", got)
	}
	if got := ExitCodeFor(&inference.APIError{StatusCode: 500, Message: "boom"}); got != 1 {
		t.Fatalf("500: %d", got)
	}
}

func TestFlattenArgs(t *testing.T) {
	got := FlattenArgs(`{"command":"ls -la","timeout":5000}`)
	if !strings.Contains(got, "command:") || !strings.Contains(got, "ls -la") || !strings.Contains(got, "timeout: 5000") {
		t.Fatalf("flatten: %q", got)
	}
	// Partial JSON while streaming must not error out.
	got = FlattenArgs(`{"command":"ls -l`)
	if !strings.Contains(got, "ls -l") {
		t.Fatalf("partial flatten: %q", got)
	}
	if FlattenArgs("") != "" {
		t.Fatal("empty args")
	}
}

func TestEstimateTokensGrows(t *testing.T) {
	empty := EstimateTokens(nil)
	big := EstimateTokens([]*inference.Message{inference.NewTextMessage("user", strings.Repeat("word ", 400))})
	if big <= empty {
		t.Fatalf("estimate should grow: %d vs %d", empty, big)
	}
}

func TestReadOnlyFiltersWriteTools(t *testing.T) {
	mb := newMockBackend(t, textResp("ok"))
	cfg := testAgentCfg(t, mb.srv.URL)
	cfg.ReadOnly = true
	a := New(cfg)
	for _, d := range a.Registry().Definitions() {
		if d.Function.Name == "bash" || d.Function.Name == "write_file" {
			t.Fatalf("write tool %q exposed in read-only mode", d.Function.Name)
		}
	}
	if _, ok := a.Registry().Get("read_file"); !ok {
		t.Fatal("read_file should remain available")
	}
}

func TestCancelDuringStream(t *testing.T) {
	// A slow server; cancel mid-flight.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		f := w.(http.Flusher)
		for i := 0; i < 200; i++ {
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"tick\"}}]}\n\n")
			f.Flush()
			if d, ok := r.Context().Deadline(); ok {
				_ = d
			}
			select {
			case <-r.Context().Done():
				return
			default:
			}
			select {
			case <-time.After(20 * time.Millisecond):
			case <-r.Context().Done():
				return
			}
		}
	}))
	defer srv.Close()

	cfg := testAgentCfg(t, srv.URL)
	cfg.Stream = true
	a := New(cfg)
	go func() {
		time.Sleep(200 * time.Millisecond)
		a.Cancel()
	}()
	_, err := a.Submit(context.Background(), "stream forever")
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "cancel") {
		t.Fatalf("cancel error: %v", err)
	}
}

// Requirement 040: summary extraction from subagent output.
func TestExtractSummary(t *testing.T) {
	if got := extractSummary("noise\n=== Final Output ===\nthe summary"); got != "the summary" {
		t.Fatalf("marker: %q", got)
	}
	long := strings.Repeat("x", 6000)
	got := extractSummary(long)
	if !strings.HasPrefix(got, "xxx") || !strings.HasSuffix(got, "[output truncated]") || len(got) > 5100 {
		t.Fatalf("truncation: len=%d", len(got))
	}
	if got := extractSummary(""); got != "" {
		t.Fatalf("empty: %q", got)
	}
	if got := extractSummary("plain conclusion"); got != "plain conclusion" {
		t.Fatalf("plain: %q", got)
	}
}
