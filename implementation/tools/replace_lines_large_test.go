package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ==================== LARGE FILE TESTS ====================

func TestReplaceLinesTool_Execute_LargeFile_100Lines_ReplaceMiddle(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")

	// Create a file with 100 lines
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, "Original line "+string(rune(i/10+48))+string(rune(i%10+48)))
	}
	content := strings.Join(lines, "\n") + "\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Replace lines 45-55 with 3 new lines
	replacementLines := []string{"New line A", "New line B", "New line C"}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "45",
		"end":   "55",
		"lines": strings.Join(replacementLines, "\n"),
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	// Read the file and verify
	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	fileLines := strings.Split(strings.TrimRight(fileContent, "\n"), "\n")
	expectedTotal := 44 + 3 + 45 // 44 before (1-44) + 3 replacement + 45 after (56-100)
	if len(fileLines) != expectedTotal {
		t.Errorf("Expected %d lines, got %d", expectedTotal, len(fileLines))
	}

	// Verify first line is unchanged
	if !strings.Contains(fileLines[0], "Original line 01") {
		t.Errorf("First line should be unchanged")
	}

	// Verify replacement lines
	if !strings.Contains(fileLines[44], "New line A") {
		t.Errorf("Line 45 should be 'New line A'")
	}
}

func TestReplaceLinesTool_Execute_LargeFile_500Lines_ReplaceBeginning(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")

	// Create a file with 500 lines
	var lines []string
	for i := 1; i <= 500; i++ {
		lines = append(lines, "Line "+string(rune(i%10+48)))
	}
	content := strings.Join(lines, "\n") + "\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Replace lines 1-10 with 20 new lines
	var replacementLines []string
	for i := 1; i <= 20; i++ {
		replacementLines = append(replacementLines, "Replacement "+string(rune(i+48)))
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "10",
		"lines": strings.Join(replacementLines, "\n"),
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	fileLines := strings.Split(strings.TrimRight(fileContent, "\n"), "\n")
	expectedTotal := 20 + 490 // 20 replacement + 490 remaining
	if len(fileLines) != expectedTotal {
		t.Errorf("Expected %d lines, got %d", expectedTotal, len(fileLines))
	}
}

func TestReplaceLinesTool_Execute_LargeFile_ReplaceEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")

	// Create a file with 50 lines
	var lines []string
	for i := 1; i <= 50; i++ {
		lines = append(lines, "Line "+string(rune(i/10+48))+string(rune(i%10+48)))
	}
	content := strings.Join(lines, "\n") + "\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Replace lines 45-55 (beyond file end) with 5 new lines
	replacementLines := []string{"New 1", "New 2", "New 3", "New 4", "New 5"}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "45",
		"end":   "55",
		"lines": strings.Join(replacementLines, "\n"),
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	fileLines := strings.Split(strings.TrimRight(fileContent, "\n"), "\n")
	// 44 original + 5 replacement = 49 (lines 45-50 were replaced, 51-55 didn't exist)
	if len(fileLines) != 49 {
		t.Errorf("Expected 49 lines, got %d", len(fileLines))
	}
}

func TestReplaceLinesTool_Execute_LargeFile_CompleteReplacement(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")

	// Create a file with 100 lines
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, "Original "+string(rune(i%10+48)))
	}
	content := strings.Join(lines, "\n") + "\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Replace entire file with 50 new lines
	var replacementLines []string
	for i := 1; i <= 50; i++ {
		replacementLines = append(replacementLines, "New "+string(rune(i%10+48)))
	}

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "100",
		"lines": strings.Join(replacementLines, "\n"),
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	fileLines := strings.Split(strings.TrimRight(fileContent, "\n"), "\n")
	if len(fileLines) != 50 {
		t.Errorf("Expected 50 lines, got %d", len(fileLines))
	}
}

// ==================== COMPLEX CONTENT TESTS ====================

func TestReplaceLinesTool_Execute_ComplexContent_WithUnicode(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "unicode.txt")

	// Create a file with unicode content
	initialContent := "Line 1: Hello\n" +
		"Line 2: Original\n" +
		"Line 3: World\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Replace with unicode content
	replacement := "Ligne 1: Bonjour\n" +
		"Ligne 2: \u00c9t\u00e9\n" +
		"Ligne 3: Hiver"

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "3",
		"lines": replacement,
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	if !strings.Contains(fileContent, "\u00c9t\u00e9") {
		t.Errorf("Expected output to contain French characters")
	}
}

func TestReplaceLinesTool_Execute_ComplexContent_CodeReplacement(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "code.txt")

	// Create a file with code
	initialContent := `function oldFunction() {
    const x = 1;
    const y = 2;
    return x + y;
}`

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Replace with new code
	newCode := `function newFunction(a, b) {
    const result = a * b;
    console.log("Result:", result);
    return result;
}`

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "5",
		"lines": newCode,
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	if !strings.Contains(fileContent, "function newFunction") {
		t.Errorf("Expected output to contain new function")
	}
	if !strings.Contains(fileContent, "a * b") {
		t.Errorf("Expected output to contain 'a * b'")
	}
}

