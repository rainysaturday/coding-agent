package tools

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// ReplaceLinesTool implements line replacement functionality
type ReplaceLinesTool struct{}

// NewReplaceLinesTool creates a new ReplaceLinesTool
func NewReplaceLinesTool() *ReplaceLinesTool {
	return &ReplaceLinesTool{}
}

// Name returns the tool name
func (t *ReplaceLinesTool) Name() string {
	return "replace_lines"
}

// Description returns a human-readable description
func (t *ReplaceLinesTool) Description() string {
	return "Replace a line range with new lines"
}

// Execute replaces a range of lines with new lines
func (t *ReplaceLinesTool) Execute(params map[string]string) ToolResult {
	path, ok := params["path"]
	if !ok || path == "" {
		return ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	startStr, ok := params["start"]
	if !ok || startStr == "" {
		return ToolResult{
			Success: false,
			Error:   "missing required parameter: start",
		}
	}

	endStr, ok := params["end"]
	if !ok || endStr == "" {
		return ToolResult{
			Success: false,
			Error:   "missing required parameter: end",
		}
	}

	linesToReplace, ok := params["lines"]
	if !ok {
		return ToolResult{
			Success: false,
			Error:   "missing required parameter: lines",
		}
	}

	start, err := strconv.Atoi(startStr)
	if err != nil || start < 1 {
		return ToolResult{
			Success: false,
			Error:   "invalid start parameter: must be a positive integer",
		}
	}

	end, err := strconv.Atoi(endStr)
	if err != nil || end < 1 {
		return ToolResult{
			Success: false,
			Error:   "invalid end parameter: must be a positive integer",
		}
	}

	if start > end {
		return ToolResult{
			Success: false,
			Error:   "start line cannot be greater than end line",
		}
	}

	replacementLines := parseLines(linesToReplace)

	// Read existing file if it exists
	var existingLines []string
	_, err = os.Stat(path)
	if err == nil {
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

	// Calculate indices (0-indexed)
	startIdx := start - 1
	endIdx := end // end is exclusive for slicing

	// Handle start beyond file end (append)
	if startIdx >= len(existingLines) {
		// Append replacement lines
		existingLines = append(existingLines, replacementLines...)
	} else {
		// Adjust endIdx if beyond file
		if endIdx > len(existingLines) {
			endIdx = len(existingLines)
		}

		// Replace the range
		newLines := make([]string, 0, len(existingLines)-endIdx+startIdx+len(replacementLines))
		// Lines before the range
		newLines = append(newLines, existingLines[:startIdx]...)
		// Replacement lines
		newLines = append(newLines, replacementLines...)
		// Lines after the range
		newLines = append(newLines, existingLines[endIdx:]...)
		existingLines = newLines
	}

	// Handle empty file replacement
	if len(existingLines) == 0 && len(replacementLines) == 0 {
		// Write empty file
		err = os.WriteFile(path, []byte(""), 0644)
		if err != nil {
			return ToolResult{
				Success: false,
				Error:   "failed to write file: " + err.Error(),
			}
		}
		return ToolResult{
			Success: true,
			Output:  "Successfully replaced lines (empty result)",
		}
	}

	// Write the file
	content := strings.Join(existingLines, "\n")
	if len(existingLines) > 0 {
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
		Output:  "Successfully replaced lines " + startStr + "-" + endStr + " with " + strconv.Itoa(len(replacementLines)) + " line(s)",
	}
}
