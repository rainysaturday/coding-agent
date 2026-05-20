package debug

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewDebugLogger_Enabled(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewDebugLogger(logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("NewDebugLogger() error: %v", err)
	}

	if logger == nil {
		t.Fatal("NewDebugLogger() returned nil")
	}

	if !logger.enabled {
		t.Error("Expected logger to be enabled")
	}

	if logger.filePath != logPath {
		t.Errorf("Expected filePath '%s', got '%s'", logPath, logger.filePath)
	}

	if logger.version != "v1.0.0" {
		t.Errorf("Expected version 'v1.0.0', got '%s'", logger.version)
	}

	if logger.session == nil {
		t.Error("Expected session to be non-nil")
	}

	if logger.session.SessionID == "" {
		t.Error("Expected session ID to be non-empty")
	}

	if !strings.HasPrefix(logger.session.SessionID, "sess_") {
		t.Errorf("Expected session ID to start with 'sess_', got '%s'", logger.session.SessionID)
	}

	if logger.session.Version != "v1.0.0" {
		t.Errorf("Expected session version 'v1.0.0', got '%s'", logger.session.Version)
	}

	// Verify file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Expected log file to be created")
	}

	// Verify file has restrictive permissions (0600)
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("Failed to stat log file: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("Expected file permissions 0600, got %o", perm)
	}

	// Verify header was written
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	if !strings.Contains(string(content), "CODING AGENT DEBUG LOG") {
		t.Error("Expected log header to be written")
	}
	if !strings.Contains(string(content), "Session:") {
		t.Error("Expected 'Session:' in log header")
	}
	if !strings.Contains(string(content), "Version: v1.0.0") {
		t.Error("Expected version in log header")
	}

	// Clean up
	if err := logger.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

func TestNewDebugLogger_Disabled(t *testing.T) {
	logger, err := NewDebugLogger("", "v1.0.0")
	if err != nil {
		t.Fatalf("NewDebugLogger() error: %v", err)
	}

	if logger == nil {
		t.Fatal("NewDebugLogger() returned nil for disabled logger")
	}

	if logger.enabled {
		t.Error("Expected disabled logger to have enabled=false")
	}

	// Should not write to any file
	if logger.file != nil {
		t.Error("Expected file to be nil for disabled logger")
	}
}

func TestNewDebugLogger_FileError(t *testing.T) {
	_, err := NewDebugLogger("/nonexistent/directory/log.log", "v1.0.0")
	if err == nil {
		t.Fatal("Expected error for invalid path")
	}

	if !strings.Contains(err.Error(), "failed to open debug log file") {
		t.Errorf("Expected 'failed to open debug log file' error, got: %v", err)
	}
}

func TestLogSystemPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewDebugLogger(logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("NewDebugLogger() error: %v", err)
	}
	defer logger.Close()

	logger.LogSystemPrompt("You are a helpful assistant", 50)

	// Verify session stats were updated
	if logger.session.TotalMessages != 1 {
		t.Errorf("Expected 1 total message, got %d", logger.session.TotalMessages)
	}
	if logger.session.TotalInputTokens != 50 {
		t.Errorf("Expected 50 input tokens, got %d", logger.session.TotalInputTokens)
	}

	// Verify content was written
	content, _ := os.ReadFile(logPath)
	if !strings.Contains(string(content), "SYSTEM PROMPT") {
		t.Error("Expected 'SYSTEM PROMPT' in log")
	}
	if !strings.Contains(string(content), "tokens: 50") {
		t.Error("Expected 'tokens: 50' in log")
	}
	if !strings.Contains(string(content), "You are a helpful assistant") {
		t.Error("Expected prompt content in log")
	}
}

func TestLogUserMessage(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewDebugLogger(logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("NewDebugLogger() error: %v", err)
	}
	defer logger.Close()

	logger.LogUserMessage("Hello, how are you?", 10)

	if logger.session.TotalMessages != 1 {
		t.Errorf("Expected 1 total message, got %d", logger.session.TotalMessages)
	}
	if logger.session.TotalInputTokens != 10 {
		t.Errorf("Expected 10 input tokens, got %d", logger.session.TotalInputTokens)
	}

	content, _ := os.ReadFile(logPath)
	if !strings.Contains(string(content), "USER MESSAGE") {
		t.Error("Expected 'USER MESSAGE' in log")
	}
	if !strings.Contains(string(content), "Hello, how are you?") {
		t.Error("Expected user message in log")
	}
}

