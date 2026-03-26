package stats

import (
	"testing"
	"time"
)

func TestNewStats(t *testing.T) {
	s := NewStats()
	if s == nil {
		t.Fatal("NewStats returned nil")
	}
	if s.GetInputTokens() != 0 {
		t.Errorf("Expected initial input tokens 0, got %d", s.GetInputTokens())
	}
	if s.GetOutputTokens() != 0 {
		t.Errorf("Expected initial output tokens 0, got %d", s.GetOutputTokens())
	}
}

func TestAddInputTokens(t *testing.T) {
	s := NewStats()
	s.AddInputTokens(100)
	s.AddInputTokens(50)

	if s.GetInputTokens() != 150 {
		t.Errorf("Expected input tokens 150, got %d", s.GetInputTokens())
	}
}

func TestAddOutputTokens(t *testing.T) {
	s := NewStats()
	s.AddOutputTokens(200)
	s.AddOutputTokens(75)

	if s.GetOutputTokens() != 275 {
		t.Errorf("Expected output tokens 275, got %d", s.GetOutputTokens())
	}
}

func TestSetInputTokens(t *testing.T) {
	s := NewStats()
	s.AddInputTokens(100)
	s.SetInputTokens(500)

	if s.GetInputTokens() != 500 {
		t.Errorf("Expected input tokens 500, got %d", s.GetInputTokens())
	}
}

func TestSetOutputTokens(t *testing.T) {
	s := NewStats()
	s.AddOutputTokens(200)
	s.SetOutputTokens(800)

	if s.GetOutputTokens() != 800 {
		t.Errorf("Expected output tokens 800, got %d", s.GetOutputTokens())
	}
}

func TestGetTotalTokens(t *testing.T) {
	s := NewStats()
	s.AddInputTokens(100)
	s.AddOutputTokens(50)

	if s.GetTotalTokens() != 150 {
		t.Errorf("Expected total tokens 150, got %d", s.GetTotalTokens())
	}
}

func TestGetTokensPerSecond(t *testing.T) {
	s := NewStats()
	s.AddInputTokens(100)
	s.AddOutputTokens(100)

	// Wait a short time to ensure non-zero elapsed time
	time.Sleep(100 * time.Millisecond)

	tokensPerSecond := s.GetTokensPerSecond()
	if tokensPerSecond <= 0 {
		t.Errorf("Expected positive tokens per second, got %f", tokensPerSecond)
	}
}

func TestTokensPerSecondZeroTime(t *testing.T) {
	s := NewStats()
	s.AddInputTokens(100)
	s.AddOutputTokens(100)

	// Immediately check tokens per second (should handle zero elapsed time)
	tokensPerSecond := s.GetTokensPerSecond()
	// Should not panic and should return 0 if no time has elapsed
	_ = tokensPerSecond
}

func TestToolCallTracking(t *testing.T) {
	s := NewStats()

	s.AddToolCall()
	s.AddToolCall()
	s.AddFailedToolCall()
	s.AddToolCall()

	if s.GetTotalToolCalls() != 3 {
		t.Errorf("Expected total tool calls 3, got %d", s.GetTotalToolCalls())
	}
	if s.GetFailedToolCalls() != 1 {
		t.Errorf("Expected failed tool calls 1, got %d", s.GetFailedToolCalls())
	}
}

func TestIterationTracking(t *testing.T) {
	s := NewStats()

	s.IncrementIteration()
	s.IncrementIteration()
	s.IncrementIteration()

	if s.GetIterationCount() != 3 {
		t.Errorf("Expected iteration count 3, got %d", s.GetIterationCount())
	}
}

func TestReset(t *testing.T) {
	s := NewStats()
	s.AddInputTokens(1000)
	s.AddOutputTokens(500)
	s.AddToolCall()
	s.AddFailedToolCall()
	s.IncrementIteration()

	// Wait a bit to ensure uptime changes
	time.Sleep(10 * time.Millisecond)

	s.Reset()

	if s.GetInputTokens() != 0 {
		t.Errorf("Expected input tokens 0 after reset, got %d", s.GetInputTokens())
	}
	if s.GetOutputTokens() != 0 {
		t.Errorf("Expected output tokens 0 after reset, got %d", s.GetOutputTokens())
	}
	if s.GetTotalToolCalls() != 0 {
		t.Errorf("Expected total tool calls 0 after reset, got %d", s.GetTotalToolCalls())
	}
	if s.GetFailedToolCalls() != 0 {
		t.Errorf("Expected failed tool calls 0 after reset, got %d", s.GetFailedToolCalls())
	}
	if s.GetIterationCount() != 0 {
		t.Errorf("Expected iteration count 0 after reset, got %d", s.GetIterationCount())
	}
}

func TestGetSummary(t *testing.T) {
	s := NewStats()
	s.AddInputTokens(100)
	s.AddOutputTokens(50)
	s.AddToolCall()
	s.AddFailedToolCall()
	s.IncrementIteration()

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	summary := s.GetSummary()

	if summary.InputTokens != 100 {
		t.Errorf("Expected summary input tokens 100, got %d", summary.InputTokens)
	}
	if summary.OutputTokens != 50 {
		t.Errorf("Expected summary output tokens 50, got %d", summary.OutputTokens)
	}
	if summary.TotalToolCalls != 1 {
		t.Errorf("Expected summary total tool calls 1, got %d", summary.TotalToolCalls)
	}
	if summary.FailedToolCalls != 1 {
		t.Errorf("Expected summary failed tool calls 1, got %d", summary.FailedToolCalls)
	}
	if summary.IterationCount != 1 {
		t.Errorf("Expected summary iteration count 1, got %d", summary.IterationCount)
	}
	if summary.Uptime <= 0 {
		t.Errorf("Expected positive uptime, got %v", summary.Uptime)
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := NewStats()

	done := make(chan bool)

	// Goroutine 1: Add input tokens
	go func() {
		for i := 0; i < 1000; i++ {
			s.AddInputTokens(1)
		}
		done <- true
	}()

	// Goroutine 2: Add output tokens
	go func() {
		for i := 0; i < 1000; i++ {
			s.AddOutputTokens(1)
		}
		done <- true
	}()

	// Goroutine 3: Read tokens
	go func() {
		for i := 0; i < 1000; i++ {
			_ = s.GetInputTokens()
			_ = s.GetOutputTokens()
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	<-done
	<-done
	<-done

	if s.GetInputTokens() != 1000 {
		t.Errorf("Expected input tokens 1000, got %d", s.GetInputTokens())
	}
	if s.GetOutputTokens() != 1000 {
		t.Errorf("Expected output tokens 1000, got %d", s.GetOutputTokens())
	}
}
