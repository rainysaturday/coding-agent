# Requirement 029: System Prompt Environment Information

## Description

The system prompt provided to the LLM must include essential environment information to help the agent understand its context. This includes:

1. **Current Working Directory**: The folder where the agent is running
2. **Agent Executable Path**: The path to the coding-agent binary itself (enabling sub-agent spawning)
3. **System Information**: Brief knowledge about the OS and architecture

This information enables the agent to:
- Know where it should operate (working directory)
- Spawn sub-agents when needed using the `-p` parameter
- Understand the system environment for appropriate command generation

## Acceptance Criteria

- [ ] System prompt includes the current working directory
- [ ] System prompt includes the path to the coding-agent executable
- [ ] System prompt includes OS information (e.g., Linux, macOS, Windows)
- [ ] System prompt includes architecture information (e.g., amd64, arm64)
- [ ] Environment information is gathered at runtime
- [ ] Environment information is included in the system prompt at the beginning
- [ ] Environment information is preserved during context compression
- [ ] Environment information is human-readable and concise
- [ ] Implementation uses Go standard library (no external dependencies)

## Implementation Details

### Environment Information Sources

| Information | Source |
|-------------|--------|
| Current Working Directory | `os.Getwd()` |
| Executable Path | `os.Executable()` |
| OS | `runtime.GOOS` |
| Architecture | `runtime.GOARCH` |

### System Prompt Addition

The environment information should be added near the beginning of the system prompt, after the initial introduction:

```
You are a helpful coding assistant. You have access to the following tools.

ENVIRONMENT INFORMATION:
- Current Working Directory: /path/to/working/dir
- Agent Executable: /path/to/coding-agent
- Operating System: linux
- Architecture: amd64

You can use the agent executable to spawn sub-agents for parallel tasks using the -p parameter:
  coding-agent -p "Your task here"

TOOL CALLING FORMAT:
...
```

### Go Implementation Example

```go
import (
    "os"
    "runtime"
)

func getEnvironmentInfo() (string, error) {
    // Get current working directory
    cwd, err := os.Getwd()
    if err != nil {
        cwd = "unknown"
    }
    
    // Get executable path
    exePath, err := os.Executable()
    if err != nil {
        exePath = "unknown"
    }
    
    // Get OS and architecture
    osInfo := runtime.GOOS
    archInfo := runtime.GOARCH
    
    return fmt.Sprintf(`ENVIRONMENT INFORMATION:
- Current Working Directory: %s
- Agent Executable: %s
- Operating System: %s
- Architecture: %s

You can use the agent executable to spawn sub-agents for parallel tasks using the -p parameter:
  coding-agent -p "Your task here"
`, cwd, exePath, osInfo, archInfo), nil
}
```

### Build System Integration

The environment information should be gathered at runtime (not build time) because:
- The working directory changes based on where the agent is invoked
- The executable path depends on installation location
- OS and architecture are runtime properties

### System Prompt Construction

```go
func buildSystemPrompt() string {
    // Get environment info
    envInfo, err := getEnvironmentInfo()
    if err != nil {
        envInfo = "ENVIRONMENT INFORMATION: unavailable"
    }
    
    return fmt.Sprintf(`You are a helpful coding assistant. You have access to the following tools.

%s

TOOL CALLING FORMAT:
...
`, envInfo)
}
```

## Examples

### Example System Prompt with Environment Info

```
You are a helpful coding assistant. You have access to the following tools.

ENVIRONMENT INFORMATION:
- Current Working Directory: /home/user/projects/my-app
- Agent Executable: /usr/local/bin/coding-agent
- Operating System: linux
- Architecture: amd64

You can use the agent executable to spawn sub-agents for parallel tasks using the -p parameter:
  coding-agent -p "Your task here"

TOOL CALLING FORMAT:
- When you need to use a tool, the API will present you with the available tools
...

AVAILABLE TOOLS:

1. bash
   Description: Execute a bash command in the terminal
   ...
```

### Sub-Agent Usage

The agent can spawn sub-agents for parallel or isolated tasks:

```
[Assistant] I'll spawn a sub-agent to handle the documentation task.

Calling tool: bash (command: "coding-agent -p 'Generate README.md' &")

[Success] bash completed
```

## Use Cases

### Parallel Task Execution

When a task has independent subtasks, the agent can spawn sub-agents:

```
User: "Build the frontend and backend separately"

Agent:
1. Spawn sub-agent for frontend: coding-agent -p "Build frontend"
2. Spawn sub-agent for backend: coding-agent -p "Build backend"
3. Wait for both to complete
4. Combine results
```

### Isolated Sandboxed Tasks

For risky operations, spawn a sub-agent in a different directory:

```
Agent:
1. Create temp directory
2. Spawn sub-agent: coding-agent -p "Process files" --cwd /tmp/sandbox
3. Review results
4. Clean up
```

### Context-Aware Operations

The agent knows its environment and can make appropriate decisions:

- On Linux: Use bash commands, Linux paths
- On macOS: Use macOS-specific commands if needed
- On Windows: Use PowerShell/cmd commands, Windows paths

## Error Handling

If environment information cannot be retrieved:

| Scenario | Behavior |
|----------|----------|
| `os.Getwd()` fails | Use "unknown" for working directory |
| `os.Executable()` fails | Use "unknown" for executable path |
| Runtime info unavailable | Use "unknown" for OS/arch |
| All info unavailable | Display "ENVIRONMENT INFORMATION: unavailable" |

## Security Considerations

- Environment paths may reveal system structure
- Do not expose sensitive paths (e.g., /etc/shadow)
- Consider sanitizing paths if needed
- Working directory is typically safe to expose

## Testing Requirements

### Unit Tests

- [ ] `getEnvironmentInfo()` returns valid information
- [ ] Working directory is correctly retrieved
- [ ] Executable path is correctly retrieved
- [ ] OS and architecture are correctly reported
- [ ] Error handling works when info unavailable

### Integration Tests

- [ ] System prompt includes environment info
- [ ] Environment info is accurate for current system
- [ ] Sub-agent spawning works with executable path
- [ ] Environment info is preserved during compression

## Related Requirements

- **015-tool-prefix-prompt.md**: System prompt structure
- **025-non-interactive-one-shot-mode.md**: One-shot mode with -p parameter
- **024-zero-external-dependencies.md**: Use stdlib only for environment info
- **001-go-runtime.md**: Go runtime for environment detection

## Acceptance Checklist

- [ ] System prompt includes current working directory
- [ ] System prompt includes agent executable path
- [ ] System prompt includes OS information
- [ ] System prompt includes architecture information
- [ ] Environment info is gathered at runtime
- [ ] Environment info is human-readable
- [ ] Implementation uses Go standard library only
- [ ] Error handling for unavailable info
- [ ] Environment info preserved during compression
- [ ] Sub-agent spawning documented in system prompt
