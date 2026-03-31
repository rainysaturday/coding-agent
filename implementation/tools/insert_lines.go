package tools

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// InsertLinesTool implements line insertion functionality
type InsertLinesTool struct{}

// NewInsertLinesTool creates a new InsertLinesTool
func NewInsertLinesTool() *InsertLinesTool {
	return &InsertLinesTool{}
}

// Name returns the tool name
func (t *InsertLinesTool) Name() string {
	return "insert_lines"
}

// Description returns a human-readable description
func (t *InsertLinesTool) Description() string {
	return "Insert lines at a specific line number"
}

// Execute inserts lines at a specified line number in a file
func (t *InsertLinesTool) Execute(params map[string]string) ToolResult {
	path, ok := params["path"]
	if !ok || path == "" {
		return ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	lineStr, ok := params["line"]
	if !ok || lineStr == "" {
		return ToolResult{
			Success: false,
			Error:   "missing required parameter: line",
		}
	}

	linesToInsert, ok := params["lines"]
	if !ok {
		return ToolResult{
			Success: false,
			Error:   "missing required parameter: lines",
		}
	}

	insertLine, err := strconv.Atoi(lineStr)
	if err != nil || insertLine < 1 {
		return ToolResult{
			Success: false,
			Error:   "invalid line parameter: must be a positive integer",
		}
	}

	// Parse lines to insert
	insertLines := parseLines(linesToInsert)

	// Read existing file if it exists
	var existingLines []string
	_, err = os.Stat(path)
	if err == nil {
		// File exists, read it
		file, err := os.Open(path)
		if err != nil {
			return ToolResult{
				Success: false,
				Error:   "failed to open file: " + err.Error(),
			}
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			existingLines = append(existingLines, scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			file.Close()
			return ToolResult{
				Success: false,
				Error:   "failed to read file: " + err.Error(),
			}
		}
		file.Close()
	}

	// Insert lines at the specified position (1-indexed)
	// Line 1 means insert at the beginning (index 0)
	// Line beyond file end means append
	insertIndex := insertLine - 1
	if insertIndex > len(existingLines) {
		insertIndex = len(existingLines)
	}

	// Build new content
	var newLines []string
	// Add existing lines before insertion point
	newLines = append(newLines, existingLines[:insertIndex]...)
	// Add new lines
	newLines = append(newLines, insertLines...)
	// Add existing lines after insertion point
	newLines = append(newLines, existingLines[insertIndex:]...)

	// Write the file
	content := strings.Join(newLines, "\n")
	if len(newLines) > 0 {
		content += "\n"
	}

	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return ToolResult{
			Success: false,
			Error:   "failed to write file: " + err.Error(),
		}
	}

	return ToolResult{
		Success: true,
		Output:  "Successfully inserted " + strconv.Itoa(len(insertLines)) + " lines at line " + lineStr,
	}
}

// parseLines parses a string of lines separated by \n
// Handles trailing newlines gracefully
func parseLines(linesStr string) []string {
	if linesStr == "" {
		return []string{}
	}
	// Remove trailing newline if present to avoid empty string at end
	linesStr = strings.TrimSuffix(linesStr, "\n")
	if linesStr == "" {
		return []string{}
	}
	return strings.Split(linesStr, "\n")
}
