package tui

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coding-agent/harness/colors"
	"github.com/coding-agent/harness/config"
)

// twSink collects what the typewriter writes. It plays the role of the terminal
// writer, including taking a lock, so the typewriter's locking contract is
// exercised the same way it is in the TUI.
type twSink struct {
	mu sync.Mutex
	sb strings.Builder
	// writes records each individual write step, so tests can see how the text
	// was released rather than only what ended up on screen.
	writes []string
}

func (s *twSink) write(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sb.WriteString(text)
	s.writes = append(s.writes, text)
}

func (s *twSink) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sb.String()
}

func (s *twSink) steps() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.writes)
}

// drainWait waits until the typewriter has released everything, and reports how
// long that took.
func drainWait(tw *Typewriter, timeout time.Duration) (time.Duration, bool) {
	start := time.Now()
	deadline := start.Add(timeout)
	for tw.Pending() > 0 {
		if time.Now().After(deadline) {
			return time.Since(start), false
		}
		time.Sleep(time.Millisecond)
	}
	return time.Since(start), true
}

// A buffered chunk must not reach the terminal all at once: it is released a
// character at a time, and the whole buffer lands inside roughly one window.
func TestTypewriter_ReleasesOneCharacterAtATime(t *testing.T) {
	sink := &twSink{}
	tw := NewTypewriter(true, sink.write)
	tw.SetWindow(100 * time.Millisecond)

	text := strings.Repeat("a", 100)
	tw.Push(text, "")

	if got := tw.Pending(); got == 0 {
		t.Fatal("the chunk should be buffered, not written synchronously")
	}

	elapsed, drained := drainWait(tw, 2*time.Second)
	if !drained {
		t.Fatalf("the buffer never drained, %d runes still pending", tw.Pending())
	}

	if got := sink.String(); got != text {
		t.Errorf("the rendered text changed: got %q", got)
	}
	// The whole buffer must fit in one window: 100 runes at a 100ms window is
	// 1ms per character, so the drain lands just inside the window.
	if elapsed < 70*time.Millisecond || elapsed > 160*time.Millisecond {
		t.Errorf("draining 100 runes took %v, expected the 100ms window", elapsed)
	}
	// A hundred characters must not have been emitted in a handful of steps.
	if steps := sink.steps(); steps < 50 {
		t.Errorf("expected one write step per character, got %d steps", steps)
	}
}

// The interval is re-derived from the buffer every time it grows: a burst of
// tokens speeds the animation up instead of queueing behind the old pace.
func TestTypewriter_RecalculatesSpeedAsTheBufferGrows(t *testing.T) {
	sink := &twSink{}
	tw := NewTypewriter(true, sink.write)
	tw.SetWindow(100 * time.Millisecond)

	// A trickle: 10 runes share the window, so ~10ms per character.
	tw.Push(strings.Repeat("x", 10), "")
	time.Sleep(30 * time.Millisecond)
	partial := tw.Pending()
	if partial >= 10 {
		t.Errorf("nothing was released during the first 30ms (%d pending)", partial)
	}
	if partial <= 0 {
		t.Errorf("the trickle drained too fast (%d pending after 30ms)", partial)
	}

	// A burst: the buffer is now ~100 runes, so the pace must jump to ~1ms per
	// character and the whole thing must be on screen within about a window.
	tw.Push(strings.Repeat("y", 90), "")
	elapsed, drained := drainWait(tw, 2*time.Second)
	if !drained {
		t.Fatalf("the burst never drained, %d runes still pending", tw.Pending())
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("the burst was not sped up: %v to drain ~%d runes", elapsed, partial+90)
	}
	if got := sink.String(); !strings.Contains(got, strings.Repeat("y", 90)) {
		t.Error("the burst text was not fully rendered")
	}
}

// The defining property of the model: whatever is buffered has to be on screen
// within one window, no matter how big the buffer is. A 300-rune burst asks for
// a third of a millisecond per character, which is below the timer resolution,
// so a naive "sleep per character" loop would take three windows. The absolute
// schedule keeps it to one.
func TestTypewriter_WholeBufferFitsInOneWindow(t *testing.T) {
	const window = 100 * time.Millisecond

	for _, runes := range []int{10, 100, 300} {
		sink := &twSink{}
		tw := NewTypewriter(true, sink.write)
		tw.SetWindow(window)

		text := strings.Repeat("a", runes)
		tw.Push(text, "")

		elapsed, drained := drainWait(tw, 5*time.Second)
		if !drained {
			t.Fatalf("%d runes never drained (%d pending)", runes, tw.Pending())
		}
		if elapsed < window/2 {
			t.Errorf("%d runes drained in %v: faster than the window, so it was not animated", runes, elapsed)
		}
		if elapsed > window+window/2 {
			t.Errorf("%d runes took %v to drain, expected one %v window", runes, elapsed, window)
		}
		if got := sink.String(); got != text {
			t.Errorf("%d runes: the rendered text changed", runes)
		}
	}
}

