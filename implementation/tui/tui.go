// Package tui implements the minimal terminal user interface: a raw-mode
// line editor with history, streaming output rendering with tool feedback,
// context size display and slash commands.
package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/coding-agent/coding-agent/agent"
	"github.com/coding-agent/coding-agent/colors"
	"github.com/coding-agent/coding-agent/config"
	"github.com/coding-agent/coding-agent/inference"
	"github.com/coding-agent/coding-agent/version"
)

// evtTurnComplete is a sentinel pushed after an agent turn finishes.
const evtTurnComplete agent.EventType = "turn_complete"

// TUI is the terminal user interface.
type TUI struct {
	a   *agent.Agent
	cfg *config.Config

	out    *os.File
	events chan agent.Event
	keys   chan Key
	kerr   chan error
	raw    *rawMode

	history   []string
	histIdx   int
	draft     []rune
	cursor    int
	promptOn  bool
	streaming bool
	toolLine  bool
	running   bool
	quit      bool
}

// New creates a TUI for the given agent.
func New(a *agent.Agent, cfg *config.Config) *TUI {
	return &TUI{
		a:       a,
		cfg:     cfg,
		out:     os.Stdout,
		events:  make(chan agent.Event, 256),
		keys:    make(chan Key, 64),
		kerr:    make(chan error, 1),
		histIdx: -1,
	}
}

// Run starts the interactive loop. It returns the process exit code.
func (t *TUI) Run(ctx context.Context) int {
	if t.cfg.Goal != "" {
		t.a.SetGoal(t.cfg.Goal)
	}
	t.a.SetEventCallback(func(e agent.Event) {
		select {
		case t.events <- e:
		case <-ctx.Done():
		}
	})

	isTTY := isTerminal(os.Stdin)
	if isTTY {
		t.raw = &rawMode{fd: int(os.Stdin.Fd())}
		if err := t.raw.Enter(); err != nil {
			t.raw = nil
			isTTY = false
		}
	}
	defer func() {
		if t.raw != nil {
			t.raw.Restore()
		}
	}()

	t.banner()
	if t.cfg.Goal != "" {
		t.line(colors.GetColor("magenta"), "Goal mode active: "+t.cfg.Goal)
	}

	if isTTY {
		go func() {
			kr := newKeyReader(os.Stdin)
			for {
				k, err := kr.Read()
				if err != nil {
					t.kerr <- err
					return
				}
				t.keys <- k
			}
		}()
	} else {
		// Non-TTY stdin: read prompts line by line until EOF.
		go func() {
			kr := newKeyReader(os.Stdin)
			for {
				k, err := kr.Read()
				if err != nil {
					t.kerr <- err
					return
				}
				t.keys <- k
			}
		}()
	}

	t.drawStatus()
	t.drawPrompt()

	for !t.quit {
		select {
		case <-ctx.Done():
			t.quit = true
		case err := <-t.kerr:
			if err != nil && err != io.EOF {
				t.line(colors.GetColor("red"), "Input error: "+err.Error())
			}
			t.quit = true
		case k := <-t.keys:
			t.handleKey(ctx, k)
		case e := <-t.events:
			t.handleEvent(e)
		}
	}

	return t.shutdown()
}

// shutdown performs the exit flow including the automatic dump.
func (t *TUI) shutdown() int {
	if t.raw != nil {
		t.raw.Restore()
	}
	if !t.cfg.NoDumpOnExit {
		if path, err := t.a.DumpContext(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to dump context: %v\n", err)
		} else {
			fmt.Printf("Context dumped to: %s\n", path)
		}
	}
	fmt.Println("Goodbye.")
	return 0
}

// ---------------------------------------------------------------------------
// banner and status
// ---------------------------------------------------------------------------

func (t *TUI) banner() {
	fmt.Println(colors.Paint("cyan", "Coding Agent "+version.String()))
	mode := "interactive"
	if t.cfg.ReadOnly {
		mode += " (read-only)"
	}
	fmt.Printf("Model: %s | Endpoint: %s | Context: %s | Theme: %s | Mode: %s\n",
		t.cfg.Model, t.cfg.APIEndpoint, comma(t.a.ContextLimit()), t.cfg.Theme, mode)
	fmt.Println("Type /help for commands. Ctrl+C cancels a running operation; Ctrl+C at the prompt exits.")
}

