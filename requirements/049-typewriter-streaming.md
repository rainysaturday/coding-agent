# Requirement 049: Typewriter Streaming

## Description
Streaming an LLM response token by token makes the output arrive in bursts: a
token is dumped the moment it arrives, then nothing happens for a moment, then
another chunk appears. The result feels jerky, and it feels *slow*, because the
screen is idle whenever the network is idle.

The TUI must instead buffer the incoming tokens and release them **one letter at
a time**, at a pace derived from the buffer itself: the whole buffer must be
printable within **0.1 seconds**. That interval is **recalculated continuously** —
every time a token arrives and enlarges the buffer, the per-letter interval is
recomputed as `0.1s / (letters still waiting)`. A small buffer therefore prints
quickly (a handful of letters spread over a tenth of a second), a large buffer
prints more slowly, and the output always catches up with the stream within a
tenth of a second.

The effect is purely cosmetic: the text, its colors and its order are unchanged,
and nothing else (tool calls, prompt bar, stats) is delayed by more than that
window.

## Acceptance Criteria
- [x] Streamed model output is buffered and released one letter at a time
- [x] The per-letter interval is `window / pending letters` with a window of
      100 ms, recomputed on every arrival (a burst of 100 letters prints at
      ~1 ms/letter, a buffer of 10 letters at ~10 ms/letter)
- [x] The whole buffer is printable within the 100 ms window; the printer never
      falls further behind the stream than that window allows
- [x] Multi-byte characters are released whole, never as partial runes
- [x] Colors are still applied, and are emitted when the color changes rather
      than around every letter
- [x] The printer stops when the buffer is empty, so an idle stream costs nothing
- [x] Status output (goal notices, compression notices, scheduled deliveries,
      the "thinking complete" rule, in-place tool-call lines) is written whole
      rather than animated, but still in order — it never overtakes text that is
      already on its way to the screen
- [x] Anything that prints to the terminal directly (prompt bar, stats, errors,
      one-shot result) waits for the buffer to drain first, so output stays in
      order
- [x] The animation is enabled by default and can be turned off with
      `--no-typewriter`, `CODING_AGENT_TYPEWRITER=false` or `typewriter=false` in
      the config file
- [x] One-shot (`-p`) mode streams through the same typewriter and flushes
      before the final result is printed
- [x] No external dependencies are introduced (requirement 024)

## Implementation Notes
`implementation/tui/typewriter.go` holds the queue. `Push` appends animated
text, `PushNow`/`PushRaw` append whole (unanimated) segments, and `Flush` blocks
until the buffer is empty and the last color has been closed. A single printer
goroutine drains the queue; it recomputes the interval whenever the buffer
grows, so the pace always reflects the current buffer size. A `wake` signal
interrupts the pending wait when new text arrives, which is what keeps the drain
inside the window even when a burst lands mid-sleep.

The wait is scheduled against an **absolute deadline** rather than by sleeping
per letter. A timer on Linux overshoots by up to a millisecond, and a large
buffer asks for less than that per letter (300 buffered letters in 0.1 s is
0.33 ms each), so sleeping per letter would drift to three times the window.
Anchoring the schedule on a deadline lets the loop skip a wait when it falls
behind: measured, a 10, 100 or 300 letter buffer all drain in ~100 ms, one
letter per step.

The TUI owns one typewriter (`tui.tw`). `StreamChunkWithType` routes normal and
reasoning text through `Push`, status messages through `PushNow` and control
output through `PushRaw`; every entry point that prints directly
(`AddOutput`, `StreamEnd`, `Prompt`, `DisplayStats`, …) calls `FlushStream`
first. The typewriter never takes the terminal lock itself — the write callback
and the segment hook do, so the printer cannot deadlock against a caller that is
already holding it.
