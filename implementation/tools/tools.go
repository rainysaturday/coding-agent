package tools

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Tool defines the interface for all tools
type Tool interface {
	// Name returns the tool name
	Name() string

	// Description returns a human-readable description
	Description() string

	// Execute executes the tool with the given parameters
	Execute(params map[string]string) ToolResult
}

// ToolRegistry holds all available tools
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register registers a tool
func (r *ToolRegistry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

// Get returns a tool by name
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tool names
func (r *ToolRegistry) List() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// ToolCallRequest represents the JSON structure for a tool call
type ToolCallRequest struct {
	Name       string            `json:"name"`
	Parameters map[string]string `json:"parameters"`
}

// ToolCall represents a parsed tool call
type ToolCall struct {
	Name   string
	Params map[string]string
	Raw    string
}

// ParseToolCall parses a tool call string into a ToolCall struct
// Format: [TOOL:{"name":"tool_name","parameters":{"param1":"value1",...}}]
func ParseToolCall(input string) (*ToolCall, error) {
	// Find the opening [TOOL:
	startIdx := strings.Index(input, "[TOOL:")
	if startIdx == -1 {
		return nil, fmt.Errorf("invalid tool call format: %s", input)
	}

	// Find the matching closing ]
	// We need to handle nested braces in JSON, accounting for strings
	jsonStart := startIdx + 6 // len("[TOOL:")
	endIdx := findMatchingJSONEnd(input, jsonStart)

	if endIdx == -1 {
		return nil, fmt.Errorf("invalid tool call format: %s", input)
	}

	// Extract JSON - endIdx points to the closing ]
	jsonStr := input[jsonStart:endIdx]

	// Parse JSON
	var request ToolCallRequest
	if err := json.Unmarshal([]byte(jsonStr), &request); err != nil {
		return nil, fmt.Errorf("invalid JSON in tool call: '%w' for tool call %s", err, input)
	}

	if request.Name == "" {
		return nil, fmt.Errorf("tool name is required")
	}

	// Convert parameters to map[string]string format
	params := make(map[string]string)
	for k, v := range request.Parameters {
		params[k] = v
	}

	return &ToolCall{
		Name:   request.Name,
		Params: params,
		Raw:    input,
	}, nil
}

// findMatchingJSONEnd finds the end index of a JSON object starting at the given position
// Properly handles nested objects, arrays, and strings with special characters
// Returns the index of the closing ']' if found, or -1 if not found
func findMatchingJSONEnd(s string, start int) int {
	if start >= len(s) {
		return -1
	}

	// The character at start should be '{' for valid JSON
	if s[start] != '{' {
		return -1
	}

	braceCount := 0
	inString := false
	escapeNext := false

	for i := start; i < len(s); i++ {
		c := s[i]

		if escapeNext {
			escapeNext = false
			continue
		}

		if c == '\\' && inString {
			escapeNext = true
			continue
		}

		if c == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if c == '{' {
			braceCount++
		} else if c == '}' {
			braceCount--
			if braceCount == 0 {
				// Found the matching close brace
				// Return index of ']' if present
				if i+1 < len(s) && s[i+1] == ']' {
					return i + 1
				}
				return i
			}
		}
	}

	return -1
}

// FormatToolCall formats a tool call from name and parameters using JSON format
// Uses JSON escaping for special characters in values
func FormatToolCall(name string, params map[string]string) string {
	// Convert to ToolCallRequest format
	request := ToolCallRequest{
		Name:       name,
		Parameters: params,
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(request)
	if err != nil {
		// Fallback to manual formatting if JSON marshal fails
		return fmt.Sprintf("[TOOL:{\"name\":\"%s\",\"parameters\":{}}]", name)
	}

	return fmt.Sprintf("[TOOL:%s]", string(jsonBytes))
}

// ExtractToolCalls extracts all tool calls from a text
// Supports the JSON-based format: [TOOL:{...}]
// Skips malformed tool calls and continues searching for valid ones
func ExtractToolCalls(text string) ([]*ToolCall, error) {
	calls := make([]*ToolCall, 0)

	// Find all [TOOL:... patterns
	searchStart := 0
	for {
		startIdx := strings.Index(text[searchStart:], "[TOOL:")
		if startIdx == -1 {
			break
		}
		startIdx += searchStart // Adjust to global index

		// Find the matching closing ]
		jsonStart := startIdx + 6 // len("[TOOL:")
		endIdx := findMatchingJSONEnd(text, jsonStart)

		if endIdx == -1 {
			// No valid closing found, move past this [TOOL:
			searchStart = startIdx + 1
			continue
		}

		// Extract the tool call
		toolCallStr := text[startIdx : endIdx+1]
		call, err := ParseToolCall(toolCallStr)
		if err == nil {
			calls = append(calls, call)
		}
		// Skip malformed tool calls and continue searching

		// Move past this tool call attempt
		searchStart = endIdx + 1
	}

	return calls, nil
}

// FormatToolResult formats a tool result for context (for LLM)
func FormatToolResult(toolName string, result ToolResult) string {
	if result.Success {
		jsonData, _ := json.MarshalIndent(result, "", "  ")
		return fmt.Sprintf("Tool '%s' executed successfully:\n%s", toolName, string(jsonData))
	}
	return fmt.Sprintf("Tool '%s' failed: %s", toolName, result.Error)
}

// GetRelevantParameter returns a brief parameter summary for TUI display
func GetRelevantParameter(toolName string, params map[string]string) string {
	switch toolName {
	case "bash":
		if cmd, ok := params["command"]; ok {
			if len(cmd) > 40 {
				return "command: \"" + cmd[:37] + "...\""
			}
			return "command: \"" + cmd + "\""
		}
	case "read_file", "write_file":
		if path, ok := params["path"]; ok {
			return "path: \"" + path + "\""
		}
	case "read_lines":
		if path, ok := params["path"]; ok {
			start, _ := params["start"]
			end, _ := params["end"]
			return fmt.Sprintf("path: \"%s\", lines: %s-%s", path, start, end)
		}
	case "insert_lines":
		if path, ok := params["path"]; ok {
			line, _ := params["line"]
			return fmt.Sprintf("path: \"%s\", line: %s", path, line)
		}
	case "replace_lines":
		if path, ok := params["path"]; ok {
			start, _ := params["start"]
			end, _ := params["end"]
			return fmt.Sprintf("path: \"%s\", lines: %s-%s", path, start, end)
		}
	}
	return ""
}

// TruncateOutput truncates output for TUI display with ellipsis indication
func TruncateOutput(output string, maxLen int) string {
	if output == "" {
		return ""
	}

	lines := strings.Split(output, "\n")
	var result []string
	totalLen := 0

	for _, line := range lines {
		if totalLen+len(line) > maxLen {
			remaining := maxLen - totalLen
			if remaining > 10 {
				result = append(result, line[:remaining-3]+"...")
			}
			result = append(result, fmt.Sprintf("[... truncated %d characters ...]", len(output)-totalLen))
			break
		}
		result = append(result, line)
		totalLen += len(line) + 1 // +1 for newline
	}

	return strings.Join(result, "\n")
}

// ValidateToolCall validates a tool call against expected parameters
func ValidateToolCall(toolName string, params map[string]string, requiredParams []string) error {
	for _, param := range requiredParams {
		if _, ok := params[param]; !ok {
			return fmt.Errorf("missing required parameter: %s", param)
		}
	}
	return nil
}

// ParseNumericParam parses a parameter as a number with validation
func ParseNumericParam(params map[string]string, key string) (int, error) {
	val, ok := params[key]
	if !ok {
		return 0, fmt.Errorf("missing parameter: %s", key)
	}
	num, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid numeric value for %s: %s", key, val)
	}
	return num, nil
}
