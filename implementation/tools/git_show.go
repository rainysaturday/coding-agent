// Package tools implements the tool execution system for the coding agent.
// This file contains the git_show tool implementation.
package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// executeGitShow shows details of a specific commit with context support.
func (te *ToolExecutor) executeGitShow(ctx context.Context, params map[string]interface{}) *ToolResult {
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

	commit := "HEAD"
	if c, ok := params["commit"].(string); ok && c != "" {
		commit = c
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

	// Build git show command
	args := []string{"show", commit}

	// Add format flags based on options
	for _, flag := range flags {
		switch flag {
		case "stat":
			args = append(args, "--stat")
		case "patch", "p":
			args = append(args, "-p")
		case "name-status":
			args = append(args, "--name-status")
		case "name-only":
			args = append(args, "--name-only")
		case "shortstat":
			args = append(args, "--shortstat")
		case "stat-numstat":
			args = append(args, "--numstat")
		case "oneline":
			args = append(args, "--oneline")
		case "s":
			args = append(args, "--oneline", "--no-patch")
		case "no-patch":
			args = append(args, "--no-patch")
		case "summary":
			args = append(args, "--summary")
		case "r":
			args = append(args, "-C")
		case "M":
			args = append(args, "--find-copies")
		}
	}

	// Resolve path: if it's a git repo root, use it as cmd.Dir;
	// if it's a subdirectory within a repo, find the repo root and use -- <subpath>
	showCmdDir := path
	showSubpath := ""
	if path != "." {
		// Check if path is itself a git repo root
		repoRootCmd := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "--show-toplevel")
		if repoRootOut, repoRootErr := repoRootCmd.Output(); repoRootErr == nil {
			repoRoot := strings.TrimSpace(string(repoRootOut))
			if repoRoot == path || repoRoot == "." {
				showCmdDir = path
			} else {
				showCmdDir = repoRoot
				relPath, relErr := filepath.Rel(repoRoot, path)
				if relErr == nil {
					showSubpath = relPath
				}
			}
		}
	}

	// Add subpath to limit show scope
	if showSubpath != "" {
		args = append(args, "--", showSubpath)
	}

	// Execute git show with context for cancellation support
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = showCmdDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if it was cancelled
		if ctx.Err() != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("git show was cancelled: %v", ctx.Err()),
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
					"path":   path,
					"commit": commit,
					"flags":  flags,
				},
			}
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git show failed: %s", string(output)),
		}
	}

	resultStr := strings.TrimSpace(string(output))
	if resultStr == "" {
		resultStr = "No information available for the specified commit."
	}

	// Truncate if excessively large (> 50KB)
	if len(resultStr) > 50000 {
		resultStr = resultStr[:50000] + "\n... [output truncated due to size]"
	}

	return &ToolResult{
		Success: true,
		Output:  resultStr,
		Extra: map[string]interface{}{
			"commitReference": commit,
		},
	}
}
