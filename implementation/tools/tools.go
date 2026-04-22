// Package tools implements the tool execution system for the coding agent.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
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
	case "file_rename":
		result = te.executeFileRename(tc.Parameters)
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
	case "git_stash":
		result = te.executeGitStash(tc.Parameters)
	case "json_transformer":
		result = te.executeJsonTransformer(tc.Parameters)
	case "project_diagnostics":
		result = te.executeProjectDiagnostics(tc.Parameters)
	case "run_lint":
		result = te.executeRunLint(tc.Parameters)
	case "process_management":
		result = te.executeProcessManagement(tc.Parameters)
	case "env_var":
		result = te.executeEnvVar(tc.Parameters)
	case "file_compare":
		result = te.executeFileCompare(tc.Parameters)
	case "changelog":
		result = te.executeChangelog(tc.Parameters)
	case "git_tag":
		result = te.executeGitTag(tc.Parameters)
	case "run_build":
		result = te.executeRunBuild(tc.Parameters)
	case "run_coverage":
		result = te.executeRunCoverage(tc.Parameters)
	case "git_merge":
		result = te.executeGitMerge(tc.Parameters)
	case "git_revert":
		result = te.executeGitRevert(tc.Parameters)
	case "generate_docs":
		result = te.executeGenerateDocs(tc.Parameters)
	case "code_metrics":
		result = te.executeCodeMetrics(tc.Parameters)
	case "dependency_audit":
		result = te.executeDependencyAudit(tc.Parameters)
	case "testgen":
		result = te.executeTestGen(tc.Parameters)
	case "code_review":
		result = te.executeCodeReview(tc.Parameters)
	case "interactive_session":
		result = te.executeInteractiveSession(tc.Parameters)
	case "http_request":
		result = te.executeHttpRequest(tc.Parameters)
	case "csv_transformer":
		result = te.executeCsvTransformer(tc.Parameters)
	case "git_blame":
		result = te.executeGitBlame(tc.Parameters)
	case "git_cherry_pick":
		result = te.executeGitCherryPick(tc.Parameters)
	case "git_commit_msg":
		result = te.executeGitCommitMsg(tc.Parameters)
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

// executeFileRename moves a file and updates all code references (imports, includes, etc.)
// across the codebase that reference the old filename.
func (te *ToolExecutor) executeFileRename(params map[string]interface{}) *ToolResult {
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

	// Prevent directory traversal on destination
	if strings.HasPrefix(cleanDest, "..") {
		return &ToolResult{
			Success: false,
			Error:   "invalid destination path: directory traversal not allowed",
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

	// Extract the old and new basenames for reference search
	oldBase := filepath.Base(cleanSrc)
	newBase := filepath.Base(cleanDest)

	// If old and new basenames are the same (rename within directory), find references
	var referencesUpdated []string
	if oldBase != newBase {
		referencesUpdated = te.findAndUpdateReferences(cleanSrc, cleanDest, oldBase, newBase, params)
	}

	// Build result
	output := fmt.Sprintf("Renamed '%s' -> '%s'", cleanSrc, cleanDest)
	if len(referencesUpdated) > 0 {
		output += fmt.Sprintf(" (updated %d file reference(s))", len(referencesUpdated))
	}

	return &ToolResult{
		Success: true,
		Output:  output,
		Extra: map[string]interface{}{
			"source":           cleanSrc,
			"destination":      cleanDest,
			"operation":        "rename",
			"referencesUpdated": referencesUpdated,
		},
	}
}

// findAndUpdateReferences searches for files referencing the old filename
// and updates them to use the new filename.
func (te *ToolExecutor) findAndUpdateReferences(cleanSrc, cleanDest, oldBase, newBase string, params map[string]interface{}) []string {
	var updated []string

	// Determine the search scope
	searchPathsParam, hasSearchPaths := params["search_paths"]
	var searchPatterns []string
	if hasSearchPaths {
		switch v := searchPathsParam.(type) {
		case []interface{}:
			for _, p := range v {
				if ps, ok := p.(string); ok {
					searchPatterns = append(searchPatterns, ps)
				}
			}
		case string:
			searchPatterns = append(searchPatterns, v)
		}
	}

	if len(searchPatterns) == 0 {
		// Default: search all files recursively, excluding common non-code files
		searchPatterns = []string{"**/*"}
	}

	// File extensions that commonly contain import/reference statements
	codeExtensions := map[string]bool{
		".go":    true,
		".py":    true,
		".js":    true,
		".ts":    true,
		".tsx":   true,
		".jsx":   true,
		".java":  true,
		".rs":    true,
		".rb":    true,
		".php":   true,
		".cs":    true,
		".cpp":   true,
		".c":     true,
		".h":     true,
		".hpp":   true,
		".cc":    true,
		".hh":    true,
		".swift": true,
		".kt":    true,
		".kts":   true,
		".scala": true,
		".ml":    true,
		".mli":   true,
		".ex":    true,
		".exs":   true,
		".erl":   true,
		".hrl":   true,
		".lua":   true,
		".sh":    true,
		".bash":  true,
		".zsh":   true,
		".ps1":   true,
		".bat":   true,
		".cmake": true,
		".yaml":  true,
		".yml":   true,
		".toml":  true,
		".json":  true,
		".xml":   true,
		".html":  true,
		".css":   true,
		".scss":  true,
		".less":  true,
		".md":    true,
		".txt":   true,
		".rst":   true,
	}

	// Determine which files to search
	var targetFiles []string
	for _, pattern := range searchPatterns {
		matches, err := te.globRecursive(pattern, 5000)
		if err != nil {
			continue
		}
		for _, match := range matches {
			ext := strings.ToLower(filepath.Ext(match))
			if codeExtensions[ext] {
				// Exclude the renamed file itself (it already has the new name)
				if filepath.Clean(match) != filepath.Clean(cleanDest) {
					targetFiles = append(targetFiles, match)
				}
			}
		}
	}

	if len(targetFiles) == 0 {
		return updated
	}

	// Build regex patterns to match various import/reference styles
	// These patterns look for the old filename in context where it's likely an import/reference
	patterns := []string{
		// Go imports: "old_file.go" or old_file.go
		`"` + regexp.QuoteMeta(oldBase) + `"`,
		// Single-quoted strings: 'old_file.go'
		`'` + regexp.QuoteMeta(oldBase) + `'`,
		// Python imports: from old_file import or import old_file
		`from\s+` + regexp.QuoteMeta(oldBase) + `\s+import`,
		`import\s+` + regexp.QuoteMeta(oldBase),
		// JavaScript/TypeScript: require('old_file') or import from 'old_file'
		`require\s*\(\s*['"]` + regexp.QuoteMeta(oldBase) + `['"]\s*\)`,
		`from\s+['"]` + regexp.QuoteMeta(oldBase) + `['"]`,
		`import\s+.*\s+from\s+['"]` + regexp.QuoteMeta(oldBase) + `['"]`,
		// Shell: source old_file.sh or . old_file.sh
		`\b(source|\. )\s+` + regexp.QuoteMeta(oldBase),
		// Markdown: [text](old_file) or ![alt](old_file)
		`\[` + regexp.QuoteMeta(oldBase) + `\]`,
		// HTML: src="old_file" or href="old_file"
		`(?:src|href)\s*=\s*["']` + regexp.QuoteMeta(oldBase) + `["']`,
		// C/C++: #include "old_file" or #include <old_file>
		`#\s*include\s+["<]` + regexp.QuoteMeta(oldBase) + `[">]`,
		// XML/HTML: <file>old_file</file>
		`<file>` + regexp.QuoteMeta(oldBase) + `</file>`,
	}

	// Deduplicate patterns
	seen := make(map[string]bool)
	var uniquePatterns []string
	for _, p := range patterns {
		if !seen[p] {
			seen[p] = true
			uniquePatterns = append(uniquePatterns, p)
		}
	}

	// Search and replace in each file
	for _, filePath := range targetFiles {
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		originalContent := string(content)
		modified := false

		// Check if file contains the old filename at all (quick check)
		if !strings.Contains(originalContent, oldBase) {
			continue
		}

		// Use simple string replacement (more reliable than regex for this use case)
		// We want to replace the old filename where it appears as a reference, not just anywhere
		// Count occurrences
		occurrences := strings.Count(originalContent, oldBase)

		if occurrences > 0 {
			// Replace all occurrences of the old base with the new base
			newContent := strings.ReplaceAll(originalContent, oldBase, newBase)
			if newContent != originalContent {
				// Write back the modified content
				if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
					continue
				}
				updated = append(updated, fmt.Sprintf("%s (%d reference(s))", filePath, occurrences))
				modified = true
			}
		}

		_ = modified
		_ = uniquePatterns
	}

	return updated
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

// executeGitStash manages git stashes with list, save, pop, apply, and drop actions.
func (te *ToolExecutor) executeGitStash(params map[string]interface{}) *ToolResult {
	action, hasAction := params["action"]
	if !hasAction {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: action (list, save, pop, apply, drop)",
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
		return te.gitStashList(params)
	case "save":
		return te.gitStashSave(params)
	case "pop":
		return te.gitStashPop(params)
	case "apply":
		return te.gitStashApply(params)
	case "drop":
		return te.gitStashDrop(params)
	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s. Valid actions: list, save, pop, apply, drop", actionStr),
		}
	}
}

// gitStashList lists all stashes with their messages and dates.
func (te *ToolExecutor) gitStashList(params map[string]interface{}) *ToolResult {
	// Use --format for structured output: index:message:date
	args := []string{"stash", "list", "--format=%(refname:short)%09%(subject)%09%(authordate:short)"}
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git stash list failed: %s", string(output)),
		}
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		return &ToolResult{
			Success: true,
			Output:  "No stashes found.",
			Extra: map[string]interface{}{
				"tool":          "git_stash",
				"action":        "list",
				"stashCount":    0,
				"message":       "No stashes found",
			},
		}
	}

	// Count stashes
	lines := strings.Split(outputStr, "\n")
	stashCount := 0
	var formattedStashes strings.Builder
	for _, line := range lines {
		if line == "" {
			continue
		}
		stashCount++
		formattedStashes.WriteString(line + "\n")
	}

	result := &ToolResult{
		Success: true,
		Output:  formattedStashes.String(),
		Extra: map[string]interface{}{
			"tool":        "git_stash",
			"action":      "list",
			"stashCount":  stashCount,
			"message":     fmt.Sprintf("Found %d stash(es)", stashCount),
		},
	}

	return result
}

// gitStashSave creates a new stash.
func (te *ToolExecutor) gitStashSave(params map[string]interface{}) *ToolResult {
	message, hasMessage := params["message"]
	includeUntracked := false
	if u, ok := params["include_untracked"].(bool); ok && u {
		includeUntracked = true
	}
	includeIgnored := false
	if i, ok := params["include_ignored"].(bool); ok && i {
		includeIgnored = true
	}

	// Build stash args
	args := []string{"stash", "push"}

	// Add message
	if hasMessage {
		msgStr, ok := message.(string)
		if !ok || msgStr == "" {
			return &ToolResult{
				Success: false,
				Error:   "message must be a non-empty string",
			}
		}
		args = append(args, "-m", msgStr)
	}

	// Add flags for untracked and ignored files
	if includeUntracked && includeIgnored {
		args = append(args, "-u") // -u includes both untracked and ignored
	} else if includeUntracked {
		args = append(args, "-u") // -u includes untracked files
	} else if includeIgnored {
		args = append(args, "--include-untracked") // --include-untracked includes ignored files too
	}

	// Optionally specify a pathspec to stash only specific files
	if pathspec, ok := params["pathspec"].(string); ok && pathspec != "" {
		args = append(args, "--", pathspec)
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git stash push failed: %s", string(output)),
		}
	}

	// Extract stash reference (e.g., "stash@{0}")
	var stashRef string
	refRe := regexp.MustCompile(`(stash@\{\d+\})`)
	if matches := refRe.FindStringSubmatch(string(output)); len(matches) > 1 {
		stashRef = matches[1]
	}

	var msg string
	if hasMessage {
		msgStr, _ := message.(string)
		msg = fmt.Sprintf("Stashed with message: %s", msgStr)
	} else {
		msg = "Changes stashed successfully"
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Created stash: %s\n%s", stashRef, msg),
		Extra: map[string]interface{}{
			"tool":           "git_stash",
			"action":         "save",
			"stashRef":       stashRef,
			"includeUntracked": includeUntracked,
			"includeIgnored":   includeIgnored,
			"hasMessage":       hasMessage,
			"message":          msg,
		},
	}
}

// gitStashPop applies and removes the most recent stash.
func (te *ToolExecutor) gitStashPop(params map[string]interface{}) *ToolResult {
	// Default to most recent stash (stash@{0})
	stashRef, _ := params["stash_ref"].(string)
	if stashRef == "" {
		stashRef = "stash@{0}"
	}

	args := []string{"stash", "pop", stashRef}
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if the stash doesn't exist
		if strings.Contains(string(output), "did not match") {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("stash '%s' does not exist. Use git_stash action='list' to see available stashes.", stashRef),
			}
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git stash pop failed: %s", string(output)),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  string(output),
		Extra: map[string]interface{}{
			"tool":     "git_stash",
			"action":   "pop",
			"stashRef": stashRef,
			"message":  fmt.Sprintf("Popped stash %s", stashRef),
		},
	}
}

// gitStashApply applies a stash without removing it.
func (te *ToolExecutor) gitStashApply(params map[string]interface{}) *ToolResult {
	stashRef, hasRef := params["stash_ref"].(string)
	if !hasRef || stashRef == "" {
		// Default to most recent stash
		stashRef = "stash@{0}"
	}

	args := []string{"stash", "apply", stashRef}
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		if strings.Contains(string(output), "did not match") {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("stash '%s' does not exist. Use git_stash action='list' to see available stashes.", stashRef),
			}
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git stash apply failed: %s", string(output)),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  string(output),
		Extra: map[string]interface{}{
			"tool":     "git_stash",
			"action":   "apply",
			"stashRef": stashRef,
			"message":  fmt.Sprintf("Applied stash %s (stash preserved)", stashRef),
		},
	}
}

// gitStashDrop removes a specific stash.
func (te *ToolExecutor) gitStashDrop(params map[string]interface{}) *ToolResult {
	stashRef, hasRef := params["stash_ref"].(string)
	if !hasRef || stashRef == "" {
		// Default to most recent stash
		stashRef = "stash@{0}"
	}

	args := []string{"stash", "drop", stashRef}
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		if strings.Contains(string(output), "did not match") {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("stash '%s' does not exist. Use git_stash action='list' to see available stashes.", stashRef),
			}
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git stash drop failed: %s", string(output)),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  string(output),
		Extra: map[string]interface{}{
			"tool":     "git_stash",
			"action":   "drop",
			"stashRef": stashRef,
			"message":  fmt.Sprintf("Dropped stash %s", stashRef),
		},
	}
}

// jsonPathSegment represents a segment in a JSON path.
type jsonPathSegment struct {
	Key     string
	Index   int
	HasIndex bool
}

// parseJSONPath parses a JSON path string into segments.
// Supports dot notation like ".foo.bar" and bracket notation like "foo.bar[0]".
func parseJSONPath(path string) ([]jsonPathSegment, error) {
	if path == "" || path == "." {
		return nil, nil
	}

	// Remove leading dot
	path = strings.TrimPrefix(path, ".")

	var segments []jsonPathSegment
	remaining := path

	for remaining != "" {
		// Find the next dot or bracket
		nextDot := strings.Index(remaining, ".")
		nextBracket := strings.Index(remaining, "[")

		var key string
		var remainder string

		if nextBracket >= 0 && (nextDot == -1 || nextBracket < nextDot) {
			// Bracket comes first
			key = remaining[:nextBracket]
			// Find closing bracket
			closeBracket := strings.Index(remaining, "]")
			if closeBracket == -1 {
				return nil, fmt.Errorf("unmatched bracket in path")
			}
			indexStr := remaining[nextBracket+1 : closeBracket]
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return nil, fmt.Errorf("invalid array index in path: %s", indexStr)
			}
			segments = append(segments, jsonPathSegment{
				Key:      key,
				Index:    index,
				HasIndex: true,
			})
			remainder = remaining[closeBracket+1:]
		} else if nextDot >= 0 {
			key = remaining[:nextDot]
			remainder = remaining[nextDot+1:]
			segments = append(segments, jsonPathSegment{
				Key:  key,
				HasIndex: false,
			})
		} else {
			// Rest is just a key
			key = remaining
			segments = append(segments, jsonPathSegment{
				Key:      key,
				HasIndex: false,
			})
			remainder = ""
		}
		remaining = remainder
	}

	return segments, nil
}

// resolvePath resolves a JSON path on a JSON value and returns the value at that path.
func resolvePath(value interface{}, segments []jsonPathSegment) (interface{}, error) {
	if len(segments) == 0 {
		return value, nil
	}

	current := value
	for i, seg := range segments {
		switch v := current.(type) {
		case map[string]interface{}:
			key := seg.Key
			if seg.HasIndex {
				// Array index on a map key - this shouldn't happen in normal paths
				// but handle it: first get the map value, then index into it
				if child, ok := v[key]; ok {
					if arr, ok := child.([]interface{}); ok {
						current = resolveArrayIndex(arr, seg.Index)
					} else {
						current = nil
					}
					if current == nil {
						return nil, fmt.Errorf("path not found at segment %d: %s", i, key)
					}
				} else {
					return nil, fmt.Errorf("path not found: key '%s' does not exist", key)
				}
			} else {
				if child, ok := v[key]; ok {
					current = child
				} else {
					return nil, fmt.Errorf("path not found: key '%s' does not exist", key)
				}
			}
		case []interface{}:
			if seg.HasIndex {
				current = resolveArrayIndex(v, seg.Index)
				if current == nil {
					return nil, fmt.Errorf("path not found: array index %d out of bounds", seg.Index)
				}
			} else {
				// Array key is unusual - treat as index 0
				if idx, err := strconv.Atoi(seg.Key); err == nil {
					current = resolveArrayIndex(v, idx)
					if current == nil {
						return nil, fmt.Errorf("path not found: array index %d out of bounds", idx)
					}
				} else {
					return nil, fmt.Errorf("invalid array access: key '%s' is not a valid index", seg.Key)
				}
			}
		default:
			return nil, fmt.Errorf("cannot access field '%s' on %T", seg.Key, current)
		}
	}
	return current, nil
}

// resolveArrayIndex safely resolves an index into an array.
func resolveArrayIndex(arr []interface{}, index int) interface{} {
	if index < 0 || index >= len(arr) {
		return nil
	}
	return arr[index]
}

// setValueAtPath sets a value at a specific JSON path.
func setValueAtPath(value interface{}, segments []jsonPathSegment, newValue interface{}) (interface{}, error) {
	if len(segments) == 0 {
		return newValue, nil
	}

	current := value

	// Navigate to the parent of the target
	for i := 0; i < len(segments)-1; i++ {
		seg := segments[i]
		switch v := current.(type) {
		case map[string]interface{}:
			if child, ok := v[seg.Key]; ok {
				current = child
			} else {
				// Create intermediate object
				newChild := make(map[string]interface{})
				v[seg.Key] = newChild
				current = newChild
			}
		case []interface{}:
			idx := seg.Index
			if idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("array index %d out of bounds", idx)
			}
			current = v[idx]
		default:
			return nil, fmt.Errorf("cannot navigate into %T", current)
		}
	}

	// Set the final segment
	lastSeg := segments[len(segments)-1]
	switch v := current.(type) {
	case map[string]interface{}:
		v[lastSeg.Key] = newValue
	case []interface{}:
		idx := lastSeg.Index
		if idx < 0 || idx >= len(v) {
			return nil, fmt.Errorf("array index %d out of bounds", idx)
		}
		v[idx] = newValue
	default:
		return nil, fmt.Errorf("cannot set value on %T", current)
	}

	return value, nil
}

// parseValueFromString parses a value string into a JSON-compatible Go value.
func parseValueFromString(s string) (interface{}, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil, fmt.Errorf("empty value")
	}

	// Handle special values
	if strings.EqualFold(trimmed, "null") || trimmed == "nil" {
		return nil, nil
	}
	if strings.EqualFold(trimmed, "true") {
		return true, nil
	}
	if strings.EqualFold(trimmed, "false") {
		return false, nil
	}

	// Try parsing as JSON first (handles strings, numbers, arrays, objects)
	var result interface{}
	if err := json.Unmarshal([]byte(trimmed), &result); err == nil {
		return result, nil
	}

	// Try as a plain string
	return trimmed, nil
}

// convertToYAML converts a Go value to a simple YAML string (no external deps).
func convertToYAML(value interface{}, indent int) string {
	return convertToYAMLRecursive(value, 0, indent)
}

func convertToYAMLRecursive(value interface{}, depth int, indent int) string {
	prefix := strings.Repeat(" ", depth*indent)

	switch v := value.(type) {
	case map[string]interface{}:
		var result strings.Builder
		// Sort keys for deterministic output
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			val := v[k]
			if val == nil {
				result.WriteString(fmt.Sprintf("%s%s: null\n", prefix, k))
			} else if nested, ok := val.(map[string]interface{}); ok {
				result.WriteString(fmt.Sprintf("%s%s:\n", prefix, k))
				result.WriteString(convertToYAMLRecursive(nested, depth+1, indent))
			} else if nestedArr, ok := val.([]interface{}); ok {
				result.WriteString(fmt.Sprintf("%s%s:\n", prefix, k))
				for _, item := range nestedArr {
					if itemMap, ok := item.(map[string]interface{}); ok {
						// Inline map as a YAML mapping on the same line
						result.WriteString(fmt.Sprintf("%s  - ", prefix))
						first := true
						for mk, mv := range itemMap {
							if !first {
								result.WriteString(", ")
							}
							result.WriteString(fmt.Sprintf("%s: %s", mk, yamlValueString(mv)))
							first = false
						}
						result.WriteString("\n")
					} else {
						result.WriteString(fmt.Sprintf("%s  - %s\n", prefix, yamlValueString(item)))
					}
				}
			} else {
				result.WriteString(fmt.Sprintf("%s%s: %s\n", prefix, k, yamlValueString(val)))
			}
		}
		return result.String()

	case []interface{}:
		var result strings.Builder
		for _, item := range v {
			if itemMap, ok := item.(map[string]interface{}); ok {
				result.WriteString(fmt.Sprintf("%s- ", prefix))
				first := true
				for mk, mv := range itemMap {
					if !first {
						result.WriteString(", ")
					}
					result.WriteString(fmt.Sprintf("%s: %s", mk, yamlValueString(mv)))
					first = false
				}
				result.WriteString("\n")
			} else {
				result.WriteString(fmt.Sprintf("%s- %s\n", prefix, yamlValueString(item)))
			}
		}
		return result.String()

	default:
		return fmt.Sprintf("%s%s\n", prefix, yamlValueString(v))
	}
}

// yamlValueString converts a value to its YAML representation.
func yamlValueString(value interface{}) string {
	switch v := value.(type) {
	case string:
		// Quote strings that could be misinterpreted
		if v == "" || v == "true" || v == "false" || v == "null" || v == "yes" || v == "no" ||
			v == "on" || v == "off" || v == "~" || v == "{}" || v == "[]" ||
			strings.Contains(v, ":") || strings.Contains(v, "#") || strings.Contains(v, "\n") ||
			strings.HasPrefix(v, " ") || strings.HasSuffix(v, " ") {
			return "'" + strings.ReplaceAll(v, "'", "''") + "'"
		}
		// Check if it looks like a number
		if _, err := strconv.ParseFloat(v, 64); err == nil {
			return "'" + v + "'"
		}
		return v
	case float64:
		// Check if it's an integer value
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// convertToEnv converts a JSON object to environment variable format.
func convertToEnv(value interface{}) string {
	result, ok := value.(map[string]interface{})
	if !ok {
		return ""
	}

	var lines []string
	for key, val := range result {
		envKey := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
		envKey = strings.ReplaceAll(envKey, "-", "_")
		envKey = regexp.MustCompile(`[^A-Z0-9_]`).ReplaceAllString(envKey, "_")

		var envVal string
		switch v := val.(type) {
		case string:
			envVal = v
		case float64:
			if v == float64(int64(v)) {
				envVal = fmt.Sprintf("%d", int64(v))
			} else {
				envVal = fmt.Sprintf("%v", v)
			}
		case bool:
			envVal = fmt.Sprintf("%t", v)
		case nil:
			envVal = ""
		case map[string]interface{}:
			// Flatten nested objects
			flattened := flattenJSON(v, envKey)
			lines = append(lines, flattened...)
			continue
		case []interface{}:
			// Convert array to comma-separated
			var items []string
			for _, item := range v {
				items = append(items, fmt.Sprintf("%v", item))
			}
			envVal = strings.Join(items, ",")
		default:
			envVal = fmt.Sprintf("%v", v)
		}
		lines = append(lines, fmt.Sprintf("%s=%s", envKey, envVal))
	}

	sort.Strings(lines)
	return strings.Join(lines, "\n") + "\n"
}

// flattenJSON flattens a nested JSON object into key-value pairs with a given prefix.
func flattenJSON(obj map[string]interface{}, prefix string) []string {
	var lines []string
	for key, val := range obj {
		envKey := strings.ToUpper(strings.Join([]string{prefix, key}, "_"))
		envKey = strings.ReplaceAll(envKey, "-", "_")
		envKey = regexp.MustCompile(`[^A-Z0-9_]`).ReplaceAllString(envKey, "_")

		switch v := val.(type) {
		case map[string]interface{}:
			lines = append(lines, flattenJSON(v, envKey)...)
		case []interface{}:
			var items []string
			for _, item := range v {
				items = append(items, fmt.Sprintf("%v", item))
			}
			lines = append(lines, fmt.Sprintf("%s=%s", envKey, strings.Join(items, ",")))
		case nil:
			lines = append(lines, fmt.Sprintf("%s=", envKey))
		case float64:
			if v == float64(int64(v)) {
				lines = append(lines, fmt.Sprintf("%s=%d", envKey, int64(v)))
			} else {
				lines = append(lines, fmt.Sprintf("%s=%v", envKey, v))
			}
		default:
			lines = append(lines, fmt.Sprintf("%s=%v", envKey, v))
		}
	}
	return lines
}

// validateJSON checks if the required fields exist in the JSON data.
type validationResult struct {
	Valid      bool     `json:"valid"`
	Errors     []string `json:"errors"`
	Found      []string `json:"found"`
	TotalField int      `json:"total_fields"`
}

// executeJsonTransformer handles JSON transformation operations.
func (te *ToolExecutor) executeJsonTransformer(params map[string]interface{}) *ToolResult {
	command, ok := params["command"].(string)
	if !ok || command == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: command",
		}
	}

	command = strings.ToLower(strings.TrimSpace(command))

	// Load JSON data from file_path or json_string
	var jsonData interface{}
	var sourceDesc string

	filePath, hasFile := params["file_path"]
	jsonStr, hasJsonStr := params["json_string"]

	if hasFile && hasJsonStr {
		return &ToolResult{
			Success: false,
			Error:   "cannot specify both file_path and json_string",
		}
	}

	if hasFile {
		fPath, ok := filePath.(string)
		if !ok || fPath == "" {
			return &ToolResult{
				Success: false,
				Error:   "file_path must be a string",
			}
		}
		content, err := os.ReadFile(fPath)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to read file: %v", err),
			}
		}
		if err := json.Unmarshal(content, &jsonData); err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to parse JSON: %v", err),
			}
		}
		sourceDesc = fmt.Sprintf("file: %s", fPath)
	} else if hasJsonStr {
		jStr, ok := jsonStr.(string)
		if !ok || jStr == "" {
			return &ToolResult{
				Success: false,
				Error:   "json_string must be a non-empty string",
			}
		}
		if err := json.Unmarshal([]byte(jStr), &jsonData); err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to parse JSON: %v", err),
			}
		}
		sourceDesc = "json_string input"
	} else {
		return &ToolResult{
			Success: false,
			Error:   "must provide either file_path or json_string",
		}
	}

	var result *ToolResult

	switch command {
	case "extract":
		result = te.jsonExtract(jsonData, params)
	case "set":
		result = te.jsonSet(jsonData, params)
	case "merge":
		result = te.jsonMerge(jsonData, params)
	case "validate":
		result = te.jsonValidate(jsonData, params)
	case "format":
		result = te.jsonFormat(jsonData, params, sourceDesc)
	case "convert_to_yaml":
		result = te.jsonConvertToYAML(jsonData)
	case "convert_to_env":
		result = te.jsonConvertToEnv(jsonData)
	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown command: %s. Valid commands: extract, set, merge, validate, format, convert_to_yaml, convert_to_env", command),
		}
	}

	return result
}

// jsonExtract extracts a value at a given path from JSON.
func (te *ToolExecutor) jsonExtract(jsonData interface{}, params map[string]interface{}) *ToolResult {
	path, ok := params["path"].(string)
	if !ok || path == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path for 'extract' command",
		}
	}

	segments, err := parseJSONPath(path)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid path: %v", err),
		}
	}

	value, err := resolvePath(jsonData, segments)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("path resolution failed: %v", err),
		}
	}

	// Format the extracted value
	var output string
	switch v := value.(type) {
	case string:
		output = v
	case nil:
		output = "null"
	default:
		// Pretty-print the value
		pretty, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			output = fmt.Sprintf("%v", v)
		} else {
			output = string(pretty)
		}
	}

	return &ToolResult{
		Success: true,
		Output:  output,
		Extra: map[string]interface{}{
			"tool":   "json_transformer",
			"action": "extract",
			"path":   path,
		},
	}
}

// jsonSet sets a value at a given path in JSON.
func (te *ToolExecutor) jsonSet(jsonData interface{}, params map[string]interface{}) *ToolResult {
	path, ok := params["path"].(string)
	if !ok || path == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path for 'set' command",
		}
	}

	valueParam, hasValue := params["value"]
	if !hasValue {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: value for 'set' command",
		}
	}

	// Parse the value
	newValue, err := parseValueFromString(fmt.Sprintf("%v", valueParam))
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to parse value: %v", err),
		}
	}

	segments, err := parseJSONPath(path)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid path: %v", err),
		}
	}

	result, err := setValueAtPath(jsonData, segments, newValue)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to set value: %v", err),
		}
	}

	// Output the updated JSON
	pretty, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to format result: %v", err),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  string(pretty),
		Extra: map[string]interface{}{
			"tool":   "json_transformer",
			"action": "set",
			"path":   path,
		},
	}
}

// jsonMerge merges multiple JSON sources.
func (te *ToolExecutor) jsonMerge(baseJSON interface{}, params map[string]interface{}) *ToolResult {
	// Start with the base JSON
	merged, ok := baseJSON.(map[string]interface{})
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "base JSON must be an object for 'merge' command",
		}
	}

	// Collect all JSON data to merge
	var jsonValues []interface{}

	// Additional file paths
	if filesParam, hasFiles := params["files"]; hasFiles {
		if files, ok := filesParam.([]interface{}); ok {
			for _, f := range files {
				fPath, ok := f.(string)
				if !ok {
					continue
				}
				content, err := os.ReadFile(fPath)
				if err != nil {
					continue
				}
				var data interface{}
				if err := json.Unmarshal(content, &data); err == nil {
					jsonValues = append(jsonValues, data)
				}
			}
		}
	}

	// Additional JSON strings
	if jsonStrsParam, hasStrs := params["json_strings"]; hasStrs {
		if jsonStrs, ok := jsonStrsParam.([]interface{}); ok {
			for _, js := range jsonStrs {
				jsStr, ok := js.(string)
				if !ok {
					continue
				}
				var data interface{}
				if err := json.Unmarshal([]byte(jsStr), &data); err == nil {
					jsonValues = append(jsonValues, data)
				}
			}
		}
	}

	// Deep merge each JSON value into merged
	for _, data := range jsonValues {
		if obj, ok := data.(map[string]interface{}); ok {
			deepMerge(merged, obj)
		}
	}

	pretty, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to format merged result: %v", err),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  string(pretty),
		Extra: map[string]interface{}{
			"tool":   "json_transformer",
			"action": "merge",
			"filesMerged": len(jsonValues) + 1,
		},
	}
}

// deepMerge merges source into dest recursively.
func deepMerge(dest, src map[string]interface{}) {
	for k, v := range src {
		if destVal, exists := dest[k]; exists {
			if destMap, ok := destVal.(map[string]interface{}); ok {
				if srcMap, ok := v.(map[string]interface{}); ok {
					deepMerge(destMap, srcMap)
					continue
				}
			}
		}
		dest[k] = v
	}
}

// jsonValidate validates JSON against required fields.
func (te *ToolExecutor) jsonValidate(jsonData interface{}, params map[string]interface{}) *ToolResult {
	requiredFields, hasFields := params["required_fields"]
	if !hasFields {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: required_fields for 'validate' command",
		}
	}

	var requiredPaths []string
	switch v := requiredFields.(type) {
	case []interface{}:
		for _, f := range v {
			if fStr, ok := f.(string); ok {
				requiredPaths = append(requiredPaths, fStr)
			}
		}
	case string:
		requiredPaths = []string{v}
	}

	if len(requiredPaths) == 0 {
		return &ToolResult{
			Success: false,
			Error:   "required_fields must be a non-empty list",
		}
	}

	var errors []string
	var found []string

	for _, fieldPath := range requiredPaths {
		segments, err := parseJSONPath(fieldPath)
		if err != nil {
			errors = append(errors, fmt.Sprintf("invalid path '%s': %v", fieldPath, err))
			continue
		}
		if _, err := resolvePath(jsonData, segments); err != nil {
			errors = append(errors, fmt.Sprintf("missing required field: %s", fieldPath))
		} else {
			found = append(found, fieldPath)
		}
	}

	valid := len(errors) == 0
	result := validationResult{
		Valid:      valid,
		Errors:     errors,
		Found:      found,
		TotalField: len(requiredPaths),
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to format result: %v", err),
		}
	}

	status := "PASSED"
	if !valid {
		status = "FAILED"
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Validation %s: %d/%d fields valid\n%s", status, len(found), len(requiredPaths), string(output)),
		Extra: map[string]interface{}{
			"tool":       "json_transformer",
			"action":     "validate",
			"validation": result,
		},
	}
}

