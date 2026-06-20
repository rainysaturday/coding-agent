package inference

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/coding-agent/harness/config"
)

func TestRequestBody_Marshal(t *testing.T) {
	temp := 0.7
	req := RequestBody{
		Model:       "llama3",
		Messages:    []*Message{{Role: "user", Content: "test"}},
		Stream:      true,
		Temperature: &temp,
		MaxTokens:   4096,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if parsed["model"] != "llama3" {
		t.Errorf("Expected model 'llama3', got %v", parsed["model"])
	}

	if parsed["stream"] != true {
		t.Errorf("Expected stream true, got %v", parsed["stream"])
	}
}

func TestRequestBody_WithoutTemperature(t *testing.T) {
	req := RequestBody{
		Model:     "test",
		Messages:  []*Message{{Role: "user", Content: "test"}},
		Stream:    false,
		MaxTokens: 1024,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(jsonData, &parsed)

	// Temperature should not be present when nil
	if _, exists := parsed["temperature"]; exists {
		t.Error("Expected temperature to be omitted when nil")
	}
}

func TestInferenceRequest_WithContext(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This will fail because there's no server, but tests the interface
	_, err := client.InferenceRequest(ctx, nil, "test")
	if err == nil {
		t.Error("Expected error when no server is running")
	}
}

func TestInferenceRequestStream_WithContext(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This will fail because there's no server, but tests the interface
	_, err := client.InferenceRequestStream(ctx, nil, "test", nil)
	if err == nil {
		t.Error("Expected error when no server is running")
	}
}

func TestInferenceRequestWithCallbackTyped(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var callback = func(chunk StreamingChunk) {
		// noop
	}

	_, err := client.InferenceRequestWithCallbackTyped(ctx, nil, "test", callback)
	if err == nil {
		t.Error("Expected error when no server is running")
	}
	// Callback should not be called since connection fails before streaming
}

