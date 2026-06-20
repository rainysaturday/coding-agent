package tui

import (
	"testing"
	"time"

	"github.com/coding-agent/harness/agent"
	"github.com/coding-agent/harness/config"
)

func TestSetContextSize(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.SetContextSize(50000, 128000)

	// Just verify it doesn't panic
}

func TestShowContextSize(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Just verify it doesn't panic
	tui.ShowContextSize()
}

func TestPrintContextSize_LowUsage(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.SetContextSize(10000, 128000)
	// Low usage should show checkmark
	tui.printContextSize()
}


func TestPrintContextSize_HighUsage(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.SetContextSize(100000, 128000)
	// High usage (75-90%)
	tui.printContextSize()
}


func TestDisplayStats_ZeroContext(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	stats := &agent.Stats{
		InputTokens:     1000,
		OutputTokens:    500,
		ToolCalls:       10,
		FailedToolCalls: 1,
		Iterations:      5,
		StartTime:       time.Now().Add(-1 * time.Minute),
	}

	// Just verify it doesn't panic
	tui.SetContextSize(0, 128000)
	tui.DisplayStats(stats)
}

func TestDisplayStats_NonZeroContext(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	stats := &agent.Stats{
		InputTokens:     1000,
		OutputTokens:    500,
		ToolCalls:       10,
		FailedToolCalls: 1,
		Iterations:      5,
		StartTime:       time.Now().Add(-2 * time.Minute),
	}

	tui.SetContextSize(64000, 128000)
	// Should display with percentage
	tui.DisplayStats(stats)
}

func TestDisplayStats_ZeroStartTime(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	stats := &agent.Stats{
		InputTokens:  1000,
		OutputTokens: 500,
		StartTime:    time.Time{}, // Zero time
	}

	// Should not panic with zero start time
	tui.DisplayStats(stats)
}

