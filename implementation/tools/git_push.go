package tools

import (
	"fmt"
	"os/exec"
)

// executeGitPush handles git push operations.
func (te *ToolExecutor) executeGitPush(params map[string]interface{}) *ToolResult {
	action, ok := params["action"].(string)
	if !ok || action == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: action (use 'push', 'push_remote', 'force_push', 'set_upstream', or 'push_tags')",
		}
	}

	switch action {
	case "push":
		return te.gitPushPush(params)
	case "push_remote":
		return te.gitPushPushRemote(params)
	case "force_push":
		return te.gitPushForcePush(params)
	case "set_upstream":
		return te.gitPushSetUpstream(params)
	case "push_tags":
		return te.gitPushPushTags(params)
	default:
		return &ToolResult{
			Success: false,
			Error: fmt.Sprintf("unknown action: %s. Valid actions: 'push', 'push_remote', 'force_push', 'set_upstream', 'push_tags'", action),
		}
	}
}

// getCurrentBranchName is a helper that resolves the branch name from params or the current branch.
// Returns error if neither branch param nor current branch is available.
func (te *ToolExecutor) getCurrentBranchName(params map[string]interface{}, paramName string) (string, *ToolResult) {
	branch, hasBranch := params[paramName].(string)
	if hasBranch && branch != "" {
		return branch, nil
	}
	b, err := te.getCurrentBranch()
	if err != nil {
		return "", &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to get current branch: %v", err),
		}
	}
	return b, nil
}

// gitPushPush pushes the current branch to its upstream remote.
func (te *ToolExecutor) gitPushPush(params map[string]interface{}) *ToolResult {
	// Determine remote (default: origin)
	remote, _ := params["remote"].(string)
	if remote == "" {
		remote = "origin"
	}

	// Determine branch (default: current branch)
	branch, result := te.getCurrentBranchName(params, "branch")
	if result != nil {
		return result
	}

	// Build push arguments
	args := []string{"push", remote, branch}

	// Optional: set upstream
	if setUpstream, ok := params["set_upstream"].(bool); ok && setUpstream {
		args = append(args, "--set-upstream")
	}

	// Optional: push all branches
	if all, ok := params["all"].(bool); ok && all {
		args = []string{"push", remote, "--all"}
	}

	// Optional: push tags
	if tags, ok := params["tags"].(bool); ok && tags {
		args = append(args, "--tags")
	}

	// Optional: delete remote branch
	if delete, ok := params["delete"].(bool); ok && delete {
		args = append(args, "--delete", branch)
	}

	// Optional: dry-run
	if dryRun, ok := params["dry_run"].(bool); ok && dryRun {
		args = append(args, "--dry-run")
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  outputStr,
			Error:   fmt.Sprintf("git push failed: %s\nOutput: %s", err, outputStr),
			Extra: map[string]interface{}{
				"tool":   "git_push",
				"action": "push",
				"remote": remote,
				"branch": branch,
			},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  outputStr,
		Extra: map[string]interface{}{
			"tool":   "git_push",
			"action": "push",
			"remote": remote,
			"branch": branch,
		},
	}
}

// gitPushPushRemote pushes a specific branch to a specific remote.
func (te *ToolExecutor) gitPushPushRemote(params map[string]interface{}) *ToolResult {
	// Required: remote and branch
	remote, ok := params["remote"].(string)
	if !ok || remote == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: remote (e.g., 'origin')",
		}
	}

	sourceBranch, ok := params["source_branch"].(string)
	if !ok || sourceBranch == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: source_branch",
		}
	}

	// Optional: target branch on remote (defaults to same as source)
	targetBranch, _ := params["target_branch"].(string)
	if targetBranch == "" {
		targetBranch = sourceBranch
	}

	// Build push arguments
	args := []string{"push", remote, sourceBranch + ":" + targetBranch}

	// Optional: force push
	if force, ok := params["force"].(bool); ok && force {
		args = append(args, "--force")
	}

	// Optional: set upstream
	if setUpstream, ok := params["set_upstream"].(bool); ok && setUpstream {
		args = append(args, "--set-upstream")
	}

	// Optional: dry-run
	if dryRun, ok := params["dry_run"].(bool); ok && dryRun {
		args = append(args, "--dry-run")
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  outputStr,
			Error:   fmt.Sprintf("git push failed: %s\nOutput: %s", err, outputStr),
			Extra: map[string]interface{}{
				"tool":          "git_push",
				"action":        "push_remote",
				"remote":        remote,
				"source_branch": sourceBranch,
				"target_branch": targetBranch,
			},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  outputStr,
		Extra: map[string]interface{}{
			"tool":          "git_push",
			"action":        "push_remote",
			"remote":        remote,
			"source_branch": sourceBranch,
			"target_branch": targetBranch,
		},
	}
}

