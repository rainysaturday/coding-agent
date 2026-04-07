# Requirement 006: Write File Tool

## Description
The harness must support a `write_file` tool that allows writing contents to files via OpenAI's tool calling interface.

## Acceptance Criteria
- [ ] Tool named `write_file` is available
- [ ] Accepts file path and content as input parameters
- [ ] Writes content to specified file
- [ ] Creates file if it does not exist
- [ ] Overwrites file if it exists
- [ ] Creates parent directories if needed
- [ ] Handles permission errors gracefully
- [ ] Handles disk full errors gracefully
- [ ] Tool call failures are tracked in statistics

## Tool Definition (OpenAI Format)

```json
{
  "type": "function",
  "function": {
    "name": "write_file",
    "description": "Write content to a file",
    "parameters": {
      "type": "object",
      "properties": {
        "path": {
          "type": "string",
          "description": "Path to the file to write"
        },
        "content": {
          "type": "string",
          "description": "Content to write to the file"
        }
      },
      "required": ["path", "content"]
    }
  }
}
```

## Tool Call Format

```json
{
  "id": "call_abc123",
  "type": "function",
  "function": {
    "name": "write_file",
    "arguments": "{\"path\":\"/path/to/file.txt\",\"content\":\"Hello World\"}"
  }
}
```

### Parameters
- `path`: Path to the file to write (required, string)
- `content`: Content to write to the file (required, string)
  - Multi-line content uses `\n` escape sequences
  - All special characters must be JSON-escaped

### Examples

**Write simple text:**
```json
{
  "id": "call_001",
  "type": "function",
  "function": {
    "name": "write_file",
    "arguments": "{\"path\":\"/tmp/hello.txt\",\"content\":\"Hello, World!\"}"
  }
}
```

**Write a script:**
```json
{
  "id": "call_002",
  "type": "function",
  "function": {
    "name": "write_file",
    "arguments": "{\"path\":\"/tmp/script.sh\",\"content\":\"#!/bin/bash\\necho \\\"Hello\\\"\\nexit 0\"}"
  }
}
```

**Write code:**
```json
{
  "id": "call_003",
  "type": "function",
  "function": {
    "name": "write_file",
    "arguments": "{\"path\":\"./main.go\",\"content\":\"package main\\n\\nimport \\\"fmt\\\"\\n\\nfunc main() {\\n    fmt.Println(\\\"Hello\\\")\\n}\"}"
  }
}
```

## Return Values

On success:
- `success`: `true`
- `path`: The path that was written

On failure:
- `error`: Description of the error (permission denied, disk full, etc.)
- `success`: `false`
