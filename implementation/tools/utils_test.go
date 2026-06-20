package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ===== Tests for utility functions =====

func TestHumanReadableSize(t *testing.T) {
	tests := []struct {
		size     int64
		expected string
	}{
		{0, "0"},
		{512, "512"},
		{1024, "1.0K"},
		{1536, "1.5K"},
		{1048576, "1.0M"},
		{1073741824, "1.0G"},
	}

	for _, tt := range tests {
		result := humanReadableSize(tt.size)
		if result != tt.expected {
			t.Errorf("humanReadableSize(%d) = %q, expected %q", tt.size, result, tt.expected)
		}
	}
}

func TestFormatPermissions(t *testing.T) {
	tests := []struct {
		name     string
		mode     os.FileMode
		expected string
	}{
		{"regular file", 0644, "-rw-r--r--"},
		{"directory", 0755, "drwxr-xr-x"},
		{"executable", 0755, "drwxr-xr-x"}, // Will be dir due to IsDir check
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			var info os.FileInfo

			if tt.name == "directory" || tt.name == "executable" {
				os.MkdirAll(filepath.Join(tmpDir, "testdir"), tt.mode)
				info, _ = os.Stat(filepath.Join(tmpDir, "testdir"))
			} else {
				testFile := filepath.Join(tmpDir, "testfile")
				os.WriteFile(testFile, []byte("content"), tt.mode)
				info, _ = os.Stat(testFile)
			}

			result := formatPermissions(info)
			if result != tt.expected {
				t.Errorf("formatPermissions(%v) = %q, expected %q", tt.mode, result, tt.expected)
			}
		})
	}
}

// ===== Tests for formatPermissionsRecursive =====

func TestFormatPermissionsRecursive_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	info, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	entry := walkEntry{
		path:     tmpDir,
		isDir:    true,
		info:     info,
		modTime:  info.ModTime(),
		fileSize: 0,
	}

	result := formatPermissionsRecursive(entry)
	if result[0] != 'd' {
		t.Errorf("Expected directory type 'd', got '%s'", string(result[0]))
	}
}

func TestFormatPermissionsRecursive_File(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	os.WriteFile(tmpFile, []byte("content"), 0644)
	info, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	entry := walkEntry{
		path:     tmpFile,
		isDir:    false,
		info:     info,
		modTime:  info.ModTime(),
		fileSize: info.Size(),
	}

	result := formatPermissionsRecursive(entry)
	if result[0] != '-' {
		t.Errorf("Expected file type '-', got '%s'", string(result[0]))
	}
}

// ===== Tests for formatRecursiveLongList =====

func TestFormatRecursiveLongList_Empty(t *testing.T) {
	result := formatRecursiveLongList(nil, "", nil)
	// Empty entries should return empty string
	if result != "" {
		t.Errorf("Expected empty result for nil entries, got: %s", result)
	}
}