// jsonFormat formats/beautifies JSON.
func (te *ToolExecutor) jsonFormat(jsonData interface{}, params map[string]interface{}, sourceDesc string) *ToolResult {
	indent := 2
	if indentParam, ok := params["indent"]; ok {
		switch v := indentParam.(type) {
		case float64:
			indent = int(v)
		case int:
			indent = v
		case string:
			if n, err := strconv.Atoi(v); err == nil {
				indent = n
			}
		}
	}

	pretty, err := json.MarshalIndent(jsonData, "", strings.Repeat(" ", indent))
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to format JSON: %v", err),
		}
	}

	output := string(pretty) + "\n"

	return &ToolResult{
		Success: true,
		Output:  output,
		Extra: map[string]interface{}{
			"tool":      "json_transformer",
			"action":    "format",
			"indent":    indent,
			"source":    sourceDesc,
			"charCount": len(output),
		},
	}
}

// jsonConvertToYAML converts JSON to YAML format.
func (te *ToolExecutor) jsonConvertToYAML(jsonData interface{}) *ToolResult {
	yaml := convertToYAML(jsonData, 2)
	return &ToolResult{
		Success: true,
		Output:  yaml,
		Extra: map[string]interface{}{
			"tool":      "json_transformer",
			"action":    "convert_to_yaml",
			"charCount": len(yaml),
		},
	}
}

// jsonConvertToEnv converts JSON to environment variable format.
func (te *ToolExecutor) jsonConvertToEnv(jsonData interface{}) *ToolResult {
	if _, ok := jsonData.(map[string]interface{}); !ok {
		return &ToolResult{
			Success: false,
			Error:   "convert_to_env requires a JSON object at the top level",
		}
	}

	env := convertToEnv(jsonData)
	return &ToolResult{
		Success: true,
		Output:  env,
		Extra: map[string]interface{}{
			"tool":      "json_transformer",
			"action":    "convert_to_env",
			"charCount": len(env),
		},
	}
}

// projectDiagnosticsResult represents the complete diagnostic report.
type projectDiagnosticsResult struct {
	Tool          string        `json:"tool"`
	Summary       diagSummary   `json:"summary"`
	Issues        []diagIssue   `json:"issues"`
	FilesScanned  int           `json:"files_scanned"`
	ScanDuration  string        `json:"scan_duration"`
	PathsSearched []string      `json:"paths_searched"`
	Target        string        `json:"target"`
	Mode          string        `json:"mode"`
}

type diagSummary struct {
	TotalIssues      int             `json:"total_issues"`
	TotalFiles       int             `json:"total_files"`
	IssuesBySeverity map[string]int  `json:"issues_by_severity"`
	IssuesByCategory map[string]int  `json:"issues_by_category"`
	FilesChecked     int             `json:"files_checked"`
	ChecksRun        []string        `json:"checks_run"`
}

type diagIssue struct {
	Severity   string `json:"severity"`
	Category   string `json:"category"`
	FilePath   string `json:"file_path"`
	Line       int    `json:"line,omitempty"`
	Message    string `json:"message"`
	Content    string `json:"content,omitempty"`
	Recommendation string `json:"recommendation,omitempty"`
}

// executeProjectDiagnostics scans the codebase for common issues and quality problems.
func (te *ToolExecutor) executeProjectDiagnostics(params map[string]interface{}) *ToolResult {
	startTime := time.Now()

	// Determine scope: paths parameter or current directory
	pathsParam, hasPaths := params["paths"]
	var targetPaths []string
	if hasPaths {
		switch v := pathsParam.(type) {
		case []interface{}:
			for _, p := range v {
				targetPaths = append(targetPaths, fmt.Sprintf("%v", p))
			}
		case string:
			if strings.Contains(v, ",") {
				for _, p := range strings.Split(v, ",") {
					targetPaths = append(targetPaths, strings.TrimSpace(p))
				}
			} else {
				targetPaths = []string{v}
			}
		}
	}

	if len(targetPaths) == 0 {
		targetPaths = []string{"."}
	}

	// Determine scan depth
	maxDepth := 10
	if md, ok := params["max_depth"].(float64); ok {
		maxDepth = int(md)
	} else if md, ok := params["max_depth"].(int); ok {
		maxDepth = md
	} else if md, ok := params["max_depth"].(string); ok {
		if n, err := strconv.Atoi(md); err == nil {
			maxDepth = n
		}
	}

	// Determine target description for reporting
	targetDesc := "current directory"
	if len(targetPaths) == 1 && targetPaths[0] != "." {
		targetDesc = targetPaths[0]
	} else if len(targetPaths) > 1 {
		targetDesc = fmt.Sprintf("%d paths", len(targetPaths))
	}

	// Scan mode
	scanMode := "full"
	if sm, ok := params["mode"].(string); ok {
		scanMode = sm
	}

	// Collect files to scan
	var allFiles []string
	for _, tp := range targetPaths {
		matches, err := te.globRecursive(tp, 5000)
		if err != nil {
			continue
		}
		// Limit depth based on maxDepth
		for _, f := range matches {
			rel, err := filepath.Rel(targetPaths[0], f)
			if err == nil {
				depth := strings.Count(rel, string(filepath.Separator))
				if depth <= maxDepth {
					allFiles = append(allFiles, f)
				}
			} else {
				allFiles = append(allFiles, f)
			}
		}
	}

	// Filter out binary files and vendor directories
	var codeFiles []string
	for _, f := range allFiles {
		if shouldIncludeFile(f) {
			codeFiles = append(codeFiles, f)
		}
	}

	// Run diagnostic checks
	var issues []diagIssue
	filesScanned := 0

	// Always run these checks
	issues = append(issues, te.checkTODOs(codeFiles)...)
	issues = append(issues, te.checkEmptyFiles(codeFiles)...)
	issues = append(issues, te.checkLargeFiles(codeFiles)...)
	issues = append(issues, te.checkHardcodedSecrets(codeFiles)...)

	filesScanned = len(codeFiles)

	// Run mode-specific checks
	switch scanMode {
	case "basic":
		// Only TODOs and empty files
	case "full":
		issues = append(issues, te.checkLargeFiles(codeFiles)...)
		issues = append(issues, te.checkHardcodedSecrets(codeFiles)...)
	case "quick":
		// Just TODOs
	default:
		// Unknown mode, run all
	}

	// Deduplicate issues (some checks might overlap)
	issues = deduplicateIssues(issues)

	// Build summary
	issuesBySeverity := map[string]int{"low": 0, "medium": 0, "high": 0, "critical": 0}
	issuesByCategory := map[string]int{}
	for _, issue := range issues {
		issuesBySeverity[issue.Severity]++
		issuesByCategory[issue.Category]++
	}

	checksRun := []string{"TODOs and markers", "empty files", "large files", "hardcoded secrets"}

	elapsed := time.Since(startTime)

	result := projectDiagnosticsResult{
		Tool:     "project_diagnostics",
		Summary: diagSummary{
			TotalIssues:      len(issues),
			TotalFiles:       len(allFiles),
			IssuesBySeverity: issuesBySeverity,
			IssuesByCategory: issuesByCategory,
			FilesChecked:     filesScanned,
			ChecksRun:        checksRun,
		},
		Issues:        issues,
		FilesScanned:  filesScanned,
		ScanDuration:  elapsed.String(),
		PathsSearched: targetPaths,
		Target:        targetDesc,
		Mode:          scanMode,
	}

	// Format output
	output := formatDiagnosticsResult(result)

	return &ToolResult{
		Success: true,
		Output:  output,
		Extra: map[string]interface{}{
			"tool":           "project_diagnostics",
			"issues_found":   len(issues),
			"files_scanned":  filesScanned,
			"mode":           scanMode,
			"target":         targetDesc,
			"duration":       elapsed.String(),
			"issues_by_severity": issuesBySeverity,
			"issues_by_category": issuesByCategory,
		},
	}
}

// shouldIncludeFile checks if a file should be included in diagnostics.
func shouldIncludeFile(path string) bool {
	// Skip binary files, version control, and build artifacts
	skipDirs := map[string]bool{
		".git": true, ".svn": true, ".hg": true,
		"vendor": true, "node_modules": true, "dist": true, "build": true,
		".venv": true, "__pycache__": true, "venv": true,
	}

	// Check if path contains any skipped directory
	rel, err := filepath.Rel(".", path)
	if err != nil {
		rel = path
	}

	parts := strings.Split(filepath.Dir(rel), string(filepath.Separator))
	for _, part := range parts {
		if skipDirs[part] {
			return false
		}
	}

	// Skip binary file extensions
	binaryExts := map[string]bool{
		".bin": true, ".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".o": true, ".a": true, ".pyc": true, ".pyo": true,
		".class": true, ".jar": true, ".war": true,
		".zip": true, ".tar": true, ".gz": true, ".bz2": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
		".ico": true, ".svg": true, ".pdf": true, ".doc": true,
		".docx": true, ".xls": true, ".xlsx": true,
	}
	_ = binaryExts
	ext := filepath.Ext(path)
	if binaryExts[strings.ToLower(ext)] {
		return false
	}

	return true
}

// checkTODOs finds TODO, FIXME, HACK, WARN, and XXX markers in source files.
func (te *ToolExecutor) checkTODOs(files []string) []diagIssue {
	// Patterns for markers: TODO, FIXME, HACK, WARN, XXX, NOTE, DEPRECATED, TEMP
	patterns := []struct {
		regex     *regexp.Regexp
		severity  string
		category  string
		message   string
		recommend string
	}{
		{regexp.MustCompile(`(?i)\bTODO\b.*:?\s*(.*)`), "low", "todo", "TODO marker found", "Complete or remove this TODO"},
		{regexp.MustCompile(`(?i)\bFIXME\b.*:?\s*(.*)`), "medium", "todo", "FIXME marker found", "Resolve this issue"},
		{regexp.MustCompile(`(?i)\bHACK\b.*:?\s*(.*)`), "medium", "todo", "HACK marker found", "Replace with proper implementation"},
		{regexp.MustCompile(`(?i)\bWARN\b.*:?\s*(.*)`), "low", "todo", "WARN marker found", "Review and address warning"},
		{regexp.MustCompile(`(?i)\bXXX\b.*:?\s*(.*)`), "high", "todo", "XXX marker found", "Urgent: needs immediate attention"},
		{regexp.MustCompile(`(?i)\bDEPRECATED\b.*:?\s*(.*)`), "medium", "deprecation", "Deprecated code/function found", "Plan migration away from deprecated API"},
		{regexp.MustCompile(`(?i)\bTEMP\b.*:?\s*(.*)`), "low", "todo", "Temporary code found", "Review and make permanent or remove"},
	}

	var issues []diagIssue
	textExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true,
		".java": true, ".rs": true, ".c": true, ".cpp": true, ".h": true,
		".hpp": true, ".cs": true, ".rb": true, ".php": true, ".swift": true,
		".kt": true, ".scala": true, ".sh": true, ".bash": true,
		".md": true, ".html": true, ".css": true, ".scss": true,
	}

	for _, file := range files {
		ext := filepath.Ext(file)
		if !textExts[ext] {
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		for lineNum, line := range lines {
			for _, p := range patterns {
				if p.regex.MatchString(line) {
					match := p.regex.FindStringSubmatch(line)
					contentStr := ""
					recommendStr := p.recommend
					if len(match) > 1 {
						contentStr = strings.TrimSpace(match[1])
						if contentStr == "" {
							recommendStr = p.recommend
						}
					}
					issues = append(issues, diagIssue{
						Severity:       p.severity,
						Category:       p.category,
						FilePath:       file,
						Line:           lineNum + 1,
						Message:        p.message,
						Content:        strings.TrimSpace(line[:min(200, len(line))]),
						Recommendation: recommendStr,
					})
				}
			}
		}
	}

	return issues
}

// checkEmptyFiles finds empty or near-empty files.
func (te *ToolExecutor) checkEmptyFiles(files []string) []diagIssue {
	var issues []diagIssue

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		// Skip if file is completely empty (0 bytes) or very small (< 20 bytes)
		if info.Size() < 20 {
			content, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			// Only flag if truly empty or whitespace-only
			if len(content) == 0 || strings.TrimSpace(string(content)) == "" {
				issues = append(issues, diagIssue{
					Severity:       "low",
					Category:       "empty_file",
					FilePath:       file,
					Message:        "Empty or near-empty file",
					Recommendation: "Remove the file if unused, or add initial content",
				})
			}
		}
	}

	return issues
}

// checkLargeFiles finds files that exceed the line threshold.
func (te *ToolExecutor) checkLargeFiles(files []string) []diagIssue {
	var issues []diagIssue
	const maxLines = 500

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		// Handle trailing newline
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}

		if len(lines) > maxLines {
			issues = append(issues, diagIssue{
				Severity:       "medium",
				Category:       "large_file",
				FilePath:       file,
				Line:           len(lines),
				Message:        fmt.Sprintf("Large file (%d lines, max recommended: %d)", len(lines), maxLines),
				Recommendation: "Consider splitting into smaller, focused files",
			})
		}
	}

	return issues
}

// checkHardcodedSecrets finds potential hardcoded secrets in source files.
func (te *ToolExecutor) checkHardcodedSecrets(files []string) []diagIssue {
	// Patterns that might indicate hardcoded secrets/keys
	patterns := []struct {
		regex       *regexp.Regexp
		severity    string
		category    string
		message     string
		recommend   string
	}{
		{regexp.MustCompile(`(?i)(api[_-]?key|apikey)\s*[:=]\s*["'][a-zA-Z0-9_\-]{20,}["']`), "critical", "security", "Potential hardcoded API key", "Move to environment variable or config file"},
		{regexp.MustCompile(`(?i)(secret|password|passwd|pwd)\s*[:=]\s*["'][^"']{8,}["']`), "critical", "security", "Potential hardcoded secret/password", "Use environment variable or secrets manager"},
		{regexp.MustCompile(`(?i)(token|auth[_-]?token|access[_-]?token)\s*[:=]\s*["'][a-zA-Z0-9_\-]{20,}["']`), "critical", "security", "Potential hardcoded token", "Move to environment variable or config file"},
		{regexp.MustCompile(`(?i)(aws[_-]?secret|aws[_-]?access)\s*[:=]\s*["'][a-zA-Z0-9/+=]{20,}["']`), "critical", "security", "Potential hardcoded AWS credentials", "Use IAM roles or environment variables"},
		{regexp.MustCompile(`(?i)(private[_-]?key)\s*[:=]\s*["'][a-zA-Z0-9_\-]{32,}["']`), "critical", "security", "Potential hardcoded private key", "Never store private keys in source code"},
		{regexp.MustCompile(`(?i)(db[_-]?password|database[_-]?password)\s*[:=]\s*["'][^"']+["']`), "critical", "security", "Potential hardcoded database password", "Use environment variable or secrets manager"},
		{regexp.MustCompile(`(?i)(ghp_[a-zA-Z0-9]{36})`), "critical", "security", "Potential GitHub Personal Access Token", "Revoke and rotate this token immediately"},
		{regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9]{48})`), "critical", "security", "Potential API secret key", "Move to environment variable or secrets manager"},
		{regexp.MustCompile(`(?i)(Bearer\s+[a-zA-Z0-9_\-\.]+)`), "high", "security", "Potential hardcoded bearer token", "Move authentication to environment variable"},
		{regexp.MustCompile(`(?i)http[s]?://[^/\s]+:[^/\s]+@[^/\s]+`), "high", "security", "Potential hardcoded credentials in URL", "Use environment variables for credentials"},
	}

	var issues []diagIssue

	for _, file := range files {
		// Skip non-source files
		ext := filepath.Ext(file)
		textExts := map[string]bool{
			".go": true, ".py": true, ".js": true, ".ts": true, ".java": true,
			".rs": true, ".c": true, ".cpp": true, ".h": true, ".yaml": true,
			".yml": true, ".json": true, ".toml": true, ".env": true,
		}
		if !textExts[strings.ToLower(ext)] {
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		for lineNum, line := range lines {
			for _, p := range patterns {
				if p.regex.MatchString(line) {
					// Skip common false positives
					if isFalsePositiveSecret(line) {
						continue
					}
					issues = append(issues, diagIssue{
						Severity:       p.severity,
						Category:       p.category,
						FilePath:       file,
						Line:           lineNum + 1,
						Message:        p.message,
						Content:        strings.TrimSpace(line[:min(200, len(line))]),
						Recommendation: p.recommend,
					})
				}
			}
		}
	}

	return issues
}

// isFalsePositiveSecret checks if a line is a common false positive.
func isFalsePositiveSecret(line string) bool {
	lower := strings.ToLower(line)

	// Skip placeholder values
	placeholders := []string{"your_", "changeme", "xxx", "TODO", "FIXME", "insert_", "replace_", "example", "placeholder", "<", ">"}
	for _, ph := range placeholders {
		if strings.Contains(lower, ph) {
			return true
		}
	}

	// Skip template/config files that define structure
	if strings.Contains(lower, "environment variable") ||
		strings.Contains(lower, "set this") ||
		strings.Contains(lower, "set this to") ||
		strings.Contains(lower, "environment") && strings.Contains(lower, "variable") {
		return true
	}

	return false
}

// deduplicateIssues removes duplicate diagnostic issues.
func deduplicateIssues(issues []diagIssue) []diagIssue {
	seen := make(map[string]bool)
	var unique []diagIssue

	for _, issue := range issues {
		key := fmt.Sprintf("%s:%s:%d:%s", issue.FilePath, issue.Category, issue.Line, issue.Message)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, issue)
		}
	}

	return unique
}

// formatDiagnosticsResult formats the diagnostic report for display.
func formatDiagnosticsResult(result projectDiagnosticsResult) string {
	var output strings.Builder

	output.WriteString(fmt.Sprintf("=== Project Diagnostics Report ===\n\n"))
	output.WriteString(fmt.Sprintf("Target: %s\n", result.Target))
	output.WriteString(fmt.Sprintf("Mode: %s scan\n", result.Mode))
	output.WriteString(fmt.Sprintf("Files scanned: %d\n", result.FilesScanned))
	output.WriteString(fmt.Sprintf("Scan duration: %s\n\n", result.ScanDuration))

	// Summary
	output.WriteString("--- Summary ---\n")
	output.WriteString(fmt.Sprintf("Total issues found: %d\n", result.Summary.TotalIssues))
	output.WriteString(fmt.Sprintf("Severity breakdown:\n"))

	severityOrder := []string{"critical", "high", "medium", "low"}
	for _, sev := range severityOrder {
		count := result.Summary.IssuesBySeverity[sev]
		if count > 0 {
			output.WriteString(fmt.Sprintf("  %-10s: %d\n", sev, count))
		}
	}

	if len(result.Summary.IssuesByCategory) > 0 {
		output.WriteString("\nCategory breakdown:\n")
		for cat, count := range result.Summary.IssuesByCategory {
			output.WriteString(fmt.Sprintf("  %-20s: %d\n", cat, count))
		}
	}

	output.WriteString(fmt.Sprintf("\nChecks run: %s\n\n", strings.Join(result.Summary.ChecksRun, ", ")))

	// Detailed issues by severity
	if len(result.Issues) > 0 {
		for _, sev := range severityOrder {
			var sevIssues []diagIssue
			for _, issue := range result.Issues {
				if issue.Severity == sev {
					sevIssues = append(sevIssues, issue)
				}
			}

			if len(sevIssues) > 0 {
				output.WriteString(fmt.Sprintf("--- %s ---\n", strings.ToUpper(sev)))

				for _, issue := range sevIssues {
					output.WriteString(fmt.Sprintf("\n  File: %s\n", issue.FilePath))
					if issue.Line > 0 {
						output.WriteString(fmt.Sprintf("  Line: %d\n", issue.Line))
					}
					output.WriteString(fmt.Sprintf("  Type: %s\n", issue.Category))
					output.WriteString(fmt.Sprintf("  Issue: %s\n", issue.Message))
					if issue.Content != "" {
						output.WriteString(fmt.Sprintf("  Content: %s\n", issue.Content))
					}
					if issue.Recommendation != "" {
						output.WriteString(fmt.Sprintf("  Suggestion: %s\n", issue.Recommendation))
					}
					output.WriteString("\n")
				}
			}
		}
	} else {
		output.WriteString("--- No issues found! ---\n")
		output.WriteString("The project looks clean. Keep it up!\n")
	}

	return output.String()
}

// executeRunLint runs linters for the current project and returns structured results.
// Auto-detects project type and runs appropriate linters.
func (te *ToolExecutor) executeRunLint(params map[string]interface{}) *ToolResult {
	startTime := time.Now()

	// Determine lint command
	command, hasCommand := params["command"].(string)
	if !hasCommand || command == "" {
		// Auto-detect project type and linters
		command = te.detectLintCommand()
	}

	if command == "" {
		return &ToolResult{
			Success: false,
			Error:   "no project type detected. Supported project types: Go (go.mod), Python (requirements.txt, pyproject.toml, setup.py), Node.js (package.json), Shell scripts. Provide a custom 'command' parameter to override.",
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

	// Set working directory
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
	}

	// Truncate output if too long
	maxOutputLen := 10000
	if len(outputStr) > maxOutputLen {
		outputStr = outputStr[:maxOutputLen] + "\n... [output truncated, exceeded 10000 character limit]"
	}

	linted := exitCode == 0

	// Generate summary
	summary := te.generateLintSummary(outputStr, linted, command)

	result := &ToolResult{
		Success:  linted,
		ExitCode: exitCode,
		Output:   outputStr,
		Extra: map[string]interface{}{
			"tool":    "run_lint",
			"linted":  linted,
			"command": command,
			"summary": summary,
			"duration": time.Since(startTime).String(),
		},
	}

	return result
}

// detectLintCommand auto-detects the lint command based on project files.
func (te *ToolExecutor) detectLintCommand() string {
	cwd, _ := os.Getwd()

	// Check for Go project
	if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
		// Try go vet first, then gofmt
		cmd := exec.Command("go", "vet", "./...")
		cmd.Dir = cwd
		if _, err := cmd.CombinedOutput(); err == nil {
			return "go vet ./..."
		}
		// Fall back to gofmt
		cmd = exec.Command("gofmt", "-l", ".")
		cmd.Dir = cwd
		if _, err := cmd.CombinedOutput(); err == nil {
			return "gofmt -l ."
		}
		return "go vet ./..."
	}

	// Check for Python project
	if _, err := os.Stat(filepath.Join(cwd, "requirements.txt")); err == nil {
		// Try flake8 first
		cmd := exec.Command("which", "flake8")
		if output, err := cmd.CombinedOutput(); err == nil && strings.TrimSpace(string(output)) != "" {
			return "flake8 ."
		}
		// Fall back to pylint
		cmd = exec.Command("which", "pylint")
		if output, err := cmd.CombinedOutput(); err == nil && strings.TrimSpace(string(output)) != "" {
			return "pylint ."
		}
		return "flake8 ."
	}
	if _, err := os.Stat(filepath.Join(cwd, "pyproject.toml")); err == nil {
		cmd := exec.Command("which", "flake8")
		if output, err := cmd.CombinedOutput(); err == nil && strings.TrimSpace(string(output)) != "" {
			return "flake8 ."
		}
		cmd = exec.Command("which", "pylint")
		if output, err := cmd.CombinedOutput(); err == nil && strings.TrimSpace(string(output)) != "" {
			return "pylint ."
		}
		return "flake8 ."
	}
	if _, err := os.Stat(filepath.Join(cwd, "setup.py")); err == nil {
		cmd := exec.Command("which", "flake8")
		if output, err := cmd.CombinedOutput(); err == nil && strings.TrimSpace(string(output)) != "" {
			return "flake8 ."
		}
		cmd = exec.Command("which", "pylint")
		if output, err := cmd.CombinedOutput(); err == nil && strings.TrimSpace(string(output)) != "" {
			return "pylint ."
		}
		return "flake8 ."
	}

	// Check for Node.js project
	if _, err := os.Stat(filepath.Join(cwd, "package.json")); err == nil {
		// Check if eslint is available
		cmd := exec.Command("which", "eslint")
		if output, err := cmd.CombinedOutput(); err == nil && strings.TrimSpace(string(output)) != "" {
			return "eslint ."
		}
		// Check for npm test script with lint
		content, _ := os.ReadFile(filepath.Join(cwd, "package.json"))
		if bytes.Contains(content, []byte("\"lint\"")) || bytes.Contains(content, []byte("\"lint:fix\"")) {
			return "npm run lint"
		}
		return "eslint ."
	}

	// Check for shell scripts
	hasShell := false
	matches, _ := te.globRecursive("*.sh", 10)
	for _, f := range matches {
		if shouldIncludeFile(f) {
			hasShell = true
			break
		}
	}
	if hasShell {
		cmd := exec.Command("which", "shellcheck")
		if output, err := cmd.CombinedOutput(); err == nil && strings.TrimSpace(string(output)) != "" {
			return "shellcheck *.sh"
		}
	}

	return ""
}

// generateLintSummary creates a human-readable summary of lint results.
func (te *ToolExecutor) generateLintSummary(output string, linted bool, command string) string {
	var summary strings.Builder

	if linted {
		summary.WriteString("Linting passed successfully.")
	} else {
		summary.WriteString("Linting found issues.")
	}

	// Count issues based on output patterns
	issueCount := 0

	// Go vet: "command goes here" or "flag provided but not defined"
	if strings.Contains(command, "go vet") || strings.Contains(command, "gofmt") {
		// Count lines that look like issue reports
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, ":") && !strings.HasPrefix(strings.TrimSpace(line), "-") {
				// Could be a file:line: message or just a file name
				if strings.HasSuffix(line, ".go") || strings.Contains(line, "go: download") {
					continue
				}
				issueCount++
			}
		}
		// gofmt -l outputs file names when files need formatting
		if strings.Contains(command, "gofmt") {
			issueCount = strings.Count(output, "\n")
			if issueCount > 0 && !strings.HasSuffix(output, "\n") {
				issueCount++
			}
			if issueCount == 0 && len(strings.TrimSpace(output)) == 0 {
				return "Linting passed: no formatting issues found."
			}
			summary.WriteString(fmt.Sprintf(" Found %d file(s) needing formatting.", issueCount))
			return summary.String()
		}
	}

	// Python linters
	if strings.Contains(command, "flake8") || strings.Contains(command, "pylint") {
		// flake8: "file.py:line:col: E501 line too long"
		flake8Pattern := regexp.MustCompile(`^[^:]+:\d+:\d+:\s*\w+\d+`)
		for _, line := range strings.Split(output, "\n") {
			if flake8Pattern.MatchString(line) {
				issueCount++
			}
		}
	}

	if strings.Contains(command, "pylint") {
		// pylint: "file.py:line: [msg_type] message"
		pylintPattern := regexp.MustCompile(`^[^:]+:\d+:\s*\[?\w+\]?\s*\d+:\d+`)
		for _, line := range strings.Split(output, "\n") {
			if pylintPattern.MatchString(line) {
				issueCount++
			}
		}
	}

	// ESLint
	if strings.Contains(command, "eslint") {
		eslintPattern := regexp.MustCompile(`^[^:]+:\d+:\d+\s+(warning|error)\s+`)
		for _, line := range strings.Split(output, "\n") {
			if eslintPattern.MatchString(line) {
				issueCount++
			}
		}
		// Also check summary line: "X problem(s)"
		if strings.Contains(output, "problem") {
			issueCount = 0
			summary.WriteString(fmt.Sprintf(" ESLint reported issues in the output above. Command: %s", command))
			return summary.String()
		}
	}

	// ShellCheck
	if strings.Contains(command, "shellcheck") {
		shellCheckPattern := regexp.MustCompile(`^[^:]+:\d+:\d+:\s*\w+\s+:`)
		for _, line := range strings.Split(output, "\n") {
			if shellCheckPattern.MatchString(line) {
				issueCount++
			}
		}
	}

	if !linted && issueCount > 0 {
		summary.WriteString(fmt.Sprintf(" Found approximately %d issue(s).", issueCount))
	} else if !linted && issueCount == 0 {
		summary.WriteString(" Check the output above for details.")
	}

	summary.WriteString(fmt.Sprintf("\nCommand: %s", command))

	return summary.String()
}

// executeProcessManagement handles process management operations: process_list, process_kill, port_check, system_info.
func (te *ToolExecutor) executeProcessManagement(params map[string]interface{}) *ToolResult {
	action, hasAction := params["action"]
	if !hasAction {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: action (process_list, process_kill, port_check, system_info)",
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
	case "process_list":
		return te.executeProcessList(params)
	case "process_kill":
		return te.executeProcessKill(params)
	case "port_check":
		return te.executePortCheck(params)
	case "system_info":
		return te.executeSystemInfo(params)
	default:
		return &ToolResult{
			Success: false,
			Error: fmt.Sprintf("unknown action: %s. Valid actions: process_list, process_kill, port_check, system_info", actionStr),
		}
	}
}

// processEntry represents a single process in the process list.
type processEntry struct {
	PID        int     `json:"pid"`
	Name       string  `json:"name"`
	User       string  `json:"user,omitempty"`
	CPUPercent float64 `json:"cpu_percent,omitempty"`
	MemoryMB   float64 `json:"memory_mb,omitempty"`
	CommandLine string `json:"command_line,omitempty"`
}

// executeProcessList lists running processes with optional filtering.
func (te *ToolExecutor) executeProcessList(params map[string]interface{}) *ToolResult {
	// Filter by name/regex
	filter, hasFilter := params["filter"].(string)
	// Filter by user
	user, hasUser := params["user"].(string)
	// Limit results
	limit := 50
	hasLimit := false
	if limitParam, ok := params["limit"]; ok {
		hasLimit = true
		switch v := limitParam.(type) {
		case float64:
			limit = int(v)
		case int:
			limit = v
		case string:
			if n, err := strconv.Atoi(v); err == nil {
				limit = n
			}
		}
	}
	// Sort order
	sortBy := "pid"
	hasSort := false
	if sortParam, ok := params["sort"]; ok {
		hasSort = true
		switch v := sortParam.(type) {
		case string:
			sortBy = v
		}
	}

	// Get processes based on OS
	var processes []processEntry
	var err error

	switch getOS() {
	case "linux":
		processes, err = te.listProcessesFromProc(user)
	case "darwin":
		processes, err = te.listProcessesFromPs(user)
	default:
		// Windows or other - fall back to ps
		processes, err = te.listProcessesFromPs(user)
	}

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to list processes: %v", err),
		}
	}

	// Apply filter if specified
	if hasFilter && filter != "" {
		re, err2 := regexp.Compile(filter)
		if err2 != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("invalid filter regex: %v", err2),
			}
		}
		filtered := make([]processEntry, 0, len(processes))
		for _, p := range processes {
			if re.MatchString(p.Name) || re.MatchString(p.CommandLine) {
				filtered = append(filtered, p)
			}
		}
		processes = filtered
	}

	// Apply sort
	switch sortBy {
	case "cpu":
		sort.Slice(processes, func(i, j int) bool {
			return processes[i].CPUPercent > processes[j].CPUPercent
		})
	case "memory":
		sort.Slice(processes, func(i, j int) bool {
			return processes[i].MemoryMB > processes[j].MemoryMB
		})
	default: // "pid"
		sort.Slice(processes, func(i, j int) bool {
			return processes[i].PID < processes[j].PID
		})
	}

	// Limit results
	if len(processes) > limit {
		processes = processes[:limit]
	}

	// Format output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d process(es):\n\n", len(processes)))
	output.WriteString(fmt.Sprintf("%-8s %-20s %-10s %-10s %s\n", "PID", "NAME", "MEMORY(MB)", "CPU%", "COMMAND"))
	output.WriteString(strings.Repeat("-", 90) + "\n")

	for _, p := range processes {
		cmd := p.CommandLine
		if len(cmd) > 45 {
			cmd = cmd[:42] + "..."
		}
		output.WriteString(fmt.Sprintf("%-8d %-20s %-10.1f %-10.1f %s\n",
			p.PID, p.Name, p.MemoryMB, p.CPUPercent, cmd))
	}

	result := &ToolResult{
		Success: true,
		Output:  output.String(),
		Extra: map[string]interface{}{
			"tool":           "process_management",
			"action":         "process_list",
			"totalProcesses": len(processes),
		},
	}

	if hasFilter {
		result.Extra["filter"] = filter
	}
	if hasUser {
		result.Extra["user"] = user
	}
	if hasLimit {
		result.Extra["limit"] = limit
	}
	if hasSort {
		result.Extra["sort"] = sortBy
	}

	return result
}

