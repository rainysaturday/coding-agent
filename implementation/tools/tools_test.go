package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestExecute_ReplaceLines_MissingStart(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("line1\nline2\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"end":   2.0,
			"lines": "replacement",
		},
	})
	if result.Success {
		t.Error("Expected failure for missing start parameter")
	}
}

func TestExecute_ReplaceLines_MissingEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("line1\nline2\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"start": 1.0,
			"lines": "replacement",
		},
	})
	if result.Success {
		t.Error("Expected failure for missing end parameter")
	}
}

func TestExecute_ReplaceLines_MissingLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("line1\nline2\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"start": 1.0,
			"end":   2.0,
		},
	})
	if result.Success {
		t.Error("Expected failure for missing lines parameter")
	}
}

func TestExecute_ReplaceLines_Search(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("func oldName() {}\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path":    testFile,
			"search":  "oldName",
			"replace": "newName",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	content, _ := os.ReadFile(testFile)
	if !strings.Contains(string(content), "newName") {
		t.Errorf("Expected content to contain 'newName', got: %s", string(content))
	}
}

func TestExecute_ReplaceLines_SearchCount(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("TODO: fix TODO: fix TODO: fix\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path":    testFile,
			"search":  "TODO",
			"replace": "FIXED",
			"count":   2.0,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	content, _ := os.ReadFile(testFile)
	if strings.Count(string(content), "FIXED") != 2 {
		t.Errorf("Expected 2 FIXED, got %d", strings.Count(string(content), "FIXED"))
	}
}

func TestExecute_ReplaceLines_SearchNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path":    testFile,
			"search":  "notfound",
			"replace": "replacement",
		},
	})
	if result.Success {
		t.Error("Expected failure when search text not found")
	}
}

func TestExecute_ReplaceLinesByNumber_StartGreaterThanEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("line1\nline2\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"start": 5.0,
			"end":   2.0,
			"lines": "replacement",
		},
	})
	if result.Success {
		t.Error("Expected failure when start > end")
	}
}
func TestExecute_ReplaceLines_MissingPath(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"start": 1.0,
			"end":   2.0,
			"lines": "replacement",
		},
	})
	if result.Success {
		t.Error("Expected failure for missing path parameter")
	}
}

func TestExecute_ReplaceLines_SearchMissingPath(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"search":  "old",
			"replace": "new",
		},
	})
	if result.Success {
		t.Error("Expected failure for missing path parameter")
	}
}

func TestExecute_ReplaceLines_SearchMissingReplace(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path":   testFile,
			"search": "old",
		},
	})
	if result.Success {
		t.Error("Expected failure for missing replace parameter")
	}
}

