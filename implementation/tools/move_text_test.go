package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ===== Tests for move_text tool =====

// TestExecute_MoveText_MissingSourcePath verifies that missing source_path parameter returns an error.
func TestExecute_MoveText_MissingSourcePath(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_start": 1.0,
			"source_end":   3.0,
			"target_path":  "/tmp/target.txt",
			"target_line":  1.0,
		},
	})
	if result.Success {
		t.Error("Expected failure for missing source_path parameter")
	}
	if !strings.Contains(result.Error, "missing required parameter") {
		t.Errorf("Expected 'missing required parameter' error, got: %s", result.Error)
	}
}

// TestExecute_MoveText_MissingSourceStart verifies that missing source_start parameter returns an error.
func TestExecute_MoveText_MissingSourceStart(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	os.WriteFile(sourceFile, []byte("line1\nline2\nline3\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path": sourceFile,
			"source_end":  3.0,
			"target_path": filepath.Join(tmpDir, "target.txt"),
			"target_line": 1.0,
		},
	})
	if result.Success {
		t.Error("Expected failure for missing source_start parameter")
	}
	if !strings.Contains(result.Error, "missing required parameter") {
		t.Errorf("Expected 'missing required parameter' error, got: %s", result.Error)
	}
}

// TestExecute_MoveText_MissingSourceEnd verifies that missing source_end parameter returns an error.
func TestExecute_MoveText_MissingSourceEnd(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	os.WriteFile(sourceFile, []byte("line1\nline2\nline3\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  sourceFile,
			"source_start": 1.0,
			"target_path":  filepath.Join(tmpDir, "target.txt"),
			"target_line":  1.0,
		},
	})
	if result.Success {
		t.Error("Expected failure for missing source_end parameter")
	}
}

// TestExecute_MoveText_MissingTargetPath verifies that missing target_path parameter returns an error.
func TestExecute_MoveText_MissingTargetPath(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	os.WriteFile(sourceFile, []byte("line1\nline2\nline3\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  sourceFile,
			"source_start": 1.0,
			"source_end":   3.0,
			"target_line":  1.0,
		},
	})
	if result.Success {
		t.Error("Expected failure for missing target_path parameter")
	}
}

// TestExecute_MoveText_MissingTargetLine verifies that missing target_line parameter returns an error.
func TestExecute_MoveText_MissingTargetLine(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	os.WriteFile(sourceFile, []byte("line1\nline2\nline3\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path": sourceFile,
			"source_start": 1.0,
			"source_end":  3.0,
			"target_path": filepath.Join(tmpDir, "target.txt"),
		},
	})
	if result.Success {
		t.Error("Expected failure for missing target_line parameter")
	}
}

// TestExecute_MoveText_SourceNotFound verifies that a non-existent source file returns an error.
func TestExecute_MoveText_SourceNotFound(t *testing.T) {
	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  "/nonexistent/source.txt",
			"source_start": 1.0,
			"source_end":   3.0,
			"target_path":  "/tmp/target.txt",
			"target_line":  1.0,
		},
	})
	if result.Success {
		t.Error("Expected failure for non-existent source file")
	}
	if !strings.Contains(result.Error, "source file not found") {
		t.Errorf("Expected 'source file not found' error, got: %s", result.Error)
	}
}

// TestExecute_MoveText_InvalidLineRange verifies that source_start > source_end returns an error.
func TestExecute_MoveText_InvalidLineRange(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	os.WriteFile(sourceFile, []byte("line1\nline2\nline3\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  sourceFile,
			"source_start": 5.0,
			"source_end":   2.0,
			"target_path":  filepath.Join(tmpDir, "target.txt"),
			"target_line":  1.0,
		},
	})
	if result.Success {
		t.Error("Expected failure for invalid line range")
	}
	if !strings.Contains(result.Error, "invalid line range") {
		t.Errorf("Expected 'invalid line range' error, got: %s", result.Error)
	}
}

// TestExecute_MoveText_LineRangeOutOfBounds verifies that requesting lines beyond file length returns an error.
func TestExecute_MoveText_LineRangeOutOfBounds(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	os.WriteFile(sourceFile, []byte("line1\nline2\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  sourceFile,
			"source_start": 10.0,
			"source_end":   15.0,
			"target_path":  filepath.Join(tmpDir, "target.txt"),
			"target_line":  1.0,
		},
	})
	if result.Success {
		t.Error("Expected failure for line range out of bounds")
	}
	if !strings.Contains(result.Error, "out of bounds") {
		t.Errorf("Expected 'out of bounds' error, got: %s", result.Error)
	}
}

