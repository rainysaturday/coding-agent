# Requirement 035: Sub-Agent Tool

## Description

The harness must support a `sub_agent` tool that allows spawning parallel sub-agents to work on delegated tasks. This tool enables the agent to delegate work to parallel processes, each running the coding-agent harness in one-shot mode. Sub-agents can:

- Execute independent tasks concurrently with the main agent
- Access the full tool set (bash, file operations, text manipulation, etc.)
- Be configured with a custom timeout
- Return their output to the parent agent for aggregation

This is implemented by spawning a new process of the coding-agent executable with the `-p` flag for one-shot mode.

## Acceptance Criteria

- [x] Tool named `sub_agent` is available
- [x] Accepts a `prompt` parameter with the task description
- [x] Accepts an optional `timeout` parameter (default: 300 seconds / 5 minutes)
- [x] Spawns a new coding-agent process in one-shot mode
- [x] Sub-agents run concurrently (non-blocking from parent)
- [x] Captures stdout/stderr from sub-agent
- [x] Returns sub-agent output to parent agent
- [x] Handles sub-agent timeout with clear error message
- [x] Handles sub-agent failures with exit code
- [x] Locates the coding-agent executable automatically
- [x] Uses current working directory for sub-agent context
- [x] Tool call failures are tracked in statistics
- [x] Supports both relative and absolute executable paths
- [x] Provides clear feedback on sub-agent completion

## Tool Definition (OpenAI Format)

```json
{
  "type": "function",
  "function": {
    "name": "sub_agent",
    "description": "Spawn a parallel sub-agent to handle a delegated task",
    "parameters": {
      "type": "object",
      "properties": {
        "prompt": {
          "type": "string",
          "description": "The task/prompt to delegate to the sub-agent"
        },
        "timeout": {
          "type": "integer",
          "description": "Maximum seconds to wait for sub-agent (default: 300)"
        }
      },
      "required": ["prompt"]
    }
  }
}
```

## Tool Call Format

```json
{
  "id": "call_sub_agent_001",
  "type": "function",
  "function": {
    "name": "sub_agent",
    "arguments": "{\"prompt\":\"Create a Go module file with go.mod for package github.com/example/myproject\",\"timeout\":120}"
  }
}
```

### Parameters

- `prompt`: The task/prompt to delegate to the sub-agent (required, string)
  - Should be a clear, self-contained task description
  - The sub-agent will execute in one-shot mode with this prompt
- `timeout`: Maximum seconds to wait (optional, integer, default: 300)
  - Range: 1-3600 seconds recommended
  - Sub-agent process is killed if timeout is exceeded

## Return Values

### Success

```json
{
  "success": true,
  "output": "=== Agent Execution Log ===\n\n[Step] User prompt\nTool: write_file\nParameters: {\"path\":\"go.mod\",\"content\":\"module github.com/example/myproject\\n\\ngo 1.22\"}\nResult: File written successfully: go.mod\n\n=== Final Output ===\nCreated go.mod for github.com/example/myproject\n\n=== Summary ===\nSteps executed: 1\nTokens used: 150\nDuration: 2.34s\n",
  "exit_code": 0
}
```

### Timeout

```json
{
  "success": false,
  "error": "sub-agent timed out after 60 seconds",
  "exit_code": 1
}
```

### Failure

```json
{
  "success": false,
  "error": "sub-agent failed (exit code 1): Error: API authentication failed",
  "exit_code": 1
}
```

## Behavior Notes

### Executable Discovery

The tool locates the coding-agent executable in this order:

1. `./coding-agent` (current working directory)
2. `./implementation/coding-agent` (implementation directory)
3. `coding-agent` (from PATH via `exec.LookPath`)

If none are found, an error is returned.

### Execution Model

1. A new process is spawned for each sub-agent call
2. The process runs the coding-agent in one-shot mode (`-p` flag)
3. The working directory is set to the current working directory
4. Output is captured from stdout/stderr
5. The parent waits for the sub-agent to complete (up to timeout)

### Timeout Handling

- If the sub-agent exceeds the timeout, it is killed
- The error message indicates the timeout duration
- No partial results are returned for timed-out agents

### Error Handling

