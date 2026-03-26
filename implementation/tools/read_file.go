package tools

import (
	"os"
)

// ReadFileTool implements file reading functionality
type ReadFileTool struct{}

// NewReadFileTool creates a new ReadFileTool
func NewReadFileTool() *ReadFileTool {
	return &ReadFileTool{}
}

// Name returns the tool name
func (t *ReadFileTool) Name() string {
	return "read_file"
}

// Description returns a human-readable description
func (t *ReadFileTool) Description() string {
	return "Read the contents of a file"
}

// Execute reads a file and returns its contents
func (t *ReadFileTool) Execute(params map[string]string) ToolResult {
	path, ok := params["path"]
	if !ok || path == "" {
		return ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	// Read the file
	content, err := os.ReadFile(path)

	result := ToolResult{
		Success: err == nil,
	}

	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Output = string(content)
	return result
}