// TestExecute_MoveText_InvalidSourceStart verifies that source_start < 1 returns an error.
func TestExecute_MoveText_InvalidSourceStart(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	os.WriteFile(sourceFile, []byte("line1\nline2\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  sourceFile,
			"source_start": 0.0,
			"source_end":   2.0,
			"target_path":  filepath.Join(tmpDir, "target.txt"),
			"target_line":  1.0,
		},
	})
	if result.Success {
		t.Error("Expected failure for invalid source_start")
	}
	if !strings.Contains(result.Error, "invalid source_start") {
		t.Errorf("Expected 'invalid source_start' error, got: %s", result.Error)
	}
}

// TestExecute_MoveText_InvalidTargetLine verifies that target_line < 1 returns an error.
func TestExecute_MoveText_InvalidTargetLine(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	os.WriteFile(sourceFile, []byte("line1\nline2\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  sourceFile,
			"source_start": 1.0,
			"source_end":   2.0,
			"target_path":  filepath.Join(tmpDir, "target.txt"),
			"target_line":  0.0,
		},
	})
	if result.Success {
		t.Error("Expected failure for invalid target_line")
	}
	if !strings.Contains(result.Error, "invalid target_line") {
		t.Errorf("Expected 'invalid target_line' error, got: %s", result.Error)
	}
}

// TestExecute_MoveText_CrossFile_Basic tests moving lines from one file to another.
func TestExecute_MoveText_CrossFile_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	targetFile := filepath.Join(tmpDir, "target.txt")

	// Create source file with 5 lines
	os.WriteFile(sourceFile, []byte("line 1\nline 2\nline 3\nline 4\nline 5\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  sourceFile,
			"source_start": 2.0,
			"source_end":   4.0,
			"target_path":  targetFile,
			"target_line":  1.0,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	// Verify source file has the moved lines removed
	sourceContent, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Fatalf("Failed to read source file: %v", err)
	}
	if !strings.Contains(string(sourceContent), "line 1") {
		t.Error("Expected source file to still contain 'line 1'")
	}
	if !strings.Contains(string(sourceContent), "line 5") {
		t.Error("Expected source file to still contain 'line 5'")
	}
	if strings.Contains(string(sourceContent), "line 2") {
		t.Error("Expected source file NOT to contain 'line 2' (moved)")
	}
	if strings.Contains(string(sourceContent), "line 3") {
		t.Error("Expected source file NOT to contain 'line 3' (moved)")
	}
	if strings.Contains(string(sourceContent), "line 4") {
		t.Error("Expected source file NOT to contain 'line 4' (moved)")
	}

	// Verify target file has the moved lines
	targetContent, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("Failed to read target file: %v", err)
	}
	if !strings.Contains(string(targetContent), "line 2") {
		t.Error("Expected target file to contain 'line 2'")
	}
	if !strings.Contains(string(targetContent), "line 3") {
		t.Error("Expected target file to contain 'line 3'")
	}
	if !strings.Contains(string(targetContent), "line 4") {
		t.Error("Expected target file to contain 'line 4'")
	}

	// Verify Extra data
	if result.Extra == nil {
		t.Fatal("Expected Extra map in result")
	}
	if linesMoved, ok := result.Extra["linesMoved"].(int); !ok || linesMoved != 3 {
		t.Errorf("Expected linesMoved to be 3, got %v", result.Extra["linesMoved"])
	}
}

