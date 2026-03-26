package tui

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"coding-agent/context"
	"coding-agent/stats"
)

func TestNewTUI(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)

	if tui == nil {
		t.Fatal("NewTUI returned nil")
	}
	if tui.stats != s {
		t.Error("TUI stats not set correctly")
	}
}

func TestAddOutput(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)

	tui.AddOutput("line1")
	tui.AddOutput("line2")

	if len(tui.output) != 2 {
		t.Errorf("Expected 2 output lines, got %d", len(tui.output))
	}
	if tui.output[0] != "line1" {
		t.Errorf("Expected 'line1', got '%s'", tui.output[0])
	}
}

func TestAddOutputf(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)

	tui.AddOutputf("Hello %s", "World")

	if len(tui.output) != 1 {
		t.Errorf("Expected 1 output line, got %d", len(tui.output))
	}
	if tui.output[0] != "Hello World" {
		t.Errorf("Expected 'Hello World', got '%s'", tui.output[0])
	}
}

func TestClearOutput(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)

	tui.AddOutput("line1")
	tui.AddOutput("line2")
	tui.ClearOutput()

	if len(tui.output) != 0 {
		t.Errorf("Expected 0 output lines after clear, got %d", len(tui.output))
	}
}

func TestProcessCommand_Stats(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)
	ctx := context.NewContext("system", 1000)

	result := tui.ProcessCommand("stats", ctx)
	if !result {
		t.Error("ProcessCommand should return true for stats command")
	}
}

func TestProcessCommand_Clear(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)
	ctx := context.NewContext("system", 1000)

	tui.AddOutput("test line")

	result := tui.ProcessCommand("clear", ctx)
	if !result {
		t.Error("ProcessCommand should return true for clear command")
	}
	if len(tui.output) != 0 {
		t.Error("Output should be cleared after clear command")
	}
}

func TestProcessCommand_Quit(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)
	ctx := context.NewContext("system", 1000)

	result := tui.ProcessCommand("quit", ctx)
	if result {
		t.Error("ProcessCommand should return false for quit command")
	}
}

func TestProcessCommand_Exit(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)
	ctx := context.NewContext("system", 1000)

	result := tui.ProcessCommand("exit", ctx)
	if result {
		t.Error("ProcessCommand should return false for exit command")
	}
}

func TestProcessCommand_Invalid(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)
	ctx := context.NewContext("system", 1000)

	result := tui.ProcessCommand("invalid command", ctx)
	if !result {
		t.Error("ProcessCommand should return true for invalid command (passes to LLM)")
	}
}

func TestProcessCommand_CaseInsensitive(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)
	ctx := context.NewContext("system", 1000)

	commands := []string{"STATS", "Stats", "CLEAR", "Clear", "QUIT", "Quit"}
	for _, cmd := range commands {
		result := tui.ProcessCommand(cmd, ctx)
		if cmd == "QUIT" || cmd == "Quit" {
			if result {
				t.Errorf("ProcessCommand should return false for '%s'", cmd)
			}
		} else {
			if !result {
				t.Errorf("ProcessCommand should return true for '%s'", cmd)
			}
		}
	}
}

func TestProcessCommand_ClearHistory(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)
	ctx := context.NewContext("system", 1000)

	tui.AddToHistory("test history")
	if tui.GetHistoryCount() != 1 {
		t.Errorf("Expected 1 history entry, got %d", tui.GetHistoryCount())
	}

	result := tui.ProcessCommand("clear-history", ctx)
	if !result {
		t.Error("ProcessCommand should return true for clear-history command")
	}
	if tui.GetHistoryCount() != 0 {
		t.Error("History should be cleared after clear-history command")
	}
}

// Test with captured stdout
func TestDisplayOutput(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	s := stats.NewStats()
	tui := NewTUI(s)
	tui.AddOutput("test output")
	tui.DisplayOutput()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "test output") {
		t.Errorf("Expected output to contain 'test output', got '%s'", output)
	}
}

func TestStatsDisplay(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	s := stats.NewStats()
	s.AddInputTokens(100)
	s.AddOutputTokens(50)
	s.AddToolCall()

	tui := NewTUI(s)
	ctx := context.NewContext("system", 1000)
	tui.DisplayStats(ctx)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Runtime Statistics") {
		t.Error("Expected stats output to contain 'Runtime Statistics'")
	}
	if !strings.Contains(output, "100") {
		t.Error("Expected stats output to contain input token count")
	}
	if !strings.Contains(output, "50") {
		t.Error("Expected stats output to contain output token count")
	}
}

func TestWelcomeDisplay(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	s := stats.NewStats()
	tui := NewTUI(s)
	tui.DisplayWelcome()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Coding Agent") {
		t.Error("Expected welcome output to contain 'Coding Agent'")
	}
}

func TestHistoryNavigation(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)

	// Add some history
	tui.AddToHistory("prompt1")
	tui.AddToHistory("prompt2")
	tui.AddToHistory("prompt3")

	if tui.GetHistoryCount() != 3 {
		t.Errorf("Expected 3 history entries, got %d", tui.GetHistoryCount())
	}

	// Test up navigation
	result := tui.GetPreviousHistory("")
	if result != "prompt3" {
		t.Errorf("Expected 'prompt3', got '%s'", result)
	}

	result = tui.GetPreviousHistory("")
	if result != "prompt2" {
		t.Errorf("Expected 'prompt2', got '%s'", result)
	}

	// Test down navigation
	result = tui.GetNextHistory("")
	if result != "prompt3" {
		t.Errorf("Expected 'prompt3', got '%s'", result)
	}

	result = tui.GetNextHistory("")
	if result != "" {
		t.Errorf("Expected empty string at end of history, got '%s'", result)
	}
}

func TestHistoryMaxSize(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)
	tui.maxHistory = 3

	// Add more than max history
	for i := 0; i < 5; i++ {
		tui.AddToHistory("prompt" + string(rune('0'+i)))
	}

	if tui.GetHistoryCount() != 3 {
		t.Errorf("Expected max 3 history entries, got %d", tui.GetHistoryCount())
	}
}

func TestHistoryNoDuplicate(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)

	tui.AddToHistory("prompt1")
	tui.AddToHistory("prompt1") // Duplicate

	if tui.GetHistoryCount() != 1 {
		t.Errorf("Expected 1 history entry (no duplicates), got %d", tui.GetHistoryCount())
	}
}

func TestEmptyHistoryNavigation(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)

	// Empty history should not crash
	result := tui.GetPreviousHistory("current")
	if result != "current" {
		t.Errorf("Expected current input unchanged, got '%s'", result)
	}

	result = tui.GetNextHistory("current")
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}
}

func TestContextDisplay(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)
	ctx := context.NewContext("system", 1000)
	ctx.AddUserMessage("test message")

	// Just verify it doesn't crash
	tui.DisplayContextInfo(ctx)
}

func TestContextDisplayWarning(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)
	
	// Create context with small max size to trigger warning
	ctx := context.NewContext("system", 50)
	// Add enough content to exceed 75%
	for i := 0; i < 20; i++ {
		ctx.AddUserMessage("test message number " + string(rune('0'+i)))
	}

	// Just verify it doesn't crash
	tui.DisplayContextInfo(ctx)
}
