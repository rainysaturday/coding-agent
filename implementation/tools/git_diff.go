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

// gitDiffParams holds the parsed parameters for a git_diff operation.
type gitDiffParams struct {
	path       string
	reference1 string
	reference2 string
	flags      []string
}

// parseGitDiffParams extracts and validates git_diff parameters from the tool params map.
func parseGitDiffParams(params map[string]interface{}) *gitDiffParams {
	path := "."
	if p, ok := params["path"].(string); ok && p != "" {
		path = p
	}

	reference1 := ""
	if c, ok := params["reference1"].(string); ok && c != "" {
		reference1 = c
	}
	if reference1 == "" {
		if c, ok := params["commit1"].(string); ok && c != "" {
			reference1 = c
		}
	}

	reference2 := ""
	if c, ok := params["reference2"].(string); ok && c != "" {
		reference2 = c
	}
	if reference2 == "" {
		if c, ok := params["commit2"].(string); ok && c != "" {
			reference2 = c
		}
	}

	flags := parseFlagsParamToSlice(params)

	return &gitDiffParams{path: path, reference1: reference1, reference2: reference2, flags: flags}
}

// gitDiffAllowedFlags lists the flags buildGitDiffArgs understands; it backs
// the "unsupported flag" error so a mistyped flag is reported instead of
// silently dropped.
var gitDiffAllowedFlags = []string{
	"M", "color", "compact-summary", "ignore-all-space",
	"ignore-space-at-eol", "ignore-space-change", "minimal", "name-only",
	"name-status", "numstat", "p", "patch", "patience", "r", "raw",
	"shortstat", "stat", "stat-numstat", "stat-width", "summary", "unified",
}

// buildGitDiffArgs builds the git diff command arguments. Leading dashes on
// flags are optional, value-taking flags accept "name=value", and unrecognised
// flags are reported as an error.
func buildGitDiffArgs(gp *gitDiffParams) ([]string, error) {
	args := []string{"diff"}

	var unknown []string
	for _, rawFlag := range gp.flags {
		flag, flagValue := splitGitFlag(normalizeGitFlag(rawFlag))
		if flag == "" {
			continue
		}
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
		case "stat-width":
			width, err := parseFlagInt(flag, flagValue, 0)
			if err != nil {
				return nil, err
			}
			args = append(args, fmt.Sprintf("--stat-width=%d", width))
		case "summary":
			args = append(args, "--summary")
		case "compact-summary":
			args = append(args, "--compact-summary")
		case "ignore-space-at-eol":
			args = append(args, "--ignore-space-at-eol")
		case "ignore-space-change":
			args = append(args, "-b")
		case "ignore-all-space":
			args = append(args, "-w")
		case "unified":
			// Honour the requested width instead of always emitting 3, which
			// made "unified=10" look like it worked while it did nothing.
			lines, err := parseFlagInt(flag, flagValue, 3)
			if err != nil {
				return nil, err
			}
			args = append(args, fmt.Sprintf("--unified=%d", lines))
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
		default:
			unknown = append(unknown, rawFlag)
		}
	}

	if len(unknown) > 0 {
		return nil, unknownGitFlagsError("git_diff", unknown, gitDiffAllowedFlags)
	}

	if gp.reference1 != "" && gp.reference2 != "" {
		args = append(args, gp.reference1, gp.reference2)
	} else if gp.reference1 != "" {
		args = append(args, gp.reference1)
	} else if gp.reference2 != "" {
		args = append(args, gp.reference2)
	}

	return args, nil
}

// executeGitDiff shows the diff between two commits, branches, or the working tree with context support.
func (te *ToolExecutor) executeGitDiff(ctx context.Context, params map[string]interface{}) *ToolResult {
	gp := parseGitDiffParams(params)

	// Validate path exists and is accessible
	if _, err := os.Stat(gp.path); err != nil {
		return &ToolResult{Success: false, Error: fmt.Sprintf("path not found or not accessible: %s", gp.path)}
	}

	args, err := buildGitDiffArgs(gp)
	if err != nil {
		return &ToolResult{Success: false, Error: err.Error()}
	}

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

	// Execute git diff with context for cancellation support
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = cmdDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		if ctx.Err() != nil {
			return &ToolResult{Success: false, Error: fmt.Sprintf("git diff was cancelled: %v", ctx.Err())}
		}
		gitCmd := exec.CommandContext(ctx, "git", "-C", gp.path, "rev-parse", "--show-toplevel")
		if _, err2 := gitCmd.CombinedOutput(); err2 != nil {
			return &ToolResult{Success: false, Error: "not a git repository"}
		}
		return &ToolResult{Success: false, Error: fmt.Sprintf("git diff failed: %s", string(output))}
	}

	resultStr := strings.TrimSpace(string(output))
	if resultStr == "" {
		resultStr = "No differences found."
	}

	if len(resultStr) > 50000 {
		resultStr = TruncateBytesAtRuneBoundary(resultStr, 50000, "\n... [output truncated due to size]")
	}

	return &ToolResult{
		Success: true,
		Output:  resultStr,
		Extra:   map[string]interface{}{"path": gp.path, "reference1": gp.reference1, "reference2": gp.reference2, "flags": gp.flags},
	}
}
