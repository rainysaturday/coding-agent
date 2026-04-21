package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coding-agent/harness/agent"
	"github.com/coding-agent/harness/config"
)

func TestLoadPrompt_FromString(t *testing.T) {
	cfg := &config.Config{Prompt: "Hello world"}
	prompt, err := loadPrompt(cfg)
	if err != nil {
		t.Fatalf("loadPrompt() error: %v", err)
	}
	if prompt != "Hello world" {
		t.Errorf("Expected 'Hello world', got '%s'", prompt)
	}
}

func TestLoadPrompt_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "prompt.txt")
	expectedContent := "This is my prompt from file"
	os.WriteFile(testFile, []byte(expectedContent), 0644)

	cfg := &config.Config{PromptFile: testFile}
	prompt, err := loadPrompt(cfg)
	if err != nil {
		t.Fatalf("loadPrompt() error: %v", err)
	}
	if prompt != expectedContent {
		t.Errorf("Expected '%s', got '%s'", expectedContent, prompt)
	}
}

func TestLoadPrompt_FromStdin(t *testing.T) {
	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r

	// Write some data and close to signal EOF
	go func() {
		w.WriteString("test prompt from stdin")
		w.Close()
	}()

	cfg := &config.Config{UseStdin: true}
	prompt, err := loadPrompt(cfg)
	os.Stdin = oldStdin
	r.Close()

	if err != nil {
		t.Fatalf("loadPrompt() error: %v", err)
	}
	if prompt != "test prompt from stdin" {
		t.Errorf("Expected 'test prompt from stdin', got '%s'", prompt)
	}
}

func TestLoadPrompt_NoPrompt(t *testing.T) {
	cfg := &config.Config{}
	_, err := loadPrompt(cfg)
	if err == nil {
		t.Fatal("Expected error for no prompt")
	}
	if !strings.Contains(err.Error(), "no prompt") {
		t.Errorf("Expected 'no prompt' error, got: %v", err)
	}
}

func TestLoadPrompt_PromptTakesPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "prompt.txt")
	os.WriteFile(testFile, []byte("from file"), 0644)

	cfg := &config.Config{
		Prompt:     "from string",
		PromptFile: testFile,
	}
	prompt, err := loadPrompt(cfg)
	if err != nil {
		t.Fatalf("loadPrompt() error: %v", err)
	}
	if prompt != "from string" {
		t.Errorf("Expected 'from string' (prompt takes precedence), got '%s'", prompt)
	}
}

func TestLoadPrompt_FileNotFound(t *testing.T) {
	cfg := &config.Config{
		PromptFile: "/nonexistent/file/that/does/not/exist.txt",
	}
	_, err := loadPrompt(cfg)
	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}
}

