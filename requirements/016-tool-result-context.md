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
- [ ] **Streaming mode**: Tool calls are accumulated from partial deltas before execution
- [ ] **Streaming mode**: Tool execution waits until all tool call data is received
- [ ] **Non-streaming mode**: Tool call feedback is displayed to the user (stdout)
- [ ] **Streaming mode**: Tool call feedback is streamed to the callback
- [ ] The Message struct has a `ToolCalls` field to store tool calls from the API
- [ ] Assistant messages containing tool calls include the `tool_calls` field when serialized and sent to the API
- [ ] Assistant messages added to context preserve the `tool_calls` from the API response

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

### Sending Messages to API

After executing a tool, send the results back to continue the conversation. Both assistant messages (with tool calls) and tool result messages must be included in the context:

```go
// Build assistant message with tool calls for context
assistantMessage := &Message{
    Role:       "assistant",
    Content:    response.Content,
    ToolCalls:  response.APIToolCalls,  // Include tool calls from API
}

// Build tool message for context
toolMessage := &Message{
    Role:       "tool",
    Content:    resultMessage,
    ToolCallId: tc.ID,  // Include tool_call_id for matching
}
```

**Important:** The `Message` struct must include a `ToolCalls` field to store tool calls. When serializing messages to send to the API, assistant messages containing tool calls must include the `tool_calls` JSON field. This ensures the LLM can see which tool calls were made in previous turns.

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

## Streaming Mode Handling

### Tool Call Accumulation

In streaming mode, tool calls arrive as partial deltas that need to be accumulated before execution:

1. **Partial Tool Call Deltas**: The API sends incremental updates:
   - Each chunk may contain partial `tool_calls` data
   - Tool call ID is consistent across chunks
   - Function name and arguments are appended incrementally

2. **Accumulation Logic**:
   - Use a map keyed by tool call ID to merge partial data
   - Append function name from first chunk that contains it
   - Append function arguments from each subsequent chunk
   - Wait for `[DONE]` marker before processing tool calls

3. **Execution Timing**:
   - Tool calls are only executed after the full stream is complete
   - This ensures all arguments are received before execution
   - Prevents partial/incomplete tool call execution

### Example Streaming Flow

```
Chunk 1: {delta: {tool_calls: [{id: "call_1", function: {name: "bash"}}]}}
Chunk 2: {delta: {tool_calls: [{id: "call_1", function: {arguments: "{\"command\""}}]}}
Chunk 3: {delta: {tool_calls: [{id: "call_1", function: {arguments: ":\"ls -la\"}"}}]}}
[DONE]

Accumulated: {id: "call_1", function: {name: "bash", arguments: "{\"command\":\"ls -la\"}"}}
```

## User Feedback

### Streaming Mode (`--stream`)

- Tool call status is sent via stream callback as each chunk arrives
- Tool execution status is streamed immediately after tool completes
- User sees real-time feedback: "[Running] bash: ls -la" → "[Success] bash completed"

### Non-Streaming Mode (`--no-stream`)

- Tool call status is printed to stdout before execution
- Tool result status is printed to stdout after execution
- User sees feedback: "[Running] bash: ls -la" → "[Success] bash completed"

### Feedback Format

**Tool Call Start:**
```
[Running] bash: ls -la
[Reading] file: /path/to/file.txt
[Writing] file: /path/to/file.txt
[Replacing] 'oldVar' in: /path/to/file.txt
```

**Tool Result:**
```
[Success] bash completed
Output:
total 24
drwxrwxrwt 1 user user 4096 ...

[Success] read 10 lines
Content:
1: line 1
2: line 2
...

[Success] replaced 'oldVar' 1 time(s)
```

## Security Considerations

- Tool results may contain sensitive data
- AI should not expose sensitive information
- Tool execution sandboxing recommended for production
- Validate tool result content before adding to context
- Consider rate limiting on tool call frequency
