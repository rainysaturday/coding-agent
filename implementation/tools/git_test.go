package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Helper to setup a git repo
func setupGitRepo(t *testing.T, tmpDir string) {
	t.Helper()
	exec.Command("git", "init", tmpDir).Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
}

// ===== Tests for git_log tool =====

func TestExecute_GitLog_NotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_log",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})
	if result.Success {
		t.Error("Expected failure for non-git repository")
	}
	if !strings.Contains(result.Error, "not a git repository") {
		t.Errorf("Expected 'not a git repository' error, got: %s", result.Error)
	}
}

func TestExecute_GitLog_NoCommits(t *testing.T) {
	tmpDir := t.TempDir()
	// Initialize empty git repo
	setupGitRepo(t, tmpDir)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_log",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success for empty git repo, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "No commits found") {
		t.Errorf("Expected 'No commits found', got: %s", result.Output)
	}
}

func TestExecute_GitLog_WithCommits(t *testing.T) {
	tmpDir := t.TempDir()
	// Initialize git repo and create a commit
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_log",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "Initial commit") {
		t.Errorf("Expected 'Initial commit' in output, got: %s", result.Output)
	}
}

func TestExecute_GitLog_CountParameter(t *testing.T) {
	tmpDir := t.TempDir()
	// Initialize git repo and create multiple commits
	setupGitRepo(t, tmpDir)
	for i := 1; i <= 5; i++ {
		testFile := filepath.Join(tmpDir, fmt.Sprintf("file%d.txt", i))
		os.WriteFile(testFile, []byte(fmt.Sprintf("content %d", i)), 0644)
		exec.Command("git", "-C", tmpDir, "add", ".").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", fmt.Sprintf("Commit %d", i)).Run()
	}

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_log",
		Parameters: map[string]interface{}{
			"path":  tmpDir,
			"count": 2.0,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	// Should only show 2 commits
	if strings.Count(result.Output, "Commit ") < 2 {
		t.Errorf("Expected at least 2 commits in output, got: %s", result.Output)
	}
}

func TestExecute_GitLog_OnelineFlag(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_log",
		Parameters: map[string]interface{}{
			"path":  tmpDir,
			"flags": []interface{}{"oneline"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	// Oneline format should contain commit hash
	if !strings.Contains(result.Output, "Initial commit") {
		t.Errorf("Expected 'Initial commit' in output, got: %s", result.Output)
	}
}

func TestExecute_GitLog_InReadOnlyMode(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_log",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})
	if !result.Success {
		t.Fatalf("Expected git_log to succeed in read-only mode, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "Initial commit") {
		t.Errorf("Expected 'Initial commit' in output, got: %s", result.Output)
	}
}

func TestExecute_GitLog_InReadOnlyMode_Allowed(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	te.SetReadOnly(true)

	// git_log should work in read-only mode
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_log",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})
	if !result.Success {
		t.Fatalf("Expected git_log to succeed in read-only mode, got: %s", result.Error)
	}
}

func TestExecute_GitLog_ReferenceParameter(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_log",
		Parameters: map[string]interface{}{
			"path":      tmpDir,
			"reference": "HEAD",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "Initial commit") {
		t.Errorf("Expected 'Initial commit' in output, got: %s", result.Output)
	}
}

func TestExecute_GitLog_PathLimit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create files in different subdirectories
	os.Mkdir(filepath.Join(tmpDir, "subdir1"), 0755)
	os.Mkdir(filepath.Join(tmpDir, "subdir2"), 0755)
	testFile1 := filepath.Join(tmpDir, "subdir1", "test.txt")
	testFile2 := filepath.Join(tmpDir, "subdir2", "test.txt")
	os.WriteFile(testFile1, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Add subdir1").Run()

	os.WriteFile(testFile2, []byte("foo bar\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Add subdir2").Run()

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_log",
		Parameters: map[string]interface{}{
			"path":  filepath.Join(tmpDir, "subdir1"),
			"count": 10.0,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
}

func TestExecute_GitLog_MergesFlag(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create a simple commit (no merge in this simple test)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_log",
		Parameters: map[string]interface{}{
			"path":  tmpDir,
			"flags": []interface{}{"m"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	// No merges in this repo, so output should indicate no commits
}

func TestExecute_GitLog_ExtraInfo(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_log",
		Parameters: map[string]interface{}{
			"path":      tmpDir,
			"count":     5.0,
			"reference": "HEAD",
			"flags":     []interface{}{"oneline"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if result.Extra == nil {
		t.Fatal("Expected Extra map")
	}
	if count, ok := result.Extra["count"].(int); !ok || count != 5 {
		t.Errorf("Expected count 5 in Extra, got %v", result.Extra)
	}
	if ref, ok := result.Extra["reference"].(string); !ok || ref != "HEAD" {
		t.Errorf("Expected reference 'HEAD' in Extra, got %v", result.Extra)
	}
}

func TestExecuteGitLog_NArgs(t *testing.T) {
	te := NewToolExecutor()

	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_log",
		Parameters: map[string]interface{}{
			"n": 10,
		},
	})

	// May succeed or fail depending on git repo state
	_ = result
}

func TestExecuteGitLog_Simple(t *testing.T) {
	te := NewToolExecutor()

	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_log",
	})

	// May succeed or fail depending on git repo state
	_ = result
}

func TestExecuteGitLog_NonCtx(t *testing.T) {
	te := NewToolExecutor()
	result := te.executeGitLog(context.Background(), map[string]interface{}{})
	// May succeed or fail depending on git repo state
	_ = result
}

// ===== Tests for git_show tool =====

func TestExecute_GitShow_NotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_show",
		Parameters: map[string]interface{}{
			"path":   tmpDir,
			"commit": "HEAD",
		},
	})
	if result.Success {
		t.Error("Expected failure for non-git repository")
	}
	if !strings.Contains(result.Error, "not a git repository") {
		t.Errorf("Expected 'not a git repository' error, got: %s", result.Error)
	}
}

func TestExecute_GitShow_NoCommits(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_show",
		Parameters: map[string]interface{}{
			"path":   tmpDir,
			"commit": "HEAD",
		},
	})
	// Should fail because HEAD doesn't exist in empty repo
	if result.Success {
		t.Error("Expected failure for HEAD in empty repo")
	}
}

func TestExecute_GitShow_WithCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_show",
		Parameters: map[string]interface{}{
			"path":   tmpDir,
			"commit": "HEAD",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "Initial commit") {
		t.Errorf("Expected 'Initial commit' in output, got: %s", result.Output)
	}
}

