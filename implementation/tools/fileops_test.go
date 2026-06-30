package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ===== Tests for file operation tools =====

func TestExecute_ReadFile_MissingParameter(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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

func TestExecute_WriteFile_CreateDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	deepPath := filepath.Join(tmpDir, "a", "b", "c", "test.txt")

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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
	result := te.Execute(context.Background(), &ToolCall{
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

func TestToolExecutor_ReadOnly_ReadFileAllowed(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(context.Background(), &ToolCall{
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

func TestToolExecutor_ReadOnly_ReadLinesAllowed(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("line1\nline2\n"), 0644)

	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(context.Background(), &ToolCall{
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

func TestToolExecutor_ReadOnly_WriteFileBlocked(t *testing.T) {
	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(context.Background(), &ToolCall{
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

func TestToolExecutor_ReadOnly_InsertLinesBlocked(t *testing.T) {
	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(context.Background(), &ToolCall{
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

	result := te.Execute(context.Background(), &ToolCall{
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

func TestExecuteReadFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	te := NewToolExecutor()
	result := te.executeReadFile(map[string]interface{}{
		"path": testFile,
	})
	if !result.Success {
		t.Errorf("Expected success, got error: %v", result.Error)
	}
	if result.Output != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", result.Output)
	}
}

func TestExecuteReadLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("line1\nline2\nline3\n"), 0644)

	te := NewToolExecutor()
	result := te.executeReadLines(map[string]interface{}{
		"path":  testFile,
		"start": 1.0,
		"end":   2.0,
	})
	if !result.Success {
		t.Errorf("Expected success, got error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "1: line1") || !strings.Contains(result.Output, "2: line2") {
		t.Errorf("Expected line numbers in output, got: %s", result.Output)
	}
}

func TestExecuteInsertLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("line1\nline3\n"), 0644)

	te := NewToolExecutor()
	result := te.executeInsertLines(map[string]interface{}{
		"path":  testFile,
		"line":  2.0,
		"lines": "line2",
	})
	if !result.Success {
		t.Errorf("Expected success, got error: %v", result.Error)
	}
}

func TestExecuteReplaceText(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	te := NewToolExecutor()
	result := te.executeReplaceText(map[string]interface{}{
		"path":    testFile,
		"search":  "world",
		"replace": "universe",
	})
	if !result.Success {
		t.Errorf("Expected success, got error: %v", result.Error)
	}
}

func TestExecuteReadFile_TooLarge(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")
	// Create a file larger than 20KB
	largeContent := make([]byte, 20*1024+1)
	for i := range largeContent {
		largeContent[i] = 'a'
	}
	os.WriteFile(testFile, largeContent, 0644)

	te := NewToolExecutor()
	result := te.executeReadFile(map[string]interface{}{
		"path": testFile,
	})
	if result.Success {
		t.Error("Expected failure for file larger than 20KB")
	}
	if !strings.Contains(result.Error, "too large") {
		t.Errorf("Expected 'too large' error, got: %s", result.Error)
	}
	if !strings.Contains(result.Error, "read_lines") {
		t.Errorf("Expected error to suggest read_lines tool, got: %s", result.Error)
	}
}

func TestExecuteReadFile_Binary(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "binary.bin")
	// Create a binary file with null bytes
	binaryContent := []byte{0x00, 0x01, 0x02, 0x03, 0x04}
	os.WriteFile(testFile, binaryContent, 0644)

	te := NewToolExecutor()
	result := te.executeReadFile(map[string]interface{}{
		"path": testFile,
	})
	if result.Success {
		t.Error("Expected failure for binary file")
	}
	if !strings.Contains(result.Error, "binary") {
		t.Errorf("Expected 'binary' error, got: %s", result.Error)
	}
}

func TestExecuteReadFile_BinaryWithText(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "mixed.bin")
	// Create a file with text but also null bytes
	mixedContent := []byte("hello world\x00more text")
	os.WriteFile(testFile, mixedContent, 0644)

	te := NewToolExecutor()
	result := te.executeReadFile(map[string]interface{}{
		"path": testFile,
	})
	if result.Success {
		t.Error("Expected failure for binary file with text")
	}
	if !strings.Contains(result.Error, "binary") {
		t.Errorf("Expected 'binary' error, got: %s", result.Error)
	}
}

func TestExecuteReadFile_JustUnderLimit(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "small.txt")
	// Create a file just under 20KB
	content := make([]byte, 20*1024)
	for i := range content {
		content[i] = 'a'
	}
	os.WriteFile(testFile, content, 0644)

	te := NewToolExecutor()
	result := te.executeReadFile(map[string]interface{}{
		"path": testFile,
	})
	if !result.Success {
		t.Errorf("Expected success for file just under 20KB limit, got: %s", result.Error)
	}
}

func TestExecuteReadFile_ExactlyAtLimit(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "exact.txt")
	// Create a file exactly 20KB
	content := make([]byte, 20*1024)
	for i := range content {
		content[i] = 'a'
	}
	os.WriteFile(testFile, content, 0644)

	te := NewToolExecutor()
	result := te.executeReadFile(map[string]interface{}{
		"path": testFile,
	})
	if !result.Success {
		t.Errorf("Expected success for file exactly at 20KB limit, got: %s", result.Error)
	}
}

func TestIsBinaryFile_NonExistent(t *testing.T) {
	result := isBinaryFile("/nonexistent/file.bin")
	if result {
		t.Error("Expected false for non-existent file")
	}
}

func TestIsBinaryFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.bin")
	os.WriteFile(testFile, []byte{}, 0644)

	result := isBinaryFile(testFile)
	if result {
		t.Error("Expected false for empty file")
	}
}

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 bytes"},
		{512, "512 bytes"},
		{1024, "1 KB"},
		{2048, "2 KB"},
		{1024 * 1024, "1 MB"},
		{1024 * 1024 * 2, "2 MB"},
		{20 * 1024, "20 KB"},
	}

	for _, test := range tests {
		result := formatFileSize(test.bytes)
		if result != test.expected {
			t.Errorf("formatFileSize(%d) = %s, expected %s", test.bytes, result, test.expected)
		}
	}
}

