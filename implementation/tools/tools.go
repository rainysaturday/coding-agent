// Package tools implements the tool execution system for the coding agent.
package tools

import (
	"encoding/json"
	"fmt"
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

// Execute executes a tool call and returns the result.
func (te *ToolExecutor) Execute(tc *ToolCall) *ToolResult {
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
		result = te.executeBash(tc.Parameters)
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
		result = te.executeListFiles(tc.Parameters)
	case "grep":
		result = te.executeGrep(tc.Parameters)
	case "git_log":
		result = te.executeGitLog(tc.Parameters)
	case "git_show":
		result = te.executeGitShow(tc.Parameters)
	case "git_diff":
		result = te.executeGitDiff(tc.Parameters)
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
// read_file, list_files, read_lines, grep, git_log, and git_show are safe read-only operations.
func isReadOnlyTool(name string) bool { //nolint:funlen
	return name == "read_file" || name == "list_files" || name == "read_lines" || name == "grep" || name == "git_log" || name == "git_show" || name == "git_diff"
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
		Output:  fmt.Sprintf("Replaced '%s' with '%s' %d time(s) in: %s", searchText, replaceText, replacementsMade, path),
		Path:    path,
		Extra: map[string]interface{}{
			"search":           searchText,
			"replacementsMade": replacementsMade,
			"totalOccurrences": totalOccurrences,
		},
	}
}

// executeListFiles lists files and directories, formatted like ls.
func (te *ToolExecutor) executeListFiles(params map[string]interface{}) *ToolResult {
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

	// Read directory
	entries, err := os.ReadDir(path)
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
	var output string
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

	// Format: permissions links owner group size timestamp name
	return fmt.Sprintf("%s  1 user  group  %s  %s  %s", permStr, sizeStr, timeStr, name)
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

// executePatch applies a unified diff patch to a file.
// matchResult represents a single grep match.
type matchResult struct {
	filePath string
	lineNum  int
	line     string
}

// executeGrep searches through file contents using grep-like pattern matching.
func (te *ToolExecutor) executeGrep(params map[string]interface{}) *ToolResult {
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

	// Parse parameters and flags first (flags affect regex compilation)
	path := "."
	if p, ok := params["path"].(string); ok && p != "" {
		path = p
	}

	flags := map[string]bool{
		"i": false, // case-insensitive
		"r": false, // recursive
		"c": false, // count only
		"n": false, // line numbers (default false per requirement 034)
		"v": false, // invert match
		"l": false, // filenames only
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
			entries, err := os.ReadDir(path)
			if err != nil {
				return &ToolResult{
					Success: false,
					Error:   formatFileError(err, path),
				}
			}
			for _, entry := range entries {
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

// executeGitLog views the commit history of a git repository.
func (te *ToolExecutor) executeGitLog(params map[string]interface{}) *ToolResult {
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

	// Add format flags based on options (must come before reference and path)
	for _, flag := range flags {
		switch flag {
		case "s":
			args = append(args, "--pretty=format:%h (%d) %s")
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
			args = append(args, "--grep="+reference)
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

	// Execute git log in the specified directory (repo root)
	cmd := exec.Command("git", args...)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if it's a git repository
		gitCmd := exec.Command("git", "rev-parse", "--show-toplevel")
		gitCmd.Dir = path
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
					"path":        path,
					"count":       count,
					"reference":   reference,
					"flags":       flags,
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

	return &ToolResult{
		Success: true,
		Output:  resultStr,
		Extra: map[string]interface{}{
			"path":        path,
			"count":       count,
			"reference":   reference,
			"flags":       flags,
		},
	}
}

// executeGitShow shows details of a specific commit.
func (te *ToolExecutor) executeGitShow(params map[string]interface{}) *ToolResult {
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

	// Execute git show in the specified directory (repo root or subdirectory)
	cmd := exec.Command("git", args...)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if it's a git repository
		gitCmd := exec.Command("git", "rev-parse", "--show-toplevel")
		gitCmd.Dir = path
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
					"path":     path,
					"commit":   commit,
					"flags":    flags,
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

// executeGitDiff shows the diff between two commits, branches, or the working tree.
func (te *ToolExecutor) executeGitDiff(params map[string]interface{}) *ToolResult {
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

	// Execute git diff in the specified directory (repo root)
	cmd := exec.Command("git", args...)
	cmd.Dir = path
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if it's a git repository
		gitCmd := exec.Command("git", "rev-parse", "--show-toplevel")
		gitCmd.Dir = path
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
			"path":        path,
			"reference1":  reference1,
			"reference2":  reference2,
			"flags":       flags,
		},
	}
}
