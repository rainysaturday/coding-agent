# Requirement 040: Subagent Tool

## Description
The coding agent harness must include a `subagent` tool that allows the main agent to spawn a completely separate, independent agent process. The subagent runs as a recursive invocation of the same coding-agent binary with its own persona, prompt, and execution context. The main agent receives only the summary/conclusion from the subagent, avoiding context overflow. This enables parallel task delegation and specialized expertise invocation.

## Acceptance Criteria
- [ ] `subagent` tool is available in the standard tool set
- [ ] `subagent` tool accepts `prompt` (required) and `persona` (optional) parameters
- [ ] Subagent runs as a separate process using the same binary
- [ ] Subagent inherits the current working directory
- [ ] Subagent receives the specified persona (or main agent's persona if none specified)
- [ ] Subagent uses `--summary-only` mode to return only the conclusion
- [ ] Subagent uses `--no-stream` for cleaner output
- [ ] Main agent receives only the summary, not the full execution log
- [ ] Clear TUI feedback shows when a subagent is running and its result
- [ ] Subagent errors are reported clearly to the main agent
- [ ] Subagent is described in the system prompt as a tool for parallel task delegation

## Tool Definition

```json
{
  "type": "function",
  "function": {
    "name": "subagent",
    "description": "Spawn a sub-agent to work on a task independently. The sub-agent runs as a separate process and returns only its conclusion/summary.",
    "parameters": {
      "type": "object",
      "properties": {
        "prompt": {
          "type": "string",
          "description": "The task description for the sub-agent. Be specific and clear about what you want the sub-agent to accomplish."
        },
        "persona": {
          "type": "string",
          "description": "A persona to give the sub-agent. For example: 'Expert Go developer', 'Code reviewer focused on security', 'Documentation writer'."
        }
      },
      "required": ["prompt"]
    }
  }
}
```

## Command-Line Execution

The subagent is spawned by executing the coding-agent binary with the following flags:

```bash
coding-agent \
    --prompt-file - \      # Read prompt from stdin
    --summary-only \       # Only return the summary/conclusion
    --no-stream \          # Disable streaming output
    --quiet \              # Minimize noise
    --persona "..." \      # Optional: custom persona
    --read-only            # Optional: inherit read-only mode
```

## Process Flow

1. Main agent decides to use the `subagent` tool
2. Main agent provides a `prompt` and optionally a `persona`
3. Main agent spawns a subprocess with the subagent parameters
4. Subagent runs independently with its own persona
5. Subagent returns only its conclusion/summary
6. Main agent receives the summary and continues its work
7. TUI shows clear feedback about the subagent's result

## System Prompt Description

The system prompt should include instructions like:

```
You can use the coding-agent to spawn sub-agents for parallel tasks using the subagent tool.
When you need to run a subagent, use the 'subagent' tool with a clear task description.
The subagent will run independently and return its conclusion/summary.
```

## TUI Feedback

### Subagent Start
When a subagent starts, the TUI should display:
```
[Subagent] Starting subagent task...
```

### Subagent Result (Success)
```
[Subagent] Task completed
Summary:
{brief summary of what the subagent accomplished}
```

### Subagent Result (Failure)
```
[Subagent] Failed: {error message}
```

## Output Constraints

### Summary Extraction
The main agent should receive only:
- A brief summary of the work completed
- The conclusion or final answer
- Any critical findings or issues

The main agent should NOT receive:
- The full execution log
- Individual tool calls made by the subagent
- Step-by-step details of the subagent's work
- More than ~5000 characters of output (truncated if necessary)

### Context Management
- Subagent output is limited to prevent context overflow
- Only the conclusion/summary is passed back to the main agent
- The subagent's internal reasoning is not visible to the main agent

## Implementation Details

### ExecuteSubagent Function
```go
func ExecuteSubagent(params map[string]interface{}) *ToolResult {
    prompt, ok := params["prompt"].(string)
    if !ok || prompt == "" {
        return &ToolResult{Success: false, Error: "missing required parameter: prompt"}
    }
    
    persona := ""
    if p, ok := params["persona"].(string); ok && p != "" {
        persona = p
    }
    
    // Build command arguments
    args := []string{
        "--prompt-file", "-",
        "--summary-only",
        "--no-stream",
        "--quiet",
    }
    
    if persona != "" {
        args = append(args, "--persona", persona)
    }
    
    // Spawn subprocess
    cmd := exec.Command(binaryPath, args...)
    cmd.Stdin = strings.NewReader(prompt)
    
    // Capture output and extract summary
    output, err := cmd.Output()
    summary := extractSummary(string(output))
    
    return &ToolResult{
        Success: true,
        Output:  fmt.Sprintf("Subagent completed.\n\nSummary:\n%s", summary),
    }
}
```

### extractSummary Function
```go
func extractSummary(output string) string {
    // Try to find "=== Final Output ===" marker
    if idx := strings.Index(output, "=== Final Output ==="); idx != -1 {
        return strings.TrimSpace(output[idx+len("=== Final Output ==="):])
    }
    
    // Fall back to last substantial text block
    // Limit length to avoid context overflow
    if len(output) > 5000 {
        return output[:5000] + "\n... [output truncated]"
    }
    
    return output
}
```

## Use Cases

### Parallel Investigation
```
Main Agent: "I need to check both the frontend and backend for bugs"
Main Agent: Uses subagent with persona "Frontend expert" for UI bugs
Main Agent: Uses subagent with persona "Backend expert" for API bugs
Main Agent: Receives two separate summaries and combines them
```

### Specialized Code Review
```
Main Agent: "Review this code for security issues"
Main Agent: Uses subagent with persona "Security specialist"
Main Agent: Receives security review summary
```

### Documentation Generation
```
Main Agent: "Generate docs for these functions"
Main Agent: Uses subagent with persona "Technical writer"
Main Agent: Receives documentation summary
```

## Testing Requirements

### Unit Tests
- [ ] ExecuteSubagent handles missing prompt parameter
- [ ] ExecuteSubagent correctly passes persona to subprocess
- [ ] ExecuteSubagent extracts summary from output
- [ ] extractSummary handles "=== Final Output ===" marker
- [ ] extractSummary truncates long output appropriately
- [ ] extractSummary handles empty output
- [ ] ExecuteSubagent reports subprocess errors correctly

### Integration Tests
- [ ] Subagent spawns successfully
- [ ] Subagent receives correct prompt and persona
- [ ] Subagent returns valid summary
- [ ] Main agent receives subagent result in clear format
- [ ] Subagent works with read-only mode
- [ ] Subagent works with various personas

### Error Handling Tests
- [ ] Subagent handles binary not found error
- [ ] Subagent handles subprocess timeout
- [ ] Subagent handles invalid persona gracefully
- [ ] Subagent error messages are clear and actionable
