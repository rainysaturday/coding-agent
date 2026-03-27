package tools

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseToolCall_Bash(t *testing.T) {
	input := `[tool:bash(command="ls -la /home")]`
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
	input := `[tool:read_file(path="/path/to/file.txt")]`
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
	input := `[tool:write_file(path="/path/to/file.txt", content="Hello World")]`
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
	input := `[tool:read_lines(path="/path/to/file.txt", start=1, end=10)]`
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
	input := `[tool:insert_lines(path="/path/to/file.txt", line=5, lines="new line 1\nnew line 2")]`
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
	input := `[tool:replace_lines(path="/path/to/file.txt", start=1, end=5, lines="replacement")]`
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
		"[tool:]",
		"[tool:bash]",
		"tool:bash(command=\"test\")",
	}

	for _, input := range tests {
		_, err := ParseToolCall(input)
		if err == nil {
			t.Errorf("Expected error for input '%s', got nil", input)
		}
	}
}

func TestParseToolCall_EmptyParams(t *testing.T) {
	input := `[tool:bash()]`
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

func TestFormatToolCall_Bash(t *testing.T) {
	params := map[string]string{"command": "ls -la"}
	result := FormatToolCall("bash", params)
	expected := `[tool:bash(command="ls -la")]`
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestFormatToolCall_WriteFile(t *testing.T) {
	params := map[string]string{
		"path":    "/tmp/test.txt",
		"content": "Hello World",
	}
	result := FormatToolCall("write_file", params)

	// Check that the result contains the tool name and parameters
	if !contains(result, "write_file") {
		t.Errorf("Expected result to contain 'write_file'")
	}
	if !contains(result, "path") {
		t.Errorf("Expected result to contain 'path'")
	}
	if !contains(result, "content") {
		t.Errorf("Expected result to contain 'content'")
	}
}

func TestFormatToolCall_NumericParams(t *testing.T) {
	params := map[string]string{
		"start": "1",
		"end":   "10",
	}
	result := FormatToolCall("read_lines", params)
	// Numeric values should not be quoted
	if contains(result, "start=\"1\"") {
		t.Errorf("Expected numeric params without quotes, got '%s'", result)
	}
}

func TestFormatToolCall_MultilineContent(t *testing.T) {
	params := map[string]string{
		"content": "line1\nline2\nline3",
	}
	result := FormatToolCall("write_file", params)
	// Multiline content should use raw mode
	if !contains(result, RawStartMarker) {
		t.Errorf("Expected raw mode marker in output, got '%s'", result)
	}
	if !contains(result, RawEndMarker) {
		t.Errorf("Expected raw mode end marker in output, got '%s'", result)
	}
}

func TestExtractToolCalls(t *testing.T) {
	text := `
Here is my response:
[tool:bash(command="ls -la")]
And then I'll read the file:
[tool:read_file(path="/tmp/test.txt")]
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

func TestFormatToolResult_Success(t *testing.T) {
	result := ToolResult{
		Success: true,
		Output:  "test output",
	}
	formatted := FormatToolResult("bash", result)

	if !contains(formatted, "bash") {
		t.Error("Expected formatted result to contain tool name")
	}
	if !contains(formatted, "successfully") {
		t.Error("Expected formatted result to contain 'successfully'")
	}
	if !contains(formatted, "test output") {
		t.Error("Expected formatted result to contain output")
	}
}

func TestFormatToolResult_Failure(t *testing.T) {
	result := ToolResult{
		Success: false,
		Error:   "permission denied",
	}
	formatted := FormatToolResult("read_file", result)

	if !contains(formatted, "read_file") {
		t.Error("Expected formatted result to contain tool name")
	}
	if !contains(formatted, "failed") {
		t.Error("Expected formatted result to contain 'failed'")
	}
	if !contains(formatted, "permission denied") {
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
		lines[i] = fmt.Sprintf("line %d", i)
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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

type mockTool struct {
	name string
}

func (m *mockTool) Name() string                         { return m.name }
func (m *mockTool) Description() string                  { return "mock tool" }
func (m *mockTool) Execute(params map[string]string) ToolResult {
	return ToolResult{Success: true}
}

func TestParseToolCall_RawMode_WriteFile(t *testing.T) {
	input := `[tool:write_file(path="/tmp/test.txt", content=<<<RAW>>>
line 1
line 2
line 3
<<<END_RAW>>>)]`
	call, err := ParseToolCall(input)
	if err != nil {
		t.Fatalf("Failed to parse raw mode tool call: %v", err)
	}

	if call.Name != "write_file" {
		t.Errorf("Expected tool name 'write_file', got '%s'", call.Name)
	}
	if call.Params["path"] != "/tmp/test.txt" {
		t.Errorf("Expected path '/tmp/test.txt', got '%s'", call.Params["path"])
	}
	expectedContent := "line 1\nline 2\nline 3"
	if call.Params["content"] != expectedContent {
		t.Errorf("Expected content '%s', got '%s'", expectedContent, call.Params["content"])
	}
}

func TestParseToolCall_RawMode_InsertLines(t *testing.T) {
	input := `[tool:insert_lines(path="/tmp/file.txt", line=5, lines=<<<RAW>>>
new line 1
new line 2
<<<END_RAW>>>)]`
	call, err := ParseToolCall(input)
	if err != nil {
		t.Fatalf("Failed to parse raw mode tool call: %v", err)
	}

	if call.Name != "insert_lines" {
		t.Errorf("Expected tool name 'insert_lines', got '%s'", call.Name)
	}
	if call.Params["path"] != "/tmp/file.txt" {
		t.Errorf("Expected path '/tmp/file.txt', got '%s'", call.Params["path"])
	}
	if call.Params["line"] != "5" {
		t.Errorf("Expected line '5', got '%s'", call.Params["line"])
	}
	expectedLines := "new line 1\nnew line 2"
	if call.Params["lines"] != expectedLines {
		t.Errorf("Expected lines '%s', got '%s'", expectedLines, call.Params["lines"])
	}
}

func TestParseToolCall_RawMode_MixedParams(t *testing.T) {
	// Test with multiple standard params and one raw param
	input := `[tool:write_file(path="/tmp/test.txt", mode="w", content=<<<RAW>>>
#!/bin/bash
echo "Hello"
<<<END_RAW>>>)]`
	call, err := ParseToolCall(input)
	if err != nil {
		t.Fatalf("Failed to parse raw mode tool call: %v", err)
	}

	if call.Params["path"] != "/tmp/test.txt" {
		t.Errorf("Expected path '/tmp/test.txt', got '%s'", call.Params["path"])
	}
	if call.Params["mode"] != "w" {
		t.Errorf("Expected mode 'w', got '%s'", call.Params["mode"])
	}
	expectedContent := "#!/bin/bash\necho \"Hello\""
	if call.Params["content"] != expectedContent {
		t.Errorf("Expected content '%s', got '%s'", expectedContent, call.Params["content"])
	}
}

func TestFormatToolCall_RawMode(t *testing.T) {
	params := map[string]string{
		"path":    "/tmp/script.sh",
		"content": "#!/bin/bash\necho 'Hello'",
	}
	result := FormatToolCall("write_file", params)
	
	if !contains(result, "write_file") {
		t.Error("Expected result to contain 'write_file'")
	}
	if !contains(result, RawStartMarker) {
		t.Error("Expected result to contain raw start marker")
	}
	if !contains(result, RawEndMarker) {
		t.Error("Expected result to contain raw end marker")
	}
	// Should not have escaped newlines
	if contains(result, "\\n") {
		t.Error("Expected no escaped newlines in raw mode")
	}
}

func TestExtractToolCalls_RawMode(t *testing.T) {
	text := `Here is my response:
[tool:write_file(path="/tmp/test.txt", content=<<<RAW>>>
line 1
line 2
<<<END_RAW>>>)]
And then I'll read it.`
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

func TestSplitParams(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    `path="/tmp/test.txt", content="hello"`,
			expected: []string{`path="/tmp/test.txt"`, ` content="hello"`},
		},
		{
			input:    `path="/tmp/test.txt"`,
			expected: []string{`path="/tmp/test.txt"`},
		},
		{
			input:    `a=1, b=2, c=3`,
			expected: []string{`a=1`, ` b=2`, ` c=3`},
		},
	}

	for _, tt := range tests {
		result := splitParams(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("Input %q: expected %d parts, got %d", tt.input, len(tt.expected), len(result))
			continue
		}
		for i, exp := range tt.expected {
			if result[i] != exp {
				t.Errorf("Input %q: expected part %d to be %q, got %q", tt.input, i, exp, result[i])
			}
		}
	}
}

func TestParseRawParams_Empty(t *testing.T) {
	params := make(map[string]string)
	err := parseRawParams("", params)
	if err != nil {
		t.Errorf("Expected no error for empty input, got %v", err)
	}
	if len(params) != 0 {
		t.Errorf("Expected empty params, got %v", params)
	}
}

func TestParseRawParams_MissingEndMarker(t *testing.T) {
	params := make(map[string]string)
	input := `content=<<<RAW>>>
some content
no end marker`
	err := parseRawParams(input, params)
	if err == nil {
		t.Error("Expected error for missing end marker")
	}
}
