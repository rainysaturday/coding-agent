// Package tools implements the tool execution system for the coding agent.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	case "git_status":
		result = te.executeGitStatus(tc.Parameters)
	case "git_diff":
		result = te.executeGitDiff(tc.Parameters)
	case "git_log":
		result = te.executeGitLog(tc.Parameters)
	case "git_show":
		result = te.executeGitShow(tc.Parameters)
	case "git_add":
		result = te.executeGitAdd(tc.Parameters)
	case "git_commit":
		result = te.executeGitCommit(tc.Parameters)
	case "find":
		result = te.executeFind(tc.Parameters)
	case "web_fetch":
		result = te.executeWebFetch(tc.Parameters)
	case "move_file":
		result = te.executeMoveFile(tc.Parameters)
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

// executeGitStatus checks git status of the working directory.
func (te *ToolExecutor) executeGitStatus(params map[string]interface{}) *ToolResult {
	// Default to short format
	format := "short"
	if f, ok := params["format"].(string); ok {
		format = f
	}

	args := []string{"status"}
	switch format {
	case "short":
		args = append(args, "--short")
	case "porcelain", "porcelain=v2":
		args = append(args, "--porcelain")
	case "long":
		args = append(args, "--long")
	}

	// Only include untracked files if explicitly requested (default: include)
	if only, ok := params["include_untracked"].(bool); ok && !only {
		args = append(args, "--untracked-files=no")
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git status failed: %s", string(output)),
		}
	}

	result := &ToolResult{
		Success: true,
		Output:  string(output),
		Extra: map[string]interface{}{
			"tool": "git_status",
		},
	}

	// Parse short output to provide summary
	if format == "short" {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		staged := 0
		unstaged := 0
		untracked := 0
		for _, line := range lines {
			if line == "" {
				continue
			}
			if len(line) >= 2 {
				idxStatus := line[0]
				wtStatus := line[1]
				if idxStatus != ' ' {
					staged++
				}
				if wtStatus != ' ' && idxStatus != '?' {
					unstaged++
				}
				if idxStatus == '?' {
					untracked++
				}
			}
		}
		result.Extra["stagedFiles"] = staged
		result.Extra["unstagedFiles"] = unstaged
		result.Extra["untrackedFiles"] = untracked
	}

	return result
}

// executeGitDiff shows the diff of changes (staged or unstaged).
func (te *ToolExecutor) executeGitDiff(params map[string]interface{}) *ToolResult {
	// Determine what to show: staged, unstaged, or specific file
	showStaged := false
	if s, ok := params["staged"].(bool); ok && s {
		showStaged = true
	}

	// If a specific file is provided, show diff for that file
	file, hasFile := params["file"].(string)

	args := []string{"diff"}
	if showStaged {
		args = append(args, "--cached")
	}
	if !showStaged && hasFile {
		// For unstaged diff of a specific file
		args = append(args, file)
	} else if showStaged && hasFile {
		args = append(args, "--", file)
	}

	// Limit output size to avoid overwhelming the context
	maxLinesParam, hasMaxLines := params["max_lines"]
	maxLines := 200 // Default limit
	if hasMaxLines {
		switch v := maxLinesParam.(type) {
		case float64:
			maxLines = int(v)
		case int:
			maxLines = v
		case string:
			if n, err := strconv.Atoi(v); err == nil {
				maxLines = n
			}
		}
	}

	// Add number flag for line context
	args = append(args, "-U3")

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git diff failed: %s", string(output)),
		}
	}

	// Truncate output if too long
	outputStr := string(output)
	if maxLines > 0 {
		lines := strings.Split(outputStr, "\n")
		if len(lines) > maxLines {
			outputStr = strings.Join(lines[:maxLines], "\n") + "\n... [output truncated, " + strconv.Itoa(len(lines)-maxLines) + " more lines]"
		}
	}

	// Count changed files and lines
	changedFiles := 0
	addedLines := 0
	deletedLines := 0
	for _, line := range strings.Split(outputStr, "\n") {
		if strings.HasPrefix(line, "diff --git") {
			changedFiles++
		}
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			addedLines++
		}
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			deletedLines++
		}
	}

	result := &ToolResult{
		Success: true,
		Output:  outputStr,
		Extra: map[string]interface{}{
			"tool":         "git_diff",
			"staged":       showStaged,
			"changedFiles": changedFiles,
			"linesAdded":   addedLines,
			"linesDeleted": deletedLines,
		},
	}

	if hasFile {
		result.Extra["file"] = file
	}

	return result
}

