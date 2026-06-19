# Requirement 045: Move Text Tool

## Description

The harness must support a `move_text` tool that allows the LLM to move text blocks between lines in the same file or to other files. This tool combines read and write operations into a single atomic action, enabling efficient text reorganization across files. The tool automatically creates target files if they do not exist.

## Motivation

Moving text blocks is a common refactoring operation that currently requires multiple tool calls (read_lines + delete + insert/write). The `move_text` tool consolidates this into a single operation, reducing context switching and improving reliability by ensuring atomicity.

## Acceptance Criteria

### Core Functionality
- [ ] Tool named `move_text` is available
- [ ] Accepts source file path as input parameter (required)
- [ ] Accepts source start line as input parameter (required, 1-indexed)
- [ ] Accepts source end line as input parameter (required, 1-indexed, inclusive)
- [ ] Accepts target file path as input parameter (required)
- [ ] Accepts target line number as input parameter (required, 1-indexed insertion point)
- [ ] Moves the specified text block from source to target
- [ ] Removes the text block from the source file after copying
- [ ] Inserts the text block at the specified target line
- [ ] Works within the same file (move lines within a file)
- [ ] Works across different files (move lines between files)
- [ ] Automatically creates target file if it does not exist
- [ ] Automatically creates parent directories for target file if needed
- [ ] Handles edge cases (empty source range, target beyond file end, etc.)
- [ ] Returns the moved content in the response for verification
- [ ] Tool call failures are tracked in statistics

### Error Handling
- [ ] Returns error if source file does not exist
- [ ] Returns error if source line range is invalid (start > end)
- [ ] Returns error if source line range exceeds file length
- [ ] Returns error if source file is empty and lines are requested
- [ ] Handles permission errors gracefully
- [ ] Handles disk full errors gracefully

## Tool Definition (OpenAI Format)

```json
{
  "type": "function",
  "function": {
    "name": "move_text",
    "description": "Move text block(s) from one location to another. Can move within the same file or between files. Source lines are removed and inserted at the target location. Target file is created if it doesn't exist.",
    "parameters": {
      "type": "object",
      "properties": {
        "source_path": {
          "type": "string",
          "description": "Path to the source file containing the text to move"
        },
        "source_start": {
          "type": "integer",
          "description": "Starting line number in source file (1-indexed, inclusive)"
        },
        "source_end": {
          "type": "integer",
          "description": "Ending line number in source file (1-indexed, inclusive)"
        },
        "target_path": {
          "type": "string",
          "description": "Path to the target file where text will be moved"
        },
        "target_line": {
          "type": "integer",
          "description": "Line number in target file to insert before (1-indexed)"
        }
      },
      "required": ["source_path", "source_start", "source_end", "target_path", "target_line"]
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
    "name": "move_text",
    "arguments": "{\"source_path\":\"./src/main.go\",\"source_start\":5,\"source_end\":10,\"target_path\":\"./src/utils.go\",\"target_line\":1}"
  }
}
```

### Parameters

- `source_path`: Path to the source file (required, string)
  - File must exist
  - Relative and absolute paths supported
- `source_start`: Starting line number (required, integer, 1-indexed)
  - Must be >= 1
  - Must be <= source_end
- `source_end`: Ending line number (required, integer, 1-indexed)
  - Must be >= source_start
  - Must be <= total lines in source file
- `target_path`: Path to the target file (required, string)
  - File is created if it doesn't exist
  - Parent directories are created automatically
  - Can be the same as source_path (for intra-file moves)
- `target_line`: Line number to insert before (required, integer, 1-indexed)
  - Inserting at line 1 puts content at the beginning
  - Inserting beyond file end appends to the end

### Examples

**Move lines within the same file (reorder code):**

Move lines 10-15 to line 3 (reordering within the file):
```json
{
  "id": "call_001",
  "type": "function",
  "function": {
    "name": "move_text",
    "arguments": "{\"source_path\":\"./script.py\",\"source_start\":10,\"source_end\":15,\"target_path\":\"./script.py\",\"target_line\":3}"
  }
}
```

**Move a function to a new file:**

