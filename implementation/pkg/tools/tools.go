package tools

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Tool represents a tool that can be executed
type Tool struct {
	Name        string
	Description string
	Handler     func(map[string]interface{}) (map[string]interface{}, error)
}

// Tools is a collection of available tools
type Tools struct {
	toolMap map[string]Tool
}

// NewTools creates a new Tools instance with default tools
func NewTools() *Tools {
	t := &Tools{
		toolMap: make(map[string]Tool),
	}
	t.RegisterTool(t.BashTool())
	t.RegisterTool(t.ReadFileTool())
	t.RegisterTool(t.WriteFileTool())
	t.RegisterTool(t.ReadLineTool())
	t.RegisterTool(t.InsertLinesTool())
	t.RegisterTool(t.ReplaceLinesTool())
	return t
}

// RegisterTool registers a tool
func (t *Tools) RegisterTool(tool Tool) {
	t.toolMap[tool.Name] = tool
}

// CallTool calls a tool by name
func (t *Tools) CallTool(name string, args map[string]interface{}) (map[string]interface{}, error) {
	tool, ok := t.toolMap[name]
	if !ok {
		return nil, &ToolError{ToolName: name, Message: "tool not found"}
	}
	return tool.Handler(args)
}

// ListTools returns a list of available tools
func (t *Tools) ListTools() []string {
	tools := make([]string, 0, len(t.toolMap))
	for name := range t.toolMap {
		tools = append(tools, name)
	}
	return tools
}

// BashTool returns the bash tool definition
func (t *Tools) BashTool() Tool {
	return Tool{
		Name: "bash",
		Description: "Execute a bash command",
		Handler: func(args map[string]interface{}) (map[string]interface{}, error) {
			cmdStr, ok := args["command"].(string)
			if !ok {
				return nil, &ToolError{ToolName: "bash", Message: "command must be a string"}
			}

			parts := strings.Fields(cmdStr)
			if len(parts) == 0 {
				return nil, &ToolError{ToolName: "bash", Message: "no command provided"}
			}

			cmd := exec.Command(parts[0], parts[1:]...)
			output, err := cmd.CombinedOutput()

			result := map[string]interface{}{
				"output": string(output),
				"error":  "",
			}

			if err != nil {
				result["error"] = err.Error()
			}

			return result, nil
		},
	}
}

// ReadFileTool returns the read_file tool definition
func (t *Tools) ReadFileTool() Tool {
	return Tool{
		Name: "read_file",
		Description: "Read the contents of a file",
		Handler: func(args map[string]interface{}) (map[string]interface{}, error) {
			path, ok := args["path"].(string)
			if !ok {
				return nil, &ToolError{ToolName: "read_file", Message: "path must be a string"}
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return nil, &ToolError{ToolName: "read_file", Message: err.Error()}
			}

			return map[string]interface{}{
				"content": string(content),
				"path":    path,
			}, nil
		},
	}
}

// WriteFileTool returns the write_file tool definition
func (t *Tools) WriteFileTool() Tool {
	return Tool{
		Name: "write_file",
		Description: "Write content to a file",
		Handler: func(args map[string]interface{}) (map[string]interface{}, error) {
			path, ok := args["path"].(string)
			if !ok {
				return nil, &ToolError{ToolName: "write_file", Message: "path must be a string"}
			}

			content, ok := args["content"].(string)
			if !ok {
				return nil, &ToolError{ToolName: "write_file", Message: "content must be a string"}
			}

			err := os.WriteFile(path, []byte(content), 0644)
			if err != nil {
				return nil, &ToolError{ToolName: "write_file", Message: err.Error()}
			}

			return map[string]interface{}{
				"path":    path,
				"success": true,
			}, nil
		},
	}
}

