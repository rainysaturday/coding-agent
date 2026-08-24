package webui

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coding-agent/harness/config"
)

// TestSSEWriter_Framing verifies the SSE writer produces correct
// "event:" / "data:" frames separated by blank lines.
func TestSSEWriter_Framing(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/events", nil)
	sw := newSSEWriter(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected event-stream content type, got %q", ct)
	}

	sw.raw("state", `{"running":false}`)
	sw.raw("chunk", `{"text":"hi","contentType":0}`)

	body := rec.Body.String()
	if !strings.Contains(body, "event: state\n") {
		t.Errorf("missing 'event: state' line:\n%s", body)
	}
	if !strings.Contains(body, "data: {\"running\":false}\n\n") {
		t.Errorf("missing state data frame:\n%s", body)
	}
	if !strings.Contains(body, "event: chunk\n") {
		t.Errorf("missing 'event: chunk' line:\n%s", body)
	}
	if !strings.Contains(body, "data: {\"text\":\"hi\",\"contentType\":0}\n\n") {
		t.Errorf("missing chunk data frame:\n%s", body)
	}
}

// TestSSEWriter_EventMarshalsPayload verifies the named event helper
// marshals a payload and writes a valid frame.
func TestSSEWriter_EventMarshalsPayload(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/events", nil)
	sw := newSSEWriter(rec, req)

	sw.event("done", doneEvent{OK: true})

	body := rec.Body.String()
	if !strings.Contains(body, "event: done\n") {
		t.Errorf("missing 'event: done' line:\n%s", body)
	}
	if !strings.Contains(body, `"ok":true`) {
		t.Errorf("missing marshaled payload:\n%s", body)
	}
}

// TestSessionManager_CreateAndGet verifies sessions are created lazily and
// reused for the same id.
func TestSessionManager_CreateAndGet(t *testing.T) {
	cfg := config.DefaultConfig()
	m := NewSessionManager(cfg)

	s1 := m.Get("abc")
	if s1 == nil {
		t.Fatal("Get returned nil")
	}
	if s1.ID != "abc" {
		t.Errorf("expected ID 'abc', got %q", s1.ID)
	}

	// Same id returns the same session.
	s2 := m.Get("abc")
	if s2 != s1 {
		t.Error("expected same session for same id")
	}

	// Empty id generates a fresh, non-empty id each time.
	s3 := m.Get("")
	if s3.ID == "" {
		t.Error("expected a generated id for empty session id")
	}
}

// TestSessionManager_EmptyIDGeneratesUniqueSessions verifies that empty ids
// never collide (each Get generates a new session).
func TestSessionManager_EmptyIDGeneratesUniqueSessions(t *testing.T) {
	cfg := config.DefaultConfig()
	m := NewSessionManager(cfg)
	a := m.Get("")
	b := m.Get("")
	if a == b {
		t.Error("expected distinct sessions for distinct empty-id calls")
	}
}

// TestSession_RunSerialization verifies that a second run is rejected while
// one is already in progress.
func TestSession_RunSerialization(t *testing.T) {
	cfg := config.DefaultConfig()
	m := NewSessionManager(cfg)
	s := m.Get("sess")

	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	if !s.isRunning() {
		t.Error("expected isRunning true after manual flag")
	}
	if err := s.run(context.Background(), "hello"); err == nil {
		t.Error("expected run to be rejected while a run is in progress")
	}

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}
