# Requirement 044: Context Dump/Load Feature

## Description
The coding agent harness must support dumping the current conversation context to a JSON file for later reloading, enabling session persistence and continuity across agent restarts. This includes the `/dump` interactive command for manual dumps, a `--load` CLI flag for loading contexts on startup, and automatic dump-on-exit behavior.

## Acceptance Criteria

### Context Dump (`/dump` command)
- [ ] Interactive `/dump` command dumps the full conversation context to a JSON file
- [ ] Dumped context includes all messages from all iterations (including after compression)
- [ ] Dumped context includes the latest messages in full
- [ ] Context is dumped to the system's temp directory by default
- [ ] Default filename is `coding-agent-context.json`
- [ ] If a context file already exists, a unique filename is generated (e.g., `coding-agent-context-2.json`, `coding-agent-context-3.json`)
- [ ] The file path of the dumped context is printed to stdout after dumping
- [ ] Dumping preserves the message sequence: `[first_user_message, ...compressed_summary, ...recent_messages...]`
- [ ] Dumping preserves all message roles, content, reasoning, tool calls, and tool call IDs
- [ ] Dumping includes session metadata (start time, iteration count, stats)
- [ ] Failed dump operations log an error but do not crash the agent
- [ ] The dump JSON file is valid and parseable

### Context Load (`--load` CLI flag)
- [ ] `--load` flag accepts a path to a context JSON file
- [ ] Loading a context file restores the conversation to that state
- [ ] Only the **last iteration** is loaded (the most recent user prompt and all subsequent messages up to the current point)
- [ ] When loading, the agent continues from where the previous session left off
- [ ] The user prompt from the last iteration is used as the starting point for the resumed conversation
- [ ] Loading a context that doesn't exist returns a clear error message
- [ ] Loading a malformed context file returns a clear error message
- [ ] The number of loaded messages is displayed after loading
- [ ] Loading works in both interactive and one-shot modes

### Automatic Dump on Exit (`--no-dump-on-exit` flag)
- [ ] Context is automatically dumped to the temp folder when exiting interactive mode
- [ ] `--no-dump-on-exit` flag disables automatic dump on exit
- [ ] The automatic dump uses the same filename as `/dump` (with unique suffixes if needed)
- [ ] The file path of the auto-dumped context is printed to stdout before exiting
- [ ] Auto-dump is suppressed in one-shot mode (only relevant for interactive mode)
- [ ] Failed auto-dump operations log a warning but do not prevent clean exit

### JSON Context File Format
The dumped context file follows this structure:

```json
{
  "version": 1,
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:35:00Z",
  "session": {
    "start_time": "2024-01-15T10:30:00Z",
    "iterations": 15,
    "compression_count": 2,
    "stats": {
      "input_tokens": 12500,
      "output_tokens": 8500,
      "tool_calls": 25,
      "failed_tool_calls": 2
    }
  },
  "iterations": [
    {
      "index": 1,
      "messages": [
        {
          "role": "system",
          "content": "You are a helpful coding assistant..."
        },
        {
          "role": "user",
          "content": "Original user prompt"
        },
        {
          "role": "assistant",
          "content": "I'll help with that"
        },
        {
          "role": "tool",
          "content": "Tool result",
          "tool_call_id": "call_123"
        }
      ],
      "stats": {
        "input_tokens": 100,
        "output_tokens": 50,
        "tool_calls": 1,
        "failed_tool_calls": 0
      }
    },
    {
      "index": 3,
      "messages": [
        {
          "role": "system",
          "content": "You are a helpful coding assistant..."
        },
        {
          "role": "user",
          "content": "Original user prompt"
        },
        {
          "role": "assistant",
          "content": "Conversation summary: ..."
        },
        {
          "role": "user",
          "content": "Continue working on the task"
        }
      ],
      "stats": {
        "input_tokens": 1100,
        "output_tokens": 600,
        "tool_calls": 10,
        "failed_tool_calls": 0
      }
    }
  ]
}
```

#### Key Format Notes
- **`iterations`**: An array of full context snapshots, recorded only on **agent exit** or **context compression** events. Each iteration contains:
  - `index`: The iteration number at the time of the snapshot
  - `messages`: The full conversation context including the system prompt as the first message, followed by all conversation messages (user, assistant, tool)
  - `stats`: Cumulative token counts, tool call counts, etc. at the time of the snapshot