// ReadLineTool returns the read_lines tool definition
func (t *Tools) ReadLineTool() Tool {
	return Tool{
		Name: "read_lines",
		Description: "Read a specific line range from a file",
		Handler: func(args map[string]interface{}) (map[string]interface{}, error) {
			path, ok := args["path"].(string)
			if !ok {
				return nil, &ToolError{ToolName: "read_lines", Message: "path must be a string"}
			}

			start, ok := args["start"].(float64)
			if !ok {
				return nil, &ToolError{ToolName: "read_lines", Message: "start must be a number"}
			}

			end, ok := args["end"].(float64)
			if !ok {
				return nil, &ToolError{ToolName: "read_lines", Message: "end must be a number"}
			}

			startLine := int(start)
			endLine := int(end)

			content, err := os.ReadFile(path)
			if err != nil {
				return nil, &ToolError{ToolName: "read_lines", Message: err.Error()}
			}

			lines := strings.Split(string(content), "\n")
			
			if startLine > len(lines) {
				return map[string]interface{}{
					"content": "",
					"message": "start line beyond file end",
				}, nil
			}

			if endLine > len(lines) {
				endLine = len(lines)
			}

			if startLine > endLine {
				return map[string]interface{}{
					"content": "",
					"message": "start line is greater than end line",
				}, nil
			}

			resultLines := lines[startLine-1:endLine]
			resultContent := strings.Join(resultLines, "\n")

			return map[string]interface{}{
				"content": resultContent,
				"start":   startLine,
				"end":     endLine,
			}, nil
		},
	}
}

// InsertLinesTool returns the insert_lines tool definition
func (t *Tools) InsertLinesTool() Tool {
	return Tool{
		Name: "insert_lines",
		Description: "Insert lines at a specific line number",
		Handler: func(args map[string]interface{}) (map[string]interface{}, error) {
			path, ok := args["path"].(string)
			if !ok {
				return nil, &ToolError{ToolName: "insert_lines", Message: "path must be a string"}
			}

			lineNum, ok := args["line"].(float64)
			if !ok {
				return nil, &ToolError{ToolName: "insert_lines", Message: "line must be a number"}
			}

			linesStr, ok := args["lines"].(string)
			if !ok {
				return nil, &ToolError{ToolName: "insert_lines", Message: "lines must be a string"}
			}

			insertLines := strings.Split(linesStr, "\n")
			lineNumInt := int(lineNum)

			content, err := os.ReadFile(path)
			if err != nil {
				return nil, &ToolError{ToolName: "insert_lines", Message: err.Error()}
			}

			existingLines := strings.Split(string(content), "\n")

			// Insert at beginning if line 1
			if lineNumInt <= 1 {
				existingLines = append(insertLines, existingLines...)
			} else if lineNumInt >= len(existingLines) {
				existingLines = append(existingLines, insertLines...)
			} else {
				existingLines = append(existingLines[:lineNumInt], append(insertLines, existingLines[lineNumInt:]...)...)
			}

			newContent := strings.Join(existingLines, "\n")
			err = os.WriteFile(path, []byte(newContent), 0644)
			if err != nil {
				return nil, &ToolError{ToolName: "insert_lines", Message: err.Error()}
			}

			return map[string]interface{}{
				"success": true,
				"inserted": len(insertLines),
				"at_line": lineNumInt,
			}, nil
		},
	}
}

// ReplaceLinesTool returns the replace_lines tool definition
func (t *Tools) ReplaceLinesTool() Tool {
	return Tool{
		Name: "replace_lines",
		Description: "Replace a line range with new lines",
		Handler: func(args map[string]interface{}) (map[string]interface{}, error) {
			path, ok := args["path"].(string)
			if !ok {
				return nil, &ToolError{ToolName: "replace_lines", Message: "path must be a string"}
			}

			start, ok := args["start"].(float64)
			if !ok {
				return nil, &ToolError{ToolName: "replace_lines", Message: "start must be a number"}
			}

			end, ok := args["end"].(float64)
			if !ok {
				return nil, &ToolError{ToolName: "replace_lines", Message: "end must be a number"}
			}

			linesStr, ok := args["lines"].(string)
			if !ok {
				return nil, &ToolError{ToolName: "replace_lines", Message: "lines must be a string"}
			}

			replaceLines := strings.Split(linesStr, "\n")
			startLine := int(start)
			endLine := int(end)

			content, err := os.ReadFile(path)
			if err != nil {
				return nil, &ToolError{ToolName: "replace_lines", Message: err.Error()}
			}

			existingLines := strings.Split(string(content), "\n")

			if startLine > endLine {
				return nil, &ToolError{ToolName: "replace_lines", Message: "start line is greater than end line"}
			}

			if startLine > len(existingLines) {
				existingLines = append(existingLines, replaceLines...)
			} else {
				endLine = min(endLine, len(existingLines))
				existingLines = append(existingLines[:startLine-1], append(replaceLines, existingLines[endLine:]...)...)
			}

			newContent := strings.Join(existingLines, "\n")
			err = os.WriteFile(path, []byte(newContent), 0644)
			if err != nil {
				return nil, &ToolError{ToolName: "replace_lines", Message: err.Error()}
			}

			return map[string]interface{}{
				"success": true,
				"replaced": endLine - startLine + 1,
				"with":    len(replaceLines),
			}, nil
		},
	}
}

