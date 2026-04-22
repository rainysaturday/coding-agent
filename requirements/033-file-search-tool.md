# Requirement 033: File Search Tool (Glob)

## Description

The harness must support a `glob` tool that allows searching for files matching glob patterns. This tool enables the agent to discover files in the codebase for inspection, modification, or analysis. It supports:

- Simple patterns: `*.go`, `*.json`, `README.*`
- Directory-specific patterns: `src/*.go`, `config/*.yaml`
- Recursive patterns: `**/*.go`, `**/test_*.py`, `src/**/test.go`

The tool is implemented using Go's `filepath.Glob` for simple patterns and a custom recursive directory walker for `**` patterns.

## Acceptance Criteria

- [x] Tool named `glob` is available
- [x] Supports simple glob patterns (e.g., `*.go`, `*.json`)
- [x] Supports directory-prefixed patterns (e.g., `src/*.go`)
- [x] Supports recursive `**` patterns (e.g., `**/*.go`, `**/test.go`)
- [x] Returns file paths that match the pattern
- [x] Returns file metadata (size, modification time)
- [x] Supports `max_results` parameter to limit output
- [x] Returns empty result gracefully when no files match
- [x] Handles invalid patterns gracefully
- [x] Tool call failures are tracked in statistics
- [x] Results include file type (file vs directory)
- [x] Handles files with special characters in names
- [x] Handles permission errors on subdirectories gracefully
- [x] Supports absolute and relative patterns
- [x] Pattern matching respects case sensitivity

## Tool Definition (OpenAI Format)

```json
{
  "type": "function",
  "function": {
    "name": "glob",
    "description": "Search for files matching a glob pattern",
    "parameters": {
      "type": "object",
      "properties": {
        "pattern": {
          "type": "string",
          "description": "Glob pattern to search for (e.g., '*.go', 'src/**/*.ts', '**/test.js')"
        },
        "max_results": {
          "type": "integer",
          "description": "Maximum number of results to return (default: 100)"
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
  "id": "call_glob_001",
  "type": "function",
  "function": {
    "name": "glob",
    "arguments": "{\"pattern\":\"**/*.go\",\"max_results\":50}"
  }
}
```

### Parameters

- `pattern`: Glob pattern to search for (required, string)
  - Supports `*` for single-level wildcards
  - Supports `**` for recursive directory matching
  - Supports `?` for single character wildcards
  - Supports character classes `[abc]`
- `max_results`: Maximum number of results to return (optional, integer, default: 100)

## Return Values

### Success with Results

```json
{
  "success": true,
  "output": "Found 5 file(s) matching '*.go':\n\n  main.go (1234 bytes, modified 2024-01-15 10:30:00)\n  utils/helper.go (567 bytes, modified 2024-01-14 08:20:00)\n  utils/format.go (890 bytes, modified 2024-01-13 15:45:00)\n  config/settings.go (234 bytes, modified 2024-01-12 09:10:00)\n  handler/handler.go (1100 bytes, modified 2024-01-11 14:30:00)",
  "pattern": "*.go",
  "matchesFound": 5
}
```

### Success with No Results

```json
{
  "success": true,
  "output": "No files found matching pattern: nonexistent_*.xyz",
  "pattern": "nonexistent_*.xyz",
  "matchesFound": 0
}
```

### Failure

```json
{
  "success": false,
  "error": "missing required parameter: pattern"
}
```

## Behavior Notes

### Pattern Matching

| Pattern | Description | Matches |
|---------|-------------|---------|
| `*.go` | All .go files in current directory | `main.go`, `util.go` |
| `src/*.go` | All .go files in src/ | `src/main.go`, `src/handler.go` |
| `**/*.go` | All .go files recursively | `src/main.go`, `lib/util.go` |
| `**/test.go` | test.go at any depth | `src/test.go`, `lib/test.go` |
| `src/**/test.go` | test.go in any subdirectory of src/ | `src/lib/test.go` |
| `*.go` | All .go files in current directory | `main.go`, `util.go` |
| `src/**` | All files recursively in src/ | `src/main.go`, `src/lib/util.go` |

### Recursive Glob (`**`)

The `**` pattern is expanded recursively:

