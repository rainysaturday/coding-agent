package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds all configuration for the coding agent
type Config struct {
	// Inference backend configuration
	InferenceEndpoint string `json:"inference_endpoint"`
	APIKey            string `json:"api_key"`
	Model             string `json:"model"`

	// Context configuration
	ContextSize int `json:"context_size"`

	// Streaming configuration
	InitialTokenTimeout int  `json:"initial_token_timeout"`
	StreamingEnabled    bool `json:"streaming_enabled"`

	// Connection configuration
	ConnectionTimeout int `json:"connection_timeout"`
	ReadTimeout       int `json:"read_timeout"`

	// Iteration configuration
	MaxIterations int `json:"max_iterations"`

	// Config file path
	ConfigFile string `json:"-"`
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	return &Config{
		InferenceEndpoint:   "http://localhost:8080/v1",
		APIKey:              "not-needed",
		Model:               "llama-cpp",
		ContextSize:         128000,
		InitialTokenTimeout: 7200, // 2 hours
		StreamingEnabled:    true,
		ConnectionTimeout:   30,
		ReadTimeout:         300,
		MaxIterations:       50,
		ConfigFile:          defaultConfigPath(),
	}
}

func defaultConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".coding-agent-config.json"
	}
	return filepath.Join(homeDir, ".coding-agent-config.json")
}

// Load loads configuration from file, environment variables, and command-line flags
func Load(configFile string) (*Config, error) {
	cfg := DefaultConfig()

	// Load from config file if exists
	if configFile != "" {
		cfg.ConfigFile = configFile
		if err := cfg.loadFromFile(); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	} else if cfg.ConfigFile != "" {
		if err := cfg.loadFromFile(); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	// Override with environment variables
	cfg.loadFromEnv()

	return cfg, nil
}

func (c *Config) loadFromFile() error {
	data, err := os.ReadFile(c.ConfigFile)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, c)
}

func (c *Config) loadFromEnv() {
	if v := os.Getenv("CODING_AGENT_ENDPOINT"); v != "" {
		c.InferenceEndpoint = v
	}
	if v := os.Getenv("CODING_AGENT_API_KEY"); v != "" {
		c.APIKey = v
	}
	if v := os.Getenv("CODING_AGENT_MODEL"); v != "" {
		c.Model = v
	}
	if v := os.Getenv("CODING_AGENT_CONTEXT_SIZE"); v != "" {
		if size, err := strconv.Atoi(v); err == nil && size > 0 {
			c.ContextSize = size
		}
	}
	if v := os.Getenv("CODING_AGENT_INITIAL_TOKEN_TIMEOUT"); v != "" {
		if timeout, err := strconv.Atoi(v); err == nil && timeout >= 10 {
			c.InitialTokenTimeout = timeout
		}
	}
	if v := os.Getenv("CODING_AGENT_STREAMING"); v != "" {
		c.StreamingEnabled = strings.ToLower(v) != "false"
	}
	if v := os.Getenv("CODING_AGENT_CONNECTION_TIMEOUT"); v != "" {
		if timeout, err := strconv.Atoi(v); err == nil && timeout > 0 {
			c.ConnectionTimeout = timeout
		}
	}
	if v := os.Getenv("CODING_AGENT_READ_TIMEOUT"); v != "" {
		if timeout, err := strconv.Atoi(v); err == nil && timeout > 0 {
			c.ReadTimeout = timeout
		}
	}
	if v := os.Getenv("CODING_AGENT_MAX_ITERATIONS"); v != "" {
		if max, err := strconv.Atoi(v); err == nil && max > 0 {
			c.MaxIterations = max
		}
	}
}

// Save saves the configuration to the config file
func (c *Config) Save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.ConfigFile, data, 0644)
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.ContextSize <= 0 {
		return &ConfigError{"context_size must be positive"}
	}
	if c.InitialTokenTimeout < 10 {
		return &ConfigError{"initial_token_timeout must be at least 10 seconds"}
	}
	if c.ConnectionTimeout <= 0 {
		return &ConfigError{"connection_timeout must be positive"}
	}
	if c.ReadTimeout <= 0 {
		return &ConfigError{"read_timeout must be positive"}
	}
	if c.MaxIterations <= 0 {
		return &ConfigError{"max_iterations must be positive"}
	}
	if c.InferenceEndpoint == "" {
		return &ConfigError{"inference_endpoint cannot be empty"}
	}
	return nil
}

// ConfigError represents a configuration error
type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return "config error: " + e.Message
}
