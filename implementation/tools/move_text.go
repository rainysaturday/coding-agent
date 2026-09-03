// Package tools implements the tool execution system for the coding agent.
// This file contains the move_text tool implementation.
//
// The move_text tool enables moving text blocks between lines in the same file
// or to other files. It atomically extracts lines from a source location and
// inserts them at a target location, automatically creating target files and
// directories as needed.
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// moveTextParams holds the parsed parameters for a move_text operation.
type moveTextParams struct {
	sourcePath  string
	sourceStart int
	sourceEnd   int
	targetPath  string
	targetLine  int
}

// parseMoveTextParams extracts and validates move_text parameters from the tool params map.
func parseMoveTextParams(params map[string]interface{}) (*moveTextParams, *ToolResult) {
	sourcePath, ok := params["source_path"].(string)
	if !ok {
		return nil, &ToolResult{Success: false, Error: "missing required parameter: source_path"}
	}
	sourceStart, errMsg := parseIntParamStrict(params, "source_start")
	if errMsg != "" {
		return nil, &ToolResult{Success: false, Error: errMsg}
	}
	sourceEnd, errMsg := parseIntParamStrict(params, "source_end")
	if errMsg != "" {
		return nil, &ToolResult{Success: false, Error: errMsg}
	}
	targetPath, ok := params["target_path"].(string)
	if !ok {
		return nil, &ToolResult{Success: false, Error: "missing required parameter: target_path"}
	}
	targetLine, errMsg := parseIntParamStrict(params, "target_line")
	if errMsg != "" {
		return nil, &ToolResult{Success: false, Error: errMsg}
	}

	mp := &moveTextParams{
		sourcePath:  sourcePath,
		sourceStart: sourceStart,
		sourceEnd:   sourceEnd,
		targetPath:  targetPath,
		targetLine:  targetLine,
	}

	// Validate line number constraints
	if mp.sourceStart < 1 {
		return nil, &ToolResult{Success: false, Error: fmt.Sprintf("invalid source_start: %d (must be >= 1)", mp.sourceStart)}
	}
	if mp.sourceEnd < mp.sourceStart {
		return nil, &ToolResult{Success: false, Error: fmt.Sprintf("invalid line range: source_start (%d) > source_end (%d)", mp.sourceStart, mp.sourceEnd)}
	}
	if mp.targetLine < 1 {
		return nil, &ToolResult{Success: false, Error: fmt.Sprintf("invalid target_line: %d (must be >= 1)", mp.targetLine)}
	}

	return mp, nil
}

// executeMoveText moves a text block from source location to target location.
//
// The operation works as follows:
// 1. Validates all input parameters
// 2. Reads the source file and extracts the specified line range
// 3. Removes the extracted lines from the source file
// 4. Creates the target file (and parent directories) if needed
// 5. Inserts the extracted content at the specified target line
//
// For same-file moves, line numbers are adjusted after removal to ensure
// correct insertion position.
func (te *ToolExecutor) executeMoveText(params map[string]interface{}) *ToolResult {
	mp, errResult := parseMoveTextParams(params)
	if errResult != nil {
		return errResult
	}

	// Read source file
	sourceContent, err := os.ReadFile(mp.sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ToolResult{Success: false, Error: fmt.Sprintf("source file not found: %s", mp.sourcePath)}
		}
		return &ToolResult{Success: false, Error: formatFileError(err, mp.sourcePath)}
	}

	sourceLines := splitLines(string(sourceContent))

	// Validate line range
	if mp.sourceStart > len(sourceLines) {
		return &ToolResult{Success: false, Error: fmt.Sprintf("source line range out of bounds: requested lines %d-%d but file has only %d lines", mp.sourceStart, mp.sourceEnd, len(sourceLines))}
	}
	if mp.sourceEnd > len(sourceLines) {
		mp.sourceEnd = len(sourceLines)
	}

	sourceStartIdx := mp.sourceStart - 1
	sourceEndIdx := mp.sourceEnd - 1

	// Extract lines to move
	movedLines := make([]string, sourceEndIdx-sourceStartIdx+1)
	copy(movedLines, sourceLines[sourceStartIdx:sourceEndIdx+1])
	movedContent := strings.Join(movedLines, "\n")
	linesMoved := len(movedLines)

	// Remove lines from source
	remainingLines := make([]string, 0, len(sourceLines)-linesMoved)
	remainingLines = append(remainingLines, sourceLines[:sourceStartIdx]...)
	remainingLines = append(remainingLines, sourceLines[sourceEndIdx+1:]...)

	// Determine operation mode
	isSameFile := filepath.Clean(mp.sourcePath) == filepath.Clean(mp.targetPath)

	if isSameFile {
		return te.executeSameFileMove(mp, movedLines, movedContent, linesMoved, remainingLines)
	}

	return te.executeCrossFileMove(mp, movedLines, movedContent, linesMoved, remainingLines)
}

