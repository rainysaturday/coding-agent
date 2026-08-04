package tools

import (
	"strings"
	"testing"
)

func TestValidateToolNames_Valid(t *testing.T) {
	if err := ValidateToolNames([]string{"read_file", "bash", "grep"}); err != nil {
		t.Errorf("Expected no error for valid tools, got: %v", err)
	}
}

func TestValidateToolNames_Unknown(t *testing.T) {
	err := ValidateToolNames([]string{"read_file", "not_a_tool"})
	if err == nil {
		t.Fatal("Expected error for unknown tool name")
	}
	if !strings.Contains(err.Error(), "not_a_tool") {
		t.Errorf("Expected error to mention unknown tool, got: %v", err)
	}
}

func TestValidateToolNames_EmptyEntries(t *testing.T) {
	if err := ValidateToolNames([]string{"", "bash", ""}); err != nil {
		t.Errorf("Expected empty entries to be ignored, got: %v", err)
	}
}
