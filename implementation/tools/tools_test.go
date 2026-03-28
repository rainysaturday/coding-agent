package tools

import (
	"strings"
	"testing"
)

func TestParseToolCall_Bash(t *testing.T) {
	input := `[TOOL:{"name":"bash","parameters":{"command":"ls -la /home"}}]`
	call, err := ParseToolCall(input)
	if err != nil {
		t.Fatalf("Failed to parse tool call: %v", err)
	}

	if call.Name != "bash" {
		t.Errorf("Expected tool name 'bash', got '%s'", call.Name)
	}
	if call.Params["command"] != "ls -la /home" {
		t.Errorf("Expected command 'ls -la /home', got '%s'", call.Params["command"])
	}
}

func TestParseToolCall_ReadFile(t *testing.T) {
	input := `[TOOL:{"name":"read_file","parameters":{"path":"/path/to/file.txt"}}]`
	call, err := ParseToolCall(input)
	if err != nil {
		t.Fatalf("Failed to parse tool call: %v", err)
	}

	if call.Name != "read_file" {
		t.Errorf("Expected tool name 'read_file', got '%s'", call.Name)
	}
	if call.Params["path"] != "/path/to/file.txt" {
		t.Errorf("Expected path '/path/to/file.txt', got '%s'", call.Params["path"])
	}
}

func TestParseToolCall_WriteFile(t *testing.T) {
	input := `[TOOL:{"name":"write_file","parameters":{"path":"/path/to/file.txt","content":"Hello World"}}]`
	call, err := ParseToolCall(input)
	if err != nil {
		t.Fatalf("Failed to parse tool call: %v", err)
	}

	if call.Name != "write_file" {
		t.Errorf("Expected tool name 'write_file', got '%s'", call.Name)
	}
	if call.Params["path"] != "/path/to/file.txt" {
		t.Errorf("Expected path '/path/to/file.txt', got '%s'", call.Params["path"])
	}
	if call.Params["content"] != "Hello World" {
		t.Errorf("Expected content 'Hello World', got '%s'", call.Params["content"])
	}
}

func TestParseToolCall_ReadLines(t *testing.T) {
	input := `[TOOL:{"name":"read_lines","parameters":{"path":"/path/to/file.txt","start":"1","end":"10"}}]`
	call, err := ParseToolCall(input)
	if err != nil {
		t.Fatalf("Failed to parse tool call: %v", err)
	}

	if call.Name != "read_lines" {
		t.Errorf("Expected tool name 'read_lines', got '%s'", call.Name)
	}
	if call.Params["path"] != "/path/to/file.txt" {
		t.Errorf("Expected path '/path/to/file.txt', got '%s'", call.Params["path"])
	}
	if call.Params["start"] != "1" {
		t.Errorf("Expected start '1', got '%s'", call.Params["start"])
	}
	if call.Params["end"] != "10" {
		t.Errorf("Expected end '10', got '%s'", call.Params["end"])
	}
}

func TestParseToolCall_InsertLines(t *testing.T) {
	input := `[TOOL:{"name":"insert_lines","parameters":{"path":"/path/to/file.txt","line":"5","lines":"new line 1\nnew line 2"}}]`
	call, err := ParseToolCall(input)
	if err != nil {
		t.Fatalf("Failed to parse tool call: %v", err)
	}

	if call.Name != "insert_lines" {
		t.Errorf("Expected tool name 'insert_lines', got '%s'", call.Name)
	}
	if call.Params["line"] != "5" {
		t.Errorf("Expected line '5', got '%s'", call.Params["line"])
	}
	expectedContent := "new line 1\nnew line 2"
	if call.Params["lines"] != expectedContent {
		t.Errorf("Expected lines to contain newline, got '%s'", call.Params["lines"])
	}
}

