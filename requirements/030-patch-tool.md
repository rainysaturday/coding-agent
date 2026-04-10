# Requirement 030: Patch Tool

## Description

The harness must support a `patch` tool that allows updating files based on standard diff blocks (unified diff format). This tool enables the agent to make precise code modifications by applying diff patches to existing files. It should be implemented by a CLI call to the system standard `patch` cli tool.

## Acceptance Criteria

- [ ] Tool named `patch` is available
- [ ] Accepts file path as input parameter
- [ ] Accepts diff content as input parameter (unified diff format)
- [ ] Applies the diff patch to the specified file
- [ ] Validates the patch can be applied before making changes
- [ ] Returns success/failure status with details
- [ ] Provides clear error messages when patch fails
- [ ] Handles patches that don't match file content gracefully
- [ ] Handles file not found errors gracefully
- [ ] Handles permission errors gracefully
- [ ] Handles invalid diff format gracefully
- [ ] Tool call failures are tracked in statistics
- [ ] Supports multiple hunk patches in a single diff
- [ ] Supports context lines in patches
- [ ] Supports addition (+), deletion (-), and context ( ) lines

## Tool Definition (OpenAI Format)

```json
{
  "type": "function",
  "function": {
    "name": "patch",
    "description": "Apply a unified diff patch to a file",
    "parameters": {
      "type": "object",
      "properties": {
        "path": {
          "type": "string",
          "description": "Path to the file to patch"
        },
        "diff": {
          "type": "string",
          "description": "Unified diff content to apply (diff -u format)"
        }
      },
      "required": ["path", "diff"]
    }
  }
}
```

## Tool Call Format

```json
{
  "id": "call_patch123",
  "type": "function",
  "function": {
    "name": "patch",
    "arguments": "{\"path\":\"/path/to/file.go\",\"diff\":\"--- a/file.go\\n+++ b/file.go\\n@@ -10,7 +10,8 @@\\n func main() {\\n-    fmt.Println(\\\"hello\\\")\\n+    fmt.Println(\\\"hello world\\\")\\n+    fmt.Println(\\\"done\\\")\\n }\\n\"}"
  }
}
```

### Parameters

- `path`: Path to the file to patch (required, string)
- `diff`: Unified diff content to apply (required, string, diff -u format)

### Examples

**Simple single-line change:**

```json
{
  "id": "call_001",
  "type": "function",
  "function": {
    "name": "patch",
    "arguments": "{\"path\":\"main.go\",\"diff\":\"--- a/main.go\\n+++ b/main.go\\n@@ -5 +5 @@\\n-    oldLine()\\n+    newLine()\\n\"}"
  }
}
```

**Multiple hunks:**

```json
{
  "id": "call_002",
  "type": "function",
  "function": {
    "name": "patch",
    "arguments": "{\"path\":\"utils.go\",\"diff\":\"--- a/utils.go\\n+++ b/utils.go\\n@@ -10,3 +10,4 @@\\n func Helper() {\\n     // context\\n+    // new comment\\n     oldCode()\\n@@ -20,3 +21,3 @@\\n func Another() {\\n-    old()\\n+    updated()\\n }\\n\"}"
  }
}
```

**Adding new lines:**

```json
{
  "id": "call_003",
  "type": "function",
  "function": {
    "name": "patch",
    "arguments": "{\"path\":\"README.md\",\"diff\":\"--- a/README.md\\n+++ b/README.md\\n@@ -5,0 +6,2 @@\\n+## New Section\\n+\\n+This is new content.\\n\"}"
  }
}
```

**Deleting lines:**

```json
{
  "id": "call_004",
  "type": "function",
  "function": {
    "name": "patch",
    "arguments": "{\"path\":\"config.yaml\",\"diff\":\"--- a/config.yaml\\n+++ b/config.yaml\\n@@ -10,2 +10,0 @@\\n-old_setting: value\\n-# deprecated\\n\"}"
  }
}
```

## Diff Format Specification

The patch tool accepts standard unified diff format (as produced by `diff -u` or `git diff`):

```
--- a/original_path
+++ b/modified_path
@@ -start,count +start,count @@
 context line
-removed line
+added line
 context line
```

### Diff Components

| Symbol      | Meaning                       |
| ----------- | ----------------------------- |
| ` ` (space) | Context line (unchanged)      |
| `-`         | Line to remove                |
| `+`         | Line to add                   |
| `@@ ... @@` | Hunk header with line numbers |

### Hunk Header Format

```
@@ -start,count +start,count @@
```

- `-start,count`: Original file position (start line, number of lines)
- `+start,count`: New file position (start line, number of lines)

## Return Values

On success:

- `output`: Summary of changes applied (e.g., "Applied 1 hunk to file")
- `success`: `true`
- `patches_applied`: Number of hunks successfully applied

On failure:

- `error`: Description of the error
- `success`: `false`
- `details`: Additional context about why the patch failed

### Success Example

```json
{
  "success": true,
  "output": "Applied 1 hunk to main.go",
  "patches_applied": 1
}
```

