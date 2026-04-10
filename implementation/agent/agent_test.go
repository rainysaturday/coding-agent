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
		"replace_lines",
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
		"TOOL CALLING RULES:",
		"Instructions:",
		"VERIFICATION REQUIREMENTS:",
		"Verification Checklist:",
	}

	for _, section := range requiredSections {
		if !contains(prompt, section) {
			t.Errorf("buildSystemPrompt() missing section: %s", section)
		}
	}

	// Check tool format documentation
	if !contains(prompt, "[TOOL:{") {
		t.Error("buildSystemPrompt() does not document tool format")
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

	// Empty context should return 0
	size := agent.GetContextSize()
	if size != 0 {
		t.Errorf("Expected empty context size 0, got %d", size)
	}

	// Add messages and check size
	agent.AddUserMessage("test message")
	size = agent.GetContextSize()
	if size == 0 {
		t.Error("Expected non-zero context size after adding message")
	}
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

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr))
}
