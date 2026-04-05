# Requirement 026: Configurable Max Iterations

## Description
The coding agent harness must support a configurable maximum iteration limit to prevent infinite loops when the agent is stuck in a cycle of tool calls. This provides a safety mechanism that allows the user to intervene when the agent is not making progress.

## Acceptance Criteria
- [ ] Maximum iterations is configurable via command-line flag (`--max-iterations`)
- [ ] Maximum iterations is configurable via environment variable (`CODING_AGENT_MAX_ITERATIONS`)
- [ ] Maximum iterations is configurable via config file (`max_iterations`)
- [ ] Default maximum iterations is 1000
- [ ] Maximum iterations can be set as positive integer value
- [ ] Validation ensures max iterations is positive
- [ ] Agent stops and reports error when max iterations exceeded
- [ ] Iteration count is tracked and displayed in statistics
- [ ] User is notified when approaching max iterations (optional warning)
- [ ] Max iterations is included in help documentation

## Configuration

### Command-Line Flag
```bash
# Set max iterations via command line
coding-agent --max-iterations 500

# Short form (if applicable)
coding-agent -i 500
```

### Environment Variable
```bash
export CODING_AGENT_MAX_ITERATIONS=500
coding-agent
```

### Config File
```
max_iterations=500
```

## Default Value
- **Default:** 1000 iterations
- **Minimum:** 1 iteration
- **Recommended:** 100-1000 depending on task complexity

## Behavior

### Iteration Counting
- Each tool call execution counts as one iteration
- Multiple tool calls in a single LLM response count separately
- Context compression operations do not count as iterations
- User prompts in interactive mode reset the iteration counter

### Max Iterations Exceeded
When the maximum iterations is exceeded:
```
Error: maximum iterations (1000) exceeded
```

The agent will:
1. Stop executing further tool calls
2. Return an error to the user
3. Include iteration count in error message
4. Preserve conversation context for analysis

### Statistics Display
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

### Simple Tasks
For straightforward tasks that require few tool calls:
```bash
coding-agent --max-iterations 50 -p "Create a simple Hello World program"
```

### Complex Tasks
For complex tasks requiring many iterations:
```bash
coding-agent --max-iterations 2000 -p "Build a complete REST API with authentication"
```

### Debugging/Testing
For testing and debugging with lower limits:
```bash
coding-agent --max-iterations 10 -p "Test task"
```

### Production Safety
For production use with reasonable limits:
```bash
export CODING_AGENT_MAX_ITERATIONS=1000
coding-agent
```

## Error Handling

### Invalid Configuration
```bash
# Negative value
coding-agent --max-iterations -100
Error: max iterations must be positive

# Zero value
coding-agent --max-iterations 0
Error: max iterations must be positive

# Non-numeric value
coding-agent --max-iterations abc
Error: invalid max-iterations: invalid syntax
```

### Iteration Limit Reached
```
Error: maximum iterations (1000) exceeded
```

## Implementation Details

### Agent Loop
```go
iteration := 0
for {
    iteration++
    if iteration > a.maxIterations {
        return nil, fmt.Errorf("maximum iterations (%d) exceeded", a.maxIterations)
    }
    
    // ... agent logic ...
}
```

### Configuration Chain
1. Config file loads first (lowest priority)
2. Environment variables override config file
3. Command-line flags override environment variables (highest priority)

### Iteration Tracking
- Iterations are tracked per agent run/session
- Reset on new user prompt in interactive mode
- Included in statistics for monitoring
- Used for debugging and optimization

## Related Requirements
- **016-tool-result-context-integration.md**: Tool calling and iteration
- **020-tui-ctrl-c-cancellation.md**: Alternative cancellation method
- **003-runtime-statistics.md**: Statistics display
- **025-non-interactive-one-shot-mode.md**: One-shot mode execution

## Testing Requirements

### Unit Tests
- [ ] Default max iterations is 1000
- [ ] Command-line flag sets max iterations correctly
- [ ] Environment variable sets max iterations correctly
- [ ] Config file sets max iterations correctly
- [ ] Command-line overrides environment variable
- [ ] Environment variable overrides config file
- [ ] Invalid values are rejected with proper error
- [ ] Agent stops when max iterations exceeded
- [ ] Iteration count is accurate

### Integration Tests
- [ ] Agent completes task within iteration limit
- [ ] Agent stops at iteration limit with error
- [ ] Statistics show correct iteration count
- [ ] Error message includes iteration limit
- [ ] Context is preserved after iteration limit

## Security Considerations
- Prevents resource exhaustion from infinite loops
- Protects against API cost runaway
- Allows user control over execution length
- Should be combined with timeout mechanisms

## Recommendations

### Default Settings
- **Interactive Mode:** 1000 iterations
- **One-Shot Mode:** 1000 iterations
- **CI/CD:** 500 iterations (faster failure)
- **Testing:** 50-100 iterations

### Monitoring
- Watch iteration count in statistics
- Set lower limits for testing
- Increase limits for complex tasks
- Use Ctrl+C for immediate cancellation

### Best Practices
1. Start with default (1000) for most tasks
2. Lower limits for simple/quick tasks
3. Higher limits for complex multi-step tasks
4. Monitor iteration count during long runs
5. Use Ctrl+C if agent seems stuck
6. Review tool call patterns if hitting limits frequently
