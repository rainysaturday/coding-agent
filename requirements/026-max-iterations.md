# Requirement 026: Configurable Maximum Iterations

## Description
The coding agent harness must support a configurable maximum iteration limit to prevent infinite loops when the agent is stuck in a cycle of tool calls. This limit controls how many times the agent can iterate (make tool calls) before stopping and returning control to the user.

## Acceptance Criteria
- [ ] Maximum iterations is configurable via command-line flag `--max-iterations`
- [ ] Maximum iterations is configurable via environment variable `CODING_AGENT_MAX_ITERATIONS`
- [ ] Maximum iterations is configurable via config file `max_iterations` parameter
- [ ] Default maximum iterations is 1000
- [ ] Maximum iterations must be a positive integer
- [ ] Validation ensures minimum value of 1
- [ ] Agent stops and reports error when iteration limit is exceeded
- [ ] Current iteration count is tracked and displayed in statistics
- [ ] Iteration limit prevents infinite tool call loops
- [ ] User can resume after hitting iteration limit by providing new input

## Implementation Details

### Configuration Sources

**Command-line flag:**
```bash
coding-agent --max-iterations 500
```

**Environment variable:**
```bash
export CODING_AGENT_MAX_ITERATIONS=500
coding-agent
```

**Config file:**
```
max_iterations=500
```

### Priority Order
1. Command-line flag (highest priority)
2. Environment variable
3. Config file
4. Default value (lowest priority)

### Default Behavior
- Default: 1000 iterations
- This allows for complex multi-step tasks while preventing runaway loops
- Users can increase for very complex tasks or decrease for faster termination

### Error Message Format
When the iteration limit is exceeded:
```
Error: maximum iterations (1000) exceeded
```

### Statistics Integration
The iteration count should be displayed in the `stats` command output:
```
==================================================
Runtime Statistics
==================================================
Input Tokens:      1,500
Output Tokens:       950
Tokens/Second:      12.5
Tool Calls:           5
Failed Calls:         0
Iterations:           3
Max Iterations:    1000
Uptime:            10m 30s
==================================================
```

## Use Cases

### Complex Multi-Step Tasks
```bash
# Allow for extensive code refactoring
coding-agent --max-iterations 2000 -p "Refactor the entire codebase to use modern Go patterns"
```

### Quick Tasks
```bash
# Limit iterations for simple tasks
coding-agent --max-iterations 10 -p "List the files in the current directory"
```

### Testing and Debugging
```bash
# Lower limit for faster testing
coding-agent --max-iterations 5 -p "Test this function"
```

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| max-iterations = 0 | Validation error: must be positive |
| max-iterations = 1 | Agent can make 1 tool call then stops |
| max-iterations = -1 | Validation error: must be positive |
| Very large value (e.g., 100000) | Allowed, but may lead to long execution |
| Non-numeric value | Validation error: invalid max-iterations |

## Agent Behavior

### Normal Operation
1. Agent starts with iteration counter at 0
2. Each tool call iteration increments counter
3. Before each iteration, check if counter >= max
4. If limit exceeded, stop and return error
5. If no tool calls in response, task is complete

### Iteration Limit Hit
```
[Agent] Processing request...
[Iteration 1] Calling tool: bash
[Iteration 2] Calling tool: read_file
[Iteration 3] Calling tool: write_file
...
[Iteration 1000] Calling tool: bash
[Error] maximum iterations (1000) exceeded
```

### Resumption After Limit
After hitting the limit, the user can:
1. Review what was accomplished
2. Provide new prompt to continue
3. Adjust max-iterations and retry
4. Cancel and start fresh

## Configuration File Format

```ini
# Max iterations for agent loop protection
max_iterations=1000

# Other settings...
model=llama3
temperature=0.7
context_size=128000
```

## Environment Variable Format

```bash
# Set max iterations to 500
export CODING_AGENT_MAX_ITERATIONS=500

# Run agent (will use 500 iterations)
coding-agent -p "Your task here"
```

## Testing Requirements

### Unit Tests
- [ ] Default value is 1000
- [ ] Command-line flag overrides default
- [ ] Environment variable overrides default
- [ ] Config file value is loaded correctly
- [ ] Priority order is correct (CLI > env > config > default)
- [ ] Validation rejects zero or negative values
- [ ] Validation rejects non-numeric values

### Integration Tests
- [ ] Agent stops at configured iteration limit
- [ ] Error message includes the limit value
- [ ] Statistics show current iteration count
- [ ] Agent can be resumed after hitting limit
- [ ] Different limits work correctly (1, 10, 100, 1000)

### Stress Tests
- [ ] Very high limit (10000+) doesn't cause memory issues
- [ ] Very low limit (1) works correctly
- [ ] Rapid iteration doesn't bypass limit

## Security Considerations

- Iteration limit prevents resource exhaustion attacks
- Lower limits reduce potential for abuse
- No sensitive information exposed in iteration errors
- Limit applies per session, not globally

## Related Requirements
- **003-runtime-statistics.md**: Statistics display including iterations
- **025-non-interactive-one-shot-mode.md**: One-shot mode execution
- **016-tool-result-context-integration.md**: Tool call iteration process