// listProcessesFromProc reads processes from /proc filesystem (Linux).
func (te *ToolExecutor) listProcessesFromProc(userFilter string) ([]processEntry, error) {
	var processes []processEntry

	// Read /proc for process directories
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("cannot read /proc: %v", err)
	}

	// Get system memory info for memory percentage calculation
	var totalMemKB uint64
	memInfo, err := os.ReadFile("/proc/meminfo")
	if err == nil {
		re := regexp.MustCompile(`MemTotal:\s+(\d+) kB`)
		if matches := re.FindSubmatch(memInfo); len(matches) > 1 {
			if val, err2 := strconv.ParseUint(string(matches[1]), 10, 64); err2 == nil {
				totalMemKB = val
			}
		}
	}

	// Get UID to username mapping
	uidMap := te.buildUIDMap()

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if directory name is a PID (numeric)
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		// Read process name from /proc/[pid]/comm
		commPath := filepath.Join("/proc", entry.Name(), "comm")
		comm, err := os.ReadFile(commPath)
		if err != nil {
			continue // Skip inaccessible processes
		}
		name := strings.TrimSpace(string(comm))

		// Read command line from /proc/[pid]/cmdline
		cmdline, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		cmdLineStr := ""
		if err == nil {
			// cmdline is null-separated, replace nulls with spaces
			cmdLineStr = strings.ReplaceAll(string(cmdline), "\x00", " ")
			cmdLineStr = strings.TrimSpace(cmdLineStr)
		}

		// Read memory info from /proc/[pid]/status
		var memoryMB float64
		status, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "status"))
		if err == nil && totalMemKB > 0 {
			re := regexp.MustCompile(`VmRSS:\s+(\d+) kB`)
			if matches := re.FindSubmatch(status); len(matches) > 1 {
				if vmRSS, err2 := strconv.ParseUint(string(matches[1]), 10, 64); err2 == nil {
					memoryMB = float64(vmRSS) / 1024.0
				}
			}
		}

		// Read CPU time from /proc/[pid]/stat
		var cpuPercent float64
		stat, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "stat"))
		if err == nil {
			cpuPercent = te.calculateCPUPercent(stat)
		}

		// Get username from UID
		var username string
		if uidMap != nil {
			if stat, err := os.Stat(filepath.Join("/proc", entry.Name())); err == nil {
				if sys, ok := stat.Sys().(*syscall.Stat_t); ok {
					uid := sys.Uid
					if uname, exists := uidMap[uid]; exists {
						username = uname
					}
				}
			}
		}

		// Apply user filter
		if userFilter != "" && username != "" && username != userFilter {
			continue
		}

		processes = append(processes, processEntry{
			PID:         pid,
			Name:        name,
			User:        username,
			CPUPercent:  cpuPercent,
			MemoryMB:    memoryMB,
			CommandLine: cmdLineStr,
		})
	}

	return processes, nil
}

// calculateCPUPercent calculates CPU usage from /proc/[pid]/stat data.
func (te *ToolExecutor) calculateCPUPercent(stat []byte) float64 {
	// Parse /proc/[pid]/stat format
	// Format: pid (comm) state utime stime ...
	re := regexp.MustCompile(`^\d+\s+\(.+?\)\s+\S+\s+(?:\d+\s+){11}(\d+)\s+(\d+)`)
	if matches := re.FindSubmatch(stat); len(matches) > 2 {
		utime, err1 := strconv.ParseUint(string(matches[1]), 10, 64)
		stime, err2 := strconv.ParseUint(string(matches[2]), 10, 64)
		if err1 == nil && err2 == nil {
			// Calculate total ticks
			totalTicks := float64(utime + stime)
			// Get uptime
			uptimeData, err := os.ReadFile("/proc/uptime")
			if err == nil {
				reUptime := regexp.MustCompile(`^(\d+\.\d+)`)
				if uptimeMatches := reUptime.FindSubmatch(uptimeData); len(uptimeMatches) > 1 {
					if uptimeSec, err3 := strconv.ParseFloat(string(uptimeMatches[1]), 64); err3 == nil {
						// Get clock ticks per second
						ticksPerSec := float64(os.Getpagesize()) // fallback; should be sysconf(_SC_CLK_TCK)
						if ticksPerSec == 0 {
							ticksPerSec = 100 // Common default
						}
						// Calculate percentage
						seconds := totalTicks / ticksPerSec
						pct := (seconds / uptimeSec) * 100.0
						if pct > 100 {
							pct = 100
						}
						return pct
					}
				}
			}
		}
	}
	return 0
}

// buildUIDMap creates a mapping from UID to username by reading /etc/passwd.
func (te *ToolExecutor) buildUIDMap() map[uint32]string {
	uidMap := make(map[uint32]string)
	passwd, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return nil
	}
	re := regexp.MustCompile(`^(\w+):x:(\d+):`)
	for _, line := range strings.Split(string(passwd), "\n") {
		if matches := re.FindStringSubmatch(line); len(matches) > 2 {
			if uid, err := strconv.ParseUint(matches[2], 10, 32); err == nil {
				uidMap[uint32(uid)] = matches[1]
			}
		}
	}
	return uidMap
}

// listProcessesFromPs lists processes using the ps command (macOS/Windows fallback).
func (te *ToolExecutor) listProcessesFromPs(userFilter string) ([]processEntry, error) {
	// Use ps with custom format for cross-platform compatibility
	args := []string{
		"ps", "-eo", "pid,comm,rss,pcpu,args",
		"--sort", "pid",
	}

	cmd := exec.Command("ps", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ps command failed: %v", err)
	}

	var processes []processEntry
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	// Skip header line
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Parse ps output: PID COMMAND RSS PCPU COMMAND_LINE
		parts := strings.Fields(line)
		if len(parts) < 5 {
			continue
		}

		pid, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}

		name := parts[1]
		rssKB, _ := strconv.ParseUint(parts[2], 10, 64)
		cpuPct, _ := strconv.ParseFloat(parts[3], 64)

		// Command line is everything after RSS and CPU%
		cmdLine := strings.Join(parts[4:], " ")

		// Apply user filter (ps without -u doesn't show user, skip filtering)
		if userFilter != "" {
			continue // Can't filter by user without user column
		}

		processes = append(processes, processEntry{
			PID:         pid,
			Name:        name,
			CPUPercent:  cpuPct,
			MemoryMB:    float64(rssKB) / 1024.0,
			CommandLine: cmdLine,
		})
	}

	return processes, nil
}

// executeProcessKill kills a process by PID or name.
func (te *ToolExecutor) executeProcessKill(params map[string]interface{}) *ToolResult {
	pidParam, hasPID := params["pid"]
	nameParam, hasName := params["name"]
	force := false
	if f, ok := params["force"].(bool); ok {
		force = f
	}

	if !hasPID && !hasName {
		return &ToolResult{
			Success: false,
			Error:   "must provide either 'pid' (integer) or 'name' (string) to kill",
		}
	}

	var targetPIDs []int

	if hasPID {
		var pid int
		switch v := pidParam.(type) {
		case float64:
			pid = int(v)
		case int:
			pid = v
		case string:
			if n, err := strconv.Atoi(v); err == nil {
				pid = n
			} else {
				return &ToolResult{
					Success: false,
					Error:   fmt.Sprintf("invalid PID: %v", v),
				}
			}
		}
		targetPIDs = append(targetPIDs, pid)
	}

	if hasName {
		nameStr, ok := nameParam.(string)
		if !ok {
			return &ToolResult{
				Success: false,
				Error:   "process name must be a string",
			}
		}

		// Find processes by name
		processes, err := te.listProcessesFromPs("")
		if err != nil {
			// Try Linux /proc approach
			processes, err = te.listProcessesFromProc("")
			if err != nil {
				return &ToolResult{
					Success: false,
					Error:   fmt.Sprintf("failed to find processes named '%s': %v", nameStr, err),
				}
			}
		}

		found := false
		for _, p := range processes {
			if p.Name == nameStr {
				targetPIDs = append(targetPIDs, p.PID)
				found = true
			}
		}
		if !found {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("no process found with name '%s'", nameStr),
			}
		}
	}

	// Kill each process
	var killed []int
	var failed []int
	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}

	for _, pid := range targetPIDs {
		proc, err := os.FindProcess(pid)
		if err != nil {
			failed = append(failed, pid)
			continue
		}
		if err := proc.Signal(sig); err != nil {
			failed = append(failed, pid)
		} else {
			killed = append(killed, pid)
		}
	}

	result := &ToolResult{
		Success: len(failed) == 0,
	}

	var output strings.Builder
	if len(killed) > 0 {
		output.WriteString(fmt.Sprintf("Sent %s to PID(s) %s\n",
			sig.String(), strings.Join(intSliceToString(killed), ", ")))
	}
	if len(failed) > 0 {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to kill PID(s) %s", strings.Join(intSliceToString(failed), ", "))
		output.WriteString(result.Error)
	}

	result.Output = output.String()

	return result
}

// executePortCheck checks if a specific port is in use.
func (te *ToolExecutor) executePortCheck(params map[string]interface{}) *ToolResult {
	portParam, hasPort := params["port"]
	if !hasPort {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: port",
		}
	}

	var port int
	switch v := portParam.(type) {
	case float64:
		port = int(v)
	case int:
		port = v
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			port = n
		} else {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("invalid port number: %v", v),
			}
		}
	}

	if port < 1 || port > 65535 {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid port number: %d (must be 1-65535)", port),
		}
	}

	protocol := "tcp"
	if p, ok := params["protocol"].(string); ok {
		if p == "udp" || p == "tcp" {
			protocol = p
		} else {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("invalid protocol: %s (must be 'tcp' or 'udp')", p),
			}
		}
	}

	var inUse bool
	var ownerInfo string
	var err error

	switch getOS() {
	case "linux":
		inUse, ownerInfo, err = te.checkPortLinux(port, protocol)
	case "darwin":
		inUse, ownerInfo, err = te.checkPortDarwin(port, protocol)
	default:
		// Windows or other - try netstat
		inUse, ownerInfo, err = te.checkPortNetstat(port, protocol)
	}

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to check port %d/%s: %v", port, protocol, err),
		}
	}

	result := &ToolResult{
		Success: true,
		Extra: map[string]interface{}{
			"tool":        "process_management",
			"action":      "port_check",
			"port":        port,
			"protocol":    protocol,
			"inUse":       inUse,
			"ownerInfo":   ownerInfo,
		},
	}

	if inUse {
		result.Output = fmt.Sprintf("Port %d/%s is IN USE. %s", port, protocol, ownerInfo)
	} else {
		result.Output = fmt.Sprintf("Port %d/%s is available.", port, protocol)
	}

	return result
}

// checkPortLinux checks port availability using /proc/net/tcp and /proc/net/udp.
func (te *ToolExecutor) checkPortLinux(port int, protocol string) (bool, string, error) {
	if protocol == "tcp" || protocol == "both" {
		// Check /proc/net/tcp and /proc/net/tcp6
		tcpFiles := []string{"/proc/net/tcp", "/proc/net/tcp6"}
		for _, tcpFile := range tcpFiles {
			data, err := os.ReadFile(tcpFile)
			if err != nil {
				continue
			}

			portHex := fmt.Sprintf("%X", port)
			for _, line := range strings.Split(string(data), "\n") {
				parts := strings.Fields(line)
				if len(parts) < 2 {
					continue
				}
				// Format: local_address:port state ...
				addrPort := parts[0]
				colonIdx := strings.LastIndex(addrPort, ":")
				if colonIdx == -1 {
					continue
				}
				heardPort := strings.ToUpper(addrPort[colonIdx+1:])
				if heardPort == portHex {
					// Found the port in use
					pid := ""
					if inode := parts[9]; inode != "0" && len(inode) > 0 {
						// Find the PID by looking through /proc/*/fd
						entries, _ := os.ReadDir("/proc")
						for _, entry := range entries {
							if !entry.IsDir() {
								continue
							}
							fdDir := filepath.Join("/proc", entry.Name(), "fd")
							fdEntries, _ := os.ReadDir(fdDir)
							for _, fd := range fdEntries {
								link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
								if err == nil && strings.Contains(link, inode) {
									pid = entry.Name()
									break
								}
							}
							if pid != "" {
								break
							}
						}
					}
					owner := fmt.Sprintf("PID %s", pid)
					if pid == "" {
						owner = "Unknown owner"
					}
					return true, owner, nil
				}
			}
		}
	}

	if protocol == "udp" || protocol == "both" {
		udpFiles := []string{"/proc/net/udp", "/proc/net/udp6"}
		for _, udpFile := range udpFiles {
			data, err := os.ReadFile(udpFile)
			if err != nil {
				continue
			}

			portHex := fmt.Sprintf("%X", port)
			for _, line := range strings.Split(string(data), "\n") {
				parts := strings.Fields(line)
				if len(parts) < 2 {
					continue
				}
				addrPort := parts[0]
				colonIdx := strings.LastIndex(addrPort, ":")
				if colonIdx == -1 {
					continue
				}
				heardPort := strings.ToUpper(addrPort[colonIdx+1:])
				if heardPort == portHex {
					return true, "UDP listener on port " + fmt.Sprint(port), nil
				}
			}
		}
	}

	return false, "", nil
}

// checkPortDarwin checks port availability using lsof on macOS.
func (te *ToolExecutor) checkPortDarwin(port int, protocol string) (bool, string, error) {
	args := []string{
		"-i", fmt.Sprintf("%s:%d", protocol, port),
		"-n", "-P",
	}

	cmd := exec.Command("lsof", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// lsof returns non-zero when port is not in use
		// Check if the error is just "no process found"
		if strings.Contains(string(output), "COMMAND") {
			return false, "", nil
		}
		return false, "", fmt.Errorf("lsof error: %v", err)
	}

	// Port is in use - parse the output for owner info
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) > 1 {
		// First data line: COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME
		parts := strings.Fields(lines[1])
		if len(parts) >= 3 {
			owner := fmt.Sprintf("Process: %s (PID: %s)", parts[0], parts[1])
			if len(parts) > 2 {
				owner = fmt.Sprintf("Process: %s (PID: %s) by user: %s", parts[0], parts[1], parts[2])
			}
			return true, owner, nil
		}
	}

	return true, fmt.Sprintf("Port %d/%s is in use", port, protocol), nil
}

// checkPortNetstat checks port availability using netstat (Windows/fallback).
func (te *ToolExecutor) checkPortNetstat(port int, protocol string) (bool, string, error) {
	var args []string
	protocolUpper := strings.ToUpper(protocol)

	if protocolUpper == "TCP" || protocol == "" {
		args = []string{"netstat", "-an", "-o", "-p", "TCP"}
	} else {
		args = []string{"netstat", "-an", "-o", "-p", protocolUpper}
	}

	cmd := exec.Command("cmd", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, "", fmt.Errorf("netstat error: %v", err)
	}

	portStr := fmt.Sprintf(":%d", port)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, portStr) && (strings.Contains(line, "LISTENING") || strings.Contains(line, protocolUpper)) {
			// Extract PID from last column
			parts := strings.Fields(line)
			if len(parts) > 0 {
				pid := parts[len(parts)-1]
				return true, fmt.Sprintf("PID: %s", pid), nil
			}
		}
	}

	return false, "", nil
}

// executeSystemInfo shows system resource usage (CPU, memory, disk).
func (te *ToolExecutor) executeSystemInfo(params map[string]interface{}) *ToolResult {
	format := "short"
	if f, ok := params["format"].(string); ok {
		format = f
	}

	var output strings.Builder
	var extra map[string]interface{}

	switch getOS() {
	case "linux":
		memory, cpu, disk := te.getLinuxResources()
		output.WriteString(fmt.Sprintf("System Resource Usage (Linux):\n\n"))
		output.WriteString(fmt.Sprintf("Memory:\n"))
		output.WriteString(fmt.Sprintf("  Total:    %.1f GB\n", float64(memory.Total)/1024/1024/1024))
		output.WriteString(fmt.Sprintf("  Used:     %.1f GB\n", float64(memory.Used)/1024/1024/1024))
		output.WriteString(fmt.Sprintf("  Available: %.1f GB\n", float64(memory.Available)/1024/1024/1024))
		output.WriteString(fmt.Sprintf("  Usage:    %.1f%%\n\n", memory.UsagePercent))

		output.WriteString(fmt.Sprintf("CPU:\n"))
		output.WriteString(fmt.Sprintf("  Load Average: %.2f, %.2f, %.2f\n", cpu.Load1, cpu.Load5, cpu.Load15))
		output.WriteString(fmt.Sprintf("  Cores:        %d\n\n", cpu.Cores))

		output.WriteString(fmt.Sprintf("Disk:\n"))
		output.WriteString(fmt.Sprintf("  Total:    %.1f GB\n", float64(disk.Total)/1024/1024/1024))
		output.WriteString(fmt.Sprintf("  Used:     %.1f GB\n", float64(disk.Used)/1024/1024/1024))
		output.WriteString(fmt.Sprintf("  Available: %.1f GB\n", float64(disk.Available)/1024/1024/1024))
		output.WriteString(fmt.Sprintf("  Usage:    %.1f%%\n", disk.UsagePercent))

		extra = map[string]interface{}{
			"tool":         "process_management",
			"action":       "system_info",
			"memory":       memory,
			"cpu":          cpu,
			"disk":         disk,
			"format":       format,
		}

	case "darwin":
		memory, cpu := te.getDarwinResources()
		output.WriteString(fmt.Sprintf("System Resource Usage (macOS):\n\n"))
		output.WriteString(fmt.Sprintf("Memory:\n"))
		output.WriteString(fmt.Sprintf("  Total:     %.1f GB\n", float64(memory.Total)/1024/1024/1024))
		output.WriteString(fmt.Sprintf("  Used:      %.1f GB\n", float64(memory.Used)/1024/1024/1024))
		output.WriteString(fmt.Sprintf("  Available: %.1f GB\n", float64(memory.Available)/1024/1024/1024))
		output.WriteString(fmt.Sprintf("  Usage:     %.1f%%\n\n", memory.UsagePercent))

		output.WriteString(fmt.Sprintf("CPU:\n"))
		output.WriteString(fmt.Sprintf("  Cores:  %d\n", cpu.Cores))

		extra = map[string]interface{}{
			"tool":   "process_management",
			"action": "system_info",
			"memory": memory,
			"cpu":    cpu,
			"format": format,
		}

	default:
		output.WriteString(fmt.Sprintf("System Resource Usage (%s):\n\n", getOS()))
		output.WriteString("Resource details not available for this platform.\n")

		extra = map[string]interface{}{
			"tool":   "process_management",
			"action": "system_info",
			"format": format,
		}
	}

	result := &ToolResult{
		Success: true,
		Output:  output.String(),
		Extra:   extra,
	}

	return result
}

// resourceInfo holds system resource usage data.
type resourceInfo struct {
	Total        uint64  `json:"total"`
	Used         uint64  `json:"used"`
	Available    uint64  `json:"available"`
	UsagePercent float64 `json:"usage_percent"`
}

// cpuInfo holds CPU information.
type cpuInfo struct {
	Cores  int     `json:"cores"`
	Load1  float64 `json:"load_1"`
	Load5  float64 `json:"load_5"`
	Load15 float64 `json:"load_15"`
}

// diskInfo holds disk usage information.
type diskInfo struct {
	Total        uint64  `json:"total"`
	Used         uint64  `json:"used"`
	Available    uint64  `json:"available"`
	UsagePercent float64 `json:"usage_percent"`
}

// getLinuxResources reads system resource info from /proc and /sys on Linux.
func (te *ToolExecutor) getLinuxResources() (*resourceInfo, *cpuInfo, *diskInfo) {
	// Memory info
	memory := te.getLinuxMemory()

	// CPU info
	cpu := te.getLinuxCPU()

	// Disk info
	disk := te.getLinuxDisk()

	return memory, cpu, disk
}

// getLinuxMemory reads memory info from /proc/meminfo.
func (te *ToolExecutor) getLinuxMemory() *resourceInfo {
	mem := &resourceInfo{}

	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return mem
	}

	parseMemInfo := func(key string) uint64 {
		re := regexp.MustCompile(key + `:\s+(\d+) kB`)
		matches := re.FindSubmatch(data)
		if len(matches) > 1 {
			if val, err := strconv.ParseUint(string(matches[1]), 10, 64); err == nil {
				return val * 1024 // Convert kB to bytes
			}
		}
		return 0
	}

	mem.Total = parseMemInfo("MemTotal")
	mem.Available = parseMemInfo("MemAvailable")
	if mem.Total > 0 && mem.Available > 0 {
		mem.Used = mem.Total - mem.Available
	} else {
		mem.Used = parseMemInfo("MemTotal") - parseMemInfo("MemFree") - parseMemInfo("Buffers") - parseMemInfo("Cached")
	}

	if mem.Total > 0 {
		mem.UsagePercent = float64(mem.Used) / float64(mem.Total) * 100
	}

	return mem
}

// getLinuxCPU reads CPU info from /proc/loadavg and /proc/cpuinfo.
func (te *ToolExecutor) getLinuxCPU() *cpuInfo {
	cpu := &cpuInfo{}

	// Load averages from /proc/loadavg
	loadData, err := os.ReadFile("/proc/loadavg")
	if err == nil {
		re := regexp.MustCompile(`^(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)`)
		if matches := re.FindSubmatch(loadData); len(matches) > 3 {
			cpu.Load1, _ = strconv.ParseFloat(string(matches[1]), 64)
			cpu.Load5, _ = strconv.ParseFloat(string(matches[2]), 64)
			cpu.Load15, _ = strconv.ParseFloat(string(matches[3]), 64)
		}
	}

	// CPU cores from /proc/cpuinfo
	cpuData, err := os.ReadFile("/proc/cpuinfo")
	if err == nil {
		cpu.Cores = strings.Count(string(cpuData), "processor")
		if cpu.Cores == 0 {
			// Fallback: use runtime.NumCPU()
			cpu.Cores = runtime.NumCPU()
		}
	} else {
		cpu.Cores = runtime.NumCPU()
	}

	return cpu
}

// getLinuxDisk reads disk usage using syscall.Statfs.
func (te *ToolExecutor) getLinuxDisk() *diskInfo {
	disk := &diskInfo{}

	var stat syscall.Statfs_t
	err := syscall.Statfs("/", &stat)
	if err != nil {
		return disk
	}

	total := uint64(stat.Bsize) * uint64(stat.Blocks)
	available := uint64(stat.Bsize) * uint64(stat.Bavail)
	used := total - (uint64(stat.Bsize) * uint64(stat.Bfree))

	disk.Total = total
	disk.Used = used
	disk.Available = available

	if total > 0 {
		disk.UsagePercent = float64(used) / float64(total) * 100
	}

	return disk
}

// getDarwinResources reads system resource info from sysctl on macOS.
func (te *ToolExecutor) getDarwinResources() (*resourceInfo, *cpuInfo) {
	memory := &resourceInfo{}
	cpu := &cpuInfo{}

	// Memory info
	memoryTotal, _ := exec.Command("sysctl", "-n", "hw.memsize").CombinedOutput()
	if val, err := strconv.ParseUint(strings.TrimSpace(string(memoryTotal)), 10, 64); err == nil {
		memory.Total = val
	}

	// Memory pages
	pageSize, _ := exec.Command("sysctl", "-n", "hw.pagesize").CombinedOutput()
	pageSizeVal := uint64(4096) // Default
	if val, err := strconv.ParseUint(strings.TrimSpace(string(pageSize)), 10, 64); err == nil {
		pageSizeVal = val
	}

	// Get memory stats via vm_stat
	vmStat, err := exec.Command("vm_stat").CombinedOutput()
	if err == nil {
		pageSizeBytes := float64(pageSizeVal)
		pages := make(map[string]float64)
		for _, line := range strings.Split(strings.TrimSpace(string(vmStat)), "\n") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				key := strings.Trim(parts[0], ".")
				if val, err := strconv.ParseFloat(strings.Trim(parts[1], " "), 64); err == nil {
					pages[key] = val * pageSizeBytes
				}
			}
		}

		// Calculate used memory (active + inactive + wired - speculative)
		used := pages["Pages wired down"] + pages["Pages active"] + pages["Pages inactive"]
		if spec, ok := pages["Pages speculative"]; ok {
			used -= spec
		}
		memory.Used = uint64(used)
		memory.Available = memory.Total - memory.Used

		if memory.Total > 0 {
			memory.UsagePercent = float64(memory.Used) / float64(memory.Total) * 100
		}
	}

	// CPU info
	cpuCores, _ := exec.Command("sysctl", "-n", "hw.ncpu").CombinedOutput()
	if val, err := strconv.Atoi(strings.TrimSpace(string(cpuCores))); err == nil {
		cpu.Cores = val
	}

	return memory, cpu
}

// getOS returns the current operating system.
func getOS() string {
	return runtime.GOOS
}

// intSliceToString converts []int to []string.
func intSliceToString(ints []int) []string {
	strs := make([]string, len(ints))
	for i, v := range ints {
		strs[i] = strconv.Itoa(v)
	}
	return strs
}

// parseEnvFile parses a .env file and returns key-value pairs.
// Supports KEY=VALUE, # comments, single and double quoted values.
func parseEnvFile(content string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Find the first '=' sign
		eqIdx := strings.Index(line, "=")
		if eqIdx == -1 {
			continue // Skip malformed lines
		}

		key := strings.TrimSpace(line[:eqIdx])
		value := strings.TrimSpace(line[eqIdx+1:])

		if key == "" {
			continue // Skip lines with empty keys
		}

		// Remove surrounding quotes if present
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}

		result[key] = value
	}

	return result
}

// executeEnvVar manages environment variables with get, set, unset, list, and source actions.
func (te *ToolExecutor) executeEnvVar(params map[string]interface{}) *ToolResult {
	action, ok := params["action"].(string)
	if !ok || action == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: action (get, set, unset, list, source)",
		}
	}

	switch action {
	case "get":
		return te.envVarGet(params)
	case "set":
		return te.envVarSet(params)
	case "unset":
		return te.envVarUnset(params)
	case "list":
		return te.envVarList(params)
	case "source":
		return te.envVarSource(params)
	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s (valid: get, set, unset, list, source)", action),
		}
	}
}

// envVarGet reads a specific environment variable.
func (te *ToolExecutor) envVarGet(params map[string]interface{}) *ToolResult {
	name, ok := params["name"].(string)
	if !ok || name == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: name",
		}
	}

	value := os.Getenv(name)
	if value == "" {
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Environment variable '%s' is not set", name),
			Extra: map[string]interface{}{
				"action": "get",
				"name":   name,
				"set":    false,
			},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("%s=%s", name, value),
		Path:    name,
		Extra: map[string]interface{}{
			"action": "get",
			"name":   name,
			"value":  value,
			"set":    true,
		},
	}
}

// envVarSet sets an environment variable for the current process.
func (te *ToolExecutor) envVarSet(params map[string]interface{}) *ToolResult {
	name, ok := params["name"].(string)
	if !ok || name == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: name",
		}
	}

	value, ok := params["value"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: value",
		}
	}

	os.Setenv(name, value)

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Set environment variable '%s' to '%s'", name, value),
		Extra: map[string]interface{}{
			"action": "set",
			"name":   name,
		},
	}
}

// envVarUnset unsets an environment variable.
func (te *ToolExecutor) envVarUnset(params map[string]interface{}) *ToolResult {
	name, ok := params["name"].(string)
	if !ok || name == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: name",
		}
	}

	os.Unsetenv(name)

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Unset environment variable '%s'", name),
		Extra: map[string]interface{}{
			"action": "unset",
			"name":   name,
		},
	}
}

// envVarList lists environment variables with optional prefix filtering.
func (te *ToolExecutor) envVarList(params map[string]interface{}) *ToolResult {
	prefix, hasPrefix := params["prefix"].(string)
	showAll, hasShowAll := params["show_all"].(bool)
	// Also support string "true"/"false"
	if !hasShowAll {
		if s, ok := params["show_all"].(string); ok {
			showAll = strings.EqualFold(s, "true")
		}
	}

	var lines []string
	count := 0

	for _, env := range os.Environ() {
		eqIdx := strings.Index(env, "=")
		if eqIdx == -1 {
			continue
		}
		name := env[:eqIdx]
		value := env[eqIdx+1:]

		// Apply prefix filter
		if hasPrefix && prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}

		// Skip empty values unless show_all is true
		if !showAll && value == "" {
			continue
		}

		if value == "" {
			lines = append(lines, fmt.Sprintf("  %s= (empty)", name))
		} else if len(value) > 100 {
			lines = append(lines, fmt.Sprintf("  %s=%s... (%d chars)", name, value[:100], len(value)))
		} else {
			lines = append(lines, fmt.Sprintf("  %s=%s", name, value))
		}
		count++
	}

	if len(lines) == 0 {
		return &ToolResult{
			Success: true,
			Output:  "No environment variables found",
			Extra: map[string]interface{}{
				"action":    "list",
				"count":     0,
				"hasPrefix": hasPrefix,
			},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Found %d environment variable(s):\n\n%s", count, strings.Join(lines, "\n")),
		Extra: map[string]interface{}{
			"action":    "list",
			"count":     count,
			"hasPrefix": hasPrefix,
		},
	}
}

// envVarSource sources a .env file and loads its variables.
func (te *ToolExecutor) envVarSource(params map[string]interface{}) *ToolResult {
	path, ok := params["path"].(string)
	if !ok || path == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	overwrite, hasOverwrite := params["overwrite"].(bool)
	if !hasOverwrite {
		if s, ok := params["overwrite"].(string); ok {
			overwrite = strings.EqualFold(s, "true")
		}
	}

	// Read the .env file
	content, err := os.ReadFile(path)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read .env file: %v", err),
		}
	}

	// Parse the .env file
	pairs := parseEnvFile(string(content))

	if len(pairs) == 0 {
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("No variables found in %s", path),
			Extra: map[string]interface{}{
				"action":   "source",
				"path":     path,
				"loaded":   0,
				"skipped":  0,
			},
		}
	}

	loaded := 0
	skipped := 0
	skippedVars := []string{}

	for name, value := range pairs {
		_, exists := os.LookupEnv(name)
		if exists && !overwrite {
			skipped++
			skippedVars = append(skippedVars, name)
			continue
		}
		os.Setenv(name, value)
		loaded++
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Loaded %d variable(s) from %s", loaded, path))

	if skipped > 0 {
		output.WriteString(fmt.Sprintf(", skipped %d (already set): %s", skipped, strings.Join(skippedVars, ", ")))
	} else if overwrite && skipped > 0 {
		// Shouldn't happen, but just in case
		output.WriteString(fmt.Sprintf(", overwrote %d existing variable(s)", skipped))
	}

	output.WriteString("\n")

	return &ToolResult{
		Success: true,
		Output:  output.String(),
		Extra: map[string]interface{}{
			"action":  "source",
			"path":    path,
			"loaded":  loaded,
			"skipped": skipped,
		},
	}
}

// executeFileCompare compares two files and returns a structured diff.
func (te *ToolExecutor) executeFileCompare(params map[string]interface{}) *ToolResult {
	src1, ok := params["file1"].(string)
	if !ok || src1 == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: file1",
		}
	}

	src2, ok := params["file2"].(string)
	if !ok || src2 == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: file2",
		}
	}

	// Optional context lines (default: 3)
	contextLines := 3
	if ctx, ok := params["context"].(float64); ok {
		contextLines = int(ctx)
	}

	// Clean paths to prevent directory traversal
	cleanPath1 := filepath.Clean(src1)
	cleanPath2 := filepath.Clean(src2)

	// Prevent directory traversal
	for _, p := range []string{cleanPath1, cleanPath2} {
		if strings.HasPrefix(p, "..") {
			return &ToolResult{
				Success: false,
				Error:   "invalid path: directory traversal not allowed",
			}
		}
	}

	// Read file 1
	data1, err := os.ReadFile(cleanPath1)
	if os.IsNotExist(err) {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("file not found: %s", cleanPath1),
		}
	}
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("cannot read file1: %v", err),
		}
	}

	// Read file 2
	data2, err := os.ReadFile(cleanPath2)
	if os.IsNotExist(err) {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("file not found: %s", cleanPath2),
		}
	}
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("cannot read file2: %v", err),
		}
	}

	// Check for binary content
	isBinary1 := isBinaryContent(data1)
	isBinary2 := isBinaryContent(data2)
	if isBinary1 || isBinary2 {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("cannot compare binary files (file1=%v, file2=%v)", isBinary1, isBinary2),
		}
	}

	// Split files into lines
	lines1 := splitLines(data1)
	lines2 := splitLines(data2)

	// Compute diff using LCS-based algorithm
	diff := computeDiff(lines1, lines2, contextLines)

	// Count statistics
	addedLines := 0
	removedLines := 0
	unchangedLines := 0
	for _, h := range diff.Hunks {
		for _, line := range h.Lines {
			switch line.Type {
			case "+":
				addedLines++
			case "-":
				removedLines++
			case " ":
				unchangedLines++
			}
		}
	}

	// Build output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("--- %s\n", cleanPath1))
	output.WriteString(fmt.Sprintf("+++ %s\n", cleanPath2))
	output.WriteString(fmt.Sprintf("\nFiles differ: %d lines added, %d lines removed, %d lines unchanged\n", addedLines, removedLines, unchangedLines))
	output.WriteString("\n")

	for _, hunk := range diff.Hunks {
		output.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", hunk.Start1, hunk.Count1, hunk.Start2, hunk.Count2))
		for _, line := range hunk.Lines {
			prefix := " "
			switch line.Type {
			case "+":
				prefix = "+"
			case "-":
				prefix = "-"
			}
			output.WriteString(fmt.Sprintf("%s%s\n", prefix, line.Text))
		}
		output.WriteString("\n")
	}

	if diff.TotalHunks == 0 {
		output.WriteString("Files are identical\n")
	}

	return &ToolResult{
		Success:  true,
		Output:   output.String(),
		Extra: map[string]interface{}{
			"file1":           cleanPath1,
			"file2":           cleanPath2,
			"added_lines":     addedLines,
			"removed_lines":   removedLines,
			"unchanged_lines": unchangedLines,
			"hunks":           diff.TotalHunks,
		},
	}
}

