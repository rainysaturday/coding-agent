// Package tools implements the tool execution system for the coding agent.
// This file contains shared utility functions used across multiple tools.
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// File permission constants used across tool implementations.
const (
	FilePermWrite = os.FileMode(0644)
	FilePermDir   = os.FileMode(0755)
	FilePermRead  = os.FileMode(0400)
)

// countLines counts the number of lines in text.
// A line is defined as text terminated by a newline character.
// An empty string has 0 lines. Text without a trailing newline still counts as a line.
func countLines(text string) int {
	if text == "" {
		return 0
	}
	count := strings.Count(text, "\n")
	if !strings.HasSuffix(text, "\n") {
		count++
	}
	return count
}

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

// hasFlag checks if a flag is present in a slice of flags.
func hasFlag(flags []string, flag string) bool {
	for _, f := range flags {
		if f == flag {
			return true
		}
	}
	return false
}

// isCancelled returns true if the error indicates the operation was cancelled.
func isCancelled(err error) bool {
	return err == context.Canceled
}

// parseBoolParam extracts a boolean from a map parameter value.
func parseBoolParam(params map[string]interface{}, key string) bool {
	if v, ok := params[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
		if s, ok := v.(string); ok {
			return s == "true" || s == "1" || s == "yes"
		}
	}
	return false
}

// parseIntParam extracts an integer from a map parameter value.
func parseIntParam(params map[string]interface{}, key string, defaultValue int) int {
	if v, ok := params[key]; ok {
		switch val := v.(type) {
		case float64:
			return int(val)
		case int:
			return val
		case string:
			if i, err := strconv.Atoi(val); err == nil {
				return i
			}
		}
	}
	return defaultValue
}

// ensureDirectory creates parent directories if needed for a file path.
func ensureDirectory(path string) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		return os.MkdirAll(dir, FilePermDir)
	}
	return nil
}

// truncateLargeOutput truncates output to max bytes and adds a truncation marker.
func truncateLargeOutput(output string, maxBytes int) string {
	if len(output) <= maxBytes {
		return output
	}
	return output[:maxBytes] + "\n... [output truncated due to size]"
}

// parseFlagValue extracts the value portion from a flag like "flag=value" or "flag".
func parseFlagValue(flag string) (name string, value string) {
	parts := strings.SplitN(flag, "=", 2)
	name = parts[0]
	if len(parts) > 1 {
		value = parts[1]
	}
	return
}

// isGitRepo checks if the given path is a git repository.
func isGitRepo(path string) bool {
	if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
		return false
	}
	return true
}

// parseFlagsParamToMap extracts flags from the "flags" parameter and stores them in a map[string]bool.
// This is used by tools like grep and list_files that use single-char flags.
func parseFlagsParamToMap(params map[string]interface{}) map[string]bool {
	flags := make(map[string]bool)
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
	return flags
}

// parseFlagsParamToSlice extracts flags from the "flags" parameter and returns them as a []string.
// This is used by tools like git_log, git_show, and git_diff that use multi-char flags.
func parseFlagsParamToSlice(params map[string]interface{}) []string {
	var flags []string
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
	return flags
}