// contextLine renders the context size indicator with warning colors
// (requirement 021).
func (t *TUI) contextLine() string {
	tok := t.a.ContextTokens()
	limit := t.a.ContextLimit()
	pct := 100 * float64(tok) / float64(limit)
	slot := "green"
	warn := ""
	switch {
	case pct > 90:
		slot = "red"
		warn = " ⚠⚠"
	case pct > 75:
		slot = "yellow"
		warn = " ⚠"
	case pct >= 50:
		slot = "yellow"
	}
	txt := fmt.Sprintf("[Tokens: %s / %s (%.1f%%)%s]", comma(tok), comma(limit), pct, warn)
	return colors.Paint(slot, txt)
}

func (t *TUI) drawStatus() {
	fmt.Fprint(t.out, "\r\033[K"+t.contextLine()+"\n")
}

// ---------------------------------------------------------------------------
// prompt / line editor
// ---------------------------------------------------------------------------

func (t *TUI) drawPrompt() {
	t.promptOn = true
	fmt.Fprint(t.out, "\r\033[K> "+string(t.draft))
	if back := len(t.draft) - t.cursor; back > 0 {
		fmt.Fprintf(t.out, "\033[%dD", back)
	}
}

func (t *TUI) clearPrompt() {
	if t.promptOn {
		fmt.Fprint(t.out, "\r\033[K")
		t.promptOn = false
	}
}

// line prints a complete styled line, preserving an in-progress prompt.
func (t *TUI) line(color, text string) {
	t.endStream()
	t.clearToolLine()
	t.clearPrompt()
	if color != "" {
		fmt.Fprintln(t.out, color+text+colors.GetColor("reset"))
	} else {
		fmt.Fprintln(t.out, text)
	}
	if !t.running {
		t.drawPrompt()
	}
}

// lineBlock prints multi-line text with an optional color prefix per line.
func (t *TUI) lineBlock(color, text string) {
	for _, l := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		t.line(color, l)
	}
}

// endStream terminates an open streaming output run with a newline.
func (t *TUI) endStream() {
	if t.streaming {
		fmt.Fprintln(t.out)
		t.streaming = false
	}
}

// ---------------------------------------------------------------------------
// key handling
// ---------------------------------------------------------------------------

func (t *TUI) handleKey(ctx context.Context, k Key) {
	switch k.Kind {
	case KeyCtrlC:
		if t.running {
			t.a.Cancel()
			t.line(colors.GetColor("yellow"), "Cancelling...")
			return
		}
		t.quit = true
	case KeyCtrlD:
		if len(t.draft) == 0 && !t.running {
			t.quit = true
		}
	case KeyCtrlU:
		if !t.running {
			t.draft = nil
			t.cursor = 0
			t.drawPrompt()
		}
	case KeyEnter:
		if t.running {
			return
		}
		line := strings.TrimSpace(string(t.draft))
		t.draft = nil
		t.cursor = 0
		t.histIdx = -1
		t.clearPrompt()
		if line == "" {
			t.drawPrompt()
			return
		}
		t.pushHistory(line)
		if strings.HasPrefix(line, "/") {
			t.handleCommand(ctx, line)
			t.drawStatus()
			t.drawPrompt()
			return
		}
		t.startRun(ctx, line)
	case KeyBackspace:
		if !t.running && t.cursor > 0 {
			t.draft = append(t.draft[:t.cursor-1], t.draft[t.cursor:]...)
			t.cursor--
			t.drawPrompt()
		}
	case KeyDelete:
		if !t.running && t.cursor < len(t.draft) {
			t.draft = append(t.draft[:t.cursor], t.draft[t.cursor+1:]...)
			t.drawPrompt()
		}
	case KeyLeft:
		if !t.running && t.cursor > 0 {
			t.cursor--
			t.drawPrompt()
		}
	case KeyRight:
		if !t.running && t.cursor < len(t.draft) {
			t.cursor++
			t.drawPrompt()
		}
	case KeyHome:
		if !t.running {
			t.cursor = 0
			t.drawPrompt()
		}
	case KeyEnd:
		if !t.running {
			t.cursor = len(t.draft)
			t.drawPrompt()
		}
	case KeyUp, KeyDown:
		if t.running {
			return
		}
		t.navigateHistory(k.Kind == KeyUp)
	case KeyRune:
		if !t.running {
			t.draft = append(t.draft, k.Rune)
			t.cursor = len(t.draft)
			t.drawPrompt()
		}
	default:
		// ignore
	}
}