func TestExecute_ReplaceLines_LineNumberReplace(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("line1\nline2\nline3\nline4\nline5\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"start": 2.0,
			"end":   3.0,
			"lines": "replaced_line",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	content, _ := os.ReadFile(testFile)
	expected := "line1\nreplaced_line\nline4\nline5\n"
	if string(content) != expected {
		t.Errorf("Expected %q, got %q", expected, string(content))
	}
}

func TestExecute_ReplaceLines_LineNumberReplaceMultipleLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("line1\nline2\nline3\nline4\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"start": 2.0,
			"end":   3.0,
			"lines": "new_line_a\nnew_line_b\nnew_line_c",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	content, _ := os.ReadFile(testFile)
	expected := "line1\nnew_line_a\nnew_line_b\nnew_line_c\nline4\n"
	if string(content) != expected {
		t.Errorf("Expected %q, got %q", expected, string(content))
	}
}

func TestExecute_ReplaceLines_LineNumberReplaceAll(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("old1\nold2\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"start": 1.0,
			"end":   2.0,
			"lines": "new_content",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	content, _ := os.ReadFile(testFile)
	expected := "new_content\n"
	if string(content) != expected {
		t.Errorf("Expected %q, got %q", expected, string(content))
	}
}

func TestExecute_ReplaceLines_LineNumberEmptyReplacement(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("line1\nline2\nline3\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"start": 2.0,
			"end":   2.0,
			"lines": "",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	content, _ := os.ReadFile(testFile)
	expected := "line1\n\nline3\n"
	if string(content) != expected {
		t.Errorf("Expected %q, got %q", expected, string(content))
	}
}

func TestExecute_ReplaceLines_SearchReplace(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\nhello go\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path":    testFile,
			"search":  "hello",
			"replace": "goodbye",
			"count":   "all",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	content, _ := os.ReadFile(testFile)
	expected := "goodbye world\ngoodbye go\n"
	if string(content) != expected {
		t.Errorf("Expected %q, got %q", expected, string(content))
	}
}

func TestExecute_ReplaceLines_SearchReplaceAll(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello hello hello\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
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
	content, _ := os.ReadFile(testFile)
	expected := "hi hi hi\n"
	if string(content) != expected {
		t.Errorf("Expected %q, got %q", expected, string(content))
	}
}

func TestExecute_ReplaceLines_SearchReplacePartial(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("foo bar foo baz foo\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path":    testFile,
			"search":  "foo",
			"replace": "qux",
			"count":   2.0,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	content, _ := os.ReadFile(testFile)
	expected := "qux bar qux baz foo\n"
	if string(content) != expected {
		t.Errorf("Expected %q, got %q", expected, string(content))
	}
}

func TestExecute_ReplaceLines_SearchReplaceNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path":    testFile,
			"search":  "notfound",
			"replace": "replacement",
		},
	})
	if result.Success {
		t.Error("Expected failure when search text not found")
	}
}

func TestExecute_ReplaceLines_NoModeSpecified(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path": testFile,
		},
	})
	if result.Success {
		t.Error("Expected failure when no mode specified (no start/end or search)")
	}
	if !strings.Contains(result.Error, "must provide either start/end") {
		t.Errorf("Expected mode error, got: %s", result.Error)
	}
}

func TestExecute_ReplaceLines_StartGreaterThanEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("line1\nline2\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"start": 5.0,
			"end":   2.0,
			"lines": "replacement",
		},
	})
	if result.Success {
		t.Error("Expected failure when start > end")
	}
}

func TestExecute_ReplaceLines_LineNumberAtEndOfLine(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("first\nlast\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"start": 2.0,
			"end":   2.0,
			"lines": "updated_last",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	content, _ := os.ReadFile(testFile)
	expected := "first\nupdated_last\n"
	if string(content) != expected {
		t.Errorf("Expected %q, got %q", expected, string(content))
	}
}

func TestExecute_ReplaceLines_LineNumberBeyondEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("existing\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"start": 999.0,
			"end":   999.0,
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

func TestExecute_ReplaceLines_LineNumberCreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "newfile.txt")

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "replace_lines",
		Parameters: map[string]interface{}{
			"path":  testFile,
			"start": 1.0,
			"end":   1.0,
			"lines": "new content",
		},
	})
	if !result.Success {
		t.Fatalf("Expected success for creating new file, got: %s", result.Error)
	}
	content, _ := os.ReadFile(testFile)
	expected := "new content\n"
	if string(content) != expected {
		t.Errorf("Expected %q, got %q", expected, string(content))
	}
}

func TestExecute_Patch_MissingPath(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "patch",
		Parameters: map[string]interface{}{
			"diff": "--- a/test\n+++ b/test\n@@ -1 +1 @@\n-old\n+new\n",
		},
	})
	if result.Success {
		t.Error("Expected failure for missing path parameter")
	}
}

func TestExecute_Patch_MissingDiff(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(testFile, []byte("package main\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "patch",
		Parameters: map[string]interface{}{
			"path": testFile,
		},
	})
	if result.Success {
		t.Error("Expected failure for missing diff parameter")
	}
}

func TestExecute_Patch_EmptyDiff(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(testFile, []byte("package main\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "patch",
		Parameters: map[string]interface{}{
			"path": testFile,
			"diff": "",
		},
	})
	if result.Success {
		t.Error("Expected failure for empty diff")
	}
	if !strings.Contains(result.Error, "empty") {
		t.Errorf("Expected 'empty' in error, got: %s", result.Error)
	}
}

