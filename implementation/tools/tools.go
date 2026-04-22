// Package tools implements the tool execution system for the coding agent.
package tools

import (
	"bytes"
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
	"sort"
	"strconv"
	"strings"
	"text/template"
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
	case "git_branch":
		result = te.executeGitBranch(tc.Parameters)
	case "find":
		result = te.executeFind(tc.Parameters)
	case "web_fetch":
		result = te.executeWebFetch(tc.Parameters)
	case "move_file":
		result = te.executeMoveFile(tc.Parameters)
	case "list_dir":
		result = te.executeListDir(tc.Parameters)
	case "copy_file":
		result = te.executeCopyFile(tc.Parameters)
	case "delete_file":
		result = te.executeDeleteFile(tc.Parameters)
	case "scaffold":
		result = te.executeScaffold(tc.Parameters)
	case "run_tests":
		result = te.executeRunTests(tc.Parameters)
	case "project_tree":
		result = te.executeProjectTree(tc.Parameters)
	case "code_navigation":
		result = te.executeCodeNavigation(tc.Parameters)
	case "check_links":
		result = te.executeCheckLinks(tc.Parameters)
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

// executeGitBranch manages git branches: list, create, checkout, delete, rename, and set upstream.
func (te *ToolExecutor) executeGitBranch(params map[string]interface{}) *ToolResult {
	action, hasAction := params["action"]
	if !hasAction {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: action (list, create, checkout, delete, rename, set_upstream)",
		}
	}

	actionStr, ok := action.(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "action must be a string",
		}
	}

	switch actionStr {
	case "list":
		return te.gitBranchList(params)
	case "create":
		return te.gitBranchCreate(params)
	case "checkout":
		return te.gitBranchCheckout(params)
	case "delete":
		return te.gitBranchDelete(params)
	case "rename":
		return te.gitBranchRename(params)
	case "set_upstream":
		return te.gitBranchSetUpstream(params)
	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s. Valid actions: list, create, checkout, delete, rename, set_upstream", actionStr),
		}
	}
}

// gitBranchList lists all branches (local and remote) with the current branch highlighted.
func (te *ToolExecutor) gitBranchList(params map[string]interface{}) *ToolResult {
	// Check for local branches
	localArgs := []string{"branch", "--format=%(refname:short)"}
	localCmd := exec.Command("git", localArgs...)
	localOutput, localErr := localCmd.CombinedOutput()

	// Check for remote branches
	remoteArgs := []string{"branch", "-r", "--format=%(refname:short)"}
	remoteCmd := exec.Command("git", remoteArgs...)
	remoteOutput, _ := remoteCmd.CombinedOutput()

	var output strings.Builder

	// Get current branch name for highlighting
	currentBranch := ""
	headCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	if headOutput, err := headCmd.CombinedOutput(); err == nil {
		currentBranch = strings.TrimSpace(string(headOutput))
	}

	// Parse local branches
	localBranches := strings.Split(strings.TrimSpace(string(localOutput)), "\n")
	if localErr != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to list local branches: %s", string(localOutput)),
		}
	}

	localCount := 0
	for _, br := range localBranches {
		br = strings.TrimSpace(br)
		if br == "" {
			continue
		}
		localCount++
		prefix := "  "
		if br == currentBranch {
			prefix = "* "
		}
		output.WriteString(fmt.Sprintf("%s%s\n", prefix, br))
	}

	// Parse remote branches
	remoteBranches := strings.Split(strings.TrimSpace(string(remoteOutput)), "\n")
	hasRemote := false
	for _, br := range remoteBranches {
		br = strings.TrimSpace(br)
		if br == "" {
			continue
		}
		hasRemote = true
		// Track upstream info
		upstreamInfo := ""
		upstreamCmd := exec.Command("git", "branch", "-vv", "--format=%(upstream:short)", br)
		if upstreamOutput, err := upstreamCmd.CombinedOutput(); err == nil {
			upstream := strings.TrimSpace(string(upstreamOutput))
			if upstream != "" {
				upstreamInfo = " -> " + upstream
			}
		}
		// Remove "origin/" prefix for display if desired, but keep it for clarity
		output.WriteString(fmt.Sprintf("  %s%s\n", br, upstreamInfo))
	}

	if localCount == 0 && !hasRemote {
		return &ToolResult{
			Success: true,
			Output:  "No branches found in this repository.",
			Extra: map[string]interface{}{
				"tool":       "git_branch",
				"action":     "list",
				"localCount":  0,
				"remoteCount": 0,
			},
		}
	}

	result := &ToolResult{
		Success: true,
		Output:  output.String(),
		Extra: map[string]interface{}{
			"tool":        "git_branch",
			"action":      "list",
			"localCount":  localCount,
			"currentBranch": currentBranch,
		},
	}

	// Count remote branches for extra info
	remoteCount := 0
	for _, br := range remoteBranches {
		if strings.TrimSpace(br) != "" {
			remoteCount++
		}
	}
	result.Extra["remoteCount"] = remoteCount

	return result
}

// gitBranchCreate creates a new branch, optionally from a specified base branch.
func (te *ToolExecutor) gitBranchCreate(params map[string]interface{}) *ToolResult {
	name, hasName := params["name"]
	if !hasName {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: name",
		}
	}

	nameStr, ok := name.(string)
	if !ok || nameStr == "" {
		return &ToolResult{
			Success: false,
			Error:   "name must be a non-empty string",
		}
	}

	// Validate branch name (git rules)
	if err := validateBranchName(nameStr); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid branch name: %v", err),
		}
	}

	// Check if branch already exists
	checkCmd := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+nameStr)
	if _, err := checkCmd.CombinedOutput(); err == nil {
		// Branch exists, check if it's a remote branch
		remoteCheck := exec.Command("git", "rev-parse", "--verify", "refs/remotes/origin/"+nameStr)
		if remoteCheckOut, remoteErr := remoteCheck.CombinedOutput(); remoteErr == nil {
			// It's a remote branch, not a local one - allow creating
			_ = remoteCheckOut
		} else {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("branch '%s' already exists", nameStr),
			}
		}
	}

	// Build create command
	args := []string{"branch", nameStr}

	// Check for base branch
	if base, ok := params["start_point"].(string); ok && base != "" {
		// Verify base branch exists
		verifyCmd := exec.Command("git", "rev-parse", "--verify", base)
		if _, err := verifyCmd.CombinedOutput(); err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("base branch '%s' does not exist", base),
			}
		}
		args = append(args, base)
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create branch: %s", string(output)),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Created branch '%s'", nameStr),
		Extra: map[string]interface{}{
			"tool":    "git_branch",
			"action":  "create",
			"name":    nameStr,
			"message": fmt.Sprintf("Created branch '%s'", nameStr),
		},
	}
}

// gitBranchCheckout switches to a different branch.
func (te *ToolExecutor) gitBranchCheckout(params map[string]interface{}) *ToolResult {
	name, hasName := params["name"]
	if !hasName {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: name",
		}
	}

	nameStr, ok := name.(string)
	if !ok || nameStr == "" {
		return &ToolResult{
			Success: false,
			Error:   "name must be a non-empty string",
		}
	}

	// Check if branch exists (local or remote)
	exists := false

	// Check local
	checkCmd := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+nameStr)
	if _, err := checkCmd.CombinedOutput(); err == nil {
		exists = true
	}

	// Check remote if not local
	if !exists {
		remoteCheck := exec.Command("git", "rev-parse", "--verify", "refs/remotes/origin/"+nameStr)
		if _, err := remoteCheck.CombinedOutput(); err == nil {
			exists = true
		}
	}

	if !exists {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("branch '%s' does not exist", nameStr),
		}
	}

	// Build checkout command
	args := []string{"checkout", nameStr}

	// Check for create flag (checkout -b for new branch)
	if create, ok := params["create"].(bool); ok && create {
		args = []string{"checkout", "-b", nameStr}

		// Check for base branch
		if base, ok := params["start_point"].(string); ok && base != "" {
			args = append(args, base)
		}

		cmd := exec.Command("git", args...)
		output, err := cmd.CombinedOutput()

		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to create and checkout branch: %s", string(output)),
			}
		}

		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Created and checked out branch '%s'", nameStr),
			Extra: map[string]interface{}{
				"tool":    "git_branch",
				"action":  "checkout",
				"name":    nameStr,
				"created": true,
				"message": fmt.Sprintf("Created and checked out branch '%s'", nameStr),
			},
		}
	}

	// Validate branch name before checkout
	if err := validateBranchName(nameStr); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid branch name: %v", err),
		}
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to checkout branch: %s", string(output)),
		}
	}

	// Get the new current branch
	newBranch := nameStr
	if newBranchOut, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").CombinedOutput(); err == nil {
		newBranch = strings.TrimSpace(string(newBranchOut))
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Switched to branch '%s'", newBranch),
		Extra: map[string]interface{}{
			"tool":       "git_branch",
			"action":     "checkout",
			"name":       nameStr,
			"newBranch":  newBranch,
			"message":    fmt.Sprintf("Switched to branch '%s'", newBranch),
		},
	}
}

// gitBranchDelete deletes a local branch.
func (te *ToolExecutor) gitBranchDelete(params map[string]interface{}) *ToolResult {
	name, hasName := params["name"]
	if !hasName {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: name",
		}
	}

	nameStr, ok := name.(string)
	if !ok || nameStr == "" {
		return &ToolResult{
			Success: false,
			Error:   "name must be a non-empty string",
		}
	}

	// Check if branch exists
	checkCmd := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+nameStr)
	if _, err := checkCmd.CombinedOutput(); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("branch '%s' does not exist", nameStr),
		}
	}

	// Build delete command
	args := []string{"branch", "-d", nameStr}
	force := false

	// Use -D for force delete if specified
	if f, ok := params["force"].(bool); ok && f {
		force = true
		args = []string{"branch", "-D", nameStr}
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// If -d failed because branch not merged, suggest -D
		if force {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to delete branch '%s': %s", nameStr, string(output)),
			}
		}
		output2, _ := exec.Command("git", "branch", "-D", nameStr).CombinedOutput()
		if err2 := exec.Command("git", "branch", "-D", nameStr).Run(); err2 != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to delete branch '%s' (try force delete: git_branch action='delete' name='%s' force=true). Output: %s", nameStr, nameStr, string(output2)),
			}
		}
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Force deleted branch '%s'", nameStr),
			Extra: map[string]interface{}{
				"tool":    "git_branch",
				"action":  "delete",
				"name":    nameStr,
				"forced":  true,
				"message": fmt.Sprintf("Force deleted branch '%s'", nameStr),
			},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Deleted branch '%s'", nameStr),
		Extra: map[string]interface{}{
			"tool":    "git_branch",
			"action":  "delete",
			"name":    nameStr,
			"message": fmt.Sprintf("Deleted branch '%s'", nameStr),
		},
	}
}

