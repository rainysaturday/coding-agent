# Requirement 014: Tool Calling Format

## Description
The coding agent harness must use a standardized tool calling format for all tool invocations. The format must be consistent and machine-readable.

## Acceptance Criteria
- [ ] Tool calling format is clearly documented
- [ ] Format supports calling any available tool
- [ ] Format includes tool name as first element
- [ ] Format includes tool parameters as key-value pairs
- [ ] Format can be parsed by both humans and the inference engine
- [ ] Format is consistent across all tool types
- [ ] Format supports nested parameters when needed
- [ ] Invalid tool calls are detected and reported as errors
- [ ] Tool calling format is included in system prompt/prefix

## Tool Calling Format Specification

### Syntax

**Standard Mode (for short, single-line values):**
```
[tool:tool_name(param_name="param_value", ...)]
```

**Raw Mode (for multi-line content without escaping):**
```
[tool:tool_name(path="file.txt", content=<<<RAW>>>
line 1
line 2
line 3
<<<END_RAW>>>)]
```

### Examples

**Bash Tool:**
```
[tool:bash(command="ls -la /home/user")]
```

**Read File Tool:**
```
[tool:read_file(path="/path/to/file.txt")]
```

**Write File Tool (Standard Mode - short content):**
```
[tool:write_file(path="/path/to/file.txt", content="Hello World")]
```

**Write File Tool (Raw Mode - multi-line content):**
```
[tool:write_file(path="/path/to/file.txt", content=<<<RAW>>>
#!/bin/bash
# This is a shell script
echo "Hello World"
for i in {1..10}; do
    echo "Count: $i"
done
<<<END_RAW>>>)]
```

**Read Lines Tool:**
```
[tool:read_lines(path="/path/to/file.txt", start=1, end=10)]
```

**Bash Tool (Raw Mode):**
```
[tool:bash(command=<<<RAW>>>
#!/bin/bash
echo "Starting script..."
for i in {1..5}; do
    echo "Iteration: $i"
done
echo "Done!"
<<<END_RAW>>>)]
```

**Insert Lines Tool (Raw Mode):**
```
[tool:insert_lines(path="/path/to/file.txt", line=5, lines=<<<RAW>>>
new line 1
new line 2
new line 3
<<<END_RAW>>>)]
```

**Replace Lines Tool (Raw Mode):**
```
[tool:replace_lines(path="/path/to/file.txt", start=1, end=5, lines=<<<RAW>>>
replacement line 1
replacement line 2
<<<END_RAW>>>)]
```

### Parameter Rules

**Standard Mode:**
- All parameters are passed as strings in quotes
- Multi-line content uses `\n` for line breaks
- Numeric values can be unquoted integers
- Special characters must be properly escaped
- Best for short, single-line values

**Raw Mode:**
- Content between `<<<RAW>>>` and `<<<END_RAW>>>` is treated literally
- No escaping required - newlines, quotes, and special characters preserved as-is
- `<<<RAW>>>` marker must be on its own line immediately after the `=`
- `<<<END_RAW>>>` marker must be on its own line before the closing `)`
- Best for multi-line content, code, scripts, and documents
- The raw content preserves exact formatting and indentation
- No limit on content size

### Marker Selection

**Use Standard Mode when:**
- Content is short (< 2 lines)
- Content has no special characters
- Content is a simple path, command, or single-line value

**Use Raw Mode when:**
- Content spans multiple lines
- Content contains quotes, backslashes, or special characters
- Content is code, scripts, or formatted documents
- You want to avoid escaping overhead
