package inference

import (
	"testing"
)

func TestFormatToolCallArgs_EmptyMap(t *testing.T) {
	result := formatToolCallArgs(map[string]interface{}{}, 80)
	if result != "{}" {
		t.Errorf("Expected '{}', got %q", result)
	}
}

func TestFormatToolCallArgs_NilParams(t *testing.T) {
	result := formatToolCallArgs(nil, 80)
	if result != "{}" {
		t.Errorf("Expected '{}', got %q", result)
	}
}

func TestFormatToolCallArgs_SingleParam(t *testing.T) {
	result := formatToolCallArgs(map[string]interface{}{"key": "value"}, 80)
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

func TestFormatToolCallArgs_MaxWidthTruncation(t *testing.T) {
	// Use a very small max width to trigger truncation
	result := formatToolCallArgs(map[string]interface{}{"key": "value"}, 10)
	if result == "" {
		t.Error("Expected non-empty result")
	}
	// Should be truncated
	if len(result) > 15 {
		t.Error("Expected truncated result")
	}
}

func TestFormatToolCallArgs_ManualParams(t *testing.T) {
	// Test with manually constructed params like bash tool would receive
	params := map[string]interface{}{
		"command": "echo hello",
		"args":    []interface{}{"arg1", "arg2"},
	}
	result := formatToolCallArgs(params, 100)
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

// ===== Tests for formatJSONMapWithMaxWidth =====

func TestFormatJSONMapWithMaxWidth(t *testing.T) {
	m := map[string]interface{}{"key": "value"}
	result := formatJSONMapWithMaxWidth(m, 80)
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

func TestFormatJSONArrayWithMaxWidth(t *testing.T) {
	arr := []interface{}{"a", "b", "c"}
	result := formatJSONArrayWithMaxWidth(arr, 80)
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

// ===== Tests for handleStreamResponse with callbacks =====

