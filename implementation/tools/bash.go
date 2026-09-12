// Package tools implements the tool execution system for the coding agent.
// This file contains the bash tool implementation.
package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Default timeout for bash commands in milliseconds
const defaultBashTimeoutMs = 30000

// Default maximum number of lines of output returned by the bash tool before it
// is truncated. Keeping only the tail (last lines) is most useful because for
// bash commands the end of output typically contains the actual results.
const defaultBashMaxOutputLines = 200

// truncateBashOutput returns the last maxLines lines of the given output. If the
// output has more than maxLines lines, a truncation notice is prepended so the
// LLM knows some output was omitted. A maxLines value <= 0 disables truncation
// entirely and returns the output unchanged.
func truncateBashOutput(output string, maxLines int) string {
	if maxLines <= 0 {
		return output
	}
	// Split into lines, dropping a trailing empty element produced by a trailing
	// newline so we don't count it as content.
	lines := strings.Split(output, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= maxLines {
		return output
	}
	notice := fmt.Sprintf("... [output truncated: showing last %d of %d lines]\n", maxLines, len(lines))
	return notice + strings.Join(lines[len(lines)-maxLines:], "\n")
}

// executeBash executes a bash command with context support for cancellation.
func (te *ToolExecutor) executeBash(ctx context.Context, params map[string]interface{}) *ToolResult {
	command, ok := params["command"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: command",
		}
	}

	// Parse optional timeout parameter (in milliseconds), default to 30 seconds
	timeoutMs := defaultBashTimeoutMs
	if timeoutParam, hasTimeout := params["timeout"]; hasTimeout {
		switch v := timeoutParam.(type) {
		case float64:
			timeoutMs = int(v)
		case int:
			timeoutMs = v
		case string:
			if t, err := strconv.Atoi(v); err == nil && t > 0 {
				timeoutMs = t
			}
		}
	}

	// Ensure timeout is positive
	if timeoutMs <= 0 {
		timeoutMs = defaultBashTimeoutMs
	}

	// Parse optional max_output_lines parameter. Default to 200 lines. A value
	// <= 0 disables truncation entirely, returning the full output.
	maxOutputLines := parseIntParam(params, "max_output_lines", defaultBashMaxOutputLines)

	// Create a child context that respects both the parent cancellation and the timeout
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	// Channel to receive command result
	type cmdResult struct {
		output []byte
		err    error
	}
	resultChan := make(chan cmdResult, 1)

	go func() {
		cmd := exec.CommandContext(ctx, "bash", "-c", command)
		// Put the child in its own process group so cancellation kills the whole
		// group (including grandchildren) rather than just the bash shell.
		configureBashCommand(cmd)
		output, err := cmd.CombinedOutput()
		resultChan <- cmdResult{output: output, err: err}
	}()

	// Wait for either completion, timeout, or cancellation
	select {
	case <-ctx.Done():
		// Collect whatever partial output the killed process produced so the
		// timeout/cancel message can surface it instead of discarding it.
		var partial string
		select {
		case res := <-resultChan:
			partial = truncateBashOutput(string(res.output), maxOutputLines)
		case <-time.After(200 * time.Millisecond):
		}
		// Timeout or cancellation occurred
		if ctx.Err() == context.DeadlineExceeded {
			return &ToolResult{
				Success:  false,
				ExitCode: 124, // Convention: 124 for timeout (like GNU timeout)
				Error:    fmt.Sprintf("command timed out after %dms (timeout exceeded). The command did not complete within the specified timeout period. Consider increasing the timeout parameter (in milliseconds) if the command needs more time, or optimizing the command to run faster.\nPartial output:\n%s", timeoutMs, partial),
			}
		}
		if isCancelled(ctx.Err()) {
			return &ToolResult{
				Success:  false,
				ExitCode: 130, // Convention: 130 for SIGINT (like bash)
				Error:    fmt.Sprintf("command was cancelled by the user\nPartial output:\n%s", partial),
			}
		}
		return &ToolResult{
			Success:  false,
			ExitCode: 1,
			Error:    fmt.Sprintf("command failed with context error: %v", ctx.Err()),
		}
	case res := <-resultChan:
		// Extract exit code. If the process could not be started at all (e.g.
		// the shell binary was missing), the error is not an *exec.ExitError, so
		// fall back to the conventional 127 ("command not found") rather than
		// reporting a successful run with ExitCode 0 (I-13).
		exitCode := 0
		if res.err != nil {
			if exitError, ok := res.err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			} else {
				exitCode = 127
			}
		}

		// Apply output truncation (default: last 200 lines). max_output_lines
		// <= 0 disables truncation and returns the full output.
		output := truncateBashOutput(string(res.output), maxOutputLines)

		result := &ToolResult{
			ExitCode: exitCode,
		}

		if res.err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("command failed: %v\nOutput: %s", res.err, output)
		} else {
			result.Success = true
			result.Output = output
		}

		return result
	}
}