func TestFormatRecursiveLongList_Simple(t *testing.T) {
	tmpDir := t.TempDir()
	info, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	entries := []walkEntry{
		{
			path:     tmpDir,
			isDir:    true,
			info:     info,
			modTime:  info.ModTime(),
			fileSize: 0,
		},
	}

	result := formatRecursiveLongList(entries, tmpDir, nil)
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

func TestFormatRecursiveLongList_WithFlags(t *testing.T) {
	tmpDir := t.TempDir()
	info, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	entries := []walkEntry{
		{
			path:     tmpDir,
			isDir:    true,
			info:     info,
			modTime:  info.ModTime(),
			fileSize: 0,
		},
	}

	flags := map[string]bool{
		"human-readable": true,
	}

	result := formatRecursiveLongList(entries, tmpDir, flags)
	if result == "" {
		t.Error("Expected non-empty result with flags")
	}
}

// ===== Tests for truncateOutput =====

func TestTruncateOutput_Empty(t *testing.T) {
	result := truncateOutput("", 10)
	if result != "(empty file)" {
		t.Errorf("Expected '(empty file)', got %q", result)
	}
}

func TestTruncateOutput_NoTruncation(t *testing.T) {
	text := "line1\nline2\nline3"
	result := truncateOutput(text, 10)
	if result != text {
		t.Errorf("Expected %q, got %q", text, result)
	}
}

func TestTruncateOutput_WithTruncation(t *testing.T) {
	lines := make([]string, 15)
	for i := range lines {
		lines[i] = fmt.Sprintf("line%d", i+1)
	}
	text := strings.Join(lines, "\n")
	result := truncateOutput(text, 5)
	expected := strings.Join(lines[:5], "\n") + "\n... [content truncated]"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// ===== Tests for truncateString =====

func TestTruncateString_NoTruncation(t *testing.T) {
	s := "hello"
	result := truncateString(s, 10)
	if result != s {
		t.Errorf("Expected %q, got %q", s, result)
	}
}

func TestTruncateString_WithTruncation(t *testing.T) {
	s := "hello world"
	result := truncateString(s, 5)
	expected := "hello..."
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// ===== Tests for splitLines and joinLines =====

// TestSplitLines_Empty verifies splitLines handles empty content.
func TestSplitLines_Empty(t *testing.T) {
	result := splitLines("")
	if len(result) != 0 {
		t.Errorf("Expected empty slice for empty content, got %v", result)
	}
}

// TestSplitLines_NoTrailingNewline verifies splitLines handles content without trailing newline.
func TestSplitLines_NoTrailingNewline(t *testing.T) {
	result := splitLines("a\nb\nc")
	if len(result) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(result))
	}
	if result[0] != "a" || result[1] != "b" || result[2] != "c" {
		t.Errorf("Unexpected content: %v", result)
	}
}

// TestSplitLines_WithTrailingNewline verifies splitLines handles trailing newline correctly.
func TestSplitLines_WithTrailingNewline(t *testing.T) {
	result := splitLines("a\nb\nc\n")
	if len(result) != 3 {
		t.Errorf("Expected 3 lines (no extra empty), got %d: %v", len(result), result)
	}
}

// TestSplitLines_SingleLine verifies splitLines handles single line content.
func TestSplitLines_SingleLine(t *testing.T) {
	result := splitLines("only")
	if len(result) != 1 {
		t.Errorf("Expected 1 line, got %d", len(result))
	}
	if result[0] != "only" {
		t.Errorf("Expected 'only', got '%s'", result[0])
	}
}

// TestJoinLines_Empty verifies joinLines handles empty slice.
func TestJoinLines_Empty(t *testing.T) {
	result := joinLines([]string{})
	if result != "" {
		t.Errorf("Expected empty string for empty slice, got %q", result)
	}
}

// TestJoinLines_SingleLine verifies joinLines adds trailing newline for single line.
func TestJoinLines_SingleLine(t *testing.T) {
	result := joinLines([]string{"hello"})
	if result != "hello\n" {
		t.Errorf("Expected 'hello\\n', got %q", result)
	}
}

// TestJoinLines_MultipleLines verifies joinLines produces correct output.
func TestJoinLines_MultipleLines(t *testing.T) {
	result := joinLines([]string{"a", "b", "c"})
	expected := "a\nb\nc\n"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// ===== ExecuteCtx cancellation tests =====

func TestExecuteCtx_Cancelled(t *testing.T) {
	te := NewToolExecutor()
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel before execution
	cancel()

	result := te.Execute(ctx, &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "sleep 10",
		},
	})
	if result.Success {
		t.Error("Expected failure for pre-cancelled context")
	}
}

func TestExecuteCtx_Bash_Cancelled(t *testing.T) {
	te := NewToolExecutor()
	ctx, cancel := context.WithCancel(context.Background())

	// Start a long-running command and cancel it
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result := te.Execute(ctx, &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "sleep 30",
		},
	})
	// Should not panic
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}
