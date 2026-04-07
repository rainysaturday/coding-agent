# Requirement 015: Tool Prefix Prompt

## Description

The list of available tools and their descriptions must ALWAYS be prefixed to the context as a fixed system prompt. This ensures the LLM has consistent knowledge of available tools. When using OpenAI's tool calling API, tools are provided both in the system prompt (for context) and in the API request's `tools` field (for native tool calling).

## Acceptance Criteria

- [ ] Tool list is included in system prompt at the beginning of every conversation
- [ ] Tool descriptions are included in system prompt
- [ ] System prompt is preserved during context compression
- [ ] System prompt is NOT modified or removed during conversation
- [ ] System prompt is updated when new tools are added
- [ ] System prompt includes tool descriptions and usage guidelines
- [ ] System prompt is sent with every inference request
- [ ] System prompt remains constant throughout conversation lifecycle
- [ ] Tools are also provided in API request's `tools` field for native tool calling
- [ ] System prompt instructs agent to double-check and verify all work
- [ ] Agent verifies correctness of files created/modified before considering task complete
- [ ] Agent tests code execution or validates output when possible
- [ ] Agent confirms changes match user requirements

## Implementation Details

### System Prompt Structure

```
You are a helpful coding assistant. You have access to the following tools.

TOOL CALLING FORMAT:
- When you need to use a tool, the API will present you with the available tools
- Respond by calling the appropriate tool with the required parameters
- You do NOT need to construct JSON manually - the tool calling API handles the formatting
- Simply provide the tool name and parameter values when prompted to call a tool
- Each tool has specific parameters that must be provided (marked as "required")

EXAMPLE workflow:
1. User asks you to list files in a directory
2. You respond by calling the "bash" tool with command="ls -la /path"
3. The API executes the tool and returns the output
4. You see the result and can continue your response or call more tools

AVAILABLE TOOLS:

1. bash
   Description: Execute a bash command in the terminal
   Parameters:
     - command (string, required): The bash command to execute
   How to call: Use the bash tool when you need to run shell commands, install packages, build projects, check file system, etc.
   Example use case: "ls -la", "cat file.txt", "go build", "git status"

2. read_file
   Description: Read the contents of a file
   Parameters:
     - path (string, required): The path to the file to read
   How to call: Use read_file to view the contents of any file before making changes.
   Example use case: Reading source files, configuration files, documentation

3. write_file
   Description: Write content to a file
   Parameters:
     - path (string, required): The path to the file to write
     - content (string, required): The content to write to the file
   How to call: Use write_file to create new files or completely overwrite existing files.
   Example use case: Creating new source files, writing configuration, saving output
   Note: For multi-line content, use \n to represent newlines in the content parameter

4. read_lines
   Description: Read a specific line range from a file
   Parameters:
     - path (string, required): The path to the file
     - start (integer, required): The starting line number (1-indexed)
     - end (integer, required): The ending line number (1-indexed)
   How to call: Use read_lines when you only need to view a portion of a large file.
   Example use case: Viewing lines 1-50 of a large source file, checking specific sections

5. insert_lines
   Description: Insert lines at a specific line number
   Parameters:
     - path (string, required): The path to the file
     - line (integer, required): The line number where insertion should occur (1-indexed)
     - lines (string, required): The lines to insert (use \n for newlines)
   How to call: Use insert_lines to add new content without replacing existing content.
   Example use case: Adding imports, inserting new functions, adding comments
   Note: Inserting at line 1 adds at the beginning; inserting beyond file length appends

6. replace_lines
   Description: Replace a range of lines in a file by line numbers
   Parameters:
     - path (string, required): The path to the file
     - start (integer, required): Starting line number (1-indexed)
     - end (integer, required): Ending line number (1-indexed)
     - lines (string, required): Replacement lines (use \n for newlines)
   How to call: Use replace_lines when you know the exact line numbers to replace.
   Example use case: Replacing lines 10-20 with new content, deleting a range of lines

7. replace_text
   Description: Find and replace text in a file by searching for a pattern
   Parameters:
     - path (string, required): The path to the file
     - search (string, required): Text pattern to find (exact match, not regex)
     - replace (string, required): Replacement text
     - count (integer, optional): Number of occurrences to replace (default: 1, use -1 for all)
   How to call: Use replace_text when you know the text to find but not the line numbers.
   Example use case: Renaming variables, updating function names, fixing typos throughout a file

TOOL CALLING BEST PRACTICES:
1. Always read a file first (using read_file or read_lines) to understand its contents
2. When modifying files, be precise about what you're changing
3. For multi-line content, properly format with \n for newlines
4. Verify your changes by re-reading files after writing
5. Test code by running appropriate commands (go build, go test, etc.)
```

