package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseToolCall(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ToolCall
		wantErr bool
	}{
		{
			name:  "valid bash command",
			input: `[TOOL:{"name":"bash","parameters":{"command":"ls -la"}}]`,
			want: &ToolCall{
				Name: "bash",
				Parameters: map[string]interface{}{
					"command": "ls -la",
				},
			},
			wantErr: false,
		},
		{
			name:  "valid read_file",
			input: `[TOOL:{"name":"read_file","parameters":{"path":"/test/file.txt"}}]`,
			want: &ToolCall{
				Name: "read_file",
				Parameters: map[string]interface{}{
					"path": "/test/file.txt",
				},
			},
			wantErr: false,
		},
		{
			name:  "valid write_file",
			input: `[TOOL:{"name":"write_file","parameters":{"path":"/test/file.txt","content":"hello"}}]`,
			want: &ToolCall{
				Name: "write_file",
				Parameters: map[string]interface{}{
					"path":    "/test/file.txt",
					"content": "hello",
				},
			},
			wantErr: false,
		},
		{
			name:  "valid read_lines",
			input: `[TOOL:{"name":"read_lines","parameters":{"path":"file.txt","start":1,"end":10}}]`,
			want: &ToolCall{
				Name: "read_lines",
				Parameters: map[string]interface{}{
					"path":  "file.txt",
					"start": 1.0,
					"end":   10.0,
				},
			},
			wantErr: false,
		},
		{
			name:  "valid insert_lines",
			input: `[TOOL:{"name":"insert_lines","parameters":{"path":"file.txt","line":5,"lines":"new line"}}]`,
			want: &ToolCall{
				Name: "insert_lines",
				Parameters: map[string]interface{}{
					"path":  "file.txt",
					"line":  5.0,
					"lines": "new line",
				},
			},
			wantErr: false,
		},
		{
			name:  "valid replace_lines",
			input: `[TOOL:{"name":"replace_lines","parameters":{"path":"file.txt","start":1,"end":5,"lines":"replacement"}}]`,
			want: &ToolCall{
				Name: "replace_lines",
				Parameters: map[string]interface{}{
					"path":  "file.txt",
					"start": 1.0,
					"end":   5.0,
					"lines": "replacement",
				},
			},
			wantErr: false,
		},
		{
			name:  "valid openai format",
			input: `{"id":"call_abc123","type":"function","function":{"name":"bash","arguments":"{\"command\":\"ls\"}"}}`,
			want: &ToolCall{
				ID:   "call_abc123",
				Name: "bash",
				Parameters: map[string]interface{}{
					"command": "ls",
				},
			},
			wantErr: false,
		},
		{
			name:    "invalid json",
			input:   `[TOOL:{invalid json}]`,
			wantErr: true,
		},
		{
			name:    "missing name",
			input:   `[TOOL:{"parameters":{}}]`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseToolCall(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseToolCall() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && tt.want != nil {
				if result.Name != tt.want.Name {
					t.Errorf("ParseToolCall() name = %v, want %v", result.Name, tt.want.Name)
				}
				for k, v := range tt.want.Parameters {
					if result.Parameters[k] != v {
						t.Errorf("ParseToolCall() parameter %v = %v, want %v", k, result.Parameters[k], v)
					}
				}
			}
		})
	}
}

func TestExecuteBash(t *testing.T) {
	te := NewToolExecutor()

	tests := []struct {
		name        string
		command     string
		wantSuccess bool
		checkOutput func(string) bool
	}{
		{
			name:        "simple echo",
			command:     "echo hello",
			wantSuccess: true,
			checkOutput: func(out string) bool { return strings.Contains(out, "hello") },
		},
		{
			name:        "pwd command",
			command:     "pwd",
			wantSuccess: true,
			checkOutput: func(out string) bool { return len(out) > 0 },
		},
		{
			name:        "command with quotes",
			command:     `echo "hello world"`,
			wantSuccess: true,
			checkOutput: func(out string) bool { return strings.Contains(out, "hello world") },
		},
		{
			name:        "failed command",
			command:     "nonexistent_command_xyz",
			wantSuccess: false,
			checkOutput: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]interface{}{
				"command": tt.command,
			}
			result := te.executeBash(params)

			if result.Success != tt.wantSuccess {
				t.Errorf("executeBash() success = %v, want %v", result.Success, tt.wantSuccess)
			}

			if tt.wantSuccess && tt.checkOutput != nil && !tt.checkOutput(result.Output) {
				t.Errorf("executeBash() output check failed: %s", result.Output)
			}
		})
	}
}

