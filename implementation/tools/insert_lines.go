// Package tools implements the tool execution system for the coding agent.
// This file contains the insert_lines tool implementation.
package tools

import (
	"fmt"
	"os"
	"strings"
)

// executeInsertLines inserts lines at a specific position.
func (te *ToolExecutor) executeInsertLines(params map[string]interface{}) *ToolResult {
	path, ok := params["path"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	insertLine, errMsg := parseIntParamStrict(params, "line")
	if errMsg != "" {
		return &ToolResult{
			Success: false,
			Error:   errMsg,
		}
	}

	insertLines, ok := params["lines"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: lines",
		}
	}

	// Use splitLines (which strips a trailing empty element) so inserted text
	// ending in a newline is handled consistently with the file's existing
	// content, rather than inserting an extra empty line.
	newLines := splitLines(insertLines)

	// Read existing content or create empty
	var existingLines []string
	content, err := os.ReadFile(path)
	if err == nil {
		existingLines = splitLines(string(content))
	}

	// Adjust to 0-indexed
	insertIdx := insertLine - 1

	// Handle edge cases
	if insertIdx < 0 {
		insertIdx = 0
	}
	if insertIdx > len(existingLines) {
		insertIdx = len(existingLines)
	}

	// Insert lines
	resultLines := make([]string, 0, len(existingLines)+len(newLines))
	resultLines = append(resultLines, existingLines[:insertIdx]...)
	resultLines = append(resultLines, newLines...)
	resultLines = append(resultLines, existingLines[insertIdx:]...)

	// Write back
	output := strings.Join(resultLines, "\n")
	if len(resultLines) > 0 {
		output += "\n"
	}

	// Create parent directories if needed
	if err := ensureDirectory(path); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("cannot create directory: %v", err),
		}
	}

	if err := WriteFilePreservePerm(path, []byte(output)); err != nil {
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, path),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Inserted %d line(s) at line %d in: %s\n--- Content inserted ---\n%s", len(newLines), insertLine, path, truncateOutput(insertLines, 10)),
		Path:    path,
		Extra: map[string]interface{}{
			"line":          insertLine,
			"linesInserted": len(newLines),
		},
	}
}
