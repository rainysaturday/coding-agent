// Package webui implements the web user interface for the coding agent.
//
// It is a dependency-free (stdlib-only) web front-end over the existing agent
// package. It provides a REST + Server-Sent Events (SSE) API and serves an
// embedded single-page application compiled into the binary via go:embed.
package webui

import (
	"github.com/coding-agent/harness/agent"
	"github.com/coding-agent/harness/inference"
)

// Content type constants mirroring inference.StreamingContentType, exposed to the
// frontend as integers over the wire.
const (
	contentNormal      = inference.StreamingContentTypeNormal
	contentReasoning   = inference.StreamingContentTypeReasoning
	contentGoal        = inference.StreamingContentTypeGoal
	contentCompression = inference.StreamingContentTypeCompression
)

// chunkEvent is sent for each streamed token/text fragment.
type chunkEvent struct {
	Text        string `json:"text"`
	ContentType int    `json:"contentType"`
	IsToolCall  bool   `json:"isToolCall"`
}

// statsEvent carries runtime statistics to the frontend.
type statsEvent struct {
	InputTokens      int     `json:"inputTokens"`
	OutputTokens     int     `json:"outputTokens"`
	TokensPerSecond  float64 `json:"tokensPerSecond"`
	ToolCalls        int     `json:"toolCalls"`
	FailedToolCalls  int     `json:"failedToolCalls"`
	Iterations       int     `json:"iterations"`
	CompressionCount int     `json:"compressionCount"`
	StartTime        int64   `json:"startTime"`
}

// toolCallInfo describes a tool call for the result event.
type toolCallInfo struct {
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters"`
	Arguments  string                 `json:"arguments,omitempty"`
}

// stepEvent describes one execution step for the result event.
type stepEvent struct {
	Action     string        `json:"action"`
	ToolCall   *toolCallInfo `json:"toolCall,omitempty"`
	ToolResult *string       `json:"toolResult,omitempty"`
}

// resultEvent is sent once an agent run completes.
type resultEvent struct {
	FinalOutput string      `json:"finalOutput"`
	Reasoning   string      `json:"reasoning"`
	TokenUsage  int         `json:"tokenUsage"`
	Steps       []stepEvent `json:"steps"`
	Stats       statsEvent  `json:"stats"`
}

// stateEvent describes the current session state (context size, read-only, goal).
type stateEvent struct {
	ContextSize  int    `json:"contextSize"`
	MaxContext   int    `json:"maxContextSize"`
	ReadOnly     bool   `json:"readOnly"`
	Goal         string `json:"goal"`
	GoalActive   bool   `json:"goalActive"`
	Running      bool   `json:"running"`
	Theme        string `json:"theme"`
	WebAddr      string `json:"webAddr"`
	WebPort      int    `json:"webPort"`
	Model        string `json:"model"`
	APIEndpoint  string `json:"apiEndpoint"`
	Streaming    bool   `json:"streaming"`
	HasContext   bool   `json:"hasContext"`
	HistoryCount int    `json:"historyCount"`
}

// historyToolCall describes one tool call within a history message.
type historyToolCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// historyMessage is a single conversation message returned via /api/history so
// a reconnecting client can render the existing conversation.
type historyMessage struct {
	Role      string            `json:"role"`
	Content   string            `json:"content,omitempty"`
	Reasoning string            `json:"reasoning,omitempty"`
	ToolCalls []historyToolCall `json:"toolCalls,omitempty"`
}

// historyResponse is the payload returned by /api/history.
type historyResponse struct {
	Messages []historyMessage `json:"messages"`
}

// errorEvent is sent when an agent run fails.
type errorEvent struct {
	Message string `json:"message"`
}

// doneEvent marks the end of an agent run.
type doneEvent struct {
	OK bool `json:"ok"`
}

// chatRequest is the body of POST /api/chat.
type chatRequest struct {
	Session string `json:"session"`
	Prompt  string `json:"prompt"`
}

// commandRequest is the body of POST /api/command.
type commandRequest struct {
	Session string `json:"session"`
	Command string `json:"command"`
}

// commandResponse is the body returned by POST /api/command.
type commandResponse struct {
	OK     bool       `json:"ok"`
	Output string     `json:"output"`
	State  stateEvent `json:"state"`
}

// chatResponse is the body returned by POST /api/chat.
type chatResponse struct {
	OK      bool   `json:"ok"`
	Session string `json:"session"`
}

// cancelRequest is the body of POST /api/cancel.
type cancelRequest struct {
	Session string `json:"session"`
}

// toStats converts agent.Stats to a statsEvent.
func toStats(s *agent.Stats) statsEvent {
	if s == nil {
		return statsEvent{}
	}
	var start int64
	if !s.StartTime.IsZero() {
		start = s.StartTime.Unix()
	}
	return statsEvent{
		InputTokens:      s.InputTokens,
		OutputTokens:     s.OutputTokens,
		TokensPerSecond:  s.TokensPerSecond,
		ToolCalls:        s.ToolCalls,
		FailedToolCalls:  s.FailedToolCalls,
		Iterations:       s.Iterations,
		CompressionCount: s.CompressionCount,
		StartTime:        start,
	}
}
