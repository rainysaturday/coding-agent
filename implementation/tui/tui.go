// Package tui implements the terminal user interface.
package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"

	"github.com/coding-agent/harness/agent"
	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
)

// TUI represents the terminal user interface.
type TUI struct {
	config                    *config.Config
	output                    []string
	history                   []string
	historyIndex              int
	maxHistory                int
	cancelled                 bool
	mu                        sync.Mutex
	streaming                 bool
	streamBuffer              strings.Builder // Normal (non-reasoning) content
	reasoningBuffer           strings.Builder // Reasoning/thinking content
	contextSize               int
	lastContentWasToolCall    bool
	maxContextSize            int
	inputLine                 string // Current input line buffer (shared with history navigation)
	currentInput              string // Stores typed input when navigating to history
	transitionedFromReasoning bool   // Whether we've transitioned from reasoning to normal content
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
		inputLine:      "",
		currentInput:   "",
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

	// Reset input buffer for new input
	t.mu.Lock()
	t.inputLine = ""
	t.mu.Unlock()

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

// readLineWithHistory reads a line with arrow key and Ctrl+P/N history navigation.
// Uses raw mode to capture individual characters and escape sequences.
func (t *TUI) readLineWithHistory() (string, error) {
	fd := int(os.Stdin.Fd())

	// Save current terminal state
	oldState, err := term.GetState(fd)
	if err != nil {
		// Fallback to buffered reading if raw mode is not available
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimRight(input, "\r\n"), nil
	}
	defer term.Restore(fd, oldState)

	// Enter raw mode
	newState, err := term.MakeRaw(fd)
	if err != nil {
		return "", err
	}
	defer term.Restore(fd, newState)

	for {
		// Read a single byte
		var buf [1]byte
		_, err := os.Stdin.Read(buf[:])
		if err != nil {
			return t.inputLine, err
		}
		b := buf[0]

		// Handle escape sequences (arrow keys start with ESC = 0x1b)
		if b == 0x1b {
			// Read next two bytes for escape sequence
			var seq [2]byte
			_, err1 := os.Stdin.Read(seq[0:1])
			if err1 == nil {
				_, err2 := os.Stdin.Read(seq[1:2])
				_ = err2
				if seq[0] == '[' {
					switch seq[1] {
					case 'A': // Up arrow
						t.handleHistoryUp()
						continue
					case 'B': // Down arrow
						t.handleHistoryDown()
						continue
					}
				}
			}
			// Not a recognized escape sequence, treat as regular characters
			t.inputLine += string(b)
			t.inputLine += string(seq[0])
			t.inputLine += string(seq[1])
			os.Stdout.Write([]byte{b, seq[0], seq[1]})
			continue
		}

		// Handle control characters
		switch b {
		case 3: // Ctrl+C
			t.mu.Lock()
			t.cancelled = true
			t.mu.Unlock()
			fmt.Println("\n[Cancelled]")
			return "", fmt.Errorf("cancelled")
		case 16: // Ctrl+P - Previous history
			t.handleHistoryUp()
			continue
		case 14: // Ctrl+N - Next history
			t.handleHistoryDown()
			continue
		case 13: // Enter/Carriage Return
			fmt.Println()
			return t.inputLine, nil
		case 127, 8: // Backspace/Delete
			if len(t.inputLine) > 0 {
				t.inputLine = t.inputLine[:len(t.inputLine)-1]
				fmt.Print("\b \b")
			}
			continue
		case 4: // Ctrl+D - End of input
			if len(t.inputLine) == 0 {
				fmt.Println("\nGoodbye!")
				return "", fmt.Errorf("EOF")
			}
			// If there's text, treat as Enter
			fmt.Println()
			return t.inputLine, nil
		default:
			// Regular printable character
			if b >= 32 && b < 127 || b >= 0x80 {
				t.inputLine += string(b)
				os.Stdout.Write([]byte{b})
			}
		}
	}
}

// handleHistoryUp navigates to the previous history entry.
func (t *TUI) handleHistoryUp() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.history) == 0 {
		return
	}

	// Save current typed input when first entering history mode
	if t.historyIndex == -1 {
		t.currentInput = t.inputLine
	}

	if t.historyIndex == -1 {
		t.historyIndex = 0
	} else if t.historyIndex < len(t.history)-1 {
		t.historyIndex++
	}

	// Clear entire line and set input to history entry
	t.inputLine = ""
	t.printContextSizeInternal()
	fmt.Printf("> ")
	if t.historyIndex < len(t.history) {
		t.inputLine = t.history[t.historyIndex]
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
		t.printContextSizeInternal()
		fmt.Printf("> ")
		t.inputLine = t.history[t.historyIndex]
		fmt.Print(t.history[t.historyIndex])
	} else {
		// Exiting history mode - restore the current typed input
		t.printContextSizeInternal()
		fmt.Printf("> ")
		t.inputLine = t.currentInput
		if t.currentInput != "" {
			fmt.Print(t.currentInput)
		}
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
	t.StreamChunkWithType(text, inference.StreamingContentTypeNormal)
}

