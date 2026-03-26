package tools

import (
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
	// Multiline content should have escaped newlines
	if !contains(result, "\\n") {
		t.Errorf("Expected escaped newlines in output, got '%s'", result)
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
