package context

import (
	"testing"
)

func TestNewContext(t *testing.T) {
	systemPrompt := "You are a helpful assistant."
	ctx := NewContext(systemPrompt, 1000)

	if ctx.GetSystemPrompt() != systemPrompt {
		t.Errorf("Expected system prompt '%s', got '%s'", systemPrompt, ctx.GetSystemPrompt())
	}
	if ctx.GetMessageCount() != 1 {
		t.Errorf("Expected 1 message (system), got %d", ctx.GetMessageCount())
	}
}

func TestAddUserMessage(t *testing.T) {
	ctx := NewContext("system", 1000)
	ctx.AddUserMessage("Hello")

	messages := ctx.GetMessages()
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}
	if messages[1].Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", messages[1].Role)
	}
	if messages[1].Content != "Hello" {
		t.Errorf("Expected content 'Hello', got '%s'", messages[1].Content)
	}
}

func TestAddAssistantMessage(t *testing.T) {
	ctx := NewContext("system", 1000)
	ctx.AddAssistantMessage("Hi there!")

	messages := ctx.GetMessages()
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}
	if messages[1].Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", messages[1].Role)
	}
	if messages[1].Content != "Hi there!" {
		t.Errorf("Expected content 'Hi there!', got '%s'", messages[1].Content)
	}
}

func TestAddToolResult_Success(t *testing.T) {
	ctx := NewContext("system", 1000)
	ctx.AddToolResult("bash", true, "output", "")

	messages := ctx.GetMessages()
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}
	if messages[1].Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", messages[1].Role)
	}
	expected := "Tool 'bash' executed successfully:\noutput"
	if messages[1].Content != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, messages[1].Content)
	}
}

func TestAddToolResult_Failure(t *testing.T) {
	ctx := NewContext("system", 1000)
	ctx.AddToolResult("read_file", false, "", "file not found")

	messages := ctx.GetMessages()
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}
	expected := "Tool 'read_file' failed: file not found"
	if messages[1].Content != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, messages[1].Content)
	}
}

func TestGetRecentMessages(t *testing.T) {
	ctx := NewContext("system", 1000)
	ctx.AddUserMessage("msg1")
	ctx.AddAssistantMessage("msg2")
	ctx.AddUserMessage("msg3")
	ctx.AddAssistantMessage("msg4")

	recent := ctx.GetRecentMessages(2)
	if len(recent) != 2 {
		t.Errorf("Expected 2 recent messages, got %d", len(recent))
	}
	if recent[0].Content != "msg3" {
		t.Errorf("Expected 'msg3', got '%s'", recent[0].Content)
	}
	if recent[1].Content != "msg4" {
		t.Errorf("Expected 'msg4', got '%s'", recent[1].Content)
	}
}

func TestGetRecentMessages_All(t *testing.T) {
	ctx := NewContext("system", 1000)
	ctx.AddUserMessage("msg1")
	ctx.AddAssistantMessage("msg2")

	recent := ctx.GetRecentMessages(10)
	if len(recent) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(recent))
	}
}

func TestClear(t *testing.T) {
	ctx := NewContext("my system prompt", 1000)
	ctx.AddUserMessage("msg1")
	ctx.AddAssistantMessage("msg2")

	ctx.Clear()

	messages := ctx.GetMessages()
	if len(messages) != 1 {
		t.Errorf("Expected 1 message after clear, got %d", len(messages))
	}
	if messages[0].Content != "my system prompt" {
		t.Errorf("Expected system prompt preserved, got '%s'", messages[0].Content)
	}
}

func TestCompress(t *testing.T) {
	ctx := NewContext("system prompt", 1000)
	ctx.AddUserMessage("msg1")
	ctx.AddAssistantMessage("msg2")

	ctx.Compress("summary of conversation")

	messages := ctx.GetMessages()
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages after compression, got %d", len(messages))
	}
	if messages[0].Role != "system" {
		t.Errorf("Expected system role preserved, got '%s'", messages[0].Role)
	}
	if messages[1].Role != "user" {
		t.Errorf("Expected user role for summary, got '%s'", messages[1].Role)
	}
	if messages[1].Content != "Conversation summary: summary of conversation" {
		t.Errorf("Expected summary content, got '%s'", messages[1].Content)
	}
}

func TestEstimateTokenCount(t *testing.T) {
	ctx := NewContext("system", 1000)
	ctx.AddUserMessage("Hello") // ~1 token per 4 chars + overhead

	count := ctx.EstimateTokenCount()
	if count <= 0 {
		t.Errorf("Expected positive token count, got %d", count)
	}
}

func TestIsOverLimit(t *testing.T) {
	// Small limit
	ctx := NewContext("system", 10)
	ctx.AddUserMessage("This is a longer message that should exceed the limit")

	if !ctx.IsOverLimit() {
		t.Error("Expected context to be over limit")
	}

	// Large limit
	ctx2 := NewContext("system", 1000000)
	ctx2.AddUserMessage("Hello")

	if ctx2.IsOverLimit() {
		t.Error("Expected context to be under limit")
	}
}

func TestFormatForAPI(t *testing.T) {
	ctx := NewContext("system", 1000)
	ctx.AddUserMessage("Hello")
	ctx.AddAssistantMessage("Hi!")

	messages := ctx.FormatForAPI()
	if len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(messages))
	}
}

func TestContextString(t *testing.T) {
	ctx := NewContext("system", 1000)
	ctx.AddUserMessage("Hello")

	str := ctx.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}
	if !contains(str, "Context") {
		t.Error("Expected string to contain 'Context'")
	}
}

func TestTruncate(t *testing.T) {
	short := "hello"
	result := truncate(short, 100)
	if result != short {
		t.Errorf("Expected '%s', got '%s'", short, result)
	}

	long := "this is a very long string that will be truncated"
	result = truncate(long, 10)
	expected := "this is a ..." // "this is a " (10 chars) + "..." (3 chars)
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