Extract lines 20-45 to a new utility file:
```json
{
  "id": "call_002",
  "type": "function",
  "function": {
    "name": "move_text",
    "arguments": "{\"source_path\":\"./src/main.go\",\"source_start\":20,\"source_end\":45,\"target_path\":\"./src/utils.go\",\"target_line\":1}"
  }
}
```

**Move header comment to beginning of file:**

Move lines 50-55 to line 1 (promote comments to top):
```json
{
  "id": "call_003",
  "type": "function",
  "function": {
    "name": "move_text",
    "arguments": "{\"source_path\":\"./README.md\",\"source_start\":50,\"source_end\":55,\"target_path\":\"./README.md\",\"target_line\":1}"
  }
}
```

**Move code block to nested new file:**

Create nested directories and move content:
```json
{
  "id": "call_004",
  "type": "function",
  "function": {
    "name": "move_text",
    "arguments": "{\"source_path\":\"./config.yaml\",\"source_start\":10,\"source_end\":20,\"target_path\":\"./config/staging/overrides.yaml\",\"target_line\":1}"
  }
}
```

## Return Values

### On Success

```json
{
  "success": true,
  "path": "<target_path>",
  "output": "Moved 6 line(s) from <source_path> (lines 10-15) to <target_path> (line 3)\n--- Moved content ---\n<first 10 lines of moved content>",
  "extra": {
    "sourcePath": "<source_path>",
    "sourceStart": 10,
    "sourceEnd": 15,
    "targetPath": "<target_path>",
    "targetLine": 3,
    "linesMoved": 6,
    "content": "<full moved content>"
  }
}
```

Fields:
- `success`: `true`
- `path`: The target path that was modified
- `output`: Human-readable summary of the operation
- `extra.sourcePath`: Source file path
- `extra.sourceStart`: Source start line (1-indexed)
- `extra.sourceEnd`: Source end line (1-indexed)
- `extra.targetPath`: Target file path
- `extra.targetLine`: Target insertion line
- `extra.linesMoved`: Number of lines moved
- `extra.content`: Full content that was moved (for verification)

### On Failure

```json
{
  "success": false,
  "error": "<error description>"
}
```

Example errors:
- `"source file not found: ./nonexistent.txt"`
- `"invalid line range: source_start (10) > source_end (5)"`
- `"source line range out of bounds: requested lines 10-15 but file has only 8 lines"`
- `"permission denied: ./readonly/file.txt"`

## Behavior Notes

### Same-File Moves

When source and target are the same file:
1. Lines are extracted from source position
2. Remaining lines are renumbered
3. Lines are inserted at adjusted target position
4. If target_line was after source range, it's adjusted downward by the number of removed lines

Example: In a file with 20 lines, moving lines 5-8 to line 2:
- Extract lines 5-8
- Remaining: lines 1-4, 9-20 (now 16 lines)
- Insert at line 2: lines 1, [moved 5-8], lines 4, 9-20

### Cross-File Moves

When source and target are different files:
1. Lines are read from source file
2. Lines are removed from source file
3. Target file is created if it doesn't exist
4. Lines are inserted at target position

### Empty Target File

If the target file doesn't exist:
- It is created with the moved content
- Parent directories are created recursively if needed
- Target line is ignored (content becomes the entire file)

### Edge Cases

- Moving a single line: `source_start == source_end`
- Moving to beginning: `target_line == 1`
- Moving to end: `target_line` beyond file length appends
- Moving all lines from source: Source file becomes empty (but still exists)
- Source range with trailing empty line: Empty line at end of range is preserved

## Recommendation for LLMs

**When to use move_text:**

- When reorganizing code within a file (e.g., moving functions, reordering sections)
- When extracting code from one file to another
- When consolidating scattered related code into one location
- When cleaning up temporary code blocks and moving them to permanent locations

**Best practices:**

1. Always read the file first using `read_file` or `read_lines` to understand line numbers
2. For same-file moves, account for line number changes after removal
3. Verify the move by reading both files after the operation
4. Use descriptive target paths that indicate the purpose of the moved content
5. Consider using `replace_text` for simple text transformations instead

**When NOT to use move_text:**

- Use `replace_text` for simple find-and-replace operations
- Use `insert_lines` when you just need to add content without removing anything
- Use `write_file` when completely overwriting a file's contents
