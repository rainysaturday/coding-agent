// Package debug implements the optional conversation debug log.
//
// When enabled the full exchange with the LLM (system prompt, user messages,
// assistant messages, tool calls and tool results) is appended to a log file.
// Sensitive values (API keys, tokens) are redacted before they are written.
package debug

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Logger writes redacted debug records to a file.
type Logger struct {
	mu      sync.Mutex
	path    string
	enabled bool
	file    *os.File
	started time.Time
}

// New creates a logger. When enabled is false the logger is a no-op.
func New(path string, enabled bool) *Logger {
	if path == "" {
		path = "debug.log"
	}
	l := &Logger{path: path, enabled: enabled, started: time.Now()}
	if enabled {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not open debug log %q: %v\n", path, err)
			l.enabled = false
			return l
		}
		l.file = f
		fmt.Fprintf(f, "\n==== coding-agent debug session %s ====\n", time.Now().UTC().Format(time.RFC3339))
	}
	return l
}

// Enabled reports whether debug logging is active.
func (l *Logger) Enabled() bool {
	if l == nil {
		return false
	}
	return l.enabled
}

// Path returns the debug log path.
func (l *Logger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Logf writes a formatted record.
func (l *Logger) Logf(format string, args ...interface{}) {
	if l == nil || !l.enabled {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	stamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	elapsed := time.Since(l.started).Round(time.Millisecond)
	if _, err := l.file.WriteString(fmt.Sprintf("[%s +%s] %s", stamp, elapsed, Redact(msg))); err != nil {
		l.enabled = false
		fmt.Fprintf(os.Stderr, "warning: debug log write failed: %v\n", err)
	}
}

// Section writes a labelled block of content.
func (l *Logger) Section(label, content string) {
	if l == nil || !l.enabled {
		return
	}
	l.Logf("----- %s -----\n%s\n----- end %s -----", label, strings.TrimRight(content, "\n"), label)
}

// Close flushes and closes the log file.
func (l *Logger) Close() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		l.file.Sync()
		l.file.Close()
		l.file = nil
	}
}

var (
	authRe    = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)([^\s",}]+)`)
	apiKeyRe  = regexp.MustCompile(`(?i)("(?:api_key|apikey|api-key|token|access_token|authorization)"\s*:\s*")([^"]+)(")`)
	bearerRe  = regexp.MustCompile(`(?i)(bearer\s+)([A-Za-z0-9_\-\.]{8,})`)
	ghTokenRe = regexp.MustCompile(`\b(gh[uopsr]_[A-Za-z0-9_]{8,}|github_pat_[A-Za-z0-9_]{8,}|sk-[A-Za-z0-9\-_]{8,})`)
	passRe    = regexp.MustCompile(`(?i)("(?:password|passwd|secret)"\s*:\s*")([^"]+)(")`)
)

// Redact removes API keys and tokens from a string.
func Redact(s string) string {
	s = authRe.ReplaceAllString(s, "$1[REDACTED]")
	s = apiKeyRe.ReplaceAllString(s, "$1[REDACTED]$3")
	s = bearerRe.ReplaceAllString(s, "$1[REDACTED]")
	s = ghTokenRe.ReplaceAllString(s, "[REDACTED]")
	s = passRe.ReplaceAllString(s, "$1[REDACTED]$3")
	return s
}

var (
	mu            sync.Mutex
	defaultLogger *Logger
)

// Init installs the global debug logger. An empty path enables console only
// logging.
func Init(path string, enabled bool) *Logger {
	mu.Lock()
	defer mu.Unlock()
	if !enabled {
		defaultLogger = nil
		return nil
	}
	defaultLogger = New(path, enabled)
	return defaultLogger
}

// Default returns the global logger (nil when debugging is disabled).
func Default() *Logger {
	mu.Lock()
	defer mu.Unlock()
	return defaultLogger
}

// Close closes the global logger.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if defaultLogger != nil {
		defaultLogger.Close()
		defaultLogger = nil
	}
}