// diffLine represents a single line in a diff output.
type diffLine struct {
	Type string // "+", "-", " "
	Text string
}

// diffHunk represents a contiguous block of changes.
type diffHunk struct {
	Start1 int // starting line in file1
	Count1 int // number of lines from file1
	Start2 int // starting line in file2
	Count2 int // number of lines from file2
	Lines  []diffLine
}

// diffResult represents the result of a file comparison.
type diffResult struct {
	TotalHunks int
	Hunks      []diffHunk
}

// computeDiff computes a diff between two sets of lines using a simple LCS algorithm.
// It supports context lines for unified diff output.
func computeDiff(lines1, lines2 []string, context int) *diffResult {
	m := len(lines1)
	n := len(lines2)

	// If files are identical, return no hunks
	if m == n {
		for i := 0; i < m; i++ {
			if lines1[i] != lines2[i] {
				goto notIdentical
			}
		}
		return &diffResult{}
	}

notIdentical:
	// Compute LCS table
	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if lines1[i-1] == lines2[j-1] {
				lcs[i][j] = lcs[i-1][j-1] + 1
			} else {
				if lcs[i-1][j] >= lcs[i][j-1] {
					lcs[i][j] = lcs[i-1][j]
				} else {
					lcs[i][j] = lcs[i][j-1]
				}
			}
		}
	}

	// Traceback to find edit operations
	var edits []editOp
	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && lines1[i-1] == lines2[j-1] {
			edits = append(edits, editOp{type_: " ", line1: i, line2: j, text: lines1[i-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || lcs[i][j-1] >= lcs[i-1][j]) {
			edits = append(edits, editOp{type_: "+", line1: 0, line2: j, text: lines2[j-1]})
			j--
		} else {
			edits = append(edits, editOp{type_: "-", line1: i, line2: 0, text: lines1[i-1]})
			i--
		}
	}

	// Reverse edits
	for i, j := 0, len(edits)-1; i < j; i, j = i+1, j-1 {
		edits[i], edits[j] = edits[j], edits[i]
	}

	// Group edits into hunks with context
	return groupIntoHunks(edits, context)
}

// editOp represents a single edit operation.
type editOp struct {
	type_  string
	line1  int
	line2  int
	text   string
}

// groupIntoHunks groups edit operations into hunk blocks with context lines.
func groupIntoHunks(edits []editOp, context int) *diffResult {
	if len(edits) == 0 {
		return &diffResult{}
	}

	// Find ranges of non-context edits
	var nonContextRanges []struct{ start, end int }
	var lastNCStart, lastNCEnd int
	foundFirst := false

	for i, e := range edits {
		if e.type_ != " " {
			if !foundFirst {
				lastNCStart = i
				foundFirst = true
			}
			lastNCEnd = i
		} else {
			if foundFirst {
				nonContextRanges = append(nonContextRanges, struct{ start, end int }{lastNCStart, lastNCEnd})
				foundFirst = false
			}
		}
	}
	if foundFirst {
		nonContextRanges = append(nonContextRanges, struct{ start, end int }{lastNCStart, lastNCEnd})
	}

	// Build hunks by merging nearby ranges
	var rawHunks []struct{ start, end int }
	if len(nonContextRanges) == 0 {
		return &diffResult{}
	}

	curStart := nonContextRanges[0].start - context
	curEnd := nonContextRanges[0].end + context
	if curStart < 0 {
		curStart = 0
	}
	if curEnd >= len(edits) {
		curEnd = len(edits) - 1
	}

	for i := 1; i < len(nonContextRanges); i++ {
		rngStart := nonContextRanges[i].start - context
		rngEnd := nonContextRanges[i].end + context
		if rngStart <= curEnd+1 {
			// Merge
			if rngEnd > curEnd {
				curEnd = rngEnd
			}
		} else {
			rawHunks = append(rawHunks, struct{ start, end int }{curStart, curEnd})
			curStart = rngStart
			if curStart < 0 {
				curStart = 0
			}
			curEnd = rngEnd
			if curEnd >= len(edits) {
				curEnd = len(edits) - 1
			}
		}
	}
	rawHunks = append(rawHunks, struct{ start, end int }{curStart, curEnd})

	// Build result hunks
	var result diffResult
	for _, rng := range rawHunks {
		var hunk diffHunk

		// Collect lines for this hunk
		for i := rng.start; i <= rng.end && i < len(edits); i++ {
			e := edits[i]
			hunk.Lines = append(hunk.Lines, diffLine{
				Type: e.type_,
				Text: e.text,
			})

			if e.type_ != " " {
				if e.line1 > 0 {
					if hunk.Start1 == 0 {
						hunk.Start1 = e.line1 - (rng.start - i)
						if hunk.Start1 < 1 {
							hunk.Start1 = 1
						}
					}
				}
				if e.line2 > 0 {
					if hunk.Start2 == 0 {
						hunk.Start2 = e.line2 - (rng.start - i)
						if hunk.Start2 < 1 {
							hunk.Start2 = 1
						}
					}
				}
			}
			if e.type_ == "-" {
				hunk.Count1++
			}
			if e.type_ == " " {
				hunk.Count1++
				if hunk.Start1 == 0 {
					hunk.Start1 = e.line1
				}
			}
			if e.type_ == "+" {
				hunk.Count2++
			}
			if e.type_ == " " {
				hunk.Count2++
				if hunk.Start2 == 0 {
					hunk.Start2 = e.line2
				}
			}
		}

		// Fixup start positions
		if hunk.Start1 == 0 {
			hunk.Start1 = 1
		}
		if hunk.Start2 == 0 {
			hunk.Start2 = 1
		}

		result.Hunks = append(result.Hunks, hunk)
	}

	result.TotalHunks = len(result.Hunks)
	return &result
}

// isBinaryContent checks if data appears to be binary content.
func isBinaryContent(data []byte) bool {
	// Check for null bytes (common binary indicator)
	nullCount := 0
	total := len(data)
	if total > 8192 {
		total = 8192
	}
	for i := 0; i < total; i++ {
		if data[i] == 0 {
			nullCount++
		}
	}
	return nullCount > 0 && float64(nullCount)/float64(total) > 0.01
}

// splitLines splits raw file data into individual lines.
func splitLines(data []byte) []string {
	text := string(data)
	// Remove trailing newline for consistent line counting
	if len(text) > 0 && text[len(text)-1] == '\n' {
		text = text[:len(text)-1]
	}
	if text == "" {
		return []string{}
	}
	return strings.Split(text, "\n")
}

// conventionalCommitRegex matches conventional commit messages: type(scope): description or type: description
var conventionalCommitRegex = regexp.MustCompile(`^(\w+)(?:\([^)]*\))?!?:\s*(.*)$`)

// categoryMapping maps conventional commit types to changelog categories.
var categoryMapping = map[string]string{
	"feat":      "Features",
	"feature":   "Features",
	"feat!":     "Breaking Changes",
	"!":         "Breaking Changes",
	"fix":       "Bug Fixes",
	"bugfix":    "Bug Fixes",
	"perf":      "Performance Improvements",
	"performance": "Performance Improvements",
	"refactor":  "Code Refactoring",
	"style":     "Styles",
	"docs":      "Documentation",
	"doc":       "Documentation",
	"chore":     "Chores",
	"test":      "Tests",
	"build":     "Build System",
	"ci":        "Continuous Integration",
	"revert":    "Reverts",
}

// commitEntry represents a single parsed commit entry for changelog generation.
type commitEntry struct {
	Type    string // conventional commit type
	Category string // mapped changelog category
	Scope   string // optional scope from commit
	Message string // commit description
	Hash    string // short commit hash
}

// executeChangelog generates changelog entries from git commit history.
func (te *ToolExecutor) executeChangelog(params map[string]interface{}) *ToolResult {
	action := "generate"
	if a, ok := params["action"].(string); ok {
		action = a
	}

	switch action {
	case "generate":
		return te.changelogGenerate(params)
	case "add":
		return te.changelogAdd(params)
	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown changelog action: %s (use 'generate' or 'add')", action),
		}
	}
}

// changelogGenerate generates changelog from git commits.
func (te *ToolExecutor) changelogGenerate(params map[string]interface{}) *ToolResult {
	// Get commit range
	fromTag, hasFrom := params["from_tag"].(string)
	toTag, hasTo := params["to_tag"].(string)
	if !hasTo {
		toTag = "HEAD"
	}

	// Get unreleased flag
	unreleased := false
	if u, ok := params["unreleased"].(bool); ok {
		unreleased = u
	}

	// Custom header
	customHeader, hasHeader := params["header"].(string)

	// Build git log arguments
	args := []string{"log", "--pretty=format:%H %s", "--date-order"}
	if hasFrom && fromTag != "" {
		if unreleased {
			// For unreleased: find commits from the tag that exist in HEAD
			// but have no further tags between them and HEAD
			args = append(args, fmt.Sprintf("%s..HEAD", fromTag))
		} else {
			args = append(args, fmt.Sprintf("%s..%s", fromTag, toTag))
		}
	} else if unreleased {
		// Find the most recent tag before HEAD
		tagCmd := exec.Command("git", "describe", "--tags", "--abbrev=0", "HEAD~100")
		tagOutput, err := tagCmd.CombinedOutput()
		if err == nil {
			tag := strings.TrimSpace(string(tagOutput))
			if tag != "" {
				args = append(args, fmt.Sprintf("%s..HEAD", tag))
			} else {
				// No tag found, include all commits
				args = append(args, "--all")
			}
		}
	} else {
		args = append(args, fmt.Sprintf("%s..%s", fromTag, toTag))
	}

	cmd := exec.Command("git", args...)
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git log failed: %s", string(cmdOutput)),
		}
	}

	// Parse commits
	commitLines := strings.TrimSpace(string(cmdOutput))
	if commitLines == "" {
		return &ToolResult{
			Success: true,
			Output:  "No commits found in the specified range.",
		}
	}

	var commits []commitEntry
	for _, line := range strings.Split(commitLines, "\n") {
		if line == "" {
			continue
		}

		// Parse: full_hash subject
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}

		commitHash := parts[0]
		subject := parts[1]

		// Skip merge commits
		if strings.HasPrefix(subject, "Merge ") {
			continue
		}

		// Match conventional commit pattern
		matches := conventionalCommitRegex.FindStringSubmatch(subject)
		if matches == nil {
			continue
		}

		commitType := matches[1]
		description := matches[2]

		// Handle scope
		scope := ""
		spaceIdx := strings.Index(description, " ")
		if spaceIdx > 0 {
			// Check for scope in description (e.g., "feat(core): add feature")
			if strings.HasPrefix(description, "(") {
				closeIdx := strings.Index(description, ")")
				if closeIdx > 0 {
					scope = description[1:closeIdx]
					description = description[closeIdx+2:]
				}
			}
		}

		// Determine category
		category := "Other Changes"
		if !strings.HasSuffix(commitType, "!") {
			if mapping, ok := categoryMapping[commitType]; ok {
				category = mapping
			}
		} else {
			category = "Breaking Changes"
		}
		// Double-check for breaking change indicator
		if strings.HasSuffix(commitType, "!") || strings.Contains(subject, " BREAKING CHANGE:") {
			category = "Breaking Changes"
		}

		// Short hash
		shortHash := commitHash[:7]

		// Clean up description (capitalize first letter, remove trailing period)
		if len(description) > 0 {
			description = strings.ToLower(description[:1]) + description[1:]
			description = strings.TrimRight(description, ".")
		}

		commits = append(commits, commitEntry{
			Type:     commitType,
			Category: category,
			Scope:    scope,
			Message:  description,
			Hash:     shortHash,
		})
	}

	if len(commits) == 0 {
		return &ToolResult{
			Success: true,
			Output:  "No conventional commits found in the specified range.",
		}
	}

	// Group commits by category
	categories := make(map[string][]string)
	categoryOrder := []string{"Breaking Changes", "Features", "Bug Fixes", "Performance Improvements", "Code Refactoring", "Styles", "Documentation", "Tests", "Build System", "Continuous Integration", "Chores", "Reverts", "Other Changes"}

	for _, c := range commits {
		msg := "- " + c.Message
		if c.Scope != "" {
			msg = fmt.Sprintf("- **%s**: %s", c.Scope, c.Message)
		}
		categories[c.Category] = append(categories[c.Category], msg)
	}

	// Build output
	var output strings.Builder
	if hasHeader && customHeader != "" {
		output.WriteString(customHeader + "\n\n")
	}

	if unreleased {
		output.WriteString("## [Unreleased]\n\n")
	}

	for _, cat := range categoryOrder {
		entries, exists := categories[cat]
		if !exists {
			continue
		}
		output.WriteString(fmt.Sprintf("### %s\n\n", cat))
		for _, entry := range entries {
			output.WriteString(entry + "\n")
		}
		output.WriteString("\n")
	}

	// Add footer with commit hashes
	output.WriteString("### Commits\n\n")
	for _, c := range commits {
		if c.Scope != "" {
			output.WriteString(fmt.Sprintf("- [`%s`](https://github.com/placeholder/placeholder/commit/%s) [%s] %s\n", c.Hash, c.Hash, c.Scope, c.Message))
		} else {
			output.WriteString(fmt.Sprintf("- [`%s`](https://github.com/placeholder/placeholder/commit/%s) %s\n", c.Hash, c.Hash, c.Message))
		}
	}

	return &ToolResult{
		Success: true,
		Output:  output.String(),
		Extra: map[string]interface{}{
			"tool":         "changelog",
			"action":       "generate",
			"totalCommits": len(commits),
			"categories":   len(categories),
			"unreleased":   unreleased,
		},
	}
}

// changelogAdd appends generated entries to an existing CHANGELOG file.
func (te *ToolExecutor) changelogAdd(params map[string]interface{}) *ToolResult {
	tag, hasTag := params["tag"].(string)
	if !hasTag {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: tag",
		}
	}

	date, hasDate := params["date"].(string)
	if !hasDate {
		date = time.Now().Format("2006-01-02")
	}

	path, hasPath := params["path"].(string)
	if !hasPath {
		path = "CHANGELOG.md"
	}

	unreleased := false
	if u, ok := params["unreleased"].(bool); ok {
		unreleased = u
	}

	// Check if CHANGELOG exists
	existingContent := ""
	exists := true
	if _, err := os.Stat(path); os.IsNotExist(err) {
		exists = false
	} else {
		content, err := os.ReadFile(path)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to read changelog: %v", err),
			}
		}
		existingContent = string(content)
	}

	// Generate changelog content
	genResult := te.changelogGenerate(params)
	if !genResult.Success {
		return genResult
	}

	generatedContent := genResult.Output

	// Handle unreleased mode: move unreleased section to the new tag
	if unreleased && exists {
		// Find unreleased section in existing content
		re := regexp.MustCompile(`(?s)## \[Unreleased\]\n\n(.*?)(?=## |\z)`)
		match := re.FindStringSubmatch(existingContent)

		if match != nil {
			unreleasedEntries := match[1]
			// Remove unreleased section from existing content
			existingContent = re.ReplaceAllString(existingContent, "")

			// Build new entry with tag
			newSection := fmt.Sprintf("## [%s] - %s\n\n%s\n", tag, date, strings.TrimSpace(unreleasedEntries))

			// Prepend the new section to existing content (or the generated content)
			if existingContent == "" {
				existingContent = newSection
			} else {
				// Insert after the title/header
				existingContent = insertAfterHeader(existingContent, newSection)
			}
		}

		// Write updated changelog
		if err := os.WriteFile(path, []byte(existingContent), 0644); err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to write changelog: %v", err),
			}
		}

		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Added unreleased changes under tag [%s] - %s in %s", tag, date, path),
			Extra: map[string]interface{}{
				"tool":       "changelog",
				"action":     "add",
				"tag":        tag,
				"date":       date,
				"file":       path,
				"unreleased": true,
			},
		}
	}

	// Standard add: append new entry
	newEntry := fmt.Sprintf("## [%s] - %s\n\n%s", tag, date, strings.TrimSpace(generatedContent))

	var newContent string
	if !exists {
		// Create new file with title
		newContent = "# Changelog\n\n" + newEntry + "\n"
	} else {
		// Insert after the title
		newContent = insertAfterHeader(existingContent, newEntry)
	}

	// Write changelog
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to write changelog: %v", err),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Added changelog entry for [%s] - %s in %s", tag, date, path),
		Extra: map[string]interface{}{
			"tool":   "changelog",
			"action": "add",
			"tag":    tag,
			"date":   date,
			"file":   path,
		},
	}
}

// insertAfterHeader inserts content after the first # title line in markdown content.
func insertAfterHeader(content, toInsert string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			// Insert after the title line
			result := make([]string, 0, len(lines)+2)
			result = append(result, lines[:i+1]...)
			result = append(result, "")
			result = append(result, toInsert)
			result = append(result, "\n")
			result = append(result, lines[i+1:]...)
			return strings.Join(result, "\n")
		}
	}
	// No title found, prepend
	return "# Changelog\n\n" + toInsert + "\n"
}

// executeGitTag manages git tags: list, create, delete, and show.
func (te *ToolExecutor) executeGitTag(params map[string]interface{}) *ToolResult {
	action, hasAction := params["action"]
	if !hasAction {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: action",
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
		return te.executeGitTagList(params)
	case "create":
		return te.executeGitTagCreate(params)
	case "delete":
		return te.executeGitTagDelete(params)
	case "show":
		return te.executeGitTagShow(params)
	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s. Valid actions: 'list', 'create', 'delete', 'show'", actionStr),
		}
	}
}

// executeGitTagList lists git tags with optional filtering.
func (te *ToolExecutor) executeGitTagList(params map[string]interface{}) *ToolResult {
	// Optional: filter by pattern
	pattern, hasPattern := params["pattern"].(string)

	// Optional: max number of results
	maxResults := 50
	if mr, ok := params["max_results"].(float64); ok {
		maxResults = int(mr)
	} else if mr, ok := params["max_results"].(int); ok {
		maxResults = mr
	} else if mr, ok := params["max_results"].(string); ok {
		if n, err := strconv.Atoi(mr); err == nil {
			maxResults = n
		}
	}

	// Sort order (default: version sort)
	sortOrder := "version"
	if so, ok := params["sort"].(string); ok {
		sortOrder = so
	}

	args := []string{"tag", "--sort=" + sortOrder}

	if hasPattern {
		args = append(args, pattern)
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git tag list failed: %s", string(output)),
		}
	}

	tagLines := strings.Split(strings.TrimSpace(string(output)), "\n")
	// Filter empty lines
	var tags []string
	for _, line := range tagLines {
		line = strings.TrimSpace(line)
		if line != "" {
			tags = append(tags, line)
		}
	}

	// Limit results
	if len(tags) > maxResults {
		tags = tags[:maxResults]
	}

	result := &ToolResult{
		Success: true,
		Output: fmt.Sprintf("Found %d tag(s):\n\n%s", len(tags), strings.Join(tags, "\n")),
		Extra: map[string]interface{}{
			"tool":       "git_tag",
			"action":     "list",
			"totalTags":  len(tagLines),
			"returned":   len(tags),
			"pattern":    pattern,
			"sortOrder":  sortOrder,
		},
	}

	if !hasPattern {
		result.Extra["pattern"] = "(none)"
	}

	return result
}

// executeGitTagCreate creates a new git tag.
func (te *ToolExecutor) executeGitTagCreate(params map[string]interface{}) *ToolResult {
	name, hasName := params["name"].(string)
	if !hasName {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: name",
		}
	}

	// Validate tag name
	if !isValidTagName(name) {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid tag name: %s. Tag names must not contain spaces, tildes, carets, colons, or question marks", name),
		}
	}

	// Check if tag already exists
	checkCmd := exec.Command("git", "tag", "-l", name)
	checkOutput, err := checkCmd.CombinedOutput()
	if err == nil && strings.TrimSpace(string(checkOutput)) == name {
		// Check if it's an existing tag
		existingCmd := exec.Command("git", "tag", "-v", name, "/dev/null")
		_, existingErr := existingCmd.CombinedOutput()
		if existingErr == nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("tag '%s' already exists", name),
				Extra: map[string]interface{}{
					"tool":  "git_tag",
					"action": "create",
					"name":  name,
				},
			}
		}
	}

	// Check for optional message
	message, hasMessage := params["message"].(string)

	// Check for optional force flag
	force := false
	if f, ok := params["force"].(bool); ok {
		force = f
	}

	// Check for optional light weight flag
	annotated := true // default
	if lw, ok := params["annotated"].(bool); ok {
		annotated = lw
	} else if lw, ok := params["lightweight"].(bool); ok {
		annotated = !lw
	}

	args := []string{"tag"}

	if annotated {
		args = append(args, "-a")
		if hasMessage {
			args = append(args, "-m", message)
		} else {
			// Lightweight annotated tag with no message
			args = append(args, "-m", "")
		}
	}

	if force {
		args = append(args, "-f")
	}

	args = append(args, name)

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git tag create failed: %s", string(output)),
			Extra: map[string]interface{}{
				"tool":        "git_tag",
				"action":      "create",
				"name":        name,
				"annotated":   annotated,
				"force":       force,
			},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Created %s tag '%s'", map[bool]string{true: "annotated", false: "lightweight"}[annotated], name),
		Extra: map[string]interface{}{
			"tool":      "git_tag",
			"action":    "create",
			"name":      name,
			"annotated": annotated,
			"message":   message,
		},
	}
}

// executeGitTagDelete deletes a git tag.
func (te *ToolExecutor) executeGitTagDelete(params map[string]interface{}) *ToolResult {
	name, hasName := params["name"].(string)
	if !hasName {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: name",
		}
	}

	// Check if tag exists
	checkCmd := exec.Command("git", "tag", "-l", name)
	checkOutput, err := checkCmd.CombinedOutput()
	if err == nil && strings.TrimSpace(string(checkOutput)) != name {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("tag '%s' does not exist", name),
			Extra: map[string]interface{}{
				"tool":   "git_tag",
				"action": "delete",
				"name":   name,
			},
		}
	}

	args := []string{"tag", "-d", name}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git tag delete failed: %s", string(output)),
			Extra: map[string]interface{}{
				"tool":   "git_tag",
				"action": "delete",
				"name":   name,
			},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Deleted tag '%s'", name),
		Extra: map[string]interface{}{
			"tool":   "git_tag",
			"action": "delete",
			"name":   name,
		},
	}
}

// executeGitTagShow shows details about a specific git tag.
func (te *ToolExecutor) executeGitTagShow(params map[string]interface{}) *ToolResult {
	name, hasName := params["name"].(string)
	if !hasName {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: name",
		}
	}

	// First check if tag exists
	checkCmd := exec.Command("git", "tag", "-l", name)
	checkOutput, err := checkCmd.CombinedOutput()
	if err == nil && strings.TrimSpace(string(checkOutput)) != name {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("tag '%s' does not exist", name),
			Extra: map[string]interface{}{
				"tool":   "git_tag",
				"action": "show",
				"name":   name,
			},
		}
	}

	// Get tag details using git show (includes tagger, date, message, object ref)
	args := []string{"show", name}
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git show %s failed: %s", name, string(output)),
			Extra: map[string]interface{}{
				"tool":   "git_tag",
				"action": "show",
				"name":   name,
			},
		}
	}

	// Parse tag details from the output
	tagType := "tag"
	tagger := ""
	taggerDate := ""
	objectRef := ""
	message := ""

	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")

	for i, line := range lines {
		if strings.HasPrefix(line, "tag ") {
			parts := strings.SplitN(line, " ", 3)
			if len(parts) >= 2 {
				tagType = strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(line, "Tagger:") {
			tagger = strings.TrimSpace(strings.TrimPrefix(line, "Tagger:"))
		}
		if strings.HasPrefix(line, "Date:") {
			taggerDate = strings.TrimSpace(strings.TrimPrefix(line, "Date:"))
		}
		if strings.HasPrefix(line, "Object:") {
			objectRef = strings.TrimSpace(line)
		}
		// Message starts after the blank line following the tag metadata
		if i > 0 && strings.TrimSpace(lines[i-1]) == "" && tagger != "" {
			if message == "" {
				message = strings.TrimSpace(line)
			} else {
				message += "\n" + strings.TrimSpace(line)
			}
		}
	}

	return &ToolResult{
		Success: true,
		Output:  outputStr,
		Extra: map[string]interface{}{
			"tool":       "git_tag",
			"action":     "show",
			"name":       name,
			"type":       tagType,
			"tagger":     tagger,
			"taggerDate": taggerDate,
			"objectRef":  objectRef,
			"message":    strings.TrimSpace(message),
		},
	}
}

// isValidTagName checks if a git tag name is valid.
func isValidTagName(name string) bool {
	if name == "" {
		return false
	}
	// Git tag names cannot contain: spaces, ~, ^, :, ?, *, [, \
	for _, ch := range name {
		switch ch {
		case ' ', '~', '^', ':', '?', '*', '[', '\\':
			return false
		}
	}
	// Cannot start with - or /
	if name[0] == '-' || name[0] == '/' {
		return false
	}
	return true
}

// executeRunBuild executes a build command for the current project and returns structured results.
// Auto-detects project type from common project files (go.mod, package.json, Cargo.toml, etc.).
func (te *ToolExecutor) executeRunBuild(params map[string]interface{}) *ToolResult {
	// Determine build command
	command, hasCommand := params["command"].(string)

	if !hasCommand || command == "" {
		// Auto-detect project type
		command = te.detectBuildCommand()
	}

	if command == "" {
		return &ToolResult{
			Success: false,
			Error:   "no project type detected. Supported project types: Go (go.mod), Node.js (package.json), Rust (Cargo.toml), Java/Maven (pom.xml), Java/Gradle (build.gradle), Python (setup.py, pyproject.toml), Makefile with build target. Provide a custom 'command' parameter to override.",
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

	// Determine timeout (default: 120 seconds / 2 minutes, builds can be slower)
	timeoutSeconds := 120
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
	}

	// Truncate output if too long
	maxOutputLen := 10000
	if len(outputStr) > maxOutputLen {
		outputStr = outputStr[:maxOutputLen] + "\n... [output truncated, exceeded 10000 character limit]"
	}

	// Determine if build succeeded
	passed := exitCode == 0

	result := &ToolResult{
		Success:  passed,
		ExitCode: exitCode,
		Output:   outputStr,
		Extra: map[string]interface{}{
			"tool":    "run_build",
			"passed":  passed,
			"command": command,
		},
	}

	return result
}

// detectBuildCommand auto-detects the build command based on project files.
func (te *ToolExecutor) detectBuildCommand() string {
	cwd, _ := os.Getwd()

	// Check for Go project
	if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
		return "go build ./..."
	}

	// Check for Node.js project
	if _, err := os.Stat(filepath.Join(cwd, "package.json")); err == nil {
		return "npm run build"
	}

	// Check for Rust project
	if _, err := os.Stat(filepath.Join(cwd, "Cargo.toml")); err == nil {
		return "cargo build"
	}

	// Check for Java/Maven project
	if _, err := os.Stat(filepath.Join(cwd, "pom.xml")); err == nil {
		return "mvn compile"
	}

	// Check for Java/Gradle project
	if _, err := os.Stat(filepath.Join(cwd, "build.gradle")); err == nil {
		return "gradle build"
	}
	if _, err := os.Stat(filepath.Join(cwd, "build.gradle.kts")); err == nil {
		return "gradle build"
	}

	// Check for Python project
	if _, err := os.Stat(filepath.Join(cwd, "setup.py")); err == nil {
		return "python -m build"
	}
	if _, err := os.Stat(filepath.Join(cwd, "pyproject.toml")); err == nil {
		return "python -m build"
	}

	// Check for Makefile with build target
	if _, err := os.Stat(filepath.Join(cwd, "Makefile")); err == nil {
		content, _ := os.ReadFile(filepath.Join(cwd, "Makefile"))
		if bytes.Contains(content, []byte("build:")) || bytes.Contains(content, []byte("build :")) {
			return "make build"
		}
	}

	return ""
}

// coverageFile represents coverage data for a single file.
type coverageFile struct {
	Path         string  `json:"path"`
	CoveredLines int     `json:"covered_lines"`
	TotalLines   int     `json:"total_lines"`
	Percentage   float64 `json:"percentage"`
}

// coveragePackage represents coverage data for a package/module.
type coveragePackage struct {
	Name       string  `json:"name"`
	Covered    int     `json:"covered"`
	Total      int     `json:"total"`
	Percentage float64 `json:"percentage"`
}

// coverageReport holds the full coverage report.
type coverageReport struct {
	OverallPercentage  float64            `json:"overall"`
	TotalCovered       int                `json:"total_covered"`
	TotalLines         int                `json:"total_lines"`
	Files              []coverageFile     `json:"files"`
	Packages           []coveragePackage  `json:"packages,omitempty"`
	LowCoverageFiles   []coverageFile     `json:"low_coverage_files"`
	NoCoverageFiles    []coverageFile     `json:"no_coverage_files"`
	Command            string             `json:"command"`
	ExitCode           int                `json:"exit_code"`
}

// executeRunCoverage runs project tests with coverage analysis and returns structured results.
func (te *ToolExecutor) executeRunCoverage(params map[string]interface{}) *ToolResult {
	// Determine coverage command
	command, hasCommand := params["command"].(string)
	customArgs := ""
	if argsParam, hasArgs := params["args"]; hasArgs {
		switch v := argsParam.(type) {
		case []interface{}:
			parts := make([]string, len(v))
			for i, a := range v {
				parts[i] = fmt.Sprintf("%v", a)
			}
			customArgs = strings.Join(parts, " ")
		case string:
			customArgs = v
		}
	}

	if !hasCommand || command == "" {
		// Auto-detect project type
		command, customArgs = te.detectCoverageCommand()
		if command == "" {
			return &ToolResult{
				Success: false,
				Error:   "no project type detected. Supported project types: Go (go.mod), Node.js (package.json), Python (requirements.txt, pyproject.toml). Provide a custom 'command' parameter to override.",
			}
		}
	}

	// Determine timeout (default: 120 seconds)
	timeoutSeconds := 120
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
	if customArgs != "" {
		fullCmd = command + " " + customArgs
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
	}

	// Determine project type for coverage parsing
	projectType := te.detectProjectType()

	// Parse coverage report
	var tempCleanup func()
	report := te.parseCoverageReport(outputStr, projectType, fullCmd, exitCode, &tempCleanup)

	// Clean up temp files after generating report
	if tempCleanup != nil {
		defer tempCleanup()
	}

	// Generate human-readable summary
	summary := te.generateCoverageSummary(report)

	// Truncate raw output if too long
	maxOutputLen := 15000
	if len(outputStr) > maxOutputLen {
		outputStr = outputStr[:maxOutputLen] + "\n... [output truncated, exceeded 15000 character limit]"
	}

	result := &ToolResult{
		Success:  exitCode == 0,
		ExitCode: exitCode,
		Output:   outputStr,
		Extra: map[string]interface{}{
			"tool":             "run_coverage",
			"passed":           exitCode == 0,
			"command":          fullCmd,
			"summary":          summary,
			"overall":          report.OverallPercentage,
			"files":            report.Files,
			"packages":         report.Packages,
			"lowCoverageFiles": report.LowCoverageFiles,
			"noCoverageFiles":  report.NoCoverageFiles,
		},
	}

	return result
}

// detectCoverageCommand auto-detects the coverage command based on project files.
func (te *ToolExecutor) detectCoverageCommand() (string, string) {
	cwd, _ := os.Getwd()

	// Check for Go project
	if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
		return "go test -coverprofile=coverage.out -covermode=count ./...", ""
	}

	// Check for Node.js project
	if _, err := os.Stat(filepath.Join(cwd, "package.json")); err == nil {
		return "npx c8", "--reporter=lcov --reporter=text --"
	}

	// Check for Python project
	if _, err := os.Stat(filepath.Join(cwd, "requirements.txt")); err == nil {
		return "python -m pytest", "--cov=. --cov-report=term-missing --cov-report=json:.coverage.json"
	}
	if _, err := os.Stat(filepath.Join(cwd, "pyproject.toml")); err == nil {
		return "python -m pytest", "--cov=. --cov-report=term-missing --cov-report=json:.coverage.json"
	}
	if _, err := os.Stat(filepath.Join(cwd, "setup.py")); err == nil {
		return "python -m pytest", "--cov=. --cov-report=term-missing --cov-report=json:.coverage.json"
	}

	return "", ""
}

// parseCoverageReport parses coverage output and returns a structured report.
func (te *ToolExecutor) parseCoverageReport(output string, projectType string, command string, exitCode int, tempCleanup *func()) coverageReport {
	report := coverageReport{
		OverallPercentage: 0,
		TotalCovered:      0,
		TotalLines:        0,
		Files:             []coverageFile{},
		Packages:          []coveragePackage{},
		LowCoverageFiles:  []coverageFile{},
		NoCoverageFiles:   []coverageFile{},
		Command:           command,
		ExitCode:          exitCode,
	}

	switch projectType {
	case "go":
		report = te.parseGoCoverage(output, tempCleanup)
	case "node":
		report = te.parseNodeCoverage(output)
	case "python":
		report = te.parsePythonCoverage(output)
	default:
		// Fallback: try to extract any percentage from output
		report = te.parseFallbackCoverage(output)
	}

	// Sort files by percentage (lowest first) for easier review
	sort.Slice(report.Files, func(i, j int) bool {
		return report.Files[i].Percentage < report.Files[j].Percentage
	})

	return report
}

