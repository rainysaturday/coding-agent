package inference

import (
	"testing"
	"time"

	"github.com/coding-agent/harness/config"
)

func TestNewInferenceClient(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	if client == nil {
		t.Fatal("NewInferenceClient() returned nil")
	}

	if client.model != "llama3" {
		t.Errorf("Expected model 'llama3', got '%s'", client.model)
	}

	if client.temperature != nil {
		t.Errorf("Expected temperature nil, got %f", *client.temperature)
	}

	if client.maxTokens != 64000 {
		t.Errorf("Expected maxTokens 64000, got %d", client.maxTokens)
	}

	if client.contextSize != 128000 {
		t.Errorf("Expected contextSize 128000, got %d", client.contextSize)
	}

	if client.streaming != true {
		t.Errorf("Expected streaming true, got %v", client.streaming)
	}

	if client.maxRetries != 3 {
		t.Errorf("Expected maxRetries 3, got %d", client.maxRetries)
	}

	if client.retryDelay != 1*time.Second {
		t.Errorf("Expected retryDelay 1s, got %v", client.retryDelay)
	}

	if client.client == nil {
		t.Error("Expected http client to be initialized")
	}
}

func TestNewInferenceClient_CustomConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Model = "custom-model"
	cfg.MaxTokens = 8192
	cfg.Streaming = false
	cfg.APIEndpoint = "http://custom:9000"

	client := NewInferenceClient(cfg)

	if client.model != "custom-model" {
		t.Errorf("Expected model 'custom-model', got '%s'", client.model)
	}

	if client.maxTokens != 8192 {
		t.Errorf("Expected maxTokens 8192, got %d", client.maxTokens)
	}

	if client.streaming != false {
		t.Errorf("Expected streaming false, got %v", client.streaming)
	}

	if client.endpoint != "http://custom:9000" {
		t.Errorf("Expected endpoint 'http://custom:9000', got '%s'", client.endpoint)
	}
}

func TestNewInferenceClient_Timeout(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.InitialTokenTimeout = 300
	cfg.ReadTimeout = 600

	client := NewInferenceClient(cfg)

	if client.timeout != 600*time.Second {
		t.Errorf("Expected timeout 600s (read timeout), got %v", client.timeout)
	}
}

func TestSetEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	endpoints := []string{
		"http://localhost:8080",
		"http://api.example.com/v1",
		"https://api.openai.com/v1",
	}

	for _, endpoint := range endpoints {
		client.SetEndpoint(endpoint)
		if client.endpoint != endpoint {
			t.Errorf("SetEndpoint(%q) failed, got %q", endpoint, client.endpoint)
		}
	}
}

func TestSetAPIKey(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	keys := []string{
		"",
		"sk-test-key-123",
		"another-key-abc",
	}

	for _, key := range keys {
		client.SetAPIKey(key)
		if client.apiKey != key {
			t.Errorf("SetAPIKey(%q) failed, got %q", key, client.apiKey)
		}
	}
}

func TestGetTools_Empty(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewInferenceClient(cfg)

	tools := client.GetTools()
	if len(tools) != 0 {
		t.Errorf("Expected empty tools, got %d", len(tools))
	}
}

func TestInferenceClient_SetMaxDisplayWidth(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	ic.SetMaxDisplayWidth(120)
	if ic.maxDisplayWidth != 120 {
		t.Errorf("Expected maxDisplayWidth 120, got %d", ic.maxDisplayWidth)
	}
}

func TestInferenceClient_SetAPIKey(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	ic.SetAPIKey("test-key-123")
	if ic.apiKey != "test-key-123" {
		t.Errorf("Expected apiKey 'test-key-123', got %q", ic.apiKey)
	}
}

func TestInferenceClient_GetTools(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	tools := []ToolDefinition{
		{Type: "function", Function: FunctionDefinition{Name: "tool1", Description: "First tool", Parameters: ParameterSchema{Type: "object"}}},
	}
	ic.SetTools(tools)

	result := ic.GetTools()
	if len(result) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(result))
	}
	if result[0].Function.Name != "tool1" {
		t.Errorf("Expected tool name 'tool1', got %q", result[0].Function.Name)
	}
}

