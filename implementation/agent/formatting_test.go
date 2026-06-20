package agent

import (
	"fmt"
	"strings"
	"testing"

	"github.com/coding-agent/harness/inference"
	"github.com/coding-agent/harness/tools"
)

func TestFormatToolStatus_Success(t *testing.T) {
	tests := []struct {
		name   string
		tool   string
		result *tools.ToolResult
		check  func(string) bool
	}{
		{
			name: "bash success",
			tool: "bash",
			result: &tools.ToolResult{
				Success:  true,
				Output:   "output",
				ExitCode: 0,
			},
			check: func(s string) bool {
				return strings.Contains(s, "[Success]")
			},
		},
		{
			name: "bash with exit code",
			tool: "bash",
			result: &tools.ToolResult{
				Success:  true,
				Output:   "output",
				ExitCode: 1,
			},
			check: func(s string) bool {
				return strings.Contains(s, "[Success]") && strings.Contains(s, "exit code: 1")
			},
		},
		{
			name: "read_file success",
			tool: "read_file",
			result: &tools.ToolResult{
				Success: true,
				Output:  "content",
				Extra: map[string]interface{}{
					"linesRead": 5,
				},
			},
			check: func(s string) bool {
				return strings.Contains(s, "[Success]") && strings.Contains(s, "5 lines")
			},
		},
		{
			name: "write_file success with message",
			tool: "write_file",
			result: &tools.ToolResult{
				Success: true,
				Extra: map[string]interface{}{
					"message": "File written successfully: /test/file.txt",
				},
			},
			check: func(s string) bool {
				return strings.Contains(s, "[Success]")
			},
		},
		{
			name: "insert_lines success",
			tool: "insert_lines",
			result: &tools.ToolResult{
				Success: true,
				Output:  "Inserted 3 line(s) at line 5 in: /test/file.txt\n--- Content inserted ---\nline1\nline2\nline3",
				Extra: map[string]interface{}{
					"linesInserted": 3,
				},
			},
			check: func(s string) bool {
				return strings.Contains(s, "inserted 3") || strings.Contains(s, "Inserted 3")
			},
		},
		{
			name: "replace_text success",
			tool: "replace_text",
			result: &tools.ToolResult{
				Success: true,
				Output:  "Replaced 'old' with 'new' 2 time(s) in: /test/file.txt\n--- Preview ---\nreplaced line",
				Extra: map[string]interface{}{
					"replacementsMade": 2,
					"search":           "old",
				},
			},
			check: func(s string) bool {
				return strings.Contains(s, "Replaced") && strings.Contains(s, "'old'")
			},
		},
		{
			name: "unknown tool success",
			tool: "custom_tool",
			result: &tools.ToolResult{
				Success: true,
			},
			check: func(s string) bool {
				return strings.Contains(s, "[Success]")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatToolStatus(tt.tool, tt.result)
			if !tt.check(result) {
				t.Errorf("formatToolStatus() for %s: %q does not match check", tt.tool, result)
			}
		})
	}
}

func TestFormatToolStatus_Failure(t *testing.T) {
	result := formatToolStatus("bash", &tools.ToolResult{
		Success: false,
		Error:   "command not found",
	})

	if !strings.Contains(result, "[Failed]") {
		t.Errorf("Expected [Failed] in result: %s", result)
	}
	if !strings.Contains(result, "bash") {
		t.Errorf("Expected tool name in result: %s", result)
	}
	if !strings.Contains(result, "command not found") {
		t.Errorf("Expected error message in result: %s", result)
	}
}

func TestFormatToolStatus_ReadLines(t *testing.T) {
	result := formatToolStatus("read_lines", &tools.ToolResult{
		Success: true,
		Output:  "1: line1\n2: line2\n3: line3",
		Extra: map[string]interface{}{
			"linesRead": 3,
		},
	})

	if !strings.Contains(result, "[Success]") {
		t.Error("Expected success status")
	}
}

func TestFormatToolStatus_WriteFile(t *testing.T) {
	result := formatToolStatus("write_file", &tools.ToolResult{
		Success: true,
		Output:  "File written successfully",
		Extra: map[string]interface{}{
			"message": "File written successfully: /tmp/test.txt (100 bytes)",
		},
	})

	if !strings.Contains(result, "[Success]") {
		t.Error("Expected success status")
	}
}