// gitBranchRename renames a local branch.
func (te *ToolExecutor) gitBranchRename(params map[string]interface{}) *ToolResult {
	oldName, hasOldName := params["old_name"]
	newName, hasNewName := params["new_name"]

	if !hasOldName || !hasNewName {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameters: old_name and new_name",
		}
	}

	oldNameStr, ok := oldName.(string)
	if !ok || oldNameStr == "" {
		return &ToolResult{
			Success: false,
			Error:   "old_name must be a non-empty string",
		}
	}

	newNameStr, ok := newName.(string)
	if !ok || newNameStr == "" {
		return &ToolResult{
			Success: false,
			Error:   "new_name must be a non-empty string",
		}
	}

	// Validate new branch name
	if err := validateBranchName(newNameStr); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid new branch name: %v", err),
		}
	}

	// Check if old branch exists
	checkCmd := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+oldNameStr)
	if _, err := checkCmd.CombinedOutput(); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("branch '%s' does not exist", oldNameStr),
		}
	}

	// Check if new branch already exists
	newCheck := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+newNameStr)
	if _, err := newCheck.CombinedOutput(); err == nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("branch '%s' already exists", newNameStr),
		}
	}

	// Build rename command (git branch -m for current, -M for force)
	args := []string{"branch", "-m", oldNameStr, newNameStr}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to rename branch: %s", string(output)),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Renamed branch '%s' -> '%s'", oldNameStr, newNameStr),
		Extra: map[string]interface{}{
			"tool":      "git_branch",
			"action":    "rename",
			"old_name":  oldNameStr,
			"new_name":  newNameStr,
			"message":   fmt.Sprintf("Renamed branch '%s' -> '%s'", oldNameStr, newNameStr),
		},
	}
}

// gitBranchSetUpstream sets or changes the upstream tracking branch for a branch.
func (te *ToolExecutor) gitBranchSetUpstream(params map[string]interface{}) *ToolResult {
	name, hasName := params["name"]
	remote, hasRemote := params["remote"]
	branch, hasBranch := params["branch"]

	if !hasName {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: name",
		}
	}

	nameStr, ok := name.(string)
	if !ok || nameStr == "" {
		return &ToolResult{
			Success: false,
			Error:   "name must be a non-empty string",
		}
	}

	// Build command
	args := []string{"branch", "--set-upstream-to="}

	// Construct the upstream reference
	upstreamRef := ""
	if hasRemote && hasBranch {
		remoteStr, rOk := remote.(string)
		branchStr, bOk := branch.(string)
		if rOk && bOk {
			upstreamRef = remoteStr + "/" + branchStr
		}
	} else if hasRemote {
		remoteStr, rOk := remote.(string)
		if rOk && remoteStr != "" {
			// Use the same branch name on the remote
			upstreamRef = remoteStr + "/" + nameStr
		}
	} else {
		return &ToolResult{
			Success: false,
			Error:   "must provide both 'remote' and 'branch' parameters, or just 'remote'",
		}
	}

	if upstreamRef == "" {
		return &ToolResult{
			Success: false,
			Error:   "failed to construct upstream reference",
		}
	}

	args = append(args, upstreamRef)

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to set upstream: %s", string(output)),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Set upstream for branch '%s' to '%s'", nameStr, upstreamRef),
		Extra: map[string]interface{}{
			"tool":        "git_branch",
			"action":      "set_upstream",
			"name":        nameStr,
			"upstream":    upstreamRef,
			"message":     fmt.Sprintf("Set upstream for branch '%s' to '%s'", nameStr, upstreamRef),
		},
	}
}

// validateBranchName validates a git branch name according to git rules.
func validateBranchName(name string) error {
	if name == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	if len(name) > 255 {
		return fmt.Errorf("branch name exceeds 255 characters")
	}

	// Check for invalid characters and patterns
	invalidPatterns := []string{
		"..",
		" ",
		"\t",
		"\r",
		"\n",
		"^",
		":",
		"?",
		"*",
		"[",
		"\\",
		"<",
		">",
		"|",
		"!",
		"~",
	}

	for _, pattern := range invalidPatterns {
		if strings.Contains(name, pattern) {
			return fmt.Errorf("branch name contains invalid character: '%s'", pattern)
		}
	}

	// Check for leading/trailing dots and slashes
	if strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") {
		return fmt.Errorf("branch name cannot start or end with '/'")
	}
	if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") {
		return fmt.Errorf("branch name cannot start or end with '.'")
	}

	// Check for invalid leading patterns
	if strings.HasPrefix(name, "-") || strings.HasPrefix(name, "+") {
		return fmt.Errorf("branch name cannot start with '-' or '+'")
	}

	// Check that it doesn't contain only special characters
	if strings.HasPrefix(name, "@{") {
		return fmt.Errorf("branch name cannot start with '@{'")
	}

	return nil
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

// executeCopyFile copies a file from source to destination.
func (te *ToolExecutor) executeCopyFile(params map[string]interface{}) *ToolResult {
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

	// Optional overwrite parameter (default: false)
	overwrite := false
	if ow, ok := params["overwrite"].(bool); ok {
		overwrite = ow
	}

	// Clean paths to prevent directory traversal
	cleanSrc := filepath.Clean(src)
	cleanDest := filepath.Clean(dest)

	// Validate source exists
	srcInfo, err := os.Stat(cleanSrc)
	if os.IsNotExist(err) {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("source file not found: %s", cleanSrc),
		}
	}
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("cannot access source file: %v", err),
		}
	}

	// Cannot copy directories
	if srcInfo.IsDir() {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("source is a directory, not a file: %s", cleanSrc),
		}
	}

	// Prevent directory traversal on source
	if strings.HasPrefix(cleanSrc, "..") {
		return &ToolResult{
			Success: false,
			Error:   "invalid source path: directory traversal not allowed",
		}
	}

	// Check if destination already exists
	destExists := false
	if _, err := os.Stat(cleanDest); err == nil {
		destExists = true
	}

	if destExists && !overwrite {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("destination already exists: %s (use overwrite=true to overwrite)", cleanDest),
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

	// Read source file
	srcContent, err := os.ReadFile(cleanSrc)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read source file: %v", err),
		}
	}

	// Get source file permissions
	srcInfo2, err := os.Stat(cleanSrc)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to get source file info: %v", err),
		}
	}
	srcPerm := srcInfo2.Mode()

	// Write to destination
	if err := os.WriteFile(cleanDest, srcContent, srcPerm); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to write destination file: %v", err),
		}
	}

	result := &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Copied '%s' -> '%s' (%d bytes)", cleanSrc, cleanDest, len(srcContent)),
		Extra: map[string]interface{}{
			"source":      cleanSrc,
			"destination": cleanDest,
			"operation":   "copy",
			"bytesCopied": len(srcContent),
		},
	}

	if destExists {
		result.Extra["overwritten"] = true
	}

	return result
}

// executeListDir lists directory contents with metadata.
func (te *ToolExecutor) executeListDir(params map[string]interface{}) *ToolResult {
	// Determine path (default: current directory)
	path := "."
	if p, ok := params["path"].(string); ok && p != "" {
		path = p
	}

	// Determine recursive flag
	recursive := false
	if r, ok := params["recursive"].(bool); ok && r {
		recursive = true
	}

	// Determine max results (default: 100)
	maxResults := 100
	if mr, ok := params["max_results"]; ok {
		switch v := mr.(type) {
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

	// Determine show hidden flag (default: false)
	showHidden := false
	if sh, ok := params["show_hidden"].(bool); ok && sh {
		showHidden = true
	}

	// Clean path and check it exists
	cleanPath := filepath.Clean(path)

	info, err := os.Stat(cleanPath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("cannot access directory '%s': %v", cleanPath, err),
		}
	}

	if !info.IsDir() {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("path is not a directory: %s", cleanPath),
		}
	}

	// Collect entries
	var entries []map[string]interface{}
	var totalSize int64
	var dirCount, fileCount int

	if recursive {
		err = filepath.Walk(cleanPath, func(walkPath string, fileInfo os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip inaccessible paths
			}

			// Skip the root directory itself
			if walkPath == cleanPath {
				return nil
			}

			// Check hidden files/directories
			baseName := filepath.Base(walkPath)
			if !showHidden && strings.HasPrefix(baseName, ".") {
				if fileInfo.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Limit results
			if len(entries) >= maxResults {
				return filepath.SkipDir
			}

			// Calculate relative path
			relPath, _ := filepath.Rel(cleanPath, walkPath)

			entry := map[string]interface{}{
				"name":         baseName,
				"path":         relPath,
				"type":         mapTypeName(fileInfo.IsDir(), fileInfo.Mode()),
				"size":         fileInfo.Size(),
				"modified":     fileInfo.ModTime().UTC().Format(time.RFC3339),
			}
			entries = append(entries, entry)

			if fileInfo.IsDir() {
				dirCount++
			} else {
				fileCount++
				totalSize += fileInfo.Size()
			}

			return nil
		})
	} else {
		dirEntries, err := os.ReadDir(cleanPath)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to read directory '%s': %v", cleanPath, err),
			}
		}

		for _, dirEntry := range dirEntries {
			// Skip hidden files unless explicitly requested
			if !showHidden && strings.HasPrefix(dirEntry.Name(), ".") {
				continue
			}

			// Limit results
			if len(entries) >= maxResults {
				break
			}

			info, err := dirEntry.Info()
			if err != nil {
				continue
			}

			entry := map[string]interface{}{
				"name":     dirEntry.Name(),
				"path":     dirEntry.Name(),
				"type":     mapTypeName(dirEntry.IsDir(), info.Mode()),
				"size":     info.Size(),
				"modified": info.ModTime().UTC().Format(time.RFC3339),
			}
			entries = append(entries, entry)

			if dirEntry.IsDir() {
				dirCount++
			} else {
				fileCount++
				totalSize += info.Size()
			}
		}
	}

	// Format output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Directory: %s\n", cleanPath))
	output.WriteString(fmt.Sprintf("%-40s %-10s %12s  %s\n", "NAME", "TYPE", "SIZE", "MODIFIED"))
	output.WriteString(strings.Repeat("-", 80) + "\n")

	for _, entry := range entries {
		name := entry["name"].(string)
		entryType := entry["type"].(string)
		size := entry["size"].(int64)
		modified := entry["modified"].(string)

		sizeStr := formatFileSize(size)
		truncName := name
		if len(name) > 38 {
			truncName = "…" + name[len(name)-35:]
		}

		output.WriteString(fmt.Sprintf("%-40s %-10s %12s  %s\n", truncName, entryType, sizeStr, modified))
	}

	if len(entries) >= maxResults {
		output.WriteString(fmt.Sprintf("\n... (showing %d of %d+ entries, limited by max_results)\n", maxResults, maxResults+1))
	}

	// Build summary
	result := &ToolResult{
		Success: true,
		Output:  output.String(),
		Extra: map[string]interface{}{
			"tool":        "list_dir",
			"path":        cleanPath,
			"recursive":   recursive,
			"total_items": len(entries),
			"directories": dirCount,
			"files":       fileCount,
			"total_size":  totalSize,
		},
	}

	return result
}

