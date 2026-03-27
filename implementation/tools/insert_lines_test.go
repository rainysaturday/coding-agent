package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInsertLinesTool_Name(t *testing.T) {
	tool := NewInsertLinesTool()
	if tool.Name() != "insert_lines" {
		t.Errorf("Expected name 'insert_lines', got '%s'", tool.Name())
	}
}

func TestInsertLinesTool_Description(t *testing.T) {
	tool := NewInsertLinesTool()
	desc := tool.Description()
	if desc != "Insert lines at a specific line number" {
		t.Errorf("Expected description 'Insert lines at a specific line number', got '%s'", desc)
	}
}

func TestInsertLinesTool_Execute_InsertAtBeginning(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "line1\nline2\nline3\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewInsertLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"line":  "1",
		"lines": "new line 1\nnew line 2",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "new line 1\nnew line 2\nline1\nline2\nline3\n"
	if string(content) != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, string(content))
	}
}

func TestInsertLinesTool_Execute_InsertInMiddle(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "line1\nline2\nline3\nline4\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewInsertLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"line":  "3",
		"lines": "inserted line",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "line1\nline2\ninserted line\nline3\nline4\n"
	if string(content) != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, string(content))
	}
}

func TestInsertLinesTool_Execute_InsertAtEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "line1\nline2\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewInsertLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"line":  "3",
		"lines": "appended line",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "line1\nline2\nappended line\n"
	if string(content) != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, string(content))
	}
}

func TestInsertLinesTool_Execute_BeyondEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "line1\nline2\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewInsertLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"line":  "100",
		"lines": "appended",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
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

func TestInsertLinesTool_Execute_CreateNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "new.txt")

	tool := NewInsertLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"line":  "1",
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

func TestInsertLinesTool_Execute_MissingPath(t *testing.T) {
	tool := NewInsertLinesTool()
	result := tool.Execute(map[string]string{
		"line":  "1",
		"lines": "test",
	})

	if result.Success {
		t.Error("Expected failure for missing path")
	}
}

func TestInsertLinesTool_Execute_MissingLine(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewInsertLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"lines": "test",
	})

	if result.Success {
		t.Error("Expected failure for missing line")
	}
}

func TestInsertLinesTool_Execute_MissingLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewInsertLinesTool()
	result := tool.Execute(map[string]string{
		"path": testFile,
		"line": "1",
	})

	if result.Success {
		t.Error("Expected failure for missing lines")
	}
}

func TestInsertLinesTool_Execute_InvalidLine(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewInsertLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"line":  "abc",
		"lines": "test",
	})

	if result.Success {
		t.Error("Expected failure for invalid line")
	}
}

func TestInsertLinesTool_Execute_ZeroLine(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewInsertLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"line":  "0",
		"lines": "test",
	})

	if result.Success {
		t.Error("Expected failure for zero line")
	}
}

func TestInsertLinesTool_Execute_EmptyLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "line1\nline2\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewInsertLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"line":  "2",
		"lines": "",
	})

	if !result.Success {
		t.Errorf("Expected success for empty lines, got error: %s", result.Error)
	}

	// File should be unchanged
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != initialContent {
		t.Errorf("Expected unchanged content '%s', got '%s'", initialContent, string(content))
	}
}

func TestParseInsertLines_RawMode(t *testing.T) {
	input := `[tool:insert_lines(path="/tmp/test.txt", line=5, lines=<<<RAW>>>
new line 1
new line 2
<<<END_RAW>>>)]`
	
	call, err := ParseToolCall(input)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	
	if call.Name != "insert_lines" {
		t.Errorf("Expected 'insert_lines', got '%s'", call.Name)
	}
	if call.Params["path"] != "/tmp/test.txt" {
		t.Errorf("Expected path '/tmp/test.txt', got '%s'", call.Params["path"])
	}
	if call.Params["line"] != "5" {
		t.Errorf("Expected line '5', got '%s'", call.Params["line"])
	}
	expected := "new line 1\nnew line 2"
	if call.Params["lines"] != expected {
		t.Errorf("Expected '%s', got '%s'", expected, call.Params["lines"])
	}
}

func TestInsertLines_Tool_RawMode(t *testing.T) {
	// Create test file
	testFile := "/tmp/insert_test.txt"
	initialContent := "line 1\nline 2\nline 3\n"
	
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testFile)
	
	// Parse raw mode tool call
	input := `[tool:insert_lines(path="/tmp/insert_test.txt", line=2, lines=<<<RAW>>>
inserted A
inserted B
<<<END_RAW>>>)]`
	
	call, err := ParseToolCall(input)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	
	// Execute
	result := NewInsertLinesTool().Execute(call.Params)
	if !result.Success {
		t.Fatalf("Expected success, got error: %s", result.Error)
	}
	
	// Verify
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	
	expected := "line 1\ninserted A\ninserted B\nline 2\nline 3\n"
	if string(content) != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, string(content))
	}
}

func TestFormatInsertLines_RawMode(t *testing.T) {
	params := map[string]string{
		"path": "/tmp/test.txt",
		"line": "5",
		"lines": "line1\nline2\nline3",
	}
	result := FormatToolCall("insert_lines", params)
	
	if !contains(result, "insert_lines") {
		t.Error("Expected 'insert_lines' in result")
	}
	if !contains(result, RawStartMarker) {
		t.Error("Expected raw mode marker")
	}
	if !contains(result, RawEndMarker) {
		t.Error("Expected raw mode end marker")
	}
}
