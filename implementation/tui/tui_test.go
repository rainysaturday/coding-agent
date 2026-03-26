package tui

import (
	"bytes"
	"os"
	"strings"
	"testing"

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

	result := tui.ProcessCommand("stats")
	if !result {
		t.Error("ProcessCommand should return true for stats command")
	}
}

func TestProcessCommand_Clear(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)

	tui.AddOutput("test line")

	result := tui.ProcessCommand("clear")
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

	result := tui.ProcessCommand("quit")
	if result {
		t.Error("ProcessCommand should return false for quit command")
	}
}

func TestProcessCommand_Exit(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)

	result := tui.ProcessCommand("exit")
	if result {
		t.Error("ProcessCommand should return false for exit command")
	}
}

func TestProcessCommand_Invalid(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)

	result := tui.ProcessCommand("invalid command")
	if !result {
		t.Error("ProcessCommand should return true for invalid command (passes to LLM)")
	}
}

func TestProcessCommand_CaseInsensitive(t *testing.T) {
	s := stats.NewStats()
	tui := NewTUI(s)

	commands := []string{"STATS", "Stats", "CLEAR", "Clear", "QUIT", "Quit"}
	for _, cmd := range commands {
		result := tui.ProcessCommand(cmd)
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
	tui.DisplayStats()

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
