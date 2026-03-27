# Requirement 015: Tool Prefix Prompt

## Description
The list of available tools and their calling format must ALWAYS be prefixed to the context as a fixed system prompt. This ensures the inference engine has consistent knowledge of available tools.

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
  Format: [tool:bash(command="command string")]
  Example: [tool:bash(command="ls -la")]
  Multi-line: [tool:bash(command=<<<RAW>>>
echo "line 1"
<<<END_RAW>>>)]
  
- read_file: Read the contents of a file
  Format: [tool:read_file(path="file path")]
  Example: [tool:read_file(path="/path/to/file.txt")]
  
- write_file: Write content to a file
  Format: [tool:write_file(path="file path", content="file content")]
  Example: [tool:write_file(path="/path/to/file.txt", content="Hello")]
  Multi-line: [tool:write_file(path="file.txt", content=<<<RAW>>>
line 1
<<<END_RAW>>>)]
  
- read_lines: Read a specific line range from a file
  Format: [tool:read_lines(path="file path", start=line_number, end=line_number)]
  Example: [tool:read_lines(path="/path/to/file.txt", start=1, end=10)]
  
- insert_lines: Insert lines at a specific line number
  Format: [tool:insert_lines(path="file path", line=line_number, lines="lines to insert")]
  Example: [tool:insert_lines(path="/path/to/file.txt", line=5, lines="new line")]
  Multi-line: [tool:insert_lines(path="file.txt", line=5, lines=<<<RAW>>>
line 1
<<<END_RAW>>>)]
  
- replace_lines: Replace a line range with new lines
  Format: [tool:replace_lines(path="file path", start=line_number, end=line_number, lines="replacement lines")]
  Example: [tool:replace_lines(path="/path/to/file.txt", start=1, end=5, lines="new content")]
  Multi-line: [tool:replace_lines(path="file.txt", start=1, end=3, lines=<<<RAW>>>
line 1
<<<END_RAW>>>)]

TOOL CALLING RULES:
- Use the exact format shown above for tool calls
- Tool calls must be enclosed in square brackets
- Tool name must match exactly (case-sensitive)
- Parameters must be properly quoted
- Multi-line content: Use raw mode with <<<RAW>>> and <<<END_RAW>>> markers

Instructions:
- Analyze the user's request and determine if tools are needed
- Use tools when they can help complete the task
- Always explain your reasoning before calling tools
- Provide clear explanations of tool results
- Continue the conversation after tool execution
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

## Security Considerations

- System prompt must not contain sensitive information
- Tool descriptions should be generic and safe
- System prompt should not reveal internal implementation details
- Tool parameters should be validated before execution
- Never allow system prompt modification by user input
