# Requirement 011: Read Lines Tool

## Description
The harness must support a `read_lines` tool that allows reading a specific line range from a file via OpenAI's tool calling interface.

## Acceptance Criteria
- [ ] Tool named `read_lines` is available
- [ ] Accepts file path as input parameter
- [ ] Accepts start line number as input parameter (1-indexed)
- [ ] Accepts end line number as input parameter (1-indexed)
- [ ] Returns only the specified line range
- [ ] Handles start > end by returning empty result or error
- [ ] Handles start line beyond file end gracefully
- [ ] Handles end line beyond file end by reading to end of file
- [ ] Handles file not found errors gracefully
- [ ] Handles permission errors gracefully
- [ ] Returns line numbers with content for reference
- [ ] Tool call failures are tracked in statistics

## Tool Definition (OpenAI Format)

```json
{
  "type": "function",
  "function": {
    "name": "read_lines",
    "description": "Read a specific line range from a file",
    "parameters": {
      "type": "object",
      "properties": {
        "path": {
          "type": "string",
          "description": "Path to the file to read"
        },
        "start": {
          "type": "integer",
          "description": "Starting line number (1-indexed)"
        },
        "end": {
          "type": "integer",
          "description": "Ending line number (1-indexed)"
        }
      },
      "required": ["path", "start", "end"]
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
    "name": "read_lines",
    "arguments": "{\"path\":\"/path/to/file.txt\",\"start\":1,\"end\":10}"
  }
}
```

### Parameters
- `path`: Path to the file to read (required, string)
- `start`: Starting line number (required, integer, 1-indexed)
- `end`: Ending line number (required, integer, 1-indexed)

### Examples

**Read first 10 lines:**
```json
{
  "id": "call_001",
  "type": "function",
  "function": {
    "name": "read_lines",
    "arguments": "{\"path\":\"/path/to/file.txt\",\"start\":1,\"end\":10}"
  }
}
```

**Read lines 100-200:**
```json
{
  "id": "call_002",
  "type": "function",
  "function": {
    "name": "read_lines",
    "arguments": "{\"path\":\"/path/to/large.txt\",\"start\":100,\"end\":200}"
  }
}
```

**Read specific line:**
```json
{
  "id": "call_003",
  "type": "function",
  "function": {
    "name": "read_lines",
    "arguments": "{\"path\":\"/path/to/file.txt\",\"start\":42,\"end\":42}"
  }
}
```

## Return Values

On success:
- `output`: The requested lines with line numbers (format: "1: line content")
- `start`: The start line that was requested
- `end`: The end line that was requested
- `success`: `true`

On failure:
- `error`: Description of the error
- `success`: `false`

## Behavior Notes

- Line numbers are 1-indexed (first line is line 1)
- If `start` > `end`, returns empty result or error
- If `start` is beyond file length, returns empty result
- If `end` is beyond file length, reads to end of file
- Returned output includes line numbers for reference
