# Requirement 026: Configurable Max Iterations

## Description
The coding agent harness must support a configurable maximum iteration limit to prevent infinite loops when the agent makes tool calls. This limit controls how many times the agent can iterate (make tool calls) before stopping and returning control to the user or reporting completion.

## Acceptance Criteria
- [ ] Maximum iteration limit is configurable via environment variable
- [ ] Maximum iteration limit is configurable via command-line flag
- [ ] Maximum iteration limit is configurable via config file
- [ ] Default maximum iteration limit is 1000 iterations
- [ ] Iteration limit can be set as integer value
- [ ] Validation ensures iteration limit is positive (minimum 1)
- [ ] Agent tracks current iteration count during execution
- [ ] Agent stops when iteration limit is reached
- [ ] Clear error message is displayed when limit is exceeded
- [ ] Iteration count is displayed in runtime statistics
- [ ] Iteration count is reset on new conversation session
- [ ] Maximum iterations can be disabled (set to 0 or -1 for unlimited)

## Command-Line Interface

### Usage
```bash
# Set max iterations via flag
coding-agent --max-iterations 500

# Interactive mode with custom limit
coding-agent --max-iterations 2000

# One-shot mode with limit
coding-agent -p "Create a complex application" --max-iterations 1000
```

### Help Output
```bash
$ coding-agent --help
...
      --max-iterations int   Maximum tool call iterations (default: 1000, 0=unlimited)
...
```

## Environment Variables

```bash
# Set via environment variable
export CODING_AGENT_MAX_ITERATIONS=500
coding-agent

# Disable iteration limit (unlimited)
export CODING_AGENT_MAX_ITERATIONS=0
coding-agent
```

## Configuration File

```ini
# In config file
max_iterations = 500

# Or disable limit
max_iterations = 0
```

## Implementation Details

### Agent Behavior

**Normal Operation:**
1. Agent receives user request
2. Agent begins processing (iteration 0)
3. Agent makes tool call (iteration 1)
4. Agent receives tool result
5. Agent continues or completes
6. Repeat until task complete or limit reached

**When Limit is Reached:**
```bash
[Warning] Maximum iterations (1000) exceeded. Task may be incomplete.
[Assistant] I have reached the maximum iteration limit. The task may require more steps than allowed. Please review the current state and continue if needed.
```

### Iteration Tracking

```go
type Agent struct {
    maxIterations int
    iteration     int
    // ... other fields
}

func (a *Agent) Run(prompt string) (*Result, error) {
    a.iteration = 0
    
    for {
        a.iteration++
        
        if a.maxIterations > 0 && a.iteration > a.maxIterations {
            return nil, fmt.Errorf("maximum iterations (%d) exceeded", a.maxIterations)
        }
        
        // ... process request
    }
}
```

### Statistics Display

```bash
==================================================
Runtime Statistics
==================================================
Input Tokens:       1,500
Output Tokens:        950
Tokens/Second:       12.5
Tool Calls:             5
Failed Calls:           0
Iterations:            50 / 1000
Uptime:            10m 30s
==================================================
```

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| Max iterations = 0 | Unlimited iterations allowed |
| Max iterations = 1 | Single tool call then stop |
| Max iterations negative | Treated as unlimited |
| Very large value | Allowed but may cause long runs |
| Default (1000) | Standard protection against infinite loops |

## Safety Considerations

### Why Max Iterations?

1. **Prevent Infinite Loops**: LLMs can get stuck in loops making the same tool calls
2. **Resource Management**: Prevent excessive API calls and compute usage
3. **User Control**: Users can adjust based on task complexity
4. **Debugging**: Helps identify when tasks require too many steps

### Recommended Values

| Use Case | Recommended Limit |
|----------|-------------------|
| Simple tasks | 50-100 |
| Moderate complexity | 200-500 |
| Complex applications | 500-1000 |
| Research/exploration | 0 (unlimited) |

## Error Handling

### Iteration Limit Exceeded
```go
if a.maxIterations > 0 && a.iteration > a.maxIterations {
    return nil, fmt.Errorf("maximum iterations (%d) exceeded. Task may be incomplete.", a.maxIterations)
}
```

### User Response
When limit is reached, the agent should:
1. Display clear warning message
2. Show current state of work
3. Suggest user can continue with new prompt
4. Return partial results if available

## Testing Requirements

### Unit Tests
- [ ] Default max iterations is 1000
- [ ] Config file max iterations is read correctly
- [ ] Environment variable max iterations is read correctly
- [ ] Command-line flag max iterations overrides other sources
- [ ] Zero value disables iteration limit
- [ ] Negative value disables iteration limit
- [ ] Iteration count increments correctly
- [ ] Agent stops when limit is reached
- [ ] Error message is displayed when limit exceeded
- [ ] Statistics show iteration count

### Integration Tests
- [ ] Agent completes simple task within limit
- [ ] Agent stops at configured limit
- [ ] Agent can be configured for unlimited iterations
- [ ] Multiple sessions reset iteration count
- [ ] Statistics accurately reflect iteration count

## Related Requirements
- **003-runtime-statistics.md**: Statistics display including iterations
- **016-tool-result-context-integration.md**: Tool call iteration process
- **025-non-interactive-one-shot-mode.md**: One-shot mode execution
