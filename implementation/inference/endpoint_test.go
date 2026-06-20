package inference

import (
	"testing"

	"github.com/coding-agent/harness/config"
)

func TestIsCopilotEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	// Default is not copilot
	if ic.isCopilotEndpoint() {
		t.Error("Expected isCopilotEndpoint to be false for default endpoint")
	}

	// Copilot endpoints
	ic.SetEndpoint("https://copilot.githubcopilot.com/v1")
	if !ic.isCopilotEndpoint() {
		t.Error("Expected isCopilotEndpoint to be true for Copilot endpoint")
	}

	// Not a Copilot endpoint
	ic.SetEndpoint("https://other-server.com/api")
	if ic.isCopilotEndpoint() {
		t.Error("Expected isCopilotEndpoint to be false for non-Copilot endpoint")
	}
}

func TestIsGitHubModelsEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	ic := NewInferenceClient(cfg)

	// GitHub Models endpoint (models.github.ai)
	ic.SetEndpoint("https://models.github.ai")
	if !ic.isGitHubModelsEndpoint() {
		t.Error("Expected isGitHubModelsEndpoint to be true for GitHub Models endpoint")
	}

	// GitHub Models endpoint with path
	ic.SetEndpoint("https://models.github.ai/v1")
	if !ic.isGitHubModelsEndpoint() {
		t.Error("Expected isGitHubModelsEndpoint to be true for GitHub Models endpoint with path")
	}

	// Not a GitHub Models endpoint
	ic.SetEndpoint("https://other-server.com/api")
	if ic.isGitHubModelsEndpoint() {
		t.Error("Expected isGitHubModelsEndpoint to be false for non-GitHub Models endpoint")
	}
}

