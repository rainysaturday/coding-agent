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
	config         *config.Config
	output         []string
	history        []string
	historyIndex   int
	maxHistory     int
	cancelled      bool
	mu             sync.Mutex
	streaming      bool
	streamBuffer   strings.Builder
	contextSize    int
	maxContextSize int
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
		config:         cfg,
		output:         make([]string, 0),
		history:        make([]string, 0),
		historyIndex:   -1,
		maxHistory:     maxHistory,
		contextSize:    0,
		maxContextSize: cfg.ContextSize,
	}
}

// Prompt displays the prompt and reads user input with history navigation.
// Escape key can be pressed during agent operation to cancel.
func (t *TUI) Prompt() (string, error) {
	t.mu.Lock()
	t.cancelled = false
	t.mu.Unlock()

	// Display context size
	t.printContextSize()

	// Display prompt
	fmt.Printf("> ")

	// Read input with history navigation support
	input, err := t.readLineWithHistory()
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

// HandleEscape checks if escape was pressed and cancels if needed.
// This can be called periodically during long operations.
func (t *TUI) HandleEscape() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cancelled
}

// readLineWithHistory reads a line with arrow key history navigation.
func (t *TUI) readLineWithHistory() (string, error) {
	var line []byte
	reader := bufio.NewReader(os.Stdin)
	
	for {
		char, err := reader.ReadByte()
		if err != nil {
			return string(line), err
		}

		// Handle escape sequences (arrow keys)
		if char == 27 { // ESC
			// Read the rest of the escape sequence
			next1, err := reader.ReadByte()
			if err != nil {
				continue
			}
			if next1 == 91 { // '['
				next2, err := reader.ReadByte()
				if err != nil {
					continue
				}
				switch next2 {
				case 65: // Up arrow
					t.handleHistoryUpBytes(&line)
				case 66: // Down arrow
					t.handleHistoryDownBytes(&line)
				}
				continue
			}
			// Not an arrow key, just continue
			continue
		}

		// Handle control characters
		switch char {
		case 127, 8: // Backspace
			if len(line) > 0 {
				line = line[:len(line)-1]
				fmt.Print("\b \b")
			}
		case 13, 10: // Enter
			fmt.Println()
			return string(line), nil
		case 3: // Ctrl+C
			t.mu.Lock()
			t.cancelled = true
			t.mu.Unlock()
			fmt.Println("\n[Cancelled]")
			return "", fmt.Errorf("cancelled")
		default:
			if char >= 32 && char < 127 { // Printable ASCII
				line = append(line, char)
				fmt.Printf("%c", char)
			}
		}
	}
}

// handleHistoryUpBytes navigates to the previous history entry using byte slice.
func (t *TUI) handleHistoryUpBytes(line *[]byte) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.history) == 0 {
		return
	}

	if t.historyIndex == -1 {
		t.historyIndex = 0
	} else if t.historyIndex < len(t.history)-1 {
		t.historyIndex++
	}

	// Clear line and display history entry
	fmt.Print("\r\033[K> ")
	if t.historyIndex < len(t.history) {
		*line = []byte(t.history[t.historyIndex])
		fmt.Print(t.history[t.historyIndex])
	}
}

// handleHistoryDownBytes navigates to the next history entry using byte slice.
func (t *TUI) handleHistoryDownBytes(line *[]byte) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.history) == 0 {
		return
	}

	if t.historyIndex > 0 {
		t.historyIndex--
		fmt.Print("\r\033[K> ")
		*line = []byte(t.history[t.historyIndex])
		fmt.Print(t.history[t.historyIndex])
	} else {
		fmt.Print("\r\033[K> ")
		*line = []byte{}
		t.historyIndex = -1
	}
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

// StreamChunk outputs a chunk of streaming text immediately.
func (t *TUI) StreamChunk(text string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Buffer the chunk
	t.streamBuffer.WriteString(text)

	// Print immediately without newline for smooth streaming
	fmt.Print(text)
}

// StreamEnd finalizes a streaming session.
func (t *TUI) StreamEnd() {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Ensure we're on a new line
	fmt.Println()

	// Store the complete streamed content in output
	content := t.streamBuffer.String()
	if content != "" {
		t.output = append(t.output, content)
	}
	t.streamBuffer.Reset()
	t.streaming = false
}

// StartStream begins a new streaming session.
func (t *TUI) StartStream() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.streaming = true
	t.streamBuffer.Reset()
}

// IsStreaming returns whether we're currently streaming.
func (t *TUI) IsStreaming() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.streaming
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
	size := t.contextSize
	max := t.maxContextSize
	t.mu.Unlock()

	fmt.Println("==================================================")
	fmt.Println("Runtime Statistics")
	fmt.Println("==================================================")
	fmt.Printf("Input Tokens:      %d\n", stats.InputTokens)
	fmt.Printf("Output Tokens:     %d\n", stats.OutputTokens)
	fmt.Printf("Tokens/Second:     %.1f\n", stats.TokensPerSecond)
	fmt.Printf("Context Size:      %d / %d", size, max)
	if max > 0 {
		fmt.Printf(" (%.1f%%)", float64(size)/float64(max)*100)
	}
	fmt.Println()
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

// SetContextSize updates the current context size.
func (t *TUI) SetContextSize(size, max int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.contextSize = size
	t.maxContextSize = max
}

// printContextSize prints the current context size indicator.
func (t *TUI) printContextSize() {
	t.mu.Lock()
	size := t.contextSize
	max := t.maxContextSize
	t.mu.Unlock()

	if max > 0 {
		percentage := float64(size) / float64(max) * 100
		var indicator string
		var color string

		switch {
		case percentage < 50:
			indicator = "✓"
			color = ColorGreen
		case percentage < 75:
			indicator = "⚠"
			color = ColorYellow
		case percentage < 90:
			indicator = "⚠⚠"
			color = ColorYellow
		default:
			indicator = "⚠⚠⚠"
			color = ColorRed
		}

		fmt.Printf("%s[Context: %d / %d (%.1f%%) %s]%s ", color, size, max, percentage, indicator, ColorReset)
	} else {
		fmt.Printf("[Context: %d tokens] ", size)
	}
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
