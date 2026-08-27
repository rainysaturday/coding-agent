package webui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coding-agent/harness/config"
)

// newTestMux builds a Server and a mux wired the same way as Serve() so tests
// can drive the HTTP handlers through httptest without binding a real port.
func newTestMux(t *testing.T) (*Server, *http.ServeMux) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Theme = "dark"
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.route)
	mux.HandleFunc("/assets/", srv.handleAsset)
	mux.HandleFunc("/api/state", srv.handleState)
	mux.HandleFunc("/api/history", srv.handleHistory)
	mux.HandleFunc("/api/chat", srv.handleChat)
	mux.HandleFunc("/api/events", srv.handleEvents)
	mux.HandleFunc("/api/command", srv.handleCommand)
	mux.HandleFunc("/api/cancel", srv.handleCancel)
	return srv, mux
}

func doJSON(t *testing.T, mux http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestState_ReturnsValidJSON(t *testing.T) {
	_, mux := newTestMux(t)
	rec := doJSON(t, mux, http.MethodGet, "/api/state?session=s1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var st stateEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if st.MaxContext != 128000 {
		t.Errorf("expected MaxContext 128000, got %d", st.MaxContext)
	}
	if st.ReadOnly {
		t.Error("expected ReadOnly false")
	}
	if st.Theme != "dark" {
		t.Errorf("expected theme dark, got %q", st.Theme)
	}
}

func TestHistory_ReturnsConversation(t *testing.T) {
	srv, mux := newTestMux(t)
	sess := srv.sessions.Get("hist-session")
	// Seed a small conversation (user -> assistant).
	sess.agent.AddUserMessage("hello")
	sess.agent.AddAssistantMessage("hi there")

	rec := doJSON(t, mux, http.MethodGet, "/api/history?session=hist-session", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp historyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(resp.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d: %+v", len(resp.Messages), resp.Messages)
	}
	if resp.Messages[0].Role != "user" || resp.Messages[0].Content != "hello" {
		t.Errorf("unexpected first message: %+v", resp.Messages[0])
	}
	if resp.Messages[1].Role != "assistant" || resp.Messages[1].Content != "hi there" {
		t.Errorf("unexpected second message: %+v", resp.Messages[1])
	}
}

func TestIndex_ServesHTML(t *testing.T) {
	_, mux := newTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("unexpected content type: %s", ct)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("Coding Agent")) {
		t.Error("index.html does not mention Coding Agent")
	}
}

func TestAssets_AreServed(t *testing.T) {
	_, mux := newTestMux(t)
	for _, path := range []string{"/assets/app.js", "/assets/styles.css", "/assets/index.html"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", path, rec.Code)
		}
		if rec.Body.Len() == 0 {
			t.Errorf("%s: empty body", path)
		}
	}
}

func TestAsset_PathTraversalRejected(t *testing.T) {
	_, mux := newTestMux(t)
	// The ServeMux cleans/normalizes ".." paths before dispatch, so a traversal
	// attempt must never result in 200 serving an asset. Accept a redirect
	// (normalization) or an explicit 403/404 as safe rejection.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/assets/../../etc/passwd", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("path traversal must not be served; got 200")
	}
}

func TestAsset_MissingReturnsNotFound(t *testing.T) {
	_, mux := newTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/assets/nope.js", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestCommand_ClearHistory(t *testing.T) {
	_, mux := newTestMux(t)
	rec := doJSON(t, mux, http.MethodPost, "/api/command", commandRequest{Session: "s2", Command: "/clear-history"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp commandResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !resp.OK {
		t.Error("expected OK true")
	}
	if resp.Output != "History cleared." {
		t.Errorf("unexpected output: %q", resp.Output)
	}
}

func TestCommand_GoalActivation(t *testing.T) {
	_, mux := newTestMux(t)
	rec := doJSON(t, mux, http.MethodPost, "/api/command", commandRequest{Session: "s3", Command: "/goal build a feature"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp commandResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !resp.State.GoalActive {
		t.Error("expected goal active after /goal")
	}
	if resp.State.Goal != "build a feature" {
		t.Errorf("expected goal 'build a feature', got %q", resp.State.Goal)
	}

	// Deactivate it again.
	rec = doJSON(t, mux, http.MethodPost, "/api/command", commandRequest{Session: "s3", Command: "/goal-off"})
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.State.GoalActive {
		t.Error("expected goal inactive after /goal-off")
	}
}

func TestCommand_ReadOnlyToggle(t *testing.T) {
	_, mux := newTestMux(t)
	rec := doJSON(t, mux, http.MethodPost, "/api/command", commandRequest{Session: "s4", Command: "/read-only"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp commandResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !resp.State.ReadOnly {
		t.Error("expected ReadOnly true after /read-only")
	}
}

func TestCommand_UnknownCommand(t *testing.T) {
	_, mux := newTestMux(t)
	rec := doJSON(t, mux, http.MethodPost, "/api/command", commandRequest{Session: "s5", Command: "/nope"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp commandResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !bytes.Contains([]byte(resp.Output), []byte("Unknown command")) {
		t.Errorf("expected unknown command message, got %q", resp.Output)
	}
}

func TestChat_InvalidBody(t *testing.T) {
	_, mux := newTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewBufferString("{not json"))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestChat_EmptyPrompt(t *testing.T) {
	_, mux := newTestMux(t)
	rec := doJSON(t, mux, http.MethodPost, "/api/chat", chatRequest{Session: "s6", Prompt: "   "})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCancel_ReturnsOK(t *testing.T) {
	_, mux := newTestMux(t)
	rec := doJSON(t, mux, http.MethodPost, "/api/cancel", cancelRequest{Session: "s7"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var out map[string]bool
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !out["ok"] {
		t.Error("expected ok true")
	}
}
