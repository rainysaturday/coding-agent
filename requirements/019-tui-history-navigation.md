# Requirement 019: TUI History Navigation

## Description
The TUI must support history navigation using up and down arrow keys to recall previously entered prompts. This makes it easier for users to resend, modify, or reference previous inputs without retyping.

## Acceptance Criteria
- [ ] Up arrow key displays the previous prompt from history
- [ ] Down arrow key displays the next prompt from history (when navigating backwards)
- [ ] History is maintained in chronological order (newest first for up navigation)
- [ ] Empty history does not cause errors or crashes
- [ ] Current input is preserved when navigating history
- [ ] History navigation works seamlessly with text editing
- [ ] Clear visual indication of history navigation (cursor position maintained)
- [ ] History persists for the duration of the session
- [ ] Maximum history size is configurable to prevent excessive memory usage
- [ ] History can be cleared via command (e.g., `clear-history`)

## Implementation Details

### Navigation Behavior

**Up Arrow Key:**
- Moves to the previous prompt in history
- At the beginning of history, no further navigation (stay on current input)
- Preserves current cursor position in the input text

**Down Arrow Key:**
- Moves to the next prompt in history
- At the end of history (most recent), clears to empty input line
- Allows returning to typing a new prompt

### History Storage

```
History (newest first):
[0] "Most recent prompt"
[1] "Second most recent prompt"
[2] "Third most recent prompt"
...
```

When user presses:
- **Up**: Index increments (0 -> 1 -> 2)
- **Down**: Index decrements (2 -> 1 -> 0 -> empty)

### Maximum History Size

- Default: 100 entries
- Configurable via environment variable or config file
- Oldest entries discarded when limit exceeded

### Edge Cases

| Scenario | Behavior |
|----------|----------|
| Empty history | No-op, no error |
| At beginning of history | No-op, stay on current |
| At end of history | Clear input to empty |
| Editing then up | Save current, load previous |
| Empty input + up | Load previous (don't store empty) |

### User Experience

1. User types prompt: "List files in /tmp"
2. User submits, gets response
3. User presses ↑: "List files in /tmp" reappears in input
4. User can modify: "List files in /home instead"
5. User submits new version
6. User presses ↑: "List files in /home instead" reappears
7. User presses ↑: "List files in /tmp" reappears
8. User presses ↓: "List files in /home instead" reappears
9. User presses ↓: Empty input line (ready for new prompt)

### Command Support

- `clear-history`: Clears the input history
- History is separate from conversation context
- History is session-only (not persisted to disk)

## Security Considerations

- Do not store sensitive information (passwords, API keys) in history
- Consider warning users about sensitive data in prompts
- History is cleared on exit (no disk persistence)
- Consider option to disable history for security-sensitive use

## Testing Requirements

- Test up/down navigation with various history sizes
- Test navigation at boundaries (beginning/end of history)
- Test editing behavior during navigation
- Test with empty history
- Test with maximum history size
- Test that current input is preserved when navigating