// StreamChunkWithType outputs a chunk of streaming text with a specific content type.
// For tool call parameter updates (indicated by "[Tool Call] " prefix), it uses
// ANSI cursor positioning to update the line in place instead of appending.
func (t *TUI) StreamChunkWithType(text string, contentType inference.StreamingContentType) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Apply appropriate color and buffer based on content type
	switch contentType {
	case inference.StreamingContentTypeReasoning:
		t.reasoningBuffer.WriteString(text)
		fmt.Printf("%s%s%s", ColorDim, text, ColorReset)
	case inference.StreamingContentTypeGoal:
		// Goal messages are displayed in magenta to stand out
		t.streamBuffer.WriteString(text)
		fmt.Printf("%s%s%s", ColorMagenta, text, ColorReset)
	case inference.StreamingContentTypeCompression:
		// Context compression messages are displayed in magenta to stand out
		t.streamBuffer.WriteString(text)
		fmt.Printf("%s%s%s", ColorMagenta, text, ColorReset)
	default:
		// Transitioning from reasoning to normal content - add separator
		if t.transitionedFromReasoning == false && t.reasoningBuffer.Len() > 0 {
			fmt.Println()
			fmt.Printf("%s--- Thinking Complete ---%s\n\n", ColorDim, ColorReset)
			t.transitionedFromReasoning = true
		}
		t.streamBuffer.WriteString(text)

		// Check if this is a tool call parameter update
		// Tool call updates start with "[Tool Call] " prefix (e.g., "[Tool Call] bash (command: "value")")
		// We want to update the previous [Tool Call] line in place using ANSI cursor positioning
		if strings.HasPrefix(text, "[Tool Call] ") {
			// Use ANSI cursor positioning to update the line in place:
			// \033[2K - Clear entire current line
			// \r - Return cursor to start of line
			// Then print the new full tool call with parameters
			fmt.Print("\033[2K\r" + text)
			t.lastContentWasToolCall = true
		} else {
			// For non-tool-call content: ensure we start on a new line if previous content was a tool call
			if t.lastContentWasToolCall {
				fmt.Print("\n")
			}
			// Regular content - just print it
			fmt.Print(text)
			t.lastContentWasToolCall = false
		}
	}
}

// StreamReasoningChunk streams reasoning/thinking content with dim color.
func (t *TUI) StreamReasoningChunk(text string) {
	t.StreamChunkWithType(text, inference.StreamingContentTypeReasoning)
}

// StreamNormalChunk streams regular content with normal color.
func (t *TUI) StreamNormalChunk(text string) {
	t.StreamChunkWithType(text, inference.StreamingContentTypeNormal)
}

// StreamGoalChunk streams goal-related content with magenta color.
func (t *TUI) StreamGoalChunk(text string) {
	t.StreamChunkWithType(text, inference.StreamingContentTypeGoal)
}

// StreamEnd finalizes a streaming session.
func (t *TUI) StreamEnd() {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Ensure we're on a new line
	fmt.Println()

	// Store reasoning content if present
	reasoning := t.reasoningBuffer.String()
	if reasoning != "" {
		t.output = append(t.output, "[Reasoning] "+reasoning)
	}

	// Store the normal (non-reasoning) content
	content := t.streamBuffer.String()
	if content != "" {
		t.output = append(t.output, content)
	}

	t.streamBuffer.Reset()
	t.reasoningBuffer.Reset()
	t.transitionedFromReasoning = false
	t.streaming = false
}

// StartStream begins a new streaming session.
func (t *TUI) StartStream() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.streaming = true
	t.streamBuffer.Reset()
	t.reasoningBuffer.Reset()
	t.transitionedFromReasoning = false
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
	if stats.CompressionCount > 0 {
		fmt.Printf("Compressions:      %d\n", stats.CompressionCount)
	}
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
	if t.maxHistory > 0 && len(t.history) > t.maxHistory {
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
	t.mu.Lock()
	defer t.mu.Unlock()
	t.printContextSizeInternal()
	fmt.Println()
}

// printContextSize prints the current context size indicator.
func (t *TUI) printContextSize() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.printContextSizeInternal()
}

// printContextSizeInternal prints the current context size indicator.
// Caller must hold t.mu.
func (t *TUI) printContextSizeInternal() {
	size := t.contextSize
	max := t.maxContextSize

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
	ColorReset   = "\033[0m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m" // Magenta for goal messages
	ColorCyan    = "\033[36m"
	ColorDim     = "\033[90m" // Dim/bright black for reasoning content
)

// Clear screen ANSI code
const (
	ClearScreen = "\033[2J\033[H"
)

// clearScreen clears the terminal screen.
func clearScreen() {
	fmt.Print(ClearScreen)
}
