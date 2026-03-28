package tools

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// ==================== LARGE FILE TESTS ====================

func TestReadLinesTool_Execute_LargeFile_100Lines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")

	// Create a file with 100 lines
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, "Line number: "+string(rune('0'+i/10))+string(rune('0'+i%10)))
	}
	content := strings.Join(lines, "\n") + "\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "45",
		"end":   "55",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	// Verify we got exactly 11 lines
	outputLines := strings.Split(result.Output, "\n")
	if len(outputLines) != 11 {
		t.Errorf("Expected 11 lines, got %d", len(outputLines))
	}

	// Verify first and last line
	if !strings.Contains(outputLines[0], "45:") {
		t.Errorf("Expected first line to contain '45:', got '%s'", outputLines[0])
	}
	if !strings.Contains(outputLines[10], "55:") {
		t.Errorf("Expected last line to contain '55:', got '%s'", outputLines[10])
	}
}

func TestReadLinesTool_Execute_LargeFile_1000Lines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "very_large.txt")

	// Create a file with 1000 lines
	var lines []string
	for i := 1; i <= 1000; i++ {
		lines = append(lines, "Line "+string(rune(i/100+48))+string(rune((i/10)%10+48))+string(rune(i%10+48)))
	}
	content := strings.Join(lines, "\n") + "\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "500",
		"end":   "510",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	outputLines := strings.Split(result.Output, "\n")
	if len(outputLines) != 11 {
		t.Errorf("Expected 11 lines, got %d", len(outputLines))
	}
}

func TestReadLinesTool_Execute_LargeFile_StartNearEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")

	// Create a file with 200 lines
	var lines []string
	for i := 1; i <= 200; i++ {
		lines = append(lines, "Line "+string(rune(i+48)))
	}
	content := strings.Join(lines, "\n") + "\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "195",
		"end":   "205",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	outputLines := strings.Split(result.Output, "\n")
	if len(outputLines) != 6 { // Only 195-200 exist
		t.Errorf("Expected 6 lines (195-200), got %d", len(outputLines))
	}
}

func TestReadLinesTool_Execute_LargeFile_ReadingAll(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")

	// Create a file with 50 lines
	var lines []string
	for i := 1; i <= 50; i++ {
		lines = append(lines, "Line "+string(rune(i+48)))
	}
	content := strings.Join(lines, "\n") + "\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "50",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	outputLines := strings.Split(result.Output, "\n")
	if len(outputLines) != 50 {
		t.Errorf("Expected 50 lines, got %d", len(outputLines))
	}
}

// ==================== COMPLEX CONTENT TESTS ====================

func TestReadLinesTool_Execute_ComplexContent_WithUnicode(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "unicode.txt")

	// Create a file with unicode content
	content := "Line 1: Hello World\n" +
		"Line 2: \u00e9\u00e0\u00fc\u00f6\n" +
		"Line 3: \u4e2d\u6587\n" +
		"Line 4: \ud55c\uae00\n" +
		"Line 5: \U0001F600\U0001F601\n" +
		"Line 6: \u03b1\u03b2\u03b3\n" +
		"Line 7: \u00a9\u00ae\u2122\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "2",
		"end":   "6",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	if !strings.Contains(result.Output, "中文") {
		t.Errorf("Expected output to contain Chinese characters")
	}
	if !strings.Contains(result.Output, "2:") {
		t.Errorf("Expected output to contain line number 2")
	}
}

func TestReadLinesTool_Execute_ComplexContent_EmptyLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty_lines.txt")

	// Create a file with empty lines interspersed
	content := "Line 1\n" +
		"\n" +
		"Line 3\n" +
		"\n" +
		"\n" +
		"Line 6\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "2",
		"end":   "6",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	outputLines := strings.Split(result.Output, "\n")
	if len(outputLines) != 5 {
		t.Errorf("Expected 5 lines, got %d", len(outputLines))
	}

	// Verify empty lines are preserved (line 2 is empty, line 3 is "Line 3", etc.)
	if !strings.HasPrefix(outputLines[0], "2:") {
		t.Errorf("Expected first output line to start with '2:', got '%s'", outputLines[0])
	}
	if !strings.HasPrefix(outputLines[1], "3:") {
		t.Errorf("Expected second output line to start with '3:', got '%s'", outputLines[1])
	}
}

func TestReadLinesTool_Execute_ComplexContent_WithTabs(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "tabs.txt")

	// Create a file with tabs
	content := "Line\t1\twith\ttabs\n" +
		"\t\tLeading tabs\n" +
		"Line 3\n" +
		"Trailing tabs\t\t\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "4",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	if !strings.Contains(result.Output, "\t") {
		t.Errorf("Expected output to contain tabs")
	}
}

func TestReadLinesTool_Execute_ComplexContent_LongLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "long_lines.txt")

	// Create a file with very long lines (500 chars each)
	longLine1 := strings.Repeat("A", 500)
	longLine2 := strings.Repeat("B", 500)
	longLine3 := strings.Repeat("C", 500)
	content := longLine1 + "\n" + longLine2 + "\n" + longLine3 + "\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "3",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	if !strings.Contains(result.Output, longLine1) {
		t.Errorf("Expected output to contain first long line")
	}
}