// executeSameFileMove handles moving lines within the same file.
func (te *ToolExecutor) executeSameFileMove(mp *moveTextParams, movedLines []string, movedContent string, linesMoved int, remainingLines []string) *ToolResult {
	insertIdx := mp.targetLine - 1
	if insertIdx < 0 {
		insertIdx = 0
	}
	if insertIdx > len(remainingLines) {
		insertIdx = len(remainingLines)
	}

	finalLines := make([]string, 0, len(remainingLines)+linesMoved)
	finalLines = append(finalLines, remainingLines[:insertIdx]...)
	finalLines = append(finalLines, movedLines...)
	finalLines = append(finalLines, remainingLines[insertIdx:]...)

	output := joinLines(finalLines)
	if err := WriteFilePreservePerm(mp.sourcePath, []byte(output)); err != nil {
		return &ToolResult{Success: false, Error: formatFileError(err, mp.sourcePath)}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Moved %d line(s) within %s (lines %d-%d -> line %d)\n--- Moved content ---\n%s", linesMoved, mp.sourcePath, mp.sourceStart, mp.sourceEnd, mp.targetLine, truncateOutput(movedContent, 10)),
		Path:    mp.sourcePath,
		Extra:   map[string]interface{}{"sourcePath": mp.sourcePath, "sourceStart": mp.sourceStart, "sourceEnd": mp.sourceEnd, "targetPath": mp.targetPath, "targetLine": mp.targetLine, "linesMoved": linesMoved, "content": movedContent},
	}
}

// executeCrossFileMove handles moving lines between different files.
//
// The target is written first so a failed target write leaves the source
// untouched (no data loss). If the subsequent source write fails, the target
// is rolled back to its original content so the move is atomic from the user's
// perspective.
func (te *ToolExecutor) executeCrossFileMove(mp *moveTextParams, movedLines []string, movedContent string, linesMoved int, remainingLines []string) *ToolResult {
	// Prepare target directory (may create it). This never touches the source.
	if err := ensureDirectory(mp.targetPath); err != nil {
		return &ToolResult{Success: false, Error: fmt.Sprintf("cannot create directory: %v", err)}
	}

	// Read the target file if it exists (it may not).
	var targetLines []string
	var originalTargetContent []byte
	targetExisted := false
	targetContent, err := os.ReadFile(mp.targetPath)
	if err == nil {
		targetExisted = true
		originalTargetContent = targetContent
		targetLines = splitLines(string(targetContent))
	} else if !os.IsNotExist(err) {
		return &ToolResult{Success: false, Error: formatFileError(err, mp.targetPath)}
	}

	insertIdx := mp.targetLine - 1
	if insertIdx < 0 {
		insertIdx = 0
	}
	if insertIdx > len(targetLines) {
		insertIdx = len(targetLines)
	}

	finalTargetLines := make([]string, 0, len(targetLines)+linesMoved)
	finalTargetLines = append(finalTargetLines, targetLines[:insertIdx]...)
	finalTargetLines = append(finalTargetLines, movedLines...)
	finalTargetLines = append(finalTargetLines, targetLines[insertIdx:]...)

	targetOutput := joinLines(finalTargetLines)
	sourceOutput := joinLines(remainingLines)

	// Write the TARGET first so a failure here leaves the source untouched.
	if err := WriteFilePreservePerm(mp.targetPath, []byte(targetOutput)); err != nil {
		return &ToolResult{Success: false, Error: formatFileError(err, mp.targetPath)}
	}

	// Now write the modified source. If it fails, roll back the target so the
	// moved block does not exist in two places (or vanish from both).
	if err := WriteFilePreservePerm(mp.sourcePath, []byte(sourceOutput)); err != nil {
		rollbackMsg := ""
		if targetExisted {
			if rerr := WriteFilePreservePerm(mp.targetPath, originalTargetContent); rerr != nil {
				rollbackMsg = fmt.Sprintf(" (rollback failed: %v)", rerr)
			}
		} else {
			if rerr := os.Remove(mp.targetPath); rerr != nil {
				rollbackMsg = fmt.Sprintf(" (rollback failed: %v)", rerr)
			}
		}
		return &ToolResult{Success: false, Error: formatFileError(err, mp.sourcePath) + rollbackMsg}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Moved %d line(s) from %s (lines %d-%d) to %s (line %d)\n--- Moved content ---\n%s", linesMoved, mp.sourcePath, mp.sourceStart, mp.sourceEnd, mp.targetPath, mp.targetLine, truncateOutput(movedContent, 10)),
		Path:    mp.targetPath,
		Extra:   map[string]interface{}{"sourcePath": mp.sourcePath, "sourceStart": mp.sourceStart, "sourceEnd": mp.sourceEnd, "targetPath": mp.targetPath, "targetLine": mp.targetLine, "linesMoved": linesMoved, "content": movedContent},
	}
}

// splitLines splits file content into lines, handling trailing newlines properly.
// A trailing newline does not create an extra empty line element.
//
// For example:
//
//	"a\nb\n" -> ["a", "b"]
//	"a\nb"   -> ["a", "b"]
//	""       -> []
func splitLines(content string) []string {
	if content == "" {
		return []string{}
	}
	lines := strings.Split(content, "\n")
	// Remove trailing empty element caused by trailing newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// joinLines joins a slice of lines into file content with proper newline handling.
// Adds a trailing newline if there are any lines (standard text file format).
//
// For example:
//
//	["a", "b"] -> "a\nb\n"
//	[]         -> ""
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}
