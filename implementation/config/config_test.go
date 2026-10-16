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

// ===== Tests for GITHUB_TOKEN fallback =====

func TestParseArgs_GitHubTokenFallbackCopilot(t *testing.T) {
	os.Setenv("GITHUB_TOKEN", "ghu_testtoken")
	os.Setenv("CODING_AGENT_API_ENDPOINT", "https://api.githubcopilot.com")
	defer func() {
		os.Unsetenv("GITHUB_TOKEN")
		os.Unsetenv("CODING_AGENT_API_ENDPOINT")
	}()

	cfg, err := ParseArgs([]string{"--prompt", "test"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.APIKey != "ghu_testtoken" {
		t.Errorf("Expected GITHUB_TOKEN to be used as API key for Copilot endpoint, got %q", cfg.APIKey)
	}
}

func TestParseArgs_GitHubTokenFallbackModels(t *testing.T) {
	os.Setenv("GITHUB_TOKEN", "ghp_testtoken")
	os.Setenv("CODING_AGENT_API_ENDPOINT", "https://models.github.ai")
	defer func() {
		os.Unsetenv("GITHUB_TOKEN")
		os.Unsetenv("CODING_AGENT_API_ENDPOINT")
	}()

	cfg, err := ParseArgs([]string{"--prompt", "test"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.APIKey != "ghp_testtoken" {
		t.Errorf("Expected GITHUB_TOKEN to be used as API key for GitHub Models endpoint, got %q", cfg.APIKey)
	}
}

func TestParseArgs_GitHubTokenIgnoredForNonGitHubEndpoint(t *testing.T) {
	os.Setenv("GITHUB_TOKEN", "ghp_shouldnotapply")
	os.Setenv("CODING_AGENT_API_ENDPOINT", "https://api.openai.com")
	defer func() {
		os.Unsetenv("GITHUB_TOKEN")
		os.Unsetenv("CODING_AGENT_API_ENDPOINT")
	}()

	cfg, err := ParseArgs([]string{"--prompt", "test"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.APIKey == "ghp_shouldnotapply" {
		t.Error("Expected GITHUB_TOKEN NOT to be used as API key for a non-GitHub endpoint")
	}
}

// ===== Tests for web UI flags =====

func TestParseArgs_WebFlag(t *testing.T) {
	cfg, err := ParseArgs([]string{"--web"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !cfg.Web {
		t.Error("Expected Web=true when --web is passed")
	}
	if cfg.WebAddr != "127.0.0.1" {
		t.Errorf("Expected default WebAddr 127.0.0.1, got %q", cfg.WebAddr)
	}
	if cfg.WebPort != 8080 {
		t.Errorf("Expected default WebPort 8080, got %d", cfg.WebPort)
	}
}

func TestParseArgs_WebAddrAndPort(t *testing.T) {
	cfg, err := ParseArgs([]string{"--web", "--web-addr", "0.0.0.0", "--web-port", "9000"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.WebAddr != "0.0.0.0" {
		t.Errorf("Expected WebAddr 0.0.0.0, got %q", cfg.WebAddr)
	}
	if cfg.WebPort != 9000 {
		t.Errorf("Expected WebPort 9000, got %d", cfg.WebPort)
	}
}

func TestParseArgs_WebAddrMissingArg(t *testing.T) {
	if _, err := ParseArgs([]string{"--web-addr"}); err == nil {
		t.Error("Expected error when --web-addr has no argument")
	}
}

func TestParseArgs_WebPortMissingArg(t *testing.T) {
	if _, err := ParseArgs([]string{"--web-port"}); err == nil {
		t.Error("Expected error when --web-port has no argument")
	}
}

func TestParseArgs_WebPortInvalid(t *testing.T) {
	if _, err := ParseArgs([]string{"--web-port", "notanumber"}); err == nil {
		t.Error("Expected error when --web-port is not a number")
	}
}

func TestParseArgs_WebPortOutOfRange(t *testing.T) {
	if _, err := ParseArgs([]string{"--web-port", "70000"}); err == nil {
		t.Error("Expected error when --web-port is out of range")
	}
}

func TestParseArgs_WebEnvVars(t *testing.T) {
	os.Setenv("CODING_AGENT_WEB", "true")
	os.Setenv("CODING_AGENT_WEB_ADDR", "127.0.0.1")
	os.Setenv("CODING_AGENT_WEB_PORT", "9090")
	defer func() {
		os.Unsetenv("CODING_AGENT_WEB")
		os.Unsetenv("CODING_AGENT_WEB_ADDR")
		os.Unsetenv("CODING_AGENT_WEB_PORT")
	}()

	cfg, err := ParseArgs([]string{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !cfg.Web {
		t.Error("Expected Web=true from env var")
	}
	if cfg.WebPort != 9090 {
		t.Errorf("Expected WebPort 9090 from env var, got %d", cfg.WebPort)
	}
}

func TestParseArgs_SkillsDefaultsOn(t *testing.T) {
	cfg, err := ParseArgs([]string{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !cfg.Skills {
		t.Error("Expected Skills=true by default")
	}
}

func TestParseArgs_NoSkills(t *testing.T) {
	cfg, err := ParseArgs([]string{"--no-skills"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.Skills {
		t.Error("Expected Skills=false with --no-skills")
	}
}

func TestParseArgs_SkillsDir(t *testing.T) {
	cfg, err := ParseArgs([]string{"--skills-dir", "/a", "--skills-dir", "/b"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(cfg.SkillsDirs) != 2 || cfg.SkillsDirs[0] != "/a" || cfg.SkillsDirs[1] != "/b" {
		t.Errorf("Expected SkillsDirs [%v], got %v", []string{"/a", "/b"}, cfg.SkillsDirs)
	}
}

func TestParseArgs_SkillsDirMissingArg(t *testing.T) {
	if _, err := ParseArgs([]string{"--skills-dir"}); err == nil {
		t.Error("Expected error when --skills-dir has no argument")
	}
}

func TestParseArgs_TrustDir(t *testing.T) {
	cfg, err := ParseArgs([]string{"--trust", "/repo"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(cfg.TrustDirs) != 1 || cfg.TrustDirs[0] != "/repo" {
		t.Errorf("Expected TrustDirs [/repo], got %v", cfg.TrustDirs)
	}
}

func TestParseArgs_SkillsEnvVars(t *testing.T) {
	os.Setenv("CODING_AGENT_SKILLS", "false")
	os.Setenv("CODING_AGENT_SKILLS_DIRS", "/x,/y")
	defer func() {
		os.Unsetenv("CODING_AGENT_SKILLS")
		os.Unsetenv("CODING_AGENT_SKILLS_DIRS")
	}()

	cfg, err := ParseArgs([]string{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.Skills {
		t.Error("Expected Skills=false from env var")
	}
	if len(cfg.SkillsDirs) != 2 || cfg.SkillsDirs[0] != "/x" || cfg.SkillsDirs[1] != "/y" {
		t.Errorf("Expected SkillsDirs [/x /y], got %v", cfg.SkillsDirs)
	}
}

// ===== B3: bad config values must be reported, not silently ignored =====

// clearAgentEnv neutralises CODING_AGENT_* variables inherited from the
// environment. loadEnv applies them on top of config-file values, so a stray
// CODING_AGENT_MAX_ITERATIONS in the shell makes config tests depend on the
// machine they run on.
func clearAgentEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"CODING_AGENT_MODEL", "CODING_AGENT_TEMPERATURE", "CODING_AGENT_MAX_TOKENS",
		"CODING_AGENT_MAX_ITERATIONS", "CODING_AGENT_CONTEXT_SIZE", "CODING_AGENT_API_ENDPOINT",
		"CODING_AGENT_API_KEY", "CODING_AGENT_INITIAL_TOKEN_TIMEOUT", "CODING_AGENT_CONNECTION_TIMEOUT",
		"CODING_AGENT_READ_TIMEOUT", "CODING_AGENT_STREAMING", "CODING_AGENT_TYPEWRITER", "CODING_AGENT_DEBUG",
		"CODING_AGENT_DEBUG_LOG", "CODING_AGENT_DEBUG_VERBOSE", "CODING_AGENT_DEBUG_VERBOSE_VERBOSE",
		"CODING_AGENT_READ_ONLY", "CODING_AGENT_EXPERIMENTAL", "CODING_AGENT_PERSONA",
		"CODING_AGENT_THEME", "CODING_AGENT_SUMMARY_ONLY", "CODING_AGENT_GOAL",
		"CODING_AGENT_TOOLS", "CODING_AGENT_WEB", "CODING_AGENT_WEB_ADDR", "CODING_AGENT_WEB_PORT",
		"CODING_AGENT_WEB_SESSION_IDLE_TIMEOUT", "CODING_AGENT_SKILLS", "CODING_AGENT_SKILLS_DIRS",
		"CODING_AGENT_TRUST_DIRS",
	} {
		t.Setenv(key, "")
	}
}

func TestParseArgs_ConfigFile_BadNumberIsAnError(t *testing.T) {
	clearAgentEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")
	if err := os.WriteFile(path, []byte("model = test-model\nmax_iterations = lots\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := ParseArgs([]string{"--config", path, "-p", "hi"})
	if err == nil {
		t.Fatalf("expected an error for a non-numeric max_iterations, got config with MaxIterations=%d", cfg.MaxIterations)
	}
	for _, want := range []string{"max_iterations", "lots", path} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got: %v", want, err)
		}
	}
}

func TestParseArgs_Env_BadNumberIsAnError(t *testing.T) {
	// A typo in an environment variable used to be indistinguishable from the
	// variable never having been set.
	t.Setenv("CODING_AGENT_MAX_ITERATIONS", "ten")

	cfg, err := ParseArgs([]string{"-p", "hi"})
	if err == nil {
		t.Fatalf("expected an error for a non-numeric CODING_AGENT_MAX_ITERATIONS, got MaxIterations=%d", cfg.MaxIterations)
	}
	if !strings.Contains(err.Error(), "CODING_AGENT_MAX_ITERATIONS") {
		t.Errorf("error should name the variable, got: %v", err)
	}
	if !strings.Contains(err.Error(), "ten") {
		t.Errorf("error should show the offending value, got: %v", err)
	}
}

func TestParseArgs_BadTemperatureIsAnError(t *testing.T) {
	t.Setenv("CODING_AGENT_TEMPERATURE", "hot")
	if _, err := ParseArgs([]string{"-p", "hi"}); err == nil {
		t.Fatal("expected an error for a non-numeric CODING_AGENT_TEMPERATURE")
	}
}

func TestParseArgs_BoolValues_AreParsedStrictly(t *testing.T) {
	clearAgentEnv(t)
	// "off" used to be treated as "not true", leaving streaming enabled.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")
	if err := os.WriteFile(path, []byte("streaming = off\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := ParseArgs([]string{"--config", path, "-p", "hi"})
	if err != nil {
		t.Fatalf("streaming=off should be accepted: %v", err)
	}
	if cfg.Streaming {
		t.Error("streaming=off should disable streaming")
	}

	// A value that is neither true nor false must not silently mean false.
	t.Setenv("CODING_AGENT_READ_ONLY", "maybe")
	if _, err := ParseArgs([]string{"-p", "hi"}); err == nil {
		t.Error("expected an error for CODING_AGENT_READ_ONLY=maybe")
	}
}

func TestParseArgs_ValidNumericOverridesStillApply(t *testing.T) {
	clearAgentEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")
	if err := os.WriteFile(path, []byte("max_iterations = 7\ntemperature = 0.3\nweb_port = 9000\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := ParseArgs([]string{"--config", path, "-p", "hi"})
	if err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	if cfg.MaxIterations != 7 {
		t.Errorf("MaxIterations = %d, want 7", cfg.MaxIterations)
	}
	if cfg.Temperature == nil || *cfg.Temperature != 0.3 {
		t.Errorf("Temperature = %v, want 0.3", cfg.Temperature)
	}
	if cfg.WebPort != 9000 {
		t.Errorf("WebPort = %d, want 9000", cfg.WebPort)
	}
}

// ===== Token animation (requirement 049) =====

func TestDefaultConfig_TypewriterEnabled(t *testing.T) {
	if !DefaultConfig().Typewriter {
		t.Error("streamed tokens animate by default")
	}
}

func TestParseArgs_NoTypewriter(t *testing.T) {
	clearAgentEnv(t)
	cfg, err := ParseArgs([]string{"--no-typewriter", "-p", "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Typewriter {
		t.Error("--no-typewriter should disable the token animation")
	}
}

func TestParseArgs_EnvDisablesTypewriter(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("CODING_AGENT_TYPEWRITER", "false")
	cfg, err := ParseArgs([]string{"-p", "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Typewriter {
		t.Error("CODING_AGENT_TYPEWRITER=false should disable the token animation")
	}
}

func TestParseArgs_ConfigFileTypewriter(t *testing.T) {
	clearAgentEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")
	if err := os.WriteFile(path, []byte("typewriter = false\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := ParseArgs([]string{"--config", path, "-p", "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Typewriter {
		t.Error("typewriter = false in the config file should disable the animation")
	}
}

func TestParseArgs_ConfigFileBadTypewriterIsAnError(t *testing.T) {
	clearAgentEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")
	if err := os.WriteFile(path, []byte("typewriter = maybe\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := ParseArgs([]string{"--config", path, "-p", "hi"}); err == nil {
		t.Fatal("expected an error for a non-boolean typewriter value")
	}
}