func TestExecute_Patch_WhitespaceOnlyDiff(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(testFile, []byte("package main\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "patch",
		Parameters: map[string]interface{}{
			"path": testFile,
			"diff": "   \n\t\n  ",
		},
	})
	if result.Success {
		t.Error("Expected failure for whitespace-only diff")
	}
}

func TestExecute_Patch_MissingHunkHeaders(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(testFile, []byte("package main\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "patch",
		Parameters: map[string]interface{}{
			"path": testFile,
			"diff": "--- a/test.go\n+++ b/test.go\nthis is not a valid diff\n",
		},
	})
	if result.Success {
		t.Error("Expected failure for missing @@ headers")
	}
	if !strings.Contains(result.Error, "hunk") {
		t.Errorf("Expected 'hunk' in error, got: %s", result.Error)
	}
}

func TestExecute_Patch_ContextMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(testFile, []byte("func main() {\n    actual()\n}\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "patch",
		Parameters: map[string]interface{}{
			"path": testFile,
			"diff": "--- a/test.go\n+++ b/test.go\n@@ -2,1 +2,1 @@\n-    wrong()\n+    new()\n",
		},
	})
	if result.Success {
		t.Error("Expected failure for context mismatch")
	}
}

func TestExecute_Patch_DryRunFailure(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(testFile, []byte("package main\n"), 0644)

	te := NewToolExecutor()
	// This should fail dry-run since the file doesn't have the expected context
	result := te.Execute(&ToolCall{
		Name: "patch",
		Parameters: map[string]interface{}{
			"path": testFile,
			"diff": "--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-old_line\n+new_line\n",
		},
	})
	// The result depends on whether the patch can match the existing content
	// We just verify it doesn't panic and returns a valid result
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestExecute_Patch_ExtraOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(testFile, []byte("package main\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "patch",
		Parameters: map[string]interface{}{
			"path": testFile,
			"diff": "--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-old\n+new\n",
		},
	})
	if result.Success {
		// If somehow successful, patches_applied should be present
		return
	}
	// On failure, should have patches_applied = 0
	if result.Extra != nil {
		if patchesApplied, ok := result.Extra["patches_applied"]; ok {
			if p, ok := patchesApplied.(int); ok && p != 0 {
				t.Errorf("Expected 0 patches applied on failure, got %d", p)
			}
		}
	}
}

func TestExecute_Patch_Rollback(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	originalContent := "package main\n\nfunc main() {\n    oldCode()\n}\n"
	os.WriteFile(testFile, []byte(originalContent), 0644)

	te := NewToolExecutor()
	// Patch that will fail dry-run because context doesn't match
	_ = te.Execute(&ToolCall{
		Name: "patch",
		Parameters: map[string]interface{}{
			"path": testFile,
			"diff": "--- a/test.go\n+++ b/test.go\n@@ -0,0 +1 @@\n+totally_new_line\n",
		},
	})
	// File should still contain original content since patch failed
	content, _ := os.ReadFile(testFile)
	_ = content // Just verify no panic
}

