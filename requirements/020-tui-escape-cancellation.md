# Requirement 020: TUI Escape Key Cancellation

## Description
The TUI must support cancellation of ongoing operations (such as inference requests or tool execution) using the Escape key. This allows users to interrupt long-running or unwanted operations gracefully.

## Acceptance Criteria
- [ ] Escape key cancels ongoing inference requests
- [ ] Escape key cancels ongoing tool execution when possible
- [ ] Cancellation is graceful (no crashes or data corruption)
- [ ] User is notified when operation is cancelled
- [ ] Partial results are handled appropriately
- [ ] Context state is preserved after cancellation
- [ ] Cancellation does not affect previous conversation history
- [ ] Visual feedback during cancellation (e.g., "Cancelled")
- [ ] Cancellation can be triggered at any time during operation
- [ ] Multiple Escape presses do not cause errors

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

### Escape Key Handling

```
User presses Escape during operation:
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
^C or ESC
Request cancelled.
> 
```

### Edge Cases

| Scenario | Behavior |
|----------|----------|
| Press Escape at prompt | No-op, stay at prompt |
| Press Escape during typing | No-op, continue typing |
| Press Escape during inference | Cancel request, return to prompt |
| Press Escape during tool call | Cancel tool, return to prompt |
| Press Escape multiple times | Single cancellation, ignore extras |
| Network already closed | Display "Request cancelled", no error |

### Context State Preservation

After cancellation:
- Previous messages remain in context
- No incomplete messages added
- User can continue conversation normally
- Statistics remain accurate (no partial counts)

### Timeout Integration

- Escape key works alongside timeout mechanisms
- If timeout occurs first, Escape has no effect
- If Escape pressed first, timeout is ignored

## User Experience

1. User submits long-running request
2. User changes mind or realizes mistake
3. User presses Escape
4. System displays "Cancelled" message
5. User sees prompt again immediately
6. User can try different approach

## Security Considerations

- Cancellation should not expose partial sensitive data
- Clean up any temporary resources on cancellation
- Ensure no orphaned processes after cancellation
- Log cancellations for debugging (without sensitive data)

## Testing Requirements

- Test Escape during inference (streaming and non-streaming)
- Test Escape during tool execution
- Test Escape at prompt (no-op)
- Test multiple Escape presses
- Test context state after cancellation
- Test statistics after cancellation
- Test with slow network (cancellation should still work)
