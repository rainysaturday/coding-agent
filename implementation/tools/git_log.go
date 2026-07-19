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

// gitLogParams holds the parsed parameters for a git_log operation.
type gitLogParams struct {
	path      string
	reference string
	count     int
	flags     []string
	grep      string
}

// parseGitLogParams extracts and validates git_log parameters from the tool params map.
func parseGitLogParams(params map[string]interface{}) *gitLogParams {
	path := "."
	if p, ok := params["path"].(string); ok && p != "" {
		path = p
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

	flags := parseFlagsParamToSlice(params)

	grep := ""
	if gp, ok := params["grep"].(string); ok && gp != "" {
		grep = gp
	}

	return &gitLogParams{path: path, reference: reference, count: count, flags: flags, grep: grep}
}

// buildGitLogArgs builds the git log command arguments.
func buildGitLogArgs(gp *gitLogParams) []string {
	args := []string{"log", fmt.Sprintf("--max-count=%d", gp.count)}

	for _, flag := range gp.flags {
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
			if gp.grep != "" {
				args = append(args, "--grep="+gp.grep)
			}
		case "decorate":
			args = append(args, "--decorate")
		case "graph":
			args = append(args, "--graph")
		case "first-parent":
			args = append(args, "--first-parent")
		}
	}

	if gp.reference != "" {
		args = append(args, gp.reference)
	}

	return args
}

// executeGitLog views the commit history of a git repository with context support.
func (te *ToolExecutor) executeGitLog(ctx context.Context, params map[string]interface{}) *ToolResult {
	gp := parseGitLogParams(params)

	// Validate path exists and is accessible
	if _, err := os.Stat(gp.path); err != nil {
		return &ToolResult{Success: false, Error: fmt.Sprintf("path not found or not accessible: %s", gp.path)}
	}

	// Validate conflicting format flags
	if hasFlag(gp.flags, "oneline") {
		if hasFlag(gp.flags, "stat") {
			return &ToolResult{Success: false, Error: "conflicting flags: --oneline cannot be used with --stat"}
		}
		if hasFlag(gp.flags, "patch") {
			return &ToolResult{Success: false, Error: "conflicting flags: --oneline cannot be used with --patch"}
		}
		if hasFlag(gp.flags, "shortstat") {
			return &ToolResult{Success: false, Error: "conflicting flags: --oneline cannot be used with --shortstat"}
		}
	}

	args := buildGitLogArgs(gp)

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

	// Execute git log with context for cancellation support
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = cmdDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		if ctx.Err() != nil {
			return &ToolResult{Success: false, Error: fmt.Sprintf("git log was cancelled: %v", ctx.Err())}
		}
		gitCmd := exec.CommandContext(ctx, "git", "-C", gp.path, "rev-parse", "--show-toplevel")
		if _, err2 := gitCmd.CombinedOutput(); err2 != nil {
			return &ToolResult{Success: false, Error: "not a git repository"}
		}
		if strings.Contains(string(output), "does not have any commits yet") {
			return &ToolResult{
				Success: true,
				Output:  "No commits found.",
				Extra:   map[string]interface{}{"path": gp.path, "count": gp.count, "reference": gp.reference, "flags": gp.flags},
			}
		}
		return &ToolResult{Success: false, Error: fmt.Sprintf("git log failed: %s", string(output))}
	}

	resultStr := strings.TrimSpace(string(output))
	if resultStr == "" {
		resultStr = "No commits found."
	}

	if len(resultStr) > 50000 {
		resultStr = resultStr[:50000] + "\n... [output truncated due to size]"
	}

	return &ToolResult{
		Success: true,
		Output:  resultStr,
		Extra:   map[string]interface{}{"path": gp.path, "count": gp.count, "reference": gp.reference, "flags": gp.flags},
	}
}
