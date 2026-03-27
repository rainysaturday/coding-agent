package tools

import (
	"encoding/json"
	"fmt"
	"regexp"
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

// ToolCall represents a parsed tool call
type ToolCall struct {
	Name   string
	Params map[string]string
	Raw    string
}

// ParseToolCall parses a tool call string into a ToolCall struct
// Format: [tool:tool_name(param_name="param_value", ...)]
// Supports raw mode with <<<RAW>>> and <<<END_RAW>>> markers
func ParseToolCall(input string) (*ToolCall, error) {
	// Pattern to match tool calls
	// [tool:tool_name(param_name="param_value", param_name2="param_value2")]
	// Use [\s\S]* to match newlines in raw mode
	pattern := regexp.MustCompile(`\[tool:(\w+)\(([\s\S]*)\)\]`)
	match := pattern.FindStringSubmatch(input)

	if match == nil {
		return nil, fmt.Errorf("invalid tool call format: %s", input)
	}

	name := match[1]
	paramsStr := match[2]

	params := make(map[string]string)
	if paramsStr != "" {
		// Parse parameters
		err := parseParams(paramsStr, params)
		if err != nil {
			return nil, fmt.Errorf("invalid tool parameters: %v", err)
		}
	}

	return &ToolCall{
		Name:   name,
		Params: params,
		Raw:    input,
	}, nil
}

// Raw mode markers
const (
	RawStartMarker = "<<<RAW>>>"
	RawEndMarker   = "<<<END_RAW>>>"
)

// parseParams parses parameter string into a map
func parseParams(paramsStr string, params map[string]string) error {
	// Handle empty string
	paramsStr = strings.TrimSpace(paramsStr)
	if paramsStr == "" {
		return nil
	}

	// Check for raw mode in the entire params string
	if strings.Contains(paramsStr, RawStartMarker) {
		return parseRawParams(paramsStr, params)
	}

	// Split by comma, but be careful with quoted strings
	var currentParam strings.Builder
	var inQuotes bool
	var quoteChar rune
	escaped := false

	for _, r := range paramsStr {
		if escaped {
			currentParam.WriteRune(r)
			escaped = false
			continue
		}

		if r == '\\' {
			currentParam.WriteRune(r)
			escaped = true
			continue
		}

		if r == '"' || r == '\'' {
			if !inQuotes {
				inQuotes = true
				quoteChar = r
				currentParam.WriteRune(r)
			} else if r == quoteChar {
				inQuotes = false
				quoteChar = 0
				currentParam.WriteRune(r)
			} else {
				currentParam.WriteRune(r)
			}
			continue
		}

		if r == ',' && !inQuotes {
			paramStr := strings.TrimSpace(currentParam.String())
			if paramStr != "" {
				if err := parseSingleParam(paramStr, params); err != nil {
					return err
				}
			}
			currentParam.Reset()
			continue
		}

		currentParam.WriteRune(r)
	}

	// Parse the last parameter
	paramStr := strings.TrimSpace(currentParam.String())
	if paramStr != "" {
		if err := parseSingleParam(paramStr, params); err != nil {
			return err
		}
	}

	return nil
}

// parseRawParams parses parameters with raw mode content
// Format: key=<<<RAW>>>...content...<<<END_RAW>>>
func parseRawParams(paramsStr string, params map[string]string) error {
	// Split into segments by finding raw mode blocks and standard segments
	remaining := paramsStr
	
	for len(remaining) > 0 {
		remaining = strings.TrimSpace(remaining)
		if remaining == "" {
			break
		}
		
		// Find the next raw start marker
		rawStartIdx := strings.Index(remaining, RawStartMarker)
		
		if rawStartIdx == -1 {
			// No more raw mode, parse remaining as standard params
			// Split by comma and parse each
			parts := splitParams(remaining)
			for _, part := range parts {
				if err := parseSingleParam(strings.TrimSpace(part), params); err != nil {
					return err
				}
			}
			break
		}
		
		// Parse standard params before the raw marker
		if rawStartIdx > 0 {
			beforeRaw := remaining[:rawStartIdx]
			beforeRaw = strings.TrimRight(beforeRaw, " \t,")
			if beforeRaw != "" {
				parts := splitParams(beforeRaw)
				for _, part := range parts {
					if err := parseSingleParam(strings.TrimSpace(part), params); err != nil {
						return err
					}
				}
			}
		}
		
		// Find the key for the raw parameter
		beforeMarker := remaining[:rawStartIdx]
		beforeMarker = strings.TrimRight(beforeMarker, " \t,")
		
		// Find the last comma to isolate this parameter
		lastCommaIdx := strings.LastIndex(beforeMarker, ",")
		var thisParam string
		if lastCommaIdx == -1 {
			thisParam = beforeMarker
		} else {
			thisParam = beforeMarker[lastCommaIdx+1:]
		}
		
		// Extract key from "key=" 
		equalsIdx := strings.Index(thisParam, "=")
		key := ""
		if equalsIdx != -1 {
			key = strings.TrimSpace(thisParam[:equalsIdx])
		}
		
		// Find the end marker
		contentStart := rawStartIdx + len(RawStartMarker)
		endMarkerIdx := strings.Index(remaining[contentStart:], RawEndMarker)
		if endMarkerIdx == -1 {
			return fmt.Errorf("raw mode missing end marker")
		}
		
		contentEnd := contentStart + endMarkerIdx
		content := remaining[contentStart:contentEnd]
		
		// Trim leading and trailing newlines
		content = strings.TrimPrefix(content, "\n")
		content = strings.TrimSuffix(content, "\n")
		
		if key != "" {
			params[key] = content
		}
		
		// Move past the end marker
		remaining = remaining[contentEnd+len(RawEndMarker):]
	}
	
	return nil
}

// splitParams splits parameter string by commas, respecting quotes
func splitParams(paramsStr string) []string {
	var result []string
	var current strings.Builder
	inQuotes := false
	quoteChar := byte(0)
	
	for i := 0; i < len(paramsStr); i++ {
		b := paramsStr[i]
		
		if inQuotes {
			current.WriteByte(b)
			if b == quoteChar {
				inQuotes = false
			}
			continue
		}
		
		if b == '"' || b == '\'' {
			inQuotes = true
			quoteChar = b
			current.WriteByte(b)
			continue
		}
		
		if b == ',' {
			result = append(result, current.String())
			current.Reset()
			continue
		}
		
		current.WriteByte(b)
	}
	
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	
	return result
}

// parseSingleParam parses a single parameter like key="value" or key=123
func parseSingleParam(paramStr string, params map[string]string) error {
	parts := strings.SplitN(paramStr, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid parameter format: %s", paramStr)
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	// Remove quotes if present
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
	}

	// Handle escaped newlines
	value = strings.ReplaceAll(value, "\\n", "\n")

	params[key] = value
	return nil
}

// FormatToolCall formats a tool call from name and parameters
// Uses raw mode for multi-line content
func FormatToolCall(name string, params map[string]string) string {
	var sb strings.Builder
	sb.WriteString("[tool:")
	sb.WriteString(name)
	sb.WriteString("(")

	first := true
	for key, value := range params {
		if !first {
			sb.WriteString(", ")
		}
		first = false

		// Check if value contains newlines - use raw mode
		if strings.Contains(value, "\n") {
			sb.WriteString(fmt.Sprintf("%s=%s\n%s\n%s", key, RawStartMarker, value, RawEndMarker))
		} else if _, err := strconv.Atoi(value); err == nil {
			// Numeric value, no quotes
			sb.WriteString(fmt.Sprintf("%s=%s", key, value))
		} else {
			// String value with quotes
			sb.WriteString(fmt.Sprintf("%s=\"%s\"", key, value))
		}
	}

	sb.WriteString(")]")
	return sb.String()
}

// ExtractToolCalls extracts all tool calls from a text
// Supports both standard mode and raw mode with multi-line content
func ExtractToolCalls(text string) ([]*ToolCall, error) {
	// Pattern for standard mode (no newlines in params)
	standardPattern := regexp.MustCompile(`\[tool:\w+\([^]]+\)\]`)
	
	// Pattern for raw mode (allows newlines between markers)
	rawPattern := regexp.MustCompile(`\[tool:\w+\([^)]*<<<RAW>>>(.*?)<<<END_RAW>>>\)[\s\S]*?\]`)
	
	// Find raw mode matches first
	rawMatches := rawPattern.FindAllString(text, -1)
	
	// Find standard mode matches
	standardMatches := standardPattern.FindAllString(text, -1)
	
	// Combine and deduplicate
	allMatches := make(map[string]bool)
	calls := make([]*ToolCall, 0)
	
	for _, match := range rawMatches {
		if !allMatches[match] {
			allMatches[match] = true
			call, err := ParseToolCall(match)
			if err == nil {
				calls = append(calls, call)
			}
		}
	}
	
	for _, match := range standardMatches {
		if !allMatches[match] {
			allMatches[match] = true
			call, err := ParseToolCall(match)
			if err == nil {
				calls = append(calls, call)
			}
		}
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
