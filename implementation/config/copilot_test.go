package config

import (
	"testing"
)

func TestIsGitHubCopilotEndpoint_True(t *testing.T) {
	endpoints := []string{
		"https://api.githubcopilot.com",
		"http://api.githubcopilot.com",
		"https://api.githubcopilot.com/chat/completions",
		"https://copilot.githubcopilot.com/v1",
	}

	for _, endpoint := range endpoints {
		if !IsGitHubCopilotEndpoint(endpoint) {
			t.Errorf("Expected Copilot detection for %q to return true", endpoint)
		}
	}
}

func TestIsGitHubCopilotEndpoint_False(t *testing.T) {
	endpoints := []string{
		"http://localhost:8080",
		"https://api.openai.com/v1",
		"https://models.github.ai",
		"https://api.anthropic.com/v1",
		"https://githubcopilot.com.evil.example",
		"https://evil-githubcopilot.com",
	}

	for _, endpoint := range endpoints {
		if IsGitHubCopilotEndpoint(endpoint) {
			t.Errorf("Expected Copilot detection for %q to return false", endpoint)
		}
	}
}

func TestIsGitHubModelsEndpoint(t *testing.T) {
	trueCases := []string{
		"https://models.github.ai",
		"https://models.github.ai/v1",
		"http://models.github.ai",
	}
	falseCases := []string{
		"http://localhost:8080",
		"https://api.openai.com/v1",
		"https://api.githubcopilot.com",
		"https://github.ai.evil.example",
		"https://evil-models.github.ai.example",
	}

	for _, endpoint := range trueCases {
		if !IsGitHubModelsEndpoint(endpoint) {
			t.Errorf("Expected GitHub Models detection for %q to return true", endpoint)
		}
	}
	for _, endpoint := range falseCases {
		if IsGitHubModelsEndpoint(endpoint) {
			t.Errorf("Expected GitHub Models detection for %q to return false", endpoint)
		}
	}
}
