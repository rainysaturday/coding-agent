package tui

import (
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/coding-agent/harness/colors"
)

// typewriterWindow is the time budget the renderer gives itself to paint
// everything it currently holds. Streaming a token stream straight to the
// terminal makes the output arrive in irregular bursts: a few very long tokens
// followed by a pause while the model thinks. Buffering briefly and releasing
// the text one character at a time turns those bursts into a smooth, lively
// animation.
//
// The per-character interval is derived from the buffer: whatever is buffered
// must be on screen within one window, so interval = window / buffered runes.
// A long token therefore prints fast (many runes share the window) and a trickle
// prints at a relaxed pace, without ever stalling behind the stream.
const typewriterWindow = 100 * time.Millisecond

// twSegment is one run of text waiting to be painted.
type twSegment struct {
	text  string
	color string
	// whole segments are written in a single step instead of a character at a
	// time. Status messages qualify: they are single messages, not a token
	// stream, so animating them would only make them late. They still go
	// through the queue, because they must not overtake the text ahead of them.
	whole bool
	// raw segments are written verbatim. They carry their own ANSI codes (the
	// "thinking complete" rule, in-place tool-call updates) and must not be
	// re-coloured or split apart.
	raw bool
	// lineEnd records whether the segment ends with a newline, so the owner can
	// repaint the prompt bar at line boundaries only.
	lineEnd bool
}

// Typewriter renders streamed text one character at a time from a small buffer.
//
// It is deliberately the only writer for the text it owns: everything that has
// to appear in a given order (a separator, a tool-call status line) is queued
// through it as well, so nothing overtakes anything else.
//
// Locking contract: callers must NOT hold the lock that guards the terminal
// writer. The write callback and the segment hook take that lock themselves, so
// calling Push/PushNow/Flush while holding it would deadlock.
type Typewriter struct {
	// window is the drain budget described above. It is a field rather than a
	// constant so tests can run the model at a different scale.
	window time.Duration
	// enabled is false when the caller asked for plain, immediate output
	// (--no-typewriter). Segments are then written whole, exactly as they were
	// before the animation existed.
	enabled bool
	// write serializes terminal access; it must take whatever lock protects
	// stdout.
	write func(string)
	// onSegmentEnd is notified once a segment has been fully written.
	onSegmentEnd func(lineEnd bool)

	mu sync.Mutex
	// queue holds the segments still to be painted, in arrival order.
	queue []twSegment
	// pending counts the runes still buffered in animated segments. Raw
	// segments are excluded: they are painted in one step and must not slow the
	// animation down.
	pending int
	// interval is the delay between two characters, recomputed whenever the
	// buffer grows.
	interval time.Duration
	// running reports whether the printer goroutine is alive. It exits once the
	// buffer drains, so an idle typewriter costs nothing.
	running bool
	// painting reports that the printer is in the middle of a write step.
	painting bool
	// openColor is the color currently in effect on the terminal. It lives in
	// the shared state rather than in the printer goroutine so Flush can tell
	// that a closing reset is still owed.
	openColor string
	// fast requests that the remaining buffer be painted without delay.
	fast bool

	// wake pokes the printer: new text arrived, or a flush was requested.
	wake chan struct{}
	// tick announces that one write step completed, letting Flush wait for
	// progress instead of polling.
	tick chan struct{}
}

// NewTypewriter creates a typewriter whose output goes through write. The write
// callback is invoked with the terminal lock held by the callback itself.
func NewTypewriter(enabled bool, write func(string)) *Typewriter {
	return &Typewriter{
		enabled: enabled,
		window:  typewriterWindow,
		write:   write,
		wake:    make(chan struct{}, 1),
		tick:    make(chan struct{}, 1),
	}
}

// SetWindow overrides the drain window, which is the time the whole buffer has
// to be painted.
func (tw *Typewriter) SetWindow(d time.Duration) {
	if d <= 0 {
		return
	}
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.window = d
}

