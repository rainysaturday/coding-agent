# Requirement 014: Tool Calling Format

## Description
The coding agent harness must use a standardized JSON-based tool calling format for all tool invocations. The format must be consistent, machine-readable, and optimized for LLM generation.

## Acceptance Criteria
- [ ] Tool calling format is clearly documented
- [ ] Format supports calling any available tool
- [ ] Format uses JSON structure for parameters
- [ ] Format can be parsed by both humans and the inference engine
- [ ] Format is consistent across all tool types
- [ ] Invalid tool calls are detected and reported as errors
- [ ] Tool calling format is included in system prompt/prefix

## Tool Calling Format Specification

### Syntax

```
[TOOL:{"name":"tool_name","parameters":{...}}]
```

Where:
- `name`: The tool name (string, must match registered tool exactly)
- `parameters`: A JSON object containing all tool-specific parameters

### Parameter Rules

- All parameter values must be valid JSON values (strings, numbers, booleans, arrays, objects)
- String values must be properly JSON-escaped (quotes, backslashes, newlines, etc.)
- Numeric values can be unquoted JSON numbers
- Multi-line content uses `\n` escape sequences in JSON strings
- The entire tool call must be valid JSON inside the `[TOOL:...]` wrapper

### Examples

**Bash Tool (simple command):**
```
[TOOL:{"name":"bash","parameters":{"command":"ls -la /home/user"}}]
```

**Read File Tool:**
```
[TOOL:{"name":"read_file","parameters":{"path":"/path/to/file.txt"}}]
```

**Write File Tool (single line):**
```
[TOOL:{"name":"write_file","parameters":{"path":"/path/to/file.txt","content":"Hello World"}}]
```

**Write File Tool (multi-line content):**
```
[TOOL:{"name":"write_file","parameters":{"path":"/path/to/script.sh","content":"#!/bin/bash\necho \"Hello World\"\nfor i in {1..10}; do\n    echo \"Count: $i\"\ndone"}}]
```

**Read Lines Tool:**
```
[TOOL:{"name":"read_lines","parameters":{"path":"/path/to/file.txt","start":1,"end":10}}]
```

**Insert Lines Tool (multi-line content):**
```
[TOOL:{"name":"insert_lines","parameters":{"path":"/path/to/file.txt","line":5,"lines":"new line 1\nnew line 2\nnew line 3"}}]
```

**Replace Lines Tool:**
```
[TOOL:{"name":"replace_lines","parameters":{"path":"/path/to/file.txt","start":1,"end":5,"lines":"replacement line 1\nreplacement line 2"}}]
```

**Bash Tool (multi-line script):**
```
[TOOL:{"name":"bash","parameters":{"command":"#!/bin/bash\necho \"Starting script...\"\nfor i in {1..5}; do\n    echo \"Iteration: $i\"\ndone\necho \"Done!\""}}]
```

### Format Guidelines

**When to use this format:**
- Always use the JSON-based format `[TOOL:{...}]` for all tool calls
- Use JSON string escaping (`\n`, `\"`, `\\`, etc.) for special characters
- Use JSON numbers (no quotes) for numeric parameters like line numbers
- Keep the tool call on a single line when possible for readability

**LLM Instructions for Generating Tool Calls:**
1. Identify which tool is needed for the task
2. Construct a valid JSON object with `name` and `parameters` keys
3. JSON-escape any special characters in string values
4. Wrap the JSON in `[TOOL:...]` brackets
5. Ensure the entire string is valid JSON inside the wrapper

### Error Handling

**Invalid Tool Call Detection:**
- Missing or malformed `[TOOL:...]` wrapper
- Invalid JSON syntax inside the wrapper
- Unknown tool name (not registered)
- Missing required parameters
- Wrong parameter types

**Error Response Format:**
```
Error: Invalid tool call - <specific error message>
```

### Migration Notes

**From Old Format to New Format:**

| Old Format | New Format |
|------------|------------|
| `[tool:bash(command="ls -la")]` | `[TOOL:{"name":"bash","parameters":{"command":"ls -la"}}]` |
| `[tool:read_file(path="/tmp/test.txt")]` | `[TOOL:{"name":"read_file","parameters":{"path":"/tmp/test.txt"}}]` |
| `[tool:write_file(path="file.txt", content="hello\nworld")]` | `[TOOL:{"name":"write_file","parameters":{"path":"file.txt","content":"hello\nworld"}}]` |
| `[tool:read_lines(path="file.txt", start=1, end=10)]` | `[TOOL:{"name":"read_lines","parameters":{"path":"file.txt","start":1,"end":10}}]` |

**Removed Features:**
- Raw mode markers (`<<<RAW>>>` / `<<<END_RAW>>>`) are no longer needed
- JSON escaping handles all multi-line content uniformly
