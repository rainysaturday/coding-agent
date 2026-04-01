package tui

import (
	"testing"

	"github.com/coding-agent/harness/agent"
	"github.com/coding-agent/harness/config"
)

func TestNewTUI(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	if tui == nil {
		t.Fatal("NewTUI() returned nil")
	}

	if tui.config != cfg {
		t.Error("NewTUI() did not store config")
	}

	if tui.history == nil {
		t.Error("NewTUI() history is nil")
	}

	if tui.output == nil {
		t.Error("NewTUI() output is nil")
	}

	if tui.maxHistory != 100 {
		t.Errorf("Expected maxHistory 100, got %d", tui.maxHistory)
	}
}

func TestAddToHistory(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Add prompts to history
	tui.addToHistory("first prompt")
	tui.addToHistory("second prompt")
	tui.addToHistory("third prompt")

	// Verify history by navigating through it
	result := tui.NavigateHistory(1)
	if result != "third prompt" {
		t.Errorf("Expected 'third prompt', got '%s'", result)
	}

	result = tui.NavigateHistory(1)
	if result != "second prompt" {
		t.Errorf("Expected 'second prompt', got '%s'", result)
	}

	result = tui.NavigateHistory(1)
	if result != "first prompt" {
		t.Errorf("Expected 'first prompt', got '%s'", result)
	}
}

func TestAddToHistoryMaxLimit(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)
	tui.maxHistory = 3

	// Add more prompts than max
	for i := 0; i < 5; i++ {
		tui.addToHistory("prompt " + string(rune('0'+i)))
	}

	// Navigate through history to verify max limit
	count := 0
	for {
		result := tui.NavigateHistory(1)
		if result == "" {
			break
		}
		count++
	}
	if count != 3 {
		t.Errorf("Expected max 3 history entries, got %d", count)
	}
}

func TestNavigateHistory(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Add some history (newest first in storage)
	tui.addToHistory("prompt 1")
	tui.addToHistory("prompt 2")
	tui.addToHistory("prompt 3")

	// Navigate forward through history (direction 1)
	// Should go from newest to oldest
	result := tui.NavigateHistory(1)
	if result != "prompt 3" {
		t.Errorf("Expected 'prompt 3', got '%s'", result)
	}

	result = tui.NavigateHistory(1)
	if result != "prompt 2" {
		t.Errorf("Expected 'prompt 2', got '%s'", result)
	}

	result = tui.NavigateHistory(1)
	if result != "prompt 1" {
		t.Errorf("Expected 'prompt 1', got '%s'", result)
	}

	// Past end of history, should return empty
	result = tui.NavigateHistory(1)
	if result != "" {
		t.Errorf("Expected empty past end, got '%s'", result)
	}

	// Navigate backward (direction -1)
	// From past-end, going back should get last item
	result = tui.NavigateHistory(-1)
	if result != "prompt 1" {
		t.Errorf("Expected 'prompt 1' going back, got '%s'", result)
	}

	result = tui.NavigateHistory(-1)
	if result != "prompt 2" {
		t.Errorf("Expected 'prompt 2' going back, got '%s'", result)
	}

	result = tui.NavigateHistory(-1)
	if result != "prompt 3" {
		t.Errorf("Expected 'prompt 3' going back, got '%s'", result)
	}

	// Past beginning, clamps to first item (doesn't go negative)
	result = tui.NavigateHistory(-1)
	if result != "prompt 3" {
		t.Errorf("Expected 'prompt 3' (clamped), got '%s'", result)
	}
}

func TestNavigateHistoryEmpty(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	result := tui.NavigateHistory(1)
	if result != "" {
		t.Errorf("Expected empty for empty history, got '%s'", result)
	}
}

func TestClearHistory(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Add some history
	tui.addToHistory("prompt 1")
	tui.addToHistory("prompt 2")

	// Clear history
	tui.ClearHistory()

	// Try to navigate - should be empty
	result := tui.NavigateHistory(1)
	if result != "" {
		t.Errorf("Expected empty result after clear, got '%s'", result)
	}
}

func TestCancelOperation(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Cancel operation - just verify it doesn't panic
	tui.CancelOperation()
}

func TestAddOutput(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// This would normally print to stdout, but we can't easily test that
	// We'll just verify it doesn't panic
	tui.AddOutput("test output")
	tui.AddOutputf("formatted %s", "output")
}

func TestClearOutput(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Add some output
	tui.AddOutput("test 1")
	tui.AddOutput("test 2")

	// Clear output - just verify it doesn't panic
	tui.ClearOutput()
}

func TestDisplayStats(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Create mock stats using the correct type
	stats := &agent.Stats{
		InputTokens:     100,
		OutputTokens:    50,
		ToolCalls:       5,
		FailedToolCalls: 1,
		Iterations:      3,
	}

	// This would print to stdout, just verify it doesn't panic
	tui.DisplayStats(stats)
}

func TestPrintColored(t *testing.T) {
	// Just verify it doesn't panic
	printColored(ColorRed, "test")
	printColored(ColorGreen, "test")
	printColored(ColorYellow, "test")
	printColored(ColorBlue, "test")
	printColored(ColorCyan, "test")
}

func TestColors(t *testing.T) {
	if ColorReset == "" {
		t.Error("ColorReset is empty")
	}
	if ColorRed == "" {
		t.Error("ColorRed is empty")
	}
	if ColorGreen == "" {
		t.Error("ColorGreen is empty")
	}
	if ColorYellow == "" {
		t.Error("ColorYellow is empty")
	}
	if ColorBlue == "" {
		t.Error("ColorBlue is empty")
	}
	if ColorCyan == "" {
		t.Error("ColorCyan is empty")
	}
}
