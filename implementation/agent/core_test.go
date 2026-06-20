package agent

import (
	"path/filepath"
	"testing"

	"github.com/coding-agent/harness/colors"
	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
)

func TestSetStreamCallback(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	if agent == nil {
		t.Fatal("NewAgent() returned nil")
	}

	// Set a stream callback - should not panic
	agent.SetStreamCallback(func(chunk inference.StreamingChunk) {
		// noop for test
	})
}

func TestSetContextSizeCallback(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Set a context size callback - should not panic
	agent.SetContextSizeCallback(func(size, max int) {
		if size < 0 {
			t.Error("Expected non-negative size")
		}
		if max < 0 {
			t.Error("Expected non-negative max")
		}
	})
}

func TestGetTools(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	tools := agent.GetTools()
	if len(tools) == 0 {
		t.Error("Expected at least one tool")
	}
}

func TestCloseDebugLogger_NoLogger(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Debug = false
	agent := NewAgent(cfg)

	err := agent.CloseDebugLogger()
	if err != nil {
		t.Errorf("Expected no error when closing nil debug logger, got: %v", err)
	}
}

func TestSetAPIEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Should not panic
	agent.SetAPIEndpoint("http://custom-endpoint:8080/v1")
}

func TestSetAPIKey(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Should not panic
	agent.SetAPIKey("test-api-key-12345")
}

func TestSetMaxDisplayWidth(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Should not panic
	agent.SetMaxDisplayWidth(80)
	agent.SetMaxDisplayWidth(120)
	agent.SetMaxDisplayWidth(0)
}

func TestGetToolExecutor(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	executor := agent.GetToolExecutor()
	if executor == nil {
		t.Fatal("GetToolExecutor() returned nil")
	}
}

func TestExitCodes_Constants(t *testing.T) {
	if ExitSuccess != 0 {
		t.Errorf("Expected ExitSuccess = 0, got %d", ExitSuccess)
	}
	if ExitError != 1 {
		t.Errorf("Expected ExitError = 1, got %d", ExitError)
	}
	if ExitUsageError != 2 {
		t.Errorf("Expected ExitUsageError = 2, got %d", ExitUsageError)
	}
	if ExitAuthError != 3 {
		t.Errorf("Expected ExitAuthError = 3, got %d", ExitAuthError)
	}
	if ExitContextLimit != 4 {
		t.Errorf("Expected ExitContextLimit = 4, got %d", ExitContextLimit)
	}
}

func TestColorConstants(t *testing.T) {
	expected := map[string]string{
		"reset":   "\033[0m",
		"green":   "\033[32m",
		"yellow":  "\033[33m",
		"red":     "\033[31m",
		"cyan":    "\033[36m",
		"blue":    "\033[34m",
		"magenta": "\033[35m",
		"dim":     "\033[90m",
	}

	for slot, want := range expected {
		got := colors.GetColor(slot)
		if got != want {
			t.Errorf("GetColor(%q) = %q, want %q", slot, got, want)
		}
	}
}

func TestNewAgent_DebugMode(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "debug.log")

	cfg := &config.Config{
		Debug:         true,
		DebugLog:      logPath,
		APIEndpoint:   "http://localhost:8080",
		Model:         "test-model",
		MaxTokens:     1000,
		ContextSize:   4096,
		MaxIterations: 50,
		ReadOnly:      false,
		Streaming:     true,
		ShowVersion:   false,
		ShowHelp:      false,
		Verbose:       false,
		Quiet:         false,
		Temperature:   nil,

		Prompt:     "",
		PromptFile: "",
		UseStdin:   false,
	}

	ag := NewAgent(cfg)
	if ag == nil {
		t.Fatal("NewAgent returned nil")
	}

	if ag.debugLogger == nil {
		t.Error("Expected debug logger to be non-nil when Debug is true")
	}
}

func TestNewAgent_ReadOnlyMode(t *testing.T) {
	cfg := &config.Config{
		Debug:         false,
		APIEndpoint:   "http://localhost:8080",
		Model:         "test-model",
		MaxTokens:     1000,
		ContextSize:   4096,
		MaxIterations: 50,
		ReadOnly:      true,
		Streaming:     true,
		ShowVersion:   false,
		ShowHelp:      false,
		Verbose:       false,
		Quiet:         false,
	}

	ag := NewAgent(cfg)
	if ag == nil {
		t.Fatal("NewAgent returned nil")
	}

	// In read-only mode, write tools should not be available
	tools := ag.GetTools()
	for _, tool := range tools {
		if tool.Function.Name == "write_file" || tool.Function.Name == "insert_lines" || tool.Function.Name == "replace_text" {
			t.Errorf("Tool '%s' should not be available in read-only mode", tool.Function.Name)
		}
	}
}

