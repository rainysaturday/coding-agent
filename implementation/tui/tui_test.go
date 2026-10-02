package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/coding-agent/coding-agent/agent"
	"github.com/coding-agent/coding-agent/config"
)

func TestKeyReaderPlainAndControl(t *testing.T) {
	in := "ab\r\n\x7f\x03\x04\x15\x17\t"
	kr := newKeyReader(strings.NewReader(in))
	want := []KeyKind{KeyRune, KeyRune, KeyEnter, KeyEnter, KeyBackspace, KeyCtrlC, KeyCtrlD, KeyCtrlU, KeyCtrlW, KeyTab}
	for i, w := range want {
		k, err := kr.Read()
		if err != nil {
			t.Fatalf("key %d: %v", i, err)
		}
		if k.Kind != w {
			t.Fatalf("key %d: got %v want %v", i, k.Kind, w)
		}
	}
}

func TestKeyReaderEscapeSequences(t *testing.T) {
	in := "\x1b[A\x1b[B\x1b[C\x1b[D\x1b[H\x1b[F\x1b[3~\x1b[1~\x1b[4~"
	kr := newKeyReader(strings.NewReader(in))
	want := []KeyKind{KeyUp, KeyDown, KeyRight, KeyLeft, KeyHome, KeyEnd, KeyDelete, KeyHome, KeyEnd}
	for i, w := range want {
		k, err := kr.Read()
		if err != nil {
			t.Fatalf("key %d: %v", i, err)
		}
		if k.Kind != w {
			t.Fatalf("key %d: got %v want %v", i, k.Kind, w)
		}
	}
}

func TestKeyReaderBareEscape(t *testing.T) {
	kr := newKeyReader(strings.NewReader("\x1b"))
	k, err := kr.Read()
	if err != nil {
		t.Fatal(err)
	}
	if k.Kind != KeyEscape {
		t.Fatalf("bare escape: %v", k.Kind)
	}
}

func TestKeyReaderUnicode(t *testing.T) {
	kr := newKeyReader(strings.NewReader("é"))
	k, err := kr.Read()
	if err != nil {
		t.Fatal(err)
	}
	if k.Kind != KeyRune || k.Rune != 'é' {
		t.Fatalf("unicode rune: %+v", k)
	}
}

func TestComma(t *testing.T) {
	cases := map[int]string{0: "0", 999: "999", 1000: "1,000", 1234567: "1,234,567", -4200: "-4,200"}
	for n, want := range cases {
		if got := comma(n); got != want {
			t.Fatalf("comma(%d) = %q want %q", n, got, want)
		}
	}
}

func TestTruncateLines(t *testing.T) {
	got := truncateLines("a\nb\nc\nd\ne", 2, 1000)
	if len(got) != 3 || got[0] != "a" || !strings.Contains(got[2], "3 more lines") {
		t.Fatalf("line truncation: %v", got)
	}
	got = truncateLines(strings.Repeat("x", 100), 10, 10)
	if len(got) != 1 || !strings.Contains(got[0], "[truncated]") {
		t.Fatalf("char truncation: %v", got)
	}
	if truncateLines("", 5, 5) != nil {
		t.Fatal("empty input")
	}
}

func newTestTUI(t *testing.T) *TUI {
	t.Helper()
	cfg := config.NewDefault()
	cfg.MaxHistory = 3
	f, err := os.CreateTemp(t.TempDir(), "tui")
	if err != nil {
		t.Fatal(err)
	}
	return &TUI{cfg: cfg, out: f, events: make(chan agent.Event, 8), histIdx: -1}
}

func TestHistoryNavigation(t *testing.T) {
	tr := newTestTUI(t)
	tr.pushHistory("first")
	tr.pushHistory("second")
	tr.pushHistory("third")

	tr.navigateHistory(true)
	if string(tr.draft) != "third" {
		t.Fatalf("up 1: %q", string(tr.draft))
	}
	tr.navigateHistory(true)
	if string(tr.draft) != "second" {
		t.Fatalf("up 2: %q", string(tr.draft))
	}
	tr.navigateHistory(false)
	if string(tr.draft) != "third" {
		t.Fatalf("down: %q", string(tr.draft))
	}
	tr.navigateHistory(false)
	if tr.draft != nil {
		t.Fatalf("down past newest should clear draft: %q", string(tr.draft))
	}

	// History capped at MaxHistory, newest kept.
	tr.pushHistory("fourth")
	if len(tr.history) != 3 || tr.history[0] != "second" || tr.history[2] != "fourth" {
		t.Fatalf("history cap: %v", tr.history)
	}
}

func TestHelpTextCoversCommands(t *testing.T) {
	for _, cmd := range []string{"/stats", "/clear", "/clear-history", "/compress", "/goal", "/goal-off", "/dump", "/read-only", "/skills", "/skill", "/version", "/help", "/exit"} {
		if !strings.Contains(helpText, cmd) {
			t.Fatalf("help missing %s", cmd)
		}
	}
}
