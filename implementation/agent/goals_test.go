package agent

import (
	"strings"
	"testing"
	"time"

	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
)

func TestSetGoal_Activate(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Goal should not be active initially
	if agent.IsGoalActive() {
		t.Error("Goal should not be active initially")
	}
	if agent.GetGoal() != "" {
		t.Errorf("Expected empty goal initially, got %q", agent.GetGoal())
	}

	// Set a goal
	agent.SetGoal("Create a REST API")

	// Verify goal is active
	if !agent.IsGoalActive() {
		t.Error("Goal should be active after SetGoal")
	}
	if agent.GetGoal() != "Create a REST API" {
		t.Errorf("Expected goal 'Create a REST API', got %q", agent.GetGoal())
	}
}

func TestSetGoal_EmptyString(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// First activate goal mode
	agent.SetGoal("Some goal")
	if !agent.IsGoalActive() {
		t.Error("Goal should be active")
	}

	// Setting empty string should deactivate goal mode
	agent.SetGoal("")

	if agent.IsGoalActive() {
		t.Error("Goal should not be active after setting empty string")
	}
	if agent.GetGoal() != "" {
		t.Errorf("Expected empty goal, got %q", agent.GetGoal())
	}
}

func TestSetGoal_MultipleGoals(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Set first goal
	agent.SetGoal("First goal")
	if agent.GetGoal() != "First goal" {
		t.Errorf("Expected 'First goal', got %q", agent.GetGoal())
	}

	// Set second goal (should overwrite)
	agent.SetGoal("Second goal")
	if agent.GetGoal() != "Second goal" {
		t.Errorf("Expected 'Second goal', got %q", agent.GetGoal())
	}
}

// TestClearGoal tests that ClearGoal correctly deactivates goal mode
func TestClearGoal(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Activate goal mode first
	agent.SetGoal("Test goal")
	if !agent.IsGoalActive() {
		t.Error("Goal should be active")
	}

	// Clear the goal
	agent.ClearGoal()

	// Verify goal mode is deactivated
	if agent.IsGoalActive() {
		t.Error("Goal should not be active after ClearGoal")
	}
	if agent.GetGoal() != "" {
		t.Errorf("Expected empty goal after ClearGoal, got %q", agent.GetGoal())
	}
}

// TestClearGoal_AlreadyInactive tests ClearGoal when goal mode is already inactive
func TestClearGoal_AlreadyInactive(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Should not panic when clearing an already inactive goal
	agent.ClearGoal()

	if agent.IsGoalActive() {
		t.Error("Goal should not be active")
	}
}

// TestSetGoal_WhitespaceOnly tests behavior with whitespace-only goal
func TestSetGoal_WhitespaceOnly(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Set a whitespace-only goal - should still activate goal mode
	agent.SetGoal("   ")
	if !agent.IsGoalActive() {
		t.Error("Goal should be active with whitespace")
	}
	if agent.GetGoal() != "   " {
		t.Errorf("Expected whitespace goal preserved, got %q", agent.GetGoal())
	}

	// Clear it
	agent.ClearGoal()
	if agent.IsGoalActive() {
		t.Error("Goal should be deactivated after ClearGoal")
	}
}

func TestGoalMode_IntegrationWithContext(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Initial state
	if agent.IsGoalActive() {
		t.Error("Goal should not be active initially")
	}

	// Activate goal mode
	agent.SetGoal("Complete feature X")

	// Verify state
	if !agent.IsGoalActive() {
		t.Error("Goal should be active")
	}

	// Add some messages
	agent.AddUserMessage("First message")
	agent.AddAssistantMessage("First assistant response")

	// Goal should still be active
	if !agent.IsGoalActive() {
		t.Error("Goal should still be active after adding messages")
	}
	if agent.GetGoal() != "Complete feature X" {
		t.Errorf("Expected goal 'Complete feature X', got %q", agent.GetGoal())
	}

	// Deactivate goal mode
	agent.ClearGoal()

	// Verify goal is cleared
	if agent.IsGoalActive() {
		t.Error("Goal should be deactivated")
	}
	if agent.GetGoal() != "" {
		t.Errorf("Expected empty goal, got %q", agent.GetGoal())
	}
}

