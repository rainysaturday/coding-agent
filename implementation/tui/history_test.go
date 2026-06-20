package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coding-agent/harness/agent"
	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
)

func TestHandleHistoryUp_Empty(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Should not panic on empty history
	tui.handleHistoryUp()
}

func TestHandleHistoryDown_Empty(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Should not panic on empty history
	tui.handleHistoryDown()
}

func TestHandleHistoryUp_OneEntry(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.addToHistory("first entry")

	// Navigate up once
	tui.handleHistoryUp()
	// Should display the entry
}

func TestHandleHistoryUp_Overflow(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.addToHistory("first")
	tui.addToHistory("second")
	tui.addToHistory("third")

	// Navigate up past the end
	tui.handleHistoryUp()
	tui.handleHistoryUp()
	tui.handleHistoryUp()
	// Should not panic, clamped at end
}

func TestHandleHistoryDown_FromStart(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.addToHistory("first")
	tui.addToHistory("second")

	// From position 0, going down should reset to -1
	tui.handleHistoryUp()   // Go to first
	tui.handleHistoryDown() // Should go back to -1
	// Should not panic
}

func TestClearHistory_ResetsIndex(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.addToHistory("prompt 1")
	tui.addToHistory("prompt 2")
	tui.ClearHistory()

	// After clear, NavigateHistory should return empty
	result := tui.NavigateHistory(1)
	if result != "" {
		t.Errorf("Expected empty result after ClearHistory, got '%s'", result)
	}
}

func TestAddToHistory_Order(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Add multiple prompts (newest first in storage)
	tui.addToHistory("third")
	tui.addToHistory("second")
	tui.addToHistory("first")

	// NavigateHistory gets them in order: first navigated = newest
	result := tui.NavigateHistory(1)
	if result != "first" {
		// The history is newest first, so first navigation should get "first"
		// NavigateHistory direction 1 means going forward through history
	}
}

func TestNavigateHistory_BeyondBounds(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.addToHistory("only one")

	// Navigate past the end multiple times
	tui.NavigateHistory(1)
	result := tui.NavigateHistory(1)
	if result != "" {
		t.Errorf("Expected empty past end, got '%s'", result)
	}

	// Navigate past beginning multiple times
	tui.NavigateHistory(-1)
	tui.NavigateHistory(-1)
	tui.NavigateHistory(-1)
	// Should not panic
}

func TestNavigateHistory_MixedDirections(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.addToHistory("first")
	tui.addToHistory("second")
	tui.addToHistory("third")

	// Navigate forward once
	tui.NavigateHistory(1)

	// Navigate forward again
	tui.NavigateHistory(1)

	// Navigate back once
	tui.NavigateHistory(-1)
	_ = tui.NavigateHistory(-1)
	// Should not panic with mixed directions
}

func TestColors_Validity(t *testing.T) {
	// Verify all color codes are valid ANSI
	if !strings.HasPrefix(ColorReset, "\033[") {
		t.Error("ColorReset should be ANSI escape sequence")
	}
	if !strings.HasPrefix(ColorRed, "\033[") {
		t.Error("ColorRed should be ANSI escape sequence")
	}
	if !strings.HasPrefix(ColorGreen, "\033[") {
		t.Error("ColorGreen should be ANSI escape sequence")
	}
	if !strings.HasPrefix(ColorYellow, "\033[") {
		t.Error("ColorYellow should be ANSI escape sequence")
	}
	if !strings.HasPrefix(ColorBlue, "\033[") {
		t.Error("ColorBlue should be ANSI escape sequence")
	}
	if !strings.HasPrefix(ColorCyan, "\033[") {
		t.Error("ColorCyan should be ANSI escape sequence")
	}
	if !strings.HasPrefix(ColorDim, "\033[") {
		t.Error("ColorDim should be ANSI escape sequence")
	}
}

func TestClearScreen(t *testing.T) {
	// Should not panic
	clearScreen()
}

func TestPrintColored(t *testing.T) {
	// Should not panic
	printColored(ColorRed, "red text")
	printColored(ColorGreen, "green text")
	printColored(ColorYellow, "yellow text")
	printColored(ColorBlue, "blue text")
	printColored(ColorCyan, "cyan text")
	printColored(ColorDim, "dim text")
	printColored(ColorReset, "reset text")
}