| Scenario | Behavior |
|----------|----------|
| Executable not found | Returns error immediately |
| Prompt is empty | Returns error |
| Sub-agent crashes | Returns exit code and output |
| Sub-agent times out | Returns timeout error |
| Sub-agent API error | Returns sub-agent output with error |

## Implementation Requirements

### Process Spawning

The implementation must:

1. Locate the coding-agent executable
2. Create a subprocess with `-p "prompt"` arguments
3. Set the working directory to current directory
4. Configure timeout using `context.WithTimeout`
5. Capture combined stdout/stderr output
6. Wait for completion or timeout

### Timeout Management

The implementation must:

1. Create a context with the specified timeout
2. Use `exec.CommandContext` for automatic cancellation
3. Handle `context.DeadlineExceeded` as a timeout
4. Return appropriate error for timed-out agents

### Output Handling

The implementation must:

1. Capture all sub-agent output (stdout + stderr)
2. Include output in the tool result
3. Preserve the sub-agent's formatting and structure
4. Handle large output without excessive memory usage

## Usage Patterns

### Pattern 1: Parallel File Creation

```
User: "Create a Go project structure with main.go, utils.go, and go.mod"

Agent:
1. Call sub_agent for go.mod creation
2. Call sub_agent for main.go creation  
3. Call sub_agent for utils.go creation
4. Wait for all sub-agents to complete
5. Report completion
```

### Pattern 2: Independent Research Tasks

```
User: "Research both the database schema and API design"

Agent:
1. Call sub_agent for database research
2. Call sub_agent for API design research
3. Wait for both to complete
4. Aggregate results
```

### Pattern 3: Delegated Code Review

```
User: "Review these three files for issues"

Agent:
1. Call sub_agent to review auth.go
2. Call sub_agent to review api.go
3. Call sub_agent to review config.go
4. Wait for all reviews
5. Summarize findings
```

## Testing Requirements

### Unit Tests

- [x] Locate coding-agent executable
- [x] Spawn sub-agent with valid prompt
- [x] Handle empty prompt
- [x] Handle custom timeout
- [x] Handle default timeout
- [x] Handle non-existent executable

### Integration Tests

- [x] Sub-agent executes successfully
- [x] Sub-agent output is captured
- [x] Sub-agent timeout works
- [x] Sub-agent failure is reported
- [x] Sub-agent exit code is returned
- [x] Multiple sub-agents can run concurrently
- [x] Sub-agent uses correct working directory

### Edge Cases

- [x] Very long prompts
- [x] Prompts with special characters
- [x] Prompts with newlines
- [x] Very short timeouts (1 second)
- [x] Very long timeouts (3600 seconds)
- [x] Sub-agent that fails with non-zero exit code
- [x] Sub-agent that produces no output
- [x] Sub-agent that produces large output

## Security Considerations

- **Executable Path**: Validate the coding-agent executable path
- **Working Directory**: Use the current working directory only
- **Timeout**: Always enforce timeout to prevent hung processes
- **Output Limits**: Handle large outputs without memory exhaustion
- **No Shell Injection**: Sub-agent is invoked directly (no shell)

## Performance Considerations

- **Process Overhead**: Each sub-agent spawns a new process
- **Resource Usage**: Multiple concurrent sub-agents consume more resources
- **Timeout**: Set appropriate timeouts to prevent resource exhaustion
- **Output Size**: Large outputs from sub-agents should be handled efficiently

## Related Requirements

- **025-non-interactive-one-shot-mode.md**: One-shot mode used by sub-agents
- **004-bash-tool.md**: Sub-agents can use bash tool
- **005-read-file-tool.md**: Sub-agents can use file tools
- **034-conversation-save-load.md**: Sub-agents don't save/load sessions

## Acceptance Checklist

- [x] Tool named `sub_agent` is available
- [x] Accepts prompt parameter (required)
- [x] Accepts timeout parameter (optional, default: 300s)
- [x] Spawns coding-agent in one-shot mode
- [x] Captures stdout/stderr output
- [x] Returns sub-agent output to parent
- [x] Handles timeout with clear error
- [x] Handles failures with exit code
- [x] Locates executable automatically
- [x] Uses current working directory
- [x] Tracks failures in statistics
- [x] Supports parallel sub-agent execution
- [x] Implementation uses Go standard library only
- [x] Unit tests pass
- [x] Integration tests pass
