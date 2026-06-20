package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
)

func TestGetContextSize_Initial(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	size := agent.GetContextSize()
	// Should be positive due to system prompt
	if size <= 0 {
		t.Errorf("Expected positive context size, got %d", size)
	}
}

func TestGetActualContextSize(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	size := agent.GetActualContextSize()
	if size <= 0 {
		t.Errorf("Expected positive actual context size, got %d", size)
	}

	// Add messages and verify size increases
	agent.AddUserMessage("test message")
	size2 := agent.GetActualContextSize()
	if size2 <= size {
		t.Errorf("Expected size to increase after adding message, was %d, now %d", size, size2)
	}
}

func TestAddUserMessage(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	agent.AddUserMessage("Hello")
	if len(agent.context) != 1 {
		t.Errorf("Expected 1 message, got %d", len(agent.context))
	}
	if agent.context[0].Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", agent.context[0].Role)
	}
}

func TestAddAssistantMessage(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	agent.AddAssistantMessage("Hi")
	if len(agent.context) != 1 {
		t.Errorf("Expected 1 message, got %d", len(agent.context))
	}
	if agent.context[0].Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", agent.context[0].Role)
	}
}

func TestClearContext_AfterMessages(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	agent.AddUserMessage("msg1")
	agent.AddUserMessage("msg2")
	agent.AddAssistantMessage("reply1")

	if len(agent.context) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(agent.context))
	}

	agent.ClearContext()

	if len(agent.context) != 0 {
		t.Errorf("Expected 0 messages after clear, got %d", len(agent.context))
	}
}

func TestGetContextSize_AfterClear(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	initialSize := agent.GetContextSize()

	agent.AddUserMessage("test message to add tokens")
	largerSize := agent.GetContextSize()
	if largerSize <= initialSize {
		t.Errorf("Expected size to increase, was %d, now %d", initialSize, largerSize)
	}

	agent.ClearContext()

	afterClearSize := agent.GetContextSize()
	// Should be back to approximately initial size (system prompt only)
	if afterClearSize > largerSize {
		t.Errorf("Expected size to decrease after clear, was %d, now %d", largerSize, afterClearSize)
	}
}

func TestDumpAndLoadContext(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// 1. Set up initial state
	agent.AddUserMessage("Hello, this is a test message")
	agent.AddAssistantMessage("Hi! How can I help you?")
	
	// Simulate some stats
	agent.mu.Lock()
	agent.stats.InputTokens = 100
	agent.stats.OutputTokens = 200
	agent.stats.ToolCalls = 1
	agent.stats.Iterations = 1
	agent.mu.Unlock()

	// Record the current state into iteration history before dumping
	agent.recordIteration()

	// 2. Dump context
	path, err := agent.DumpContext()
	if err != nil {
		t.Fatalf("DumpContext() failed: %v", err)
	}
	defer os.Remove(path)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("Dumped file does not exist at %s", path)
	}

	// 3. Load context into a new agent
	cfg2 := config.DefaultConfig()
	agent2 := NewAgent(cfg2)
	err = agent2.LoadContext(path)
	if err != nil {
		t.Fatalf("LoadContext() failed: %v", err)
	}

	// 4. Verify messages
	if len(agent2.context) != 2 {
		t.Errorf("Expected 2 messages after load, got %d", len(agent2.context))
	}
	if agent2.context[0].Role != "user" || agent2.context[0].Content != "Hello, this is a test message" {
		t.Errorf("First message not restored correctly: %+v", agent2.context[0])
	}
	if agent2.context[1].Role != "assistant" || agent2.context[1].Content != "Hi! How can I help you?" {
		t.Errorf("Second message not restored correctly: %+v", agent2.context[1])
	}

	// 5. Verify stats
	stats2 := agent2.GetStats()
	if stats2.InputTokens != 100 {
		t.Errorf("Expected 100 input tokens, got %d", stats2.InputTokens)
	}
	if stats2.OutputTokens != 200 {
		t.Errorf("Expected 200 output tokens, got %d", stats2.OutputTokens)
	}
	if stats2.ToolCalls != 1 {
		t.Errorf("Expected 1 tool call, got %d", stats2.ToolCalls)
	}
	if stats2.Iterations != 1 {
		t.Errorf("Expected 1 iteration, got %d", stats2.Iterations)
	}
}

func TestLoadContext_FileNotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	err := agent.LoadContext("/tmp/non_existent_context_file_12345.json")
	if err == nil {
		t.Error("Expected error when loading non-existent file, got nil")
	}
}