- **`session`**: Top-level session metadata (start time, total iterations, total compression count, final stats)
- When loading, only the **last iteration** is restored, providing the most recent conversation state

### Command-Line Interface

#### New Flags
```bash
# Load a previous context
coding-agent --load /tmp/coding-agent-context.json

# Disable automatic dump on exit
coding-agent --no-dump-on-exit

# Combine load with one-shot mode
coding-agent --load /tmp/coding-agent-context.json -p "Continue from here"
```

#### Interactive Commands
```bash
# Dump current context
/dump

# Help shows new commands
/clear
/compress
/dump        # New: dump context to file
/goal <prompt>
/goal-off
```

### Implementation Details

#### Context Structure
```go
// ContextDump represents a serializable conversation context.
type ContextDump struct {
    Version     int           `json:"version"`
    CreatedAt   time.Time     `json:"created_at"`
    UpdatedAt   time.Time     `json:"updated_at"`
    Session     SessionInfo   `json:"session"`
    Iterations  []Iteration   `json:"iterations"`
}

// Iteration represents a full snapshot of the context at a point in time.
// Each iteration includes the system prompt as the first message.
type Iteration struct {
    Index    int                 `json:"index"`
    Messages []*inference.Message `json:"messages"` // [system, user, assistant, tool, ...]
    Stats    StatsInfo           `json:"stats"`
}

// SessionInfo holds session metadata.
type SessionInfo struct {
    StartTime       time.Time      `json:"start_time"`
    Iterations      int            `json:"iterations"`
    CompressionCount int           `json:"compression_count"`
    Stats           StatsInfo      `json:"stats"`
}

// StatsInfo holds runtime statistics.
type StatsInfo struct {
    InputTokens     int `json:"input_tokens"`
    OutputTokens    int `json:"output_tokens"`
    ToolCalls       int `json:"tool_calls"`
    FailedToolCalls int `json:"failed_tool_calls"`
}
```

#### Dump Flow
```go
func (a *Agent) DumpContext() (string, error) {
    a.mu.Lock()
    dump := &ContextDump{
        Version:    1,
        CreatedAt:  a.stats.StartTime,
        UpdatedAt:  time.Now(),
        Session: SessionInfo{
            StartTime:        a.stats.StartTime,
            Iterations:       a.stats.Iterations,
            CompressionCount: a.compressionCount,
            Stats: StatsInfo{
                InputTokens:     a.stats.InputTokens,
                OutputTokens:    a.stats.OutputTokens,
                ToolCalls:       a.stats.ToolCalls,
                FailedToolCalls: a.stats.FailedToolCalls,
            },
        },
        Iterations: a.iterationHistory,
    }
    a.mu.Unlock()

    // Determine filename with unique suffix
    basePath := filepath.Join(os.TempDir(), "coding-agent-context.json")
    filePath := generateUniquePath(basePath)

    // Write JSON file
    data, err := json.MarshalIndent(dump, "", "  ")
    if err != nil {
        return "", fmt.Errorf("failed to marshal context: %w", err)
    }

    if err := os.WriteFile(filePath, data, 0644); err != nil {
        return "", fmt.Errorf("failed to write context file: %w", err)
    }

    return filePath, nil
}
```