func TestExecute_GitShow_DefaultCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	// Don't specify commit parameter - should default to HEAD
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_show",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success with default commit, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "Initial commit") {
		t.Errorf("Expected 'Initial commit' in output, got: %s", result.Output)
	}
}

func TestExecute_GitShow_StatFlag(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_show",
		Parameters: map[string]interface{}{
			"path":   tmpDir,
			"commit": "HEAD",
			"flags":  []interface{}{"stat"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	// Stat format shows file changes summary
	if !strings.Contains(result.Output, "1 file changed") && !strings.Contains(result.Output, "test.txt") {
		t.Errorf("Expected file change info in stat output, got: %s", result.Output)
	}
}

func TestExecute_GitShow_NameStatusFlag(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_show",
		Parameters: map[string]interface{}{
			"path":   tmpDir,
			"commit": "HEAD",
			"flags":  []interface{}{"name-status"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	// Name status shows A/M/D with filenames
	if !strings.Contains(result.Output, "A\t") && !strings.Contains(result.Output, "test.txt") {
		t.Errorf("Expected name-status output, got: %s", result.Output)
	}
}

func TestExecute_GitShow_PathLimit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_show",
		Parameters: map[string]interface{}{
			"path":   tmpDir,
			"commit": "HEAD",
			"flags":  []interface{}{"stat"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "Initial commit") {
		t.Errorf("Expected commit info in output, got: %s", result.Output)
	}
}

func TestExecute_GitShow_InReadOnlyMode(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_show",
		Parameters: map[string]interface{}{
			"path":   tmpDir,
			"commit": "HEAD",
		},
	})
	if !result.Success {
		t.Fatalf("Expected git_show to succeed in read-only mode, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "Initial commit") {
		t.Errorf("Expected 'Initial commit' in output, got: %s", result.Output)
	}
}

func TestExecute_GitShow_InReadOnlyMode_Allowed(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	te.SetReadOnly(true)

	// git_show should work in read-only mode
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_show",
		Parameters: map[string]interface{}{
			"path":   tmpDir,
			"commit": "HEAD",
		},
	})
	if !result.Success {
		t.Fatalf("Expected git_show to succeed in read-only mode, got: %s", result.Error)
	}
}

func TestExecute_GitShow_CommitNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_show",
		Parameters: map[string]interface{}{
			"path":   tmpDir,
			"commit": "nonexistent-commit-sha",
		},
	})
	// Should fail because commit doesn't exist
	if result.Success {
		t.Error("Expected failure for nonexistent commit")
	}
}