1. The pattern is split at the first `**` occurrence
2. Everything before `**` becomes the base directory
3. Everything after `**` becomes the file pattern
4. The directory walker searches all subdirectories

### Max Results Limiting

- Default limit: 100 results
- If results exceed limit, the first N results are returned
- The `matchesFound` field reflects the actual count before limiting

### Error Handling

| Scenario | Behavior |
|----------|----------|
| Invalid pattern | Returns error with glob error details |
| No matching files | Returns success with empty result |
| Permission denied | Skips inaccessible directories |
| Missing pattern parameter | Returns error |
| Invalid max_results | Falls back to default (100) |

## Implementation Requirements

### Simple Glob

For patterns without `**`:

1. Use Go's `filepath.Glob()` function
2. Return matching file paths with metadata
3. Handle errors from Glob

### Recursive Glob

For patterns with `**`:

1. Split pattern into base directory and remaining pattern
2. Walk the directory tree starting from base
3. Match each file against the remaining pattern
4. Collect all matches up to `max_results`
5. Skip directories from results (only files)

### File Metadata

For each match, provide:

- File path (relative or absolute)
- File size in bytes
- Last modification time
- Whether it's a directory

## Usage Patterns

### Pattern 1: Find All Go Files

```
User: "List all Go files in this project"

Agent:
1. Call glob with pattern="**/*.go"
2. Review the results
3. Use read_file to inspect specific files
```

### Pattern 2: Find Test Files

```
User: "Find all test files"

Agent:
1. Call glob with pattern="**/*_test.go"
2. Review the test file list
```

### Pattern 3: Find Config Files

```
User: "Where are the configuration files?"

Agent:
1. Call glob with pattern="**/*.json"
2. Call glob with pattern="**/*.yaml"
3. Present the configuration file locations
```

### Pattern 4: Find Files in Specific Directory

```
User: "Show me all TypeScript files in src/"

Agent:
1. Call glob with pattern="src/**/*.ts"
2. Present the results
```

## Testing Requirements

### Unit Tests

- [x] Simple glob pattern matching
- [x] Recursive glob pattern matching
- [x] Pattern with `**` at beginning
- [x] Pattern with `**` in middle
- [x] Pattern with `**` at end
- [x] No matching files returns empty result
- [x] Max results limiting works
- [x] Invalid pattern returns error
- [x] Missing pattern parameter returns error

### Integration Tests

- [x] Search in real directory structure
- [x] Handle nested directories
- [x] Handle permission-denied subdirectories
- [x] Handle files with spaces in names
- [x] Handle files with Unicode names
- [x] Verify file metadata accuracy
- [x] Verify max_results truncation

### Edge Cases

- [x] Empty directory
- [x] Very large directory trees
- [x] Symlinks
- [x] Hidden files (starting with `.`)
- [x] Pattern with no wildcards (exact path)
- [x] Absolute path patterns
- [x] Very deep directory nesting

## Performance Considerations

- **Directory Walking**: Use efficient recursive walking
- **Memory**: Limit results with `max_results` to avoid memory issues
- **Speed**: Skip directories that can't match the pattern
- **Large Trees**: Handle projects with thousands of files

## Security Considerations

- **Directory Traversal**: Only search within the current working directory
- **Permission Errors**: Gracefully handle permission-denied directories
- **Symlinks**: Follow symlinks but avoid infinite loops

## Related Requirements

- **005-read-file-tool.md**: Reading files after discovery
- **011-read-lines-tool.md**: Reading specific lines after discovery
- **013-replace-text-tool.md**: Modifying discovered files

## Acceptance Checklist

- [x] Tool named `glob` is available
- [x] Supports simple glob patterns (*, ?, [...])
- [x] Supports recursive ** patterns
- [x] Returns file paths matching the pattern
- [x] Returns file metadata (size, modification time, type)
- [x] Supports max_results parameter (default: 100)
- [x] Returns empty result when no files match
- [x] Handles invalid patterns gracefully
- [x] Handles permission errors gracefully
- [x] Supports absolute and relative patterns
- [x] Tool call failures are tracked in statistics
- [x] Implementation uses Go standard library only
- [x] Unit tests pass
- [x] Integration tests pass
