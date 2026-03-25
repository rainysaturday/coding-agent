package tools

import (
	"os"
	"os/exec"
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

// ToolError represents an error from a tool
type ToolError struct {
	ToolName string
	Message  string
}

func (e *ToolError) Error() string {
	return "tool " + e.ToolName + " error: " + e.Message
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
