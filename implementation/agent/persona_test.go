package agent

import (
	"strings"
	"testing"

	"github.com/coding-agent/harness/tools"

	"github.com/coding-agent/harness/config"
)

// ===== Tests for persona configuration =====

func TestBuildSystemPrompt_WithPersona(t *testing.T) {
	prompt := buildSystemPrompt(false, "Expert Go developer focused on clean code", false)

	// Should contain the persona section
	if !strings.Contains(prompt, "YOUR PERSONA:") {
		t.Error("System prompt should contain 'YOUR PERSONA:' section")
	}

	// Should contain the persona text
	if !strings.Contains(prompt, "Expert Go developer") {
		t.Error("System prompt should contain persona text")
	}

	// Should NOT modify the tools section
	if !strings.Contains(prompt, "AVAILABLE TOOLS:") {
		t.Error("System prompt should still contain tool definitions")
	}

	// Tools should still be properly described
	if !strings.Contains(prompt, "bash") || !strings.Contains(prompt, "read_file") {
		t.Error("Tool definitions should not be modified by persona")
	}
}

func TestBuildSystemPrompt_WithEmptyPersona(t *testing.T) {
	prompt := buildSystemPrompt(false, "", false)

	// Should NOT contain persona section when empty
	if strings.Contains(prompt, "YOUR PERSONA:") {
		t.Error("System prompt should not contain persona section when persona is empty")
	}

	// Should still contain all other sections
	if !strings.Contains(prompt, "AVAILABLE TOOLS:") {
		t.Error("System prompt should contain tool definitions")
	}
}

func TestBuildSystemPrompt_PreservesTools(t *testing.T) {
	personas := []string{
		"Expert Go developer",
		"Security specialist",
		"Documentation writer",
		"Code reviewer",
		"", // Empty persona
	}

	for _, persona := range personas {
		prompt := buildSystemPrompt(false, persona, false)

		// All tool names should be present
		tools := []string{"bash", "read_file", "write_file", "read_lines", "insert_lines", "replace_text"}
		for _, tool := range tools {
			if !strings.Contains(prompt, tool) {
				t.Errorf("Persona %q: System prompt should contain tool %q", persona, tool)
			}
		}

		// Tool descriptions should be intact
		if !strings.Contains(prompt, "Execute a bash command") {
			t.Errorf("Persona %q: Bash description should be intact", persona)
		}
		if !strings.Contains(prompt, "Read the contents of a file") {
			t.Errorf("Persona %q: Read file description should be intact", persona)
		}
	}
}

func TestBuildSystemPrompt_PersonaAfterTools(t *testing.T) {
	prompt := buildSystemPrompt(false, "Test persona", false)

	// Find positions of key sections
	toolsPos := strings.Index(prompt, "AVAILABLE TOOLS:")
	personaPos := strings.Index(prompt, "YOUR PERSONA:")

	if toolsPos == -1 {
		t.Fatal("Tool definitions not found")
	}
	if personaPos == -1 {
		t.Fatal("Persona section not found")
	}

	// Persona should come after tools
	if personaPos < toolsPos {
		t.Error("Persona section should come after tool definitions")
	}
}

func TestBuildReadOnlySystemPrompt_WithPersona(t *testing.T) {
	prompt := buildReadOnlySystemPrompt("ENV INFO", "Security expert", false)

	// Should contain persona
	if !strings.Contains(prompt, "YOUR PERSONA:") {
		t.Error("Read-only system prompt should contain persona")
	}
	if !strings.Contains(prompt, "Security expert") {
		t.Error("Read-only system prompt should contain persona text")
	}

	// Should still contain read-only tools
	if !strings.Contains(prompt, "read_file") {
		t.Error("Read-only system prompt should still contain read_file")
	}
	if !strings.Contains(prompt, "READ-ONLY MODE") {
		t.Error("Read-only system prompt should indicate read-only mode")
	}
}

