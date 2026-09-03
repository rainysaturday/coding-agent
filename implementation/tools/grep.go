// Package tools implements the tool execution system for the coding agent.
// This file contains the grep tool implementation.
package tools

import (
	"bufio"
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

// grepParams holds the parsed parameters for a grep operation.
type grepParams struct {
	pattern string
	path    string
	flags   map[string]bool
}

// parseGrepParams extracts and validates grep parameters from the tool params map.
func parseGrepParams(params map[string]interface{}) (*grepParams, *ToolResult) {
	pattern, ok := params["pattern"].(string)
	if !ok {
		return nil, &ToolResult{Success: false, Error: "missing required parameter: pattern"}
	}
	if strings.TrimSpace(pattern) == "" {
		return nil, &ToolResult{Success: false, Error: "pattern cannot be empty"}
	}

	path := "."
	if p, ok := params["path"].(string); ok && p != "" {
		path = p
	}

	flags := parseFlagsParamToMap(params)
	for _, k := range []string{"i", "r", "c", "n", "v", "l", "a", "f"} {
		if _, ok := flags[k]; !ok {
			flags[k] = false
		}
	}

	return &grepParams{pattern: pattern, path: path, flags: flags}, nil
}

// compileGrepRegex handles pattern file reading and regex compilation.
func compileGrepRegex(pattern string, flags map[string]bool) (*regexp.Regexp, *ToolResult) {
	if flags["f"] {
		patternContent, err := os.ReadFile(pattern)
		if err != nil {
			return nil, &ToolResult{Success: false, Error: fmt.Sprintf("failed to read pattern file: %v", err)}
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
			return nil, &ToolResult{Success: false, Error: "pattern file is empty"}
		}
		pattern = strings.Join(escaped, "|")
	}

	patternToCompile := pattern
	if flags["i"] {
		patternToCompile = "(?i)" + pattern
	}
	re, err := regexp.Compile(patternToCompile)
	if err != nil {
		return nil, &ToolResult{Success: false, Error: fmt.Sprintf("invalid regex pattern: %v", err)}
	}
	return re, nil
}

// formatGrepResults formats grep results based on flags.
func formatGrepResults(results []matchResult, flags map[string]bool) string {
	var output strings.Builder

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
	} else if flags["c"] {
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
		for _, r := range results {
			if flags["n"] {
				output.WriteString(fmt.Sprintf("%s:%d:%s\n", r.filePath, r.lineNum, r.line))
			} else {
				output.WriteString(fmt.Sprintf("%s:%s\n", r.filePath, r.line))
			}
		}
	}

	return strings.TrimSuffix(output.String(), "\n")
}

