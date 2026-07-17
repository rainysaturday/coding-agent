package agent

import (
	"testing"

	"github.com/coding-agent/harness/config"
)

func TestReportContextSize_CallbackCalled(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	var receivedSize, receivedMax int
	agent.reportContextSize(func(size, max int) {
		receivedSize = size
		receivedMax = max
	}, 100, cfg.ContextSize)

	if receivedSize != 100 {
		t.Errorf("Expected size 100, got %d", receivedSize)
	}
	if receivedMax != cfg.ContextSize {
		t.Errorf("Expected max %d, got %d", cfg.ContextSize, receivedMax)
	}
}

func TestReportContextSize_NilCallback(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Should not panic with nil callback
	agent.reportContextSize(nil, 0, cfg.ContextSize)
}

func TestReportContextSize_ZeroMax(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	var receivedMax int
	agent.reportContextSize(func(size, max int) {
		_ = size
		receivedMax = max
	}, 50, 0)

	if receivedMax != 0 {
		t.Errorf("Expected max 0, got %d", receivedMax)
	}
}
