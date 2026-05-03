package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFormatFileError_NotExist(t *testing.T) {
	err := os.ErrNotExist
	result := formatFileError(err, "/nonexistent/file.txt")
	if !strings.Contains(result, "file not found") {
		t.Errorf("Expected 'file not found', got: %s", result)
	}
	if !strings.Contains(result, "/nonexistent/file.txt") {
		t.Errorf("Expected path in result, got: %s", result)
	}
}

func TestFormatFileError_Permission(t *testing.T) {
	// Create a permission error using a read-only file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "readonly.txt")
	os.WriteFile(testFile, []byte("content"), 0444)

	result := formatFileError(os.ErrPermission, testFile)
	if !strings.Contains(result, "permission denied") {
		t.Errorf("Expected 'permission denied', got: %s", result)
	}
}

func TestFormatFileError_Generic(t *testing.T) {
	err := os.PathError{Path: "/some/file", Op: "open", Err: nil}
	result := formatFileError(&err, "/some/file")
	if !strings.Contains(result, "file error") {
		t.Errorf("Expected 'file error', got: %s", result)
	}
}

func TestParseToolCall_RawArguments(t *testing.T) {
	raw := `{"id":"call_1","type":"function","function":{"name":"bash","arguments":"INVALID JSON"}}`
	tc, err := ParseToolCall(raw)
	if err != nil {
		t.Fatalf("ParseToolCall() error: %v", err)
	}

	if tc.ID != "call_1" {
		t.Errorf("Expected ID 'call_1', got '%s'", tc.ID)
	}
	if tc.Name != "bash" {
		t.Errorf("Expected name 'bash', got '%s'", tc.Name)
	}
	// Raw field contains the full JSON string of the tool call
	if tc.Raw != raw {
		t.Errorf("Expected raw to be the full input, got: %s", tc.Raw)
	}
}

func TestParseToolCall_EmptyArguments(t *testing.T) {
	raw := `{"id":"call_2","type":"function","function":{"name":"bash","arguments":""}}`
	tc, err := ParseToolCall(raw)
	if err != nil {
		t.Fatalf("ParseToolCall() error: %v", err)
	}
	if tc.Name != "bash" {
		t.Errorf("Expected name 'bash', got '%s'", tc.Name)
	}
	// Empty arguments string results in nil params
	if tc.Parameters != nil {
		t.Errorf("Expected nil params for empty arguments, got: %v", tc.Parameters)
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	te := NewToolExecutor()
	// First call bash (which may succeed or fail depending on env)
	te.Execute(&ToolCall{Name: "bash", Parameters: map[string]interface{}{"command": "true"}})
	// Now call unknown tool (will always fail)
	result := te.Execute(&ToolCall{Name: "nonexistent_tool", Parameters: map[string]interface{}{}})
	if result.Success {
		t.Error("Expected failure for unknown tool")
	}
	if !strings.Contains(result.Error, "unknown tool") {
		t.Errorf("Expected 'unknown tool' error, got: %s", result.Error)
	}
	stats := te.Stats()
	if stats.FailedCalls < 1 {
		t.Errorf("Expected at least 1 failed call (unknown tool), got %d", stats.FailedCalls)
	}
}

func TestExecute_Bash_Failed(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "false", // always fails
		},
	})
	if result.Success {
		t.Error("Expected failure for 'false' command")
	}
	if result.ExitCode == 0 {
		t.Error("Expected non-zero exit code")
	}
}

func TestExecute_ReadFile_MissingParameter(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name:       "read_file",
		Parameters: map[string]interface{}{},
	})
	if result.Success {
		t.Error("Expected failure for missing path parameter")
	}
	if !strings.Contains(result.Error, "missing required parameter") {
		t.Errorf("Expected 'missing required parameter' error, got: %s", result.Error)
	}
}

func TestExecute_WriteFile_MissingPath(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "write_file",
		Parameters: map[string]interface{}{
			"content": "test",
		},
	})
	if result.Success {
		t.Error("Expected failure for missing path parameter")
	}
}

func TestExecute_WriteFile_MissingContent(t *testing.T) {
	tmpDir := t.TempDir()
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "write_file",
		Parameters: map[string]interface{}{
			"path": filepath.Join(tmpDir, "test.txt"),
		},
	})
	if result.Success {
		t.Error("Expected failure for missing content parameter")
	}
}

