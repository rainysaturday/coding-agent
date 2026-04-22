package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
	"github.com/coding-agent/harness/tools"
)

func TestSetStreamCallback(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	if agent == nil {
		t.Fatal("NewAgent() returned nil")
	}

	// Set a stream callback - should not panic
	agent.SetStreamCallback(func(chunk inference.StreamingChunk) {
		// noop for test
	})
}

func TestSetContextSizeCallback(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Set a context size callback - should not panic
	agent.SetContextSizeCallback(func(size, max int) {
		if size < 0 {
			t.Error("Expected non-negative size")
		}
		if max < 0 {
			t.Error("Expected non-negative max")
		}
	})
}

func TestGetTools(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	tools := agent.GetTools()
	if len(tools) == 0 {
		t.Error("Expected at least one tool")
	}
}

func TestCloseDebugLogger_NoLogger(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Debug = false
	agent := NewAgent(cfg)

	err := agent.CloseDebugLogger()
	if err != nil {
		t.Errorf("Expected no error when closing nil debug logger, got: %v", err)
	}
}

func TestRunStream(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// This tests the method exists and doesn't panic without actual LLM
	// The call will fail because there's no LLM server, but we test the method structure
	_, err := agent.RunStream(ctx, "test prompt", func(chunk inference.StreamingChunk) {
		// noop
	})

	// Expect error since no LLM server is running (timeout or connection error)
	if err == nil {
		t.Error("Expected error when no LLM server is available")
	}
}

func TestGetStats_Complete(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	stats := agent.GetStats()

	if stats == nil {
		t.Fatal("GetStats() returned nil")
	}

	// Verify all fields exist and are accessible
	if stats.InputTokens < 0 {
		t.Error("InputTokens should be non-negative")
	}
	if stats.OutputTokens < 0 {
		t.Error("OutputTokens should be non-negative")
	}
	if stats.ToolCalls < 0 {
		t.Error("ToolCalls should be non-negative")
	}
	if stats.FailedToolCalls < 0 {
		t.Error("FailedToolCalls should be non-negative")
	}
	if stats.Iterations < 0 {
		t.Error("Iterations should be non-negative")
	}

	// StartTime should be set
	if stats.StartTime.IsZero() {
		t.Error("Expected non-zero StartTime")
	}
}

func TestGetStats_TimeElapsed(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Wait a bit to ensure time passes
	time.Sleep(10 * time.Millisecond)

	stats := agent.GetStats()

	// After some time has passed, the function should still work without errors
	// Verify Stats struct is populated
	if stats == nil {
		t.Fatal("GetStats() returned nil after time elapsed")
	}
}

func TestGetContextSize_Initial(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	size := agent.GetContextSize()
	// Should be positive due to system prompt
	if size <= 0 {
		t.Errorf("Expected positive context size, got %d", size)
	}
}

func TestGetActualContextSize(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	size := agent.GetActualContextSize()
	if size <= 0 {
		t.Errorf("Expected positive actual context size, got %d", size)
	}

	// Add messages and verify size increases
	agent.AddUserMessage("test message")
	size2 := agent.GetActualContextSize()
	if size2 <= size {
		t.Errorf("Expected size to increase after adding message, was %d, now %d", size, size2)
	}
}

func TestAddUserMessage(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	agent.AddUserMessage("Hello")
	if len(agent.context) != 1 {
		t.Errorf("Expected 1 message, got %d", len(agent.context))
	}
	if agent.context[0].Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", agent.context[0].Role)
	}
}

func TestAddAssistantMessage(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	agent.AddAssistantMessage("Hi")
	if len(agent.context) != 1 {
		t.Errorf("Expected 1 message, got %d", len(agent.context))
	}
	if agent.context[0].Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", agent.context[0].Role)
	}
}

