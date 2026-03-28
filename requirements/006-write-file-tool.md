# Requirement 006: Write File Tool

## Description
The harness must support a `write_file` tool that allows writing contents to files.

## Acceptance Criteria
- [ ] Tool named `write_file` is available
- [ ] Accepts file path and content as input parameters
- [ ] Writes content to specified file
- [ ] Creates file if it does not exist
- [ ] Overwrites file if it exists
- [ ] Handles permission errors gracefully
- [ ] Handles disk full errors gracefully
- [ ] Tool call failures are tracked in statistics

## Tool Usage

### Single-line Content
```
[TOOL:{"name":"write_file","parameters":{"path":"/path/to/file.txt","content":"Hello World"}}]
```

### Multi-line Content
```
[TOOL:{"name":"write_file","parameters":{"path":"/path/to/file.txt","content":"line 1\nline 2\nline 3"}}]
```

### Parameters
- `path`: Path to the file to write (required, string)
- `content`: Content to write to the file (required, string)
  - Multi-line content uses `\n` escape sequences
  - All special characters must be JSON-escaped

### Examples

**Write simple text:**
```
[TOOL:{"name":"write_file","parameters":{"path":"/tmp/hello.txt","content":"Hello, World!"}}]
```

**Write a script:**
```
[TOOL:{"name":"write_file","parameters":{"path":"/tmp/script.sh","content":"#!/bin/bash\necho \"Hello\"\nexit 0"}}]
```

**Write code:**
```
[TOOL:{"name":"write_file","parameters":{"path":"./main.go","content":"package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"Hello\")\n}"}}]
```

## Return Values

On success:
- `success`: `true`
- `path`: The path that was written

On failure:
- `error`: Description of the error (permission denied, disk full, etc.)
- `success`: `false`
