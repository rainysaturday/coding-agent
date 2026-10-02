package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// clearEnv neutralises any CODING_AGENT_* variables inherited from the
// environment so tests are deterministic.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "CODING_AGENT_") {
			t.Setenv(strings.SplitN(kv, "=", 2)[0], "")
		}
	}
}

func TestDefaults(t *testing.T) {
	c := NewDefault()
	if c.ContextSize != 128000 {
		t.Fatalf("context size default: %d", c.ContextSize)
	}
	if c.ContextThreshold != 0.8 {
		t.Fatalf("threshold default: %v", c.ContextThreshold)
	}
	if c.MaxIterations != 1000 {
		t.Fatalf("max iterations default: %d", c.MaxIterations)
	}
	if c.InitialTokenTimeout != 86400 {
		t.Fatalf("initial token timeout default: %d", c.InitialTokenTimeout)
	}
	if !c.Stream {
		t.Fatal("stream should default to on")
	}
	if c.MaxHistory != 100 {
		t.Fatalf("history default: %d", c.MaxHistory)
	}
	if c.Theme != "dark" {
		t.Fatalf("theme default: %s", c.Theme)
	}
	if c.AutoCompress != true {
		t.Fatal("auto compress should default on")
	}
}

func TestParseFlagsBasics(t *testing.T) {
	c := NewDefault()
	err := c.ParseFlags([]string{
		"-p", "hello",
		"--model", "gpt-4",
		"--api-endpoint", "http://example.com:1234",
		"--context-size", "4096",
		"--max-iterations", "5",
		"--temperature", "0.5",
		"--no-stream",
		"--read-only",
		"--theme", "gruvbox",
		"--web", "--web-port", "9090",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.Model != "gpt-4" || c.APIEndpoint != "http://example.com:1234" {
		t.Fatalf("flags not applied: %+v", c)
	}
	if c.ContextSize != 4096 || c.MaxIterations != 5 {
		t.Fatal("numeric flags not applied")
	}
	if c.Temperature == nil || *c.Temperature != 0.5 {
		t.Fatal("temperature not captured")
	}
	if c.Stream {
		t.Fatal("--no-stream not honored")
	}
	if !c.ReadOnly {
		t.Fatal("--read-only not honored")
	}
	if c.Theme != "gruvbox" {
		t.Fatal("--theme not honored")
	}
	if c.Mode != WebMode || c.WebPort != 9090 {
		t.Fatalf("web mode not detected: %v %d", c.Mode, c.WebPort)
	}
	if c.Mode != WebMode {
		t.Fatal("web mode should win over one-shot")
	}
}

func TestModeDetection(t *testing.T) {
	c := NewDefault()
	if err := c.ParseFlags([]string{"-p", "x"}); err != nil {
		t.Fatal(err)
	}
	if c.Mode != OneShotMode {
		t.Fatalf("expected one-shot, got %v", c.Mode)
	}
	c = NewDefault()
	if err := c.ParseFlags([]string{"--stdin"}); err != nil {
		t.Fatal(err)
	}
	if c.Mode != OneShotMode {
		t.Fatal("stdin should trigger one-shot mode")
	}
	c = NewDefault()
	if err := c.ParseFlags([]string{"--load", "/tmp/x.json"}); err != nil {
		t.Fatal(err)
	}
	if c.Mode != OneShotMode {
		t.Fatal("--load should trigger one-shot mode")
	}
}

func TestEnvAndFlagPrecedence(t *testing.T) {
	clearEnv(t)
	t.Setenv("CODING_AGENT_MODEL", "env-model")
	t.Setenv("CODING_AGENT_THEME", "light")
	c, err := Load([]string{"--model", "flag-model"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Model != "flag-model" {
		t.Fatalf("flag should override env: %s", c.Model)
	}
	if c.Theme != "light" {
		t.Fatalf("env should apply when no flag: %s", c.Theme)
	}
}

func TestEnvCommon(t *testing.T) {
	clearEnv(t)
	t.Setenv("CODING_AGENT_API_ENDPOINT", "http://env:1/v1")
	t.Setenv("CODING_AGENT_API_KEY", "secret")
	t.Setenv("CODING_AGENT_CONTEXT_SIZE", "2048")
	t.Setenv("CODING_AGENT_MAX_ITERATIONS", "7")
	t.Setenv("CODING_AGENT_READ_ONLY", "true")
	c, err := Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.APIEndpoint != "http://env:1/v1" || c.APIKey != "secret" {
		t.Fatalf("env not applied: %+v", c)
	}
	if c.ContextSize != 2048 || c.MaxIterations != 7 || !c.ReadOnly {
		t.Fatalf("env not applied: %+v", c)
	}
}

func TestInvalidTheme(t *testing.T) {
	clearEnv(t)
	_, err := Load([]string{"--theme", "neon"})
	if err == nil {
		t.Fatal("invalid theme should error")
	}
}

func TestValidateInitialTokenTimeout(t *testing.T) {
	c := NewDefault()
	c.InitialTokenTimeout = 5
	if err := c.Validate(); err == nil {
		t.Fatal("timeout below minimum should fail validation")
	}
	c.InitialTokenTimeout = 0
	if err := c.Validate(); err == nil {
		t.Fatal("zero timeout should fail validation")
	}
	c = NewDefault()
	if err := c.Validate(); err != nil {
		t.Fatalf("defaults should validate: %v", err)
	}
}

func TestConfigFileINI(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "rc")
	content := `# comment
model = file-model
context_size = 555

[mcp_server.files]
command = /bin/echo
args = a b
enabled = true
transport = stdio
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load([]string{"--config", path})
	if err != nil {
		t.Fatal(err)
	}
	if c.Model != "file-model" {
		t.Fatalf("config file model not applied: %s", c.Model)
	}
	if c.ContextSize != 555 {
		t.Fatalf("config file context size not applied: %d", c.ContextSize)
	}
	srv, ok := c.MCPServers["files"]
	if !ok || !srv.Enabled || srv.Command != "/bin/echo" || len(srv.Args) != 2 {
		t.Fatalf("mcp section not parsed: %+v", c.MCPServers)
	}
}

func TestMCPServerEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv("CODING_AGENT_MCP_WEATHER_COMMAND", "/usr/bin/weather")
	t.Setenv("CODING_AGENT_MCP_WEATHER_ENABLED", "true")
	c, err := Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, ok := c.MCPServers["weather"]
	if !ok || !srv.Enabled || srv.Command != "/usr/bin/weather" {
		t.Fatalf("mcp env not applied: %+v", c.MCPServers)
	}
	if len(c.EnabledMCPServers()) != 1 {
		t.Fatalf("expected 1 enabled server, got %d", len(c.EnabledMCPServers()))
	}
}

func TestCopilotEndpointPaths(t *testing.T) {
	if !IsCopilotEndpoint("https://api.githubcopilot.com") {
		t.Fatal("copilot endpoint not detected")
	}
	if IsCopilotEndpoint("http://localhost:8080") {
		t.Fatal("false copilot detection")
	}
	if !IsGitHubModelsEndpoint("https://models.github.ai/inference") {
		t.Fatal("github models endpoint not detected")
	}
}
