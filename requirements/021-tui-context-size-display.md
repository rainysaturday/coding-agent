# Requirement 021: TUI Context Size Display

## Description
The current context size (in tokens) must always be visible in the TUI. This allows users to monitor how much conversation history is being maintained and understand when context compression may occur.

## Acceptance Criteria
- [ ] Current context size is displayed in the TUI
- [ ] Context size is shown in tokens
- [ ] Context size updates in real-time as conversation progresses
- [ ] Maximum context size limit is visible or can be shown on demand
- [ ] Context size is displayed prominently but non-intrusively
- [ ] Context size is included in statistics display
- [ ] Warning displayed when approaching context limit
- [ ] Percentage of context used can be shown
- [ ] Context size display does not interfere with main TUI functionality

## Implementation Details

### Display Location

Context size should be displayed in one of these locations:

**Option 1: Status Bar (Bottom)**
```
> Your prompt here
─────────────────────────────────────────────────────────────
Tokens: 2,450 / 128,000 (1.9%) | Tool Calls: 5 | Uptime: 10m
```

**Option 2: Top Right Corner**
```
Tokens: 2,450 / 128,000
─────────────────────────────────────────────────────────────
> Your prompt here
```

**Option 3: Inline with Prompt**
```
[Tokens: 2,450 / 128,000] > Your prompt here
```

### Real-Time Updates

- Update after each message is added
- Update after tool results are processed
- Update after context compression
- No visible flicker or jitter during updates

### Warning Thresholds

| Usage | Display |
|-------|---------|
| < 50% | Normal (green or neutral) |
| 50-75% | Warning (yellow) |
| 75-90% | High (orange) |
| > 90% | Critical (red) |

### Statistics Integration

Context size should be included in the `stats` command output:
```
==================================================
Runtime Statistics
==================================================
Input Tokens:      1,500
Output Tokens:       950
Current Context:   2,450 / 128,000 (1.9%)
Tokens/Second:      12.5
Tool Calls:           5
Failed Calls:         0
Iterations:           3
Uptime:            10m 30s
==================================================
```

### Context Size Estimation

- Use character-based estimation (1 token ≈ 4 characters)
- Include system prompt in count
- Include all user and assistant messages
- Include tool results in count

### Display Format

```
Context: 2,450 / 128,000 tokens (1.9%)
```

Or abbreviated for compact display:
```
Ctx: 2,450 / 128K
```

### Commands

- `stats` - Full statistics including context size
- `context` - Show detailed context information (optional)

## User Experience

### Normal Operation
```
[Tokens: 2,450 / 128,000] > List the files in the current directory
```

### High Usage Warning
```
[Tokens: 100,000 / 128,000 ⚠] > Continue the conversation...
```

### Critical Usage
```
[Tokens: 120,000 / 128,000 ⚠⚠] > Help, context is almost full!
```

## Security Considerations

- Context size estimation should not reveal sensitive information
- Display should not expose internal implementation details
- User should understand when context may be compressed

## Testing Requirements

- Test context size display updates correctly
- Test warning thresholds display properly
- Test statistics command includes context size
- Test context size after compression
- Test with various message sizes
- Test display does not interfere with input