func TestReplaceLinesTool_Execute_ComplexContent_MultipleNewlines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "newlines.txt")

	// Create a file
	initialContent := "Line 1\nLine 2\nLine 3\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Replace with content containing multiple consecutive newlines
	replacement := "Line A\n\n\nLine D"

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "3",
		"lines": replacement,
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	fileLines := strings.Split(strings.TrimRight(fileContent, "\n"), "\n")
	// Should have 5 lines: "Line A", "", "", "Line D"
	if len(fileLines) != 4 {
		t.Errorf("Expected 4 lines, got %d: %v", len(fileLines), fileLines)
	}
}

func TestReplaceLinesTool_Execute_ComplexContent_SpecialCharacters(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "special.txt")

	// Create a file
	initialContent := "Line 1\nLine 2\nLine 3\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Replace with special characters
	replacement := "$PATH && $HOME\n`command` $(shell)\n<xml>test</xml>\n{\"key\": \"value\"}"

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "3",
		"lines": replacement,
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	if !strings.Contains(fileContent, "$PATH") {
		t.Errorf("Expected output to contain $PATH")
	}
	if !strings.Contains(fileContent, "<xml>") {
		t.Errorf("Expected output to contain <xml>")
	}
}

func TestReplaceLinesTool_Execute_ComplexContent_NoTrailingNewline(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "no_newline.txt")

	// Create a file without trailing newline
	initialContent := "Line 1\nLine 2\nLine 3" // No trailing newline

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Replace with content without trailing newline
	replacement := "New 1\nNew 2"

	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "3",
		"lines": replacement,
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	// File should end with newline after replacement
	if !strings.HasSuffix(fileContent, "\n") {
		t.Errorf("Expected file to end with newline")
	}
}

// ==================== SEQUENTIAL REPLACEMENT TESTS ====================

func TestReplaceLinesTool_Execute_SequentialReplacements(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "sequential.txt")

	// Create initial file
	initialContent := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()

	// First replacement: lines 2-3
	result1 := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "2",
		"end":   "3",
		"lines": "Replaced A\nReplaced B",
	})
	if !result1.Success {
		t.Errorf("First replacement failed: %s", result1.Error)
	}

	// Second replacement: lines 4-5 (now at different position)
	result2 := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "4",
		"end":   "5",
		"lines": "Replaced C",
	})
	if !result2.Success {
		t.Errorf("Second replacement failed: %s", result2.Error)
	}

	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	fileLines := strings.Split(strings.TrimRight(fileContent, "\n"), "\n")
	if len(fileLines) != 4 {
		t.Errorf("Expected 4 lines, got %d", len(fileLines))
	}

	if !strings.Contains(fileLines[0], "Line 1") {
		t.Errorf("First line should be unchanged")
	}
	if !strings.Contains(fileLines[1], "Replaced A") {
		t.Errorf("Second line should be 'Replaced A'")
	}
}

func TestReplaceLinesTool_Execute_LargeFile_SequentialReplacements(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large_sequential.txt")

	// Create a file with 100 lines
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, "Line "+string(rune(i%10+48)))
	}
	content := strings.Join(lines, "\n") + "\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()

	// Multiple sequential replacements
	replacements := []struct {
		start, end int
		newLines   string
	}{
		{10, 15, "Block 1A\nBlock 1B"},
		{30, 35, "Block 2A\nBlock 2B\nBlock 2C"},
		{50, 55, "Block 3"},
		{70, 75, "Block 4A\nBlock 4B\nBlock 4C\nBlock 4D"},
	}

	for i, r := range replacements {
		result := tool.Execute(map[string]string{
			"path":  testFile,
			"start": string(rune(r.start/10+48)) + string(rune(r.start%10+48)),
			"end":   string(rune(r.end/10+48)) + string(rune(r.end%10+48)),
			"lines": r.newLines,
		})
		if !result.Success {
			t.Errorf("Replacement %d failed: %s", i+1, result.Error)
		}
	}

	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	if !strings.Contains(fileContent, "Block 1A") {
		t.Errorf("Expected content to contain 'Block 1A'")
	}
	if !strings.Contains(fileContent, "Block 4D") {
		t.Errorf("Expected content to contain 'Block 4D'")
	}
}

// ==================== BOUNDARY VALUE TESTS ====================

func TestReplaceLinesTool_Execute_Boundary_EmptyReplacement(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty_replace.txt")

	// Create a file
	initialContent := "Line 1\nLine 2\nLine 3\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Replace with empty content (delete lines)
	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "2",
		"end":   "2",
		"lines": "",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	fileLines := strings.Split(strings.TrimRight(fileContent, "\n"), "\n")
	if len(fileLines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(fileLines))
	}
}

func TestReplaceLinesTool_Execute_Boundary_ReplaceWithSingleLine(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "single.txt")

	// Create a file with many lines
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, "Line "+string(rune(i%10+48)))
	}
	content := strings.Join(lines, "\n") + "\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Replace 10 lines with 1 line
	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "45",
		"end":   "54",
		"lines": "Single replacement",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	fileLines := strings.Split(strings.TrimRight(fileContent, "\n"), "\n")
	expectedTotal := 44 + 1 + 46 // 44 before + 1 replacement + 46 after (100-54=46)
	if len(fileLines) != expectedTotal {
		t.Errorf("Expected %d lines, got %d", expectedTotal, len(fileLines))
	}
}

