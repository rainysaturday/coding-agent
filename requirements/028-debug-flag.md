# Requirement 028: Debug Flag for Conversation Logging

## Description

The coding agent harness must support a `--debug` flag that continuously saves the entire conversation with the LLM to a debug log file. When enabled, all messages exchanged with the LLM (system prompt, user messages, assistant responses, tool calls, and tool results) are logged to a file for debugging and analysis purposes.

## Acceptance Criteria

- [ ] Accept `--debug` flag via command-line argument
- [ ] Debug log file is created when `--debug` flag is enabled
- [ ] All LLM conversation messages are logged to the debug file
- [ ] Log includes system prompt, user messages, assistant responses
- [ ] Log includes tool calls and their arguments
- [ ] Log includes tool results and execution output
- [ ] Log includes timestamps for each message
- [ ] Log includes token usage information for each request
- [ ] Log file path is configurable (default: `debug.log` in current directory)
- [ ] Debug logging works in both interactive and one-shot modes
- [ ] Debug logging works in both streaming and non-streaming modes
- [ ] Log file can be configured via environment variable (`CODING_AGENT_DEBUG_LOG`)
- [ ] Log file can be configured via command-line flag (`--debug-log path`)
- [ ] Log format is structured and human-readable
- [ ] Log file is truncated on new session
- [ ] Debug mode does not interfere with normal operation
- [ ] Sensitive information (API keys) is redacted from logs

## Command-Line Interface

### Basic Usage

```bash
# Enable debug logging with default filename
coding-agent --debug

# Enable debug logging with custom filename
coding-agent --debug --debug-log /path/to/debug.log

# One-shot mode with debug logging
coding-agent --prompt "Create a Go function" --debug

# Interactive mode with debug logging
coding-agent --debug
```

### Help Output

```bash
$ coding-agent --help
Minimal Coding Agent Harness

Usage:
  coding-agent [OPTIONS] [COMMAND]

Options:
  -p, --prompt string      Prompt for one-shot mode (non-interactive)
      --stdin              Read prompt from stdin
      --prompt-file path   Read prompt from file
      --debug              Enable debug logging (saves conversation to file)
      --debug-log path     Path to debug log file (default: debug.log)
      --model string       Model to use (default: "llama3")
      --temperature float  Inference temperature (default: 0.7)
      --max-tokens int     Maximum tokens to generate (default: 4096)
      --verbose            Enable verbose output
      --quiet              Suppress non-essential output
      --output file        Write results to file
      --no-stream          Disable streaming output
  -h, --help               Show this help message
  -v, --version            Show version information

Examples:
  coding-agent --debug
  coding-agent --debug --debug-log /tmp/agent-debug.log
  coding-agent -p "Task" --debug
```

## Environment Variable Configuration

```bash
# Enable debug mode via environment variable
export CODING_AGENT_DEBUG=true
coding-agent

# Specify debug log path via environment variable
export CODING_AGENT_DEBUG_LOG=/var/log/coding-agent/debug.log
coding-agent --debug
```

## Log File Format

### Default Log File Path

- **Default:** `debug.log` in the current working directory
- **Environment Variable:** `CODING_AGENT_DEBUG_LOG`
- **Command-Line Flag:** `--debug-log path`

### Log Format

Each log entry includes:

- Timestamp (ISO 8601 format)
- Message role (system, user, assistant, tool)
- Content or tool call details
- Token usage (if available)
- Request/response metadata

### Example Log Output

```
================================================================================
CODING AGENT DEBUG LOG
Session: 2024-01-15T10:30:00Z
Version: a1b2c3d [clean]
================================================================================

[2024-01-15T10:30:00Z] SYSTEM PROMPT (tokens: 1250)
--------------------------------------------------------------------------------
You are a helpful coding assistant. You have access to the following tools...
[... full system prompt content ...]

[2024-01-15T10:30:01Z] USER MESSAGE (tokens: 15)
--------------------------------------------------------------------------------
Create a Go hello world program

[2024-01-15T10:30:02Z] ASSISTANT RESPONSE (tokens: 50)
--------------------------------------------------------------------------------
I'll create a simple Go hello world program for you.

[2024-01-15T10:30:02Z] TOOL CALL: write_file
--------------------------------------------------------------------------------
Tool ID: call_abc123
Parameters:
{
  "path": "hello.go",
  "content": "package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"Hello, World!\")\n}"
}

[2024-01-15T10:30:03Z] TOOL RESULT: write_file
--------------------------------------------------------------------------------
Tool ID: call_abc123
Status: success
Output: File written successfully

[2024-01-15T10:30:03Z] ASSISTANT RESPONSE (tokens: 25)
--------------------------------------------------------------------------------
I've created hello.go with a simple Go program that prints "Hello, World!".

[2024-01-15T10:30:03Z] SESSION SUMMARY
--------------------------------------------------------------------------------
Total Messages: 6
Total Input Tokens: 1285
Total Output Tokens: 75
Total Tool Calls: 1
Duration: 3.2s
================================================================================
```