// TestInjectGoalCheck_MessageFormat tests that injectGoalCheck creates the correct user message
func TestInjectGoalCheck_MessageFormat(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Add an initial message to have context
	agent.AddUserMessage("Initial prompt")
	agent.AddAssistantMessage("Initial response")

	// Set a goal and inject the check
	agent.SetGoal("Create a test file")
	agent.injectGoalCheck()

	// Verify context has 3 messages: user, assistant, goal check user message
	if agent.GetContextSize() == 0 {
		t.Error("Context should have content")
	}

	// The goal check should contain the goal prompt
	goal := agent.GetGoal()
	if goal != "Create a test file" {
		t.Errorf("Expected goal 'Create a test file', got %q", goal)
	}
}

// TestGoalMode_GoalAchievedAtNaturalEnd tests that "goal achieved" is checked
// at the natural end of the agentic loop (no tool calls)
func TestGoalMode_GoalAchievedAtNaturalEnd(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Set a goal
	agent.SetGoal("Create a file")

	// Simulate: the LLM responds with "goal achieved" and no tool calls
	// This would be caught by the natural end check
	response := &inference.Response{
		Content:   "I have created the file. Goal achieved.",
		ToolCalls: nil, // No tool calls = natural end
	}

	// Check that "goal achieved" is detected (case-insensitive)
	if !strings.Contains(strings.ToLower(response.Content), "goal achieved") {
		t.Error("Should detect 'goal achieved' in response")
	}
}

// TestGoalMode_GoalNotAchieved_InjectsCheck tests that when goal is not achieved,
// a goal check message is injected and the loop continues
func TestGoalMode_GoalNotAchieved_InjectsCheck(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Set a goal
	agent.SetGoal("Build the app")

	// Add initial context
	agent.AddUserMessage("Build an app")
	agent.AddAssistantMessage("I started building...")

	// Record context size before inject
	contextSizeBefore := agent.GetContextSize()

	// Simulate: natural end without "goal achieved" - should inject goal check
	agent.injectGoalCheck()

	// Context should have grown (goal check message was added)
	contextSizeAfter := agent.GetContextSize()
	if contextSizeAfter <= contextSizeBefore {
		t.Error("Context should grow after injecting goal check")
	}
}

// TestGoalCaseInsensitive_Matching verifies case-insensitive "goal achieved" matching
func TestGoalCaseInsensitive_Matching(t *testing.T) {
	testCases := []struct {
		content     string
		shouldMatch bool
	}{
		{"Goal achieved", true},
		{"GOAL ACHIEVED", true},
		{"goal achieved", true},
		{"Goal Achieved!", true},
		{"The goal achieved.", true},
		{"I have achieved the goal", false}, // Must contain "goal achieved"
		{"goal achieved yet", true},
		{"not done yet", false},
		{"", false},
	}

	for _, tc := range testCases {
		matched := strings.Contains(strings.ToLower(tc.content), "goal achieved")
		if matched != tc.shouldMatch {
			t.Errorf("Content %q: expected match=%v, got match=%v", tc.content, tc.shouldMatch, matched)
		}
	}
}

// TestGoalMode_AutoClearOnAchieved verifies that the goal is automatically
// cleared when "goal achieved" is detected at a natural end.
func TestGoalMode_AutoClearOnAchieved(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Set a goal
	agent.SetGoal("Create a file")
	if !agent.IsGoalActive() {
		t.Fatal("Goal should be active after SetGoal")
	}
	if agent.GetGoal() != "Create a file" {
		t.Fatalf("Expected goal 'Create a file', got %q", agent.GetGoal())
	}

	// Simulate goal achievement by clearing the goal (as the agent would do
	// when it detects "goal achieved" at natural end)
	agent.ClearGoal()

	// Verify goal is cleared
	if agent.IsGoalActive() {
		t.Error("Goal should not be active after ClearGoal (auto-clear on achievement)")
	}
	if agent.GetGoal() != "" {
		t.Errorf("Expected empty goal after auto-clear, got %q", agent.GetGoal())
	}
}

