package agent

import (
	"fmt"
	"strings"

	"github.com/coding-agent/harness/colors"
	"github.com/coding-agent/harness/inference"
	"github.com/coding-agent/harness/tools"
)

// getToolParamStr extracts a string parameter from tool params map.
func getToolParamStr(key string, params map[string]interface{}) string {
	if p, ok := params[key].(string); ok {
		return p
	}
	return ""
}

// getToolParamInt extracts an int parameter from tool params map (stored as float64 from JSON).
func getToolParamInt(key string, params map[string]interface{}) int {
	if p, ok := params[key].(float64); ok {
		return int(p)
	}
	return 0
}

// streamToolCallWithFullParams streams the full tool call with complete parameters.
// This is called when a tool is about to be executed, showing the full command/parameters
// (not truncated like during streaming display).
func streamToolCallWithFullParams(tc *tools.ToolCall, callback StreamCallback) {
	var msg string
	params := tc.Parameters

	// Format the full parameters for display
	var paramsStr string
	if len(params) > 0 {
		paramsStr = formatFullToolParams(params)
	}

	switch tc.Name {
	case "bash":
		cmd := getToolParamStr("command", params)
		if cmd != "" {
			msg = fmt.Sprintf("\n%s[Bash] %s%s\n", colors.GetColor("cyan"), cmd, colors.GetColor("reset"))
		} else if paramsStr != "" {
			msg = fmt.Sprintf("\n%s[Bash] (%s)%s\n", colors.GetColor("cyan"), paramsStr, colors.GetColor("reset"))
		}
	case "read_file":
		msg = fmt.Sprintf("\n%s[Read] %s%s\n", colors.GetColor("cyan"), getToolParamStr("path", params), colors.GetColor("reset"))
	case "read_lines":
		msg = fmt.Sprintf("\n%s[Read] %s%s\n", colors.GetColor("cyan"), getToolParamStr("path", params), colors.GetColor("reset"))
	case "write_file":
		msg = fmt.Sprintf("\n%s[Write] %s%s\n", colors.GetColor("cyan"), getToolParamStr("path", params), colors.GetColor("reset"))
	case "insert_lines":
		msg = fmt.Sprintf("\n%s[Insert] %s%s\n", colors.GetColor("cyan"), getToolParamStr("path", params), colors.GetColor("reset"))
	case "replace_text":
		path := getToolParamStr("path", params)
		search := getToolParamStr("search", params)
		if len(search) > 30 {
			search = tools.TruncateRunes(search, 30)
		}
		msg = fmt.Sprintf("\n%s[Replace] '%s' in: %s%s\n", colors.GetColor("cyan"), search, path, colors.GetColor("reset"))
	case "list_files":
		path := getToolParamStr("path", params)
		if path != "" {
			msg = fmt.Sprintf("\n%s[ListFiles] %s%s\n", colors.GetColor("cyan"), path, colors.GetColor("reset"))
		} else {
			msg = fmt.Sprintf("\n%s[ListFiles] .%s\n", colors.GetColor("cyan"), colors.GetColor("reset"))
		}
	case "grep":
		msg = fmt.Sprintf("\n%s[Grep] %s%s\n", colors.GetColor("cyan"), getToolParamStr("pattern", params), colors.GetColor("reset"))
	case "git_log":
		path := getToolParamStr("path", params)
		reference := getToolParamStr("reference", params)
		if reference != "" {
			msg = fmt.Sprintf("\n%s[GitLog] %s in: %s%s\n", colors.GetColor("cyan"), reference, path, colors.GetColor("reset"))
		} else {
			msg = fmt.Sprintf("\n%s[GitLog] HEAD in: %s%s\n", colors.GetColor("cyan"), path, colors.GetColor("reset"))
		}
	case "git_show":
		commit := getToolParamStr("commit", params)
		if commit != "" {
			msg = fmt.Sprintf("\n%s[GitShow] commit: %s%s\n", colors.GetColor("cyan"), commit, colors.GetColor("reset"))
		} else {
			msg = fmt.Sprintf("\n%s[GitShow] HEAD%s\n", colors.GetColor("cyan"), colors.GetColor("reset"))
		}
	case "git_diff":
		ref1 := getToolParamStr("reference1", params)
		ref2 := getToolParamStr("reference2", params)
		if ref1 != "" && ref2 != "" {
			msg = fmt.Sprintf("\n%s[GitDiff] %s..%s%s\n", colors.GetColor("cyan"), ref1, ref2, colors.GetColor("reset"))
		} else if ref1 != "" {
			msg = fmt.Sprintf("\n%s[GitDiff] %s%s\n", colors.GetColor("cyan"), ref1, colors.GetColor("reset"))
		} else {
			msg = fmt.Sprintf("\n%s[GitDiff] working tree%s\n", colors.GetColor("cyan"), colors.GetColor("reset"))
		}
	case "move_text":
		msg = fmt.Sprintf("\n%s[MoveText] %s%s\n", colors.GetColor("cyan"), getToolParamStr("source_path", params), colors.GetColor("reset"))
	default:
		if paramsStr != "" {
			msg = fmt.Sprintf("\n%s[Tool: %s] (%s)%s\n", colors.GetColor("cyan"), tc.Name, paramsStr, colors.GetColor("reset"))
		} else {
			msg = fmt.Sprintf("\n%s[Tool: %s]%s\n", colors.GetColor("cyan"), tc.Name, colors.GetColor("reset"))
		}
	}

	if callback != nil {
		callback(inference.StreamingChunk{
			Text:        msg,
			ContentType: inference.StreamingContentTypeNormal,
		})
	} else if msg != "" {
		fmt.Print(msg)
	}
}

