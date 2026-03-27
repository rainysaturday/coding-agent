# Requirement 022: No User Input Echo

## Description
The TUI must NOT echo back the user's input after submission. Since the user can already see what they typed on the screen, echoing it back creates unnecessary clutter and redundancy in the output display.

## Acceptance Criteria
- [ ] User input is NOT echoed back after submission
- [ ] User input is still added to conversation context for LLM
- [ ] Only assistant responses and tool feedback are displayed after user input
- [ ] History navigation works without echoing
- [ ] Internal processing (context integration) is not affected

## Rationale

**Why not echo user input:**

1. **Redundancy**: The user just typed the input and can see it on the current line
2. **Clutter**: Echoing creates unnecessary vertical space consumption
3. **UX Best Practice**: Modern terminals and chat interfaces don't repeat user input
4. **Cleaner Display**: More space for meaningful assistant responses and tool output

**What should be displayed instead:**

- Assistant responses (streamed in real-time)
- Tool call feedback ("Calling tool: name (params)")
- Tool execution results (success/error messages)
- Context size indicator
- Statistics (when requested)

## Example Flow

### Current Behavior (BAD)
```
[Tokens: 45 / 128000 (0.0%)]
> List files in /tmp

[User] List files in /tmp

[Assistant] I'll list the files...

Calling tool: bash (command: "ls /tmp")
...
```

### Desired Behavior (GOOD)
```
[Tokens: 45 / 128000 (0.0%)]
> List files in /tmp

[Assistant] I'll list the files...

Calling tool: bash (command: "ls /tmp")
...
```

## Implementation Notes

### What to Remove
```go
// REMOVE this line from main.go:
tui.AddOutputf("[User] %s", input)
```

### What to Keep
```go
// KEEP these (they're necessary):
ctx.AddUserMessage(input)  // Add to context for LLM
tui.AddToHistory(input)    // Add to history for navigation
```

### Context vs Display
- **Context**: User input MUST be added to conversation context (LLM needs to see it)
- **Display**: User input should NOT be echoed to TUI output (redundant)

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| Normal input | No echo, just process |
| History navigation | No echo of recalled input |
| Multi-line input | No echo (if supported) |
| Cancelled input | Show "Cancelled" message only |
| Command input (stats, clear) | No echo, just execute |

## Related Requirements

- **002-tui-input-prompt.md**: Input prompt functionality
- **017-tui-tool-feedback.md**: Tool feedback display (should still show)
- **019-tui-history-navigation.md**: History navigation (no echo of recalled items)
