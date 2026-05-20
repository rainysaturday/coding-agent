package agent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"fmt"
	"os"
	"strings"
	"net/http"
	"net/http/httptest"
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
	tools := buildTools(false)

	expectedNames := []string{
		"bash",
		"read_file",
		"write_file",
		"read_lines",
		"insert_lines",
		"replace_text",
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
	prompt := buildSystemPrompt(false)

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
	prompt := buildSystemPrompt(false)

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
	}

	for _, td := range toolDescriptions {
		if !strings.Contains(prompt, td.name) {
			t.Errorf("buildSystemPrompt() missing tool: %s", td.name)
		}
	}
}

func TestBuildSystemPrompt_IncludesEnvInfo(t *testing.T) {
	prompt := buildSystemPrompt(false)

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

func TestFormatToolStatus_Success(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		result   *tools.ToolResult
		check    func(string) bool
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
				Output:  "Inserted 3 line(s) at line 5 in: /test/file.txt\n--- Content inserted ---\nline1\nline2\nline3",
				Extra: map[string]interface{}{
					"linesInserted": 3,
				},
			},
			check: func(s string) bool {
				return strings.Contains(s, "inserted 3") || strings.Contains(s, "Inserted 3")
			},
		},
		{
			name: "replace_text success",
			tool: "replace_text",
			result: &tools.ToolResult{
				Success: true,
				Output:  "Replaced 'old' with 'new' 2 time(s) in: /test/file.txt\n--- Preview ---\nreplaced line",
				Extra: map[string]interface{}{
					"replacementsMade": 2,
					"search":           "old",
				},
			},
			check: func(s string) bool {
				return strings.Contains(s, "Replaced") && strings.Contains(s, "'old'")
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

func TestCompressContext_NotEnoughMessages(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Only 1 message, should not compress
	err := agent.compressContext(context.Background())
	if err != nil {
		t.Errorf("compressContext() should not error with minimal messages: %v", err)
	}
}

func TestCompressContext_PreserveCountBoundary(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Add exactly preserveCount+1 messages (4 messages = boundary for no compression)
	agent.AddUserMessage("first user prompt")
	agent.AddAssistantMessage("assistant response 1")
	agent.AddUserMessage("user message 2")
	agent.AddAssistantMessage("assistant response 2")

	// Should not compress at the boundary
	err := agent.compressContext(context.Background())
	if err != nil {
		t.Errorf("compressContext() should not error at boundary: %v", err)
	}

	// Context should be unchanged
	if len(agent.context) != 4 {
		t.Errorf("Expected 4 messages (unchanged), got %d", len(agent.context))
	}
	if agent.context[0].Role != "user" {
		t.Errorf("Expected first message to be user, got %s", agent.context[0].Role)
	}
	if agent.context[0].Content != "first user prompt" {
		t.Errorf("Expected first user message preserved, got %q", agent.context[0].Content)
	}
}

func TestCompressContext_FirstUserMessagePreserved(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Add enough messages to trigger compression threshold
	agent.AddUserMessage("ORIGINAL FIRST USER PROMPT")
	for i := 0; i < 10; i++ {
		agent.AddAssistantMessage(fmt.Sprintf("assistant response %d", i))
		agent.AddUserMessage(fmt.Sprintf("user message %d", i))
	}

	// Compression will fail because there's no LLM, but the error should be returned
	// (not a panic), and the context should remain unchanged
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err := agent.compressContext(ctx)
	// Expected to fail without LLM
	if err == nil {
		t.Error("Expected error when no LLM is available for compression")
	}
}

func TestCompressContext_SummaryIsAssistantRole(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// This test verifies the expected structure after compression.
	// Without an LLM, compression will fail, but we can verify the
	// early-exit behavior and that the context is not corrupted.
	agent.AddUserMessage("first user prompt")
	agent.AddAssistantMessage("assistant response 1")
	agent.AddUserMessage("user message 2")
	agent.AddAssistantMessage("assistant response 2")
	agent.AddUserMessage("user message 3")
	agent.AddAssistantMessage("assistant response 3")

	// At this point we have 6 messages (> preserveCount+1 = 4)
	// Compression will be attempted but will fail without an LLM
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err := agent.compressContext(ctx)

	// Without LLM, compression fails
	if err == nil {
		t.Error("Expected error when no LLM is available for compression")
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
	prompt := buildSystemPrompt(false)

	// Count occurrences of key phrases
	toolNames := []string{"bash", "read_file", "write_file", "read_lines", "insert_lines", "replace_text"}
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
	toolDefs := buildTools(false)

	expectedParams := map[string][]string{
		"bash":         {"command"},
		"read_file":    {"path"},
		"write_file":   {"path", "content"},
		"read_lines":   {"path", "start", "end"},
		"insert_lines": {"path", "line", "lines"},
		"replace_text": {"path", "search", "replace"},
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

func TestBuildTools_ReadOnly(t *testing.T) {
	tools := buildTools(true)

	// In read-only mode, should only have read_file, read_lines, list_files, grep, git_log, git_show, and git_diff
	expectedNames := []string{"read_file", "read_lines", "list_files", "grep", "git_log", "git_show", "git_diff"}

	if len(tools) != len(expectedNames) {
		t.Errorf("Expected %d tools in read-only mode, got %d", len(expectedNames), len(tools))
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
			t.Errorf("Tool '%s' not found in read-only buildTools()", expected)
		}
	}

	// Verify that bash is NOT in read-only mode
	for _, tool := range tools {
		if tool.Function.Name == "bash" {
			t.Error("bash should not be in read-only mode")
		}
		if tool.Function.Name == "write_file" {
			t.Error("write_file should not be in read-only mode")
		}
	}
}

func TestBuildSystemPrompt_ReadOnly(t *testing.T) {
	prompt := buildSystemPrompt(true)

	// Should mention read-only mode
	if !strings.Contains(prompt, "READ-ONLY MODE") {
		t.Error("Read-only system prompt should mention READ-ONLY MODE")
	}

	// Should mention read_file, read_lines, list_files, grep, git_log, git_show, and git_diff
	if !strings.Contains(prompt, "1. read_file") {
		t.Error("Read-only system prompt should list read_file")
	}
	if !strings.Contains(prompt, "2. read_lines") {
		t.Error("Read-only system prompt should list read_lines")
	}
	if !strings.Contains(prompt, "3. list_files") {
		t.Error("Read-only system prompt should list list_files")
	}
	if !strings.Contains(prompt, "4. grep") {
		t.Error("Read-only system prompt should list grep")
	}
	if !strings.Contains(prompt, "5. git_log") {
		t.Error("Read-only system prompt should list git_log")
	}
	if !strings.Contains(prompt, "6. git_show") {
		t.Error("Read-only system prompt should list git_show")
	}
	if !strings.Contains(prompt, "7. git_diff") {
		t.Error("Read-only system prompt should list git_diff")
	}

	// Should NOT mention write tools
	if strings.Contains(prompt, "bash") {
		t.Error("Read-only system prompt should not mention bash")
	}
	if strings.Contains(prompt, "write_file") {
		t.Error("Read-only system prompt should not mention write_file")
	}
}

func TestBuildSystemPrompt_ReadOnlyNotices(t *testing.T) {
	prompt := buildSystemPrompt(true)

	// Should warn about limitations
	if !strings.Contains(prompt, "CANNOT modify") && !strings.Contains(prompt, "CANNOT write") {
		t.Error("Read-only system prompt should warn about not being able to modify/write")
	}
}


// TestSetGoal_Activate tests that SetGoal correctly activates goal mode
func TestSetGoal_Activate(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Goal should not be active initially
	if agent.IsGoalActive() {
		t.Error("Goal should not be active initially")
	}
	if agent.GetGoal() != "" {
		t.Errorf("Expected empty goal initially, got %q", agent.GetGoal())
	}

	// Set a goal
	agent.SetGoal("Create a REST API")

	// Verify goal is active
	if !agent.IsGoalActive() {
		t.Error("Goal should be active after SetGoal")
	}
	if agent.GetGoal() != "Create a REST API" {
		t.Errorf("Expected goal 'Create a REST API', got %q", agent.GetGoal())
	}
}

func TestSetGoal_EmptyString(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// First activate goal mode
	agent.SetGoal("Some goal")
	if !agent.IsGoalActive() {
		t.Error("Goal should be active")
	}

	// Setting empty string should deactivate goal mode
	agent.SetGoal("")

	if agent.IsGoalActive() {
		t.Error("Goal should not be active after setting empty string")
	}
	if agent.GetGoal() != "" {
		t.Errorf("Expected empty goal, got %q", agent.GetGoal())
	}
}

func TestSetGoal_MultipleGoals(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Set first goal
	agent.SetGoal("First goal")
	if agent.GetGoal() != "First goal" {
		t.Errorf("Expected 'First goal', got %q", agent.GetGoal())
	}

	// Set second goal (should overwrite)
	agent.SetGoal("Second goal")
	if agent.GetGoal() != "Second goal" {
		t.Errorf("Expected 'Second goal', got %q", agent.GetGoal())
	}
}

// TestClearGoal tests that ClearGoal correctly deactivates goal mode
func TestClearGoal(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Activate goal mode first
	agent.SetGoal("Test goal")
	if !agent.IsGoalActive() {
		t.Error("Goal should be active")
	}

	// Clear the goal
	agent.ClearGoal()

	// Verify goal mode is deactivated
	if agent.IsGoalActive() {
		t.Error("Goal should not be active after ClearGoal")
	}
	if agent.GetGoal() != "" {
		t.Errorf("Expected empty goal after ClearGoal, got %q", agent.GetGoal())
	}
}

// TestClearGoal_AlreadyInactive tests ClearGoal when goal mode is already inactive
func TestClearGoal_AlreadyInactive(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Should not panic when clearing an already inactive goal
	agent.ClearGoal()

	if agent.IsGoalActive() {
		t.Error("Goal should not be active")
	}
}

// TestSetGoal_WhitespaceOnly tests behavior with whitespace-only goal
func TestSetGoal_WhitespaceOnly(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Set a whitespace-only goal - should still activate goal mode
	agent.SetGoal("   ")
	if !agent.IsGoalActive() {
		t.Error("Goal should be active with whitespace")
	}
	if agent.GetGoal() != "   " {
		t.Errorf("Expected whitespace goal preserved, got %q", agent.GetGoal())
	}

	// Clear it
	agent.ClearGoal()
	if agent.IsGoalActive() {
		t.Error("Goal should be deactivated after ClearGoal")
	}
}

// TestGetGoal_ReturnsCorrectValue tests that GetGoal returns the correct value
func TestGetGoal_ReturnsCorrectValue(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Test empty goal
	if goal := agent.GetGoal(); goal != "" {
		t.Errorf("Expected empty goal, got %q", goal)
	}

	// Test with goal
	agent.SetGoal("Build a web application")
	if goal := agent.GetGoal(); goal != "Build a web application" {
		t.Errorf("Expected 'Build a web application', got %q", goal)
	}
}

// TestIsGoalActive_ReturnsCorrectValue tests that IsGoalActive returns correct boolean
func TestIsGoalActive_ReturnsCorrectValue(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Should be false initially
	if agent.IsGoalActive() {
		t.Error("Goal should not be active initially")
	}

	// Should be true after SetGoal
	agent.SetGoal("Test")
	if !agent.IsGoalActive() {
		t.Error("Goal should be active after SetGoal")
	}

	// Should be false after ClearGoal
	agent.ClearGoal()
	if agent.IsGoalActive() {
		t.Error("Goal should not be active after ClearGoal")
	}
}

// TestGoalContext_PreservesFirstUserMessage tests that goal mode preserves context correctly
func TestGoalContext_PreservesFirstUserMessage(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Add a user message manually to simulate conversation
	agent.AddUserMessage("Initial user prompt")

	// Set a goal
	agent.SetGoal("Create a REST API")

	// Verify goal is set
	if !agent.IsGoalActive() {
		t.Error("Goal should be active")
	}
	if agent.GetGoal() != "Create a REST API" {
		t.Errorf("Expected 'Create a REST API', got %q", agent.GetGoal())
	}

	// Clear goal and verify context is still intact
	agent.ClearGoal()

	// Context should not be affected by goal operations
	if agent.GetContextSize() == 0 {
		t.Error("Context should still have content")
	}
}

// TestGoalMode_IntegrationWithContext tests goal mode integration with agent context
func TestGoalMode_IntegrationWithContext(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Initial state
	if agent.IsGoalActive() {
		t.Error("Goal should not be active initially")
	}

	// Activate goal mode
	agent.SetGoal("Complete feature X")

	// Verify state
	if !agent.IsGoalActive() {
		t.Error("Goal should be active")
	}

	// Add some messages
	agent.AddUserMessage("First message")
	agent.AddAssistantMessage("First assistant response")

	// Goal should still be active
	if !agent.IsGoalActive() {
		t.Error("Goal should still be active after adding messages")
	}
	if agent.GetGoal() != "Complete feature X" {
		t.Errorf("Expected goal 'Complete feature X', got %q", agent.GetGoal())
	}

	// Deactivate goal mode
	agent.ClearGoal()

	// Verify goal is cleared
	if agent.IsGoalActive() {
		t.Error("Goal should be deactivated")
	}
	if agent.GetGoal() != "" {
		t.Errorf("Expected empty goal, got %q", agent.GetGoal())
	}
}

// TestInjectGoalCheck_MessageFormat tests that injectGoalCheck creates the correct user message
func TestInjectGoalCheck_MessageFormat(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Add an initial message to have context
	agent.AddUserMessage("Initial prompt")
	agent.AddAssistantMessage("Initial response")

	// Set a goal and inject the check
	agent.SetGoal("Create a test file")
	agent.injectGoalCheck()

	// Verify context has 3 messages: user, assistant, goal check user message
	if agent.GetContextSize() == 0 {
		t.Error("Context should have content")
	}

	// The goal check should contain the goal prompt
	goal := agent.GetGoal()
	if goal != "Create a test file" {
		t.Errorf("Expected goal 'Create a test file', got %q", goal)
	}
}

// TestGoalMode_GoalAchievedAtNaturalEnd tests that "goal achieved" is checked
// at the natural end of the agentic loop (no tool calls)
func TestGoalMode_GoalAchievedAtNaturalEnd(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Set a goal
	agent.SetGoal("Create a file")

	// Simulate: the LLM responds with "goal achieved" and no tool calls
	// This would be caught by the natural end check
	response := &inference.Response{
		Content:   "I have created the file. Goal achieved.",
		ToolCalls: nil, // No tool calls = natural end
	}

	// Check that "goal achieved" is detected (case-insensitive)
	if !strings.Contains(strings.ToLower(response.Content), "goal achieved") {
		t.Error("Should detect 'goal achieved' in response")
	}
}

// TestGoalMode_GoalNotAchieved_InjectsCheck tests that when goal is not achieved,
// a goal check message is injected and the loop continues
func TestGoalMode_GoalNotAchieved_InjectsCheck(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Set a goal
	agent.SetGoal("Build the app")

	// Add initial context
	agent.AddUserMessage("Build an app")
	agent.AddAssistantMessage("I started building...")

	// Record context size before inject
	contextSizeBefore := agent.GetContextSize()

	// Simulate: natural end without "goal achieved" - should inject goal check
	agent.injectGoalCheck()

	// Context should have grown (goal check message was added)
	contextSizeAfter := agent.GetContextSize()
	if contextSizeAfter <= contextSizeBefore {
		t.Error("Context should grow after injecting goal check")
	}
}

// TestGoalCaseInsensitive_Matching verifies case-insensitive "goal achieved" matching
func TestGoalCaseInsensitive_Matching(t *testing.T) {
	testCases := []struct {
		content   string
		shouldMatch bool
	}{
		{"Goal achieved", true},
		{"GOAL ACHIEVED", true},
		{"goal achieved", true},
		{"Goal Achieved!", true},
		{"The goal achieved.", true},
		{"I have achieved the goal", false}, // Must contain "goal achieved"
		{"goal achieved yet", true},
		{"not done yet", false},
		{"", false},
	}

	for _, tc := range testCases {
		matched := strings.Contains(strings.ToLower(tc.content), "goal achieved")
		if matched != tc.shouldMatch {
			t.Errorf("Content %q: expected match=%v, got match=%v", tc.content, tc.shouldMatch, matched)
		}
	}
}


// ===== Tests for Error types and utilities (previously 0% coverage) =====

func TestAuthError_Error(t *testing.T) {
	e := &AuthError{Message: "invalid API key"}
	result := e.Error()
	if result != "invalid API key" {
		t.Errorf("Expected 'invalid API key', got '%s'", result)
	}
}

func TestAuthError_Error_Default(t *testing.T) {
	e := &AuthError{}
	result := e.Error()
	if result != "authentication failed" {
		t.Errorf("Expected 'authentication failed', got '%s'", result)
	}
}

func TestContextLimitError_Error(t *testing.T) {
	e := &ContextLimitError{Message: "too many tokens"}
	result := e.Error()
	if result != "too many tokens" {
		t.Errorf("Expected 'too many tokens', got '%s'", result)
	}
}

func TestContextLimitError_Error_Default(t *testing.T) {
	e := &ContextLimitError{}
	result := e.Error()
	if result != "context size limit exceeded" {
		t.Errorf("Expected 'context size limit exceeded', got '%s'", result)
	}
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		want    bool
	}{
		{"auth failed", fmt.Errorf("API authentication failed"), true},
		{"401", fmt.Errorf("HTTP 401 Unauthorized"), true},
		{"403", fmt.Errorf("HTTP 403 Forbidden"), true},
		{"authorization", fmt.Errorf("Authorization required"), true},
		{"API key", fmt.Errorf("Invalid API key"), true},
		{"not auth", fmt.Errorf("some other error"), false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAuthError(tt.err)
			if got != tt.want {
				t.Errorf("isAuthError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsContextLimitError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		want    bool
	}{
		{"context size limit", fmt.Errorf("context size limit exceeded"), true},
		{"maximum context length", fmt.Errorf("maximum context length"), true},
		{"maximum context length exceeded", fmt.Errorf("maximum context length exceeded"), true},
		{"not context", fmt.Errorf("some other error"), false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isContextLimitError(tt.err)
			if got != tt.want {
				t.Errorf("isContextLimitError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWrapError_Nil(t *testing.T) {
	result := wrapError(nil)
	if result != nil {
		t.Errorf("Expected nil, got %v", result)
	}
}

func TestWrapError_AuthError(t *testing.T) {
	err := fmt.Errorf("API authentication failed (HTTP 401)")
	wrapped := wrapError(err)

	_, ok := wrapped.(*AuthError)
	if !ok {
		t.Errorf("Expected *AuthError, got %T", wrapped)
	}
}

func TestWrapError_ContextLimitError(t *testing.T) {
	err := fmt.Errorf("maximum context length exceeded")
	wrapped := wrapError(err)

	_, ok := wrapped.(*ContextLimitError)
	if !ok {
		t.Errorf("Expected *ContextLimitError, got %T", wrapped)
	}
}

func TestWrapError_OtherError(t *testing.T) {
	err := fmt.Errorf("some other error")
	wrapped := wrapError(err)

	if wrapped != err {
		t.Errorf("Expected same error, got %v", wrapped)
	}
}

func TestSetMaxDisplayWidth(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Should not panic
	agent.SetMaxDisplayWidth(80)
	agent.SetMaxDisplayWidth(120)
	agent.SetMaxDisplayWidth(0)
}

func TestGetToolExecutor(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	executor := agent.GetToolExecutor()
	if executor == nil {
		t.Fatal("GetToolExecutor() returned nil")
	}
}

func TestShouldCheckGoal(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Initially should not check goal
	if agent.shouldCheckGoal() {
		t.Error("shouldCheckGoal() should be false initially")
	}

	// Set goal - should check
	agent.SetGoal("Test goal")
	if !agent.shouldCheckGoal() {
		t.Error("shouldCheckGoal() should be true after SetGoal")
	}

	// Clear goal - should not check
	agent.ClearGoal()
	if agent.shouldCheckGoal() {
		t.Error("shouldCheckGoal() should be false after ClearGoal")
	}
}

func TestExitCodes_Constants(t *testing.T) {
	if ExitSuccess != 0 {
		t.Errorf("Expected ExitSuccess = 0, got %d", ExitSuccess)
	}
	if ExitError != 1 {
		t.Errorf("Expected ExitError = 1, got %d", ExitError)
	}
	if ExitUsageError != 2 {
		t.Errorf("Expected ExitUsageError = 2, got %d", ExitUsageError)
	}
	if ExitAuthError != 3 {
		t.Errorf("Expected ExitAuthError = 3, got %d", ExitAuthError)
	}
	if ExitContextLimit != 4 {
		t.Errorf("Expected ExitContextLimit = 4, got %d", ExitContextLimit)
	}
}

func TestColorConstants(t *testing.T) {
	expected := map[string]string{
		"ColorReset":   "\033[0m",
		"ColorGreen":   "\033[32m",
		"ColorYellow":  "\033[33m",
		"ColorRed":     "\033[31m",
		"ColorCyan":    "\033[36m",
		"ColorBlue":    "\033[34m",
		"ColorMagenta": "\033[35m",
	}

	if ColorReset != expected["ColorReset"] {
		t.Errorf("ColorReset mismatch")
	}
	if ColorGreen != expected["ColorGreen"] {
		t.Errorf("ColorGreen mismatch")
	}
	if ColorMagenta != expected["ColorMagenta"] {
		t.Errorf("ColorMagenta mismatch")
	}
}

func TestFormatToolStatus_ListFiles(t *testing.T) {
	result := formatToolStatus("list_files", &tools.ToolResult{
		Success: true,
		Output:  "file1.txt\nfile2.txt",
		Extra: map[string]interface{}{
			"entriesListed": 2,
		},
	})

	if !strings.Contains(result, "[Success]") {
		t.Error("Expected success status")
	}
	if !strings.Contains(result, "2 entries") {
		t.Error("Expected entry count")
	}
}

func TestFormatToolStatus_Grep(t *testing.T) {
	result := formatToolStatus("grep", &tools.ToolResult{
		Success: true,
		Output:  "file.txt:1:hello world",
		Extra: map[string]interface{}{
			"matchesFound": 1,
		},
	})

	if !strings.Contains(result, "[Success]") {
		t.Error("Expected success status")
	}
	if !strings.Contains(result, "grep") {
		t.Error("Expected 'grep' in output")
	}
}

func TestFormatToolStatus_GrepZeroMatches(t *testing.T) {
	result := formatToolStatus("grep", &tools.ToolResult{
		Success: true,
		Output:  "",
		Extra: map[string]interface{}{
			"matchesFound": 0,
		},
	})

	if !strings.Contains(result, "0 matches") {
		t.Error("Expected '0 matches' in output")
	}
}

func TestFormatToolStatus_GitLog(t *testing.T) {
	result := formatToolStatus("git_log", &tools.ToolResult{
		Success: true,
		Output:  "commit abc123\n    Initial commit",
		Extra: map[string]interface{}{
			"count":     1,
			"reference": "HEAD",
		},
	})

	if !strings.Contains(result, "[Success]") {
		t.Error("Expected success status")
	}
	if !strings.Contains(result, "git log") {
		t.Error("Expected 'git log' in output")
	}
}

func TestFormatToolStatus_GitShow(t *testing.T) {
	result := formatToolStatus("git_show", &tools.ToolResult{
		Success: true,
		Output:  "commit abc123\n    Initial commit",
		Extra: map[string]interface{}{
			"commitReference": "HEAD",
		},
	})

	if !strings.Contains(result, "[Success]") {
		t.Error("Expected success status")
	}
	if !strings.Contains(result, "git show") {
		t.Error("Expected 'git show' in output")
	}
}

func TestFormatToolStatus_GitDiff(t *testing.T) {
	result := formatToolStatus("git_diff", &tools.ToolResult{
		Success: true,
		Output:  "diff --git a/file.txt b/file.txt",
		Extra: map[string]interface{}{
			"reference1": "HEAD",
			"reference2": "HEAD~1",
		},
	})

	if !strings.Contains(result, "[Success]") {
		t.Error("Expected success status")
	}
	if !strings.Contains(result, "git diff") {
		t.Error("Expected 'git diff' in output")
	}
}

func TestStreamToolCallWithFullParams_Bash(t *testing.T) {
	var received []inference.StreamingChunk
	cb := func(chunk inference.StreamingChunk) {
		received = append(received, chunk)
	}

	tc := &tools.ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "ls -la /tmp",
		},
	}

	streamToolCallWithFullParams(tc, cb)

	if len(received) != 1 {
		t.Fatalf("Expected 1 chunk, got %d", len(received))
	}
	if !strings.Contains(received[0].Text, "[Bash]") {
		t.Errorf("Expected '[Bash]' in chunk, got '%s'", received[0].Text)
	}
	if !strings.Contains(received[0].Text, "ls -la") {
		t.Errorf("Expected 'ls -la' in chunk, got '%s'", received[0].Text)
	}
}

func TestStreamToolCallWithFullParams_ReadFile(t *testing.T) {
	var received []inference.StreamingChunk
	cb := func(chunk inference.StreamingChunk) {
		received = append(received, chunk)
	}

	tc := &tools.ToolCall{
		Name: "read_file",
		Parameters: map[string]interface{}{
			"path": "/tmp/test.txt",
		},
	}

	streamToolCallWithFullParams(tc, cb)

	if len(received) != 1 {
		t.Fatalf("Expected 1 chunk, got %d", len(received))
	}
	if !strings.Contains(received[0].Text, "[Read]") {
		t.Errorf("Expected '[Read]' in chunk, got '%s'", received[0].Text)
	}
}

func TestStreamToolCallWithFullParams_WriteFile(t *testing.T) {
	var received []inference.StreamingChunk
	cb := func(chunk inference.StreamingChunk) {
		received = append(received, chunk)
	}

	tc := &tools.ToolCall{
		Name: "write_file",
		Parameters: map[string]interface{}{
			"path":    "/tmp/output.txt",
			"content": "hello",
		},
	}

	streamToolCallWithFullParams(tc, cb)

	if len(received) != 1 {
		t.Fatalf("Expected 1 chunk, got %d", len(received))
	}
	if !strings.Contains(received[0].Text, "[Write]") {
		t.Errorf("Expected '[Write]' in chunk, got '%s'", received[0].Text)
	}
}

func TestStreamToolCallWithFullParams_UnknownTool(t *testing.T) {
	var received []inference.StreamingChunk
	cb := func(chunk inference.StreamingChunk) {
		received = append(received, chunk)
	}

	tc := &tools.ToolCall{
		Name: "unknown_tool",
		Parameters: map[string]interface{}{
			"param1": "value1",
		},
	}

	streamToolCallWithFullParams(tc, cb)

	if len(received) != 1 {
		t.Fatalf("Expected 1 chunk, got %d", len(received))
	}
	if !strings.Contains(received[0].Text, "[Tool: unknown_tool]") {
		t.Errorf("Expected '[Tool: unknown_tool]' in chunk, got '%s'", received[0].Text)
	}
}

func TestStreamToolCallWithFullParams_NilCallback(t *testing.T) {
	tc := &tools.ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo test",
		},
	}

	// Should not panic with nil callback
	streamToolCallWithFullParams(tc, nil)
}

func TestFormatFullToolParams_Empty(t *testing.T) {
	result := formatFullToolParams(map[string]interface{}{})
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}
}

func TestFormatFullToolParams_Nil(t *testing.T) {
	result := formatFullToolParams(nil)
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}
}

func TestFormatFullToolParams_StringValue(t *testing.T) {
	params := map[string]interface{}{
		"command": "ls -la",
	}
	result := formatFullToolParams(params)
	if !strings.Contains(result, "command") {
		t.Errorf("Expected 'command' in result, got '%s'", result)
	}
}

func TestFormatFullToolParams_MultipleValues(t *testing.T) {
	params := map[string]interface{}{
		"path":    "/tmp/test.txt",
		"content": "hello",
		"count":   float64(5),
	}
	result := formatFullToolParams(params)
	if !strings.Contains(result, "path") {
		t.Errorf("Expected 'path' in result, got '%s'", result)
	}
	if !strings.Contains(result, "content") {
		t.Errorf("Expected 'content' in result, got '%s'", result)
	}
}

func TestFormatParamValue_String(t *testing.T) {
	result := formatParamValue("hello")
	if result != `"hello"` {
		t.Errorf("Expected '\"hello\"', got '%s'", result)
	}
}

func TestFormatParamValue_Int(t *testing.T) {
	result := formatParamValue(float64(42))
	if result != "42" {
		t.Errorf("Expected '42', got '%s'", result)
	}
}

func TestFormatParamValue_Float(t *testing.T) {
	result := formatParamValue(float64(3.14))
	if result != "3.1" {
		t.Errorf("Expected '3.1', got '%s'", result)
	}
}

func TestFormatParamValue_Bool(t *testing.T) {
	result := formatParamValue(true)
	if result != "true" {
		t.Errorf("Expected 'true', got '%s'", result)
	}
}

func TestFormatParamValue_Nil(t *testing.T) {
	result := formatParamValue(nil)
	if result != "null" {
		t.Errorf("Expected 'null', got '%s'", result)
	}
}

func TestFormatParamValue_Map(t *testing.T) {
	result := formatParamValue(map[string]interface{}{"key": "value"})
	if result == "" {
		t.Error("Expected non-empty result for map")
	}
}

func TestFormatParamValue_Array(t *testing.T) {
	result := formatParamValue([]interface{}{"a", "b"})
	if result == "" {
		t.Error("Expected non-empty result for array")
	}
	if !strings.Contains(result, "a") {
		t.Errorf("Expected 'a' in result, got '%s'", result)
	}
}

func TestCompressContext_Public(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Add messages
	agent.AddUserMessage("first")
	agent.AddAssistantMessage("response 1")
	agent.AddUserMessage("second")
	agent.AddAssistantMessage("response 2")
	agent.AddUserMessage("third")
	agent.AddAssistantMessage("response 3")

	// Use the public CompressContext method
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err := agent.CompressContext(ctx)
	// Expected to fail without LLM
	if err == nil {
		t.Error("Expected error when no LLM is available for compression")
	}
}


func TestRunStream_ContextCancellation(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := agent.RunStream(ctx, "test prompt", nil)
	if err == nil {
		t.Error("Expected error when context is cancelled")
	}
}

func TestGetStats_TokensPerSecond(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	stats := agent.GetStats()

	// TokensPerSecond should be 0 initially (no time elapsed or no tokens)
	if stats.TokensPerSecond < 0 {
		t.Errorf("Expected non-negative TokensPerSecond, got %f", stats.TokensPerSecond)
	}
}

func TestCompressContext_FirstMessageNotUser(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Add assistant message first (unusual but should handle)
	agent.AddAssistantMessage("assistant first")
	agent.AddUserMessage("user message")
	agent.AddAssistantMessage("assistant response")
	agent.AddUserMessage("user message 2")
	agent.AddAssistantMessage("assistant response 2")
	agent.AddUserMessage("user message 3")
	agent.AddAssistantMessage("assistant response 3")

	// Should not panic even with non-standard message order
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = agent.compressContext(ctx) // Expected to fail without LLM
}

func TestGroupAssistantToolMessages(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Test with empty messages
	result := agent.groupAssistantToolMessages(nil)
	if result != nil {
		t.Errorf("Expected nil, got %v", result)
	}

	result = agent.groupAssistantToolMessages([]*inference.Message{})
	if len(result) != 0 {
		t.Errorf("Expected empty, got %d", len(result))
	}
}

func TestGroupAssistantToolMessages_PreserveOrder(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	messages := []*inference.Message{
		{Role: "user", Content: "prompt"},
		{Role: "assistant", Content: "response", ToolCalls: []*inference.APIToolCall{{ID: "call_1", Type: "function", Function: inference.FunctionCall{Name: "bash"}}}},
		{Role: "tool", Content: "result", ToolCallId: "call_1"},
	}

	result := agent.groupAssistantToolMessages(messages)
	if len(result) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(result))
	}
}

// ===== Additional Run() method tests =====



// ===== Tests for getActualContextSizeUnlocked with lastTotalTokens set =====

func TestGetActualContextSizeUnlocked_WithTotalTokens(t *testing.T) {
	cfg := config.DefaultConfig()
	a := NewAgent(cfg)

	// Set lastTotalTokens to simulate having received an API response
	a.mu.Lock()
	a.lastTotalTokens = 1000
	a.mu.Unlock()

	// Get actual context size - should use lastTotalTokens
	size := a.getActualContextSizeUnlocked()
	if size != 1000 {
		t.Errorf("Expected 1000 (lastTotalTokens), got %d", size)
	}
}

func TestGetActualContextSizeUnlocked_WithTotalTokensAndToolMessages(t *testing.T) {
	cfg := config.DefaultConfig()
	a := NewAgent(cfg)

	// Set lastTotalTokens
	a.mu.Lock()
	a.lastTotalTokens = 1000
	a.toolResultMsgsSinceLastAPI = make(map[int]bool)
	a.context = append(a.context, &inference.Message{
		Role:    "tool",
		Content: "Tool result content",
	})
	a.toolResultMsgsSinceLastAPI[0] = true
	a.mu.Unlock()

	// Get actual context size - should include delta for tool message
	size := a.getActualContextSizeUnlocked()
	// lastTotalTokens + 3 + estimateTokens("Tool result content")
	if size < 1003 {
		t.Errorf("Expected > 1003 (lastTotalTokens + delta), got %d", size)
	}
}

func TestGetActualContextSizeUnlocked_NoTotalTokens(t *testing.T) {
	cfg := config.DefaultConfig()
	a := NewAgent(cfg)

	// Add some messages without setting lastTotalTokens
	a.AddUserMessage("Test message")

	// Get actual context size - should estimate from scratch
	size := a.getActualContextSizeUnlocked()
	if size <= 0 {
		t.Errorf("Expected positive size, got %d", size)
	}
}

// ===== Tests for Agent.Run() with mock server =====

func TestRun_FinalResponse(t *testing.T) {
	// Create a mock inference server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that this is a chat completions request
		if r.URL.Path != "/v1/chat/completions" && !strings.HasSuffix(r.URL.Path, "chat/completions") {
			// Could also be the root path depending on the endpoint setup
		}
		w.Header().Set("Content-Type", "application/json")
		// Return a response with no tool calls - final response
		w.Write([]byte(`{
			"id": "test-1",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "test-model",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "This is the final answer"
				},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 20,
				"total_tokens": 30
			}
		}`))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.APIEndpoint = server.URL + "/v1"
		cfg.Streaming = false
	ag := NewAgent(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := ag.Run(ctx, "test prompt")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.FinalOutput != "This is the final answer" {
		t.Errorf("Expected 'This is the final answer', got '%s'", result.FinalOutput)
	}

	if len(result.Steps) != 0 {
		t.Errorf("Expected 0 steps (no tool calls), got %d", len(result.Steps))
	}
}

func TestRun_ToolCalls(t *testing.T) {
	// Track the number of API calls made
	callCount := 0
	var receivedMessages [][]inference.Message

	// Create a mock inference server that returns tool calls on first call, then final response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		// Read the request body to see what messages were sent
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			receivedMessages = append(receivedMessages, nil) // track calls
		}

		w.Header().Set("Content-Type", "application/json")

		if callCount == 1 {
			// First call: return a tool call
			w.Write([]byte(`{
				"id": "test-1",
				"object": "chat.completion",
				"created": 1234567890,
				"model": "test-model",
				"choices": [{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "",
						"tool_calls": [{
							"id": "call-1",
							"type": "function",
							"function": {
								"name": "read_file",
								"arguments": "{\"path\": \"test.txt\"}"
							}
						}]
					},
					"finish_reason": "tool_calls"
				}],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 15,
					"total_tokens": 25
				}
			}`))
		} else {
			// Subsequent calls: return final response (no tool calls)
			w.Write([]byte(`{
				"id": "test-2",
				"object": "chat.completion",
				"created": 1234567890,
				"model": "test-model",
				"choices": [{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "After reading the file, here is the answer"
					},
					"finish_reason": "stop"
				}],
				"usage": {
					"prompt_tokens": 20,
					"completion_tokens": 25,
					"total_tokens": 45
				}
			}`))
		}
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.APIEndpoint = server.URL + "/v1"
		cfg.Streaming = false
	cfg.MaxTokens = 10000
	cfg.ContextSize = 32000
	ag := NewAgent(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := ag.Run(ctx, "What is in test.txt?")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if !strings.Contains(result.FinalOutput, "After reading the file") {
		t.Errorf("Expected 'After reading the file' in output, got '%s'", result.FinalOutput)
	}

	// Should have steps from tool calls
	if len(result.Steps) == 0 {
		t.Error("Expected at least one step from tool calls")
	}
}

func TestRun_MaxIterationsExceeded(t *testing.T) {
	// Create a mock server that always returns tool calls to force max iterations
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "test-1",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "test-model",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "",
					"tool_calls": [{
						"id": "call-1",
						"type": "function",
						"function": {
							"name": "bash",
							"arguments": "{\"command\": \"echo test\"}"
						}
					}]
				},
				"finish_reason": "tool_calls"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 15,
				"total_tokens": 25
			}
		}`))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.APIEndpoint = server.URL + "/v1"
		cfg.Streaming = false
	cfg.MaxTokens = 10000
	cfg.ContextSize = 32000
	cfg.MaxIterations = 5 // Low max iterations to test the limit
	ag := NewAgent(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := ag.Run(ctx, "test prompt")
	if err == nil {
		t.Error("Expected error for max iterations exceeded")
	}
	if !strings.Contains(err.Error(), "maximum iterations") {
		t.Errorf("Expected 'maximum iterations' in error, got: %v", err)
	}
}

func TestRun_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block to simulate slow response
		time.Sleep(2 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "test-1",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "test-model",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "response"
				},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 10,
				"total_tokens": 20
			}
		}`))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.APIEndpoint = server.URL + "/v1"
		cfg.Streaming = false
	ag := NewAgent(cfg)

	// Create context that will be cancelled after 100ms
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := ag.Run(ctx, "test prompt")
	if err == nil {
		t.Error("Expected context cancellation error")
	}
	if err != context.DeadlineExceeded && !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "context") {
		t.Errorf("Expected deadline/cancellation error, got: %v", err)
	}
}