// formatFullToolParams formats tool parameters as a readable string for display.
func formatFullToolParams(params map[string]interface{}) string {
	if len(params) == 0 {
		return ""
	}

	var parts []string
	for key, value := range params {
		parts = append(parts, fmt.Sprintf("%s: %s", key, formatParamValue(value)))
	}
	return strings.Join(parts, ", ")
}

// formatParamValue formats a single parameter value for display.
func formatParamValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%.1f", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case nil:
		return "null"
	case map[string]interface{}:
		return formatFullToolParams(v)
	case []interface{}:
		var items []string
		for _, item := range v {
			items = append(items, formatParamValue(item))
		}
		return "[" + strings.Join(items, ", ") + "]"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// streamResult streams a tool result status message with color.
// If callback is nil, prints to stdout instead.
func streamResult(toolName string, result *tools.ToolResult, callback StreamCallback) {
	status := formatToolStatus(toolName, result)
	if callback != nil {
		callback(inference.StreamingChunk{
			Text:        status,
			ContentType: inference.StreamingContentTypeNormal,
		})
	} else {
		fmt.Print(status)
	}
}

// formatToolStatus formats a tool status message for display with colors.
func formatToolStatus(toolName string, result *tools.ToolResult) string {
	if result.Success {
		switch toolName {
		case "bash":
			// Show exit code and truncated output (tail end is more useful for bash commands)
			output := result.Output
			lines := strings.Split(output, "\n")
			if len(lines) > 5 {
				lines = lines[len(lines)-5:]
				output = "... [output truncated]\n" + strings.Join(lines, "\n")
			}
			exitCode := ""
			if result.ExitCode != 0 {
				exitCode = fmt.Sprintf(" (exit code: %d)", result.ExitCode)
			}
			return fmt.Sprintf("%s[Success] bash completed%s\nOutput:\n%s%s\n", colors.GetColor("green"), exitCode, output, colors.GetColor("reset"))
		case "read_file":
			output := result.Output
			lines := strings.Split(output, "\n")
			if len(lines) > 10 {
				lines = lines[:10]
				output = strings.Join(lines, "\n") + "\n... [content truncated]"
			}
			// Use accurate line count from Extra if available
			linesRead := len(strings.Split(result.Output, "\n"))
			if result.Extra != nil {
				if lr, ok := result.Extra["linesRead"].(int); ok {
					linesRead = lr
				}
			}
			return fmt.Sprintf("%s[Success] read %d lines\nContent:\n%s%s\n", colors.GetColor("green"), linesRead, output, colors.GetColor("reset"))
		case "write_file":
			// Show the file path, size, and truncated content preview
			output := result.Output
			// Extract path and content length from Extra if available
			if result.Extra != nil {
				if msg, ok := result.Extra["message"].(string); ok && msg != "" {
					output = msg
				}
				if contentLen, ok := result.Extra["contentLength"].(int); ok && contentLen > 0 {
					output += fmt.Sprintf(" (%d bytes)", contentLen)
				}
			}
			return fmt.Sprintf("%s[Success] %s%s\n", colors.GetColor("green"), output, colors.GetColor("reset"))
		case "read_lines":
			// Show the lines that were read, truncated if too long
			output := result.Output
			lines := strings.Split(output, "\n")
			linesRead := len(lines)
			if linesRead > 10 {
				lines = lines[:10]
				output = strings.Join(lines, "\n") + "\n... [output truncated]"
				linesRead = len(lines)
			}
			return fmt.Sprintf("%s[Success] read %d lines\nContent:\n%s%s\n", colors.GetColor("green"), linesRead, output, colors.GetColor("reset"))
		case "insert_lines":
			// Show the full output including path, line count, and content preview
			output := result.Output
			return fmt.Sprintf("%s[Success] %s%s\n", colors.GetColor("green"), output, colors.GetColor("reset"))
		case "replace_text":
			// Show the full output including search, replace, count, and preview
			output := result.Output
			return fmt.Sprintf("%s[Success] %s%s\n", colors.GetColor("green"), output, colors.GetColor("reset"))
		case "move_text":
			// Show the move_text result
			output := result.Output
			return fmt.Sprintf("%s[Success] %s%s\n", colors.GetColor("green"), output, colors.GetColor("reset"))
		case "list_files":
			// Show the actual file listing with path and count
			output := result.Output

			entries := 0
			if e, ok := result.Extra["entriesListed"].(int); ok {
				entries = e
			}
			// Truncate output if too long
			output = tools.TruncateOutputByLen(output, tools.MaxDisplayOutput, "listing truncated")
			return fmt.Sprintf("%s[Success] listed %d entries%s\n%s\n", colors.GetColor("green"), entries, colors.GetColor("reset"), output)
		case "grep":
			// Show the actual grep results with match count
			output := result.Output
			matches := 0
			if m, ok := result.Extra["matchesFound"].(int); ok {
				matches = m
			}
			// Truncate output if too long
			output = tools.TruncateOutputByLen(output, tools.MaxToolOutput, "output truncated")
			if matches == 0 {
				return fmt.Sprintf("%s[Success] grep: 0 matches found%s\n", colors.GetColor("green"), colors.GetColor("reset"))
			}
			return fmt.Sprintf("%s[Success] grep: %d matches found\n%s%s\n", colors.GetColor("green"), matches, output, colors.GetColor("reset"))
		case "git_log":
			// Show the git log output with summary info
			output := result.Output
			reference := "HEAD"
			if r, ok := result.Extra["reference"].(string); ok && r != "" {
				reference = r
			}
			count := 0
			if c, ok := result.Extra["count"].(int); ok {
				count = c
			}
			// Truncate output if too long
			output = tools.TruncateOutputByLen(output, tools.MaxToolOutput, "log truncated")
			return fmt.Sprintf("%s[Success] git log: %d commits from %s\n%s%s\n", colors.GetColor("green"), count, reference, output, colors.GetColor("reset"))
		case "git_show":
			// Show the git show output with commit details
			output := result.Output
			commit := "HEAD"
			if c, ok := result.Extra["commitReference"].(string); ok && c != "" {
				commit = c
			}
			// Truncate output if too long
			output = tools.TruncateOutputByLen(output, tools.MaxToolOutput, "output truncated")
			return fmt.Sprintf("%s[Success] git show %s\n%s%s\n", colors.GetColor("green"), commit, output, colors.GetColor("reset"))
		case "git_diff":
			// Show the git diff output with summary info
			output := result.Output
			ref1 := ""
			if r, ok := result.Extra["reference1"].(string); ok && r != "" {
				ref1 = r
			}
			ref2 := ""
			if r, ok := result.Extra["reference2"].(string); ok && r != "" {
				ref2 = r
			}
			// Truncate output if too long
			output = tools.TruncateOutputByLen(output, tools.MaxToolOutput, "diff truncated")
			msg := fmt.Sprintf("%s[Success] git diff", colors.GetColor("green"))
			if ref1 != "" {
				msg += fmt.Sprintf(" %s", ref1)
			}
			if ref2 != "" {
				msg += fmt.Sprintf(" %s", ref2)
			}
			msg += "\n" + output + colors.GetColor("reset")
			return msg
		case "subagent":
			// Show subagent success with clear visual separation
			return fmt.Sprintf("%s[Subagent] Task completed\nOutput:\n%s%s\n", colors.GetColor("cyan"), result.Output, colors.GetColor("reset"))
		default:
			return fmt.Sprintf("%s[Success] tool completed%s\n", colors.GetColor("green"), colors.GetColor("reset"))
		}
	}
	failureName := toolName
	if toolName == "subagent" {
		failureName = "Subagent"
	}
	return fmt.Sprintf("%s[Failed] %s\nError: %s%s\n", colors.GetColor("red"), failureName, result.Error, colors.GetColor("reset"))
}
