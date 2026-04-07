# Requirement 005: Read File Tool

## Description
The harness must support a `read_file` tool that allows reading file contents via OpenAI's tool calling interface.

## Acceptance Criteria
- [ ] Tool named `read_file` is available
- [ ] Accepts file path as input parameter
- [ ] Reads file contents and returns them
- [ ] Handles file not found errors gracefully
- [ ] Handles permission errors gracefully
- [ ] Supports reading text files
- [ ] Tool call failures are tracked in statistics

## Tool Definition (OpenAI Format)

```json
{
  "type": "function",
  "function": {
    "name": "read_file",
    "description": "Read the contents of a file",
    "parameters": {
      "type": "object",
      "properties": {
        "path": {
          "type": "string",
          "description": "Path to the file to read"
        }
      },
      "required": ["path"]
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
    "name": "read_file",
    "arguments": "{\"path\":\"/path/to/file.txt\"}"
  }
}
```

### Parameters
- `path`: Path to the file to read (required, string)

### Examples

**Read a file:**
```json
{
  "id": "call_001",
  "type": "function",
  "function": {
    "name": "read_file",
    "arguments": "{\"path\":\"/home/user/document.txt\"}"
  }
}
```

**Read a source file:**
```json
{
  "id": "call_002",
  "type": "function",
  "function": {
    "name": "read_file",
    "arguments": "{\"path\":\"./src/main.go\"}"
  }
}
```

## Return Values

On success:
- `output`: Full contents of the file
- `success`: `true`
- `path`: Path of the file that was read

On failure:
- `error`: Description of the error (file not found, permission denied, etc.)
- `success`: `false`