func TestReadLinesTool_Execute_ComplexContent_CodeLikeContent(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "code.txt")

	// Create a file with code-like content (indentation, special chars)
	content := `function test() {
    const x = 1;
    const y = 2;
    if (x > y) {
        console.log("x is greater");
    } else {
        console.log("y is greater");
    }
    return x + y;
}`

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "2",
		"end":   "6",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	if !strings.Contains(result.Output, "const x = 1") {
		t.Errorf("Expected output to contain 'const x = 1'")
	}
	if !strings.Contains(result.Output, "2:") {
		t.Errorf("Expected output to contain line number 2")
	}
}

func TestReadLinesTool_Execute_ComplexContent_SpecialCharacters(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "special.txt")

	// Create a file with special characters
	content := "Line 1: $PATH && $HOME\n" +
		"Line 2: `backticks` and $(command)\n" +
		"Line 3: <xml>tag</xml>\n" +
		"Line 4: {\"key\": \"value\"}\n" +
		"Line 5: # Comment /* block */ // line\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "5",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	if !strings.Contains(result.Output, "$PATH") {
		t.Errorf("Expected output to contain $PATH")
	}
	if !strings.Contains(result.Output, "<xml>") {
		t.Errorf("Expected output to contain <xml>")
	}
}

func TestReadLinesTool_Execute_ComplexContent_NoTrailingNewline(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "no_newline.txt")

	// Create a file without trailing newline
	content := "Line 1\nLine 2\nLine 3" // No trailing newline

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "3",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	outputLines := strings.Split(result.Output, "\n")
	if len(outputLines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(outputLines))
	}
}

// ==================== BOUNDARY VALUE TESTS ====================

func TestReadLinesTool_Execute_Boundary_LargeLineNumbers(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "boundary.txt")

	// Create a file with 10 lines
	content := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7\nLine 8\nLine 9\nLine 10\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "9",
		"end":   "999999",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	outputLines := strings.Split(result.Output, "\n")
	if len(outputLines) != 2 {
		t.Errorf("Expected 2 lines (9 and 10), got %d", len(outputLines))
	}
}

func TestReadLinesTool_Execute_Boundary_SingleLineFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "single.txt")

	// Create a file with 100 lines - use proper number formatting
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, "Line "+strconv.Itoa(i))
	}
	content := strings.Join(lines, "\n") + "\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "50",
		"end":   "50",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	if result.Output != "50: Line 50" {
		t.Errorf("Expected '50: Line 50', got '%s'", result.Output)
	}
}

func TestReadLinesTool_Execute_Boundary_StartEqualsEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "equal.txt")

	content := "Line 1\nLine 2\nLine 3\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "2",
		"end":   "2",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	if result.Output != "2: Line 2" {
		t.Errorf("Expected '2: Line 2', got '%s'", result.Output)
	}
}

// ==================== WINDOWS LINE ENDINGS TESTS ====================

func TestReadLinesTool_Execute_WindowsLineEndings(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "windows.txt")

	// Create a file with Windows line endings (CRLF)
	content := "Line 1\r\nLine 2\r\nLine 3\r\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()
	result := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "3",
	})

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	// Note: bufio.Scanner strips \r, so lines should not contain \r
	if strings.Contains(result.Output, "\r") {
		t.Errorf("Expected output without CR characters")
	}
}

// ==================== MULTI-FILE COMPLEX TESTS ====================

func TestReadLinesTool_Execute_MultipleSequentialReads(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "sequential.txt")

	// Create a file with 50 lines
	var lines []string
	for i := 1; i <= 50; i++ {
		lines = append(lines, "Line "+string(rune(i+48)))
	}
	content := strings.Join(lines, "\n") + "\n"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadLinesTool()

	// Read lines 1-10
	result1 := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "1",
		"end":   "10",
	})
	if !result1.Success {
		t.Errorf("First read failed: %s", result1.Error)
	}

	// Read lines 20-30
	result2 := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "20",
		"end":   "30",
	})
	if !result2.Success {
		t.Errorf("Second read failed: %s", result2.Error)
	}

	// Read lines 40-50
	result3 := tool.Execute(map[string]string{
		"path":  testFile,
		"start": "40",
		"end":   "50",
	})
	if !result3.Success {
		t.Errorf("Third read failed: %s", result3.Error)
	}

	// Verify each read is correct
	lines1 := strings.Split(result1.Output, "\n")
	lines2 := strings.Split(result2.Output, "\n")
	lines3 := strings.Split(result3.Output, "\n")

	if len(lines1) != 10 {
		t.Errorf("First read expected 10 lines, got %d", len(lines1))
	}
	if len(lines2) != 11 {
		t.Errorf("Second read expected 11 lines, got %d", len(lines2))
	}
	if len(lines3) != 11 {
		t.Errorf("Third read expected 11 lines, got %d", len(lines3))
	}
}
