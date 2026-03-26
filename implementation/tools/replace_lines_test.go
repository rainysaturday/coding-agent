package tools

import (
	"os"
	"path/filepath"
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
	if desc != "Replace a line range with new lines" {
		t.Errorf("Expected description 'Replace a line range with new lines', got '%s'", desc)
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