// mapTypeName returns a string representation of the file type.
func mapTypeName(isDir bool, mode os.FileMode) string {
	if isDir {
		return "dir"
	}
	if mode&os.ModeSymlink != 0 {
		return "symlink"
	}
	if mode&os.ModeNamedPipe != 0 {
		return "pipe"
	}
	if mode&os.ModeSocket != 0 {
		return "socket"
	}
	if mode&os.ModeDevice != 0 {
		if mode&os.ModeCharDevice != 0 {
			return "char_device"
		}
		return "block_device"
	}
	return "file"
}

// formatFileSize formats a file size in bytes to a human-readable string.
func formatFileSize(size int64) string {
	const unit = 1024
	if size == 0 {
		return "0 B"
	}
	if size < 0 {
		return fmt.Sprintf("%d B", size)
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for size >= unit && i < len(units)-1 {
		size /= unit
		i++
	}
	return fmt.Sprintf("%d %s", size, units[i])
}

// executeDeleteFile deletes a file from the filesystem.
func (te *ToolExecutor) executeDeleteFile(params map[string]interface{}) *ToolResult {
	path, ok := params["path"].(string)
	if !ok || path == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	// Clean path to prevent directory traversal
	cleanPath := filepath.Clean(path)

	// Prevent directory traversal to system directories
	if strings.Contains(path, "..") {
		if filepath.IsAbs(cleanPath) && (strings.HasPrefix(cleanPath, "/etc") || strings.HasPrefix(cleanPath, "/root") || strings.HasPrefix(cleanPath, "/home") || cleanPath == "/") {
			return &ToolResult{
				Success: false,
				Error:   "invalid path: directory traversal not allowed",
			}
		}
		if strings.HasPrefix(cleanPath, "..") {
			return &ToolResult{
				Success: false,
				Error:   "invalid path: directory traversal not allowed",
			}
		}
	}

	// Check if file exists
	info, err := os.Stat(cleanPath)
	if os.IsNotExist(err) {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("file not found: %s", cleanPath),
		}
	}
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("cannot access file: %v", err),
		}
	}

	// Prevent deleting directories
	if info.IsDir() {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("path is a directory, not a file: %s (use rmdir or rm for directories)", cleanPath),
		}
	}

	// Check permissions before attempting deletion
	if err := os.Remove(cleanPath); err != nil {
		if os.IsPermission(err) {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("permission denied: cannot delete %s", cleanPath),
			}
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to delete file: %v", err),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Deleted file: %s", cleanPath),
		Path:    cleanPath,
		Extra: map[string]interface{}{
			"operation": "delete",
		},
	}
}

// Template represents a code scaffolding template with variable substitution.
type Template struct {
	Name        string
	Description string
	Files       map[string]string // file path -> content
}

// backtick returns a backtick character for use in templates.
const backtickChar = "`"

// builtInTemplates contains all built-in scaffolding templates.
var builtInTemplates = map[string]Template{
	"go_struct": {
		Name:        "go_struct",
		Description: "Generate a Go struct definition with JSON tags and common methods",
		Files: map[string]string{
			"{{.Name}}.go": `// Package {{.Package}} provides {{.Description}}.
package {{.Package}}

// {{.Name}} represents {{.Description}}.
type {{.Name}} struct {
{{range $i, $field := .Fields}}	{{ $field.Name }} {{ $field.Type }} {{- if $field.JSONTag }} {{backtick}}json:"{{ $field.JSONTag }}"{{backtick}}{{- end }}
{{end}}}

// New{{.Name}} creates a new {{.Name}} instance.
func New{{.Name}}() *{{.Name}} {
	return &{{.Name}}{}
}

// Get{{.Name}} returns the {{.Name}} as a pointer.
func (r *{{.Name}}) Get{{.Name}}() *{{.Name}} {
	return r
}
`,
		},
	},
	"go_handler": {
		Name:        "go_handler",
		Description: "Generate a Go HTTP handler function with context support",
		Files: map[string]string{
			"{{.Name}}.go": `package {{.Package}}

import (
	"encoding/json"
	"net/http"
)

// {{.Name}}Handler handles HTTP {{.Method}} requests for {{.Description}}.
func {{.Name}}Handler(w http.ResponseWriter, r *http.Request) {
	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Parse request body
	var req {{.RequestType}}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Process the request
	{{.Body}}

	// Send response
	resp := {{.ResponseType}}{}
	json.NewEncoder(w).Encode(resp)
}

// {{.Name}}Request represents the request body.
type {{.RequestType}} struct {
{{range $i, $field := .Fields}}	{{ $field.Name }} {{ $field.Type }} {{- if $field.JSONTag }} {{backtick}}json:"{{ $field.JSONTag }}"{{backtick}}{{- end }}
{{end}}}

// {{.Response}}Response represents the response body.
type {{.ResponseType}} struct {
{{range $i, $field := .Fields}}	{{ $field.Name }} {{ $field.Type }} {{- if $field.JSONTag }} {{backtick}}json:"{{ $field.JSONTag }}"{{backtick}}{{- end }}
{{end}}}
`,
		},
	},
	"go_service": {
		Name:        "go_service",
		Description: "Generate a Go service struct with a method",
		Files: map[string]string{
			"{{.Name}}.go": `package {{.Package}}

import (
	"context"
	"fmt"
)

// {{.Name}}Service handles {{.Description}}.
type {{.Name}}Service struct {
{{range $i, $field := .Fields}}	{{ $field.Name }} {{ $field.Type }}
{{end}}}

// New{{.Name}}Service creates a new {{.Name}}Service instance.
func New{{.Name}}Service() *{{.Name}}Service {
	return &{{.Name}}Service{}
}

// {{.MethodName}} {{.MethodDescription}}.
func (s *{{.Name}}Service) {{.MethodName}}(ctx context.Context, req *{{.RequestType}}) (*{{.ResponseType}}, error) {
	// Validate input
	if req == nil {
		return nil, fmt.Errorf("invalid request: nil")
	}

	{{.Body}}

	return &{{.ResponseType}}{
{{range $i, $field := .Fields}}		{{ $field.Name }}: req.{{ $field.Name }},
{{end}}	}, nil
}
`,
		},
	},
	"python_class": {
		Name:        "python_class",
		Description: "Generate a Python class with __init__, __repr__, and a method",
		Files: map[string]string{
			"{{.Name}}.py": `"""Module for {{.Description}}."""

from __future__ import annotations


class {{.Name}}:
	"""{{.Description}}."""

	def __init__(self{{range $i, $field := .Fields}}, {{ $field.Name }}: {{ $field.Type }}{{end}}) -> None:
		"""Initialize {{.Name}}."""
{{range $i, $field := .Fields}}		self.{{ $field.Name }} = {{ $field.Name }}
{{end}}		self._{{.PrivateVar}} = None

	def __repr__(self) -> str:
		"""Return string representation of {{.Name}}."""
		return f"{self.__class__.__name__}(...)"

{{range $i, $field := .Fields}}
	def get_{{ $field.Name }}(self) -> {{ $field.Type }}:
		"""Get {{ $field.Name }}."""
		return self.{{ $field.Name }}

{{end}}	{{.Body}}
`,
		},
	},
	"python_dataclass": {
		Name:        "python_dataclass",
		Description: "Generate a Python dataclass",
		Files: map[string]string{
			"{{.Name}}.py": `"""Module for {{.Description}}."""

from __future__ import annotations
from dataclasses import dataclass, field
{{range $i, $field := .Fields}}
@dataclass
class {{ $field.Name }}:
	"""{{ $field.Description }}."""
{{ $field.Name }}: {{ $field.Type }}
{{end}}
@dataclass
class {{.Name}}:
	"""{{.Description}}."""
{{range $i, $field := .Fields}}	{{ $field.Name }}: {{ $field.Type }}
{{end}}		{{.PrivateVar}}: str = field(default_factory=str)
`,
		},
	},
	"proto_message": {
		Name:        "proto_message",
		Description: "Generate a Protobuf message definition",
		Files: map[string]string{
			"{{.Name}}.proto": `syntax = "proto3";

package {{.Package}};

option go_package = "{{.GoPackage}}";

// {{.Description}}
message {{.Name}} {
{{range $i, $field := .Fields}}
	{{ $field.TypeProto }} {{ $field.Name }} = {{ $field.Number }};
{{end}}}
`,
		},
	},
	"openapi_schema": {
		Name:        "openapi_schema",
		Description: "Generate an OpenAPI schema for a resource",
		Files: map[string]string{
			"{{.Name}}_schema.yaml": `openapi: "3.0.3"
info:
  title: "{{.Name}}"
  description: "{{.Description}}"
  version: "1.0.0"
paths:
  /{{.PluralName}}:
    get:
      summary: "List {{.PluralName}}"
      responses:
        "200":
          description: "Successful response"
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/{{.Name}}"
    post:
      summary: "Create {{.Name}}"
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/{{.Name}}Input"
      responses:
        "201":
          description: "Created"
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/{{.Name}}"
components:
  schemas:
    {{.Name}}:
      type: object
      required:
{{range $i, $field := .Fields}}        - {{ $field.Name }}
{{end}}      properties:
{{range $i, $field := .Fields}}        {{ $field.Name }}:
          type: {{ $field.SchemaType }}
{{end}}    {{.Name}}Input:
      type: object
      required:
{{range $i, $field := .Fields}}        - {{ $field.Name }}
{{end}}      properties:
{{range $i, $field := .Fields}}        {{ $field.Name }}:
          type: {{ $field.SchemaType }}
{{end}}
`,
		},
	},
	"go_test": {
		Name:        "go_test",
		Description: "Generate a Go test file with table-driven tests",
		Files: map[string]string{
			"{{.Name}}_test.go": `package {{.Package}}

import (
	"testing"
)

{{range $i, $test := .Tests}}
// Test{{ $test.Name }} tests {{ $test.Description }}.
func Test{{ $test.Name }}(t *testing.T) {
	t.Parallel()
{{if $test.Setup}}	// Setup
	{{ $test.Setup }}
{{end}}	// Execute
	result := {{ $test.Call }}
{{if $test.Assert}}	// Assert
	{{ $test.Assert }}
{{end}}}

{{end}}func TestMain(m *testing.M) {
	// TODO: Add test setup/teardown
	m.Run()
}
`,
		},
	},
}