// Window returns the configured drain window.
func (tw *Typewriter) Window() time.Duration {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	return tw.window
}

// SetSegmentEndHook installs a callback invoked after each segment has been
// fully written, with whether that segment ended on a newline.
func (tw *Typewriter) SetSegmentEndHook(fn func(lineEnd bool)) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.onSegmentEnd = fn
}

// Enabled reports whether the animation is active.
func (tw *Typewriter) Enabled() bool {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	return tw.enabled
}

// Push queues animated text in the given color ("" for the terminal default).
// It returns as soon as the text is buffered; a printer goroutine releases it
// one character at a time.
func (tw *Typewriter) Push(text, color string) {
	if text == "" {
		return
	}
	seg := twSegment{text: text, color: color, lineEnd: strings.HasSuffix(text, "\n")}

	if !tw.Enabled() {
		tw.write(color + text + colors.GetColor("reset"))
		tw.notifySegmentEnd(seg)
		return
	}

	tw.enqueueAndStart(seg)
}

// PushNow queues colored text that is written whole, without animation, as soon
// as everything ahead of it has been written. Status lines (goal, compression,
// scheduled deliveries) use this: they are single messages, not a token stream,
// so animating them would only make them late.
func (tw *Typewriter) PushNow(text, color string) {
	if text == "" {
		return
	}
	seg := twSegment{
		text:    text,
		color:   color,
		whole:   true,
		lineEnd: strings.HasSuffix(text, "\n"),
	}
	tw.enqueueAndStart(seg)
}

// PushRaw queues control output that is written verbatim: no color wrapping, no
// animation, no splitting. Used for the reasoning separator and for tool-call
// lines that are rewritten in place.
func (tw *Typewriter) PushRaw(text string) {
	if text == "" {
		return
	}
	seg := twSegment{text: text, whole: true, raw: true, lineEnd: strings.HasSuffix(text, "\n")}
	tw.enqueueAndStart(seg)
}

// enqueueAndStart queues a segment and makes sure a printer is alive to drain
// it.
func (tw *Typewriter) enqueueAndStart(seg twSegment) {
	tw.mu.Lock()
	tw.enqueue(seg)
	start := !tw.running
	tw.running = true
	tw.mu.Unlock()

	if start {
		go tw.print()
	}
	tw.poke(tw.wake)
}

// Flush blocks until everything buffered has been written. It is what callers
// use before printing through some other path, so ordering survives.
func (tw *Typewriter) Flush() {
	for {
		tw.mu.Lock()
		tw.fast = true
		pending := len(tw.queue)
		painting := tw.painting
		running := tw.running
		// A color left open still needs its reset: the caller is about to print
		// through some other path and must not inherit the stream's color.
		open := tw.openColor != ""
		tw.mu.Unlock()

		if pending == 0 && !painting && !open {
			// The flush is satisfied: drop the request so the next arrival is
			// animated again instead of being dumped in one go.
			tw.mu.Lock()
			tw.fast = false
			tw.mu.Unlock()
			return
		}
		if !running {
			// The printer exited on an empty buffer just before this push.
			tw.mu.Lock()
			tw.running = true
			tw.mu.Unlock()
			go tw.print()
		}
		tw.poke(tw.wake)

		// Wait for the next completed step, then re-check. A stale tick token
		// only costs one extra iteration.
		<-tw.tick
	}
}

// Pending returns the number of runes still waiting to be painted.
func (tw *Typewriter) Pending() int {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	return tw.pending
}

// enqueue appends a segment and re-derives the character interval from the new
// buffer size. This is the "constantly recalculated" part of the model: every
// arrival re-plans the pace so the whole buffer still fits in one window.
// Caller must hold tw.mu.
func (tw *Typewriter) enqueue(seg twSegment) {
	tw.queue = append(tw.queue, seg)
	if seg.whole {
		return
	}
	tw.pending += utf8.RuneCountInString(seg.text)
	tw.interval = tw.window / time.Duration(tw.pending)
}