func TestReplaceLinesTool_Execute_Boundary_AppendToLargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "append.txt")

	// Create a file with 100 lines
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, "Line "+string(rune(i%10+48)))
	}
	content := strings.Join(lines, "\n") + "\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Append beyond file end
	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "101",
		"end":   "101",
		"lines": "Appended line 1\nAppended line 2",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	fileLines := strings.Split(strings.TrimRight(fileContent, "\n"), "\n")
	if len(fileLines) != 102 {
		t.Errorf("Expected 102 lines, got %d", len(fileLines))
	}

	if !strings.Contains(fileLines[100], "Appended line 1") {
		t.Errorf("Expected appended line at position 101")
	}
}

// ==================== WINDOWS LINE ENDINGS TESTS ====================

func TestReplaceLinesTool_Execute_WindowsLineEndings(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "windows.txt")

	// Create a file with Windows line endings
	initialContent := "Line 1\r\nLine 2\r\nLine 3\r\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Replace middle line
	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "2",
		"end":   "2",
		"lines": "Replaced line",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	if !strings.Contains(fileContent, "Replaced line") {
		t.Errorf("Expected content to contain 'Replaced line'")
	}
}

// ==================== EDGE CASE TESTS ====================

func TestReplaceLinesTool_Execute_EdgeCase_ExpandShrink(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "expand_shrink.txt")

	// Create a file
	initialContent := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()

	// Expand: replace 2 lines with 5
	result1 := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "2",
		"end":   "3",
		"lines": "New 1\nNew 2\nNew 3\nNew 4\nNew 5",
	})
	if !result1.Success {
		t.Errorf("Expand replacement failed: %s", result1.Error)
	}

	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	fileLines := strings.Split(strings.TrimRight(fileContent, "\n"), "\n")
	if len(fileLines) != 8 { // 1 + 5 + 2 = 8
		t.Errorf("After expand: Expected 8 lines, got %d", len(fileLines))
	}

	// Shrink: replace 5 lines with 2
	result2 := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "2",
		"end":   "6",
		"lines": "Shrink A\nShrink B",
	})
	if !result2.Success {
		t.Errorf("Shrink replacement failed: %s", result2.Error)
	}

	contentBytes, err = os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent = string(contentBytes)

	fileLines = strings.Split(strings.TrimRight(fileContent, "\n"), "\n")
	if len(fileLines) != 5 { // 1 + 2 + 2 = 5
		t.Errorf("After shrink: Expected 5 lines, got %d", len(fileLines))
	}
}

func TestReplaceLinesTool_Execute_EdgeCase_OverlappingRanges(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "overlapping.txt")

	// Create a file
	initialContent := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReplaceLinesTool()

	// First replacement
	result1 := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "2",
		"end":   "4",
		"lines": "A\nB\nC",
	})
	if !result1.Success {
		t.Errorf("First replacement failed: %s", result1.Error)
	}

	// Second replacement with overlapping conceptual range
	result2 := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "3",
		"end":   "5",
		"lines": "X\nY",
	})
	if !result2.Success {
		t.Errorf("Second replacement failed: %s", result2.Error)
	}

	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	fileLines := strings.Split(strings.TrimRight(fileContent, "\n"), "\n")
	// After first: Line 1, A, B, C, Line 5 (5 lines)
	// After second: Line 1, A, X, Y (4 lines)
	if len(fileLines) != 4 {
		t.Errorf("Expected 4 lines, got %d", len(fileLines))
	}
}

func TestReplaceLinesTool_Execute_EdgeCase_SingleCharacterLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "single_char.txt")

	// Create a file with single character lines
	initialContent := "a\nb\nc\nd\ne\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Replace with single character lines
	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "2",
		"end":   "4",
		"lines": "x\ny",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	fileLines := strings.Split(strings.TrimRight(fileContent, "\n"), "\n")
	if len(fileLines) != 4 {
		t.Errorf("Expected 4 lines, got %d", len(fileLines))
	}

	if fileLines[0] != "a" {
		t.Errorf("First line should be 'a'")
	}
	if fileLines[1] != "x" {
		t.Errorf("Second line should be 'x'")
	}
}

func TestReplaceLinesTool_Execute_EdgeCase_WhitespaceOnlyLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "whitespace.txt")

	// Create a file with whitespace-only lines
	initialContent := "Line 1\n   \n\t\nLine 4\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Replace whitespace lines
	tool := NewReplaceLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "2",
		"end":   "3",
		"lines": "Replaced",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	contentBytes, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileContent := string(contentBytes)

	fileLines := strings.Split(strings.TrimRight(fileContent, "\n"), "\n")
	if len(fileLines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(fileLines))
	}

	if !strings.Contains(fileLines[0], "Line 1") {
		t.Errorf("First line should be 'Line 1'")
	}
}
