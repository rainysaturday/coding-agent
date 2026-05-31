// Package tools implements the tool execution system for the coding agent.
package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
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
	stats    *Stats
	readOnly bool
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

// SetReadOnly sets whether the executor is in read-only mode.
// In read-only mode, only read_file and list_files tools are allowed.
func (te *ToolExecutor) SetReadOnly(readOnly bool) {
	te.readOnly = readOnly
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

// Execute executes a tool call with context support for cancellation.
// If the context is cancelled during execution, the tool will be interrupted
// and a cancellation result will be returned.
func (te *ToolExecutor) Execute(ctx context.Context, tc *ToolCall) *ToolResult {
	te.stats.TotalCalls++

	// Check if tool is allowed in read-only mode
	if te.readOnly && !isReadOnlyTool(tc.Name) {
		te.stats.FailedCalls++
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Tool '%s' is not available in read-only mode", tc.Name),
			Extra: map[string]interface{}{
				"tool_name": tc.Name,
			},
		}
	}

	var result *ToolResult

	switch tc.Name {
	case "bash":
		result = te.executeBash(ctx, tc.Parameters)
	case "read_file":
		result = te.executeReadFile(tc.Parameters)
	case "write_file":
		result = te.executeWriteFile(tc.Parameters)
	case "read_lines":
		result = te.executeReadLines(tc.Parameters)
	case "insert_lines":
		result = te.executeInsertLines(tc.Parameters)
	case "replace_text":
		result = te.executeReplaceText(tc.Parameters)
	case "list_files":
		result = te.executeListFiles(ctx, tc.Parameters)
	case "grep":
		result = te.executeGrep(ctx, tc.Parameters)
	case "git_log":
		result = te.executeGitLog(ctx, tc.Parameters)
	case "git_show":
		result = te.executeGitShow(ctx, tc.Parameters)
	case "git_diff":
		result = te.executeGitDiff(ctx, tc.Parameters)
	case "subagent":
		result = ExecuteSubagent(tc.Parameters)
	case "view_image":
		result = te.executeViewImage(tc.Parameters)
	default:
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

// isReadOnlyTool checks if a tool is allowed in read-only mode.
// read_file, list_files, read_lines, grep, git_log, git_show, and view_image are safe read-only operations.
func isReadOnlyTool(name string) bool { //nolint:funlen
	return name == "read_file" || name == "list_files" || name == "read_lines" || name == "grep" || name == "git_log" || name == "git_show" || name == "git_diff" || name == "view_image"
}

// Default timeout for bash commands in milliseconds
const defaultBashTimeoutMs = 30000

// isCancelled returns true if the operation was cancelled by the user.
func isCancelled(err error) bool {
	return err == context.Canceled
}

// executeBash executes a bash command with context support for cancellation.
func (te *ToolExecutor) executeBash(ctx context.Context, params map[string]interface{}) *ToolResult {
	command, ok := params["command"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: command",
		}
	}

	// Parse optional timeout parameter (in milliseconds), default to 30 seconds
	timeoutMs := defaultBashTimeoutMs
	if timeoutParam, hasTimeout := params["timeout"]; hasTimeout {
		switch v := timeoutParam.(type) {
		case float64:
			timeoutMs = int(v)
		case int:
			timeoutMs = v
		case string:
			if t, err := strconv.Atoi(v); err == nil && t > 0 {
				timeoutMs = t
			}
		}
	}

	// Ensure timeout is positive
	if timeoutMs <= 0 {
		timeoutMs = defaultBashTimeoutMs
	}

	// Create a child context that respects both the parent cancellation and the timeout
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	// Channel to receive command result
	type cmdResult struct {
		output []byte
		err    error
	}
	resultChan := make(chan cmdResult, 1)

	go func() {
		cmd := exec.CommandContext(ctx, "bash", "-c", command)
		output, err := cmd.CombinedOutput()
		resultChan <- cmdResult{output: output, err: err}
	}()

	// Wait for either completion, timeout, or cancellation
	select {
	case <-ctx.Done():
		// Timeout or cancellation occurred
		if ctx.Err() == context.DeadlineExceeded {
			return &ToolResult{
				Success:  false,
				ExitCode: 124, // Convention: 124 for timeout (like GNU timeout)
				Error:    fmt.Sprintf("command timed out after %dms (timeout exceeded). The command did not complete within the specified timeout period. Consider increasing the timeout parameter (in milliseconds) if the command needs more time, or optimizing the command to run faster.", timeoutMs),
			}
		}
		if isCancelled(ctx.Err()) {
			return &ToolResult{
				Success:  false,
				ExitCode: 130, // Convention: 130 for SIGINT (like bash)
				Error:    "command was cancelled by the user",
			}
		}
		return &ToolResult{
			Success:  false,
			ExitCode: 1,
			Error:    fmt.Sprintf("command failed with context error: %v", ctx.Err()),
		}
	case res := <-resultChan:
		// Extract exit code
		exitCode := 0
		if res.err != nil {
			if exitError, ok := res.err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			}
		}

		result := &ToolResult{
			ExitCode: exitCode,
		}

		if res.err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("command failed: %v\nOutput: %s", res.err, string(res.output))
		} else {
			result.Success = true
			result.Output = string(res.output)
		}

		return result
	}
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
			"linesRead":     countLines(string(content)),
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
		Output:  fmt.Sprintf("File written: %s (%d bytes)\n--- Content preview ---\n%s", path, len(content), truncateOutput(content, 20)),
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

	startVal, ok := params["start"].(float64)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "parameter 'start' must be a number",
		}
	}

	endVal, ok := params["end"].(float64)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "parameter 'end' must be a number",
		}
	}

	startLine := int(startVal)
	endLine := int(endVal)

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
		Output:  fmt.Sprintf("Inserted %d line(s) at line %d in: %s\n--- Content inserted ---\n%s", len(newLines), insertLine, path, truncateOutput(insertLines, 10)),
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

	if searchText == "" {
		return &ToolResult{
			Success: false,
			Error:   "search text cannot be empty",
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
		Output:  fmt.Sprintf("Replaced '%s' with '%s' %d time(s) in: %s\n--- Preview ---\n%s", truncateString(searchText, 30), truncateString(replaceText, 30), replacementsMade, path, previewReplacement(originalContent, searchText, replaceText, 5)),
		Path:    path,
		Extra: map[string]interface{}{
			"search":           searchText,
			"replacementsMade": replacementsMade,
			"totalOccurrences": totalOccurrences,
		},
	}
}