// TestExecute_MoveText_CrossFile_ToExistingFile tests moving lines to an existing target file.
func TestExecute_MoveText_CrossFile_ToExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	targetFile := filepath.Join(tmpDir, "target.txt")

	// Create source file
	os.WriteFile(sourceFile, []byte("source A\nsource B\nsource C\n"), 0644)
	// Create target file with existing content
	os.WriteFile(targetFile, []byte("target X\ntarget Y\ntarget Z\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  sourceFile,
			"source_start": 2.0,
			"source_end":   2.0,
			"target_path":  targetFile,
			"target_line":  2.0,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	// Verify target file has the inserted line at position 2
	targetContent, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("Failed to read target file: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(targetContent), "\n"), "\n")
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines in target, got %d: %v", len(lines), lines)
	}
	if lines[0] != "target X" {
		t.Errorf("Expected first line 'target X', got '%s'", lines[0])
	}
	if lines[1] != "source B" {
		t.Errorf("Expected second line 'source B', got '%s'", lines[1])
	}
	if lines[2] != "target Y" {
		t.Errorf("Expected third line 'target Y', got '%s'", lines[2])
	}
	if lines[3] != "target Z" {
		t.Errorf("Expected fourth line 'target Z', got '%s'", lines[3])
	}

	// Verify source file has line removed
	sourceContent, _ := os.ReadFile(sourceFile)
	sourceLines := strings.Split(strings.TrimSuffix(string(sourceContent), "\n"), "\n")
	if len(sourceLines) != 2 {
		t.Errorf("Expected 2 lines remaining in source, got %d", len(sourceLines))
	}
}

// TestExecute_MoveText_CrossFile_CreateTarget verifies target file is created if it doesn't exist.
func TestExecute_MoveText_CrossFile_CreateTarget(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	targetFile := filepath.Join(tmpDir, "new_file.txt")

	os.WriteFile(sourceFile, []byte("line 1\nline 2\nline 3\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  sourceFile,
			"source_start": 1.0,
			"source_end":   2.0,
			"target_path":  targetFile,
			"target_line":  1.0,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	// Verify target file was created
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		t.Error("Expected target file to be created")
	}

	// Verify content
	targetContent, _ := os.ReadFile(targetFile)
	if !strings.Contains(string(targetContent), "line 1") {
		t.Error("Expected target to contain 'line 1'")
	}
	if !strings.Contains(string(targetContent), "line 2") {
		t.Error("Expected target to contain 'line 2'")
	}
}

// TestExecute_MoveText_CrossFile_CreateNestedDirectories verifies parent directories are created.
func TestExecute_MoveText_CrossFile_CreateNestedDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	targetFile := filepath.Join(tmpDir, "deep", "nested", "dir", "target.txt")

	os.WriteFile(sourceFile, []byte("line 1\nline 2\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  sourceFile,
			"source_start": 1.0,
			"source_end":   1.0,
			"target_path":  targetFile,
			"target_line":  1.0,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	// Verify target file exists
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		t.Error("Expected target file to be created with nested directories")
	}
}

// TestExecute_MoveText_SameFile_MoveToBeginning tests moving lines to the beginning of the same file.
func TestExecute_MoveText_SameFile_MoveToBeginning(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create file with content: A B C D E
	os.WriteFile(testFile, []byte("A\nB\nC\nD\nE\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  testFile,
			"source_start": 4.0,
			"source_end":   4.0,
			"target_path":  testFile,
			"target_line":  1.0,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	// Expected result: D A B C E (moved line 4 to line 1)
	content, _ := os.ReadFile(testFile)
	lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	if len(lines) != 5 {
		t.Errorf("Expected 5 lines, got %d: %v", len(lines), lines)
	}
	expected := []string{"D", "A", "B", "C", "E"}
	for i, exp := range expected {
		if lines[i] != exp {
			t.Errorf("Line %d: expected '%s', got '%s'", i+1, exp, lines[i])
		}
	}
}

// TestExecute_MoveText_SameFile_MoveToEnd tests moving lines to the end of the same file.
func TestExecute_MoveText_SameFile_MoveToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create file: A B C D E
	os.WriteFile(testFile, []byte("A\nB\nC\nD\nE\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  testFile,
			"source_start": 2.0,
			"source_end":   2.0,
			"target_path":  testFile,
			"target_line":  999.0, // beyond end
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	// Expected: A C D E B (moved line 2 to end)
	content, _ := os.ReadFile(testFile)
	lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	expected := []string{"A", "C", "D", "E", "B"}
	for i, exp := range expected {
		if i < len(lines) && lines[i] != exp {
			t.Errorf("Line %d: expected '%s', got '%s'", i+1, exp, lines[i])
		}
	}
}

// TestExecute_MoveText_SameFile_MultiLineBlock tests moving multiple lines within the same file.
func TestExecute_MoveText_SameFile_MultiLineBlock(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create file: 1 2 3 4 5 6 7 8
	os.WriteFile(testFile, []byte("1\n2\n3\n4\n5\n6\n7\n8\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  testFile,
			"source_start": 3.0,
			"source_end":   5.0,
			"target_path":  testFile,
			"target_line":  2.0,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	// Move lines 3-5 (which are "3", "4", "5") to position 2
	// After removal: 1 2 6 7 8
	// Insert at 2: 1 3 4 5 2 6 7 8
	content, _ := os.ReadFile(testFile)
	lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	expected := []string{"1", "3", "4", "5", "2", "6", "7", "8"}
	for i, exp := range expected {
		if i < len(lines) && lines[i] != exp {
			t.Errorf("Line %d: expected '%s', got '%s'", i+1, exp, lines[i])
		}
	}
}

// TestExecute_MoveText_SameFile_MoveToEnd_Adjusted verifies line adjustment when moving to end.
func TestExecute_MoveText_SameFile_MoveToEnd_Adjusted(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create file: A B C D E
	os.WriteFile(testFile, []byte("A\nB\nC\nD\nE\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  testFile,
			"source_start": 1.0,
			"source_end":   1.0,
			"target_path":  testFile,
			"target_line":  5.0, // After removal of line 1, file has 4 lines, so 5 is append
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	// Move line 1 to line 5
	// After removal: B C D E (4 lines)
	// Adjusted target: 5 - 1 = 4 (still append since it's at the end)
	// Result: B C D E A
	content, _ := os.ReadFile(testFile)
	lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	expected := []string{"B", "C", "D", "E", "A"}
	for i, exp := range expected {
		if i < len(lines) && lines[i] != exp {
			t.Errorf("Line %d: expected '%s', got '%s'", i+1, exp, lines[i])
		}
	}
}

// TestExecute_MoveText_SingleLine tests moving a single line between files.
func TestExecute_MoveText_SingleLine(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	targetFile := filepath.Join(tmpDir, "target.txt")

	os.WriteFile(sourceFile, []byte("first\nmiddle\nlast\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  sourceFile,
			"source_start": 2.0,
			"source_end":   2.0,
			"target_path":  targetFile,
			"target_line":  1.0,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	// Verify source
	sourceContent, _ := os.ReadFile(sourceFile)
	sourceLines := strings.Split(strings.TrimSuffix(string(sourceContent), "\n"), "\n")
	if len(sourceLines) != 2 {
		t.Errorf("Expected 2 lines in source, got %d", len(sourceLines))
	}

	// Verify target has only the moved line
	targetContent, _ := os.ReadFile(targetFile)
	targetLines := strings.Split(strings.TrimSuffix(string(targetContent), "\n"), "\n")
	if len(targetLines) != 1 {
		t.Errorf("Expected 1 line in target, got %d", len(targetLines))
	}
	if targetLines[0] != "middle" {
		t.Errorf("Expected 'middle' in target, got '%s'", targetLines[0])
	}
}

// TestExecute_MoveText_MoveAllLines tests moving all lines from source to target.
func TestExecute_MoveText_MoveAllLines(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	targetFile := filepath.Join(tmpDir, "target.txt")

	os.WriteFile(sourceFile, []byte("line 1\nline 2\nline 3\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  sourceFile,
			"source_start": 1.0,
			"source_end":   3.0,
			"target_path":  targetFile,
			"target_line":  1.0,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	// Verify source is now empty
	sourceContent, _ := os.ReadFile(sourceFile)
	if strings.TrimSpace(string(sourceContent)) != "" {
		t.Errorf("Expected source file to be empty, got: %s", sourceContent)
	}

	// Verify target has all content
	targetContent, _ := os.ReadFile(targetFile)
	if !strings.Contains(string(targetContent), "line 1") ||
		!strings.Contains(string(targetContent), "line 2") ||
		!strings.Contains(string(targetContent), "line 3") {
		t.Errorf("Expected target to have all lines, got: %s", targetContent)
	}
}

// TestExecute_MoveText_AppendToTarget tests appending to end of existing target file.
func TestExecute_MoveText_AppendToTarget(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	targetFile := filepath.Join(tmpDir, "target.txt")

	os.WriteFile(sourceFile, []byte("new line A\nnew line B\n"), 0644)
	os.WriteFile(targetFile, []byte("existing 1\nexisting 2\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  sourceFile,
			"source_start": 1.0,
			"source_end":   2.0,
			"target_path":  targetFile,
			"target_line":  999.0, // append
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	targetContent, _ := os.ReadFile(targetFile)
	lines := strings.Split(strings.TrimSuffix(string(targetContent), "\n"), "\n")
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines, got %d: %v", len(lines), lines)
	}
	if lines[2] != "new line A" {
		t.Errorf("Expected 'new line A' at position 3, got '%s'", lines[2])
	}
	if lines[3] != "new line B" {
		t.Errorf("Expected 'new line B' at position 4, got '%s'", lines[3])
	}
}

// TestExecute_MoveText_ReturnsContent verifies the moved content is returned in Extra.
func TestExecute_MoveText_ReturnsContent(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	targetFile := filepath.Join(tmpDir, "target.txt")

	content := "hello world\ngoodbye world\n"
	os.WriteFile(sourceFile, []byte(content), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  sourceFile,
			"source_start": 1.0,
			"source_end":   2.0,
			"target_path":  targetFile,
			"target_line":  1.0,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	if result.Extra == nil {
		t.Fatal("Expected Extra map")
		return
	}

	// Verify content in Extra
	if movedContent, ok := result.Extra["content"].(string); !ok {
		t.Error("Expected 'content' in Extra")
	} else {
		if movedContent != "hello world\ngoodbye world" {
			t.Errorf("Unexpected content: %q", movedContent)
		}
	}

	// Verify paths in Extra
	if srcPath, ok := result.Extra["sourcePath"].(string); !ok || srcPath != sourceFile {
		t.Errorf("Unexpected sourcePath: %v", result.Extra["sourcePath"])
	}
	if tgtPath, ok := result.Extra["targetPath"].(string); !ok || tgtPath != targetFile {
		t.Errorf("Unexpected targetPath: %v", result.Extra["targetPath"])
	}
}

// TestExecute_MoveText_OutputContainsSummary verifies the output contains a human-readable summary.
func TestExecute_MoveText_OutputContainsSummary(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	targetFile := filepath.Join(tmpDir, "target.txt")

	os.WriteFile(sourceFile, []byte("line1\nline2\nline3\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  sourceFile,
			"source_start": 1.0,
			"source_end":   3.0,
			"target_path":  targetFile,
			"target_line":  1.0,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	// Output should contain summary information
	if !strings.Contains(result.Output, "Moved") {
		t.Errorf("Expected 'Moved' in output, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "line(s)") {
		t.Errorf("Expected 'line(s)' in output, got: %s", result.Output)
	}
}

// TestToolExecutor_ReadOnly_MoveTextBlocked verifies move_text is blocked in read-only mode.
func TestToolExecutor_ReadOnly_MoveTextBlocked(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	os.WriteFile(sourceFile, []byte("line1\nline2\n"), 0644)

	te := NewToolExecutor()
	te.SetReadOnly(true)

	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  sourceFile,
			"source_start": 1.0,
			"source_end":   1.0,
			"target_path":  filepath.Join(tmpDir, "target.txt"),
			"target_line":  1.0,
		},
	})
	if result.Success {
		t.Error("Expected move_text to fail in read-only mode")
	}
	if !strings.Contains(result.Error, "not available in read-only mode") {
		t.Errorf("Expected 'not available in read-only mode' error, got: %s", result.Error)
	}
}

// TestExecute_MoveText_SameFilePath_Cleaned verifies same-file detection works with path normalization.
func TestExecute_MoveText_SameFilePath_Cleaned(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	os.WriteFile(testFile, []byte("A\nB\nC\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  filepath.Join(tmpDir, "test.txt"),
			"source_start": 3.0,
			"source_end":   3.0,
			"target_path":  filepath.Join(tmpDir, "test.txt"),
			"target_line":  1.0,
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	// Move line 3 to line 1: C A B
	content, _ := os.ReadFile(testFile)
	lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	expected := []string{"C", "A", "B"}
	for i, exp := range expected {
		if i < len(lines) && lines[i] != exp {
			t.Errorf("Line %d: expected '%s', got '%s'", i+1, exp, lines[i])
		}
	}
}

// TestExecute_MoveText_CrossFile_AppendToExisting tests appending to existing target at specific position.
func TestExecute_MoveText_CrossFile_AppendToExisting(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	targetFile := filepath.Join(tmpDir, "target.txt")

	os.WriteFile(sourceFile, []byte("source 1\nsource 2\nsource 3\n"), 0644)
	os.WriteFile(targetFile, []byte("target A\ntarget B\n"), 0644)

	te := NewToolExecutor()
	result := te.Execute(context.Background(), &ToolCall{
		Name: "move_text",
		Parameters: map[string]interface{}{
			"source_path":  sourceFile,
			"source_start": 2.0,
			"source_end":   2.0,
			"target_path":  targetFile,
			"target_line":  3.0, // append after existing content
		},
	})
	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	targetContent, _ := os.ReadFile(targetFile)
	lines := strings.Split(strings.TrimSuffix(string(targetContent), "\n"), "\n")
	expected := []string{"target A", "target B", "source 2"}
	for i, exp := range expected {
		if i < len(lines) && lines[i] != exp {
			t.Errorf("Line %d: expected '%s', got '%s'", i+1, exp, lines[i])
		}
	}
}