// executeGitLog shows commit history.
func (te *ToolExecutor) executeGitLog(params map[string]interface{}) *ToolResult {
	// Default options
	branches := []string{"HEAD"}
	maxCount := 20
	pretty := "medium" // medium = subject, body, author, date

	if b, ok := params["branches"].([]interface{}); ok {
		branches = make([]string, len(b))
		for i, br := range b {
			branches[i] = fmt.Sprintf("%v", br)
		}
	} else if b, ok := params["branch"].(string); ok && b != "" {
		branches = []string{b}
	} else if b, ok := params["branches"].(string); ok && b != "" {
		branches = []string{b}
	}

	if c, ok := params["max_count"].(float64); ok {
		maxCount = int(c)
	} else if c, ok := params["max_count"].(int); ok {
		maxCount = c
	} else if c, ok := params["max_count"].(string); ok {
		if n, err := strconv.Atoi(c); err == nil {
			maxCount = n
		}
	}

	if p, ok := params["format"].(string); ok {
		switch p {
		case "short":
			pretty = "short"
		case "full":
			pretty = "full"
		case "fuller":
			pretty = "fuller"
		case "raw":
			pretty = "raw"
		case "oneline":
			pretty = "oneline"
		default:
			pretty = p
		}
	}

	args := []string{"log", fmt.Sprintf("--%s", pretty), fmt.Sprintf("-n%d", maxCount)}
	// Add separator between branches
	args = append(args, "--")
	args = append(args, branches...)

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git log failed: %s", string(output)),
		}
	}

	// Truncate if too long
	outputStr := string(output)
	maxLines := 100
	lines := strings.Split(outputStr, "\n")
	if len(lines) > maxLines {
		outputStr = strings.Join(lines[:maxLines], "\n") + "\n... [output truncated]"
	}

	result := &ToolResult{
		Success: true,
		Output:  outputStr,
		Extra: map[string]interface{}{
			"tool":        "git_log",
			"maxCount":    maxCount,
			"branches":    branches,
			"commitCount": strings.Count(outputStr, "commit "),
		},
	}

	return result
}

// executeGitShow shows file content at a specific commit/ref.
func (te *ToolExecutor) executeGitShow(params map[string]interface{}) *ToolResult {
	ref, hasRef := params["ref"].(string)
	if !hasRef {
		ref = "HEAD"
	}

	path, hasPath := params["path"].(string)
	if !hasPath {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	maxLines := 200
	if ml, ok := params["max_lines"].(float64); ok {
		maxLines = int(ml)
	} else if ml, ok := params["max_lines"].(int); ok {
		maxLines = ml
	} else if ml, ok := params["max_lines"].(string); ok {
		if n, err := strconv.Atoi(ml); err == nil {
			maxLines = n
		}
	}

	args := []string{"show", fmt.Sprintf("%s:%s", ref, path)}
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git show failed: %s", string(output)),
		}
	}

	// Truncate if too long
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	if len(lines) > maxLines {
		outputStr = strings.Join(lines[:maxLines], "\n") + "\n... [output truncated, " + strconv.Itoa(len(lines)-maxLines) + " more lines]"
	}

	result := &ToolResult{
		Success: true,
		Output:  outputStr,
		Path:    path,
		Extra: map[string]interface{}{
			"tool":       "git_show",
			"ref":        ref,
			"path":       path,
			"contentLen": len(outputStr),
		},
	}

	return result
}

// executeGitAdd stages files for commit.
func (te *ToolExecutor) executeGitAdd(params map[string]interface{}) *ToolResult {
	filesParam, hasFiles := params["files"]

	var files []string
	if hasFiles {
		switch v := filesParam.(type) {
		case []interface{}:
			files = make([]string, len(v))
			for i, f := range v {
				files[i] = fmt.Sprintf("%v", f)
			}
		case string:
			// Single file or comma-separated
			if strings.Contains(v, ",") {
				for _, f := range strings.Split(v, ",") {
					files = append(files, strings.TrimSpace(f))
				}
			} else {
				files = []string{v}
			}
		}
	}

	if !hasFiles {
		// Stage all modified files (but not untracked)
		args := []string{"add", "-u"}
		cmd := exec.Command("git", args...)
		output, err := cmd.CombinedOutput()

		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("git add -u failed: %s", string(output)),
			}
		}

		return &ToolResult{
			Success: true,
			Output:  "Staged all tracked modified files",
			Extra: map[string]interface{}{
				"tool":     "git_add",
				"mode":     "update",
				"files":    []string{},
				"message":  "Staged all tracked modified files",
			},
		}
	}

	if len(files) == 0 {
		return &ToolResult{
			Success: false,
			Error:   "no files to stage",
		}
	}

	args := append([]string{"add"}, files...)
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git add failed: %s", string(output)),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Staged %d file(s): %s", len(files), strings.Join(files, ", ")),
		Extra: map[string]interface{}{
			"tool":    "git_add",
			"mode":    "specific",
			"files":   files,
			"message": fmt.Sprintf("Staged %d file(s)", len(files)),
		},
	}
}

