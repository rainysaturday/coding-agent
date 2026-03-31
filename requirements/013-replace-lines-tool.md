# Requirement 013: Replace Lines Tool

## Description

The harness must support a `replace_lines` tool that allows replacing content in a file. The tool supports two modes:

1. **Line-number mode**: Replace a specific range of lines
2. **Search-and-replace mode**: Find and replace content by matching text patterns

## Acceptance Criteria

- [ ] Tool named `replace_lines` is available
- [ ] Accepts file path as input parameter (required)
- [ ] **Line-number mode parameters**:
  - [ ] Accepts start line number (optional when using search mode)
  - [ ] Accepts end line number (optional when using search mode)
  - [ ] Accepts replacement lines as input parameter
- [ ] **Search-and-replace mode parameters**:
  - [ ] Accepts `search` parameter - text pattern to find (required when not using line numbers)
  - [ ] Accepts `replace` parameter - replacement text (required when using search)
  - [ ] Accepts `count` parameter - number of occurrences to replace (optional, defaults to 1)
- [ ] Replaces the specified content with new content
- [ ] **Line-number mode behavior**:
  - [ ] Handles start > end by returning error
  - [ ] Handles start line beyond file end by appending
  - [ ] Handles end line beyond file end by replacing to end
  - [ ] Replacing entire file with empty content is supported
- [ ] **Search-and-replace mode behavior**:
  - [ ] Finds first occurrence of search text
  - [ ] Replaces matching text with replacement text
  - [ ] Supports replacing multiple occurrences with `count` parameter
  - [ ] Returns error if search text not found
  - [ ] Returns number of replacements made
- [ ] Creates file if it does not exist (line-number mode only)
- [ ] Preserves file encoding and line endings
- [ ] Handles permission errors gracefully
- [ ] Handles disk full errors gracefully
- [ ] Returns confirmation of replacement with details
- [ ] Tool call failures are tracked in statistics

## Tool Usage

### Line-Number Mode

#### Single Line Replacement

```
[TOOL:{"name":"replace_lines","parameters":{"path":"/path/to/file.txt","start":1,"end":1,"lines":"replacement line"}}]
```

#### Multi-line Range Replacement

```
[TOOL:{"name":"replace_lines","parameters":{"path":"/path/to/file.txt","start":1,"end":5,"lines":"replacement line 1\nreplacement line 2\nreplacement line 3"}}]
```

### Search-and-Replace Mode

#### Replace First Occurrence

```
[TOOL:{"name":"replace_lines","parameters":{"path":"/path/to/file.txt","search":"old function name","replace":"new function name"}}]
```

#### Replace Multiple Occurrences

```
[TOOL:{"name":"replace_lines","parameters":{"path":"/path/to/file.txt","search":"TODO","replace":"IMPLEMENTED","count":5}}]
```

#### Replace Multi-line Block

```
[TOOL:{"name":"replace_lines","parameters":{"path":"/path/to/file.txt","search":"func oldHandler() {\n    // old code\n}","replace":"func newHandler() {\n    // new code\n}"}}]
```

## Parameters

### Required Parameters (either mode)

- `path`: File path to modify (required, string)

### Line-Number Mode Parameters (alternative to search mode)

- `start`: Start line number (required when not using search, integer, 1-indexed)
- `end`: End line number (required when not using search, integer, 1-indexed)
- `lines`: Replacement lines (required when not using search, string)
  - Multi-line content uses `\n` escape sequences
  - All special characters must be JSON-escaped

### Search-and-Replace Mode Parameters (alternative to line numbers)

- `search`: Text pattern to find (required when not using start/end, string)
  - Supports multi-line patterns with `\n` escape sequences
  - Exact string matching (not regex)
  - Case-sensitive matching
- `replace`: Replacement text (required when using search, string)
  - Multi-line content uses `\n` escape sequences
  - All special characters must be JSON-escaped
- `count`: Number of occurrences to replace (optional, integer, defaults to 1)
  - Use -1 or "all" to replace all occurrences

## Examples

### Line-Number Mode Examples

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

### Search-and-Replace Mode Examples

**Rename a variable:**

```
[TOOL:{"name":"replace_lines","parameters":{"path":"./main.go","search":"oldVariableName","replace":"newVariableName"}}]
```

**Update a configuration value:**

```
[TOOL:{"name":"replace_lines","parameters":{"path":"./config.yaml","search":"debug: false","replace":"debug: true"}}]
```

**Replace a function implementation:**

```
[TOOL:{"name":"replace_lines","parameters":{"path":"./handlers.go","search":"func fetchData() string {\n    return \"old\"\n}","replace":"func fetchData() string {\n    return \"new\"\n}"}}]
```

**Replace all TODOs:**

```
[TOOL:{"name":"replace_lines","parameters":{"path":"./src/main.go","search":"// TODO:","replace":"// IMPLEMENTED:","count":-1}}]
```

## Return Values

### On Success

#### Line-Number Mode

- `success`: `true`
- `path`: The path that was modified
- `start`: The start line that was replaced
- `end`: The end line that was replaced
- `linesReplaced`: Number of lines that were replaced
- `linesInserted`: Number of new lines inserted

#### Search-and-Replace Mode

- `success`: `true`
- `path`: The path that was modified
- `search`: The search pattern that was used
- `replacementsMade`: Number of replacements made
- `totalOccurrences`: Total occurrences found in file

### On Failure

- `error`: Description of the error
- `success`: `false`

## Behavior Notes

### Line-Number Mode

- Line numbers are 1-indexed
- `start` must be <= `end`
- If `start` is beyond file length, appends new content
- If `end` is beyond file length, replaces to end of file
- Empty `lines` parameter effectively deletes the line range
- Replacing with empty string and range 1 to large number clears file

### Search-and-Replace Mode

- Search is case-sensitive
- Multi-line patterns are supported (use `\n` for newlines in JSON)
- If `count` is not specified, only the first occurrence is replaced
- If `count` is -1 or "all", all occurrences are replaced
- If search text is not found, returns an error
- If search text appears in multiple places, only `count` occurrences are replaced starting from the first match

## Recommendation for LLMs

**When to use Search-and-Replace mode:**

- When you know the exact text to find but not the line numbers
- For simple find-and-replace operations
- When renaming variables, functions, or configuration values
- When the file is large and counting lines is error-prone

**When to use Line-Number mode:**

- When you need precise control over line ranges
- When you've already read the file and know the exact line numbers
- For operations that affect specific line ranges (e.g., "replace lines 50-100")

**Best practices:**

1. Always read the file first using `read_file` or `read_lines` to understand its contents
2. For search-and-replace, use a unique search pattern to avoid unintended replacements
3. Verify changes by reading the file after replacement
4. For multi-line replacements, ensure proper JSON escaping of newlines (`\n`)
