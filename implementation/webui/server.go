package webui

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coding-agent/harness/agent"
	"github.com/coding-agent/harness/colors"
	"github.com/coding-agent/harness/config"
)

// Server is the web UI HTTP server.
type Server struct {
	cfg      *config.Config
	sessions *SessionManager
	baseCtx  context.Context
	httpSrv  *http.Server
	reapStop chan struct{} // closed on shutdown to stop the session reaper
}

// NewServer creates a web UI server from a config.
func NewServer(cfg *config.Config) *Server {
	return &Server{
		cfg:      cfg,
		sessions: NewSessionManager(cfg),
		baseCtx:  context.Background(),
		reapStop: make(chan struct{}),
	}
}

// Serve starts the HTTP server and blocks until the server shuts down.
// It returns the server so callers can perform graceful shutdown.
func (s *Server) Serve() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.WebAddr, s.cfg.WebPort)

	mux := s.buildMux()

	s.httpSrv = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start the session reaper so idle sessions are GC'd (requirement 046).
	if timeout := s.cfg.WebSessionIdleTimeout; timeout > 0 {
		go s.reapSessions(time.Duration(timeout) * time.Second)
	}

	fmt.Printf("%s============================================================%s\n", colors.GetColor("blue"), colors.GetColor("reset"))
	fmt.Printf("%s  Coding Agent Web UI%s\n", colors.GetColor("blue"), colors.GetColor("reset"))
	fmt.Printf("%s============================================================%s\n", colors.GetColor("blue"), colors.GetColor("reset"))
	fmt.Printf("  %sWeb UI running at: %shttp://%s%s\n", colors.GetColor("cyan"), colors.GetColor("reset"), addr, colors.GetColor("reset"))
	fmt.Printf("  %sPress Ctrl+C to stop.%s\n", colors.GetColor("dim"), colors.GetColor("reset"))
	fmt.Println()

	return s.httpSrv.ListenAndServe()
}

// buildMux constructs the HTTP routing for the web UI. It is factored out so
// tests can drive the exact same routing used in production. All /api/ routes
// are wrapped in an origin/content-type guard so a foreign web page cannot
// drive tool execution on the shared session.
func (s *Server) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.route)
	mux.HandleFunc("/assets/", s.handleAsset)
	mux.HandleFunc("/api/state", s.guardAPI(s.handleState))
	mux.HandleFunc("/api/history", s.guardAPI(s.handleHistory))
	mux.HandleFunc("/api/chat", s.guardAPI(s.handleChat))
	mux.HandleFunc("/api/events", s.guardAPI(s.handleEvents))
	mux.HandleFunc("/api/command", s.guardAPI(s.handleCommand))
	mux.HandleFunc("/api/cancel", s.guardAPI(s.handleCancel))
	mux.HandleFunc("/api/reset", s.guardAPI(s.handleReset))
	return mux
}

// guardAPI wraps an /api/ handler with CSRF / cross-origin protections for
// state-changing POST requests:
//
//   - Requests whose Origin (or Referer) is present but does not match the
//     server's own Host are rejected. Browsers always attach Origin to POSTs,
//     so a foreign page cannot silently drive the shared session.
//   - JSON endpoints require Content-Type: application/json, which forces a
//     CORS preflight that a cross-origin page cannot satisfy.
func (s *Server) guardAPI(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			next(w, r)
			return
		}
		// Reject cross-origin browser requests.
		if origin := r.Header.Get("Origin"); origin != "" {
			if !sameOrigin(origin, r) {
				http.Error(w, "cross-origin request rejected", http.StatusForbidden)
				return
			}
		} else if ref := r.Header.Get("Referer"); ref != "" {
			if !sameOrigin(ref, r) {
				http.Error(w, "cross-origin request rejected", http.StatusForbidden)
				return
			}
		}
		// Require JSON content type for JSON POST endpoints.
		ct := r.Header.Get("Content-Type")
		if ct == "" || !strings.HasPrefix(ct, "application/json") {
			http.Error(w, "content-type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		next(w, r)
	}
}