func TestLoadContext_InvalidJSON(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "invalid.json")
	os.WriteFile(tmpFile, []byte(`{ "invalid": "json"...`), 0644)

	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	err := agent.LoadContext(tmpFile)
	if err == nil {
		t.Error("Expected error when loading invalid JSON, got nil")
	}
}

func TestLoadContext_UnsupportedVersion(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "version.json")
	content := `{"version": 999, "session": {}}`
	os.WriteFile(tmpFile, []byte(content), 0644)

	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	err := agent.LoadContext(tmpFile)
	if err == nil {
		t.Error("Expected error when loading unsupported version, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported context version") {
		t.Errorf("Expected 'unsupported context version' in error, got: %v", err)
	}
}



// TestGetGoal_ReturnsCorrectValue tests that GetGoal returns the correct value
func TestGetGoal_ReturnsCorrectValue(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Test empty goal
	if goal := agent.GetGoal(); goal != "" {
		t.Errorf("Expected empty goal, got %q", goal)
	}

	// Test with goal
	agent.SetGoal("Build a web application")
	if goal := agent.GetGoal(); goal != "Build a web application" {
		t.Errorf("Expected 'Build a web application', got %q", goal)
	}
}

// TestIsGoalActive_ReturnsCorrectValue tests that IsGoalActive returns correct boolean
func TestIsGoalActive_ReturnsCorrectValue(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Should be false initially
	if agent.IsGoalActive() {
		t.Error("Goal should not be active initially")
	}

	// Should be true after SetGoal
	agent.SetGoal("Test")
	if !agent.IsGoalActive() {
		t.Error("Goal should be active after SetGoal")
	}

	// Should be false after ClearGoal
	agent.ClearGoal()
	if agent.IsGoalActive() {
		t.Error("Goal should not be active after ClearGoal")
	}
}

// TestGoalContext_PreservesFirstUserMessage tests that goal mode preserves context correctly
func TestGoalContext_PreservesFirstUserMessage(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Add a user message manually to simulate conversation
	agent.AddUserMessage("Initial user prompt")

	// Set a goal
	agent.SetGoal("Create a REST API")

	// Verify goal is set
	if !agent.IsGoalActive() {
		t.Error("Goal should be active")
	}
	if agent.GetGoal() != "Create a REST API" {
		t.Errorf("Expected 'Create a REST API', got %q", agent.GetGoal())
	}

	// Clear goal and verify context is still intact
	agent.ClearGoal()

	// Context should not be affected by goal operations
	if agent.GetContextSize() == 0 {
		t.Error("Context should still have content")
	}
}

// TestGoalMode_IntegrationWithContext tests goal mode integration with agent context
func TestGetActualContextSizeUnlocked_WithTotalTokens(t *testing.T) {
	cfg := config.DefaultConfig()
	a := NewAgent(cfg)

	// Set lastTotalTokens to simulate having received an API response
	a.mu.Lock()
	a.lastTotalTokens = 1000
	a.mu.Unlock()

	// Get actual context size - should use lastTotalTokens
	size := a.getActualContextSizeUnlocked()
	if size != 1000 {
		t.Errorf("Expected 1000 (lastTotalTokens), got %d", size)
	}
}

func TestGetActualContextSizeUnlocked_WithTotalTokensAndToolMessages(t *testing.T) {
	cfg := config.DefaultConfig()
	a := NewAgent(cfg)

	// Set lastTotalTokens
	a.mu.Lock()
	a.lastTotalTokens = 1000
	a.toolResultMsgsSinceLastAPI = make(map[int]bool)
	a.context = append(a.context, &inference.Message{
		Role:    "tool",
		Content: "Tool result content",
	})
	a.toolResultMsgsSinceLastAPI[0] = true
	a.mu.Unlock()

	// Get actual context size - should include delta for tool message
	size := a.getActualContextSizeUnlocked()
	// lastTotalTokens + 3 + estimateTokens("Tool result content")
	if size < 1003 {
		t.Errorf("Expected > 1003 (lastTotalTokens + delta), got %d", size)
	}
}

