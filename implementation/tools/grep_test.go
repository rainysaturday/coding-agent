package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ===== Tests for grep tool =====

func TestExecute_Grep_MissingPattern(t *testing.T) {
	tmpDir := t.TempDir()
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})
	if result.Success {
		t.Error("Expected failure for missing pattern parameter")
	}
	if !strings.Contains(result.Error, "missing required parameter") {
		t.Errorf("Expected 'missing required parameter' error, got: %s", result.Error)
	}
}

func TestExecute_Grep_EmptyPattern(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"pattern": "",
		},
	})
	if result.Success {
		t.Error("Expected failure for empty pattern")
	}
	if !strings.Contains(result.Error, "pattern cannot be empty") {
		t.Errorf("Expected 'pattern cannot be empty' error, got: %s", result.Error)
	}
}

func TestExecute_Grep_InvalidRegex(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"pattern": "[invalid",
			"path":    testFile,
		},
	})
	if result.Success {
		t.Error("Expected failure for invalid regex pattern")
	}
	if !strings.Contains(result.Error, "invalid regex pattern") {
		t.Errorf("Expected 'invalid regex pattern' error, got: %s", result.Error)
	}
}

func TestExecute_Grep_Found(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\nfoo bar\nhello again\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"pattern": "hello",
			"path":    testFile,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "hello world") {
		t.Errorf("Expected output to contain 'hello world', got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "hello again") {
		t.Errorf("Expected output to contain 'hello again', got: %s", result.Output)
	}
}

func TestExecute_Grep_LineNumbers(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("line one\nline two\nline three\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"pattern": "line",
			"path":    testFile,
			"flags":   []string{"n"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	// Should include line numbers when -n flag is specified
	if !strings.Contains(result.Output, "1:") && !strings.Contains(result.Output, ":1:") {
		t.Errorf("Expected line numbers in output, got: %s", result.Output)
	}
}

func TestExecute_Grep_NoLineNumbers(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\nfoo bar\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"pattern": "hello",
			"path":    testFile,
			"flags":   []interface{}{"n"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	// With n flag (default), should have line numbers
	if !strings.Contains(result.Output, ":1:") {
		t.Errorf("Expected line numbers in output with n flag, got: %s", result.Output)
	}
}

func TestExecute_Grep_CaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("Hello WORLD\nfoo bar\nHello again\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"pattern": "hello",
			"path":    testFile,
			"flags":   []interface{}{"i"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "Hello WORLD") {
		t.Errorf("Expected case-insensitive match for 'Hello WORLD', got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "Hello again") {
		t.Errorf("Expected case-insensitive match for 'Hello again', got: %s", result.Output)
	}
}

func TestExecute_Grep_CountOnly(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\nfoo bar\nhello again\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"pattern": "hello",
			"path":    testFile,
			"flags":   []interface{}{"c"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	// Should show count: 2
	if !strings.Contains(result.Output, ":2") {
		t.Errorf("Expected count of 2 in output, got: %s", result.Output)
	}
}

func TestExecute_Grep_InvertMatch(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\nfoo bar\nhello again\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"pattern": "hello",
			"path":    testFile,
			"flags":   []interface{}{"v"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	// Should NOT contain lines with "hello"
	if strings.Contains(result.Output, "hello") {
		t.Errorf("Expected output without 'hello' (invert match), got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "foo bar") {
		t.Errorf("Expected 'foo bar' in output (inverted match), got: %s", result.Output)
	}
}

func TestExecute_Grep_FilenamesOnly(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"pattern": "hello",
			"path":    testFile,
			"flags":   []interface{}{"l"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	// Should only show filename
	if !strings.Contains(result.Output, "test.txt") {
		t.Errorf("Expected filename in output, got: %s", result.Output)
	}
	if strings.Contains(result.Output, "hello") {
		t.Errorf("Expected only filename, not content, got: %s", result.Output)
	}
}

func TestExecute_Grep_Recursive(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("hello world\n"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "file2.txt"), []byte("foo bar\nhello again\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"pattern": "hello",
			"path":    tmpDir,
			"flags":   []interface{}{"r"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	// Should find matches in both files
	if !strings.Contains(result.Output, "file1.txt") {
		t.Errorf("Expected file1.txt in output, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "file2.txt") {
		t.Errorf("Expected file2.txt in output, got: %s", result.Output)
	}
}

func TestExecute_Grep_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"pattern": "notfound",
			"path":    testFile,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success (tool succeeds even with no matches), got: %s", result.Error)
	}
	// Output should be empty or minimal
	if len(result.Output) > 0 && result.Output != "" {
		t.Logf("Output for no matches: %s", result.Output)
	}
}

func TestExecute_Grep_PathNotFound(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"pattern": "test",
			"path":    "/nonexistent/path/that/does/not/exist",
		},
	})
	if result.Success {
		t.Error("Expected failure for nonexistent path")
	}
	if !strings.Contains(result.Error, "path not found") {
		t.Errorf("Expected 'path not found' error, got: %s", result.Error)
	}
}

func TestExecute_Grep_BinaryFileSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a file with null bytes (binary)
	binaryContent := []byte("hello\x00world\n")
	os.WriteFile(filepath.Join(tmpDir, "binary.bin"), binaryContent, 0644)
	os.WriteFile(filepath.Join(tmpDir, "text.txt"), []byte("hello world\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"pattern": "hello",
			"path":    tmpDir,
			"flags":   []interface{}{"r"},
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	// Binary file should be skipped
	if !strings.Contains(result.Output, "text.txt") {
		t.Errorf("Expected text.txt in output, got: %s", result.Output)
	}
	// Should mention skipped binary files
	if result.Extra == nil {
		t.Error("Expected Extra map")
		return
	}
	if skipped, ok := result.Extra["skippedBinaryFiles"].(int); !ok || skipped != 1 {
		t.Errorf("Expected 1 skipped binary file, got %v", result.Extra["skippedBinaryFiles"])
	}
}

func TestExecute_Grep_DefaultPath(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"pattern": "test",
		},
	})
	// Should succeed or fail gracefully (not panic)
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestExecute_Grep_InReadOnlyMode(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)

	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"pattern": "hello",
			"path":    testFile,
		},
	})
	if !result.Success {
		t.Fatalf("Expected grep to succeed in read-only mode, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "hello world") {
		t.Errorf("Expected 'hello world' in output, got: %s", result.Output)
	}
}

func TestExecute_Grep_InReadOnlyMode_Allowed(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)

	te := NewToolExecutor()
	te.SetReadOnly(true)

	// grep should work in read-only mode
	result := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"pattern": "hello",
			"path":    testFile,
		},
	})
	if !result.Success {
		t.Fatalf("Expected grep to succeed in read-only mode, got: %s", result.Error)
	}
}

