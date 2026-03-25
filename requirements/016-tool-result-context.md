# Requirement 016: Tool Result Context Integration

## Description
When a tool is called and executed, the result of the tool call must be added back into the conversation context. This allows the AI to see the tool output and iterate further based on the results.

## Acceptance Criteria
- [ ] Tool call results are automatically added to the conversation context
- [ ] Results are added as a user message after tool execution
- [ ] AI can see and respond to tool results in subsequent turns
- [ ] Multiple tool calls can be chained (AI calls tool -> sees result -> calls another tool)
- [ ] Tool results include both success output and error messages
- [ ] Tool result format is clear and machine-readable
- [ ] Tool results preserve the original tool call for reference
- [ ] Context size is monitored after adding tool results
- [ ] Iteration continues until task is complete or AI determines no more tool calls needed
- [ ] Maximum iteration limit is enforced to prevent infinite loops

## Tool Result Format

### Success Case
```
Tool 'tool_name' executed successfully:
{
  "result_key": "result_value",
  "another_key": 123
}
```

### Error Case
```
Tool 'tool_name' failed: error message
```

### Example Flow

**User:** "List the files in /tmp and save the listing to /tmp/files.txt"

**AI:** "I'll list the files in /tmp first using the bash tool."
```
[tool:bash(command="ls -la /tmp")]
```

**Tool Result (added to context):**
```
Tool 'bash' executed successfully:
{
  "output": "total 24\ndrwxrwxrwt 1 user user 4096 ...",
  "error": ""
}
```

**AI (seeing the result):** "The /tmp directory contains several files. Now I'll save this listing to a file."
```
[tool:write_file(path="/tmp/files.txt", content="total 24\ndrwxrwxrwt 1 user user...")]
```

**Tool Result:**
```
Tool 'write_file' executed successfully:
{
  "success": true,
  "path": "/tmp/files.txt"
}
```

**AI:** "I have successfully saved the file listing to /tmp/files.txt. The task is complete."

## Context Integration Details

### Message Structure
After a tool call, the context contains:
```
[SYSTEM PROMPT - with all tools]
[User: Original request]
[Assistant: "I'll do X" + tool call]
[User: "Tool 'X' result: {...}"]  <-- Added automatically
[Assistant: Next response based on result]
```

### Iteration Process
1. AI detects need for tool
2. AI generates tool call in message
3. System executes tool
4. System adds result as user message
5. AI receives result and can:
   - Continue with next tool call
   - Provide final answer
   - Ask for clarification
   - Report error or completion

### Maximum Iteration Limit
- Default maximum: 50 tool call iterations
- Prevents infinite loops
- Configurable via environment variable
- Exceeding limit triggers error and final response

## Implementation Notes

### Tool Result Message
The tool result message should:
- Clearly indicate which tool was called
- Show success or failure status
- Include the full result JSON or error message
- Be formatted for AI readability

### Context Size Management
- After adding tool results, check if context exceeds limit
- If exceeded, trigger compression
- Ensure tool results are preserved during compression

### Iteration Tracking
- Count tool calls within a single conversation session
- Reset counter on context clear
- Display iteration count in statistics
- Warn when approaching maximum iterations

## Security Considerations

- Tool results may contain sensitive data
- AI should not expose sensitive information
- Tool execution sandboxing recommended for production
- Validate tool result content before adding to context
- Consider rate limiting on tool call frequency
