// Package tools implements the tool execution system for the coding agent.
// This file contains the grep tool implementation.
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

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