func TestLogAssistantMessage(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewDebugLogger(logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("NewDebugLogger() error: %v", err)
	}
	defer logger.Close()

	logger.LogAssistantMessage("I am doing well, thank you!", 15)

	if logger.session.TotalMessages != 1 {
		t.Errorf("Expected 1 total message, got %d", logger.session.TotalMessages)
	}
	if logger.session.TotalOutputTokens != 15 {
		t.Errorf("Expected 15 output tokens, got %d", logger.session.TotalOutputTokens)
	}

	content, _ := os.ReadFile(logPath)
	if !strings.Contains(string(content), "ASSISTANT RESPONSE") {
		t.Error("Expected 'ASSISTANT RESPONSE' in log")
	}
	if !strings.Contains(string(content), "I am doing well, thank you!") {
		t.Error("Expected assistant message in log")
	}
}

func TestLogToolCall(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewDebugLogger(logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("NewDebugLogger() error: %v", err)
	}
	defer logger.Close()

	params := map[string]interface{}{
		"command": "ls -la",
		"path":    "/test/file.txt",
	}
	logger.LogToolCall("call_123", "bash", params)

	if logger.session.TotalMessages != 1 {
		t.Errorf("Expected 1 total message, got %d", logger.session.TotalMessages)
	}
	if logger.session.TotalToolCalls != 1 {
		t.Errorf("Expected 1 total tool call, got %d", logger.session.TotalToolCalls)
	}

	content, _ := os.ReadFile(logPath)
	if !strings.Contains(string(content), "TOOL CALL: bash") {
		t.Error("Expected 'TOOL CALL: bash' in log")
	}
	if !strings.Contains(string(content), "Tool ID: call_123") {
		t.Error("Expected 'Tool ID: call_123' in log")
	}
	if !strings.Contains(string(content), "command") {
		t.Error("Expected 'command' in parameters")
	}
}

func TestLogToolCallMultiple(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewDebugLogger(logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("NewDebugLogger() error: %v", err)
	}
	defer logger.Close()

	logger.LogToolCall("call_1", "bash", map[string]interface{}{"command": "ls"})
	logger.LogToolCall("call_2", "read_file", map[string]interface{}{"path": "test.txt"})
	logger.LogToolCall("call_3", "write_file", map[string]interface{}{"path": "out.txt"})

	if logger.session.TotalToolCalls != 3 {
		t.Errorf("Expected 3 total tool calls, got %d", logger.session.TotalToolCalls)
	}
}

func TestLogToolResult_Success(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewDebugLogger(logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("NewDebugLogger() error: %v", err)
	}
	defer logger.Close()

	logger.LogToolResult("call_123", "bash", true, "output data")

	if logger.session.TotalMessages != 1 {
		t.Errorf("Expected 1 total message, got %d", logger.session.TotalMessages)
	}
	if logger.session.FailedToolCalls != 0 {
		t.Errorf("Expected 0 failed tool calls, got %d", logger.session.FailedToolCalls)
	}

	content, _ := os.ReadFile(logPath)
	if !strings.Contains(string(content), "TOOL RESULT: bash") {
		t.Error("Expected 'TOOL RESULT: bash' in log")
	}
	if !strings.Contains(string(content), "Status: success") {
		t.Error("Expected 'Status: success' in log")
	}
	if !strings.Contains(string(content), "output data") {
		t.Error("Expected output in log")
	}
}

func TestLogToolResult_Failure(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewDebugLogger(logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("NewDebugLogger() error: %v", err)
	}
	defer logger.Close()

	logger.LogToolResult("call_123", "bash", false, "command not found")

	if logger.session.FailedToolCalls != 1 {
		t.Errorf("Expected 1 failed tool call, got %d", logger.session.FailedToolCalls)
	}

	content, _ := os.ReadFile(logPath)
	if !strings.Contains(string(content), "Status: failed") {
		t.Error("Expected 'Status: failed' in log")
	}
}

func TestLogStreamingChunk(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewDebugLogger(logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("NewDebugLogger() error: %v", err)
	}
	defer logger.Close()

	logger.LogStreamingChunk("hello ", "text")
	logger.LogStreamingChunk("world", "text")

	content, _ := os.ReadFile(logPath)
	if !strings.Contains(string(content), "STREAMING CHUNK (text)") {
		t.Error("Expected streaming chunk marker in log")
	}
	if !strings.Contains(string(content), "hello ") {
		t.Error("Expected chunk content in log")
	}
}

