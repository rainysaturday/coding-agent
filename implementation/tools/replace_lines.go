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
	return "Replace lines in a file by line numbers or search-and-replace"
}

// Execute replaces content in a file, supporting two modes:
// 1. Line-number mode: Replace a specific range of lines (requires start, end, lines)
// 2. Search-and-replace mode: Find and replace text patterns (requires search, replace)
func (t *ReplaceLinesTool) Execute(params map[string]string) ToolResult {
	path, ok := params["path"]
	if !ok || path == "" {
		return ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	// Determine mode: search-and-replace or line-number
	_, hasSearch := params["search"]
	_, hasReplace := params["replace"]
	_, hasStart := params["start"]
	_, hasEnd := params["end"]

	if hasSearch && hasReplace {
		// Search-and-replace mode
		return t.executeSearchReplace(path, params)
	} else if hasStart && hasEnd {
		// Line-number mode
		return t.executeLineReplace(path, params)
	} else {
		return ToolResult{
			Success: false,
			Error:   "must use either (search and replace) OR (start and end and lines) parameters",
		}
	}
}

// executeSearchReplace implements search-and-replace mode
func (t *ReplaceLinesTool) executeSearchReplace(path string, params map[string]string) ToolResult {
	searchText, ok := params["search"]
	if !ok || searchText == "" {
		return ToolResult{
			Success: false,
			Error:   "missing required parameter: search",
		}
	}

	replaceText, ok := params["replace"]
	if !ok {
		return ToolResult{
			Success: false,
			Error:   "missing required parameter: replace",
		}
	}

	// Parse count parameter
	count := 1 // Default: replace first occurrence only
	if countStr, ok := params["count"]; ok && countStr != "" {
		if countStr == "all" || countStr == "-1" {
			count = -1 // Replace all occurrences
		} else {
			parsedCount, err := strconv.Atoi(countStr)
			if err != nil {
				return ToolResult{
					Success: false,
					Error:   "invalid count parameter: must be an integer or 'all'",
				}
			}
			count = parsedCount
		}
	}

	// Read existing file
	content, err := os.ReadFile(path)
	if err != nil {
		return ToolResult{
			Success: false,
			Error:   "failed to read file: " + err.Error(),
		}
	}

	fileContent := string(content)

	// Count total occurrences before replacement
	totalOccurrences := strings.Count(fileContent, searchText)

	if totalOccurrences == 0 {
		return ToolResult{
			Success: false,
			Error:   "search text not found in file",
		}
	}

	// Perform replacement
	var result string
	var replacementsMade int

	if count == -1 || count > totalOccurrences {
		// Replace all occurrences
		result = strings.ReplaceAll(fileContent, searchText, replaceText)
		replacementsMade = totalOccurrences
	} else {
		// Replace only first 'count' occurrences
		result = t.replaceFirstNOccurrences(fileContent, searchText, replaceText, count)
		replacementsMade = count
	}

	// Write the file
	err = os.WriteFile(path, []byte(result), 0644)
	if err != nil {
		return ToolResult{
			Success: false,
			Error:   "failed to write file: " + err.Error(),
		}
	}

	return ToolResult{
		Success: true,
		Output:  "Successfully replaced " + strconv.Itoa(replacementsMade) + " occurrence(s) of search text",
	}
}

// replaceFirstNOccurrences replaces the first n occurrences of search in content
func (t *ReplaceLinesTool) replaceFirstNOccurrences(content, search, replace string, n int) string {
	if n <= 0 || search == "" {
		return content
	}

	result := content
	for i := 0; i < n; i++ {
		idx := strings.Index(result, search)
		if idx == -1 {
			break
		}
		result = result[:idx] + replace + result[idx+len(search):]
	}

	return result
}

// executeLineReplace implements line-number mode (original behavior)
func (t *ReplaceLinesTool) executeLineReplace(path string, params map[string]string) ToolResult {
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