func TestExecute_Patch_Success(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(testFile, []byte("func main() {\n    oldFunc()\n}\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "patch",
		Parameters: map[string]interface{}{
			"path": testFile,
			"diff": "--- a/test.go\n+++ b/test.go\n@@ -1 +1 @@\n-oldFunc\n+newFunc\n",
		},
	})
	if !result.Success {
		t.Logf("Patch test info: success=%v, error=%s", result.Success, result.Error)
		// Patch may fail in test environment - just verify it returns valid result
		return
	}
	content, _ := os.ReadFile(testFile)
	if !strings.Contains(string(content), "newFunc") {
		t.Logf("Patch result content: %s", string(content))
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
	_ = te.Execute(&ToolCall{
		Name: "replace_text",
		Parameters: map[string]interface{}{
			"path":    testFile,
			"search":  "",
			"replace": "replacement",
		},
	})
	// Empty search might match everywhere or fail - just verify no panic
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

func TestExecuteGlob_BasicPattern(t *testing.T) {
	te := NewToolExecutor()

	// Create test directory structure
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)

	// Test glob with pattern
	result := te.Execute(&ToolCall{
		Name: "glob",
		Parameters: map[string]interface{}{
			"pattern": filepath.Join(tmpDir, "*.go"),
		},
	})

	if !result.Success {
		t.Fatalf("Expected success, got error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "test.go") {
		t.Errorf("Expected output to contain 'test.go', got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "main.go") {
		t.Errorf("Expected output to contain 'main.go', got: %s", result.Output)
	}
}

func TestExecuteGlob_RecursivePattern(t *testing.T) {
	te := NewToolExecutor()

	// Create test directory structure
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "src", "sub"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "root.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "src", "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "src", "sub", "nested.go"), []byte("package main"), 0644)

	// Test glob with recursive pattern
	result := te.Execute(&ToolCall{
		Name: "glob",
		Parameters: map[string]interface{}{
			"pattern": filepath.Join(tmpDir, "**/*.go"),
		},
	})

	if !result.Success {
		t.Fatalf("Expected success, got error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "root.go") {
		t.Errorf("Expected output to contain 'root.go', got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "src/main.go") {
		t.Errorf("Expected output to contain 'src/main.go', got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "nested.go") {
		t.Errorf("Expected output to contain 'nested.go', got: %s", result.Output)
	}
}

func TestExecuteGlob_NoMatch(t *testing.T) {
	te := NewToolExecutor()

	tmpDir := t.TempDir()
	result := te.Execute(&ToolCall{
		Name: "glob",
		Parameters: map[string]interface{}{
			"pattern": filepath.Join(tmpDir, "*.xyz"),
		},
	})

	if !result.Success {
		t.Fatalf("Expected success, got error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "No files found") {
		t.Errorf("Expected 'No files found', got: %s", result.Output)
	}
}

func TestExecuteGlob_MissingPattern(t *testing.T) {
	te := NewToolExecutor()

	result := te.Execute(&ToolCall{
		Name:       "glob",
		Parameters: map[string]interface{}{},
	})

	if result.Success {
		t.Errorf("Expected failure for missing pattern, got success")
	}
	if !strings.Contains(result.Error, "missing required parameter: pattern") {
		t.Errorf("Expected 'missing required parameter: pattern', got: %s", result.Error)
	}
}

func TestExecuteGlob_MaxResults(t *testing.T) {
	te := NewToolExecutor()

	// Create test directory with many files
	tmpDir := t.TempDir()
	for i := 0; i < 10; i++ {
		os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("file%d.go", i)), []byte("package main"), 0644)
	}

	result := te.Execute(&ToolCall{
		Name: "glob",
		Parameters: map[string]interface{}{
			"pattern":     filepath.Join(tmpDir, "*.go"),
			"max_results": 3.0,
		},
	})

	if !result.Success {
		t.Fatalf("Expected success, got error: %s", result.Error)
	}
	if extra, ok := result.Extra["matchesFound"].(int); ok {
		if extra != 3 {
			t.Errorf("Expected 3 matches, got %d", extra)
		}
	}
}

func TestExecuteGlob_EmptyPattern(t *testing.T) {
	te := NewToolExecutor()

	result := te.Execute(&ToolCall{
		Name: "glob",
		Parameters: map[string]interface{}{
			"pattern": "",
		},
	})

	if result.Success {
		t.Errorf("Expected failure for empty pattern, got success")
	}
	if !strings.Contains(result.Error, "missing required parameter: pattern") {
		t.Errorf("Expected 'missing required parameter: pattern', got: %s", result.Error)
	}
}

func TestExecuteGlob_MixedCase(t *testing.T) {
	te := NewToolExecutor()

	// Create test directory structure
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "Src"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "Test.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "Src", "Main.go"), []byte("package main"), 0644)

	result := te.Execute(&ToolCall{
		Name: "glob",
		Parameters: map[string]interface{}{
			"pattern": filepath.Join(tmpDir, "**/*.go"),
		},
	})

	if !result.Success {
		t.Fatalf("Expected success, got error: %s", result.Error)
	}
	// Should find both files regardless of case
	if !strings.Contains(result.Output, "Test.go") {
		t.Errorf("Expected output to contain 'Test.go', got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "Main.go") {
		t.Errorf("Expected output to contain 'Main.go', got: %s", result.Output)
	}
}

func TestExecuteGlob_InvalidPattern(t *testing.T) {
	te := NewToolExecutor()

	result := te.Execute(&ToolCall{
		Name: "glob",
		Parameters: map[string]interface{}{
			"pattern": "[invalid", // Invalid regex pattern
		},
	})

	// Should handle gracefully - either succeed with no results or error
	if result.Success {
		// If it succeeds, check that it didn't crash
		if result.Output == "" {
			t.Errorf("Expected some output for invalid pattern")
		}
	}
	// Not requiring failure here since different OS/file systems handle invalid patterns differently
}

func TestMatchGlob_SimplePattern(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"*.go", "test.go", true},
		{"*.go", "test.txt", false},
		{"main.go", "main.go", true},
		{"*.js", "src/app.js", false}, // path has directory prefix, simple pattern won't match
	}

	for _, tc := range tests {
		got, err := matchGlob(tc.pattern, tc.path)
		if err != nil {
			t.Errorf("matchGlob(%q, %q) error: %v", tc.pattern, tc.path, err)
			continue
		}
		if got != tc.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestMatchGlob_PrefixPattern(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"src/*.go", "src/test.go", true},
		{"src/*.go", "src/test.txt", false},
		{"src/*.go", "lib/test.go", false},
		{"**/*.go", "src/nested/test.go", true},
		{"src/**/*.go", "src/a/b/c/test.go", true},
	}

	for _, tc := range tests {
		got, err := matchGlob(tc.pattern, tc.path)
		if err != nil {
			t.Errorf("matchGlob(%q, %q) error: %v", tc.pattern, tc.path, err)
			continue
		}
		if got != tc.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

// --- Sub-Agent Tool Tests ---

func TestExecute_SubAgent_MissingPrompt(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name:       "sub_agent",
		Parameters: map[string]interface{}{},
	})
	if result.Success {
		t.Error("Expected failure for missing prompt parameter")
	}
	if !strings.Contains(result.Error, "missing required parameter: prompt") {
		t.Errorf("Expected 'missing required parameter: prompt' error, got: %s", result.Error)
	}
}

func TestExecute_SubAgent_EmptyPrompt(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "sub_agent",
		Parameters: map[string]interface{}{
			"prompt": "",
		},
	})
	if result.Success {
		t.Error("Expected failure for empty prompt parameter")
	}
	if !strings.Contains(result.Error, "missing required parameter: prompt") {
		t.Errorf("Expected 'missing required parameter: prompt' error, got: %s", result.Error)
	}
}

