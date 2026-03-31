package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplaceLinesTool_Name(t *testing.T) {
	tool := NewReplaceLinesTool()
	if tool.Name() != "replace_lines" {
		t.Errorf("Expected name 'replace_lines', got '%s'", tool.Name())
	}
}

func TestReplaceLinesTool_Description(t *testing.T) {
	tool := NewReplaceLinesTool()
	desc := tool.Description()
	// Description should mention search-and-replace capability
	if !strings.Contains(desc, "search") && !strings.Contains(desc, "Replace lines") {
		t.Errorf("Expected description to mention search-and-replace or line replacement, got '%s'", desc)
	}
}

func TestReplaceLinesTool_Execute_SingleLine(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "line1\nline2\nline3\nline4\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "2",
		"end":   "2",
		"lines": "replaced line",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "line1\nreplaced line\nline3\nline4\n"
	if string(content) != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, string(content))
	}
}

func TestReplaceLinesTool_Execute_MultipleLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "line1\nline2\nline3\nline4\nline5\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "2",
		"end":   "4",
		"lines": "new1\nnew2",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "line1\nnew1\nnew2\nline5\n"
	if string(content) != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, string(content))
	}
}

func TestReplaceLinesTool_Execute_ReplaceAll(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "line1\nline2\nline3\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "3",
		"lines": "replacement",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "replacement\n"
	if string(content) != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, string(content))
	}
}

func TestReplaceLinesTool_Execute_ReplaceAllWithEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "line1\nline2\nline3\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "3",
		"lines": "",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := ""
	if string(content) != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, string(content))
	}
}

func TestReplaceLinesTool_Execute_BeyondEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "line1\nline2\nline3\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "2",
		"end":   "100",
		"lines": "replacement",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "line1\nreplacement\n"
	if string(content) != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, string(content))
	}
}

func TestReplaceLinesTool_Execute_StartBeyondEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "line1\nline2\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "100",
		"end":   "200",
		"lines": "appended",
	})

	if !result.Success {
		t.Errorf("Expected success for start beyond file, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "line1\nline2\nappended\n"
	if string(content) != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, string(content))
	}
}

func TestReplaceLinesTool_Execute_CreateNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "new.txt")

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "1",
		"lines": "first line\nsecond line",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "first line\nsecond line\n"
	if string(content) != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, string(content))
	}
}

func TestReplaceLinesTool_Execute_StartGreaterThanEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "5",
		"end":   "3",
		"lines": "test",
	})

	if result.Success {
		t.Error("Expected failure for start > end")
	}
	if result.Error == "" {
		t.Error("Expected error message")
	}
}

func TestReplaceLinesTool_Execute_MissingPath(t *testing.T) {
	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"start": "1",
		"end":   "1",
		"lines": "test",
	})

	if result.Success {
		t.Error("Expected failure for missing path")
	}
}

func TestReplaceLinesTool_Execute_MissingStart(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"end":   "1",
		"lines": "test",
	})

	if result.Success {
		t.Error("Expected failure for missing start")
	}
}

func TestReplaceLinesTool_Execute_MissingEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"lines": "test",
	})

	if result.Success {
		t.Error("Expected failure for missing end")
	}
}

func TestReplaceLinesTool_Execute_MissingLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "1",
	})

	if result.Success {
		t.Error("Expected failure for missing lines")
	}
}

func TestReplaceLinesTool_Execute_InvalidStart(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "abc",
		"end":   "1",
		"lines": "test",
	})

	if result.Success {
		t.Error("Expected failure for invalid start")
	}
}

func TestReplaceLinesTool_Execute_ZeroStart(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "0",
		"end":   "1",
		"lines": "test",
	})

	if result.Success {
		t.Error("Expected failure for zero start")
	}
}

func TestParseReplaceLines_JSONFormat(t *testing.T) {
	input := `[TOOL:{"name":"replace_lines","parameters":{"path":"/tmp/test.txt","start":"1","end":"3","lines":"replacement A\nreplacement B"}}]`

	call, err := ParseToolCall(input)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if call.Name != "replace_lines" {
		t.Errorf("Expected 'replace_lines', got '%s'", call.Name)
	}
	if call.Params["path"] != "/tmp/test.txt" {
		t.Errorf("Expected path '/tmp/test.txt', got '%s'", call.Params["path"])
	}
	if call.Params["start"] != "1" {
		t.Errorf("Expected start '1', got '%s'", call.Params["start"])
	}
	if call.Params["end"] != "3" {
		t.Errorf("Expected end '3', got '%s'", call.Params["end"])
	}
	expected := "replacement A\nreplacement B"
	if call.Params["lines"] != expected {
		t.Errorf("Expected '%s', got '%s'", expected, call.Params["lines"])
	}
}

func TestReplaceLines_Tool_JSONFormat(t *testing.T) {
	// Create test file
	testFile := "/tmp/replace_json_test.txt"
	initialContent := "line 1\nline 2\nline 3\nline 4\n"

	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	// Parse JSON format tool call
	input := `[TOOL:{"name":"replace_lines","parameters":{"path":"/tmp/replace_json_test.txt","start":"2","end":"3","lines":"replaced A\nreplaced B"}}]`

	call, err := ParseToolCall(input)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Execute
	result := NewReplaceLinesTool().Execute(call.Params)
	if !result.Success {
		t.Fatalf("Expected success, got error: %s", result.Error)
	}

	// Verify
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "line 1\nreplaced A\nreplaced B\nline 4\n"
	if string(content) != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, string(content))
	}
}

func TestFormatReplaceLines_JSONFormat(t *testing.T) {
	params := map[string]string{
		"path":  "/tmp/test.txt",
		"start": "1",
		"end":   "5",
		"lines": "line1\nline2\nline3",
	}
	result := FormatToolCall("replace_lines", params)

	if !strings.Contains(result, "replace_lines") {
		t.Error("Expected 'replace_lines' in result")
	}
	if !strings.Contains(result, "\\n") {
		t.Error("Expected escaped newlines in result")
	}
	if !strings.HasPrefix(result, "[TOOL:") || !strings.HasSuffix(result, "]") {
		t.Error("Expected result to be wrapped in [TOOL:...]")
	}
}

func TestReplaceLinesTool_Execute_WithTrailingNewline(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create a test file with 5 lines
	originalContent := "line1\nline2\nline3\nline4\nline5\n"
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Replace lines 2-4 with content that has a trailing newline
	// This simulates what might happen if the LLM generates trailing newlines
	replacementWithTrailingNewline := "new line A\nnew line B\n"

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "2",
		"end":   "4",
		"lines": replacementWithTrailingNewline,
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	// Read the file and verify
	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	// Expected: line1 + new line A + new line B + line5
	expectedContent := "line1\nnew line A\nnew line B\nline5\n"
	if fileContent != expectedContent {
		t.Errorf("File content mismatch.\nGot:\n%q\nWant:\n%q", fileContent, expectedContent)
	}
}