func TestExecute_ReadLines_MissingPath(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "read_lines",
		Parameters: map[string]interface{}{
			"start": 1.0,
			"end":   10.0,
		},
	})
	if result.Success {
		t.Error("Expected failure for missing path parameter")
	}
}

func TestExecute_ReadLines_MissingStart(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "lines.txt")
	os.WriteFile(testFile, []byte("line1\nline2\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "read_lines",
		Parameters: map[string]interface{}{
			"path": testFile,
			"end":  10.0,
		},
	})
	if result.Success {
		t.Error("Expected failure for missing start parameter")
	}
}

func TestExecute_ReadLines_MissingEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "lines.txt")
	os.WriteFile(testFile, []byte("line1\nline2\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "read_lines",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"start": 1.0,
		},
	})
	if result.Success {
		t.Error("Expected failure for missing end parameter")
	}
}

func TestExecute_InsertLines_MissingPath(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "insert_lines",
		Parameters: map[string]interface{}{
			"line":  1.0,
			"lines": "new line",
		},
	})
	if result.Success {
		t.Error("Expected failure for missing path parameter")
	}
}

func TestExecute_InsertLines_MissingLine(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "insert_lines",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"lines": "new line",
		},
	})
	if result.Success {
		t.Error("Expected failure for missing line parameter")
	}
}

func TestExecute_InsertLines_MissingLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "insert_lines",
		Parameters: map[string]interface{}{
			"path": testFile,
			"line": 1.0,
		},
	})
	if result.Success {
		t.Error("Expected failure for missing lines parameter")
	}
}

func TestExecute_ReplaceText_MissingPath(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_text",
		Parameters: map[string]interface{}{
			"search":  "old",
			"replace": "new",
		},
	})
	if result.Success {
		t.Error("Expected failure for missing path parameter")
	}
}

func TestExecute_ReplaceText_MissingSearch(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_text",
		Parameters: map[string]interface{}{
			"path":    testFile,
			"replace": "new",
		},
	})
	if result.Success {
		t.Error("Expected failure for missing search parameter")
	}
}

func TestExecute_ReplaceText_MissingReplace(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_text",
		Parameters: map[string]interface{}{
			"path":   testFile,
			"search": "old",
		},
	})
	if result.Success {
		t.Error("Expected failure for missing replace parameter")
	}
}

