package context

import (
	"context"
	"coding-agent-harness/pkg/inference"
)

// Context holds the conversation context with compression support
type Context struct {
	systemPrompt   string
	messages       []inference.Message
	contextSize    int
	compressed     bool
	currentSize    int
}

// NewContext creates a new context with the given system prompt
func NewContext(systemPrompt string, contextSize int) *Context {
	return &Context{
		systemPrompt: systemPrompt,
		messages:     []inference.Message{},
		contextSize:  contextSize,
		compressed:   false,
		currentSize:  0,
	}
}

// AddUserMessage adds a user message to the context
func (c *Context) AddUserMessage(content string) {
	c.messages = append(c.messages, inference.Message{
		Role:    "user",
		Content: content,
	})
	c.currentSize++
}

// AddAssistantMessage adds an assistant message to the context
func (c *Context) AddAssistantMessage(content string) {
	c.messages = append(c.messages, inference.Message{
		Role:    "assistant",
		Content: content,
	})
	c.currentSize++
}

// GetMessages returns all messages including system prompt
func (c *Context) GetMessages() []inference.Message {
	messages := make([]inference.Message, len(c.messages)+1)
	messages[0] = inference.Message{
		Role:    "system",
		Content: c.systemPrompt,
	}
	copy(messages[1:], c.messages)
	return messages
}

// GetCurrentSize returns the current context size
func (c *Context) GetCurrentSize() int {
	return c.currentSize
}

// GetContextSize returns the configured context size limit
func (c *Context) GetContextSize() int {
	return c.contextSize
}

// NeedsCompression checks if the context needs compression
func (c *Context) NeedsCompression() bool {
	return c.currentSize > c.contextSize
}

// Compress attempts to compress the context by summarizing it
func (c *Context) Compress(client *inference.Client) error {
	if !c.NeedsCompression() {
		return nil
	}

	// Create a prompt for summarization
	summaryPrompt := "Please summarize the following conversation history while preserving key information. Keep the summary concise but complete:\n\n" + c.formatHistory()

	// Send summarization request
	ctx := context.Background()
	resp, err := client.Chat(ctx, c.getContextWithSummaryPrompt(summaryPrompt), c.contextSize/2)
	if err != nil {
		return err
	}

	// Update context with compressed version
	c.messages = []inference.Message{
		{
			Role:    "system",
			Content: c.systemPrompt + "\n\nContext Summary:\n" + resp.Choices[0].Message.Content,
		},
	}
	c.currentSize = 1
	c.compressed = true

	return nil
}

// getContextWithSummaryPrompt creates a context for summarization
func (c *Context) getContextWithSummaryPrompt(summaryPrompt string) []inference.Message {
	return []inference.Message{
		{
			Role:    "user",
			Content: summaryPrompt,
		},
	}
}

// formatHistory formats the conversation history for summarization
func (c *Context) formatHistory() string {
	var result string
	for _, msg := range c.messages {
		result += msg.Role + ": " + msg.Content + "\n\n"
	}
	return result
}

// Reset resets the context to initial state
func (c *Context) Reset() {
	c.messages = []inference.Message{}
	c.currentSize = 0
	c.compressed = false
}