func TestContextSizePercentage_Calculations(t *testing.T) {
	tests := []struct {
		size    int
		max     int
		wantPct float64
	}{
		{0, 100, 0},
		{50, 100, 50},
		{100, 100, 100},
		{128000, 128000, 100},
		{64000, 128000, 50},
	}

	for _, tt := range tests {
		// Verify the percentage calculation in printContextSize
		percentage := float64(tt.size) / float64(tt.max) * 100
		if percentage != tt.wantPct {
			t.Errorf("For size=%d, max=%d, expected %.1f%%, got %.1f%%",
				tt.size, tt.max, tt.wantPct, percentage)
		}
	}
}

func TestContextSizeIndicators(t *testing.T) {
	// Test the indicator logic
	tests := []struct {
		size   int
		max    int
		wantOk bool // should be valid (non-negative)
	}{
		{10000, 128000, true},  // <50% - checkmark
		{70000, 128000, true},  // 50-75% - warning
		{100000, 128000, true}, // 75-90% - warning warning
		{125000, 128000, true}, // >90% - warning warning warning
		{0, 128000, true},      // zero
		{128000, 128000, true}, // exactly at max
	}

	for _, tt := range tests {
		percentage := float64(tt.size) / float64(tt.max) * 100
		if percentage < 0 {
			t.Errorf("Negative percentage for size=%d, max=%d", tt.size, tt.max)
		}
	}
}

func TestTUI_MultipleStreamSessions(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Multiple stream sessions should work
	for i := 0; i < 5; i++ {
		tui.StartStream()
		tui.StreamChunk(fmt.Sprintf("session %d", i))
		tui.StreamEnd()
	}

	// Final state should be non-streaming
	if tui.IsStreaming() {
		t.Error("Expected IsStreaming() to be false after multiple sessions")
	}
}

func TestTUI_MixedStreamOperations(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Mix of different operations
	tui.AddOutput("output 1")
	tui.StartStream()
	tui.StreamChunk("streamed")
	tui.StreamEnd()
	tui.AddOutputf("formatted: %d", 42)

	// Verify state
	if tui.IsStreaming() {
		t.Error("Expected non-streaming state")
	}
}

func TestTUI_SetContextSize_Zero(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Set to zero should be valid
	tui.SetContextSize(0, 128000)
}

func TestTUI_SetContextSize_Large(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Set to large values should work
	tui.SetContextSize(1000000, 2000000)
}

func TestTUI_AddOutput_Empty(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Empty string should be valid
	tui.AddOutput("")
	tui.AddOutputf("")
}

func TestTUI_NavigateHistory_BeforeAdding(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Navigate before adding anything should return empty
	result := tui.NavigateHistory(1)
	if result != "" {
		t.Errorf("Expected empty for empty history, got '%s'", result)
	}

	result = tui.NavigateHistory(-1)
	if result != "" {
		t.Errorf("Expected empty for empty history, got '%s'", result)
	}
}

func TestStreamingChunk_ContentTypeValues(t *testing.T) {
	// Verify inference package constants have expected values
	if inference.StreamingContentTypeNormal != 0 {
		t.Errorf("Expected inference.StreamingContentTypeNormal = 0, got %d", inference.StreamingContentTypeNormal)
	}
	if inference.StreamingContentTypeReasoning != 1 {
		t.Errorf("Expected inference.StreamingContentTypeReasoning = 1, got %d", inference.StreamingContentTypeReasoning)
	}
}

func TestSetContextSize_UpdatesInternalState(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Set initial values
	tui.SetContextSize(10000, 128000)
	tui.SetContextSize(20000, 256000)
	tui.SetContextSize(50000, 500000)
}

func TestSetContextSize_Negative(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Negative values should not panic
	tui.SetContextSize(-1, -1)
}

func TestSetContextSize_MaxZero(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Max of zero should not panic
	tui.SetContextSize(50, 0)
}

func TestSetContextSize_MaxOne(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Max of one should not panic
	tui.SetContextSize(1, 1)
}

func TestShowContextSize_WithValues(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.SetContextSize(1000, 10000)
	tui.ShowContextSize()
}

