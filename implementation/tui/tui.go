package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"coding-agent/stats"
)

// TUI represents the terminal user interface
type TUI struct {
	reader     *bufio.Reader
	output     []string
	stats      *stats.Stats
	width      int
	height     int
	cursorLine int
}

// NewTUI creates a new TUI instance
func NewTUI(stats *stats.Stats) *TUI {
	return &TUI{
		reader: bufio.NewReader(os.Stdin),
		output: make([]string, 0),
		stats:  stats,
		width:  80,
		height: 24,
	}
}

// ClearScreen clears the terminal screen
func (t *TUI) ClearScreen() {
	// ANSI escape code to clear screen and move cursor to home
	fmt.Print("\033[2J\033[H")
}

// DisplayOutput displays all output messages
func (t *TUI) DisplayOutput() {
	for _, line := range t.output {
		fmt.Println(line)
	}
}

// AddOutput adds a line to the output
func (t *TUI) AddOutput(line string) {
	t.output = append(t.output, line)
}

// AddOutputf adds a formatted line to the output
func (t *TUI) AddOutputf(format string, args ...interface{}) {
	t.output = append(t.output, fmt.Sprintf(format, args...))
}

// ClearOutput clears the output buffer
func (t *TUI) ClearOutput() {
	t.output = make([]string, 0)
}

// ReadInput reads input from the user
func (t *TUI) ReadInput(prompt string) (string, error) {
	fmt.Print(prompt)
	input, err := t.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

// DisplayStats displays current statistics
func (t *TUI) DisplayStats() {
	summary := t.stats.GetSummary()

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("Runtime Statistics")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("Input Tokens:     %d\n", summary.InputTokens)
	fmt.Printf("Output Tokens:    %d\n", summary.OutputTokens)
	fmt.Printf("Tokens/Second:    %.2f\n", summary.TokensPerSecond)
	fmt.Printf("Tool Calls:       %d\n", summary.TotalToolCalls)
	fmt.Printf("Failed Calls:     %d\n", summary.FailedToolCalls)
	fmt.Printf("Iterations:       %d\n", summary.IterationCount)
	fmt.Printf("Uptime:           %v\n", summary.Uptime)
	fmt.Println(strings.Repeat("=", 50))
}

// DisplayPrompt displays the input prompt
func (t *TUI) DisplayPrompt() {
	fmt.Print("\n> ")
}

// DisplayWelcome displays the welcome message
func (t *TUI) DisplayWelcome() {
	t.ClearScreen()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("  Minimal Coding Agent Harness")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()
	fmt.Println("Type your request below. Use Ctrl+C to exit.")
	fmt.Println("Type 'stats' to view statistics, 'clear' to clear output.")
	fmt.Println()
}

// DisplayError displays an error message
func (t *TUI) DisplayError(format string, args ...interface{}) {
	fmt.Printf("\033[31mERROR: %s\033[0m\n", fmt.Sprintf(format, args...))
}

// DisplayInfo displays an info message
func (t *TUI) DisplayInfo(format string, args ...interface{}) {
	fmt.Printf("\033[36mINFO: %s\033[0m\n", fmt.Sprintf(format, args...))
}

// DisplaySuccess displays a success message
func (t *TUI) DisplaySuccess(format string, args ...interface{}) {
	fmt.Printf("\033[32m%s\033[0m\n", fmt.Sprintf(format, args...))
}

// ProcessCommand processes special commands
func (t *TUI) ProcessCommand(input string) bool {
	switch strings.ToLower(input) {
	case "stats":
		t.DisplayStats()
		return true
	case "clear":
		t.ClearOutput()
		t.ClearScreen()
		t.DisplayWelcome()
		return true
	case "quit":
		return false
	case "exit":
		return false
	}
	return true
}