func TestExecuteReadFile(t *testing.T) {
	te := NewToolExecutor()

	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "line 1\nline 2\nline 3\n"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		want    *ToolResult
		wantErr bool
	}{
		{
			name: "read existing file",
			path: testFile,
			want: &ToolResult{
				Success: true,
				Output:  testContent,
			},
		},
		{
			name: "read non-existent file",
			path: "/nonexistent/file.txt",
			want: &ToolResult{
				Success: false,
			},
		},
		{
			name: "missing path parameter",
			path: "",
			want: &ToolResult{
				Success: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]interface{}{"path": tt.path}
			result := te.executeReadFile(params)

			if result.Success != tt.want.Success {
				t.Errorf("executeReadFile() success = %v, want %v", result.Success, tt.want.Success)
			}

			if tt.want.Success && result.Output != tt.want.Output {
				t.Errorf("executeReadFile() output mismatch")
			}
		})
	}
}

func TestExecuteWriteFile(t *testing.T) {
	te := NewToolExecutor()

	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		path    string
		content string
		want    *ToolResult
	}{
		{
			name:    "write new file",
			path:    filepath.Join(tmpDir, "new.txt"),
			content: "hello world",
			want: &ToolResult{
				Success: true,
			},
		},
		{
			name:    "write multi-line file",
			path:    filepath.Join(tmpDir, "multiline.txt"),
			content: "line 1\nline 2\nline 3",
			want: &ToolResult{
				Success: true,
			},
		},
		{
			name:    "overwrite existing file",
			path:    filepath.Join(tmpDir, "overwrite.txt"),
			content: "original",
			want: &ToolResult{
				Success: true,
			},
		},
		{
			name:    "missing path",
			path:    "",
			content: "test",
			want: &ToolResult{
				Success: false,
			},
		},
		{
			name:    "missing content",
			path:    filepath.Join(tmpDir, "nocontent.txt"),
			content: "",
			want: &ToolResult{
				Success: true, // Empty content is valid
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]interface{}{
				"path":    tt.path,
				"content": tt.content,
			}
			result := te.executeWriteFile(params)

			if result.Success != tt.want.Success {
				t.Errorf("executeWriteFile() success = %v, want %v", result.Success, tt.want.Success)
			}

			if tt.want.Success && result.Success {
				// Verify file was written
				content, err := os.ReadFile(tt.path)
				if err != nil {
					t.Errorf("Failed to read written file: %v", err)
					return
				}
				if string(content) != tt.content {
					t.Errorf("Written content mismatch: got %q, want %q", string(content), tt.content)
				}
			}
		})
	}
}

func TestExecuteWriteFileOutput(t *testing.T) {
	te := NewToolExecutor()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "Hello, World!"

	params := map[string]interface{}{
		"path":    testFile,
		"content": content,
	}

	result := te.executeWriteFile(params)

	if !result.Success {
		t.Fatalf("Expected success, got error: %s", result.Error)
	}

	// Verify output contains useful information
	if result.Output == "" {
		t.Error("Expected output to contain success message")
	}

	if !strings.Contains(result.Output, "File written successfully") {
		t.Errorf("Expected output to contain 'File written successfully', got: %s", result.Output)
	}

	if !strings.Contains(result.Output, testFile) {
		t.Errorf("Expected output to contain file path %s, got: %s", testFile, result.Output)
	}

	if !strings.Contains(result.Output, "bytes") {
		t.Errorf("Expected output to contain byte count, got: %s", result.Output)
	}

	// Verify file was actually written
	writtenContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	if string(writtenContent) != content {
		t.Errorf("Expected content %q, got %q", content, string(writtenContent))
	}
}

