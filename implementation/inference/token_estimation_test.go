package inference

import (
	"strings"
	"testing"
)

func TestEstimateTokens_Empty(t *testing.T) {
	if result := EstimateTokens(""); result != 0 {
		t.Errorf("Expected 0 tokens for empty string, got %d", result)
	}
}

func TestEstimateTokens_SingleWord(t *testing.T) {
	result := EstimateTokens("hello")
	if result < 1 {
		t.Errorf("Expected at least 1 token, got %d", result)
	}
}

func TestEstimateTokens_MultipleWords(t *testing.T) {
	text := "this is a test with multiple words for token estimation"
	result := EstimateTokens(text)
	words := len(strings.Fields(text))
	// Should be roughly proportional to word count
	if result < words/2 || result > words*3 {
		t.Errorf("Expected reasonable token count for %d words, got %d", words, result)
	}
}

func TestEstimateTokens_Code(t *testing.T) {
	code := "func main() { fmt.Println(\"hello\"); }"
	result := EstimateTokens(code)
	if result < 1 {
		t.Errorf("Expected at least 1 token for code, got %d", result)
	}
}

func TestEstimateContextSize_OnlySystem(t *testing.T) {
	systemPrompt := "You are a helpful assistant."
	result := EstimateContextSize(nil, nil, systemPrompt)
	if result <= 0 {
		t.Errorf("Expected positive result for system prompt only, got %d", result)
	}
}

func TestEstimateContextSize_WithMessages(t *testing.T) {
	systemPrompt := "System prompt"
	messages := []*Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	}
	result := EstimateContextSize(messages, nil, systemPrompt)
	if result <= 0 {
		t.Errorf("Expected positive result, got %d", result)
	}
}

func TestEstimateContextSize_WithTools(t *testing.T) {
	systemPrompt := "System"
	tools := []ToolDefinition{
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "bash",
				Description: "Execute a bash command",
				Parameters: ParameterSchema{
					Type: "object",
					Properties: map[string]Property{
						"command": {Type: "string", Description: "Command to run"},
					},
					Required: []string{"command"},
				},
			},
		},
	}
	result := EstimateContextSize(nil, tools, systemPrompt)
	if result <= 0 {
		t.Errorf("Expected positive result with tools, got %d", result)
	}

	// Adding tools should increase count
	resultNoTools := EstimateContextSize(nil, nil, systemPrompt)
	if result <= resultNoTools {
		t.Errorf("Expected result with tools > without tools: %d <= %d", result, resultNoTools)
	}
}

func TestEstimateContextSize_AllComponents(t *testing.T) {
	systemPrompt := "System prompt for the assistant"
	messages := []*Message{
		{Role: "user", Content: "This is a user message with some content"},
	}
	tools := []ToolDefinition{
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "test_tool",
				Description: "Test tool description",
				Parameters: ParameterSchema{
					Type: "object",
					Properties: map[string]Property{
						"param1": {Type: "string", Description: "First parameter"},
						"param2": {Type: "integer", Description: "Second parameter"},
					},
					Required: []string{"param1"},
				},
			},
		},
	}

	result := EstimateContextSize(messages, tools, systemPrompt)

	// Verify components are additive
	resultNoSys := EstimateContextSize(messages, tools, "")
	resultNoMsgs := EstimateContextSize(nil, tools, systemPrompt)
	resultNoTools := EstimateContextSize(messages, nil, systemPrompt)

	// Total should be >= each component
	if result < resultNoSys || result < resultNoMsgs || result < resultNoTools {
		t.Errorf("Expected total to be sum of all components")
	}
}

