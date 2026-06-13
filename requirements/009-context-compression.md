# Requirement 009: Context Compression

## Description
When the conversation context grows beyond the configured context size limit, the harness must automatically compress the context by summarizing the conversation history. The compression must preserve the absolute first user prompt and place the summary as an assistant message to maintain conversational integrity.

## Acceptance Criteria
- [ ] Context size is monitored during conversation
- [ ] Compression triggered when context exceeds configured limit (80% of max context size)
- [ ] Compression requests summarization from the inference engine
- [ ] System prompt is preserved (injected by the inference client on every API call)
- [ ] The absolute first user message (original prompt) is always preserved intact
- [ ] The compressed summary is stored as an **assistant** message (role: "assistant")
- [ ] The last 3 messages are preserved after compression
- [ ] After compression, the context sequence is: `[first_user_message, assistant_summary, ...last_3_messages...]`
- [ ] Compression happens transparently to the user
- [ ] Compression success/failure is logged
- [ ] Failed compression does not crash the agent
- [ ] Compression reduces context size below the limit
- [ ] Tool messages are grouped with their preceding assistant messages to maintain causality

## Compression Algorithm

### Before Compression

```
[system_prompt]
[user: "Original user prompt"]
[assistant: "First response"]
[tool: "Tool result 1"]
[assistant: "Second response"]
[tool: "Tool result 2"]
[... many more messages ...]
[user: "Recent user input"]
[assistant: "Recent response with tool calls"]
[tool: "Most recent tool result"]
```

### After Compression

```
[system_prompt]
[user: "Original user prompt"]                         <-- preserved intact
[assistant: "Conversation summary: ..."]                <-- summary as assistant message
[user: "Recent user input"]                            <-- from last 3 messages
[assistant: "Recent response with tool calls"]         <-- from last 3 messages
[tool: "Most recent tool result"]                      <-- from last 3 messages
```

### Key Rules

1. **First user message**: The absolute first user message in the context must always be preserved verbatim. This is the original prompt that initiated the conversation.

2. **Summary as assistant message**: The compressed summary must be an assistant message, not a user message. This maintains the conversational role sequence (user -> assistant -> ...).

3. **Last 3 messages preserved**: The last 3 messages from before compression are appended after the summary, preserving the most recent conversation state.

4. **Tool message causality**: Tool messages are only preserved if their corresponding assistant message (the one that made the tool call) is also in the preserved set. Orphaned tool messages are discarded.

5. **System prompt**: The system prompt is NOT stored in the context array. It is injected by the inference client on every API call.

## Summary Request Format

The compression request sent to the LLM should include:
- All messages between the first user message and the last 3 preserved messages
- A request to produce a concise summary preserving key information, decisions, and results

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Inference call fails | Log error, continue without compression |
| Summary is empty | Treat as failure, skip compression |
| Not enough messages to compress | Skip compression silently |

## Implementation Notes

- The summary message content should be prefixed with "Conversation summary: " to clearly indicate it is a compressed representation
- After compression, the token tracking baseline (`lastTotalTokens`) is reset so the context size is recalculated on the next API call
- Cumulative session statistics (total input/output tokens) are preserved and not overwritten by compression

## Testing Requirements

- [ ] Compression preserves the first user message verbatim
- [ ] Summary is stored as an assistant message
- [ ] Last 3 messages are preserved after compression
- [ ] Context sequence after compression follows the expected pattern
- [ ] Tool messages are properly grouped with assistant messages
- [ ] Compression with minimal messages is handled gracefully
- [ ] Failed compression does not crash the agent

## Related Requirements
- **044-context-dump-load.md**: Context dump includes compressed history