// templateField represents a field in a scaffolding template.
type templateField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	JSONTag     string `json:"json_tag,omitempty"`
	TypeProto   string `json:"type_proto,omitempty"`
	Number      int    `json:"number,omitempty"`
	SchemaType  string `json:"schema_type,omitempty"`
	Description string `json:"description,omitempty"`
}

// templateTest represents a test case in the go_test template.
type templateTest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Setup       string `json:"setup,omitempty"`
	Call        string `json:"call"`
	Assert      string `json:"assert,omitempty"`
}

// executeScaffold generates code from built-in templates with variable substitution.
func (te *ToolExecutor) executeScaffold(params map[string]interface{}) *ToolResult {
	templateName, ok := params["template"].(string)
	if !ok || templateName == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: template",
		}
	}

	template, exists := builtInTemplates[templateName]
	if !exists {
		available := make([]string, 0, len(builtInTemplates))
		for k := range builtInTemplates {
			available = append(available, k)
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown template: %s. Available templates: %v", templateName, available),
		}
	}

	// Parse fields (struct fields, method fields, etc.)
	var fields []templateField
	if fieldsParam, hasFields := params["fields"]; hasFields {
		switch v := fieldsParam.(type) {
		case []interface{}:
			for _, item := range v {
				if fieldMap, ok := item.(map[string]interface{}); ok {
					field := templateField{}
					if name, ok := fieldMap["name"].(string); ok {
						field.Name = name
					}
					if t, ok := fieldMap["type"].(string); ok {
						field.Type = t
					}
					if tag, ok := fieldMap["json_tag"].(string); ok {
						field.JSONTag = tag
					}
					if tp, ok := fieldMap["type_proto"].(string); ok {
						field.TypeProto = tp
					}
					if n, ok := fieldMap["number"].(float64); ok {
						field.Number = int(n)
					} else if n, ok := fieldMap["number"].(int); ok {
						field.Number = n
					} else if n, ok := fieldMap["number"].(int64); ok {
						field.Number = int(n)
					}
					if st, ok := fieldMap["schema_type"].(string); ok {
						field.SchemaType = st
					}
					if desc, ok := fieldMap["description"].(string); ok {
						field.Description = desc
					}
					fields = append(fields, field)
				}
			}
		}
	}

	// Parse tests (for go_test template)
	var tests []templateTest
	if testsParam, hasTests := params["tests"]; hasTests {
		switch v := testsParam.(type) {
		case []interface{}:
			for _, item := range v {
				if testMap, ok := item.(map[string]interface{}); ok {
					test := templateTest{}
					if name, ok := testMap["name"].(string); ok {
						test.Name = name
					}
					if desc, ok := testMap["description"].(string); ok {
						test.Description = desc
					}
					if setup, ok := testMap["setup"].(string); ok {
						test.Setup = setup
					}
					if call, ok := testMap["call"].(string); ok {
						test.Call = call
					}
					if assert, ok := testMap["assert"].(string); ok {
						test.Assert = assert
					}
					tests = append(tests, test)
				}
			}
		}
	}

	// Build template data map
	data := map[string]interface{}{
		"Package":      "default",
		"Name":         "MyStruct",
		"Description":  "A generated struct",
		"Fields":       fields,
		"Tests":        tests,
		"Method":       "GET",
		"Body":         "// TODO: implement business logic",
		"RequestType":  "Request",
		"ResponseType": "Response",
		"PrivateVar":   "data",
		"GoPackage":    "github.com/example/pkg",
		"PluralName":   "items",
	}

	// Override with explicit parameters
	if pkg, ok := params["package"].(string); ok && pkg != "" {
		data["Package"] = pkg
	}
	if name, ok := params["name"].(string); ok && name != "" {
		data["Name"] = name
	}
	if desc, ok := params["description"].(string); ok && desc != "" {
		data["Description"] = desc
	}
	if method, ok := params["method"].(string); ok && method != "" {
		data["Method"] = method
	}
	if body, ok := params["body"].(string); ok && body != "" {
		data["Body"] = body
	}
	if reqType, ok := params["request_type"].(string); ok && reqType != "" {
		data["RequestType"] = reqType
	}
	if respType, ok := params["response_type"].(string); ok && respType != "" {
		data["ResponseType"] = respType
	}
	if methodName, ok := params["method_name"].(string); ok && methodName != "" {
		data["MethodName"] = methodName
	}
	if methodDesc, ok := params["method_description"].(string); ok && methodDesc != "" {
		data["MethodDescription"] = methodDesc
	}
	if goPkg, ok := params["go_package"].(string); ok && goPkg != "" {
		data["GoPackage"] = goPkg
	}
	if plural, ok := params["plural_name"].(string); ok && plural != "" {
		data["PluralName"] = plural
	}

	// Process each template file with text/template
	var generatedFiles []map[string]interface{}
	for filePath, content := range template.Files {
		// Apply template substitution
		processedFile, err := processTemplate(content, data)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("template processing error in %s: %v", filePath, err),
			}
		}

		// Resolve the file path (also may contain template vars)
		resolvedPath, err := processTemplate(filePath, data)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("template processing error for path %s: %v", filePath, err),
			}
		}

		// Create parent directories
		dir := filepath.Dir(resolvedPath)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return &ToolResult{
					Success: false,
					Error:   fmt.Sprintf("cannot create directory %s: %v", dir, err),
				}
			}
		}

		// Write the file
		if err := os.WriteFile(resolvedPath, []byte(processedFile), 0644); err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to write %s: %v", resolvedPath, err),
			}
		}

		generatedFiles = append(generatedFiles, map[string]interface{}{
			"path":     resolvedPath,
			"size":     len(processedFile),
			"template": filepath.Base(filePath),
		})
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Generated %d file(s) from template '%s':\n", len(generatedFiles), templateName),
		Extra: map[string]interface{}{
			"tool":      "scaffold",
			"template":  templateName,
			"files":     generatedFiles,
			"filesList": extractPaths(generatedFiles),
		},
	}
}

// processTemplate applies Go text/template substitution to a string with the given data.
func processTemplate(tmpl string, data map[string]interface{}) (string, error) {
	t, err := parseTemplate(tmpl)
	if err != nil {
		return "", fmt.Errorf("parse error: %v", err)
	}

	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute error: %v", err)
	}

	return buf.String(), nil
}

// templateCache caches parsed templates to avoid re-parsing.
var templateCache = make(map[string]*TemplateParser)
var templateCacheMu struct{}

// TemplateParser wraps a compiled template.
type TemplateParser struct {
	tmpl *template.Template
}

// parseTemplate compiles and caches a template.
func parseTemplate(tmpl string) (*template.Template, error) {
	// Simple caching: check if we've seen this exact template before
	// Since Go doesn't have sync.Map in stdlib without imports, we'll use a simple map
	// In production, use sync.Map. For now, re-parse each time (templates are small).
	return template.New("").Funcs(template.FuncMap{
		"backtick": func() string { return backtickChar },
		"lower":    strings.ToLower,
		"upper":    strings.ToUpper,
	}).Parse(tmpl)
}

// extractPaths extracts file paths from the generated files list.
func extractPaths(files []map[string]interface{}) []string {
	var paths []string
	for _, f := range files {
		if p, ok := f["path"].(string); ok {
			paths = append(paths, p)
		}
	}
	return paths
}

// executeRunTests executes tests for the current project and reports structured results.
func (te *ToolExecutor) executeRunTests(params map[string]interface{}) *ToolResult {
	// Determine test command
	command, hasCommand := params["command"].(string)

	if !hasCommand || command == "" {
		// Auto-detect project type
		command = te.detectTestCommand()
	}

	if command == "" {
		return &ToolResult{
			Success: false,
			Error:   "no project type detected. Supported project types: Go (go.mod), Node.js (package.json), Python (requirements.txt, pyproject.toml), Makefile with test target. Provide a custom 'command' parameter to override.",
		}
	}

	// Build arguments
	var args []string
	if argsParam, hasArgs := params["args"]; hasArgs {
		switch v := argsParam.(type) {
		case []interface{}:
			for _, a := range v {
				args = append(args, fmt.Sprintf("%v", a))
			}
		case string:
			args = append(args, v)
		}
	}

	// Determine timeout (default: 60 seconds)
	timeoutSeconds := 60
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

	// Build full command
	fullCmd := command
	if len(args) > 0 {
		fullCmd = command + " " + strings.Join(args, " ")
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", fullCmd)

	// Set working directory to current directory
	cwd, _ := os.Getwd()
	cmd.Dir = cwd

	// Execute command
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Extract exit code
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
	} else {
		// Check if the output indicates test failure even with exit code 0
		// (some test runners return 0 but output "FAIL")
		if strings.Contains(outputStr, "FAIL") && !strings.Contains(outputStr, "ok  ") {
			exitCode = 1
		}
	}

	// Truncate output if too long
	maxOutputLen := 10000
	if len(outputStr) > maxOutputLen {
		outputStr = outputStr[:maxOutputLen] + "\n... [output truncated, exceeded 10000 character limit]"
	}

	// Determine if tests passed
	passed := exitCode == 0

	// Generate summary
	summary := te.generateTestSummary(outputStr, passed, command)

	result := &ToolResult{
		Success:  passed,
		ExitCode: exitCode,
		Output:   outputStr,
		Extra: map[string]interface{}{
			"tool":    "run_tests",
			"passed":  passed,
			"command": command,
			"summary": summary,
		},
	}

	return result
}

