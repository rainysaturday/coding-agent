package stats

import (
	"fmt"
	"strconv"
	"sync"
	"time"
)

// Stats holds runtime statistics for the coding agent
type Stats struct {
	mu              sync.RWMutex
	totalInputTokens  int
	totalOutputTokens int
	startTime         time.Time
	lastTokenTime     time.Time
	toolCalls         int
	failedToolCalls   int
}

// NewStats creates a new Stats instance
func NewStats() *Stats {
	return &Stats{
		startTime:       time.Now(),
		lastTokenTime:   time.Now(),
	}
}

// RecordInputToken records an input token
func (s *Stats) RecordInputToken() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalInputTokens++
}

// RecordOutputToken records an output token
func (s *Stats) RecordOutputToken() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalOutputTokens++
	s.lastTokenTime = time.Now()
}

// RecordToolCall increments the tool call counter
func (s *Stats) RecordToolCall() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toolCalls++
}

// RecordFailedToolCall increments the failed tool call counter
func (s *Stats) RecordFailedToolCall() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failedToolCalls++
}

// GetTokensPerSecond calculates tokens per second
func (s *Stats) GetTokensPerSecond() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	totalTokens := float64(s.totalInputTokens + s.totalOutputTokens)
	elapsed := time.Since(s.lastTokenTime).Seconds()
	
	if elapsed == 0 {
		return 0
	}
	
	return totalTokens / elapsed
}

// GetTotalInputTokens returns total input tokens
func (s *Stats) GetTotalInputTokens() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalInputTokens
}

// GetTotalOutputTokens returns total output tokens
func (s *Stats) GetTotalOutputTokens() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalOutputTokens
}

// GetToolCalls returns total tool calls
func (s *Stats) GetToolCalls() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.toolCalls
}

// GetFailedToolCalls returns failed tool calls
func (s *Stats) GetFailedToolCalls() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.failedToolCalls
}

// GetElapsedTime returns time since start
func (s *Stats) GetElapsedTime() time.Duration {
	return time.Since(s.startTime)
}

// String returns a formatted string of all statistics
func (s *Stats) String() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	tps := 0.0
	elapsed := time.Since(s.lastTokenTime).Seconds()
	if elapsed > 0 {
		tps = float64(s.totalInputTokens+s.totalOutputTokens) / elapsed
	}
	
	return "=== Runtime Statistics ===\n" +
		"Total Input Tokens:  " + formatNumber(s.totalInputTokens) + "\n" +
		"Total Output Tokens: " + formatNumber(s.totalOutputTokens) + "\n" +
		"Tokens/Second:       " + formatNumber(float64(tps)) + "\n" +
		"Tool Calls:          " + formatNumber(s.toolCalls) + "\n" +
		"Failed Tool Calls:   " + formatNumber(s.failedToolCalls) + "\n" +
		"Elapsed Time:        " + s.startTime.Format("0h0m0s") + "\n" +
		"========================"
}

// Reset clears all statistics
func (s *Stats) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalInputTokens = 0
	s.totalOutputTokens = 0
	s.toolCalls = 0
	s.failedToolCalls = 0
	s.startTime = time.Now()
	s.lastTokenTime = time.Now()
}

// formatNumber formats a number for display
func formatNumber(n interface{}) string {
	switch v := n.(type) {
	case int:
		return strconv.Itoa(v)
	case float64:
		return strconv.FormatFloat(v, 'f', 2, 64)
	default:
		return fmt.Sprintf("%v", n)
	}
}
