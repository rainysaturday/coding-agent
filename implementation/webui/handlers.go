package webui

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handleIndex serves the embedded single-page app shell.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	data, ok := staticAsset("index.html")
	if !ok {
		http.Error(w, "index.html not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(data)
}

// handleAsset serves embedded static assets (js, css, svg, png, ico).
func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/assets/")
	if name == "" || strings.Contains(name, "..") || strings.Contains(name, "/") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	data, ok := staticAsset(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	switch {
	case strings.HasSuffix(name, ".js"):
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case strings.HasSuffix(name, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(name, ".svg"):
		w.Header().Set("Content-Type", "image/svg+xml")
	case strings.HasSuffix(name, ".png"):
		w.Header().Set("Content-Type", "image/png")
	case strings.HasSuffix(name, ".ico"):
		w.Header().Set("Content-Type", "image/x-icon")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Write(data)
}

// handleState returns the current session state as JSON.
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	sess := s.sessions.Get(session)
	writeJSON(w, http.StatusOK, sess.state())
}

// handleChat starts an agent run for a session.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt is required"})
		return
	}
	sess := s.sessions.Get(req.Session)
	// Record prompt in history before dispatch.
	sess.addToHistory(prompt)

	if strings.HasPrefix(prompt, "/") {
		out := s.dispatchCommand(sess, prompt)
		writeJSON(w, http.StatusOK, out)
		return
	}

	if sess.isRunning() {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "a run is already in progress"})
		return
	}

	// Run in the background so the SSE stream can deliver events.
	go func() {
		_ = sess.run(s.baseCtx, prompt)
	}()

	writeJSON(w, http.StatusAccepted, chatResponse{OK: true, Session: sess.ID})
}

// handleEvents is the SSE endpoint: streams chunks, results, stats, state.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	session := r.URL.Query().Get("session")
	sess := s.sessions.Get(session)
	sw := newSSEWriter(w, r)

	// Send the initial state immediately (helps reconnecting clients).
	sw.event("state", sess.state())

	sub := sess.addSubscriber()
	defer sess.removeSubscriber(sub)

	// Start a keep-alive ping loop.
	go sw.keepAlive(sub.stop)

	for {
		select {
		case <-r.Context().Done():
			return
		case <-sub.stop:
			return
		case msg := <-sub.ch:
			sw.raw(msg.event, msg.data)
		}
	}
}

// handleCommand dispatches a slash command and returns the result.
func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	var req commandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	cmd := strings.TrimSpace(req.Command)
	if cmd == "" || !strings.HasPrefix(cmd, "/") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "command must start with '/'"})
		return
	}
	sess := s.sessions.Get(req.Session)
	out := s.dispatchCommand(sess, cmd)
	writeJSON(w, http.StatusOK, out)
}

// handleCancel cancels the current run for a session.
func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	var req cancelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	sess := s.sessions.Get(req.Session)
	sess.cancel()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
