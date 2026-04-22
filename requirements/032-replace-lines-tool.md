# Requirement 032: Replace Lines Tool

## Description

The harness must support a `replace_lines` tool that allows replacing lines in a file. This tool provides two modes of operation:

1. **Line-number mode**: Replace lines by specifying start and end line numbers with replacement content
2. **Search-and-replace mode**: Find text by searching for a pattern and replace it with new content

This tool combines the functionality of `replace_text` with line-range targeting for more precise file modifications.

## Acceptance Criteria

- [x] Tool named `replace_lines` is available
- [x] Supports line-number mode with start/end parameters and replacement content
- [x] Supports search-and-replace mode with search/replace parameters
- [x] Line-number mode uses 1-indexed line numbers
- [x] Search-and-replace mode supports count parameter for limiting replacements
- [x] Handles files that don't exist (creates parent directories)
- [x] Handles line numbers beyond file length (appends)
- [x] Handles empty files
- [x] Returns success/failure status with details
- [x] Provides clear error messages for invalid parameters
- [x] Tool call failures are tracked in statistics
- [x] Preserves file permissions when writing
- [x] Creates parent directories if they don't exist
- [x] Supports replacing single lines and ranges of lines
- [x] Supports replacing with multi-line content
- [x] Search-and-replace mode defaults to replacing first occurrence
- [x] Search-and-replace mode supports replacing all occurrences

## Tool Definition (OpenAI Format)

### Line-Number Mode

```json
{
  "type": "function",
  "function": {
    "name": "replace_lines",
    "description": "Replace lines in a file by line number range",
    "parameters": {
      "type": "object",
      "properties": {
        "path": {
          "type": "string",
          "description": "Path to the file to modify"
        },
        "start": {
          "type": "integer",
          "description": "Starting line number (1-indexed)"
        },
        "end": {
          "type": "integer",
          "description": "Ending line number (1-indexed, inclusive)"
        },
        "lines": {
          "type": "string",
          "description": "Replacement lines content (newlines separate lines)"
        }
      },
      "required": ["path", "start", "end", "lines"]
    }
  }
}
```

### Search-and-Replace Mode

```json
{
  "type": "function",
  "function": {
    "name": "replace_lines",
    "description": "Search and replace text in a file",
    "parameters": {
      "type": "object",
      "properties": {
        "path": {
          "type": "string",
          "description": "Path to the file to modify"
        },
        "search": {
          "type": "string",
          "description": "Text to search for"
        },
        "replace": {
          "type": "string",
          "description": "Text to replace with"
        },
        "count": {
          "type": "integer",
          "description": "Number of occurrences to replace (default: 1, use -1 or 'all' for all)"
        }
      },
      "required": ["path", "search", "replace"]
    }
  }
}
```

## Tool Call Format

### Line-Number Mode Example

```json
{
  "id": "call_replace_lines_001",
  "type": "function",
  "function": {
    "name": "replace_lines",
    "arguments": "{\"path\":\"main.go\",\"start\":10,\"end\":15,\"lines\":\"func NewHandler() *Handler {\\n    return &Handler{}\\n}\"}"
  }
}
```

### Search-and-Replace Mode Example

```json
{
  "id": "call_replace_lines_002",
  "type": "function",
  "function": {
    "name": "replace_lines",
    "arguments": "{\"path\":\"config.go\",\"search\":\"OldConfig\",\"replace\":\"NewConfig\",\"count\":1}"
  }
}
```

### Parameters (Line-Number Mode)

- `path`: Path to the file to modify (required, string)
- `start`: Starting line number, 1-indexed (required, integer)
- `end`: Ending line number, 1-indexed, inclusive (required, integer)
- `lines`: Replacement content, newlines separate lines (required, string)

### Parameters (Search-and-Replace Mode)

- `path`: Path to the file to modify (required, string)
- `search`: Text to search for (required, string)
- `replace`: Text to replace with (required, string)
- `count`: Number of occurrences to replace (optional, integer, default: 1)

## Return Values

### Line-Number Mode Success

```json
{
  "success": true,
  "output": "Replaced lines 10-15 with 2 line(s) in: main.go",
  "path": "main.go",
  "start": 10,
  "end": 15,
  "linesReplaced": 6,
  "linesInserted": 2
}
```

### Search-and-Replace Mode Success

```json
{
  "success": "true",
  "output": "Replaced 'OldConfig' with 'NewConfig' 3 time(s) in: config.go",
  "path": "config.go",
  "search": "OldConfig",
  "replacementsMade": 3,
  "totalOccurrences": 5
}
```

### Failure

```json
{
  "success": false,
  "error": "search text not found: OldConfig",
  "path": "config.go"
}
```

## Behavior Notes

### Mode Detection

The tool automatically determines which mode to use based on parameters:

