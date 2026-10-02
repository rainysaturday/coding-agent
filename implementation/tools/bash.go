package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// BashTool executes shell commands in the working directory.
type BashTool struct {
	run *RunContext
}

// NewBashTool creates the bash tool.
func NewBashTool(run *RunContext) *BashTool { return &BashTool{run: run} }

func (t *BashTool) Name() string { return "bash" }

func (t *BashTool) Description() string {
	return "Execute a bash command in the terminal"
}

func (t *BashTool) Schema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"command":          prop("string", "The bash command to execute"),
		"timeout":          prop("integer", "Timeout in milliseconds for the command (default: 30000). Use this for long-running commands."),
		"max_output_lines": prop("integer", "Maximum number of lines of output to return (default: 200). Only the last N lines are shown. Set to 0 or a negative value to disable truncation and return the full output. Use a larger value if you need more output, or 0 to get everything."),
	}, []string{"command"})
}

func (t *BashTool) ReadOnly() bool { return false }

const bashMaxBytes = 4 << 20

func (t *BashTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	command := getString(args, "command")
	if strings.TrimSpace(command) == "" {
		return "", errors.New("missing required parameter: command")
	}
	timeoutMS := getInt(args, "timeout", t.run.BashTimeoutMS)
	if timeoutMS <= 0 {
		timeoutMS = t.run.BashTimeoutMS
	}
	maxLines := args["max_output_lines"]
	limit := t.run.MaxOutputLines
	if maxLines != nil {
		limit = getInt(args, "max_output_lines", t.run.MaxOutputLines)
	}

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()

	shell := "bash"
	if _, err := exec.LookPath(shell); err != nil {
		shell = "sh"
	}
	cmd := exec.CommandContext(runCtx, shell, "-c", command)
	cmd.Dir = t.run.CWD

	var stdout, stderr bytes.Buffer
	stdoutW := &limitedWriter{buf: &stdout, limit: bashMaxBytes}
	stderrW := &limitedWriter{buf: &stderr, limit: bashMaxBytes}
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW

	err := cmd.Run()
	timedOut := errors.Is(runCtx.Err(), context.DeadlineExceeded)

	out := truncateLines(stdout.String(), limit)
	errOut := truncateLines(stderr.String(), limit)

	var b strings.Builder
	b.WriteString(out)
	if stdoutW.dropped > 0 {
		b.WriteString(fmt.Sprintf("\n[stdout truncated after %d bytes]", bashMaxBytes))
	}
	if stderrW.dropped > 0 {
		b.WriteString(fmt.Sprintf("\n[stderr truncated after %d bytes]", bashMaxBytes))
	}
	if strings.TrimSpace(errOut) != "" {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("stderr:\n")
		b.WriteString(errOut)
	}
	if timedOut {
		// GNU timeout convention: exit code 124 (requirement 004).
		msg := fmt.Sprintf("command timed out after %dms (timeout exceeded). The command did not complete within the specified timeout period. Consider increasing the timeout parameter (exit status 124)", timeoutMS)
		if b.Len() > 0 {
			msg += ": partial output:\n" + b.String()
		}
		return "", errors.New(msg)
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Non-zero exit: report the exit code and any captured output so
			// the model can act on it (requirements 004 + 018).
			msg := fmt.Sprintf("exit status %d", exitErr.ExitCode())
			if b.Len() > 0 {
				msg += ": " + b.String()
			}
			return "", errors.New(msg)
		}
		return "", fmt.Errorf("failed to run command: %w", err)
	}
	if b.Len() == 0 {
		return "(no output)", nil
	}
	return b.String(), nil
}

// limitedWriter captures output up to a byte limit.
type limitedWriter struct {
	buf     *bytes.Buffer
	limit   int
	dropped int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	room := w.limit - w.buf.Len()
	if room <= 0 {
		w.dropped += len(p)
		return len(p), nil
	}
	if len(p) > room {
		w.buf.Write(p[:room])
		w.dropped += len(p) - room
	} else {
		w.buf.Write(p)
	}
	return len(p), nil
}

// truncateLines keeps the last n lines of s (n <= 0 disables truncation).
func truncateLines(s string, n int) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	if n <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	kept := lines[len(lines)-n:]
	return fmt.Sprintf("... [output truncated: showing last %d of %d lines]\n%s", n, len(lines), strings.Join(kept, "\n"))
}
