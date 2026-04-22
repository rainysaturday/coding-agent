# Requirement 032: List Files Tool

## Description
The harness must support a `list_files` tool that allows listing directory contents via OpenAI's tool calling interface, formatted like the `ls` command. It should display files, directories, sizes, permissions, timestamps, and other metadata.

## Acceptance Criteria
- [ ] Tool named `list_files` is available
- [ ] Accepts directory path as input parameter
- [ ] Accepts optional flags for extended output (similar to ls flags: -l, -a, -h, -t, -S, etc.)
- [ ] Returns directory listing with file/folder names, permissions, sizes, and timestamps
- [ ] Handles directory not found errors gracefully
- [ ] Handles permission errors gracefully
- [ ] Supports recursive listing option
- [ ] Distinguishes between files and directories in output
- [ ] Tool call failures are tracked in statistics

## Tool Definition (OpenAI Format)

```json
{
  "type": "function",
  "function": {
    "name": "list_files",
    "description": "List files and directories in a path, similar to the ls command",
    "parameters": {
      "type": "object",
      "properties": {
        "path": {
          "type": "string",
          "description": "Path to the file or directory to list (defaults to current directory if not specified)"
        },
        "flags": {
          "type": "array",
          "items": {
            "type": "string"
          },
          "description": "List of ls-style flags to control output (e.g., 'l' for long format, 'a' for all including hidden, 'h' for human-readable sizes, 't' for time-sorted, 'S' for size-sorted)"
        }
      },
      "required": []
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
    "name": "list_files",
    "arguments": "{\"path\":\"/workspace\",\"flags\":[\"l\",\"a\"]}"
  }
}
```

### Parameters
- `path`: Path to list (optional, string, defaults to current directory)
  - Can be an absolute or relative path
  - If the path points to a file, returns information about that single file
- `flags`: Array of single-character ls-style flags (optional, array of strings)
  - `"l"`: Long format (permissions, owner, size, timestamp, name)
  - `"a"`: Show hidden files (those starting with `.`)
  - `"h"`: Human-readable file sizes (e.g., `1.2K`, `3.4M`)
  - `"t"`: Sort by modification time, newest first
  - `"S"`: Sort by file size, largest first
  - `"r"`: Reverse the order of sort
  - Multiple flags can be combined (e.g., `["l", "a", "h"]`)
  - If no flags are provided, defaults to a simple list of names

### Examples

**Simple directory listing (default format):**
```json
{
  "id": "call_001",
  "type": "function",
  "function": {
    "name": "list_files",
    "arguments": "{\"path\":\"/workspace\"}"
  }
}
```

**Long format with human-readable sizes:**
```json
{
  "id": "call_002",
  "type": "function",
  "function": {
    "name": "list_files",
    "arguments": "{\"path\":\"/workspace\",\"flags\":[\"l\",\"h\"]}"
  }
}
```

**Show all files including hidden, sorted by time:**
```json
{
  "id": "call_003",
  "type": "function",
  "function": {
    "name": "list_files",
    "arguments": "{\"path\":\".\",\"flags\":[\"l\",\"a\",\"t\"]}"
  }
}
```

**List current directory with default flags:**
```json
{
  "id": "call_004",
  "type": "function",
  "function": {
    "name": "list_files",
    "arguments": "{}"
  }
}
```

## Return Values

On success with default format (no flags):
```
file1.txt
file2.txt
directory1/
directory2/
```

On success with long format (`-l`):
```
drwxr-xr-x  2 user  staff   4096 Apr 22 10:30  directory1/
-rw-r--r--  1 user  staff  12288 Apr 21 15:45  file1.txt
-rw-r--r--  1 user  staff   2048 Apr 20 09:12  file2.txt
```

On success with long format and human-readable sizes (`-lh`):
```
drwxr-xr-x  2 user  staff  4.0K Apr 22 10:30  directory1/
-rw-r--r--  1 user  staff   12K Apr 21 15:45  file1.txt
-rw-r--r--  1 user  staff  2.0K Apr 20 09:12  file2.txt
```

On failure:
- `error`: Description of the error (directory not found, permission denied, etc.)
- `success`: `false`

### Output Format Details

When `flags` contains `"l"` (long format), each line contains:
- **Permissions**: File type and Unix permissions (e.g., `drwxr-xr-x` for directory, `-rw-r--r--` for regular file)
- **Link count**: Number of hard links
- **Owner**: User name
- **Group**: Group name
- **Size**: File size in bytes (or human-readable if `-h` flag is set)
- **Modified**: Last modification timestamp
- **Name**: File or directory name (directories are suffixed with `/`)

When `flags` does NOT contain `"l"` (default/simple format), output is one entry per line:
- Regular files: just the name
- Directories: name with trailing `/`
