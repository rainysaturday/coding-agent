// Package web serves the browser based UI: a single page app with SSE
// streaming, implemented with the standard library only (requirement 046).
package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coding-agent/coding-agent/agent"
	"github.com/coding-agent/coding-agent/colors"
	"github.com/coding-agent/coding-agent/config"
	"github.com/coding-agent/coding-agent/inference"
	"github.com/coding-agent/coding-agent/version"
)

//go:embed static/*
var staticFS embed.FS

// idleTimeout bounds session memory: sessions unused for this long are GC'd.
const idleTimeout = 30 * time.Minute

// Server hosts the web UI.
type Server struct {
	cfg      *config.Config
	sessions map[string]*session
	mu       sync.Mutex
	log      *http.Server
}

// session is one agent conversation.
type session struct {
	agent    *agent.Agent
	cfg      *config.Config
	runMu    sync.Mutex
	lastUsed time.Time
	id       string

	cmu      sync.Mutex
	clients  map[chan string]bool
	history  []string
	cancelFn context.CancelFunc
	running  bool
}

// NewServer creates the web server for the resolved configuration.
func NewServer(cfg *config.Config) *Server {
	return &Server{cfg: cfg, sessions: map[string]*session{}}
}

// Handler returns the fully routed HTTP handler (used by Serve and tests).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/assets/style.css", s.handleStyle)
	mux.HandleFunc("/assets/app.js", s.handleAsset("static/app.js", "application/javascript"))
	mux.HandleFunc("/assets/app.css", s.handleAsset("static/app.css", "text/css"))
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/command", s.handleCommand)
	mux.HandleFunc("/api/cancel", s.handleCancel)
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/history", s.handleHistory)
	return mux
}

// Serve starts the HTTP server and blocks until ctx is cancelled.
func (s *Server) Serve(ctx context.Context) error {
	mux := s.Handler()

	addr := fmt.Sprintf("%s:%d", s.cfg.WebAddr, s.cfg.WebPort)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	s.log = srv

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	s.gcLoop(ctx)

	fmt.Fprintf(os.Stderr, "Web UI listening on http://%s\n", addr)
	err := srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// gcLoop removes idle sessions.
func (s *Server) gcLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			for id, sess := range s.sessions {
				if time.Since(sess.lastUsed) > idleTimeout {
					delete(s.sessions, id)
				}
			}
			s.mu.Unlock()
		}
	}
}

// getSession returns (creating when needed) the session for an ID.
func (s *Server) getSession(id string) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		id = fmt.Sprintf("s-%d", time.Now().UnixNano())
	}
	sess, ok := s.sessions[id]
	if !ok {
		cfg := *s.cfg
		sess = &session{
			agent:    agent.New(&cfg),
			cfg:      &cfg,
			id:       id,
			lastUsed: time.Now(),
			clients:  map[chan string]bool{},
		}
		if s.cfg.Goal != "" {
			sess.agent.SetGoal(s.cfg.Goal)
		}
		s.sessions[id] = sess
	}
	sess.lastUsed = time.Now()
	return sess
}

// broadcast sends a JSON payload to all SSE clients of a session.
func (sess *session) broadcast(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	sess.cmu.Lock()
	defer sess.cmu.Unlock()
	for ch := range sess.clients {
		select {
		case ch <- string(data):
		default:
		}
	}
}

// ---------------------------------------------------------------------------
// static pages
// ---------------------------------------------------------------------------

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	page, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "missing embedded page", http.StatusInternalServerError)
		return
	}
	theme := s.cfg.Theme
	if q := r.URL.Query().Get("theme"); q != "" && colors.ValidTheme(q) {
		theme = q
	}
	_ = colors.ApplyTheme(theme)
	html := strings.Replace(string(page), "/*__THEME__*/", colors.GetCurrentTheme().CSSVariables(), 1)
	html = strings.Replace(html, "__THEME_NAME__", theme, 1)
	html = strings.Replace(html, "__VERSION__", version.String(), 1)
	html = strings.Replace(html, "__MODEL__", s.cfg.Model, 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, html)
}

func (s *Server) handleStyle(w http.ResponseWriter, r *http.Request) {
	theme := s.cfg.Theme
	if q := r.URL.Query().Get("theme"); q != "" && colors.ValidTheme(q) {
		theme = q
	}
	_ = colors.ApplyTheme(theme)
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	fmt.Fprint(w, colors.GetCurrentTheme().CSSVariables())
}