func TestDumpContext_MultipleIterations(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Simulate a conversation with multiple turns
	agent.AddUserMessage("First prompt")
	agent.AddAssistantMessage("First response")
	agent.AddUserMessage("Second prompt")
	agent.AddAssistantMessage("Second response")

	// Record two iterations (simulating exit and compression)
	agent.mu.Lock()
	agent.stats.Iterations = 1
	agent.mu.Unlock()
	agent.recordIteration()

	agent.mu.Lock()
	agent.stats.Iterations = 2
	agent.stats.InputTokens = 100
	agent.stats.OutputTokens = 50
	agent.mu.Unlock()
	agent.recordIteration()

	// Dump and verify iterations are saved
	path, err := agent.DumpContext()
	if err != nil {
		t.Fatalf("DumpContext() failed: %v", err)
	}
	defer os.Remove(path)

	// Read and parse the dump file
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read dump file: %v", err)
	}

	var dump ContextDump
	if err := json.Unmarshal(data, &dump); err != nil {
		t.Fatalf("Failed to parse dump file: %v", err)
	}

	if len(dump.Iterations) != 2 {
		t.Errorf("Expected 2 iterations, got %d", len(dump.Iterations))
	}

	// Verify first iteration
	if dump.Iterations[0].Index != 1 {
		t.Errorf("Expected first iteration index 1, got %d", dump.Iterations[0].Index)
	}
	if len(dump.Iterations[0].Messages) != 5 { // system + 2 user + 2 assistant
		t.Errorf("Expected 3 messages in first iteration, got %d", len(dump.Iterations[0].Messages))
	}
	if dump.Iterations[0].Messages[0].Role != "system" {
		t.Errorf("Expected first message to be system, got %s", dump.Iterations[0].Messages[0].Role)
	}

	// Verify second iteration
	if dump.Iterations[1].Index != 2 {
		t.Errorf("Expected second iteration index 2, got %d", dump.Iterations[1].Index)
	}
	if dump.Iterations[1].Stats.InputTokens != 100 {
		t.Errorf("Expected 100 input tokens in second iteration, got %d", dump.Iterations[1].Stats.InputTokens)
	}
}

func TestDumpContext_IterationContainsSystemPrompt(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	agent.AddUserMessage("Test prompt")

	// Record iteration
	agent.recordIteration()

	// Dump and verify
	path, err := agent.DumpContext()
	if err != nil {
		t.Fatalf("DumpContext() failed: %v", err)
	}
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read dump file: %v", err)
	}

	var dump ContextDump
	if err := json.Unmarshal(data, &dump); err != nil {
		t.Fatalf("Failed to parse dump file: %v", err)
	}

	if len(dump.Iterations) == 0 {
		t.Fatal("Expected at least one iteration")
	}

	iteration := dump.Iterations[0]
	if len(iteration.Messages) == 0 {
		t.Fatal("Expected at least one message in iteration")
	}

	if iteration.Messages[0].Role != "system" {
		t.Errorf("Expected first message to be system, got %s", iteration.Messages[0].Role)
	}
	if iteration.Messages[0].Content == "" {
		t.Error("Expected system prompt content to be non-empty")
	}
}

func TestLoadContext_LastIterationOnly(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Add messages and record two iterations
	agent.AddUserMessage("First prompt")
	agent.AddAssistantMessage("First response")
	agent.mu.Lock()
	agent.stats.Iterations = 1
	agent.mu.Unlock()
	agent.recordIteration()

	agent.AddUserMessage("Second prompt")
	agent.AddAssistantMessage("Second response")
	agent.mu.Lock()
	agent.stats.Iterations = 2
	agent.mu.Unlock()
	agent.recordIteration()

	// Dump
	path, err := agent.DumpContext()
	if err != nil {
		t.Fatalf("DumpContext() failed: %v", err)
	}
	defer os.Remove(path)

	// Load into new agent
	cfg2 := config.DefaultConfig()
	agent2 := NewAgent(cfg2)
	err = agent2.LoadContext(path)
	if err != nil {
		t.Fatalf("LoadContext() failed: %v", err)
	}

	// Should only have 3 messages: system is separate, context has: user1, assistant1, user2, assistant2
	// Actually, LoadContext loads the last iteration's messages excluding the system prompt
	if len(agent2.context) != 4 {
		t.Errorf("Expected 4 messages (user1, assistant1, user2, assistant2) after load, got %d", len(agent2.context))
	}
	if agent2.context[0].Content != "First prompt" {
		t.Errorf("Expected first message 'First prompt', got '%s'", agent2.context[0].Content)
	}
	if agent2.context[3].Content != "Second response" {
		t.Errorf("Expected last message 'Second response', got '%s'", agent2.context[3].Content)
	}
}

func TestLoadContext_EmptyIterations(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "empty_iterations.json")
	content := `{
		"version": 1,
		"session": {},
		"iterations": []
	}`
	os.WriteFile(tmpFile, []byte(content), 0644)

	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	err := agent.LoadContext(tmpFile)
	if err == nil {
		t.Error("Expected error when loading empty iterations, got nil")
	}
	if !strings.Contains(err.Error(), "no iterations") {
		t.Errorf("Expected 'no iterations' in error, got: %v", err)
	}
}