func TestParseToolCall_ReplaceLines(t *testing.T) {
	input := `[TOOL:{"name":"replace_lines","parameters":{"path":"/path/to/file.txt","start":"1","end":"5","lines":"replacement"}}]`
	call, err := ParseToolCall(input)
	if err != nil {
		t.Fatalf("Failed to parse tool call: %v", err)
	}

	if call.Name != "replace_lines" {
		t.Errorf("Expected tool name 'replace_lines', got '%s'", call.Name)
	}
	if call.Params["start"] != "1" {
		t.Errorf("Expected start '1', got '%s'", call.Params["start"])
	}
	if call.Params["end"] != "5" {
		t.Errorf("Expected end '5', got '%s'", call.Params["end"])
	}
}

func TestParseToolCall_InvalidFormat(t *testing.T) {
	tests := []string{
		"not a tool call",
		"[TOOL:]",
		"[TOOL:]",
		"TOOL:{\"name\":\"bash\"}",
		"old format [tool:bash(command=\"test\")]",
	}

	for _, input := range tests {
		_, err := ParseToolCall(input)
		if err == nil {
			t.Errorf("Expected error for input '%s', got nil", input)
		}
	}
}

func TestParseToolCall_EmptyParams(t *testing.T) {
	input := `[TOOL:{"name":"bash","parameters":{}}]`
	call, err := ParseToolCall(input)
	if err != nil {
		t.Fatalf("Failed to parse tool call: %v", err)
	}

	if call.Name != "bash" {
		t.Errorf("Expected tool name 'bash', got '%s'", call.Name)
	}
	if len(call.Params) != 0 {
		t.Errorf("Expected empty params, got %v", call.Params)
	}
}

func TestParseToolCall_MissingName(t *testing.T) {
	input := `[TOOL:{"parameters":{"command":"test"}}]`
	_, err := ParseToolCall(input)
	if err == nil {
		t.Error("Expected error for missing name")
	}
}

