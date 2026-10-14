package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// ===== Tests for utility functions =====

func TestHumanReadableSize(t *testing.T) {
	tests := []struct {
		size     int64
		expected string
	}{
		{0, "0"},
		{512, "512"},
		{1024, "1.0K"},
		{1536, "1.5K"},
		{1048576, "1.0M"},
		{1073741824, "1.0G"},
	}

	for _, tt := range tests {
		result := humanReadableSize(tt.size)
		if result != tt.expected {
			t.Errorf("humanReadableSize(%d) = %q, expected %q", tt.size, result, tt.expected)
		}
	}
}

func TestFormatPermissions(t *testing.T) {
	tests := []struct {
		name     string
		mode     os.FileMode
		expected string
	}{
		{"regular file", 0644, "-rw-r--r--"},
		{"directory", 0755, "drwxr-xr-x"},
		{"executable", 0755, "drwxr-xr-x"}, // Will be dir due to IsDir check
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			var info os.FileInfo

			if tt.name == "directory" || tt.name == "executable" {
				os.MkdirAll(filepath.Join(tmpDir, "testdir"), tt.mode)
				info, _ = os.Stat(filepath.Join(tmpDir, "testdir"))
			} else {
				testFile := filepath.Join(tmpDir, "testfile")
				os.WriteFile(testFile, []byte("content"), tt.mode)
				info, _ = os.Stat(testFile)
			}

			result := formatPermissions(info)
			if result != tt.expected {
				t.Errorf("formatPermissions(%v) = %q, expected %q", tt.mode, result, tt.expected)
			}
		})
	}
}

// ===== Tests for formatPermissionsRecursive =====

func TestFormatPermissionsRecursive_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	info, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	entry := walkEntry{
		path:     tmpDir,
		isDir:    true,
		info:     info,
		modTime:  info.ModTime(),
		fileSize: 0,
	}

	result := formatPermissionsRecursive(entry)
	if result[0] != 'd' {
		t.Errorf("Expected directory type 'd', got '%s'", string(result[0]))
	}
}

func TestFormatPermissionsRecursive_File(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	os.WriteFile(tmpFile, []byte("content"), 0644)
	info, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	entry := walkEntry{
		path:     tmpFile,
		isDir:    false,
		info:     info,
		modTime:  info.ModTime(),
		fileSize: info.Size(),
	}

	result := formatPermissionsRecursive(entry)
	if result[0] != '-' {
		t.Errorf("Expected file type '-', got '%s'", string(result[0]))
	}
}

// ===== Tests for formatRecursiveLongList =====

func TestFormatRecursiveLongList_Empty(t *testing.T) {
	result := formatRecursiveLongList(nil, "", nil)
	// Empty entries should return empty string
	if result != "" {
		t.Errorf("Expected empty result for nil entries, got: %s", result)
	}
}