// detectTestCommand auto-detects the test command based on project files.
func (te *ToolExecutor) detectTestCommand() string {
	cwd, _ := os.Getwd()

	// Check for Go project
	if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
		return "go test ./..."
	}

	// Check for Node.js project
	if _, err := os.Stat(filepath.Join(cwd, "package.json")); err == nil {
		return "npm test"
	}

	// Check for Python project
	if _, err := os.Stat(filepath.Join(cwd, "requirements.txt")); err == nil {
		return "python -m pytest"
	}
	if _, err := os.Stat(filepath.Join(cwd, "pyproject.toml")); err == nil {
		return "python -m pytest"
	}
	if _, err := os.Stat(filepath.Join(cwd, "setup.py")); err == nil {
		return "python -m pytest"
	}

	// Check for Makefile with test target
	if _, err := os.Stat(filepath.Join(cwd, "Makefile")); err == nil {
		// Verify Makefile has a test target
		content, _ := os.ReadFile(filepath.Join(cwd, "Makefile"))
		if bytes.Contains(content, []byte("test:")) || bytes.Contains(content, []byte("test :")) {
			return "make test"
		}
	}

	return ""
}

// generateTestSummary creates a human-readable summary of test results.
func (te *ToolExecutor) generateTestSummary(output string, passed bool, command string) string {
	var summary strings.Builder

	if passed {
		summary.WriteString("Tests passed successfully.")
	} else {
		summary.WriteString("Tests failed.")
	}

	// Count test packages
	packageCount := strings.Count(output, "ok  ")
	if packageCount == 0 {
		packageCount = strings.Count(output, "ok ")
	}

	// Count failures
	failureCount := strings.Count(output, "--- FAIL:")
	failureCount += strings.Count(output, "FAIL:")

	// Count errors
	errorCount := strings.Count(output, "FAIL")

	summary.WriteString(fmt.Sprintf("\nCommand: %s", command))
	if packageCount > 0 {
		summary.WriteString(fmt.Sprintf("\nPackages: %d", packageCount))
	}
	if failureCount > 0 {
		summary.WriteString(fmt.Sprintf("\nFailures: %d", failureCount))
	}
	if errorCount > 0 {
		summary.WriteString(fmt.Sprintf("\nErrors: %d", errorCount))
	}

	// Extract individual failure names
	failingTests := extractFailingTests(output)
	if len(failingTests) > 0 {
		summary.WriteString("\nFailing tests:\n")
		for _, test := range failingTests[:min(len(failingTests), 10)] {
			summary.WriteString(fmt.Sprintf("  - %s\n", test))
		}
		if len(failingTests) > 10 {
			summary.WriteString(fmt.Sprintf("  ... and %d more\n", len(failingTests)-10))
		}
	}

	return summary.String()
}

// extractFailingTests extracts names of failing tests from output.
func extractFailingTests(output string) []string {
	var failures []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Match patterns like "--- FAIL: TestFoo (0.00s)" or "FAIL: TestBar:"
		if strings.HasPrefix(strings.TrimSpace(line), "--- FAIL:") {
			// Extract test name: "--- FAIL: TestFoo (0.00s)"
			rest := strings.TrimPrefix(strings.TrimSpace(line), "--- FAIL:")
			name := strings.TrimSpace(rest)
			if idx := strings.Index(name, "("); idx != -1 {
				name = name[:idx]
			}
			name = strings.TrimSpace(name)
			if name != "" {
				failures = append(failures, name)
			}
		} else if strings.Contains(line, "FAIL:") && strings.Contains(line, "Test") {
			// Match patterns like "FAIL: TestBar:"
			parts := strings.Split(line, "FAIL:")
			if len(parts) > 1 {
				name := strings.TrimSpace(parts[1])
				if idx := strings.Index(name, "("); idx != -1 {
					name = name[:idx]
				}
				if idx := strings.Index(name, ":"); idx != -1 {
					name = name[:idx]
				}
				name = strings.TrimSpace(name)
				if name != "" {
					failures = append(failures, name)
				}
			}
		}
	}
	return failures
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// treeEntry represents a single entry in the directory tree.
type treeEntry struct {
	Name      string
	RelPath   string // Full relative path from root
	IsDir     bool
	IsSymlink bool
	Mode      os.FileMode
	Size      int64
	ModTime   time.Time
}

// pathNode represents a node in the directory tree structure.
type pathNode struct {
	name     string
	entry    treeEntry
	children map[string]*pathNode
	isLeaf   bool // true if this node is an actual file/dir entry, not just a path component
}

// executeProjectTree generates a visual directory tree with file metadata.
func (te *ToolExecutor) executeProjectTree(params map[string]interface{}) *ToolResult {
	// Determine path (default: current directory)
	path := "."
	if p, ok := params["path"].(string); ok && p != "" {
		path = p
	}

	// Clean path
	cleanPath := filepath.Clean(path)

	// Check if path exists and is a directory
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("path not found: %s", cleanPath),
			}
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("cannot access path: %v", err),
		}
	}
	if !info.IsDir() {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("path is not a directory: %s", cleanPath),
		}
	}

	// Determine max depth (default: 3)
	maxDepth := 3
	if md, ok := params["max_depth"].(float64); ok {
		maxDepth = int(md)
	} else if md, ok := params["max_depth"].(int); ok {
		maxDepth = md
	} else if md, ok := params["max_depth"].(string); ok {
		if n, err := strconv.Atoi(md); err == nil {
			maxDepth = n
		}
	}

	// Determine show hidden flag (default: true - show hidden files)
	showHidden := true
	if sh, ok := params["show_hidden"].(bool); ok {
		showHidden = sh
	}

	// Determine max entries (default: 100 per level)
	maxEntries := 100
	if me, ok := params["max_entries"]; ok {
		switch v := me.(type) {
		case float64:
			maxEntries = int(v)
		case int:
			maxEntries = v
		case string:
			if n, err := strconv.Atoi(v); err == nil {
				maxEntries = n
			}
		}
	}

	// Collect all entries recursively with depth tracking
	var allEntries []treeEntry

	walkFn := func(walkPath string, d os.DirEntry, depth int) error {
		if depth > maxDepth {
			return filepath.SkipDir
		}

		// Skip the root directory itself
		rel, _ := filepath.Rel(cleanPath, walkPath)
		if rel == "." || rel == "" {
			return nil
		}

		// Check hidden files/directories
		baseName := filepath.Base(walkPath)
		if !showHidden && strings.HasPrefix(baseName, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Get file info
		fileInfo, err := d.Info()
		if err != nil {
			return nil
		}

		// Check if it's a symlink
		isSymlink := d.Type()&os.ModeSymlink != 0

		allEntries = append(allEntries, treeEntry{
			Name:      baseName,
			RelPath:   rel,
			IsDir:     d.IsDir() || (isSymlink && fileInfo != nil && fileInfo.IsDir()),
			IsSymlink: isSymlink,
			Mode:      fileInfo.Mode(),
			Size:      fileInfo.Size(),
			ModTime:   fileInfo.ModTime(),
		})

		return nil
	}

	// Walk the directory
	err = filepath.WalkDir(cleanPath, func(walkPath string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}
		rel, _ := filepath.Rel(cleanPath, walkPath)
		depth := 0
		if rel != "." && rel != "" {
			depth = strings.Count(rel, string(filepath.Separator)) + 1
		}
		return walkFn(walkPath, d, depth)
	})
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to walk directory: %v", err),
		}
	}

	// Build the tree structure
	treeOutput := buildTree(cleanPath, allEntries, maxEntries)

	// Count stats
	totalDirs := 0
	totalFiles := 0
	totalSymlinks := 0
	totalSize := int64(0)
	for _, entry := range allEntries {
		if entry.IsSymlink {
			totalSymlinks++
		} else if entry.IsDir {
			totalDirs++
		} else {
			totalFiles++
			totalSize += entry.Size
		}
	}

	return &ToolResult{
		Success: true,
		Output:  treeOutput,
		Extra: map[string]interface{}{
			"tool":          "project_tree",
			"path":          cleanPath,
			"maxDepth":      maxDepth,
			"totalDirs":     totalDirs,
			"totalFiles":    totalFiles,
			"totalSymlinks": totalSymlinks,
			"totalSize":     totalSize,
		},
	}
}

// buildTree constructs a visual tree string from flat entries.
func buildTree(rootPath string, entries []treeEntry, maxEntries int) string {
	var buf strings.Builder

	// Root directory header
	cleanRoot := filepath.Clean(rootPath)
	if cleanRoot == "." {
		buf.WriteString("📁 .\n")
	} else {
		buf.WriteString(fmt.Sprintf("📁 %s\n", cleanRoot))
	}

	if len(entries) == 0 {
		buf.WriteString("  (empty)\n")
		return buf.String()
	}

	// Build a map from relative path to entry for quick lookup
	entryByPath := make(map[string]treeEntry)
	for _, e := range entries {
		entryByPath[e.RelPath] = e
	}

	// Create root node
	root := &pathNode{
		name:     "",
		entry:    treeEntry{IsDir: true},
		children: make(map[string]*pathNode),
	}

	// Sort entries by their relative path length first (parents before children)
	sorted := make([]treeEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		// Split by path segments
		segmentsI := strings.Count(sorted[i].RelPath, string(filepath.Separator))
		segmentsJ := strings.Count(sorted[j].RelPath, string(filepath.Separator))
		if segmentsI != segmentsJ {
			return segmentsI < segmentsJ
		}
		return sorted[i].RelPath < sorted[j].RelPath
	})

	// Insert each entry into the tree
	for _, e := range sorted {
		parts := strings.Split(e.RelPath, string(filepath.Separator))
		current := root

		for _, part := range parts {
			if _, exists := current.children[part]; !exists {
				// Find the entry for this path component
				var ent treeEntry
				for idx := 0; idx < len(parts); idx++ {
					if parts[idx] == part {
						// Build the segment path up to this point
						segment := ""
						for k := 0; k <= idx; k++ {
							if k > 0 {
								segment += string(filepath.Separator)
							}
							segment += parts[k]
						}
						if ie, ok := entryByPath[segment]; ok {
							ent = ie
						} else {
							ent = treeEntry{Name: part, RelPath: part}
						}
						break
					}
				}
				current.children[part] = &pathNode{
					name:     part,
					entry:    ent,
					children: make(map[string]*pathNode),
				}
			}
			current = current.children[part]
		}

		// Mark this node as an actual entry
		current.isLeaf = true
		if e.Name != "" {
			current.entry = e
		}
	}

	// Render the tree
	renderTree(root, "", true, &buf, maxEntries, 0)

	if len(entries) > maxEntries {
		buf.WriteString(fmt.Sprintf("\n... (%d entries shown, limited by max_entries)", maxEntries))
	}

	return buf.String()
}

