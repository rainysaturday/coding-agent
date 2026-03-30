# Requirement 025: Non-Interactive One-Shot Prompting Mode

## Description
The coding agent harness must support a non-interactive one-shot prompting mode where the CLI process accepts an initial prompt as a command-line argument or stdin, starts the agent to process it, and exits automatically when the agent completes its work.

## Acceptance Criteria
- [ ] Accept prompt via command-line flag (e.g., `--prompt "..."` or `-p "..."`)
- [ ] Accept prompt via stdin in non-interactive mode
- [ ] Agent runs autonomously without requiring user input
- [ ] CLI exits automatically when agent completes the task
- [ ] Exit code indicates success (0) or failure (non-zero)
- [ ] Output is written to stdout/stderr in a clean format
- [ ] No interactive TUI is displayed in one-shot mode
- [ ] Tool calls and results are logged to stdout for visibility
- [ ] Final summary/results are clearly presented before exit

## Command-Line Interface

### Basic Usage
```bash
# One-shot mode with prompt flag
coding-agent --prompt "Create a Go function that adds two numbers"

# One-shot mode with prompt flag (short form)
coding-agent -p "Fix the bug in main.go"

# One-shot mode with stdin
echo "Refactor the code in utils.go" | coding-agent --stdin

coding-agent < prompt.txt

# One-shot mode with file input
coding-agent --prompt-file prompt.txt
```

### Exit Codes
```bash
0  - Success: Agent completed the task
1  - Error: Agent failed or encountered an error
2  - Usage error: Invalid arguments or flags
3  - Authentication error: API key missing or invalid
4  - Context limit: Task exceeded context limits
```

### Help Output
```bash
$ coding-agent --help
Minimal Coding Agent Harness

Usage:
  coding-agent [OPTIONS] [COMMAND]

Options:
  -p, --prompt string      Prompt for one-shot mode (non-interactive)
      --stdin              Read prompt from stdin
      --prompt-file path   Read prompt from file
      --model string       Model to use (default: "llama3")
      --temperature float  Inference temperature (default: 0.7)
      --max-tokens int     Maximum tokens to generate (default: 4096)
      --verbose            Enable verbose output
      --quiet              Suppress non-essential output
      --output file        Write results to file
      --no-stream          Disable streaming output
  -h, --help               Show this help message
  -v, --version            Show version information

Examples:
  coding-agent -p "Create a REST API in Go"
  coding-agent --prompt-file task.txt
  echo "Fix bug" | coding-agent --stdin
```

## Implementation Details

### Mode Detection
```go
type RunMode int

const (
    InteractiveMode RunMode = iota
    OneShotMode
)

type Config struct {
    Mode        RunMode
    Prompt      string
    PromptFile  string
    UseStdin    bool
    Verbose     bool
    Quiet       bool
    OutputFile  string
    // ... other config
}

func detectMode(cfg *Config) (RunMode, error) {
    if cfg.Prompt != "" || cfg.PromptFile != "" || cfg.UseStdin {
        return OneShotMode, nil
    }
    return InteractiveMode, nil
}
```

### One-Shot Mode Execution Flow
```go
func runOneShotMode(cfg *Config) error {
    // 1. Load prompt
    prompt, err := loadPrompt(cfg)
    if err != nil {
        return fmt.Errorf("failed to load prompt: %w", err)
    }
    
    // 2. Initialize agent
    agent := NewAgent(cfg)
    
    // 3. Run agent with prompt
    result, err := agent.Run(prompt)
    if err != nil {
        return fmt.Errorf("agent execution failed: %w", err)
    }
    
    // 4. Output result
    if cfg.OutputFile != "" {
        err = writeToFile(cfg.OutputFile, result)
    } else {
        err = printResult(result, cfg)
    }
    
    if err != nil {
        return fmt.Errorf("failed to output result: %w", err)
    }
    
    return nil
}
```

### Prompt Loading
```go
func loadPrompt(cfg *Config) (string, error) {
    if cfg.Prompt != "" {
        return cfg.Prompt, nil
    }
    
    if cfg.PromptFile != "" {
        content, err := os.ReadFile(cfg.PromptFile)
        if err != nil {
            return "", err
        }
        return string(content), nil
    }
    
    if cfg.UseStdin {
        reader := bufio.NewReader(os.Stdin)
        var prompt strings.Builder
        for {
            line, err := reader.ReadString('\n')
            if err != nil {
                if err == io.EOF {
                    break
                }
                return "", err
            }
            prompt.WriteString(line)
        }
        return strings.TrimSpace(prompt.String()), nil
    }
    
    return "", fmt.Errorf("no prompt provided")
}
```