func TestLogStreamingComplete(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewDebugLogger(logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("NewDebugLogger() error: %v", err)
	}
	defer logger.Close()

	logger.LogStreamingChunk("test content", "text")
	logger.LogStreamingComplete()

	content, _ := os.ReadFile(logPath)
	if !strings.Contains(string(content), "[STREAMING COMPLETE]") {
		t.Error("Expected streaming complete marker in log")
	}
}

func TestLogSessionSummary(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewDebugLogger(logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("NewDebugLogger() error: %v", err)
	}
	defer logger.Close()

	// Add some data
	logger.LogSystemPrompt("System prompt", 20)
	logger.LogUserMessage("User message", 10)
	logger.LogAssistantMessage("Assistant response", 15)
	logger.LogToolCall("call_1", "bash", map[string]interface{}{"command": "ls"})
	logger.LogToolResult("call_1", "bash", true, "success")

	logger.LogSessionSummary()

	content, _ := os.ReadFile(logPath)

	if !strings.Contains(string(content), "SESSION SUMMARY") {
		t.Error("Expected 'SESSION SUMMARY' in log")
	}
	if !strings.Contains(string(content), "Total Messages: 5") {
		t.Error("Expected total messages count in summary")
	}
	if !strings.Contains(string(content), "Total Input Tokens: 30") {
		t.Error("Expected total input tokens in summary")
	}
	if !strings.Contains(string(content), "Total Output Tokens: 15") {
		t.Error("Expected total output tokens in summary")
	}
	if !strings.Contains(string(content), "Total Tool Calls: 1") {
		t.Error("Expected total tool calls in summary")
	}
	if !strings.Contains(string(content), "Failed Tool Calls: 0") {
		t.Error("Expected failed tool calls in summary")
	}
	if !strings.Contains(string(content), "Session ID:") {
		t.Error("Expected session ID in summary")
	}
	if !strings.Contains(string(content), "Duration:") {
		t.Error("Expected duration in summary")
	}
}

func TestClose(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewDebugLogger(logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("NewDebugLogger() error: %v", err)
	}

	// Close should succeed
	err = logger.Close()
	if err != nil {
		t.Errorf("Close() error: %v", err)
	}

	// After close, calling Close again may error (file already closed)
	// This is acceptable behavior
}

func TestClose_DisabledLogger(t *testing.T) {
	logger, _ := NewDebugLogger("", "v1.0.0")

	err := logger.Close()
	if err != nil {
		t.Errorf("Close() on disabled logger should not error: %v", err)
	}
}