// executeListFiles lists files and directories, formatted like ls.
// Supports context cancellation and various flags similar to ls.
func (te *ToolExecutor) executeListFiles(ctx context.Context, params map[string]interface{}) *ToolResult {
	path := "."
	if p, ok := params["path"].(string); ok && p != "" {
		path = p
	}

	// Parse flags
	flags := map[string]bool{
		"l": false,
		"a": false,
		"h": false,
		"t": false,
		"S": false,
		"r": false,
		"R": false,
	}

	if flagsParam, ok := params["flags"]; ok {
		switch v := flagsParam.(type) {
		case []interface{}:
			for _, f := range v {
				if flagStr, ok := f.(string); ok {
					if len(flagStr) == 1 {
						flags[flagStr] = true
					}
				}
			}
		case []string:
			for _, flagStr := range v {
				if len(flagStr) == 1 {
					flags[flagStr] = true
				}
			}
		}
	}

	// Check if path is a file or directory
	info, err := os.Stat(path)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, path),
		}
	}

	// If it's a file, return information about that single file
	if !info.IsDir() {
		if flags["l"] {
			line := formatFileLong(info, flags)
			return &ToolResult{
				Success: true,
				Output:  line,
				Extra: map[string]interface{}{
					"entriesListed": 1,
					"path":          path,
				},
			}
		}
		return &ToolResult{
			Success: true,
			Output:  info.Name(),
			Extra: map[string]interface{}{
				"entriesListed": 1,
				"path":          path,
			},
		}
	}

	var entries []os.DirEntry
	var output string

	// Handle recursive listing
	if flags["R"] {
		// Use filepath.Walk to get entries with relative paths
		var resultEntries []walkEntry
		walkErr := filepath.Walk(path, func(filePath string, fileInfo os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			// Skip .git directories
			if strings.Contains(filePath, "/.git/") || strings.HasSuffix(filePath, "/.git") {
				if fileInfo.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			// Skip hidden files/dirs unless "a" flag is set
			if !flags["a"] {
				baseName := fileInfo.Name()
				if strings.HasPrefix(baseName, ".") {
					if fileInfo.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			relPath, _ := filepath.Rel(path, filePath)
			resultEntries = append(resultEntries, walkEntry{
				path:     relPath,
				isDir:    fileInfo.IsDir(),
				info:     fileInfo,
				modTime:  fileInfo.ModTime(),
				fileSize: fileInfo.Size(),
			})
			return nil
		})
		if walkErr != nil {
			return &ToolResult{
				Success: false,
				Error:   formatFileError(walkErr, path),
			}
		}

		// Sort entries
		sort.Slice(resultEntries, func(i, j int) bool {
			// Directories first
			iIsDir := resultEntries[i].isDir
			jIsDir := resultEntries[j].isDir
			if iIsDir != jIsDir {
				return iIsDir
			}
			// Then by sort criteria
			switch {
			case flags["t"]:
				if flags["r"] {
					return resultEntries[i].modTime.Before(resultEntries[j].modTime)
				}
				return resultEntries[i].modTime.After(resultEntries[j].modTime)
			case flags["S"]:
				if flags["r"] {
					return resultEntries[i].fileSize < resultEntries[j].fileSize
				}
				return resultEntries[i].fileSize > resultEntries[j].fileSize
			default:
				if flags["r"] {
					return resultEntries[i].path > resultEntries[j].path
				}
				return resultEntries[i].path < resultEntries[j].path
			}
		})

		if flags["l"] {
			output = formatRecursiveLongList(resultEntries, path, flags)
		} else {
			var names []string
			for _, e := range resultEntries {
				name := e.path
				if e.isDir {
					name += "/"
				}
				names = append(names, name)
			}
			output = strings.Join(names, "\n")
		}

		return &ToolResult{
			Success: true,
			Output:  output,
			Extra: map[string]interface{}{
				"entriesListed": len(resultEntries),
				"path":          path,
			},
		}
	}

	// Read directory (non-recursive)
	entries, err = os.ReadDir(path)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, path),
		}
	}

	// Filter entries
	var filtered []os.DirEntry
	for _, entry := range entries {
		name := entry.Name()
		if !flags["a"] && strings.HasPrefix(name, ".") {
			continue
		}
		filtered = append(filtered, entry)
	}

	// Sort entries
	sort.Slice(filtered, func(i, j int) bool {
		// Directories first
		iIsDir := filtered[i].IsDir()
		jIsDir := filtered[j].IsDir()
		if iIsDir != jIsDir {
			return iIsDir
		}

		// Then by the specified sort criteria
		switch {
		case flags["t"]:
			iInfo, _ := filtered[i].Info()
			jInfo, _ := filtered[j].Info()
			if flags["r"] {
				return iInfo.ModTime().Before(jInfo.ModTime())
			}
			return iInfo.ModTime().After(jInfo.ModTime())
		case flags["S"]:
			iInfo, _ := filtered[i].Info()
			jInfo, _ := filtered[j].Info()
			if flags["r"] {
				return iInfo.Size() < jInfo.Size()
			}
			return iInfo.Size() > jInfo.Size()
		default:
			if flags["r"] {
				return filtered[i].Name() > filtered[j].Name()
			}
			return filtered[i].Name() < filtered[j].Name()
		}
	})

	// Format output
	if flags["l"] {
		output = formatLongList(filtered, flags)
	} else {
		output = formatSimpleList(filtered)
	}

	return &ToolResult{
		Success: true,
		Output:  output,
		Extra: map[string]interface{}{
			"entriesListed": len(filtered),
			"path":          path,
		},
	}
}

