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
	// ---- Parameter Extraction ----
	// Extract and validate all required parameters from the input map.
	// JSON numbers are parsed as float64, so we convert to int.

	sourcePath, ok := params["source_path"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: source_path",
		}
	}

	sourceStartF, ok := params["source_start"].(float64)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: source_start",
		}
	}
	sourceStart := int(sourceStartF)

	sourceEndF, ok := params["source_end"].(float64)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: source_end",
		}
	}
	sourceEnd := int(sourceEndF)

	targetPath, ok := params["target_path"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: target_path",
		}
	}

	targetLineF, ok := params["target_line"].(float64)
	if !ok {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: target_line",
		}
	}
	targetLine := int(targetLineF)

	// ---- Parameter Validation ----
	// Validate line number constraints before any file operations.
	if sourceStart < 1 {
		return &ToolResult{
			Success: false,
			Error: fmt.Sprintf("invalid source_start: %d (must be >= 1)", sourceStart),
		}
	}
	if sourceEnd < sourceStart {
		return &ToolResult{
			Success: false,
			Error: fmt.Sprintf("invalid line range: source_start (%d) > source_end (%d)", sourceStart, sourceEnd),
		}
	}
	if targetLine < 1 {
		return &ToolResult{
			Success: false,
			Error: fmt.Sprintf("invalid target_line: %d (must be >= 1)", targetLine),
		}
	}

	// ---- Read Source File ----
	// Read the source file content and split into lines.
	// Returns error if the source file doesn't exist or can't be read.
	sourceContent, err := os.ReadFile(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("source file not found: %s", sourcePath),
			}
		}
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, sourcePath),
		}
	}

	// Parse source content into lines, handling trailing newline properly.
	sourceLines := splitLines(string(sourceContent))

	// Validate that the requested line range exists in the file.
	if sourceStart > len(sourceLines) {
		return &ToolResult{
			Success: false,
			Error: fmt.Sprintf("source line range out of bounds: requested lines %d-%d but file has only %d lines",
				sourceStart, sourceEnd, len(sourceLines)),
		}
	}

	// Clamp source_end to file length (in case file has fewer lines than expected).
	if sourceEnd > len(sourceLines) {
		sourceEnd = len(sourceLines)
	}

	// Convert 1-indexed line numbers to 0-indexed slice indices.
	sourceStartIdx := sourceStart - 1
	sourceEndIdx := sourceEnd - 1

	// ---- Extract Lines to Move ----
	// Extract the text block from the source lines.
	movedLines := make([]string, sourceEndIdx-sourceStartIdx+1)
	copy(movedLines, sourceLines[sourceStartIdx:sourceEndIdx+1])

	// Store the moved content for the response.
	movedContent := strings.Join(movedLines, "\n")
	linesMoved := len(movedLines)

	// ---- Remove Lines from Source ----
	// Build the new source content without the moved lines.
	remainingLines := make([]string, 0, len(sourceLines)-linesMoved)
	remainingLines = append(remainingLines, sourceLines[:sourceStartIdx]...)
	remainingLines = append(remainingLines, sourceLines[sourceEndIdx+1:]...)

	// ---- Determine Operation Mode ----
	// Check if this is a same-file move or cross-file move.
	isSameFile := filepath.Clean(sourcePath) == filepath.Clean(targetPath)

	if isSameFile {
		// ---- Same-File Move ----
		// For same-file moves, we need to handle the target line carefully.
		// When moving lines from earlier to later positions:
		//   - If target is after the source range, the lines after source shift up
		//   - We should NOT adjust targetLine since the user's intent is based on
		//     the original file structure
		// The clamping below handles cases where target exceeds remaining length.

		// Convert to 0-indexed and clamp to valid range.
		insertIdx := targetLine - 1
		if insertIdx < 0 {
			insertIdx = 0
		}
		if insertIdx > len(remainingLines) {
			insertIdx = len(remainingLines)
		}

		// Insert the moved lines at the adjusted position.
		finalLines := make([]string, 0, len(remainingLines)+linesMoved)
		finalLines = append(finalLines, remainingLines[:insertIdx]...)
		finalLines = append(finalLines, movedLines...)
		finalLines = append(finalLines, remainingLines[insertIdx:]...)

		// Write the modified content back to the file.
		output := joinLines(finalLines)
		if err := os.WriteFile(sourcePath, []byte(output), 0644); err != nil {
			return &ToolResult{
				Success: false,
				Error:   formatFileError(err, sourcePath),
			}
		}

		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Moved %d line(s) within %s (lines %d-%d -> line %d)\n--- Moved content ---\n%s",
				linesMoved, sourcePath, sourceStart, sourceEnd, targetLine, truncateOutput(movedContent, 10)),
			Path: sourcePath,
			Extra: map[string]interface{}{
				"sourcePath":  sourcePath,
				"sourceStart": sourceStart,
				"sourceEnd":   sourceEnd,
				"targetPath":  targetPath,
				"targetLine":  targetLine,
				"linesMoved":  linesMoved,
				"content":     movedContent,
			},
		}
	}

	// ---- Cross-File Move ----
	// Write the modified source file (with lines removed).
	sourceOutput := joinLines(remainingLines)
	if err := os.WriteFile(sourcePath, []byte(sourceOutput), 0644); err != nil {
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, sourcePath),
		}
	}

	// ---- Prepare Target File ----
	// Create parent directories for the target file if they don't exist.
	if err := ensureDirectory(targetPath); err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("cannot create directory: %v", err),
		}
	}

	// Read existing target content (if file exists).
	var targetLines []string
	targetContent, err := os.ReadFile(targetPath)
	if err == nil {
		targetLines = splitLines(string(targetContent))
	}
	// If file doesn't exist, targetLines remains nil/empty.

	// Convert target_line to 0-indexed and clamp to valid range.
	insertIdx := targetLine - 1
	if insertIdx < 0 {
		insertIdx = 0
	}
	if insertIdx > len(targetLines) {
		insertIdx = len(targetLines)
	}

	// Insert the moved lines at the target position.
	finalTargetLines := make([]string, 0, len(targetLines)+linesMoved)
	finalTargetLines = append(finalTargetLines, targetLines[:insertIdx]...)
	finalTargetLines = append(finalTargetLines, movedLines...)
	finalTargetLines = append(finalTargetLines, targetLines[insertIdx:]...)

	// Write the target file with inserted content.
	targetOutput := joinLines(finalTargetLines)
	if err := os.WriteFile(targetPath, []byte(targetOutput), 0644); err != nil {
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, targetPath),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Moved %d line(s) from %s (lines %d-%d) to %s (line %d)\n--- Moved content ---\n%s",
			linesMoved, sourcePath, sourceStart, sourceEnd, targetPath, targetLine, truncateOutput(movedContent, 10)),
		Path: targetPath,
		Extra: map[string]interface{}{
			"sourcePath":  sourcePath,
			"sourceStart": sourceStart,
			"sourceEnd":   sourceEnd,
			"targetPath":  targetPath,
			"targetLine":  targetLine,
			"linesMoved":  linesMoved,
			"content":     movedContent,
		},
	}
}

// splitLines splits file content into lines, handling trailing newlines properly.
// A trailing newline does not create an extra empty line element.
//
// For example:
//   "a\nb\n" -> ["a", "b"]
//   "a\nb"   -> ["a", "b"]
//   ""       -> []
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
//   ["a", "b"] -> "a\nb\n"
//   []         -> ""
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}