func TestFormatRecursiveLongList_Simple(t *testing.T) {
	tmpDir := t.TempDir()
	info, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	entries := []walkEntry{
		{
			path:     tmpDir,
			isDir:    true,
			info:     info,
			modTime:  info.ModTime(),
			fileSize: 0,
		},
	}

	result := formatRecursiveLongList(entries, tmpDir, nil)
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

func TestFormatRecursiveLongList_WithFlags(t *testing.T) {
	tmpDir := t.TempDir()
	info, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	entries := []walkEntry{
		{
			path:     tmpDir,
			isDir:    true,
			info:     info,
			modTime:  info.ModTime(),
			fileSize: 0,
		},
	}

	flags := map[string]bool{
		"human-readable": true,
	}

	result := formatRecursiveLongList(entries, tmpDir, flags)
	if result == "" {
		t.Error("Expected non-empty result with flags")
	}
}

// ===== Tests for truncateOutput =====

func TestTruncateOutput_Empty(t *testing.T) {
	result := truncateOutput("", 10)
	if result != "(empty file)" {
		t.Errorf("Expected '(empty file)', got %q", result)
	}
}

func TestTruncateOutput_NoTruncation(t *testing.T) {
	text := "line1\nline2\nline3"
	result := truncateOutput(text, 10)
	if result != text {
		t.Errorf("Expected %q, got %q", text, result)
	}
}

func TestTruncateOutput_WithTruncation(t *testing.T) {
	lines := make([]string, 15)
	for i := range lines {
		lines[i] = fmt.Sprintf("line%d", i+1)
	}
	text := strings.Join(lines, "\n")
	result := truncateOutput(text, 5)
	expected := strings.Join(lines[:5], "\n") + "\n... [content truncated]"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// ===== Tests for truncateString =====

func TestTruncateString_NoTruncation(t *testing.T) {
	s := "hello"
	result := truncateString(s, 10)
	if result != s {
		t.Errorf("Expected %q, got %q", s, result)
	}
}

func TestTruncateString_WithTruncation(t *testing.T) {
	s := "hello world"
	result := truncateString(s, 5)
	expected := "hello..."
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// ===== Tests for splitLines and joinLines =====

// TestSplitLines_Empty verifies splitLines handles empty content.
func TestSplitLines_Empty(t *testing.T) {
	result := splitLines("")
	if len(result) != 0 {
		t.Errorf("Expected empty slice for empty content, got %v", result)
	}
}

// TestSplitLines_NoTrailingNewline verifies splitLines handles content without trailing newline.
func TestSplitLines_NoTrailingNewline(t *testing.T) {
	result := splitLines("a\nb\nc")
	if len(result) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(result))
	}
	if result[0] != "a" || result[1] != "b" || result[2] != "c" {
		t.Errorf("Unexpected content: %v", result)
	}
}

// TestSplitLines_WithTrailingNewline verifies splitLines handles trailing newline correctly.
func TestSplitLines_WithTrailingNewline(t *testing.T) {
	result := splitLines("a\nb\nc\n")
	if len(result) != 3 {
		t.Errorf("Expected 3 lines (no extra empty), got %d: %v", len(result), result)
	}
}

// TestSplitLines_SingleLine verifies splitLines handles single line content.
func TestSplitLines_SingleLine(t *testing.T) {
	result := splitLines("only")
	if len(result) != 1 {
		t.Errorf("Expected 1 line, got %d", len(result))
	}
	if result[0] != "only" {
		t.Errorf("Expected 'only', got '%s'", result[0])
	}
}

// TestJoinLines_Empty verifies joinLines handles empty slice.
func TestJoinLines_Empty(t *testing.T) {
	result := joinLines([]string{})
	if result != "" {
		t.Errorf("Expected empty string for empty slice, got %q", result)
	}
}

// TestJoinLines_SingleLine verifies joinLines adds trailing newline for single line.
func TestJoinLines_SingleLine(t *testing.T) {
	result := joinLines([]string{"hello"})
	if result != "hello\n" {
		t.Errorf("Expected 'hello\\n', got %q", result)
	}
}

// TestJoinLines_MultipleLines verifies joinLines produces correct output.
func TestJoinLines_MultipleLines(t *testing.T) {
	result := joinLines([]string{"a", "b", "c"})
	expected := "a\nb\nc\n"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// ===== ExecuteCtx cancellation tests =====

func TestExecuteCtx_Cancelled(t *testing.T) {
	te := NewToolExecutor()
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel before execution
	cancel()

	result := te.Execute(ctx, &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "sleep 10",
		},
	})
	if result.Success {
		t.Error("Expected failure for pre-cancelled context")
	}
}

func TestExecuteCtx_Bash_Cancelled(t *testing.T) {
	te := NewToolExecutor()
	ctx, cancel := context.WithCancel(context.Background())

	// Start a long-running command and cancel it
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result := te.Execute(ctx, &ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "sleep 30",
		},
	})
	// Should not panic
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestTruncateBytesAtRuneBoundary(t *testing.T) {
	// A 2-byte UTF-8 character whose cut lands mid-rune must not be split (I-11).
	// "ä" is 0xC3 0xA9. Repeating it and cutting at an odd byte offset used to
	// leave a lone continuation byte.
	for _, maxBytes := range []int{1, 3, 5, 7} {
		out := TruncateBytesAtRuneBoundary(strings.Repeat("ä", 20), maxBytes, "...")
		if !utf8.ValidString(out) {
			t.Errorf("TruncateBytesAtRuneBoundary(maxBytes=%d) produced invalid UTF-8: %q", maxBytes, out)
		}
	}
	// ASCII truncation appends the suffix.
	got := TruncateBytesAtRuneBoundary("abcdef", 3, "...")
	if got != "abc..." {
		t.Errorf("TruncateBytesAtRuneBoundary = %q, want %q", got, "abc...")
	}
	// Short strings are returned unchanged.
	if got := TruncateBytesAtRuneBoundary("hi", 10, "..."); got != "hi" {
		t.Errorf("TruncateBytesAtRuneBoundary(short) = %q, want %q", got, "hi")
	}
}

func TestTruncateRunes(t *testing.T) {
	// A 3-byte CJK character should not be split in the middle.
	cjk := "你好世界"
	truncated := TruncateRunes(cjk, 2)
	if truncated != "你好..." {
		t.Errorf("TruncateRunes(%q, 2) = %q, want %q", cjk, truncated, "你好...")
	}
	if !utf8.ValidString(truncated) {
		t.Errorf("TruncateRunes produced invalid UTF-8: %q", truncated)
	}
	// Short strings are returned unchanged.
	if got := TruncateRunes("hi", 5); got != "hi" {
		t.Errorf("TruncateRunes(%q, 5) = %q, want %q", "hi", got, "hi")
	}
	// ASCII still truncates with "...".
	if got := TruncateRunes("hello world", 5); got != "hello..." {
		t.Errorf("TruncateRunes(%q, 5) = %q, want %q", "hello world", got, "hello...")
	}
}