#### Load Flow
```go
func (a *Agent) LoadContext(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("failed to read context file: %w", err)
    }

    var dump ContextDump
    if err := json.Unmarshal(data, &dump); err != nil {
        return fmt.Errorf("failed to parse context file: %w", err)
    }

    if dump.Version != 1 {
        return fmt.Errorf("unsupported context version: %d", dump.Version)
    }

    if len(dump.Iterations) == 0 {
        return fmt.Errorf("context file contains no iterations")
    }

    a.mu.Lock()
    defer a.mu.Unlock()

    // Restore from the last iteration snapshot
    lastSnapshot := dump.Iterations[len(dump.Iterations)-1]

    // Restore system prompt from the first message of the last snapshot
    if len(lastSnapshot.Messages) > 0 && lastSnapshot.Messages[0].Role == "system" {
        a.systemPrompt = lastSnapshot.Messages[0].Content
        // Restore messages excluding the system prompt
        a.context = make([]*inference.Message, 0, len(lastSnapshot.Messages)-1)
        for _, msg := range lastSnapshot.Messages[1:] {
            cpy := *msg
            a.context = append(a.context, &cpy)
        }
    } else {
        // Fallback: use messages as is
        a.context = make([]*inference.Message, 0, len(lastSnapshot.Messages))
        for _, msg := range lastSnapshot.Messages {
            cpy := *msg
            a.context = append(a.context, &cpy)
        }
    }

    // Restore session metadata and stats
    a.stats.StartTime = dump.Session.StartTime
    a.stats.Iterations = dump.Session.Iterations
    a.compressionCount = dump.Session.CompressionCount
    a.stats.InputTokens = dump.Session.Stats.InputTokens
    a.stats.OutputTokens = dump.Session.Stats.OutputTokens
    a.stats.ToolCalls = dump.Session.Stats.ToolCalls
    a.stats.FailedToolCalls = dump.Session.Stats.FailedToolCalls
    a.lastTotalTokens = inference.EstimateContextSize(a.context, a.inference.GetTools(), a.systemPrompt)
    a.toolResultMsgsSinceLastAPI = make(map[int]bool)

    return nil
}
```

#### Iteration Recording
Iterations are recorded automatically at two points:
1. **On Agent Exit**: Before `Run()` returns a result, `recordIteration()` captures the full context
2. **On Context Compression**: Before `compressContext()` modifies the context, `recordIterationUnlocked()` captures the pre-compression state

This ensures that the dump contains:
- The final state of the conversation (when the agent finishes)
- The full state of the conversation before each compression event (preserving history that would otherwise be lost)

### Error Handling

| Scenario | Behavior |
|----------|----------|
| Context file doesn't exist | Clear error: "Context file not found: {path}" |
| Malformed JSON file | Clear error: "Failed to parse context file: {error}" |
| Unsupported version | Clear error: "Unsupported context version: N" |
| Write permission denied | Warning logged, exit continues (non-fatal) |
| Disk full | Warning logged, exit continues (non-fatal) |

### Security Considerations
- Context files are stored in the system temp directory (typically `/tmp` on Unix)
- Context files may contain sensitive information (source code, API keys, etc.)
- Users should be aware that dumped contexts contain full conversation history
- No encryption is applied to context files by default
- File permissions are set to 0644 (readable by owner and group)

### Testing Requirements

#### Unit Tests
- [ ] `TestDumpAndLoadContext`: Full dump-load cycle preserves messages, system prompt, and session stats
- [ ] `TestDumpContext_MultipleIterations`: Dump includes all recorded iterations (exit + compression snapshots)
- [ ] `TestDumpContext_IterationContainsSystemPrompt`: Each iteration's first message is the system prompt
- [ ] `TestDumpContext_GeneratesUniqueFilenames`: Multiple dumps generate unique filenames
- [ ] `TestLoadContext_LastIterationOnly`: Only the last iteration is restored
- [ ] `TestLoadContext_RejectsUnsupportedVersions`: Version mismatch returns error
- [ ] `TestLoadContext_FileNotFound`: Missing file returns clear error
- [ ] `TestLoadContext_InvalidJSON`: Malformed JSON returns clear error
- [ ] `TestLoadContext_InvalidVersionEmptyIterations`: Version 1 with empty iterations returns error
- [ ] `TestRecordIteration_AtExit`: Agent records iteration before returning from Run()
- [ ] `TestRecordIteration_AtCompression`: Agent records iteration before modifying context during compression

#### Integration Tests
- [ ] Full dump-load cycle preserves conversation state
- [ ] Context loaded with compressed history works correctly
- [ ] Agent continues from loaded context with new input
- [ ] Multiple dump cycles generate unique filenames
- [ ] Auto-dump on exit after user types Ctrl+C
- [ ] Auto-dump on exit after normal `/clear` session end

## Related Requirements
- **009-context-compression.md**: Context compression affects what is preserved in dump
- **025-non-interactive-one-shot-mode.md**: One-shot mode interaction with context loading
- **028-debug-flag.md**: Debug logging is separate from context dump
