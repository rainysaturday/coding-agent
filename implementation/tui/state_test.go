package tui

import (
	"fmt"
	"testing"

	"github.com/coding-agent/harness/agent"
	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
)

func TestTUI_SetContextSize_RapidUpdates(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	for i := 0; i < 100; i++ {
		tui.SetContextSize(i, i*2)
	}
}

func TestTUI_MultipleCancelOperations(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	for i := 0; i < 10; i++ {
		tui.CancelOperation()
	}
}

func TestTUI_MixedOperations(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.AddOutput("test")
	tui.AddOutputf("formatted %d", 1)
	tui.StartStream()
	tui.StreamChunk("streaming")
	tui.StreamEnd()
	tui.SetContextSize(100, 1000)
	tui.addToHistory("history")
	tui.NavigateHistory(1)
	tui.CancelOperation()
	tui.ClearOutput()
	tui.ClearHistory()
}

func TestTUI_StreamEnd_EmptyStream(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()
	tui.StreamEnd()
}

func TestTUI_IsStreaming_NeverStarted(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	if tui.IsStreaming() {
		t.Error("Expected IsStreaming to be false")
	}
}

func TestTUI_StreamChunkWithType_NilContent(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StreamChunkWithType("", inference.StreamingContentTypeNormal)
	tui.StreamChunkWithType("", inference.StreamingContentTypeReasoning)
}

func TestTUI_ShowContextSize_NoContext(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.ShowContextSize()
}

func TestTUI_SetContextSize_ToMax(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.SetContextSize(128000, 128000)
}

func TestTUI_SetContextSize_ExceedsMax(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.SetContextSize(200000, 128000)
}

func TestTUI_AddOutput_CalledManyTimes(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	for i := 0; i < 100; i++ {
		tui.AddOutput(fmt.Sprintf("output %d", i))
	}
}

func TestTUI_StartStream_AlreadyStreaming(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()
	tui.StartStream()
}

func TestTUI_StreamEnd_NeverStarted(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StreamEnd()
}

func TestTUI_IsStreaming_AfterClearOutput(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()
	tui.StreamChunk("test")
	tui.ClearOutput()
	tui.StreamEnd()

	if tui.IsStreaming() {
		t.Error("Expected IsStreaming to be false")
	}
}

func TestTUI_IsStreaming_AfterClearHistory(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.addToHistory("test")
	tui.StartStream()
	tui.ClearHistory()
	tui.StreamEnd()

	if tui.IsStreaming() {
		t.Error("Expected IsStreaming to be false")
	}
}

func TestTUI_IsStreaming_AfterSetContextSize(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()
	tui.SetContextSize(100, 1000)
	tui.StreamEnd()

	if tui.IsStreaming() {
		t.Error("Expected IsStreaming to be false")
	}
}

func TestTUI_IsStreaming_AfterAddOutput(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()
	tui.AddOutput("test")
	tui.StreamEnd()

	if tui.IsStreaming() {
		t.Error("Expected IsStreaming to be false")
	}
}

func TestTUI_IsStreaming_AfterAddOutputf(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()
	tui.AddOutputf("formatted %s", "test")
	tui.StreamEnd()

	if tui.IsStreaming() {
		t.Error("Expected IsStreaming to be false")
	}
}

func TestTUI_IsStreaming_AfterCancelOperation(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()
	tui.CancelOperation()
	tui.StreamEnd()

	if tui.IsStreaming() {
		t.Error("Expected IsStreaming to be false")
	}
}

func TestTUI_IsStreaming_AfterNavigateHistory(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.addToHistory("test")
	tui.StartStream()
	tui.NavigateHistory(1)
	tui.StreamEnd()

	if tui.IsStreaming() {
		t.Error("Expected IsStreaming to be false")
	}
}

func TestTUI_IsStreaming_AfterDisplayStats(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()
	tui.SetContextSize(100, 1000)
	stats := &agent.Stats{InputTokens: 100}
	tui.DisplayStats(stats)
	tui.StreamEnd()

	if tui.IsStreaming() {
		t.Error("Expected IsStreaming to be false")
	}
}

func TestTUI_IsStreaming_AfterShowContextSize(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()
	tui.SetContextSize(100, 1000)
	tui.ShowContextSize()
	tui.StreamEnd()

	if tui.IsStreaming() {
		t.Error("Expected IsStreaming to be false")
	}
}

// ===== Additional tests for addToHistory =====

func TestHandleHistoryDown_FromStart_WithInput(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)
	tui.addToHistory("previous1")
	tui.addToHistory("previous2")

	tui.mu.Lock()
	tui.currentInput = "current text"
	tui.inputLine = "current text"
	tui.mu.Unlock()

	tui.handleHistoryUp()
	tui.handleHistoryDown()

	if tui.historyIndex != -1 {
		t.Errorf("Expected historyIndex to be -1, got %d", tui.historyIndex)
	}
}

// ===== Tests for StreamGoalChunk =====

func TestStreamGoalChunk(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StreamGoalChunk("Goal achieved!")

	tui.mu.Lock()
	defer tui.mu.Unlock()

	// StreamGoalChunk writes to streamBuffer, not output
	if tui.streamBuffer.String() != "Goal achieved!" {
		t.Errorf("Expected 'Goal achieved!' in streamBuffer, got %q", tui.streamBuffer.String())
	}
}

// ===== Tests for StreamChunkWithType with Goal content type =====

func TestStreamChunkWithType_Goal(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StreamChunkWithType("Goal check", inference.StreamingContentTypeGoal)

	tui.mu.Lock()
	defer tui.mu.Unlock()

	if tui.streamBuffer.String() != "Goal check" {
		t.Errorf("Expected 'Goal check' in streamBuffer, got %q", tui.streamBuffer.String())
	}
}

// ===== Tests for StreamChunkWithType with Reasoning content type =====

func TestStreamChunkWithType_Reasoning(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StreamChunkWithType("Thinking...", inference.StreamingContentTypeReasoning)

	tui.mu.Lock()
	defer tui.mu.Unlock()

	if tui.reasoningBuffer.String() != "Thinking..." {
		t.Errorf("Expected 'Thinking...' in reasoningBuffer, got %q", tui.reasoningBuffer.String())
	}
}