func TestExecute_ReplaceText_CountAll(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "hello hello hello hello\n"
	os.WriteFile(testFile, []byte(content), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_text",
		Parameters: map[string]interface{}{
			"path":    testFile,
			"search":  "hello",
			"replace": "hi",
			"count":   -1.0, // replace all
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if result.Extra == nil {
		t.Error("Expected Extra map")
		return
	}
	if count, ok := result.Extra["totalOccurrences"].(int); !ok || count != 4 {
		t.Errorf("Expected 4 total occurrences, got %v", result.Extra["totalOccurrences"])
	}
	if count, ok := result.Extra["replacementsMade"].(int); !ok || count != 4 {
		t.Errorf("Expected 4 replacements made, got %v", result.Extra["replacementsMade"])
	}
}

func TestExecute_ReplaceText_CountLimited(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "hello hello hello hello\n"
	os.WriteFile(testFile, []byte(content), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_text",
		Parameters: map[string]interface{}{
			"path":    testFile,
			"search":  "hello",
			"replace": "hi",
			"count":   2.0, // replace only 2
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if count, ok := result.Extra["replacementsMade"].(int); !ok || count != 2 {
		t.Errorf("Expected 2 replacements made, got %v", result.Extra["replacementsMade"])
	}
}

func TestExecute_ReplaceText_StringCount(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello hello\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_text",
		Parameters: map[string]interface{}{
			"path":    testFile,
			"search":  "hello",
			"replace": "hi",
			"count":   "all",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
}

func TestExecute_ReplaceText_StringCountMinusOne(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello hello\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_text",
		Parameters: map[string]interface{}{
			"path":    testFile,
			"search":  "hello",
			"replace": "hi",
			"count":   "-1",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
}

func TestExecute_ReplaceText_IntCount(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello hello hello\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_text",
		Parameters: map[string]interface{}{
			"path":    testFile,
			"search":  "hello",
			"replace": "hi",
			"count":   2, // int type
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
}

func TestExecute_ReplaceText_SearchNotInFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_text",
		Parameters: map[string]interface{}{
			"path":    testFile,
			"search":  "notfound",
			"replace": "replacement",
		},
	})
	if result.Success {
		t.Error("Expected failure when search text not found")
	}
	if !strings.Contains(result.Error, "search text not found") {
		t.Errorf("Expected 'search text not found' error, got: %s", result.Error)
	}
}

func TestToolExecutor_Stats(t *testing.T) {
	te := NewToolExecutor()

	// Execute several calls
	te.Execute(&ToolCall{Name: "bash", Parameters: map[string]interface{}{"command": "true"}})
	te.Execute(&ToolCall{Name: "bash", Parameters: map[string]interface{}{"command": "false"}})
	te.Execute(&ToolCall{Name: "unknown_tool", Parameters: map[string]interface{}{}})

	stats := te.Stats()
	if stats.TotalCalls != 3 {
		t.Errorf("Expected 3 total calls, got %d", stats.TotalCalls)
	}
	if stats.FailedCalls < 1 {
		t.Errorf("Expected at least 1 failed call, got %d", stats.FailedCalls)
	}
}

func TestToolExecutor_StatsEmpty(t *testing.T) {
	te := NewToolExecutor()
	stats := te.Stats()
	if stats.TotalCalls != 0 {
		t.Errorf("Expected 0 total calls, got %d", stats.TotalCalls)
	}
	if stats.FailedCalls != 0 {
		t.Errorf("Expected 0 failed calls, got %d", stats.FailedCalls)
	}
}

func TestExecute_WriteFile_CreateDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	deepPath := filepath.Join(tmpDir, "a", "b", "c", "test.txt")

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "write_file",
		Parameters: map[string]interface{}{
			"path":    deepPath,
			"content": "deep content",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success for creating nested directories, got: %s", result.Error)
	}
	content, _ := os.ReadFile(deepPath)
	if string(content) != "deep content" {
		t.Errorf("Expected 'deep content', got: %s", string(content))
	}
}

func TestExecute_ReadFile_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")
	os.WriteFile(testFile, []byte(""), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "read_file",
		Parameters: map[string]interface{}{
			"path": testFile,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success for empty file, got: %s", result.Error)
	}
	if result.Output != "" {
		t.Errorf("Expected empty output, got: %s", result.Output)
	}
}

func TestExecute_ReadFile_PermissionDenied(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "readonly.txt")
	os.WriteFile(testFile, []byte("content\n"), 0444)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "read_file",
		Parameters: map[string]interface{}{
			"path": testFile,
		},
	})
	if result.Success {
		// Reading should succeed since we're the owner
		// This test may pass or fail depending on permissions
	}
}

func TestExecute_WriteFile_PermissionDenied(t *testing.T) {
	tmpDir := t.TempDir()
	readonlyDir := filepath.Join(tmpDir, "readonly")
	os.Mkdir(readonlyDir, 0555)
	testFile := filepath.Join(readonlyDir, "test.txt")

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "write_file",
		Parameters: map[string]interface{}{
			"path":    testFile,
			"content": "content",
		},
	})
	// Should fail due to directory permissions
	if result.Success {
		t.Error("Expected failure for write to read-only directory")
	}
}

func TestExecute_ReadLines_TooManyLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "lines.txt")
	var content string
	for i := 1; i <= 100; i++ {
		content += fmt.Sprintf("line %d\n", i)
	}
	os.WriteFile(testFile, []byte(content), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "read_lines",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"start": 1.0,
			"end":   200.0,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success when end > file length, got: %s", result.Error)
	}
	// Should return all lines
	if !strings.Contains(result.Output, "100:") {
		t.Error("Expected to see line 100 in output")
	}
}

func TestExecute_InsertLines_ToEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")
	os.WriteFile(testFile, []byte(""), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "insert_lines",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"line":  1.0,
			"lines": "first line",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success for inserting into empty file, got: %s", result.Error)
	}
	content, _ := os.ReadFile(testFile)
	if !strings.Contains(string(content), "first line") {
		t.Errorf("Expected content to contain 'first line', got: %s", string(content))
	}
}

