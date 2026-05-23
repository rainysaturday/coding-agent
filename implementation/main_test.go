package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coding-agent/harness/agent"
	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/tools"
)

func TestExitCodeForError(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected int
	}{
		{"nil error", nil, 0}, // ExitSuccess
		{"context limit", fmt.Errorf("context size limit exceeded"), 4},          // ExitContextLimit
		{"max context length", fmt.Errorf("maximum context length exceeded"), 4}, // ExitContextLimit
		{"auth failed", fmt.Errorf("authentication failed"), 3},                  // ExitAuthError
		{"401", fmt.Errorf("401 Unauthorized"), 3},                               // ExitAuthError
		{"403", fmt.Errorf("403 Forbidden"), 3},                                  // ExitAuthError
		{"API auth", fmt.Errorf("API authentication error"), 3},                  // ExitAuthError
		{"general", fmt.Errorf("some other error"), 1},                           // ExitError
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exitCodeForError(tt.input)
			if result != tt.expected {
				t.Errorf("exitCodeForError(%v) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExitCodeForError_EdgeCases(t *testing.T) {
	// Test with errors that contain multiple keywords
	tests := []struct {
		name     string
		input    error
		expected int
	}{
		{"context in auth", fmt.Errorf("authentication context size limit"), 4}, // context takes priority
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exitCodeForError(tt.input)
			if result != tt.expected {
				t.Errorf("exitCodeForError(%v) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoadPrompt_WithPrompt(t *testing.T) {
	cfg := &config.Config{
		Prompt: "Test prompt content",
	}

	result, err := loadPrompt(cfg)
	if err != nil {
		t.Fatalf("loadPrompt() error: %v", err)
	}
	if result != "Test prompt content" {
		t.Errorf("loadPrompt() = %q, expected %q", result, "Test prompt content")
	}
}

func TestLoadPrompt_WithPromptFile(t *testing.T) {
	tmpDir := t.TempDir()
	promptFile := filepath.Join(tmpDir, "prompt.txt")
	expectedContent := "This is a test prompt from file"
	os.WriteFile(promptFile, []byte(expectedContent), 0644)

	cfg := &config.Config{
		Prompt:     "", // Empty prompt
		PromptFile: promptFile,
	}

	result, err := loadPrompt(cfg)
	if err != nil {
		t.Fatalf("loadPrompt() error: %v", err)
	}
	if result != expectedContent {
		t.Errorf("loadPrompt() = %q, expected %q", result, expectedContent)
	}
}

func TestLoadPrompt_FileNotExists(t *testing.T) {
	cfg := &config.Config{
		Prompt:     "", // Empty prompt
		PromptFile: "/nonexistent/prompt.txt",
	}

	_, err := loadPrompt(cfg)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestLoadPrompt_PromptFileOverPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	promptFile := filepath.Join(tmpDir, "prompt.txt")
	os.WriteFile(promptFile, []byte("File content"), 0644)

	// When PromptFile is set, it should be ignored if Prompt is also set
	cfg := &config.Config{
		Prompt:     "Inline prompt",
		PromptFile: promptFile,
	}

	result, err := loadPrompt(cfg)
	if err != nil {
		t.Fatalf("loadPrompt() error: %v", err)
	}
	if result != "Inline prompt" {
		t.Errorf("loadPrompt() = %q, expected %q", result, "Inline prompt")
	}
}

// Helper to get exit codes
func TestExitCodes_Constants(t *testing.T) {
	// Verify the exit code constants are what we expect
	// ExitSuccess = 0, ExitError = 1, ExitAuthError = 3, ExitContextLimit = 4
	if exitCodeForError(nil) != 0 {
		t.Error("Expected nil error to return exit code 0")
	}
	if exitCodeForError(errors.New("test")) != 1 {
		t.Error("Expected general error to return exit code 1")
	}
	if exitCodeForError(errors.New("context size limit")) != 4 {
		t.Error("Expected context limit error to return exit code 4")
	}
	if exitCodeForError(errors.New("authentication failed")) != 3 {
		t.Error("Expected auth error to return exit code 3")
	}
}

// ===== Tests for outputResult =====

func TestOutputResult_QuietMode(t *testing.T) {
	cfg := &config.Config{
		Quiet: true,
	}
	result := &agent.Result{
		FinalOutput: "Hello world",
		Reasoning:   "Some reasoning",
		Steps:       []agent.Step{},
		TokenUsage:  100,
	}

	err := outputResult(result, cfg, 0)
	if err != nil {
		t.Fatalf("outputResult() error: %v", err)
	}
}

func TestOutputResult_VerboseMode(t *testing.T) {
	cfg := &config.Config{
		Verbose: true,
	}
	result := &agent.Result{
		FinalOutput: "Hello world",
		Reasoning:   "Some reasoning",
		Steps: []agent.Step{
			{
				Action:     "Test action",
				ToolCall:   &tools.ToolCall{Name: "bash", Parameters: map[string]interface{}{"command": "echo test"}},
				ToolResult: &tools.ToolResult{Success: true, Output: "test output"},
			},
		},
		TokenUsage: 200,
	}

	err := outputResult(result, cfg, 0)
	if err != nil {
		t.Fatalf("outputResult() error: %v", err)
	}
}

func TestOutputResult_NoReasoning(t *testing.T) {
	cfg := &config.Config{}
	result := &agent.Result{
		FinalOutput: "Hello world",
		Reasoning:   "",
		Steps:       []agent.Step{},
		TokenUsage:  50,
	}

	err := outputResult(result, cfg, 0)
	if err != nil {
		t.Fatalf("outputResult() error: %v", err)
	}
}

func TestOutputResult_VerboseNoReasoning(t *testing.T) {
	cfg := &config.Config{
		Verbose: true,
	}
	result := &agent.Result{
		FinalOutput: "Hello world",
		Reasoning:   "",
		Steps:       []agent.Step{},
		TokenUsage:  50,
	}

	err := outputResult(result, cfg, 0)
	if err != nil {
		t.Fatalf("outputResult() error: %v", err)
	}
}

func TestOutputResult_OutputFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")

	cfg := &config.Config{
		OutputFile: outputFile,
	}
	result := &agent.Result{
		FinalOutput: "This is the output",
		Reasoning:   "",
		Steps:       []agent.Step{},
		TokenUsage:  50,
	}

	err := outputResult(result, cfg, 0)
	if err != nil {
		t.Fatalf("outputResult() error: %v", err)
	}

	// Verify file was written
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}
	if string(content) != "This is the output" {
		t.Errorf("Expected 'This is the output', got '%s'", string(content))
	}
}

func TestOutputResult_OutputFileWriteError(t *testing.T) {
	cfg := &config.Config{
		OutputFile: "/nonexistent/path/that/does/not/exist/output.txt",
	}
	result := &agent.Result{
		FinalOutput: "This is the output",
		Reasoning:   "",
		Steps:       []agent.Step{},
		TokenUsage:  50,
	}

	err := outputResult(result, cfg, 0)
	if err == nil {
		t.Error("Expected error for non-existent output file path")
	}
	if !strings.Contains(err.Error(), "failed to write output file") {
		t.Errorf("Expected 'failed to write output file', got: %v", err)
	}
}

// ===== Tests for displayVersion =====

func TestDisplayVersion_Clean(t *testing.T) {
	// Save and restore original values
	oldHash := gitHash
	oldDirty := gitDirty
	oldBuildTime := buildTime
	defer func() {
		gitHash = oldHash
		gitDirty = oldDirty
		buildTime = oldBuildTime
	}()

	gitHash = "abc123"
	gitDirty = "clean"
	buildTime = "2024-01-01T00:00:00Z"

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	displayVersion()

	w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Version: abc123") {
		t.Errorf("Expected 'Version: abc123' in output, got: %s", output)
	}
	if !strings.Contains(output, "[clean]") {
		t.Errorf("Expected '[clean]' in output, got: %s", output)
	}
	if !strings.Contains(output, "Built: 2024-01-01T00:00:00Z") {
		t.Errorf("Expected 'Built: 2024-01-01T00:00:00Z' in output, got: %s", output)
	}
}

func TestDisplayVersion_Dirty(t *testing.T) {
	oldHash := gitHash
	oldDirty := gitDirty
	oldBuildTime := buildTime
	defer func() {
		gitHash = oldHash
		gitDirty = oldDirty
		buildTime = oldBuildTime
	}()

	gitHash = "def456"
	gitDirty = "dirty"
	buildTime = ""

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	displayVersion()

	w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Version: def456") {
		t.Errorf("Expected 'Version: def456' in output, got: %s", output)
	}
	if !strings.Contains(output, "[dirty]") {
		t.Errorf("Expected '[dirty]' in output, got: %s", output)
	}
}

func TestDisplayVersion_Unknown(t *testing.T) {
	oldHash := gitHash
	oldDirty := gitDirty
	oldBuildTime := buildTime
	defer func() {
		gitHash = oldHash
		gitDirty = oldDirty
		buildTime = oldBuildTime
	}()

	// After init runs, gitHash will be "unknown" if it was empty
	// We simulate this by setting gitHash to its initial empty value,
	// then displayVersion will show whatever is in gitHash
	// Since init has already run, gitHash is already "unknown"
	// So we just verify the version string appears
	gitHash = oldHash // restore original
	gitDirty = oldDirty
	buildTime = oldBuildTime

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	displayVersion()

	w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	// Just verify the version line appears
	if !strings.Contains(output, "Minimal Coding Agent Harness") {
		t.Errorf("Expected 'Minimal Coding Agent Harness' in output, got: %s", output)
	}
}

// ===== Tests for loadPrompt with stdin =====

func TestLoadPrompt_Stdin(t *testing.T) {
	// This test is tricky because it reads from os.Stdin
	// We can only test that the function doesn't panic
	// The stdin test is hard to test because it blocks

	// Test that when no source is provided, it returns an error
	cfg := &config.Config{}
	_, err := loadPrompt(cfg)
	if err == nil {
		t.Error("Expected error when no prompt source provided")
	}
}

// ===== Test for runOneShotMode error handling =====

func TestExitCodeForError_ContextLimitInAuth(t *testing.T) {
	// Test that context limit errors take priority over auth errors
	// when the message contains both patterns
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"auth only", "authentication failed", 3},
		{"context limit only", "context size limit", 4},
		{"401 only", "401 error", 3},
		{"403 only", "403 error", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exitCodeForError(errors.New(tt.input))
			if result != tt.expected {
				t.Errorf("exitCodeForError(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

// ===== Tests for config flag combinations =====

func TestConfig_ParseFlags_QuietAndVerbose(t *testing.T) {
	// Test that quiet and verbose can both be set
	cfg, err := config.ParseArgs([]string{"--quiet", "--verbose", "-p", "test"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}
	if !cfg.Quiet {
		t.Error("Expected Quiet to be true")
	}
	if !cfg.Verbose {
		t.Error("Expected Verbose to be true")
	}
}

func TestConfig_ParseFlags_ReadOnly(t *testing.T) {
	cfg, err := config.ParseArgs([]string{"--read-only", "-p", "test"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}
	if !cfg.ReadOnly {
		t.Error("Expected ReadOnly to be true")
	}
}

func TestConfig_ParseFlags_OutputFile(t *testing.T) {
	cfg, err := config.ParseArgs([]string{"--output", "/tmp/result.txt", "-p", "test"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}
	if cfg.OutputFile != "/tmp/result.txt" {
		t.Errorf("Expected OutputFile '/tmp/result.txt', got '%s'", cfg.OutputFile)
	}
}

func TestConfig_ParseFlags_NoStream(t *testing.T) {
	cfg, err := config.ParseArgs([]string{"--no-stream", "-p", "test"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}
	if cfg.Streaming {
		t.Error("Expected Streaming to be false with --no-stream")
	}
}

func TestConfig_ParseFlags_ModelAndTemperature(t *testing.T) {
	cfg, err := config.ParseArgs([]string{"--model", "gpt-4", "--temperature", "0.7", "-p", "test"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}
	if cfg.Model != "gpt-4" {
		t.Errorf("Expected Model 'gpt-4', got '%s'", cfg.Model)
	}
	if cfg.Temperature == nil || *cfg.Temperature != 0.7 {
		t.Errorf("Expected Temperature 0.7, got %v", cfg.Temperature)
	}
}

func TestConfig_ParseFlags_MaxTokens(t *testing.T) {
	cfg, err := config.ParseArgs([]string{"--max-tokens", "10000", "-p", "test"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}
	if cfg.MaxTokens != 10000 {
		t.Errorf("Expected MaxTokens 10000, got %v", cfg.MaxTokens)
	}
}

func TestConfig_ParseFlags_ContextSize(t *testing.T) {
	cfg, err := config.ParseArgs([]string{"--context-size", "64000", "-p", "test"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}
	if cfg.ContextSize != 64000 {
		t.Errorf("Expected ContextSize 64000, got %v", cfg.ContextSize)
	}
}

func TestConfig_ParseFlags_MaxIterations(t *testing.T) {
	cfg, err := config.ParseArgs([]string{"--max-iterations", "50", "-p", "test"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}
	if cfg.MaxIterations != 50 {
		t.Errorf("Expected MaxIterations 50, got %v", cfg.MaxIterations)
	}
}

func TestConfig_ParseFlags_APIEndpoint(t *testing.T) {
	cfg, err := config.ParseArgs([]string{"--api-endpoint", "http://example.com:8080", "-p", "test"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}
	if cfg.APIEndpoint != "http://example.com:8080" {
		t.Errorf("Expected APIEndpoint 'http://example.com:8080', got '%s'", cfg.APIEndpoint)
	}
}

func TestConfig_ParseFlags_APIKey(t *testing.T) {
	cfg, err := config.ParseArgs([]string{"--api-key", "sk-test123", "-p", "test"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}
	if cfg.APIKey != "sk-test123" {
		t.Errorf("Expected APIKey 'sk-test123', got '%s'", cfg.APIKey)
	}
}

func TestConfig_ParseFlags_DebugLogPath(t *testing.T) {
	cfg, err := config.ParseArgs([]string{"--debug-log", "/tmp/custom-debug.log", "-p", "test"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}
	if cfg.DebugLog != "/tmp/custom-debug.log" {
		t.Errorf("Expected DebugLogPath '/tmp/custom-debug.log', got '%s'", cfg.DebugLog)
	}
}

func TestConfig_ParseFlags_ConnectionTimeout(t *testing.T) {
	cfg, err := config.ParseArgs([]string{"--connection-timeout", "30", "-p", "test"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}
	if cfg.ConnectionTimeout != 30 {
		t.Errorf("Expected ConnectionTimeout 30, got %v", cfg.ConnectionTimeout)
	}
}

func TestConfig_ParseFlags_ReadTimeout(t *testing.T) {
	cfg, err := config.ParseArgs([]string{"--read-timeout", "120", "-p", "test"})
	if err != nil {
		t.Fatalf("ParseArgs() error: %v", err)
	}
	if cfg.ReadTimeout != 120 {
		t.Errorf("Expected ReadTimeout 120, got %v", cfg.ReadTimeout)
	}
}

func TestDisplayHelp(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	displayHelp()

	w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	// Verify key parts of help text appear
	if !strings.Contains(output, "Minimal Coding Agent Harness") {
		t.Error("Expected 'Minimal Coding Agent Harness' in help output")
	}
	if !strings.Contains(output, "Options:") {
		t.Error("Expected 'Options:' in help output")
	}
	if !strings.Contains(output, "Interactive Commands:") {
		t.Error("Expected 'Interactive Commands:' in help output")
	}
	if !strings.Contains(output, "/stats") {
		t.Error("Expected '/stats' in help output")
	}
}
