# Requirement 015: Tool Prefix Prompt

## Description
The list of available tools and their JSON-based calling format must ALWAYS be prefixed to the context as a fixed system prompt. This ensures the inference engine has consistent knowledge of available tools.

## Acceptance Criteria
- [ ] Tool list is included in system prompt at the beginning of every conversation
- [ ] Tool calling format is included in system prompt
- [ ] System prompt is preserved during context compression
- [ ] System prompt is NOT modified or removed during conversation
- [ ] System prompt is updated when new tools are added
- [ ] System prompt includes tool descriptions
- [ ] System prompt is sent with every inference request
- [ ] System prompt remains constant throughout conversation lifecycle
- [ ] System prompt is compressed along with context when needed

## Implementation Details

### System Prompt Structure

```
You are a helpful coding assistant. You have access to the following tools:

AVAILABLE TOOLS:
- bash: Execute a bash command
  Format: [TOOL:{"name":"bash","parameters":{"command":"command string"}}]
  Example: [TOOL:{"name":"bash","parameters":{"command":"ls -la"}}]
  Multi-line: [TOOL:{"name":"bash","parameters":{"command":"line1\nline2\nline3"}}]
  
- read_file: Read the contents of a file
  Format: [TOOL:{"name":"read_file","parameters":{"path":"file path"}}]
  Example: [TOOL:{"name":"read_file","parameters":{"path":"/path/to/file.txt"}}]
  
- write_file: Write content to a file
  Format: [TOOL:{"name":"write_file","parameters":{"path":"file path","content":"file content"}}]
  Example: [TOOL:{"name":"write_file","parameters":{"path":"/path/to/file.txt","content":"Hello"}}]
  Multi-line: [TOOL:{"name":"write_file","parameters":{"path":"file.txt","content":"line1\nline2"}}]
  
- read_lines: Read a specific line range from a file
  Format: [TOOL:{"name":"read_lines","parameters":{"path":"file path","start":line_number,"end":line_number}}]
  Example: [TOOL:{"name":"read_lines","parameters":{"path":"/path/to/file.txt","start":1,"end":10}}]
  
- insert_lines: Insert lines at a specific line number
  Format: [TOOL:{"name":"insert_lines","parameters":{"path":"file path","line":line_number,"lines":"lines to insert"}}]
  Example: [TOOL:{"name":"insert_lines","parameters":{"path":"/path/to/file.txt","line":5,"lines":"new line"}}]
  Multi-line: [TOOL:{"name":"insert_lines","parameters":{"path":"file.txt","line":5,"lines":"line1\nline2"}}]
  
- replace_lines: Replace a line range with new lines
  Format: [TOOL:{"name":"replace_lines","parameters":{"path":"file path","start":line_number,"end":line_number,"lines":"replacement lines"}}]
  Example: [TOOL:{"name":"replace_lines","parameters":{"path":"/path/to/file.txt","start":1,"end":5,"lines":"new content"}}]
  Multi-line: [TOOL:{"name":"replace_lines","parameters":{"path":"file.txt","start":1,"end":3,"lines":"line1\nline2"}}]

TOOL CALLING RULES:
- Use the exact JSON format shown above for tool calls
- Tool calls must be enclosed in [TOOL:...] brackets
- The content inside brackets must be valid JSON
- Tool name must match exactly (case-sensitive, use underscore not hyphen)
- Parameters must be in a JSON object under the "parameters" key
- String values must be properly JSON-escaped (use \n for newlines, \" for quotes)
- Numeric values should not be quoted (e.g., "start":1 not "start":"1")

Instructions:
- Analyze the user's request and determine if tools are needed
- Use tools when they can help complete the task
- Always explain your reasoning before calling tools
- Provide clear explanations of tool results
- Continue the conversation after tool execution
- Generate valid JSON inside the [TOOL:...] wrapper
```

### System Prompt Management

1. **Initial Setup**: System prompt is created with all available tools
2. **Context Creation**: System prompt is the first message in the context
3. **Compression**: System prompt is preserved and prepended to summary
4. **Tool Updates**: When tools are added/removed, system prompt is regenerated
5. **Persistence**: System prompt remains unchanged throughout conversation

### Compression Behavior

During context compression:
1. Original system prompt is extracted and preserved
2. Conversation history is summarized
3. New context = system prompt + summary
4. System prompt content is NOT summarized
5. Only the conversation history is compressed

### System Prompt Injection Point

```
[SYSTEM PROMPT - ALWAYS FIRST]
[USER MESSAGE 1]
[ASSISTANT RESPONSE 1]
[USER MESSAGE 2]
[ASSISTANT RESPONSE 2]
...
```

## JSON Format Details

### Structure
```json
{
  "name": "tool_name",
  "parameters": {
    "param1": "value1",
    "param2": 123,
    "param3": "value with\nnewlines"
  }
}
```

### Escaping Rules
- Newlines: `\n`
- Carriage returns: `\r`
- Tabs: `\t`
- Double quotes: `\"`
- Backslashes: `\\`
- Unicode: `\uXXXX`

### Common Mistakes to Avoid
- Using old format `[tool:name(param="value")]` - NOT SUPPORTED
- Forgetting the `parameters` key wrapper
- Using hyphens instead of underscores in tool names
- Forgetting to escape quotes in strings
- Quoting numeric values (use `"start":1` not `"start":"1"`)

## Security Considerations

- System prompt must not contain sensitive information
- Tool descriptions should be generic and safe
- System prompt should not reveal internal implementation details
- Tool parameters should be validated before execution
- Never allow system prompt modification by user input