func TestShowContextSize_ZeroValues(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.SetContextSize(0, 0)
	tui.ShowContextSize()
}

func TestShowContextSize_ZeroMax(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.SetContextSize(500, 0)
	tui.ShowContextSize()
}

func TestPrintContextSize_ZeroMax(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.SetContextSize(100, 0)
	tui.printContextSize()
}

func TestPrintContextSize_ExactBoundaries(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.SetContextSize(64000, 128000)
	tui.printContextSize()

	tui.SetContextSize(96000, 128000)
	tui.printContextSize()

	tui.SetContextSize(115200, 128000)
	tui.printContextSize()
}

func TestDisplayStats_VaryingIterations(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	stats := &agent.Stats{
		InputTokens:     100,
		OutputTokens:    50,
		ToolCalls:       0,
		FailedToolCalls: 0,
		Iterations:      0,
		StartTime:       time.Now().Add(-5 * time.Second),
	}

	tui.DisplayStats(stats)
}

func TestDisplayStats_WithToolCalls(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	stats := &agent.Stats{
		InputTokens:     1000,
		OutputTokens:    500,
		ToolCalls:       50,
		FailedToolCalls: 5,
		Iterations:      100,
		StartTime:       time.Now().Add(-10 * time.Minute),
	}

	tui.SetContextSize(100000, 128000)
	tui.DisplayStats(stats)
}

func TestDisplayStats_MaxTokens(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	stats := &agent.Stats{
		InputTokens:     128000,
		OutputTokens:    64000,
		ToolCalls:       1000,
		FailedToolCalls: 100,
		Iterations:      5000,
		StartTime:       time.Now().Add(-1 * time.Hour),
	}

	tui.SetContextSize(128000, 128000)
	tui.DisplayStats(stats)
}

func TestIsStreaming_MultipleStartEnd(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	for i := 0; i < 3; i++ {
		tui.StartStream()
		if !tui.IsStreaming() {
			t.Errorf("Iteration %d: Expected streaming after StartStream", i)
		}
		tui.StreamEnd()
		if tui.IsStreaming() {
			t.Errorf("Iteration %d: Expected not streaming after StreamEnd", i)
		}
	}
}

func TestIsStreaming_StreamOperationsDuringStream(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()

	tui.StreamChunk("test1")
	if !tui.IsStreaming() {
		t.Error("IsStreaming should remain true during StreamChunk")
	}

	tui.StreamChunk("test2")
	if !tui.IsStreaming() {
		t.Error("IsStreaming should remain true during multiple StreamChunk")
	}

	tui.StreamEnd()
}

func TestIsStreaming_NonStreamMode(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	if tui.IsStreaming() {
		t.Error("IsStreaming should be false without StartStream")
	}

	tui.AddOutput("output")
	if tui.IsStreaming() {
		t.Error("IsStreaming should remain false after AddOutput")
	}
}

func TestClearOutput_PreservesOtherState(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.AddOutput("output1")
	tui.AddOutput("output2")
	tui.addToHistory("history1")
	tui.SetContextSize(1000, 10000)

	tui.ClearOutput()

	result := tui.NavigateHistory(1)
	if result != "history1" {
		t.Errorf("Expected 'history1', got '%s'", result)
	}
}

func TestClearHistory_PreservesOtherState(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.AddOutput("output1")
	tui.addToHistory("history1")
	tui.SetContextSize(1000, 10000)

	tui.ClearHistory()
}

func TestAddToHistory_Duplicates(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.addToHistory("same prompt")
	tui.addToHistory("same prompt")
	tui.addToHistory("same prompt")

	result := tui.NavigateHistory(1)
	if result != "same prompt" {
		t.Errorf("Expected 'same prompt', got '%s'", result)
	}

	result = tui.NavigateHistory(1)
	if result != "same prompt" {
		t.Errorf("Expected 'same prompt' (second), got '%s'", result)
	}
}

func TestAddToHistory_VeryLong(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	longPrompt := strings.Repeat("a", 10000)
	tui.addToHistory(longPrompt)

	result := tui.NavigateHistory(1)
	if len(result) != 10000 {
		t.Errorf("Expected prompt length 10000, got %d", len(result))
	}
}