func (s *Server) handleAsset(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := staticFS.ReadFile(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType+"; charset=utf-8")
		_, _ = w.Write(data)
	}
}

// ---------------------------------------------------------------------------
// API
// ---------------------------------------------------------------------------

type apiRequest struct {
	Session string `json:"session"`
	Prompt  string `json:"prompt"`
	Command string `json:"command"`
}

func decodeRequest(r *http.Request) (*apiRequest, error) {
	var req apiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	return &req, nil
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// handleChat starts an agent run for a prompt.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	req, err := decodeRequest(r)
	if err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	sess := s.getSession(req.Session)
	writeJSON(w, map[string]string{"session": sess.id, "status": "started"})
	s.runPrompt(sess, req.Prompt)
}

// runPrompt runs the agent serialized against the session run mutex.
func (s *Server) runPrompt(sess *session, prompt string) {
	sess.runMu.Lock()
	defer sess.runMu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	sess.cmu.Lock()
	sess.cancelFn = cancel
	sess.running = true
	sess.cmu.Unlock()
	defer func() {
		cancel()
		sess.cmu.Lock()
		sess.running = false
		sess.cancelFn = nil
		sess.cmu.Unlock()
	}()

	sess.agent.SetEventCallback(func(e agent.Event) { sess.broadcast(e) })
	if prompt != "" {
		sess.historyAdd(prompt)
	}
	_, err := sess.agent.Submit(ctx, prompt)
	if err != nil {
		sess.broadcast(agent.Event{Type: agent.EventError, Text: err.Error(), Failed: true})
	}
	sess.broadcast(agent.Event{Type: agent.EventDone})
	sess.broadcast(map[string]interface{}{"type": "state", "state": stateOf(sess)})
}

// historyAdd records prompt history for the session (persisted across refresh).
func (sess *session) historyAdd(prompt string) {
	sess.cmu.Lock()
	defer sess.cmu.Unlock()
	sess.history = append(sess.history, prompt)
	if max := sess.cfg.MaxHistory; max > 0 && len(sess.history) > max {
		sess.history = sess.history[len(sess.history)-max:]
	}
}

// handleEvents streams session events over SSE.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	sess := s.getSession(r.URL.Query().Get("session"))
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan string, 256)
	sess.cmu.Lock()
	sess.clients[ch] = true
	sess.cmu.Unlock()
	defer func() {
		sess.cmu.Lock()
		delete(sess.clients, ch)
		sess.cmu.Unlock()
	}()

	// Initial state frame.
	init, _ := json.Marshal(map[string]interface{}{"type": "state", "state": stateOf(sess)})
	fmt.Fprintf(w, "data: %s\n\n", init)
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

// handleCommand executes a slash command.
func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	req, err := decodeRequest(r)
	if err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	sess := s.getSession(req.Session)
	out := s.runCommand(sess, req.Command)
	writeJSON(w, map[string]interface{}{"session": sess.id, "output": out, "state": stateOf(sess)})
}

// runCommand mirrors the TUI slash commands.
func (s *Server) runCommand(sess *session, line string) string {
	cmd, rest, _ := strings.Cut(strings.TrimSpace(line), " ")
	rest = strings.TrimSpace(rest)
	switch cmd {
	case "/stats":
		return statsText(sess.agent)
	case "/clear":
		return "cleared"
	case "/clear-history":
		sess.cmu.Lock()
		sess.history = nil
		sess.cmu.Unlock()
		return "Input history cleared."
	case "/compress":
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := sess.agent.Compress(ctx); err != nil {
			return "Compression failed: " + err.Error()
		}
		return "Context compressed."
	case "/goal":
		if rest == "" {
			goal, start := sess.agent.Goal()
			if goal == "" {
				return "Goal mode: inactive"
			}
			return fmt.Sprintf("Goal mode: %q (elapsed %s)", goal, agent.FormatDuration(time.Since(start)))
		}
		sess.agent.SetGoal(rest)
		go s.runPrompt(sess, rest)
		return "Goal set: " + rest
	case "/goal-off":
		sess.agent.ClearGoal()
		return "Goal mode deactivated."
	case "/dump":
		path, err := sess.agent.DumpContext()
		if err != nil {
			return "Dump failed: " + err.Error()
		}
		return "Context dumped to: " + path
	case "/read-only":
		v := !sess.agent.ReadOnly()
		sess.agent.SetReadOnly(v)
		if v {
			return "Read-only mode enabled."
		}
		return "Read-only mode disabled."
	case "/skills":
		mgr := sess.agent.Skills()
		if mgr == nil {
			return "Skills are disabled."
		}
		var b strings.Builder
		for _, sk := range mgr.List() {
			fmt.Fprintf(&b, "%s: %s\n", sk.Name, sk.Description)
		}
		if b.Len() == 0 {
			return "No skills discovered."
		}
		return strings.TrimRight(b.String(), "\n")
	case "/skill":
		mgr := sess.agent.Skills()
		if mgr == nil {
			return "Skills are disabled."
		}
		content, err := mgr.Activate(rest)
		if err != nil {
			return err.Error()
		}
		sess.agent.AddMessage(inference.NewTextMessage("user", "Skill '"+rest+"' manually activated by the user:\n\n"+content))
		return "Skill '" + rest + "' activated."
	case "/help":
		return "Commands: /stats /clear /clear-history /compress /goal <text> /goal-off /dump /read-only /skills /skill <name>"
	default:
		return "Unknown command: " + cmd
	}
}

