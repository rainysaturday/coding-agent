package inference

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"coding-agent/context"
	"coding-agent/stats"
)

func TestNewInferenceClient(t *testing.T) {
	s := stats.NewStats()
	client := NewInferenceClient(
		"http://localhost:8080/v1",
		"test-key",
		"llama-model",
		4096,
		7200,
		30,
		300,
		true,
		s,
	)

	if client == nil {
		t.Fatal("NewInferenceClient returned nil")
	}
	if client.endpoint != "http://localhost:8080/v1" {
		t.Errorf("Expected endpoint 'http://localhost:8080/v1', got '%s'", client.endpoint)
	}
}

func TestInferenceClient_NonStreaming(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		response := Response{
			Choices: []Choice{
				{
					Message: Message{
						Role:    "assistant",
						Content: "Hello! How can I help you?",
					},
					FinishReason: "stop",
				},
			},
			Usage: Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	s := stats.NewStats()
	client := NewInferenceClient(
		server.URL+"/v1",
		"test-key",
		"llama-model",
		4096,
		7200,
		30,
		300,
		false, // Non-streaming
		s,
	)

	ctx := context.NewContext("system", 1000)
	ctx.AddUserMessage("Hello")

	var completed bool

	err := client.ChatCompletion(ChatCompletionRequest{
		Context: ctx,
		OnComplete: func() {
			completed = true
		},
		OnError: func(err error) {
			t.Errorf("Unexpected error: %v", err)
		},
	})

	if err != nil {
		t.Fatalf("ChatCompletion failed: %v", err)
	}

	if !completed {
		t.Error("OnComplete was not called")
	}

	if ctx.GetMessageCount() != 3 {
		t.Errorf("Expected 3 messages (system, user, assistant), got %d", ctx.GetMessageCount())
	}

	// Check stats
	if s.GetInputTokens() != 10 {
		t.Errorf("Expected 10 input tokens, got %d", s.GetInputTokens())
	}
	if s.GetOutputTokens() != 5 {
		t.Errorf("Expected 5 output tokens, got %d", s.GetOutputTokens())
	}
}

func TestInferenceClient_Streaming(t *testing.T) {
	// Create a test server for streaming
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		// Send streaming chunks
		response := StreamResponse{
			Choices: []StreamChoice{
				{
					Delta: Message{
						Content: "Hello",
					},
				},
				{
					Delta: Message{
						Content: "!",
					},
				},
				{
					Delta:        Message{},
					FinishReason: "stop",
				},
			},
		}

		for _, choice := range response.Choices {
			data, _ := json.Marshal(StreamResponse{Choices: []StreamChoice{choice}})
			w.Write([]byte("data: " + string(data) + "\n\n"))
			w.(http.Flusher).Flush()
		}
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	s := stats.NewStats()
	client := NewInferenceClient(
		server.URL+"/v1",
		"test-key",
		"llama-model",
		4096,
		7200,
		30,
		300,
		true, // Streaming
		s,
	)

	ctx := context.NewContext("system", 1000)
	ctx.AddUserMessage("Hello")

	tokensReceived := ""
	var completed bool

	err := client.ChatCompletion(ChatCompletionRequest{
		Context: ctx,
		OnToken: func(token string) {
			tokensReceived += token
		},
		OnComplete: func() {
			completed = true
		},
		OnError: func(err error) {
			t.Errorf("Unexpected error: %v", err)
		},
	})

	if err != nil {
		t.Fatalf("ChatCompletion failed: %v", err)
	}

	if !completed {
		t.Error("OnComplete was not called")
	}

	if tokensReceived != "Hello!" {
		t.Errorf("Expected 'Hello!', got '%s'", tokensReceived)
	}
}

func TestInferenceClient_APIError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	s := stats.NewStats()
	client := NewInferenceClient(
		server.URL+"/v1",
		"test-key",
		"llama-model",
		4096,
		7200,
		30,
		300,
		false,
		s,
	)

	ctx := context.NewContext("system", 1000)
	ctx.AddUserMessage("Hello")

	err := client.ChatCompletion(ChatCompletionRequest{
		Context: ctx,
		OnError: func(err error) {
			t.Errorf("Unexpected error callback: %v", err)
		},
	})

	if err == nil {
		t.Error("Expected error for API failure")
	}
}

func TestInferenceClient_InvalidJSON(t *testing.T) {
	// Create a test server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	s := stats.NewStats()
	client := NewInferenceClient(
		server.URL+"/v1",
		"test-key",
		"llama-model",
		4096,
		7200,
		30,
		300,
		false,
		s,
	)

	ctx := context.NewContext("system", 1000)
	ctx.AddUserMessage("Hello")

	err := client.ChatCompletion(ChatCompletionRequest{
		Context: ctx,
		OnError: func(err error) {
			t.Errorf("Unexpected error callback: %v", err)
		},
	})

	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestInferenceClient_NoApiKey(t *testing.T) {
	// Create a test server that checks for no API key
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify no Authorization header
		auth := r.Header.Get("Authorization")
		if auth != "" {
			t.Errorf("Expected no Authorization header, got '%s'", auth)
		}

		response := Response{
			Choices: []Choice{
				{
					Message: Message{
						Role:    "assistant",
						Content: "Response",
					},
				},
			},
			Usage: Usage{
				PromptTokens:     5,
				CompletionTokens: 3,
				TotalTokens:      8,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	s := stats.NewStats()
	client := NewInferenceClient(
		server.URL+"/v1",
		"", // No API key
		"llama-model",
		4096,
		7200,
		30,
		300,
		false,
		s,
	)

	ctx := context.NewContext("system", 1000)
	ctx.AddUserMessage("Hello")

	err := client.ChatCompletion(ChatCompletionRequest{
		Context: ctx,
	})

	if err != nil {
		t.Fatalf("ChatCompletion failed: %v", err)
	}
}

func TestInferenceClient_NoApiKeyPlaceholder(t *testing.T) {
	// Test with "not-needed" placeholder API key
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify no Authorization header
		auth := r.Header.Get("Authorization")
		if auth != "" {
			t.Errorf("Expected no Authorization header, got '%s'", auth)
		}

		response := Response{
			Choices: []Choice{
				{
					Message: Message{
						Role:    "assistant",
						Content: "Response",
					},
				},
			},
			Usage: Usage{
				PromptTokens:     5,
				CompletionTokens: 3,
				TotalTokens:      8,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	s := stats.NewStats()
	client := NewInferenceClient(
		server.URL+"/v1",
		"not-needed", // Placeholder API key
		"llama-model",
		4096,
		7200,
		30,
		300,
		false,
		s,
	)

	ctx := context.NewContext("system", 1000)
	ctx.AddUserMessage("Hello")

	err := client.ChatCompletion(ChatCompletionRequest{
		Context: ctx,
	})

	if err != nil {
		t.Fatalf("ChatCompletion failed: %v", err)
	}
}