1. **Line-number mode**: When `start` and `end` parameters are provided
2. **Search-and-replace mode**: When `search` parameter is provided

If neither mode's parameters are provided, an error is returned.

### Line-Number Mode Behavior

1. Lines from `start` to `end` (inclusive, 1-indexed) are replaced
2. If `start` > `end`, an error is returned
3. If `start` is beyond file length, lines are appended to end
4. If `end` is beyond file length, it's clamped to file length
5. The replacement content replaces the specified range

### Search-and-Replace Mode Behavior

1. Finds all occurrences of `search` text in the file
2. Replaces the first `count` occurrences (default: 1)
3. If `count` is -1 or "all", replaces all occurrences
4. Returns total number of replacements made
5. Returns total number of occurrences found

### Edge Cases

| Scenario | Behavior |
|----------|----------|
| File doesn't exist | Creates parent directories, appends content |
| Start line beyond file | Appends to end of file |
| End line beyond file | Clamps to file length |
| Empty file (line-number mode) | Appends replacement lines |
| Start > End | Returns error |
| Search text not found | Returns error with message |
| Count > occurrences | Replaces all occurrences |

## Implementation Requirements

### Line-Number Mode

The implementation must:

1. Read the target file into memory
2. Split into lines (handling trailing newline)
3. Validate start/end line numbers
4. Replace the specified range with new lines
5. Join lines back and write to file
6. Preserve file permissions

### Search-and-Replace Mode

The implementation must:

1. Read the target file into memory
2. Count total occurrences of search text
3. Perform replacement up to `count` occurrences
4. Write modified content back to file
5. Return replacement statistics

## Usage Patterns

### Pattern 1: Replacing a Function

```
User: "Update the GetConfig function to include logging"

Agent:
1. Read the file to find the function
2. Determine the start and end line numbers
3. Call replace_lines with the new function body
4. Verify the change was applied
```

### Pattern 2: Batch Text Replacement

```
User: "Replace all instances of 'TODO' with 'FIXME' in main.go"

Agent:
1. Call replace_lines with search="TODO", replace="FIXME", count=-1
2. Verify replacements were made
```

### Pattern 3: Fixing a Single Line

```
User: "Change the variable name from 'tmp' to 'tempDir' in line 42"

Agent:
1. Call replace_lines with start=42, end=42, lines="tempDir := getTempPath()"
2. Verify the change
```

## Testing Requirements

### Unit Tests

- [x] Replace a single line by number
- [x] Replace multiple lines by number range
- [x] Replace lines beyond file end (append)
- [x] Replace in empty file (append)
- [x] Replace with multi-line content
- [x] Search and replace first occurrence
- [x] Search and replace all occurrences
- [x] Search and replace limited count
- [x] Search text not found returns error
- [x] Invalid line numbers return error
- [x] Start > end returns error

### Integration Tests

- [x] Replace lines in existing file
- [x] Replace lines in file with no trailing newline
- [x] Replace creates missing parent directories
- [x] Preserve file permissions
- [x] Verify file content after replacement
- [x] Handle large files
- [x] Handle Unicode content

### Edge Cases

- [x] Line numbers equal (replace single line)
- [x] Line numbers beyond file length
- [x] Empty replacement lines
- [x] Search text appears at file start
- [x] Search text appears at file end
- [x] Overlapping search occurrences

## Security Considerations

- **Path Traversal**: Validate file paths to prevent directory traversal
- **File Permissions**: Preserve original file permissions
- **Large Files**: Handle files efficiently without excessive memory usage

## Performance Considerations

- **In-Memory Processing**: File is read entirely into memory
- **String Operations**: Use efficient string manipulation for replacements
- **File Write**: Atomic write to prevent partial writes

## Related Requirements

- **005-read-file-tool.md**: Reading files before modification
- **006-write-file-tool.md**: Alternative for full file replacement
- **011-read-lines-tool.md**: Reading specific lines for context
- **012-insert-lines-tool.md**: Alternative for simple insertions
- **013-replace-text-tool.md**: Similar search-and-replace functionality
- **033-file-search-tool.md**: Find files before modifying them

## Acceptance Checklist

- [x] Tool named `replace_lines` is available
- [x] Supports line-number mode (start/end/lines)
- [x] Supports search-and-replace mode (search/replace/count)
- [x] Line numbers are 1-indexed
- [x] Handles files that don't exist
- [x] Creates parent directories as needed
- [x] Handles line numbers beyond file length
- [x] Returns success/failure status
- [x] Provides clear error messages
- [x] Tracks failures in statistics
- [x] Preserves file permissions
- [x] Supports multi-line replacement content
- [x] Search mode supports count limiting
- [x] Search mode defaults to first occurrence
- [x] Implementation uses Go standard library only
- [x] Unit tests pass
- [x] Integration tests pass
