package tools

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// executeGitCherryPick handles cherry-pick operations for git repositories.
func (te *ToolExecutor) executeGitCherryPick(params map[string]interface{}) *ToolResult {
	action, ok := params["action"].(string)
	if !ok || action == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: action (use 'cherry-pick', 'abort', 'list', or 'preview')",
		}
	}

	switch action {
	case "cherry-pick":
		return te.gitCherryPickCherryPick(params)
	case "abort":
		return te.gitCherryPickAbort(params)
	case "list":
		return te.gitCherryPickList(params)
	case "preview":
		return te.gitCherryPickPreview(params)
	default:
		return &ToolResult{
			Success: false,
			Error: fmt.Sprintf("unknown action: %s. Valid actions: 'cherry-pick', 'abort', 'list', 'preview'", action),
		}
	}
}

// gitCherryPickCherryPick applies a commit or range of commits from another branch.
func (te *ToolExecutor) gitCherryPickCherryPick(params map[string]interface{}) *ToolResult {
	// Required: commit hash or range
	commit, hasCommit := params["commit"].(string)
	if !hasCommit || commit == "" {
		return &ToolResult{
			Success: false,
			Error: "missing required parameter: commit (commit hash or range like 'abc123..def456')",
		}
	}

	// Optional: continue after conflict
	if cont, ok := params["continue"].(bool); ok && cont {
		args := []string{"cherry-pick", "--continue"}

		// Optional: sign-off
		if sign, ok := params["signoff"].(bool); ok && sign {
			args = append(args, "--signoff")
		}

		cmd := exec.Command("git", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return &ToolResult{
				Success: false,
				Error: fmt.Sprintf("cherry-pick continue failed: %s\nOutput: %s", err, string(output)),
			}
		}

		return &ToolResult{
			Success: true,
			Output: string(output),
			Extra: map[string]interface{}{
				"tool":    "git_cherry_pick",
				"action":  "cherry-pick",
				"message": "Cherry-pick continue successful.",
			},
		}
	}

	// Build cherry-pick arguments
	args := []string{"cherry-pick"}

	// Optional: allow-empty
	if allowEmpty, ok := params["allow_empty"].(bool); ok && allowEmpty {
		args = append(args, "--allow-empty")
	}

	// Optional: signoff
	if signoff, ok := params["signoff"].(bool); ok && signoff {
		args = append(args, "--signoff")
	}

	// Optional: edit commit message
	if edit, ok := params["edit"].(bool); ok && edit {
		args = append(args, "--edit")
	}

	// Optional: no-commit (stage changes without committing)
	if noCommit, ok := params["no_commit"].(bool); ok && noCommit {
		args = append(args, "--no-commit")
	}

	// Optional: commit message override
	if msg, ok := params["message"].(string); ok && msg != "" {
		args = append(args, "-m", msg)
	}

	// Optional: strategy for resolving conflicts
	if strategy, ok := params["strategy"].(string); ok && strategy != "" {
		args = append(args, "-X", strategy)
	}

	// Add commit hash or range
	args = append(args, commit)

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	// Check for conflicts
	outputStr := string(output)
	hasConflict := strings.Contains(outputStr, "CONFLICT") || strings.Contains(outputStr, "could not apply")

	if err != nil && hasConflict {
		// Parse conflict details
		conflictDetails := te.parseCherryPickConflict(outputStr, string(output))

		return &ToolResult{
			Success: false,
			Output:  outputStr,
			Error:   fmt.Sprintf("cherry-pick conflict detected while applying %s\n%s", commit, conflictDetails),
			Extra: map[string]interface{}{
				"tool":       "git_cherry_pick",
				"action":     "cherry-pick",
				"hasConflict": true,
				"conflictDetails": conflictDetails,
			},
		}
	}

	if err != nil {
		return &ToolResult{
			Success: false,
			Output:  outputStr,
			Error: fmt.Sprintf("cherry-pick failed: %s\nOutput: %s", err, outputStr),
			Extra: map[string]interface{}{
				"tool":    "git_cherry_pick",
				"action":  "cherry-pick",
				"message": fmt.Sprintf("Failed to cherry-pick %s", commit),
			},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  outputStr,
		Extra: map[string]interface{}{
			"tool":    "git_cherry_pick",
			"action":  "cherry-pick",
			"message": fmt.Sprintf("Successfully cherry-picked %s", commit),
		},
	}
}

// parseCherryPickConflict extracts conflict information from git output.
func (te *ToolExecutor) parseCherryPickConflict(output, rawOutput string) string {
	var lines []string

	// Extract conflict markers
	conflictLines := strings.Split(output, "\n")
	inConflict := false

	for _, line := range conflictLines {
		if strings.Contains(line, "CONFLICT") {
			inConflict = true
			lines = append(lines, line)
		} else if inConflict && strings.TrimSpace(line) != "" {
			lines = append(lines, "  "+line)
		} else if inConflict && strings.TrimSpace(line) == "" {
			inConflict = false
		}
	}

	// Extract conflicted files from git status
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusOutput, err := statusCmd.CombinedOutput()
	if err == nil {
		statusLines := strings.Split(strings.TrimSpace(string(statusOutput)), "\n")
		var conflictedFiles []string
		for _, line := range statusLines {
			// UU = both modified (conflict)
			if strings.HasPrefix(line, "UU ") || strings.HasPrefix(line, "UU\t") {
				parts := strings.SplitN(line, "\t", 2)
				if len(parts) > 1 {
					conflictedFiles = append(conflictedFiles, parts[1])
				}
			}
		}

		if len(conflictedFiles) > 0 {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, "Conflicted files:")
			for _, f := range conflictedFiles {
				lines = append(lines, "  - "+f)
			}
		}
	}

	return strings.Join(lines, "\n")
}