// formatSimpleList returns a simple one-per-line listing (like `ls`).
func formatSimpleList(entries []os.DirEntry) string {
	var lines []string
	for _, entry := range entries {
		if entry.IsDir() {
			lines = append(lines, entry.Name()+"/")
		} else {
			lines = append(lines, entry.Name())
		}
	}
	return strings.Join(lines, "\n")
}

// formatLongList returns a long-format listing (like `ls -l`).
func formatLongList(entries []os.DirEntry, flags map[string]bool) string {
	var lines []string
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		lines = append(lines, formatFileLong(info, flags))
	}
	return strings.Join(lines, "\n")
}

// formatFileLong returns a long-format line for a single file info.
func formatFileLong(info os.FileInfo, flags map[string]bool) string {
	// Permissions
	permStr := formatPermissions(info)

	// Size
	size := info.Size()
	var sizeStr string
	if flags["h"] {
		sizeStr = humanReadableSize(size)
	} else {
		sizeStr = fmt.Sprintf("%d", size)
	}

	// Modification time (format: "Jan 02 15:04" or "Jan 02 2006" if old)
	modTime := info.ModTime()
	now := time.Now()
	age := now.Sub(modTime)
	var timeStr string
	if age > 365*24*time.Hour {
		timeStr = modTime.Format("Jan 02  2006")
	} else {
		timeStr = modTime.Format("Jan 02 15:04")
	}

	// Name (with / suffix for directories)
	name := info.Name()
	if info.IsDir() {
		name += "/"
	}

	// Get ownership info via platform-specific function
	linkCount, owner, group := getFileInfoDetails(info.Sys())

	// Format: permissions links owner group size timestamp name
	return fmt.Sprintf("%s  %s  %s  %s  %s  %s  %s", permStr, linkCount, owner, group, sizeStr, timeStr, name)
}

// formatRecursiveLongList formats a list of walkEntries for recursive long-format output.
func formatRecursiveLongList(entries []walkEntry, basePath string, flags map[string]bool) string {
	var lines []string
	for _, e := range entries {
		// Permissions
		permStr := formatPermissionsRecursive(e)

		// Size
		var sizeStr string
		if flags["h"] {
			sizeStr = humanReadableSize(e.fileSize)
		} else {
			sizeStr = fmt.Sprintf("%d", e.fileSize)
		}

		// Modification time
		now := time.Now()
		age := now.Sub(e.modTime)
		var timeStr string
		if age > 365*24*time.Hour {
			timeStr = e.modTime.Format("Jan 02  2006")
		} else {
			timeStr = e.modTime.Format("Jan 02 15:04")
		}

		// Name (with / suffix for directories)
		name := e.path
		if e.isDir {
			name += "/"
		}

		lines = append(lines, fmt.Sprintf("%s  1  ?  ?  %s  %s  %s", permStr, sizeStr, timeStr, name))
	}
	return strings.Join(lines, "\n")
}

// formatPermissionsRecursive returns a Unix-style permission string for recursive listing.
func formatPermissionsRecursive(e walkEntry) string {
	mode := e.info.Mode()

	var fileType byte
	switch {
	case mode.IsDir():
		fileType = 'd'
	case mode&os.ModeSymlink != 0:
		fileType = 'l'
	case mode.IsRegular():
		fileType = '-'
	default:
		fileType = '-'
	}

	perm := mode.Perm()
	var permStr bytes.Buffer
	permStr.WriteByte(fileType)

	for _, bit := range []struct {
		set   string
		clear string
		mode  os.FileMode
	}{
		{"r", "-", 0400},
		{"w", "-", 0200},
		{"x", "-", 0100},
		{"r", "-", 0040},
		{"w", "-", 0020},
		{"x", "-", 0010},
		{"r", "-", 0004},
		{"w", "-", 0002},
		{"x", "-", 0001},
	} {
		if perm&bit.mode != 0 {
			permStr.WriteString(bit.set)
		} else {
			permStr.WriteString(bit.clear)
		}
	}

	return permStr.String()
}

// formatPermissions returns a Unix-style permission string.
func formatPermissions(info os.FileInfo) string {
	mode := info.Mode()

	// File type
	var fileType byte
	switch {
	case mode.IsDir():
		fileType = 'd'
	case mode&os.ModeSymlink != 0:
		fileType = 'l'
	case mode.IsRegular():
		fileType = '-'
	default:
		fileType = '-'
	}

	result := string(fileType)
	// Owner permissions
	result += formatTriple(uint8((mode >> 6) & 07))
	// Group permissions
	result += formatTriple(uint8((mode >> 3) & 07))
	// Other permissions
	result += formatTriple(uint8(mode & 07))
	return result
}