// ToolCall represents a tool call
type ToolCall struct {
	Name string
	Args map[string]interface{}
}

// ToolError represents an error from a tool
type ToolError struct {
	ToolName string
	Message  string
}

func (e *ToolError) Error() string {
	return "tool " + e.ToolName + " error: " + e.Message
}

// FormatToolResult formats a tool result as a user message
func FormatToolResult(toolName string, result map[string]interface{}, err error) string {
	var sb strings.Builder
	
	sb.WriteString(fmt.Sprintf("Tool '%s' executed:", toolName))
	
	if err != nil {
		sb.WriteString(fmt.Sprintf(" FAILED: %v\n", err))
	} else {
		sb.WriteString(" SUCCESS\n")
		
		// Format the result as JSON-like output
		sb.WriteString("{\n")
		for k, v := range result {
			sb.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
		}
		sb.WriteString("}\n")
	}
	
	return sb.String()
}

// ExtractToolCalls extracts tool calls from user message
func ExtractToolCalls(msg string) []ToolCall {
	var calls []ToolCall
	
	// Pattern: [tool:tool_name(param1="val1", param2="val2")]
	start := strings.Index(msg, "[tool:")
	for start != -1 {
		end := strings.Index(msg[start:], "]")
		if end == -1 {
			break
		}
		end += start
		callStr := msg[start+6 : end] // Skip "[tool:"
		
		// Extract tool name
		colonIdx := strings.Index(callStr, "(")
		if colonIdx == -1 {
			start = strings.Index(msg[end+1:], "[tool:")
			if start != -1 {
				start += end + 1
			}
			continue
		}
		
		toolName := callStr[:colonIdx]
		paramsStr := callStr[colonIdx+1 : len(callStr)-1] // Skip "(" and ")"
		
		// Parse parameters
		args := parseParameters(paramsStr)
		
		calls = append(calls, ToolCall{
			Name: toolName,
			Args: args,
		})
		
		// Continue searching
		start = strings.Index(msg[end+1:], "[tool:")
		if start != -1 {
			start += end + 1
		}
	}
	
	return calls
}

// parseParameters parses the parameter string into a map
func parseParameters(paramsStr string) map[string]interface{} {
	args := make(map[string]interface{})
	
	// Simple parameter parsing
	// Format: param1="value1", param2=value2, param3="line1\nline2"
	current := ""
	inQuote := false
	quoteChar := byte(0)
	
	for i := 0; i < len(paramsStr); i++ {
		c := paramsStr[i]
		
		if !inQuote {
			if c == '"' || c == '\'' {
				inQuote = true
				quoteChar = c
				continue
			}
			if c == ',' {
				// Save parameter
				saveParam(current, args)
				current = ""
				continue
			}
		} else {
			if c == quoteChar && (i == 0 || paramsStr[i-1] != '\\') {
				inQuote = false
				quoteChar = 0
				continue
			}
		}
		
		current += string(c)
	}
	
	// Save last parameter
	if current != "" {
		saveParam(current, args)
	}
	
	return args
}

// saveParam saves a parsed parameter to the args map
func saveParam(paramStr string, args map[string]interface{}) {
	paramStr = strings.TrimSpace(paramStr)
	if paramStr == "" {
		return
	}
	
	eqIdx := strings.Index(paramStr, "=")
	if eqIdx == -1 {
		return
	}
	
	key := strings.TrimSpace(paramStr[:eqIdx])
	value := strings.TrimSpace(paramStr[eqIdx+1:])
	
	// Remove quotes if present
	if (value[0] == '"' && value[len(value)-1] == '"') ||
		(value[0] == '\'' && value[len(value)-1] == '\'') {
		value = value[1 : len(value)-1]
		// Unescape newlines
		value = strings.ReplaceAll(value, "\\n", "\n")
	}
	
	// Try to parse as integer
	if intValue, err := strconv.Atoi(value); err == nil {
		args[key] = intValue
		return
	}
	
	args[key] = value
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