// parseGoCoverage parses Go coverage output from `go tool cover` or cover profile.
func (te *ToolExecutor) parseGoCoverage(output string, tempCleanup *func()) coverageReport {
	report := coverageReport{
		LowCoverageFiles: []coverageFile{},
		NoCoverageFiles:  []coverageFile{},
	}

	// First try parsing go tool cover -func output
	report = te.parseGoCoverFunc(output)

	// If no data from func output, try parsing cover profile file
	if len(report.Files) == 0 {
		report = te.parseGoCoverProfile(tempCleanup)
	}

	// If still no data, try parsing go test -json output
	if len(report.Files) == 0 {
		report = te.parseGoTestJSON(output)
	}

	return report
}

// parseGoCoverFunc parses "go tool cover -func=coverage.out" style output.
func (te *ToolExecutor) parseGoCoverFunc(output string) coverageReport {
	report := coverageReport{
		LowCoverageFiles: []coverageFile{},
		NoCoverageFiles:  []coverageFile{},
	}

	lines := strings.Split(output, "\n")
	var totalCovered, totalLines int

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip header and summary lines
		if strings.HasPrefix(line, "total:") || line == "" {
			// Parse summary line: "total:                                                    of statements: xxx.x%"
			if strings.HasPrefix(line, "total:") {
				pct := te.extractPercentage(line)
				report.OverallPercentage = pct
			}
			continue
		}

		// Parse file/func lines: "path/to/file.go:funcname line.col line.col statements covered total percent"
		parts := strings.Fields(line)
		if len(parts) < 5 {
			continue
		}

		filePath := parts[0]
		if strings.HasPrefix(filePath, "/") || strings.HasPrefix(filePath, "./") {
			// Remove leading path
			if idx := strings.Index(filePath, ":"); idx > 0 {
				filePath = filePath[:idx]
			}
		}

		// Try to extract covered/total from the last numeric fields
		var covered, total int
		for i := len(parts) - 1; i >= 1; i-- {
			if pct := te.extractPercentage(parts[i]); pct > 0 {
				// Find the two numbers before the percentage
				if i >= 3 {
					if c, err := strconv.Atoi(parts[i-2]); err == nil {
						covered = c
					}
					if t, err := strconv.Atoi(parts[i-1]); err == nil {
						total = t
					}
				}
				break
			}
		}

		if total > 0 {
			percentage := float64(covered) / float64(total) * 100
			cf := coverageFile{
				Path:         filePath,
				CoveredLines: covered,
				TotalLines:   total,
				Percentage:   percentage,
			}
			report.Files = append(report.Files, cf)
			totalCovered += covered
			totalLines += total
		}
	}

	report.TotalCovered = totalCovered
	report.TotalLines = totalLines

	if totalLines > 0 && report.OverallPercentage == 0 {
		report.OverallPercentage = float64(totalCovered) / float64(totalLines) * 100
	}

	report.NoCoverageFiles = report.getNoCoverageFiles()
	report.LowCoverageFiles = report.getLowCoverageFiles()

	return report
}

// parseGoCoverProfile parses a Go coverage profile file.
func (te *ToolExecutor) parseGoCoverProfile(tempCleanup *func()) coverageReport {
	report := coverageReport{
		LowCoverageFiles: []coverageFile{},
		NoCoverageFiles:  []coverageFile{},
	}

	// Look for coverage.out file
	coverFile := "coverage.out"
	if _, err := os.Stat(coverFile); os.IsNotExist(err) {
		// Try common names
		for _, name := range []string{"coverage.txt", "cover.out", "out.cover", ".coverage.out"} {
			if _, err := os.Stat(name); err == nil {
				coverFile = name
				break
			}
		}
	}

	if _, err := os.Stat(coverFile); os.IsNotExist(err) {
		return report
	}

	data, err := os.ReadFile(coverFile)
	if err != nil {
		return report
	}

	// Cleanup function
	cleanup := func() { os.Remove(coverFile) }
	*tempCleanup = cleanup

	content := string(data)
	lines := strings.Split(content, "\n")

	// Go cover profile format:
	// mode: count
	// path/to/file.go:line.col.line.col numStatements numCovered
	// Or: mode: set
	// path/to/file.go:line.col.1 line.col.num

	fileData := make(map[string]*goFileCover)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}

		// Parse: path/to/file.go:line.col.line.col numStatements numCovered
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}

		filePath := line[:colonIdx]
		rest := strings.TrimSpace(line[colonIdx+1:])

		// Split by space to get statement count and covered count
		parts := strings.Fields(rest)
		if len(parts) < 2 {
			continue
		}

		total, err1 := strconv.Atoi(parts[0])
		covered, err2 := strconv.Atoi(parts[1])

		if err1 != nil || err2 != nil {
			continue
		}

		if existing, ok := fileData[filePath]; ok {
			existing.Total += total
			existing.Covered += covered
		} else {
			fileData[filePath] = &goFileCover{
				Total:   total,
				Covered: covered,
			}
		}
	}

	var totalCovered, totalLines int
	for path, fc := range fileData {
		percentage := 0.0
		if fc.Total > 0 {
			percentage = float64(fc.Covered) / float64(fc.Total) * 100
		}

		cf := coverageFile{
			Path:         path,
			CoveredLines: fc.Covered,
			TotalLines:   fc.Total,
			Percentage:   percentage,
		}
		report.Files = append(report.Files, cf)
		totalCovered += fc.Covered
		totalLines += fc.Total
	}

	report.TotalCovered = totalCovered
	report.TotalLines = totalLines
	if totalLines > 0 {
		report.OverallPercentage = float64(totalCovered) / float64(totalLines) * 100
	}

	report.NoCoverageFiles = report.getNoCoverageFiles()
	report.LowCoverageFiles = report.getLowCoverageFiles()

	return report
}

// goFileCover tracks coverage for a single Go file.
type goFileCover struct {
	Total   int
	Covered int
}

// parseGoTestJSON parses "go test -json" output for coverage data.
func (te *ToolExecutor) parseGoTestJSON(output string) coverageReport {
	report := coverageReport{
		LowCoverageFiles: []coverageFile{},
		NoCoverageFiles:  []coverageFile{},
	}

	lines := strings.Split(output, "\n")
	var totalCovered, totalLines int

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for Action: "output" lines that contain coverage
		if !strings.Contains(line, `"Action":"output"`) && !strings.Contains(line, `"Action": "output"`) {
			continue
		}

		// Try to extract package stats from output
		// Format: ok  \tgithub.com/user/project/pkg\t2.345s\tcoverage: XX.X% of statements
		var covered, total int

		// Try to parse "X.XX of statements" pattern
		statPattern := `([0-9]+)\.([0-9]+)\s*of\s*statements`
		re := regexp.MustCompile(statPattern)
		matches := re.FindStringSubmatch(line)
		if len(matches) >= 3 {
			covered, _ = strconv.Atoi(matches[1])
			total, _ = strconv.Atoi(matches[2])
		}

		if total > 0 {
			// Extract package name from the line
			pkgPattern := `(\S+)\s+coverage:`
			pkgRe := regexp.MustCompile(pkgPattern)
			pkgMatches := pkgRe.FindStringSubmatch(line)
			pkgName := "unknown"
			if len(pkgMatches) > 1 {
				pkgName = pkgMatches[1]
			}

			cf := coverageFile{
				Path:         pkgName,
				CoveredLines: covered,
				TotalLines:   total,
				Percentage:   float64(covered) / float64(total) * 100,
			}
			report.Files = append(report.Files, cf)
			totalCovered += covered
			totalLines += total
		}
	}

	report.TotalCovered = totalCovered
	report.TotalLines = totalLines
	if totalLines > 0 {
		report.OverallPercentage = float64(totalCovered) / float64(totalLines) * 100
	}

	report.NoCoverageFiles = report.getNoCoverageFiles()
	report.LowCoverageFiles = report.getLowCoverageFiles()

	return report
}

// parseNodeCoverage parses Node.js coverage output (from c8, nyc, or istanbul).
func (te *ToolExecutor) parseNodeCoverage(output string) coverageReport {
	report := coverageReport{
		LowCoverageFiles: []coverageFile{},
		NoCoverageFiles:  []coverageFile{},
	}

	// Try to find coverage table in output
	lines := strings.Split(output, "\n")
	var totalCovered, totalLines int

	for _, line := range lines {
		// Skip separator lines
		if strings.HasPrefix(line, "---") {
			continue
		}

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Parse data lines - use regex to find percentage patterns
		re := regexp.MustCompile(`(\S+)\s*\|\s*([0-9.]+)%\s*\|`)
		matches := re.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}

		filePath := matches[1]
		percentage, err := strconv.ParseFloat(matches[2], 64)
		if err != nil {
			continue
		}

		// Skip summary lines
		if strings.HasPrefix(filePath, "Total") || strings.HasPrefix(filePath, "----------") || filePath == "File" || percentage == 100.0 && strings.HasPrefix(filePath, "Total") {
			continue
		}

		// We don't have exact line counts from c8 text output, estimate
		// Estimate total lines based on uncovered line ranges
		uncoveredRe := regexp.MustCompile(`\|.*\| (.+)$`)
		uncoveredMatches := uncoveredRe.FindStringSubmatch(line)
		estimatedTotal := 20 // Default estimate

		if len(uncoveredMatches) > 1 {
			uncoveredStr := uncoveredMatches[1]
			// Parse uncovered line numbers like "15,23-25,30"
			estimatedTotal = te.estimateTotalLines(uncoveredStr)
		}

		coveredLines := int(float64(estimatedTotal) * percentage / 100)

		cf := coverageFile{
			Path:         filePath,
			CoveredLines: coveredLines,
			TotalLines:   estimatedTotal,
			Percentage:   percentage,
		}
		report.Files = append(report.Files, cf)
		totalCovered += coveredLines
		totalLines += estimatedTotal
	}

	report.TotalCovered = totalCovered
	report.TotalLines = totalLines
	if totalLines > 0 {
		report.OverallPercentage = float64(totalCovered) / float64(totalLines) * 100
	}

	report.NoCoverageFiles = report.getNoCoverageFiles()
	report.LowCoverageFiles = report.getLowCoverageFiles()

	return report
}

// estimateTotalLines estimates total lines from uncovered line specifications.
func (te *ToolExecutor) estimateTotalLines(uncoveredStr string) int {
	total := 0
	for _, part := range strings.Split(uncoveredStr, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) == 2 {
				start, err1 := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
				end, err2 := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
				if err1 == nil && err2 == nil {
					total += end - start + 1
				}
			}
		} else {
			if _, err := strconv.Atoi(part); err == nil {
				total++
			}
		}
	}
	// At minimum, add uncovered lines; add a reasonable total
	if total > 0 {
		return total + 5 // Assume some covered lines exist
	}
	return 10
}

// parsePythonCoverage parses Python coverage output.
func (te *ToolExecutor) parsePythonCoverage(output string) coverageReport {
	report := coverageReport{
		LowCoverageFiles: []coverageFile{},
		NoCoverageFiles:  []coverageFile{},
	}

	// First try parsing JSON coverage file
	jsonReport := tryParsePythonJSONCoverage()
	if len(jsonReport.Files) > 0 {
		return jsonReport
	}

	// If no JSON data, try parsing term-missing output
	report = te.parsePythonTermMissing(output)

	return report
}

// tryParsePythonJSONCoverage tries to parse pytest-cov JSON output.
func tryParsePythonJSONCoverage() coverageReport {
	report := coverageReport{
		LowCoverageFiles: []coverageFile{},
		NoCoverageFiles:  []coverageFile{},
	}

	// Look for .coverage.json file
	jsonFile := ".coverage.json"
	if _, err := os.Stat(jsonFile); os.IsNotExist(err) {
		return report
	}

	data, err := os.ReadFile(jsonFile)
	if err != nil {
		return report
	}

	// Cleanup
	cleanup := func() { os.Remove(jsonFile) }
	tempCleanup := cleanup
	tempCleanup()

	var coverageData struct {
		Files map[string]struct {
			Stats struct {
				TotalLines     int     `json:"num_statements"`
				CoveredLines   int     `json:"num_executed"`
				PercentCovered float64 `json:"percent_covered"`
			} `json:"missing"`
		} `json:"files"`
		Totals struct {
			TotalStatements int     `json:"num_statements"`
			TotalExecuted   int     `json:"num_executed"`
			PctCovered      float64 `json:"percent_covered"`
		} `json:"totals"`
	}

	if err := json.Unmarshal(data, &coverageData); err != nil {
		return report
	}

	var totalCovered, totalLines int
	for path, fileData := range coverageData.Files {
		total := fileData.Stats.TotalLines
		covered := fileData.Stats.CoveredLines
		pct := fileData.Stats.PercentCovered

		if total == 0 && pct > 0 {
			total = int(float64(covered) / (pct / 100))
		}

		cf := coverageFile{
			Path:         path,
			CoveredLines: covered,
			TotalLines:   total,
			Percentage:   pct,
		}
		report.Files = append(report.Files, cf)
		totalCovered += covered
		totalLines += total
	}

	report.TotalCovered = totalCovered
	report.TotalLines = totalLines
	if coverageData.Totals.TotalStatements > 0 {
		report.OverallPercentage = coverageData.Totals.PctCovered
	} else if totalLines > 0 {
		report.OverallPercentage = float64(totalCovered) / float64(totalLines) * 100
	}

	report.NoCoverageFiles = report.getNoCoverageFiles()
	report.LowCoverageFiles = report.getLowCoverageFiles()

	return report
}

// parsePythonTermMissing parses pytest-cov term-missing output.
func (te *ToolExecutor) parsePythonTermMissing(output string) coverageReport {
	report := coverageReport{
		LowCoverageFiles: []coverageFile{},
		NoCoverageFiles:  []coverageFile{},
	}

	lines := strings.Split(output, "\n")
	fileData := make(map[string]*pythonFileCover)
	var totalCovered, totalLines int

	// Skip header lines until we find data
	inDataSection := false
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for the summary line: "NAME                      STMTS   MISS  COVER   MISSING"
		if strings.Contains(line, "STMTS") && strings.Contains(line, "MISS") {
			inDataSection = true
			continue
		}

		// Look for Total line
		if strings.HasPrefix(line, "TOTAL") || strings.HasPrefix(line, "total") {
			inDataSection = false
			continue
		}

		// Skip non-data lines
		if !inDataSection || strings.HasPrefix(line, "Name") || strings.HasPrefix(line, "---") || line == "" {
			continue
		}

		// Parse data line: "name.py                    100      10    90%   15-20, 25"
		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}

		filePath := parts[0]
		total, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}

		miss, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}

		covered := total - miss
		_ = strings.TrimSuffix(parts[3], "%") // Parse percentage for validation

		fileData[filePath] = &pythonFileCover{
			Total:   total,
			Covered: covered,
		}
		totalCovered += covered
		totalLines += total
	}

	var totalCovered2, totalLines2 int
	for path, fc := range fileData {
		percentage := 0.0
		if fc.Total > 0 {
			percentage = float64(fc.Covered) / float64(fc.Total) * 100
		}

		cf := coverageFile{
			Path:         path,
			CoveredLines: fc.Covered,
			TotalLines:   fc.Total,
			Percentage:   percentage,
		}
		report.Files = append(report.Files, cf)
		totalCovered2 += fc.Covered
		totalLines2 += fc.Total
	}

	report.TotalCovered = totalCovered2
	report.TotalLines = totalLines2
	if totalLines2 > 0 {
		report.OverallPercentage = float64(totalCovered2) / float64(totalLines2) * 100
	}

	report.NoCoverageFiles = report.getNoCoverageFiles()
	report.LowCoverageFiles = report.getLowCoverageFiles()

	return report
}

// pythonFileCover tracks coverage for a single Python file.
type pythonFileCover struct {
	Total   int
	Covered int
}

// parseFallbackCoverage attempts to parse coverage from arbitrary output.
func (te *ToolExecutor) parseFallbackCoverage(output string) coverageReport {
	report := coverageReport{
		LowCoverageFiles: []coverageFile{},
		NoCoverageFiles:  []coverageFile{},
	}

	// Try to extract any percentage from the output
	if pct := te.extractPercentage(output); pct > 0 {
		report.OverallPercentage = pct
	}

	return report
}

// extractPercentage extracts a percentage value from a string.
func (te *ToolExecutor) extractPercentage(s string) float64 {
	re := regexp.MustCompile(`([0-9]+\.?[0-9]*)\s*%`)
	matches := re.FindStringSubmatch(s)
	if len(matches) >= 2 {
		pct, err := strconv.ParseFloat(matches[1], 64)
		if err == nil {
			return pct
		}
	}
	return 0
}

// getNoCoverageFiles returns files with 0% coverage.
func (r coverageReport) getNoCoverageFiles() []coverageFile {
	var noCov []coverageFile
	for _, f := range r.Files {
		if f.CoveredLines == 0 && f.TotalLines > 0 {
			noCov = append(noCov, f)
		}
	}
	return noCov
}

// getLowCoverageFiles returns files with < 50% coverage.
func (r coverageReport) getLowCoverageFiles() []coverageFile {
	var lowCov []coverageFile
	for _, f := range r.Files {
		if f.Percentage > 0 && f.Percentage < 50 {
			lowCov = append(lowCov, f)
		}
	}
	return lowCov
}

// detectProjectType detects the project type from common files.
func (te *ToolExecutor) detectProjectType() string {
	cwd, _ := os.Getwd()

	if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
		return "go"
	}
	if _, err := os.Stat(filepath.Join(cwd, "package.json")); err == nil {
		return "node"
	}
	if _, err := os.Stat(filepath.Join(cwd, "requirements.txt")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(cwd, "pyproject.toml")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(cwd, "setup.py")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(cwd, "Cargo.toml")); err == nil {
		return "rust"
	}

	return ""
}

// generateCoverageSummary generates a human-readable summary of the coverage report.
func (te *ToolExecutor) generateCoverageSummary(report coverageReport) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Coverage: %.1f%% (%d/%d lines)\n", report.OverallPercentage, report.TotalCovered, report.TotalLines))

	if report.TotalLines > 0 {
		// Rating
		switch {
		case report.OverallPercentage >= 90:
			sb.WriteString("Rating: Excellent\n")
		case report.OverallPercentage >= 75:
			sb.WriteString("Rating: Good\n")
		case report.OverallPercentage >= 50:
			sb.WriteString("Rating: Fair\n")
		default:
			sb.WriteString("Rating: Poor\n")
		}
	}

	if len(report.NoCoverageFiles) > 0 {
		sb.WriteString(fmt.Sprintf("\n⚠ %d file(s) with NO coverage:\n", len(report.NoCoverageFiles)))
		for _, f := range report.NoCoverageFiles {
			sb.WriteString(fmt.Sprintf("  - %s (0%%)\n", f.Path))
		}
	}

	if len(report.LowCoverageFiles) > 0 {
		sb.WriteString(fmt.Sprintf("\n⚠ %d file(s) with LOW coverage (<50%%):\n", len(report.LowCoverageFiles)))
		for _, f := range report.LowCoverageFiles {
			sb.WriteString(fmt.Sprintf("  - %s (%.1f%%)\n", f.Path, f.Percentage))
		}
	}

	if report.ExitCode != 0 {
		sb.WriteString(fmt.Sprintf("\nTests exited with code %d\n", report.ExitCode))
	}

	return sb.String()
}

// executeGitMerge handles git merge operations: merge, abort, status, squash, merge_pr.
func (te *ToolExecutor) executeGitMerge(params map[string]interface{}) *ToolResult {
	action, ok := params["action"].(string)
	if !ok || action == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: action",
		}
	}

	switch action {
	case "merge":
		return te.gitMergeMerge(params)
	case "abort":
		return te.gitMergeAbort(params)
	case "status":
		return te.gitMergeStatus(params)
	case "squash":
		return te.gitMergeSquash(params)
	case "merge_pr":
		return te.gitMergePR(params)
	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown merge action: %s. Valid actions: merge, abort, status, squash, merge_pr", action),
		}
	}
}

// gitMergeMerge performs a standard git merge of source into the current branch.
func (te *ToolExecutor) gitMergeMerge(params map[string]interface{}) *ToolResult {
	source, ok := params["source"].(string)
	if !ok || source == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: source (branch to merge)",
		}
	}

	target, ok := params["target"].(string)
	if !ok || target == "" {
		target = "HEAD"
	}

	commitMsg, hasCommitMsg := params["commit_message"]

	// First, checkout target branch if different from current
	if target != "HEAD" {
		// Check if we need to checkout
		checkoutCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		currentBranch, err := checkoutCmd.Output()
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to determine current branch: %v", err),
			}
		}
		if strings.TrimSpace(string(currentBranch)) != target {
			// Check if there are uncommitted changes
			statusCmd := exec.Command("git", "diff-index", "--quiet", "HEAD", "--")
			if statusCmd.Run() != nil {
				return &ToolResult{
					Success: false,
					Error:   fmt.Sprintf("cannot checkout '%s': you have uncommitted changes. Commit or stash them first.", target),
				}
			}
			// Checkout target branch
			checkoutCmd := exec.Command("git", "checkout", target)
			checkoutOutput, checkoutErr := checkoutCmd.CombinedOutput()
			if checkoutErr != nil {
				return &ToolResult{
					Success: false,
					Error:   fmt.Sprintf("failed to checkout branch '%s': %s", target, string(checkoutOutput)),
				}
			}
		}
		target = "HEAD"
	}

	// Verify source branch exists
	verifyCmd := exec.Command("git", "rev-parse", "--verify", source)
	if verifyCmd.Run() != nil {
		output, _ := verifyCmd.CombinedOutput()
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("source branch '%s' does not exist: %s", source, string(output)),
		}
	}

	// Check if the target branch has uncommitted changes
	statusCmd := exec.Command("git", "diff-index", "--quiet", "HEAD", "--")
	if statusCmd.Run() != nil {
		diffOutput, _ := statusCmd.CombinedOutput()
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("cannot merge: you have uncommitted changes. Commit or stash them first.\nDifferences:\n%s", string(diffOutput)),
		}
	}

	// Perform the merge
	args := []string{"merge", "--no-edit", source}

	// Use custom commit message if provided
	if hasCommitMsg {
		msgStr, _ := commitMsg.(string)
		if msgStr != "" {
			args = []string{"merge", "-m", msgStr, source}
		}
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if this is a merge conflict
		if strings.Contains(string(output), "CONFLICT") || strings.Contains(string(output), "has local changes") {
			// Get conflicting files
			conflictFiles := te.getConflictingFiles()
			conflictSummary := ""
			if len(conflictFiles) > 0 {
				conflictSummary = "\nConflicting files:\n"
				for _, f := range conflictFiles {
					conflictSummary += fmt.Sprintf("  - %s\n", f)
				}
			}
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("merge conflict: %s%s", string(output), conflictSummary),
				Extra: map[string]interface{}{
					"conflict":    true,
					"conflicts":   conflictFiles,
					"merge_in_progress": true,
				},
			}
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("merge failed: %s", string(output)),
		}
	}

	// Get merge commit hash
	commitCmd := exec.Command("git", "rev-parse", "HEAD")
	commitHash, _ := commitCmd.Output()

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Successfully merged '%s' into current branch\nCommit: %s", source, strings.TrimSpace(string(commitHash))),
		Extra: map[string]interface{}{
			"tool":      "git_merge",
			"action":    "merge",
			"source":    source,
			"target":    target,
			"commitHash": strings.TrimSpace(string(commitHash)),
		},
	}
}

// gitMergeAbort aborts an in-progress merge.
func (te *ToolExecutor) gitMergeAbort(params map[string]interface{}) *ToolResult {
	// Check if a merge is in progress
	mergeHeadPath := ".git/MERGE_HEAD"
	if _, err := os.Stat(mergeHeadPath); os.IsNotExist(err) {
		return &ToolResult{
			Success: false,
			Error:   "no merge in progress. MERGE_HEAD not found.",
		}
	}

	cmd := exec.Command("git", "merge", "--abort")
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to abort merge: %s", string(output)),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  "Merge aborted successfully. Working tree restored to pre-merge state.",
		Extra: map[string]interface{}{
			"tool":      "git_merge",
			"action":    "abort",
			"message":   "Merge aborted successfully",
		},
	}
}

// gitMergeStatus checks if a merge is in progress and lists conflicting files.
func (te *ToolExecutor) gitMergeStatus(params map[string]interface{}) *ToolResult {
	// Check if a merge is in progress
	mergeHeadPath := ".git/MERGE_HEAD"
	if _, err := os.Stat(mergeHeadPath); os.IsNotExist(err) {
		return &ToolResult{
			Success: true,
			Output:  "No merge in progress.",
			Extra: map[string]interface{}{
				"tool":           "git_merge",
				"action":         "status",
				"merge_in_progress": false,
				"conflicts":      []string{},
			},
		}
	}

	// Get conflicting files
	conflictFiles := te.getConflictingFiles()

	// Get the source branch being merged
	sourceBranch := "unknown"
	mergeHeadContent, _ := os.ReadFile(mergeHeadPath)
	mergeHeadHash := strings.TrimSpace(string(mergeHeadContent))

	// Try to find the branch name for this commit
	branchCmd := exec.Command("git", "branch", "--contains", mergeHeadHash)
	branchOutput, _ := branchCmd.CombinedOutput()
	branches := strings.Split(strings.TrimSpace(string(branchOutput)), "\n")
	for _, b := range branches {
		b = strings.TrimSpace(b)
		if b != "" && !strings.Contains(b, "*") {
			sourceBranch = strings.TrimPrefix(b, "  ")
			break
		} else if b != "" && strings.Contains(b, "*") {
			// This is the current branch, skip
			continue
		}
	}

	// If no non-current branch found, try get-branch
	if sourceBranch == "unknown" || sourceBranch == "" {
		// Fallback: try to get from reflog
		reflogCmd := exec.Command("git", "reflog", "show", "--format=%gs", "-1")
		reflogOutput, _ := reflogCmd.CombinedOutput()
		reflogStr := string(reflogOutput)
		// Parse "merge <branch>: Merge commit message"
		if strings.HasPrefix(reflogStr, "merge ") {
			parts := strings.SplitN(reflogStr[6:], ":", 2)
			if len(parts) > 0 {
				sourceBranch = strings.TrimSpace(parts[0])
			}
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Merge in progress: %s into current branch.\nConflicts: %d file(s)", sourceBranch, len(conflictFiles)),
		Extra: map[string]interface{}{
			"tool":              "git_merge",
			"action":            "status",
			"merge_in_progress": true,
			"source_branch":     sourceBranch,
			"merge_head":        mergeHeadHash,
			"conflicts":         conflictFiles,
			"conflictCount":     len(conflictFiles),
		},
	}
}

// gitMergeSquash performs a squash merge of source into the current branch.
func (te *ToolExecutor) gitMergeSquash(params map[string]interface{}) *ToolResult {
	source, ok := params["source"].(string)
	if !ok || source == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: source (branch to squash merge)",
		}
	}

	target, ok := params["target"].(string)
	if !ok || target == "" {
		target = "HEAD"
	}

	// Verify source branch exists
	verifyCmd := exec.Command("git", "rev-parse", "--verify", source)
	if verifyCmd.Run() != nil {
		output, _ := verifyCmd.CombinedOutput()
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("source branch '%s' does not exist: %s", source, string(output)),
		}
	}

	// Checkout target branch if different from current
	if target != "HEAD" {
		checkoutCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		currentBranch, err := checkoutCmd.Output()
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to determine current branch: %v", err),
			}
		}
		if strings.TrimSpace(string(currentBranch)) != target {
			statusCmd := exec.Command("git", "diff-index", "--quiet", "HEAD", "--")
			if statusCmd.Run() != nil {
				return &ToolResult{
					Success: false,
					Error:   fmt.Sprintf("cannot checkout '%s': you have uncommitted changes. Commit or stash them first.", target),
				}
			}
			checkoutCmd := exec.Command("git", "checkout", target)
			checkoutOutput, checkoutErr := checkoutCmd.CombinedOutput()
			if checkoutErr != nil {
				return &ToolResult{
					Success: false,
					Error:   fmt.Sprintf("failed to checkout branch '%s': %s", target, string(checkoutOutput)),
				}
			}
		}
		target = "HEAD"
	}

	// Check for uncommitted changes
	statusCmd := exec.Command("git", "diff-index", "--quiet", "HEAD", "--")
	if statusCmd.Run() != nil {
		diffOutput, _ := statusCmd.CombinedOutput()
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("cannot squash merge: you have uncommitted changes.\nDifferences:\n%s", string(diffOutput)),
		}
	}

	// Perform squash merge
	cmd := exec.Command("git", "merge", "--squash", source)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Clean up the squash state
		exec.Command("git", "merge", "--abort").Run()
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("squash merge failed: %s", string(output)),
		}
	}

	// Check if there are actually changes to commit
	statusCmd = exec.Command("git", "diff-index", "--quiet", "--cached", "HEAD", "--")
	if statusCmd.Run() != nil {
		// No changes - clean up
		exec.Command("git", "merge", "--abort").Run()
		return &ToolResult{
			Success: true,
			Output:  "Squash merge completed but no changes to commit. The source branch was already fully merged.",
			Extra: map[string]interface{}{
				"tool":            "git_merge",
				"action":          "squash",
				"source":          source,
				"target":          target,
				"noChangesToCommit": true,
			},
		}
	}

	// Commit the squashed changes
	commitMsg, hasCommitMsg := params["commit_message"]
	args := []string{"commit", "-m", "Squash merge of branch '" + source + "'"}
	if hasCommitMsg {
		msgStr, _ := commitMsg.(string)
		if msgStr != "" {
			args = []string{"commit", "-m", msgStr}
		}
	}
	commitCmd := exec.Command("git", args...)
	commitOutput, commitErr := commitCmd.CombinedOutput()
	if commitErr != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to commit squashed changes: %s", string(commitOutput)),
		}
	}

	// Get commit hash
	revCmd := exec.Command("git", "rev-parse", "HEAD")
	commitHash, _ := revCmd.Output()

	// Clean up squash state
	exec.Command("git", "reset", "HEAD").Run()
	exec.Command("git", "checkout", "--", ".").Run()

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Squash merge of '%s' completed.\nSquashed into single commit: %s", source, strings.TrimSpace(string(commitHash))),
		Extra: map[string]interface{}{
			"tool":         "git_merge",
			"action":       "squash",
			"source":       source,
			"target":       target,
			"commitHash":   strings.TrimSpace(string(commitHash)),
			"message":      "Squash merge completed successfully",
		},
	}
}

// gitMergePR merges a GitHub pull request using the GitHub API.
func (te *ToolExecutor) gitMergePR(params map[string]interface{}) *ToolResult {
	// Get parameters
	token, ok := params["github_token"].(string)
	if !ok || token == "" {
		// Try environment variable
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		token = os.Getenv("GITHUB_API_TOKEN")
	}
	if token == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: github_token (or set GITHUB_TOKEN environment variable)",
		}
	}

	repo, ok := params["repo"].(string)
	if !ok || repo == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: repo (format: owner/repo)",
		}
	}

	prNumFloat, okFloat := params["pr_number"].(float64)
	prNumIntRaw, okInt := params["pr_number"].(int)
	if !okFloat && !okInt {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: pr_number",
		}
	}
	var prNumber float64
	if okFloat {
		prNumber = prNumFloat
	} else {
		prNumber = float64(prNumIntRaw)
	}

	mergeMethod, ok := params["merge_method"].(string)
	if !ok || mergeMethod == "" {
		mergeMethod = "merge"
	}

	// Validate repo format
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid repo format '%s': expected 'owner/repo'", repo),
		}
	}

	// Validate merge method
	validMethods := map[string]bool{
		"merge":  true,
		"squash": true,
		"rebase": true,
	}
	if !validMethods[mergeMethod] {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid merge method: %s. Valid methods: merge, squash, rebase", mergeMethod),
		}
	}

	// Validate PR number
	prNumInt := int(prNumber)
	if prNumInt <= 0 {
		return &ToolResult{
			Success: false,
			Error:   "pr_number must be a positive integer",
		}
	}

	// Build the API URL
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%d/merge", repo, prNumInt)

	// Create request body
	reqBody := map[string]interface{}{
		"merge_method": mergeMethod,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Create HTTP request
	req, err := http.NewRequest("PUT", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create request: %v", err),
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "coding-agent")

	// Execute request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to merge PR: %v", err),
		}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// Handle response
	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		// Parse response for merge commit SHA
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		sha := ""
		if s, ok := result["sha"].(string); ok {
			sha = s
		}

		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Successfully merged PR #%d into %s (method: %s)\nMerge commit: %s", prNumInt, repo, mergeMethod, sha),
			Extra: map[string]interface{}{
				"tool":         "git_merge",
				"action":       "merge_pr",
				"pr_number":    prNumInt,
				"repo":         repo,
				"merge_method": mergeMethod,
				"sha":          sha,
			},
		}
	} else if resp.StatusCode == 403 {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("GitHub API returned 403: check your token permissions. Response: %s", string(respBody)),
			Extra: map[string]interface{}{
				"tool":         "git_merge",
				"action":       "merge_pr",
				"pr_number":    prNumInt,
				"repo":         repo,
				"http_status":  resp.StatusCode,
				"response":     string(respBody),
			},
		}
	} else if resp.StatusCode == 405 {
		// Merge method not allowed
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("GitHub API returned 405: merge method '%s' not allowed for this PR. Response: %s", mergeMethod, string(respBody)),
			Extra: map[string]interface{}{
				"tool":         "git_merge",
				"action":       "merge_pr",
				"pr_number":    prNumInt,
				"repo":         repo,
				"merge_method": mergeMethod,
				"http_status":  resp.StatusCode,
				"response":     string(respBody),
			},
		}
	} else {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("GitHub API error (status %d): %s", resp.StatusCode, string(respBody)),
			Extra: map[string]interface{}{
				"tool":         "git_merge",
				"action":       "merge_pr",
				"pr_number":    prNumInt,
				"repo":         repo,
				"merge_method": mergeMethod,
				"http_status":  resp.StatusCode,
				"response":     string(respBody),
			},
		}
	}
}

