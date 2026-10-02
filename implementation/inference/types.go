package inference

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Reasoning field names used by the different backends.
const (
	ReasoningFieldOpenAI   = "reasoning"
	ReasoningFieldLlamaCPP = "reasoning_content"
)

// ContentPart is a multi-modal message part (text or image).
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL carries a public URL or a base64 data URI.
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// ToolCall is a tool invocation requested by the model.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall carries the tool name and its JSON encoded arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolCallDelta is an incremental tool call fragment from a stream.
type ToolCallDelta struct {
	Index    int           `json:"index"`
	ID       string        `json:"id,omitempty"`
	Type     string        `json:"type,omitempty"`
	Function *FunctionCall `json:"function,omitempty"`
}

// Message is a chat message in the OpenAI format.
type Message struct {
	Role           string
	Content        interface{} // string, []ContentPart or nil
	Reasoning      string
	ReasoningField string // "reasoning" or "reasoning_content"
	ToolCalls      []ToolCall
	ToolCallID     string
	Name           string
}

// NewTextMessage builds a plain text message.
func NewTextMessage(role, content string) *Message {
	return &Message{Role: role, Content: content}
}

// Text returns the plain text content of the message.
func (m *Message) Text() string {
	switch v := m.Content.(type) {
	case nil:
		return ""
	case string:
		return v
	case []ContentPart:
		var b strings.Builder
		for _, p := range v {
			if p.Type == "text" {
				b.WriteString(p.Text)
			}
		}
		return b.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// SetText replaces the content with plain text.
func (m *Message) SetText(s string) { m.Content = s }

// HasToolCalls reports whether the message requests tool calls.
func (m *Message) HasToolCalls() bool { return len(m.ToolCalls) > 0 }

// MarshalJSON implements custom marshalling so that content may be a string,
// an array of content parts, or null (assistant messages with tool calls).
func (m Message) MarshalJSON() ([]byte, error) {
	obj := map[string]interface{}{"role": m.Role}
	if m.Content == nil {
		obj["content"] = nil
	} else {
		obj["content"] = m.Content
	}
	if m.Reasoning != "" {
		field := m.ReasoningField
		if field == "" {
			field = ReasoningFieldLlamaCPP
		}
		obj[field] = m.Reasoning
	}
	if len(m.ToolCalls) > 0 {
		calls := make([]ToolCall, len(m.ToolCalls))
		for i, tc := range m.ToolCalls {
			calls[i] = tc
			if calls[i].Type == "" {
				calls[i].Type = "function"
			}
		}
		obj["tool_calls"] = calls
	}
	if m.ToolCallID != "" {
		obj["tool_call_id"] = m.ToolCallID
	}
	if m.Name != "" {
		obj["name"] = m.Name
	}
	return json.Marshal(obj)
}

// UnmarshalJSON accepts both string and multi-modal content, and both
// reasoning field spellings.
func (m *Message) UnmarshalJSON(data []byte) error {
	var raw struct {
		Role             string          `json:"role"`
		Content          json.RawMessage `json:"content"`
		Reasoning        string          `json:"reasoning"`
		ReasoningContent string          `json:"reasoning_content"`
		ToolCalls        []ToolCall      `json:"tool_calls"`
		ToolCallID       string          `json:"tool_call_id"`
		Name             string          `json:"name"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	m.Role = raw.Role
	m.ToolCalls = raw.ToolCalls
	m.ToolCallID = raw.ToolCallID
	m.Name = raw.Name
	if len(raw.Content) > 0 && string(raw.Content) != "null" {
		var s string
		if err := json.Unmarshal(raw.Content, &s); err == nil {
			m.Content = s
		} else {
			var parts []ContentPart
			if err := json.Unmarshal(raw.Content, &parts); err == nil {
				m.Content = parts
			} else {
				var generic interface{}
				if err := json.Unmarshal(raw.Content, &generic); err != nil {
					return err
				}
				m.Content = generic
			}
		}
	}
	if raw.Reasoning != "" {
		m.Reasoning = raw.Reasoning
		m.ReasoningField = ReasoningFieldOpenAI
	}
	if raw.ReasoningContent != "" {
		m.Reasoning = raw.ReasoningContent
		m.ReasoningField = ReasoningFieldLlamaCPP
	}
	return nil
}

// ToolDefinition is an OpenAI function tool definition.
type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a callable tool.
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// Usage carries token accounting for a completion.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	CachedTokens     int `json:"cached_tokens,omitempty"`
}

// Timings is the llama.cpp specific timings block.
type Timings struct {
	CacheN              int     `json:"cache_n"`
	PromptN             int     `json:"prompt_n"`
	PromptMS            float64 `json:"prompt_ms"`
	PromptPerTokenMS    float64 `json:"prompt_per_token_ms"`
	PromptPerSecond     float64 `json:"prompt_per_second"`
	PredictedN          int     `json:"predicted_n"`
	PredictedMS         float64 `json:"predicted_ms"`
	PredictedPerTokenMS float64 `json:"predicted_per_token_ms"`
	PredictedPerSecond  float64 `json:"predicted_per_second"`
}

// Request is the chat completion request body.
type Request struct {
	Model       string           `json:"model"`
	Messages    []*Message       `json:"messages"`
	Stream      bool             `json:"stream"`
	Temperature *float64         `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	TopP        *float64         `json:"top_p,omitempty"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	ToolChoice  string           `json:"tool_choice,omitempty"`
}

// Response is the normalised completion result.
type Response struct {
	Content      string
	Reasoning    string
	ToolCalls    []ToolCall
	FinishReason string
	Usage        Usage
	Model        string
	ID           string
}

// Choice is a completion choice in a non-streaming response.
type Choice struct {
	Index        int      `json:"index"`
	FinishReason string   `json:"finish_reason"`
	Message      *Message `json:"message"`
	Delta        *Delta   `json:"delta,omitempty"`
}

// Delta is the incremental content of a streamed chunk.
type Delta struct {
	Role             string          `json:"role,omitempty"`
	Content          *string         `json:"content"`
	Reasoning        *string         `json:"reasoning"`
	ReasoningContent *string         `json:"reasoning_content"`
	ToolCalls        []ToolCallDelta `json:"tool_calls,omitempty"`
}

// Chunk is a streaming response chunk.
type Chunk struct {
	ID      string    `json:"id"`
	Object  string    `json:"object"`
	Created int64     `json:"created"`
	Model   string    `json:"model"`
	Choices []Choice  `json:"choices"`
	Usage   *Usage    `json:"usage,omitempty"`
	Timings *Timings  `json:"timings,omitempty"`
	Error   *APIError `json:"error,omitempty"`
}

// APIError describes an inference backend error.
type APIError struct {
	StatusCode int    `json:"-"`
	Message    string `json:"message"`
	Hint       string `json:"hint,omitempty"`
}

func (e *APIError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s\nhint: %s", e.Message, e.Hint)
	}
	return e.Message
}

// StreamCallbacks receives incremental output while a completion streams.
type StreamCallbacks struct {
	Content       func(string)
	Reasoning     func(string)
	ToolCallDelta func(index int, delta ToolCallDelta)
}