### OpenAI Tool Schema

In addition to the system prompt, tools must be provided in the API request's `tools` field:

```json
{
  "model": "gpt-4o",
  "messages": [...],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "bash",
        "description": "Execute a bash command in the terminal",
        "parameters": {
          "type": "object",
          "properties": {
            "command": {
              "type": "string",
              "description": "The bash command to execute"
            }
          },
          "required": ["command"]
        }
      }
    },
    {
      "type": "function",
      "function": {
        "name": "replace_lines",
        "description": "Replace content in a file by line numbers (replace lines in a specific range)",
        "parameters": {
          "type": "object",
          "properties": {
            "path": {
              "type": "string",
              "description": "File path to modify"
            },
            "start": {
              "type": "integer",
              "description": "Start line number (1-indexed)"
            },
            "end": {
              "type": "integer",
              "description": "End line number (1-indexed)"
            },
            "lines": {
              "type": "string",
              "description": "Replacement lines (use \\n for newlines)"
            }
          },
          "required": ["path", "start", "end", "lines"]
        }
      }
    },
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
    // ... other tools
  ],
  "tool_choice": "auto"
}
```

### System Prompt Management

1. **Initial Setup**: System prompt is created with all available tools
2. **Context Creation**: System prompt is the first message in the context
3. **API Request**: Tools are provided in both system prompt and `tools` field
4. **Compression**: System prompt is preserved and prepended to summary
5. **Tool Updates**: When tools are added/removed, system prompt is regenerated
6. **Persistence**: System prompt remains unchanged throughout conversation

### Compression Behavior

During context compression:
1. Original system prompt is extracted and preserved
2. Conversation history is summarized
3. New context = system prompt + summary
4. System prompt content is NOT summarized
5. Only the conversation history is compressed

### Verification Requirements

- ALWAYS double-check your work before considering a task complete
- Verify that created/modified files exist and contain the expected content
- Test code execution when possible (e.g., run go build, go test)
- Validate that changes meet the user's requirements
- If you make multiple changes, verify each one independently
- Re-read files after writing to confirm content was written correctly
- Run validation commands (e.g., `go vet`, `gofmt -d`, `cat` to view files)
- If verification fails, fix the issue and re-verify
- Provide a final verification summary before concluding the task

**Verification Checklist:**
1. Files exist at the expected paths
2. File content matches the intended changes
3. Code compiles without errors (for Go code)
4. Code follows Go formatting standards (gofmt)
5. Changes align with user requirements
6. No unintended side effects or broken dependencies

### System Prompt Injection Point

```
[SYSTEM PROMPT - ALWAYS FIRST]
[USER MESSAGE 1]
[ASSISTANT RESPONSE 1]
[USER MESSAGE 2]
[ASSISTANT RESPONSE 2]
...
```

## Tool Result Format

After tool execution, results should be sent back to the LLM as tool messages:

```json
{
  "role": "tool",
  "tool_call_id": "call_abc123",
  "content": "Tool execution output or result"
}
```

## Error Handling

When a tool call fails, the error should be communicated back to the LLM:

```json
{
  "role": "tool",
  "tool_call_id": "call_abc123",
  "content": "Error: Invalid tool call - <specific error message>"
}
```

## Security Considerations

- System prompt must not contain sensitive information
- Tool descriptions should be generic and safe
- System prompt should not reveal internal implementation details
- Tool parameters should be validated before execution
- Never allow system prompt modification by user input
