package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsGitHubCopilotEndpoint_True(t *testing.T) {
	endpoints := []string{
		"https://api.githubcopilot.com",
		"http://api.githubcopilot.com",
		"https://api.githubcopilot.com/chat/completions",
	}

	for _, endpoint := range endpoints {
		if !strings.Contains(endpoint, "githubcopilot.com") {
			t.Errorf("Expected Copilot detection for %q to return true", endpoint)
		}
	}
}

func TestIsGitHubCopilotEndpoint_False(t *testing.T) {
	endpoints := []string{
		"http://localhost:8080",
		"https://api.openai.com/v1",
		"https://models.github.ai",
		"https://api.anthropic.com/v1",
	}

	for _, endpoint := range endpoints {
		if strings.Contains(endpoint, "githubcopilot.com") {
			t.Errorf("Expected Copilot detection for %q to return false", endpoint)
		}
	}
}

func TestParseArgs_APIEndpointFlag(t *testing.T) {
	args := []string{"--api-endpoint", "https://api.githubcopilot.com"}
	cfg, err := ParseArgs(args)
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}

	if cfg.APIEndpoint != "https://api.githubcopilot.com" {
		t.Errorf("Expected APIEndpoint 'https://api.githubcopilot.com', got '%s'", cfg.APIEndpoint)
	}
}

func TestParseArgs_APIEndpointFlagWithAPIKey(t *testing.T) {
	args := []string{
		"--api-endpoint", "https://api.githubcopilot.com",
		"--api-key", "ghu_test_token",
		"--model", "gpt-4o",
	}
	cfg, err := ParseArgs(args)
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}

	if cfg.APIEndpoint != "https://api.githubcopilot.com" {
		t.Errorf("Expected APIEndpoint 'https://api.githubcopilot.com', got '%s'", cfg.APIEndpoint)
	}

	if cfg.APIKey != "ghu_test_token" {
		t.Errorf("Expected APIKey 'ghu_test_token', got '%s'", cfg.APIKey)
	}
}

func TestLoadEnv_GitHubTokenCopilotEndpoint(t *testing.T) {
	// Set up environment
	endpoint := "https://api.githubcopilot.com"
	githubToken := "ghu_test_copilot_token"

	os.Setenv("CODING_AGENT_API_ENDPOINT", endpoint)
	os.Setenv("GITHUB_TOKEN", githubToken)
	defer os.Unsetenv("CODING_AGENT_API_ENDPOINT")
	defer os.Unsetenv("GITHUB_TOKEN")

	cfg := DefaultConfig()
	loadEnv(cfg)

	// GITHUB_TOKEN should be used as fallback for API key
	if cfg.APIKey != githubToken {
		t.Errorf("Expected APIKey to be '%s' from GITHUB_TOKEN, got '%s'", githubToken, cfg.APIKey)
	}
}

func TestLoadEnv_GitHubTokenNotUsedForNonCopilotEndpoint(t *testing.T) {
	// Set up environment
	os.Setenv("CODING_AGENT_API_ENDPOINT", "http://localhost:8080")
	os.Setenv("GITHUB_TOKEN", "ghu_test_token")
	os.Unsetenv("CODING_AGENT_API_KEY")
	defer os.Unsetenv("CODING_AGENT_API_ENDPOINT")
	defer os.Unsetenv("GITHUB_TOKEN")

	cfg := DefaultConfig()
	loadEnv(cfg)

	// GITHUB_TOKEN should NOT be used for non-Copilot endpoints
	if cfg.APIKey != "" {
		t.Errorf("Expected APIKey to be empty for non-Copilot endpoint, got '%s'", cfg.APIKey)
	}
}

func TestLoadEnv_APIKeyOverridesGitHubToken(t *testing.T) {
	// Set up environment
	os.Setenv("CODING_AGENT_API_ENDPOINT", "https://api.githubcopilot.com")
	os.Setenv("CODING_AGENT_API_KEY", "sk_test_key")
	os.Setenv("GITHUB_TOKEN", "ghu_test_token")
	defer os.Unsetenv("CODING_AGENT_API_ENDPOINT")
	defer os.Unsetenv("CODING_AGENT_API_KEY")
	defer os.Unsetenv("GITHUB_TOKEN")

	cfg := DefaultConfig()
	loadEnv(cfg)

	// CODING_AGENT_API_KEY should take precedence
	if cfg.APIKey != "sk_test_key" {
		t.Errorf("Expected APIKey to be 'sk_test_key', got '%s'", cfg.APIKey)
	}
}

func TestLoadEnv_NoGitHubTokenFallback(t *testing.T) {
	// Set up environment without GITHUB_TOKEN
	os.Setenv("CODING_AGENT_API_ENDPOINT", "https://api.githubcopilot.com")
	os.Unsetenv("GITHUB_TOKEN")
	os.Unsetenv("CODING_AGENT_API_KEY")
	defer os.Unsetenv("CODING_AGENT_API_ENDPOINT")

	cfg := DefaultConfig()
	loadEnv(cfg)

	// APIKey should remain empty
	if cfg.APIKey != "" {
		t.Errorf("Expected APIKey to be empty, got '%s'", cfg.APIKey)
	}
}

func TestLoadConfigFile_APIEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.txt")

	configContent := `api_endpoint=https://api.githubcopilot.com
api_key=ghu_test_token
model=gpt-4o`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg := DefaultConfig()
	err = loadConfigFile(configPath, cfg)
	if err != nil {
		t.Fatalf("loadConfigFile() error: %v", err)
	}

	if cfg.APIEndpoint != "https://api.githubcopilot.com" {
		t.Errorf("Expected APIEndpoint 'https://api.githubcopilot.com', got '%s'", cfg.APIEndpoint)
	}

	if cfg.APIKey != "ghu_test_token" {
		t.Errorf("Expected APIKey 'ghu_test_token', got '%s'", cfg.APIKey)
	}
}

func TestDefaultConfig_APIEndpoint(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.APIEndpoint != "http://localhost:8080" {
		t.Errorf("Expected default APIEndpoint 'http://localhost:8080', got '%s'", cfg.APIEndpoint)
	}

	if cfg.APIKey != "" {
		t.Errorf("Expected default APIKey to be empty, got '%s'", cfg.APIKey)
	}
}

func TestParseArgs_APIEndpointRequiresArgument(t *testing.T) {
	args := []string{"--api-endpoint"}
	_, err := ParseArgs(args)

	if err == nil {
		t.Fatal("Expected error for --api-endpoint without argument")
	}

	expectedMsg := "--api-endpoint requires an argument"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestLoadEnv_GitHubTokenWithConfigFile(t *testing.T) {
	// Set up environment with config file + GITHUB_TOKEN
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.txt")

	configContent := `api_endpoint=https://api.githubcopilot.com
model=gpt-4o`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	os.Setenv("GITHUB_TOKEN", "ghu_test_token")
	defer os.Unsetenv("GITHUB_TOKEN")

	cfg := DefaultConfig()
	cfg.ConfigFile = configPath
	err = loadConfigFile(configPath, cfg)
	if err != nil {
		t.Fatalf("loadConfigFile() error: %v", err)
	}

	// API key should come from GITHUB_TOKEN
	loadEnv(cfg)

	if cfg.APIKey != "ghu_test_token" {
		t.Errorf("Expected APIKey 'ghu_test_token', got '%s'", cfg.APIKey)
	}
}