// formatTriple formats three permission bits as rwx.
func formatTriple(perm uint8) string {
	var s string
	if perm&4 != 0 {
		s += "r"
	} else {
		s += "-"
	}
	if perm&2 != 0 {
		s += "w"
	} else {
		s += "-"
	}
	if perm&1 != 0 {
		s += "x"
	} else {
		s += "-"
	}
	return s
}

// humanReadableSize converts a byte count to human-readable format.
func humanReadableSize(size int64) string {
	const (
		KB = 1 << 10
		MB = 1 << 20
		GB = 1 << 30
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.1fG", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.1fM", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.1fK", float64(size)/KB)
	default:
		return fmt.Sprintf("%d", size)
	}
}

// countLines counts the number of lines in text.
// A line is defined as text terminated by a newline character.
// An empty string has 0 lines. Text without a trailing newline still counts as a line.
// truncateOutput truncates text to a maximum number of lines for display purposes.
// It adds a "[truncated]" suffix if the content was truncated.
func truncateOutput(text string, maxLines int) string {
	if text == "" {
		return "(empty file)"
	}
	lines := strings.Split(text, "\n")
	if len(lines) > maxLines {
		return strings.Join(lines[:maxLines], "\n") + "\n... [content truncated]"
	}
	return text
}

// truncateString truncates a string to a maximum length, adding "..." if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// previewReplacement shows the first N lines of the content after replacement for verification.
func previewReplacement(original, search, replace string, maxLines int) string {
	lines := strings.Split(original, "\n")
	var result []string
	for _, line := range lines {
		if strings.Contains(line, search) {
			result = append(result, strings.Replace(line, search, replace, 1))
		} else {
			result = append(result, line)
		}
	}
	if len(result) > maxLines {
		return strings.Join(result[:maxLines], "\n") + "\n... [truncated]"
	}
	return strings.Join(result, "\n")
}
func countLines(text string) int {
	if text == "" {
		return 0
	}
	// Count newlines; if the text doesn't end with a newline, add one more
	count := strings.Count(text, "\n")
	if !strings.HasSuffix(text, "\n") {
		count++
	}
	return count
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

// walkEntry holds a directory entry with its metadata for recursive listing.
type walkEntry struct {
	path     string
	isDir    bool
	info     os.FileInfo
	modTime  time.Time
	fileSize int64
}

// matchResult represents a single grep match.
type matchResult struct {
	filePath string
	lineNum  int
	line     string
}

// executeGrep searches through file contents using grep-like pattern matching.
// Supports context cancellation, recursive search, and various grep-like flags.
func (te *ToolExecutor) executeGrep(ctx context.Context, params map[string]interface{}) *ToolResult {
	pattern, ok := params["pattern"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: pattern",
		}
	}
	if strings.TrimSpace(pattern) == "" {
		return &ToolResult{
			Success: false,
			Error:   "pattern cannot be empty",
		}
	}

	// Parse parameters and flags first (flags affect pattern handling and search)
	path := "."
	if p, ok := params["path"].(string); ok && p != "" {
		path = p
	}

	flags := map[string]bool{
		"i": false, // case-insensitive
		"r": false, // recursive
		"c": false, // count only
		"n": false, // line numbers
		"v": false, // invert match
		"l": false, // filenames only
		"a": false, // show all files including hidden
		"f": false, // use file as pattern source
	}

	if flagsParam, ok := params["flags"]; ok {
		switch v := flagsParam.(type) {
		case []interface{}:
			for _, f := range v {
				if flagStr, ok := f.(string); ok {
					if len(flagStr) == 1 {
						flags[flagStr] = true
					}
				}
			}
		case []string:
			for _, flagStr := range v {
				if len(flagStr) == 1 {
					flags[flagStr] = true
				}
			}
		}
	}

	// If "f" flag is set, use pattern as a file path containing patterns (one per line)
	if flags["f"] {
		patternContent, err := os.ReadFile(pattern)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("failed to read pattern file: %v", err),
			}
		}
		lines := strings.Split(strings.TrimSpace(string(patternContent)), "\n")
		var escaped []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				escaped = append(escaped, regexp.QuoteMeta(trimmed))
			}
		}
		if len(escaped) == 0 {
			return &ToolResult{
				Success: false,
				Error:   "pattern file is empty",
			}
		}
		pattern = strings.Join(escaped, "|")
	}

	// Compile the regex pattern.
	// If case-insensitive flag is set, compile with (?i) prefix
	// so the pattern itself handles case insensitivity.
	patternToCompile := pattern
	if flags["i"] {
		patternToCompile = "(?i)" + pattern
	}
	re, err := regexp.Compile(patternToCompile)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid regex pattern: %v", err),
		}
	}

	// Build a slice to collect results
	var results []matchResult
	var skipCount int
	var binaryCount int
	const maxResults = 5000

	// Check if path is a single file
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		// It's a file, search directly
		res, sc, bc := te.searchFile(path, re, flags, maxResults)
		skipCount += sc
		binaryCount += bc
		results = append(results, res...)
	} else if info != nil && info.IsDir() {
		// It's a directory, search recursively if flag is set
		if flags["r"] {
			err = filepath.Walk(path, func(filePath string, fileInfo os.FileInfo, walkErr error) error {
				// Check for cancellation
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				if walkErr != nil {
					return nil // Skip inaccessible files
				}
				// Skip hidden directories and files (unless "a" flag is set)
				if !flags["a"] {
					if strings.HasPrefix(fileInfo.Name(), ".") && strings.Count(filePath, "/") > 0 {
						if fileInfo.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
				}
				// Skip .git directory
				if strings.Contains(filePath, "/.git/") || strings.HasSuffix(filePath, "/.git") {
					if fileInfo.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if !fileInfo.IsDir() {
					if len(results) >= maxResults {
						return filepath.SkipDir
					}
					res, sc, bc := te.searchFile(filePath, re, flags, maxResults-len(results))
					skipCount += sc
					binaryCount += bc
					results = append(results, res...)
				}
				return nil
			})
		} else {
			// Non-recursive, just list files in the directory
			// Check for cancellation before reading directory
			select {
			case <-ctx.Done():
				return &ToolResult{
					Success: false,
					Error:   "operation was cancelled",
				}
			default:
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				return &ToolResult{
					Success: false,
					Error:   formatFileError(err, path),
				}
			}
			for _, entry := range entries {
				// Check for cancellation between files
				select {
				case <-ctx.Done():
					return &ToolResult{
						Success: false,
						Error:   "operation was cancelled",
					}
				default:
				}
				if entry.IsDir() {
					continue
				}
				fullPath := filepath.Join(path, entry.Name())
				if len(results) >= maxResults {
					break
				}
				res, sc, bc := te.searchFile(fullPath, re, flags, maxResults-len(results))
				skipCount += sc
				binaryCount += bc
				results = append(results, res...)
			}
		}
	} else {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("path not found: %s", path),
		}
	}

	// Build output
	var output strings.Builder

	// If filenames-only flag, just list matching files
	if flags["l"] {
		fileSet := make(map[string]bool)
		for _, r := range results {
			fileSet[r.filePath] = true
		}
		files := make([]string, 0, len(fileSet))
		for f := range fileSet {
			files = append(files, f)
		}
		sort.Strings(files)
		for _, f := range files {
			output.WriteString(f + "\n")
		}
	} else {
		// If count flag, show count per file
		if flags["c"] {
			countMap := make(map[string]int)
			for _, r := range results {
				countMap[r.filePath]++
			}
			files := make([]string, 0, len(countMap))
			for f := range countMap {
				files = append(files, f)
			}
			sort.Strings(files)
			for _, f := range files {
				output.WriteString(fmt.Sprintf("%s:%d\n", f, countMap[f]))
			}
		} else {
			// Show matching lines
			for _, r := range results {
				if flags["n"] {
					output.WriteString(fmt.Sprintf("%s:%d:%s\n", r.filePath, r.lineNum, r.line))
				} else {
					output.WriteString(fmt.Sprintf("%s:%s\n", r.filePath, r.line))
				}
			}
		}
	}

	resultStr := strings.TrimSuffix(output.String(), "\n")

	// Add info about skipped files
	var extraInfo strings.Builder
	if binaryCount > 0 {
		extraInfo.WriteString(fmt.Sprintf("\n[Skipped %d binary file(s)]", binaryCount))
	}
	if skipCount > 0 {
		extraInfo.WriteString(fmt.Sprintf("\n[Skipped %d inaccessible file(s)]", skipCount))
	}
	if len(results) >= maxResults {
		extraInfo.WriteString(fmt.Sprintf("\n[Output truncated at %d results]", maxResults))
	}

	extra := map[string]interface{}{
		"matchesFound": len(results),
		"path":         path,
		"pattern":      pattern,
	}
	if binaryCount > 0 {
		extra["skippedBinaryFiles"] = binaryCount
	}
	if skipCount > 0 {
		extra["skippedFiles"] = skipCount
	}

	return &ToolResult{
		Success: true,
		Output:  resultStr,
		Extra:   extra,
	}
}