func TestParseToolCall_InvalidJSON(t *testing.T) {
	input := `[TOOL:{not valid json}]`
	_, err := ParseToolCall(input)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestFormatToolCall_Bash(t *testing.T) {
	params := map[string]string{"command": "ls -la"}
	result := FormatToolCall("bash", params)
	
	// Check that the result contains the tool name and parameters
	if !strings.Contains(result, "bash") {
		t.Errorf("Expected result to contain 'bash'")
	}
	if !strings.Contains(result, "command") {
		t.Errorf("Expected result to contain 'command'")
	}
	if !strings.Contains(result, "ls -la") {
		t.Errorf("Expected result to contain 'ls -la'")
	}
	if !strings.HasPrefix(result, "[TOOL:") || !strings.HasSuffix(result, "]") {
		t.Errorf("Expected result to be wrapped in [TOOL:...]: %s", result)
	}
}

func TestFormatToolCall_WriteFile(t *testing.T) {
	params := map[string]string{
		"path":    "/tmp/test.txt",
		"content": "Hello World",
	}
	result := FormatToolCall("write_file", params)

	// Check that the result contains the tool name and parameters
	if !strings.Contains(result, "write_file") {
		t.Errorf("Expected result to contain 'write_file'")
	}
	if !strings.Contains(result, "path") {
		t.Errorf("Expected result to contain 'path'")
	}
	if !strings.Contains(result, "content") {
		t.Errorf("Expected result to contain 'content'")
	}
}

func TestFormatToolCall_MultilineContent(t *testing.T) {
	params := map[string]string{
		"content": "line1\nline2\nline3",
	}
	result := FormatToolCall("write_file", params)
	
	// Multiline content should be JSON-escaped
	if !strings.Contains(result, "\\n") {
		t.Errorf("Expected escaped newlines in output, got '%s'", result)
	}
}

func TestFormatToolCall_SpecialChars(t *testing.T) {
	params := map[string]string{
		"command": `echo "Hello \"World\""`,
	}
	result := FormatToolCall("bash", params)
	
	// Check that quotes are properly escaped
	if !strings.Contains(result, "\\\"") {
		t.Errorf("Expected escaped quotes in output")
	}
}

func TestExtractToolCalls(t *testing.T) {
	text := `
Here is my response:
[TOOL:{"name":"bash","parameters":{"command":"ls -la"}}]
And then I'll read the file:
[TOOL:{"name":"read_file","parameters":{"path":"/tmp/test.txt"}}]
`
	calls, err := ExtractToolCalls(text)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	if len(calls) != 2 {
		t.Errorf("Expected 2 tool calls, got %d", len(calls))
	}

	if calls[0].Name != "bash" {
		t.Errorf("Expected first call to be 'bash', got '%s'", calls[0].Name)
	}
	if calls[1].Name != "read_file" {
		t.Errorf("Expected second call to be 'read_file', got '%s'", calls[1].Name)
	}
}

func TestExtractToolCalls_None(t *testing.T) {
	text := "This is just regular text with no tool calls."
	calls, err := ExtractToolCalls(text)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	if len(calls) != 0 {
		t.Errorf("Expected 0 tool calls, got %d", len(calls))
	}
}

func TestExtractToolCalls_Multiline(t *testing.T) {
	text := `
I'll write a script:
[TOOL:{"name":"write_file","parameters":{"path":"/tmp/test.sh","content":"#!/bin/bash\necho hello"}}]
Done.
`
	calls, err := ExtractToolCalls(text)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	if len(calls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Name != "write_file" {
		t.Errorf("Expected tool name 'write_file', got '%s'", calls[0].Name)
	}
}

func TestFormatToolResult_Success(t *testing.T) {
	result := ToolResult{
		Success: true,
		Output:  "test output",
	}
	formatted := FormatToolResult("bash", result)

	if !strings.Contains(formatted, "bash") {
		t.Error("Expected formatted result to contain tool name")
	}
	if !strings.Contains(formatted, "successfully") {
		t.Error("Expected formatted result to contain 'successfully'")
	}
	if !strings.Contains(formatted, "test output") {
		t.Error("Expected formatted result to contain output")
	}
}

func TestFormatToolResult_Failure(t *testing.T) {
	result := ToolResult{
		Success: false,
		Error:   "permission denied",
	}
	formatted := FormatToolResult("read_file", result)

	if !strings.Contains(formatted, "read_file") {
		t.Error("Expected formatted result to contain tool name")
	}
	if !strings.Contains(formatted, "failed") {
		t.Error("Expected formatted result to contain 'failed'")
	}
	if !strings.Contains(formatted, "permission denied") {
		t.Error("Expected formatted result to contain error message")
	}
}

func TestToolRegistry(t *testing.T) {
	registry := NewToolRegistry()

	// Register a mock tool
	registry.Register(&mockTool{name: "test"})

	// Get the tool
	tool, ok := registry.Get("test")
	if !ok {
		t.Fatal("Expected to find 'test' tool")
	}
	if tool.Name() != "test" {
		t.Errorf("Expected tool name 'test', got '%s'", tool.Name())
	}

	// List tools
	tools := registry.List()
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(tools))
	}
}

func TestToolRegistry_GetNonExistent(t *testing.T) {
	registry := NewToolRegistry()
	_, ok := registry.Get("nonexistent")
	if ok {
		t.Error("Expected to not find 'nonexistent' tool")
	}
}