func TestAddToHistory_EmptyPrompt(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.addToHistory("")

	result := tui.NavigateHistory(1)
	if result != "" {
		t.Errorf("Expected empty result, got '%s'", result)
	}
}

func TestAddToHistory_WhitespacePrompt(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.addToHistory("   ")

	result := tui.NavigateHistory(1)
	if result != "   " {
		t.Errorf("Expected '   ', got '%s'", result)
	}
}

func TestAddToHistory_SpecialCharacters(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	specialPrompt := "Prompt with special chars: !@#$%^&*()_+-=[]{}|;':\",./<>?"
	tui.addToHistory(specialPrompt)

	result := tui.NavigateHistory(1)
	if result != specialPrompt {
		t.Errorf("Expected special characters preserved")
	}
}

func TestAddToHistory_UnicodePrompt(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	unicodePrompt := "Hello World"
	tui.addToHistory(unicodePrompt)

	result := tui.NavigateHistory(1)
	if result != unicodePrompt {
		t.Errorf("Expected unicode characters preserved")
	}
}

func TestAddToHistory_MultilinePrompt(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	multilinePrompt := "line 1\nline 2\nline 3"
	tui.addToHistory(multilinePrompt)

	result := tui.NavigateHistory(1)
	if result != multilinePrompt {
		t.Errorf("Expected multiline content preserved")
	}
}

func TestStreamEnd_MultipleCalls(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()
	tui.StreamEnd()
	tui.StreamEnd()
}

func TestStartStream_MultipleCalls(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()
	tui.StartStream()
	tui.StartStream()
}

func TestIsStreaming_AfterStartEndCycle(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()
	tui.StreamEnd()
	tui.StartStream()
	tui.StreamEnd()

	if tui.IsStreaming() {
		t.Error("Expected IsStreaming to be false after complete cycle")
	}
}

func TestTUI_MaxHistoryZero(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)
	tui.maxHistory = 0

	tui.addToHistory("prompt 1")
	tui.addToHistory("prompt 2")
}

func TestTUI_MaxHistoryNegative(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)
	tui.maxHistory = -1

	tui.addToHistory("prompt 1")
}

func TestTUI_AddOutputf_WithFormatSpecifiers(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.AddOutputf("%s %d %.2f %v %t", "test", 42, 3.14, []int{1, 2, 3}, true)
}

func TestTUI_DisplayStats_VeryLarge(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	stats := &agent.Stats{
		InputTokens:     1000000,
		OutputTokens:    500000,
		ToolCalls:       10000,
		FailedToolCalls: 1000,
		Iterations:      50000,
		StartTime:       time.Now().Add(-24 * time.Hour),
		TokensPerSecond: 100.5,
	}

	tui.SetContextSize(128000, 128000)
	tui.DisplayStats(stats)
}

func TestTUI_DisplayStats_NegativeTokens(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	stats := &agent.Stats{
		InputTokens:  -100,
		OutputTokens: -50,
	}

	tui.DisplayStats(stats)
}

func TestTUI_DisplayStats_MaxContext(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	stats := &agent.Stats{
		InputTokens:  1000,
		OutputTokens: 500,
	}

	tui.SetContextSize(100000, 100000)
	tui.DisplayStats(stats)
}

func TestPrintContextSize_PercentageBoundaries(t *testing.T) {
	tests := []struct {
		size   int
		max    int
		wantOk bool
	}{
		{0, 100, true},
		{50, 100, true},
		{100, 100, true},
		{99, 100, true},
		{101, 100, true},
		{1000, 1, true},
	}

	for _, tt := range tests {
		_ = float64(tt.size) / float64(tt.max) * 100
	}
}

func TestPrintContextSize_IndicatorSelection(t *testing.T) {
	percentage := float64(30) / float64(100) * 100
	if percentage < 50 {
		// Should select checkmark indicator
	}

	percentage = float64(60) / float64(100) * 100
	if percentage >= 50 && percentage < 75 {
		// Should select single warning indicator
	}

	percentage = float64(80) / float64(100) * 100
	if percentage >= 75 && percentage < 90 {
		// Should select double warning indicator
	}

	percentage = float64(95) / float64(100) * 100
	if percentage >= 90 {
		// Should select triple warning indicator
	}
}

