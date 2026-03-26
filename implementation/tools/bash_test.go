package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBashTool_Name(t *testing.T) {
	tool := NewBashTool()
	if tool.Name() != "bash" {
		t.Errorf("Expected name 'bash', got '%s'", tool.Name())
	}
}

func TestBashTool_Description(t *testing.T) {
	tool := NewBashTool()
	desc := tool.Description()
	if desc != "Execute a bash command" {
		t.Errorf("Expected description 'Execute a bash command', got '%s'", desc)
	}
}

func TestBashTool_Execute_Success(t *testing.T) {
	tool := NewBashTool()

	result := tool.Execute(map[string]string{
		"command": "echo hello",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}
	if result.Output != "hello" {
		t.Errorf("Expected output 'hello', got '%s'", result.Output)
	}
}

func TestBashTool_Execute_ListCommand(t *testing.T) {
	tool := NewBashTool()

	result := tool.Execute(map[string]string{
		"command": "ls",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}
	// Output should contain some files
	if result.Output == "" {
		t.Error("Expected non-empty output")
	}
}

func TestBashTool_Execute_Failure(t *testing.T) {
	tool := NewBashTool()

	result := tool.Execute(map[string]string{
		"command": "nonexistent_command_12345",
	})

	if result.Success {
		t.Error("Expected failure for nonexistent command")
	}
	if result.Error == "" {
		t.Error("Expected error message")
	}
}

func TestBashTool_Execute_MissingCommand(t *testing.T) {
	tool := NewBashTool()

	result := tool.Execute(map[string]string{})

	if result.Success {
		t.Error("Expected failure for missing command parameter")
	}
	if result.Error == "" {
		t.Error("Expected error message for missing command")
	}
}

func TestBashTool_Execute_EmptyCommand(t *testing.T) {
	tool := NewBashTool()

	result := tool.Execute(map[string]string{
		"command": "",
	})

	// Empty command should fail
	if result.Success {
		t.Error("Expected failure for empty command")
	}
}

func TestBashTool_Execute_WithStderr(t *testing.T) {
	tool := NewBashTool()

	// This command produces stderr
	result := tool.Execute(map[string]string{
		"command": "echo 'error message' >&2",
	})

	if !result.Success {
		t.Errorf("Command may succeed or fail, error: %s", result.Error)
	}
	// stderr should be captured in combined output
	if result.Output == "" {
		t.Error("Expected stderr in output")
	}
}

func TestBashTool_Execute_ComplexCommand(t *testing.T) {
	tool := NewBashTool()

	result := tool.Execute(map[string]string{
		"command": "echo -n 'test' && echo ' output'",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}
	// Combined output
	if result.Output == "" {
		t.Error("Expected output from complex command")
	}
}

func TestBashTool_Execute_PipeCommand(t *testing.T) {
	tool := NewBashTool()

	result := tool.Execute(map[string]string{
		"command": "echo 'hello world' | wc -w",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}
	if result.Output != "2" {
		t.Errorf("Expected output '2', got '%s'", result.Output)
	}
}

func TestBashTool_Execute_GrepCommand(t *testing.T) {
	tool := NewBashTool()

	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\nline2 with match\nline3"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result := tool.Execute(map[string]string{
		"command": "grep 'match' " + testFile,
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}
	if result.Output != "line2 with match" {
		t.Errorf("Expected output 'line2 with match', got '%s'", result.Output)
	}
}

func TestBashTool_Execute_ExitCode(t *testing.T) {
	tool := NewBashTool()

	// Command that exits with non-zero
	result := tool.Execute(map[string]string{
		"command": "exit 1",
	})

	if result.Success {
		t.Error("Expected failure for exit code 1")
	}
}
