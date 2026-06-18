// Package tools implements the tool execution system for the coding agent.
// This file contains the bash tool implementation.
package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"time"
)

// Default timeout for bash commands in milliseconds
const defaultBashTimeoutMs = 30000

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
		output, err := cmd.CombinedOutput()
		resultChan <- cmdResult{output: output, err: err}
	}()

	// Wait for either completion, timeout, or cancellation
	select {
	case <-ctx.Done():
		// Timeout or cancellation occurred
		if ctx.Err() == context.DeadlineExceeded {
			return &ToolResult{
				Success:  false,
				ExitCode: 124, // Convention: 124 for timeout (like GNU timeout)
				Error:    fmt.Sprintf("command timed out after %dms (timeout exceeded). The command did not complete within the specified timeout period. Consider increasing the timeout parameter (in milliseconds) if the command needs more time, or optimizing the command to run faster.", timeoutMs),
			}
		}
		if isCancelled(ctx.Err()) {
			return &ToolResult{
				Success:  false,
				ExitCode: 130, // Convention: 130 for SIGINT (like bash)
				Error:    "command was cancelled by the user",
			}
		}
		return &ToolResult{
			Success:  false,
			ExitCode: 1,
			Error:    fmt.Sprintf("command failed with context error: %v", ctx.Err()),
		}
	case res := <-resultChan:
		// Extract exit code
		exitCode := 0
		if res.err != nil {
			if exitError, ok := res.err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			}
		}

		result := &ToolResult{
			ExitCode: exitCode,
		}

		if res.err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("command failed: %v\nOutput: %s", res.err, string(res.output))
		} else {
			result.Success = true
			result.Output = string(res.output)
		}

		return result
	}
}
