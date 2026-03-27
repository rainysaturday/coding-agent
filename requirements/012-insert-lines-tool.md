# Requirement 012: Insert Lines Tool

## Description
The harness must support an `insert_lines` tool that allows inserting a set of lines at a specified line number in a file.

## Acceptance Criteria
- [ ] Tool named `insert_lines` is available
- [ ] Accepts file path as input parameter
- [ ] Accepts line number as input parameter (1-indexed position to insert before)
- [ ] Accepts lines to insert as input parameter
- [ ] Supports multi-line content using raw mode markers
- [ ] Inserts lines before the specified line number
- [ ] Inserting at line 1 inserts at beginning of file
- [ ] Inserting beyond file end appends to end of file
- [ ] Existing lines are shifted down after insertion
- [ ] Creates file if it does not exist
- [ ] Handles permission errors gracefully
- [ ] Handles disk full errors gracefully
- [ ] Returns confirmation of insertion with details
- [ ] Tool call failures are tracked in statistics

## Tool Usage

### Standard Mode (single line)
```
[tool:insert_lines(path="/path/to/file.txt", line=5, lines="single line to insert")]
```

### Raw Mode (multi-line content)
```
[tool:insert_lines(path="/path/to/file.txt", line=5, lines=<<<RAW>>>
new line 1
new line 2
new line 3
<<<END_RAW>>>)]
```

### Parameters
- `path`: File path to modify (required)
- `line`: Line number to insert before (required, 1-indexed)
- `lines`: Lines to insert (required)
  - Use standard mode for single lines
  - Use raw mode (`<<<RAW>>>`...`<<<END_RAW>>>`) for multi-line content