// gitPushForcePush performs a force push of the current or specified branch.
func (te *ToolExecutor) gitPushForcePush(params map[string]interface{}) *ToolResult {
	// Determine remote (default: origin)
	remote, _ := params["remote"].(string)
	if remote == "" {
		remote = "origin"
	}

	// Determine branch (default: current branch)
	branch, result := te.getCurrentBranchName(params, "branch")
	if result != nil {
		return result
	}

	// Determine force mode: hard force (--force-with-lease) is safer than --force
	forceHard, _ := params["force_hard"].(bool)

	// Build push arguments
	args := []string{"push", remote, branch}

	if forceHard {
		args = append(args, "--force-with-lease")
	} else {
		args = append(args, "--force")
	}

	// Optional: delete remote branch
	if delete, ok := params["delete"].(bool); ok && delete {
		args = append(args, "--delete", branch)
	}

	// Optional: dry-run
	if dryRun, ok := params["dry_run"].(bool); ok && dryRun {
		args = append(args, "--dry-run")
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  outputStr,
			Error:   fmt.Sprintf("git force push failed: %s\nOutput: %s", err, outputStr),
			Extra: map[string]interface{}{
				"tool":      "git_push",
				"action":    "force_push",
				"remote":    remote,
				"branch":    branch,
				"forceMode": "hard",
			},
		}
	}

	forceMode := "hard (--force-with-lease)"
	if !forceHard {
		forceMode = "soft (--force)"
	}

	return &ToolResult{
		Success: true,
		Output:  outputStr,
		Extra: map[string]interface{}{
			"tool":      "git_push",
			"action":    "force_push",
			"remote":    remote,
			"branch":    branch,
			"forceMode": forceMode,
		},
	}
}

// gitPushSetUpstream sets upstream tracking for the current branch.
func (te *ToolExecutor) gitPushSetUpstream(params map[string]interface{}) *ToolResult {
	// Determine remote (default: origin)
	remote, _ := params["remote"].(string)
	if remote == "" {
		remote = "origin"
	}

	// Determine branch (default: current branch)
	branch, result := te.getCurrentBranchName(params, "branch")
	if result != nil {
		return result
	}

	// Set upstream
	args := []string{"push", "-u", remote, branch}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  outputStr,
			Error:   fmt.Sprintf("git set upstream failed: %s\nOutput: %s", err, outputStr),
			Extra: map[string]interface{}{
				"tool":   "git_push",
				"action": "set_upstream",
				"remote": remote,
				"branch": branch,
			},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  outputStr,
		Extra: map[string]interface{}{
			"tool":   "git_push",
			"action": "set_upstream",
			"remote": remote,
			"branch": branch,
		},
	}
}

// gitPushPushTags pushes tags to the remote.
func (te *ToolExecutor) gitPushPushTags(params map[string]interface{}) *ToolResult {
	// Determine remote (default: origin)
	remote, _ := params["remote"].(string)
	if remote == "" {
		remote = "origin"
	}

	// Build push arguments
	args := []string{"push", remote, "--tags"}

	// Optional: delete remote tags that don't exist locally
	if delete, ok := params["delete"].(bool); ok && delete {
		args = append(args, "--delete")
	}

	// Optional: push specific tag
	if tag, ok := params["tag"].(string); ok && tag != "" {
		args = []string{"push", remote, tag}
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  outputStr,
			Error:   fmt.Sprintf("git push tags failed: %s\nOutput: %s", err, outputStr),
			Extra: map[string]interface{}{
				"tool":   "git_push",
				"action": "push_tags",
				"remote": remote,
			},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  outputStr,
		Extra: map[string]interface{}{
			"tool":   "git_push",
			"action": "push_tags",
			"remote": remote,
		},
	}
}
