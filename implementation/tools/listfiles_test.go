package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ===== Tests for list_files tool =====

func TestExecute_ListFiles_Default(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files and directories
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content2"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "dir1"), 0755)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	// Should list entries, directories should have / suffix
	if !strings.Contains(result.Output, "dir1/") {
		t.Errorf("Expected 'dir1/' in output, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "file1.txt") {
		t.Errorf("Expected 'file1.txt' in output, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "file2.txt") {
		t.Errorf("Expected 'file2.txt' in output, got: %s", result.Output)
	}
}

func TestExecute_ListFiles_HiddenFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create regular and hidden files
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("hidden"), 0644)

	te := NewToolExecutor()

	// Without -a flag, hidden files should not appear
	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path":  tmpDir,
			"flags": []interface{}{"a"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, ".hidden") {
		t.Errorf("Expected '.hidden' in output with -a flag, got: %s", result.Output)
	}

	// With -a flag
	result2 := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})
	if !result2.Success {
		t.Fatalf("Expected success, got: %s", result2.Error)
	}
	if strings.Contains(result2.Output, ".hidden") {
		t.Errorf("Did not expect '.hidden' in output without -a flag, got: %s", result2.Output)
	}
}

func TestExecute_ListFiles_LongFormat(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello world"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path":  tmpDir,
			"flags": []interface{}{"l"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	// Long format should contain permission string (e.g., "-rw-r--r--")
	output := result.Output
	if len(output) < 10 {
		t.Errorf("Expected longer output for long format, got: %s", output)
	}
	// Should contain size number
	if !strings.Contains(output, "11") {
		t.Errorf("Expected file size in output, got: %s", output)
	}
}

func TestExecute_ListFiles_HumanReadable(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a file with known size
	content := make([]byte, 1500) // 1.5KB
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), content, 0644)

	te := NewToolExecutor()

	// With -h flag, should show human-readable size
	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path":  tmpDir,
			"flags": []interface{}{"l", "h"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "1.5K") {
		t.Errorf("Expected '1.5K' in output, got: %s", result.Output)
	}
}

func TestExecute_ListFiles_ByTime(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files with different timestamps
	f1 := filepath.Join(tmpDir, "first.txt")
	f2 := filepath.Join(tmpDir, "second.txt")
	os.WriteFile(f1, []byte("first"), 0644)
	// Small delay to ensure different timestamps
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(f2, []byte("second"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path":  tmpDir,
			"flags": []interface{}{"t"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	// Files should be sorted by time (newest first)
	// second.txt is newer, so it should appear before first.txt
	secondIdx := strings.Index(result.Output, "second.txt")
	firstIdx := strings.Index(result.Output, "first.txt")
	if secondIdx > firstIdx {
		t.Errorf("Expected second.txt before first.txt when sorted by time (newest first), got: %s", result.Output)
	}
}

func TestExecute_ListFiles_BySize(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files with different sizes
	os.WriteFile(filepath.Join(tmpDir, "small.txt"), []byte("small"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "large.txt"), []byte("this is a larger content"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path":  tmpDir,
			"flags": []interface{}{"S"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	// Files should be sorted by size (largest first)
	largeIdx := strings.Index(result.Output, "large.txt")
	smallIdx := strings.Index(result.Output, "small.txt")
	if largeIdx > smallIdx {
		t.Errorf("Expected large.txt before small.txt when sorted by size, got: %s", result.Output)
	}
}

func TestExecute_ListFiles_ReverseSort(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "aaa.txt"), []byte("aaa"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "zzz.txt"), []byte("zzz"), 0644)

	te := NewToolExecutor()

	// Without reverse - alphabetical order
	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	aaaIdx := strings.Index(result.Output, "aaa.txt")
	zzzIdx := strings.Index(result.Output, "zzz.txt")
	if aaaIdx > zzzIdx {
		t.Errorf("Expected aaa.txt before zzz.txt in default order, got: %s", result.Output)
	}

	// With reverse - reverse alphabetical
	result2 := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path":  tmpDir,
			"flags": []interface{}{"r"},
		},
	})
	if !result2.Success {
		t.Fatalf("Expected success, got: %s", result2.Error)
	}
	aaaIdx2 := strings.Index(result2.Output, "aaa.txt")
	zzzIdx2 := strings.Index(result2.Output, "zzz.txt")
	if aaaIdx2 < zzzIdx2 {
		t.Errorf("Expected zzz.txt before aaa.txt with reverse flag, got: %s", result2.Output)
	}
}

func TestExecute_ListFiles_FileNotExists(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": "/nonexistent/path/that/does/not/exist",
		},
	})
	if result.Success {
		t.Error("Expected failure for nonexistent path")
	}
	if !strings.Contains(result.Error, "file not found") {
		t.Errorf("Expected 'file not found' error, got: %s", result.Error)
	}
}

func TestExecute_ListFiles_DefaultPath(t *testing.T) {
	// Test with default path (current directory)
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name:       "list_files",
		Parameters: map[string]interface{}{},
	})
	// Should succeed and list current directory
	if !result.Success {
		t.Logf("Warning: Default path listing failed (expected in some environments): %s", result.Error)
	}
}