// findMatch represents a single match in a file search result.
type findMatch struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// executeFind searches file contents for matching patterns.
func (te *ToolExecutor) executeFind(params map[string]interface{}) *ToolResult {
	pattern, ok := params["pattern"].(string)
	if !ok || pattern == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: pattern",
		}
	}

	// Optional: search within specific files/directories using glob patterns
	pathsParam, hasPaths := params["paths"]
	var pathPatterns []string
	if hasPaths {
		switch v := pathsParam.(type) {
		case []interface{}:
			pathPatterns = make([]string, len(v))
			for i, p := range v {
				pathPatterns[i] = fmt.Sprintf("%v", p)
			}
		case string:
			// Single pattern or comma-separated
			if strings.Contains(v, ",") {
				for _, p := range strings.Split(v, ",") {
					pathPatterns = append(pathPatterns, strings.TrimSpace(p))
				}
			} else {
				pathPatterns = []string{v}
			}
		}
	}

	// Case sensitivity
	caseInsensitive := false
	if ci, ok := params["case_insensitive"].(bool); ok {
		caseInsensitive = ci
	}

	// Max results limit
	maxResultsParam, hasMaxResults := params["max_results"]
	maxResults := 50 // Default limit to prevent context overflow
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

	// Compile the regex pattern
	flags := ""
	if caseInsensitive {
		flags = "(?i)"
	}
	re, err := regexp.Compile(flags + pattern)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid regex pattern: %v", err),
		}
	}

	// Determine which files to search
	var targetFiles []string
	if hasPaths {
		// Search in files matching the given glob patterns
		for _, p := range pathPatterns {
			matches, err := te.globRecursive(p, maxResults*10) // Allow more candidates
			if err != nil {
				continue
			}
			targetFiles = append(targetFiles, matches...)
		}
	} else {
		// Default: search all files recursively
		matches, err := te.globRecursive("**", maxResults*10)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to discover files: %v", err),
			}
		}
		targetFiles = matches
	}

	if len(targetFiles) == 0 {
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("No files found to search (pattern: %s)", pattern),
			Extra: map[string]interface{}{
				"pattern":     pattern,
				"filesSearched": 0,
				"matchesFound":  0,
			},
		}
	}

	// Search through files for matches
	var allMatches []findMatch
	var filesSearched int

	for _, filePath := range targetFiles {
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue // Skip unreadable files
		}

		filesSearched++
		lines := strings.Split(string(content), "\n")

		// Handle trailing newline
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}

		for lineNum, line := range lines {
			if re.MatchString(line) {
				allMatches = append(allMatches, findMatch{
					Path:    filePath,
					Line:    lineNum + 1, // 1-indexed
					Content: line,
				})

				if len(allMatches) >= maxResults {
					goto doneSearching
				}
			}
		}
	}

doneSearching:
	// Format output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d match(es) in %d file(s) for pattern '%s':\n\n",
		len(allMatches), filesSearched, pattern))

	for _, m := range allMatches {
		// Escape content for display (handle control characters)
		displayContent := m.Content
		if len(displayContent) > 200 {
			displayContent = displayContent[:200] + "..."
		}
		output.WriteString(fmt.Sprintf("  %s:%d: %s\n", m.Path, m.Line, displayContent))
	}

	return &ToolResult{
		Success: true,
		Output:  output.String(),
		Extra: map[string]interface{}{
			"pattern":       pattern,
			"caseInsensitive": caseInsensitive,
			"filesSearched": filesSearched,
			"matchesFound":  len(allMatches),
			"maxResults":    maxResults,
		},
	}
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