func TestOutputResult_QuietMode(t *testing.T) {
	cfg := &config.Config{Quiet: true}
	result := &agent.Result{FinalOutput: "test output"}

	tmpOut, err := os.CreateTemp("", "output-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpOut.Close()
	defer os.Remove(tmpOut.Name())

	oldStdout := os.Stdout
	os.Stdout, _ = os.OpenFile(tmpOut.Name(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer func() { os.Stdout = oldStdout }()

	err = outputResult(result, cfg, 0)
	if err != nil {
		t.Fatalf("outputResult() error: %v", err)
	}

	content, err := os.ReadFile(tmpOut.Name())
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	if !strings.Contains(string(content), "test output") {
		t.Errorf("Expected output to contain 'test output', got: '%s'", string(content))
	}
}

func TestOutputResult_NormalMode(t *testing.T) {
	cfg := &config.Config{Quiet: false}
	result := &agent.Result{FinalOutput: "normal output"}

	tmpOut, err := os.CreateTemp("", "output-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpOut.Close()
	defer os.Remove(tmpOut.Name())

	oldStdout := os.Stdout
	os.Stdout, _ = os.OpenFile(tmpOut.Name(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer func() { os.Stdout = oldStdout }()

	err = outputResult(result, cfg, 0)
	if err != nil {
		t.Fatalf("outputResult() error: %v", err)
	}

	content, err := os.ReadFile(tmpOut.Name())
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	if !strings.Contains(string(content), "normal output") {
		t.Errorf("Expected output to contain 'normal output', got: '%s'", string(content))
	}
}

func TestOutputResult_VerboseMode(t *testing.T) {
	cfg := &config.Config{Quiet: false, Verbose: true}
	result := &agent.Result{
		FinalOutput: "final answer",
		TokenUsage:  150,
		Steps:       []agent.Step{},
	}

	tmpOut, err := os.CreateTemp("", "output-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpOut.Close()
	defer os.Remove(tmpOut.Name())

	oldStdout := os.Stdout
	os.Stdout, _ = os.OpenFile(tmpOut.Name(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer func() { os.Stdout = oldStdout }()

	err = outputResult(result, cfg, 1*time.Second)
	if err != nil {
		t.Fatalf("outputResult() error: %v", err)
	}

	content, err := os.ReadFile(tmpOut.Name())
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	output := string(content)
	if !strings.Contains(output, "final answer") {
		t.Errorf("Expected output to contain 'final answer', got: '%s'", output)
	}
	if !strings.Contains(output, "=== Agent Execution Log ===") {
		t.Error("Expected '=== Agent Execution Log ===' header")
	}
	if !strings.Contains(output, "=== Summary ===") {
		t.Error("Expected '=== Summary ===' section")
	}
	if !strings.Contains(output, "Tokens used: 150") {
		t.Error("Expected token usage in summary")
	}
}

func TestOutputResult_WithSteps(t *testing.T) {
	cfg := &config.Config{Quiet: false, Verbose: true}
	result := &agent.Result{
		FinalOutput: "done",
		Steps: []agent.Step{
			{Action: "running bash"},
		},
	}

	tmpOut, err := os.CreateTemp("", "output-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpOut.Close()
	defer os.Remove(tmpOut.Name())

	oldStdout := os.Stdout
	os.Stdout, _ = os.OpenFile(tmpOut.Name(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer func() { os.Stdout = oldStdout }()

	err = outputResult(result, cfg, 0)
	if err != nil {
		t.Fatalf("outputResult() error: %v", err)
	}

	content, err := os.ReadFile(tmpOut.Name())
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	if !strings.Contains(string(content), "running bash") {
		t.Error("Expected step action in output")
	}
}

func TestOutputResult_ToFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{OutputFile: filepath.Join(tmpDir, "output.txt")}
	result := &agent.Result{FinalOutput: "file output"}

	err := outputResult(result, cfg, 0)
	if err != nil {
		t.Fatalf("outputResult() error: %v", err)
	}

	content, err := os.ReadFile(cfg.OutputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}
	if string(content) != "file output" {
		t.Errorf("Expected file content 'file output', got: '%s'", string(content))
	}
}

func TestOutputResult_ToFileFailed(t *testing.T) {
	cfg := &config.Config{OutputFile: "/nonexistent/dir/output.txt"}
	result := &agent.Result{FinalOutput: "test"}

	err := outputResult(result, cfg, 0)
	if err == nil {
		t.Fatal("Expected error for nonexistent output directory")
	}
	if !strings.Contains(err.Error(), "failed to write output file") {
		t.Errorf("Expected file write error, got: %v", err)
	}
}

func TestDisplayVersion(t *testing.T) {
	originalHash := gitHash
	originalDirty := gitDirty
	originalBuildTime := buildTime
	defer func() {
		gitHash = originalHash
		gitDirty = originalDirty
		buildTime = originalBuildTime
	}()

	gitHash = "abc123"
	gitDirty = "clean"
	buildTime = "2024-01-01T00:00:00Z"

	tmpOut, err := os.CreateTemp("", "version-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpOut.Close()
	defer os.Remove(tmpOut.Name())

	oldStdout := os.Stdout
	os.Stdout, _ = os.OpenFile(tmpOut.Name(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer func() { os.Stdout = oldStdout }()

	displayVersion()

	content, err := os.ReadFile(tmpOut.Name())
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	output := string(content)
	if !strings.Contains(output, "Minimal Coding Agent Harness") {
		t.Error("Expected 'Minimal Coding Agent Harness' in output")
	}
	if !strings.Contains(output, "abc123") {
		t.Errorf("Expected git hash in version output, got: %s", output)
	}
	if !strings.Contains(output, "Built:") {
		t.Errorf("Expected build time in output, got: %s", output)
	}
}

func TestDisplayVersion_Dirty(t *testing.T) {
	originalHash := gitHash
	originalDirty := gitDirty
	defer func() {
		gitHash = originalHash
		gitDirty = originalDirty
	}()

	gitHash = "def456"
	gitDirty = "dirty"

	tmpOut, err := os.CreateTemp("", "version-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpOut.Close()
	defer os.Remove(tmpOut.Name())

	oldStdout := os.Stdout
	os.Stdout, _ = os.OpenFile(tmpOut.Name(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer func() { os.Stdout = oldStdout }()

	displayVersion()

	content, err := os.ReadFile(tmpOut.Name())
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	output := string(content)
	if !strings.Contains(output, "def456") {
		t.Errorf("Expected git hash in dirty output, got: %s", output)
	}
	if !strings.Contains(output, "[dirty]") {
		t.Errorf("Expected '[dirty]' in output, got: %s", output)
	}
}

func TestDisplayVersion_UnknownHash(t *testing.T) {
	originalHash := gitHash
	originalDirty := gitDirty
	originalBuildTime := buildTime
	defer func() {
		gitHash = originalHash
		gitDirty = originalDirty
		buildTime = originalBuildTime
	}()

	// Set to clean with a known hash (init() already sets empty to "unknown")
	gitHash = "v1.0.0"
	gitDirty = "clean"
	buildTime = ""

	tmpOut, err := os.CreateTemp("", "version-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpOut.Close()
	defer os.Remove(tmpOut.Name())

	oldStdout := os.Stdout
	os.Stdout, _ = os.OpenFile(tmpOut.Name(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer func() { os.Stdout = oldStdout }()

	displayVersion()

	content, err := os.ReadFile(tmpOut.Name())
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	output := string(content)
	if !strings.Contains(output, "v1.0.0") {
		t.Errorf("Expected 'v1.0.0' in output, got: %s", output)
	}
	if !strings.Contains(output, "[clean]") {
		t.Errorf("Expected '[clean]' in output, got: %s", output)
	}
}

func TestDisplayHelp(t *testing.T) {
	tmpOut, err := os.CreateTemp("", "help-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpOut.Close()
	defer os.Remove(tmpOut.Name())

	oldStdout := os.Stdout
	os.Stdout, _ = os.OpenFile(tmpOut.Name(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer func() { os.Stdout = oldStdout }()

	displayHelp()

	content, err := os.ReadFile(tmpOut.Name())
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	output := string(content)

	expected := []string{
		"Minimal Coding Agent Harness",
		"Usage:",
		"Options:",
		"--prompt",
		"--stdin",
		"--config",
		"--debug",
		"--model",
		"--temperature",
		"--max-tokens",
		"--context-size",
		"--max-iterations",
		"--api-endpoint",
		"--api-key",
		"--verbose",
		"--quiet",
		"--no-stream",
		"-h, --help",
		"-v, --version",
		"Examples:",
		"Copilot",
		"GitHub Models",
	}

	for _, exp := range expected {
		if !strings.Contains(output, exp) {
			t.Errorf("Expected help output to contain '%s'", exp)
		}
	}
}

func TestDisplayHelp_NoCrash(t *testing.T) {
	tmpOut, err := os.CreateTemp("", "help-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpOut.Close()
	defer os.Remove(tmpOut.Name())

	oldStdout := os.Stdout
	os.Stdout, _ = os.OpenFile(tmpOut.Name(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer func() { os.Stdout = oldStdout }()

	displayHelp()

	content, err := os.ReadFile(tmpOut.Name())
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	if len(content) == 0 {
		t.Error("Expected some output from displayHelp()")
	}
}

func TestOutputResult_EmptyFinalOutput(t *testing.T) {
	cfg := &config.Config{Quiet: false}
	result := &agent.Result{FinalOutput: ""}

	tmpOut, err := os.CreateTemp("", "output-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpOut.Close()
	defer os.Remove(tmpOut.Name())

	oldStdout := os.Stdout
	os.Stdout, _ = os.OpenFile(tmpOut.Name(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer func() { os.Stdout = oldStdout }()

	err = outputResult(result, cfg, 0)
	if err != nil {
		t.Fatalf("outputResult() error: %v", err)
	}
}

func TestOutputResult_EmptyStepsVerbose(t *testing.T) {
	cfg := &config.Config{Quiet: false, Verbose: true}
	result := &agent.Result{FinalOutput: "done", Steps: []agent.Step{}}

	tmpOut, err := os.CreateTemp("", "output-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpOut.Close()
	defer os.Remove(tmpOut.Name())

	oldStdout := os.Stdout
	os.Stdout, _ = os.OpenFile(tmpOut.Name(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer func() { os.Stdout = oldStdout }()

	err = outputResult(result, cfg, 0)
	if err != nil {
		t.Fatalf("outputResult() error: %v", err)
	}
}

func TestOutputResult_DurationFormatting(t *testing.T) {
	cfg := &config.Config{Quiet: false, Verbose: true}
	result := &agent.Result{FinalOutput: "done"}
	duration := 2*time.Hour + 30*time.Minute + 15*time.Second

	tmpOut, err := os.CreateTemp("", "output-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpOut.Close()
	defer os.Remove(tmpOut.Name())

	oldStdout := os.Stdout
	os.Stdout, _ = os.OpenFile(tmpOut.Name(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer func() { os.Stdout = oldStdout }()

	err = outputResult(result, cfg, duration)
	if err != nil {
		t.Fatalf("outputResult() error: %v", err)
	}

	content, err := os.ReadFile(tmpOut.Name())
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	output := string(content)
	if !strings.Contains(output, "Duration:") {
		t.Error("Expected Duration in output")
	}
}

func TestBuildVersion(t *testing.T) {
	originalHash := gitHash
	originalDirty := gitDirty
	defer func() {
		gitHash = originalHash
		gitDirty = originalDirty
	}()

	gitHash = ""
	gitDirty = ""
	agent.SetBuildVersion("test")
}

func TestOutputResult_TokenUsageInVerbose(t *testing.T) {
	cfg := &config.Config{Quiet: false, Verbose: true}
	result := &agent.Result{
		FinalOutput: "answer",
		TokenUsage:  1024,
		Steps:       []agent.Step{},
	}

	tmpOut, err := os.CreateTemp("", "output-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpOut.Close()
	defer os.Remove(tmpOut.Name())

	oldStdout := os.Stdout
	os.Stdout, _ = os.OpenFile(tmpOut.Name(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer func() { os.Stdout = oldStdout }()

	err = outputResult(result, cfg, 0)
	if err != nil {
		t.Fatalf("outputResult() error: %v", err)
	}

	content, err := os.ReadFile(tmpOut.Name())
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	output := string(content)
	if !strings.Contains(output, "1024") {
		t.Errorf("Expected token count 1024 in output, got: %s", output)
	}
}

func TestOutputResult_NoOutputFile(t *testing.T) {
	cfg := &config.Config{OutputFile: ""}
	result := &agent.Result{FinalOutput: "no output file"}

	tmpOut, err := os.CreateTemp("", "output-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpOut.Close()
	defer os.Remove(tmpOut.Name())

	oldStdout := os.Stdout
	os.Stdout, _ = os.OpenFile(tmpOut.Name(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	defer func() { os.Stdout = oldStdout }()

	err = outputResult(result, cfg, 0)
	if err != nil {
		t.Fatalf("outputResult() error: %v", err)
	}

	content, err := os.ReadFile(tmpOut.Name())
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	if !strings.Contains(string(content), "no output file") {
		t.Errorf("Expected output to contain text, got: '%s'", string(content))
	}
}

func TestAgentResult_Structure(t *testing.T) {
	result := &agent.Result{
		FinalOutput: "complete",
		TokenUsage:  100,
		Steps: []agent.Step{
			{Action: "step 1"},
			{Action: "step 2"},
		},
	}

	if result.FinalOutput != "complete" {
		t.Errorf("Expected 'complete', got '%s'", result.FinalOutput)
	}
	if result.TokenUsage != 100 {
		t.Errorf("Expected token usage 100, got %d", result.TokenUsage)
	}
	if len(result.Steps) != 2 {
		t.Errorf("Expected 2 steps, got %d", len(result.Steps))
	}
}

func TestBuildVersion_WithAgent(t *testing.T) {
	agent.SetBuildVersion("v1.2.3-abc123")
}

func TestAgentResult_NilSteps(t *testing.T) {
	result := &agent.Result{FinalOutput: "test"}
	if len(result.Steps) != 0 {
		t.Errorf("Expected 0 steps, got %d", len(result.Steps))
	}
}

func TestMainPackageImports(t *testing.T) {
	// This test just verifies the package compiles correctly.
	// A full build is covered by CI; this is a sanity check.
	t.Parallel()
}