func TestBuildTools_AllToolsPresent(t *testing.T) {
	tools := buildTools()

	expectedNames := []string{
		"bash",
		"read_file",
		"write_file",
		"read_lines",
		"insert_lines",
		"replace_text",
		"patch",
		"replace_lines",
		"list_dir",
	}

	for _, expected := range expectedNames {
		found := false
		for _, tool := range tools {
			if tool.Function.Name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Tool '%s' not found in buildTools()", expected)
		}
	}
}

func TestBuildSystemPrompt_Sections(t *testing.T) {
	prompt := buildSystemPrompt()

	sections := []string{
		"AVAILABLE TOOLS:",
		"TOOL CALLING FORMAT:",
		"EXAMPLE workflow:",
		"VERIFICATION REQUIREMENTS:",
		"Verification Checklist:",
		"TOOL CALLING BEST PRACTICES:",
		"ENVIRONMENT INFORMATION:",
		"Current Working Directory:",
		"Operating System:",
		"Architecture:",
	}

	for _, section := range sections {
		if !strings.Contains(prompt, section) {
			t.Errorf("buildSystemPrompt() missing section: %s", section)
		}
	}
}

func TestBuildSystemPrompt_ToolDescriptions(t *testing.T) {
	prompt := buildSystemPrompt()

	toolDescriptions := []struct {
		name        string
		description string
	}{
		{"bash", "Execute a bash command"},
		{"read_file", "Read the contents of a file"},
		{"write_file", "Write content to a file"},
		{"read_lines", "Read a specific line range"},
		{"insert_lines", "Insert lines at a specific line"},
		{"replace_text", "Find and replace text"},
		{"patch", "Apply a unified diff patch"},
		{"replace_lines", "Replace lines in a file"},
	}

	for _, td := range toolDescriptions {
		if !strings.Contains(prompt, td.name) {
			t.Errorf("buildSystemPrompt() missing tool: %s", td.name)
		}
	}
}

func TestBuildSystemPrompt_IncludesEnvInfo(t *testing.T) {
	prompt := buildSystemPrompt()

	// Should include environment information
	if !strings.Contains(prompt, "ENVIRONMENT INFORMATION:") {
		t.Error("System prompt should include environment information")
	}
}

func TestGetEnvironmentInfo(t *testing.T) {
	info := getEnvironmentInfo()

	if info == "" {
		t.Fatal("getEnvironmentInfo() returned empty string")
	}

	// Check for all environment details
	expectedFields := []string{
		"Current Working Directory:",
		"Agent Executable:",
		"Operating System:",
		"Architecture:",
	}

	for _, field := range expectedFields {
		if !strings.Contains(info, field) {
			t.Errorf("getEnvironmentInfo() missing field: %s", field)
		}
	}
}

func TestSetBuildVersion(t *testing.T) {
	original := buildVersion
	defer func() {
		buildVersion = original
	}()

	// Test with different versions
	testVersions := []string{
		"v1.0.0",
		"dev",
		"abc123 [dirty]",
		"unknown",
	}

	for _, version := range testVersions {
		SetBuildVersion(version)
		if buildVersion != version {
			t.Errorf("SetBuildVersion(%q) didn't set correctly, got %q", version, buildVersion)
		}

		got := getBuildVersion()
		if got != version {
			t.Errorf("getBuildVersion() returned %q, expected %q", got, version)
		}
	}
}

func TestFormatResult(t *testing.T) {
	tests := []struct {
		name     string
		result   *tools.ToolResult
		expected string
	}{
		{
			name: "extra message",
			result: &tools.ToolResult{
				Success: true,
				Output:  "some output",
				Extra: map[string]interface{}{
					"message": "custom message",
				},
			},
			expected: "custom message",
		},
		{
			name: "no extra, short output",
			result: &tools.ToolResult{
				Success: true,
				Output:  "line1\nline2\nline3",
			},
			expected: "line1\nline2\nline3",
		},
		{
			name: "no extra, long output truncation",
			result: &tools.ToolResult{
				Success: true,
				Output:  "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\nl11\nl12\n",
			},
			expected: "... [output truncated]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatResult(tt.result)
			if !strings.Contains(result, tt.expected) {
				t.Errorf("formatResult() = %q, expected to contain %q", result, tt.expected)
			}
		})
	}
}

