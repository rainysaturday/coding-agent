package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// gitFlagPattern accepts safe git flags such as -n, --stat, --grep=value.
var gitFlagPattern = regexp.MustCompile(`^-{1,2}[A-Za-z][A-Za-z0-9_.=-]*$`)

func sanitizeFlags(flags []string) ([]string, error) {
	out := make([]string, 0, len(flags))
	for _, f := range flags {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if !strings.HasPrefix(f, "-") {
			f = "-" + f
		}
		if !gitFlagPattern.MatchString(f) {
			return nil, fmt.Errorf("unsupported git flag %q (flags must look like -n, --stat, --grep=value)", f)
		}
		out = append(out, f)
	}
	return out, nil
}

func runGit(ctx context.Context, repo string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repo
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if strings.Contains(msg, "not a git repository") {
			return "", fmt.Errorf("%s is not a git repository", repo)
		}
		if msg == "" {
			return "", fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
		}
		// git writes useful information to stderr for some commands
		if strings.TrimSpace(stdout.String()) != "" {
			return stdout.String(), nil
		}
		return "", errors.New(msg)
	}
	return stdout.String(), nil
}

func repoPath(run *RunContext, args map[string]interface{}) string {
	p := getString(args, "path")
	if p == "" {
		return run.CWD
	}
	return run.resolvePath(p)
}

// ---------------------------------------------------------------------------
// git_log
// ---------------------------------------------------------------------------

// GitLogTool shows commit logs.
type GitLogTool struct{ run *RunContext }

// NewGitLogTool creates the git_log tool.
func NewGitLogTool(run *RunContext) *GitLogTool { return &GitLogTool{run: run} }

func (t *GitLogTool) Name() string { return "git_log" }

func (t *GitLogTool) Description() string { return "Show commit logs from a git repository" }

func (t *GitLogTool) Schema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"path":      prop("string", "Path to the git repository (defaults to current directory)"),
		"reference": prop("string", "Git reference to view log from (branch name, tag, or commit hash; defaults to HEAD)"),
		"count":     prop("integer", "Number of commits to display (defaults to 10)"),
		"flags":     arrayProp("string", "List of git log flags to control output (e.g., '--oneline', '--stat', '--patch', '--follow', '--grep')"),
		"grep":      prop("string", "Search commit messages for this pattern (used with '--grep' flag)"),
	}, nil)
}

func (t *GitLogTool) ReadOnly() bool { return true }

func (t *GitLogTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	repo := repoPath(t.run, args)
	count := getInt(args, "count", 10)
	flags, err := sanitizeFlags(getStringSlice(args, "flags"))
	if err != nil {
		return "", err
	}
	gitArgs := []string{"-c", "color.ui=false", "log", "--no-color", fmt.Sprintf("-n%d", count)}
	gitArgs = append(gitArgs, flags...)
	if g := getString(args, "grep"); g != "" {
		gitArgs = append(gitArgs, "--grep="+g)
	}
	if ref := getString(args, "reference"); ref != "" {
		gitArgs = append(gitArgs, ref)
	}
	out, err := runGit(ctx, repo, gitArgs...)
	if err != nil {
		return "", err
	}
	return truncateLines(out, t.run.MaxOutputLines), nil
}

// ---------------------------------------------------------------------------
// git_show
// ---------------------------------------------------------------------------

// GitShowTool shows a commit.
type GitShowTool struct{ run *RunContext }

// NewGitShowTool creates the git_show tool.
func NewGitShowTool(run *RunContext) *GitShowTool { return &GitShowTool{run: run} }

func (t *GitShowTool) Name() string { return "git_show" }

func (t *GitShowTool) Description() string { return "Show information about a git commit" }

func (t *GitShowTool) Schema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"commit": prop("string", "Commit to show (defaults to HEAD)"),
		"flags":  arrayProp("string", "List of git show flags to control output (e.g., '--stat', '--patch', '--name-status')"),
		"path":   prop("string", "Path to the git repository (defaults to current directory)"),
	}, nil)
}

func (t *GitShowTool) ReadOnly() bool { return true }

func (t *GitShowTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	repo := repoPath(t.run, args)
	flags, err := sanitizeFlags(getStringSlice(args, "flags"))
	if err != nil {
		return "", err
	}
	commit := getString(args, "commit")
	if commit == "" {
		commit = "HEAD"
	}
	gitArgs := []string{"-c", "color.ui=false", "show", "--no-color"}
	gitArgs = append(gitArgs, flags...)
	gitArgs = append(gitArgs, commit)
	out, err := runGit(ctx, repo, gitArgs...)
	if err != nil {
		return "", err
	}
	return truncateLines(out, t.run.MaxOutputLines), nil
}

// ---------------------------------------------------------------------------
// git_diff
// ---------------------------------------------------------------------------

// GitDiffTool shows differences between git references.
type GitDiffTool struct{ run *RunContext }

// NewGitDiffTool creates the git_diff tool.
func NewGitDiffTool(run *RunContext) *GitDiffTool { return &GitDiffTool{run: run} }

func (t *GitDiffTool) Name() string { return "git_diff" }

func (t *GitDiffTool) Description() string {
	return "Show changes between commits, commit and working tree, etc."
}

func (t *GitDiffTool) Schema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"path":       prop("string", "Path to the git repository (defaults to current directory)"),
		"reference1": prop("string", "First git reference for comparison (commit hash, branch, tag; omit for working tree)"),
		"reference2": prop("string", "Second git reference for comparison (commit hash, branch, tag; omit for index or working tree)"),
		"flags":      arrayProp("string", "List of git diff flags to control output (e.g., '--stat', '--patch', '--name-status', '--numstat', '--summary', '--color')"),
	}, nil)
}

func (t *GitDiffTool) ReadOnly() bool { return true }

func (t *GitDiffTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	repo := repoPath(t.run, args)
	flags, err := sanitizeFlags(getStringSlice(args, "flags"))
	if err != nil {
		return "", err
	}
	gitArgs := []string{"-c", "color.ui=false", "diff", "--no-color"}
	gitArgs = append(gitArgs, flags...)
	if r1 := getString(args, "reference1"); r1 != "" {
		gitArgs = append(gitArgs, r1)
		if r2 := getString(args, "reference2"); r2 != "" {
			gitArgs = append(gitArgs, r2)
		}
	}
	out, err := runGit(ctx, repo, gitArgs...)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out) == "" {
		return "(no changes)", nil
	}
	return truncateLines(out, t.run.MaxOutputLines), nil
}
