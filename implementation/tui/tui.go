package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"coding-agent/context"
	"coding-agent/stats"
)

// TUI represents the terminal user interface
type TUI struct {
	reader        *bufio.Reader
	output        []string
	stats         *stats.Stats
	width         int
	height        int
	cursorLine    int
	history       []string
	historyIndex  int
	maxHistory    int
}

// NewTUI creates a new TUI instance
func NewTUI(stats *stats.Stats) *TUI {
	return &TUI{
		reader:       bufio.NewReader(os.Stdin),
		output:       make([]string, 0),
		stats:        stats,
		width:        80,
		height:       24,
		history:      make([]string, 0),
		historyIndex: -1,
		maxHistory:   100,
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

// ReadInput reads input from the user with history navigation support
// Returns the input string and a boolean indicating if escape was pressed
func (t *TUI) ReadInput(prompt string) (string, bool, error) {
	fmt.Print(prompt)
	
	var input strings.Builder
	buffer := make([]byte, 1024)
	canceled := false
	
	for {
		n, err := t.reader.Read(buffer)
		if err != nil {
			return "", false, err
		}
		
		for i := 0; i < n; i++ {
			b := buffer[i]
			
			// Check for escape sequence
			if b == 27 { // ESC
				// Check for arrow keys or cancel
				if i+1 < n && buffer[i+1] == '[' {
					i++ // skip '['
					if i+1 < n {
						switch buffer[i+1] {
						case 'A': // Up arrow
							// Clear current line and show previous history
							fmt.Print("\r\033[2K")
							historyItem := t.GetPreviousHistory(input.String())
							input.Reset()
							input.WriteString(historyItem)
							fmt.Print(prompt + historyItem)
							i++ // skip 'A'
							continue
						case 'B': // Down arrow
							// Clear current line and show next history
							fmt.Print("\r\033[2K")
							historyItem := t.GetNextHistory(input.String())
							input.Reset()
							input.WriteString(historyItem)
							fmt.Print(prompt + historyItem)
							i++ // skip 'B'
							continue
						}
					}
				}
				// Plain ESC - cancel
				canceled = true
				fmt.Print("\r\033[2K" + prompt)
				return "", true, nil
			}
			
			switch b {
			case '\n', '\r':
				// Enter key - submit
				fmt.Println()
				return strings.TrimSpace(input.String()), canceled, nil
			case '\b', 127: // Backspace
				if input.Len() > 0 {
					input.Reset()
					// Redraw line
					fmt.Print("\r\033[2K" + prompt + input.String())
				}
			default:
				input.WriteByte(b)
				fmt.Printf("%c", b)
			}
		}
	}
}

// DisplayStats displays current statistics with context size
func (t *TUI) DisplayStats(ctx *context.Context) {
	summary := t.stats.GetSummary()

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("Runtime Statistics")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("Input Tokens:      %d\n", summary.InputTokens)
	fmt.Printf("Output Tokens:     %d\n", summary.OutputTokens)
	if ctx != nil {
		tokenCount := ctx.EstimateTokenCount()
		maxSize := ctx.GetMaxSize()
		percentage := 0.0
		if maxSize > 0 {
			percentage = float64(tokenCount) / float64(maxSize) * 100
		}
		fmt.Printf("Current Context:   %d / %d (%.1f%%)\n", tokenCount, maxSize, percentage)
	}
	fmt.Printf("Tokens/Second:     %.2f\n", summary.TokensPerSecond)
	fmt.Printf("Tool Calls:        %d\n", summary.TotalToolCalls)
	fmt.Printf("Failed Calls:      %d\n", summary.FailedToolCalls)
	fmt.Printf("Iterations:        %d\n", summary.IterationCount)
	fmt.Printf("Input History:     %d entries\n", t.GetHistoryCount())
	fmt.Printf("Uptime:            %v\n", summary.Uptime)
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

// AddToHistory adds a prompt to the input history
func (t *TUI) AddToHistory(input string) {
	if input == "" {
		return
	}
	// Don't add duplicates at the top of history
	if len(t.history) > 0 && t.history[0] == input {
		return
	}
	t.history = append([]string{input}, t.history...)
	if len(t.history) > t.maxHistory {
		t.history = t.history[:t.maxHistory]
	}
	t.historyIndex = -1
}

// GetPreviousHistory returns the previous item in history
func (t *TUI) GetPreviousHistory(currentInput string) string {
	if len(t.history) == 0 {
		return currentInput
	}
	// If at start or no current navigation, start from beginning
	if t.historyIndex == -1 {
		t.historyIndex = 0
		return t.history[0]
	}
	// Move to previous
	if t.historyIndex < len(t.history)-1 {
		t.historyIndex++
		return t.history[t.historyIndex]
	}
	// Already at oldest
	return t.history[t.historyIndex]
}

// GetNextHistory returns the next item in history
func (t *TUI) GetNextHistory(currentInput string) string {
	if len(t.history) == 0 {
		return ""
	}
	if t.historyIndex == -1 {
		return ""
	}
	// Move to next (more recent)
	if t.historyIndex > 0 {
		t.historyIndex--
		return t.history[t.historyIndex]
	}
	// At most recent, clear to empty
	t.historyIndex = -1
	return ""
}

// ClearHistory clears the input history
func (t *TUI) ClearHistory() {
	t.history = make([]string, 0)
	t.historyIndex = -1
}

// GetHistoryCount returns the number of items in history
func (t *TUI) GetHistoryCount() int {
	return len(t.history)
}

// ProcessCommand processes special commands
func (t *TUI) ProcessCommand(input string, ctx *context.Context) bool {
	switch strings.ToLower(input) {
	case "stats":
		t.DisplayStats(ctx)
		return true
	case "clear":
		t.ClearOutput()
		t.ClearScreen()
		t.DisplayWelcome()
		return true
	case "clear-history":
		t.ClearHistory()
		t.AddOutput("Input history cleared")
		return true
	case "quit":
		return false
	case "exit":
		return false
	}
	return true
}

// DisplayContextInfo displays the current context size information
func (t *TUI) DisplayContextInfo(ctx *context.Context) {
	if ctx == nil {
		return
	}
	tokenCount := ctx.EstimateTokenCount()
	maxSize := ctx.GetMaxSize()
	percentage := 0.0
	if maxSize > 0 {
		percentage = float64(tokenCount) / float64(maxSize) * 100
	}
	
	var indicator string
	if percentage > 90 {
		indicator = " ⚠⚠"
	} else if percentage > 75 {
		indicator = " ⚠"
	} else {
		indicator = ""
	}
	
	fmt.Printf("\r\033[2K[Tokens: %d / %d (%.1f%%)%s] ", tokenCount, maxSize, percentage, indicator)
}
