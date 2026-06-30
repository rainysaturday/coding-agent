// Package tools implements the tool execution system for the coding agent.
// This file contains the read_file tool implementation.
package tools

import (
	"os"
)

const (
	// maxReadFileSize is the maximum file size in bytes allowed for read_file.
	// Files larger than this are rejected to prevent the LLM from reading
	// binary files or very large files accidentally.
	maxReadFileSize = 20 * 1024 // 20KB
)

// executeReadFile reads a file.
func (te *ToolExecutor) executeReadFile(params map[string]interface{}) *ToolResult {
	path, ok := params["path"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	// Check file size before reading
	fileInfo, err := os.Stat(path)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, path),
		}
	}

	if fileInfo.Size() > maxReadFileSize {
		return &ToolResult{
			Success: false,
			Error:   formatReadFileTooLargeError(path, fileInfo.Size()),
		}
	}

	// Check if file is binary
	if isBinaryFile(path) {
		return &ToolResult{
			Success: false,
			Error:   formatReadFileBinaryError(path),
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

// isBinaryFile checks if a file is binary by looking at the first bytes.
// It reads the first 512 bytes and checks for null bytes, which are
// strong indicators of binary content.
func isBinaryFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil {
		return false
	}
	buf = buf[:n]

	for _, b := range buf {
		if b == 0 {
			return true
		}
	}
	return false
}

// formatReadFileTooLargeError returns an error message for files that exceed the size limit.
func formatReadFileTooLargeError(path string, size int64) string {
	return "file is too large to read (" + formatFileSize(size) + "). Please use the read_lines tool to read the file in smaller chunks, or use the bash tool to read specific portions of the file."
}

// formatReadFileBinaryError returns an error message for binary files.
func formatReadFileBinaryError(path string) string {
	return "file appears to be a binary file: " + path + ". Binary files cannot be read as text. Use the view_image tool for image files, or use the bash tool to inspect binary file contents."
}

// formatFileSize formats a file size in bytes to a human-readable string.
func formatFileSize(bytes int64) string {
	if bytes < 1024 {
		return formatInt(bytes) + " bytes"
	}
	if bytes < 1024*1024 {
		return formatInt(bytes/1024) + " KB"
	}
	return formatInt(bytes/(1024*1024)) + " MB"
}

// formatInt converts an int to a string.
func formatInt(n int64) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + formatInt(-n)
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
