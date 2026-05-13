# Requirement 017: TUI Tool Call Feedback

## Description
The coding agent harness must provide brief but relevant feedback to the user in the TUI when tools are called and executed. This ensures the user understands what actions are being taken and their outcomes.

## Acceptance Criteria
- [ ] Tool call initiation is displayed in the TUI before execution
- [ ] Tool call display includes tool name and parameters
- [ ] **Tool parameters are displayed in real-time as they are streamed from the LLM**
- [ ] Tool execution results are displayed in the TUI after completion
- [ ] Success messages are brief and informative (e.g., "File written successfully")
- [ ] Tool output is displayed in a readable format (truncated if too long)
- [ ] Error messages are displayed prominently for failed tool calls
- [ ] Multiple tool calls in a sequence are clearly separated in display
- [ ] Tool feedback does not overwhelm the TUI with excessive output
- [ ] Long outputs are truncated with indication that content was truncated
- [ ] Users can see which tool was called and what it did at a glance

## Implementation Details

### Tool Call Display Format (Streaming)

When a tool is being called, the TUI displays the tool name and parameters **as they are streamed in from the LLM**. The format is:

```
[Tool Call] tool_name (parameter: "value", another_param: "value")
```

**Important**: Parameters should be displayed in real-time as they arrive during streaming. The parameters are shown in a flat, single-line format (not pretty-printed JSON) to keep the display clean. The line updates **in-place** using ANSI cursor positioning (clear line + carriage return) so the user sees a single evolving line rather than multiple appended lines.

### Streaming Implementation Notes

The tool call parameters are streamed in real-time from the LLM response. The display should:

1. Show the tool name when first detected
2. Continuously update **in place on the same line** to show accumulating parameters as they arrive
3. Format parameters in a **flat, single-line format** (e.g., `command: "ls -la", path: "/tmp"`) rather than pretty-printed multi-line JSON
4. Use ANSI escape sequences (`\033[2K\r`) to clear and re-print the line for in-place updates
5. **Truncate long parameter values** to fit within the terminal width, with a "(N chars)" suffix indicating the original length
6. **Ensure total argument display stays within terminal width** - if multiple parameters would exceed the limit, values are truncated progressively
7. Handle multiple tool calls in sequence correctly

### Success Result Display Format

For successful tool execution:
```
Tool 'tool_name' executed successfully:
{brief result summary or output}
```

Examples:
```
Tool 'bash' executed successfully:
total 24
drwxrwxrwt 1 user user 4096 ...

Tool 'write_file' executed successfully:
File written to /path/to/file.txt
```

### Long Output Handling

If tool output exceeds a reasonable display length (e.g., 500 characters):
```
Tool 'bash' executed successfully:
[Output truncated - 1234 characters]
First 10 lines shown:
line1
line2
...
```

### Error Result Display Format

For failed tool execution:
```
Tool 'tool_name' failed: error message
```

Examples:
```
Tool 'read_file' failed: file not found: /nonexistent/file.txt
Tool 'bash' failed: exit status 1
Tool 'write_file' failed: permission denied
```

### Display Styling

- Tool call messages: Normal text or subtle highlight
- Success messages: Green or neutral color
- Error messages: Red or warning color
- Output content: Monospace or indented for readability

## User Experience Guidelines

1. **Brevity**: Feedback should be concise and to the point
2. **Clarity**: Users should immediately understand what happened
3. **Completeness**: Enough information to diagnose issues
4. **Real-time visibility**: Tool parameters should be visible as they stream in, not only after completion
5. **Non-intrusive**: Feedback should not interrupt user flow
6. **Accessible**: All users can see and understand the feedback

## Example Flow

**User Request**: "List the files in /tmp and save to /tmp/list.txt"

**TUI Display**:
```
> List the files in /tmp and save to /tmp/list.txt

[User] List the files in /tmp and save to /tmp/list.txt

[Assistant] I'll list the files in /tmp first.

[Tool Call] bash (command: "ls -la /tmp")

Tool 'bash' executed successfully:
total 24
drwxrwxrwt 1 user user 4096 ...
-rw-rw-r-- 1 user user 123 test.txt

[Assistant] Now I'll save this listing to a file.

[Tool Call] write_file (path: "/tmp/list.txt", content: "...")

Tool 'write_file' executed successfully:
File written to /tmp/list.txt

[Assistant] I've listed the files in /tmp and saved the output to /tmp/list.txt.
```

## Streaming Implementation Notes

The tool call parameters are streamed in real-time from the LLM response. The display should:

1. Show the tool name when first detected
2. Continuously update **in place on the same line** to show accumulating parameters as they arrive
3. Format parameters in a **flat, single-line format** (e.g., `command: "ls -la", path: "/tmp"`) rather than pretty-printed multi-line JSON
4. Use ANSI escape sequences (`\033[2K\r`) to clear and re-print the line for in-place updates
5. Truncate very long parameter values to prevent terminal flooding
6. Handle multiple tool calls in sequence correctly

## Security Considerations

- Do not display sensitive information in tool feedback
- Redact API keys, passwords, or tokens from output
- Consider output length to prevent terminal flooding
- Sanitize any user-provided content in tool parameters
