package tui

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/coding-agent/harness/colors"
	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
)

// captureStdout redirects os.Stdout for the duration of fn and returns what was
// written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stdout
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()

	fn()

	w.Close()
	os.Stdout = saved
	out := <-done
	r.Close()
	return out
}

func TestSetScheduleCount(t *testing.T) {
	tui := NewTUI(config.DefaultConfig())

	if tui.ScheduleCount() != 0 {
		t.Errorf("a fresh TUI should show no schedules, got %d", tui.ScheduleCount())
	}
	tui.SetScheduleCount(3)
	if tui.ScheduleCount() != 3 {
		t.Errorf("expected the counter to be 3, got %d", tui.ScheduleCount())
	}
	tui.SetScheduleCount(0)
	if tui.ScheduleCount() != 0 {
		t.Errorf("expected the counter to be cleared, got %d", tui.ScheduleCount())
	}
}

func TestPrintScheduleCountInternal(t *testing.T) {
	tui := NewTUI(config.DefaultConfig())

	// No schedules: the counter must not clutter the prompt bar.
	out := captureStdout(t, func() {
		tui.printScheduleCountInternal()
	})
	if out != "" {
		t.Errorf("expected no counter output with 0 schedules, got %q", out)
	}

	tui.SetScheduleCount(2)
	out = captureStdout(t, func() {
		tui.printScheduleCountInternal()
	})
	if !strings.Contains(out, "[Schedules: 2]") {
		t.Errorf("expected a [Schedules: 2] counter, got %q", out)
	}
}

// The counter must appear on the prompt bar, next to the context indicator.
func TestPromptBarShowsScheduleCount(t *testing.T) {
	tui := NewTUI(config.DefaultConfig())
	tui.SetContextSize(100, 1000)
	tui.SetScheduleCount(4)

	out := captureStdout(t, func() {
		tui.printContextSize()
	})
	if !strings.Contains(out, "[Context:") {
		t.Errorf("prompt bar lost the context indicator: %q", out)
	}
	if !strings.Contains(out, "[Schedules: 4]") {
		t.Errorf("prompt bar is missing the schedule counter: %q", out)
	}
	if strings.Index(out, "[Schedules:") < strings.Index(out, "[Context:") {
		t.Error("the schedule counter should follow the context indicator")
	}
}

// A scheduled delivery is rendered with the dedicated schedule colour and, when
// the user is typing, the prompt line is repainted so the input survives.
func TestStreamChunkWithTypeSchedule(t *testing.T) {
	tui := NewTUI(config.DefaultConfig())

	out := captureStdout(t, func() {
		tui.StreamChunkWithType("\n[Scheduled message #1] check the build\n", inference.StreamingContentTypeSchedule)
	})
	if !strings.Contains(out, "[Scheduled message #1] check the build") {
		t.Errorf("schedule notice was not printed: %q", out)
	}
	if !strings.Contains(out, colors.GetColor("cyan")) {
		t.Error("schedule notices should be highlighted in cyan")
	}
	if tui.streamBuffer.Len() == 0 {
		t.Error("schedule notices should be part of the streamed output")
	}
}

func TestScheduleDeliveryRepaintsPromptLine(t *testing.T) {
	tui := NewTUI(config.DefaultConfig())
	tui.SetScheduleCount(1)

	// Simulate the user having typed "fix the" at the prompt.
	tui.mu.Lock()
	tui.promptActive = true
	tui.inputLine = "fix the"
	tui.mu.Unlock()

	out := captureStdout(t, func() {
		tui.StreamChunkWithType("\n[Scheduled message #2] stand up\n", inference.StreamingContentTypeSchedule)
	})

	if !strings.Contains(out, "\033[2K") {
		t.Error("the prompt line should be cleared before repainting")
	}
	if !strings.Contains(out, "> fix the") {
		t.Errorf("the typed input should be restored after the notice, got %q", out)
	}
	if !strings.Contains(out, "[Schedules: 1]") {
		t.Error("the repainted prompt bar should still show the schedule counter")
	}
}

