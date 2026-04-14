package types

import "time"

type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	Name             string     `json:"name,omitempty"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolDefinition struct {
	Type     string                 `json:"type"`
	Function map[string]interface{} `json:"function"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatResponse struct {
	Message Message
	Usage   Usage
}

type StreamDelta struct {
	Content          string
	ReasoningContent string
	ToolCalls        []ToolCall
	Done             bool
	Usage            Usage
}

type ToolExecution struct {
	ToolName string
	Success  bool
	Duration time.Duration
}

type Stats struct {
	StartTime       time.Time
	InputTokens     int
	OutputTokens    int
	ToolCalls       int
	FailedToolCalls int
}

func NewStats() *Stats {
	return &Stats{StartTime: time.Now()}
}

func (s *Stats) RecordUsage(u Usage) {
	s.InputTokens += u.PromptTokens
	s.OutputTokens += u.CompletionTokens
}

func (s *Stats) RecordToolCall(success bool) {
	s.ToolCalls++
	if !success {
		s.FailedToolCalls++
	}
}

func (s *Stats) TokensPerSecond() float64 {
	d := time.Since(s.StartTime).Seconds()
	if d <= 0 {
		return 0
	}
	return float64(s.InputTokens+s.OutputTokens) / d
}
