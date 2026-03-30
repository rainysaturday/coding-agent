package tools

import (
	"bufio"
	"os"
	"strconv"
)

// ReadLinesTool implements line-range reading functionality
type ReadLinesTool struct{}

// NewReadLinesTool creates a new ReadLinesTool
func NewReadLinesTool() *ReadLinesTool {
	return &ReadLinesTool{}
}

// Name returns the tool name
func (t *ReadLinesTool) Name() string {
	return "read_lines"
}

// Description returns a human-readable description
func (t *ReadLinesTool) Description() string {
	return "Read a specific line range from a file"
}

// Execute reads a specific line range from a file
func (t *ReadLinesTool) Execute(params map[string]string) ToolResult {
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
			Success: true,
			Output:  "",
		}
	}

	// Open the file
	file, err := os.Open(path)
	if err != nil {
		return ToolResult{
			Success: false,
			Error:   err.Error(),
		}
	}
	defer file.Close()

	// Read the specified line range
	var lines []string
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if lineNum >= start && lineNum <= end {
			lines = append(lines, strconv.Itoa(lineNum)+": "+scanner.Text())
		}
		if lineNum > end {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return ToolResult{
			Success: false,
			Error:   err.Error(),
		}
	}

	result := ToolResult{
		Success: true,
	}

	if len(lines) > 0 {
		result.Output = joinLines(lines)
	}

	return result
}

// joinLines joins lines with newlines
func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}
