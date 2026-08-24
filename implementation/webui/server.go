package webui

import (
	"context"
	"fmt"
	"net/http"
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
}

// NewServer creates a web UI server from a config.
func NewServer(cfg *config.Config) *Server {
	return &Server{
		cfg:      cfg,
		sessions: NewSessionManager(cfg),
		baseCtx:  context.Background(),
	}
}

// Serve starts the HTTP server and blocks until the server shuts down.
// It returns the server so callers can perform graceful shutdown.
func (s *Server) Serve() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.WebAddr, s.cfg.WebPort)

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.route)
	mux.HandleFunc("/assets/", s.handleAsset)
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/command", s.handleCommand)
	mux.HandleFunc("/api/cancel", s.handleCancel)

	s.httpSrv = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	fmt.Printf("%s============================================================%s\n", colors.GetColor("blue"), colors.GetColor("reset"))
	fmt.Printf("%s  Coding Agent Web UI%s\n", colors.GetColor("blue"), colors.GetColor("reset"))
	fmt.Printf("%s============================================================%s\n", colors.GetColor("blue"), colors.GetColor("reset"))
	fmt.Printf("  %sWeb UI running at: %shttp://%s%s\n", colors.GetColor("cyan"), colors.GetColor("reset"), addr, colors.GetColor("reset"))
	fmt.Printf("  %sPress Ctrl+C to stop.%s\n", colors.GetColor("dim"), colors.GetColor("reset"))
	fmt.Println()

	return s.httpSrv.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpSrv != nil {
		return s.httpSrv.Shutdown(ctx)
	}
	return nil
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
		sess.agent.GetToolExecutor().SetReadOnly(true)
		output = "[Read-only mode enabled: write operations disabled]"
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
