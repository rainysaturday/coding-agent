// Package config handles configuration for the coding agent.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration for the agent.
type Config struct {
	// Mode
	Prompt       string
	PromptFile   string
	UseStdin     bool
	ShowHelp     bool
	ShowVersion  bool
	ConfigFile   string

	// Inference settings
	Model        string
	Temperature  float64
	MaxTokens    int
	ContextSize  int
	Streaming    bool

	// API settings
	APIEndpoint  string
	APIKey       string

	// Output settings
	Verbose      bool
	Quiet        bool
	OutputFile   string

	// Timeout settings (in seconds)
	InitialTokenTimeout int
	ConnectionTimeout   int
	ReadTimeout         int
}

// DefaultConfig returns a config with default values.
func DefaultConfig() *Config {
	return &Config{
		Model:               "llama3",
		Temperature:         0.7,
		MaxTokens:           4096,
		ContextSize:         128000,
		Streaming:           true,
		InitialTokenTimeout: 7200, // 2 hours default
		ConnectionTimeout:   30,   // 30 seconds for connection
		ReadTimeout:         300,  // 5 minutes for reading response
		APIEndpoint:         "http://localhost:8080", // llama.cpp default
	}
}

// ParseArgs parses command-line arguments and returns a Config.
func ParseArgs(args []string) (*Config, error) {
	cfg := DefaultConfig()

	// First pass: check for config file flag
	for i := 0; i < len(args); i++ {
		if args[i] == "--config" && i+1 < len(args) {
			cfg.ConfigFile = args[i+1]
			break
		}
	}

	// Load config file if specified
	if cfg.ConfigFile != "" {
		if err := loadConfigFile(cfg.ConfigFile, cfg); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Load from environment variables (overrides config file)
	loadEnv(cfg)

	// Parse command-line arguments (overrides env and config file)
	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch arg {
		case "-h", "--help":
			cfg.ShowHelp = true
		case "-v", "--version":
			cfg.ShowVersion = true
		case "-p", "--prompt":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--prompt requires an argument")
			}
			i++
			cfg.Prompt = args[i]
		case "--stdin":
			cfg.UseStdin = true
		case "--prompt-file":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--prompt-file requires an argument")
			}
			i++
			cfg.PromptFile = args[i]
		case "--config":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--config requires an argument")
			}
			i++
			cfg.ConfigFile = args[i]
		case "--model":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--model requires an argument")
			}
			i++
			cfg.Model = args[i]
		case "--temperature":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--temperature requires an argument")
			}
			i++
			temp, err := strconv.ParseFloat(args[i], 64)
			if err != nil {
				return nil, fmt.Errorf("invalid temperature: %v", err)
			}
			cfg.Temperature = temp
		case "--max-tokens":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--max-tokens requires an argument")
			}
			i++
			maxTokens, err := strconv.Atoi(args[i])
			if err != nil {
				return nil, fmt.Errorf("invalid max-tokens: %v", err)
			}
			cfg.MaxTokens = maxTokens
		case "--context-size":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--context-size requires an argument")
			}
			i++
			ctxSize, err := strconv.Atoi(args[i])
			if err != nil {
				return nil, fmt.Errorf("invalid context-size: %v", err)
			}
			cfg.ContextSize = ctxSize
		case "--no-stream":
			cfg.Streaming = false
		case "--connection-timeout":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--connection-timeout requires an argument")
			}
			i++
			connTimeout, err := strconv.Atoi(args[i])
			if err != nil {
				return nil, fmt.Errorf("invalid connection-timeout: %v", err)
			}
			cfg.ConnectionTimeout = connTimeout
		case "--read-timeout":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--read-timeout requires an argument")
			}
			i++
			readTimeout, err := strconv.Atoi(args[i])
			if err != nil {
				return nil, fmt.Errorf("invalid read-timeout: %v", err)
			}
			cfg.ReadTimeout = readTimeout
		case "--verbose":
			cfg.Verbose = true
		case "--quiet":
			cfg.Quiet = true
		case "--output":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--output requires an argument")
			}
			i++
			cfg.OutputFile = args[i]
		default:
			if strings.HasPrefix(arg, "-") {
				return nil, fmt.Errorf("unknown flag: %s", arg)
			}
		}
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// loadConfigFile loads configuration from a config file.
// Supports simple KEY=VALUE format (one per line, # for comments).
func loadConfigFile(path string, cfg *Config) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		value = strings.Trim(value, "\"'")

		switch key {
		case "model":
			cfg.Model = value
		case "temperature":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				cfg.Temperature = v
			}
		case "max_tokens":
			if v, err := strconv.Atoi(value); err == nil {
				cfg.MaxTokens = v
			}
		case "context_size":
			if v, err := strconv.Atoi(value); err == nil {
				cfg.ContextSize = v
			}
		case "streaming":
			cfg.Streaming = value != "false" && value != "0"
		case "api_endpoint":
			cfg.APIEndpoint = value
		case "api_key":
			cfg.APIKey = value
		case "initial_token_timeout":
			if v, err := strconv.Atoi(value); err == nil {
				cfg.InitialTokenTimeout = v
			}
		case "connection_timeout":
			if v, err := strconv.Atoi(value); err == nil {
				cfg.ConnectionTimeout = v
			}
		case "read_timeout":
			if v, err := strconv.Atoi(value); err == nil {
				cfg.ReadTimeout = v
			}
		case "verbose":
			cfg.Verbose = value == "true" || value == "1"
		case "quiet":
			cfg.Quiet = value == "true" || value == "1"
		}
	}

	return nil
}