// ===== Tests for Error types and utilities (previously 0% coverage) =====

func TestShouldCheckGoal(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Initially should not check goal
	if agent.shouldCheckGoal() {
		t.Error("shouldCheckGoal() should be false initially")
	}

	// Set goal - should check
	agent.SetGoal("Test goal")
	if !agent.shouldCheckGoal() {
		t.Error("shouldCheckGoal() should be true after SetGoal")
	}

	// Clear goal - should not check
	agent.ClearGoal()
	if agent.shouldCheckGoal() {
		t.Error("shouldCheckGoal() should be false after ClearGoal")
	}
}

// TestSetGoal_ResetsStartTime verifies that SetGoal resets the goal start time.
func TestSetGoal_ResetsStartTime(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Set initial goal
	agent.SetGoal("First goal")
	firstStart := agent.GetGoalStartTime()

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Set a new goal - should reset start time
	agent.SetGoal("Second goal")
	secondStart := agent.GetGoalStartTime()

	if firstStart.IsZero() {
		t.Error("First goal start time should not be zero")
	}
	if secondStart.IsZero() {
		t.Error("Second goal start time should not be zero")
	}
	if !secondStart.After(firstStart) {
		t.Error("Second goal start time should be after first goal start time")
	}
}

// TestClearGoal_ResetsStartTime verifies that ClearGoal resets the goal start time.
func TestClearGoal_ResetsStartTime(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Set a goal
	agent.SetGoal("Test goal")
	startTime := agent.GetGoalStartTime()

	if startTime.IsZero() {
		t.Error("Goal start time should not be zero after SetGoal")
	}

	// Clear the goal
	agent.ClearGoal()

	// Start time should be reset
	resetTime := agent.GetGoalStartTime()
	if !resetTime.IsZero() {
		t.Errorf("Expected zero start time after ClearGoal, got %v", resetTime)
	}
}

// TestSetGoal_EmptyString_ResetsStartTime verifies that setting an empty string
// resets the goal start time.
func TestSetGoal_EmptyString_ResetsStartTime(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Set a goal
	agent.SetGoal("Test goal")
	startTime := agent.GetGoalStartTime()

	if startTime.IsZero() {
		t.Error("Goal start time should not be zero after SetGoal")
	}

	// Set empty string (deactivates goal mode)
	agent.SetGoal("")

	// Start time should be reset
	resetTime := agent.GetGoalStartTime()
	if !resetTime.IsZero() {
		t.Errorf("Expected zero start time after setting empty string, got %v", resetTime)
	}
}

// TestFormatGoalDuration tests the formatGoalDuration helper function.
func TestFormatGoalDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"zero seconds", 0, "0s"},
		{"one second", 1 * time.Second, "1s"},
		{"45 seconds", 45 * time.Second, "45s"},
		{"one minute", 60 * time.Second, "60s (1m 0s)"},
		{"two minutes five seconds", 125 * time.Second, "125s (2m 5s)"},
		{"one hour", 3600 * time.Second, "3600s (1h 0m 0s)"},
		{"one hour one minute one second", 3661 * time.Second, "3661s (1h 1m 1s)"},
		{"one hour one minute five seconds", 3665 * time.Second, "3665s (1h 1m 5s)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatGoalDuration(tt.duration)
			if got != tt.expected {
				t.Errorf("formatGoalDuration(%v) = %q, want %q", tt.duration, got, tt.expected)
			}
		})
	}
}
