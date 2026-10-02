package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// resolvePath makes path absolute relative to the working directory.
func (r *RunContext) resolvePath(p string) string {
	if p == "" {
		return r.CWD
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(r.CWD, p))
}

// isBinary reports whether the content looks like binary data.
func isBinary(data []byte) bool {
	if bytes.IndexByte(data, 0) >= 0 {
		return true
	}
	if !utf8.Valid(data) {
		return true
	}
	return false
}

func sniffBinary(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	buf := make([]byte, 8192)
	n, _ := f.Read(buf)
	return isBinary(buf[:n]), nil
}

func readFileString(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Split(s, "\n")
}

// splitContentLines splits file content into lines without producing a
// trailing empty element for files that end with a newline.
func splitContentLines(s string) []string {
	s = strings.TrimSuffix(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}

// numberLines prefixes each line with its 1-indexed line number,
// matching the "N: content" format required by requirement 011.
func numberLines(lines []string, start int) string {
	var b strings.Builder
	for i, l := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%d: %s", start+i, l)
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// read_file
// ---------------------------------------------------------------------------

// ReadFileTool reads a text file (with a size limit and binary detection).
type ReadFileTool struct{ run *RunContext }

// NewReadFileTool creates the read_file tool.
func NewReadFileTool(run *RunContext) *ReadFileTool { return &ReadFileTool{run: run} }

func (t *ReadFileTool) Name() string { return "read_file" }

func (t *ReadFileTool) Description() string { return "Read the contents of a file" }

func (t *ReadFileTool) Schema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"path": prop("string", "Path to the file to read"),
	}, []string{"path"})
}

func (t *ReadFileTool) ReadOnly() bool { return true }

func (t *ReadFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path := getString(args, "path")
	if path == "" {
		return "", errors.New("missing required parameter: path")
	}
	abs := t.run.resolvePath(path)
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory, not a file", path)
	}
	bin, err := sniffBinary(abs)
	if err != nil {
		return "", err
	}
	if bin {
		return "", fmt.Errorf("%s appears to be a binary file; read_file only supports text files (use view_image for images or bash for binary inspection)", path)
	}
	limit := t.run.MaxReadBytes
	if limit <= 0 {
		limit = 20 * 1024
	}
	if info.Size() > limit {
		return "", fmt.Errorf("file too large: %s is %d bytes (max %d bytes); use read_lines to read the file in smaller chunks", path, info.Size(), limit)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ---------------------------------------------------------------------------
// read_lines
// ---------------------------------------------------------------------------

// ReadLinesTool reads a specific line range from a file.
type ReadLinesTool struct{ run *RunContext }

// NewReadLinesTool creates the read_lines tool.
func NewReadLinesTool(run *RunContext) *ReadLinesTool { return &ReadLinesTool{run: run} }

func (t *ReadLinesTool) Name() string { return "read_lines" }

func (t *ReadLinesTool) Description() string { return "Read a specific line range from a file" }

func (t *ReadLinesTool) Schema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"path":  prop("string", "Path to the file to read"),
		"start": prop("integer", "Starting line number (1-indexed)"),
		"end":   prop("integer", "Ending line number (1-indexed)"),
	}, []string{"path", "start", "end"})
}

func (t *ReadLinesTool) ReadOnly() bool { return true }

func (t *ReadLinesTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path := getString(args, "path")
	if path == "" {
		return "", errors.New("missing required parameter: path")
	}
	start := getInt(args, "start", 1)
	end := getInt(args, "end", 0)
	if start < 1 {
		// Requirement 018: invalid input is reported with an actionable message.
		return "", errors.New("invalid start parameter: must be a positive integer")
	}
	abs := t.run.resolvePath(path)
	content, err := readFileString(abs)
	if err != nil {
		return "", err
	}
	lines := splitContentLines(content)
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	if start > len(lines) {
		return fmt.Sprintf("[line %d is beyond end of file (%d lines)]", start, len(lines)), nil
	}
	if end < start {
		return "", fmt.Errorf("end (%d) must be >= start (%d)", end, start)
	}
	// Requirement 011: returned output includes line numbers ("N: content").
	return numberLines(lines[start-1:end], start), nil
}

// ---------------------------------------------------------------------------
// write_file
// ---------------------------------------------------------------------------

// WriteFileTool writes (or overwrites) a file.
type WriteFileTool struct{ run *RunContext }

// NewWriteFileTool creates the write_file tool.
func NewWriteFileTool(run *RunContext) *WriteFileTool { return &WriteFileTool{run: run} }

func (t *WriteFileTool) Name() string { return "write_file" }

func (t *WriteFileTool) Description() string {
	return "Write content to a file"
}