func TestExecute_GitShow_ExtraInfo(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_show",
		Parameters: map[string]interface{}{
			"path":   tmpDir,
			"commit": "HEAD",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if result.Extra == nil {
		t.Fatal("Expected Extra map")
	}
	if ref, ok := result.Extra["commitReference"].(string); !ok || ref != "HEAD" {
		t.Errorf("Expected commitReference 'HEAD' in Extra, got %v", result.Extra)
	}
}

func TestExecute_GitShow_EmptyOutput(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "--allow-empty", "-m", "Empty commit").Run()

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_show",
		Parameters: map[string]interface{}{
			"path":   tmpDir,
			"commit": "HEAD",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
}

func TestExecuteGitShow_Simple(t *testing.T) {
	te := NewToolExecutor()

	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_show",
		Parameters: map[string]interface{}{
			"commit": "HEAD",
		},
	})

	// May succeed or fail depending on git repo state
	_ = result
}

func TestExecuteGitShow_NonCtx(t *testing.T) {
	te := NewToolExecutor()
	result := te.executeGitShow(context.Background(), map[string]interface{}{})
	// May succeed or fail depending on git repo state
	_ = result
}

// ===== Tests for git_diff tool =====

func TestExecute_GitDiff_NotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_diff",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})
	if result.Success {
		t.Error("Expected failure for non-git repository")
	}
	if !strings.Contains(result.Error, "not a git repository") {
		t.Errorf("Expected 'not a git repository' error, got: %s", result.Error)
	}
}

func TestExecute_GitDiff_NoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_diff",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
}

func TestExecute_GitDiff_WithChanges(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	// Make a change
	os.WriteFile(testFile, []byte("hello world modified\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_diff",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "hello world") {
		t.Errorf("Expected diff content, got: %s", result.Output)
	}
}

func TestExecute_GitDiff_TwoReferences(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// First commit
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "First commit").Run()

	// Second commit
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Second commit").Run()

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_diff",
		Parameters: map[string]interface{}{
			"path":       tmpDir,
			"reference1": "HEAD~1",
			"reference2": "HEAD",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "hello") {
		t.Errorf("Expected diff content, got: %s", result.Output)
	}
}

func TestExecute_GitDiff_StatFlag(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	// Make a change
	os.WriteFile(testFile, []byte("hello world modified\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_diff",
		Parameters: map[string]interface{}{
			"path":  tmpDir,
			"flags": []interface{}{"stat"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
}

func TestExecute_GitDiff_InReadOnlyMode(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_diff",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})
	if !result.Success {
		t.Fatalf("Expected git_diff to succeed in read-only mode, got: %s", result.Error)
	}
}

func TestExecute_GitDiff_ExtraInfo(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_diff",
		Parameters: map[string]interface{}{
			"path":       tmpDir,
			"reference1": "HEAD",
			"reference2": "HEAD~1",
		},
	})
	// Git diff may succeed or fail - just check it doesn't panic
	_ = result.Extra
}

func TestExecuteGitDiff_NoArgs(t *testing.T) {
	te := NewToolExecutor()

	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_diff",
	})

	// May succeed or fail depending on git repo state
	_ = result
}

func TestExecuteGitDiff_SingleArg(t *testing.T) {
	te := NewToolExecutor()

	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_diff",
		Parameters: map[string]interface{}{
			"arg": "HEAD~1",
		},
	})

	// May succeed or fail depending on git repo state
	_ = result
}

func TestExecuteGitDiff_TwoArgs(t *testing.T) {
	te := NewToolExecutor()

	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_diff",
		Parameters: map[string]interface{}{
			"arg":  "HEAD~2",
			"arg2": "HEAD~1",
		},
	})

	// May succeed or fail depending on git repo state
	_ = result
}

