package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// sseWriter writes Server-Sent Events to an http.ResponseWriter.
// It handles the SSE framing (event:, data:, blank line) and flushing.
type sseWriter struct {
	w http.ResponseWriter
}

// newSSEWriter initializes the response for SSE and returns a writer.
func newSSEWriter(w http.ResponseWriter, r *http.Request) *sseWriter {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	// Flush headers immediately so the client can start reading.
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return &sseWriter{w: w}
}

// event writes a named event with a JSON payload.
func (s *sseWriter) event(name string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		// Fall back to a plain error event.
		s.raw("error", `{"message":"failed to marshal event"}`)
		return
	}
	s.raw(name, string(data))
}

// raw writes a named event with an already-serialized data line.
func (s *sseWriter) raw(name, data string) {
	fmt.Fprintf(s.w, "event: %s\n", name)
	fmt.Fprintf(s.w, "data: %s\n\n", data)
	s.flush()
}

// comment writes a comment line (used as a keep-alive ping).
func (s *sseWriter) comment(msg string) {
	fmt.Fprintf(s.w, ": %s\n\n", msg)
	s.flush()
}

// flush flushes buffered data to the client, if the writer supports it.
func (s *sseWriter) flush() {
	if f, ok := s.w.(http.Flusher); ok {
		f.Flush()
	}
}

// jsonMarshal is a small helper to marshal a value to JSON bytes.
func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// keepAlive periodically writes comment lines until stop is closed.
// It is intended to run in its own goroutine per SSE connection.
func (s *sseWriter) keepAlive(stop <-chan struct{}) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.comment("ping")
		case <-stop:
			return
		}
	}
}
