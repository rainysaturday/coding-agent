package webui

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAPI_RejectsCrossOrigin verifies that a POST from a foreign origin is
// rejected, preventing a malicious page from driving the shared session.
func TestAPI_RejectsCrossOrigin(t *testing.T) {
	_, mux := newTestMux(t)
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewBufferString(`{"prompt":"echo pwned"}`))
	req.Host = "127.0.0.1:8080"
	req.Header.Set("Origin", "http://evil.example")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for cross-origin POST, got %d", rec.Code)
	}
}

// TestAPI_RejectsNullOrigin verifies that a null Origin (sandboxed iframe,
// file://) is rejected.
func TestAPI_RejectsNullOrigin(t *testing.T) {
	_, mux := newTestMux(t)
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewBufferString(`{"prompt":"echo pwned"}`))
	req.Host = "127.0.0.1:8080"
	req.Header.Set("Origin", "null")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for null origin POST, got %d", rec.Code)
	}
}

// TestAPI_RejectsNonJSONContentType verifies that a POST with a non-JSON
// content type (a CORS "simple request" that skips preflight) is rejected.
func TestAPI_RejectsNonJSONContentType(t *testing.T) {
	_, mux := newTestMux(t)
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewBufferString(`{"prompt":"echo pwned"}`))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("expected 415 for non-JSON content type, got %d", rec.Code)
	}
}

// TestAPI_AcceptsSameOrigin verifies that a same-origin JSON POST is accepted.
func TestAPI_AcceptsSameOrigin(t *testing.T) {
	_, mux := newTestMux(t)
	req := httptest.NewRequest(http.MethodPost, "/api/cancel", bytes.NewBufferString(`{"session":"default"}`))
	req.Host = "127.0.0.1:8080"
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for same-origin POST, got %d", rec.Code)
	}
}

// TestAPI_AllowsGETWithoutJSON verifies that GET /api/state is unaffected by
// the JSON content-type requirement.
func TestAPI_AllowsGETWithoutJSON(t *testing.T) {
	_, mux := newTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/state?session=s", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for GET /api/state, got %d", rec.Code)
	}
}