// Changing the counter while the user is typing refreshes the prompt bar.
func TestSetScheduleCountRefreshesActivePrompt(t *testing.T) {
	tui := NewTUI(config.DefaultConfig())
	tui.mu.Lock()
	tui.promptActive = true
	tui.inputLine = "ping"
	tui.mu.Unlock()

	out := captureStdout(t, func() {
		tui.SetScheduleCount(2)
	})
	if !strings.Contains(out, "[Schedules: 2]") {
		t.Errorf("the active prompt bar should be redrawn with the new counter, got %q", out)
	}
	if !strings.Contains(out, "> ping") {
		t.Errorf("the typed input should survive the redraw, got %q", out)
	}

	// With no active prompt nothing is printed.
	tui.mu.Lock()
	tui.promptActive = false
	tui.mu.Unlock()
	out = captureStdout(t, func() {
		tui.SetScheduleCount(3)
	})
	if out != "" {
		t.Errorf("no redraw should happen away from the prompt, got %q", out)
	}
}

// A scheduled run prints model output while the user is at the prompt, so the
// input line must survive it.
func TestScheduledRunOutputRepaintsPromptLine(t *testing.T) {
	tui := NewTUI(config.DefaultConfig())
	tui.SetScheduleCount(1)

	tui.mu.Lock()
	tui.promptActive = true
	tui.inputLine = "fix the"
	tui.mu.Unlock()

	out := captureStdout(t, func() {
		tui.StreamChunkWithType("\nbuild is green now\n", inference.StreamingContentTypeNormal)
	})
	if !strings.Contains(out, "build is green now") {
		t.Errorf("model output was not printed: %q", out)
	}
	if !strings.Contains(out, "\033[2K") {
		t.Error("the prompt line should be repainted after a line of scheduled output")
	}
	if !strings.Contains(out, "> fix the") {
		t.Errorf("the typed input should be restored, got %q", out)
	}
}

// Repainting happens at line boundaries only, and never while a run owns the
// terminal, so streaming stays flicker-free.
func TestStreamedOutputRepaintOnlyAtLineBoundaries(t *testing.T) {
	tui := NewTUI(config.DefaultConfig())
	tui.mu.Lock()
	tui.promptActive = true
	tui.inputLine = "ping"
	tui.mu.Unlock()

	out := captureStdout(t, func() {
		tui.StreamChunkWithType("partial", inference.StreamingContentTypeNormal)
	})
	if strings.Contains(out, "\033[2K") {
		t.Errorf("mid-word chunks should not repaint the prompt, got %q", out)
	}

	tui.mu.Lock()
	tui.promptActive = false
	tui.mu.Unlock()
	out = captureStdout(t, func() {
		tui.StreamChunkWithType("a full line\n", inference.StreamingContentTypeNormal)
	})
	if strings.Contains(out, "\033[2K") {
		t.Errorf("no repaint should happen while a run owns the terminal, got %q", out)
	}
}

// When a scheduled run's output ends mid-line, the prompt bar must be restored
// on a fresh line instead of being glued to the answer.
func TestEndScheduledOutputRestoresPrompt(t *testing.T) {
	tui := NewTUI(config.DefaultConfig())
	tui.SetScheduleCount(2)
	tui.mu.Lock()
	tui.promptActive = true
	tui.inputLine = "next task"
	tui.mu.Unlock()

	// Output without a trailing newline leaves the cursor mid-line.
	tui.StreamChunkWithType("done, all tests pass", inference.StreamingContentTypeNormal)

	out := captureStdout(t, func() {
		tui.EndScheduledOutput()
	})
	if !strings.Contains(out, "\n") {
		t.Error("the output line should be closed before the prompt is redrawn")
	}
	if !strings.Contains(out, "> next task") {
		t.Errorf("the prompt and typed input should be restored, got %q", out)
	}
	if !strings.Contains(out, "[Schedules: 2]") {
		t.Error("the restored prompt bar should keep the schedule counter")
	}
}

// Nothing is printed when no prompt is waiting (e.g. a run owns the terminal).
func TestEndScheduledOutputIdleOnly(t *testing.T) {
	tui := NewTUI(config.DefaultConfig())
	tui.mu.Lock()
	tui.promptActive = false
	tui.mu.Unlock()

	out := captureStdout(t, func() {
		tui.EndScheduledOutput()
	})
	if out != "" {
		t.Errorf("expected no output when no prompt is active, got %q", out)
	}
}