### Output Formatting
```go
func printResult(result *AgentResult, cfg *Config) error {
    if cfg.Quiet {
        // Minimal output - just the final answer
        fmt.Println(result.FinalOutput)
        return nil
    }
    
    // Verbose output with tool calls
    if cfg.Verbose {
        fmt.Println("=== Agent Execution Log ===")
        for _, step := range result.Steps {
            fmt.Printf("\n[Step] %s\n", step.Action)
            if step.ToolCall != nil {
                fmt.Printf("Tool: %s\n", step.ToolCall.Name)
                fmt.Printf("Parameters: %s\n", step.ToolCall.Parameters)
            }
            if step.ToolResult != nil {
                fmt.Printf("Result: %s\n", step.ToolResult.Output)
            }
        }
        fmt.Println("\n=== Final Output ===")
    }
    
    fmt.Println(result.FinalOutput)
    
    // Summary statistics
    if cfg.Verbose {
        fmt.Printf("\n=== Summary ===")
        fmt.Printf("Steps executed: %d\n", len(result.Steps))
        fmt.Printf("Tokens used: %d\n", result.TokenUsage)
        fmt.Printf("Duration: %s\n", result.Duration)
    }
    
    return nil
}
```

## Agent Behavior in One-Shot Mode

### Autonomous Execution
- Agent receives the initial prompt
- Agent analyzes the request
- Agent makes tool calls autonomously
- Agent continues until task is complete
- Agent provides final output
- Agent exits (CLI terminates)

### No User Intervention
- No interactive prompts for confirmation
- No waiting for user input between steps
- No TUI for navigation or editing
- Agent makes decisions autonomously

### Error Handling
```go
func (a *Agent) Run(prompt string) (*AgentResult, error) {
    ctx := context.Background()
    
    // Initialize context
    err := a.InitializeContext(prompt)
    if err != nil {
        return nil, err
    }
    
    // Main execution loop
    for {
        // Get LLM response
        response, err := a.CallInference(ctx)
        if err != nil {
            return nil, err
        }
        
        // Check if response contains tool calls
        if response.HasToolCalls() {
            // Execute tool calls
            for _, toolCall := range response.ToolCalls {
                result := a.ExecuteTool(toolCall)
                a.AddToContext(result)
            }
            continue // Loop for next response
        }
        
        // Check if task is complete
        if response.IsFinalAnswer() {
            return &AgentResult{
                FinalOutput: response.Content,
                Steps:       a.ExecutionSteps,
                TokenUsage:  a.TotalTokens,
                Duration:    time.Since(a.StartTime),
            }, nil
        }
        
        // Check for errors
        if response.HasError() {
            return nil, fmt.Errorf("agent error: %s", response.Error)
        }
    }
}
```

## Logging and Visibility

### Tool Call Logging
```bash
# Verbose mode shows all tool interactions
$ coding-agent -p "Create a Go file" --verbose

[Step] Creating new file
Tool: write_file
Parameters: {"path":"main.go","content":"package main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}"}
Result: File written successfully

[Step] Task complete
Final Output: Created main.go with a simple Go program.

=== Summary ===
Steps executed: 1
Tokens used: 245
Duration: 2.3s
```

### Quiet Mode
```bash
# Quiet mode shows only final output
$ coding-agent -p "Create a Go file" --quiet
Created main.go with a simple Go program.
```

## Use Cases

### CI/CD Integration
```bash
# Automated code review
coding-agent --prompt "Review the code in src/ for security issues" \
    --output review-report.txt

# Automated refactoring
coding-agent -p "Refactor legacy code in old_module.go to use modern Go patterns"

# Documentation generation
coding-agent --prompt "Generate documentation for the API in api.go" \
    --output docs.md
```

### Script Integration
```bash
#!/bin/bash
# Automated task execution

coding-agent -p "$(cat task.txt)" \
    --output result.txt

if [ $? -eq 0 ]; then
    echo "Task completed successfully"
    cat result.txt
else
    echo "Task failed"
    exit 1
fi
```

### Batch Processing
```bash
# Process multiple tasks
for task in tasks/*.txt; do
    coding-agent --prompt-file "$task" \
        --output "results/$(basename $task .txt).txt"
done
```

## Testing Requirements

### Unit Tests
- [ ] Prompt loading from flag works correctly
- [ ] Prompt loading from stdin works correctly
- [ ] Prompt loading from file works correctly
- [ ] Exit code is 0 on success
- [ ] Exit code is non-zero on failure
- [ ] Output is written correctly
- [ ] Verbose mode shows execution details
- [ ] Quiet mode shows only final output

### Integration Tests
- [ ] One-shot mode completes simple tasks
- [ ] One-shot mode handles multi-step tasks
- [ ] One-shot mode handles tool calls correctly
- [ ] One-shot mode exits when task is complete
- [ ] One-shot mode handles errors gracefully

## Related Requirements
- **001-go-runtime.md**: Go runtime requirements
- **015-tool-prefix-prompt.md**: System prompt requirements
- **024-zero-external-dependencies.md**: No external dependencies
