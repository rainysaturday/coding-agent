// Package tools implements the tool execution system for the coding agent.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	Success  bool                   `json:"success"`
	Output   string                 `json:"output,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Path     string                 `json:"path,omitempty"`
	ExitCode int                    `json:"exit_code,omitempty"`
	Extra    map[string]interface{} `json:"-"`
}

// ToolCall represents a tool call parsed from the LLM response (OpenAI format compatible).
// Supports both legacy format (name/parameters) and OpenAI format (function/arguments).
type ToolCall struct {
	ID         string                 `json:"id,omitempty"` // OpenAI tool call ID
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Arguments  string                 `json:"arguments,omitempty"` // OpenAI: raw JSON string of arguments
	Raw        string                 `json:"-"`
}

// ToolExecutor handles tool execution.
type ToolExecutor struct {
	stats *Stats
}

// Stats holds tool execution statistics.
type Stats struct {
	TotalCalls  int `json:"total_calls"`
	FailedCalls int `json:"failed_calls"`
}

// NewToolExecutor creates a new tool executor.
func NewToolExecutor() *ToolExecutor {
	return &ToolExecutor{
		stats: &Stats{},
	}
}

// Stats returns the current statistics.
func (te *ToolExecutor) Stats() *Stats {
	return te.stats
}

// ParseToolCall parses a tool call from the raw string.
// Parses OpenAI format: {"id":"...","type":"function","function":{"name":"...","arguments":"..."}}
func ParseToolCall(raw string) (*ToolCall, error) {
	// Use a wrapper struct to properly handle the nested function object
	var wrapper struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}

	if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
		return nil, fmt.Errorf("invalid tool call JSON: %v", err)
	}

	if wrapper.Function.Name == "" {
		return nil, fmt.Errorf("missing tool name in tool call")
	}

	// Parse arguments JSON string into parameters
	var params map[string]interface{}
	if wrapper.Function.Arguments != "" {
		if err := json.Unmarshal([]byte(wrapper.Function.Arguments), &params); err != nil {
			// If arguments parsing fails, keep raw arguments
			params = map[string]interface{}{
				"_raw_arguments": wrapper.Function.Arguments,
			}
		}
	}

	tc := &ToolCall{
		ID:         wrapper.ID,
		Name:       wrapper.Function.Name,
		Parameters: params,
		Arguments:  wrapper.Function.Arguments,
		Raw:        raw,
	}
	return tc, nil
}

// Execute executes a tool call and returns the result.
func (te *ToolExecutor) Execute(tc *ToolCall) *ToolResult {
	te.stats.TotalCalls++

	var result *ToolResult

	switch tc.Name {
	case "bash":
		result = te.executeBash(tc.Parameters)
	case "read_file":
		result = te.executeReadFile(tc.Parameters)
	case "write_file":
		result = te.executeWriteFile(tc.Parameters)
	case "patch":
		result = te.executePatch(tc.Parameters)
	case "read_lines":
		result = te.executeReadLines(tc.Parameters)
	case "insert_lines":
		result = te.executeInsertLines(tc.Parameters)
	case "replace_text":
		result = te.executeReplaceText(tc.Parameters)
	case "replace_lines":
		result = te.executeReplaceLines(tc.Parameters)
	case "glob":
		result = te.executeGlob(tc.Parameters)
	case "sub_agent":
		result = te.executeSubAgent(tc.Parameters)
	default:
		te.stats.FailedCalls++
		result = &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown tool: %s", tc.Name),
		}
	}

	if !result.Success {
		te.stats.FailedCalls++
	}

	return result
}

// executeBash executes a bash command.
func (te *ToolExecutor) executeBash(params map[string]interface{}) *ToolResult {
	command, ok := params["command"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: command",
		}
	}

	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.CombinedOutput()

	// Extract exit code
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
	}

	result := &ToolResult{
		ExitCode: exitCode,
	}

	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("command failed: %v\nOutput: %s", err, string(output))
	} else {
		result.Success = true
		result.Output = string(output)
	}

	return result
}

// executeReadFile reads a file.
func (te *ToolExecutor) executeReadFile(params map[string]interface{}) *ToolResult {
	path, ok := params["path"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, path),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  string(content),
		Path:    path,
		Extra: map[string]interface{}{
			"linesRead":     len(strings.Split(string(content), "\n")),
			"contentLength": len(content),
		},
	}
}

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
		Output:  fmt.Sprintf("File written successfully: %s (%d bytes)", path, len(content)),
		Path:    path,
		Extra: map[string]interface{}{
			"message":       fmt.Sprintf("File written successfully: %s", path),
			"contentLength": len(content),
		},
	}
}

// executeReadLines reads specific lines from a file.
func (te *ToolExecutor) executeReadLines(params map[string]interface{}) *ToolResult {
	path, ok := params["path"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	start, ok := params["start"].(float64)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: start",
		}
	}

	end, ok := params["end"].(float64)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: end",
		}
	}

	startLine := int(start)
	endLine := int(end)

	if startLine > endLine {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("start line (%d) must be <= end line (%d)", startLine, endLine),
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

	// Format output with line numbers
	var output strings.Builder
	for i, line := range selectedLines {
		lineNum := startIdx + i + 1
		output.WriteString(fmt.Sprintf("%d: %s\n", lineNum, line))
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

// executeInsertLines inserts lines at a specific position.
func (te *ToolExecutor) executeInsertLines(params map[string]interface{}) *ToolResult {
	path, ok := params["path"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	lineNum, ok := params["line"].(float64)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: line",
		}
	}

	insertLines, ok := params["lines"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: lines",
		}
	}

	insertLine := int(lineNum)
	newLines := strings.Split(insertLines, "\n")

	// Read existing content or create empty
	var existingLines []string
	content, err := os.ReadFile(path)
	if err == nil {
		existingLines = strings.Split(string(content), "\n")
		// Handle trailing newline
		if len(existingLines) > 0 && existingLines[len(existingLines)-1] == "" {
			existingLines = existingLines[:len(existingLines)-1]
		}
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
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("cannot create directory: %v", err),
			}
		}
	}

	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, path),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Inserted %d line(s) at line %d in: %s", len(newLines), insertLine, path),
		Path:    path,
		Extra: map[string]interface{}{
			"line":          insertLine,
			"linesInserted": len(newLines),
		},
	}
}

// executeReplaceText replaces text in a file by searching for a pattern.
func (te *ToolExecutor) executeReplaceText(params map[string]interface{}) *ToolResult {
	path, ok := params["path"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	searchText, ok := params["search"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: search",
		}
	}

	replaceText, ok := params["replace"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: replace",
		}
	}

	countParam, hasCount := params["count"]
	count := 1 // Default to 1 replacement
	if hasCount {
		switch v := countParam.(type) {
		case float64:
			count = int(v)
		case int:
			count = v
		case string:
			if v == "all" || v == "-1" {
				count = -1 // Replace all
			} else if c, err := strconv.Atoi(v); err == nil {
				count = c
			}
		}
	}

	// Read existing content
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("file not found: %s", path),
			}
		}
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, path),
		}
	}

	originalContent := string(content)

	// Count total occurrences
	totalOccurrences := strings.Count(originalContent, searchText)

	if totalOccurrences == 0 {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("search text not found: %s", searchText),
		}
	}

	// Perform replacement
	var newContent string
	var replacementsMade int
	if count < 0 || count > totalOccurrences {
		// Replace all
		newContent = strings.ReplaceAll(originalContent, searchText, replaceText)
		replacementsMade = totalOccurrences
	} else {
		// Replace only count occurrences
		newContent = originalContent
		for i := 0; i < count; i++ {
			idx := strings.Index(newContent, searchText)
			if idx == -1 {
				break
			}
			newContent = newContent[:idx] + replaceText + newContent[idx+len(searchText):]
		}
		replacementsMade = count
	}

	// Write back
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, path),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Replaced '%s' with '%s' %d time(s) in: %s", searchText, replaceText, replacementsMade, path),
		Path:    path,
		Extra: map[string]interface{}{
			"search":           searchText,
			"replacementsMade": replacementsMade,
			"totalOccurrences": totalOccurrences,
		},
	}
}

// executeReplaceLines replaces lines in a file.
func (te *ToolExecutor) executeReplaceLines(params map[string]interface{}) *ToolResult {
	path, ok := params["path"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	// Check if using line-number mode or search-and-replace mode
	_, hasStart := params["start"]
	_, hasEnd := params["end"]
	_, hasSearch := params["search"]

	if hasStart && hasEnd {
		// Line-number mode
		return te.replaceLinesByNumber(path, params)
	} else if hasSearch {
		// Search-and-replace mode
		return te.replaceLinesBySearch(path, params)
	} else {
		return &ToolResult{
			Success: false,
			Error:   "must provide either start/end (line-number mode) or search (search-and-replace mode)",
		}
	}
}

// replaceLinesByNumber replaces lines by line numbers.
func (te *ToolExecutor) replaceLinesByNumber(path string, params map[string]interface{}) *ToolResult {
	start, ok := params["start"].(float64)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: start",
		}
	}

	end, ok := params["end"].(float64)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: end",
		}
	}

	replacementLines, ok := params["lines"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: lines",
		}
	}

	startLine := int(start)
	endLine := int(end)
	newLines := strings.Split(replacementLines, "\n")

	if startLine > endLine {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("start line (%d) must be <= end line (%d)", startLine, endLine),
		}
	}

	// Read existing content
	var existingLines []string
	content, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return &ToolResult{
				Success: false,
				Error:   formatFileError(err, path),
			}
		}
		// File doesn't exist, start fresh
		existingLines = []string{}
	} else {
		existingLines = strings.Split(string(content), "\n")
		// Handle trailing newline
		if len(existingLines) > 0 && existingLines[len(existingLines)-1] == "" {
			existingLines = existingLines[:len(existingLines)-1]
		}
	}

	// Adjust to 0-indexed
	startIdx := startLine - 1
	endIdx := endLine

	// Handle edge cases
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx > len(existingLines) {
		// Append at end
		existingLines = append(existingLines, newLines...)
	} else {
		if endIdx > len(existingLines) {
			endIdx = len(existingLines)
		}
		// Replace the range
		resultLines := make([]string, 0, len(existingLines)-endIdx+startIdx+len(newLines))
		resultLines = append(resultLines, existingLines[:startIdx]...)
		resultLines = append(resultLines, newLines...)
		resultLines = append(resultLines, existingLines[endIdx:]...)
		existingLines = resultLines
	}

	// Write back
	output := strings.Join(existingLines, "\n")
	if len(existingLines) > 0 {
		output += "\n"
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

	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, path),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Replaced lines %d-%d with %d line(s) in: %s", startLine, endLine, len(newLines), path),
		Path:    path,
		Extra: map[string]interface{}{
			"start":         startLine,
			"end":           endLine,
			"linesReplaced": endLine - startLine + 1,
			"linesInserted": len(newLines),
		},
	}
}

// replaceLinesBySearch replaces content by searching for text.
func (te *ToolExecutor) replaceLinesBySearch(path string, params map[string]interface{}) *ToolResult {
	searchText, ok := params["search"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: search",
		}
	}

	replaceText, ok := params["replace"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: replace",
		}
	}

	countParam, hasCount := params["count"]
	count := 1 // Default to 1 replacement
	if hasCount {
		switch v := countParam.(type) {
		case float64:
			count = int(v)
		case int:
			count = v
		case string:
			if v == "all" || v == "-1" {
				count = -1 // Replace all
			} else if c, err := strconv.Atoi(v); err == nil {
				count = c
			}
		}
	}

	// Read existing content
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("file not found: %s", path),
			}
		}
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, path),
		}
	}

	originalContent := string(content)

	// Count total occurrences
	totalOccurrences := strings.Count(originalContent, searchText)

	if totalOccurrences == 0 {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("search text not found: %s", searchText),
		}
	}

	// Perform replacement
	var newContent string
	var replacementsMade int
	if count < 0 || count > totalOccurrences {
		// Replace all
		newContent = strings.ReplaceAll(originalContent, searchText, replaceText)
		replacementsMade = totalOccurrences
	} else {
		// Replace only count occurrences
		newContent = originalContent
		for i := 0; i < count; i++ {
			idx := strings.Index(newContent, searchText)
			if idx == -1 {
				break
			}
			newContent = newContent[:idx] + replaceText + newContent[idx+len(searchText):]
		}
		replacementsMade = count
	}

	// Write back
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, path),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Replaced '%s' with '%s' %d time(s) in: %s", searchText, replaceText, replacementsMade, path),
		Path:    path,
		Extra: map[string]interface{}{
			"search":           searchText,
			"replacementsMade": replacementsMade,
			"totalOccurrences": totalOccurrences,
		},
	}
}

// formatFileError formats a file error into a user-friendly message.
func formatFileError(err error, path string) string {
	if os.IsNotExist(err) {
		return fmt.Sprintf("file not found: %s", path)
	}
	if os.IsPermission(err) {
		return fmt.Sprintf("permission denied: %s", path)
	}
	return fmt.Sprintf("file error: %v", err)
}

// executePatch applies a unified diff patch to a file.
func (te *ToolExecutor) executePatch(params map[string]interface{}) *ToolResult {
	path, ok := params["path"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	diff, ok := params["diff"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: diff",
		}
	}

	// Validate path to prevent directory traversal
	cleanPath := filepath.Clean(path)

	// Check for directory traversal attempts that resolve to system directories
	if strings.Contains(path, "..") {
		// Block if clean path resolves to system directories
		if filepath.IsAbs(cleanPath) && (strings.HasPrefix(cleanPath, "/etc") || strings.HasPrefix(cleanPath, "/root") || strings.HasPrefix(cleanPath, "/home") || cleanPath == "/") {
			return &ToolResult{
				Success: false,
				Error:   "invalid path: directory traversal not allowed",
			}
		}
		// Block if clean path still has ".." components
		if strings.HasPrefix(cleanPath, "..") {
			return &ToolResult{
				Success: false,
				Error:   "invalid path: directory traversal not allowed",
			}
		}
	}

	// Check if file exists
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("file not found: %s", cleanPath),
		}
	}

	// Validate diff format is not empty
	if strings.TrimSpace(diff) == "" {
		return &ToolResult{
			Success: false,
			Error:   "diff content cannot be empty",
		}
	}

	// Validate basic diff structure
	if !strings.Contains(diff, "@@") {
		return &ToolResult{
			Success: false,
			Error:   "invalid diff format: missing hunk headers (@@)",
		}
	}

	// Create a temporary file to store the diff
	tmpFile, err := os.CreateTemp("", "patch-*.diff")
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create temporary file: %v", err),
		}
	}
	defer os.Remove(tmpFile.Name())

	// Write diff to temporary file
	if _, err := tmpFile.WriteString(diff); err != nil {
		tmpFile.Close()
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to write diff to temporary file: %v", err),
		}
	}
	tmpFile.Close()

	// Get original file permissions
	origInfo, err := os.Stat(cleanPath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to get file info: %v", err),
		}
	}
	origPerm := origInfo.Mode()

	// Create a backup of the original file content for rollback
	backupContent, err := os.ReadFile(cleanPath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read original file: %v", err),
		}
	}

	// Apply the patch using the system patch command
	cmd := exec.Command("patch", "--dry-run", "-o", os.DevNull, cleanPath, tmpFile.Name())
	dryRunOutput, dryRunErr := cmd.CombinedOutput()

	if dryRunErr != nil {
		// Restore is not needed since dry-run doesn't modify
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("patch validation failed: %s\nDetails: %s", dryRunErr, string(dryRunOutput)),
			Extra: map[string]interface{}{
				"patches_applied": 0,
			},
		}
	}

	// Apply the patch for real (in-place modification)
	cmd = exec.Command("patch", cleanPath, tmpFile.Name())
	patchOutput, patchErr := cmd.CombinedOutput()

	if patchErr != nil {
		// Restore original file content
		if err := os.WriteFile(cleanPath, backupContent, origPerm); err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("patch failed and rollback also failed: %v\nPatch error: %s", err, string(patchOutput)),
				Extra: map[string]interface{}{
					"patches_applied": 0,
				},
			}
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("patch application failed: %s", string(patchOutput)),
			Extra: map[string]interface{}{
				"patches_applied": 0,
			},
		}
	}

	// Count number of hunks applied
	hunkCount := strings.Count(diff, "@@")

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Applied %d hunk(s) to %s", hunkCount, cleanPath),
		Path:    cleanPath,
		Extra: map[string]interface{}{
			"patches_applied": hunkCount,
		},
	}
}

// executeGlob searches for files matching a glob pattern.
func (te *ToolExecutor) executeGlob(params map[string]interface{}) *ToolResult {
	pattern, ok := params["pattern"].(string)
	if !ok || pattern == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: pattern",
		}
	}

	maxResultsParam, hasMaxResults := params["max_results"]
	maxResults := 100 // Default limit
	if hasMaxResults {
		switch v := maxResultsParam.(type) {
		case float64:
			maxResults = int(v)
		case int:
			maxResults = v
		case string:
			if n, err := strconv.Atoi(v); err == nil {
				maxResults = n
			}
		}
	}

	// Normalize pattern - handle ** for recursive matching
	// filepath.Glob doesn't support ** directly, so we need custom handling
	var matches []string
	var err error

	// Check if pattern contains **
	if strings.Contains(pattern, "**") {
		matches, err = te.globRecursive(pattern, maxResults)
	} else {
		matches, err = filepath.Glob(pattern)
	}

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("glob error: %v", err),
		}
	}

	if len(matches) == 0 {
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("No files found matching pattern: %s", pattern),
			Extra: map[string]interface{}{
				"pattern":      pattern,
				"matchesFound": 0,
			},
		}
	}

	// Limit results
	if len(matches) > maxResults {
		matches = matches[:maxResults]
	}

	// Format output with file info
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d file(s) matching '%s':\n\n", len(matches), pattern))

	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			output.WriteString(fmt.Sprintf("  %s [error getting info]\n", match))
		} else {
			size := info.Size()
			modTime := info.ModTime().Format("2006-01-02 15:04:05")
			if info.IsDir() {
				output.WriteString(fmt.Sprintf("  %s/ (directory)\n", match))
			} else {
				output.WriteString(fmt.Sprintf("  %s (%d bytes, modified %s)\n", match, size, modTime))
			}
		}
	}

	return &ToolResult{
		Success: true,
		Output:  output.String(),
		Extra: map[string]interface{}{
			"pattern":      pattern,
			"matchesFound": len(matches),
		},
	}
}

// executeSubAgent spawns a sub-agent process to handle a delegated task.
func (te *ToolExecutor) executeSubAgent(params map[string]interface{}) *ToolResult {
	prompt, ok := params["prompt"].(string)
	if !ok || prompt == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: prompt",
		}
	}

	// Determine timeout (default 300 seconds / 5 minutes)
	timeoutSeconds := 300
	if timeoutParam, hasTimeout := params["timeout"]; hasTimeout {
		switch v := timeoutParam.(type) {
		case float64:
			timeoutSeconds = int(v)
		case int:
			timeoutSeconds = v
		case string:
			if n, err := strconv.Atoi(v); err == nil {
				timeoutSeconds = n
			}
		}
	}

	// Find the coding-agent executable
	// Try the current working directory first, then PATH
	var executablePath string
	cwd, _ := os.Getwd()
	for _, candidate := range []string{
		filepath.Join(cwd, "coding-agent"),
		filepath.Join(cwd, "implementation", "coding-agent"),
		"coding-agent",
	} {
		if resolved, err := filepath.Abs(candidate); err == nil {
			if _, err := os.Stat(resolved); err == nil {
				executablePath = resolved
				break
			}
		}
	}

	// If not found, try to find via exec.LookPath
	if executablePath == "" {
		if lookedPath, err := exec.LookPath("coding-agent"); err == nil {
			executablePath = lookedPath
		}
	}

	if executablePath == "" {
		return &ToolResult{
			Success: false,
			Error:   "coding-agent executable not found in PATH or working directory",
		}
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// Spawn the sub-agent process with context
	cmd := exec.CommandContext(ctx, executablePath, "-p", prompt)

	// Set working directory to current directory
	cmd.Dir = cwd

	// Set up output capture
	output, err := cmd.CombinedOutput()
	if err != nil && ctx.Err() == context.DeadlineExceeded {
		err = fmt.Errorf("sub-agent timed out after %d seconds", timeoutSeconds)
		output = []byte("")
	}

	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
	}

	result := &ToolResult{
		ExitCode: exitCode,
	}

	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("sub-agent failed (exit code %d): %s", exitCode, string(output))
	} else {
		result.Success = true
		result.Output = string(output)
	}

	return result
}

// globRecursive performs recursive glob matching for patterns containing **.
// It expands ** into proper recursive directory traversal.
func (te *ToolExecutor) globRecursive(pattern string, maxResults int) ([]string, error) {
	var matches []string

	// Split pattern into base directory and remaining pattern
	// Pattern can be: "**/*.go", "src/**/*.ts", "/path/**", "**/test.go"
	// We need to find the first ** and split there
	firstStar := strings.Index(pattern, "**")
	if firstStar == -1 {
		// No ** found, fall back to regular glob
		return filepath.Glob(pattern)
	}

	// The base is everything before the first **
	var baseDir string
	before := strings.TrimRight(pattern[:firstStar], "/")
	if before == "" {
		baseDir = "."
	} else if filepath.IsAbs(before) {
		baseDir = before
	} else {
		baseDir = "."
		// relative pattern like "src/**" - base is "src"
		if before != "." {
			baseDir = before
		}
	}

	// The remaining pattern is everything after the first **
	after := strings.TrimLeft(pattern[firstStar+2:], "/")
	// after could be "*.go" or "" or "src/**"

	// Walk the directory tree
	err := filepath.WalkDir(baseDir, func(walkPath string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if maxResults > 0 && len(matches) >= maxResults {
			return filepath.SkipDir
		}

		// Always skip directories themselves from results
		if d.IsDir() {
			return nil
		}

		// Get relative path from baseDir
		relPath, err := filepath.Rel(baseDir, walkPath)
		if err != nil {
			return nil
		}

		// Match: apply the remaining pattern against the relative path
		// If after is empty, match everything
		// If after is "*.go", match all .go files at any depth
		// If after is "src/*.ts", match .ts files only under a "src" subdir at any depth
		if after == "" {
			matches = append(matches, walkPath)
		} else if strings.Contains(after, "/") {
			// Multi-level remaining pattern like "src/**/*.go" or "src/*.ts"
			// We need to check if relPath ends with a matching suffix
			lastSlash := strings.LastIndex(after, "/")
			dirPattern := after[:lastSlash]
			filePattern := after[lastSlash+1:]

			// Get the suffix of relPath after removing the filename
			dirSuffix := relPath[:len(relPath)-len(filepath.Base(relPath))]
			dirSuffix = strings.TrimSuffix(dirSuffix, "/")

			// Check if the directory suffix matches the dirPattern
			if strings.Contains(dirPattern, "**") {
				// Handle ** in remaining pattern too (nested **)
				// For "src/**" we check if relPath starts with "src/"
				if strings.HasPrefix(relPath, strings.TrimRight(dirPattern, "/**")) {
					if matched, _ := filepath.Match(filePattern, filepath.Base(relPath)); matched {
						matches = append(matches, walkPath)
					}
				}
			} else {
				if matched, _ := filepath.Match(dirPattern, dirSuffix); matched {
					if fileMatched, _ := filepath.Match(filePattern, filepath.Base(relPath)); fileMatched {
						matches = append(matches, walkPath)
					}
				}
			}
		} else {
			// Simple pattern like "*.go" - match against filename at any depth
			if matched, _ := filepath.Match(after, filepath.Base(relPath)); matched {
				matches = append(matches, walkPath)
			}
		}

		return nil
	})

	return matches, err
}

// matchGlob checks if a path matches a glob pattern.
// It handles * and ? wildcards similar to filepath.Match.
func matchGlob(pattern, path string) (bool, error) {
	// Handle ** patterns in remaining pattern (shouldn't happen in practice after globRecursive splits,
	// but handle for robustness)
	if strings.Contains(pattern, "**") {
		// Split on ** to get prefix and suffix
		idx := strings.Index(pattern, "**")
		prefixPattern := strings.TrimRight(pattern[:idx], "/")
		afterPattern := strings.TrimLeft(pattern[idx+2:], "/")

		// Check if path starts with prefixPattern
		if prefixPattern != "" && !strings.HasPrefix(path, prefixPattern+"/") {
			return false, nil
		}
		// Match remaining pattern against the path (or filename if no prefix)
		if afterPattern == "" {
			return true, nil
		}
		if strings.Contains(afterPattern, "/") {
			// Nested pattern - use matchGlob recursively
			return matchGlob(afterPattern, path)
		}
		// Simple suffix pattern like *.go - match against filename
		return filepath.Match(afterPattern, filepath.Base(path))
	}

	// Handle patterns with "/" - split into prefix + filename
	if strings.Contains(pattern, "/") {
		lastSlash := strings.LastIndex(pattern, "/")
		prefixPattern := pattern[:lastSlash]
		filePattern := pattern[lastSlash+1:]

		// The prefix path should match the directory part
		prefixPath := path[:len(path)-len(filepath.Base(path))]
		// Trim trailing slashes for matching
		prefixPath = strings.TrimRight(prefixPath, "/")
		prefixMatch, err := filepath.Match(prefixPattern, prefixPath)
		if err != nil || !prefixMatch {
			return false, err
		}
		fileMatch, err := filepath.Match(filePattern, filepath.Base(path))
		return fileMatch, err
	}

	// No "/" - use filepath.Match directly
	return filepath.Match(pattern, path)
}