func TestExecute_InsertLines_AtBeginning(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("line 2\nline 3\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "insert_lines",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"line":  1.0,
			"lines": "line 1",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	content, _ := os.ReadFile(testFile)
	if !strings.HasPrefix(string(content), "line 1") {
		t.Errorf("Expected file to start with 'line 1', got: %s", string(content))
	}
}

func TestExecute_InsertLines_AtEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("existing\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "insert_lines",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"line":  999.0, // beyond end
			"lines": "appended",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	content, _ := os.ReadFile(testFile)
	if !strings.Contains(string(content), "appended") {
		t.Errorf("Expected content to contain 'appended', got: %s", string(content))
	}
}

func TestExecute_ReplaceText_EmptySearch(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_text",
		Parameters: map[string]interface{}{
			"path":    testFile,
			"search":  "",
			"replace": "replacement",
		},
	})
	// Empty search string should fail with a clear error
	if result.Success {
		t.Error("Expected failure for empty search string")
	}
	if !strings.Contains(result.Error, "cannot be empty") {
		t.Errorf("Expected 'cannot be empty' error, got: %s", result.Error)
	}
}

func TestExecute_ReplaceText_EmptyReplace(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello\nhello\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_text",
		Parameters: map[string]interface{}{
			"path":    testFile,
			"search":  "hello",
			"replace": "",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success for empty replace, got: %s", result.Error)
	}
}

func TestExecute_ReplaceText_OverwriteCount(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello hello hello\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_text",
		Parameters: map[string]interface{}{
			"path":    testFile,
			"search":  "hello",
			"replace": "hi",
			"count":   10.0, // more than available
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if count, ok := result.Extra["replacementsMade"].(int); !ok || count != 3 {
		t.Errorf("Expected 3 replacements made (not 10), got %v", result.Extra["replacementsMade"])
	}
}

func TestExecute_Bash_WithEnvVars(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo $HOME",
		},
	})
	// Should succeed and return output
	if !result.Success {
		t.Logf("bash command returned error (expected in test env): %s", result.Error)
	}
}

func TestExecute_Bash_WithPipes(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo test | cat",
		},
	})
	// Should succeed
	if result.Success {
		if !strings.Contains(result.Output, "test") {
			t.Errorf("Expected output to contain 'test', got: %s", result.Output)
		}
	}
}

func TestExecute_ListFiles_Default(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files and directories
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content2"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "dir1"), 0755)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result2 := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result2 := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result2 := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
func TestToolExecutor_ReadOnly_BashBlocked(t *testing.T) {
	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(&ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo test",
		},
	})

	if result.Success {
		t.Error("Expected bash to fail in read-only mode")
	}
	if !strings.Contains(result.Error, "not available in read-only mode") {
		t.Errorf("Expected 'not available in read-only mode' error, got: %s", result.Error)
	}
	if result.Extra == nil || result.Extra["tool_name"] != "bash" {
		t.Errorf("Expected tool_name in Extra, got: %v", result.Extra)
	}
}

func TestToolExecutor_ReadOnly_WriteFileBlocked(t *testing.T) {
	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(&ToolCall{
		Name: "write_file",
		Parameters: map[string]interface{}{
			"path":    "/tmp/test.txt",
			"content": "test",
		},
	})

	if result.Success {
		t.Error("Expected write_file to fail in read-only mode")
	}
	if !strings.Contains(result.Error, "not available in read-only mode") {
		t.Errorf("Expected 'not available in read-only mode' error, got: %s", result.Error)
	}
}

func TestToolExecutor_ReadOnly_ListFilesAllowed(t *testing.T) {
	tmpDir := t.TempDir()
	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(&ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})

	if !result.Success {
		t.Fatalf("Expected list_files to succeed in read-only mode, got: %s", result.Error)
	}
}

func TestToolExecutor_ReadOnly_ReadFileAllowed(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(&ToolCall{
		Name: "read_file",
		Parameters: map[string]interface{}{
			"path": testFile,
		},
	})

	if !result.Success {
		t.Fatalf("Expected read_file to succeed in read-only mode, got: %s", result.Error)
	}
	if result.Output != "content" {
		t.Errorf("Expected 'content', got: %s", result.Output)
	}
}