func TestParseFlagsParamToMap(t *testing.T) {
	// Documented dash-prefixed forms (e.g. "-n", "-r") must be accepted.
	params := map[string]interface{}{
		"flags": []interface{}{"-n", "-r", "-i"},
	}
	flags, err := parseFlagsParamToMap(params, grepAllowedFlags)
	if err != nil {
		t.Fatalf("dash-prefixed flags rejected: %v", err)
	}
	for _, k := range []string{"n", "r", "i"} {
		if !flags[k] {
			t.Errorf("Expected flag %q to be set, got %v", k, flags)
		}
	}

	// Bare single-char forms work too.
	params = map[string]interface{}{"flags": []string{"l", "a"}}
	flags, err = parseFlagsParamToMap(params, listFilesAllowedFlags)
	if err != nil {
		t.Fatalf("bare flags rejected: %v", err)
	}
	if !flags["l"] || !flags["a"] {
		t.Errorf("Expected bare single-char flags to be set, got %v", flags)
	}

	// A cluster of single-char flags expands into its members.
	flags, err = parseFlagsParamToMap(map[string]interface{}{"flags": []string{"-nir"}}, grepAllowedFlags)
	if err != nil {
		t.Fatalf("flag cluster rejected: %v", err)
	}
	if !flags["n"] || !flags["i"] || !flags["r"] {
		t.Errorf("cluster -nir should set n, i and r, got %v", flags)
	}

	// A flag the tool does not implement is an error, not a silent no-op. This
	// is the grep/list_files version of the git flag hole: a caller who writes
	// "--line-numbers" or "oneline" must be told, not quietly ignored.
	for _, bad := range []string{"--line-numbers", "oneline", "stat", "z"} {
		if _, err := parseFlagsParamToMap(map[string]interface{}{"flags": []string{bad}}, grepAllowedFlags); err == nil {
			t.Errorf("expected an error for unsupported flag %q, got none", bad)
		} else if !strings.Contains(err.Error(), bad) {
			t.Errorf("error must name the offending flag %q, got: %v", bad, err)
		} else if !strings.Contains(err.Error(), "supported flags") {
			t.Errorf("error must list the supported flags, got: %v", err)
		}
	}

	// A non-string flag entry is reported rather than dropped.
	if _, err := parseFlagsParamToMap(map[string]interface{}{"flags": []interface{}{7}}, grepAllowedFlags); err == nil {
		t.Error("expected an error for a non-string flag entry")
	}

	// Missing flags param yields an empty map.
	got, err := parseFlagsParamToMap(map[string]interface{}{}, grepAllowedFlags)
	if err != nil {
		t.Fatalf("missing flags param should be fine, got: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Expected empty map without flags param, got %v", got)
	}
}

func TestParseIntParamStrict(t *testing.T) {
	// Numeric forms are accepted.
	if v, err := parseIntParamStrict(map[string]interface{}{"n": 5.0}, "n"); err != "" || v != 5 {
		t.Errorf("float64: got (%d, %q), want (5, \"\")", v, err)
	}
	if v, err := parseIntParamStrict(map[string]interface{}{"n": 7}, "n"); err != "" || v != 7 {
		t.Errorf("int: got (%d, %q), want (7, \"\")", v, err)
	}
	if v, err := parseIntParamStrict(map[string]interface{}{"n": "9"}, "n"); err != "" || v != 9 {
		t.Errorf("numeric string: got (%d, %q), want (9, \"\")", v, err)
	}

	// Missing parameter reports "missing required parameter".
	if _, err := parseIntParamStrict(map[string]interface{}{}, "n"); !strings.Contains(err, "missing required parameter: n") {
		t.Errorf("missing: got %q, want 'missing required parameter: n'", err)
	}

	// Non-numeric string reports a type-aware error, not "missing".
	if _, err := parseIntParamStrict(map[string]interface{}{"n": "abc"}, "n"); !strings.Contains(err, "n must be a number, got string") {
		t.Errorf("bad string: got %q, want type-aware error", err)
	}

	// Wrong type reports a type-aware error.
	if _, err := parseIntParamStrict(map[string]interface{}{"n": true}, "n"); !strings.Contains(err, "n must be a number, got bool") {
		t.Errorf("bad type: got %q, want type-aware error", err)
	}
}

