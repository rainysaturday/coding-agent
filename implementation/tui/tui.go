// Package tui implements the terminal user interface.
package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coding-agent/harness/agent"
	"github.com/coding-agent/harness/config"
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

// StreamingContentType represents the type of content being streamed.
type StreamingContentType int

const (
	StreamingContentTypeNormal StreamingContentType = iota
	StreamingContentTypeReasoning
)

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
// Ctrl+C can be pressed during agent operation to cancel.
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

// readLineWithHistory reads a line with Ctrl+P/Ctrl+N history navigation.
// Uses ReadString to read the full line at once, preventing character echo issues.
// In canonical mode, the terminal handles echoing, so we just need to process the input.
func (t *TUI) readLineWithHistory() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	var line []byte

	for {
		// Read the full line (blocks until Enter is pressed)
		input, err := reader.ReadString('\n')
		if err != nil {
			return string(line), err
		}

		// Remove trailing newline/carriage return
		input = strings.TrimRight(input, "\r\n")

		// Handle empty input (just Enter pressed)
		if input == "" {
			fmt.Println()
			return string(line), nil
		}

		// Check if this is a control character sequence (single character)
		if len(input) == 1 {
			switch input[0] {
			case 3: // Ctrl+C
				t.mu.Lock()
				t.cancelled = true
				t.mu.Unlock()
				fmt.Println("\n[Cancelled]")
				return "", fmt.Errorf("cancelled")
			case 16: // Ctrl+P - Previous history
				t.handleHistoryUp()
				continue // Continue to read next input
			case 14: // Ctrl+N - Next history
				t.handleHistoryDown()
				continue // Continue to read next input
			case 127, 8: // Backspace/Delete
				if len(line) > 0 {
					line = line[:len(line)-1]
					fmt.Print("\b \b")
				}
				continue
			}
		}

		// Handle escape sequences (arrow keys, etc.) - ignore them since we use Ctrl+P/N
		if len(input) > 0 && input[0] == 27 { // ESC
			continue
		}

		// Regular text input - append to line and return
		// Note: Terminal already echoed the characters in canonical mode
		line = append(line, []byte(input)...)
		return string(line), nil
	}
}

// handleHistoryUp navigates to the previous history entry.
func (t *TUI) handleHistoryUp() {
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
		fmt.Print(t.history[t.historyIndex])
	}
}

// handleHistoryDown navigates to the next history entry.
func (t *TUI) handleHistoryDown() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.history) == 0 {
		return
	}

	if t.historyIndex > 0 {
		t.historyIndex--
		fmt.Print("\r\033[K> ")
		fmt.Print(t.history[t.historyIndex])
	} else {
		fmt.Print("\r\033[K> ")
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
// This is the legacy method that defaults to normal content type.
func (t *TUI) StreamChunk(text string) {
	t.StreamChunkWithType(text, StreamingContentTypeNormal)
}

// StreamChunkWithType outputs a chunk of streaming text with a specific content type.
func (t *TUI) StreamChunkWithType(text string, contentType StreamingContentType) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Buffer the chunk
	t.streamBuffer.WriteString(text)

	// Apply appropriate color based on content type
	var color string
	switch contentType {
	case StreamingContentTypeReasoning:
		color = ColorDim
	default:
		color = ColorReset
	}

	// Print immediately without newline for smooth streaming
	if color != ColorReset {
		fmt.Printf("%s%s%s", color, text, ColorReset)
	} else {
		fmt.Print(text)
	}
}

// StreamReasoningChunk streams reasoning/thinking content with dim color.
func (t *TUI) StreamReasoningChunk(text string) {
	t.StreamChunkWithType(text, StreamingContentTypeReasoning)
}

// StreamNormalChunk streams regular content with normal color.
func (t *TUI) StreamNormalChunk(text string) {
	t.StreamChunkWithType(text, StreamingContentTypeNormal)
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

// ShowContextSize displays the current context size to the user.
func (t *TUI) ShowContextSize() {
	t.printContextSize()
	fmt.Println()
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
	ColorDim    = "\033[90m" // Dim/bright black for reasoning content
)

// Clear screen ANSI code
const (
	ClearScreen = "\033[2J\033[H"
)

// clearScreen clears the terminal screen.
func clearScreen() {
	fmt.Print(ClearScreen)
}