// searchFile searches a single file for matching lines.
func (te *ToolExecutor) searchFile(filePath string, re *regexp.Regexp, flags map[string]bool, maxResults int) ([]matchResult, int, int) {
	if maxResults <= 0 {
		return nil, 0, 0
	}

	// Check if file is binary by reading first few bytes
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, 1, 0 // Skip count
	}

	// Simple binary detection: check for null bytes
	isBinary := false
	for i := 0; i < len(data) && i < 512; i++ {
		if data[i] == 0 {
			isBinary = true
			break
		}
	}
	if isBinary {
		return nil, 0, 1 // Binary count
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	// Handle trailing newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var results []matchResult
	for i, line := range lines {
		if len(results) >= maxResults {
			break
		}

		// The regex is already compiled with (?i) prefix if case-insensitive
		// was requested, so we can match directly on the original line.
		matched := re.MatchString(line)
		if flags["v"] {
			matched = !matched
		}

		if matched {
			results = append(results, matchResult{
				filePath: filePath,
				lineNum:  i + 1,
				line:     line,
			})
		}
	}

	return results, 0, 0
}

// hasFlag checks if a flag is present in a slice of flags.
func hasFlag(flags []string, flag string) bool {
	for _, f := range flags {
		if f == flag {
			return true
		}
	}
	return false
}