### Structured Log Format (Optional JSON)

For programmatic parsing, JSON format can be enabled:

```bash
coding-agent --debug --debug-format json
```

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "session_id": "sess_abc123",
  "version": "a1b2c3d",
  "messages": [
    {
      "role": "system",
      "content": "...",
      "tokens": 1250
    },
    {
      "role": "user",
      "content": "Create a Go hello world program",
      "tokens": 15
    },
    {
      "role": "assistant",
      "content": "I'll create a simple Go hello world program for you.",
      "tokens": 50
    },
    {
      "role": "tool_call",
      "tool_id": "call_abc123",
      "tool_name": "write_file",
      "parameters": {
        "path": "hello.go",
        "content": "..."
      }
    },
    {
      "role": "tool_result",
      "tool_id": "call_abc123",
      "tool_name": "write_file",
      "status": "success",
      "output": "File written successfully"
    }
  ],
  "summary": {
    "total_messages": 6,
    "total_input_tokens": 1285,
    "total_output_tokens": 75,
    "total_tool_calls": 1,
    "duration_seconds": 3.2
  }
}
```

## Implementation Details

### Debug Logger Interface

```go
type DebugLogger interface {
    LogSystemPrompt(prompt string, tokenCount int)
    LogUserMessage(content string, tokenCount int)
    LogAssistantMessage(content string, tokenCount int)
    LogToolCall(toolID, toolName string, parameters map[string]interface{})
    LogToolResult(toolID, toolName string, success bool, output string)
    LogSessionSummary(summary SessionSummary)
    Close() error
}
```

### Session Summary Structure

```go
type SessionSummary struct {
    SessionID       string
    StartTime       time.Time
    EndTime         time.Time
    TotalMessages   int
    TotalInputTokens int
    TotalOutputTokens int
    TotalToolCalls   int
    FailedToolCalls  int
    DurationSeconds float64
}
```

### Message Logging

```go
func (d *debugLogger) LogUserMessage(content string, tokenCount int) {
    d.writeLog("[%s] USER MESSAGE (tokens: %d)\n",
        time.Now().Format(time.RFC3339), tokenCount)
    d.writeLog("%s\n", content)
    d.session.TotalMessages++
    d.session.TotalInputTokens += tokenCount
}

func (d *debugLogger) LogAssistantMessage(content string, tokenCount int) {
    d.writeLog("[%s] ASSISTANT RESPONSE (tokens: %d)\n",
        time.Now().Format(time.RFC3339), tokenCount)
    d.writeLog("%s\n", content)
    d.session.TotalMessages++
    d.session.TotalOutputTokens += tokenCount
}

func (d *debugLogger) LogToolCall(toolID, toolName string, parameters map[string]interface{}) {
    d.writeLog("[%s] TOOL CALL: %s\n",
        time.Now().Format(time.RFC3339), toolName)
    d.writeLog("Tool ID: %s\n", toolID)
    d.writeLog("Parameters:\n%s\n", formatJSON(parameters))
    d.session.TotalToolCalls++
}

func (d *debugLogger) LogToolResult(toolID, toolName string, success bool, output string) {
    status := "success"
    if !success {
        status = "failed"
        d.session.FailedToolCalls++
    }
    d.writeLog("[%s] TOOL RESULT: %s\n",
        time.Now().Format(time.RFC3339), toolName)
    d.writeLog("Tool ID: %s\n", toolID)
    d.writeLog("Status: %s\n", status)
    d.writeLog("Output: %s\n", output)
}
```

### Redaction of Sensitive Data

```go
func redactSensitiveData(content string) string {
    // Redact API keys
    content = regexp.MustCompile(`(api[_-]?key|apikey)\s*[:=]\s*["']?[a-zA-Z0-9]{20,}["']?`).
        ReplaceAllString(content, "$1: [REDACTED]")

    // Redact tokens
    content = regexp.MustCompile(`(token|secret)\s*[:=]\s*["']?[a-zA-Z0-9]{16,}["']?`).
        ReplaceAllString(content, "$1: [REDACTED]")

    return content
}
```

## Streaming Mode Handling

### Streaming Debug Logging

In streaming mode, debug logging should capture:

- Each streaming chunk with its type (reasoning vs. normal)
- Accumulated tool call data
- Final complete message

```go
func (d *debugLogger) LogStreamingChunk(content string, chunkType string) {
    d.writeLog("[%s] STREAMING CHUNK (%s)\n",
        time.Now().Format(time.RFC3339), chunkType)
    d.writeLog("%s", content)  // No newline for streaming chunks
}

