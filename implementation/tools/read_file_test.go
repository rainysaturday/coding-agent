package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFileTool_Name(t *testing.T) {
	tool := NewReadFileTool()
	if tool.Name() != "read_file" {
		t.Errorf("Expected name 'read_file', got '%s'", tool.Name())
	}
}

func TestReadFileTool_Description(t *testing.T) {
	tool := NewReadFileTool()
	desc := tool.Description()
	if desc != "Read the contents of a file" {
		t.Errorf("Expected description 'Read the contents of a file', got '%s'", desc)
	}
}

func TestReadFileTool_Execute_Success(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "Hello, World!"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadFileTool()
	result := tool.Execute(map[string]string{
		"path": testFile,
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}
	if result.Output != content {
		t.Errorf("Expected output '%s', got '%s'", content, result.Output)
	}
}

func TestReadFileTool_Execute_Multiline(t *testing.T) {
	// Create a temporary file with multiple lines
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "multiline.txt")
	content := "line1\nline2\nline3\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadFileTool()
	result := tool.Execute(map[string]string{
		"path": testFile,
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}
	if result.Output != content {
		t.Errorf("Expected output '%s', got '%s'", content, result.Output)
	}
}

func TestReadFileTool_Execute_FileNotFound(t *testing.T) {
	tool := NewReadFileTool()
	result := tool.Execute(map[string]string{
		"path": "/nonexistent/path/file.txt",
	})

	if result.Success {
		t.Error("Expected failure for non-existent file")
	}
	if result.Error == "" {
		t.Error("Expected error message")
	}
}

func TestReadFileTool_Execute_MissingPath(t *testing.T) {
	tool := NewReadFileTool()
	result := tool.Execute(map[string]string{})

	if result.Success {
		t.Error("Expected failure for missing path parameter")
	}
	if result.Error == "" {
		t.Error("Expected error message for missing path")
	}
}

func TestReadFileTool_Execute_EmptyPath(t *testing.T) {
	tool := NewReadFileTool()
	result := tool.Execute(map[string]string{
		"path": "",
	})

	if result.Success {
		t.Error("Expected failure for empty path")
	}
}

func TestReadFileTool_Execute_EmptyFile(t *testing.T) {
	// Create an empty temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")
	err := os.WriteFile(testFile, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	tool := NewReadFileTool()
	result := tool.Execute(map[string]string{
		"path": testFile,
	})

	if !result.Success {
		t.Errorf("Expected success for empty file, got error: %s", result.Error)
	}
	if result.Output != "" {
		t.Errorf("Expected empty output, got '%s'", result.Output)
	}
}

func TestReadFileTool_Execute_BinaryContent(t *testing.T) {
	// Create a file with binary-like content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "binary.txt")
	content := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}

	err := os.WriteFile(testFile, content, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadFileTool()
	result := tool.Execute(map[string]string{
		"path": testFile,
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}
	// The content should be preserved
	if len(result.Output) != len(content) {
		t.Errorf("Expected output length %d, got %d", len(content), len(result.Output))
	}
}

func TestReadFileTool_Execute_PermissionDenied(t *testing.T) {
	// Create a file and remove read permissions
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "noperm.txt")
	err := os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Remove read permissions
	err = os.Chmod(testFile, 0000)
	if err != nil {
		t.Fatalf("Failed to change permissions: %v", err)
	}
	defer os.Chmod(testFile, 0644) // Restore for cleanup

	tool := NewReadFileTool()
	result := tool.Execute(map[string]string{
		"path": testFile,
	})

	// May succeed or fail depending on user permissions
	// Just ensure no panic
	_ = result
}