// renderTree recursively renders a tree node with tree-drawing characters.
func renderTree(node *pathNode, prefix string, isLast bool, buf *strings.Builder, maxEntries int, depth int) {
	if node.name == "" {
		// Root node - render children directly
		children := sortedChildren(node.children)
		for i, child := range children {
			if depth > maxEntries {
				buf.WriteString(prefix + "... (more entries omitted)\n")
				return
			}
			renderTree(child, "", i == len(children)-1, buf, maxEntries, depth+1)
		}
		return
	}

	// Draw this node
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	displayName := node.name
	var icon string

	if node.entry.IsSymlink {
		icon = "🔗 "
		// Try to read symlink target
		if target, err := os.Readlink(filepath.Join(prefix, displayName)); err == nil {
			displayName = fmt.Sprintf("%s -> %s", displayName, target)
		}
	} else if node.entry.IsDir {
		icon = "📁 "
	} else {
		// File with extension icon
		ext := strings.ToLower(filepath.Ext(displayName))
		switch ext {
		case ".go":
			icon = "🐹 "
		case ".py":
			icon = "🐍 "
		case ".rs":
			icon = "🦀 "
		case ".js", ".mjs", ".cjs":
			icon = "📜 "
		case ".ts", ".tsx":
			icon = "🔷 "
		case ".json":
			icon = "📋 "
		case ".yaml", ".yml":
			icon = "⚙️ "
		case ".toml":
			icon = "📝 "
		case ".md":
			icon = "📄 "
		case ".html":
			icon = "🌐 "
		case ".css":
			icon = "🎨 "
		case ".sh":
			icon = "⚡ "
		case ".txt":
			icon = "📃 "
		case ".xml":
			icon = "📰 "
		case ".proto":
			icon = "🔧 "
		case ".sql":
			icon = "🗃️ "
		case ".env":
			icon = "🔒 "
		default:
			icon = "📄 "
		}
	}

	// Format size for files
	sizeStr := ""
	if !node.entry.IsDir && node.entry.Size > 0 {
		sizeStr = fmt.Sprintf(" (%s)", formatFileSize(node.entry.Size))
	}

	buf.WriteString(fmt.Sprintf("%s%s%s%s%s\n", prefix, connector, icon, displayName, sizeStr))

	// Render children if directory
	if node.entry.IsDir || node.entry.IsSymlink {
		children := sortedChildren(node.children)
		if len(children) > 0 {
			newPrefix := prefix
			if prefix == "" {
				newPrefix = "    "
			} else if isLast {
				newPrefix = prefix + "    "
			} else {
				newPrefix = prefix + "│   "
			}

			for i, child := range children {
				renderTree(child, newPrefix, i == len(children)-1, buf, maxEntries, depth+1)
			}
		} else {
			buf.WriteString(prefix + "    (empty)\n")
		}
	}
}

// sortedChildren returns children sorted with directories first, then files alphabetically.
func sortedChildren(children map[string]*pathNode) []*pathNode {
	result := make([]*pathNode, 0, len(children))
	for _, c := range children {
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool {
		// Directories first
		if result[i].entry.IsDir != result[j].entry.IsDir {
			return result[i].entry.IsDir
		}
		// Alphabetical within same type
		return result[i].name < result[j].name
	})
	return result
}

// executeCodeNavigation provides code navigation capabilities: find definitions, find references, and find implementations.
func (te *ToolExecutor) executeCodeNavigation(params map[string]interface{}) *ToolResult {
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: query (symbol name to search for)",
		}
	}

	mode, ok := params["mode"].(string)
	if !ok || mode == "" {
		mode = "definitions" // default mode
	}
	switch mode {
	case "definitions", "references", "implementations":
		// valid modes
	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid mode: %s. Valid modes: definitions, references, implementations", mode),
		}
	}

	// Optional: file_type to limit search (e.g., "go", "python", "typescript")
	fileType, hasFileType := params["file_type"].(string)
	if !hasFileType {
		fileType = ""
	}

	// Optional: paths to restrict search
	pathsParam, hasPaths := params["paths"]
	var searchPaths []string
	if hasPaths {
		switch v := pathsParam.(type) {
		case []interface{}:
			for _, p := range v {
				searchPaths = append(searchPaths, fmt.Sprintf("%v", p))
			}
		case string:
			searchPaths = []string{v}
		}
	}

	// Optional: max_results (default: 30)
	maxResults := 30
	if mr, ok := params["max_results"].(float64); ok {
		maxResults = int(mr)
	} else if mr, ok := params["max_results"].(int); ok {
		maxResults = mr
	} else if mr, ok := params["max_results"].(string); ok {
		if n, err := strconv.Atoi(mr); err == nil {
			maxResults = n
		}
	}

	// Determine file extension patterns based on file_type
	var filePatterns []string
	if fileType != "" {
		filePatterns = getFileExtensionPatterns(fileType)
	} else {
		filePatterns = getAllFileExtensionPatterns()
	}

	// Build glob patterns for search
	var globPatterns []string
	for _, ext := range filePatterns {
		if len(searchPaths) > 0 {
			for _, sp := range searchPaths {
				globPatterns = append(globPatterns, fmt.Sprintf("%s/**/*.%s", sp, ext))
			}
		} else {
			globPatterns = append(globPatterns, fmt.Sprintf("**/*.%s", ext))
		}
	}

	// Build regex patterns based on mode and file_type
	var patterns []string
	if mode == "definitions" {
		patterns = getDefinitionPatterns(query, fileType)
	} else if mode == "references" {
		patterns = getReferencePatterns(query, fileType)
	} else if mode == "implementations" {
		patterns = getImplementationPatterns(query, fileType)
	}

	// Search using grep via bash
	var allResults []string
	var filesSearched int
	var totalMatches int

	for _, pattern := range patterns {
		for _, ext := range filePatterns {
			var grepArgs string
			if len(searchPaths) > 0 {
				grepArgs = fmt.Sprintf("grep -rn --include=*.%s -e '%s' %s 2>/dev/null | head -n %d", ext, pattern, strings.Join(searchPaths, " "), maxResults)
			} else {
				grepArgs = fmt.Sprintf("grep -rn --include=*.%s -e '%s' . 2>/dev/null | head -n %d", ext, pattern, maxResults)
			}

			cmd := exec.Command("bash", "-c", grepArgs)
			output, err := cmd.CombinedOutput()
			if err != nil {
				continue // grep returns non-zero if no matches, that's OK
			}

			outputStr := strings.TrimSpace(string(output))
			if outputStr == "" {
				continue
			}

			lines := strings.Split(outputStr, "\n")
			filesSearched++
			for _, line := range lines {
				if len(allResults) >= maxResults {
					goto done
				}
				allResults = append(allResults, line)
				totalMatches++
			}
		}
	}

done:
	// Format results
	var output strings.Builder

	switch mode {
	case "definitions":
		output.WriteString(fmt.Sprintf("Found %d definition(s) for '%s' in %d file(s):\n\n", totalMatches, query, filesSearched))
	case "references":
		output.WriteString(fmt.Sprintf("Found %d reference(s) to '%s' in %d file(s):\n\n", totalMatches, query, filesSearched))
	case "implementations":
		output.WriteString(fmt.Sprintf("Found %d implementation(s) for '%s' in %d file(s):\n\n", totalMatches, query, filesSearched))
	}

	for _, result := range allResults {
		output.WriteString("  " + result + "\n")
	}

	if totalMatches == 0 {
		output.WriteString("  (no results found)")
	}

	if len(allResults) >= maxResults {
		output.WriteString(fmt.Sprintf("\n\n... (showing %d of %d+ results, limited by max_results)", maxResults, maxResults))
	}

	return &ToolResult{
		Success: true,
		Output:  output.String(),
		Extra: map[string]interface{}{
			"tool":         "code_navigation",
			"mode":         mode,
			"query":        query,
			"fileType":     fileType,
			"filesSearched": filesSearched,
			"matchesFound": totalMatches,
		},
	}
}

// getFileExtensionPatterns returns file extension patterns for a given language.
func getFileExtensionPatterns(fileType string) []string {
	switch strings.ToLower(fileType) {
	case "go":
		return []string{"go"}
	case "python", "py":
		return []string{"py"}
	case "javascript", "js":
		return []string{"js", "jsx"}
	case "typescript", "ts":
		return []string{"ts", "tsx"}
	case "rust", "rs":
		return []string{"rs"}
	case "java":
		return []string{"java"}
	case "c":
		return []string{"c", "h"}
	case "cpp", "cxx", "cc":
		return []string{"cpp", "cxx", "cc", "hpp", "hxx", "hh"}
	case "ruby", "rb":
		return []string{"rb"}
	case "php":
		return []string{"php"}
	case "csharp", "cs":
		return []string{"cs"}
	case "swift":
		return []string{"swift"}
	case "kotlin", "kt":
		return []string{"kt", "kts"}
	case "scala":
		return []string{"scala", "sc"}
	case "shell", "bash", "sh":
		return []string{"sh", "bash"}
	case "yaml", "yml":
		return []string{"yaml", "yml"}
	case "json":
		return []string{"json"}
	default:
		return []string{fileType}
	}
}