// gitCherryPickAbort cancels an in-progress cherry-pick.
func (te *ToolExecutor) gitCherryPickAbort(params map[string]interface{}) *ToolResult {
	args := []string{"cherry-pick", "--abort"}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &ToolResult{
			Success: false,
			Error: fmt.Sprintf("cherry-pick abort failed: %s\nOutput: %s", err, string(output)),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  "Cherry-pick session aborted successfully.",
		Extra: map[string]interface{}{
			"tool":    "git_cherry_pick",
			"action":  "abort",
			"message": "Cherry-pick session aborted.",
		},
	}
}

// gitCherryPickList shows commits available to cherry-pick from a target branch.
func (te *ToolExecutor) gitCherryPickList(params map[string]interface{}) *ToolResult {
	// Required: target branch to list commits from
	targetBranch, hasTarget := params["target"].(string)
	if !hasTarget || targetBranch == "" {
		return &ToolResult{
			Success: false,
			Error: "missing required parameter: target (branch name to list commits from)",
		}
	}

	// Get current branch
	currentBranchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	currentBranchOutput, err := currentBranchCmd.CombinedOutput()
	if err != nil {
		return &ToolResult{
			Success: false,
			Error: fmt.Sprintf("failed to get current branch: %s\nOutput: %s", err, string(currentBranchOutput)),
		}
	}
	currentBranch := strings.TrimSpace(string(currentBranchOutput))

	// Find commits in target branch that are NOT in current branch
	// Using --not to exclude commits already in current branch
	listArgs := []string{"log", "--format=%H|%an|%ae|%ad|%s", "--date=short", fmt.Sprintf("%s..%s", currentBranch, targetBranch)}

	cmd := exec.Command("git", listArgs...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error: fmt.Sprintf("failed to list commits from %s: %s\nOutput: %s", targetBranch, err, string(output)),
		}
	}

	// Parse commits
	type commitEntry struct {
		hash     string
		shortHash string
		author   string
		email    string
		date     string
		message  string
	}

	var commits []commitEntry
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 5)
		if len(parts) >= 5 {
			hash := parts[0]
			short := hash
			if len(hash) > 7 {
				short = hash[:7]
			}
			commits = append(commits, commitEntry{
				hash:        hash,
				shortHash:   short,
				author:      parts[1],
				email:       parts[2],
				date:        parts[3],
				message:     parts[4],
			})
		}
	}

	// Sort by date (oldest first for cherry-pick order)
	sort.Slice(commits, func(i, j int) bool {
		return commits[i].date < commits[j].date
	})

	// Limit output
	maxResults := 50
	if mr, ok := params["max_results"].(float64); ok {
		maxResults = int(mr)
	}
	if len(commits) > maxResults {
		commits = commits[:maxResults]
	}

	// Build output
	var outputStr strings.Builder
	outputStr.WriteString(fmt.Sprintf("Commits available to cherry-pick from '%s' into '%s':\n\n", targetBranch, currentBranch))
	outputStr.WriteString(fmt.Sprintf("Total commits available: %d\n", len(commits)))

	if len(commits) > 0 {
		outputStr.WriteString("\nCommits (oldest first):\n")
		for i, c := range commits {
			outputStr.WriteString(fmt.Sprintf("  %d. %s %s by %s <%s> on %s\n     %s\n",
				i+1, c.shortHash, c.date, c.author, c.email, c.date, c.message))
		}

		outputStr.WriteString("\nTo cherry-pick a specific commit:\n")
		outputStr.WriteString("  git_cherry_pick(action='cherry-pick', commit='<hash>')\n\n")
		outputStr.WriteString("To cherry-pick a range of commits:\n")
		outputStr.WriteString("  git_cherry_pick(action='cherry-pick', commit='<start_hash>..<end_hash>')\n")
	} else {
		outputStr.WriteString("\nNo commits available to cherry-pick. The target branch is already up to date with the current branch.\n")
	}

	return &ToolResult{
		Success: true,
		Output:  outputStr.String(),
		Extra: map[string]interface{}{
			"tool":          "git_cherry_pick",
			"action":        "list",
			"targetBranch":  targetBranch,
			"currentBranch": currentBranch,
			"totalCommits":  len(commits),
		},
	}
}