// getConflictingFiles returns a list of files with merge conflicts.
func (te *ToolExecutor) getConflictingFiles() []string {
	var files []string

	// Method 1: Use git diff --name-only --diff-filter=U
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				files = append(files, line)
			}
		}
		if len(files) > 0 {
			return files
		}
	}

	// Method 2: Search for conflict markers in tracked files
	cmd = exec.Command("git", "ls-files")
	output, err = cmd.Output()
	if err != nil {
		return files
	}

	fileList := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, filePath := range fileList {
		filePath = strings.TrimSpace(filePath)
		if filePath == "" {
			continue
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		// Check for conflict markers
		contentStr := string(content)
		if strings.Contains(contentStr, "<<<<<<< ") && strings.Contains(contentStr, "=======") && strings.Contains(contentStr, ">>>>>>> ") {
			files = append(files, filePath)
		}
	}

	return files
}

// executeGenerateDocs generates documentation for code files.
func (te *ToolExecutor) executeGenerateDocs(params map[string]interface{}) *ToolResult {
	path, ok := params["path"].(string)
	if !ok || path == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	// Optional: output format (default: markdown)
	format := "markdown"
	if f, hasFormat := params["format"]; hasFormat {
		if fStr, ok := f.(string); ok && fStr != "" {
			format = fStr
		}
	}

	// Optional: detail level (default: detailed)
	detail := "detailed"
	if d, hasDetail := params["detail"]; hasDetail {
		if dStr, ok := d.(string); ok && dStr != "" {
			detail = dStr
		}
	}

	// Optional: include comments from source (default: true)
	includeComments := true
	if ic, hasInclude := params["include_comments"]; hasInclude {
		if icBool, ok := ic.(bool); ok {
			includeComments = icBool
		}
	}

	// Check if path is a file or directory
	info, err := os.Stat(path)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("cannot access path: %v", err),
		}
	}

	if info.IsDir() {
		return te.generateDocsForDirectory(path, format, detail, includeComments)
	}
	return te.generateDocsForFile(path, format, detail, includeComments)
}

// generateDocsForFile generates documentation for a single file.
func (te *ToolExecutor) generateDocsForFile(filePath, format, detail string, includeComments bool) *ToolResult {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read file: %v", err),
		}
	}

	lang := detectLanguage(filePath)
	doc, err := generateDocumentation(string(content), lang, format, detail, includeComments, filePath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to generate documentation: %v", err),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  doc,
		Path:    filePath,
		Extra: map[string]interface{}{
			"tool":            "generate_docs",
			"language":        lang,
			"format":          format,
			"detail":          detail,
			"contentLength":   len(content),
			"docLength":       len(doc),
			"includeComments": includeComments,
		},
	}
}

// generateDocsForDirectory generates documentation for all files in a directory.
func (te *ToolExecutor) generateDocsForDirectory(dirPath, format, detail string, includeComments bool) *ToolResult {
	var results []string
	var filesProcessed int
	var errors []string

	// Collect all source files
	sourceFiles, err := collectSourceFiles(dirPath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to scan directory: %v", err),
		}
	}

	for _, filePath := range sourceFiles {
		content, err := os.ReadFile(filePath)
		if err != nil {
			errors = append(errors, fmt.Sprintf("  %s: %v", filePath, err))
			continue
		}

		lang := detectLanguage(filePath)
		doc, err := generateDocumentation(string(content), lang, format, detail, includeComments, filePath)
		if err != nil {
			errors = append(errors, fmt.Sprintf("  %s: %v", filePath, err))
			continue
		}

		results = append(results, fmt.Sprintf("### %s\n\n%s", filePath, doc))
		filesProcessed++
	}

	// Build output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Generated documentation for %d file(s) in '%s':\n\n", filesProcessed, dirPath))

	for _, r := range results {
		output.WriteString(r)
		output.WriteString("\n\n---\n\n")
	}

	if len(errors) > 0 {
		output.WriteString(fmt.Sprintf("\n**Errors (%d):**\n", len(errors)))
		for _, e := range errors {
			output.WriteString(e + "\n")
		}
	}

	return &ToolResult{
		Success: true,
		Output:  output.String(),
		Path:    dirPath,
		Extra: map[string]interface{}{
			"tool":           "generate_docs",
			"format":         format,
			"detail":         detail,
			"filesProcessed": filesProcessed,
			"errors":         errors,
		},
	}
}

// collectSourceFiles recursively collects all source files in a directory.
func collectSourceFiles(dirPath string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(dirPath, func(walkPath string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip directories
		if d.IsDir() {
			// Skip hidden directories and common non-source dirs
			base := d.Name()
			if base == ".git" || base == "vendor" || base == "node_modules" || base == ".DS_Store" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only include source files
		ext := strings.ToLower(filepath.Ext(walkPath))
		supportedExts := map[string]bool{
			".go": true, ".py": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
			".java": true, ".rs": true, ".c": true, ".h": true, ".cpp": true, ".hpp": true,
			".rb": true, ".php": true, ".cs": true, ".swift": true, ".kt": true,
		}
		if supportedExts[ext] {
			files = append(files, walkPath)
		}

		return nil
	})

	return files, err
}

// detectLanguage detects the programming language from file extension.
func detectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".c", ".h":
		return "c"
	case ".cpp", ".hpp":
		return "cpp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".cs":
		return "csharp"
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	default:
		return "generic"
	}
}

// docFunction represents a function/method signature extracted from source.
type docFunction struct {
	Name        string
	Params      string
	ReturnType  string
	Comment     string
	Description string
}

// docType represents a type/struct/class definition.
type docType struct {
	Name        string
	TypeKind    string // struct, class, interface, enum, type
	Fields      string
	Comment     string
	Description string
}

// generateDocumentation generates documentation for a source file.
func generateDocumentation(content, lang, format, detail string, includeComments bool, filePath string) (string, error) {
	// Extract symbols and comments from source
	funcs := extractFunctions(content, lang)
	types := extractTypes(content, lang)

	if format == "markdown" {
		return formatMarkdown(content, lang, filePath, funcs, types, detail, includeComments)
	} else if format == "inline" {
		return formatInline(content, lang, funcs, types, detail, includeComments)
	}

	return "", fmt.Errorf("unsupported format: %s (supported: markdown, inline)", format)
}

// extractFunctions extracts function and method signatures from source code.
func extractFunctions(content, lang string) []docFunction {
	var funcs []docFunction

	lines := strings.Split(content, "\n")

	// Language-specific extraction
	switch lang {
	case "go":
		funcs = extractGoFunctions(lines)
	case "python":
		funcs = extractPythonFunctions(lines)
	case "javascript", "typescript":
		funcs = extractJSFunctions(lines, lang)
	case "java":
		funcs = extractJavaFunctions(lines)
	case "rust":
		funcs = extractRustFunctions(lines)
	default:
		// Generic: look for common function patterns
		funcs = extractGenericFunctions(lines, lang)
	}

	return funcs
}

// extractTypes extracts type/struct/class definitions from source code.
func extractTypes(content, lang string) []docType {
	var types []docType

	lines := strings.Split(content, "\n")

	switch lang {
	case "go":
		types = extractGoTypes(lines)
	case "python":
		types = extractPythonTypes(lines)
	case "javascript", "typescript":
		types = extractJSTypes(lines, lang)
	case "java":
		types = extractJavaTypes(lines)
	case "rust":
		types = extractRustTypes(lines)
	default:
		types = extractGenericTypes(lines, lang)
	}

	return types
}

// extractGoFunctions extracts Go function declarations.
func extractGoFunctions(lines []string) []docFunction {
	var funcs []docFunction

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Track single-line comments
		if strings.HasPrefix(line, "//") {
			continue
		}

		// Check for function declaration
		if strings.HasPrefix(line, "func ") {
			comment := ""
			// Collect preceding comments
			for j := i - 1; j >= 0 && strings.HasPrefix(strings.TrimSpace(lines[j]), "//"); j-- {
				c := strings.TrimPrefix(strings.TrimSpace(lines[j]), "//")
				if c != "" {
					if comment != "" {
						comment = c + "\n" + comment
					} else {
						comment = c
					}
				}
			}

			// Parse function signature
			name, params, ret := parseGoSignature(line)
			if name != "" {
				funcs = append(funcs, docFunction{
					Name:        name,
					Params:      params,
					ReturnType:  ret,
					Comment:     comment,
					Description: comment,
				})
			}
		}

		// Check for method declaration (type. method)
		if strings.Contains(line, ". ") && strings.Contains(line, "func") {
			name, params, ret := parseGoSignature(line)
			if name != "" {
				comment := getPrecedingComment(lines, i)
				if comment != "" {
					funcs = append(funcs, docFunction{
						Name:        name,
						Params:      params,
						ReturnType:  ret,
						Comment:     comment,
						Description: comment,
					})
				}
			}
		}
	}

	return funcs
}

// getPrecedingComment returns the comment block preceding a line.
func getPrecedingComment(lines []string, lineIdx int) string {
	var comments []string
	for j := lineIdx - 1; j >= 0; j-- {
		line := strings.TrimSpace(lines[j])
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "//") {
			c := strings.TrimPrefix(line, "//")
			c = strings.TrimSpace(c)
			if c != "" {
				comments = append([]string{c}, comments...)
			}
		} else if strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "*") {
			// Block comment - collect backwards
			for k := j; k >= 0; k-- {
				bl := strings.TrimSpace(lines[k])
				if strings.HasSuffix(bl, "*/") {
					break
				}
				c := strings.TrimPrefix(bl, "*")
				c = strings.TrimSpace(c)
				if c != "" {
					comments = append([]string{c}, comments...)
				}
				if k == j {
					j = k // Don't re-process this line
				}
			}
			break
		} else {
			break
		}
	}

	return strings.Join(comments, "\n")
}

// parseGoSignature parses a Go function declaration line.
func parseGoSignature(line string) (name, params, ret string) {
	// Match: func Name(params) (ret)
	re := regexp.MustCompile(`func\s+(\w+)\s*\(([^)]*)\)`)
	matches := re.FindStringSubmatch(line)
	if matches != nil {
		name = matches[1]
		params = strings.TrimSpace(matches[2])

		// Check for return type
		if retMatch := regexp.MustCompile(`\)\s*(\([^)]+\))`).FindStringSubmatch(line); retMatch != nil {
			ret = strings.Trim(retMatch[1], "()")
		}
	}
	return
}

// extractGoTypes extracts Go struct, interface, and type definitions.
func extractGoTypes(lines []string) []docType {
	var types []docType
	i := 0

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Check for type declaration
		if strings.HasPrefix(line, "type ") {
			comment := getPrecedingComment(lines, i)

			// Parse type definition
			name := ""
			typeKind := ""
			fields := ""

			// type Name struct { ... }
			if strings.Contains(line, "struct") {
				typeKind = "struct"
				name = extractTypeName(line)
				// Collect struct fields
				fields = extractStructFields(lines, i)
			} else if strings.Contains(line, "interface") {
				typeKind = "interface"
				name = extractTypeName(line)
			} else {
				typeKind = "type"
				name = extractTypeName(line)
			}

			if name != "" {
				types = append(types, docType{
					Name:        name,
					TypeKind:    typeKind,
					Fields:      fields,
					Comment:     comment,
					Description: comment,
				})
			}
		}
		i++
	}

	return types
}

// extractTypeName extracts the type name from a type declaration line.
func extractTypeName(line string) string {
	// Skip "type" prefix
	rest := strings.TrimPrefix(line, "type ")
	// Get first word
	parts := strings.Fields(rest)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// extractStructFields collects struct field lines.
func extractStructFields(lines []string, startIdx int) string {
	var fields []string
	inBlock := false

	for i := startIdx; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		if !inBlock {
			if strings.Contains(line, "{") {
				inBlock = true
				continue
			}
			continue
		}

		if strings.Contains(line, "}") {
			break
		}

		// Skip empty lines and comments in struct
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") {
			continue
		}

		fields = append(fields, line)
	}

	return strings.Join(fields, "\n")
}

// extractPythonFunctions extracts Python function and method definitions.
func extractPythonFunctions(lines []string) []docFunction {
	var funcs []docFunction
	i := 0

	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Check for def statement
		if strings.HasPrefix(trimmed, "def ") && !strings.HasPrefix(trimmed, "def __") {
			comment := getPythonDocstring(lines, i)
			if comment == "" {
				comment = getPrecedingComment(lines, i)
			}

			name := ""
			params := ""
			// Parse: def name(params):
			re := regexp.MustCompile(`def\s+(\w+)\s*\(([^)]*)\)`)
			matches := re.FindStringSubmatch(trimmed)
			if matches != nil {
				name = matches[1]
				params = strings.TrimSpace(matches[2])
			}

			if name != "" && !strings.HasPrefix(name, "_") {
				funcs = append(funcs, docFunction{
					Name:        name,
					Params:      params,
					Comment:     comment,
					Description: comment,
				})
			}
		}
		i++
	}

	return funcs
}

// getPythonDocstring extracts the docstring following a function definition.
func getPythonDocstring(lines []string, startIdx int) string {
	i := startIdx + 1
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			i++
			continue
		}
		// Check for docstring
		if strings.HasPrefix(trimmed, `"""`) || strings.HasPrefix(trimmed, `'''`) {
			quote := trimmed[:3]
			// Single line docstring
			if strings.HasSuffix(trimmed, quote) && len(trimmed) > 6 {
				return strings.TrimSpace(trimmed[3 : len(trimmed)-3])
			}
			// Multi-line docstring
			var parts []string
			i++
			for i < len(lines) {
				if strings.Contains(lines[i], quote) {
					break
				}
				parts = append(parts, strings.TrimSpace(lines[i]))
				i++
			}
			return strings.Join(parts, "\n")
		}
		break
	}
	return ""
}

// extractPythonTypes extracts Python class definitions.
func extractPythonTypes(lines []string) []docType {
	var types []docType
	i := 0

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		if strings.HasPrefix(line, "class ") {
			comment := getPrecedingComment(lines, i)
			re := regexp.MustCompile(`class\s+(\w+)`)
			matches := re.FindStringSubmatch(line)
			name := ""
			if matches != nil {
				name = matches[1]
			}

			if name != "" && !strings.HasPrefix(name, "_") {
				types = append(types, docType{
					Name:        name,
					TypeKind:    "class",
					Comment:     comment,
					Description: comment,
				})
			}
		}
		i++
	}

	return types
}

// extractJSFunctions extracts JavaScript/TypeScript function declarations.
func extractJSFunctions(lines []string, lang string) []docFunction {
	var funcs []docFunction
	i := 0

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Match: function name(params) {
		// Match: const name = function(params) {
		// Match: const name = (params) =>
		// Match: name(params) { (method shorthand)
		// Match: export function name(params)
		// Match: export const name = (params) =>
		patterns := []string{
			`(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*\(([^)]*)\)`,
			`(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?function\s*\(([^)]*)\)`,
			`(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?\(([^)]*)\)\s*=>`,
			`(?:export\s+)?(?:async\s+)?(\w+)\s*\(([^)]*)\)\s*\{`,
		}

		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			matches := re.FindStringSubmatch(line)
			if matches != nil {
				name := matches[1]
				params := strings.TrimSpace(matches[2])

				// Skip common non-function patterns
				skipNames := []string{"if", "for", "while", "switch", "catch", "return", "new", "class", "import", "export"}
				skip := false
				for _, skipName := range skipNames {
					if name == skipName {
						skip = true
						break
					}
				}
				if skip {
					continue
				}

				comment := getPrecedingComment(lines, i)
				if !strings.HasPrefix(name, "_") {
					funcs = append(funcs, docFunction{
						Name:        name,
						Params:      params,
						Comment:     comment,
						Description: comment,
					})
				}
				break
			}
		}
		i++
	}

	return funcs
}

// extractJSTypes extracts JavaScript/TypeScript class and interface definitions.
func extractJSTypes(lines []string, lang string) []docType {
	var types []docType
	i := 0

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		if lang == "typescript" {
			// TypeScript interfaces
			if strings.HasPrefix(line, "export interface ") || strings.HasPrefix(line, "interface ") {
				comment := getPrecedingComment(lines, i)
				re := regexp.MustCompile(`(?:export\s+)?interface\s+(\w+)`)
				matches := re.FindStringSubmatch(line)
				if len(matches) > 1 {
					types = append(types, docType{
						Name:        matches[1],
						TypeKind:    "interface",
						Comment:     comment,
						Description: comment,
					})
				}
			}
			// TypeScript types
			if strings.HasPrefix(line, "export type ") || (strings.HasPrefix(line, "type ") && !strings.Contains(line, " = function")) {
				comment := getPrecedingComment(lines, i)
				re := regexp.MustCompile(`(?:export\s+)?type\s+(\w+)`)
				matches := re.FindStringSubmatch(line)
				if len(matches) > 1 {
					types = append(types, docType{
						Name:        matches[1],
						TypeKind:    "type",
						Comment:     comment,
						Description: comment,
					})
				}
			}
		}

		// Class definitions (JS and TS)
		if strings.HasPrefix(line, "export class ") || strings.HasPrefix(line, "class ") {
			comment := getPrecedingComment(lines, i)
			re := regexp.MustCompile(`(?:export\s+)?class\s+(\w+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				types = append(types, docType{
					Name:        matches[1],
					TypeKind:    "class",
					Comment:     comment,
					Description: comment,
				})
			}
		}

		i++
	}

	return types
}

// extractJavaFunctions extracts Java method declarations.
func extractJavaFunctions(lines []string) []docFunction {
	var funcs []docFunction
	i := 0

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Skip class/interface declarations
		if strings.Contains(line, "class ") || strings.Contains(line, "interface ") {
			i++
			continue
		}

		// Match method declarations (simplified)
		// Look for lines with return type, method name, and params
		if strings.Contains(line, "(") && strings.Contains(line, ")") &&
			!strings.HasPrefix(line, "if ") && !strings.HasPrefix(line, "for ") &&
			!strings.HasPrefix(line, "while ") && !strings.HasPrefix(line, "switch ") {
			// Check if it looks like a method signature
			if regexp.MustCompile(`\w+\s+\w+\s*\([^)]*\)\s*(?:throws\s+[\w,\s]+)?\s*\{?$`).MatchString(line) {
				re := regexp.MustCompile(`(\w+)\s+(\w+)\s*\(([^)]*)\)`)
				matches := re.FindStringSubmatch(line)
				if len(matches) > 2 {
					ret := matches[1]
					name := matches[2]
					params := strings.TrimSpace(matches[3])

					// Skip constructors (same name as class) and common keywords
					if name != "class" && name != "interface" && name != "new" {
						comment := getPrecedingComment(lines, i)
						funcs = append(funcs, docFunction{
							Name:        name,
							Params:      params,
							ReturnType:  ret,
							Comment:     comment,
							Description: comment,
						})
					}
				}
			}
		}
		i++
	}

	return funcs
}

// extractJavaTypes extracts Java class and interface definitions.
func extractJavaTypes(lines []string) []docType {
	var types []docType
	i := 0

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		if strings.HasPrefix(line, "public class ") || strings.HasPrefix(line, "class ") ||
			strings.HasPrefix(line, "public interface ") || strings.HasPrefix(line, "interface ") ||
			strings.HasPrefix(line, "public abstract class ") || strings.HasPrefix(line, "abstract class ") {
			comment := getPrecedingComment(lines, i)
			re := regexp.MustCompile(`(?:public\s+)?(?:abstract\s+)?(?:class|interface)\s+(\w+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				typeKind := "class"
				if strings.Contains(line, "interface") {
					typeKind = "interface"
				}
				types = append(types, docType{
					Name:        matches[1],
					TypeKind:    typeKind,
					Comment:     comment,
					Description: comment,
				})
			}
		}
		i++
	}

	return types
}

// extractRustFunctions extracts Rust function declarations.
func extractRustFunctions(lines []string) []docFunction {
	var funcs []docFunction
	i := 0

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Skip attributes (#[...])
		if strings.HasPrefix(line, "#[") {
			i++
			continue
		}

		// Match: fn name(params) -> ret { or fn name(params) {
		if strings.HasPrefix(line, "pub fn ") || strings.HasPrefix(line, "fn ") {
			comment := getPrecedingComment(lines, i)
			re := regexp.MustCompile(`(?:pub\s+)?(?:async\s+)?fn\s+(\w+)\s*\(([^)]*)\)(?:\s*->\s*(\w+[^{]*))?`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				name := matches[1]
				params := strings.TrimSpace(matches[2])
				ret := ""
				if len(matches) > 3 && matches[3] != "" {
					ret = strings.TrimSpace(matches[3])
				}
				if !strings.HasPrefix(name, "_") {
					funcs = append(funcs, docFunction{
						Name:        name,
						Params:      params,
						ReturnType:  ret,
						Comment:     comment,
						Description: comment,
					})
				}
			}
		}
		i++
	}

	return funcs
}

// extractRustTypes extracts Rust struct, enum, and trait definitions.
func extractRustTypes(lines []string) []docType {
	var types []docType
	i := 0

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Skip attributes
		if strings.HasPrefix(line, "#[") {
			i++
			continue
		}

		// Match: pub struct Name, struct Name
		if strings.HasPrefix(line, "pub struct ") || strings.HasPrefix(line, "struct ") {
			comment := getPrecedingComment(lines, i)
			re := regexp.MustCompile(`(?:pub\s+)?struct\s+(\w+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				types = append(types, docType{
					Name:        matches[1],
					TypeKind:    "struct",
					Comment:     comment,
					Description: comment,
				})
			}
		}

		// Match: pub enum Name, enum Name
		if strings.HasPrefix(line, "pub enum ") || strings.HasPrefix(line, "enum ") {
			comment := getPrecedingComment(lines, i)
			re := regexp.MustCompile(`(?:pub\s+)?enum\s+(\w+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				types = append(types, docType{
					Name:        matches[1],
					TypeKind:    "enum",
					Comment:     comment,
					Description: comment,
				})
			}
		}

		// Match: pub trait Name, trait Name
		if strings.HasPrefix(line, "pub trait ") || strings.HasPrefix(line, "trait ") {
			comment := getPrecedingComment(lines, i)
			re := regexp.MustCompile(`(?:pub\s+)?trait\s+(\w+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				types = append(types, docType{
					Name:        matches[1],
					TypeKind:    "trait",
					Comment:     comment,
					Description: comment,
				})
			}
		}

		i++
	}

	return types
}

// extractGenericFunctions extracts generic function patterns.
func extractGenericFunctions(lines []string, lang string) []docFunction {
	var funcs []docFunction
	declKeywords := map[string]bool{
		"function": true, "def": true, "fn": true, "func": true,
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		for keyword := range declKeywords {
			if (strings.HasPrefix(trimmed, keyword+" ") || strings.Contains(trimmed, " "+keyword+" ")) &&
				strings.Contains(trimmed, "(") {
				comment := getPrecedingComment(lines, i)
				re := regexp.MustCompile(`(?:function|def|fn|func)\s+(\w+)\s*\(([^)]*)\)`)
				matches := re.FindStringSubmatch(trimmed)
				if len(matches) > 1 {
					funcs = append(funcs, docFunction{
						Name:        matches[1],
						Params:      strings.TrimSpace(matches[2]),
						Comment:     comment,
						Description: comment,
					})
				}
				break
			}
		}
	}

	return funcs
}

// extractGenericTypes extracts generic type patterns.
func extractGenericTypes(lines []string, lang string) []docType {
	var types []docType
	typeKeywords := map[string]string{
		"class":    "class", "struct": "struct", "interface": "interface",
		"type":     "type", "enum": "enum", "trait": "trait",
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		for keyword, kind := range typeKeywords {
			if (strings.HasPrefix(trimmed, keyword+" ") || strings.Contains(trimmed, " "+keyword+" ")) &&
				!strings.Contains(trimmed, "(") {
				comment := getPrecedingComment(lines, i)
				re := regexp.MustCompile(`(?:class|struct|interface|type|enum|trait)\s+(\w+)`)
				matches := re.FindStringSubmatch(trimmed)
				if len(matches) > 1 {
					types = append(types, docType{
						Name:        matches[1],
						TypeKind:    kind,
						Comment:     comment,
						Description: comment,
					})
				}
				break
			}
		}
	}

	return types
}

// formatMarkdown generates Markdown documentation.
func formatMarkdown(content, lang, filePath string, funcs []docFunction, types []docType, detail string, includeComments bool) (string, error) {
	var output strings.Builder

	// File header
	output.WriteString(fmt.Sprintf("# Documentation: %s\n\n", filePath))

	// Language info
	langs := map[string]string{
		"go": "Go", "python": "Python", "javascript": "JavaScript", "typescript": "TypeScript",
		"java": "Java", "rust": "Rust", "c": "C", "cpp": "C++", "ruby": "Ruby",
		"php": "PHP", "csharp": "C#", "swift": "Swift", "kotlin": "Kotlin",
	}
	if langName, ok := langs[lang]; ok {
		output.WriteString(fmt.Sprintf("**Language:** %s\n\n", langName))
	}

	// Source length
	lineCount := strings.Count(content, "\n") + 1
	output.WriteString(fmt.Sprintf("**Lines:** %d\n\n", lineCount))

	// Types section
	if len(types) > 0 {
		output.WriteString("## Types\n\n")
		for _, t := range types {
			output.WriteString(fmt.Sprintf("### `%s` (%s)\n\n", t.Name, t.TypeKind))
			if detail == "detailed" && t.Fields != "" {
				output.WriteString("```" + lang + "\n")
				output.WriteString(t.Fields + "\n```\n\n")
			}
			if detail == "detailed" && includeComments && t.Comment != "" {
				output.WriteString("> " + strings.ReplaceAll(t.Comment, "\n", "\n> ") + "\n\n")
			}
		}
	}

	// Functions section
	if len(funcs) > 0 {
		output.WriteString("## Functions\n\n")
		for _, f := range funcs {
			output.WriteString(fmt.Sprintf("### `%s`", f.Name))
			if f.Params != "" {
				output.WriteString(fmt.Sprintf("(`%s`)", f.Params))
			}
			if f.ReturnType != "" {
				output.WriteString(fmt.Sprintf(" → `%s`", f.ReturnType))
			}
			output.WriteString("\n\n")

			if detail == "detailed" && includeComments && f.Comment != "" {
				output.WriteString("> " + strings.ReplaceAll(f.Comment, "\n", "\n> ") + "\n\n")
			}
		}
	}

	// If no symbols found
	if len(types) == 0 && len(funcs) == 0 {
		output.WriteString("_No top-level symbols detected._\n\n")
	}

	return output.String(), nil
}

// formatInline generates inline documentation comments.
func formatInline(content, lang string, funcs []docFunction, types []docType, detail string, includeComments bool) (string, error) {
	langs := map[string]string{
		"go": "//", "python": "#", "javascript": "//", "typescript": "//",
		"java": "//", "rust": "//", "c": "//", "cpp": "//", "ruby": "#",
		"php": "//", "csharp": "//", "swift": "//", "kotlin": "//",
	}
	prefix := "//"
	if p, ok := langs[lang]; ok {
		prefix = p
	}

	var output strings.Builder
	output.WriteString("// Auto-generated documentation\n\n")

	// Generate inline comments for types
	for _, t := range types {
		output.WriteString(fmt.Sprintf("%s // %s %s\n", prefix, t.TypeKind, t.Name))
		if detail == "detailed" && t.Comment != "" && includeComments {
			for _, line := range strings.Split(t.Comment, "\n") {
				output.WriteString(fmt.Sprintf("%s // %s\n", prefix, strings.TrimSpace(line)))
			}
		}
		output.WriteString("\n")
	}

	// Generate inline comments for functions
	for _, f := range funcs {
		output.WriteString(fmt.Sprintf("%s // %s(%s)", prefix, f.Name, f.Params))
		if f.ReturnType != "" {
			output.WriteString(fmt.Sprintf(" -> %s", f.ReturnType))
		}
		output.WriteString("\n")
		if detail == "detailed" && f.Comment != "" && includeComments {
			for _, line := range strings.Split(f.Comment, "\n") {
				output.WriteString(fmt.Sprintf("%s // %s\n", prefix, strings.TrimSpace(line)))
			}
		}
		output.WriteString("\n")
	}

	return output.String(), nil
}

// contains checks if a string slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}


// ===== Code Metrics Analysis Tool =====

// fileMetrics holds metrics for a single file.
type fileMetrics struct {
	Path       string       `json:"path"`
	Language   string       `json:"language"`
	Lines      lineCounts   `json:"lines"`
	Functions  []funcMetric `json:"functions"`
	Complexity int          `json:"complexity"`
}

// lineCounts holds line breakdown counts.
type lineCounts struct {
	Total   int `json:"total"`
	Code    int `json:"code"`
	Blank   int `json:"blank"`
	Comment int `json:"comment"`
}

// funcMetric holds metrics for a single function.
type funcMetric struct {
	Name       string `json:"name"`
	Line       int    `json:"line"`
	Complexity int    `json:"complexity"`
}

// codeMetricsOutput is the top-level output structure.
type codeMetricsOutput struct {
	Summary summaryMetrics `json:"summary"`
	Files   []fileMetrics  `json:"files"`
}

// summaryMetrics holds aggregate metrics.
type summaryMetrics struct {
	TotalFiles     int            `json:"total_files"`
	TotalLines     int            `json:"total_lines"`
	TotalCodeLines int            `json:"total_code_lines"`
	TotalBlankLines int           `json:"total_blank_lines"`
	TotalCommentLines int         `json:"total_comment_lines"`
	TotalFunctions int            `json:"total_functions"`
	AvgComplexity  float64        `json:"avg_complexity"`
	MaxComplexity  int            `json:"max_complexity"`
	Languages      map[string]int `json:"languages"`
}

// detectLanguage detects the programming language from a file path or extension.
func detectCodeMetricsLanguage(path string, forced string) string {
	if forced != "" {
		return strings.ToLower(forced)
	}
	ext := filepath.Ext(path)
	langs := map[string]string{
		".go": "go", ".py": "python", ".js": "javascript", ".jsx": "javascript",
		".ts": "typescript", ".tsx": "typescript", ".java": "java",
		".rs": "rust", ".c": "c", ".h": "c", ".cpp": "cpp", ".hpp": "cpp",
	}
	if lang, ok := langs[ext]; ok {
		return lang
	}
	return ""
}

// isSourceFile checks if a file is a source file (not binary).
func isSourceFile(path string) bool {
	ext := filepath.Ext(path)
	supported := map[string]bool{
		".go": true, ".py": true, ".js": true, ".jsx": true,
		".ts": true, ".tsx": true, ".java": true, ".rs": true,
		".c": true, ".h": true, ".cpp": true, ".hpp": true,
	}
	return supported[ext]
}

// isCommentLine checks if a line is a comment.
func isCommentLine(line string, lang string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	switch lang {
	case "go", "java", "c", "cpp", "rust":
		return strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*")
	case "python":
		return strings.HasPrefix(trimmed, "#")
	case "javascript", "typescript":
		return strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*")
	default:
		return false
	}
}

// isInsideBlockComment checks if the line is inside a block comment (for single-line detection).
func isInsideBlockComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "/*") && strings.Contains(trimmed, "*/") {
		return true // single-line block comment
	}
	return false
}

// countLines counts total, blank, comment, and code lines in source content.
func countLines(content string, lang string) lineCounts {
	counts := lineCounts{}
	lines := strings.Split(content, "\n")
	counts.Total = len(lines)
	
	inBlockComment := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if trimmed == "" {
			counts.Blank++
			continue
		}
		
		// Handle block comments
		switch lang {
		case "go", "java", "c", "cpp", "javascript", "typescript":
			if inBlockComment {
				counts.Comment++
				if strings.Contains(trimmed, "*/") {
					inBlockComment = false
				}
				continue
			}
			if strings.HasPrefix(trimmed, "/*") {
				counts.Comment++
				if !strings.Contains(trimmed, "*/") {
					inBlockComment = true
				}
				continue
			}
			if strings.HasPrefix(trimmed, "//") {
				counts.Comment++
				continue
			}
		case "python":
			if strings.HasPrefix(trimmed, "#") {
				counts.Comment++
				continue
			}
		case "rust":
			if strings.HasPrefix(trimmed, "//") {
				counts.Comment++
				continue
			}
			if strings.HasPrefix(trimmed, "/*") {
				counts.Comment++
				if !strings.Contains(trimmed, "*/") {
					inBlockComment = true
				}
				continue
			}
			if inBlockComment {
				counts.Comment++
				if strings.Contains(trimmed, "*/") {
					inBlockComment = false
				}
				continue
			}
		}
		
		counts.Code++
	}
	return counts
}