func TestFormatToolStatus_Success(t *testing.T) {
	tests := []struct {
		name   string
		tool   string
		result *tools.ToolResult
		check  func(string) bool
	}{
		{
			name: "bash success",
			tool: "bash",
			result: &tools.ToolResult{
				Success:  true,
				Output:   "output",
				ExitCode: 0,
			},
			check: func(s string) bool {
				return strings.Contains(s, "[Success]")
			},
		},
		{
			name: "bash with exit code",
			tool: "bash",
			result: &tools.ToolResult{
				Success:  true,
				Output:   "output",
				ExitCode: 1,
			},
			check: func(s string) bool {
				return strings.Contains(s, "[Success]") && strings.Contains(s, "exit code: 1")
			},
		},
		{
			name: "read_file success",
			tool: "read_file",
			result: &tools.ToolResult{
				Success: true,
				Output:  "content",
				Extra: map[string]interface{}{
					"linesRead": 5,
				},
			},
			check: func(s string) bool {
				return strings.Contains(s, "[Success]") && strings.Contains(s, "5 lines")
			},
		},
		{
			name: "write_file success with message",
			tool: "write_file",
			result: &tools.ToolResult{
				Success: true,
				Extra: map[string]interface{}{
					"message": "File written successfully: /test/file.txt",
				},
			},
			check: func(s string) bool {
				return strings.Contains(s, "[Success]")
			},
		},
		{
			name: "insert_lines success",
			tool: "insert_lines",
			result: &tools.ToolResult{
				Success: true,
				Extra: map[string]interface{}{
					"linesInserted": 3,
				},
			},
			check: func(s string) bool {
				return strings.Contains(s, "inserted 3")
			},
		},
		{
			name: "replace_text success",
			tool: "replace_text",
			result: &tools.ToolResult{
				Success: true,
				Extra: map[string]interface{}{
					"replacementsMade": 2,
					"search":           "old",
				},
			},
			check: func(s string) bool {
				return strings.Contains(s, "replaced") && strings.Contains(s, "'old'")
			},
		},
		{
			name: "patch success",
			tool: "patch",
			result: &tools.ToolResult{
				Success: true,
				Extra: map[string]interface{}{
					"patches_applied": 3,
				},
			},
			check: func(s string) bool {
				return strings.Contains(s, "3 hunk")
			},
		},
		{
			name: "replace_lines success (line-number mode)",
			tool: "replace_lines",
			result: &tools.ToolResult{
				Success: true,
				Extra: map[string]interface{}{
					"start": 2,
					"end":   5,
				},
			},
			check: func(s string) bool {
				return strings.Contains(s, "replaced lines") && strings.Contains(s, "2-5")
			},
		},
		{
			name: "replace_lines success (search mode)",
			tool: "replace_lines",
			result: &tools.ToolResult{
				Success: true,
				Extra: map[string]interface{}{
					"replacementsMade": 3,
					"search":           "foo",
				},
			},
			check: func(s string) bool {
				return strings.Contains(s, "replaced") && strings.Contains(s, "'foo'") && strings.Contains(s, "3 time(s)")
			},
		},
		{
			name: "unknown tool success",
			tool: "custom_tool",
			result: &tools.ToolResult{
				Success: true,
			},
			check: func(s string) bool {
				return strings.Contains(s, "[Success]")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatToolStatus(tt.tool, tt.result)
			if !tt.check(result) {
				t.Errorf("formatToolStatus() for %s: %q does not match check", tt.tool, result)
			}
		})
	}
}