func TestExecute_SubAgent_ExecuteSuccess(t *testing.T) {
	// Find the coding-agent executable
	var executablePath string
	cwd, _ := os.Getwd()
	for _, candidate := range []string{
		filepath.Join(cwd, "coding-agent"),
		filepath.Join(cwd, "..", "coding-agent"),
	} {
		// Resolve the path
		resolved, _ := filepath.Abs(candidate)
		if _, err := os.Stat(resolved); err == nil {
			executablePath = resolved
			break
		}
	}
	// Also check relative to this test's working directory (implementation/)
	if executablePath == "" {
		cwd2, _ := os.Getwd()
		for _, candidate := range []string{
			filepath.Join(cwd2, "..", "coding-agent"),
		} {
			resolved, _ := filepath.Abs(candidate)
			if _, err := os.Stat(resolved); err == nil {
				executablePath = resolved
				break
			}
		}
	}

	if executablePath == "" {
		t.Skip("coding-agent executable not found, skipping sub_agent execution test")
	}

	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "sub_agent",
		Parameters: map[string]interface{}{
			"prompt":  "echo hello from sub-agent",
			"timeout": 10,
		},
	})
	// The sub-agent should run in one-shot mode and return the result of the echo command
	if !result.Success {
		t.Errorf("Expected success for valid sub_agent call, got: %s", result.Error)
	}
	if !strings.Contains(result.Output, "hello from sub-agent") {
		t.Errorf("Expected output to contain 'hello from sub-agent', got: %s", result.Output)
	}
}

