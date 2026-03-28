# Requirement 013: Replace Lines Tool

## Description
The harness must support a `replace_lines` tool that allows replacing a range of lines in a file with new lines.

## Acceptance Criteria
- [ ] Tool named `replace_lines` is available
- [ ] Accepts file path as input parameter
- [ ] Accepts start line number as input parameter (1-indexed)
- [ ] Accepts end line number as input parameter (1-indexed)
- [ ] Accepts replacement lines as input parameter
- [ ] Replaces the specified line range with new lines
- [ ] Handles start > end by returning error
- [ ] Handles start line beyond file end by appending
- [ ] Handles end line beyond file end by replacing to end
- [ ] Replacing entire file with empty content is supported
- [ ] Creates file if it does not exist (when replacing non-existent file)
- [ ] Preserves file encoding and line endings
- [ ] Handles permission errors gracefully
- [ ] Handles disk full errors gracefully
- [ ] Returns confirmation of replacement with details
- [ ] Tool call failures are tracked in statistics

## Tool Usage

### Single Line Replacement
```
[TOOL:{"name":"replace_lines","parameters":{"path":"/path/to/file.txt","start":1,"end":3,"lines":"replacement line"}}]
```

### Multi-line Replacement
```
[TOOL:{"name":"replace_lines","parameters":{"path":"/path/to/file.txt","start":1,"end":5,"lines":"replacement line 1\nreplacement line 2\nreplacement line 3"}}]
```

### Parameters
- `path`: File path to modify (required, string)
- `start`: Start line number (required, integer, 1-indexed)
- `end`: End line number (required, integer, 1-indexed)
- `lines`: Replacement lines (required, string)
  - Multi-line content uses `\n` escape sequences
  - All special characters must be JSON-escaped

### Examples

**Replace first few lines:**
```
[TOOL:{"name":"replace_lines","parameters":{"path":"./config.txt","start":1,"end":2,"lines":"new config\nupdated setting"}}]
```

**Replace entire file:**
```
[TOOL:{"name":"replace_lines","parameters":{"path":"./old.txt","start":1,"end":100,"lines":"completely new content"}}]
```

**Clear file content:**
```
[TOOL:{"name":"replace_lines","parameters":{"path":"./temp.txt","start":1,"end":9999,"lines":""}}]
```

## Return Values

On success:
- `success`: `true`
- `path`: The path that was modified
- `start`: The start line that was replaced
- `end`: The end line that was replaced
- `linesReplaced`: Number of lines that were replaced
- `linesInserted`: Number of new lines inserted

On failure:
- `error`: Description of the error
- `success`: `false`

## Behavior Notes

- Line numbers are 1-indexed
- `start` must be <= `end`
- If `start` is beyond file length, appends new content
- If `end` is beyond file length, replaces to end of file
- Empty `lines` parameter effectively deletes the line range
- Replacing with empty string and range 1 to large number clears file