// executeGitLog views the commit history of a git repository with context support.
func (te *ToolExecutor) executeGitLog(ctx context.Context, params map[string]interface{}) *ToolResult {
	// Parse parameters
	path := "."
	if p, ok := params["path"].(string); ok && p != "" {
		path = p
	}

	// Validate path exists and is accessible
	if _, err := os.Stat(path); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("path not found or not accessible: %s", path),
		}
	}

	reference := ""
	if ref, ok := params["reference"].(string); ok && ref != "" {
		reference = ref
	}

	count := 10
	if c, ok := params["count"].(float64); ok && c > 0 {
		count = int(c)
	}
	if count > 1000 {
		count = 1000
	}

	// Parse flags
	flags := []string{}
	if flagsParam, ok := params["flags"]; ok {
		switch v := flagsParam.(type) {
		case []interface{}:
			for _, f := range v {
				if flagStr, ok := f.(string); ok {
					flags = append(flags, flagStr)
				}
			}
		case []string:
			flags = append(flags, v...)
		}
	}

	// Build git log command
	args := []string{"log", fmt.Sprintf("--max-count=%d", count)}

	// Validate conflicting format flags
	if hasFlag(flags, "oneline") {
		if hasFlag(flags, "stat") {
			return &ToolResult{
				Success: false,
				Error:   "conflicting flags: --oneline cannot be used with --stat",
			}
		}
		if hasFlag(flags, "patch") {
			return &ToolResult{
				Success: false,
				Error:   "conflicting flags: --oneline cannot be used with --patch",
			}
		}
		if hasFlag(flags, "shortstat") {
			return &ToolResult{
				Success: false,
				Error:   "conflicting flags: --oneline cannot be used with --shortstat",
			}
		}
	}

	// Add format flags based on options (must come before reference and path)
	for _, flag := range flags {
		switch flag {
		case "s":
			args = append(args, "--no-patch")
		case "m":
			args = append(args, "--merges")
		case "no-merges":
			args = append(args, "--no-merges")
		case "stat":
			args = append(args, "--stat")
		case "patch":
			args = append(args, "-p")
		case "oneline":
			args = append(args, "--oneline")
		case "shortstat":
			args = append(args, "--shortstat")
		case "follow":
			args = append(args, "--follow")
		case "grep":
			// Use dedicated grep parameter for searching commit messages
			grepParam := ""
			if gp, ok := params["grep"].(string); ok && gp != "" {
				grepParam = gp
			} else if reference != "" {
				// Fall back to reference for backwards compatibility
				grepParam = reference
			}
			if grepParam != "" {
				args = append(args, "--grep="+grepParam)
			}
		case "decorate":
			args = append(args, "--decorate")
		case "graph":
			args = append(args, "--graph")
		case "first-parent":
			args = append(args, "--first-parent")
		}
	}

	// Add reference if specified
	if reference != "" {
		args = append(args, reference)
	}

	// Resolve path: if it's a git repo root, use it as cmd.Dir;
	// if it's a subdirectory within a repo, find the repo root and use -- <subpath>
	cmdDir := path
	subpath := ""
	if path != "." {
		// Check if path is itself a git repo root
		repoRootCmd := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "--show-toplevel")
		if repoRootOut, repoRootErr := repoRootCmd.Output(); repoRootErr == nil {
			// path is a git repo root (or . itself)
			repoRoot := strings.TrimSpace(string(repoRootOut))
			if repoRoot == path || repoRoot == "." {
				cmdDir = path
			} else {
				// path is a subdirectory within a git repo
				cmdDir = repoRoot
				relPath, relErr := filepath.Rel(repoRoot, path)
				if relErr == nil {
					subpath = relPath
				}
			}
		}
	}

	// Add subpath to limit log scope
	if subpath != "" {
		args = append(args, "--", subpath)
	}

	// Execute git log with context for cancellation support
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = cmdDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if it was cancelled
		if ctx.Err() != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("git log was cancelled: %v", ctx.Err()),
			}
		}
		// Check if it's a git repository
		gitCmd := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "--show-toplevel")
		if _, err2 := gitCmd.CombinedOutput(); err2 != nil {
			return &ToolResult{
				Success: false,
				Error:   "not a git repository",
			}
		}
		// Check if the error is "no commits yet"
		if strings.Contains(string(output), "does not have any commits yet") {
			return &ToolResult{
				Success: true,
				Output:  "No commits found.",
				Extra: map[string]interface{}{
					"path":      path,
					"count":     count,
					"reference": reference,
					"flags":     flags,
				},
			}
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git log failed: %s", string(output)),
		}
	}

	resultStr := strings.TrimSpace(string(output))
	if resultStr == "" {
		resultStr = "No commits found."
	}

	// Truncate if excessively large (> 50KB)
	if len(resultStr) > 50000 {
		resultStr = resultStr[:50000] + "\n... [output truncated due to size]"
	}

	return &ToolResult{
		Success: true,
		Output:  resultStr,
		Extra: map[string]interface{}{
			"path":      path,
			"count":     count,
			"reference": reference,
			"flags":     flags,
		},
	}
}

