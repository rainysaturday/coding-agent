// Package tools implements the tool execution system for the coding agent.
// This file contains the write_file tool implementation.
package tools

import (
	"fmt"
	"os"
	"path/filepath"
)

// executeWriteFile writes to a file.
func (te *ToolExecutor) executeWriteFile(params map[string]interface{}) *ToolResult {
	path, ok := params["path"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}
	content, ok := params["content"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: content",
		}
	}

	// Create parent directories if needed
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("cannot create directory: %v", err),
			}
		}
	}

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, path),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("File written: %s (%d bytes)\n--- Content preview ---\n%s", path, len(content), truncateOutput(content, 20)),
		Path:    path,
		Extra: map[string]interface{}{
			"message":       fmt.Sprintf("File written successfully: %s", path),
			"contentLength": len(content),
		},
	}
}
