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

// SystemPromptTemplate contains the default system prompt with tool definitions
const SystemPromptTemplate = `You are a helpful coding assistant. You have access to the following tools:

AVAILABLE TOOLS:
- bash: Execute a bash command
  Format: [tool:bash(command="command string")]
  Example: [tool:bash(command="ls -la")]
  
- read_file: Read the contents of a file
  Format: [tool:read_file(path="file path")]
  Example: [tool:read_file(path="/path/to/file.txt")]
  
- write_file: Write content to a file
  Format: [tool:write_file(path="file path", content="file content")]
  Example: [tool:write_file(path="/path/to/file.txt", content="Hello")]
  
- read_lines: Read a specific line range from a file
  Format: [tool:read_lines(path="file path", start=line_number, end=line_number)]
  Example: [tool:read_lines(path="/path/to/file.txt", start=1, end=10)]
  
- insert_lines: Insert lines at a specific line number
  Format: [tool:insert_lines(path="file path", line=line_number, lines="lines to insert")]
  Example: [tool:insert_lines(path="/path/to/file.txt", line=5, lines="new line")]
  
- replace_lines: Replace a line range with new lines
  Format: [tool:replace_lines(path="file path", start=line_number, end=line_number, lines="replacement lines")]
  Example: [tool:replace_lines(path="/path/to/file.txt", start=1, end=5, lines="new content")]

TOOL CALLING RULES:
- Use the exact format shown above for tool calls
- Tool calls must be enclosed in square brackets
- Tool name must match exactly (case-sensitive)
- Parameters must be properly quoted
- Multi-line content uses \\n for line breaks

Instructions:
- Analyze the user's request and determine if tools are needed
- Use tools when they can help complete the task
- Always explain your reasoning before calling tools
- Provide clear explanations of tool results
- Continue the conversation after tool execution`

// NewContext creates a new context with the default system prompt
func NewContext(contextSize int) *Context {
	return &Context{
		systemPrompt: SystemPromptTemplate,
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
