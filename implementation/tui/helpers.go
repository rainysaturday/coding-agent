package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/coding-agent/coding-agent/agent"
)

// helpText lists the supported slash commands (requirement 002/044).
const helpText = `Supported commands:
  /stats                 Show runtime statistics
  /clear                 Clear the output display
  /clear-history         Clear the input history
  /compress              Compress the conversation context now
  /goal <prompt>         Activate goal mode and start working on the prompt
  /goal-off              Deactivate goal mode
  /dump                  Dump the conversation context to a file
  /read-only             Toggle read-only mode
  /skills                List discovered skills
  /skill <name>          Activate a skill by name
  /version               Show version information
  /help                  Show this help
  /exit, /quit           Exit (context is dumped unless --no-dump-on-exit)`

// comma formats an integer with thousands separators.
func comma(n int) string {
	s := fmt.Sprintf("%d", n)
	if n < 0 {
		s = s[1:]
	}
	var out []string
	for len(s) > 3 {
		out = append([]string{s[len(s)-3:]}, out...)
		s = s[:len(s)-3]
	}
	out = append([]string{s}, out...)
	res := strings.Join(out, ",")
	if n < 0 {
		res = "-" + res
	}
	return res
}

// isTerminal reports whether f is attached to a terminal.
func isTerminal(f *os.File) bool {
	_, _, err := ioctlWinsize(f.Fd())
	return err == nil
}

// since guards against a zero time.
func since(t time.Time) time.Duration {
	if t.IsZero() {
		return 0
	}
	return time.Since(t)
}

// fmtStats renders the statistics block including the current context size
// (requirement 021).
func fmtStats(a *agent.Agent) string {
	snap := a.Stats().Snapshot()
	var b strings.Builder
	fmt.Fprintf(&b, "Input Tokens:      %s\n", comma(snap.InputTokens))
	fmt.Fprintf(&b, "Output Tokens:     %s\n", comma(snap.OutputTokens))
	if snap.CachedTokens > 0 {
		fmt.Fprintf(&b, "Cached Tokens:     %s\n", comma(snap.CachedTokens))
	}
	fmt.Fprintf(&b, "Current Context:   %s / %s (%.1f%%)\n", comma(a.ContextTokens()), comma(a.ContextLimit()), a.ContextPercent())
	fmt.Fprintf(&b, "Tokens/Second:     %.2f\n", a.Stats().TokensPerSecond())
	fmt.Fprintf(&b, "Tool Calls:        %d\n", snap.ToolCalls)
	fmt.Fprintf(&b, "Failed Calls:      %d\n", snap.FailedToolCalls)
	fmt.Fprintf(&b, "Iterations:        %d\n", snap.Iterations)
	fmt.Fprintf(&b, "Compressions:      %d\n", snap.CompressionCount)
	fmt.Fprintf(&b, "Inference Time:    %s\n", agent.FormatDuration(snap.InferenceTime))
	fmt.Fprintf(&b, "Tool Time:         %s\n", agent.FormatDuration(snap.ToolTime))
	fmt.Fprintf(&b, "Uptime:            %s", agent.FormatDuration(time.Since(snap.StartTime)))
	return b.String()
}
