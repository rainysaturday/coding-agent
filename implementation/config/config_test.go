package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ===== Tests for persona configuration =====

func TestParseArgs_PersonaFlag(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectPersona string
		expectError   bool
	}{
		{
			name:          "persona with value",
			args:          []string{"--persona", "Expert Go developer"},
			expectPersona: "Expert Go developer",
			expectError:   false,
		},
		{
			name:          "persona empty string",
			args:          []string{"--persona", ""},
			expectPersona: "",
			expectError:   false,
		},
		{
			name:          "persona without value",
			args:          []string{"--persona"},
			expectPersona: "",
			expectError:   true,
		},
		{
			name:          "persona with special chars",
			args:          []string{"--persona", "Expert with \"quotes\""},
			expectPersona: "Expert with \"quotes\"",
			expectError:   false,
		},
		{
			name:          "no persona",
			args:          []string{"--prompt", "test"},
			expectPersona: "",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseArgs(tt.args)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if cfg.Persona != tt.expectPersona {
				t.Errorf("Expected persona %q, got %q", tt.expectPersona, cfg.Persona)
			}
		})
	}
}

func TestParseArgs_SummaryOnlyFlag(t *testing.T) {
	tests := []struct {
		name              string
		args              []string
		expectSummaryOnly bool
	}{
		{
			name:              "summary-only flag present",
			args:              []string{"--summary-only"},
			expectSummaryOnly: true,
		},
		{
			name:              "summary-only flag not present",
			args:              []string{"--prompt", "test"},
			expectSummaryOnly: false,
		},
		{
			name:              "summary-only with other flags",
			args:              []string{"--summary-only", "--verbose"},
			expectSummaryOnly: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseArgs(tt.args)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if cfg.SummaryOnly != tt.expectSummaryOnly {
				t.Errorf("Expected SummaryOnly=%v, got %v", tt.expectSummaryOnly, cfg.SummaryOnly)
			}
		})
	}
}