func TestExecuteGitDiff_Simple(t *testing.T) {
	te := NewToolExecutor()

	result := te.Execute(context.Background(), &ToolCall{
		Name: "git_diff",
	})

	// May succeed or fail depending on git repo state
	_ = result
}

func TestExecuteGitDiff(t *testing.T) {
	te := NewToolExecutor()

	result := te.executeGitDiff(context.Background(), map[string]interface{}{})
	// May succeed or fail depending on whether we're in a git repo
	if result.Error != "" && !strings.Contains(result.Error, "not a git repository") && !strings.Contains(result.Error, "git: 'diff' is not a git") {
		t.Logf("Git diff error (may be expected outside git repo): %s", result.Error)
	}
}

// ===== Flag handling: dashed vs bare spellings, unknown flags =====

func TestBuildGitLogArgs_AcceptsDashedAndBareSpellings(t *testing.T) {
	// The tool description teaches "--stat"; the allowlist used to match only
	// "stat", so documented usage was silently dropped.
	args, err := buildGitLogArgs(&gitLogParams{
		count: 5,
		flags: []string{"--oneline", "oneline", "--stat", "stat", "--no-merges"},
	})
	if err != nil {
		t.Fatalf("buildGitLogArgs() error: %v", err)
	}
	for _, want := range []string{"--oneline", "--stat", "--no-merges"} {
		if !containsString(args, want) {
			t.Errorf("args %v missing %q", args, want)
		}
	}
}

func TestBuildGitLogArgs_UnknownFlagIsReported(t *testing.T) {
	_, err := buildGitLogArgs(&gitLogParams{count: 5, flags: []string{"--nope"}})
	if err == nil {
		t.Fatal("expected an error for an unsupported flag, got nil (flag was silently dropped)")
	}
	if !strings.Contains(err.Error(), "--nope") {
		t.Errorf("error must name the offending flag, got: %v", err)
	}
	if !strings.Contains(err.Error(), "supported flags") {
		t.Errorf("error must list the supported flags, got: %v", err)
	}
}

func TestBuildGitLogArgs_GrepFlagWithoutPattern(t *testing.T) {
	// --grep with no pattern is a no-op for git; it must not look like a
	// filtered log.
	_, err := buildGitLogArgs(&gitLogParams{count: 5, flags: []string{"grep"}})
	if err == nil {
		t.Fatal("expected an error when the grep flag is used without the grep parameter")
	}
	if !strings.Contains(err.Error(), "grep") {
		t.Errorf("error should mention grep, got: %v", err)
	}
}

// setupFlagRepo creates a git repository with two commits for flag tests.
func setupFlagRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v: %s", err, out)
	}
	for i := 0; i < 2; i++ {
		name := fmt.Sprintf("f%d.txt", i)
		if err := os.WriteFile(filepath.Join(repo, name), []byte("x\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if out, err := exec.Command("git", "-C", repo, "add", name).CombinedOutput(); err != nil {
			t.Fatalf("git add: %v: %s", err, out)
		}
		if out, err := exec.Command("git", "-C", repo, "-c", "user.email=t@example.com",
			"-c", "user.name=Test", "commit", "-m", "commit "+name).CombinedOutput(); err != nil {
			t.Fatalf("git commit: %v: %s", err, out)
		}
	}
	return repo
}

func runGitLog(t *testing.T, repo string, flags []string) *ToolResult {
	t.Helper()
	te := NewToolExecutor()
	return te.Execute(context.Background(), &ToolCall{
		Name:       "git_log",
		Parameters: map[string]interface{}{"path": repo, "flags": flags},
	})
}

func TestGitLog_ConflictCheckSeesDashedFlags(t *testing.T) {
	// The conflict guard compares bare names; a dashed "--oneline" used to slip
	// past it and reach git as a confusing git error.
	result := runGitLog(t, setupFlagRepo(t), []string{"--oneline", "--stat"})
	if result.Success {
		t.Fatalf("expected the oneline/stat conflict to be reported, got output: %s", result.Output)
	}
	if !strings.Contains(result.Error, "conflicting flags") {
		t.Errorf("expected a conflict error, got: %s", result.Error)
	}
}