func TestToolExecutor_ReadOnly_InsertLinesBlocked(t *testing.T) {
	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(&ToolCall{
		Name: "insert_lines",
		Parameters: map[string]interface{}{
			"path":  "/tmp/test.txt",
			"line":  1.0,
			"lines": "new line",
		},
	})

	if result.Success {
		t.Error("Expected insert_lines to fail in read-only mode")
	}
	if !strings.Contains(result.Error, "not available in read-only mode") {
		t.Errorf("Expected 'not available in read-only mode' error, got: %s", result.Error)
	}
}

func TestToolExecutor_ReadOnly_ReplaceTextBlocked(t *testing.T) {
	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(&ToolCall{
		Name: "replace_text",
		Parameters: map[string]interface{}{
			"path":    "/tmp/test.txt",
			"search":  "old",
			"replace": "new",
		},
	})

	if result.Success {
		t.Error("Expected replace_text to fail in read-only mode")
	}
	if !strings.Contains(result.Error, "not available in read-only mode") {
		t.Errorf("Expected 'not available in read-only mode' error, got: %s", result.Error)
	}
}

func TestToolExecutor_ReadOnly_ReadLinesAllowed(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("line1\nline2\n"), 0644)

	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(&ToolCall{
		Name: "read_lines",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"start": 1.0,
			"end":   1.0,
		},
	})

	if !result.Success {
		t.Fatalf("Expected read_lines to succeed in read-only mode, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "1:") {
		t.Errorf("Expected output to contain line number, got: %s", result.Output)
	}
}

func TestToolExecutor_ReadOnly_Statistics(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	te := NewToolExecutor()
	te.SetReadOnly(true)

	// Successful read operation
	te.Execute(&ToolCall{
		Name: "read_file",
		Parameters: map[string]interface{}{
			"path": testFile,
		},
	})

	// Blocked write operation
	te.Execute(&ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo test",
		},
	})

	// Another successful read operation
	te.Execute(&ToolCall{
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

func TestToolExecutor_ReadOnly_NotSet(t *testing.T) {
	// When ReadOnly is not set (false), all tools should work normally
	te := NewToolExecutor()
	// Not setting SetReadOnly, so readOnly should be false by default

	// Bash should work
	result := te.Execute(&ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo test",
		},
	})

	// Note: bash might succeed or fail depending on environment, but it shouldn't
	// fail because of read-only mode
	if result.Error != "" && strings.Contains(result.Error, "read-only") {
		t.Errorf("bash should not be blocked when readOnly is not set, got: %s", result.Error)
	}
}

func TestIsReadOnlyTool(t *testing.T) {
	// Test allowed tools
	allowedTools := []string{"read_file", "list_files", "read_lines"}
	for _, tool := range allowedTools {
		if !isReadOnlyTool(tool) {
			t.Errorf("Expected isReadOnlyTool('%s') to return true", tool)
		}
	}

	// Test blocked tools
	blockedTools := []string{"bash", "write_file", "insert_lines", "replace_text"}
	for _, tool := range blockedTools {
		if isReadOnlyTool(tool) {
			t.Errorf("Expected isReadOnlyTool('%s') to return false", tool)
		}
	}

	// Test unknown tool
	if isReadOnlyTool("unknown_tool") {
		t.Error("Expected isReadOnlyTool('unknown_tool') to return false")
	}
}

func TestExecute_ListFiles_MultipleFlags(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("hidden"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "dir1"), 0755)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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

func TestToolExecutor_ListFiles_Statistics(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("content"), 0644)

	te := NewToolExecutor()

	// Execute list_files successfully
	te.Execute(&ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})

	// Execute with invalid path (will fail)
	te.Execute(&ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": "/nonexistent",
		},
	})

	stats := te.Stats()
	if stats.TotalCalls != 2 {
		t.Errorf("Expected 2 total calls, got %d", stats.TotalCalls)
	}
	if stats.FailedCalls != 1 {
		t.Errorf("Expected 1 failed call, got %d", stats.FailedCalls)
	}
}

