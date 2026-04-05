package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Model != "llama3" {
		t.Errorf("Expected default model 'llama3', got '%s'", cfg.Model)
	}
	if cfg.Temperature != 0.7 {
		t.Errorf("Expected default temperature 0.7, got %f", cfg.Temperature)
	}
	if cfg.MaxTokens != 4096 {
		t.Errorf("Expected default max tokens 4096, got %d", cfg.MaxTokens)
	}
	if cfg.ContextSize != 128000 {
		t.Errorf("Expected default context size 128000, got %d", cfg.ContextSize)
	}
	if cfg.Streaming != true {
		t.Errorf("Expected default streaming true, got %v", cfg.Streaming)
	}
	if cfg.InitialTokenTimeout != 7200 {
		t.Errorf("Expected default timeout 7200, got %d", cfg.InitialTokenTimeout)
	}
	if cfg.MaxIterations != 1000 {
		t.Errorf("Expected default max iterations 1000, got %d", cfg.MaxIterations)
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		check   func(*Config) bool
	}{
		{
			name:    "empty args",
			args:    []string{},
			wantErr: false,
			check:   func(c *Config) bool { return c.Model == "llama3" },
		},
		{
			name:    "help flag",
			args:    []string{"--help"},
			wantErr: false,
			check:   func(c *Config) bool { return c.ShowHelp },
		},
		{
			name:    "version flag",
			args:    []string{"--version"},
			wantErr: false,
			check:   func(c *Config) bool { return c.ShowVersion },
		},
		{
			name:    "prompt flag",
			args:    []string{"--prompt", "test prompt"},
			wantErr: false,
			check:   func(c *Config) bool { return c.Prompt == "test prompt" },
		},
		{
			name:    "prompt short flag",
			args:    []string{"-p", "short prompt"},
			wantErr: false,
			check:   func(c *Config) bool { return c.Prompt == "short prompt" },
		},
		{
			name:    "stdin flag",
			args:    []string{"--stdin"},
			wantErr: false,
			check:   func(c *Config) bool { return c.UseStdin },
		},
		{
			name:    "model flag",
			args:    []string{"--model", "custom-model"},
			wantErr: false,
			check:   func(c *Config) bool { return c.Model == "custom-model" },
		},
		{
			name:    "temperature flag",
			args:    []string{"--temperature", "0.9"},
			wantErr: false,
			check:   func(c *Config) bool { return c.Temperature == 0.9 },
		},
		{
			name:    "max-tokens flag",
			args:    []string{"--max-tokens", "8192"},
			wantErr: false,
			check:   func(c *Config) bool { return c.MaxTokens == 8192 },
		},
		{
			name:    "context-size flag",
			args:    []string{"--context-size", "65536"},
			wantErr: false,
			check:   func(c *Config) bool { return c.ContextSize == 65536 },
		},
		{
			name:    "max-iterations flag",
			args:    []string{"--max-iterations", "500"},
			wantErr: false,
			check:   func(c *Config) bool { return c.MaxIterations == 500 },
		},
		{
			name:    "no-stream flag",
			args:    []string{"--no-stream"},
			wantErr: false,
			check:   func(c *Config) bool { return !c.Streaming },
		},
		{
			name:    "verbose flag",
			args:    []string{"--verbose"},
			wantErr: false,
			check:   func(c *Config) bool { return c.Verbose },
		},
		{
			name:    "quiet flag",
			args:    []string{"--quiet"},
			wantErr: false,
			check:   func(c *Config) bool { return c.Quiet },
		},
		{
			name:    "output flag",
			args:    []string{"--output", "result.txt"},
			wantErr: false,
			check:   func(c *Config) bool { return c.OutputFile == "result.txt" },
		},
		{
			name:    "unknown flag",
			args:    []string{"--unknown"},
			wantErr: true,
			check:   nil,
		},
		{
			name:    "prompt without value",
			args:    []string{"--prompt"},
			wantErr: true,
			check:   nil,
		},
		{
			name:    "invalid temperature",
			args:    []string{"--temperature", "invalid"},
			wantErr: true,
			check:   nil,
		},
		{
			name:    "invalid max-tokens",
			args:    []string{"--max-tokens", "not-a-number"},
			wantErr: true,
			check:   nil,
		},
		{
			name:    "invalid max-iterations",
			args:    []string{"--max-iterations", "not-a-number"},
			wantErr: true,
			check:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseArgs(tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && tt.check != nil && !tt.check(cfg) {
				t.Errorf("ParseArgs() config validation failed")
			}
		})
	}
}

func TestLoadEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("CODING_AGENT_MODEL", "env-model")
	os.Setenv("CODING_AGENT_TEMPERATURE", "0.8")
	os.Setenv("CODING_AGENT_MAX_TOKENS", "2048")
	os.Setenv("CODING_AGENT_MAX_ITERATIONS", "500")
	os.Setenv("CODING_AGENT_CONTEXT_SIZE", "32768")
	os.Setenv("CODING_AGENT_API_ENDPOINT", "http://env-endpoint")
	os.Setenv("CODING_AGENT_API_KEY", "env-key")
	os.Setenv("CODING_AGENT_INITIAL_TOKEN_TIMEOUT", "3600")
	os.Setenv("CODING_AGENT_STREAMING", "false")

	cfg := DefaultConfig()
	loadEnv(cfg)

	if cfg.Model != "env-model" {
		t.Errorf("Expected model 'env-model', got '%s'", cfg.Model)
	}
	if cfg.Temperature != 0.8 {
		t.Errorf("Expected temperature 0.8, got %f", cfg.Temperature)
	}
	if cfg.MaxTokens != 2048 {
		t.Errorf("Expected max tokens 2048, got %d", cfg.MaxTokens)
	}
	if cfg.MaxIterations != 500 {
		t.Errorf("Expected max iterations 500, got %d", cfg.MaxIterations)
	}
	if cfg.ContextSize != 32768 {
		t.Errorf("Expected context size 32768, got %d", cfg.ContextSize)
	}
	if cfg.APIEndpoint != "http://env-endpoint" {
		t.Errorf("Expected API endpoint 'http://env-endpoint', got '%s'", cfg.APIEndpoint)
	}
	if cfg.APIKey != "env-key" {
		t.Errorf("Expected API key 'env-key', got '%s'", cfg.APIKey)
	}
	if cfg.InitialTokenTimeout != 3600 {
		t.Errorf("Expected timeout 3600, got %d", cfg.InitialTokenTimeout)
	}
	if cfg.Streaming != false {
		t.Errorf("Expected streaming false, got %v", cfg.Streaming)
	}

	// Clean up
	os.Unsetenv("CODING_AGENT_MODEL")
	os.Unsetenv("CODING_AGENT_TEMPERATURE")
	os.Unsetenv("CODING_AGENT_MAX_TOKENS")
	os.Unsetenv("CODING_AGENT_MAX_ITERATIONS")
	os.Unsetenv("CODING_AGENT_CONTEXT_SIZE")
	os.Unsetenv("CODING_AGENT_API_ENDPOINT")
	os.Unsetenv("CODING_AGENT_API_KEY")
	os.Unsetenv("CODING_AGENT_INITIAL_TOKEN_TIMEOUT")
	os.Unsetenv("CODING_AGENT_STREAMING")
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &Config{
				ContextSize:         128000,
				InitialTokenTimeout: 7200,
				MaxIterations:       1000,
			},
			wantErr: false,
		},
		{
			name: "invalid context size",
			cfg: &Config{
				ContextSize:         -100,
				InitialTokenTimeout: 7200,
				MaxIterations:       1000,
			},
			wantErr: true,
		},
		{
			name: "invalid timeout",
			cfg: &Config{
				ContextSize:         128000,
				InitialTokenTimeout: 5,
				MaxIterations:       1000,
			},
			wantErr: true,
		},
		{
			name: "invalid max iterations",
			cfg: &Config{
				ContextSize:         128000,
				InitialTokenTimeout: 7200,
				MaxIterations:       0,
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