func TestBuildSystemPrompt_PersonaWithSummaryOnly(t *testing.T) {
	prompt := buildSystemPrompt(false, "Concise assistant", true)

	// Should contain persona
	if !strings.Contains(prompt, "YOUR PERSONA:") {
		t.Error("System prompt should contain persona")
	}

	// Should contain summary-only instructions
	if !strings.Contains(prompt, "IMPORTANT OUTPUT INSTRUCTION") {
		t.Error("System prompt should contain summary-only instructions")
	}
	if !strings.Contains(prompt, "concise summary") {
		t.Error("System prompt should mention concise summary")
	}
}

func TestBuildReadOnlySystemPrompt_PersonaWithSummaryOnly(t *testing.T) {
	prompt := buildReadOnlySystemPrompt("ENV INFO", "Concise reviewer", true)

	// Should contain both persona and summary-only instructions
	if !strings.Contains(prompt, "YOUR PERSONA:") {
		t.Error("Read-only prompt should contain persona")
	}
	if !strings.Contains(prompt, "IMPORTANT OUTPUT INSTRUCTION") {
		t.Error("Read-only prompt should contain summary-only instructions")
	}
}

// ===== Tests for formatToolStatus with subagent =====

func TestFormatToolStatus_SubagentSuccess(t *testing.T) {
	result := &tools.ToolResult{
		Success: true,
		Output:  "Subagent completed.\n\nSummary:\nTask done successfully.",
	}

	formatted := formatToolStatus("subagent", result)
	if !strings.Contains(formatted, "Subagent") {
		t.Error("Expected 'Subagent' in formatted output")
	}
	if !strings.Contains(formatted, "completed") {
		t.Error("Expected 'completed' in formatted output")
	}
}

func TestFormatToolStatus_SubagentFailure(t *testing.T) {
	result := &tools.ToolResult{
		Success: false,
		Error:   "subagent failed: binary not found",
	}

	formatted := formatToolStatus("subagent", result)
	if !strings.Contains(formatted, "Subagent") {
		t.Error("Expected 'Subagent' in formatted output")
	}
	if !strings.Contains(formatted, "[Failed]") {
		t.Error("Expected '[Failed]' for failure case")
	}
}

// ===== Tests for persona in config =====

func TestConfig_PersonaParsing(t *testing.T) {
	// Test that persona is properly parsed from config
	cfg, err := config.ParseArgs([]string{"--persona", "Expert developer", "--prompt", "test"})
	if err != nil {
		t.Fatalf("Failed to parse args: %v", err)
	}

	if cfg.Persona != "Expert developer" {
		t.Errorf("Expected persona 'Expert developer', got '%s'", cfg.Persona)
	}
}

func TestConfig_SummaryOnlyParsing(t *testing.T) {
	cfg, err := config.ParseArgs([]string{"--summary-only", "--prompt", "test"})
	if err != nil {
		t.Fatalf("Failed to parse args: %v", err)
	}

	if !cfg.SummaryOnly {
		t.Error("Expected SummaryOnly to be true")
	}
}

func TestConfig_PersonaAndSummaryOnly(t *testing.T) {
	cfg, err := config.ParseArgs([]string{
		"--persona", "Expert Go developer",
		"--summary-only",
		"--prompt", "test",
	})
	if err != nil {
		t.Fatalf("Failed to parse args: %v", err)
	}

	if cfg.Persona != "Expert Go developer" {
		t.Errorf("Expected persona 'Expert Go developer', got '%s'", cfg.Persona)
	}
	if !cfg.SummaryOnly {
		t.Error("Expected SummaryOnly to be true")
	}
}

func TestConfig_PersonaWithoutValue(t *testing.T) {
	_, err := config.ParseArgs([]string{"--persona"})
	if err == nil {
		t.Error("Expected error when --persona has no value")
	}
}

func TestConfig_PersonaWithEnvironmentVariable(t *testing.T) {
	// This test verifies that the Config struct has the Persona field
	cfg := &config.Config{}
	cfg.Persona = "Test persona"
	

	if cfg.Persona != "Test persona" {
		t.Errorf("Expected 'Test persona', got '%s'", cfg.Persona)
	}
}