// complexityPoints returns the number of decision points in a line.
func complexityPoints(line string) int {
	points := 0
	trimmed := strings.TrimSpace(line)
	
	// Control flow keywords (only count when not inside a comment or string)
	keywords := []string{"if ", "if(", "else if", "else if(", "for ", "for(", "while ", "while(", "switch ", "switch(", "case ", "catch ", "except "}
	for _, kw := range keywords {
		if strings.Contains(trimmed, kw) {
			points++
		}
	}
	
	// Logical operators
	points += strings.Count(trimmed, "&&")
	points += strings.Count(trimmed, "||")
	
	// Ternary operator
	points += strings.Count(trimmed, "?")
	
	return points
}

// detectFunctions finds functions/methods in source code for a given language.
func detectFunctions(content string, lang string) []funcMetric {
	var funcs []funcMetric
	lines := strings.Split(content, "\n")
	
	switch lang {
	case "go":
		// Go: func name(...) {
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "func ") {
				name := strings.TrimPrefix(trimmed, "func ")
				name = strings.SplitN(name, "(", 2)[0]
				// Remove receiver info for method receivers like func (r *Receiver) Name(
				if strings.HasPrefix(name, "(") {
					continue // skip receiver declarations without name
				}
				complexity := 1
				// Look ahead for complexity in function body (up to 20 lines or until closing brace)
				depth := 1
				for j := i + 1; j < len(lines) && j < i+200 && depth > 0; j++ {
					ln := strings.TrimSpace(lines[j])
					if strings.HasPrefix(ln, "/*") || strings.HasPrefix(ln, "//") {
						continue
					}
					if strings.Contains(ln, "{") {
						depth++
					}
					if strings.Contains(ln, "}") {
						depth--
						if depth == 0 {
							break
						}
					}
					complexity += complexityPoints(ln)
				}
				funcs = append(funcs, funcMetric{
					Name:       name,
					Line:       i + 1,
					Complexity: complexity,
				})
			}
		}
	case "python":
		// Python: def name(...)
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "def ") && !strings.HasPrefix(trimmed, "def __") {
				name := strings.TrimPrefix(trimmed, "def ")
				name = strings.SplitN(name, "(", 2)[0]
				complexity := 1
				// Python complexity: count indented lines with keywords
				baseIndent := -1
				for j := i + 1; j < len(lines); j++ {
					ln := lines[j]
					trimmed := strings.TrimSpace(ln)
					if trimmed == "" {
						continue
					}
					ind := len(ln) - len(strings.TrimLeft(ln, " 	"))
					if baseIndent == -1 {
						baseIndent = ind
					}
					if ind <= baseIndent && trimmed != "" {
						break
					complexity += complexityPoints(trimmed)
				}
			}
			funcs = append(funcs, funcMetric{
					Name:       name,
					Line:       i + 1,
					Complexity: complexity,
				})
			}
		}
	case "javascript", "typescript":
		// JS/TS: function name, const name = , arrow functions, method definitions
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			// function name(...)
			if matched, _ := regexp.MatchString(`^(async\s+)?function\s+(\w+)`, trimmed); matched {
				submatches := regexp.MustCompile(`^(async\s+)?function\s+(\w+)`).FindStringSubmatch(trimmed)
				name := submatches[2]
				complexity := 1
				depth := 0
				for j := i + 1; j < len(lines) && j < i+200 && depth > -1; j++ {
					ln := strings.TrimSpace(lines[j])
					if strings.Contains(ln, "{") {
						depth++
					}
					if strings.Contains(ln, "}") {
						depth--
						if depth == 0 {
							break
						}
					}
					if depth > 0 {
						complexity += complexityPoints(ln)
					}
				}
				funcs = append(funcs, funcMetric{
					Name:       name,
					Line:       i + 1,
					Complexity: complexity,
				})
			} else if matched, _ := regexp.MatchString(`^(\w+)\s*[=:]\s*(async\s+)?function`, trimmed); matched {
				submatches := regexp.MustCompile(`^(\w+)\s*[=:]\s*(async\s+)?function`).FindStringSubmatch(trimmed)
				name := submatches[1]
				complexity := 1
				funcs = append(funcs, funcMetric{
					Name:       name,
					Line:       i + 1,
					Complexity: complexity,
				})
			} else if matched, _ := regexp.MatchString(`^(\w+)\s*[=:]\s*\(.*\)\s*=>`, trimmed); matched {
				submatches := regexp.MustCompile(`^(\w+)\s*[=:]\s*\(.*\)\s*=>`).FindStringSubmatch(trimmed)
				name := submatches[1]
				funcs = append(funcs, funcMetric{
					Name:       name,
					Line:       i + 1,
					Complexity: 1,
				})
			} else if matched, _ := regexp.MatchString(`^(\w+)\s*\(.*\)\s*\{`, trimmed); matched {
				// Method in class: name(...) {
				submatches := regexp.MustCompile(`^(\w+)\s*\(`).FindStringSubmatch(trimmed)
				if len(submatches) > 1 {
					name := submatches[1]
					if name != "if" && name != "for" && name != "while" && name != "switch" && name != "catch" {
						complexity := 1
						depth := 0
						for j := i + 1; j < len(lines) && j < i+200 && depth > -1; j++ {
							ln := strings.TrimSpace(lines[j])
							if strings.Contains(ln, "{") {
								depth++
							}
							if strings.Contains(ln, "}") {
								depth--
								if depth == 0 {
									break
								}
							}
							if depth > 0 {
								complexity += complexityPoints(ln)
							}
						}
						funcs = append(funcs, funcMetric{
							Name:       name,
							Line:       i + 1,
							Complexity: complexity,
						})
					}
				}
			}
		}
	case "java":
		// Java: modifier returnType methodName(...) {
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if matched, _ := regexp.MatchString(`^(public|private|protected|static|final|abstract|\s)+\s+\w+(\[\])*\s+(\w+)\s*\(`, trimmed); matched {
				submatches := regexp.MustCompile(`^(\w+)\s*\(`).FindStringSubmatch(trimmed)
				if len(submatches) > 1 {
					name := submatches[1]
					if name != "if" && name != "for" && name != "while" && name != "switch" && name != "catch" && name != "new" && name != "return" && name != "throw" {
						complexity := 1
						depth := 0
						for j := i + 1; j < len(lines) && j < i+200 && depth > -1; j++ {
							ln := strings.TrimSpace(lines[j])
							if strings.Contains(ln, "{") {
								depth++
							}
							if strings.Contains(ln, "}") {
								depth--
								if depth == 0 {
									break
								}
							}
							if depth > 0 {
								complexity += complexityPoints(ln)
							}
						}
						funcs = append(funcs, funcMetric{
							Name:       name,
							Line:       i + 1,
							Complexity: complexity,
						})
					}
				}
			}
		}
	case "rust":
		// Rust: fn name(...)
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "fn ") || strings.HasPrefix(trimmed, "pub fn ") {
				name := strings.Fields(trimmed)[1]
				complexity := 1
				depth := 0
				for j := i + 1; j < len(lines) && j < i+200 && depth > -1; j++ {
					ln := strings.TrimSpace(lines[j])
					if strings.Contains(ln, "{") {
						depth++
					}
					if strings.Contains(ln, "}") {
						depth--
						if depth == 0 {
							break
						}
					}
					if depth > 0 {
						complexity += complexityPoints(ln)
					}
				}
				funcs = append(funcs, funcMetric{
					Name:       name,
					Line:       i + 1,
					Complexity: complexity,
				})
			}
		}
	case "c", "cpp":
		// C/C++: returnType name(...) {
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if matched, _ := regexp.MatchString(`^[\w\s\*\&]+\s+(\w+)\s*\(`, trimmed); matched {
				submatches := regexp.MustCompile(`^[\w\s\*\&]+\s+(\w+)\s*\(`).FindStringSubmatch(trimmed)
				if len(submatches) > 1 {
					name := submatches[1]
					if name != "if" && name != "for" && name != "while" && name != "switch" && name != "catch" && name != "return" && name != "sizeof" {
						complexity := 1
						depth := 0
						for j := i + 1; j < len(lines) && j < i+200 && depth > -1; j++ {
							ln := strings.TrimSpace(lines[j])
							if strings.Contains(ln, "{") {
								depth++
							}
							if strings.Contains(ln, "}") {
								depth--
								if depth == 0 {
									break
								}
							}
							if depth > 0 {
								complexity += complexityPoints(ln)
							}
						}
						funcs = append(funcs, funcMetric{
							Name:       name,
							Line:       i + 1,
							Complexity: complexity,
						})
					}
				}
			}
		}
	}
	return funcs
}

// collectFiles returns a list of files matching the criteria.
func collectFiles(path string, maxDepth int, globPattern string) ([]string, error) {
	var files []string
	
	if maxDepth == 0 {
		// Single file mode
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			if isSourceFile(path) {
				files = append(files, path)
			}
		}
		return files, nil
	}
	
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %v", err)
	}
	
	if info, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("path not found: %s", path)
	} else if info.IsDir() {
		// Directory mode
		pattern := "**"
		if globPattern != "" {
			pattern = "**/" + globPattern
		}
		
		matches, err := filepath.Glob(filepath.Join(absPath, pattern))
		if err != nil {
			return nil, fmt.Errorf("glob error: %v", err)
		}
		
		for _, match := range matches {
			if isSourceFile(match) {
				// Check depth
				rel, err := filepath.Rel(absPath, match)
				if err == nil {
					depth := strings.Count(rel, string(filepath.Separator))
					if depth <= maxDepth {
						files = append(files, match)
					}
				}
			}
		}
	} else {
		// Single file
		if isSourceFile(absPath) {
			files = append(files, absPath)
		}
	}
	
	return files, nil
}

// executeCodeMetrics analyzes code files and returns metrics.
func (te *ToolExecutor) executeCodeMetrics(params map[string]interface{}) *ToolResult {
	path, ok := params["path"].(string)
	if !ok || path == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}
	
	language := ""
	if lang, ok := params["language"].(string); ok {
		language = lang
	}
	
	maxDepth := 5
	if md, ok := params["max_depth"].(float64); ok {
		maxDepth = int(md)
	}
	
	globPattern := ""
	if gp, ok := params["glob"].(string); ok {
		globPattern = gp
	}
	
	files, err := collectFiles(path, maxDepth, globPattern)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to collect files: %v", err),
		}
	}
	
	if len(files) == 0 {
		output := codeMetricsOutput{
			Summary: summaryMetrics{
				TotalFiles:  0,
				Languages:   make(map[string]int),
			},
			Files: []fileMetrics{},
		}
		jsonBytes, _ := json.Marshal(output)
		return &ToolResult{
			Success: true,
			Output:  string(jsonBytes),
		}
	}
	
	var allFiles []fileMetrics
	summary := summaryMetrics{
		Languages: make(map[string]int),
	}
	
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue // Skip files we can't read
		}
		
		lang := detectCodeMetricsLanguage(file, language)
		if lang == "" {
			continue // Skip unsupported languages
		}
		
		lineCounts := countLines(string(content), lang)
		functions := detectFunctions(string(content), lang)
		
		var fileComplexity int
		totalFuncComplexity := 0
		for _, fn := range functions {
			if fn.Complexity > fileComplexity {
				fileComplexity = fn.Complexity
			}
			totalFuncComplexity += fn.Complexity
		}
		
		if len(functions) > 0 {
			avgFnComplexity := float64(totalFuncComplexity) / float64(len(functions))
			fileComplexity = int(math.Round(avgFnComplexity))
		}
		
		allFiles = append(allFiles, fileMetrics{
			Path:       file,
			Language:   lang,
			Lines:      lineCounts,
			Functions:  functions,
			Complexity: fileComplexity,
		})
		
		summarizeFileMetrics(&summary, file, lang, lineCounts, functions, fileComplexity)
	}
	
	if summary.TotalFunctions > 0 {
		totalComplexity := 0
		for _, f := range allFiles {
			for _, fn := range f.Functions {
				totalComplexity += fn.Complexity
			}
		}
		summary.AvgComplexity = math.Round(float64(totalComplexity)/float64(summary.TotalFunctions)*10) / 10
	}
	
	// Set max complexity
	for _, f := range allFiles {
		if f.Complexity > summary.MaxComplexity {
			summary.MaxComplexity = f.Complexity
		}
	}
	
	// Ensure files slice is never null in JSON
	if allFiles == nil {
		allFiles = []fileMetrics{}
	}
	
	output := codeMetricsOutput{
		Summary: summary,
		Files:   allFiles,
	}
	
	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to marshal output: %v", err),
		}
	}
	
	return &ToolResult{
		Success: true,
		Output:  string(jsonBytes),
	}
}

// summarizeFileMetrics updates the summary with file-level metrics.
func summarizeFileMetrics(summary *summaryMetrics, path, lang string, lc lineCounts, funcs []funcMetric, complexity int) {
	summary.TotalFiles++
	summary.TotalLines += lc.Total
	summary.TotalCodeLines += lc.Code
	summary.TotalBlankLines += lc.Blank
	summary.TotalCommentLines += lc.Comment
	summary.TotalFunctions += len(funcs)
	
	if lang != "" {
		summary.Languages[lang]++
	}
}

// executeGitRevert reverts commits, files, or resets the working tree.
func (te *ToolExecutor) executeGitRevert(params map[string]interface{}) *ToolResult {
	action, hasAction := params["action"]
	if !hasAction {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: action",
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
		return te.executeGitRevertList(params)
	case "commit":
		return te.executeGitRevertCommit(params)
	case "files":
		return te.executeGitRevertFiles(params)
	case "soft_reset":
		return te.executeGitRevertSoftReset(params)
	case "hard_reset":
		return te.executeGitRevertHardReset(params)
	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s. Valid actions: 'list', 'commit', 'files', 'soft_reset', 'hard_reset'", actionStr),
		}
	}
}

// executeGitRevertList lists recent commits suitable for reverting.
func (te *ToolExecutor) executeGitRevertList(params map[string]interface{}) *ToolResult {
	maxCount := 20
	if mc, ok := params["max_count"].(float64); ok {
		maxCount = int(mc)
	} else if mc, ok := params["max_count"].(int); ok {
		maxCount = mc
	} else if mc, ok := params["max_count"].(string); ok {
		if n, err := strconv.Atoi(mc); err == nil {
			maxCount = n
		}
	}

	args := []string{"log", "--oneline", "--decorate", fmt.Sprintf("-n%d", maxCount)}
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git log failed: %s", string(output)),
		}
	}

	// Count commits
	commitCount := strings.Count(string(output), "\n")
	if strings.HasSuffix(string(output), "\n") {
		commitCount--
	}

	return &ToolResult{
		Success: true,
		Output:  string(output),
		Extra: map[string]interface{}{
			"tool":         "git_revert",
			"action":       "list",
			"commitCount":  commitCount,
			"maxCount":     maxCount,
		},
	}
}

// executeGitRevertCommit reverts a specific commit by hash.
func (te *ToolExecutor) executeGitRevertCommit(params map[string]interface{}) *ToolResult {
	hash, hasHash := params["hash"].(string)
	if !hasHash || hash == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: hash (commit hash to revert)",
		}
	}

	// Dry-run mode: check if the commit exists and what the revert would look like
	if dryRun, ok := params["dry_run"].(bool); ok && dryRun {
		// Verify commit exists
		verifyCmd := exec.Command("git", "rev-parse", "--verify", hash)
		if _, err := verifyCmd.CombinedOutput(); err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("commit '%s' not found or invalid", hash),
			}
		}

		// Show what the revert would look like
		showCmd := exec.Command("git", "revert", "--no-commit", "--dry-run", hash)
		showOutput, err := showCmd.CombinedOutput()
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("commit '%s' cannot be auto-reverted (conflicts may exist). Output: %s", hash, string(showOutput)),
				Extra: map[string]interface{}{
					"tool":     "git_revert",
					"action":   "commit",
					"hash":     hash,
					"dry_run":  true,
					"conflicts": true,
				},
			}
		}

		// Reset any partial state from --no-commit
		exec.Command("git", "reset", "--hard", "HEAD").Run()

		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Dry run: commit '%s' can be reverted without conflicts.", hash),
			Extra: map[string]interface{}{
				"tool":    "git_revert",
				"action":  "commit",
				"hash":    hash,
				"dry_run": true,
			},
		}
	}

	// Verify commit exists before attempting revert
	verifyCmd := exec.Command("git", "rev-parse", "--verify", hash)
	if _, err := verifyCmd.CombinedOutput(); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("commit '%s' not found or invalid", hash),
		}
	}

	// Perform the revert with --no-edit to use auto-generated message
	args := []string{"revert", "--no-edit", hash}

	// Check for signoff if requested
	if signoff, ok := params["signoff"].(bool); ok && signoff {
		args = append(args, "--signoff")
	}

	// Allow empty revert if requested
	if allowEmpty, ok := params["allow_empty"].(bool); ok && allowEmpty {
		args = append(args, "--allow-empty")
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if it's a conflict
		if strings.Contains(string(output), "conflict") || strings.Contains(string(output), "CONFLICT") {
			// Try to abort the revert
			exec.Command("git", "revert", "--abort").Run()
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("merge conflict during revert: %s", string(output)),
				Extra: map[string]interface{}{
					"tool":      "git_revert",
					"action":    "commit",
					"hash":      hash,
					"conflicts": true,
				},
			}
		}
		// Try to abort the revert
		exec.Command("git", "revert", "--abort").Run()
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git revert failed: %s", string(output)),
			Extra: map[string]interface{}{
				"tool":   "git_revert",
				"action": "commit",
				"hash":   hash,
			},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  string(output),
		Extra: map[string]interface{}{
			"tool":   "git_revert",
			"action": "commit",
			"hash":   hash,
		},
	}
}

// executeGitRevertFiles reverts specific files to their last committed state.
func (te *ToolExecutor) executeGitRevertFiles(params map[string]interface{}) *ToolResult {
	filesParam, hasFiles := params["files"]
	if !hasFiles {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: files (array of file paths to revert)",
		}
	}

	var files []string
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
	default:
		return &ToolResult{
			Success: false,
			Error:   "files must be a string or array of strings",
		}
	}

	if len(files) == 0 {
		return &ToolResult{
			Success: false,
			Error:   "no files specified for revert",
		}
	}

	// Dry-run: show what would be reverted
	if dryRun, ok := params["dry_run"].(bool); ok && dryRun {
		// Check which files exist in the index
		var existingFiles []string
		var missingFiles []string
		for _, f := range files {
			checkCmd := exec.Command("git", "ls-files", "--error-unmatch", f)
			if _, err := checkCmd.CombinedOutput(); err != nil {
				missingFiles = append(missingFiles, f)
			} else {
				existingFiles = append(existingFiles, f)
			}
		}

		if len(missingFiles) > 0 && len(existingFiles) == 0 {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("none of the specified files exist in the index: %s", strings.Join(missingFiles, ", ")),
				Extra: map[string]interface{}{
					"tool":       "git_revert",
					"action":     "files",
					"dry_run":    true,
					"missing":    missingFiles,
					"existing":   []string{},
				},
			}
		}

		result := &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Dry run: %d file(s) would be reverted to their last committed state", len(existingFiles)),
			Extra: map[string]interface{}{
				"tool":      "git_revert",
				"action":    "files",
				"dry_run":   true,
				"existing":  existingFiles,
				"missing":   missingFiles,
			},
		}
		if len(missingFiles) > 0 {
			result.Extra["warning"] = "Some files do not exist in the index and will be ignored"
		}
		return result
	}

	// Use git restore to revert files (modern approach, available in git 2.23+)
	// Falls back to git checkout if restore is not available
	args := append([]string{"restore", "--source=HEAD", "--"}, files...)
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	// If restore is not available (older git), try checkout
	if err != nil && strings.Contains(string(output), "restore") && (strings.Contains(string(output), "unrecognized") || strings.Contains(string(output), "not found")) {
		args = append([]string{"checkout", "HEAD", "--"}, files...)
		cmd = exec.Command("git", args...)
		output, err = cmd.CombinedOutput()
	}

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git revert files failed: %s", string(output)),
			Extra: map[string]interface{}{
				"tool":   "git_revert",
				"action": "files",
				"files":  files,
			},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Reverted %d file(s) to their last committed state: %s", len(files), strings.Join(files, ", ")),
		Extra: map[string]interface{}{
			"tool":   "git_revert",
			"action": "files",
			"files":  files,
		},
	}
}

// executeGitRevertSoftReset performs a soft reset to a specific commit.
func (te *ToolExecutor) executeGitRevertSoftReset(params map[string]interface{}) *ToolResult {
	hash, hasHash := params["hash"].(string)
	if !hasHash || hash == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: hash (commit to soft reset to)",
		}
	}

	// Force flag: warn if on main/master branch
	force, _ := params["force"].(bool)

	// Check current branch
	branchCmd := exec.Command("git", "branch", "--show-current")
	branchOutput, err := branchCmd.CombinedOutput()
	if err == nil {
		currentBranch := strings.TrimSpace(string(branchOutput))
		if !force && (currentBranch == "main" || currentBranch == "master") {
			// Still perform the reset but warn in output
			_ = currentBranch
		}
	}

	// Verify commit exists
	verifyCmd := exec.Command("git", "rev-parse", "--verify", hash)
	if _, err := verifyCmd.CombinedOutput(); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("commit '%s' not found or invalid", hash),
		}
	}

	args := []string{"reset", "--soft", hash}
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git soft reset failed: %s", string(output)),
			Extra: map[string]interface{}{
				"tool":   "git_revert",
				"action": "soft_reset",
				"hash":   hash,
			},
		}
	}

	// Get the commit message of the target commit for reference
	msgCmd := exec.Command("git", "log", "-1", "--format=%s", hash)
	msgOutput, _ := msgCmd.CombinedOutput()

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Soft reset to %s (%s). Changes are now staged.", hash, strings.TrimSpace(string(msgOutput))),
		Extra: map[string]interface{}{
			"tool":   "git_revert",
			"action": "soft_reset",
			"hash":   hash,
		},
	}
}

// executeGitRevertHardReset performs a hard reset to a specific commit.
func (te *ToolExecutor) executeGitRevertHardReset(params map[string]interface{}) *ToolResult {
	hash, hasHash := params["hash"].(string)
	if !hasHash || hash == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: hash (commit to hard reset to)",
		}
	}

	// Force flag is required for hard reset on protected branches
	force, _ := params["force"].(bool)

	// Check current branch
	branchCmd := exec.Command("git", "branch", "--show-current")
	branchOutput, err := branchCmd.CombinedOutput()
	if err == nil {
		currentBranch := strings.TrimSpace(string(branchOutput))
		if !force && (currentBranch == "main" || currentBranch == "master") {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("hard reset on '%s' branch requires force=true to prevent accidental data loss", currentBranch),
				Extra: map[string]interface{}{
					"tool":          "git_revert",
					"action":        "hard_reset",
					"hash":          hash,
					"currentBranch": currentBranch,
					"protection":    true,
				},
			}
		}
	}

	// Verify commit exists
	verifyCmd := exec.Command("git", "rev-parse", "--verify", hash)
	if _, err := verifyCmd.CombinedOutput(); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("commit '%s' not found or invalid", hash),
		}
	}

	// Warn user about destructive action in output
	msgCmd := exec.Command("git", "log", "-1", "--format=%s", hash)
	msgOutput, _ := msgCmd.CombinedOutput()

	args := []string{"reset", "--hard", hash}
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git hard reset failed: %s", string(output)),
			Extra: map[string]interface{}{
				"tool":   "git_revert",
				"action": "hard_reset",
				"hash":   hash,
			},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Hard reset to %s (%s). All local changes have been discarded.", hash, strings.TrimSpace(string(msgOutput))),
		Extra: map[string]interface{}{
			"tool":       "git_revert",
			"action":     "hard_reset",
			"hash":       hash,
			"destructive": true,
		},
	}
}

// executeGitRebase manages git rebases with various actions.
func (te *ToolExecutor) executeGitRebase(params map[string]interface{}) *ToolResult {
	action, hasAction := params["action"]
	if !hasAction {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: action",
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
		return te.gitRebaseList(params)
	case "start":
		return te.gitRebaseStart(params)
	case "continue":
		return te.gitRebaseContinue(params)
	case "abort":
		return te.gitRebaseAbort(params)
	case "skip":
		return te.gitRebaseSkip(params)
	case "update":
		return te.gitRebaseUpdate(params)
	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s. Valid actions: list, start, continue, abort, skip, update", actionStr),
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": actionStr,
			},
		}
	}
}

// gitRebaseList lists commits that would be rebased onto a target.
func (te *ToolExecutor) gitRebaseList(params map[string]interface{}) *ToolResult {
	target, hasTarget := params["target"]
	targetBranch := ""
	if hasTarget {
		targetBranch, _ = target.(string)
	}

	// Get current branch
	currentBranch, err := te.getCurrentBranch()
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to get current branch: %v", err),
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": "list",
			},
		}
	}

	// Default target to origin/main if not specified
	if targetBranch == "" {
		targetBranch = "origin/main"
	}

	// Check if target exists
	cmd := exec.Command("git", "rev-parse", "--verify", targetBranch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("target branch '%s' not found. Available branches:\n%s", targetBranch, te.listBranches()),
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": "list",
				"target": targetBranch,
			},
		}
	}
	_ = output // target exists

	// Get commits that would be rebased (commits in current branch not in target)
	cmd = exec.Command("git", "log", "--format=%h %s", currentBranch+".."+targetBranch, "--reverse")
	commits, err := cmd.CombinedOutput()
	if err != nil {
		// Try the other way around
		cmd = exec.Command("git", "log", "--format=%h %s", targetBranch+".."+currentBranch, "--reverse")
		commits, err = cmd.CombinedOutput()
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to list commits: %v", err),
				Extra: map[string]interface{}{
					"tool":   "git_rebase",
					"action": "list",
				},
			}
		}
	}

	commitLines := strings.TrimSpace(string(commits))
	if commitLines == "" {
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("No commits to rebase. Branch '%s' is already up to date with '%s'.", currentBranch, targetBranch),
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": "list",
				"count":  0,
			},
		}
	}

	lines := strings.Split(commitLines, "\n")
	commitList := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			commitList = append(commitList, line)
		}
	}

	result := fmt.Sprintf("Commits to rebase (%d):\n", len(commitList))
	for i, c := range commitList {
		result += fmt.Sprintf("  %d. %s\n", i+1, c)
	}
	result += fmt.Sprintf("\nTarget: %s\nBranch: %s\n", targetBranch, currentBranch)

	return &ToolResult{
		Success: true,
		Output:  result,
		Extra: map[string]interface{}{
			"tool":   "git_rebase",
			"action": "list",
			"count":  len(commitList),
			"target": targetBranch,
		},
	}
}

// gitRebaseStart starts a rebase onto a target branch.
func (te *ToolExecutor) gitRebaseStart(params map[string]interface{}) *ToolResult {
	target, hasTarget := params["target"]
	if !hasTarget {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: target (branch or commit to rebase onto)",
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": "start",
			},
		}
	}

	targetBranch, _ := target.(string)
	if targetBranch == "" {
		return &ToolResult{
			Success: false,
			Error:   "target branch cannot be empty",
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": "start",
			},
		}
	}

	// Check if target exists
	cmd := exec.Command("git", "rev-parse", "--verify", targetBranch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("target '%s' not found. Make sure the branch exists.", targetBranch),
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": "start",
				"target": targetBranch,
			},
		}
	}
	_ = output // target exists

	// Check if rebase is already in progress
	inRebase := te.checkRebaseInProgress()
	if inRebase {
		return &ToolResult{
			Success: false,
			Error:   "A rebase is already in progress. Use action='continue' to resume or action='abort' to cancel.",
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": "start",
			},
		}
	}

	// Build git rebase command
	args := []string{"rebase", targetBranch}

	// Add optional flags
	if keepEmpty, ok := params["keep_empty"].(bool); ok && keepEmpty {
		args = append(args, "--keep-empty")
	}
	if allowEmpty, ok := params["allow_empty"].(bool); ok && allowEmpty {
		args = append(args, "--allow-empty")
	}

	cmd = exec.Command("git", args...)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to start rebase: %s", string(output)),
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": "start",
				"target": targetBranch,
				"output": string(output),
			},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Rebase started onto '%s'. If conflicts arise, resolve them and use git_rebase(action='continue').", targetBranch),
		Extra: map[string]interface{}{
			"tool":   "git_rebase",
			"action": "start",
			"target": targetBranch,
		},
	}
}

// gitRebaseContinue continues a rebase after resolving conflicts.
func (te *ToolExecutor) gitRebaseContinue(params map[string]interface{}) *ToolResult {
	// Check if rebase is actually in progress
	inRebase := te.checkRebaseInProgress()
	if !inRebase {
		return &ToolResult{
			Success: false,
			Error:   "No rebase in progress. Use git_rebase(action='start', target='...') to begin a rebase first.",
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": "continue",
			},
		}
	}

	cmd := exec.Command("git", "rebase", "--continue")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it's a conflict (continue requires resolving first)
		if strings.Contains(string(output), "CONFLICT") || strings.Contains(string(output), "conflict") {
			return &ToolResult{
				Success: false,
				Error:   "Rebase conflict detected. Resolve conflicts, then use git add to mark resolved, and retry.",
				Extra: map[string]interface{}{
					"tool":   "git_rebase",
					"action": "continue",
					"output": string(output),
				},
			}
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("rebase continue failed: %s", string(output)),
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": "continue",
				"output": string(output),
			},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  "Rebase continued successfully.",
		Extra: map[string]interface{}{
			"tool":   "git_rebase",
			"action": "continue",
		},
	}
}

// gitRebaseAbort aborts an in-progress rebase.
func (te *ToolExecutor) gitRebaseAbort(params map[string]interface{}) *ToolResult {
	// Check if rebase is in progress
	inRebase := te.checkRebaseInProgress()
	if !inRebase {
		return &ToolResult{
			Success: false,
			Error:   "No rebase in progress to abort.",
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": "abort",
			},
		}
	}

	cmd := exec.Command("git", "rebase", "--abort")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to abort rebase: %s", string(output)),
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": "abort",
				"output": string(output),
			},
		}
	}

	currentBranch, _ := te.getCurrentBranch()
	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Rebase aborted. Restored to branch '%s'.", currentBranch),
		Extra: map[string]interface{}{
			"tool":   "git_rebase",
			"action": "abort",
		},
	}
}

// gitRebaseSkip skips the current commit during an interactive rebase.
func (te *ToolExecutor) gitRebaseSkip(params map[string]interface{}) *ToolResult {
	// Check if rebase is in progress
	inRebase := te.checkRebaseInProgress()
	if !inRebase {
		return &ToolResult{
			Success: false,
			Error:   "No rebase in progress to skip a commit.",
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": "skip",
			},
		}
	}

	cmd := exec.Command("git", "rebase", "--skip")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to skip commit: %s", string(output)),
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": "skip",
				"output": string(output),
			},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  "Skipped current commit and continued rebase.",
		Extra: map[string]interface{}{
			"tool":   "git_rebase",
			"action": "skip",
		},
	}
}

// gitRebaseUpdate updates the rebase todo list (for interactive rebases).
func (te *ToolExecutor) gitRebaseUpdate(params map[string]interface{}) *ToolResult {
	todo, hasTodo := params["rebase_todo"]
	if !hasTodo || todo == nil {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: rebase_todo (the updated todo list)",
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": "update",
			},
		}
	}

	todoStr, ok := todo.(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "rebase_todo must be a string",
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": "update",
			},
		}
	}

	if todoStr == "" {
		return &ToolResult{
			Success: false,
			Error:   "rebase_todo cannot be empty",
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": "update",
			},
		}
	}

	// Check if rebase is in progress
	inRebase := te.checkRebaseInProgress()
	if !inRebase {
		return &ToolResult{
			Success: false,
			Error:   "No rebase in progress. Use action='start' to begin a rebase first.",
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": "update",
			},
		}
	}

	// Write the updated todo list to .git/rebase-merge/git-rebase-todo
	rebaseTodoPath := ".git/rebase-merge/git-rebase-todo"
	if _, err := os.Stat(rebaseTodoPath); os.IsNotExist(err) {
		// Try rebase-apply directory
		rebaseTodoPath = ".git/rebase-apply/git-rebase-todo"
	}

	err := os.WriteFile(rebaseTodoPath, []byte(todoStr), 0644)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to write rebase todo: %v", err),
			Extra: map[string]interface{}{
				"tool":   "git_rebase",
				"action": "update",
			},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  "Rebase todo list updated. The rebase will continue with the new order on the next continue.",
		Extra: map[string]interface{}{
			"tool":   "git_rebase",
			"action": "update",
		},
	}
}

// checkRebaseInProgress checks if a git rebase is currently in progress.
func (te *ToolExecutor) checkRebaseInProgress() bool {
	// Check for rebase-merge directory
	if _, err := os.Stat(".git/rebase-merge"); err == nil {
		return true
	}
	// Check for rebase-apply directory
	if _, err := os.Stat(".git/rebase-apply"); err == nil {
		return true
	}
	return false
}

// getCurrentBranch gets the current git branch name.
func (te *ToolExecutor) getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// listBranches lists all local and remote branches.
func (te *ToolExecutor) listBranches() string {
	cmd := exec.Command("git", "branch", "-a")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "Unable to list branches."
	}
	return string(output)
}

