package inference

import (
	"net/http"
	"strings"
	"testing"

	"coding-agent/implementation/config"
	"coding-agent/implementation/types"
)

func TestEndpoint_Copilot(t *testing.T) {
	c := New(config.Config{APIEndpoint: "https://api.githubcopilot.com"})
	got := c.endpoint()
	want := "https://api.githubcopilot.com/chat/completions"
	if got != want {
		t.Fatalf("endpoint() = %q, want %q", got, want)
	}
}

func TestEndpoint_GitHubModels(t *testing.T) {
	c := New(config.Config{APIEndpoint: "https://models.github.ai"})
	got := c.endpoint()
	want := "https://models.github.ai/inference/chat/completions"
	if got != want {
		t.Fatalf("endpoint() = %q, want %q", got, want)
	}
}

func TestEndpoint_DefaultOpenAICompatible(t *testing.T) {
	c := New(config.Config{APIEndpoint: "https://api.example.com"})
	got := c.endpoint()
	want := "https://api.example.com/v1/chat/completions"
	if got != want {
		t.Fatalf("endpoint() = %q, want %q", got, want)
	}
}

func TestFormatHTTPError_CopilotPATHint(t *testing.T) {
	c := New(config.Config{APIEndpoint: "https://api.githubcopilot.com"})
	err := c.formatHTTPError(http.StatusBadRequest, "400 Bad Request", []byte("checking third-party user token: bad request: Personal Access Tokens are not supported for this endpoint"))
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "does not accept Personal Access Tokens") {
		t.Fatalf("expected PAT guidance in error, got: %s", msg)
	}
	if !strings.Contains(msg, "models.github.ai") {
		t.Fatalf("expected models endpoint hint in error, got: %s", msg)
	}
}

func TestShouldRetryStatus(t *testing.T) {
	if shouldRetryStatus(http.StatusBadRequest) {
		t.Fatal("400 should not be retried")
	}
	if !shouldRetryStatus(http.StatusTooManyRequests) {
		t.Fatal("429 should be retried")
	}
	if !shouldRetryStatus(http.StatusInternalServerError) {
		t.Fatal("500 should be retried")
	}
}

func TestNormalizeMessages_DefaultsToolCallType(t *testing.T) {
	in := []types.Message{
		{
			Role: "assistant",
			ToolCalls: []types.ToolCall{
				{ID: "call_1", Type: "", Function: types.ToolCallFunction{Name: "write_file", Arguments: "{}"}},
			},
		},
	}

	out := normalizeMessages(in)
	if got := out[0].ToolCalls[0].Type; got != "function" {
		t.Fatalf("expected normalized type=function, got %q", got)
	}
	if got := in[0].ToolCalls[0].Type; got != "" {
		t.Fatalf("expected input to remain unchanged, got %q", got)
	}
}
