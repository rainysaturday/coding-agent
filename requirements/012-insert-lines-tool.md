# Requirement 012: Insert Lines Tool

## Description
The harness must support an `insert_lines` tool that allows inserting a set of lines at a specified line number in a file via OpenAI's tool calling interface.

## Acceptance Criteria
- [ ] Tool named `insert_lines` is available
- [ ] Accepts file path as input parameter
- [ ] Accepts line number as input parameter (1-indexed position to insert before)
- [ ] Accepts lines to insert as input parameter
- [ ] Inserts lines before the specified line number
- [ ] Inserting at line 1 inserts at beginning of file
- [ ] Inserting beyond file end appends to end of file
- [ ] Existing lines are shifted down after insertion
- [ ] Creates file if it does not exist
- [ ] Handles permission errors gracefully
- [ ] Handles disk full errors gracefully
- [ ] Returns confirmation of insertion with details
- [ ] Tool call failures are tracked in statistics

## Tool Definition (OpenAI Format)

```json
{
  "type": "function",
  "function": {
    "name": "insert_lines",
    "description": "Insert lines at a specific line number in a file",
    "parameters": {
      "type": "object",
      "properties": {
        "path": {
          "type": "string",
          "description": "File path to modify"
        },
        "line": {
          "type": "integer",
          "description": "Line number to insert before (1-indexed)"
        },
        "lines": {
          "type": "string",
          "description": "Lines to insert (use \\n for newlines)"
        }
      },
      "required": ["path", "line", "lines"]
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
    "name": "insert_lines",
    "arguments": "{\"path\":\"/path/to/file.txt\",\"line\":5,\"lines\":\"single line to insert\"}"
  }
}
```

### Parameters
- `path`: File path to modify (required, string)
- `line`: Line number to insert before (required, integer, 1-indexed)
- `lines`: Lines to insert (required, string)
  - Multi-line content uses `\n` escape sequences
  - All special characters must be JSON-escaped

### Examples

**Insert single line:**
```json
{
  "id": "call_001",
  "type": "function",
  "function": {
    "name": "insert_lines",
    "arguments": "{\"path\":\"./notes.txt\",\"line\":1,\"lines\":\"# Header\"}"
  }
}
```

**Insert multiple lines:**
```json
{
  "id": "call_002",
  "type": "function",
  "function": {
    "name": "insert_lines",
    "arguments": "{\"path\":\"./script.sh\",\"line\":2,\"lines\":\"set -e\\nset -u\\nset -o pipefail\"}"
  }
}
```

**Insert at end of file:**
```json
{
  "id": "call_003",
  "type": "function",
  "function": {
    "name": "insert_lines",
    "arguments": "{\"path\":\"./log.txt\",\"line\":9999,\"lines\":\"New log entry\"}"
  }
}
```

## Return Values

On success:
- `success`: `true`
- `path`: The path that was modified
- `line`: The line number where insertion occurred
- `linesInserted`: Number of lines inserted

On failure:
- `error`: Description of the error
- `success`: `false`

## Behavior Notes

- Line numbers are 1-indexed
- Inserting at line 1 puts content at the very beginning
- Inserting at a line beyond file length appends to end
- Existing content is shifted down after the insertion point