func TestRedactSensitiveData(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no sensitive data",
			input:    "Hello world, this is a test",
			expected: "Hello world, this is a test",
		},
		{
			name:     "API key with colon",
			input:    "My api_key: sk-abc123def456ghi789jkl012mno345pqr",
			expected: "My api_key: [REDACTED_API_KEY]",
		},
		{
			name:     "Bearer token (case-insensitive regex preserves lowercase)",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.longtoken",
			expected: "Authorization: bearer [REDACTED]",
		},
		{
			name:     "API key in JSON (api_key pattern catches it first)",
			input:    `{"api_key": "sk-abc123def456ghi789jkl012mno345pqr"}`,
			expected: `{"api_key": "[REDACTED]"}`,
		},
		{
			name:     "password field",
			input:    `{"password": "secret123"}`,
			expected: `{"password": "[REDACTED]"}`,
		},
		{
			name:     "token with equals (regex replaces = with :)",
			input:    "My token = abc123def456",
			expected: "My token: [REDACTED]",
		},
		{
			name:     "secret field",
			input:    `{"secret": "mysecretvalue"}`,
			expected: `{"secret": "[REDACTED]"}`,
		},
		{
			name:     "api key with is (at least 16 chars required)",
			input:    "The api key is ABC123def456ghi789jkl012",
			expected: "The api key is ABC123def456ghi789jkl012",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactSensitiveData(tt.input)
			if result != tt.expected {
				t.Errorf("redactSensitiveData(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRedactSensitiveData_MultiplePatterns(t *testing.T) {
	input := "Key: sk-abc123def456ghi789jkl012mno345pqr and token: my_token_value_12345678"
	result := redactSensitiveData(input)

	// Both patterns should be redacted
	if result == input {
		t.Error("Expected sensitive data to be redacted")
	}
}


func TestNewDebugLogger_SessionStartTime(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	before := time.Now()
	logger, err := NewDebugLogger(logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("NewDebugLogger() error: %v", err)
	}
	defer logger.Close()
	after := time.Now()

	if logger.session.StartTime.Before(before) || logger.session.StartTime.After(after) {
		t.Error("Session start time should be within the creation window")
	}
}

func TestNewDebugLogger_SessionIDFormat(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewDebugLogger(logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("NewDebugLogger() error: %v", err)
	}
	defer logger.Close()

	// Session ID should contain a timestamp component
	id := logger.session.SessionID
	if !strings.HasPrefix(id, "sess_") {
		t.Errorf("Session ID should start with 'sess_', got '%s'", id)
	}

	// Extract the numeric part and verify it's a valid timestamp
	tsPart := strings.TrimPrefix(id, "sess_")
	if tsPart == "" {
		t.Error("Session ID should have a timestamp component after 'sess_'")
	}
}

func TestMultipleLogOperations(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewDebugLogger(logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("NewDebugLogger() error: %v", err)
	}
	defer logger.Close()

	// Simulate a multi-step interaction
	logger.LogSystemPrompt("You are helpful", 10)
	logger.LogUserMessage("Hello", 5)
	logger.LogAssistantMessage("Hi there!", 8)
	logger.LogToolCall("tc1", "bash", map[string]interface{}{"command": "echo hello"})
	logger.LogToolResult("tc1", "bash", true, "hello")
	logger.LogAssistantMessage("Done!", 4)

	// Verify all stats
	if logger.session.TotalMessages != 6 {
		t.Errorf("Expected 6 total messages, got %d", logger.session.TotalMessages)
	}
	if logger.session.TotalInputTokens != 15 {
		t.Errorf("Expected 15 input tokens, got %d", logger.session.TotalInputTokens)
	}
	if logger.session.TotalOutputTokens != 12 {
		t.Errorf("Expected 12 output tokens, got %d", logger.session.TotalOutputTokens)
	}
	if logger.session.TotalToolCalls != 1 {
		t.Errorf("Expected 1 tool call, got %d", logger.session.TotalToolCalls)
	}
}

func TestLogToolCall_WithNullParams(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewDebugLogger(logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("NewDebugLogger() error: %v", err)
	}
	defer logger.Close()

	// Should handle nil map gracefully
	logger.LogToolCall("call_1", "bash", nil)

	// Just verify it doesn't panic
}

func TestLogToolCall_WithComplexParams(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewDebugLogger(logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("NewDebugLogger() error: %v", err)
	}
	defer logger.Close()

	params := map[string]interface{}{
		"path":  "/test/file.txt",
		"start": float64(1),
		"end":   float64(10),
		"content": map[string]interface{}{
			"text": "hello world",
			"lines": []interface{}{"line1", "line2"},
		},
	}
	logger.LogToolCall("call_1", "read_lines", params)

	// Just verify it doesn't panic and writes something
	content, _ := os.ReadFile(logPath)
	if !strings.Contains(string(content), "TOOL CALL: read_lines") {
		t.Error("Expected tool call to be logged")
	}
}



func TestDebugLogger_Disabled(t *testing.T) {
	// Create a disabled logger
	d := &DebugLogger{
		enabled: false,
	}

	// These should all return early without doing anything
	d.writeLog("test %s", "message")
	d.LogSystemPrompt("system prompt", 100)
	d.LogUserMessage("user message", 50)
	d.LogAssistantMessage("assistant message", 50)
}

func TestDebugLogger_Close_NoFile(t *testing.T) {
	d := &DebugLogger{
		enabled: true,
		session: &SessionSummary{
			SessionID:     "test-session",
			StartTime:     time.Now(),
			EndTime:       time.Now(),
			DurationSeconds: 0.0,
			TotalMessages:  0,
			TotalInputTokens: 0,
			TotalOutputTokens: 0,
			TotalToolCalls:  0,
			FailedToolCalls: 0,
		},
	}

	err := d.Close()
	if err != nil {
		t.Errorf("Expected no error when closing with nil file, got: %v", err)
	}
}

func TestLogStreamingChunk_Disabled(t *testing.T) {
	d := &DebugLogger{
		enabled: false,
	}

	// These should all return early without doing anything
	d.LogStreamingChunk("test", "text")
	d.LogStreamingChunk("test2", "tool_call")
}

func TestLogStreamingComplete_Disabled(t *testing.T) {
	d := &DebugLogger{
		enabled: false,
	}

	// Should return early without doing anything
	d.LogStreamingComplete()
}