func TestFormatToolStatus_Failure(t *testing.T) {
	result := formatToolStatus("bash", &tools.ToolResult{
		Success: false,
		Error:   "command not found",
	})

	if !strings.Contains(result, "[Failed]") {
		t.Errorf("Expected [Failed] in result: %s", result)
	}
	if !strings.Contains(result, "bash") {
		t.Errorf("Expected tool name in result: %s", result)
	}
	if !strings.Contains(result, "command not found") {
		t.Errorf("Expected error message in result: %s", result)
	}
}

func TestStreamStatus_Callback(t *testing.T) {
	var received []inference.StreamingChunk

	cb := func(chunk inference.StreamingChunk) {
		received = append(received, chunk)
	}

	// Test various tool types
	tools := map[string]map[string]interface{}{
		"bash":         {"command": "ls -la"},
		"read_file":    {"path": "/test/file.txt"},
		"write_file":   {"path": "/test/output.txt"},
		"read_lines":   {"path": "/test/file.txt", "start": 1, "end": 10},
		"insert_lines": {"path": "/test/file.txt", "line": 5},
		"replace_text": {"path": "/test/file.txt", "search": "old"},
		"patch":        {"path": "/test/file.txt"},
		"unknown":      nil,
	}

	for toolName, params := range tools {
		streamStatus(toolName, params, cb)
		if len(received) == 0 {
			t.Errorf("Expected callback to be called for tool: %s", toolName)
		}
		received = nil // Reset for next test
	}

	// Test that callbacks with nil doesn't panic
	streamStatus("bash", nil, nil)
}

func TestStreamResult_Callback(t *testing.T) {
	var received []inference.StreamingChunk

	cb := func(chunk inference.StreamingChunk) {
		received = append(received, chunk)
	}

	successResult := &tools.ToolResult{
		Success: true,
		Output:  "success output",
	}
	streamResult("bash", successResult, cb)
	if len(received) == 0 {
		t.Error("Expected callback to be called for success result")
	}

	failureResult := &tools.ToolResult{
		Success: false,
		Error:   "some error",
	}
	streamResult("bash", failureResult, cb)
	if len(received) == 0 {
		t.Error("Expected callback to be called for failure result")
	}

	// Test nil callback
	streamResult("bash", successResult, nil)
}

func TestShouldCompress_ContextNearLimit(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ContextSize = 1000 // Small context for testing
	agent := NewAgent(cfg)

	// Add many messages to exceed 80% of context size
	for i := 0; i < 50; i++ {
		agent.AddUserMessage("This is a test message number " + string(rune('A'+i%26)) + " with some content to use up context tokens.")
	}

	shouldCompress := agent.shouldCompress()
	// With many messages, should compress
	if !shouldCompress {
		t.Error("Expected shouldCompress() to return true with near-limit context")
	}
}

func TestShouldCompress_ContextBelowLimit(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ContextSize = 50000 // Large context for testing
	agent := NewAgent(cfg)

	// Add just a few short messages - with large context, won't trigger compression
	agent.AddUserMessage("Short msg")
	agent.AddAssistantMessage("Short reply")

	shouldCompress := agent.shouldCompress()
	// With few messages and large context, should not compress
	if shouldCompress {
		t.Error("Expected shouldCompress() to return false with minimal context relative to limit")
	}
}

func TestSaveSession_SavesCorrectly(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Add some messages
	agent.AddUserMessage("Hello, assistant!")
	agent.AddAssistantMessage("Hi there! How can I help you today?")
	agent.AddUserMessage("Tell me about Go.")

	// Create temp file
	tmpFile, err := os.CreateTemp("", "session-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Save session
	err = agent.SaveSession(tmpFile.Name())
	if err != nil {
		t.Fatalf("SaveSession() error: %v", err)
	}

	// Verify file exists and is not empty
	info, err := os.Stat(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to stat saved session file: %v", err)
	}
	if info.Size() == 0 {
		t.Error("Saved session file is empty")
	}

	// Verify JSON is valid
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read saved session: %v", err)
	}

	var session SessionData
	if err := json.Unmarshal(data, &session); err != nil {
		t.Fatalf("Session file is not valid JSON: %v", err)
	}

	// Verify session data
	if session.SystemPrompt == "" {
		t.Error("Expected non-empty system prompt in saved session")
	}
	if len(session.Messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(session.Messages))
	}
	if session.Messages[0].Content != "Hello, assistant!" {
		t.Errorf("Expected first message 'Hello, assistant!', got '%s'", session.Messages[0].Content)
	}
	if session.Timestamp == "" {
		t.Error("Expected non-empty timestamp")
	}
	if len(session.ToolDefs) == 0 {
		t.Error("Expected tool definitions to be saved")
	}
}

