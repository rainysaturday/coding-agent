package inference

import (
	"encoding/json"
	"testing"
)

func TestInferenceClient_MessagesType(t *testing.T) {
	msg := Message{
		Role:       "user",
		Content:    "Hello",
		ToolCallId: "call_123",
		ToolCalls: []*APIToolCall{
			{
				ID:   "call_123",
				Type: "function",
				Function: FunctionCall{
					Name:      "bash",
					Arguments: `{"command":"echo hello"}`,
				},
			},
		},
	}

	if msg.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", msg.Role)
	}

	if msg.ToolCallId != "call_123" {
		t.Errorf("Expected ToolCallId 'call_123', got '%s'", msg.ToolCallId)
	}

	if len(msg.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(msg.ToolCalls))
	}
}

func TestInferenceClient_ResponseFields(t *testing.T) {
	resp := Response{
		Content:      "Test response",
		Reasoning:    "Let me think about this...",
		TokenUsage:   100,
		StreamUsage:  50,
		InputTokens:  60,
		OutputTokens: 40,
	}

	if resp.Content != "Test response" {
		t.Errorf("Expected content 'Test response', got '%s'", resp.Content)
	}

	if resp.Reasoning != "Let me think about this..." {
		t.Errorf("Expected reasoning content")
	}

	if resp.InputTokens+resp.OutputTokens != 100 {
		t.Errorf("Expected input+output to equal total")
	}
}

func TestToolDefinition_Structure(t *testing.T) {
	toolDef := ToolDefinition{
		Type: "function",
		Function: FunctionDefinition{
			Name:        "test_tool",
			Description: "A test tool for testing",
			Parameters: ParameterSchema{
				Type:     "object",
				Required: []string{"required_param"},
				Properties: map[string]Property{
					"required_param": {Type: "string", Description: "A required parameter"},
					"optional_param": {Type: "string", Description: "An optional parameter"},
				},
			},
		},
	}

	if toolDef.Type != "function" {
		t.Errorf("Expected type 'function', got '%s'", toolDef.Type)
	}

	if toolDef.Function.Name != "test_tool" {
		t.Errorf("Expected name 'test_tool', got '%s'", toolDef.Function.Name)
	}

	if len(toolDef.Function.Parameters.Required) != 1 {
		t.Errorf("Expected 1 required parameter, got %d", len(toolDef.Function.Parameters.Required))
	}

	if len(toolDef.Function.Parameters.Properties) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(toolDef.Function.Parameters.Properties))
	}
}

func TestParameterSchema_Structure(t *testing.T) {
	schema := ParameterSchema{
		Type:       "object",
		Required:   []string{"param1", "param2"},
		Properties: map[string]Property{},
	}

	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got '%s'", schema.Type)
	}

	schema.Properties["param1"] = Property{Type: "string", Description: "First param"}
	schema.Properties["param2"] = Property{Type: "integer", Description: "Second param"}

	if len(schema.Properties) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(schema.Properties))
	}
}

func TestFunctionCall_Structure(t *testing.T) {
	call := FunctionCall{
		Name:      "bash",
		Arguments: `{"command":"ls -la"}`,
	}

	if call.Name != "bash" {
		t.Errorf("Expected name 'bash', got '%s'", call.Name)
	}

	if call.Arguments != `{"command":"ls -la"}` {
		t.Errorf("Expected arguments, got '%s'", call.Arguments)
	}
}

func TestAPIToolCall_JSON(t *testing.T) {
	apiCall := APIToolCall{
		ID:   "call_abc",
		Type: "function",
		Function: FunctionCall{
			Name:      "read_file",
			Arguments: `{"path":"/test/file.txt"}`,
		},
	}

	jsonData, err := json.Marshal(apiCall)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if parsed["id"] != "call_abc" {
		t.Errorf("Expected id 'call_abc', got %v", parsed["id"])
	}

	fn, ok := parsed["function"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected function to be a map")
	}

	if fn["name"] != "read_file" {
		t.Errorf("Expected function name 'read_file', got %v", fn["name"])
	}
}

