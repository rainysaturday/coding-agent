package agent

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/coding-agent/harness/config"
)

func TestNewAgent(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	if agent == nil {
		t.Fatal("NewAgent() returned nil")
	}

	if agent.config != cfg {
		t.Error("NewAgent() did not store config")
	}

	if agent.systemPrompt == "" {
		t.Error("NewAgent() system prompt is empty")
	}

	if agent.stats == nil {
		t.Error("NewAgent() stats is nil")
	}
}

func TestGetEnvironmentInfo(t *testing.T) {
	envInfo := getEnvironmentInfo()

	// Check that envInfo is not empty
	if envInfo == "" {
		t.Fatal("getEnvironmentInfo() returned empty string")
	}

	// Check for required sections
	if !strings.Contains(envInfo, "ENVIRONMENT INFORMATION:") {
		t.Error("getEnvironmentInfo() missing 'ENVIRONMENT INFORMATION:' header")
	}

	// Get actual values for comparison
	cwd, _ := os.Getwd()
	exePath, _ := os.Executable()
	osInfo := runtime.GOOS
	archInfo := runtime.GOARCH

	// Check for environment fields
	if !strings.Contains(envInfo, "Current Working Directory:") {
		t.Error("getEnvironmentInfo() missing 'Current Working Directory:' field")
	}
	if !strings.Contains(envInfo, "Agent Executable:") {
		t.Error("getEnvironmentInfo() missing 'Agent Executable:' field")
	}
	if !strings.Contains(envInfo, "Operating System:") {
		t.Error("getEnvironmentInfo() missing 'Operating System:' field")
	}
	if !strings.Contains(envInfo, "Architecture:") {
		t.Error("getEnvironmentInfo() missing 'Architecture:' field")
	}

	// Check that actual values are included
	if !strings.Contains(envInfo, cwd) {
		t.Errorf("getEnvironmentInfo() does not contain actual cwd: %s", cwd)
	}
	if !strings.Contains(envInfo, exePath) {
		t.Errorf("getEnvironmentInfo() does not contain actual exePath: %s", exePath)
	}
	if !strings.Contains(envInfo, osInfo) {
		t.Errorf("getEnvironmentInfo() does not contain actual OS: %s", osInfo)
	}
	if !strings.Contains(envInfo, archInfo) {
		t.Errorf("getEnvironmentInfo() does not contain actual arch: %s", archInfo)
	}

	// Check for sub-agent spawning instruction
	if !strings.Contains(envInfo, "coding-agent -p") {
		t.Error("getEnvironmentInfo() missing sub-agent spawning instruction")
	}
}

func TestBuildSystemPromptContainsEnvironmentInfo(t *testing.T) {
	prompt := buildSystemPrompt()

	// Check that system prompt includes environment information
	if !strings.Contains(prompt, "ENVIRONMENT INFORMATION:") {
		t.Error("buildSystemPrompt() does not include environment information")
	}

	// Get actual values for comparison
	cwd, _ := os.Getwd()
	exePath, _ := os.Executable()
	osInfo := runtime.GOOS
	archInfo := runtime.GOARCH

	// Verify environment values are in the prompt
	if !strings.Contains(prompt, cwd) {
		t.Errorf("buildSystemPrompt() does not contain actual cwd: %s", cwd)
	}
	if !strings.Contains(prompt, exePath) {
		t.Errorf("buildSystemPrompt() does not contain actual exePath: %s", exePath)
	}
	if !strings.Contains(prompt, osInfo) {
		t.Errorf("buildSystemPrompt() does not contain actual OS: %s", osInfo)
	}
	if !strings.Contains(prompt, archInfo) {
		t.Errorf("buildSystemPrompt() does not contain actual arch: %s", archInfo)
	}
}