// getAllFileExtensionPatterns returns common source file extensions to search.
func getAllFileExtensionPatterns() []string {
	return []string{
		"go", "py", "js", "jsx", "ts", "tsx", "rs", "java", "c", "h",
		"cpp", "hpp", "rb", "php", "cs", "swift", "kt", "scala", "sh", "bash",
		"yaml", "yml", "toml", "lua", "r", "m", "mm",
	}
}

// getDefinitionPatterns returns regex patterns for finding definitions of a symbol.
func getDefinitionPatterns(query string, fileType string) []string {
	// Escape special regex characters in query
	escapedQuery := regexp.QuoteMeta(query)

	var patterns []string

	if fileType == "" || fileType == "go" {
		patterns = append(patterns,
			fmt.Sprintf(`^func\s+.*\b%s\b\s*\(`, escapedQuery),    // func definitions
			fmt.Sprintf(`^func\s+\(\s*\S+\s+\*?\S+\s*\)\s+%s\b`, escapedQuery), // method definitions
			fmt.Sprintf(`^type\s+%s\b\s+`, escapedQuery),           // type definitions
			fmt.Sprintf(`^var\s+\b%s\b`, escapedQuery),             // var definitions
			fmt.Sprintf(`^const\s+\b%s\b`, escapedQuery),           // const definitions
			fmt.Sprintf(`^\w+\s+\b%s\b\s*[=:]\s*func`, escapedQuery), // func variable assignments
		)
	}

	if fileType == "" || fileType == "python" || fileType == "py" {
		patterns = append(patterns,
			fmt.Sprintf(`^\s*def\s+%s\s*\(`, escapedQuery),     // function definitions
			fmt.Sprintf(`^\s*class\s+%s\b`, escapedQuery),       // class definitions
			fmt.Sprintf(`^\s*%s\s*=\s*lambda\s*`, escapedQuery),  // lambda assignments
			fmt.Sprintf(`^\s*%s\s*=\s*(async\s+)?function`, escapedQuery), // function assignments
		)
	}

	if fileType == "" || (fileType == "javascript" || fileType == "js" || fileType == "typescript" || fileType == "ts") {
		patterns = append(patterns,
			fmt.Sprintf(`^\s*function\s+%s\b`, escapedQuery),           // function declarations
			fmt.Sprintf(`^\s*(?:export\s+)?function\s+%s\b`, escapedQuery), // export function
			fmt.Sprintf(`^\s*(?:const|let|var)\s+%s\s*=`, escapedQuery),  // variable/function assignments
			fmt.Sprintf(`^\s*(?:export\s+)?(?:const|let|var)\s+%s\s*=`, escapedQuery), // export variable
			fmt.Sprintf(`^\s*class\s+%s\b`, escapedQuery),              // class declarations
			fmt.Sprintf(`^\s*\w+\.%s\s*=`, escapedQuery),               // method assignments
			fmt.Sprintf(`^\s*%s\s*:\s*(?:async\s+)?function`, escapedQuery), // object method shorthand
			fmt.Sprintf(`^\s*%s\s*:\s*\(`, escapedQuery),              // arrow function
		)
	}

	if fileType == "" || fileType == "rust" || fileType == "rs" {
		patterns = append(patterns,
			fmt.Sprintf(`^pub\s+fn\s+%s\b`, escapedQuery),       // pub fn
			fmt.Sprintf(`^fn\s+%s\b`, escapedQuery),             // fn
			fmt.Sprintf(`^pub\s+struct\s+%s\b`, escapedQuery),   // pub struct
			fmt.Sprintf(`^struct\s+%s\b`, escapedQuery),         // struct
			fmt.Sprintf(`^pub\s+enum\s+%s\b`, escapedQuery),     // pub enum
			fmt.Sprintf(`^enum\s+%s\b`, escapedQuery),           // enum
			fmt.Sprintf(`^impl\s+.*\b%s\b`, escapedQuery),       // impl blocks
		)
	}

	// Fallback: just the symbol name for any file type
	patterns = append(patterns, fmt.Sprintf(`\b%s\b`, escapedQuery))

	return patterns
}

// getReferencePatterns returns regex patterns for finding references to a symbol.
func getReferencePatterns(query string, fileType string) []string {
	escapedQuery := regexp.QuoteMeta(query)
	return []string{
		// Word-bounded reference - matches any usage of the symbol
		fmt.Sprintf(`\b%s\b`, escapedQuery),
	}
}

// getImplementationPatterns returns regex patterns for finding interface implementations.
func getImplementationPatterns(query string, fileType string) []string {
	escapedQuery := regexp.QuoteMeta(query)
	var patterns []string

	if fileType == "" || fileType == "go" {
		patterns = append(patterns,
			fmt.Sprintf(`^\s*func\s+\(\s*\*?%s\b`, escapedQuery), // method on type implementing interface
			fmt.Sprintf(`^\s*type\s+%s\b`, escapedQuery),           // the type itself
		)
	}

	if fileType == "" || fileType == "python" || fileType == "py" {
		patterns = append(patterns,
			fmt.Sprintf(`^\s*class\s+%s\b`, escapedQuery),          // class definition
			fmt.Sprintf(`^\s*class\s+\S+\s*\(\s*.*%s.*\)\s*:`, escapedQuery), // class inheriting from interface-like base
		)
	}

	if fileType == "" || (fileType == "javascript" || fileType == "js" || fileType == "typescript" || fileType == "ts") {
		patterns = append(patterns,
			fmt.Sprintf(`^\s*class\s+%s\b`, escapedQuery),              // class definition
			fmt.Sprintf(`^\s*class\s+\S+\s+extends\s+.*%s`, escapedQuery), // class extending interface-like class
			fmt.Sprintf(`^\s*\w+\.%s\s*[:=]`, escapedQuery),            // property assignment matching interface
			fmt.Sprintf(`implements\s+.*%s\b`, escapedQuery),          // implements clause
			fmt.Sprintf(`implements\s+%s\b`, escapedQuery),            // direct implements
		)
	}

	if fileType == "" || fileType == "rust" || fileType == "rs" {
		patterns = append(patterns,
			fmt.Sprintf(`^\s*impl\s+.*%s.*\s+for\s+%s\b`, escapedQuery, escapedQuery), // impl Trait for Type
			fmt.Sprintf(`^\s*impl\s+%s\b`, escapedQuery),                              // impl block for type
		)
	}

	return patterns
}

// executeCodeNavigationResult represents a single navigation result entry.
type codeNavigationResult struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Content  string `json:"content"`
	MatchType string `json:"match_type,omitempty"`
}