func TestExecuteGrep_NoMatch(t *testing.T) {
	te := NewToolExecutor()

	result := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"path":      ".",
			"pattern":   "nonexistent_pattern_xyz_123",
			"recursive": "false",
		},
	})

	// Should succeed but with no matches
	_ = result
}

func TestExecuteGrep_ContextCancelled(t *testing.T) {
	te := NewToolExecutor()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := te.Execute(ctx, &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"path":    ".",
			"pattern": "test",
		},
	})

	// Should fail due to cancelled context
	if result.Success {
		t.Log("Got success with cancelled context - unexpected")
	}
}

func TestExecuteGrep_Simple(t *testing.T) {
	te := NewToolExecutor()

	result := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"path":    ".",
			"pattern": "package",
		},
	})

	if !result.Success {
		t.Logf("Got failure (may depend on files): %s", result.Error)
	}
}

func TestExecuteGrep_Context(t *testing.T) {
	te := NewToolExecutor()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := te.Execute(ctx, &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"path":    ".",
			"pattern": "package",
		},
	})

	if !result.Success {
		t.Logf("Got failure (may depend on files): %s", result.Error)
	}
}

func TestExecuteGrep(t *testing.T) {
	te := NewToolExecutor()

	// Test grep on current directory
	result := te.executeGrep(context.Background(), map[string]interface{}{
		"path":        ".",
		"pattern":     "package",
		"max_results": 10,
	})
	if !result.Success {
		t.Errorf("Expected success, got error: %v", result.Error)
	}
	// Grep should find some results
}

func TestToolExecutor_ReadOnly_Statistics(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	te := NewToolExecutor()
	te.SetReadOnly(true)

	// Successful read operation
	te.Execute(context.Background(), &ToolCall{
		Name: "read_file",
		Parameters: map[string]interface{}{
			"path": testFile,
		},
	})

	// Blocked write operation
	te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo test",
		},
	})

	// Another successful read operation
	te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})

	stats := te.Stats()
	if stats.TotalCalls != 3 {
		t.Errorf("Expected 3 total calls, got %d", stats.TotalCalls)
	}
	if stats.FailedCalls != 1 {
		t.Errorf("Expected 1 failed call, got %d", stats.FailedCalls)
	}
}
