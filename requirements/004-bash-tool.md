# Requirement 004: Bash Tool

## Description
The harness must support a `bash` tool that allows execution of shell commands via OpenAI's tool calling interface.

## Acceptance Criteria
- [ ] Tool named `bash` is available
- [ ] Accepts command string as input parameter
- [ ] Executes command in shell environment
- [ ] Returns command output (stdout and stderr)
- [ ] Returns exit code of executed command
- [ ] Handles command execution errors gracefully
- [ ] Tool call failures are tracked in statistics

## TUI Feedback for Bash Output

When displaying bash tool results in the TUI:
- The output should show the **last lines (tail)** of the command output, not the beginning
- This is because for bash commands, the end of output typically contains the actual results and is more useful for the user
- If the output exceeds 5 lines, show only the last 5 lines with a "[output truncated]" indicator at the beginning
- Display the exit code if non-zero

Example TUI display:
```
Tool 'bash' executed successfully:
... [output truncated]
line_n-4
line_n-3
line_n-2
line_n-1
line_n
```

## Tool Definition (OpenAI Format)

```json
{
  "type": "function",
  "function": {
    "name": "bash",
    "description": "Execute a bash command in the terminal",
    "parameters": {
      "type": "object",
      "properties": {
        "command": {
          "type": "string",
          "description": "The bash command or script to execute"
        },
        "timeout": {
          "type": "integer",
          "description": "Timeout in milliseconds (default: 30000)"
        }
      },
      "required": ["command"]
    }
  }
}
```

## Tool Call Format

The OpenAI API returns tool calls in the following format:

```json
{
  "id": "call_abc123",
  "type": "function",
  "function": {
    "name": "bash",
    "arguments": "{\"command\":\"ls -la /home\"}"
  }
}
```

### Single-line Commands
```json
{
  "id": "call_abc123",
  "type": "function",
  "function": {
    "name": "bash",
    "arguments": "{\"command\":\"ls -la /home\"}"
  }
}
```

```json
{
  "id": "call_def456",
  "type": "function",
  "function": {
    "name": "bash",
    "arguments": "{\"command\":\"echo \\\"Hello World\\\"\"}"
  }
}
```

### Multi-line Scripts
```json
{
  "id": "call_ghi789",
  "type": "function",
  "function": {
    "name": "bash",
    "arguments": "{\"command\":\"#!/bin/bash\\n# Multi-line script\\necho \\\"Starting...\\\"\\nfor i in {1..10}; do\\n    echo \\\"Iteration $i\\\"\\ndone\\necho \\\"Done!\\\"\"}"
  }
}
```

### Parameters
- `command`: Shell command or script to execute (required, string)
  - Single-line commands use regular JSON strings
  - Multi-line scripts use `\n` escape sequences
  - All special characters must be JSON-escaped
- `timeout`: Command timeout in milliseconds (optional, integer, default: 30000)
  - If the command does not complete within the timeout, execution is terminated
  - Timeout value must be positive
  - If timeout occurs, the error message will clearly indicate it was a timeout


### Examples

**Simple command:**
```json
{
  "id": "call_001",
  "type": "function",
  "function": {
    "name": "bash",
    "arguments": "{\"command\":\"pwd\"}"
  }
}
```

**Command with quotes:**
```json
{
  "id": "call_002",
  "type": "function",
  "function": {
    "name": "bash",
    "arguments": "{\"command\":\"echo \\\"Hello World\\\"\"}"
  }
}
```

**Multi-line script:**
```json
{
  "id": "call_003",
  "type": "function",
  "function": {
    "name": "bash",
    "arguments": "{\"command\":\"#!/bin/bash\\nset -e\\ncd /tmp\\ncat > test.txt << EOF\\nline 1\\nline 2\\nEOF\\ncat test.txt\"}"
  }
}
```

**Complex command with special characters:**
```json
{
  "id": "call_004",
  "type": "function",
  "function": {
    "name": "bash",
    "arguments": "{\"command\":\"echo \\\"Price: $100 \\\\\\\"special\\\\\\\" items\\\"\"}"
  }
}
```

## Return Values

On success:
- `output`: Combined stdout and stderr from command execution
- `success`: `true`
- `exit_code`: 0

On failure:
- `error`: Description of the error
- `success`: `false`
- `exit_code`: Non-zero exit code
On timeout (exit code 124):
- `error`: "command timed out after Xms (timeout exceeded). The command did not complete within the specified timeout period. Consider increasing the timeout parameter..."
- `success`: `false`
- `exit_code`: 124 (convention used by GNU `timeout` command)