// navigateHistory walks the input history with the arrow keys (requirement 019).
func (t *TUI) navigateHistory(up bool) {
	if len(t.history) == 0 {
		return
	}
	if up {
		if t.histIdx < len(t.history)-1 {
			t.histIdx++
		}
	} else {
		if t.histIdx > 0 {
			t.histIdx--
		} else {
			t.histIdx = -1
		}
	}
	if t.histIdx >= 0 {
		t.draft = []rune(t.history[len(t.history)-1-t.histIdx])
	} else {
		t.draft = nil
	}
	t.cursor = len(t.draft)
	t.drawPrompt()
}

func (t *TUI) pushHistory(line string) {
	t.history = append(t.history, line)
	if max := t.cfg.MaxHistory; max > 0 && len(t.history) > max {
		t.history = t.history[len(t.history)-max:]
	}
}

// startRun launches the agent in the background.
func (t *TUI) startRun(ctx context.Context, prompt string) {
	t.running = true
	go func() {
		_, err := t.a.Submit(ctx, prompt)
		_ = err // errors are surfaced through events
		t.events <- agent.Event{Type: evtTurnComplete}
	}()
}

// ---------------------------------------------------------------------------
// event rendering
// ---------------------------------------------------------------------------

func (t *TUI) handleEvent(e agent.Event) {
	switch e.Type {
	case agent.EventContent:
		t.clearToolLine()
		fmt.Fprint(t.out, e.Text)
		t.streaming = true
	case agent.EventReasoning:
		t.clearToolLine()
		fmt.Fprint(t.out, colors.GetColor("dim")+e.Text+colors.GetColor("reset"))
		t.streaming = true
	case agent.EventToolArgs:
		t.endStream()
		t.drawToolLine(e.Tool, e.Args)
	case agent.EventToolStart:
		t.endStream()
		if !t.toolLine {
			t.drawToolLine(e.Tool, e.Args)
		}
		t.closeToolLine()
	case agent.EventToolResult:
		t.closeToolLine()
		if e.Failed {
			t.line(colors.GetColor("red"), "Tool '"+e.Tool+"' failed:")
		} else {
			t.line(colors.GetColor("green"), "Tool '"+e.Tool+"' executed successfully:")
		}
		for _, l := range truncateLines(e.Result, 20, 2000) {
			t.line(colors.GetColor("dim"), "  "+l)
		}
	case agent.EventError:
		t.endStream()
		t.closeToolLine()
		t.lineBlock(colors.GetColor("red"), e.Text)
	case agent.EventGoal:
		t.endStream()
		t.line(colors.GetColor("magenta"), e.Text)
	case agent.EventCompressed:
		t.endStream()
		t.line(colors.GetColor("magenta"), e.Text)
	case agent.EventSubagent:
		t.endStream()
		t.line(colors.GetColor("dim"), "[Subagent] "+e.Text)
	case agent.EventIteration:
		// silent
	case agent.EventDone:
		// content already streamed
	case evtTurnComplete:
		t.endStream()
		t.closeToolLine()
		fmt.Fprintln(t.out)
		t.running = false
		t.drawStatus()
		t.drawPrompt()
	}
}

// drawToolLine renders the in-place tool call line (requirement 017).
func (t *TUI) drawToolLine(name, args string) {
	cols, _ := terminalSize()
	txt := "[Tool Call] " + name
	if args != "" {
		txt += " (" + args + ")"
	}
	if len(txt) > cols-1 {
		txt = txt[:cols-1]
	}
	fmt.Fprint(t.out, "\r\033[K"+colors.GetColor("blue")+txt+colors.GetColor("reset"))
	t.toolLine = true
}

func (t *TUI) closeToolLine() {
	if t.toolLine {
		fmt.Fprintln(t.out)
		t.toolLine = false
	}
}

func (t *TUI) clearToolLine() {
	if t.toolLine {
		fmt.Fprint(t.out, "\r\033[K")
		t.toolLine = false
	}
}

// truncateLines trims a tool result for display.
func truncateLines(s string, maxLines, maxChars int) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	if len(s) > maxChars {
		s = s[:maxChars] + " ... [truncated]"
	}
	lines := strings.Split(s, "\n")
	if len(lines) > maxLines {
		omitted := len(lines) - maxLines
		lines = append(lines[:maxLines], fmt.Sprintf("... [%d more lines]", omitted))
	}
	return lines
}