// executeGitShow shows details of a specific commit with context support.
func (te *ToolExecutor) executeGitShow(ctx context.Context, params map[string]interface{}) *ToolResult {
	// Parse parameters
	path := "."
	if p, ok := params["path"].(string); ok && p != "" {
		path = p
	}

	// Validate path exists and is accessible
	if _, err := os.Stat(path); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("path not found or not accessible: %s", path),
		}
	}

	commit := "HEAD"
	if c, ok := params["commit"].(string); ok && c != "" {
		commit = c
	}

	// Parse flags
	flags := []string{}
	if flagsParam, ok := params["flags"]; ok {
		switch v := flagsParam.(type) {
		case []interface{}:
			for _, f := range v {
				if flagStr, ok := f.(string); ok {
					flags = append(flags, flagStr)
				}
			}
		case []string:
			flags = append(flags, v...)
		}
	}

	// Build git show command
	args := []string{"show", commit}

	// Add format flags based on options
	for _, flag := range flags {
		switch flag {
		case "stat":
			args = append(args, "--stat")
		case "patch", "p":
			args = append(args, "-p")
		case "name-status":
			args = append(args, "--name-status")
		case "name-only":
			args = append(args, "--name-only")
		case "shortstat":
			args = append(args, "--shortstat")
		case "stat-numstat":
			args = append(args, "--numstat")
		case "oneline":
			args = append(args, "--oneline")
		case "s":
			args = append(args, "--oneline", "--no-patch")
		case "no-patch":
			args = append(args, "--no-patch")
		case "summary":
			args = append(args, "--summary")
		case "r":
			args = append(args, "-C")
		case "M":
			args = append(args, "--find-copies")
		}
	}

	// Resolve path: if it's a git repo root, use it as cmd.Dir;
	// if it's a subdirectory within a repo, find the repo root and use -- <subpath>
	showCmdDir := path
	showSubpath := ""
	if path != "." {
		// Check if path is itself a git repo root
		repoRootCmd := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "--show-toplevel")
		if repoRootOut, repoRootErr := repoRootCmd.Output(); repoRootErr == nil {
			repoRoot := strings.TrimSpace(string(repoRootOut))
			if repoRoot == path || repoRoot == "." {
				showCmdDir = path
			} else {
				showCmdDir = repoRoot
				relPath, relErr := filepath.Rel(repoRoot, path)
				if relErr == nil {
					showSubpath = relPath
				}
			}
		}
	}

	// Add subpath to limit show scope
	if showSubpath != "" {
		args = append(args, "--", showSubpath)
	}

	// Execute git show with context for cancellation support
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = showCmdDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if it was cancelled
		if ctx.Err() != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("git show was cancelled: %v", ctx.Err()),
			}
		}
		// Check if it's a git repository
		gitCmd := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "--show-toplevel")
		if _, err2 := gitCmd.CombinedOutput(); err2 != nil {
			return &ToolResult{
				Success: false,
				Error:   "not a git repository",
			}
		}
		// Check if the error is "no commits yet"
		if strings.Contains(string(output), "does not have any commits yet") {
			return &ToolResult{
				Success: true,
				Output:  "No commits found.",
				Extra: map[string]interface{}{
					"path":   path,
					"commit": commit,
					"flags":  flags,
				},
			}
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git show failed: %s", string(output)),
		}
	}

	resultStr := strings.TrimSpace(string(output))
	if resultStr == "" {
		resultStr = "No information available for the specified commit."
	}

	// Truncate if excessively large (> 50KB)
	if len(resultStr) > 50000 {
		resultStr = resultStr[:50000] + "\n... [output truncated due to size]"
	}

	return &ToolResult{
		Success: true,
		Output:  resultStr,
		Extra: map[string]interface{}{
			"commitReference": commit,
		},
	}
}

// executeGitDiff shows the diff between two commits, branches, or the working tree with context support.
func (te *ToolExecutor) executeGitDiff(ctx context.Context, params map[string]interface{}) *ToolResult {
	// Parse parameters
	path := "."
	if p, ok := params["path"].(string); ok && p != "" {
		path = p
	}

	// Validate path exists and is accessible
	if _, err := os.Stat(path); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("path not found or not accessible: %s", path),
		}
	}

	reference1 := ""
	if c, ok := params["reference1"].(string); ok && c != "" {
		reference1 = c
	}
	// Also accept "commit1" for backwards compatibility
	if reference1 == "" {
		if c, ok := params["commit1"].(string); ok && c != "" {
			reference1 = c
		}
	}

	reference2 := ""
	if c, ok := params["reference2"].(string); ok && c != "" {
		reference2 = c
	}
	// Also accept "commit2" for backwards compatibility
	if reference2 == "" {
		if c, ok := params["commit2"].(string); ok && c != "" {
			reference2 = c
		}
	}

	// Parse flags
	flags := []string{}
	if flagsParam, ok := params["flags"]; ok {
		switch v := flagsParam.(type) {
		case []interface{}:
			for _, f := range v {
				if flagStr, ok := f.(string); ok {
					flags = append(flags, flagStr)
				}
			}
		case []string:
			flags = append(flags, v...)
		}
	}

	// Build git diff command
	args := []string{"diff"}

	// Add format flags based on options
	for _, flag := range flags {
		switch flag {
		case "stat":
			args = append(args, "--stat")
		case "patch", "p":
			args = append(args, "-p")
		case "name-status":
			args = append(args, "--name-status")
		case "name-only":
			args = append(args, "--name-only")
		case "shortstat":
			args = append(args, "--shortstat")
		case "stat-numstat", "numstat":
			args = append(args, "--numstat")
		case "color":
			args = append(args, "--color=always")
		case "stat-width", "stat-width=0":
			args = append(args, "--stat-width=0")
		case "summary":
			args = append(args, "--summary")
		case "compact-summary":
			args = append(args, "--compact-summary")
		case "ignore-space-at-eol", "ignore-space-at-eol=":
			args = append(args, "--ignore-space-at-eol")
		case "ignore-space-change":
			args = append(args, "-b")
		case "ignore-all-space":
			args = append(args, "-w")
		case "unified", "unified=":
			args = append(args, "--unified=3")
		case "raw":
			args = append(args, "--raw")
		case "r":
			args = append(args, "-C")
		case "M":
			args = append(args, "--find-copies")
		case "patience":
			args = append(args, "--patience")
		case "minimal":
			args = append(args, "--minimal")
		}
	}

	// Handle reference1 and reference2
	if reference1 != "" && reference2 != "" {
		// Diff between two commits/refs
		args = append(args, reference1, reference2)
	} else if reference1 != "" {
		// Diff between commit and working tree (or index)
		args = append(args, reference1)
	} else {
		// Default: diff working tree against index
		if reference2 != "" {
			// Diff index against a commit
			args = append(args, reference2)
		}
	}

	// Resolve path: if it's a git repo root, use it as cmd.Dir;
	// if it's a subdirectory within a repo, find the repo root and use -- <subpath>
	diffCmdDir := path
	diffSubpath := ""
	if path != "." {
		repoRootCmd := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "--show-toplevel")
		if repoRootOut, repoRootErr := repoRootCmd.Output(); repoRootErr == nil {
			repoRoot := strings.TrimSpace(string(repoRootOut))
			if repoRoot == path || repoRoot == "." {
				diffCmdDir = path
			} else {
				diffCmdDir = repoRoot
				relPath, relErr := filepath.Rel(repoRoot, path)
				if relErr == nil {
					diffSubpath = relPath
				}
			}
		}
	}

	// Add subpath to limit diff scope
	if diffSubpath != "" {
		args = append(args, "--", diffSubpath)
	}

	// Execute git diff with context for cancellation support
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = diffCmdDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if it was cancelled
		if ctx.Err() != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("git diff was cancelled: %v", ctx.Err()),
			}
		}
		// Check if it's a git repository
		gitCmd := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "--show-toplevel")
		if _, err2 := gitCmd.CombinedOutput(); err2 != nil {
			return &ToolResult{
				Success: false,
				Error:   "not a git repository",
			}
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("git diff failed: %s", string(output)),
		}
	}

	resultStr := strings.TrimSpace(string(output))
	if resultStr == "" {
		resultStr = "No differences found."
	}

	// Truncate if excessively large (> 50KB)
	if len(resultStr) > 50000 {
		resultStr = resultStr[:50000] + "\n... [output truncated due to size]"
	}

	return &ToolResult{
		Success: true,
		Output:  resultStr,
		Extra: map[string]interface{}{
			"path":       path,
			"reference1": reference1,
			"reference2": reference2,
			"flags":      flags,
		},
	}
}