// print is the printer goroutine. It exits once the buffer drains, so no
// goroutine lingers between runs.
func (tw *Typewriter) print() {
	// due is when the next character is scheduled. The schedule is absolute:
	// each step advances it by one interval instead of starting a fresh sleep.
	// A timer on Linux overshoots by up to a millisecond, which would push a
	// fast stream past its window (300 buffered runes ask for a third of that);
	// anchoring on a deadline lets the loop skip a wait and stay on schedule.
	var due time.Time

	for {
		tw.mu.Lock()
		if len(tw.queue) == 0 {
			// Close any color still in effect before declaring the buffer
			// empty, so a caller released by Flush never inherits it.
			closing := tw.openColor != ""
			if closing {
				tw.painting = true
			}
			tw.mu.Unlock()

			if closing {
				tw.write(colors.GetColor("reset"))
				tw.mu.Lock()
				tw.openColor = ""
				tw.painting = false
				tw.poke(tw.tick)
				tw.mu.Unlock()
			}

			tw.mu.Lock()
			tw.running = false
			tw.fast = false
			tw.interval = 0
			tw.mu.Unlock()
			return
		}

		seg := tw.queue[0]
		var out string
		var delay time.Duration
		var segmentDone bool

		if seg.whole {
			tw.queue = tw.queue[1:]
			if seg.raw {
				out = seg.text
				if tw.openColor != "" {
					out = colors.GetColor("reset") + out
					tw.openColor = ""
				}
			} else {
				// Self-contained: the segment opens and closes its own color,
				// leaving nothing open behind it.
				out = seg.color + seg.text + colors.GetColor("reset")
				tw.openColor = ""
			}
			segmentDone = true
		} else {
			r, size := utf8.DecodeRuneInString(seg.text)
			rest := seg.text[size:]
			if rest == "" {
				tw.queue = tw.queue[1:]
			} else {
				tw.queue[0].text = rest
			}
			tw.pending--
			segmentDone = rest == ""

			// The interval was set when the buffer last grew; it is what keeps
			// the whole buffer inside one window.
			delay = tw.interval

			// Color codes are emitted on transitions only, which keeps the
			// escape traffic down compared with wrapping every character.
			if tw.openColor != seg.color {
				if tw.openColor != "" {
					out = colors.GetColor("reset")
				}
				if seg.color != "" {
					out += seg.color
				}
				tw.openColor = seg.color
			}
			out += string(r)
		}

		tw.painting = true
		tw.mu.Unlock()

		tw.write(out)

		if segmentDone {
			tw.notifySegmentEnd(seg)
		}

		tw.mu.Lock()
		tw.painting = false
		fast := tw.fast
		tw.mu.Unlock()

		tw.poke(tw.tick)

		// A new arrival interrupts the pause: the interval is re-derived from
		// the larger buffer, so a burst speeds the animation up immediately
		// instead of queueing behind the old pace.
		if !fast && delay > 0 {
			if due.IsZero() {
				due = time.Now()
			}
			due = due.Add(delay)
			if wait := time.Until(due); wait > 0 {
				select {
				case <-time.After(wait):
				case <-tw.wake:
					// Re-anchor: the pace was just re-planned for the larger
					// buffer, so the next character goes out now.
					due = time.Now()
				}
			}
		} else {
			// A flush skipped the schedule; start a fresh one afterwards.
			due = time.Time{}
		}
	}
}

// notifySegmentEnd invokes the hook outside tw.mu: the hook takes the terminal
// lock, which the printer also needs.
func (tw *Typewriter) notifySegmentEnd(seg twSegment) {
	tw.mu.Lock()
	hook := tw.onSegmentEnd
	tw.mu.Unlock()
	if hook != nil {
		hook(seg.lineEnd)
	}
}

// poke signals a channel without ever blocking on a full one.
func (tw *Typewriter) poke(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}
