package webui

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

	// Empty id resolves to the shared default session (see
	// TestSessionManager_EmptyIDReturnsSharedDefault).
	s3 := m.Get("")
	if s3.ID != DefaultSessionID {
		t.Errorf("expected default session id %q for empty id, got %q", DefaultSessionID, s3.ID)
	}
}

// TestSessionManager_EmptyIDReturnsSharedDefault verifies that empty ids all
// resolve to the same shared DefaultSessionID, so every anonymous connection
// continues the same server-side session.
func TestSessionManager_EmptyIDReturnsSharedDefault(t *testing.T) {
	cfg := config.DefaultConfig()
	m := NewSessionManager(cfg)
	a := m.Get("")
	b := m.Get("")
	if a != b {
		t.Error("expected empty-id calls to return the same shared default session")
	}
	if a.ID != DefaultSessionID {
		t.Errorf("expected default session id %q, got %q", DefaultSessionID, a.ID)
	}
}

// TestSession_ResetClearsState verifies reset clears context, goal, and history.
func TestSession_ResetClearsState(t *testing.T) {
	cfg := config.DefaultConfig()
	m := NewSessionManager(cfg)
	s := m.Get("sess")

	s.agent.SetGoal("some goal")
	s.addToHistory("hello")
	if !s.agent.IsGoalActive() {
		t.Fatal("expected goal active after SetGoal")
	}
	if len(s.historySnapshot()) != 1 {
		t.Fatal("expected one history entry after addToHistory")
	}

	s.reset()

	if s.agent.IsGoalActive() {
		t.Error("expected goal inactive after reset")
	}
	if len(s.historySnapshot()) != 0 {
		t.Error("expected empty history after reset")
	}
	// After reset the context should be back to the system-prompt baseline:
	// a fresh session (empty conversation) reports the same size, since
	// GetContextSize always includes the system prompt.
	fresh := m.Get("fresh")
	if got := s.agent.GetContextSize(); got != fresh.agent.GetContextSize() {
		t.Errorf("expected context size %d (system-prompt baseline) after reset, got %d",
			fresh.agent.GetContextSize(), got)
	}
}


// TestSession_BroadcastDropsCounted verifies that when a subscriber's channel
// is full, dropped events are counted (I-09) and terminal events are still
// delivered by evicting the oldest queued event.
func TestSession_BroadcastDropsCounted(t *testing.T) {
	cfg := config.DefaultConfig()
	m := NewSessionManager(cfg)
	s := m.Get("sess")

	sub := s.addSubscriber()
	defer s.removeSubscriber(sub)

	// Fill the subscriber channel to capacity so broadcasts must drop.
	for {
		select {
		case sub.ch <- sseMessage{event: "chunk", data: "x"}:
		default:
			goto filled
		}
	}
filled:

	// Non-terminal events are dropped and counted.
	s.broadcast("chunk", "dropped-me")
	s.broadcast("chunk", "dropped-me-2")
	if got := atomic.LoadInt64(&sub.dropped); got < 2 {
		t.Fatalf("expected at least 2 dropped, got %d", got)
	}

	// A terminal event evicts an older queued event so it is delivered.
	done := make(chan sseMessage, 1)
	go func() {
		for msg := range sub.ch {
			if msg.event == "result" {
				done <- msg
				return
			}
		}
	}()
	s.broadcast("result", "{}")
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("terminal result event was not delivered")
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
