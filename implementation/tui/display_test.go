package tui

import (
	"testing"

	"github.com/coding-agent/harness/config"
)

func TestAddOutputf(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Should not panic
	tui.AddOutputf("formatted %s output %d", "test", 42)
}

