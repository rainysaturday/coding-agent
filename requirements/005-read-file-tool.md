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
- [ ] Rejects files larger than 20KB with a clear error message
- [ ] Rejects binary files with a clear error message
- [ ] Tool call failures are tracked in statistics

## Limits
- Maximum file size: 20KB (20,480 bytes)
- Binary files are detected by checking for null bytes in the first 512 bytes
- When a file exceeds the size limit, the tool suggests using `read_lines` to read the file in smaller chunks
- When a binary file is detected, the tool suggests using `view_image` for images or `bash` for binary inspection

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