func TestFormatToolStatus_ReadLinesTruncation(t *testing.T) {
	// Create a result with many lines
	var output string
	for i := 1; i <= 15; i++ {
		output += fmt.Sprintf("%d: line %d\n", i, i)
	}

	result := formatToolStatus("read_lines", &tools.ToolResult{
		Success: true,
		Output:  output,
	})

	if !strings.Contains(result, "[Success]") {
		t.Error("Expected success status")
	}
}

func TestFormatToolStatus_ListFiles(t *testing.T) {
	result := formatToolStatus("list_files", &tools.ToolResult{
		Success: true,
		Output:  "file1.txt\nfile2.txt",
		Extra: map[string]interface{}{
			"entriesListed": 2,
		},
	})

	if !strings.Contains(result, "[Success]") {
		t.Error("Expected success status")
	}
	if !strings.Contains(result, "2 entries") {
		t.Error("Expected entry count")
	}
}

func TestFormatToolStatus_Grep(t *testing.T) {
	result := formatToolStatus("grep", &tools.ToolResult{
		Success: true,
		Output:  "file.txt:1:hello world",
		Extra: map[string]interface{}{
			"matchesFound": 1,
		},
	})

	if !strings.Contains(result, "[Success]") {
		t.Error("Expected success status")
	}
	if !strings.Contains(result, "grep") {
		t.Error("Expected 'grep' in output")
	}
}

func TestFormatToolStatus_GrepZeroMatches(t *testing.T) {
	result := formatToolStatus("grep", &tools.ToolResult{
		Success: true,
		Output:  "",
		Extra: map[string]interface{}{
			"matchesFound": 0,
		},
	})

	if !strings.Contains(result, "0 matches") {
		t.Error("Expected '0 matches' in output")
	}
}

func TestFormatToolStatus_GitLog(t *testing.T) {
	result := formatToolStatus("git_log", &tools.ToolResult{
		Success: true,
		Output:  "commit abc123\n    Initial commit",
		Extra: map[string]interface{}{
			"count":     1,
			"reference": "HEAD",
		},
	})

	if !strings.Contains(result, "[Success]") {
		t.Error("Expected success status")
	}
	if !strings.Contains(result, "git log") {
		t.Error("Expected 'git log' in output")
	}
}

func TestFormatToolStatus_GitShow(t *testing.T) {
	result := formatToolStatus("git_show", &tools.ToolResult{
		Success: true,
		Output:  "commit abc123\n    Initial commit",
		Extra: map[string]interface{}{
			"commitReference": "HEAD",
		},
	})

	if !strings.Contains(result, "[Success]") {
		t.Error("Expected success status")
	}
	if !strings.Contains(result, "git show") {
		t.Error("Expected 'git show' in output")
	}
}

func TestFormatToolStatus_GitDiff(t *testing.T) {
	result := formatToolStatus("git_diff", &tools.ToolResult{
		Success: true,
		Output:  "diff --git a/file.txt b/file.txt",
		Extra: map[string]interface{}{
			"reference1": "HEAD",
			"reference2": "HEAD~1",
		},
	})

	if !strings.Contains(result, "[Success]") {
		t.Error("Expected success status")
	}
	if !strings.Contains(result, "git diff") {
		t.Error("Expected 'git diff' in output")
	}
}

func TestStreamToolCallWithFullParams_Bash(t *testing.T) {
	var received []inference.StreamingChunk
	cb := func(chunk inference.StreamingChunk) {
		received = append(received, chunk)
	}

	tc := &tools.ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "ls -la /tmp",
		},
	}

	streamToolCallWithFullParams(tc, cb)

	if len(received) != 1 {
		t.Fatalf("Expected 1 chunk, got %d", len(received))
	}
	if !strings.Contains(received[0].Text, "[Bash]") {
		t.Errorf("Expected '[Bash]' in chunk, got '%s'", received[0].Text)
	}
	if !strings.Contains(received[0].Text, "ls -la") {
		t.Errorf("Expected 'ls -la' in chunk, got '%s'", received[0].Text)
	}
}

