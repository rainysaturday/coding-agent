# Requirement 020: TUI Ctrl+C Cancellation

## Description
The TUI must support cancellation of ongoing operations (such as inference requests or tool execution) using Ctrl+C. This allows users to interrupt long-running or unwanted operations gracefully.

## Acceptance Criteria
- [ ] Ctrl+C cancels ongoing inference requests
- [ ] Ctrl+C cancels ongoing tool execution when possible
- [ ] Cancellation is graceful (no crashes or data corruption)
- [ ] User is notified when operation is cancelled
- [ ] Partial results are handled appropriately
- [ ] Context state is preserved after cancellation
- [ ] Cancellation does not affect previous conversation history
- [ ] Visual feedback during cancellation (e.g., "Cancelled")
- [ ] Cancellation can be triggered at any time during operation
- [ ] Multiple Ctrl+C presses do not cause errors

## Implementation Details

### Cancellation Behavior

**During Inference:**
- Stop receiving streaming tokens
- Close the HTTP connection if possible
- Display "Request cancelled" message
- Do not add incomplete response to context

**During Tool Execution:**
- Interrupt command execution if possible
- Display "Tool execution cancelled" message
- Do not add partial tool result to context
- Allow user to retry or try different approach

### Ctrl+C Handling

```
User presses Ctrl+C during operation:
1. Set cancellation flag
2. Check flag in operation loop
3. Clean up resources (close connections, etc.)
4. Display cancellation message
5. Return to prompt
```

### Graceful Shutdown

- Current message is not added to context
- Statistics are not updated for cancelled operations
- No partial tool results in context
- User can immediately type new prompt

### Visual Feedback

```
[Assistant] I'll help you with that...
^C
Request cancelled.
> 
```

### Edge Cases

| Scenario | Behavior |
|----------|----------|
| Press Ctrl+C at prompt | No-op, stay at prompt |
| Press Ctrl+C during typing | Cancel input, return to prompt |
| Press Ctrl+C during inference | Cancel request, return to prompt |
| Press Ctrl+C during tool call | Cancel tool, return to prompt |
| Press Ctrl+C multiple times | Single cancellation, ignore extras |
| Network already closed | Display "Request cancelled", no error |

### Context State Preservation

After cancellation:
- Previous messages remain in context
- No incomplete messages added
- User can continue conversation normally
- Statistics remain accurate (no partial counts)

### Timeout Integration

- Ctrl+C works alongside timeout mechanisms
- If timeout occurs first, Ctrl+C has no effect
- If Ctrl+C pressed first, timeout is ignored

## User Experience

1. User submits long-running request
2. User changes mind or realizes mistake
3. User presses Ctrl+C
4. System displays "Cancelled" message
5. User sees prompt again immediately
6. User can try different approach

## Security Considerations

- Cancellation should not expose partial sensitive data
- Clean up any temporary resources on cancellation
- Ensure no orphaned processes after cancellation
- Log cancellations for debugging (without sensitive data)

## Testing Requirements

- Test Ctrl+C during inference (streaming and non-streaming)
- Test Ctrl+C during tool execution
- Test Ctrl+C at prompt (no-op)
- Test multiple Ctrl+C presses
- Test context state after cancellation
- Test statistics after cancellation
- Test with slow network (cancellation should still work)
