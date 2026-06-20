package tui

import (
	"testing"

	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
)

func TestStreamChunk_RawText(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	// Start stream
	tui.StartStream()

	// Stream some text
	tui.StreamChunk("Hello")
	tui.StreamChunk(" ")
	tui.StreamChunk("World")

	// End stream
	tui.StreamEnd()
}

func TestStreamEnd_StoredContent(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()
	tui.StreamChunk("test content")
	tui.StreamEnd()

	// StreamEnd should not panic
}

func TestStreamChunk_WithType_Normal(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()
	tui.StreamChunkWithType("normal text", inference.StreamingContentTypeNormal)
	tui.StreamEnd()
}

func TestStreamChunk_WithType_Reasoning(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()
	tui.StreamChunkWithType("reasoning text", inference.StreamingContentTypeReasoning)
	tui.StreamEnd()
}

func TestStreamReasoningChunk(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()
	tui.StreamReasoningChunk("Thinking...")
	tui.StreamEnd()
}

func TestStreamNormalChunk(t *testing.T) {
	cfg := config.DefaultConfig()
	tui := NewTUI(cfg)

	tui.StartStream()
	tui.StreamNormalChunk("Hello world")
	tui.StreamEnd()
}

