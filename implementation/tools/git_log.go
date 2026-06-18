// Package tools implements the tool execution system for the coding agent.
// This file contains the git_log tool implementation.
package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// executeGitLog views the commit history of a git repository with context support.
func (te *ToolExecutor) executeGitLog(ctx context.Context, params map[string]interface{}) *ToolResult {
	// Parse parameters
	path := "."
	if p, ok := params["path"].(string); ok && p != "" {
		path = p
	}

	// Validate path exists and is accessible
	if _, err := os.Stat(path); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("path not found or not accessible: %s", path),
		}
	}

	reference := ""
	if ref, ok := params["reference"].(string); ok && ref != "" {
		reference = ref
	}

	count := 10
	if c, ok := params["count"].(float64); ok && c > 0 {
		count = int(c)
	}
	if count > 1000 {
		count = 1000
	}

	// Parse flags
	flags := []string{}
	if flagsParam, ok := params["flags"]; ok {
		switch v := flagsParam.(type) {
		case []interface{}:
			for _, f := range v {
				if flagStr, ok := f.(string); ok {
					flags = append(flags, flagStr)
				}
			}
		case []string:
			flags = append(flags, v...)
		}
	}

	// Build git log command
	args := []string{"log", fmt.Sprintf("--max-count=%d", count)}

	// Validate conflicting format flags
	if hasFlag(flags, "oneline") {
		if hasFlag(flags, "stat") {
			return &ToolResult{
				Success: false,
				Error:   "conflicting flags: --oneline cannot be used with --stat",
			}
		}
		if hasFlag(flags, "patch") {
			return &ToolResult{
				Success: false,
				Error:   "conflicting flags: --oneline cannot be used with --patch",
			}
		}
		if hasFlag(flags, "shortstat") {
			return &ToolResult{
				Success: false,
				Error:   "conflicting flags: --oneline cannot be used with --shortstat",
			}
		}
	}

	// Add format flags based on options (must come before reference and path)
	for _, flag := range flags {
		switch flag {
		case "s":
			args = append(args, "--no-patch")
		case "m":
			args = append(args, "--merges")
		case "no-merges":
			args = append(args, "--no-merges")
		case "stat":
			args = append(args, "--stat")
		case "patch":
			args = append(args, "-p")
		case "oneline":
			args = append(args, "--oneline")
		case "shortstat":
			args = append(args, "--shortstat")
		case "follow":
			args = append(args, "--follow")
		case "grep":
			// Use dedicated grep parameter for searching commit messages
			grepParam := ""
			if gp, ok := params["grep"].(string); ok && gp != "" {
				grepParam = gp
			} else if reference != "" {
				// Fall back to reference for backwards compatibility
				grepParam = reference
			}
			if grepParam != "" {
				args = append(args, "--grep="+grepParam)
			}
		case "decorate":
			args = append(args, "--decorate")
		case "graph":
			args = append(args, "--graph")
		case "first-parent":
			args = append(args, "--first-parent")
		}
	}

	// Add reference if specified
	if reference != "" {
		args = append(args, reference)
	}

	// Resolve path: if it's a git repo root, use it as cmd.Dir;
	// if it's a subdirectory within a repo, find the repo root and use -- <subpath>
	cmdDir := path
	subpath := ""
	if path != "." {
		// Check if path is itself a git repo root
		repoRootCmd := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "--show-toplevel")
		if repoRootOut, repoRootErr := repoRootCmd.Output(); repoRootErr == nil {
			// path is a git repo root (or . itself)
			repoRoot := strings.TrimSpace(string(repoRootOut))
			if repoRoot == path || repoRoot == "." {
				cmdDir = path
			} else {
				// path is a subdirectory within a git repo
				cmdDir = repoRoot
				relPath, relErr := filepath.Rel(repoRoot, path)
				if relErr == nil {
					subpath = relPath
				}
			}
		}
	}

	// Add subpath to limit log scope
	if subpath != "" {
		args = append(args, "--", subpath)
	}

	// Execute git log with context for cancellation support
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = cmdDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if it was cancelled
		if ctx.Err() != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("git log was cancelled: %v", ctx.Err()),
			}
		}
		// Check if it's a git repository
		gitCmd := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "--show-toplevel")
		if _, err2 := gitCmd.CombinedOutput(); err2 != nil {
			return &ToolResult{
				Success: false,
				Error:   "not a git repository",
			}
		}
		// Check if the error is "no commits yet"
		if strings.Contains(string(output), "does not have any commits yet") {
			return &ToolResult{
				Success: true,
				Output:  "No commits found.",
				Extra: map[string]interface{}{
					"path":      path,
					"count":     count,
					"reference": reference,
					"flags":     flags,
				},
			}
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git log failed: %s", string(output)),
		}
	}

	resultStr := strings.TrimSpace(string(output))
	if resultStr == "" {
		resultStr = "No commits found."
	}

	// Truncate if excessively large (> 50KB)
	if len(resultStr) > 50000 {
		resultStr = resultStr[:50000] + "\n... [output truncated due to size]"
	}

	return &ToolResult{
		Success: true,
		Output:  resultStr,
		Extra: map[string]interface{}{
			"path":      path,
			"count":     count,
			"reference": reference,
			"flags":     flags,
		},
	}
}