func TestLoadContext_SystemPromptRestored(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	originalPrompt := agent.GetSystemPrompt()

	agent.AddUserMessage("Test")
	agent.recordIteration()

	path, err := agent.DumpContext()
	if err != nil {
		t.Fatalf("DumpContext() failed: %v", err)
	}
	defer os.Remove(path)

	cfg2 := config.DefaultConfig()
	agent2 := NewAgent(cfg2)
	err = agent2.LoadContext(path)
	if err != nil {
		t.Fatalf("LoadContext() failed: %v", err)
	}

	if agent2.GetSystemPrompt() != originalPrompt {
		t.Errorf("System prompt not restored correctly\nExpected: %s\nGot: %s", originalPrompt, agent2.GetSystemPrompt())
	}
}

func TestLoadContext_StatsRestored(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	agent.AddUserMessage("Test")
	agent.mu.Lock()
	agent.stats.InputTokens = 500
	agent.stats.OutputTokens = 300
	agent.stats.ToolCalls = 5
	agent.stats.FailedToolCalls = 1
	agent.stats.Iterations = 3
	agent.mu.Unlock()
	agent.recordIteration()

	path, err := agent.DumpContext()
	if err != nil {
		t.Fatalf("DumpContext() failed: %v", err)
	}
	defer os.Remove(path)

	cfg2 := config.DefaultConfig()
	agent2 := NewAgent(cfg2)
	err = agent2.LoadContext(path)
	if err != nil {
		t.Fatalf("LoadContext() failed: %v", err)
	}

	stats := agent2.GetStats()
	if stats.InputTokens != 500 {
		t.Errorf("Expected 500 input tokens, got %d", stats.InputTokens)
	}
	if stats.OutputTokens != 300 {
		t.Errorf("Expected 300 output tokens, got %d", stats.OutputTokens)
	}
	if stats.ToolCalls != 5 {
		t.Errorf("Expected 5 tool calls, got %d", stats.ToolCalls)
	}
	if stats.FailedToolCalls != 1 {
		t.Errorf("Expected 1 failed tool call, got %d", stats.FailedToolCalls)
	}
	if stats.Iterations != 3 {
		t.Errorf("Expected 3 iterations, got %d", stats.Iterations)
	}
}

func TestDumpContext_SessionMetadata(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	agent.AddUserMessage("Test")
	agent.mu.Lock()
	agent.stats.Iterations = 5
	agent.compressionCount = 2
	agent.stats.InputTokens = 1000
	agent.stats.OutputTokens = 500
	agent.stats.ToolCalls = 10
	agent.stats.FailedToolCalls = 2
	agent.mu.Unlock()
	agent.recordIteration()

	path, err := agent.DumpContext()
	if err != nil {
		t.Fatalf("DumpContext() failed: %v", err)
	}
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read dump file: %v", err)
	}

	var dump ContextDump
	if err := json.Unmarshal(data, &dump); err != nil {
		t.Fatalf("Failed to parse dump file: %v", err)
	}

	if dump.Session.Iterations != 5 {
		t.Errorf("Expected 5 iterations in session, got %d", dump.Session.Iterations)
	}
	if dump.Session.CompressionCount != 2 {
		t.Errorf("Expected 2 compression count in session, got %d", dump.Session.CompressionCount)
	}
	if dump.Session.Stats.InputTokens != 1000 {
		t.Errorf("Expected 1000 input tokens in session stats, got %d", dump.Session.Stats.InputTokens)
	}
	if dump.Session.Stats.OutputTokens != 500 {
		t.Errorf("Expected 500 output tokens in session stats, got %d", dump.Session.Stats.OutputTokens)
	}
	if dump.Session.Stats.ToolCalls != 10 {
		t.Errorf("Expected 10 tool calls in session stats, got %d", dump.Session.Stats.ToolCalls)
	}
	if dump.Session.Stats.FailedToolCalls != 2 {
		t.Errorf("Expected 2 failed tool calls in session stats, got %d", dump.Session.Stats.FailedToolCalls)
	}
}

func TestLoadContext_Version1EmptyIterations(t *testing.T) {
	// This is a different test from TestLoadContext_EmptyIterations
	// It tests that version 1 with empty iterations returns the correct error
	tmpFile := filepath.Join(t.TempDir(), "v1_empty.json")
	content := `{"version": 1, "session": {}, "iterations": []}`
	os.WriteFile(tmpFile, []byte(content), 0644)

	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	err := agent.LoadContext(tmpFile)
	if err == nil {
		t.Error("Expected error when loading version 1 with empty iterations")
	}
}