// handleCancel cancels the running operation.
func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	req, err := decodeRequest(r)
	if err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	sess := s.getSession(req.Session)
	sess.agent.Cancel()
	writeJSON(w, map[string]string{"session": sess.id, "status": "cancelling"})
}

// handleState returns the current session state.
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	sess := s.getSession(r.URL.Query().Get("session"))
	writeJSON(w, map[string]interface{}{"session": sess.id, "state": stateOf(sess)})
}

// handleHistory returns the session prompt history.
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	sess := s.getSession(r.URL.Query().Get("session"))
	sess.cmu.Lock()
	h := append([]string(nil), sess.history...)
	sess.cmu.Unlock()
	writeJSON(w, map[string]interface{}{"session": sess.id, "history": h})
}

// stateOf snapshots the session state for the UI.
func stateOf(sess *session) map[string]interface{} {
	goal, _ := sess.agent.Goal()
	snap := sess.agent.Stats().Snapshot()
	sess.cmu.Lock()
	running := sess.running
	sess.cmu.Unlock()
	return map[string]interface{}{
		"context_tokens":  sess.agent.ContextTokens(),
		"context_limit":   sess.agent.ContextLimit(),
		"context_percent": fmt.Sprintf("%.1f", sess.agent.ContextPercent()),
		"read_only":       sess.agent.ReadOnly(),
		"goal":            goal,
		"running":         running,
		"theme":           sess.cfg.Theme,
		"model":           sess.cfg.Model,
		"version":         version.String(),
		"stats": map[string]interface{}{
			"input_tokens":      snap.InputTokens,
			"output_tokens":     snap.OutputTokens,
			"cached_tokens":     snap.CachedTokens,
			"tool_calls":        snap.ToolCalls,
			"failed_tool_calls": snap.FailedToolCalls,
			"iterations":        snap.Iterations,
			"compressions":      snap.CompressionCount,
			"tokens_per_second": fmt.Sprintf("%.2f", sess.agent.Stats().TokensPerSecond()),
			"uptime":            agent.FormatDuration(time.Since(snap.StartTime)),
		},
	}
}

// statsText renders the /stats output.
func statsText(a *agent.Agent) string {
	snap := a.Stats().Snapshot()
	var b strings.Builder
	b.WriteString("Runtime Statistics\n")
	b.WriteString(strings.Repeat("=", 50) + "\n")
	fmt.Fprintf(&b, "Input Tokens:    %d\n", snap.InputTokens)
	fmt.Fprintf(&b, "Output Tokens:   %d\n", snap.OutputTokens)
	if snap.CachedTokens > 0 {
		fmt.Fprintf(&b, "Cached Tokens:   %d\n", snap.CachedTokens)
	}
	fmt.Fprintf(&b, "Current Context: %d / %d (%.1f%%)\n", a.ContextTokens(), a.ContextLimit(), a.ContextPercent())
	fmt.Fprintf(&b, "Tokens/Second:   %.2f\n", a.Stats().TokensPerSecond())
	fmt.Fprintf(&b, "Tool Calls:      %d\n", snap.ToolCalls)
	fmt.Fprintf(&b, "Failed Calls:    %d\n", snap.FailedToolCalls)
	fmt.Fprintf(&b, "Iterations:      %d\n", snap.Iterations)
	fmt.Fprintf(&b, "Compressions:    %d\n", snap.CompressionCount)
	fmt.Fprintf(&b, "Uptime:          %s", agent.FormatDuration(time.Since(snap.StartTime)))
	return b.String()
}

// sortedKeys is a small helper used by tests and debugging.
func sortedKeys(m map[string]*session) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