func TestExecuteReadLines(t *testing.T) {
	te := NewToolExecutor()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "lines.txt")
	content := "line 1\nline 2\nline 3\nline 4\nline 5\n"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name        string
		params      map[string]interface{}
		wantSuccess bool
		checkOutput func(string) bool
	}{
		{
			name: "read first 3 lines",
			params: map[string]interface{}{
				"path":  testFile,
				"start": 1.0,
				"end":   3.0,
			},
			wantSuccess: true,
			checkOutput: func(out string) bool {
				return strings.Contains(out, "1: line 1") &&
					strings.Contains(out, "2: line 2") &&
					strings.Contains(out, "3: line 3")
			},
		},
		{
			name: "read middle lines",
			params: map[string]interface{}{
				"path":  testFile,
				"start": 2.0,
				"end":   4.0,
			},
			wantSuccess: true,
			checkOutput: func(out string) bool {
				return strings.Contains(out, "2: line 2") &&
					strings.Contains(out, "3: line 3") &&
					strings.Contains(out, "4: line 4")
			},
		},
		{
			name: "start beyond file",
			params: map[string]interface{}{
				"path":  testFile,
				"start": 100.0,
				"end":   110.0,
			},
			wantSuccess: true,
			checkOutput: nil,
		},
		{
			name: "end beyond file",
			params: map[string]interface{}{
				"path":  testFile,
				"start": 4.0,
				"end":   100.0,
			},
			wantSuccess: true,
			checkOutput: func(out string) bool {
				return strings.Contains(out, "4: line 4") &&
					strings.Contains(out, "5: line 5")
			},
		},
		{
			name: "start > end",
			params: map[string]interface{}{
				"path":  testFile,
				"start": 5.0,
				"end":   2.0,
			},
			wantSuccess: false,
		},
		{
			name: "missing start",
			params: map[string]interface{}{
				"path": testFile,
				"end":  3.0,
			},
			wantSuccess: false,
		},
		{
			name: "missing end",
			params: map[string]interface{}{
				"path":  testFile,
				"start": 1.0,
			},
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := te.executeReadLines(tt.params)

			if result.Success != tt.wantSuccess {
				t.Errorf("executeReadLines() success = %v, want %v", result.Success, tt.wantSuccess)
			}

			if tt.wantSuccess && tt.checkOutput != nil && !tt.checkOutput(result.Output) {
				t.Errorf("executeReadLines() output check failed: %s", result.Output)
			}
		})
	}
}

func TestExecuteInsertLines(t *testing.T) {
	te := NewToolExecutor()

	tmpDir := t.TempDir()

	tests := []struct {
		name         string
		setupFile    string
		setupContent string
		params       map[string]interface{}
		wantSuccess  bool
		checkResult  func(string) bool
	}{
		{
			name:         "insert at beginning",
			setupFile:    filepath.Join(tmpDir, "insert1.txt"),
			setupContent: "original line 1\noriginal line 2\n",
			params: map[string]interface{}{
				"path":  filepath.Join(tmpDir, "insert1.txt"),
				"line":  1.0,
				"lines": "new header",
			},
			wantSuccess: true,
			checkResult: func(path string) bool {
				content, _ := os.ReadFile(path)
				return strings.HasPrefix(string(content), "new header")
			},
		},
		{
			name:         "insert in middle",
			setupFile:    filepath.Join(tmpDir, "insert2.txt"),
			setupContent: "line 1\nline 2\nline 3\n",
			params: map[string]interface{}{
				"path":  filepath.Join(tmpDir, "insert2.txt"),
				"line":  2.0,
				"lines": "inserted line",
			},
			wantSuccess: true,
			checkResult: func(path string) bool {
				content, _ := os.ReadFile(path)
				return strings.Contains(string(content), "line 1") &&
					strings.Contains(string(content), "inserted line") &&
					strings.Contains(string(content), "line 2")
			},
		},
		{
			name:         "insert multi-line",
			setupFile:    filepath.Join(tmpDir, "insert3.txt"),
			setupContent: "before\nafter\n",
			params: map[string]interface{}{
				"path":  filepath.Join(tmpDir, "insert3.txt"),
				"line":  2.0,
				"lines": "new1\nnew2\nnew3",
			},
			wantSuccess: true,
			checkResult: func(path string) bool {
				content, _ := os.ReadFile(path)
				return strings.Contains(string(content), "new1") &&
					strings.Contains(string(content), "new2") &&
					strings.Contains(string(content), "new3")
			},
		},
		{
			name:         "insert at end",
			setupFile:    filepath.Join(tmpDir, "insert4.txt"),
			setupContent: "existing\n",
			params: map[string]interface{}{
				"path":  filepath.Join(tmpDir, "insert4.txt"),
				"line":  9999.0,
				"lines": "appended",
			},
			wantSuccess: true,
			checkResult: func(path string) bool {
				content, _ := os.ReadFile(path)
				return strings.HasSuffix(strings.TrimSpace(string(content)), "appended")
			},
		},
		{
			name:         "create new file",
			setupFile:    "",
			setupContent: "",
			params: map[string]interface{}{
				"path":  filepath.Join(tmpDir, "newfile.txt"),
				"line":  1.0,
				"lines": "first line",
			},
			wantSuccess: true,
			checkResult: func(path string) bool {
				content, _ := os.ReadFile(path)
				return strings.Contains(string(content), "first line")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup file if needed
			if tt.setupFile != "" {
				err := os.WriteFile(tt.setupFile, []byte(tt.setupContent), 0644)
				if err != nil {
					t.Fatalf("Failed to setup test file: %v", err)
				}
			}

			result := te.executeInsertLines(tt.params)

			if result.Success != tt.wantSuccess {
				t.Errorf("executeInsertLines() success = %v, want %v", result.Success, tt.wantSuccess)
			}

			if tt.wantSuccess && tt.checkResult != nil {
				path := tt.params["path"].(string)
				if !tt.checkResult(path) {
					content, _ := os.ReadFile(path)
					t.Errorf("executeInsertLines() result check failed: %s", string(content))
				}
			}
		})
	}
}

