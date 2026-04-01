// Package tui implements the terminal user interface.
package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/agent"
)

// TUI represents the terminal user interface.
type TUI struct {
	config       *config.Config
	output       []string
	history      []string
	historyIndex int
	maxHistory   int
	cancelled    bool
	mu           sync.Mutex
}

// NewTUI creates a new TUI instance.
func NewTUI(cfg *config.Config) *TUI {
	maxHistory := 100
	if val := os.Getenv("CODING_AGENT_MAX_HISTORY"); val != "" {
		if n, err := fmt.Sscanf(val, "%d", &maxHistory); err != nil || n == 0 {
			maxHistory = 100
		}
	}

	return &TUI{
		config:       cfg,
		output:       make([]string, 0),
		history:      make([]string, 0),
		historyIndex: -1,
		maxHistory:   maxHistory,
	}
}

// Prompt displays the prompt and reads user input.
func (t *TUI) Prompt() (string, error) {
	t.mu.Lock()
	t.cancelled = false
	t.mu.Unlock()

	// Display context size
	t.printContextSize()

	// Display prompt
	fmt.Printf("> ")

	// Read input line
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		if t.cancelled {
			return "", fmt.Errorf("cancelled")
		}
		return "", err
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}

	// Add to history
	t.addToHistory(input)

	return input, nil
}

// AddOutput adds output to be displayed.
func (t *TUI) AddOutput(message string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.output = append(t.output, message)
	fmt.Println(message)
}

// AddOutputf formats and adds output.
func (t *TUI) AddOutputf(format string, args ...interface{}) {
	t.AddOutput(fmt.Sprintf(format, args...))
}

// ClearOutput clears the output display.
func (t *TUI) ClearOutput() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.output = make([]string, 0)
	clearScreen()
}

// DisplayStats displays the runtime statistics.
func (t *TUI) DisplayStats(stats *agent.Stats) {
	t.mu.Lock()
	defer t.mu.Unlock()

	fmt.Println("==================================================")
	fmt.Println("Runtime Statistics")
	fmt.Println("==================================================")
	fmt.Printf("Input Tokens:      %d\n", stats.InputTokens)
	fmt.Printf("Output Tokens:     %d\n", stats.OutputTokens)
	fmt.Printf("Tool Calls:        %d\n", stats.ToolCalls)
	fmt.Printf("Failed Calls:      %d\n", stats.FailedToolCalls)
	fmt.Printf("Iterations:        %d\n", stats.Iterations)
	if !stats.StartTime.IsZero() {
		fmt.Printf("Uptime:            %s\n", time.Since(stats.StartTime).Round(time.Second))
	}
	fmt.Println("==================================================")
}

// AddToHistory adds a prompt to the history.
func (t *TUI) addToHistory(prompt string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Add to beginning of history (newest first)
	t.history = append([]string{prompt}, t.history...)

	// Trim if exceeds max
	if len(t.history) > t.maxHistory {
		t.history = t.history[:t.maxHistory]
	}

	t.historyIndex = -1
}

// NavigateHistory navigates through history.
func (t *TUI) NavigateHistory(direction int) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.history) == 0 {
		return ""
	}

	t.historyIndex += direction

	// Clamp index
	if t.historyIndex < 0 {
		t.historyIndex = 0
	}
	if t.historyIndex >= len(t.history) {
		t.historyIndex = len(t.history)
		return "" // Empty for "new" input
	}

	return t.history[t.historyIndex]
}

// ClearHistory clears the input history.
func (t *TUI) ClearHistory() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.history = make([]string, 0)
	t.historyIndex = -1
	fmt.Println("History cleared.")
}

// CancelOperation cancels the current operation.
func (t *TUI) CancelOperation() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.cancelled = true
}

// printContextSize prints the current context size indicator.
func (t *TUI) printContextSize() {
	// This will be updated by the agent when available
	// For now, just print a placeholder
}

// printColored prints text with ANSI color.
func printColored(color, text string) {
	fmt.Printf("%s%s%s", color, text, ColorReset)
}

// ANSI color codes
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
)

// Clear screen ANSI code
const (
	ClearScreen = "\033[2J\033[H"
)

// clearScreen clears the terminal screen.
func clearScreen() {
	fmt.Print(ClearScreen)
}