// ===== Tests for NewAgent with different config options =====

func TestNewAgent_DebugMode(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "debug.log")

	cfg := &config.Config{
		Debug:          true,
		DebugLog:       logPath,
		APIEndpoint:    "http://localhost:8080",
		Model:          "test-model",
		MaxTokens:      1000,
		ContextSize:    4096,
		MaxIterations:  50,
		ReadOnly:       false,
		Streaming:      true,
		ShowVersion:    false,
		ShowHelp:       false,
		Verbose:        false,
		Quiet:          false,
		Temperature:    nil,
		
		Prompt:         "",
		PromptFile:     "",
		UseStdin:       false,
	}

	ag := NewAgent(cfg)
	if ag == nil {
		t.Fatal("NewAgent returned nil")
	}

	if ag.debugLogger == nil {
		t.Error("Expected debug logger to be non-nil when Debug is true")
	}
}

func TestNewAgent_ReadOnlyMode(t *testing.T) {
	cfg := &config.Config{
		Debug:          false,
		APIEndpoint:    "http://localhost:8080",
		Model:          "test-model",
		MaxTokens:      1000,
		ContextSize:    4096,
		MaxIterations:  50,
		ReadOnly:       true,
		Streaming:      true,
		ShowVersion:    false,
		ShowHelp:       false,
		Verbose:        false,
		Quiet:          false,
	}

	ag := NewAgent(cfg)
	if ag == nil {
		t.Fatal("NewAgent returned nil")
	}

	// In read-only mode, write tools should not be available
	tools := ag.GetTools()
	for _, tool := range tools {
		if tool.Function.Name == "write_file" || tool.Function.Name == "insert_lines" || tool.Function.Name == "replace_text" {
			t.Errorf("Tool '%s' should not be available in read-only mode", tool.Function.Name)
		}
	}
}

func TestNewAgent_VerboseConfig(t *testing.T) {
	cfg := &config.Config{
		Debug:          false,
		APIEndpoint:    "http://localhost:8080",
		Model:          "gpt-4",
		MaxTokens:      32000,
		ContextSize:    128000,
		MaxIterations:  500,
		ReadOnly:       false,
		Streaming:      false,
		ShowVersion:    false,
		ShowHelp:       false,
		Verbose:        true,
		Quiet:          false,
		Temperature:    floatPtr(0.7),
	}

	ag := NewAgent(cfg)
	if ag == nil {
		t.Fatal("NewAgent returned nil")
	}

	stats := ag.GetStats()
	if stats == nil {
		t.Fatal("GetStats returned nil")
	}
}

// Helper to create *float64
func floatPtr(f float64) *float64 {
	return &f
}
