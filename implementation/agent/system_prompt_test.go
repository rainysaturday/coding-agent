package agent

import (
	"os"
	"strings"
	"testing"

	"github.com/coding-agent/harness/config"
)

func TestBuildSystemPrompt_Sections(t *testing.T) {
	prompt := buildSystemPrompt(false, "", false)

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
	prompt := buildSystemPrompt(false, "", false)

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
	prompt := buildSystemPrompt(false, "", false)

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

func TestGetSystemPrompt_NonEmpty(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	prompt := agent.GetSystemPrompt()
	if prompt == "" {
		t.Error("Expected non-empty system prompt")
	}
}

func TestBuildSystemPrompt_NoDuplicates(t *testing.T) {
	prompt := buildSystemPrompt(false, "", false)

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

func TestBuildSystemPrompt_ContainsSubAgentInfo(t *testing.T) {
	prompt := buildSystemPrompt(false, "", false)

	if !strings.Contains(prompt, "coding-agent") {
		t.Error("buildSystemPrompt() should contain coding-agent reference")
	}
	if !strings.Contains(prompt, "parallel tasks") {
		t.Error("buildSystemPrompt() should mention parallel tasks")
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

func TestBuildSystemPrompt_ReadOnly(t *testing.T) {
	prompt := buildSystemPrompt(true, "", false)

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
	prompt := buildSystemPrompt(true, "", false)

	// Should warn about limitations
	if !strings.Contains(prompt, "CANNOT modify") && !strings.Contains(prompt, "CANNOT write") {
		t.Error("Read-only system prompt should warn about not being able to modify/write")
	}
}

// TestSetGoal_Activate tests that SetGoal correctly activates goal mode
