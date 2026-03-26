package tools

import (
	"os"
	"path/filepath"
)

// WriteFileTool implements file writing functionality
type WriteFileTool struct{}

// NewWriteFileTool creates a new WriteFileTool
func NewWriteFileTool() *WriteFileTool {
	return &WriteFileTool{}
}

// Name returns the tool name
func (t *WriteFileTool) Name() string {
	return "write_file"
}

// Description returns a human-readable description
func (t *WriteFileTool) Description() string {
	return "Write content to a file"
}

// Execute writes content to a file
func (t *WriteFileTool) Execute(params map[string]string) ToolResult {
	path, ok := params["path"]
	if !ok || path == "" {
		return ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	content, ok := params["content"]
	if !ok {
		return ToolResult{
			Success: false,
			Error:   "missing required parameter: content",
		}
	}

	// Ensure the directory exists
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return ToolResult{
				Success: false,
				Error:   "failed to create directory: " + err.Error(),
			}
		}
	}

	// Write the file
	err := os.WriteFile(path, []byte(content), 0644)

	result := ToolResult{
		Success: err == nil,
	}

	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Output = "File written successfully"
	return result
}
