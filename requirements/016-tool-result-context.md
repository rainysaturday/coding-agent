# Requirement 016: Tool Result Context Integration

## Description
When a tool is called and executed, the result of the tool call must be added back into the conversation context. This allows the AI to see the tool output and iterate further based on the results. With OpenAI's tool calling API, tool results are sent back as `tool` role messages.

## Acceptance Criteria
- [ ] Tool call results are automatically added to the conversation context
- [ ] Results are added as tool messages after tool execution (OpenAI format)
- [ ] AI can see and respond to tool results in subsequent turns
- [ ] Multiple tool calls can be chained (AI calls tool -> sees result -> calls another tool)
- [ ] Tool results include both success output and error messages
- [ ] Tool result format follows OpenAI's tool message specification
- [ ] Tool results preserve the original tool call ID for reference
- [ ] Context size is monitored after adding tool results
- [ ] Iteration continues until task is complete or AI determines no more tool calls needed
- [ ] Maximum iteration limit is enforced to prevent infinite loops

## Tool Result Format (OpenAI)

### Success Case
```json
{
  "role": "tool",
  "tool_call_id": "call_abc123",
  "content": "Tool 'tool_name' executed successfully:\n{...output...}"
}
```

### Error Case
```json
{
  "role": "tool",
  "tool_call_id": "call_abc123",
  "content": "Tool 'tool_name' failed: error message"
}
```

### Example Flow

**User:** "List the files in /tmp and save the listing to /tmp/files.txt"

**AI:** "I'll list the files in /tmp first using the bash tool."

**OpenAI Tool Call:**
```json
{
  "id": "call_abc123",
  "type": "function",
  "function": {
    "name": "bash",
    "arguments": "{\"command\":\"ls -la /tmp\"}"
  }
}
```

**Tool Result (added to context as tool message):**
```json
{
  "role": "tool",
  "tool_call_id": "call_abc123",
  "content": "Tool 'bash' executed successfully:\ntotal 24\ndrwxrwxrwt 1 user user 4096 ..."
}
```

**AI (seeing the result):** "The /tmp directory contains several files. Now I'll save this listing to a file."

**OpenAI Tool Call:**
```json
{
  "id": "call_def456",
  "type": "function",
  "function": {
    "name": "write_file",
    "arguments": "{\"path\":\"/tmp/files.txt\",\"content\":\"total 24\\ndrwxrwxrwt 1 user user 4096...\"}"
  }
}
```

**Tool Result:**
```json
{
  "role": "tool",
  "tool_call_id": "call_def456",
  "content": "Tool 'write_file' executed successfully:\nFile written successfully"
}
```

**AI:** "I have successfully saved the file listing to /tmp/files.txt. The task is complete."

## Context Integration Details

### Message Structure
After a tool call, the context contains:
```
[SYSTEM PROMPT - with tool descriptions]
[User: Original request]
[Assistant: "I'll do X" + tool_calls in message]
[Tool: "Tool 'X' result: {...}"]  <-- Added automatically with tool_call_id
[Assistant: Next response based on result]
```

### Iteration Process
1. AI detects need for tool
2. AI generates tool call via OpenAI API's tool calling
3. System executes tool
4. System adds result as tool message with matching tool_call_id
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
- Include the original tool_call_id for matching
- Show success or failure status
- Include the full result output or error message
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

## OpenAI API Integration

### Sending Tool Results

After executing a tool, send the result back to continue the conversation:

```go
// Build tool message for context
toolMessage := &Message{
    Role:    "tool",
    Content: resultMessage,
}
// Note: tool_call_id is used for API requests, stored in context for reference
```

### API Request Format with Tool Results

```json
{
  "model": "gpt-4o",
  "messages": [
    {"role": "system", "content": "..."},
    {"role": "user", "content": "List files in /tmp"},
    {"role": "assistant", "tool_calls": [...]},
    {"role": "tool", "tool_call_id": "call_abc123", "content": "..."}
  ],
  "tools": [...]
}
```

## Security Considerations

- Tool results may contain sensitive data
- AI should not expose sensitive information
- Tool execution sandboxing recommended for production
- Validate tool result content before adding to context
- Consider rate limiting on tool call frequency