// sameOrigin reports whether an Origin or Referer URL points at the same
// host:port as the request. Origin "null" (sandboxed iframes, file://) is
// always rejected.
func sameOrigin(ref string, r *http.Request) bool {
	if ref == "" || ref == "null" {
		return false
	}
	u, err := url.Parse(ref)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	select {
	case <-s.reapStop:
		// already stopped
	default:
		close(s.reapStop)
	}
	if s.httpSrv != nil {
		return s.httpSrv.Shutdown(ctx)
	}
	return nil
}

// reapSessions periodically removes idle sessions until the server is shut
// down, bounding memory per requirement 046 ("sessions are GC'd after an idle
// timeout").
func (s *Server) reapSessions(timeout time.Duration) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-s.reapStop:
			return
		case <-t.C:
			s.sessions.Reap(timeout)
		}
	}
}

// route dispatches the root path: serve the SPA at "/", JSON otherwise.
func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/", "/index.html":
		s.handleIndex(w, r)
	default:
		http.NotFound(w, r)
	}
}

// dispatchCommand handles slash commands, mirroring handleInteractiveCommand in
// main.go. It returns a commandResponse with formatted output and fresh state.
func (s *Server) dispatchCommand(sess *Session, command string) commandResponse {
	fullCommand := strings.TrimPrefix(command, "/")
	parts := strings.SplitN(fullCommand, " ", 2)
	cmd := parts[0]
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}

	var output string
	switch cmd {
	case "stats":
		output = formatStats(sess.agent.GetStats())
	case "clear":
		output = "Output cleared."
	case "clear-history":
		sess.clearHistory()
		output = "History cleared."
	case "read-only":
		on := arg != "off"
		sess.agent.GetToolExecutor().SetReadOnly(on)
		if on {
			output = "[Read-only mode enabled: write operations disabled]"
		} else {
			output = "[Read-only mode disabled: write operations enabled]"
		}
	case "compress":
		timeout := time.Duration(sess.cfg.ReadTimeout) * time.Second
		if timeout == 0 {
			timeout = 24 * 60 * 60 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := sess.agent.CompressContext(ctx); err != nil {
			output = "[Compression failed: " + err.Error() + "]"
		} else {
			output = "[Context compressed successfully]"
		}
	case "goal":
		if arg == "" {
			output = "Usage: /goal <your goal here>"
		} else {
			sess.agent.SetGoal(arg)
			output = "[Goal mode activated: " + arg + "]"
		}
	case "goal-off":
		sess.agent.ClearGoal()
		output = "[Goal mode deactivated]"
	case "dump":
		path, err := sess.agent.DumpContext()
		if err != nil {
			output = "[Dump failed: " + err.Error() + "]"
		} else {
			output = "[Context dumped to: " + path + "]"
		}
	default:
		output = "Unknown command: /" + cmd + "\nAvailable commands: /stats, /clear, /clear-history, /read-only, /compress, /dump, /goal, /goal-off"
	}

	// Publish the updated state to subscribers.
	sess.broadcastEvent("state", sess.state())
	return commandResponse{OK: true, Output: output, State: sess.state()}
}

// formatStats renders runtime statistics as a plain-text block.
func formatStats(stats *agent.Stats) string {
	if stats == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Runtime Statistics\n")
	fmt.Fprintf(&sb, "  Input Tokens:      %d\n", stats.InputTokens)
	fmt.Fprintf(&sb, "  Output Tokens:     %d\n", stats.OutputTokens)
	fmt.Fprintf(&sb, "  Tokens/Second:     %.1f\n", stats.TokensPerSecond)
	fmt.Fprintf(&sb, "  Tool Calls:        %d\n", stats.ToolCalls)
	fmt.Fprintf(&sb, "  Failed Calls:      %d\n", stats.FailedToolCalls)
	fmt.Fprintf(&sb, "  Iterations:        %d\n", stats.Iterations)
	if stats.CompressionCount > 0 {
		fmt.Fprintf(&sb, "  Compressions:      %d\n", stats.CompressionCount)
	}
	if !stats.StartTime.IsZero() {
		fmt.Fprintf(&sb, "  Uptime:            %s\n", time.Since(stats.StartTime).Round(time.Second))
	}
	return sb.String()
}