func (d *debugLogger) LogStreamingComplete() {
    d.writeLog("\n[STREAMING COMPLETE]\n")
}
```

### Default Behavior

- **Default:** Truncate/overwrite on new session

## Configuration Chain

1. **Command-line flags** (highest priority)

   - `--debug` - Enable debug logging
   - `--debug-log path` - Specify log file path
   - `--debug-format format` - Specify log format (text/json)

2. **Environment variables** (medium priority)

   - `CODING_AGENT_DEBUG=true/false`
   - `CODING_AGENT_DEBUG_LOG=/path/to/log`
   - `CODING_AGENT_DEBUG_FORMAT=text/json`

3. **Config file** (lowest priority)
   - `debug = true`
   - `debug_log = /path/to/log`

## Use Cases

### Debugging Agent Behavior

```bash
# Full conversation logging for debugging
coding-agent --debug -p "Build a REST API"
# Review debug.log to see all tool calls and responses
```

### Analyzing Token Usage

```bash
# Log token usage for optimization
coding-agent --debug --debug-log token-analysis.log
# Review token usage patterns in the log
```

### Auditing Agent Actions

```bash
# Log all agent actions for compliance
coding-agent --debug --debug-log audit.log
# Review what actions the agent took
```

### Testing and Development

```bash
# Debug agent during development
coding-agent --debug --debug-log dev-debug.log --verbose
# Correlate debug log with console output
```

### Reproducing Issues

```bash
# Capture full context for bug reports
coding-agent --debug --debug-log bug-repro.log
# Share debug.log with developers to reproduce issues
```

## Privacy and Security

### Sensitive Data Redaction

The following must be redacted from debug logs:

- API keys and tokens
- Authentication credentials
- Sensitive file paths (if configured)
- User-defined redaction patterns

### Log File Permissions

- Debug log files should be created with restrictive permissions (e.g., `0600`)
- Only the user who created the log should have access
- Warn users about sensitive data in logs

### User Notification

```
[WARNING] Debug logging enabled. All conversation data will be saved to:
  /path/to/debug.log

This may include sensitive information. Ensure the log file is protected.
```

## Testing Requirements

### Unit Tests

- [ ] Debug logger creates file correctly
- [ ] All message types are logged
- [ ] Timestamps are accurate
- [ ] Token counts are recorded
- [ ] Tool calls and results are logged
- [ ] Sensitive data is redacted
- [ ] Session summary is accurate
- [ ] JSON format output is valid

### Integration Tests

- [ ] Debug mode works in interactive mode
- [ ] Debug mode works in one-shot mode
- [ ] Debug mode works in streaming mode
- [ ] Log file is readable after session
- [ ] Environment variable configuration works
- [ ] Command-line flag overrides environment variable

### Security Tests

- [ ] API keys are redacted from logs
- [ ] Log file has restrictive permissions
- [ ] Sensitive data patterns are detected and redacted
- [ ] No credentials are written to logs

## Related Requirements

- **025-non-interactive-one-shot-mode.md**: One-shot mode with debug logging
- **010-streaming-inference.md**: Debug logging in streaming mode
- **016-tool-result-context-integration.md**: Tool call logging
- **007-inference-backend.md**: Inference request/response logging
- **024-zero-external-dependencies.md**: Use stdlib only for logging

## Acceptance Checklist

- [ ] `--debug` flag enables debug logging
- [ ] `--debug-log path` specifies log file location
- [ ] All conversation messages are logged
- [ ] Timestamps are included
- [ ] Token usage is tracked
- [ ] Tool calls and results are logged
- [ ] Session summary is generated
- [ ] Sensitive data is redacted
- [ ] Works in interactive and one-shot modes
- [ ] Works in streaming and non-streaming modes
- [ ] Environment variable configuration works
- [ ] Log file permissions are secure
- [ ] No external logging dependencies used
