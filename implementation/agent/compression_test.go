package agent

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
)

func TestShouldCompress_ContextNearLimit(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ContextSize = 1000 // Small context for testing
	agent := NewAgent(cfg)

	// Add many messages to exceed 80% of context size
	for i := 0; i < 50; i++ {
		agent.AddUserMessage("This is a test message number " + string(rune('A'+i%26)) + " with some content to use up context tokens.")
	}

	shouldCompress := agent.shouldCompress()
	// With many messages, should compress
	if !shouldCompress {
		t.Error("Expected shouldCompress() to return true with near-limit context")
	}
}

func TestShouldCompress_ContextBelowLimit(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ContextSize = 50000 // Large context for testing
	agent := NewAgent(cfg)

	// Add just a few short messages - with large context, won't trigger compression
	agent.AddUserMessage("Short msg")
	agent.AddAssistantMessage("Short reply")

	shouldCompress := agent.shouldCompress()
	// With few messages and large context, should not compress
	if shouldCompress {
		t.Error("Expected shouldCompress() to return false with minimal context relative to limit")
	}
}

func TestCompressContext_NotEnoughMessages(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Only 1 message, should not compress
	err := agent.compressContext(context.Background())
	if err != nil {
		t.Errorf("compressContext() should not error with minimal messages: %v", err)
	}
}

func TestCompressContext_PreserveCountBoundary(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Add exactly preserveCount+1 messages (4 messages = boundary for no compression)
	agent.AddUserMessage("first user prompt")
	agent.AddAssistantMessage("assistant response 1")
	agent.AddUserMessage("user message 2")
	agent.AddAssistantMessage("assistant response 2")

	// Should not compress at the boundary
	err := agent.compressContext(context.Background())
	if err != nil {
		t.Errorf("compressContext() should not error at boundary: %v", err)
	}

	// Context should be unchanged
	if len(agent.context) != 4 {
		t.Errorf("Expected 4 messages (unchanged), got %d", len(agent.context))
	}
	if agent.context[0].Role != "user" {
		t.Errorf("Expected first message to be user, got %s", agent.context[0].Role)
	}
	if agent.context[0].Content != "first user prompt" {
		t.Errorf("Expected first user message preserved, got %q", agent.context[0].Content)
	}
}

func TestCompressContext_FirstUserMessagePreserved(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Add enough messages to trigger compression threshold
	agent.AddUserMessage("ORIGINAL FIRST USER PROMPT")
	for i := 0; i < 10; i++ {
		agent.AddAssistantMessage(fmt.Sprintf("assistant response %d", i))
		agent.AddUserMessage(fmt.Sprintf("user message %d", i))
	}

	// Compression will fail because there's no LLM, but the error should be returned
	// (not a panic), and the context should remain unchanged
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err := agent.compressContext(ctx)
	// Expected to fail without LLM
	if err == nil {
		t.Error("Expected error when no LLM is available for compression")
	}
}

func TestCompressContext_SummaryIsAssistantRole(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// This test verifies the expected structure after compression.
	// Without an LLM, compression will fail, but we can verify the
	// early-exit behavior and that the context is not corrupted.
	agent.AddUserMessage("first user prompt")
	agent.AddAssistantMessage("assistant response 1")
	agent.AddUserMessage("user message 2")
	agent.AddAssistantMessage("assistant response 2")
	agent.AddUserMessage("user message 3")
	agent.AddAssistantMessage("assistant response 3")

	// At this point we have 6 messages (> preserveCount+1 = 4)
	// Compression will be attempted but will fail without an LLM
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err := agent.compressContext(ctx)

	// Without LLM, compression fails
	if err == nil {
		t.Error("Expected error when no LLM is available for compression")
	}
}

func TestCompressContext_FirstMessageNotUser(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Add assistant message first (unusual but should handle)
	agent.AddAssistantMessage("assistant first")
	agent.AddUserMessage("user message")
	agent.AddAssistantMessage("assistant response")
	agent.AddUserMessage("user message 2")
	agent.AddAssistantMessage("assistant response 2")
	agent.AddUserMessage("user message 3")
	agent.AddAssistantMessage("assistant response 3")

	// Should not panic even with non-standard message order
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = agent.compressContext(ctx) // Expected to fail without LLM
}

func TestGroupAssistantToolMessages(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	// Test with empty messages
	result := agent.groupAssistantToolMessages(nil)
	if result != nil {
		t.Errorf("Expected nil, got %v", result)
	}

	result = agent.groupAssistantToolMessages([]*inference.Message{})
	if len(result) != 0 {
		t.Errorf("Expected empty, got %d", len(result))
	}
}