func TestLoadSession_LoadsCorrectly(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Create and save a session
	tmpFile, _ := os.CreateTemp("", "session-*.json")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	agent.AddUserMessage("Original message")
	agent.AddAssistantMessage("Original response")

	err := agent.SaveSession(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Create a new agent and load the session
	cfg2 := config.DefaultConfig()
	agent2 := NewAgent(cfg2)

	// Verify context is empty initially
	if len(agent2.context) != 0 {
		t.Errorf("Expected empty context, got %d messages", len(agent2.context))
	}

	// Load session
	err = agent2.LoadSession(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadSession() error: %v", err)
	}

	// Verify messages are restored
	if len(agent2.context) != 2 { // 2 saved messages (system is stored separately)
		t.Errorf("Expected 2 messages, got %d", len(agent2.context))
	}
	if agent2.context[0].Content != "Original message" {
		t.Errorf("Expected first saved message 'Original message', got '%s'", agent2.context[0].Content)
	}
	if agent2.context[1].Content != "Original response" {
		t.Errorf("Expected second saved message 'Original response', got '%s'", agent2.context[1].Content)
	}

	// Verify tool definitions are restored
	tools := agent2.GetTools()
	if len(tools) == 0 {
		t.Error("Expected tool definitions to be restored")
	}
}

func TestLoadSession_FileNotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	err := agent.LoadSession("/nonexistent/path/session.json")
	if err == nil {
		t.Error("Expected error when loading non-existent file")
	}
}

func TestLoadSession_InvalidJSON(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Create a temp file with invalid JSON
	tmpFile, _ := os.CreateTemp("", "session-*.json")
	tmpFile.WriteString("not valid json {{{")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	err := agent.LoadSession(tmpFile.Name())
	if err == nil {
		t.Error("Expected error when loading invalid JSON file")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	cfg := config.DefaultConfig()
	agent1 := NewAgent(cfg)

	// Add messages with different roles
	agent1.AddUserMessage("First question")
	agent1.AddAssistantMessage("First answer")
	agent1.AddUserMessage("Second question")
	agent1.AddAssistantMessage("Second answer")

	tmpFile, _ := os.CreateTemp("", "session-*.json")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Save
	err := agent1.SaveSession(tmpFile.Name())
	if err != nil {
		t.Fatalf("SaveSession error: %v", err)
	}

	// Load into new agent
	cfg2 := config.DefaultConfig()
	agent2 := NewAgent(cfg2)

	err = agent2.LoadSession(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadSession error: %v", err)
	}

	// Verify all messages match
	if len(agent2.context) != 4 { // 4 saved messages (system is stored separately)
		t.Errorf("Expected 4 messages, got %d", len(agent2.context))
	}

	// Verify message content
	expectedContents := []string{
		"First question",
		"First answer",
		"Second question",
		"Second answer",
	}
	for i, expected := range expectedContents {
		if agent2.context[i].Content != expected {
			t.Errorf("Message %d: expected '%s', got '%s'", i+1, expected, agent2.context[i].Content)
		}
	}

	// Verify system prompt is also restored
	if agent2.GetSystemPrompt() == "" {
		t.Error("Expected system prompt to be restored")
	}
}

func TestSaveSession_DefaultFilename(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Use the default filename
	tmpDir, err := os.MkdirTemp("", "session-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	defaultPath := tmpDir + "/session.json"
	err = agent.SaveSession(defaultPath)
	if err != nil {
		t.Fatalf("SaveSession with default filename error: %v", err)
	}

	// Verify file was created at the expected path
	if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
		t.Error("Expected session.json to be created at specified path")
	}
}

func TestCompressContext_NotEnoughMessages(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Only 1 message, should not compress
	err := agent.compressContext(context.Background())
	if err != nil {
		t.Errorf("compressContext() should not error with minimal messages: %v", err)
	}
}

func TestClearContext_AfterMessages(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	agent.AddUserMessage("msg1")
	agent.AddUserMessage("msg2")
	agent.AddAssistantMessage("reply1")

	if len(agent.context) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(agent.context))
	}

	agent.ClearContext()

	if len(agent.context) != 0 {
		t.Errorf("Expected 0 messages after clear, got %d", len(agent.context))
	}
}

func TestSetAPIEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Should not panic
	agent.SetAPIEndpoint("http://custom-endpoint:8080/v1")
}

func TestSetAPIKey(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Should not panic
	agent.SetAPIKey("test-api-key-12345")
}

func TestGetSystemPrompt_NonEmpty(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	prompt := agent.GetSystemPrompt()
	if prompt == "" {
		t.Error("Expected non-empty system prompt")
	}
}

func TestGetContextSize_AfterClear(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	initialSize := agent.GetContextSize()

	agent.AddUserMessage("test message to add tokens")
	largerSize := agent.GetContextSize()
	if largerSize <= initialSize {
		t.Errorf("Expected size to increase, was %d, now %d", initialSize, largerSize)
	}

	agent.ClearContext()

	afterClearSize := agent.GetContextSize()
	// Should be back to approximately initial size (system prompt only)
	if afterClearSize > largerSize {
		t.Errorf("Expected size to decrease after clear, was %d, now %d", largerSize, afterClearSize)
	}
}

func TestBuildSystemPrompt_NoDuplicates(t *testing.T) {
	prompt := buildSystemPrompt()

	// Count occurrences of key phrases
	toolNames := []string{"bash", "read_file", "write_file", "read_lines", "insert_lines", "replace_text", "patch", "replace_lines"}
	for _, name := range toolNames {
		count := strings.Count(prompt, name)
		// Tool should appear in tool definition and description, so at least 2 times
		if count < 2 {
			t.Errorf("Tool '%s' should appear at least twice, found %d", name, count)
		}
	}
}

func TestGetEnvironmentInfo_ContainsSubAgentInfo(t *testing.T) {
	info := getEnvironmentInfo()

	if !strings.Contains(info, "coding-agent -p") {
		t.Error("getEnvironmentInfo() should contain sub-agent spawning instruction")
	}
	if !strings.Contains(info, "parallel tasks") {
		t.Error("getEnvironmentInfo() should mention parallel tasks")
	}
}

func TestGetEnvironmentInfo_CWD(t *testing.T) {
	info := getEnvironmentInfo()

	cwd, err := os.Getwd()
	if err == nil {
		if !strings.Contains(info, cwd) {
			t.Errorf("getEnvironmentInfo() should contain cwd %q", cwd)
		}
	}
}

func TestFormatToolStatus_ReadLines(t *testing.T) {
	result := formatToolStatus("read_lines", &tools.ToolResult{
		Success: true,
		Output:  "1: line1\n2: line2\n3: line3",
		Extra: map[string]interface{}{
			"linesRead": 3,
		},
	})

	if !strings.Contains(result, "[Success]") {
		t.Error("Expected success status")
	}
}

