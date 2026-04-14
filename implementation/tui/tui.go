package tui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"coding-agent/implementation/agent"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
	ColorDim    = "\033[90m"
)

type TUI struct {
	history     []string
	historyMax  int
	historyIdx  int
	draftInput  string
	lastOutput  string
	streaming   bool
	toolVerbose bool
}

func New(historyMax int) *TUI {
	if historyMax <= 0 {
		historyMax = 100
	}
	return &TUI{historyMax: historyMax, historyIdx: -1}
}

func (t *TUI) StreamReasoningChunk(text string) {
	fmt.Print(ColorDim + text + ColorReset)
}

func (t *TUI) StreamNormalChunk(text string) {
	fmt.Print(text)
	t.lastOutput += text
}

func (t *TUI) StreamToolEvent(text string) {
	fmt.Println("\n" + ColorCyan + text + ColorReset)
}

func (t *TUI) Println(text string) {
	fmt.Println(text)
}

func (t *TUI) Run(ctx context.Context, ag *agent.Agent) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	for {
		fmt.Printf("%s\n", ag.ContextStatus())
		line, err := t.readInputLine(ctx)
		if err != nil {
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "/quit" || line == "/exit" {
			return nil
		}
		if line == "/stats" {
			s := ag.Stats()
			fmt.Printf("input=%d output=%d tps=%.2f tool_calls=%d failed_tool_calls=%d\n", s.InputTokens, s.OutputTokens, s.TokensPerSecond(), s.ToolCalls, s.FailedToolCalls)
			continue
		}
		if line == "/clear" {
			fmt.Print("\033[H\033[2J")
			continue
		}
		if line == "/clear-history" {
			t.history = nil
			t.historyIdx = -1
			fmt.Println("history cleared")
			continue
		}

		t.addHistory(line)
		ag.AddUserMessage(line)
		fmt.Println("[assistant]")
		out, err := ag.RunOnce(ctx, t)
		fmt.Println()
		if err != nil {
			fmt.Println(ColorRed + err.Error() + ColorReset)
			continue
		}
		if strings.TrimSpace(out) == "" {
			fmt.Println(ColorYellow + "(no textual response)" + ColorReset)
		}
	}
}

func (t *TUI) addHistory(line string) {
	if strings.TrimSpace(line) == "" {
		return
	}
	t.history = append(t.history, line)
	if len(t.history) > t.historyMax {
		t.history = t.history[1:]
	}
	t.historyIdx = len(t.history)
}

func (t *TUI) readInputLine(ctx context.Context) (string, error) {
	_ = ctx
	reader := os.Stdin
	buf := []rune{}
	t.historyIdx = len(t.history)
	t.draftInput = ""

	printPrompt := func() {
		fmt.Printf("\r\033[2K> %s", string(buf))
	}
	printPrompt()

	one := make([]byte, 1)
	for {
		n, err := reader.Read(one)
		if err != nil {
			return "", err
		}
		if n == 0 {
			continue
		}
		b := one[0]

		if b == '\n' || b == '\r' {
			fmt.Print("\n")
			return string(buf), nil
		}
		if b == 3 {
			return "", context.Canceled
		}
		if b == 127 || b == 8 {
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				printPrompt()
			}
			continue
		}
		if b == 27 {
			seq := make([]byte, 2)
			if _, err := reader.Read(seq); err != nil {
				continue
			}
			if seq[0] != '[' {
				continue
			}
			switch seq[1] {
			case 'A':
				if len(t.history) == 0 {
					continue
				}
				if t.historyIdx == len(t.history) {
					t.draftInput = string(buf)
				}
				if t.historyIdx > 0 {
					t.historyIdx--
				}
				buf = []rune(t.history[t.historyIdx])
				printPrompt()
			case 'B':
				if len(t.history) == 0 {
					continue
				}
				if t.historyIdx < len(t.history)-1 {
					t.historyIdx++
					buf = []rune(t.history[t.historyIdx])
				} else {
					t.historyIdx = len(t.history)
					buf = []rune(t.draftInput)
				}
				printPrompt()
			}
			continue
		}

		buf = append(buf, rune(b))
		printPrompt()
	}
}