// gitCherryPickPreview shows what changes a cherry-pick would produce without applying it.
func (te *ToolExecutor) gitCherryPickPreview(params map[string]interface{}) *ToolResult {
	// Required: commit hash or range
	commit, hasCommit := params["commit"].(string)
	if !hasCommit || commit == "" {
		return &ToolResult{
			Success: false,
			Error: "missing required parameter: commit (commit hash or range like 'abc123..def456')",
		}
	}

	// --dry-run doesn't exist for cherry-pick, so we use a different approach:
	// Apply with --no-commit, then diff against HEAD, then reset
	// But let's try the simplest approach first

	// Try applying with --no-commit
	argsNoCommit := []string{"cherry-pick", "--no-commit", "--strategy=resolve", "-X", "theirs", commit}

	cmd := exec.Command("git", argsNoCommit...)
	output, err := cmd.CombinedOutput()

	// Always reset to clean state after preview
	resetCmd := exec.Command("git", "reset", "--hard", "HEAD")
	resetCmd.CombinedOutput()

	outputStr := string(output)

	if err != nil {
		// Check if it's a conflict (which is still useful preview info)
		hasConflict := strings.Contains(outputStr, "CONFLICT") || strings.Contains(outputStr, "could not apply")
		if hasConflict {
			conflictDetails := te.parseCherryPickConflict(outputStr, string(output))
			return &ToolResult{
				Success: true, // Still useful information
				Output: fmt.Sprintf("Preview of cherry-picking %s:\n\nCONFLICT DETECTED\n%s\n\nChanges were not applied. Preview aborted and working tree reset.\n",
					commit, conflictDetails),
				Extra: map[string]interface{}{
					"tool":         "git_cherry_pick",
					"action":       "preview",
					"hasConflict":  true,
					"commit":       commit,
					"conflictInfo": conflictDetails,
				},
			}
		}

		return &ToolResult{
			Success: false,
			Error: fmt.Sprintf("preview failed: %s\nOutput: %s", err, outputStr),
			Extra: map[string]interface{}{
				"tool":   "git_cherry_pick",
				"action": "preview",
				"commit": commit,
			},
		}
	}

	// Get diff of what would be changed
	diffCmd := exec.Command("git", "diff", "HEAD", "--stat")
	diffOutput, _ := diffCmd.CombinedOutput()

	// Get detailed diff
	diffDetailCmd := exec.Command("git", "diff", "HEAD")
	diffDetailOutput, _ := diffDetailCmd.CombinedOutput()

	// Commit the preview changes so we can get full info
	previewCommitCmd := exec.Command("git", "commit", "-m", "__preview_cherry_pick__")
	previewCommitCmd.CombinedOutput()

	// Get commit message
	commitMsgCmd := exec.Command("git", "log", "-1", "--format=%B")
	commitMsgOutput, _ := commitMsgCmd.CombinedOutput()

	// Reset back to original state
	resetCmd2 := exec.Command("git", "reset", "--hard", "HEAD~1")
	resetCmd2.CombinedOutput()

	var outputStrBuilder strings.Builder
	outputStrBuilder.WriteString(fmt.Sprintf("Preview of cherry-picking %s:\n\n", commit))
	outputStrBuilder.WriteString(fmt.Sprintf("Commit message would be:\n%s\n\n", strings.TrimSpace(string(commitMsgOutput))))
	outputStrBuilder.WriteString(fmt.Sprintf("Changes summary:\n%s\n\n", strings.TrimSpace(string(diffOutput))))

	if len(diffDetailOutput) > 0 {
		// Truncate very large diffs
		diffStr := string(diffDetailOutput)
		if len(diffStr) > 5000 {
			diffStr = diffStr[:5000] + "...\n(truncated)"
		}
		outputStrBuilder.WriteString("Detailed changes:\n" + diffStr)
	}

	return &ToolResult{
		Success: true,
		Output:  outputStrBuilder.String(),
		Extra: map[string]interface{}{
			"tool":    "git_cherry_pick",
			"action":  "preview",
			"commit":  commit,
			"message": "Preview completed successfully. No changes were permanently applied.",
		},
	}
}

