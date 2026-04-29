# Requirement 022: No User Input Echo

## Description
The TUI must NOT echo back the user's input after submission. Since the user can already see what they typed on the screen, echoing it back creates unnecessary clutter and redundancy in the output display.

## Acceptance Criteria
- [ ] User input is NOT echoed back after submission
- [ ] User input is still added to conversation context for LLM
- [ ] Only assistant responses and tool feedback are displayed after user input
- [ ] History navigation works without echoing
- [ ] Internal processing (context integration) is not affected

## Design Rationale

User input is not echoed after submission to keep the display clean and avoid redundancy. The user can already see what they typed on the input line, so repeating it in the output adds no value.

**What is displayed after user input:**

- Assistant responses (streamed in real-time)
- Tool call feedback ("Calling tool: name (params)")
- Tool execution results (success/error messages)
- Context size indicator
- Statistics (when requested)

## Example Flow

```
[Tokens: 45 / 128000 (0.0%)]
> List files in /tmp

[Assistant] I'll list the files...

Calling tool: bash (command: "ls /tmp")
...
```

## Implementation Notes

### Context vs Display
- **Context**: User input MUST be added to conversation context (LLM needs to see it)
- **Display**: User input must NOT be echoed to TUI output (redundant)

The input is stored for LLM context and history navigation but is never re-displayed in the output area:

```go
// Add to context for the LLM
ctx.AddUserMessage(input)

// Add to history for arrow-key navigation
tui.AddToHistory(input)

// Do NOT echo user input to the output display
```

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
