package tools

import (
	"os/exec"
	"strings"
)

// BashTool implements the bash command execution tool
type BashTool struct{}

// NewBashTool creates a new BashTool
func NewBashTool() *BashTool {
	return &BashTool{}
}

// Name returns the tool name
func (t *BashTool) Name() string {
	return "bash"
}

// Description returns a human-readable description
func (t *BashTool) Description() string {
	return "Execute a bash command"
}

// Execute executes a bash command
func (t *BashTool) Execute(params map[string]string) ToolResult {
	command, ok := params["command"]
	if !ok || command == "" {
		return ToolResult{
			Success: false,
			Error:   "missing required parameter: command",
		}
	}

	// Execute the command
	cmd := exec.Command("/bin/sh", "-c", command)

	// Get stdout and stderr
	output, err := cmd.CombinedOutput()

	result := ToolResult{
		Success: err == nil,
	}

	if output != nil {
		result.Output = strings.TrimSpace(string(output))
	}

	if err != nil {
		// Include exit error in the result
		result.Error = err.Error()
		// If we have partial output, include it
		if result.Output == "" && len(output) > 0 {
			result.Output = strings.TrimSpace(string(output))
		}
	}

	return result
}
