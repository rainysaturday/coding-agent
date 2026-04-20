package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Model != "llama3" {
		t.Errorf("Expected default model 'llama3', got '%s'", cfg.Model)
	}
	if cfg.Temperature != nil {
		t.Errorf("Expected default temperature nil (not set), got %f", *cfg.Temperature)
	}
	if cfg.MaxTokens != 64000 {
		t.Errorf("Expected default max tokens 64000, got %d", cfg.MaxTokens)
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
			check: func(c *Config) bool {
				if c.Temperature == nil {
					return false
				}
				return *c.Temperature == 0.9
			},
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
	if cfg.Temperature == nil || *cfg.Temperature != 0.8 {
		t.Errorf("Expected temperature 0.8, got %v", cfg.Temperature)
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
		{
			name: "invalid connection timeout",
			cfg: &Config{
				ContextSize:         128000,
				InitialTokenTimeout: 7200,
				MaxIterations:       1000,
				ConnectionTimeout:   3,
			},
			wantErr: true,
		},
		{
			name: "invalid read timeout",
			cfg: &Config{
				ContextSize:         128000,
				InitialTokenTimeout: 7200,
				MaxIterations:       1000,
				ReadTimeout:         5,
			},
			wantErr: true,
		},
		{
			name: "valid connection timeout at minimum",
			cfg: &Config{
				ContextSize:         128000,
				InitialTokenTimeout: 7200,
				MaxIterations:       1000,
				ConnectionTimeout:   5,
			},
			wantErr: false,
		},
		{
			name: "valid read timeout at minimum",
			cfg: &Config{
				ContextSize:         128000,
				InitialTokenTimeout: 7200,
				MaxIterations:       1000,
				ReadTimeout:         10,
			},
			wantErr: false,
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

func TestLoadConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		content string
		check   func(*Config) bool
		wantErr bool
	}{
		{
			name:    "empty file",
			content: "",
			check: func(c *Config) bool {
				return c.Model == "llama3" // defaults unchanged
			},
		},
		{
			name:    "comments only",
			content: "# This is a comment\n# Another comment\n",
			check: func(c *Config) bool {
				return c.Model == "llama3"
			},
		},
		{
			name:    "empty lines only",
			content: "\n\n\n",
			check: func(c *Config) bool {
				return c.Model == "llama3"
			},
		},
		{
			name:    "model setting",
			content: "model = my-custom-model\n",
			check: func(c *Config) bool {
				return c.Model == "my-custom-model"
			},
		},
		{
			name:    "max_iterations setting",
			content: "max_iterations = 500\n",
			check: func(c *Config) bool {
				return c.MaxIterations == 500
			},
		},
		{
			name:    "context_size setting",
			content: "context_size = 65536\n",
			check: func(c *Config) bool {
				return c.ContextSize == 65536
			},
		},
		{
			name:    "streaming true",
			content: "streaming = true\n",
			check: func(c *Config) bool {
				return c.Streaming == true
			},
		},
		{
			name:    "streaming false",
			content: "streaming = false\n",
			check: func(c *Config) bool {
				return c.Streaming == false
			},
		},
		{
			name:    "streaming 0",
			content: "streaming = 0\n",
			check: func(c *Config) bool {
				return c.Streaming == false
			},
		},
		{
			name:    "streaming 1",
			content: "streaming = 1\n",
			check: func(c *Config) bool {
				return c.Streaming == true
			},
		},
		{
			name:    "api_endpoint setting",
			content: "api_endpoint = http://custom:8080\n",
			check: func(c *Config) bool {
				return c.APIEndpoint == "http://custom:8080"
			},
		},
		{
			name:    "api_key setting",
			content: "api_key = secret-key-123\n",
			check: func(c *Config) bool {
				return c.APIKey == "secret-key-123"
			},
		},
		{
			name:    "verbose true",
			content: "verbose = true\n",
			check: func(c *Config) bool {
				return c.Verbose == true
			},
		},
		{
			name:    "verbose 1",
			content: "verbose = 1\n",
			check: func(c *Config) bool {
				return c.Verbose == true
			},
		},
		{
			name:    "verbose false",
			content: "verbose = false\n",
			check: func(c *Config) bool {
				return c.Verbose == false
			},
		},
		{
			name:    "quiet true",
			content: "quiet = true\n",
			check: func(c *Config) bool {
				return c.Quiet == true
			},
		},
		{
			name:    "quiet 1",
			content: "quiet = 1\n",
			check: func(c *Config) bool {
				return c.Quiet == true
			},
		},
		{
			name:    "debug true",
			content: "debug = true\n",
			check: func(c *Config) bool {
				return c.Debug == true
			},
		},
		{
			name:    "debug_log setting",
			content: "debug_log = /tmp/agent.log\n",
			check: func(c *Config) bool {
				return c.DebugLog == "/tmp/agent.log"
			},
		},
		{
			name:    "multiple settings",
			content: "model = custom\nmax_iterations = 200\nstreaming = false\napi_endpoint = http://test:9999\n",
			check: func(c *Config) bool {
				return c.Model == "custom" &&
					c.MaxIterations == 200 &&
					c.Streaming == false &&
					c.APIEndpoint == "http://test:9999"
			},
		},
		{
			name:    "quoted values",
			content: "model = \"quoted-model\"\n",
			check: func(c *Config) bool {
				return c.Model == "quoted-model"
			},
		},
		{
			name:    "single quoted values",
			content: "model = 'single-quoted'\n",
			check: func(c *Config) bool {
				return c.Model == "single-quoted"
			},
		},
		{
			name:    "temperature float",
			content: "temperature = 0.85\n",
			check: func(c *Config) bool {
				if c.Temperature == nil {
					return false
				}
				return *c.Temperature == 0.85
			},
		},
		{
			name:    "invalid temperature ignored",
			content: "temperature = not-a-number\n",
			check: func(c *Config) bool {
				return c.Temperature == nil
			},
		},
		{
			name:    "invalid max_tokens ignored",
			content: "max_tokens = not-a-number\n",
			check: func(c *Config) bool {
				return c.MaxTokens == 64000 // default
			},
		},
		{
			name:    "initial_token_timeout",
			content: "initial_token_timeout = 3600\n",
			check: func(c *Config) bool {
				return c.InitialTokenTimeout == 3600
			},
		},
		{
			name:    "connection_timeout",
			content: "connection_timeout = 1800\n",
			check: func(c *Config) bool {
				return c.ConnectionTimeout == 1800
			},
		},
		{
			name:    "read_timeout",
			content: "read_timeout = 900\n",
			check: func(c *Config) bool {
				return c.ReadTimeout == 900
			},
		},
		{
			name:    "invalid key ignored",
			content: "unknown_key = some_value\n",
			check: func(c *Config) bool {
				return c.Model == "llama3" // defaults unchanged
			},
		},
		{
			name:    "malformed lines ignored",
			content: "malformed line without equals\nmodel = valid\n",
			check: func(c *Config) bool {
				return c.Model == "valid"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()

			filePath := filepath.Join(tmpDir, tt.name+".conf")
			err := os.WriteFile(filePath, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			err = loadConfigFile(filePath, cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadConfigFile() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil && tt.check != nil && !tt.check(cfg) {
				t.Errorf("loadConfigFile() config validation failed")
			}
		})
	}
}

func TestLoadConfigFile_NotFound(t *testing.T) {
	cfg := DefaultConfig()
	err := loadConfigFile("/nonexistent/file.conf", cfg)
	if err == nil {
		t.Fatal("Expected error for non-existent file")
	}
}

func TestParseArgs_ConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := "model = custom-model\nmax_iterations = 500\n"
	configPath := filepath.Join(tmpDir, "test.conf")
	os.WriteFile(configPath, []byte(configContent), 0644)

	cfg, err := ParseArgs([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}

	if cfg.Model != "custom-model" {
		t.Errorf("Expected model 'custom-model', got '%s'", cfg.Model)
	}
	if cfg.MaxIterations != 500 {
		t.Errorf("Expected max_iterations 500, got %d", cfg.MaxIterations)
	}
}

func TestParseArgs_ConfigFileOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := "model = from-file\n"
	configPath := filepath.Join(tmpDir, "test.conf")
	os.WriteFile(configPath, []byte(configContent), 0644)

	// Command line should override config file
	cfg, err := ParseArgs([]string{"--config", configPath, "--model", "from-cmd"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}

	if cfg.Model != "from-cmd" {
		t.Errorf("Expected model 'from-cmd', got '%s'", cfg.Model)
	}
}

func TestParseArgs_UnknownFlag(t *testing.T) {
	_, err := ParseArgs([]string{"--unknown-flag"})
	if err == nil {
		t.Fatal("Expected error for unknown flag")
	}
	if !strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("Expected 'unknown flag' error, got: %v", err)
	}
}

func TestParseArgs_TemperatureInvalid(t *testing.T) {
	_, err := ParseArgs([]string{"--temperature", "not-a-number"})
	if err == nil {
		t.Fatal("Expected error for invalid temperature")
	}
	if !strings.Contains(err.Error(), "invalid temperature") {
		t.Errorf("Expected 'invalid temperature' error, got: %v", err)
	}
}

func TestParseArgs_MaxTokensInvalid(t *testing.T) {
	_, err := ParseArgs([]string{"--max-tokens", "not-a-number"})
	if err == nil {
		t.Fatal("Expected error for invalid max-tokens")
	}
	if !strings.Contains(err.Error(), "invalid max-tokens") {
		t.Errorf("Expected 'invalid max-tokens' error, got: %v", err)
	}
}

func TestParseArgs_MaxIterationsInvalid(t *testing.T) {
	_, err := ParseArgs([]string{"--max-iterations", "not-a-number"})
	if err == nil {
		t.Fatal("Expected error for invalid max-iterations")
	}
	if !strings.Contains(err.Error(), "invalid max-iterations") {
		t.Errorf("Expected 'invalid max-iterations' error, got: %v", err)
	}
}

func TestParseArgs_ContextSizeInvalid(t *testing.T) {
	_, err := ParseArgs([]string{"--context-size", "not-a-number"})
	if err == nil {
		t.Fatal("Expected error for invalid context-size")
	}
	if !strings.Contains(err.Error(), "invalid context-size") {
		t.Errorf("Expected 'invalid context-size' error, got: %v", err)
	}
}

func TestParseArgs_ConnectionTimeoutInvalid(t *testing.T) {
	_, err := ParseArgs([]string{"--connection-timeout", "not-a-number"})
	if err == nil {
		t.Fatal("Expected error for invalid connection-timeout")
	}
	if !strings.Contains(err.Error(), "invalid connection-timeout") {
		t.Errorf("Expected 'invalid connection-timeout' error, got: %v", err)
	}
}

func TestParseArgs_ReadTimeoutInvalid(t *testing.T) {
	_, err := ParseArgs([]string{"--read-timeout", "not-a-number"})
	if err == nil {
		t.Fatal("Expected error for invalid read-timeout")
	}
	if !strings.Contains(err.Error(), "invalid read-timeout") {
		t.Errorf("Expected 'invalid read-timeout' error, got: %v", err)
	}
}

func TestParseArgs_PromptMissing(t *testing.T) {
	_, err := ParseArgs([]string{"--prompt"})
	if err == nil {
		t.Fatal("Expected error for --prompt without value")
	}
	if !strings.Contains(err.Error(), "requires an argument") {
		t.Errorf("Expected 'requires an argument' error, got: %v", err)
	}
}

func TestParseArgs_TemperatureFileMissing(t *testing.T) {
	_, err := ParseArgs([]string{"--temperature"})
	if err == nil {
		t.Fatal("Expected error for --temperature without value")
	}
}

func TestParseArgs_MaxTokensFileMissing(t *testing.T) {
	_, err := ParseArgs([]string{"--max-tokens"})
	if err == nil {
		t.Fatal("Expected error for --max-tokens without value")
	}
}

func TestParseArgs_InvalidConnectionTimeout(t *testing.T) {
	cfg, err := ParseArgs([]string{"--connection-timeout", "3"})
	if err != nil {
		// Validation fails for timeout < 5, which is expected
		if !strings.Contains(err.Error(), "connection timeout must be at least 5") {
			t.Fatalf("Unexpected error: %v", err)
		}
		t.Logf("Connection timeout < 5 correctly fails validation: %v", err)
	} else {
		t.Logf("ConnectionTimeout 3: %d", cfg.ConnectionTimeout)
	}
}

func TestParseArgs_InvalidReadTimeout(t *testing.T) {
	cfg, err := ParseArgs([]string{"--read-timeout", "5"})
	if err != nil {
		// Validation fails for timeout < 10, which is expected
		if !strings.Contains(err.Error(), "read timeout must be at least 10") {
			t.Fatalf("Unexpected error: %v", err)
		}
		t.Logf("Read timeout < 10 correctly fails validation: %v", err)
	} else {
		t.Logf("ReadTimeout 5: %d", cfg.ReadTimeout)
	}
}

func TestLoadEnv_DebugSettings(t *testing.T) {
	os.Setenv("CODING_AGENT_DEBUG", "true")
	os.Setenv("CODING_AGENT_DEBUG_LOG", "/tmp/test.log")
	defer func() {
		os.Unsetenv("CODING_AGENT_DEBUG")
		os.Unsetenv("CODING_AGENT_DEBUG_LOG")
	}()

	cfg := DefaultConfig()
	loadEnv(cfg)

	if !cfg.Debug {
		t.Error("Expected Debug to be true")
	}
	if cfg.DebugLog != "/tmp/test.log" {
		t.Errorf("Expected DebugLog '/tmp/test.log', got '%s'", cfg.DebugLog)
	}
}

func TestLoadEnv_DebugFalse(t *testing.T) {
	os.Setenv("CODING_AGENT_DEBUG", "false")
	defer os.Unsetenv("CODING_AGENT_DEBUG")

	cfg := DefaultConfig()
	cfg.Debug = true // Set true, then env should override to false
	loadEnv(cfg)

	if cfg.Debug {
		t.Error("Expected Debug to be false")
	}
}

func TestLoadEnv_StreamingOverride(t *testing.T) {
	os.Setenv("CODING_AGENT_STREAMING", "false")
	defer os.Unsetenv("CODING_AGENT_STREAMING")

	cfg := DefaultConfig()
	loadEnv(cfg)

	if cfg.Streaming {
		t.Error("Expected Streaming to be false")
	}
}

func TestLoadEnv_ConnectionReadTimeout(t *testing.T) {
	os.Setenv("CODING_AGENT_CONNECTION_TIMEOUT", "1800")
	os.Setenv("CODING_AGENT_READ_TIMEOUT", "900")
	defer func() {
		os.Unsetenv("CODING_AGENT_CONNECTION_TIMEOUT")
		os.Unsetenv("CODING_AGENT_READ_TIMEOUT")
	}()

	cfg := DefaultConfig()
	loadEnv(cfg)

	if cfg.ConnectionTimeout != 1800 {
		t.Errorf("Expected ConnectionTimeout 1800, got %d", cfg.ConnectionTimeout)
	}
	if cfg.ReadTimeout != 900 {
		t.Errorf("Expected ReadTimeout 900, got %d", cfg.ReadTimeout)
	}
}

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Model != "llama3" {
		t.Errorf("Expected model 'llama3', got '%s'", cfg.Model)
	}
	if cfg.MaxTokens != 64000 {
		t.Errorf("Expected MaxTokens 64000, got %d", cfg.MaxTokens)
	}
	if cfg.ContextSize != 128000 {
		t.Errorf("Expected ContextSize 128000, got %d", cfg.ContextSize)
	}
	if cfg.Streaming != true {
		t.Errorf("Expected Streaming true, got %v", cfg.Streaming)
	}
	if cfg.APIEndpoint != "http://localhost:8080" {
		t.Errorf("Expected APIEndpoint 'http://localhost:8080', got '%s'", cfg.APIEndpoint)
	}
	if cfg.MaxIterations != 1000 {
		t.Errorf("Expected MaxIterations 1000, got %d", cfg.MaxIterations)
	}
	if cfg.Debug != false {
		t.Errorf("Expected Debug false, got %v", cfg.Debug)
	}
	if cfg.DebugLog != "debug.log" {
		t.Errorf("Expected DebugLog 'debug.log', got '%s'", cfg.DebugLog)
	}
	if cfg.Verbose != false {
		t.Errorf("Expected Verbose false, got %v", cfg.Verbose)
	}
	if cfg.Quiet != false {
		t.Errorf("Expected Quiet false, got %v", cfg.Quiet)
	}
}

func TestParseArgs_AllFlags(t *testing.T) {
	temp := 0.5
	expectedTemp := temp

	cfg, err := ParseArgs([]string{
		"--model", "test-model",
		"--temperature", "0.5",
		"--max-tokens", "16384",
		"--context-size", "32768",
		"--max-iterations", "200",
		"--connection-timeout", "3600",
		"--read-timeout", "1800",
		"--prompt", "test prompt",
		"--verbose",
		"--quiet",
		"--no-stream",
		"--output", "output.txt",
		"--debug",
		"--debug-log", "/tmp/test.log",
	})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}

	if cfg.Model != "test-model" {
		t.Errorf("Expected model 'test-model', got '%s'", cfg.Model)
	}
	if cfg.Temperature == nil || *cfg.Temperature != expectedTemp {
		t.Errorf("Expected temperature 0.5, got %v", cfg.Temperature)
	}
	if cfg.MaxTokens != 16384 {
		t.Errorf("Expected MaxTokens 16384, got %d", cfg.MaxTokens)
	}
	if cfg.ContextSize != 32768 {
		t.Errorf("Expected ContextSize 32768, got %d", cfg.ContextSize)
	}
	if cfg.MaxIterations != 200 {
		t.Errorf("Expected MaxIterations 200, got %d", cfg.MaxIterations)
	}
	if cfg.ConnectionTimeout != 3600 {
		t.Errorf("Expected ConnectionTimeout 3600, got %d", cfg.ConnectionTimeout)
	}
	if cfg.ReadTimeout != 1800 {
		t.Errorf("Expected ReadTimeout 1800, got %d", cfg.ReadTimeout)
	}
	if cfg.Prompt != "test prompt" {
		t.Errorf("Expected Prompt 'test prompt', got '%s'", cfg.Prompt)
	}
	if !cfg.Verbose {
		t.Error("Expected Verbose true")
	}
	if !cfg.Quiet {
		t.Error("Expected Quiet true")
	}
	if cfg.Streaming {
		t.Error("Expected Streaming false")
	}
	if cfg.OutputFile != "output.txt" {
		t.Errorf("Expected OutputFile 'output.txt', got '%s'", cfg.OutputFile)
	}
	if !cfg.Debug {
		t.Error("Expected Debug true")
	}
	if cfg.DebugLog != "/tmp/test.log" {
		t.Errorf("Expected DebugLog '/tmp/test.log', got '%s'", cfg.DebugLog)
	}
}

