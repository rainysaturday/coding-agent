package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadLinesTool_Name(t *testing.T) {
	tool := NewReadLinesTool()
	if tool.Name() != "read_lines" {
		t.Errorf("Expected name 'read_lines', got '%s'", tool.Name())
	}
}

func TestReadLinesTool_Description(t *testing.T) {
	tool := NewReadLinesTool()
	desc := tool.Description()
	if desc != "Read a specific line range from a file" {
		t.Errorf("Expected description 'Read a specific line range from a file', got '%s'", desc)
	}
}

func TestReadLinesTool_Execute_Success(t *testing.T) {
	// Create a temporary file with multiple lines
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "2",
		"end":   "4",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}
	expectedOutput := "2: line2\n3: line3\n4: line4"
	if result.Output != expectedOutput {
		t.Errorf("Expected output '%s', got '%s'", expectedOutput, result.Output)
	}
}

func TestReadLinesTool_Execute_FirstLine(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "1",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}
	if result.Output != "1: line1" {
		t.Errorf("Expected output '1: line1', got '%s'", result.Output)
	}
}

func TestReadLinesTool_Execute_ToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "2",
		"end":   "100",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}
	expectedOutput := "2: line2\n3: line3"
	if result.Output != expectedOutput {
		t.Errorf("Expected output '%s', got '%s'", expectedOutput, result.Output)
	}
}

func TestReadLinesTool_Execute_StartBeyondFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "100",
		"end":   "200",
	})

	if !result.Success {
		t.Errorf("Expected success for start beyond file, got error: %s", result.Error)
	}
	if result.Output != "" {
		t.Errorf("Expected empty output, got '%s'", result.Output)
	}
}

func TestReadLinesTool_Execute_StartGreaterThanEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "5",
		"end":   "3",
	})

	if !result.Success {
		t.Errorf("Expected success for start > end, got error: %s", result.Error)
	}
	if result.Output != "" {
		t.Errorf("Expected empty output, got '%s'", result.Output)
	}
}

func TestReadLinesTool_Execute_FileNotFound(t *testing.T) {
	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  "/nonexistent/file.txt",
		"start": "1",
		"end":   "10",
	})

	if result.Success {
		t.Error("Expected failure for non-existent file")
	}
	if result.Error == "" {
		t.Error("Expected error message")
	}
}

func TestReadLinesTool_Execute_MissingPath(t *testing.T) {
	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"start": "1",
		"end":   "10",
	})

	if result.Success {
		t.Error("Expected failure for missing path")
	}
}

func TestReadLinesTool_Execute_MissingStart(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path": testFile,
		"end":  "10",
	})

	if result.Success {
		t.Error("Expected failure for missing start")
	}
}

func TestReadLinesTool_Execute_MissingEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
	})

	if result.Success {
		t.Error("Expected failure for missing end")
	}
}

func TestReadLinesTool_Execute_InvalidStart(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "abc",
		"end":   "10",
	})

	if result.Success {
		t.Error("Expected failure for invalid start")
	}
}

func TestReadLinesTool_Execute_InvalidEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "xyz",
	})

	if result.Success {
		t.Error("Expected failure for invalid end")
	}
}

func TestReadLinesTool_Execute_ZeroStart(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "0",
		"end":   "10",
	})

	if result.Success {
		t.Error("Expected failure for zero start (1-indexed)")
	}
}

func TestReadLinesTool_Execute_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")
	os.WriteFile(testFile, []byte(""), 0644)

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "10",
	})

	if !result.Success {
		t.Errorf("Expected success for empty file, got error: %s", result.Error)
	}
	if result.Output != "" {
		t.Errorf("Expected empty output, got '%s'", result.Output)
	}
}