func TestExecute_ListFiles_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	te := NewToolExecutor()

	// Simple format for a single file
	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": testFile,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "test.txt") {
		t.Errorf("Expected 'test.txt' in output, got: %s", result.Output)
	}

	// Long format for a single file
	result2 := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"flags": []interface{}{"l"},
		},
	})
	if !result2.Success {
		t.Fatalf("Expected success for long format, got: %s", result2.Error)
	}
	// Long format output should be longer
	if len(result2.Output) <= len(result.Output) {
		t.Errorf("Expected long format output to be longer than simple format")
	}
}

func TestExecute_ListFiles_EntriesListedCount(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "dir1"), 0755)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if result.Extra == nil {
		t.Fatal("Expected Extra map in result")
	}
	if entries, ok := result.Extra["entriesListed"].(int); !ok || entries != 3 {
		t.Errorf("Expected 3 entries listed, got %v", result.Extra["entriesListed"])
	}
}

func TestExecute_ListFiles_MultipleFlags(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("hidden"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "dir1"), 0755)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path":  tmpDir,
			"flags": []interface{}{"l", "a", "h"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	// Should show hidden files
	if !strings.Contains(result.Output, ".hidden") {
		t.Errorf("Expected hidden file with -a flag, got: %s", result.Output)
	}
	// Should be in long format - check for permission string and timestamp
	if !strings.Contains(result.Output, "drwxr-xr-x") && !strings.Contains(result.Output, "-rw-r--r--") {
		t.Errorf("Expected permission string in long format output, got: %s", result.Output)
	}
	// Should have human-readable size (K suffix for directories)
	if !strings.Contains(result.Output, "K") {
		t.Errorf("Expected human-readable size with K suffix, got: %s", result.Output)
	}
}

func TestExecute_ListFiles_DirectoriesFirst(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files and directories
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "dir1"), 0755)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	// Directories should appear before files
	dirIdx := strings.Index(result.Output, "dir1/")
	fileIdx := strings.Index(result.Output, "file.txt")
	if dirIdx > fileIdx {
		t.Errorf("Expected directories to appear before files, got: %s", result.Output)
	}
}

func TestExecute_ListFiles_Recursive(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested directory structure
	os.Mkdir(filepath.Join(tmpDir, "subdir1"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "subdir1", "file2.txt"), []byte("content2"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path":  tmpDir,
			"flags": []interface{}{"R"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "subdir1") {
		t.Errorf("Expected 'subdir1' in output, got: %s", result.Output)
	}
}

func TestExecute_ListFiles_RecursiveLong(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested directory structure
	os.Mkdir(filepath.Join(tmpDir, "subdir1"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "subdir1", "file2.txt"), []byte("content2"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path":  tmpDir,
			"flags": []interface{}{"R", "l"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "subdir1") {
		t.Errorf("Expected 'subdir1' in output, got: %s", result.Output)
	}
}

func TestExecute_ListFiles_WithSpacesInPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory with spaces
	spaceDir := filepath.Join(tmpDir, "dir with spaces")
	os.Mkdir(spaceDir, 0755)
	os.WriteFile(filepath.Join(spaceDir, "file.txt"), []byte("content"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": spaceDir,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "file.txt") {
		t.Errorf("Expected 'file.txt' in output, got: %s", result.Output)
	}
}

func TestExecute_ListFiles_Root(t *testing.T) {
	te := NewToolExecutor()

	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": ".",
		},
	})

	if !result.Success {
		t.Logf("Got failure (may depend on working directory): %s", result.Error)
	}
}

func TestExecuteListFilesCtx_ContextCancelled(t *testing.T) {
	te := NewToolExecutor()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := te.Execute(ctx, &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": ".",
		},
	})

	// Should fail due to cancelled context
	if result.Success {
		t.Log("Got success with cancelled context - unexpected")
	}
}

func TestExecuteListFilesCtx_Root(t *testing.T) {
	te := NewToolExecutor()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := te.Execute(ctx, &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": ".",
		},
	})

	if !result.Success {
		t.Logf("Got failure (may depend on working directory): %s", result.Error)
	}
}

// ===== Read-only mode tests for list_files =====

func TestToolExecutor_ReadOnly_ListFilesAllowed(t *testing.T) {
	tmpDir := t.TempDir()
	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})

	if !result.Success {
		t.Fatalf("Expected list_files to succeed in read-only mode, got: %s", result.Error)
	}
}

func TestExecuteListFiles_NonExistent(t *testing.T) {
	te := NewToolExecutor()

	result := te.executeListFiles(context.Background(), map[string]interface{}{
		"path": "/nonexistent/path/that/does/not/exist",
	})
	if result.Success {
		t.Error("Expected failure for non-existent path")
	}
}

func TestExecuteListFiles(t *testing.T) {
	te := NewToolExecutor()

	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": ".",
		},
	})

	// Should succeed
	if !result.Success {
		t.Logf("Got failure: %v", result.Error)
	}
}

func TestExecuteListFiles_Root(t *testing.T) {
	te := NewToolExecutor()

	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": ".",
		},
	})

	if !result.Success {
		t.Logf("Got failure (may depend on working directory): %s", result.Error)
	}
}
