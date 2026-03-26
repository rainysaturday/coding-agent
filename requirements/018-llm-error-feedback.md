# Requirement 018: LLM Error Feedback for Failed Tool Calls

## Description
When a tool call fails, the error must be reported back to the LLM in a clear, actionable format. This allows the LLM to understand what went wrong and potentially retry with corrected parameters or take alternative action.

## Acceptance Criteria
- [ ] Failed tool calls return detailed error messages to the LLM
- [ ] Error messages include the original tool call for reference
- [ ] Error messages are descriptive and actionable
- [ ] Error format is consistent across all tool types
- [ ] LLM can see the error in subsequent context
- [ ] Error messages include relevant context (file paths, command, etc.)
- [ ] System-level errors are translated to user-friendly messages
- [ ] LLM can distinguish between different error types
- [ ] Failed tool calls are tracked in statistics
- [ ] Multiple failures do not crash the agent

## Implementation Details

### Error Message Format

All tool errors must follow this format when added to context:
```
Tool 'tool_name' failed: error_message
```

Where `error_message` is descriptive and includes:
- What operation failed
- What parameters were used
- Why it failed (if known)
- Possible remediation steps (if applicable)

### Error Categories

Each tool should provide errors in these categories:

**Missing Parameters**
```
Tool 'read_file' failed: missing required parameter: path
```

**File Not Found**
```
Tool 'read_file' failed: file not found: /nonexistent/path/file.txt
```

**Permission Errors**
```
Tool 'write_file' failed: permission denied: cannot write to /root/protected/file.txt
```

**Invalid Input**
```
Tool 'read_lines' failed: invalid start parameter: must be a positive integer
```

**Command Execution Errors**
```
Tool 'bash' failed: command not found: nonexistent_command
```

**Disk Errors**
```
Tool 'write_file' failed: disk full: cannot write to /partition/low_space/file.txt
```

### Context Integration

When a tool fails, the error is added to the conversation context as a user message:
```
[SYSTEM PROMPT]
[User: Original request]
[Assistant: "I'll do X" + tool call]
[User: "Tool 'X' failed: error message"]  <-- Error added here
[Assistant: Can now respond to the error]
```

### LLM Response Handling

The LLM should be able to:
1. See the error message
2. Understand what went wrong
3. Decide to retry with corrected parameters
4. Or take alternative action
5. Or report failure to the user

### Example Flow with Error Recovery

**User**: "Read the file /etc/passwd"

**AI**: "Let me try to read that file."
```
[tool:read_file(path="/etc/passwd")]
```

**Tool Result**:
```
Tool 'read_file' failed: permission denied: cannot read /etc/passwd
```

**AI** (seeing the error): "I don't have permission to read that file. Let me try a different approach."
```
[tool:bash(command="ls -la /etc/passwd")]
```

**Tool Result**:
```
Tool 'bash' executed successfully:
-rw-r--r-- 1 root root 2000 ... /etc/passwd
```

**AI**: "The file exists but requires root permissions to read. You'll need elevated privileges to access this file."

### Error Message Guidelines

1. **Be Specific**: Include the actual path, command, or parameter that caused the issue
2. **Be Actionable**: Suggest what might fix the issue when possible
3. **Be Honest**: Don't hide the real error, but don't expose sensitive information
4. **Be Consistent**: Use similar error formats across all tools
5. **Be Complete**: Include enough context for the LLM to make decisions

### Error Translation

System errors should be translated to more readable formats:

| System Error | Translated Error |
|--------------|------------------|
| `ENOENT` | `file not found: <path>` |
| `EACCES` | `permission denied: <path>` |
| `ENOSPC` | `disk full: <path>` |
| `EINVAL` | `invalid parameter: <param>` |
| `ENOEXEC` | `cannot execute: <command>` |

### Statistics Tracking

Failed tool calls must be tracked:
- Total number of failed tool calls
- Failed calls per tool type
- Failed calls displayed in statistics summary

### Maximum Error Attempts

To prevent infinite retry loops:
- Track consecutive failures for the same tool
- After N failures (configurable), warn the LLM
- After M failures (configurable), stop retrying and report to user

## Security Considerations

- Do not expose sensitive system paths in error messages
- Sanitize file paths that may contain user input
- Do not reveal internal implementation details
- Consider rate limiting on tool call retries
- Log errors for debugging without exposing to LLM if sensitive

## Testing Requirements

- Test each error category for every tool
- Verify error messages are clear and actionable
- Verify LLM can successfully recover from common errors
- Verify statistics are updated correctly on failure
- Verify no crashes occur from malformed error handling