func TestGetRelevantParameter_Bash(t *testing.T) {
	params := map[string]string{"command": "ls -la /home/user/documents"}
	result := GetRelevantParameter("bash", params)
	expected := "command: \"ls -la /home/user/documents\""
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestGetRelevantParameter_Bash_LongCommand(t *testing.T) {
	longCmd := "this is a very long command that should be truncated for display purposes"
	params := map[string]string{"command": longCmd}
	result := GetRelevantParameter("bash", params)
	
	if !strings.Contains(result, "command:") {
		t.Error("Expected result to contain 'command:'")
	}
	if !strings.Contains(result, "...\"") {
		t.Error("Expected result to indicate truncation with '...'")
	}
}

func TestGetRelevantParameter_ReadFile(t *testing.T) {
	params := map[string]string{"path": "/path/to/file.txt"}
	result := GetRelevantParameter("read_file", params)
	expected := "path: \"/path/to/file.txt\""
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestGetRelevantParameter_ReadLines(t *testing.T) {
	params := map[string]string{"path": "/path/to/file.txt", "start": "1", "end": "10"}
	result := GetRelevantParameter("read_lines", params)
	expected := "path: \"/path/to/file.txt\", lines: 1-10"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestGetRelevantParameter_UnknownTool(t *testing.T) {
	params := map[string]string{"unknown": "value"}
	result := GetRelevantParameter("unknown_tool", params)
	if result != "" {
		t.Errorf("Expected empty result for unknown tool, got '%s'", result)
	}
}

func TestGetRelevantParameter_MissingParam(t *testing.T) {
	params := map[string]string{}
	result := GetRelevantParameter("bash", params)
	if result != "" {
		t.Errorf("Expected empty result for missing param, got '%s'", result)
	}
}

func TestTruncateOutput_Empty(t *testing.T) {
	result := TruncateOutput("", 100)
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}
}

func TestTruncateOutput_Short(t *testing.T) {
	output := "short output"
	result := TruncateOutput(output, 100)
	if result != output {
		t.Errorf("Expected '%s', got '%s'", output, result)
	}
}

func TestTruncateOutput_Long(t *testing.T) {
	output := "line1\nline2\nline3\nline4\nline5"
	result := TruncateOutput(output, 15)
	
	if !strings.Contains(result, "line1") {
		t.Error("Expected result to contain first line")
	}
	if !strings.Contains(result, "truncated") {
		t.Error("Expected result to indicate truncation")
	}
}

func TestTruncateOutput_Multiline(t *testing.T) {
	lines := make([]string, 100)
	for i := 0; i < 100; i++ {
		lines[i] = "line " + string(rune('0'+i))
	}
	output := strings.Join(lines, "\n")
	
	result := TruncateOutput(output, 50)
	
	if !strings.Contains(result, "line 0") {
		t.Error("Expected result to contain first line")
	}
	if !strings.Contains(result, "truncated") {
		t.Error("Expected result to indicate truncation")
	}
}

type mockTool struct {
	name string
}

func (m *mockTool) Name() string                         { return m.name }
func (m *mockTool) Description() string                  { return "mock tool" }
func (m *mockTool) Execute(params map[string]string) ToolResult {
	return ToolResult{Success: true}
}

func TestValidateToolCall(t *testing.T) {
	params := map[string]string{"path": "/test.txt", "start": "1"}
	
	// Valid case
	err := ValidateToolCall("read_lines", params, []string{"path", "start"})
	if err != nil {
		t.Errorf("Expected no error for valid params, got %v", err)
	}
	
	// Invalid case - missing param
	err = ValidateToolCall("read_lines", params, []string{"path", "start", "end"})
	if err == nil {
		t.Error("Expected error for missing 'end' parameter")
	}
}

func TestParseNumericParam(t *testing.T) {
	params := map[string]string{"start": "10", "end": "20"}
	
	// Valid case
	num, err := ParseNumericParam(params, "start")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if num != 10 {
		t.Errorf("Expected 10, got %d", num)
	}
	
	// Invalid case - missing
	_, err = ParseNumericParam(params, "missing")
	if err == nil {
		t.Error("Expected error for missing param")
	}
	
	// Invalid case - not numeric
	params["invalid"] = "not-a-number"
	_, err = ParseNumericParam(params, "invalid")
	if err == nil {
		t.Error("Expected error for non-numeric value")
	}
}
