package context

import (
	"fmt"
	"strings"
)

// Message represents a single message in the conversation
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Context represents the conversation context
type Context struct {
	messages     []Message
	systemPrompt string
	maxSize      int
}

// NewContext creates a new context with the given system prompt and max size
func NewContext(systemPrompt string, maxSize int) *Context {
	ctx := &Context{
		messages:     make([]Message, 0),
		systemPrompt: systemPrompt,
		maxSize:      maxSize,
	}
	// Add system prompt as the first message
	ctx.messages = append(ctx.messages, Message{
		Role:    "system",
		Content: systemPrompt,
	})
	return ctx
}

// AddUserMessage adds a user message to the context
func (c *Context) AddUserMessage(content string) {
	c.messages = append(c.messages, Message{
		Role:    "user",
		Content: content,
	})
}

// AddAssistantMessage adds an assistant message to the context
func (c *Context) AddAssistantMessage(content string) {
	c.messages = append(c.messages, Message{
		Role:    "assistant",
		Content: content,
	})
}

// AddToolResult adds a tool result as a user message
func (c *Context) AddToolResult(toolName string, success bool, output string, err string) {
	var content string
	if success {
		content = fmt.Sprintf("Tool '%s' executed successfully:\n%s", toolName, output)
	} else {
		content = fmt.Sprintf("Tool '%s' failed: %s", toolName, err)
	}
	c.messages = append(c.messages, Message{
		Role:    "user",
		Content: content,
	})
}

// GetMessages returns all messages in the context
func (c *Context) GetMessages() []Message {
	return c.messages
}

// GetSystemPrompt returns the system prompt
func (c *Context) GetSystemPrompt() string {
	return c.systemPrompt
}

// GetMessageCount returns the number of messages
func (c *Context) GetMessageCount() int {
	return len(c.messages)
}

// EstimateTokenCount estimates the token count for the context
// This is a simple approximation; real token counting should use the tokenizer
func (c *Context) EstimateTokenCount() int {
	count := 0
	for _, msg := range c.messages {
		// Rough estimate: 1 token per 4 characters
		count += len(msg.Content) / 4
		// Add overhead for role and formatting
		count += 10
	}
	return count
}

// IsOverLimit checks if the context exceeds the max size
func (c *Context) IsOverLimit() bool {
	return c.EstimateTokenCount() > c.maxSize
}

// Clear clears all messages except the system prompt
func (c *Context) Clear() {
	c.messages = []Message{
		{
			Role:    "system",
			Content: c.systemPrompt,
		},
	}
}

// GetRecentMessages returns the most recent messages
func (c *Context) GetRecentMessages(count int) []Message {
	if count >= len(c.messages) {
		return c.messages
	}
	return c.messages[len(c.messages)-count:]
}

// FormatForAPI formats the context for the API request
func (c *Context) FormatForAPI() []Message {
	return c.messages
}

// Compress creates a summary of the context while preserving the system prompt
// This is used when the context exceeds the limit
func (c *Context) Compress(summary string) {
	// Keep only system prompt and summary
	c.messages = []Message{
		{
			Role:    "system",
			Content: c.systemPrompt,
		},
		{
			Role:    "user",
			Content: "Conversation summary: " + summary,
		},
	}
}

// String returns a string representation of the context
func (c *Context) String() string {
	var sb strings.Builder
	sb.WriteString("Context (")
	sb.WriteString(fmt.Sprintf("%d messages", len(c.messages)))
	sb.WriteString(", ")
	sb.WriteString(fmt.Sprintf("est. %d tokens", c.EstimateTokenCount()))
	sb.WriteString("):\n")

	for _, msg := range c.messages {
		sb.WriteString(fmt.Sprintf("[%s] %s\n", msg.Role, truncate(msg.Content, 100)))
	}
	return sb.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