// Flush blocks until the buffer is on screen, so anything printed afterwards
// keeps its place in the output.
func TestTypewriter_FlushWaitsForTheBuffer(t *testing.T) {
	sink := &twSink{}
	tw := NewTypewriter(true, sink.write)
	tw.SetWindow(100 * time.Millisecond)

	tw.Push("streamed", "")
	tw.Flush()

	if got := tw.Pending(); got != 0 {
		t.Errorf("Flush returned with %d runes still buffered", got)
	}
	if got := sink.String(); !strings.Contains(got, "streamed") {
		t.Errorf("Flush returned before the text was written: %q", got)
	}
}

// A status line pushed after animated text must appear after it, not on top of
// it.
func TestTypewriter_PushNowKeepsOrdering(t *testing.T) {
	sink := &twSink{}
	tw := NewTypewriter(true, sink.write)
	tw.SetWindow(50 * time.Millisecond)

	tw.Push("answer", "")
	tw.PushNow("[Scheduled]", colors.GetColor("cyan"))
	tw.Flush()

	got := sink.String()
	answerAt := strings.Index(got, "answer")
	statusAt := strings.Index(got, "[Scheduled]")
	if answerAt < 0 || statusAt < 0 {
		t.Fatalf("both messages should be on screen, got %q", got)
	}
	if answerAt > statusAt {
		t.Errorf("the status line overtook the stream: %q", got)
	}
	if !strings.Contains(got, colors.GetColor("cyan")) {
		t.Error("the status line lost its color")
	}
}

// Raw segments carry their own ANSI codes: they are written verbatim, in one
// step, and close any color the animation left open.
func TestTypewriter_PushRawIsVerbatimAndClosesColor(t *testing.T) {
	sink := &twSink{}
	tw := NewTypewriter(true, sink.write)
	tw.SetWindow(20 * time.Millisecond)

	tw.Push("red", colors.GetColor("red"))
	tw.PushRaw("\033[2K\r[Tool Call] bash")
	tw.Flush()

	got := sink.String()
	if !strings.Contains(got, "\033[2K\r[Tool Call] bash") {
		t.Errorf("the raw segment was altered: %q", got)
	}
	if !strings.Contains(got, "red") {
		t.Errorf("the animated text was dropped: %q", got)
	}
	// The reset that closes the red run must come before the raw control code.
	if strings.Index(got, colors.GetColor("reset")) > strings.Index(got, "\033[2K") {
		t.Errorf("the open color was not closed before the raw segment: %q", got)
	}
}

// Multi-byte runes are released whole: splitting a rune mid-way would show
// garbage on the terminal.
func TestTypewriter_KeepsMultiByteRunesWhole(t *testing.T) {
	sink := &twSink{}
	tw := NewTypewriter(true, sink.write)
	tw.SetWindow(10 * time.Millisecond)

	text := "héllo — 世界 🎉"
	tw.Push(text, "")
	tw.Flush()

	got := sink.String()
	if got != text {
		t.Errorf("multi-byte text was mangled: got %q", got)
	}
}

// stripANSI removes the color codes the typewriter emits, leaving the text the
// user actually reads.
func stripANSI(s string) string {
	return strings.NewReplacer(
		colors.GetColor("reset"), "",
		colors.GetColor("dim"), "",
		colors.GetColor("red"), "",
		colors.GetColor("cyan"), "",
	).Replace(s)
}

// Colors are emitted on transitions rather than around every character, and the
// terminal is always left uncolored once the buffer drains.
func TestTypewriter_EmitsColorOnlyOnTransitions(t *testing.T) {
	sink := &twSink{}
	tw := NewTypewriter(true, sink.write)
	tw.SetWindow(10 * time.Millisecond)

	tw.Push("dim", colors.GetColor("dim"))
	tw.Push("plain", "")
	tw.Flush()

	got := sink.String()
	if stripped := stripANSI(got); stripped != "dimplain" {
		t.Errorf("the text changed: got %q", got)
	}
	if n := strings.Count(got, colors.GetColor("dim")); n != 1 {
		t.Errorf("expected the dim code once, got %d occurrences in %q", n, got)
	}
	// The switch from dim to the default color closes the previous run rather
	// than re-wrapping every character.
	if n := strings.Count(got, colors.GetColor("reset")); n != 1 {
		t.Errorf("expected a single reset at the color change, got %d in %q", n, got)
	}
	if !strings.HasSuffix(got, "plain") {
		t.Errorf("the terminal should be left uncolored, got %q", got)
	}
}