// executeGitCommit commits staged changes with a descriptive message.
func (te *ToolExecutor) executeGitCommit(params map[string]interface{}) *ToolResult {
	message, hasMessage := params["message"]
	amend := false
	if a, ok := params["amend"].(bool); ok {
		amend = a
	}

	// Check if there are staged changes
	checkCmd := exec.Command("git", "diff", "--cached", "--name-only")
	checkOutput, err := checkCmd.CombinedOutput()
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to check staged files: %s", string(checkOutput)),
		}
	}

	stagedFiles := strings.TrimSpace(string(checkOutput))
	if stagedFiles == "" {
		// Check if there are any commits at all
		checkCmd = exec.Command("git", "rev-parse", "--verify", "HEAD")
		checkOutput, err = checkCmd.CombinedOutput()
		if err != nil {
			// No commits yet - need to stage something first
			return &ToolResult{
				Success: false,
				Error:   "no staged changes and no commits exist yet. Use git_add to stage files first.",
			}
		}
		// First commit with --allow-empty
		if !amend {
			if hasMessage {
				message = fmt.Sprintf("%s (initial commit)", message)
			} else {
				message = "Initial commit"
			}
		}
	} else if !amend {
		message = fmt.Sprintf("%s\n\nStaged files:\n%s", message, stagedFiles)
	}

	// Build commit args
	args := []string{"commit"}
	if amend {
		args = append(args, "--amend")
	}

	if hasMessage {
		msgStr, ok := message.(string)
		if !ok {
			msgStr = fmt.Sprintf("%v", message)
		}
		args = append(args, "-m", msgStr)
	} else {
		args = append(args, "--allow-empty-message", "-m", "")
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git commit failed: %s", string(output)),
		}
	}

	result := &ToolResult{
		Success: true,
		Output:  string(output),
		Extra: map[string]interface{}{
			"tool":   "git_commit",
			"amend":  amend,
			"hasMsg": hasMessage,
		},
	}

	return result
}

// executeWebFetch fetches content from a URL using HTTP GET.
func (te *ToolExecutor) executeWebFetch(params map[string]interface{}) *ToolResult {
	rawURL, ok := params["url"].(string)
	if !ok || rawURL == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: url",
		}
	}

	// Validate URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid URL: %v", err),
		}
	}

	// Only allow http and https schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unsupported URL scheme: %s (only http and https are allowed)", parsedURL.Scheme),
		}
	}

	// Determine timeout (default: 30 seconds)
	timeoutSeconds := 30
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

	// Determine max response size (default: 10KB = 10240 bytes)
	maxSize := 10240
	if maxParam, hasMax := params["max_size"]; hasMax {
		switch v := maxParam.(type) {
		case float64:
			maxSize = int(v)
		case int:
			maxSize = v
		case string:
			if n, err := strconv.Atoi(v); err == nil {
				maxSize = n
			}
		}
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(timeoutSeconds) * time.Second,
	}

	// Create request
	req, err := http.NewRequestWithContext(context.Background(), "GET", rawURL, nil)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create request: %v", err),
		}
	}

	// Set User-Agent header
	req.Header.Set("User-Agent", "coding-agent/1.0")

	// Perform request
	resp, err := client.Do(req)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("request failed: %v", err),
		}
	}
	defer resp.Body.Close()

	// Check for HTTP errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to read error body
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body))),
		}
	}

	// Read response body with size limit
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxSize)))
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read response body: %v", err),
		}
	}

	// Check if response was truncated
	truncated := false
	if int64(len(body)) >= int64(maxSize) {
		truncated = true
	}

	result := &ToolResult{
		Success: true,
		Output:  string(body),
		Extra: map[string]interface{}{
			"status_code":   resp.StatusCode,
			"content_type":  resp.Header.Get("Content-Type"),
			"content_length": len(body),
			"truncated":     truncated,
		},
	}

	if truncated {
		result.Output += "\n... [response truncated, exceeded max_size limit]"
	}

	return result
}

// executeMoveFile moves or renames a file from source to destination.
func (te *ToolExecutor) executeMoveFile(params map[string]interface{}) *ToolResult {
	src, ok := params["source"].(string)
	if !ok || src == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: source",
		}
	}

	dest, ok := params["destination"].(string)
	if !ok || dest == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: destination",
		}
	}

	// Clean paths to prevent directory traversal
	cleanSrc := filepath.Clean(src)
	cleanDest := filepath.Clean(dest)

	// Validate source exists
	if _, err := os.Stat(cleanSrc); os.IsNotExist(err) {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("source file not found: %s", cleanSrc),
		}
	}

	// Prevent directory traversal on source
	if strings.HasPrefix(cleanSrc, "..") {
		return &ToolResult{
			Success: false,
			Error:   "invalid source path: directory traversal not allowed",
		}
	}

	// Create parent directories for destination if needed
	destDir := filepath.Dir(cleanDest)
	if destDir != "" && destDir != "." {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("cannot create destination directory: %v", err),
			}
		}
	}

	// Perform the move
	err := os.Rename(cleanSrc, cleanDest)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to move file: %v", err),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Moved '%s' -> '%s'", cleanSrc, cleanDest),
		Extra: map[string]interface{}{
			"source":     cleanSrc,
			"destination": cleanDest,
			"operation":  "move",
		},
	}
}