// executeDependencyAudit scans project dependency files for outdated versions and issues.
func (te *ToolExecutor) executeDependencyAudit(params map[string]interface{}) *ToolResult {
	// Determine the target directory
	targetDir := "."
	if p, ok := params["path"].(string); ok && p != "" {
		targetDir = p
	}

	// Determine the language (auto-detect if not specified)
	forceLang := ""
	if l, ok := params["language"].(string); ok && l != "" {
		forceLang = l
	}

	// Check if we should check for outdated versions
	checkOutdated := false
	if co, ok := params["check_outdated"].(bool); ok {
		checkOutdated = co
	}

	// Determine max depth for scanning
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

	// Detect or validate project type
	projectType := ""
	if forceLang != "" {
		projectType = strings.ToLower(forceLang)
	} else {
		// Auto-detect based on dependency files present in targetDir or subdirectories
		projectType = te.detectProjectTypeWithDepth(targetDir, maxDepth)
	}

	if projectType == "" {
		return &ToolResult{
			Success: true,
			Output:  "No dependency files found in the project directory.",
			Extra: map[string]interface{}{
				"projectType": "",
				"dependencies": []interface{}{},
				"message":     "No dependency files detected. Supported: go.mod, package.json, requirements.txt, Cargo.toml, Gemfile.",
			},
		}
	}

	var result *ToolResult

	switch projectType {
	case "go":
		result = te.auditGoDependencies(targetDir, checkOutdated)
	case "nodejs", "javascript", "typescript", "node":
		result = te.auditNodeDependencies(targetDir, checkOutdated)
	case "python", "pip":
		result = te.auditPythonDependencies(targetDir, checkOutdated)
	case "rust", "cargo":
		result = te.auditRustDependencies(targetDir, checkOutdated)
	case "ruby", "gem":
		result = te.auditRubyDependencies(targetDir, checkOutdated)
	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown project type: %s", projectType),
		}
	}

	if result != nil && result.Success {
		result.Extra["projectType"] = projectType
	}

	return result
}

// projectType represents a detected dependency file and its location.
type projectType struct {
	Language string
	DepFile  string
	LockFile bool
}

// detectProjectTypeWithDepth scans the target directory for dependency files.
func (te *ToolExecutor) detectProjectTypeWithDepth(targetDir string, maxDepth int) string {
	// Check for dependency files in the target directory first, then subdirectories
	targets := te.findDependencyFiles(targetDir, maxDepth)

	// Priority order: check more specific files first
	for _, t := range targets {
		switch t.Language {
		case "go":
			return "go"
		case "rust":
			return "rust"
		case "ruby":
			return "ruby"
		case "python":
			return "python"
		case "nodejs":
			return "nodejs"
		}
	}

	return ""
}

// findDependencyFiles searches for dependency files within maxDepth directories.
func (te *ToolExecutor) findDependencyFiles(baseDir string, maxDepth int) []projectType {
	var results []projectType

	// Walk directories up to maxDepth
	te.walkWithDepth(baseDir, 0, maxDepth, func(dir string) {
		// Check for Go dependencies
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			hasLock := false
			if _, err := os.Stat(filepath.Join(dir, "go.sum")); err == nil {
				hasLock = true
			}
			results = append(results, projectType{
				Language: "go",
				DepFile:  filepath.Join(dir, "go.mod"),
				LockFile: hasLock,
			})
		}
		// Check for Node.js dependencies
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			hasLock := false
			for _, lockName := range []string{"package-lock.json", "yarn.lock", "pnpm-lock.yaml"} {
				if _, err := os.Stat(filepath.Join(dir, lockName)); err == nil {
					hasLock = true
					break
				}
			}
			results = append(results, projectType{
				Language: "nodejs",
				DepFile:  filepath.Join(dir, "package.json"),
				LockFile: hasLock,
			})
		}
		// Check for Python dependencies
		if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil {
			results = append(results, projectType{
				Language: "python",
				DepFile:  filepath.Join(dir, "requirements.txt"),
			})
		} else if _, err := os.Stat(filepath.Join(dir, "Pipfile")); err == nil {
			results = append(results, projectType{
				Language: "python",
				DepFile:  filepath.Join(dir, "Pipfile"),
			})
		} else if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err == nil {
			// Check if it has a [tool.poetry.dependencies] or [project.dependencies] section
			content, err := os.ReadFile(filepath.Join(dir, "pyproject.toml"))
			if err == nil && (strings.Contains(string(content), "poetry") || strings.Contains(string(content), "dependencies")) {
				results = append(results, projectType{
					Language: "python",
					DepFile:  filepath.Join(dir, "pyproject.toml"),
				})
			}
		}
		// Check for Rust dependencies
		if _, err := os.Stat(filepath.Join(dir, "Cargo.toml")); err == nil {
			hasLock := false
			if _, err := os.Stat(filepath.Join(dir, "Cargo.lock")); err == nil {
				hasLock = true
			}
			results = append(results, projectType{
				Language: "rust",
				DepFile:  filepath.Join(dir, "Cargo.toml"),
				LockFile: hasLock,
			})
		}
		// Check for Ruby dependencies
		if _, err := os.Stat(filepath.Join(dir, "Gemfile")); err == nil {
			hasLock := false
			if _, err := os.Stat(filepath.Join(dir, "Gemfile.lock")); err == nil {
				hasLock = true
			}
			results = append(results, projectType{
				Language: "ruby",
				DepFile:  filepath.Join(dir, "Gemfile"),
				LockFile: hasLock,
			})
		}
	})

	return results
}

// walkWithDepth walks directories up to maxDepth levels.
func (te *ToolExecutor) walkWithDepth(baseDir string, currentDepth int, maxDepth int, callback func(dir string)) {
	if currentDepth > maxDepth {
		return
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return
	}

	callback(baseDir)

	for _, entry := range entries {
		if entry.IsDir() {
			// Skip hidden directories and common non-project dirs
			name := entry.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == ".git" {
				continue
			}
			te.walkWithDepth(filepath.Join(baseDir, name), currentDepth+1, maxDepth, callback)
		}
	}
}

// goDependency represents a Go module dependency.
type goDependency struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	LockFile bool   `json:"lock_file_present"`
	Issues   []string `json:"issues,omitempty"`
}

// auditGoDependencies parses go.mod and reports on Go dependencies.
func (te *ToolExecutor) auditGoDependencies(targetDir string, checkOutdated bool) *ToolResult {
	// Find go.mod file
	goModPath := filepath.Join(targetDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		// Try to find go.mod in subdirectories
		var foundPath string
		te.walkWithDepth(targetDir, 0, 3, func(dir string) {
			if foundPath == "" {
				if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
					foundPath = filepath.Join(dir, "go.mod")
				}
			}
		})
		if foundPath != "" {
			goModPath = foundPath
		} else {
			return &ToolResult{
				Success: false,
				Error:   "go.mod not found in project directory",
			}
		}
	}

	// Parse go.mod
	dependencies, err := te.parseGoMod(goModPath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to parse go.mod: %v", err),
		}
	}

	// Check go.sum for lock file status
	lockFilePath := filepath.Join(filepath.Dir(goModPath), "go.sum")
	hasLockFile, _ := os.Stat(lockFilePath)
	lockFilePresent := hasLockFile == nil

	// Build results
	var depResults []interface{}
	var issues int

	for _, dep := range dependencies {
		depResult := map[string]interface{}{
			"name":           dep.Name,
			"version":        dep.Version,
			"lock_file":      lockFilePresent,
			"lock_file_path": lockFilePath,
		}

		// Check for issues
		var depIssues []string
		if !lockFilePresent {
			depIssues = append(depIssues, "no go.sum file (lock file missing)")
			issues++
		} else {
			// Verify go.sum has entries for this module
			sumContent, _ := os.ReadFile(lockFilePath)
			if !bytes.Contains(sumContent, []byte(dep.Name+" "+dep.Version+"\n")) &&
				!bytes.Contains(sumContent, []byte(dep.Name+" v")) {
				depIssues = append(depIssues, "version mismatch between go.mod and go.sum")
				issues++
			}
		}

		if checkOutdated {
			// Note: Real version checking would require network access
			// For now, just note it was requested
			depResult["check_outdated"] = true
		}

		if len(depIssues) > 0 {
			depResult["issues"] = depIssues
		}

		depResults = append(depResults, depResult)
	}

	output := fmt.Sprintf("Go project dependencies (%d total):\n", len(dependencies))
	for _, dep := range dependencies {
		output += fmt.Sprintf("  %s %s\n", dep.Name, dep.Version)
	}
	if issues > 0 {
		output += fmt.Sprintf("\nIssues found: %d\n", issues)
	}

	return &ToolResult{
		Success: true,
		Output:  output,
		Extra: map[string]interface{}{
			"projectType":     "go",
			"dependencies":    depResults,
			"totalDeps":       len(dependencies),
			"lockFilePresent": lockFilePresent,
			"issuesCount":     issues,
		},
	}
}

// parseGoMod parses a go.mod file and extracts dependencies.
func (te *ToolExecutor) parseGoMod(goModPath string) ([]goDependency, error) {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, err
	}

	var deps []goDependency
	inRequireBlock := false

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Handle require block
		if strings.HasPrefix(line, "require (") {
			inRequireBlock = true
			continue
		}
		if line == ")" && inRequireBlock {
			inRequireBlock = false
			continue
		}

		// Parse single-line require
		if strings.HasPrefix(line, "require ") {
			parts := strings.Fields(strings.TrimPrefix(line, "require "))
			if len(parts) >= 2 {
				deps = append(deps, goDependency{
					Name:    parts[0],
					Version: parts[1],
				})
			}
			continue
		}

		// Parse inside require block
		if inRequireBlock {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				version := parts[1]
				// Handle indirect dependencies
				for i, p := range parts {
					if p == "// indirect" {
						version = parts[1] // Still valid
						_ = i
						break
					}
				}
				deps = append(deps, goDependency{
					Name:    parts[0],
					Version: version,
				})
			}
		}
	}

	return deps, nil
}

// nodeDependency represents a Node.js package dependency.
type nodeDependency struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	LockFile bool   `json:"lock_file_present"`
	Issues   []string `json:"issues,omitempty"`
}

// auditNodeDependencies parses package.json and reports on Node.js dependencies.
func (te *ToolExecutor) auditNodeDependencies(targetDir string, checkOutdated bool) *ToolResult {
	// Find package.json
	pkgPath := filepath.Join(targetDir, "package.json")
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		// Try to find package.json in subdirectories
		var foundPath string
		te.walkWithDepth(targetDir, 0, 3, func(dir string) {
			if foundPath == "" {
				if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
					foundPath = filepath.Join(dir, "package.json")
				}
			}
		})
		if foundPath != "" {
			pkgPath = foundPath
		} else {
			return &ToolResult{
				Success: false,
				Error:   "package.json not found in project directory",
			}
		}
	}

	// Parse package.json
	dependencies, err := te.parsePackageJson(pkgPath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to parse package.json: %v", err),
		}
	}

	// Check for lock files
	pkgDir := filepath.Dir(pkgPath)
	lockFiles := map[string]string{
		"package-lock.json": "npm lockfile",
		"yarn.lock":         "yarn lockfile",
		"pnpm-lock.yaml":    "pnpm lockfile",
	}

	var depResults []interface{}
	var issues int

	// Build flat list of all deps
	var allDeps []map[string]interface{}
	for _, dep := range dependencies.Dependencies {
		allDeps = append(allDeps, map[string]interface{}{"name": dep.Name, "version": dep.Version})
	}
	for _, dep := range dependencies.DependenciesOn {
		allDeps = append(allDeps, map[string]interface{}{"name": dep.Name, "version": dep.Version})
	}
	for _, dep := range dependencies.DevDependencies {
		allDeps = append(allDeps, map[string]interface{}{"name": dep.Name, "version": dep.Version})
	}

	lockFilePresent := false
	var lockFilePath string
	for lf, _ := range lockFiles {
		if _, err := os.Stat(filepath.Join(pkgDir, lf)); err == nil {
			lockFilePresent = true
			lockFilePath = filepath.Join(pkgDir, lf)
			break
		}
	}

	for _, dep := range allDeps {
		depResult := map[string]interface{}{
			"name":      dep["name"],
			"version":   dep["version"],
			"lock_file": lockFilePresent,
		}

		var depIssues []string
		if !lockFilePresent {
			depIssues = append(depIssues, "no lock file present")
			issues++
		}

		if len(depIssues) > 0 {
			depResult["issues"] = depIssues
		}

		depResults = append(depResults, depResult)
	}

	output := fmt.Sprintf("Node.js project dependencies (%d total):\n", len(allDeps))
	if len(dependencies.Dependencies) > 0 {
		output += "  Dependencies:\n"
		for _, dep := range dependencies.Dependencies {
			output += fmt.Sprintf("    %s@%s\n", dep.Name, dep.Version)
		}
	}
	if len(dependencies.DependenciesOn) > 0 {
		output += "  Dependencies On:\n"
		for _, dep := range dependencies.DependenciesOn {
			output += fmt.Sprintf("    %s@%s\n", dep.Name, dep.Version)
		}
	}
	if len(dependencies.DevDependencies) > 0 {
		output += "  Dev Dependencies:\n"
		for _, dep := range dependencies.DevDependencies {
			output += fmt.Sprintf("    %s@%s\n", dep.Name, dep.Version)
		}
	}
	if issues > 0 {
		output += fmt.Sprintf("\nIssues found: %d\n", issues)
	}

	return &ToolResult{
		Success: true,
		Output:  output,
		Extra: map[string]interface{}{
			"projectType":     "nodejs",
			"dependencies":    depResults,
			"totalDeps":       len(allDeps),
			"lockFilePresent": lockFilePresent,
			"lockFilePath":    lockFilePath,
			"issuesCount":     issues,
		},
	}
}

// packageJson represents a parsed package.json file.
type packageJson struct {
	Dependencies      []nodeDependency
	DependenciesOn    []nodeDependency
	DevDependencies   []nodeDependency
}

// parsePackageJson parses a package.json file and extracts dependencies.
func (te *ToolExecutor) parsePackageJson(pkgPath string) (*packageJson, error) {
	content, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil, err
	}

	var raw struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		PeerDependencies map[string]string `json:"peerDependencies"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
	}

	if err := json.Unmarshal(content, &raw); err != nil {
		return nil, err
	}

	var result packageJson

	for name, version := range raw.Dependencies {
		result.Dependencies = append(result.Dependencies, nodeDependency{
			Name:    name,
			Version: version,
		})
	}
	for name, version := range raw.PeerDependencies {
		result.DependenciesOn = append(result.DependenciesOn, nodeDependency{
			Name:    name,
			Version: version,
		})
	}
	for name, version := range raw.OptionalDependencies {
		result.DependenciesOn = append(result.DependenciesOn, nodeDependency{
			Name:    name,
			Version: version,
		})
	}
	for name, version := range raw.DevDependencies {
		result.DevDependencies = append(result.DevDependencies, nodeDependency{
			Name:    name,
			Version: version,
		})
	}

	return &result, nil
}

// pythonDependency represents a Python package dependency.
type pythonDependency struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Issues   []string `json:"issues,omitempty"`
}

// auditPythonDependencies parses requirements.txt and reports on Python dependencies.
func (te *ToolExecutor) auditPythonDependencies(targetDir string, checkOutdated bool) *ToolResult {
	// Find requirements.txt
	reqPath := filepath.Join(targetDir, "requirements.txt")
	if _, err := os.Stat(reqPath); os.IsNotExist(err) {
		// Try Pipfile
		pipfilePath := filepath.Join(targetDir, "Pipfile")
		if _, err := os.Stat(pipfilePath); err == nil {
			return te.auditPythonPipfile(pipfilePath)
		}
		// Try pyproject.toml
		pyprojectPath := filepath.Join(targetDir, "pyproject.toml")
		if _, err := os.Stat(pyprojectPath); err == nil {
			return te.auditPythonPyproject(pyprojectPath)
		}
		// Try to find requirements.txt in subdirectories
		var foundPath string
		te.walkWithDepth(targetDir, 0, 3, func(dir string) {
			if foundPath == "" {
				if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil {
					foundPath = filepath.Join(dir, "requirements.txt")
				}
			}
		})
		if foundPath != "" {
			reqPath = foundPath
		} else {
			return &ToolResult{
				Success: false,
				Error:   "No Python dependency files found (requirements.txt, Pipfile, or pyproject.toml)",
			}
		}
	}

	// Parse requirements.txt
	dependencies, err := te.parseRequirementsTxt(reqPath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to parse requirements.txt: %v", err),
		}
	}

	var depResults []interface{}
	var issues int

	for _, dep := range dependencies {
		depResult := map[string]interface{}{
			"name":    dep.Name,
			"version": dep.Version,
		}

		var depIssues []string
		if dep.Version == "" {
			depIssues = append(depIssues, "no version pinned (may cause reproducibility issues)")
		}
		if strings.Contains(dep.Version, ">=") || strings.Contains(dep.Version, "!=") {
			depIssues = append(depIssues, "flexible version constraint (may cause unexpected updates)")
		}

		if len(depIssues) > 0 {
			depIssues = append(depIssues, "no lock file")
			depIssues = append(depIssues, "no lock file")
			issues++
		}

		if len(depIssues) > 0 {
			depResult["issues"] = depIssues
		}

		depResults = append(depResults, depResult)
	}

	output := fmt.Sprintf("Python project dependencies (%d total):\n", len(dependencies))
	for _, dep := range dependencies {
		output += fmt.Sprintf("  %s%s\n", dep.Name, dep.Version)
	}
	if issues > 0 {
		output += fmt.Sprintf("\nIssues found: %d\n", issues)
	}

	return &ToolResult{
		Success: true,
		Output:  output,
		Extra: map[string]interface{}{
			"projectType": "python",
			"dependencies": depResults,
			"totalDeps":   len(dependencies),
			"issuesCount": issues,
		},
	}
}

// parseRequirementsTxt parses a requirements.txt file.
func (te *ToolExecutor) parseRequirementsTxt(path string) ([]pythonDependency, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var deps []pythonDependency
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines, comments, and options
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}

		// Parse dependency: name[extras]version
		// Examples: flask==2.3.0, requests>=2.28.0, numpy, package[extra1,extra2]>=1.0
		var name, version string

		// Extract name and version
		if strings.Contains(line, "==") {
			parts := strings.SplitN(line, "==", 2)
			name = strings.TrimSpace(parts[0])
			version = "==" + strings.TrimSpace(parts[1])
		} else if strings.Contains(line, ">=") {
			parts := strings.SplitN(line, ">=", 2)
			name = strings.TrimSpace(parts[0])
			version = ">=" + strings.TrimSpace(parts[1])
		} else if strings.Contains(line, "<=") {
			parts := strings.SplitN(line, "<=", 2)
			name = strings.TrimSpace(parts[0])
			version = "<=" + strings.TrimSpace(parts[1])
		} else if strings.Contains(line, "!=") {
			parts := strings.SplitN(line, "!=", 2)
			name = strings.TrimSpace(parts[0])
			version = "!=" + strings.TrimSpace(parts[1])
		} else if strings.Contains(line, ">") {
			parts := strings.SplitN(line, ">", 2)
			name = strings.TrimSpace(parts[0])
			version = ">" + strings.TrimSpace(parts[1])
		} else if strings.Contains(line, "<") {
			parts := strings.SplitN(line, "<", 2)
			name = strings.TrimSpace(parts[0])
			version = "<" + strings.TrimSpace(parts[1])
		} else {
			// Just a package name, no version
			// Handle extras: package[extra]
			name = line
			version = ""
		}

		// Clean up package name (remove extras)
		name = strings.TrimSpace(name)
		if idx := strings.Index(name, "["); idx >= 0 {
			name = name[:idx]
		}

		if name != "" {
			deps = append(deps, pythonDependency{
				Name:    name,
				Version: version,
			})
		}
	}

	return deps, nil
}

// auditPythonPipfile audits a Pipfile for Python dependencies.
func (te *ToolExecutor) auditPythonPipfile(pipfilePath string) *ToolResult {
	// Read Pipfile
	content, err := os.ReadFile(pipfilePath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read Pipfile: %v", err),
		}
	}

	// Simple TOML-like parsing for Pipfile sections
	dependencies, devDeps := te.parsePipfile(content)

	output := "Python project dependencies (Pipfile):\n"
	if len(dependencies) > 0 {
		output += "  Dependencies:\n"
		for name, ver := range dependencies {
			output += fmt.Sprintf("    %s%s\n", name, ver)
		}
	}
	if len(devDeps) > 0 {
		output += "  Dev Dependencies:\n"
		for name, ver := range devDeps {
			output += fmt.Sprintf("    %s%s\n", name, ver)
		}
	}

	return &ToolResult{
		Success: true,
		Output:  output,
		Extra: map[string]interface{}{
			"projectType":  "python",
			"totalDeps":    len(dependencies) + len(devDeps),
			"pipfilePath":  pipfilePath,
		},
	}
}

// parsePipfile does basic parsing of a Pipfile to extract dependencies.
func (te *ToolExecutor) parsePipfile(content []byte) (map[string]string, map[string]string) {
	dependencies := make(map[string]string)
	devDeps := make(map[string]string)

	inDependencies := false
	inDevDependencies := false

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "[packages]" {
			inDependencies = true
			inDevDependencies = false
			continue
		}
		if line == "[dev-packages]" {
			inDependencies = false
			inDevDependencies = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inDependencies = false
			inDevDependencies = false
			continue
		}

		// Parse key = value pairs
		if (inDependencies || inDevDependencies) && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				// Remove quotes
				value = strings.Trim(value, "\"'")

				if inDependencies {
					dependencies[name] = value
				} else if inDevDependencies {
					devDeps[name] = value
				}
			}
		}
	}

	return dependencies, devDeps
}

// auditPythonPyproject audits a pyproject.toml for Python dependencies.
func (te *ToolExecutor) auditPythonPyproject(pyprojectPath string) *ToolResult {
	content, err := os.ReadFile(pyprojectPath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read pyproject.toml: %v", err),
		}
	}

	// Check for Poetry dependencies
	var deps []pythonDependency
	var depsSection string

	if strings.Contains(string(content), "[tool.poetry.dependencies]") || strings.Contains(string(content), "[project.dependencies]") {
		if strings.Contains(string(content), "[tool.poetry.dependencies]") {
			depsSection = "poetry"
		} else {
			depsSection = "project"
		}
	} else {
		depsSection = "generic"
	}

	// Parse dependencies based on section type
	if depsSection == "poetry" {
		deps = te.parsePoetryDeps(content)
	} else {
		// Extract inline dependency list
		deps = te.parseInlineDeps(content)
	}

	output := fmt.Sprintf("Python project dependencies (pyproject.toml, %s style):\n", depsSection)
	for _, dep := range deps {
		output += fmt.Sprintf("  %s%s\n", dep.Name, dep.Version)
	}

	return &ToolResult{
		Success: true,
		Output:  output,
		Extra: map[string]interface{}{
			"projectType":     "python",
			"dependencies":    deps,
			"totalDeps":       len(deps),
			"pyprojectStyle":  depsSection,
		},
	}
}

// parsePoetryDeps parses Poetry-style dependencies from pyproject.toml.
func (te *ToolExecutor) parsePoetryDeps(content []byte) []pythonDependency {
	var deps []pythonDependency
	lines := strings.Split(string(content), "\n")
	inPoetryDeps := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "[tool.poetry.dependencies]" {
			inPoetryDeps = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inPoetryDeps = false
			continue
		}

		if inPoetryDeps && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				value = strings.Trim(value, "\"'")
				if name != "python" { // Skip Python version constraint
					deps = append(deps, pythonDependency{
						Name:    name,
						Version: value,
					})
				}
			}
		}
	}

	return deps
}

// parseInlineDeps parses PEP 621 inline dependency list from pyproject.toml.
func (te *ToolExecutor) parseInlineDeps(content []byte) []pythonDependency {
	var deps []pythonDependency

	// Look for dependency lines in [project] section
	inProject := false
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "[project]" || line == "[project]\n" {
			inProject = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inProject = false
			continue
		}

		if inProject && strings.HasPrefix(line, "dependencies = [") {
			// Multi-line or single-line array
			// Extract dependencies from the array
			arrayContent := line
			if strings.HasSuffix(strings.TrimSpace(line), "],") || strings.HasSuffix(strings.TrimSpace(line), "]") {
				// Single line
				arrayContent = strings.TrimPrefix(line, "dependencies = [")
				arrayContent = strings.TrimSuffix(arrayContent, "]")
				arrayContent = strings.TrimSuffix(arrayContent, ",")
				depList := strings.Split(arrayContent, ",")
				for _, d := range depList {
					d = strings.TrimSpace(d)
					if d != "" {
						dep := pythonDependency{
							Name:    strings.TrimSpace(d),
							Version: "",
						}
						if idx := strings.Index(dep.Name, ">="); idx >= 0 {
							dep.Version = dep.Name[idx:]
							dep.Name = strings.TrimSpace(dep.Name[:idx])
						} else if idx := strings.Index(dep.Name, "=="); idx >= 0 {
							dep.Version = dep.Name[idx:]
							dep.Name = strings.TrimSpace(dep.Name[:idx])
						}
						dep.Name = strings.Trim(dep.Name, "\"'")
						deps = append(deps, dep)
					}
				}
			}
			// Note: multi-line arrays would need more sophisticated parsing
			// For now, we handle the common single-line case
			break
		}
	}

	return deps
}

// rustDependency represents a Rust crate dependency.
type rustDependency struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	LockFile bool   `json:"lock_file_present"`
	Issues   []string `json:"issues,omitempty"`
}

// auditRustDependencies parses Cargo.toml and reports on Rust dependencies.
func (te *ToolExecutor) auditRustDependencies(targetDir string, checkOutdated bool) *ToolResult {
	// Find Cargo.toml
	cargoPath := filepath.Join(targetDir, "Cargo.toml")
	if _, err := os.Stat(cargoPath); os.IsNotExist(err) {
		var foundPath string
		te.walkWithDepth(targetDir, 0, 3, func(dir string) {
			if foundPath == "" {
				if _, err := os.Stat(filepath.Join(dir, "Cargo.toml")); err == nil {
					foundPath = filepath.Join(dir, "Cargo.toml")
				}
			}
		})
		if foundPath != "" {
			cargoPath = foundPath
		} else {
			return &ToolResult{
				Success: false,
				Error:   "Cargo.toml not found in project directory",
			}
		}
	}

	// Parse Cargo.toml
	dependencies, err := te.parseCargoToml(cargoPath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to parse Cargo.toml: %v", err),
		}
	}

	// Check for Cargo.lock
	cargoDir := filepath.Dir(cargoPath)
	lockPath := filepath.Join(cargoDir, "Cargo.lock")
	hasLockFile := false
	if _, err := os.Stat(lockPath); err == nil {
		hasLockFile = true
	}

	var depResults []interface{}
	var issues int

	for _, dep := range dependencies {
		depResult := map[string]interface{}{
			"name":      dep.Name,
			"version":   dep.Version,
			"lock_file": hasLockFile,
		}

		var depIssues []string
		if !hasLockFile {
			depIssues = append(depIssues, "no Cargo.lock file (lock file missing)")
			issues++
		} else {
			// Verify lock file has this crate
			lockContent, _ := os.ReadFile(lockPath)
			if !bytes.Contains(lockContent, []byte("name = \""+dep.Name+"\"")) {
				depIssues = append(depIssues, "version mismatch between Cargo.toml and Cargo.lock")
				issues++
			}
		}

		if len(depIssues) > 0 {
			depResult["issues"] = depIssues
		}

		depResults = append(depResults, depResult)
	}

	output := fmt.Sprintf("Rust project dependencies (%d total):\n", len(dependencies))
	for _, dep := range dependencies {
		output += fmt.Sprintf("  %s %s\n", dep.Name, dep.Version)
	}
	if issues > 0 {
		output += fmt.Sprintf("\nIssues found: %d\n", issues)
	}

	return &ToolResult{
		Success: true,
		Output:  output,
		Extra: map[string]interface{}{
			"projectType":     "rust",
			"dependencies":    depResults,
			"totalDeps":       len(dependencies),
			"lockFilePresent": hasLockFile,
			"lockFilePath":    lockPath,
			"issuesCount":     issues,
		},
	}
}

// parseCargoToml parses a Cargo.toml file and extracts dependencies.
func (te *ToolExecutor) parseCargoToml(cargoPath string) ([]rustDependency, error) {
	content, err := os.ReadFile(cargoPath)
	if err != nil {
		return nil, err
	}

	var deps []rustDependency
	inDependencies := false
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "[dependencies]" {
			inDependencies = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inDependencies = false
			continue
		}

		if inDependencies && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				value = strings.Trim(value, "\"'")

				// Handle different dependency formats:
				// simple: name = "1.0.0"
				// table: name = { version = "1.0.0", features = ["feature1"] }
				var version string
				if strings.HasPrefix(value, "{") {
					// Table format - extract version
					if strings.Contains(value, "version") {
						verParts := strings.Split(value, "version")
						if len(verParts) > 1 {
							ver := strings.Trim(verParts[1], " =\"'")
							ver = strings.SplitN(ver, ",", 2)[0]
							version = ver
						}
					} else {
						version = "unspecified (table format)"
					}
				} else {
					// Simple version format
					version = value
				}

				deps = append(deps, rustDependency{
					Name:    name,
					Version: version,
				})
			}
		}
	}

	return deps, nil
}

// rubyDependency represents a Ruby gem dependency.
type rubyDependency struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	LockFile bool   `json:"lock_file_present"`
	Issues   []string `json:"issues,omitempty"`
}

// auditRubyDependencies parses Gemfile and reports on Ruby dependencies.
func (te *ToolExecutor) auditRubyDependencies(targetDir string, checkOutdated bool) *ToolResult {
	// Find Gemfile
	gemPath := filepath.Join(targetDir, "Gemfile")
	if _, err := os.Stat(gemPath); os.IsNotExist(err) {
		var foundPath string
		te.walkWithDepth(targetDir, 0, 3, func(dir string) {
			if foundPath == "" {
				if _, err := os.Stat(filepath.Join(dir, "Gemfile")); err == nil {
					foundPath = filepath.Join(dir, "Gemfile")
				}
			}
		})
		if foundPath != "" {
			gemPath = foundPath
		} else {
			return &ToolResult{
				Success: false,
				Error:   "Gemfile not found in project directory",
			}
		}
	}

	// Parse Gemfile
	dependencies, err := te.parseGemfile(gemPath)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to parse Gemfile: %v", err),
		}
	}

	// Check for Gemfile.lock
	gemDir := filepath.Dir(gemPath)
	lockPath := filepath.Join(gemDir, "Gemfile.lock")
	hasLockFile := false
	if _, err := os.Stat(lockPath); err == nil {
		hasLockFile = true
	}

	var depResults []interface{}
	var issues int

	for _, dep := range dependencies {
		depResult := map[string]interface{}{
			"name":      dep.Name,
			"version":   dep.Version,
			"lock_file": hasLockFile,
		}

		var depIssues []string
		if !hasLockFile {
			depIssues = append(depIssues, "no Gemfile.lock file (lock file missing)")
			issues++
		} else {
			// Verify lock file has this gem
			lockContent, _ := os.ReadFile(lockPath)
			// Gemfile.lock has sections like "  name (version)"
			// We need to check if the gem is listed under "dependencies:"
			locked := bytes.Contains(lockContent, []byte("  "+dep.Name+" ("))
			if !locked {
				depIssues = append(depIssues, "version mismatch between Gemfile and Gemfile.lock")
				issues++
			}
		}

		if len(depIssues) > 0 {
			depResult["issues"] = depIssues
		}

		depResults = append(depResults, depResult)
	}

	output := fmt.Sprintf("Ruby project dependencies (%d total):\n", len(dependencies))
	for _, dep := range dependencies {
		output += fmt.Sprintf("  %s %s\n", dep.Name, dep.Version)
	}
	if issues > 0 {
		output += fmt.Sprintf("\nIssues found: %d\n", issues)
	}

	return &ToolResult{
		Success: true,
		Output:  output,
		Extra: map[string]interface{}{
			"projectType":     "ruby",
			"dependencies":    depResults,
			"totalDeps":       len(dependencies),
			"lockFilePresent": hasLockFile,
			"lockFilePath":    lockPath,
			"issuesCount":     issues,
		},
	}
}

// parseGemfile parses a Gemfile and extracts gem dependencies.
func (te *ToolExecutor) parseGemfile(gemPath string) ([]rubyDependency, error) {
	content, err := os.ReadFile(gemPath)
	if err != nil {
		return nil, err
	}

	var deps []rubyDependency
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Match gem declarations: gem 'name', 'version' or gem "name", "~> version"
		// Also match: gem 'name' (no version)
		if strings.HasPrefix(line, "gem ") {
			// Extract content after "gem "
			args := strings.TrimSpace(line[4:])
			// Remove trailing comment
			if idx := strings.Index(args, "#"); idx >= 0 {
				args = args[:idx]
			}
			args = strings.TrimSpace(args)

			// Parse: 'name', 'version' or "name", "version"
			if strings.Contains(args, ",") {
				parts := strings.SplitN(args, ",", 2)
				name := strings.Trim(strings.TrimSpace(parts[0]), "'\"")
				version := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
				if name != "" {
					deps = append(deps, rubyDependency{
						Name:    name,
						Version: version,
					})
				}
			} else {
				// Single argument: gem 'name'
				name := strings.Trim(args, "'\"")
				if name != "" {
					deps = append(deps, rubyDependency{
						Name:    name,
						Version: "",
					})
				}
			}
		}
	}

	return deps, nil
}