// gitCherryPickGetCommitInfo retrieves information about a commit.
func (te *ToolExecutor) gitCherryPickGetCommitInfo(hash string) map[string]interface{} {
	info := make(map[string]interface{})

	// Get short hash
	shortCmd := exec.Command("git", "rev-parse", "--short", hash)
	shortOutput, err := shortCmd.CombinedOutput()
	if err == nil {
		info["hash"] = strings.TrimSpace(string(shortOutput))
		info["short_hash"] = strings.TrimSpace(string(shortOutput))
	}

	// Get full hash
	fullCmd := exec.Command("git", "rev-parse", hash)
	fullOutput, err := fullCmd.CombinedOutput()
	if err == nil {
		info["full_hash"] = strings.TrimSpace(string(fullOutput))
	}

	// Get commit message
	msgCmd := exec.Command("git", "log", "-1", "--format=%s", hash)
	msgOutput, err := msgCmd.CombinedOutput()
	if err == nil {
		info["message"] = strings.TrimSpace(string(msgOutput))
	}

	// Get author
	authorCmd := exec.Command("git", "log", "-1", "--format=%an <%ae>", hash)
	authorOutput, err := authorCmd.CombinedOutput()
	if err == nil {
		info["author"] = strings.TrimSpace(string(authorOutput))
	}

	// Get date
	dateCmd := exec.Command("git", "log", "-1", "--format=%ad", "--date=short", hash)
	dateOutput, err := dateCmd.CombinedOutput()
	if err == nil {
		info["date"] = strings.TrimSpace(string(dateOutput))
	}

	// Get diff stats
	statsCmd := exec.Command("git", "log", "-1", "--format=%n", "--stat", hash)
	statsOutput, err := statsCmd.CombinedOutput()
	if err == nil {
		info["stat"] = strings.TrimSpace(string(statsOutput))
	}

	return info
}

// gitCherryPickValidateCommit checks if a commit hash is valid and accessible.
func (te *ToolExecutor) gitCherryPickValidateCommit(hash string) (*ToolResult, bool) {
	// Try to resolve the hash
	cmd := exec.Command("git", "rev-parse", "--verify", hash)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error: fmt.Sprintf("commit '%s' is not a valid commit reference. Output: %s",
				hash, string(output)),
		}, false
	}

	// Check if it's a branch (would be confusing to cherry-pick a branch name)
	verifyCmd := exec.Command("git", "cat-file", "-t", strings.TrimSpace(string(output)))
	verifyOutput, verifyErr := verifyCmd.CombinedOutput()
	if verifyErr == nil && strings.TrimSpace(string(verifyOutput)) == "commit" {
		return nil, true
	}

	return &ToolResult{
		Success: false,
		Error: fmt.Sprintf("'%s' is not a commit reference (it's a %s). Use a commit hash instead.",
			hash, strings.TrimSpace(string(verifyOutput))),
	}, false
}

// gitCherryPickValidateBranch checks if a branch name is valid.
func (te *ToolExecutor) gitCherryPickValidateBranch(name string) (*ToolResult, bool) {
	cmd := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+name)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error: fmt.Sprintf("branch '%s' does not exist. Output: %s", name, string(output)),
		}, false
	}

	return nil, true
}