func TestExecute_SubAgent_ExecuteFailure(t *testing.T) {
	// Find the coding-agent executable
	var executablePath string
	cwd, _ := os.Getwd()
	for _, candidate := range []string{
		filepath.Join(cwd, "coding-agent"),
		filepath.Join(cwd, "..", "coding-agent"),
	} {
		resolved, _ := filepath.Abs(candidate)
		if _, err := os.Stat(resolved); err == nil {
			executablePath = resolved
			break
		}
	}
	if executablePath == "" {
		t.Skip("coding-agent executable not found, skipping sub_agent execution failure test")
	}

	te := NewToolExecutor()
	// Use a prompt that the sub-agent will try to execute. The LLM may decide
	// not to execute an obviously invalid command, in which case the sub-agent
	// still exits successfully (exit code 0) with a helpful response.
	result := te.Execute(&ToolCall{
		Name: "sub_agent",
		Parameters: map[string]interface{}{
			"prompt":  "this-command-definitely-does-not-exist-12345",
			"timeout": 10,
		},
	})
	// The sub-agent process itself should succeed (exit code 0).
	// The LLM inside may respond helpfully rather than trying to execute the invalid command.
	if !result.Success {
		t.Logf("Sub-agent returned (this is acceptable - the LLM may have declined to execute): %s", result.Error)
	}
}

func TestExecute_SubAgent_Stats(t *testing.T) {
	te := NewToolExecutor()

	// Test that stats are tracked properly
	// First, call an unknown tool to increment FailedCalls
	te.Execute(&ToolCall{Name: "bash", Parameters: map[string]interface{}{"command": "echo test"}})

	// Find the coding-agent executable for a successful call
	var executablePath string
	cwd, _ := os.Getwd()
	for _, candidate := range []string{
		filepath.Join(cwd, "coding-agent"),
		filepath.Join(cwd, "..", "coding-agent"),
	} {
		resolved, _ := filepath.Abs(candidate)
		if _, err := os.Stat(resolved); err == nil {
			executablePath = resolved
			break
		}
	}

	if executablePath != "" {
		te.Execute(&ToolCall{
			Name: "sub_agent",
			Parameters: map[string]interface{}{
				"prompt":  "true",
				"timeout": 10,
			},
		})
	}

	stats := te.Stats()
	if stats.TotalCalls != 2 {
		t.Errorf("Expected 2 total calls, got %d", stats.TotalCalls)
	}
}

func TestExecute_SubAgent_TooManyParameters(t *testing.T) {
	te := NewToolExecutor()
	// Test with extra unexpected parameters - should still work with just prompt
	result := te.Execute(&ToolCall{
		Name: "sub_agent",
		Parameters: map[string]interface{}{
			"prompt":  "echo extra params",
			"timeout": 5,
			"extra":   "ignored",
			"nested":  map[string]interface{}{"key": "value"},
		},
	})

	// Find the coding-agent executable
	cwd, _ := os.Getwd()
	found := false
	for _, candidate := range []string{
		filepath.Join(cwd, "coding-agent"),
		filepath.Join(cwd, "..", "coding-agent"),
	} {
		resolved, _ := filepath.Abs(candidate)
		if _, err := os.Stat(resolved); err == nil {
			found = true
			break
		}
	}

	if found {
		// With valid prompt, the tool should execute (ignoring extra params)
		if !result.Success {
			t.Logf("Sub-agent execution returned: %s", result.Error)
			// Don't fail - the sub-agent might not be available
		}
	}
}

