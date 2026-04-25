# Requirement 034: Grep Tool

## Description
The harness must support a `grep` tool that allows searching through file contents using regular expressions via OpenAI's tool calling interface. This tool enables the agent to efficiently search for patterns across multiple files, similar to the `grep` or `ripgrep` command-line utility.

## Acceptance Criteria
- [ ] Tool named `grep` is available
- [ ] Accepts a search pattern (regex) as input parameter
- [ ] Accepts a path or directory to search within
- [ ] Supports recursive directory search
- [ ] Supports filtering by file extensions or patterns
- [ ] Returns matching lines with file paths and line numbers
- [ ] Handles file not found errors gracefully
- [ ] Handles permission errors gracefully
- [ ] Supports case-insensitive search option
- [ ] Tool call failures are tracked in statistics

## Tool Definition (OpenAI Format)

```json
{
  "type": "function",
  "function": {
    "name": "grep",
    "description": "Search for patterns in files using grep-like functionality",
    "parameters": {
      "type": "object",
      "properties": {
        "path": {
          "type": "string",
          "description": "Path to search within (file or directory, defaults to current directory if not specified)"
        },
        "pattern": {
          "type": "string",
          "description": "Search pattern (regular expression)"
        },
        "flags": {
          "type": "array",
          "items": {
            "type": "string"
          },
          "description": "List of grep-style flags to control output (e.g., 'i' for case-insensitive, 'r' for recursive, 'c' for count only, 'n' for line numbers, 'v' for invert match)"
        }
      },
      "required": ["pattern"]
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
    "name": "grep",
    "arguments": "{\"pattern\":\"func main\",\"path\":\"./src\",\"flags\":[\"r\",\"n\"]}"
  }
}
```

### Parameters
- `pattern`: Search pattern (required, string)
  - Regular expression pattern to search for
  - Standard POSIX Extended Regular Expression syntax
- `path`: Path to search within (optional, string, defaults to current directory)
  - Can be a file path or a directory path
  - If a directory is specified, search all files within
  - If a file is specified, search only that file
- `flags`: Array of single-character grep-style flags (optional, array of strings)
  - `"i"`: Case-insensitive search
  - `"r"`: Recursive search (search subdirectories)
  - `"c"`: Show only count of matching lines per file (no matched lines)
  - `"n"`: Include line numbers in output (default: false)
  - `"v"`: Invert match (show non-matching lines)
  - `"l"`: Show only filenames with matches (no line content)
  - `"f"`: Use file as pattern source (pattern is interpreted as a file path containing patterns)
  - Multiple flags can be combined (e.g., `["r", "n", "i"]`)

### Examples

**Search for a pattern in a specific directory recursively:**
```json
{
  "id": "call_001",
  "type": "function",
  "function": {
    "name": "grep",
    "arguments": "{\"pattern\":\"func main\",\"path\":\"./src\",\"flags\":[\"r\",\"n\"]}"
  }
}
```

**Search in the current directory for case-insensitive pattern:**
```json
{
  "id": "call_002",
  "type": "function",
  "function": {
    "name": "grep",
    "arguments": "{\"pattern\":\"TODO\",\"flags\":[\"i\",\"r\",\"n\"]}"
  }
}
```

**Search a single file:**
```json
{
  "id": "call_003",
  "type": "function",
  "function": {
    "name": "grep",
    "arguments": "{\"pattern\":\"error\",\"path\":\"./src/main.go\"}"
  }
}
```

**Count matches per file:**
```json
{
  "id": "call_004",
  "type": "function",
  "function": {
    "name": "grep",
    "arguments": "{\"pattern\":\"import\",\"path\":\"./src\",\"flags\":[\"r\",\"c\"]}"
  }
}
```

**Show only filenames with matches:**
```json
{
  "id": "call_005",
  "type": "function",
  "function": {
    "name": "grep",
    "arguments": "{\"pattern\":\"import\",\"path\":\"./src\",\"flags\":[\"r\",\"l\"]}"
  }
}
```

## Return Values

On success (default output with line numbers):
```
src/main.go:10:func main() {
src/main.go:25:func mainHandler() {
src/server.go:5:func main() {
```

On success with count flag (`-c`):
```
src/main.go:3
src/server.go:2
src/utils.go:1
```

On success with only filenames flag (`-l`):
```
src/main.go
src/server.go
src/utils.go
```

On failure:
- `error`: Description of the error (path not found, permission denied, invalid pattern, etc.)
- `success`: `false`

### Output Format Details

When flags do NOT contain `"c"` or `"l"` (default), output is in grep format:
- `filepath:linenum:matched_line`
- Multiple matches in the same file each get their own line
- Line numbers are included when `"n"` flag is set (or by default in some implementations)

When `"c"` flag is set, output shows:
- `filepath:count`
- One line per file showing the number of matching lines

When `"l"` flag is set, output shows:
- One filename per line for files that have at least one match

## Implementation Notes

- The tool should use an efficient search algorithm for large directories
- When searching recursively (`"r"` flag), skip `.git` directories and hidden files by default (unless `"a"` flag is also provided)
- Large output should be truncated with a warning message if it exceeds a reasonable limit (e.g., 5000 lines)
- Binary files should be skipped by default with a message indicating they were skipped
- The tool should handle symlinks gracefully (follow or skip based on implementation)
