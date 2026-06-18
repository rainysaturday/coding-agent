// Package tools implements the tool execution system for the coding agent.
// Each tool has its own file in this package:
//
//	tools.go       - Core types and executor dispatcher
//	utils.go       - Shared utility functions
//	bash.go        - Bash command execution
//	read_file.go   - File reading
//	write_file.go  - File writing
//	read_lines.go  - Line-range reading
//	insert_lines.go - Line insertion
//	replace_text.go - Text replacement
//	list_files.go  - Directory listing (ls-like)
//	grep.go        - Text search (grep-like)
//	git_log.go     - Git log
//	git_show.go    - Git show
//	git_diff.go    - Git diff
//	view_image.go  - Image loading
//	todo.go        - Todo store and executor
//	subagent.go    - Subagent execution
package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	Success  bool                   `json:"success"`
	Output   string                 `json:"output,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Path     string                 `json:"path,omitempty"`
	ExitCode int                    `json:"exit_code,omitempty"`
	Extra    map[string]interface{} `json:"-"`
}

// ToolCall represents a tool call parsed from the LLM response (OpenAI format compatible).
// Supports both legacy format (name/parameters) and OpenAI format (function/arguments).
type ToolCall struct {
	ID         string                 `json:"id,omitempty"` // OpenAI tool call ID
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Arguments  string                 `json:"arguments,omitempty"` // OpenAI: raw JSON string of arguments
	Raw        string                 `json:"-"`
}

// Stats tracks tool execution statistics.
type Stats struct {
	TotalCalls  int
	FailedCalls int
}

// ToolExecutor coordinates the execution of all tools.
type ToolExecutor struct {
	readOnly  bool
	todoStore *TodoStore
	stats     *Stats
}

// NewToolExecutor creates a new tool executor.
func NewToolExecutor() *ToolExecutor {
	return &ToolExecutor{
		stats:     &Stats{},
		todoStore: NewTodoStore(),
	}
}

// SetReadOnly sets the read-only mode.
func (te *ToolExecutor) SetReadOnly(readOnly bool) {
	te.readOnly = readOnly
}

// Stats returns the current execution statistics.
func (te *ToolExecutor) Stats() *Stats {
	return te.stats
}

// ParseToolCall parses a raw JSON tool call string into a ToolCall struct.
// This is used to parse OpenAI-format tool calls from LLM responses.
func ParseToolCall(raw string) (*ToolCall, error) {
	// Use a wrapper struct to properly handle the nested function object
	var wrapper struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}

	if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
		return nil, fmt.Errorf("invalid tool call JSON: %v", err)
	}

	if wrapper.Function.Name == "" {
		return nil, fmt.Errorf("missing tool name in tool call")
	}

	// Parse arguments JSON string into parameters
	var params map[string]interface{}
	if wrapper.Function.Arguments != "" {
		if err := json.Unmarshal([]byte(wrapper.Function.Arguments), &params); err != nil {
			// If arguments parsing fails, keep raw arguments
			params = map[string]interface{}{
				"_raw_arguments": wrapper.Function.Arguments,
			}
		}
	}

	tc := &ToolCall{
		ID:         wrapper.ID,
		Name:       wrapper.Function.Name,
		Parameters: params,
		Arguments:  wrapper.Function.Arguments,
		Raw:        raw,
	}
	return tc, nil
}

// Execute dispatches a tool call to the appropriate handler.
// It handles read-only mode checks, parameter validation, and statistics tracking.
func (te *ToolExecutor) Execute(ctx context.Context, tc *ToolCall) *ToolResult {
	te.stats.TotalCalls++

	// Special handling for todo tool in read-only mode:
	// add and complete are write actions that are blocked, but list and remove are allowed
	if te.readOnly && tc.Name == "todo" {
		if action, ok := tc.Parameters["action"].(string); ok {
			if action == "add" || action == "complete" {
				te.stats.FailedCalls++
				return &ToolResult{
					Success: false,
					Error:   fmt.Sprintf("Tool 'todo' action '%s' is not available in read-only mode", action),
					Extra: map[string]interface{}{
						"tool_name": tc.Name,
					},
				}
			}
		}
	}

	// Check if tool is allowed in read-only mode
	if te.readOnly && !isReadOnlyTool(tc.Name) {
		te.stats.FailedCalls++
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Tool '%s' is not available in read-only mode", tc.Name),
			Extra: map[string]interface{}{
				"tool_name": tc.Name,
			},
		}
	}

	var result *ToolResult

	switch tc.Name {
	case "bash":
		result = te.executeBash(ctx, tc.Parameters)
	case "read_file":
		result = te.executeReadFile(tc.Parameters)
	case "write_file":
		result = te.executeWriteFile(tc.Parameters)
	case "read_lines":
		result = te.executeReadLines(tc.Parameters)
	case "insert_lines":
		result = te.executeInsertLines(tc.Parameters)
	case "replace_text":
		result = te.executeReplaceText(tc.Parameters)
	case "list_files":
		result = te.executeListFiles(ctx, tc.Parameters)
	case "grep":
		result = te.executeGrep(ctx, tc.Parameters)
	case "git_log":
		result = te.executeGitLog(ctx, tc.Parameters)
	case "git_show":
		result = te.executeGitShow(ctx, tc.Parameters)
	case "git_diff":
		result = te.executeGitDiff(ctx, tc.Parameters)
	case "subagent":
		result = ExecuteSubagent(tc.Parameters)
	case "view_image":
		result = te.executeViewImage(tc.Parameters)
	case "todo":
		result = te.executeTodo(tc.Parameters)
	default:
		result = &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown tool: %s", tc.Name),
		}
	}

	if !result.Success {
		te.stats.FailedCalls++
	}

	return result
}

// isReadOnlyTool checks if a tool is allowed in read-only mode.
// read_file, list_files, read_lines, grep, git_log, git_show, and view_image are safe read-only operations.
// todo is also allowed since add/complete are blocked by earlier per-action check.
var readOnlyTools = map[string]bool{
	"read_file":   true,
	"list_files":  true,
	"read_lines":  true,
	"grep":        true,
	"git_log":     true,
	"git_show":    true,
	"git_diff":    true,
	"view_image":  true,
	"todo":        true,
}

// isReadOnlyTool checks if a tool is allowed in read-only mode.
func isReadOnlyTool(name string) bool {
	return readOnlyTools[name]
}
