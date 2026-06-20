package agent

import (
	"context"
	"testing"
	"time"

	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
	"github.com/coding-agent/harness/tools"
)

func TestRunStream(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// This tests the method exists and doesn't panic without actual LLM
	// The call will fail because there's no LLM server, but we test the method structure
	_, err := agent.RunStream(ctx, "test prompt", func(chunk inference.StreamingChunk) {
		// noop
	})

	// Expect error since no LLM server is running (timeout or connection error)
	if err == nil {
		t.Error("Expected error when no LLM server is available")
	}
}

func TestStreamStatus_Callback(t *testing.T) {
	var received []inference.StreamingChunk

	cb := func(chunk inference.StreamingChunk) {
		received = append(received, chunk)
	}

	// Test various tool types
	tools := map[string]map[string]interface{}{
		"bash":         {"command": "ls -la"},
		"read_file":    {"path": "/test/file.txt"},
		"write_file":   {"path": "/test/output.txt"},
		"read_lines":   {"path": "/test/file.txt", "start": 1, "end": 10},
		"insert_lines": {"path": "/test/file.txt", "line": 5},
		"replace_text": {"path": "/test/file.txt", "search": "old"},
		"unknown":      nil,
	}

	for toolName, params := range tools {
		streamStatus(toolName, params, cb)
		if len(received) == 0 {
			t.Errorf("Expected callback to be called for tool: %s", toolName)
		}
		received = nil // Reset for next test
	}

	// Test that callbacks with nil doesn't panic
	streamStatus("bash", nil, nil)
}

func TestStreamResult_Callback(t *testing.T) {
	var received []inference.StreamingChunk

	cb := func(chunk inference.StreamingChunk) {
		received = append(received, chunk)
	}

	successResult := &tools.ToolResult{
		Success: true,
		Output:  "success output",
	}
	streamResult("bash", successResult, cb)
	if len(received) == 0 {
		t.Error("Expected callback to be called for success result")
	}

	failureResult := &tools.ToolResult{
		Success: false,
		Error:   "some error",
	}
	streamResult("bash", failureResult, cb)
	if len(received) == 0 {
		t.Error("Expected callback to be called for failure result")
	}

	// Test nil callback
	streamResult("bash", successResult, nil)
}

func TestStreamToolCallWithFullParams_NilCallback(t *testing.T) {
	tc := &tools.ToolCall{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "echo test",
		},
	}

	// Should not panic with nil callback
	streamToolCallWithFullParams(tc, nil)
}

func TestRunStream_ContextCancellation(t *testing.T) {
	cfg := config.DefaultConfig()
	agent := NewAgent(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := agent.RunStream(ctx, "test prompt", nil)
	if err == nil {
		t.Error("Expected error when context is cancelled")
	}
}