func TestGetSystemPrompt(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	prompt := agent.GetSystemPrompt()

	if prompt == "" {
		t.Error("GetSystemPrompt() returned empty string")
	}

	// Check that prompt contains tool definitions
	expectedTools := []string{
		"bash",
		"read_file",
		"write_file",
		"read_lines",
		"insert_lines",
		"replace_text",
		"patch",
	}

	for _, tool := range expectedTools {
		if !contains(prompt, tool) {
			t.Errorf("GetSystemPrompt() does not contain tool: %s", tool)
		}
	}

	// Check for verification requirements
	if !contains(prompt, "VERIFICATION REQUIREMENTS") {
		t.Error("GetSystemPrompt() does not contain verification requirements")
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	prompt := buildSystemPrompt()

	// Check for required sections
	requiredSections := []string{
		"AVAILABLE TOOLS:",
		"TOOL CALLING FORMAT:",
		"VERIFICATION REQUIREMENTS:",
		"Verification Checklist:",
	}

	for _, section := range requiredSections {
		if !strings.Contains(prompt, section) {
			t.Errorf("buildSystemPrompt() missing section: %s", section)
		}
	}

	// Check all tools are documented in the system prompt
	tools := buildTools()
	for _, tool := range tools {
		if !strings.Contains(prompt, tool.Function.Name) {
			t.Errorf("buildSystemPrompt() missing documented tool: %s", tool.Function.Name)
		}
	}
}

func TestStats(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	stats := agent.GetStats()

	if stats == nil {
		t.Fatal("GetStats() returned nil")
	}

	if stats.InputTokens != 0 {
		t.Errorf("Expected initial input tokens 0, got %d", stats.InputTokens)
	}
	if stats.OutputTokens != 0 {
		t.Errorf("Expected initial output tokens 0, got %d", stats.OutputTokens)
	}
	if stats.ToolCalls != 0 {
		t.Errorf("Expected initial tool calls 0, got %d", stats.ToolCalls)
	}
	if stats.FailedToolCalls != 0 {
		t.Errorf("Expected initial failed tool calls 0, got %d", stats.FailedToolCalls)
	}
}

func TestClearContext(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Add some messages
	agent.AddUserMessage("test message 1")
	agent.AddAssistantMessage("test response 1")
	agent.AddUserMessage("test message 2")

	// Verify context has messages
	if len(agent.context) != 3 {
		t.Errorf("Expected 3 messages in context, got %d", len(agent.context))
	}

	// Clear context
	agent.ClearContext()

	// Verify context is empty
	if len(agent.context) != 0 {
		t.Errorf("Expected 0 messages after clear, got %d", len(agent.context))
	}
}

func TestAddUserMessage(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	message := "test user message"
	agent.AddUserMessage(message)

	if len(agent.context) != 1 {
		t.Errorf("Expected 1 message, got %d", len(agent.context))
	}

	if agent.context[0].Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", agent.context[0].Role)
	}

	if agent.context[0].Content != message {
		t.Errorf("Expected content '%s', got '%s'", message, agent.context[0].Content)
	}
}

func TestAddAssistantMessage(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	message := "test assistant message"
	agent.AddAssistantMessage(message)

	if len(agent.context) != 1 {
		t.Errorf("Expected 1 message, got %d", len(agent.context))
	}

	if agent.context[0].Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", agent.context[0].Role)
	}

	if agent.context[0].Content != message {
		t.Errorf("Expected content '%s', got '%s'", message, agent.context[0].Content)
	}
}

func TestGetContextSize(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Before any API call, context size is the system prompt estimate (non-zero)
	size := agent.GetContextSize()
	if size <= 0 {
		t.Errorf("Expected positive context size (system prompt), got %d", size)
	}

	// Simulate adding a user message and verify context size increases
	agent.AddUserMessage("test message")
	size2 := agent.GetContextSize()
	if size2 <= size {
		t.Errorf("Expected context size to increase after adding message, was %d, now %d", size, size2)
	}
	// Context size should be system prompt + "test message" (no stats manipulation needed)
	_ = size2
}

func TestSetAPIEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	endpoint := "http://test-endpoint:8080"
	agent.SetAPIEndpoint(endpoint)

	// Verify endpoint was set (this tests the inference client was properly initialized)
	// The actual endpoint is in the inference client which we can't easily access
	// but we can verify the method exists and doesn't panic
}

func TestSetAPIKey(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	key := "test-api-key"
	agent.SetAPIKey(key)

	// Verify method exists and doesn't panic
}

func TestBuildTools(t *testing.T) {
	tools := buildTools()

	if len(tools) != 7 {
		t.Errorf("Expected 7 tools, got %d", len(tools))
	}

	expectedTools := map[string]bool{
		"bash":         false,
		"read_file":    false,
		"write_file":   false,
		"read_lines":   false,
		"insert_lines": false,
		"replace_text": false,
		"patch":        false,
	}

	for _, tool := range tools {
		if _, exists := expectedTools[tool.Function.Name]; exists {
			expectedTools[tool.Function.Name] = true
		} else {
			t.Errorf("Unexpected tool: %s", tool.Function.Name)
		}
		if tool.Type != "function" {
			t.Errorf("Tool %s: expected type 'function', got '%s'", tool.Function.Name, tool.Type)
		}
		if tool.Function.Parameters.Type != "object" {
			t.Errorf("Tool %s: expected parameters type 'object', got '%s'", tool.Function.Name, tool.Function.Parameters.Type)
		}
		if len(tool.Function.Parameters.Properties) == 0 {
			t.Errorf("Tool %s: has no parameters defined", tool.Function.Name)
		}
		if len(tool.Function.Parameters.Required) == 0 {
			t.Errorf("Tool %s: has no required parameters", tool.Function.Name)
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("Tool %s not found in buildTools()", name)
		}
	}
}

func TestToolDefinitionsMatchSystemPrompt(t *testing.T) {
	// Verify that every tool registered with the API is also documented in the system prompt
	tools := buildTools()
	systemPrompt := buildSystemPrompt()

	for _, tool := range tools {
		if !strings.Contains(systemPrompt, tool.Function.Name) {
			t.Errorf("Tool '%s' is registered with the API but not documented in system prompt", tool.Function.Name)
		}
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
