package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ==================== SEARCH-AND-REPLACE MODE TESTS ====================

func TestReplaceLinesTool_SearchReplace_Simple(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "hello world\nthis is a test\nhello again\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"search": "hello",
		"replace": "goodbye",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "goodbye world\nthis is a test\nhello again\n"
	if string(content) != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, string(content))
	}
}

func TestReplaceLinesTool_SearchReplace_Multiline(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := `function oldFunc() {
    return "old";
}

function otherFunc() {
    return "other";
}`

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"search": "function oldFunc() {\n    return \"old\";\n}",
		"replace": "function newFunc() {\n    return \"new\";\n}",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.Contains(string(content), "function newFunc()") {
		t.Errorf("Expected content to contain 'function newFunc()'")
	}
	if strings.Contains(string(content), "function oldFunc()") {
		t.Errorf("Expected content to not contain 'function oldFunc()'")
	}
}

func TestReplaceLinesTool_SearchReplace_AllOccurrences(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "hello world\nhello again\nhello forever\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"search": "hello",
		"replace": "goodbye",
		"count":  "all",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "goodbye world\ngoodbye again\ngoodbye forever\n"
	if string(content) != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, string(content))
	}
}

func TestReplaceLinesTool_SearchReplace_CountLimit(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "hello world\nhello again\nhello forever\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"search": "hello",
		"replace": "goodbye",
		"count":  "2",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "goodbye world\ngoodbye again\nhello forever\n"
	if string(content) != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, string(content))
	}
}

func TestReplaceLinesTool_SearchReplace_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "hello world\nthis is a test\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"search": "notfound",
		"replace": "replacement",
	})

	if result.Success {
		t.Error("Expected failure for not found search text")
	}
	if !strings.Contains(result.Error, "not found") {
		t.Errorf("Expected 'not found' error, got: %s", result.Error)
	}
}

func TestReplaceLinesTool_SearchReplace_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")

	err := os.WriteFile(testFile, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"search": "anything",
		"replace": "replacement",
	})

	if result.Success {
		t.Error("Expected failure for search in empty file")
	}
}

func TestReplaceLinesTool_SearchReplace_CreateNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "new.txt")

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"search": "anything",
		"replace": "replacement",
	})

	if result.Success {
		t.Error("Expected failure for search in non-existent file")
	}
}

func TestReplaceLinesTool_SearchReplace_MissingSearch(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"replace": "replacement",
	})

	if result.Success {
		t.Error("Expected failure for missing search")
	}
}

func TestReplaceLinesTool_SearchReplace_MissingReplace(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"search": "test",
	})

	if result.Success {
		t.Error("Expected failure for missing replace")
	}
}

func TestReplaceLinesTool_SearchReplace_InvalidCount(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"search": "test",
		"replace": "replacement",
		"count":  "abc",
	})

	if result.Success {
		t.Error("Expected failure for invalid count")
	}
}

func TestReplaceLinesTool_SearchReplace_CountZero(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "hello world\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"search": "hello",
		"replace": "goodbye",
		"count":  "0",
	})

	if !result.Success {
		t.Errorf("Expected success with count 0, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Should not have changed anything
	if string(content) != initialContent {
		t.Errorf("Expected content unchanged, got '%s'", string(content))
	}
}

func TestReplaceLinesTool_SearchReplace_CountNegative(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "hello world\nhello again\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"search": "hello",
		"replace": "goodbye",
		"count":  "-1",
	})

	if !result.Success {
		t.Errorf("Expected success with count -1, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "goodbye world\ngoodbye again\n"
	if string(content) != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, string(content))
	}
}

func TestReplaceLinesTool_SearchReplace_EmptySearch(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"search": "",
		"replace": "replacement",
	})

	if result.Success {
		t.Error("Expected failure for empty search")
	}
}

func TestReplaceLinesTool_SearchReplace_SpecialCharacters(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := `function test() {
    const x = "hello \"world\"";
    return x;
}`

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"search": `"hello \"world\""`,
		"replace": `"goodbye universe"`,
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.Contains(string(content), `"goodbye universe"`) {
		t.Errorf("Expected content to contain 'goodbye universe'")
	}
}

func TestReplaceLinesTool_SearchReplace_Unicode(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "你好世界\nhello world\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"search": "你好世界",
		"replace": "こんにちは",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.Contains(string(content), "こんにちは") {
		t.Errorf("Expected content to contain Japanese characters")
	}
	if strings.Contains(string(content), "你好世界") {
		t.Errorf("Expected content to not contain original Chinese characters")
	}
}

// ==================== MODE SELECTION TESTS ====================

func TestReplaceLinesTool_ModeSelection_BothModes(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewReplaceLinesTool()
	// Provide both search and start/end - search mode should take precedence
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"search": "test",
		"replace": "replacement",
		"start":  "1",
		"end":    "1",
		"lines":  "ignored",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}
}

func TestReplaceLinesTool_ModeSelection_NeitherMode(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	tool := NewReplaceLinesTool()
	// Provide neither search nor start/end
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"lines":  "replacement",
	})

	if result.Success {
		t.Error("Expected failure for missing mode parameters")
	}
}

// ==================== BACKWARD COMPATIBILITY TESTS ====================

func TestReplaceLinesTool_LineMode_BackwardCompatibility(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "line1\nline2\nline3\nline4\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"start":  "2",
		"end":    "2",
		"lines":  "replaced",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "line1\nreplaced\nline3\nline4\n"
	if string(content) != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, string(content))
	}
}

// ==================== EDGE CASE TESTS ====================

func TestReplaceLinesTool_SearchReplace_OverlappingPatterns(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "aaa\n" // Contains overlapping "aa" patterns

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"search": "aa",
		"replace": "X",
		"count":  "all",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// First "aa" at position 0 is replaced, leaving "a\n"
	// The remaining "a" doesn't match "aa"
	expected := "Xa\n"
	if string(content) != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, string(content))
	}
}

func TestReplaceLinesTool_SearchReplace_PreserveLineEndings(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	// Windows line endings
	initialContent := "line1\r\nline2\r\nline3\r\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":   testFile,
		"search": "line2",
		"replace": "REPLACED",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Line endings should be preserved
	if !strings.Contains(string(content), "\r\n") {
		t.Errorf("Expected Windows line endings to be preserved")
	}
	if !strings.Contains(string(content), "REPLACED\r\n") {
		t.Errorf("Expected replaced text with line ending")
	}
}