### Failure Example

```json
{
  "success": false,
  "error": "Patch does not match file content",
  "details": "Hunk 1 at line 10 failed: expected 'oldLine()' but found 'differentCode()'"
}
```

## Behavior Notes

### Patch Application

1. **Validation**: The tool first validates that the diff format is valid
2. **File Check**: Verifies the target file exists and is readable
3. **Context Matching**: Each hunk's context lines are matched against the file
4. **Atomic Application**: All hunks in a patch are applied together
5. **Error Reporting**: If any hunk fails, the entire patch fails

### Error Handling

| Scenario             | Behavior                                        |
| -------------------- | ----------------------------------------------- |
| File not found       | Return error with "file not found" message      |
| Invalid diff format  | Return error with parse failure details         |
| Context mismatch     | Return error showing expected vs actual content |
| Permission denied    | Return error with permission message            |
| Hunk offset mismatch | Try to find best match, fail if cannot          |

### Fuzzy Matching

The tool should attempt to apply patches even if line numbers have shifted:

- Match context lines to find the correct location
- Allow small offsets in hunk positions
- Fail if context cannot be matched anywhere

## Implementation Requirements

### Diff Parsing

The patch tool must:

- Parse unified diff format correctly
- Extract hunks with their line numbers
- Identify context, addition, and deletion lines
- Handle multiple hunks in a single diff
- Handle empty hunks gracefully

### Patch Application

The patch tool must:

- Read the target file into memory
- Match hunk context against file content
- Apply deletions and additions in correct order
- Write modified content back to the file
- Preserve file permissions and encoding

### Error Recovery

If a patch fails:

- Do not modify the original file
- Provide clear error messages
- Suggest how to fix the patch
- Show the expected vs actual content

## Usage Patterns

### Pattern 1: Simple Code Modification

```
User: "Change the greeting in main.go from 'hello' to 'hello world'"

Agent:
1. Read main.go to understand current content
2. Generate a diff patch for the change
3. Call patch tool with the diff
4. Verify the change was applied
```

### Pattern 2: Multi-file Refactoring

```
User: "Rename function Foo to Bar in all files"

Agent:
1. Search for all occurrences of Foo
2. For each file, create a patch
3. Apply patches one by one
4. Report successes and failures
```

### Pattern 3: Adding Documentation

```
User: "Add a docstring to the Process function"

Agent:
1. Read the file to find Process function
2. Create a patch adding the docstring
3. Apply the patch
4. Verify the documentation was added
```

## Testing Requirements

### Unit Tests

- [ ] Parse valid unified diff format
- [ ] Parse multiple hunks correctly
- [ ] Handle single-line additions
- [ ] Handle single-line deletions
- [ ] Handle line modifications
- [ ] Handle empty hunks
- [ ] Detect invalid diff format
- [ ] Match context lines correctly

### Integration Tests

- [ ] Apply patch to existing file
- [ ] Handle file not found
- [ ] Handle permission errors
- [ ] Handle context mismatch
- [ ] Apply multi-hunk patches
- [ ] Verify file content after patch
- [ ] Preserve file permissions
- [ ] Rollback on failure (no partial changes)

### Edge Cases

- [ ] Patch for non-existent file
- [ ] Empty diff string
- [ ] Malformed hunk headers
- [ ] Negative line numbers
- [ ] Very large diffs
- [ ] Binary file patches (should fail gracefully)
- [ ] Unicode content in patches

## Security Considerations

- **Path Traversal**: Validate file paths to prevent directory traversal attacks
- **File Permissions**: Respect original file permissions when writing
- **Diff Injection**: Sanitize diff content to prevent injection attacks
- **Disk Space**: Handle large patches without exhausting disk space

## Performance Considerations

- **Large Files**: Handle files up to 10MB efficiently
- **Memory Usage**: Process patches without loading entire file multiple times
- **Speed**: Apply patches in linear time relative to file size

## Related Requirements

- **005-read-file-tool.md**: Reading files before patching
- **006-write-file-tool.md**: Alternative to patch for full file replacement
- **011-read-lines-tool.md**: Reading specific lines for context
- **012-insert-lines-tool.md**: Alternative for simple insertions
- **013-replace-lines-tool.md**: Alternative for simple replacements

## Acceptance Checklist

- [ ] Tool named `patch` is available
- [ ] Accepts file path parameter
- [ ] Accepts diff content parameter
- [ ] Applies unified diff patches correctly
- [ ] Validates diff format before applying
- [ ] Returns success/failure status
- [ ] Provides clear error messages
- [ ] Handles context line matching
- [ ] Supports multiple hunks
- [ ] Supports additions, deletions, modifications
- [ ] Handles file not found gracefully
- [ ] Handles permission errors gracefully
- [ ] Handles invalid diff format gracefully
- [ ] Does not modify file on failure
- [ ] Tracks failures in statistics
- [ ] Implementation uses Go standard library only
- [ ] Unit tests pass
- [ ] Integration tests pass
