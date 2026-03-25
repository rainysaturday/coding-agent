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
```
[tool:tool_name(param_name="param_value", ...)]
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

**Write File Tool:**
```
[tool:write_file(path="/path/to/file.txt", content="Hello World")]
```

**Read Lines Tool:**
```
[tool:read_lines(path="/path/to/file.txt", start=1, end=10)]
```

**Insert Lines Tool:**
```
[tool:insert_lines(path="/path/to/file.txt", line=5, lines="new line 1\nnew line 2")]
```

**Replace Lines Tool:**
```
[tool:replace_lines(path="/path/to/file.txt", start=1, end=5, lines="new line 1\nnew line 2")]
```

### Parameter Rules
- All parameters are passed as strings in quotes
- Multi-line content uses `\n` for line breaks
- Numeric values can be unquoted integers
- Special characters must be properly escaped
- Tool name must match exactly (case-sensitive)