// brokenLink represents a broken link found during scanning.
type brokenLink struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Type       string `json:"type"`
	LinkValue  string `json:"link_value"`
	LinkText   string `json:"link_text,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// linkInfo represents a link found during scanning.
type linkInfo struct {
	FilePath string
	Line     int
	Type     string
	Value    string
	Text     string
}

// executeCheckLinks scans files for broken internal and external links.
func (te *ToolExecutor) executeCheckLinks(params map[string]interface{}) *ToolResult {
	// Determine paths to search (default: current directory recursively)
	var pathPatterns []string
	if pathsParam, hasPaths := params["paths"]; hasPaths {
		switch v := pathsParam.(type) {
		case []interface{}:
			for _, p := range v {
				pathPatterns = append(pathPatterns, fmt.Sprintf("%v", p))
			}
		case string:
			if strings.Contains(v, ",") {
				for _, p := range strings.Split(v, ",") {
					pathPatterns = append(pathPatterns, strings.TrimSpace(p))
				}
			} else {
				pathPatterns = []string{v}
			}
		}
	}

	// Determine file types to scan (default: .md, .html, .htm)
	var fileTypes []string
	if ftParam, hasFT := params["file_types"]; hasFT {
		switch v := ftParam.(type) {
		case []interface{}:
			for _, ft := range v {
				fileTypes = append(fileTypes, strings.ToLower(fmt.Sprintf("%v", ft)))
			}
		case string:
			if strings.Contains(v, ",") {
				for _, ft := range strings.Split(v, ",") {
					fileTypes = append(fileTypes, strings.TrimSpace(strings.ToLower(ft)))
				}
			} else {
				fileTypes = []string{strings.TrimSpace(strings.ToLower(v))}
			}
		}
	}
	if len(fileTypes) == 0 {
		fileTypes = []string{".md", ".html", ".htm"}
	}

	// Determine root directory for resolving relative links (default: current directory)
	rootDir := "."
	if rd, ok := params["root_dir"].(string); ok && rd != "" {
		rootDir = rd
	}
	_ = rootDir // root_dir is used implicitly via current working directory

	// Determine timeout for external link checks (default: 10 seconds)
	timeoutSeconds := 10
	if tp, ok := params["timeout"].(float64); ok {
		timeoutSeconds = int(tp)
	} else if tp, ok := params["timeout"].(int); ok {
		timeoutSeconds = tp
	} else if tp, ok := params["timeout"].(string); ok {
		if n, err := strconv.Atoi(tp); err == nil {
			timeoutSeconds = n
		}
	}

	// Collect files to scan
	var targetFiles []string
	if len(pathPatterns) > 0 {
		for _, p := range pathPatterns {
			matches, err := te.globRecursive(p, 10000)
			if err != nil {
				continue
			}
			for _, m := range matches {
				lowerM := strings.ToLower(m)
				for _, ft := range fileTypes {
					ftClean := strings.TrimPrefix(ft, ".")
					if strings.HasSuffix(lowerM, "."+ftClean) {
						targetFiles = append(targetFiles, m)
						break
					}
				}
			}
		}
	} else {
		matches, err := te.globRecursive("**", 10000)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to discover files: %v", err),
			}
		}
		for _, m := range matches {
			lowerM := strings.ToLower(m)
			for _, ft := range fileTypes {
				ftClean := strings.TrimPrefix(ft, ".")
				if strings.HasSuffix(lowerM, "."+ftClean) {
					targetFiles = append(targetFiles, m)
					break
				}
			}
		}
	}

	if len(targetFiles) == 0 {
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("No %v files found to scan.\n\nTotal links found: 0\nBroken internal links: 0\nBroken external links: 0", fileTypes),
			Extra: map[string]interface{}{
				"tool":                "check_links",
				"totalLinksFound":     0,
				"totalFilesScanned":   0,
				"internalLinksOk":     0,
				"internalLinksBroken": 0,
				"externalLinksOk":     0,
				"externalLinksBroken": 0,
			},
		}
	}

	// Scan all files for links
	var allLinks []linkInfo
	for _, filePath := range targetFiles {
		links := scanFileForLinks(filePath, fileTypes)
		for i := range links {
			links[i].FilePath = filePath
		}
		allLinks = append(allLinks, links...)
	}

	// Validate internal links
	var internalOk, internalBroken, externalOk, externalBroken int
	var brokenLinks []brokenLink

	// Collect unique external URLs to check
	externalURLs := make(map[string][]linkInfo)

	for _, link := range allLinks {
		if link.Type == "internal" {
			fileDir := filepath.Dir(link.FilePath)
			resolvedPath := filepath.Clean(filepath.Join(fileDir, link.Value))
			pathOnly := link.Value
			if idx := strings.Index(pathOnly, "#"); idx != -1 {
				pathOnly = pathOnly[:idx]
			}
			if _, err := os.Stat(resolvedPath); err == nil {
				internalOk++
			} else {
				internalBroken++
				brokenLinks = append(brokenLinks, brokenLink{
					File:      link.FilePath,
					Line:      link.Line,
					Type:      "internal",
					LinkValue: link.Value,
					LinkText:  link.Text,
					Reason:    fmt.Sprintf("file not found: %s", resolvedPath),
				})
			}
		} else {
			externalURLs[link.Value] = append(externalURLs[link.Value], link)
		}
	}

	// Check external links with limited concurrency (max 5 simultaneous)
	semaphore := make(chan struct{}, 5)
	type urlCheckResult struct {
		url    string
		ok     bool
		code   int
		reason string
	}

	checkDone := make(chan urlCheckResult, len(externalURLs))
	for url, locations := range externalURLs {
		semaphore <- struct{}{}
		go func(u string, locs []linkInfo) {
			defer func() { <-semaphore }()

			client := &http.Client{
				Timeout: time.Duration(timeoutSeconds) * time.Second,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}

			req, err := http.NewRequestWithContext(context.Background(), "HEAD", u, nil)
			if err != nil {
				for _, loc := range locs {
					brokenLinks = append(brokenLinks, brokenLink{
						File:     loc.FilePath,
						Line:     loc.Line,
						Type:     "external",
						LinkValue: u,
						LinkText: loc.Text,
						Reason:   fmt.Sprintf("invalid URL: %v", err),
					})
				}
				checkDone <- urlCheckResult{url: u, ok: true}
				return
			}

			req.Header.Set("User-Agent", "coding-agent/1.0")
			resp, err := client.Do(req)
			if err != nil {
				// Try GET as fallback
				req, err = http.NewRequestWithContext(context.Background(), "GET", u, nil)
				if err != nil {
					for _, loc := range locs {
						brokenLinks = append(brokenLinks, brokenLink{
							File:     loc.FilePath,
							Line:     loc.Line,
							Type:     "external",
							LinkValue: u,
							LinkText: loc.Text,
							Reason:   fmt.Sprintf("request failed: %v", err),
						})
					}
					checkDone <- urlCheckResult{url: u, ok: true}
					return
				}
				resp, err = client.Do(req)
				if err != nil {
					for _, loc := range locs {
						brokenLinks = append(brokenLinks, brokenLink{
							File:     loc.FilePath,
							Line:     loc.Line,
							Type:     "external",
							LinkValue: u,
							LinkText: loc.Text,
							Reason:   fmt.Sprintf("request failed: %v", err),
						})
					}
					checkDone <- urlCheckResult{url: u, ok: true}
					return
				}
			}
			resp.Body.Close()

			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				externalOk++
			} else {
				externalBroken++
				for _, loc := range locs {
					brokenLinks = append(brokenLinks, brokenLink{
						File:       loc.FilePath,
						Line:       loc.Line,
						Type:       "external",
						LinkValue:  u,
						LinkText:   loc.Text,
						StatusCode: resp.StatusCode,
						Reason:     fmt.Sprintf("HTTP %d %s", resp.StatusCode, resp.Status),
					})
				}
			}
			checkDone <- urlCheckResult{url: u, ok: true}
		}(url, locations)
	}

	// Wait for all checks to complete
	for i := 0; i < len(externalURLs); i++ {
		<-checkDone
	}

	// Format output
	var output strings.Builder
	totalLinks := internalOk + internalBroken + externalOk + externalBroken

	output.WriteString(fmt.Sprintf("Link check complete: %d link(s) found across %d file(s)\n\n", totalLinks, len(targetFiles)))
	output.WriteString("Summary:\n")
	output.WriteString(fmt.Sprintf("  Internal links: %d ok, %d broken\n", internalOk, internalBroken))
	output.WriteString(fmt.Sprintf("  External links: %d ok, %d broken\n\n", externalOk, externalBroken))

	if len(brokenLinks) > 0 {
		output.WriteString("Broken links:\n")
		for _, bl := range brokenLinks {
			displayText := bl.LinkText
			if displayText == "" {
				displayText = bl.LinkValue
			}
			output.WriteString(fmt.Sprintf("  [%s] %s:%d: %s\n", bl.Type, bl.File, bl.Line, bl.LinkValue))
			if displayText != bl.LinkValue {
				output.WriteString(fmt.Sprintf("    Text: %s\n", displayText))
			}
			output.WriteString(fmt.Sprintf("    Reason: %s\n", bl.Reason))
		}
	} else {
		output.WriteString("All links are valid! ✓\n")
	}

	result := &ToolResult{
		Success:  internalBroken == 0 && externalBroken == 0,
		Output:   output.String(),
		Extra: map[string]interface{}{
			"tool":                "check_links",
			"totalLinksFound":     totalLinks,
			"totalFilesScanned":   len(targetFiles),
			"internalLinksOk":     internalOk,
			"internalLinksBroken": internalBroken,
			"externalLinksOk":     externalOk,
			"externalLinksBroken": externalBroken,
		},
	}

	if len(brokenLinks) > 0 {
		result.Extra["brokenLinks"] = brokenLinks
	}

	return result
}

// scanFileForLinks scans a single file for links (both internal and external).
func scanFileForLinks(filePath string, fileTypes []string) []linkInfo {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	var lowerFileTypes []string
	for _, ft := range fileTypes {
		lowerFileTypes = append(lowerFileTypes, strings.ToLower(ft))
	}

	isMarkdown := false
	for _, ft := range lowerFileTypes {
		if ft == ".md" || ft == ".markdown" || ft == ".mdown" {
			isMarkdown = true
			break
		}
	}

	isHTML := false
	for _, ft := range lowerFileTypes {
		if ft == ".html" || ft == ".htm" {
			isHTML = true
			break
		}
	}

	var links []linkInfo
	lines := strings.Split(string(content), "\n")

	for lineNum, line := range lines {
		if isMarkdown {
			links = append(links, scanMarkdownLine(line, lineNum+1)...)
		}
		if isHTML {
			links = append(links, scanHTMLLine(line, lineNum+1)...)
		}
	}

	return links
}

// scanMarkdownLine scans a single markdown line for links.
func scanMarkdownLine(line string, lineNum int) []linkInfo {
	var links []linkInfo

	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "<!--") || strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "<code>") {
		return links
	}

	// Skip code fences
	if strings.HasPrefix(trimmed, "```") {
		return links
	}

	// Match [text](url) or ![alt](url) - but not inline code
	// Use a more careful regex that avoids code blocks
	re := regexp.MustCompile(`\[(?:[^\]]*)\]\(([^)]+)\)`)
	matches := re.FindAllStringSubmatch(line, -1)
	for _, m := range matches {
		linkValue := strings.TrimSpace(m[1])

		// Clean up title in parentheses if present
		if idx := strings.LastIndex(linkValue, `" `); idx > 0 {
			linkValue = linkValue[:idx]
		} else if idx := strings.LastIndex(linkValue, `' `); idx > 0 {
			linkValue = linkValue[:idx]
		} else if idx := strings.LastIndex(linkValue, ` (`); idx > 0 {
			linkValue = linkValue[:idx]
		}

		var linkText string
		idx := strings.Index(line, "[")
		if idx >= 0 {
			// Find the matching ]
			rest := line[idx:]
			bracketEnd := strings.Index(rest, "]")
			if bracketEnd > 0 {
				linkText = rest[:bracketEnd+1]
			}
		}

		if isExternalLink(linkValue) {
			links = append(links, linkInfo{
				FilePath: "",
				Line:     lineNum,
				Type:     "external",
				Value:    linkValue,
				Text:     linkText,
			})
		} else {
			links = append(links, linkInfo{
				FilePath: "",
				Line:     lineNum,
				Type:     "internal",
				Value:    linkValue,
				Text:     linkText,
			})
		}
	}

	return links
}

// scanHTMLLine scans a single HTML line for links.
func scanHTMLLine(line string, lineNum int) []linkInfo {
	var links []linkInfo

	// Match <a href="..."> or <a href='...'>
	aRe := regexp.MustCompile(`<a\s+[^>]*href\s*=\s*["']([^"']+)["']`)
	aMatches := aRe.FindAllStringSubmatch(line, -1)
	for _, m := range aMatches {
		linkValue := m[1]
		if isExternalLink(linkValue) {
			links = append(links, linkInfo{
				Line:    lineNum,
				Type:    "external",
				Value:   linkValue,
				Text:    "",
			})
		} else {
			links = append(links, linkInfo{
				Line:    lineNum,
				Type:    "internal",
				Value:   linkValue,
				Text:    "",
			})
		}
	}

	// Match <img src="...">
	imgRe := regexp.MustCompile(`<img\s+[^>]*src\s*=\s*["']([^"']+)["']`)
	imgMatches := imgRe.FindAllStringSubmatch(line, -1)
	for _, m := range imgMatches {
		linkValue := m[1]
		if isExternalLink(linkValue) {
			links = append(links, linkInfo{
				Line:    lineNum,
				Type:    "external",
				Value:   linkValue,
				Text:    "",
			})
		} else {
			links = append(links, linkInfo{
				Line:    lineNum,
				Type:    "internal",
				Value:   linkValue,
				Text:    "",
			})
		}
	}

	return links
}

// isExternalLink checks if a link value is an external URL.
func isExternalLink(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "//")
}
