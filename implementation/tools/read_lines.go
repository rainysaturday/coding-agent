// Package tools implements the tool execution system for the coding agent.
// This file contains the read_lines tool implementation.
package tools

import (
	"fmt"
	"os"
	"strings"
)

// maxReadLinesBlock is the maximum number of lines that read_lines will return at once.
const maxReadLinesBlock = 5000

// maxReadLinesBytes bounds the size of the returned output. Even within the line
// limit, a file with very long lines (minified/JSON, wide logs) could otherwise
// return an unbounded number of bytes and blow up memory and the model context.
// This keeps read_lines symmetric with read_file's 20KB cap.
const maxReadLinesBytes = 20 * 1024 // 20KB

// executeReadLines reads specific lines from a file.
func (te *ToolExecutor) executeReadLines(params map[string]interface{}) *ToolResult {
	path, ok := params["path"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	startLine, errMsg := parseIntParamStrict(params, "start")
	if errMsg != "" {
		return &ToolResult{
			Success: false,
			Error:   errMsg,
		}
	}

	endLine, errMsg := parseIntParamStrict(params, "end")
	if errMsg != "" {
		return &ToolResult{
			Success: false,
			Error:   errMsg,
		}
	}

	// Validate start and end are positive (1-indexed line numbers)
	if startLine < 1 {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("start line must be >= 1, got %d", startLine),
		}
	}
	if endLine < 1 {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("end line must be >= 1, got %d", endLine),
		}
	}

	if startLine > endLine {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("start line (%d) must be <= end line (%d)", startLine, endLine),
		}
	}

	// Protect against requesting too many lines at once
	requestedLines := endLine - startLine + 1
	if requestedLines > maxReadLinesBlock {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("requested %d lines but maximum is %d. Use a smaller range or use the bash tool to process the file.", requestedLines, maxReadLinesBlock),
		}
	}

	// Check if file is binary before reading
	if isBinaryFile(path) {
		return &ToolResult{
			Success: false,
			Error:   formatReadFileBinaryError(path),
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, path),
		}
	}

	lines := strings.Split(string(content), "\n")
	// Handle trailing newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Adjust to 0-indexed
	startIdx := startLine - 1
	endIdx := endLine

	// Handle edge cases
	if startIdx >= len(lines) {
		return &ToolResult{
			Success: true,
			Output:  "",
			Extra: map[string]interface{}{
				"start":   startLine,
				"end":     endLine,
				"message": "start line beyond file length",
			},
		}
	}

	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	selectedLines := lines[startIdx:endIdx]

	// Format output with line numbers, bounding the total output bytes so long
	// lines cannot cause an unbounded result. Lines are truncated rune-safely
	// when they would push the output past the cap.
	var output strings.Builder
	for i, line := range selectedLines {
		lineNum := startIdx + i + 1
		prefix := fmt.Sprintf("%d: ", lineNum)
		if output.Len()+len(prefix)+len(line) <= maxReadLinesBytes {
			output.WriteString(prefix)
			output.WriteString(line)
			output.WriteString("\n")
			continue
		}
		// This line would exceed the cap. Fill remaining budget with a
		// rune-safe prefix of the line, then add a truncation marker and stop.
		remaining := maxReadLinesBytes - output.Len()
		if remaining > 0 {
			output.WriteString(prefix)
			r := []rune(line)
			if avail := remaining - output.Len(); avail > 0 && avail < len(r) {
				output.WriteString(string(r[:avail]))
			}
		}
		output.WriteString("... [output truncated due to size]")
		break
	}

	return &ToolResult{
		Success: true,
		Output:  strings.TrimSuffix(output.String(), "\n"),
		Extra: map[string]interface{}{
			"start": startLine,
			"end":   endLine,
		},
	}
}