func (t *WriteFileTool) Schema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"path":    prop("string", "Path to the file to write"),
		"content": prop("string", "Content to write to the file"),
	}, []string{"path", "content"})
}

func (t *WriteFileTool) ReadOnly() bool { return false }

func (t *WriteFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path := getString(args, "path")
	if path == "" {
		return "", errors.New("missing required parameter: path")
	}
	content := getString(args, "content")
	abs := t.run.resolvePath(path)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	existed := false
	if _, err := os.Stat(abs); err == nil {
		existed = true
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return "", err
	}
	verb := "created"
	if existed {
		verb = "wrote"
	}
	return fmt.Sprintf("Successfully %s file: %s (%d bytes)", verb, path, len(content)), nil
}

// ---------------------------------------------------------------------------
// insert_lines
// ---------------------------------------------------------------------------

// InsertLinesTool inserts lines at a specific line number.
type InsertLinesTool struct{ run *RunContext }

// NewInsertLinesTool creates the insert_lines tool.
func NewInsertLinesTool(run *RunContext) *InsertLinesTool { return &InsertLinesTool{run: run} }

func (t *InsertLinesTool) Name() string { return "insert_lines" }

func (t *InsertLinesTool) Description() string { return "Insert lines at a specific line number" }

func (t *InsertLinesTool) Schema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"path":  prop("string", "The path to the file"),
		"line":  prop("integer", "Line number where insertion should occur (1-indexed)"),
		"lines": prop("string", "The lines to insert (use \\n for newlines)"),
	}, []string{"path", "line", "lines"})
}

func (t *InsertLinesTool) ReadOnly() bool { return false }

func (t *InsertLinesTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path := getString(args, "path")
	if path == "" {
		return "", errors.New("missing required parameter: path")
	}
	line := getInt(args, "line", 1)
	newLines := splitLines(strings.ReplaceAll(getString(args, "lines"), "\r\n", "\n"))
	abs := t.run.resolvePath(path)

	var lines []string
	if data, err := os.ReadFile(abs); err == nil {
		lines = splitLines(strings.TrimRight(string(data), "\n"))
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if line < 1 {
		line = 1
	}
	if line > len(lines)+1 {
		line = len(lines) + 1
	}
	out := make([]string, 0, len(lines)+len(newLines))
	out = append(out, lines[:line-1]...)
	out = append(out, newLines...)
	out = append(out, lines[line-1:]...)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, []byte(joinLines(out)+"\n"), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("Successfully inserted %d line(s) at line %d in: %s", len(newLines), line, path), nil
}

// ---------------------------------------------------------------------------
// replace_text
// ---------------------------------------------------------------------------

// ReplaceTextTool performs a literal find/replace inside a file.
type ReplaceTextTool struct{ run *RunContext }

// NewReplaceTextTool creates the replace_text tool.
func NewReplaceTextTool(run *RunContext) *ReplaceTextTool { return &ReplaceTextTool{run: run} }

func (t *ReplaceTextTool) Name() string { return "replace_text" }

func (t *ReplaceTextTool) Description() string {
	return "Find and replace text in a file by searching for a pattern"
}

func (t *ReplaceTextTool) Schema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"path":    prop("string", "File path to modify"),
		"search":  prop("string", "Text pattern to find (exact match, not regex)"),
		"replace": prop("string", "Replacement text"),
		"count":   prop("integer", "Number of occurrences to replace (default: 1, use -1 for all)"),
	}, []string{"path", "search", "replace"})
}

func (t *ReplaceTextTool) ReadOnly() bool { return false }