// TestExecute_FlagTools_RejectUnknownFlags covers the caller-visible behaviour:
// a flag the tool does not implement must fail loudly instead of producing
// output that looks like the flag was applied.
func TestExecute_FlagTools_RejectUnknownFlags(t *testing.T) {
	te := NewToolExecutor()

	cases := []struct {
		tool  string
		flags []string
	}{
		{"grep", []string{"--line-numbers"}},
		{"grep", []string{"stat"}},
		{"list_files", []string{"--recursive"}},
		{"list_files", []string{"oneline"}},
	}
	for _, tc := range cases {
		res := te.Execute(context.Background(), &ToolCall{
			Name: tc.tool,
			Parameters: map[string]interface{}{
				"pattern": "x",
				"path":    t.TempDir(),
				"flags":   tc.flags,
			},
		})
		if res.Success {
			t.Errorf("%s: expected failure for unsupported flag %q, got output: %s", tc.tool, tc.flags[0], res.Output)
			continue
		}
		if !strings.Contains(res.Error, "unsupported flag") {
			t.Errorf("%s: expected an unsupported-flag error, got: %s", tc.tool, res.Error)
		}
	}
}

// The documented dashed forms must keep working for grep and list_files.
func TestExecute_FlagTools_DashedFormsWork(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	te := NewToolExecutor()
	res := te.Execute(context.Background(), &ToolCall{
		Name: "grep",
		Parameters: map[string]interface{}{
			"pattern": "world",
			"path":    dir,
			"flags":   []string{"-n"},
		},
	})
	if !res.Success {
		t.Fatalf("grep failed: %s", res.Error)
	}
	if !strings.Contains(res.Output, "2:") {
		t.Errorf("expected -n line numbers in output, got: %s", res.Output)
	}
}

// TestExecute_WrongTypeParams_AreNotReportedAsMissing is the B4 guard: a
// parameter that is present but has the wrong type must not be described as
// missing. The old behaviour sent whoever read the log hunting for a parameter
// that was there all along.
func TestExecute_WrongTypeParams_AreNotReportedAsMissing(t *testing.T) {
	te := NewToolExecutor()

	cases := []struct {
		tool   string
		params map[string]interface{}
		want   string
	}{
		{"read_file", map[string]interface{}{"path": 42}, "path must be a string, got int"},
		{"write_file", map[string]interface{}{"path": "x.txt", "content": 7}, "content must be a string, got int"},
		{"write_file", map[string]interface{}{"path": nil, "content": "x"}, "path must be a string, got <nil>"},
		{"grep", map[string]interface{}{"pattern": []string{"a"}}, "pattern must be a string, got []string"},
		{"read_lines", map[string]interface{}{"path": "x", "start": "first", "end": 3}, "start must be a number, got string"},
		{"move_text", map[string]interface{}{"source_path": 1, "source_start": 1, "source_end": 2, "target_path": "t", "target_line": 1}, "source_path must be a string, got int"},
		{"todo", map[string]interface{}{"action": "complete", "id": "three"}, "id must be a number, got string"},
	}

	for _, tc := range cases {
		res := te.Execute(context.Background(), &ToolCall{Name: tc.tool, Parameters: tc.params})
		if res.Success {
			t.Errorf("%s: expected failure for %v, got output", tc.tool, tc.params)
			continue
		}
		if !strings.Contains(res.Error, tc.want) {
			t.Errorf("%s: error = %q, want it to contain %q", tc.tool, res.Error, tc.want)
		}
		if strings.Contains(res.Error, "missing required parameter") {
			t.Errorf("%s: wrong-typed parameter reported as missing: %s", tc.tool, res.Error)
		}
	}
}

// A genuinely absent parameter is still reported as missing.
func TestExecute_MissingParams_AreReportedAsMissing(t *testing.T) {
	te := NewToolExecutor()
	for _, tc := range []struct{ tool, key string }{
		{"read_file", "path"},
		{"write_file", "path"},
		{"grep", "pattern"},
		{"view_image", "path"},
	} {
		res := te.Execute(context.Background(), &ToolCall{Name: tc.tool, Parameters: map[string]interface{}{}})
		if res.Success {
			t.Errorf("%s: expected failure with no parameters", tc.tool)
			continue
		}
		if !strings.Contains(res.Error, "missing required parameter: "+tc.key) {
			t.Errorf("%s: error = %q, want a missing-parameter error for %q", tc.tool, res.Error, tc.key)
		}
	}
}

// An optional parameter with the wrong type must not silently fall back to the
// default: the tool would operate somewhere other than where the caller asked.
func TestExecute_OptionalParamWrongType_Errors(t *testing.T) {
	te := NewToolExecutor()
	res := te.Execute(context.Background(), &ToolCall{
		Name:       "list_files",
		Parameters: map[string]interface{}{"path": 42},
	})
	if res.Success {
		t.Fatalf("list_files with path=42 should fail, got output: %s", res.Output)
	}
	if !strings.Contains(res.Error, "path must be a string") {
		t.Errorf("expected a type error, got: %s", res.Error)
	}
}