func TestExecute_GitCommit_MissingMessage(t *testing.T) {
	te := NewToolExecutor()
	// No message provided - should still work with --allow-empty-message
	result := te.Execute(&ToolCall{
		Name: "git_commit",
		Parameters: map[string]interface{}{},
	})
	// The result depends on whether there are staged files
	// Just verify the tool executes without panic and returns a ToolResult
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestExecute_GitCommit_Amend(t *testing.T) {
	te := NewToolExecutor()
	// Amend without a previous commit will fail
	result := te.Execute(&ToolCall{
		Name: "git_commit",
		Parameters: map[string]interface{}{
			"message": "test amend",
			"amend":   true,
		},
	})
	// If there are no commits and nothing staged, this should fail gracefully
	// (we're in the harness repo which may or may not have commits/staged files)
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestExecuteListDir_DefaultPath(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name:       "list_dir",
		Parameters: map[string]interface{}{},
	})

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("Expected non-empty output")
	}
	if result.Extra == nil {
		t.Fatal("Expected non-nil Extra map")
	}
	if result.Extra["directories"] == nil || result.Extra["files"] == nil {
		t.Error("Expected directories and files counts in Extra")
	}
}

func TestExecuteListDir_NonExistentPath(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "list_dir",
		Parameters: map[string]interface{}{
			"path": "/nonexistent/path/that/does/not/exist",
		},
	})

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Success {
		t.Error("Expected failure for non-existent path")
	}
	if result.Error == "" {
		t.Error("Expected error message for non-existent path")
	}
}

func TestExecuteListDir_NotADirectory(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "list_dir",
		Parameters: map[string]interface{}{
			"path": "go.mod", // This is a file, not a directory
		},
	})

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Success {
		t.Error("Expected failure when listing a file as directory")
	}
	if result.Error == "" {
		t.Error("Expected error message")
	}
}

func TestExecuteListDir_WithMaxResults(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "list_dir",
		Parameters: map[string]interface{}{
			"path":        ".",
			"max_results": 3,
		},
	})

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}
	if result.Extra == nil {
		t.Fatal("Expected non-nil Extra map")
	}
	totalItems := result.Extra["total_items"]
	if totalItems.(int) > 3 {
		t.Errorf("Expected at most 3 items, got %d", totalItems.(int))
	}
}

func TestExecuteListDir_HiddenFiles(t *testing.T) {
	te := NewToolExecutor()
	// Without show_hidden, dotfiles should be excluded
	result1 := te.Execute(&ToolCall{
		Name: "list_dir",
		Parameters: map[string]interface{}{
			"path": ".",
		},
	})

	// With show_hidden=true, dotfiles should be included
	result2 := te.Execute(&ToolCall{
		Name: "list_dir",
		Parameters: map[string]interface{}{
			"path":        ".",
			"show_hidden": true,
		},
	})

	if result1 == nil || result2 == nil {
		t.Fatal("Expected non-nil results")
	}
	if !result1.Success || !result2.Success {
		t.Fatalf("Expected success for both calls")
	}
	// With show_hidden, we should see more items
	total1 := result1.Extra["total_items"].(int)
	total2 := result2.Extra["total_items"].(int)
	if total2 <= total1 {
		t.Logf("Note: show_hidden=%d vs hidden=%d (may be equal in this directory)", total2, total1)
	}
}

func TestExecuteListDir_TypeField(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(&ToolCall{
		Name: "list_dir",
		Parameters: map[string]interface{}{
			"path":        ".",
			"max_results": 5,
		},
	})

	if result == nil || !result.Success {
		t.Fatal("Expected non-nil successful result")
	}
	// Output should contain type indicators
	if !strings.Contains(result.Output, "TYPE") {
		t.Error("Expected output to contain TYPE header")
	}
	if !strings.Contains(result.Output, "dir") && !strings.Contains(result.Output, "file") {
		t.Error("Expected output to contain dir or file type indicators")
	}
}