func (t *ReplaceTextTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path := getString(args, "path")
	search := getString(args, "search")
	replace := getString(args, "replace")
	count := getInt(args, "count", 1)
	if path == "" {
		return "", errors.New("missing required parameter: path")
	}
	if search == "" {
		return "", errors.New("missing required parameter: search")
	}
	abs := t.run.resolvePath(path)
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	original := string(data)
	total := strings.Count(original, search)
	if total == 0 {
		return "", fmt.Errorf("pattern not found in %s", path)
	}
	var updated string
	if count < 0 {
		updated = strings.ReplaceAll(original, search, replace)
	} else {
		updated = strings.Replace(original, search, replace, count)
	}
	if err := os.WriteFile(abs, []byte(updated), 0o644); err != nil {
		return "", err
	}
	replaced := total
	if count >= 0 && count < total {
		replaced = count
	}
	preview := summarize(updated)
	msg := fmt.Sprintf("Replaced %q with %q %d time(s) in: %s", truncate(search, 60), truncate(replace, 60), replaced, path)
	if preview != "" {
		msg += "\n--- Preview ---\n" + preview
	}
	return msg, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func summarize(updated string) string {
	lines := splitLines(updated)
	if len(lines) == 0 {
		return ""
	}
	if len(lines) > 6 {
		return joinLines(lines[:6]) + "\n... [truncated]"
	}
	return joinLines(lines)
}

// ---------------------------------------------------------------------------
// move_text
// ---------------------------------------------------------------------------

// MoveTextTool moves a block of lines between files (or within one file).
type MoveTextTool struct{ run *RunContext }

// NewMoveTextTool creates the move_text tool.
func NewMoveTextTool(run *RunContext) *MoveTextTool { return &MoveTextTool{run: run} }

func (t *MoveTextTool) Name() string { return "move_text" }

func (t *MoveTextTool) Description() string {
	return "Move a block of text from one location to another. Extracts lines from a source file and inserts them at a target location (same file or different file). Automatically creates target file and directories if needed."
}

func (t *MoveTextTool) Schema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"source_path":  prop("string", "Path to the source file to extract lines from"),
		"source_start": prop("integer", "Starting line number in source file (1-indexed)"),
		"source_end":   prop("integer", "Ending line number in source file (1-indexed, inclusive)"),
		"target_path":  prop("string", "Path to the target file to insert lines into"),
		"target_line":  prop("integer", "Line number in target file to insert before (1-indexed)"),
	}, []string{"source_path", "source_start", "source_end", "target_path", "target_line"})
}

func (t *MoveTextTool) ReadOnly() bool { return false }

func (t *MoveTextTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	srcPath := getString(args, "source_path")
	dstPath := getString(args, "target_path")
	srcStart := getInt(args, "source_start", 0)
	srcEnd := getInt(args, "source_end", 0)
	dstLine := getInt(args, "target_line", 0)
	if srcPath == "" || dstPath == "" {
		return "", errors.New("source_path and target_path are required")
	}
	if srcStart < 1 || srcEnd < srcStart {
		return "", fmt.Errorf("invalid source range %d-%d", srcStart, srcEnd)
	}
	srcAbs := t.run.resolvePath(srcPath)
	dstAbs := t.run.resolvePath(dstPath)

	srcData, err := os.ReadFile(srcAbs)
	if err != nil {
		return "", err
	}
	srcLines := splitLines(strings.TrimRight(string(srcData), "\n"))
	if srcStart > len(srcLines) {
		return "", fmt.Errorf("source_start %d is beyond end of file (%d lines)", srcStart, len(srcLines))
	}
	if srcEnd > len(srcLines) {
		srcEnd = len(srcLines)
	}
	moved := srcLines[srcStart-1 : srcEnd]

	if srcAbs == dstAbs {
		rest := append(append([]string{}, srcLines[:srcStart-1]...), srcLines[srcEnd:]...)
		if dstLine < 1 {
			dstLine = 1
		}
		if dstLine > len(rest)+1 {
			dstLine = len(rest) + 1
		}
		out := append(append([]string{}, rest[:dstLine-1]...), moved...)
		out = append(out, rest[dstLine-1:]...)
		if err := os.WriteFile(srcAbs, []byte(joinLines(out)+"\n"), 0o644); err != nil {
			return "", err
		}
		return fmt.Sprintf("Moved %d line(s) from lines %d-%d to line %d in %s", len(moved), srcStart, srcEnd, dstLine, srcPath), nil
	}

	var dstLines []string
	existed := true
	if data, err := os.ReadFile(dstAbs); err == nil {
		dstLines = splitLines(strings.TrimRight(string(data), "\n"))
	} else if os.IsNotExist(err) {
		existed = false
	} else {
		return "", err
	}
	if dstLine < 1 {
		dstLine = 1
	}
	if dstLine > len(dstLines)+1 {
		dstLine = len(dstLines) + 1
	}
	out := append(append([]string{}, dstLines[:dstLine-1]...), moved...)
	out = append(out, dstLines[dstLine-1:]...)
	if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dstAbs, []byte(joinLines(out)+"\n"), 0o644); err != nil {
		return "", err
	}
	remaining := append(append([]string{}, srcLines[:srcStart-1]...), srcLines[srcEnd:]...)
	if err := os.WriteFile(srcAbs, []byte(joinLines(remaining)+"\n"), 0o644); err != nil {
		return "", err
	}
	verb := "created"
	if existed {
		verb = "updated"
	}
	return fmt.Sprintf("Moved %d line(s) from %s (lines %d-%d) into %s at line %d; target file %s", len(moved), srcPath, srcStart, srcEnd, dstPath, dstLine, verb), nil
}
