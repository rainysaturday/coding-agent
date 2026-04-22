# Requirement #062: Git Merge Tool

## Description
A unified Git merge tool that handles merging branches, aborting merges, checking merge status, and merging pull requests from GitHub.

## Acceptance Criteria

### Core Merge
- [ ] `merge` action: merge a source branch into the current branch
  - Validates that source branch exists
  - Validates that current branch can be merged into
  - Performs the merge with structured output
  - Reports success/failure with details

### Merge Abort
- [ ] `abort` action: abort an in-progress merge
  - Only works when a merge is in progress
  - Restores working tree to pre-merge state
  - Returns structured success/failure

### Merge Status
- [ ] `status` action: check if a merge is in progress with conflicts
  - Detects MERGE_HEAD file to determine if merge is ongoing
  - Lists files with conflicts
  - Shows conflict summary

### Squash Merge
- [ ] `squash` action: perform a squash merge
  - Combines all commits from source into a single commit
  - Creates a single commit on the target branch
  - Returns the squashed commit hash

### PR Merge (GitHub)
- [ ] `merge_pr` action: merge a GitHub pull request
  - Requires `github_token` parameter
  - Requires `repo` parameter (owner/repo format)
  - Requires `pr_number` parameter
  - Supports `merge_method` parameter (merge, squash, rebase)
  - Uses GitHub API for merging
  - Returns structured PR merge result

### Error Handling
- [ ] Clear error messages for:
  - Source branch doesn't exist
  - Target branch has uncommitted changes
  - Merge conflicts
  - Not in a git repository
  - No merge in progress for abort

### Implementation Constraints
- [ ] Uses only Go stdlib (zero external dependencies)
- [ ] Git operations via `exec.Command("git", ...)`
- [ ] GitHub API via `net/http` stdlib only