// A stream that ends while still colored must close the color, otherwise the
// prompt bar inherits it.
func TestTypewriter_ClosesColorWhenTheStreamEndsColored(t *testing.T) {
	sink := &twSink{}
	tw := NewTypewriter(true, sink.write)
	tw.SetWindow(10 * time.Millisecond)

	tw.Push("thinking", colors.GetColor("dim"))
	tw.Flush()

	got := sink.String()
	if stripped := stripANSI(got); stripped != "thinking" {
		t.Errorf("the text changed: got %q", got)
	}
	if !strings.HasSuffix(got, colors.GetColor("reset")) {
		t.Errorf("a colored stream must end with a reset, got %q", got)
	}
}

// With the animation disabled the typewriter behaves the way streaming used to:
// the whole chunk is written before the call returns.
func TestTypewriter_DisabledWritesImmediately(t *testing.T) {
	sink := &twSink{}
	tw := NewTypewriter(false, sink.write)

	tw.Push("right now", colors.GetColor("dim"))

	if got := sink.String(); got != colors.GetColor("dim")+"right now"+colors.GetColor("reset") {
		t.Errorf("expected immediate colored output, got %q", got)
	}
	if got := tw.Pending(); got != 0 {
		t.Errorf("a disabled typewriter should never buffer, got %d pending", got)
	}
}

// The segment hook lets the owner repaint at line boundaries; it must fire once
// per segment, after that segment is on screen.
func TestTypewriter_SegmentHookReportsLineBoundaries(t *testing.T) {
	sink := &twSink{}
	tw := NewTypewriter(true, sink.write)
	tw.SetWindow(10 * time.Millisecond)

	var mu sync.Mutex
	var ends []bool
	tw.SetSegmentEndHook(func(lineEnd bool) {
		mu.Lock()
		defer mu.Unlock()
		// The hook runs after the segment, so the text must already be visible.
		ends = append(ends, lineEnd)
	})

	tw.Push("mid-word", "")
	tw.Push("full line\n", "")
	tw.Flush()

	mu.Lock()
	defer mu.Unlock()
	if len(ends) != 2 {
		t.Fatalf("expected one notification per segment, got %d", len(ends))
	}
	if ends[0] {
		t.Error("a mid-word segment must not report a line end")
	}
	if !ends[1] {
		t.Error("a segment ending in a newline must report a line end")
	}
}

// No goroutine lingers once the buffer has drained. The printer notices the
// empty buffer on its own schedule, so this waits for the transition rather
// than assuming it happened the instant Flush returned.
func TestTypewriter_PrinterExitsWhenIdle(t *testing.T) {
	sink := &twSink{}
	tw := NewTypewriter(true, sink.write)
	tw.SetWindow(5 * time.Millisecond)

	tw.Push("done", "")
	tw.Flush()

	deadline := time.Now().Add(2 * time.Second)
	for {
		tw.mu.Lock()
		running := tw.running
		tw.mu.Unlock()
		if !running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the printer goroutine never exited after the buffer drained")
		}
		time.Sleep(time.Millisecond)
	}
}

// The TUI must keep the stream readable when the animation is off, and must be
// able to drain it before printing anything else.
func TestTUI_StreamFlushCompletesTheStream(t *testing.T) {
	tui := NewTUI(config.DefaultConfig())

	tui.StartStream()
	tui.StreamNormalChunk("buffered output")
	tui.FlushStream()

	if got := tui.tw.Pending(); got != 0 {
		t.Errorf("FlushStream left %d runes buffered", got)
	}
	tui.StreamEnd()
}

// A stream and a status line arriving at once must not deadlock or lose text:
// Flush waits for the printer, and the printer must never wait on a caller that
// is itself waiting.
func TestTypewriter_ConcurrentPushAndFlush(t *testing.T) {
	sink := &twSink{}
	tw := NewTypewriter(true, sink.write)
	tw.SetWindow(2 * time.Millisecond)

	var wg sync.WaitGroup
	for writer := 0; writer < 4; writer++ {
		wg.Add(1)
		go func(writer int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				tw.Push(fmt.Sprintf("stream-%d-%d ", writer, i), "")
				if i%5 == 0 {
					tw.PushNow("[status]", colors.GetColor("cyan"))
				}
			}
		}(writer)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("concurrent streaming deadlocked")
	}

	tw.Flush()
	if got := tw.Pending(); got != 0 {
		t.Errorf("%d runes still buffered after Flush", got)
	}
	for writer := 0; writer < 4; writer++ {
		for i := 0; i < 20; i++ {
			want := fmt.Sprintf("stream-%d-%d", writer, i)
			if got := sink.String(); !strings.Contains(got, want) {
				t.Fatalf("missing %q in the rendered output", want)
			}
		}
	}
}
