# Requirement 039: Configurable Agent Persona

## Description
The coding agent harness must support a `--persona` flag that allows users to configure a custom persona for the agent. The persona is injected into the system prompt as an additional section that influences the agent's behavior, tone, and expertise area. This enables users to tailor the agent's responses for specific use cases such as code review, documentation writing, or security analysis.

## Acceptance Criteria
- [ ] `--persona` flag is available as a command-line option with a string argument
- [ ] Persona is injected into the system prompt under a "YOUR PERSONA" section
- [ ] The persona section appears after the tool definitions and instructions
- [ ] The system prompt part declaring tools and how to use them remains unchanged
- [ ] Persona is preserved across all iterations of the conversation
- [ ] The persona is also passed to subagent calls when the subagent tool is used
- [ ] `--summary-only` flag exists for lightweight subagent-like operations
- [ ] When `--summary-only` is enabled, only the final output is shown (no verbose logging)
- [ ] Persona can be combined with `--read-only` mode
- [ ] Persona can be combined with `--verbose` or `--quiet` flags
- [ ] No persona means the default behavior (no persona section in prompt)

## Command-Line Usage

```bash
# With persona for expert Go development
./coding-agent --persona "Expert Go developer focused on clean code and performance" \
    --prompt "Review this code"

# With persona for security review
./coding-agent --persona "Senior security engineer specializing in OWASP Top 10" \
    --prompt "Check this code for vulnerabilities"

# With persona for documentation
./coding-agent --persona "Technical writer specializing in API documentation" \
    --prompt "Generate documentation for these functions"

# Read-only mode with persona
./coding-agent --read-only --persona "Code reviewer focused on best practices" \
    --prompt "Review the code quality"

# Summary-only mode (for programmatic use)
./coding-agent --summary-only --persona "Concise technical assistant" \
    --prompt "What is the answer?"
```

## System Prompt Structure

When a persona is specified, the system prompt should include a section like:

```
...
[Existing system prompt content with tools and instructions]
...

YOUR PERSONA:
{persona_text}

... [followed by any summary-only instructions if applicable]
```

## Persona Behavior

### Influence on Responses
- The persona should influence the agent's tone, style, and depth of explanation
- A security-focused persona should highlight security considerations
- A documentation-focused persona should emphasize clear explanations
- An expert persona should provide in-depth technical analysis

### Consistency
- The persona should be consistent throughout the conversation
- All tool call descriptions and explanations should reflect the persona
- The final output should be written in the persona's voice

## Summary-Only Mode

When `--summary-only` is enabled:
- The agent is instructed to provide only a concise summary/conclusion
- No verbose explanations or step-by-step details are included
- Only the essential outcome and critical findings are reported
- This mode is primarily used by subagent calls to avoid overloading the main agent

## Implementation Details

### Config Structure
```go
type Config struct {
    Persona      string // Custom persona for the agent
    SummaryOnly  bool   // When true, only output final summary
    // ... other fields
}
```

### System Prompt Builder
```go
func buildSystemPrompt(readOnly bool, persona string, summaryOnly bool) string {
    basePrompt := /* existing system prompt with tools */
    
    // Add persona section if provided
    if persona != "" {
        basePrompt += fmt.Sprintf("\n\nYOUR PERSONA:\n%s\n", persona)
    }
    
    // Add summary-only instruction if needed
    if summaryOnly {
        basePrompt += "\n\nIMPORTANT OUTPUT INSTRUCTION: You are running in summary-only mode. Your final output should be a concise summary/conclusion of the work completed. Do NOT include verbose explanations, step-by-step details, or code. Only provide the essential outcome and any critical findings."
    }
    
    return basePrompt
}
```

### CLI Flag Parsing
```go
case "--persona":
    if i+1 >= len(args) {
        return nil, fmt.Errorf("--persona requires an argument")
    }
    i++
    cfg.Persona = args[i]

case "--summary-only":
    cfg.SummaryOnly = true
```

## Use Cases

### Code Review
```bash
./coding-agent --persona "Senior code reviewer with 10+ years experience" \
    -p "Review src/main.go for code quality issues"
```

### Security Audit
```bash
./coding-agent --persona "Security specialist focused on finding vulnerabilities" \
    --read-only \
    -p "Audit this codebase for security issues"
```

### Documentation Generation
```bash
./coding-agent --persona "Technical writer who creates clear API documentation" \
    -p "Generate documentation for the functions in api.go"
```

### Quick Summary
```bash
# Get just the answer without verbose output
./coding-agent --summary-only --persona "Concise technical assistant" \
    -p "What bugs are in this code?"
```

## Testing Requirements

### Unit Tests
- [ ] Persona is correctly added to system prompt
- [ ] No persona means no persona section in prompt
- [ ] Summary-only mode adds correct instructions
- [ ] Summary-only mode does not add persona instructions
- [ ] Persona and summary-only can be combined
- [ ] Persona works with read-only mode

### Integration Tests
- [ ] Agent uses persona in its responses
- [ ] Persona influences tone and style
- [ ] Summary-only mode outputs only the final answer
- [ ] Subagent calls can specify a persona