// loadEnv loads configuration from environment variables.
func loadEnv(cfg *Config) {
	if val := os.Getenv("CODING_AGENT_MODEL"); val != "" {
		cfg.Model = val
	}
	if val := os.Getenv("CODING_AGENT_TEMPERATURE"); val != "" {
		if temp, err := strconv.ParseFloat(val, 64); err == nil {
			cfg.Temperature = temp
		}
	}
	if val := os.Getenv("CODING_AGENT_MAX_TOKENS"); val != "" {
		if maxTokens, err := strconv.Atoi(val); err == nil {
			cfg.MaxTokens = maxTokens
		}
	}
	if val := os.Getenv("CODING_AGENT_CONTEXT_SIZE"); val != "" {
		if ctxSize, err := strconv.Atoi(val); err == nil {
			cfg.ContextSize = ctxSize
		}
	}
	if val := os.Getenv("CODING_AGENT_API_ENDPOINT"); val != "" {
		cfg.APIEndpoint = val
	}
	if val := os.Getenv("CODING_AGENT_API_KEY"); val != "" {
		cfg.APIKey = val
	}
	if val := os.Getenv("CODING_AGENT_INITIAL_TOKEN_TIMEOUT"); val != "" {
		if timeout, err := strconv.Atoi(val); err == nil {
			cfg.InitialTokenTimeout = timeout
		}
	}
	if val := os.Getenv("CODING_AGENT_CONNECTION_TIMEOUT"); val != "" {
		if timeout, err := strconv.Atoi(val); err == nil {
			cfg.ConnectionTimeout = timeout
		}
	}
	if val := os.Getenv("CODING_AGENT_READ_TIMEOUT"); val != "" {
		if timeout, err := strconv.Atoi(val); err == nil {
			cfg.ReadTimeout = timeout
		}
	}
	// Streaming can be disabled via env var
	if val := os.Getenv("CODING_AGENT_STREAMING"); val != "" {
		if val == "false" || val == "0" {
			cfg.Streaming = false
		}
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.ContextSize <= 0 {
		return fmt.Errorf("context size must be positive")
	}
	if c.InitialTokenTimeout < 10 {
		return fmt.Errorf("initial token timeout must be at least 10 seconds")
	}
	// ConnectionTimeout and ReadTimeout have defaults, only validate if explicitly set (non-zero)
	if c.ConnectionTimeout != 0 && c.ConnectionTimeout < 5 {
		return fmt.Errorf("connection timeout must be at least 5 seconds")
	}
	if c.ReadTimeout != 0 && c.ReadTimeout < 10 {
		return fmt.Errorf("read timeout must be at least 10 seconds")
	}
	return nil
}
