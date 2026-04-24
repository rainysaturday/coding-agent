// Package tools implements the tool execution system for the coding agent.
package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"
	"strconv"
	"strings"
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
	case "patch":
		result = te.executePatch(tc.Parameters)
	case "read_lines":
		result = te.executeReadLines(tc.Parameters)
	case "insert_lines":
		result = te.executeInsertLines(tc.Parameters)
	case "replace_text":
		result = te.executeReplaceText(tc.Parameters)
	case "list_files":
		result = te.executeListFiles(tc.Parameters)
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
// read_file, list_files, and read_lines are safe read-only operations.
func isReadOnlyTool(name string) bool {
	return name == "read_file" || name == "list_files" || name == "read_lines"
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

	if strings.TrimSpace(searchText) == "" {
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
	cmd := exec.Command("patch", "--dry-run", cleanPath, tmpFile.Name())
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
