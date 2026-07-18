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

// gitShowParams holds the parsed parameters for a git_show operation.
type gitShowParams struct {
	path   string
	commit string
	flags  []string
}

// parseGitShowParams extracts and validates git_show parameters from the tool params map.
func parseGitShowParams(params map[string]interface{}) *gitShowParams {
	path := "."
	if p, ok := params["path"].(string); ok && p != "" {
		path = p
	}

	commit := "HEAD"
	if c, ok := params["commit"].(string); ok && c != "" {
		commit = c
	}

	flags := parseFlagsParamToSlice(params)

	return &gitShowParams{path: path, commit: commit, flags: flags}
}

// buildGitShowArgs builds the git show command arguments.
func buildGitShowArgs(gp *gitShowParams) []string {
	args := []string{"show", gp.commit}

	for _, flag := range gp.flags {
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

	return args
}

// executeGitShow shows details of a specific commit with context support.
func (te *ToolExecutor) executeGitShow(ctx context.Context, params map[string]interface{}) *ToolResult {
	gp := parseGitShowParams(params)

	// Validate path exists and is accessible
	if _, err := os.Stat(gp.path); err != nil {
		return &ToolResult{Success: false, Error: fmt.Sprintf("path not found or not accessible: %s", gp.path)}
	}

	args := buildGitShowArgs(gp)

	// Resolve path: if it's a git repo root, use it as cmd.Dir;
	// if it's a subdirectory within a repo, find the repo root and use -- <subpath>
	cmdDir := gp.path
	subpath := ""
	if gp.path != "." {
		repoRootCmd := exec.CommandContext(ctx, "git", "-C", gp.path, "rev-parse", "--show-toplevel")
		if repoRootOut, repoRootErr := repoRootCmd.Output(); repoRootErr == nil {
			repoRoot := strings.TrimSpace(string(repoRootOut))
			if repoRoot == gp.path || repoRoot == "." {
				cmdDir = gp.path
			} else {
				cmdDir = repoRoot
				relPath, relErr := filepath.Rel(repoRoot, gp.path)
				if relErr == nil {
					subpath = relPath
				}
			}
		}
	}

	if subpath != "" {
		args = append(args, "--", subpath)
	}

	// Execute git show with context for cancellation support
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = cmdDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		if ctx.Err() != nil {
			return &ToolResult{Success: false, Error: fmt.Sprintf("git show was cancelled: %v", ctx.Err())}
		}
		gitCmd := exec.CommandContext(ctx, "git", "-C", gp.path, "rev-parse", "--show-toplevel")
		if _, err2 := gitCmd.CombinedOutput(); err2 != nil {
			return &ToolResult{Success: false, Error: "not a git repository"}
		}
		if strings.Contains(string(output), "does not have any commits yet") {
			return &ToolResult{
				Success: true,
				Output:  "No commits found.",
				Extra:   map[string]interface{}{"path": gp.path, "commit": gp.commit, "flags": gp.flags},
			}
		}
		return &ToolResult{Success: false, Error: fmt.Sprintf("git show failed: %s", string(output))}
	}

	resultStr := strings.TrimSpace(string(output))
	if resultStr == "" {
		resultStr = "No information available for the specified commit."
	}

	if len(resultStr) > 50000 {
		resultStr = resultStr[:50000] + "\n... [output truncated due to size]"
	}

	return &ToolResult{
		Success: true,
		Output:  resultStr,
		Extra:   map[string]interface{}{"commitReference": gp.commit},
	}
}
