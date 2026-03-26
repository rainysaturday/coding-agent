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
func ParseToolCall(input string) (*ToolCall, error) {
	// Pattern to match tool calls
	// [tool:tool_name(param_name="param_value", param_name2="param_value2")]
	pattern := regexp.MustCompile(`\[tool:(\w+)\((.*)\)\]`)
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

// parseParams parses parameter string into a map
func parseParams(paramsStr string, params map[string]string) error {
	// Handle empty string
	paramsStr = strings.TrimSpace(paramsStr)
	if paramsStr == "" {
		return nil
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

		// Check if value contains newlines or special chars
		if strings.Contains(value, "\n") || strings.Contains(value, "\"") {
			// Escape and quote
			escaped := strings.ReplaceAll(value, "\n", "\\n")
			escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
			sb.WriteString(fmt.Sprintf("%s=\"%s\"", key, escaped))
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
func ExtractToolCalls(text string) ([]*ToolCall, error) {
	pattern := regexp.MustCompile(`\[tool:\w+\([^]]*\)\]`)
	matches := pattern.FindAllString(text, -1)

	calls := make([]*ToolCall, 0, len(matches))
	for _, match := range matches {
		call, err := ParseToolCall(match)
		if err != nil {
			continue // Skip invalid tool calls
		}
		calls = append(calls, call)
	}

	return calls, nil
}

// FormatToolResult formats a tool result for context
func FormatToolResult(toolName string, result ToolResult) string {
	if result.Success {
		jsonData, _ := json.MarshalIndent(result, "", "  ")
		return fmt.Sprintf("Tool '%s' executed successfully:\n%s", toolName, string(jsonData))
	}
	return fmt.Sprintf("Tool '%s' failed: %s", toolName, result.Error)
}