func TestExecuteInsertLinesOutput(t *testing.T) {
	te := NewToolExecutor()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "line 1\nline 2\nline 3\n"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	params := map[string]interface{}{
		"path":  testFile,
		"line":  2.0,
		"lines": "inserted line",
	}

	result := te.executeInsertLines(params)

	if !result.Success {
		t.Fatalf("Expected success, got error: %s", result.Error)
	}

	// Verify output contains useful information
	if result.Output == "" {
		t.Error("Expected output to contain success message")
	}

	if !strings.Contains(result.Output, "Inserted") {
		t.Errorf("Expected output to contain 'Inserted', got: %s", result.Output)
	}

	if !strings.Contains(result.Output, testFile) {
		t.Errorf("Expected output to contain file path %s, got: %s", testFile, result.Output)
	}

	// Verify file was actually modified
	writtenContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	if !strings.Contains(string(writtenContent), "inserted line") {
		t.Errorf("Expected content to contain 'inserted line', got: %s", string(writtenContent))
	}
}

func TestExecutePatch(t *testing.T) {
	te := NewToolExecutor()

	tmpDir := t.TempDir()

	tests := []struct {
		name         string
		setupContent string
		diff         string
		wantSuccess  bool
		checkResult  func(string) bool
	}{
		{
			name:         "simple single-line change",
			setupContent: "package main\n\nfunc main() {\n    oldLine()\n}\n",
			diff:         "--- a/test.go\n+++ b/test.go\n@@ -4,1 +4,1 @@\n-    oldLine()\n+    newLine()\n",
			wantSuccess:  true,
			checkResult: func(path string) bool {
				content, _ := os.ReadFile(path)
				return strings.Contains(string(content), "newLine()") &&
					!strings.Contains(string(content), "oldLine()")
			},
		},
		{
			name:         "add new line",
			setupContent: "package main\n\nfunc main() {\n    fmt.Println(\"hello\")\n}\n",
			diff:         "--- a/test.go\n+++ b/test.go\n@@ -4,1 +4,2 @@\n     fmt.Println(\"hello\")\n+    fmt.Println(\"world\")\n",
			wantSuccess:  true,
			checkResult: func(path string) bool {
				content, _ := os.ReadFile(path)
				return strings.Contains(string(content), "fmt.Println(\"hello\")") &&
					strings.Contains(string(content), "fmt.Println(\"world\")")
			},
		},
		{
			name:         "delete line",
			setupContent: "package main\n\nfunc main() {\n    // comment\n    oldCode()\n}\n",
			diff:         "--- a/test.go\n+++ b/test.go\n@@ -4,2 +4,1 @@\n-    // comment\n     oldCode()\n",
			wantSuccess:  true,
			checkResult: func(path string) bool {
				content, _ := os.ReadFile(path)
				return !strings.Contains(string(content), "// comment") &&
					strings.Contains(string(content), "oldCode()")
			},
		},
		{
			name:         "file not found",
			setupContent: "",
			diff:         "--- a/nonexistent.go\n+++ b/nonexistent.go\n@@ -1,1 +1,1 @@\n-old\n+new\n",
			wantSuccess:  false,
		},
		{
			name:         "empty diff",
			setupContent: "content\n",
			diff:         "",
			wantSuccess:  false,
		},
		{
			name:         "invalid diff format",
			setupContent: "content\n",
			diff:         "this is not a valid diff\n",
			wantSuccess:  false,
		},
		{
			name:         "context mismatch",
			setupContent: "package main\n\nfunc main() {\n    actualCode()\n}\n",
			diff:         "--- a/test.go\n+++ b/test.go\n@@ -4,1 +4,1 @@\n-    wrongCode()\n+    newCode()\n",
			wantSuccess:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, "test.go")

			// Setup file if content provided
			if tt.setupContent != "" {
				err := os.WriteFile(testFile, []byte(tt.setupContent), 0644)
				if err != nil {
					t.Fatalf("Failed to setup test file: %v", err)
				}
			}

			params := map[string]interface{}{
				"path": testFile,
				"diff": tt.diff,
			}

			result := te.executePatch(params)

			if result.Success != tt.wantSuccess {
				t.Errorf("executePatch() success = %v, want %v, error = %s", result.Success, tt.wantSuccess, result.Error)
			}

			if tt.wantSuccess && tt.checkResult != nil {
				if !tt.checkResult(testFile) {
					content, _ := os.ReadFile(testFile)
					t.Errorf("executePatch() result check failed: %s", string(content))
				}
			}

			// Check for patches_applied in Extra on success
			if tt.wantSuccess && result.Success {
				if patchesApplied, ok := result.Extra["patches_applied"]; !ok {
					t.Error("Expected patches_applied in Extra on success")
				} else if patchesApplied.(int) < 1 {
					t.Errorf("Expected at least 1 patch applied, got %d", patchesApplied)
				}
			}
		})
	}
}

