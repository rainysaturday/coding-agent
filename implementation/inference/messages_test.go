package inference

import (
	"testing"

	"github.com/coding-agent/harness/config"
)

func TestBuildMessages_WithSystemPrompt(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	systemPrompt := "You are a helpful assistant."
	messages := []*Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	result := client.buildMessages(messages, systemPrompt)

	if len(result) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(result))
	}

	if result[0].Role != "system" {
		t.Errorf("First message should be system, got '%s'", result[0].Role)
	}

	if result[0].Content != systemPrompt {
		t.Errorf("System prompt mismatch")
	}

	if result[1].Content != "Hello" {
		t.Errorf("Expected second message 'Hello', got '%s'", result[1].Content)
	}
}

func TestBuildMessages_NoSystemPrompt(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	messages := []*Message{
		{Role: "user", Content: "Hello"},
	}

	result := client.buildMessages(messages, "")

	if len(result) != 1 {
		t.Errorf("Expected 1 message, got %d", len(result))
	}

	if result[0].Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", result[0].Role)
	}
}

func TestBuildMessages_EmptyMessages(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	result := client.buildMessages(nil, "System prompt")

	if len(result) != 1 {
		t.Errorf("Expected 1 message (system only), got %d", len(result))
	}

	if result[0].Role != "system" {
		t.Errorf("Expected system role, got '%s'", result[0].Role)
	}
}

func TestInferenceClient_BuildMessages_MessageOrder(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	systemPrompt := "System"
	messages := []*Message{
		{Role: "user", Content: "First"},
		{Role: "assistant", Content: "Second"},
		{Role: "user", Content: "Third"},
	}

	result := client.buildMessages(messages, systemPrompt)

	if len(result) != 4 {
		t.Fatalf("Expected 4 messages, got %d", len(result))
	}

	// Verify order: system first, then user messages in order
	if result[0].Role != "system" {
		t.Errorf("Expected first message to be system, got '%s'", result[0].Role)
	}
	if result[1].Content != "First" {
		t.Errorf("Expected second message to be 'First', got '%s'", result[1].Content)
	}
	if result[2].Content != "Second" {
		t.Errorf("Expected third message to be 'Second', got '%s'", result[2].Content)
	}
	if result[3].Content != "Third" {
		t.Errorf("Expected fourth message to be 'Third', got '%s'", result[3].Content)
	}
}

func TestBuildMessages(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	messages := []*Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}

	result := ic.buildMessages(messages, "System prompt")

	if len(result) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(result))
	}
	if result[0].Role != "system" {
		t.Errorf("Expected first message role 'system', got %q", result[0].Role)
	}
	if result[0].Content != "System prompt" {
		t.Errorf("Expected first message content 'System prompt', got %q", result[0].Content)
	}
	if result[1].Content != "Hello" {
		t.Errorf("Expected second message content 'Hello', got %q", result[1].Content)
	}
	if result[2].Content != "Hi there" {
		t.Errorf("Expected third message content 'Hi there', got %q", result[2].Content)
	}
}

func TestBuildMessages_WithSystemMessage(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	messages := []*Message{
		{Role: "user", Content: "Hello"},
	}

	result := ic.buildMessages(messages, "")

	if len(result) != 1 {
		t.Fatalf("Expected 1 message (no system prompt), got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Errorf("Expected first message role 'user', got %q", result[0].Role)
	}
}

func TestBuildMessages_Empty(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	result := ic.buildMessages(nil, "")
	if len(result) != 0 {
		t.Errorf("Expected 0 messages, got %d", len(result))
	}
}

func TestBuildMessages_EmptySystemPrompt(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	messages := []*Message{
		{Role: "user", Content: "Hello"},
	}

	result := ic.buildMessages(messages, "")
	if len(result) != 1 {
		t.Errorf("Expected 1 message when system prompt is empty, got %d", len(result))
	}
}

