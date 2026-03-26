package stats

import (
	"sync"
	"time"
)

// Stats holds runtime statistics for the coding agent
type Stats struct {
	mu                sync.RWMutex
	inputTokens       int64
	outputTokens      int64
	startTime         time.Time
	lastUpdateTime    time.Time
	totalToolCalls    int64
	failedToolCalls   int64
	iterationCount    int64
}

// NewStats creates a new Stats instance
func NewStats() *Stats {
	now := time.Now()
	return &Stats{
		startTime:      now,
		lastUpdateTime: now,
	}
}

// AddInputTokens adds to the input token count
func (s *Stats) AddInputTokens(count int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inputTokens += count
	s.lastUpdateTime = time.Now()
}

// AddOutputTokens adds to the output token count
func (s *Stats) AddOutputTokens(count int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outputTokens += count
	s.lastUpdateTime = time.Now()
}

// SetInputTokens sets the input token count
func (s *Stats) SetInputTokens(count int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inputTokens = count
}

// SetOutputTokens sets the output token count
func (s *Stats) SetOutputTokens(count int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outputTokens = count
}

// GetInputTokens returns the input token count
func (s *Stats) GetInputTokens() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.inputTokens
}

// GetOutputTokens returns the output token count
func (s *Stats) GetOutputTokens() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.outputTokens
}

// GetTotalTokens returns the total token count
func (s *Stats) GetTotalTokens() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.inputTokens + s.outputTokens
}

// GetTokensPerSecond returns the tokens per second rate
func (s *Stats) GetTokensPerSecond() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	elapsed := time.Since(s.startTime).Seconds()
	if elapsed == 0 {
		return 0
	}
	return float64(s.inputTokens+s.outputTokens) / elapsed
}

// AddToolCall increments the tool call count
func (s *Stats) AddToolCall() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalToolCalls++
}

// AddFailedToolCall increments the failed tool call count
func (s *Stats) AddFailedToolCall() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failedToolCalls++
}

// GetTotalToolCalls returns the total tool call count
func (s *Stats) GetTotalToolCalls() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalToolCalls
}

// GetFailedToolCalls returns the failed tool call count
func (s *Stats) GetFailedToolCalls() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.failedToolCalls
}

// IncrementIteration increments the iteration count
func (s *Stats) IncrementIteration() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.iterationCount++
}

// GetIterationCount returns the iteration count
func (s *Stats) GetIterationCount() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.iterationCount
}

// Reset resets all statistics
func (s *Stats) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inputTokens = 0
	s.outputTokens = 0
	s.totalToolCalls = 0
	s.failedToolCalls = 0
	s.iterationCount = 0
	s.startTime = time.Now()
	s.lastUpdateTime = time.Now()
}

// Summary returns a summary of all statistics
type Summary struct {
	InputTokens     int64
	OutputTokens    int64
	TokensPerSecond float64
	TotalToolCalls  int64
	FailedToolCalls int64
	IterationCount  int64
	Uptime          time.Duration
}

// GetSummary returns a summary of all statistics
func (s *Stats) GetSummary() Summary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	elapsed := time.Since(s.startTime)
	tokensPerSecond := float64(s.inputTokens+s.outputTokens)
	if elapsed.Seconds() > 0 {
		tokensPerSecond = tokensPerSecond / elapsed.Seconds()
	}
	return Summary{
		InputTokens:     s.inputTokens,
		OutputTokens:    s.outputTokens,
		TokensPerSecond: tokensPerSecond,
		TotalToolCalls:  s.totalToolCalls,
		FailedToolCalls: s.failedToolCalls,
		IterationCount:  s.iterationCount,
		Uptime:          elapsed,
	}
}