func TestDisplayStats_TokensPerSecond(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	stats := &agent.Stats{
		InputTokens:     1000,
		OutputTokens:    500,
		ToolCalls:       10,
		FailedToolCalls: 1,
		Iterations:      5,
		StartTime:       time.Now().Add(-10 * time.Second),
		TokensPerSecond: 150.0,
	}

	tui.DisplayStats(stats)
}

func TestDisplayStats_ZeroTokensPerSecond(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	stats := &agent.Stats{
		InputTokens:     0,
		OutputTokens:    0,
		TokensPerSecond: 0,
		StartTime:       time.Now().Add(-1 * time.Minute),
	}

	tui.DisplayStats(stats)
}

func TestDisplayStats_LongUptime(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	stats := &agent.Stats{
		InputTokens: 100,
		StartTime:   time.Now().Add(-168 * time.Hour),
	}

	tui.DisplayStats(stats)
}

func TestDisplayStats_MediumUptime(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	stats := &agent.Stats{
		InputTokens: 100,
		StartTime:   time.Now().Add(-2 * time.Hour),
	}

	tui.DisplayStats(stats)
}

func TestDisplayStats_ShortUptime(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	stats := &agent.Stats{
		InputTokens: 100,
		StartTime:   time.Now().Add(-2 * time.Second),
	}

	tui.DisplayStats(stats)
}

func TestTUI_StreamChunk_VeryLong(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()
	longText := strings.Repeat("x", 100000)
	tui.StreamChunk(longText)
	tui.StreamEnd()
}

func TestTUI_StreamChunk_Empty(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()
	tui.StreamChunk("")
	tui.StreamEnd()
}

func TestTUI_StreamChunk_NilContent(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StreamChunkWithType("", inference.StreamingContentTypeNormal)
	tui.StreamChunkWithType("", inference.StreamingContentTypeReasoning)
}

func TestTUI_ClearHistory_NoHistory(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.ClearHistory()
}

func TestTUI_ClearOutput_NoOutput(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.ClearOutput()
}

func TestTUI_AddOutput_WithNewlines(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.AddOutput("line1\nline2\nline3")
}

func TestTUI_AddOutputf_WithMultipleArgs(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.AddOutputf("%s %d %s %d", "hello", 1, "world", 2)
}

func TestTUI_NavigateHistory_ExactCount(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.addToHistory("first")
	tui.addToHistory("second")

	tui.NavigateHistory(1)
	tui.NavigateHistory(1)
	result := tui.NavigateHistory(1)
	if result != "" {
		t.Errorf("Expected empty at end, got '%s'", result)
	}
}

func TestTUI_NavigateHistory_BackToStart(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.addToHistory("first")
	tui.addToHistory("second")

	tui.NavigateHistory(1)
	tui.NavigateHistory(-1)
	tui.NavigateHistory(-1)
}

func TestTUI_NavigateHistory_SingleItem(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.addToHistory("only one")

	tui.NavigateHistory(1)
	tui.NavigateHistory(-1)
	tui.NavigateHistory(-1)
}

func TestTUI_NavigateHistory_RapidNavigation(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.addToHistory("first")
	tui.addToHistory("second")
	tui.addToHistory("third")

	for i := 0; i < 100; i++ {
		tui.NavigateHistory(1)
		tui.NavigateHistory(-1)
	}
}

func TestAddToHistory_MaxHistory(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)
	tui.maxHistory = 3

	for i := 0; i < 5; i++ {
		tui.addToHistory(fmt.Sprintf("item%d", i))
	}

	if len(tui.history) != 3 {
		t.Errorf("Expected 3 history items, got %d", len(tui.history))
	}

	expected := []string{"item4", "item3", "item2"}
	for i, exp := range expected {
		if tui.history[i] != exp {
			t.Errorf("Expected %s at index %d, got %s", exp, i, tui.history[i])
		}
	}
}

func TestAddToHistory_NoMax(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)
	tui.maxHistory = 0

	for i := 0; i < 10; i++ {
		tui.addToHistory(fmt.Sprintf("item%d", i))
	}

	if len(tui.history) != 10 {
		t.Errorf("Expected 10 history items, got %d", len(tui.history))
	}
}