// executeViewImage reads a local image file and returns it as a base64-encoded data URI
// so that the agent can send it to a vision-capable model for analysis.
func (te *ToolExecutor) executeViewImage(params map[string]interface{}) *ToolResult {
	path, ok := params["path"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	// Extract optional custom prompt for vision analysis
	var customPrompt string
	if p, hasPrompt := params["prompt"]; hasPrompt {
		if str, ok := p.(string); ok {
			customPrompt = str
		}
	}

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, path),
		}
	}

	// Validate file size (max 20MB to prevent memory issues)
	const maxImageSize = 20 * 1024 * 1024 // 20 MB
	if len(data) > maxImageSize {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("image file too large: %d bytes (max %d bytes)", len(data), maxImageSize),
		}
	}

	// Determine MIME type from file extension and content
	mimeType := detectImageMIMEType(path, data)
	if mimeType == "" {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unsupported or unrecognized image format: %s", path),
		}
	}

	// Encode as base64 data URI
	encoded := base64.StdEncoding.EncodeToString(data)
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Image loaded: %s (%s, %d bytes)", filepath.Base(path), mimeType, len(data)),
		Path:    path,
		Extra: map[string]interface{}{
			"data_uri":  dataURI,
			"mime_type": mimeType,
			"size":      len(data),
			"prompt":    customPrompt,
		},
	}
}

// detectImageMIMEType determines the MIME type of an image file.
// It first checks the file extension, then falls back to HTTP content type detection.
func detectImageMIMEType(path string, data []byte) string {
	// Allowed image MIME types
	allowedMIMETypes := map[string]bool{
		"image/png":  true,
		"image/jpeg": true,
		"image/gif":  true,
		"image/webp": true,
	}

	// First, try to detect from file extension
	ext := strings.ToLower(filepath.Ext(path))
	extMIME := map[string]string{
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".webp": "image/webp",
	}[ext]

	if extMIME != "" && allowedMIMETypes[extMIME] {
		return extMIME
	}

	// Fall back to HTTP content type detection (uses magic bytes)
	mimeType := http.DetectContentType(data)
	if allowedMIMETypes[mimeType] {
		return mimeType
	}

	// Also check for specific magic bytes as a final fallback
	if len(data) >= 3 {
		// PNG: 89 50 4E 47
		if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E {
			return "image/png"
		}
		// JPEG: FF D8 FF
		if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
			return "image/jpeg"
		}
		// GIF: 47 49 46 38
		if len(data) >= 4 && data[0] == 'G' && data[1] == 'I' && data[2] == 'F' && data[3] == '8' {
			return "image/gif"
		}
	}

	return ""
}

// ViewImageExtra contains extra data returned by view_image tool.
type ViewImageExtra struct {
	DataURI  string `json:"data_uri"`
	MIMEType string `json:"mime_type"`
	Size     int    `json:"size"`
	Prompt   string `json:"prompt,omitempty"`
}

// GetViewImageExtra extracts the view_image extra data from a ToolResult.
func GetViewImageExtra(result *ToolResult) *ViewImageExtra {
	if result == nil || result.Extra == nil {
		return nil
	}

	extra, ok := result.Extra["view_image_extra"]
	if !ok {
		// Try direct access to the fields
		dataURI, _ := result.Extra["data_uri"].(string)
		mimeType, _ := result.Extra["mime_type"].(string)
		var size int
		switch v := result.Extra["size"].(type) {
		case int:
			size = v
		case float64:
			size = int(v)
		}
		var prompt string
		if p, ok := result.Extra["prompt"].(string); ok {
			prompt = p
		}
		if dataURI != "" {
			return &ViewImageExtra{
				DataURI:  dataURI,
				MIMEType: mimeType,
				Size:     size,
				Prompt:   prompt,
			}
		}
		return nil
	}

	viewExtra, ok := extra.(*ViewImageExtra)
	if !ok {
		return nil
	}
	return viewExtra
}
