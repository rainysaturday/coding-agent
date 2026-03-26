package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ContextSize != 128000 {
		t.Errorf("Expected default context size 128000, got %d", cfg.ContextSize)
	}
	if cfg.InitialTokenTimeout != 7200 {
		t.Errorf("Expected default initial token timeout 7200, got %d", cfg.InitialTokenTimeout)
	}
	if !cfg.StreamingEnabled {
		t.Error("Expected streaming to be enabled by default")
	}
	if cfg.MaxIterations != 50 {
		t.Errorf("Expected default max iterations 50, got %d", cfg.MaxIterations)
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("CODING_AGENT_CONTEXT_SIZE", "65536")
	os.Setenv("CODING_AGENT_INITIAL_TOKEN_TIMEOUT", "3600")
	os.Setenv("CODING_AGENT_STREAMING", "false")
	os.Setenv("CODING_AGENT_MAX_ITERATIONS", "25")
	os.Setenv("CODING_AGENT_ENDPOINT", "http://test:1234/v1")

	// Clear config file path to avoid loading existing config
	cfg := DefaultConfig()
	cfg.ConfigFile = "/nonexistent/config.json"
	cfg.loadFromEnv()

	if cfg.ContextSize != 65536 {
		t.Errorf("Expected context size 65536, got %d", cfg.ContextSize)
	}
	if cfg.InitialTokenTimeout != 3600 {
		t.Errorf("Expected initial token timeout 3600, got %d", cfg.InitialTokenTimeout)
	}
	if cfg.StreamingEnabled {
		t.Error("Expected streaming to be disabled")
	}
	if cfg.MaxIterations != 25 {
		t.Errorf("Expected max iterations 25, got %d", cfg.MaxIterations)
	}
	if cfg.InferenceEndpoint != "http://test:1234/v1" {
		t.Errorf("Expected endpoint http://test:1234/v1, got %s", cfg.InferenceEndpoint)
	}

	// Cleanup
	os.Unsetenv("CODING_AGENT_CONTEXT_SIZE")
	os.Unsetenv("CODING_AGENT_INITIAL_TOKEN_TIMEOUT")
	os.Unsetenv("CODING_AGENT_STREAMING")
	os.Unsetenv("CODING_AGENT_MAX_ITERATIONS")
	os.Unsetenv("CODING_AGENT_ENDPOINT")
}

func TestLoadFromEnvInvalidValues(t *testing.T) {
	// Set invalid environment variables
	os.Setenv("CODING_AGENT_CONTEXT_SIZE", "-100")
	os.Setenv("CODING_AGENT_INITIAL_TOKEN_TIMEOUT", "5") // Below minimum of 10

	cfg := DefaultConfig()
	cfg.ConfigFile = "/nonexistent/config.json"
	cfg.loadFromEnv()

	// Invalid values should be ignored, defaults should remain
	if cfg.ContextSize != 128000 {
		t.Errorf("Expected default context size 128000, got %d (invalid value should be ignored)", cfg.ContextSize)
	}
	if cfg.InitialTokenTimeout != 7200 {
		t.Errorf("Expected default initial token timeout 7200, got %d (invalid value should be ignored)", cfg.InitialTokenTimeout)
	}

	// Cleanup
	os.Unsetenv("CODING_AGENT_CONTEXT_SIZE")
	os.Unsetenv("CODING_AGENT_INITIAL_TOKEN_TIMEOUT")
}

func TestLoadFromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config.json")

	configData := `{
		"context_size": 32000,
		"initial_token_timeout": 1800,
		"streaming_enabled": false,
		"max_iterations": 10,
		"inference_endpoint": "http://custom:9999/v1"
	}`

	err := os.WriteFile(configFile, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.ContextSize != 32000 {
		t.Errorf("Expected context size 32000, got %d", cfg.ContextSize)
	}
	if cfg.InitialTokenTimeout != 1800 {
		t.Errorf("Expected initial token timeout 1800, got %d", cfg.InitialTokenTimeout)
	}
	if cfg.StreamingEnabled {
		t.Error("Expected streaming to be disabled")
	}
	if cfg.MaxIterations != 10 {
		t.Errorf("Expected max iterations 10, got %d", cfg.MaxIterations)
	}
	if cfg.InferenceEndpoint != "http://custom:9999/v1" {
		t.Errorf("Expected endpoint http://custom:9999/v1, got %s", cfg.InferenceEndpoint)
	}
}

func TestSave(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config-save.json")

	cfg := DefaultConfig()
	cfg.ConfigFile = configFile
	cfg.ContextSize = 65536

	err := cfg.Save()
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Load the saved config
	loadedCfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if loadedCfg.ContextSize != 65536 {
		t.Errorf("Expected saved context size 65536, got %d", loadedCfg.ContextSize)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "valid config",
			cfg:     DefaultConfig(),
			wantErr: false,
		},
		{
			name: "invalid context size",
			cfg: &Config{
				ContextSize: -100,
			},
			wantErr: true,
		},
		{
			name: "invalid initial token timeout",
			cfg: &Config{
				ContextSize:         128000,
				InitialTokenTimeout: 5,
			},
			wantErr: true,
		},
		{
			name: "empty endpoint",
			cfg: &Config{
				ContextSize:         128000,
				InitialTokenTimeout: 7200,
				InferenceEndpoint:   "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigError(t *testing.T) {
	err := &ConfigError{Message: "test error"}
	if err.Error() != "config error: test error" {
		t.Errorf("Expected 'config error: test error', got '%s'", err.Error())
	}
}