// executeGrep searches through file contents using grep-like pattern matching.
// Supports context cancellation, recursive search, and various grep-like flags.
func (te *ToolExecutor) executeGrep(ctx context.Context, params map[string]interface{}) *ToolResult {
	gp, errResult := parseGrepParams(params)
	if errResult != nil {
		return errResult
	}

	re, errResult := compileGrepRegex(gp.pattern, gp.flags)
	if errResult != nil {
		return errResult
	}

	// Build a slice to collect results
	var results []matchResult
	var skipCount int
	var binaryCount int
	var oversizedCount int
	const maxResults = 5000

	// Check if path is a single file
	info, err := os.Stat(gp.path)
	if err == nil && !info.IsDir() {
		res, sc, bc, oc := te.searchFile(gp.path, re, gp.flags, maxResults)
		skipCount += sc
		binaryCount += bc
		oversizedCount += oc
		results = append(results, res...)
	} else if info != nil && info.IsDir() {
		if gp.flags["r"] {
			err = filepath.Walk(gp.path, func(filePath string, fileInfo os.FileInfo, walkErr error) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				if walkErr != nil {
					return nil
				}
				if !gp.flags["a"] {
					// Skip hidden files and directories, but never the search root
					// itself (which may itself be hidden, e.g. path=".config").
					// Comparing against the root with filepath.Rel avoids the
					// absolute-path bug where strings.Count(filePath, "/") > 0 is
					// always true for a rooted path (I-13).
					rel, relErr := filepath.Rel(gp.path, filePath)
					if relErr == nil && rel != "." && strings.HasPrefix(fileInfo.Name(), ".") {
						if fileInfo.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
				}
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
					res, sc, bc, oc := te.searchFile(filePath, re, gp.flags, maxResults-len(results))
					skipCount += sc
					binaryCount += bc
					oversizedCount += oc
					results = append(results, res...)
				}
				return nil
			})
		} else {
			select {
			case <-ctx.Done():
				return &ToolResult{Success: false, Error: "operation was cancelled"}
			default:
			}
			entries, err := os.ReadDir(gp.path)
			if err != nil {
				return &ToolResult{Success: false, Error: formatFileError(err, gp.path)}
			}
			for _, entry := range entries {
				select {
				case <-ctx.Done():
					return &ToolResult{Success: false, Error: "operation was cancelled"}
				default:
				}
				if entry.IsDir() {
					continue
				}
				fullPath := filepath.Join(gp.path, entry.Name())
				if len(results) >= maxResults {
					break
				}
				res, sc, bc, oc := te.searchFile(fullPath, re, gp.flags, maxResults-len(results))
				skipCount += sc
				binaryCount += bc
				oversizedCount += oc
				results = append(results, res...)
			}
		}
	} else {
		return &ToolResult{Success: false, Error: fmt.Sprintf("path not found: %s", gp.path)}
	}

	// Build output
	resultStr := formatGrepResults(results, gp.flags)

	// Build extra info
	var extraInfo strings.Builder
	if binaryCount > 0 {
		extraInfo.WriteString(fmt.Sprintf("\n[Skipped %d binary file(s)]", binaryCount))
	}
	if oversizedCount > 0 {
		extraInfo.WriteString(fmt.Sprintf("\n[Skipped %d oversized file(s) (larger than %d MB)]", oversizedCount, maxGrepFileSize/(1024*1024)))
	}
	if skipCount > 0 {
		extraInfo.WriteString(fmt.Sprintf("\n[Skipped %d inaccessible file(s)]", skipCount))
	}
	if len(results) >= maxResults {
		extraInfo.WriteString(fmt.Sprintf("\n[Output truncated at %d results]", maxResults))
	}

	extra := map[string]interface{}{
		"matchesFound": len(results),
		"path":         gp.path,
		"pattern":      gp.pattern,
	}
	if binaryCount > 0 {
		extra["skippedBinaryFiles"] = binaryCount
	}
	if oversizedCount > 0 {
		extra["skippedOversizedFiles"] = oversizedCount
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

// maxGrepFileSize caps how large a file grep will read, so a single
// recursive search over build artefacts, datasets or VM images cannot exhaust
// memory (I-08). Oversized files are skipped and reported separately.
const maxGrepFileSize = 10 * 1024 * 1024 // 10 MB

// searchFile searches a single file for matching lines.
func (te *ToolExecutor) searchFile(filePath string, re *regexp.Regexp, flags map[string]bool, maxResults int) ([]matchResult, int, int, int) {
	if maxResults <= 0 {
		return nil, 0, 0, 0
	}

	// Check file size up-front so we never read a huge file into memory.
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, 1, 0, 0 // inaccessible
	}
	if info.Size() > maxGrepFileSize {
		return nil, 0, 0, 1 // oversized
	}

	// Binary detection runs on the first 512 bytes before any full read.
	if isBinaryFile(filePath) {
		return nil, 0, 1, 0 // binary
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, 1, 0, 0
	}
	defer f.Close()

	// Stream line by line rather than reading the whole file into memory.
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), maxGrepFileSize)

	var results []matchResult
	lineNum := 0
	for scanner.Scan() {
		if len(results) >= maxResults {
			break
		}
		lineNum++
		line := scanner.Text()

		// The regex is already compiled with (?i) prefix if case-insensitive
		// was requested, so we can match directly on the original line.
		matched := re.MatchString(line)
		if flags["v"] {
			matched = !matched
		}

		if matched {
			results = append(results, matchResult{
				filePath: filePath,
				lineNum:  lineNum,
				line:     line,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		// A line exceeding the buffer is treated as inaccessible rather than
		// silently truncated, so the caller knows the file was not fully read.
		return nil, 1, 0, 0
	}

	return results, 0, 0, 0
}
