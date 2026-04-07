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
