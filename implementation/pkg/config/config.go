package config

import (
	"os"
	"strconv"
)

// Config holds all configuration for the coding agent harness
type Config struct {
	// Inference settings
	InferenceURL string
	APIKey       string
	ContextSize  int
	Timeout      int

	// File paths
	ConfigPath string
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		InferenceURL: getEnv("INFERENCE_URL", "http://localhost:8080/v1"),
		APIKey:       getEnv("API_KEY", "sk-no-key-required"),
		ContextSize:  getEnvInt("CONTEXT_SIZE", 128000),
		Timeout:      getEnvInt("INITIAL_TOKEN_TIMEOUT", 7200), // 2 hours in seconds
		ConfigPath:   getEnv("CONFIG_PATH", "config.yaml"),
	}
}

// LoadConfig loads configuration from environment variables and optional config file
func LoadConfig() *Config {
	cfg := DefaultConfig()
	// Config file loading can be added later
	return cfg
}

// getEnv returns the environment variable value or a default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt returns the environment variable value as int or a default
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