// The point of the fix: a documented dashed flag must actually change what git
// prints, instead of vanishing into a plain log.
func TestGitLog_DashedFlagReachesGit(t *testing.T) {
	repo := setupFlagRepo(t)

	want, err := exec.Command("git", "-C", repo, "log", "--oneline", "--no-color").CombinedOutput()
	if err != nil {
		t.Fatalf("reference git invocation failed: %v", err)
	}

	result := runGitLog(t, repo, []string{"--oneline"})
	if !result.Success {
		t.Fatalf("git_log failed: %s", result.Error)
	}
	if got := strings.TrimRight(result.Output, "\n"); got != strings.TrimRight(string(want), "\n") {
		t.Errorf("dashed --oneline did not produce git's own oneline output\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestGitLog_UnknownFlagIsReportedToEndOfLine(t *testing.T) {
	result := runGitLog(t, setupFlagRepo(t), []string{"--follow-everything"})
	if result.Success {
		t.Fatalf("expected an error for an unsupported flag, got: %s", result.Output)
	}
	if !strings.Contains(result.Error, "unsupported flag") {
		t.Errorf("expected an unsupported-flag error, got: %s", result.Error)
	}
}

func TestBuildGitShowArgs_UnknownFlagAndNumstatAlias(t *testing.T) {
	if _, err := buildGitShowArgs(&gitShowParams{commit: "HEAD", flags: []string{"bogus"}}); err == nil {
		t.Error("expected an error for an unsupported git_show flag")
	}

	args, err := buildGitShowArgs(&gitShowParams{commit: "HEAD", flags: []string{"numstat", "--stat"}})
	if err != nil {
		t.Fatalf("buildGitShowArgs() error: %v", err)
	}
	if !containsString(args, "--numstat") || !containsString(args, "--stat") {
		t.Errorf("args %v must contain --numstat and --stat", args)
	}
}

func TestBuildGitDiffArgs_ValueFlags(t *testing.T) {
	args, err := buildGitDiffArgs(&gitDiffParams{flags: []string{"--unified=10", "stat-width=80"}})
	if err != nil {
		t.Fatalf("buildGitDiffArgs() error: %v", err)
	}
	if !containsString(args, "--unified=10") {
		t.Errorf("unified=10 was ignored, args: %v", args)
	}
	if !containsString(args, "--stat-width=80") {
		t.Errorf("stat-width=80 was ignored, args: %v", args)
	}

	// Bare forms keep their documented defaults.
	args, err = buildGitDiffArgs(&gitDiffParams{flags: []string{"unified"}})
	if err != nil {
		t.Fatalf("buildGitDiffArgs() error: %v", err)
	}
	if !containsString(args, "--unified=3") {
		t.Errorf("bare unified should default to 3, args: %v", args)
	}

	// A non-numeric value is an error, not a silently dropped flag.
	if _, err := buildGitDiffArgs(&gitDiffParams{flags: []string{"unified=lots"}}); err == nil {
		t.Error("expected an error for a non-numeric unified value")
	}

	// Unknown flags are reported.
	if _, err := buildGitDiffArgs(&gitDiffParams{flags: []string{"--nonsense"}}); err == nil {
		t.Error("expected an error for an unsupported git_diff flag")
	}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// TestGitAllowedFlags_AreAllSupported keeps the "unsupported flag" error honest:
// every flag named in the allowlist must actually be understood by the builder,
// otherwise the error message would send the caller in circles.
func TestGitAllowedFlags_AreAllSupported(t *testing.T) {
	cases := []struct {
		name    string
		allowed []string
		build   func(flag string) error
	}{
		{"git_log", gitLogAllowedFlags, func(f string) error {
			_, err := buildGitLogArgs(&gitLogParams{count: 5, flags: []string{f}, grep: "x"})
			return err
		}},
		{"git_show", gitShowAllowedFlags, func(f string) error {
			_, err := buildGitShowArgs(&gitShowParams{commit: "HEAD", flags: []string{f}})
			return err
		}},
		{"git_diff", gitDiffAllowedFlags, func(f string) error {
			_, err := buildGitDiffArgs(&gitDiffParams{flags: []string{f}})
			return err
		}},
	}
	for _, tc := range cases {
		for _, flag := range tc.allowed {
			if err := tc.build(flag); err != nil {
				t.Errorf("%s: allowlist advertises %q but the builder rejects it: %v", tc.name, flag, err)
			}
		}
	}
}