// ---------------------------------------------------------------------------
// commands
// ---------------------------------------------------------------------------

func (t *TUI) handleCommand(ctx context.Context, line string) {
	cmd, rest, _ := strings.Cut(line, " ")
	rest = strings.TrimSpace(rest)
	switch cmd {
	case "/help":
		t.lineBlock("", helpText)
	case "/stats":
		t.statsBlock()
	case "/clear":
		fmt.Fprint(t.out, "\033[2J\033[H")
	case "/clear-history":
		t.history = nil
		t.histIdx = -1
		t.line("", "Input history cleared.")
	case "/compress":
		t.line(colors.GetColor("magenta"), "Compressing context...")
		t.running = true
		go func() {
			err := t.a.Compress(ctx)
			if err != nil {
				t.events <- agent.Event{Type: agent.EventError, Text: "Compression failed: " + err.Error(), Failed: true}
			}
			t.events <- agent.Event{Type: evtTurnComplete}
		}()
	case "/goal":
		if rest == "" {
			t.line(colors.GetColor("magenta"), "Goal mode: "+goalStatus(t.a))
			break
		}
		t.a.SetGoal(rest)
		t.line(colors.GetColor("magenta"), "Goal set: "+rest)
		t.startRun(ctx, rest)
	case "/goal-off":
		t.a.ClearGoal()
		t.line(colors.GetColor("magenta"), "Goal mode deactivated.")
	case "/dump":
		path, err := t.a.DumpContext()
		if err != nil {
			t.line(colors.GetColor("red"), "Dump failed: "+err.Error())
		} else {
			t.line(colors.GetColor("green"), "Context dumped to: "+path)
		}
	case "/read-only":
		newVal := !t.a.ReadOnly()
		t.a.SetReadOnly(newVal)
		if newVal {
			t.line(colors.GetColor("yellow"), "Read-only mode enabled.")
		} else {
			t.line(colors.GetColor("yellow"), "Read-only mode disabled.")
		}
	case "/skills":
		t.skillsList()
	case "/skill":
		if rest == "" {
			t.line("", "Usage: /skill <name>")
			break
		}
		t.activateSkill(rest)
	case "/version":
		t.line("", version.Full())
	case "/exit", "/quit":
		t.quit = true
	default:
		t.line(colors.GetColor("red"), "Unknown command: "+cmd+" (try /help)")
	}
}

func goalStatus(a *agent.Agent) string {
	goal, start := a.Goal()
	if goal == "" {
		return "inactive"
	}
	return fmt.Sprintf("active: %q (elapsed %s)", goal, agent.FormatDuration(since(start)))
}

func statsBlockFor(a *agent.Agent) string {
	var b strings.Builder
	b.WriteString(strings.Repeat("=", 50) + "\n")
	b.WriteString("Runtime Statistics\n")
	b.WriteString(strings.Repeat("=", 50) + "\n")
	b.WriteString(fmtStats(a))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("=", 50))
	return b.String()
}

func (t *TUI) statsBlock() {
	t.lineBlock("", statsBlockFor(t.a))
}

func (t *TUI) skillsList() {
	s := t.a.Skills()
	if s == nil {
		t.line("", "Skills are disabled.")
		return
	}
	list := s.List()
	if len(list) == 0 {
		t.line("", "No skills discovered.")
		for _, d := range s.Diagnostics() {
			t.line(colors.GetColor("dim"), "  ["+d.Level+"] "+d.Path+": "+d.Message)
		}
		return
	}
	for _, sk := range list {
		t.line("", fmt.Sprintf("  %-24s %s", sk.Name, sk.Description))
	}
}

func (t *TUI) activateSkill(name string) {
	s := t.a.Skills()
	if s == nil {
		t.line("", "Skills are disabled.")
		return
	}
	content, err := s.Activate(name)
	if err != nil {
		t.line(colors.GetColor("red"), err.Error())
		return
	}
	t.a.AddMessage(inference.NewTextMessage("user", "Skill '"+name+"' manually activated by the user:\n\n"+content))
	t.line(colors.GetColor("green"), "Skill '"+name+"' activated and added to the context.")
}