func TestExecutePatchSecurity(t *testing.T) {
	te := NewToolExecutor()

	// Test directory traversal prevention - use relative path with ..
	// Test directory traversal prevention - use relative path with ..
	params := map[string]interface{}{
		"path": "../../../etc/passwd",
		"diff": "--- a/passwd\n+++ b/passwd\n@@ -1,1 +1,1 @@\n-old\n+hacked\n",
	}

	result := te.executePatch(params)

	if result.Success {
		t.Error("Expected directory traversal to be blocked")
	}

	if !strings.Contains(result.Error, "directory traversal") {
		t.Errorf("Expected 'directory traversal' error, got: %s", result.Error)
	}
}

func TestExecuteReplaceLines(t *testing.T) {
	te := NewToolExecutor()
	tmpDir := t.TempDir()

	tests := []struct {
		name         string
		mode         string // "line-number" or "search"
		setupContent string
		params       map[string]interface{}
		wantSuccess  bool
		checkResult  func(string) bool
	}{
		{
			name:         "replace by line number",
			mode:         "line-number",
			setupContent: "line 1\nline 2\nline 3\nline 4\nline 5\n",
			params: map[string]interface{}{
				"path":  filepath.Join(tmpDir, "replace1.txt"),
				"start": 2.0,
				"end":   3.0,
				"lines": "replacement 1\nreplacement 2",
			},
			wantSuccess: true,
			checkResult: func(path string) bool {
				content, _ := os.ReadFile(path)
				return strings.Contains(string(content), "line 1") &&
					strings.Contains(string(content), "replacement 1") &&
					strings.Contains(string(content), "replacement 2") &&
					strings.Contains(string(content), "line 4")
			},
		},
		{
			name:         "replace entire file",
			mode:         "line-number",
			setupContent: "old content\n",
			params: map[string]interface{}{
				"path":  filepath.Join(tmpDir, "replace2.txt"),
				"start": 1.0,
				"end":   999.0,
				"lines": "new complete content",
			},
			wantSuccess: true,
			checkResult: func(path string) bool {
				content, _ := os.ReadFile(path)
				return strings.Contains(string(content), "new complete content")
			},
		},
		{
			name:         "search and replace",
			mode:         "search",
			setupContent: "func oldName() {\n    return \"old\"\n}\n",
			params: map[string]interface{}{
				"path":    filepath.Join(tmpDir, "replace3.txt"),
				"search":  "oldName",
				"replace": "newName",
			},
			wantSuccess: true,
			checkResult: func(path string) bool {
				content, _ := os.ReadFile(path)
				return strings.Contains(string(content), "func newName()") &&
					!strings.Contains(string(content), "oldName")
			},
		},
		{
			name:         "search replace multiple",
			mode:         "search",
			setupContent: "TODO: fix this\nTODO: fix that\nTODO: fix another\n",
			params: map[string]interface{}{
				"path":    filepath.Join(tmpDir, "replace4.txt"),
				"search":  "TODO",
				"replace": "IMPLEMENTED",
				"count":   2.0,
			},
			wantSuccess: true,
			checkResult: func(path string) bool {
				content, _ := os.ReadFile(path)
				count := strings.Count(string(content), "IMPLEMENTED")
				return count == 2
			},
		},
		{
			name:         "search not found",
			mode:         "search",
			setupContent: "some content\n",
			params: map[string]interface{}{
				"path":    filepath.Join(tmpDir, "replace5.txt"),
				"search":  "notfound",
				"replace": "replacement",
			},
			wantSuccess: false,
		},
		{
			name:         "missing parameters",
			mode:         "line-number",
			setupContent: "content\n",
			params: map[string]interface{}{
				"path": filepath.Join(tmpDir, "replace6.txt"),
			},
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup file
			if err := os.WriteFile(tt.params["path"].(string), []byte(tt.setupContent), 0644); err != nil {
				t.Fatalf("Failed to setup test file: %v", err)
			}

			result := te.executeReplaceLines(tt.params)

			if result.Success != tt.wantSuccess {
				t.Errorf("executeReplaceLines() success = %v, want %v", result.Success, tt.wantSuccess)
			}

			if tt.wantSuccess && tt.checkResult != nil {
				path := tt.params["path"].(string)
				if !tt.checkResult(path) {
					content, _ := os.ReadFile(path)
					t.Errorf("executeReplaceLines() result check failed: %s", string(content))
				}
			}
		})
	}
}

