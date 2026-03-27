# Requirement 013: Replace Lines Tool

## Description
The harness must support a `replace_lines` tool that allows replacing a range of lines in a file with new lines.

## Acceptance Criteria
- [ ] Tool named `replace_lines` is available
- [ ] Accepts file path as input parameter
- [ ] Accepts start line number as input parameter (1-indexed)
- [ ] Accepts end line number as input parameter (1-indexed)
- [ ] Accepts replacement lines as input parameter
- [ ] Supports multi-line content using raw mode markers
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

### Standard Mode (single line)
```
[tool:replace_lines(path="/path/to/file.txt", start=1, end=3, lines="replacement line")]
```

### Raw Mode (multi-line content)
```
[tool:replace_lines(path="/path/to/file.txt", start=1, end=5, lines=<<<RAW>>>
replacement line 1
replacement line 2
replacement line 3
<<<END_RAW>>>)]
```

### Parameters
- `path`: File path to modify (required)
- `start`: Start line number (required, 1-indexed)
- `end`: End line number (required, 1-indexed)
- `lines`: Replacement lines (required)
  - Use standard mode for single lines
  - Use raw mode (`<<<RAW>>>`...`<<<END_RAW>>>`) for multi-line content