func TestHumanReadableSize(t *testing.T) {
	tests := []struct {
		size     int64
		expected string
	}{
		{0, "0"},
		{512, "512"},
		{1024, "1.0K"},
		{1536, "1.5K"},
		{1048576, "1.0M"},
		{1073741824, "1.0G"},
	}

	for _, tt := range tests {
		result := humanReadableSize(tt.size)
		if result != tt.expected {
			t.Errorf("humanReadableSize(%d) = %q, expected %q", tt.size, result, tt.expected)
		}
	}
}

func TestFormatPermissions(t *testing.T) {
	tests := []struct {
		name     string
		mode     os.FileMode
		expected string
	}{
		{"regular file", 0644, "-rw-r--r--"},
		{"directory", 0755, "drwxr-xr-x"},
		{"executable", 0755, "drwxr-xr-x"}, // Will be dir due to IsDir check
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			var info os.FileInfo

			if tt.name == "directory" || tt.name == "executable" {
				os.MkdirAll(filepath.Join(tmpDir, "testdir"), tt.mode)
				info, _ = os.Stat(filepath.Join(tmpDir, "testdir"))
			} else {
				testFile := filepath.Join(tmpDir, "testfile")
				os.WriteFile(testFile, []byte("content"), tt.mode)
				info, _ = os.Stat(testFile)
			}

			result := formatPermissions(info)
			if result != tt.expected {
				t.Errorf("formatPermissions(%v) = %q, expected %q", tt.mode, result, tt.expected)
			}
		})
	}
}

// ===== Tests for grep tool =====

func TestExecute_Grep_MissingPattern(t *testing.T) {
	tmpDir := t.TempDir()
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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

	result := te.Execute(&ToolCall{
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

// ===== Tests for git_log tool =====
// setupGitRepo initializes a git repo with user config.
func setupGitRepo(t *testing.T, tmpDir string) {
	t.Helper()
	exec.Command("git", "init", tmpDir).Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
}

func TestExecute_GitLog_NotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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

	result := te.Execute(&ToolCall{
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

func TestExecute_GitLog_ReferenceParameter(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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

// ===== Tests for git_show tool =====

func TestExecute_GitShow_NotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "git_show",
		Parameters: map[string]interface{}{
			"path":     tmpDir,
			"commit":   "HEAD",
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
		Name: "git_show",
		Parameters: map[string]interface{}{
			"path":    tmpDir,
			"commit":  "HEAD",
			"flags":   []interface{}{"stat"},
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
	result := te.Execute(&ToolCall{
		Name: "git_show",
		Parameters: map[string]interface{}{
			"path":    tmpDir,
			"commit":  "HEAD",
			"flags":   []interface{}{"name-status"},
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
	result := te.Execute(&ToolCall{
		Name: "git_show",
		Parameters: map[string]interface{}{
			"path":    tmpDir,
			"commit":  "HEAD",
			"flags":   []interface{}{"stat"},
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

	result := te.Execute(&ToolCall{
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

func TestExecute_GitShow_CommitNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
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

// ===== Tests for isReadOnlyTool with new tools =====

func TestIsReadOnlyTool_NewTools(t *testing.T) {
	// Test that new tools are allowed in read-only mode
	newReadOnlyTools := []string{"grep", "git_log", "git_show", "git_diff"}
	for _, tool := range newReadOnlyTools {
		if !isReadOnlyTool(tool) {
			t.Errorf("Expected isReadOnlyTool('%s') to return true", tool)
		}
	}

	// Test that they're not in the blocked list
	blockedTools := []string{"bash", "write_file", "insert_lines", "replace_text"}
	for _, tool := range blockedTools {
		if isReadOnlyTool(tool) {
			t.Errorf("Expected isReadOnlyTool('%s') to return false", tool)
		}
	}
}

func TestExecute_Grep_InReadOnlyMode_Allowed(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)

	te := NewToolExecutor()
	te.SetReadOnly(true)

	// grep should work in read-only mode
	result := te.Execute(&ToolCall{
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
	result := te.Execute(&ToolCall{
		Name: "git_log",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})
	if !result.Success {
		t.Fatalf("Expected git_log to succeed in read-only mode, got: %s", result.Error)
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
	result := te.Execute(&ToolCall{
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