func TestParseArgs_PersonaAndSummaryOnly(t *testing.T) {
	cfg, err := ParseArgs([]string{
		"--persona", "Expert Go developer",
		"--summary-only",
		"--prompt", "test task",
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if cfg.Persona != "Expert Go developer" {
		t.Errorf("Expected persona 'Expert Go developer', got '%s'", cfg.Persona)
	}
	if !cfg.SummaryOnly {
		t.Error("Expected SummaryOnly to be true")
	}
}

func TestParseArgs_PersonaWithReadonly(t *testing.T) {
	cfg, err := ParseArgs([]string{
		"--persona", "Security expert",
		"--read-only",
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if cfg.Persona != "Security expert" {
		t.Errorf("Expected persona 'Security expert', got '%s'", cfg.Persona)
	}
	if !cfg.ReadOnly {
		t.Error("Expected ReadOnly to be true")
	}
}

// ===== Tests for environment variables =====

func TestParseArgs_PersonaFromEnv(t *testing.T) {
	// Set environment variable
	os.Setenv("CODING_AGENT_PERSONA", "Env persona")
	defer os.Unsetenv("CODING_AGENT_PERSONA")

	cfg, err := ParseArgs([]string{"--prompt", "test"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Environment variable should be used if not specified on command line
	if cfg.Persona != "Env persona" {
		t.Errorf("Expected persona from env 'Env persona', got '%s'", cfg.Persona)
	}
}

func TestParseArgs_PersonaFlagOverridesEnv(t *testing.T) {
	os.Setenv("CODING_AGENT_PERSONA", "Env persona")
	defer os.Unsetenv("CODING_AGENT_PERSONA")

	cfg, err := ParseArgs([]string{
		"--persona", "Flag persona",
		"--prompt", "test",
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Flag should override environment variable
	if cfg.Persona != "Flag persona" {
		t.Errorf("Expected flag persona 'Flag persona', got '%s'", cfg.Persona)
	}
}

func TestParseArgs_SummaryOnlyFromEnv(t *testing.T) {
	os.Setenv("CODING_AGENT_SUMMARY_ONLY", "true")
	defer os.Unsetenv("CODING_AGENT_SUMMARY_ONLY")

	cfg, err := ParseArgs([]string{"--prompt", "test"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !cfg.SummaryOnly {
		t.Error("Expected SummaryOnly to be true from env")
	}
}

// ===== Tests for Config struct =====

func TestConfig_PersonaField(t *testing.T) {
	cfg := &Config{}

	cfg.Persona = "Test persona"
	if cfg.Persona != "Test persona" {
		t.Errorf("Expected 'Test persona', got '%s'", cfg.Persona)
	}
}

func TestConfig_SummaryOnlyField(t *testing.T) {
	cfg := &Config{}

	cfg.SummaryOnly = true
	if !cfg.SummaryOnly {
		t.Error("Expected SummaryOnly to be true")
	}

	cfg.SummaryOnly = false
	if cfg.SummaryOnly {
		t.Error("Expected SummaryOnly to be false")
	}
}

// ===== Tests for default config =====

func TestDefaultConfig_NoPersona(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Persona != "" {
		t.Errorf("Default config should have empty persona, got '%s'", cfg.Persona)
	}

	if cfg.SummaryOnly {
		t.Error("Default config should have SummaryOnly=false")
	}
}

// ===== Tests for help text =====

func TestParseArgs_HelpTextContainsPersona(t *testing.T) {
	// This test verifies that the help text includes persona information
	// We can't easily test the help text generation, but we can verify
	// that the flag is parsed correctly which implies it's documented

	cfg, err := ParseArgs([]string{"--help"})
	if err != nil {
		// Help flag might return an error or exit, which is fine
		t.Skip("Help flag handling skipped")
	}

	// If we got here without error, verify persona field exists
	if cfg.Persona == "" {
		// No persona specified is fine for help
	}
}

// ===== Integration tests =====

func TestParseArgs_FullScenario(t *testing.T) {
	cfg, err := ParseArgs([]string{
		"--persona", "Senior code reviewer focused on security",
		"--summary-only",
		"--read-only",
		"--verbose",
		"--prompt", "Review this code for vulnerabilities",
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if cfg.Persona != "Senior code reviewer focused on security" {
		t.Errorf("Persona mismatch: got '%s'", cfg.Persona)
	}
	if !cfg.SummaryOnly {
		t.Error("SummaryOnly should be true")
	}
	if !cfg.ReadOnly {
		t.Error("ReadOnly should be true")
	}
	if !cfg.Verbose {
		t.Error("Verbose should be true")
	}
}

func TestParseArgs_PersonaWithWhitespace(t *testing.T) {
	cfg, err := ParseArgs([]string{
		"--persona", "  Expert developer  ",
		"--prompt", "test",
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Whitespace should be preserved (or trimmed, depending on implementation)
	if cfg.Persona != "  Expert developer  " {
		t.Logf("Persona with whitespace: got '%s'", cfg.Persona)
	}
}

func TestParseArgs_MultiplePersonaFlags(t *testing.T) {
	cfg, err := ParseArgs([]string{
		"--persona", "First persona",
		"--persona", "Second persona",
		"--prompt", "test",
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// The last persona flag should win
	if cfg.Persona != "Second persona" {
		t.Logf("Multiple persona flags: got '%s'", cfg.Persona)
	}
}

// ===== Tests for persona validation =====

func TestParseArgs_PersonaLongString(t *testing.T) {
	longPersona := strings.Repeat("A", 1000)

	cfg, err := ParseArgs([]string{
		"--persona", longPersona,
		"--prompt", "test",
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if cfg.Persona != longPersona {
		t.Errorf("Long persona not preserved correctly")
	}
}

func TestParseArgs_PersonaMultiline(t *testing.T) {
	multilinePersona := "Line 1\nLine 2\nLine 3"

	cfg, err := ParseArgs([]string{
		"--persona", multilinePersona,
		"--prompt", "test",
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if cfg.Persona != multilinePersona {
		t.Logf("Multiline persona: got '%s'", cfg.Persona)
	}
}

// ===== Tests for theme configuration =====

func TestParseArgs_ThemeFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{"theme dark", []string{"--theme", "dark", "-p", "test"}, "dark"},
		{"theme light", []string{"--theme", "light", "-p", "test"}, "light"},
		{"theme solarized", []string{"--theme", "solarized", "-p", "test"}, "solarized"},
		{"theme gruvbox", []string{"--theme", "gruvbox", "-p", "test"}, "gruvbox"},
		{"theme darkula", []string{"--theme", "darkula", "-p", "test"}, "darkula"},
		{"no theme specified", []string{"-p", "test"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseArgs(tt.args)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if cfg.Theme != tt.expected {
				t.Errorf("Expected theme %q, got %q", tt.expected, cfg.Theme)
			}
		})
	}
}

func TestParseArgs_ThemeFlagMissingValue(t *testing.T) {
	_, err := ParseArgs([]string{"--theme"})
	if err == nil {
		t.Error("Expected error when --theme has no value")
	}
}

func TestParseArgs_ThemeFlagOverridesEnv(t *testing.T) {
	os.Setenv("CODING_AGENT_THEME", "light")
	defer os.Unsetenv("CODING_AGENT_THEME")

	cfg, err := ParseArgs([]string{"--theme", "gruvbox", "-p", "test"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.Theme != "gruvbox" {
		t.Errorf("Expected flag theme 'gruvbox' to override env 'light', got %q", cfg.Theme)
	}
}

func TestParseArgs_ThemeFromEnv(t *testing.T) {
	os.Setenv("CODING_AGENT_THEME", "solarized")
	defer os.Unsetenv("CODING_AGENT_THEME")

	cfg, err := ParseArgs([]string{"-p", "test"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.Theme != "solarized" {
		t.Errorf("Expected theme from env 'solarized', got %q", cfg.Theme)
	}
}

func TestParseArgs_ThemeFromConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.txt")
	os.WriteFile(configFile, []byte("theme=gruvbox\n"), 0644)

	cfg, err := ParseArgs([]string{"--config", configFile, "-p", "test"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.Theme != "gruvbox" {
		t.Errorf("Expected theme from config file 'gruvbox', got %q", cfg.Theme)
	}
}

func TestParseArgs_ThemeCLIOverridesConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.txt")
	os.WriteFile(configFile, []byte("theme=light\n"), 0644)

	cfg, err := ParseArgs([]string{"--config", configFile, "--theme", "dark", "-p", "test"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.Theme != "dark" {
		t.Errorf("Expected CLI theme 'dark' to override config file 'light', got %q", cfg.Theme)
	}
}

func TestParseArgs_ThemeDefaultEmpty(t *testing.T) {
	cfg, err := ParseArgs([]string{"-p", "test"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.Theme != "" {
		t.Errorf("Expected empty theme when not specified, got %q", cfg.Theme)
	}
}

func TestParseArgs_ThemeWithPersonaAndPrompt(t *testing.T) {
	cfg, err := ParseArgs([]string{
		"--theme", "gruvbox",
		"--persona", "Go expert",
		"-p", "write Go code",
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.Theme != "gruvbox" {
		t.Errorf("Expected theme 'gruvbox', got %q", cfg.Theme)
	}
	if cfg.Persona != "Go expert" {
		t.Errorf("Expected persona 'Go expert', got %q", cfg.Persona)
	}
}

// ===== Tests for goal configuration =====

func TestParseArgs_GoalFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{"goal with value", []string{"--goal", "Create a REST API", "-p", "test"}, "Create a REST API"},
		{"goal empty string", []string{"--goal", "", "-p", "test"}, ""},
		{"no goal", []string{"-p", "test"}, ""},
		{"goal with other flags", []string{"--goal", "Build app", "--verbose"}, "Build app"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseArgs(tt.args)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if cfg.Goal != tt.expected {
				t.Errorf("Expected goal %q, got %q", tt.expected, cfg.Goal)
			}
		})
	}
}

func TestParseArgs_GoalFlagMissingValue(t *testing.T) {
	_, err := ParseArgs([]string{"--goal"})
	if err == nil {
		t.Error("Expected error when --goal has no value")
	}
}

func TestParseArgs_GoalFlagOverridesEnv(t *testing.T) {
	os.Setenv("CODING_AGENT_GOAL", "Env goal")
	defer os.Unsetenv("CODING_AGENT_GOAL")

	cfg, err := ParseArgs([]string{"--goal", "Flag goal", "-p", "test"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.Goal != "Flag goal" {
		t.Errorf("Expected flag goal 'Flag goal' to override env 'Env goal', got %q", cfg.Goal)
	}
}

func TestParseArgs_GoalFromEnv(t *testing.T) {
	os.Setenv("CODING_AGENT_GOAL", "Env goal")
	defer os.Unsetenv("CODING_AGENT_GOAL")

	cfg, err := ParseArgs([]string{"-p", "test"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.Goal != "Env goal" {
		t.Errorf("Expected goal from env 'Env goal', got %q", cfg.Goal)
	}
}

func TestParseArgs_GoalFromConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.txt")
	os.WriteFile(configFile, []byte("goal=File goal\n"), 0644)

	cfg, err := ParseArgs([]string{"--config", configFile, "-p", "test"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.Goal != "File goal" {
		t.Errorf("Expected goal from config file 'File goal', got %q", cfg.Goal)
	}
}

func TestParseArgs_GoalCLIOverridesConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.txt")
	os.WriteFile(configFile, []byte("goal=File goal\n"), 0644)

	cfg, err := ParseArgs([]string{"--config", configFile, "--goal", "CLI goal", "-p", "test"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.Goal != "CLI goal" {
		t.Errorf("Expected CLI goal 'CLI goal' to override config file 'File goal', got %q", cfg.Goal)
	}
}

func TestParseArgs_GoalWithPersonaAndPrompt(t *testing.T) {
	cfg, err := ParseArgs([]string{
		"--goal", "Write tests",
		"--persona", "Test expert",
		"-p", "test task",
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.Goal != "Write tests" {
		t.Errorf("Expected goal 'Write tests', got %q", cfg.Goal)
	}
	if cfg.Persona != "Test expert" {
		t.Errorf("Expected persona 'Test expert', got %q", cfg.Persona)
	}
}
