// Package tools implements the tool execution system for the coding agent.
// This file contains the git_diff tool implementation.
package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// executeGitDiff shows the diff between two commits, branches, or the working tree with context support.
func (te *ToolExecutor) executeGitDiff(ctx context.Context, params map[string]interface{}) *ToolResult {
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

	reference1 := ""
	if c, ok := params["reference1"].(string); ok && c != "" {
		reference1 = c
	}
	// Also accept "commit1" for backwards compatibility
	if reference1 == "" {
		if c, ok := params["commit1"].(string); ok && c != "" {
			reference1 = c
		}
	}

	reference2 := ""
	if c, ok := params["reference2"].(string); ok && c != "" {
		reference2 = c
	}
	// Also accept "commit2" for backwards compatibility
	if reference2 == "" {
		if c, ok := params["commit2"].(string); ok && c != "" {
			reference2 = c
		}
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

	// Build git diff command
	args := []string{"diff"}

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
		case "stat-numstat", "numstat":
			args = append(args, "--numstat")
		case "color":
			args = append(args, "--color=always")
		case "stat-width", "stat-width=0":
			args = append(args, "--stat-width=0")
		case "summary":
			args = append(args, "--summary")
		case "compact-summary":
			args = append(args, "--compact-summary")
		case "ignore-space-at-eol", "ignore-space-at-eol=":
			args = append(args, "--ignore-space-at-eol")
		case "ignore-space-change":
			args = append(args, "-b")
		case "ignore-all-space":
			args = append(args, "-w")
		case "unified", "unified=":
			args = append(args, "--unified=3")
		case "raw":
			args = append(args, "--raw")
		case "r":
			args = append(args, "-C")
		case "M":
			args = append(args, "--find-copies")
		case "patience":
			args = append(args, "--patience")
		case "minimal":
			args = append(args, "--minimal")
		}
	}

	// Handle reference1 and reference2
	if reference1 != "" && reference2 != "" {
		// Diff between two commits/refs
		args = append(args, reference1, reference2)
	} else if reference1 != "" {
		// Diff between commit and working tree (or index)
		args = append(args, reference1)
	} else {
		// Default: diff working tree against index
		if reference2 != "" {
			// Diff index against a commit
			args = append(args, reference2)
		}
	}

	// Resolve path: if it's a git repo root, use it as cmd.Dir;
	// if it's a subdirectory within a repo, find the repo root and use -- <subpath>
	diffCmdDir := path
	diffSubpath := ""
	if path != "." {
		repoRootCmd := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "--show-toplevel")
		if repoRootOut, repoRootErr := repoRootCmd.Output(); repoRootErr == nil {
			repoRoot := strings.TrimSpace(string(repoRootOut))
			if repoRoot == path || repoRoot == "." {
				diffCmdDir = path
			} else {
				diffCmdDir = repoRoot
				relPath, relErr := filepath.Rel(repoRoot, path)
				if relErr == nil {
					diffSubpath = relPath
				}
			}
		}
	}

	// Add subpath to limit diff scope
	if diffSubpath != "" {
		args = append(args, "--", diffSubpath)
	}

	// Execute git diff with context for cancellation support
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = diffCmdDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if it was cancelled
		if ctx.Err() != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("git diff was cancelled: %v", ctx.Err()),
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
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git diff failed: %s", string(output)),
		}
	}

	resultStr := strings.TrimSpace(string(output))
	if resultStr == "" {
		resultStr = "No differences found."
	}

	// Truncate if excessively large (> 50KB)
	if len(resultStr) > 50000 {
		resultStr = resultStr[:50000] + "\n... [output truncated due to size]"
	}

	return &ToolResult{
		Success: true,
		Output:  resultStr,
		Extra: map[string]interface{}{
			"path":       path,
			"reference1": reference1,
			"reference2": reference2,
			"flags":      flags,
		},
	}
}
