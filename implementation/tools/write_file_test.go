package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileTool_Name(t *testing.T) {
	tool := NewWriteFileTool()
	if tool.Name() != "write_file" {
		t.Errorf("Expected name 'write_file', got '%s'", tool.Name())
	}
}

func TestWriteFileTool_Description(t *testing.T) {
	tool := NewWriteFileTool()
	desc := tool.Description()
	if desc != "Write content to a file" {
		t.Errorf("Expected description 'Write content to a file', got '%s'", desc)
	}
}

func TestWriteFileTool_Execute_Success(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "Hello, World!"

	tool := NewWriteFileTool()
	result := tool.Execute(map[string]string{
		"path":    testFile,
		"content": content,
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	// Verify the file was written correctly
	writtenContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}
	if string(writtenContent) != content {
		t.Errorf("Expected written content '%s', got '%s'", content, string(writtenContent))
	}
}

func TestWriteFileTool_Execute_Overwrite(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "overwrite.txt")

	// Write initial content
	initialContent := "initial content"
	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create initial file: %v", err)
	}

	// Overwrite with new content
	newContent := "new content"
	tool := NewWriteFileTool()
	result := tool.Execute(map[string]string{
		"path":    testFile,
		"content": newContent,
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	// Verify the file was overwritten
	writtenContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}
	if string(writtenContent) != newContent {
		t.Errorf("Expected written content '%s', got '%s'", newContent, string(writtenContent))
	}
}

func TestWriteFileTool_Execute_CreateDirectory(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	// Path includes subdirectories that don't exist
	testFile := filepath.Join(tmpDir, "subdir1", "subdir2", "test.txt")
	content := "nested file content"

	tool := NewWriteFileTool()
	result := tool.Execute(map[string]string{
		"path":    testFile,
		"content": content,
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	// Verify the file was created in the nested directory
	writtenContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}
	if string(writtenContent) != content {
		t.Errorf("Expected written content '%s', got '%s'", content, string(writtenContent))
	}
}

func TestWriteFileTool_Execute_Multiline(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "multiline.txt")
	content := "line1\nline2\nline3\n"

	tool := NewWriteFileTool()
	result := tool.Execute(map[string]string{
		"path":    testFile,
		"content": content,
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	// Verify the file was written correctly
	writtenContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}
	if string(writtenContent) != content {
		t.Errorf("Expected written content '%s', got '%s'", content, string(writtenContent))
	}
}

func TestWriteFileTool_Execute_MissingPath(t *testing.T) {
	tool := NewWriteFileTool()
	result := tool.Execute(map[string]string{
		"content": "test content",
	})

	if result.Success {
		t.Error("Expected failure for missing path parameter")
	}
	if result.Error == "" {
		t.Error("Expected error message for missing path")
	}
}

func TestWriteFileTool_Execute_EmptyPath(t *testing.T) {
	tool := NewWriteFileTool()
	result := tool.Execute(map[string]string{
		"path":    "",
		"content": "test content",
	})

	if result.Success {
		t.Error("Expected failure for empty path")
	}
}

func TestWriteFileTool_Execute_MissingContent(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	tool := NewWriteFileTool()
	result := tool.Execute(map[string]string{
		"path": testFile,
	})

	if result.Success {
		t.Error("Expected failure for missing content parameter")
	}
	if result.Error == "" {
		t.Error("Expected error message for missing content")
	}
}

func TestWriteFileTool_Execute_EmptyContent(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")

	tool := NewWriteFileTool()
	result := tool.Execute(map[string]string{
		"path":    testFile,
		"content": "",
	})

	if !result.Success {
		t.Errorf("Expected success for empty content, got error: %s", result.Error)
	}

	// Verify the file was created and is empty
	writtenContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}
	if len(writtenContent) != 0 {
		t.Errorf("Expected empty file, got %d bytes", len(writtenContent))
	}
}

func TestWriteFileTool_Execute_PermissionDenied(t *testing.T) {
	// Try to write to a directory we don't have permission to
	// This might work or fail depending on the user
	tmpDir := t.TempDir()
	// Make the directory read-only
	os.Chmod(tmpDir, 0555)
	defer os.Chmod(tmpDir, 0755) // Restore for cleanup

	testFile := filepath.Join(tmpDir, "noperm.txt")

	tool := NewWriteFileTool()
	result := tool.Execute(map[string]string{
		"path":    testFile,
		"content": "test",
	})

	// May succeed or fail depending on user (root can write anywhere)
	// Just ensure no panic
	_ = result
}