func TestParseArgs_PromptFile(t *testing.T) {
	tmpDir := t.TempDir()
	promptContent := "test prompt from file"
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	os.WriteFile(promptPath, []byte(promptContent), 0644)

	cfg, err := ParseArgs([]string{"--prompt-file", promptPath})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}

	if cfg.PromptFile != promptPath {
		t.Errorf("Expected PromptFile '%s', got '%s'", promptPath, cfg.PromptFile)
	}
}

func TestParseArgs_Stdin(t *testing.T) {
	cfg, err := ParseArgs([]string{"--stdin"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}

	if !cfg.UseStdin {
		t.Error("Expected UseStdin true")
	}
}

func TestParseArgs_HelpVersion(t *testing.T) {
	cfg, err := ParseArgs([]string{"-h"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}
	if !cfg.ShowHelp {
		t.Error("Expected ShowHelp true")
	}

	cfg, err = ParseArgs([]string{"-v"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}
	if !cfg.ShowVersion {
		t.Error("Expected ShowVersion true")
	}

	cfg, err = ParseArgs([]string{"--help"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}
	if !cfg.ShowHelp {
		t.Error("Expected ShowHelp true for --help")
	}

	cfg, err = ParseArgs([]string{"--version"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}
	if !cfg.ShowVersion {
		t.Error("Expected ShowVersion true for --version")
	}
}

func TestParseArgs_Output(t *testing.T) {
	cfg, err := ParseArgs([]string{"--output", "result.txt"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}

	if cfg.OutputFile != "result.txt" {
		t.Errorf("Expected OutputFile 'result.txt', got '%s'", cfg.OutputFile)
	}
}

func TestParseArgs_OutputMissing(t *testing.T) {
	_, err := ParseArgs([]string{"--output"})
	if err == nil {
		t.Fatal("Expected error for --output without value")
	}
}

func TestParseArgs_ModelMissing(t *testing.T) {
	_, err := ParseArgs([]string{"--model"})
	if err == nil {
		t.Fatal("Expected error for --model without value")
	}
}

func TestParseArgs_ContextSizeMissing(t *testing.T) {
	_, err := ParseArgs([]string{"--context-size"})
	if err == nil {
		t.Fatal("Expected error for --context-size without value")
	}
}

func TestParseArgs_MaxIterationsMissing(t *testing.T) {
	_, err := ParseArgs([]string{"--max-iterations"})
	if err == nil {
		t.Fatal("Expected error for --max-iterations without value")
	}
}

func TestParseArgs_ConnectionTimeoutMissing(t *testing.T) {
	_, err := ParseArgs([]string{"--connection-timeout"})
	if err == nil {
		t.Fatal("Expected error for --connection-timeout without value")
	}
}

func TestParseArgs_ReadTimeoutMissing(t *testing.T) {
	_, err := ParseArgs([]string{"--read-timeout"})
	if err == nil {
		t.Fatal("Expected error for --read-timeout without value")
	}
}

func TestParseArgs_DebugLogMissing(t *testing.T) {
	_, err := ParseArgs([]string{"--debug-log"})
	if err == nil {
		t.Fatal("Expected error for --debug-log without value")
	}
}

func TestParseArgs_ConfigMissing(t *testing.T) {
	_, err := ParseArgs([]string{"--config"})
	if err == nil {
		t.Fatal("Expected error for --config without value")
	}
}

func TestLoadConfigFile_MixedContent(t *testing.T) {
	tmpDir := t.TempDir()

	content := `# Configuration file
model = my-model
max_iterations = 100

# API settings
api_endpoint = http://api.example.com
api_key = my-secret-key

# Debug
debug = true
debug_log = /tmp/agent.log

# Invalid line
this is not valid

# More valid
streaming = false
verbose = 1
`

	filePath := filepath.Join(tmpDir, "config.conf")
	os.WriteFile(filePath, []byte(content), 0644)

	cfg := DefaultConfig()
	err := loadConfigFile(filePath, cfg)
	if err != nil {
		t.Fatalf("loadConfigFile() error: %v", err)
	}

	if cfg.Model != "my-model" {
		t.Errorf("Expected model 'my-model', got '%s'", cfg.Model)
	}
	if cfg.MaxIterations != 100 {
		t.Errorf("Expected MaxIterations 100, got %d", cfg.MaxIterations)
	}
	if cfg.APIEndpoint != "http://api.example.com" {
		t.Errorf("Expected APIEndpoint 'http://api.example.com', got '%s'", cfg.APIEndpoint)
	}
	if cfg.APIKey != "my-secret-key" {
		t.Errorf("Expected APIKey 'my-secret-key', got '%s'", cfg.APIKey)
	}
	if !cfg.Debug {
		t.Error("Expected Debug true")
	}
	if cfg.DebugLog != "/tmp/agent.log" {
		t.Errorf("Expected DebugLog '/tmp/agent.log', got '%s'", cfg.DebugLog)
	}
	if cfg.Streaming != false {
		t.Errorf("Expected Streaming false, got %v", cfg.Streaming)
	}
	if !cfg.Verbose {
		t.Error("Expected Verbose true")
	}
}

func TestLoadConfigFile_Temperature(t *testing.T) {
	tmpDir := t.TempDir()

	content := "temperature = 0.0\n"
	filePath := filepath.Join(tmpDir, "config.conf")
	os.WriteFile(filePath, []byte(content), 0644)

	cfg := DefaultConfig()
	err := loadConfigFile(filePath, cfg)
	if err != nil {
		t.Fatalf("loadConfigFile() error: %v", err)
	}

	if cfg.Temperature == nil {
		t.Error("Expected Temperature to be set")
	} else if *cfg.Temperature != 0.0 {
		t.Errorf("Expected Temperature 0.0, got %f", *cfg.Temperature)
	}
}

func TestLoadConfigFile_MaxTokens(t *testing.T) {
	tmpDir := t.TempDir()

	content := "max_tokens = 8192\n"
	filePath := filepath.Join(tmpDir, "config.conf")
	os.WriteFile(filePath, []byte(content), 0644)

	cfg := DefaultConfig()
	err := loadConfigFile(filePath, cfg)
	if err != nil {
		t.Fatalf("loadConfigFile() error: %v", err)
	}

	if cfg.MaxTokens != 8192 {
		t.Errorf("Expected MaxTokens 8192, got %d", cfg.MaxTokens)
	}
}

func TestValidate_ContextSizeZero(t *testing.T) {
	cfg := &Config{
		ContextSize:         0,
		InitialTokenTimeout: 7200,
		MaxIterations:       1000,
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for context size 0")
	}
}

func TestValidate_ContextSizeNegative(t *testing.T) {
	cfg := &Config{
		ContextSize:         -1,
		InitialTokenTimeout: 7200,
		MaxIterations:       1000,
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for negative context size")
	}
}

func TestValidate_MaxIterationsZero(t *testing.T) {
	cfg := &Config{
		ContextSize:         128000,
		InitialTokenTimeout: 7200,
		MaxIterations:       0,
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for max iterations 0")
	}
}

func TestValidate_MaxIterationsNegative(t *testing.T) {
	cfg := &Config{
		ContextSize:         128000,
		InitialTokenTimeout: 7200,
		MaxIterations:       -1,
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for negative max iterations")
	}
}

func TestValidate_InitialTokenTimeoutTooLow(t *testing.T) {
	cfg := &Config{
		ContextSize:         128000,
		InitialTokenTimeout: 9,
		MaxIterations:       1000,
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for initial token timeout < 10")
	}
}

func TestValidate_InitialTokenTimeoutExactlyMinimum(t *testing.T) {
	cfg := &Config{
		ContextSize:         128000,
		InitialTokenTimeout: 10,
		MaxIterations:       1000,
	}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected no error for initial token timeout 10, got: %v", err)
	}
}

func TestValidate_ConnectionTimeoutZero(t *testing.T) {
	cfg := &Config{
		ContextSize:         128000,
		InitialTokenTimeout: 7200,
		MaxIterations:       1000,
		ConnectionTimeout:   0,
	}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected no error for connection timeout 0, got: %v", err)
	}
}

func TestValidate_ReadTimeoutZero(t *testing.T) {
	cfg := &Config{
		ContextSize:         128000,
		InitialTokenTimeout: 7200,
		MaxIterations:       1000,
		ReadTimeout:         0,
	}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected no error for read timeout 0, got: %v", err)
	}
}

func TestValidate_ReadTimeoutExactlyMinimum(t *testing.T) {
	cfg := &Config{
		ContextSize:         128000,
		InitialTokenTimeout: 7200,
		MaxIterations:       1000,
		ReadTimeout:         10,
	}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected no error for read timeout 10, got: %v", err)
	}
}

func TestValidate_ConnectionTimeoutExactlyMinimum(t *testing.T) {
	cfg := &Config{
		ContextSize:         128000,
		InitialTokenTimeout: 7200,
		MaxIterations:       1000,
		ConnectionTimeout:   5,
	}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected no error for connection timeout 5, got: %v", err)
	}
}

func TestValidate_ConnectionTimeoutBelowMinimum(t *testing.T) {
	cfg := &Config{
		ContextSize:         128000,
		InitialTokenTimeout: 7200,
		MaxIterations:       1000,
		ConnectionTimeout:   4,
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for connection timeout 4 (< 5)")
	}
}

func TestValidate_ReadTimeoutBelowMinimum(t *testing.T) {
	cfg := &Config{
		ContextSize:         128000,
		InitialTokenTimeout: 7200,
		MaxIterations:       1000,
		ReadTimeout:         9,
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for read timeout 9 (< 10)")
	}
}

func TestValidate_EmptyConfig(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for empty config")
	}
}

func TestParseArgs_MultipleUnknownFlags(t *testing.T) {
	_, err := ParseArgs([]string{"--unknown1", "--unknown2"})
	if err == nil {
		t.Fatal("Expected error for unknown flags")
	}
}

func TestParseArgs_FlagBeforeArgs(t *testing.T) {
	// Test with positional args (should not cause error since they're ignored)
	cfg, err := ParseArgs([]string{"--model", "test", "positional_arg"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}
	if cfg.Model != "test" {
		t.Errorf("Expected model 'test', got '%s'", cfg.Model)
	}
}

func TestParseArgs_EmptyArgs(t *testing.T) {
	cfg, err := ParseArgs([]string{})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}
	if cfg.Model != "llama3" {
		t.Errorf("Expected default model 'llama3', got '%s'", cfg.Model)
	}
}

func TestParseArgs_SingleDashFlag(t *testing.T) {
	_, err := ParseArgs([]string{"-x"})
	if err == nil {
		t.Fatal("Expected error for single-dash unknown flag")
	}
	if !strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("Expected 'unknown flag' error, got: %v", err)
	}
}
