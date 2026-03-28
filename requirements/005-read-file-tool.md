# Requirement 005: Read File Tool

## Description
The harness must support a `read_file` tool that allows reading file contents.

## Acceptance Criteria
- [ ] Tool named `read_file` is available
- [ ] Accepts file path as input parameter
- [ ] Reads file contents and returns them
- [ ] Handles file not found errors gracefully
- [ ] Handles permission errors gracefully
- [ ] Supports reading text files
- [ ] Tool call failures are tracked in statistics

## Tool Usage

```
[TOOL:{"name":"read_file","parameters":{"path":"/path/to/file.txt"}}]
```

### Parameters
- `path`: Path to the file to read (required, string)

### Examples

**Read a file:**
```
[TOOL:{"name":"read_file","parameters":{"path":"/home/user/document.txt"}}]
```

**Read a source file:**
```
[TOOL:{"name":"read_file","parameters":{"path":"./src/main.go"}}]
```

## Return Values

On success:
- `output`: Full contents of the file
- `success`: `true`

On failure:
- `error`: Description of the error (file not found, permission denied, etc.)
- `success`: `false`
