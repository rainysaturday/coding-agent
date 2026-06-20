package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ===== Core tool executor tests =====

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
	te.Execute(context.Background(), &ToolCall{Name: "bash", Parameters: map[string]interface{}{"command": "true"}})
	// Now call unknown tool (will always fail)
	result := te.Execute(context.Background(), &ToolCall{Name: "nonexistent_tool", Parameters: map[string]interface{}{}})
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

func TestToolExecutor_Stats(t *testing.T) {
	te := NewToolExecutor()

	// Execute several calls
	te.Execute(context.Background(), &ToolCall{Name: "bash", Parameters: map[string]interface{}{"command": "true"}})
	te.Execute(context.Background(), &ToolCall{Name: "bash", Parameters: map[string]interface{}{"command": "false"}})
	te.Execute(context.Background(), &ToolCall{Name: "unknown_tool", Parameters: map[string]interface{}{}})

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

func TestToolExecutor_Stats2(t *testing.T) {
	te := NewToolExecutor()

	// Execute some commands
	te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo test",
		},
	})

	stats := te.Stats()
	if stats.TotalCalls != 1 {
		t.Errorf("Expected 1 total call, got %d", stats.TotalCalls)
	}
}

func TestToolExecutor_StatsFailure(t *testing.T) {
	te := NewToolExecutor()

	// Execute a command that will fail
	te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "nonexistent_command_xyz_123",
		},
	})

	stats := te.Stats()
	if stats.FailedCalls == 0 {
		t.Error("Expected at least 1 failed call")
	}
}

// ===== isReadOnlyTool tests =====

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

// ===== ReadWrite mode tests =====

func TestSetReadOnly(t *testing.T) {
	te := NewToolExecutor()

	te.SetReadOnly(true)

	// Try to execute a write command - should fail
	result := te.Execute(context.Background(), &ToolCall{
		Name: "write_file",
		Parameters: map[string]interface{}{
			"path":    "/tmp/test.txt",
			"content": "test",
		},
	})

	if result.Success {
		t.Error("Expected failure for write_file in read-only mode")
	}
}

func TestSetReadOnly_ReadAllowed(t *testing.T) {
	te := NewToolExecutor()

	te.SetReadOnly(true)

	// Read operations should still work
	result := te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": ".",
		},
	})

	// Should succeed or at least not fail due to read-only mode
	if strings.Contains(result.Error, "read-only") {
		t.Error("list_files should be allowed in read-only mode")
	}
}

func TestToolExecutor_ReadOnly_NotSet(t *testing.T) {
	// When ReadOnly is not set (false), all tools should work normally
	te := NewToolExecutor()
	// Not setting SetReadOnly, so readOnly should be false by default

	// Bash should work
	result := te.Execute(context.Background(), &ToolCall{
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

func TestToolExecutor_Stats_NoCtx(t *testing.T) {
	te := NewToolExecutor()

	// Execute some commands
	te.Execute(context.Background(), &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo test",
		},
	})

	stats := te.Stats()
	if stats.TotalCalls != 1 {
		t.Errorf("Expected 1 total call, got %d", stats.TotalCalls)
	}

	// Execute with invalid path (will fail)
	te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": "/nonexistent",
		},
	})

	if stats.FailedCalls != 1 {
		t.Errorf("Expected 1 failed call, got %d", stats.FailedCalls)
	}
}

func TestToolExecutor_ListFiles_Statistics(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("content"), 0644)

	te := NewToolExecutor()

	// Execute list_files successfully
	te.Execute(context.Background(), &ToolCall{
		Name: "list_files",
		Parameters: map[string]interface{}{
			"path": tmpDir,
		},
	})

	// Execute with invalid path (will fail)
	te.Execute(context.Background(), &ToolCall{
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