func TestExecuteReplaceTextOutput(t *testing.T) {
	te := NewToolExecutor()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "Hello world! Hello again!\n"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	params := map[string]interface{}{
		"path":    testFile,
		"search":  "Hello",
		"replace": "Hi",
		"count":   -1.0, // Replace all
	}

	result := te.executeReplaceText(params)

	if !result.Success {
		t.Fatalf("Expected success, got error: %s", result.Error)
	}

	// Verify output contains useful information
	if result.Output == "" {
		t.Error("Expected output to contain success message")
	}

	if !strings.Contains(result.Output, "Replaced") {
		t.Errorf("Expected output to contain 'Replaced', got: %s", result.Output)
	}

	if !strings.Contains(result.Output, testFile) {
		t.Errorf("Expected output to contain file path %s, got: %s", testFile, result.Output)
	}

	// Verify file was actually modified
	writtenContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	if strings.Contains(string(writtenContent), "Hello") {
		t.Errorf("Expected content to have all 'Hello' replaced, got: %s", string(writtenContent))
	}

	if !strings.Contains(string(writtenContent), "Hi") {
		t.Errorf("Expected content to contain 'Hi', got: %s", string(writtenContent))
	}
}

func TestToolExecutorStats(t *testing.T) {
	te := NewToolExecutor()

	// Make some tool calls
	// Bash commands should succeed in most environments
	te.Execute(&ToolCall{Name: "bash", Parameters: map[string]interface{}{"command": "echo test"}})

	// Unknown tool should always fail
	te.Execute(&ToolCall{Name: "unknown_tool", Parameters: map[string]interface{}{}})

	stats := te.Stats()

	if stats.TotalCalls != 2 {
		t.Errorf("Expected 2 total calls, got %d", stats.TotalCalls)
	}
	// At least the unknown tool should have failed
	if stats.FailedCalls < 1 {
		t.Errorf("Expected at least 1 failed call, got %d", stats.FailedCalls)
	}
}