func TestStreamToolCallWithFullParams_ReadFile(t *testing.T) {
	var received []inference.StreamingChunk
	cb := func(chunk inference.StreamingChunk) {
		received = append(received, chunk)
	}

	tc := &tools.ToolCall{
		Name: "read_file",
		Parameters: map[string]interface{}{
			"path": "/tmp/test.txt",
		},
	}

	streamToolCallWithFullParams(tc, cb)

	if len(received) != 1 {
		t.Fatalf("Expected 1 chunk, got %d", len(received))
	}
	if !strings.Contains(received[0].Text, "[Read]") {
		t.Errorf("Expected '[Read]' in chunk, got '%s'", received[0].Text)
	}
}

func TestStreamToolCallWithFullParams_WriteFile(t *testing.T) {
	var received []inference.StreamingChunk
	cb := func(chunk inference.StreamingChunk) {
		received = append(received, chunk)
	}

	tc := &tools.ToolCall{
		Name: "write_file",
		Parameters: map[string]interface{}{
			"path":    "/tmp/output.txt",
			"content": "hello",
		},
	}

	streamToolCallWithFullParams(tc, cb)

	if len(received) != 1 {
		t.Fatalf("Expected 1 chunk, got %d", len(received))
	}
	if !strings.Contains(received[0].Text, "[Write]") {
		t.Errorf("Expected '[Write]' in chunk, got '%s'", received[0].Text)
	}
}

func TestStreamToolCallWithFullParams_UnknownTool(t *testing.T) {
	var received []inference.StreamingChunk
	cb := func(chunk inference.StreamingChunk) {
		received = append(received, chunk)
	}

	tc := &tools.ToolCall{
		Name: "unknown_tool",
		Parameters: map[string]interface{}{
			"param1": "value1",
		},
	}

	streamToolCallWithFullParams(tc, cb)

	if len(received) != 1 {
		t.Fatalf("Expected 1 chunk, got %d", len(received))
	}
	if !strings.Contains(received[0].Text, "[Tool: unknown_tool]") {
		t.Errorf("Expected '[Tool: unknown_tool]' in chunk, got '%s'", received[0].Text)
	}
}

func TestFormatFullToolParams_Empty(t *testing.T) {
	result := formatFullToolParams(map[string]interface{}{})
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}
}

func TestFormatFullToolParams_Nil(t *testing.T) {
	result := formatFullToolParams(nil)
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}
}

func TestFormatFullToolParams_StringValue(t *testing.T) {
	params := map[string]interface{}{
		"command": "ls -la",
	}
	result := formatFullToolParams(params)
	if !strings.Contains(result, "command") {
		t.Errorf("Expected 'command' in result, got '%s'", result)
	}
}

func TestFormatFullToolParams_MultipleValues(t *testing.T) {
	params := map[string]interface{}{
		"path":    "/tmp/test.txt",
		"content": "hello",
		"count":   float64(5),
	}
	result := formatFullToolParams(params)
	if !strings.Contains(result, "path") {
		t.Errorf("Expected 'path' in result, got '%s'", result)
	}
	if !strings.Contains(result, "content") {
		t.Errorf("Expected 'content' in result, got '%s'", result)
	}
}

func TestFormatParamValue_String(t *testing.T) {
	result := formatParamValue("hello")
	if result != `"hello"` {
		t.Errorf("Expected '\"hello\"', got '%s'", result)
	}
}

func TestFormatParamValue_Int(t *testing.T) {
	result := formatParamValue(float64(42))
	if result != "42" {
		t.Errorf("Expected '42', got '%s'", result)
	}
}

func TestFormatParamValue_Float(t *testing.T) {
	result := formatParamValue(float64(3.14))
	if result != "3.1" {
		t.Errorf("Expected '3.1', got '%s'", result)
	}
}

func TestFormatParamValue_Bool(t *testing.T) {
	result := formatParamValue(true)
	if result != "true" {
		t.Errorf("Expected 'true', got '%s'", result)
	}
}

func TestFormatParamValue_Nil(t *testing.T) {
	result := formatParamValue(nil)
	if result != "null" {
		t.Errorf("Expected 'null', got '%s'", result)
	}
}

func TestFormatParamValue_Map(t *testing.T) {
	result := formatParamValue(map[string]interface{}{"key": "value"})
	if result == "" {
		t.Error("Expected non-empty result for map")
	}
}

func TestFormatParamValue_Array(t *testing.T) {
	result := formatParamValue([]interface{}{"a", "b"})
	if result == "" {
		t.Error("Expected non-empty result for array")
	}
	if !strings.Contains(result, "a") {
		t.Errorf("Expected 'a' in result, got '%s'", result)
	}
}

