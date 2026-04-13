# Requirement 013: Replace Text Tool

## Description

The harness must support tools for replacing content in a file via OpenAI's tool calling interface. This requirement specifically covers the `replace_text` tool, which finds and replaces text by searching for a pattern.

## Acceptance Criteria

### replace_text Tool
- [ ] Tool named `replace_text` is available
- [ ] Accepts file path as input parameter (required)
- [ ] Accepts search text as input parameter (required)
- [ ] Accepts replacement text as input parameter (required)
- [ ] Accepts count parameter for number of replacements (optional, defaults to 1)
- [ ] Finds and replaces matching text
- [ ] Supports replacing multiple occurrences with count parameter
- [ ] Returns error if search text not found
- [ ] Returns number of replacements made
- [ ] Tool call failures are tracked in statistics

## Tool Definition (OpenAI Format)

### replace_text Tool Definition

```json
{
  "type": "function",
  "function": {
    "name": "replace_text",
    "description": "Find and replace text in a file by searching for a pattern",
    "parameters": {
      "type": "object",
      "properties": {
        "path": {
          "type": "string",
          "description": "File path to modify"
        },
        "search": {
          "type": "string",
          "description": "Text pattern to find (exact match, not regex)"
        },
        "replace": {
          "type": "string",
          "description": "Replacement text"
        },
        "count": {
          "type": "integer",
          "description": "Number of occurrences to replace (default: 1, use -1 for all)"
        }
      },
      "required": ["path", "search", "replace"]
    }
  }
}
```

## Tool Call Format

### replace_text Tool Call

```json
{
  "id": "call_def456",
  "type": "function",
  "function": {
    "name": "replace_text",
    "arguments": "{\"path\":\"/path/to/file.txt\",\"search\":\"old name\",\"replace\":\"new name\"}"
  }
}
```

## Parameters

### replace_text Parameters
- `path`: File path to modify (required, string)
- `search`: Text pattern to find (required, string)
  - Supports multi-line patterns with `\n` escape sequences
  - Exact string matching (not regex)
  - Case-sensitive matching
- `replace`: Replacement text (required, string)
  - Multi-line content uses `\n` escape sequences
  - All special characters must be JSON-escaped
- `count`: Number of occurrences to replace (optional, integer, defaults to 1)
  - Use -1 or "all" to replace all occurrences

## Examples

### replace_text Examples

**Rename a variable:**

```json
{
  "id": "call_004",
  "type": "function",
  "function": {
    "name": "replace_text",
    "arguments": "{\"path\":\"./main.go\",\"search\":\"oldVariableName\",\"replace\":\"newVariableName\"}"
  }
}
```

**Update a configuration value:**

```json
{
  "id": "call_005",
  "type": "function",
  "function": {
    "name": "replace_text",
    "arguments": "{\"path\":\"./config.yaml\",\"search\":\"debug: false\",\"replace\":\"debug: true\"}"
  }
}
```

**Replace a function implementation:**

```json
{
  "id": "call_006",
  "type": "function",
  "function": {
    "name": "replace_text",
    "arguments": "{\"path\":\"./handlers.go\",\"search\":\"func fetchData() {\\n    return \\\"old\\\"\\n}\",\"replace\":\"func fetchData() {\\n    return \\\"new\\\"\\n}\"}"
  }
}
```

**Replace all TODOs:**

```json
{
  "id": "call_007",
  "type": "function",
  "function": {
    "name": "replace_text",
    "arguments": "{\"path\":\"./src/main.go\",\"search\":\"// TODO:\",\"replace\":\"// IMPLEMENTED:\",\"count\":-1}"
  }
}
```

## Return Values

### On Success

#### replace_text
- `success`: `true`
- `path`: The path that was modified
- `search`: The search pattern that was used
- `replacementsMade`: Number of replacements made
- `totalOccurrences`: Total occurrences found in file

### On Failure

- `error`: Description of the error
- `success`: `false`

## Behavior Notes

### replace_text Behavior

- Search is case-sensitive
- Multi-line patterns are supported (use `\n` for newlines in JSON)
- If `count` is not specified, only the first occurrence is replaced
- If `count` is -1 or "all", all occurrences are replaced
- If search text is not found, returns an error
- If search text appears in multiple places, only `count` occurrences are replaced starting from the first match

## Recommendation for LLMs

**When to use replace_text:**

- When you know the exact text to find but not the line numbers
- For simple find-and-replace operations
- When renaming variables, functions, or configuration values
- When the file is large and counting lines is error-prone

**Best practices:**

1. Always read the file first using `read_file` or `read_lines` to understand its contents
2. Use a unique search pattern to avoid unintended replacements
3. Verify changes by reading the file after replacement
4. For multi-line replacements, ensure proper JSON escaping of newlines (`\n`)
