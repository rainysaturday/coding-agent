// Package agent implements the agentic loop: context management, inference
// calls, tool execution, compression, goal mode and session dumps.
package agent

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Stats tracks runtime statistics for a session.
type Stats struct {
	mu               sync.Mutex
	StartTime        time.Time     `json:"start_time"`
	InputTokens      int           `json:"input_tokens"`
	OutputTokens     int           `json:"output_tokens"`
	CachedTokens     int           `json:"cached_tokens"`
	ToolCalls        int           `json:"tool_calls"`
	FailedToolCalls  int           `json:"failed_tool_calls"`
	Iterations       int           `json:"iterations"`
	CompressionCount int           `json:"compression_count"`
	InferenceTime    time.Duration `json:"inference_time"`
	ToolTime         time.Duration `json:"tool_time"`
	FirstTokenTime   time.Duration `json:"first_token_time"`
}

// NewStats returns statistics initialised to now.
func NewStats() *Stats { return &Stats{StartTime: time.Now()} }

// Add merges usage information from a completion.
func (s *Stats) AddUsage(prompt, completion, cached int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.InputTokens += prompt
	s.OutputTokens += completion
	s.CachedTokens += cached
}

// AddToolCall records a tool invocation.
func (s *Stats) AddToolCall(failed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ToolCalls++
	if failed {
		s.FailedToolCalls++
	}
}

// AddIteration records one agentic iteration.
func (s *Stats) AddIteration() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Iterations++
}

// AddCompression records a compression event.
func (s *Stats) AddCompression() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CompressionCount++
}

// AddInferenceTime accumulates inference latency.
func (s *Stats) AddInferenceTime(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.InferenceTime += d
}

// AddToolTime accumulates tool execution latency.
func (s *Stats) AddToolTime(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ToolTime += d
}

// SetFirstTokenTime records the time to the first streamed token.
func (s *Stats) SetFirstTokenTime(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FirstTokenTime == 0 {
		s.FirstTokenTime = d
	}
}

// StatsSnapshot is a lock-free copy of Stats, safe to copy and pass around.
type StatsSnapshot struct {
	StartTime        time.Time     `json:"start_time"`
	InputTokens      int           `json:"input_tokens"`
	OutputTokens     int           `json:"output_tokens"`
	CachedTokens     int           `json:"cached_tokens"`
	ToolCalls        int           `json:"tool_calls"`
	FailedToolCalls  int           `json:"failed_tool_calls"`
	Iterations       int           `json:"iterations"`
	CompressionCount int           `json:"compression_count"`
	InferenceTime    time.Duration `json:"inference_time"`
	ToolTime         time.Duration `json:"tool_time"`
	FirstTokenTime   time.Duration `json:"first_token_time"`
}

// Snapshot returns a copy of the statistics.
func (s *Stats) Snapshot() StatsSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return StatsSnapshot{
		StartTime:        s.StartTime,
		InputTokens:      s.InputTokens,
		OutputTokens:     s.OutputTokens,
		CachedTokens:     s.CachedTokens,
		ToolCalls:        s.ToolCalls,
		FailedToolCalls:  s.FailedToolCalls,
		Iterations:       s.Iterations,
		CompressionCount: s.CompressionCount,
		InferenceTime:    s.InferenceTime,
		ToolTime:         s.ToolTime,
		FirstTokenTime:   s.FirstTokenTime,
	}
}

// Elapsed returns the time since the session started.
func (s *Stats) Elapsed() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Since(s.StartTime)
}

// TokensPerSecond returns the generation rate.
func (s *Stats) TokensPerSecond() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.InferenceTime <= 0 {
		return 0
	}
	return float64(s.OutputTokens) / s.InferenceTime.Seconds()
}

// FormatDuration renders d as raw seconds plus an h/m/s breakdown.
func FormatDuration(d time.Duration) string {
	secs := int(d.Seconds())
	h := secs / 3600
	m := (secs % 3600) / 60
	s := secs % 60
	if h > 0 {
		return fmt.Sprintf("%ds (%dh %dm %ds)", secs, h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%ds (%dm %ds)", secs, m, s)
	}
	return fmt.Sprintf("%ds", secs)
}

// Summary renders the statistics block shown by /stats and at exit.
func (s *Stats) Summary() string {
	cp := s.Snapshot()
	var b strings.Builder
	b.WriteString("Runtime Statistics\n")
	b.WriteString(strings.Repeat("-", 60) + "\n")
	b.WriteString(fmt.Sprintf("Elapsed time:        %s\n", FormatDuration(time.Since(cp.StartTime))))
	b.WriteString(fmt.Sprintf("Input tokens:        %d\n", cp.InputTokens))
	b.WriteString(fmt.Sprintf("Output tokens:       %d\n", cp.OutputTokens))
	if cp.CachedTokens > 0 {
		b.WriteString(fmt.Sprintf("Cached tokens:       %d\n", cp.CachedTokens))
	}
	b.WriteString(fmt.Sprintf("Tokens/second:       %.2f\n", tokensPerSecond(cp.OutputTokens, cp.InferenceTime)))
	b.WriteString(fmt.Sprintf("Iterations:          %d\n", cp.Iterations))
	b.WriteString(fmt.Sprintf("Tool calls:          %d\n", cp.ToolCalls))
	b.WriteString(fmt.Sprintf("Failed tool calls:   %d\n", cp.FailedToolCalls))
	b.WriteString(fmt.Sprintf("Compressions:        %d\n", cp.CompressionCount))
	b.WriteString(fmt.Sprintf("Inference time:      %s\n", FormatDuration(cp.InferenceTime)))
	b.WriteString(fmt.Sprintf("Tool time:           %s\n", FormatDuration(cp.ToolTime)))
	return strings.TrimRight(b.String(), "\n")
}

func tokensPerSecond(outputTokens int, inferenceTime time.Duration) float64 {
	if inferenceTime <= 0 {
		return 0
	}
	return float64(outputTokens) / inferenceTime.Seconds()
}