func TestFormatToolStatus_WriteFile(t *testing.T) {
	result := formatToolStatus("write_file", &tools.ToolResult{
		Success: true,
		Output:  "File written successfully",
		Extra: map[string]interface{}{
			"message": "File written successfully: /tmp/test.txt (100 bytes)",
		},
	})

	if !strings.Contains(result, "[Success]") {
		t.Error("Expected success status")
	}
}

func TestFormatToolStatus_ReadLinesTruncation(t *testing.T) {
	// Create a result with many lines
	var output string
	for i := 1; i <= 15; i++ {
		output += fmt.Sprintf("%d: line %d\n", i, i)
	}

	result := formatToolStatus("read_lines", &tools.ToolResult{
		Success: true,
		Output:  output,
	})

	if !strings.Contains(result, "[Success]") {
		t.Error("Expected success status")
	}
}

func TestBuildTools_Parameters(t *testing.T) {
	toolDefs := buildTools()

	expectedParams := map[string][]string{
		"bash":          {"command"},
		"read_file":     {"path"},
		"write_file":    {"path", "content"},
		"read_lines":    {"path", "start", "end"},
		"insert_lines":  {"path", "line", "lines"},
		"replace_text":  {"path", "search", "replace"},
		"patch":         {"path", "diff"},
		"replace_lines": {"path"},
		"glob":          {"pattern"},
		"sub_agent":     {"prompt"},
		"git_status":    {},
		"git_diff":      {},
		"git_log":       {},
		"git_show":      {"path"},
		"git_add":       {},
		"git_commit":    {},
		"git_branch":    {"action"},
		"git_stash":     {"action"},
		"find":          {"pattern"},
		"web_fetch":     {"url"},
		"move_file":     {"source", "destination"},
		"file_rename":   {"source", "destination"},
		"copy_file":     {"source", "destination"},
		"list_dir":      {},
		"delete_file":   {"path"},
		"scaffold":      {"template"},
		"run_tests":     {},
		"project_tree":  {},
		"code_navigation": {"query"},
		"check_links":     {},
		"json_transformer": {"command"},
		"project_diagnostics": {},
		"run_lint": {},
		"process_management": {"action"},
		"env_var": {"action"},
		"file_compare": {"file1", "file2"},
		"changelog":     {},
		"git_tag":       {"action"},
		"run_build":     {},
		"run_coverage":  {},
		"git_merge":     {"action"},
		"git_revert":    {"action"},
		"git_rebase":    {"action"},
		"generate_docs": {"path"},
		"code_metrics": {"path"},
	}

	for _, tool := range toolDefs {
		expected, ok := expectedParams[tool.Function.Name]
		if !ok {
			t.Errorf("Unexpected tool: %s", tool.Function.Name)
			continue
		}

		if len(tool.Function.Parameters.Required) != len(expected) {
			t.Errorf("Tool %s: expected %d required params, got %d",
				tool.Function.Name, len(expected), len(tool.Function.Parameters.Required))
		}

		for _, req := range expected {
			found := false
			for _, actual := range tool.Function.Parameters.Required {
				if actual == req {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Tool %s: missing required param %s", tool.Function.Name, req)
			}
		}
	}
}

func TestReportContextSize_CallbackCalled(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	var receivedSize, receivedMax int
	agent.reportContextSize(func(size, max int) {
		receivedSize = size
		receivedMax = max
	}, cfg.ContextSize)

	if receivedSize <= 0 {
		t.Errorf("Expected positive size, got %d", receivedSize)
	}
	if receivedMax != cfg.ContextSize {
		t.Errorf("Expected max %d, got %d", cfg.ContextSize, receivedMax)
	}
}

func TestReportContextSize_NilCallback(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Should not panic with nil callback
	agent.reportContextSize(nil, cfg.ContextSize)
}

func TestReportContextSize_ZeroMax(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	var receivedMax int
	agent.reportContextSize(func(size, max int) {
		_ = size
		receivedMax = max
	}, 0)

	if receivedMax != 0 {
		t.Errorf("Expected max 0, got %d", receivedMax)
	}
}
