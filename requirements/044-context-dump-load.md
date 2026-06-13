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
  "system_prompt": "You are a helpful coding assistant...",
  "messages": [
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
    },
    {
      "role": "assistant",
      "content": "I'll modify the file",
      "tool_calls": [
        {
          "id": "call_123",
          "type": "function",
          "function": {
            "name": "write_file",
            "arguments": "{\"path\": \"main.go\", \"content\": \"...\"}"
          }
        }
      ]
    },
    {
      "role": "tool",
      "content": "Tool 'write_file' executed successfully:\nFile written",
      "tool_call_id": "call_123"
    }
  ]
}
```

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
    Version     int            `json:"version"`
    CreatedAt   time.Time      `json:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at"`
    Session     SessionInfo    `json:"session"`
    SystemPrompt string        `json:"system_prompt"`
    Messages    []*Message     `json:"messages"`
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
        Version:     1,
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
        Session: SessionInfo{
            StartTime:      a.stats.StartTime,
            Iterations:     a.stats.Iterations,
            CompressionCount: a.compressionCount,
            Stats:          a.dumpStats(),
        },
        SystemPrompt: a.systemPrompt,
        Messages:     make([]*Message, len(a.context)),
    }
    copy(dump.Messages, a.context)
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

    // Update system prompt
    a.systemPrompt = dump.SystemPrompt

    // Load messages from context
    a.mu.Lock()
    a.context = dump.Messages
    a.stats.StartTime = dump.Session.StartTime
    a.stats.Iterations = dump.Session.Iterations
    a.compressionCount = dump.Session.CompressionCount
    a.mu.Unlock()

    return nil
}
```

#### Auto-Dump on Exit (Interactive Mode)
```go
func runInteractiveMode(cfg *config.Config) error {
    defer func() {
        // Auto-dump on exit (unless --no-dump-on-exit)
        if !cfg.NoDumpOnExit && ag != nil {
            filePath, err := ag.DumpContext()
            if err != nil {
                fmt.Fprintf(os.Stderr, "Warning: failed to dump context: %v\n", err)
            } else {
                fmt.Printf("\n%s[Context saved to: %s]%s\n",
                    colors.GetColor("green"), filePath, colors.GetColor("reset"))
            }
        }
    }()
    // ... rest of interactive mode
}
```

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
- [ ] Context dump creates valid JSON file
- [ ] Context dump preserves all message fields
- [ ] Context dump generates unique filenames when file exists
- [ ] Context dump includes session metadata
- [ ] Context load restores messages correctly
- [ ] Context load rejects unsupported versions
- [ ] Context load handles missing file gracefully
- [ ] Context load handles malformed JSON gracefully
- [ ] Auto-dump on exit is triggered in interactive mode
- [ ] Auto-dump on exit is suppressed with `--no-dump-on-exit`
- [ ] Auto-dump on exit is suppressed in one-shot mode
- [ ] `/dump` command works in interactive mode
- [ ] `--load` flag works in interactive mode
- [ ] `--load` flag works in one-shot mode

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
