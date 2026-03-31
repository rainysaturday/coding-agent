package main

import (
	"bufio"
	"coding-agent/config"
	"coding-agent/inference"
	"coding-agent/stats"
	"coding-agent/terminal"
	"coding-agent/tools"
	"coding-agent/tui"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Import context package with alias to avoid naming conflict
import ctxpkg "coding-agent/context"

// Version information injected at build time via ldflags
var (
	gitHash   string = "unknown"
	gitDirty  string = "unknown"
	buildTime string = ""
)

// System prompt with all tools - using JSON-based tool calling format
const systemPrompt = `You are a helpful coding assistant. You have access to the following tools:

AVAILABLE TOOLS:
- bash: Execute a bash command
  Format: [TOOL:{"name":"bash","parameters":{"command":"command string"}}]
  Example: [TOOL:{"name":"bash","parameters":{"command":"ls -la"}}]
  Multi-line: [TOOL:{"name":"bash","parameters":{"command":"line1\nline2\nline3"}}]

- read_file: Read the contents of a file
  Format: [TOOL:{"name":"read_file","parameters":{"path":"file path"}}]
  Example: [TOOL:{"name":"read_file","parameters":{"path":"/path/to/file.txt"}}]

- write_file: Write content to a file
  Format: [TOOL:{"name":"write_file","parameters":{"path":"file path","content":"file content"}}]
  Example: [TOOL:{"name":"write_file","parameters":{"path":"/path/to/file.txt","content":"Hello"}}]
  Multi-line: [TOOL:{"name":"write_file","parameters":{"path":"file.txt","content":"line1\nline2"}}]

- read_lines: Read a specific line range from a file
  Format: [TOOL:{"name":"read_lines","parameters":{"path":"file path","start":line_number,"end":line_number}}]
  Example: [TOOL:{"name":"read_lines","parameters":{"path":"/path/to/file.txt","start":1,"end":10}}]

- insert_lines: Insert lines at a specific line number
  Format: [TOOL:{"name":"insert_lines","parameters":{"path":"file path","line":line_number,"lines":"lines to insert"}}]
  Example: [TOOL:{"name":"insert_lines","parameters":{"path":"/path/to/file.txt","line":5,"lines":"new line"}}]
  Multi-line: [TOOL:{"name":"insert_lines","parameters":{"path":"file.txt","line":5,"lines":"line1\nline2"}}]

- replace_lines: Replace content in a file (two modes available)

  MODE 1: Line-number mode - Replace lines at specific positions
  Format: [TOOL:{"name":"replace_lines","parameters":{"path":"file path","start":line_number,"end":line_number,"lines":"replacement lines"}}]
  Example: [TOOL:{"name":"replace_lines","parameters":{"path":"/path/to/file.txt","start":1,"end":5,"lines":"new content"}}]
  Use when: You know exact line numbers from reading the file first

  MODE 2: Search-and-replace mode - Find and replace text patterns (RECOMMENDED for LLMs)
  Format: [TOOL:{"name":"replace_lines","parameters":{"path":"file path","search":"text to find","replace":"replacement text"}}]
  Example: [TOOL:{"name":"replace_lines","parameters":{"path":"./main.go","search":"oldFunction","replace":"newFunction"}}]
  Multi-line: [TOOL:{"name":"replace_lines","parameters":{"path":"./file.txt","search":"old\nlines","replace":"new\nlines"}}]
  Replace all: [TOOL:{"name":"replace_lines","parameters":{"path":"./file.txt","search":"TODO","replace":"DONE","count":"all"}}]
  Use when: You know the text to find but not the exact line numbers

TOOL CALLING RULES:
- Use the exact JSON format shown above for tool calls
- Tool calls must be enclosed in [TOOL:...] brackets
- The content inside brackets must be valid JSON
- Tool name must match exactly (case-sensitive, use underscore not hyphen)
- Parameters must be in a JSON object under the "parameters" key
- String values must be properly JSON-escaped (use \n for newlines, \" for quotes)
- Numeric values should be JSON numbers without quotes (e.g. "start":1, "end":10)

Instructions:
- Analyze the user's request and determine if tools are needed
- Use tools when they can help complete the task
- Always explain your reasoning before calling tools
- Provide clear explanations of tool results
- Continue the conversation after tool execution
- Generate valid JSON inside the [TOOL:...] wrapper

FILE MODIFICATION BEST PRACTICES:
- ALWAYS read a file first using read_file or read_lines before modifying it
- PREFER search-and-replace mode over line-number mode when possible (less error-prone)
- For search-and-replace: Use a unique search pattern to avoid unintended replacements
- For line-number mode: Double-check line numbers match what you read from the file
- After any file modification, read the file again to verify the changes
- If a search-and-replace fails (text not found), re-read the file and adjust your search pattern
- Be careful with multi-line replacements - ensure proper JSON escaping (\n for newlines)

VERIFICATION REQUIREMENTS:
- ALWAYS double-check your work before considering a task complete
- Verify that created/modified files exist and contain the expected content
- Test code execution when possible (e.g., run go build, go test)
- Validate that changes meet the user's requirements
- If you make multiple changes, verify each one independently
- Re-read files after writing to confirm content was written correctly
- Run validation commands (e.g., 'go vet', 'gofmt -d', 'cat' to view files)
- If verification fails, fix the issue and re-verify
- Provide a final verification summary before concluding the task

Verification Checklist:
1. Files exist at the expected paths
2. File content matches the intended changes
3. Code compiles without errors (for Go code)
4. Code follows Go formatting standards (gofmt)
5. Changes align with user requirements
6. No unintended side effects or broken dependencies`

// RunMode represents the execution mode of the agent
type RunMode int

const (
	InteractiveMode RunMode = iota
	OneShotMode
)

// AgentResult holds the result of agent execution
type AgentResult struct {
	FinalOutput  string
	Steps        []ExecutionStep
	TokenUsage   int64
	Duration     time.Duration
	Success      bool
	ErrorMessage string
}

// ExecutionStep represents a single step in agent execution
type ExecutionStep struct {
	Action     string
	ToolCall   *ToolCall
	ToolResult *ToolResult
	Timestamp  time.Time
}

// ToolCall represents a tool call
type ToolCall struct {
	Name       string
	Parameters string
}

// ToolResult represents a tool execution result
type ToolResult struct {
	Output string
	Error  string
}

// exit codes
const (
	ExitSuccess      = 0
	ExitError        = 1
	ExitUsageError   = 2
	ExitAuthError    = 3
	ExitContextLimit = 4
)

func main() {
	// Parse command-line flags
	// One-shot mode flags
	prompt := flag.String("prompt", "", "Prompt for one-shot mode (non-interactive)")
	promptShort := flag.String("p", "", "Prompt for one-shot mode (short form)")
	useStdin := flag.Bool("stdin", false, "Read prompt from stdin")
	promptFile := flag.String("prompt-file", "", "Read prompt from file")

	// Output/formatting flags
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	quiet := flag.Bool("quiet", false, "Suppress non-essential output")
	outputFile := flag.String("output", "", "Write results to file")
	noStream := flag.Bool("no-stream", false, "Disable streaming output")

	// Configuration flags
	configFile := flag.String("config", "", "Path to configuration file")
	endpoint := flag.String("endpoint", "", "Inference endpoint URL")
	contextSize := flag.Int("context-size", 0, "Context size in tokens")
	initialTokenTimeout := flag.Int("timeout", 0, "Initial token timeout in seconds")
	streaming := flag.Int("streaming", -1, "Enable/disable streaming (-1 for default, 0=false, 1=true)")
	maxIterations := flag.Int("max-iterations", 0, "Maximum tool call iterations")
	model := flag.String("model", "", "Model to use")
	_ = flag.Float64("temperature", 0, "Inference temperature") // Reserved for future use
	_ = flag.Int("max-tokens", 0, "Maximum tokens to generate") // Reserved for future use

	// Help/version
	help := flag.Bool("help", false, "Show this help message")
	helpShort := flag.Bool("h", false, "Show this help message")
	version := flag.Bool("version", false, "Show version information")
	versionShort := flag.Bool("v", false, "Show version information")

	flag.Parse()

	// Handle help
	if *help || *helpShort {
		printHelp()
		return
	}

	// Handle version
	if *version || *versionShort {
		fmt.Printf("coding-agent version %s (git: %s, dirty: %s, built: %s)\n",
			gitHash, gitHash, gitDirty, buildTime)
		return
	}

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(ExitUsageError)
	}

	// Override config with command-line flags
	if *endpoint != "" {
		cfg.InferenceEndpoint = *endpoint
	}
	if *contextSize > 0 {
		cfg.ContextSize = *contextSize
	}
	if *initialTokenTimeout > 0 {
		cfg.InitialTokenTimeout = *initialTokenTimeout
	}
	if *streaming != -1 {
		cfg.StreamingEnabled = *streaming == 1
	}
	if *maxIterations > 0 {
		cfg.MaxIterations = *maxIterations
	}
	if *model != "" {
		cfg.Model = *model
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(ExitUsageError)
	}

	// Detect run mode
	mode := detectRunMode(*prompt, *promptShort, *promptFile, *useStdin)

	// Run appropriate mode
	var result *AgentResult

	switch mode {
	case OneShotMode:
		result, err = runOneShotMode(cfg, *prompt, *promptShort, *promptFile, *useStdin,
			*verbose, *quiet, *outputFile, *noStream)
		if err != nil {
			if !*quiet {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			os.Exit(ExitError)
		}
	case InteractiveMode:
		err = runInteractiveMode(cfg, *noStream)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(ExitError)
		}
		return
	}

	// Exit with appropriate code
	if result.Success {
		os.Exit(ExitSuccess)
	} else {
		os.Exit(ExitError)
	}
}

// detectRunMode determines whether to run in interactive or one-shot mode
func detectRunMode(prompt, promptShort, promptFile string, useStdin bool) RunMode {
	if prompt != "" || promptShort != "" || promptFile != "" || useStdin {
		return OneShotMode
	}
	return InteractiveMode
}

// loadPrompt loads the prompt from various sources
func loadPrompt(prompt, promptShort, promptFile string, useStdin bool) (string, error) {
	// Check command-line flag (long form)
	if prompt != "" {
		return prompt, nil
	}

	// Check command-line flag (short form)
	if promptShort != "" {
		return promptShort, nil
	}

	// Check prompt file
	if promptFile != "" {
		content, err := os.ReadFile(promptFile)
		if err != nil {
			return "", fmt.Errorf("failed to read prompt file: %w", err)
		}
		return strings.TrimSpace(string(content)), nil
	}

	// Check stdin
	if useStdin {
		reader := bufio.NewReader(os.Stdin)
		var promptBuilder strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				return "", fmt.Errorf("failed to read from stdin: %w", err)
			}
			promptBuilder.WriteString(line)
		}
		return strings.TrimSpace(promptBuilder.String()), nil
	}

	return "", fmt.Errorf("no prompt provided")
}

// runOneShotMode executes the agent in one-shot mode
func runOneShotMode(cfg *config.Config, prompt, promptShort, promptFile string,
	useStdin bool, verbose, quiet bool, outputFile string, noStream bool) (*AgentResult, error) {

	// 1. Load prompt
	promptText, err := loadPrompt(prompt, promptShort, promptFile, useStdin)
	if err != nil {
		return nil, err
	}

	if !quiet {
		if verbose {
			fmt.Println("=== Loading Prompt ===")
		}
	}

	// 2. Initialize agent components
	statsTracker := stats.NewStats()
	toolRegistry := tools.NewToolRegistry()
	toolRegistry.Register(tools.NewBashTool())
	toolRegistry.Register(tools.NewReadFileTool())
	toolRegistry.Register(tools.NewWriteFileTool())
	toolRegistry.Register(tools.NewReadLinesTool())
	toolRegistry.Register(tools.NewInsertLinesTool())
	toolRegistry.Register(tools.NewReplaceLinesTool())

	ctx := ctxpkg.NewContext(systemPrompt, cfg.ContextSize)
	startTime := time.Now()

	// 3. Create inference client
	client := inference.NewInferenceClient(
		cfg.InferenceEndpoint,
		cfg.APIKey,
		cfg.Model,
		cfg.ContextSize,
		cfg.InitialTokenTimeout,
		cfg.ConnectionTimeout,
		cfg.ReadTimeout,
		!noStream,
		statsTracker,
	)

	// 4. Add prompt to context
	ctx.AddUserMessage(promptText)

	// 5. Run agent with prompt
	result, err := runAgent(ctx, client, toolRegistry, statsTracker, cfg.MaxIterations,
		verbose, quiet, startTime)
	if err != nil {
		return nil, fmt.Errorf("agent execution failed: %w", err)
	}

	// 6. Output result
	if outputFile != "" {
		err = writeToFile(outputFile, result.FinalOutput)
		if err != nil {
			return nil, fmt.Errorf("failed to write output to file: %w", err)
		}
		if !quiet {
			fmt.Printf("Results written to %s\n", outputFile)
		}
	} else {
		err = printResult(result, verbose, quiet)
		if err != nil {
			return nil, fmt.Errorf("failed to output result: %w", err)
		}
	}

	return result, nil
}

// runAgent executes the main agent loop
func runAgent(ctx *ctxpkg.Context, client *inference.InferenceClient,
	registry *tools.ToolRegistry, stats *stats.Stats, maxIterations int,
	verbose, quiet bool, startTime time.Time) (*AgentResult, error) {

	iterations := 0
	steps := []ExecutionStep{}

	for {
		iterations++
		stats.IncrementIteration()

		// Check iteration limit
		if iterations > maxIterations {
			return &AgentResult{
				FinalOutput:  "Maximum iterations reached. Please refine your request.",
				Steps:        steps,
				TokenUsage:   stats.GetTotalTokens(),
				Duration:     time.Since(startTime),
				Success:      false,
				ErrorMessage: "Maximum iterations reached",
			}, nil
		}

		// Check context size
		if ctx.IsOverLimit() {
			if !quiet {
				fmt.Println("Context size exceeded. Compressing context...")
			}
			if verbose {
				fmt.Println("Warning: Context was too large. Starting fresh conversation.")
			}
			// For now, just clear and warn
			ctx.Clear()
		}

		// Send to inference client
		var assistantContent string
		err := client.ChatCompletion(inference.ChatCompletionRequest{
			Context: ctx,
			OnToken: func(token string) {
				assistantContent += token
				if !quiet {
					fmt.Print(token)
				}
			},
			OnComplete: func() {
				if !quiet {
					fmt.Println()
				}
			},
			OnError: func(err error) {
				if !quiet {
					fmt.Fprintf(os.Stderr, "Inference error: %v\n", err)
				}
			},
		})
		if err != nil {
			return &AgentResult{
				FinalOutput:  "",
				Steps:        steps,
				TokenUsage:   stats.GetTotalTokens(),
				Duration:     time.Since(startTime),
				Success:      false,
				ErrorMessage: err.Error(),
			}, fmt.Errorf("inference failed: %w", err)
		}

		// Check for tool calls in assistant response
		toolCalls, err := tools.ExtractToolCalls(assistantContent)
		if err != nil {
			if !quiet {
				fmt.Fprintf(os.Stderr, "Failed to parse tool calls: %v\n", err)
			}
			stats.AddFailedToolCall()
			errorMsg := fmt.Sprintf("Failed to parse tool calls: %s", err)
			ctx.AddToolResult("unknown", false, errorMsg, errorMsg)
			continue
		}

		if len(toolCalls) == 0 {
			// No tool calls, conversation complete - this is the final answer
			return &AgentResult{
				FinalOutput: assistantContent,
				Steps:       steps,
				TokenUsage:  stats.GetTotalTokens(),
				Duration:    time.Since(startTime),
				Success:     true,
			}, nil
		}

		// Execute tool calls
		for _, tc := range toolCalls {
			// Convert map to JSON string for display
			paramsJSON, _ := json.Marshal(tc.Params)

			step := ExecutionStep{
				Action:    "Tool call",
				ToolCall:  &ToolCall{Name: tc.Name, Parameters: string(paramsJSON)},
				Timestamp: time.Now(),
			}

			// Display tool call with key parameters
			keyParam := tools.GetRelevantParameter(tc.Name, tc.Params)
			if !quiet {
				if keyParam != "" {
					if verbose {
						fmt.Printf("[Step] Calling tool: %s (%s)\n", tc.Name, keyParam)
					} else {
						fmt.Printf("Calling tool: %s (%s)\n", tc.Name, keyParam)
					}
				} else {
					if verbose {
						fmt.Printf("[Step] Calling tool: %s\n", tc.Name)
					} else {
						fmt.Printf("Calling tool: %s\n", tc.Name)
					}
				}
			}

			tool, ok := registry.Get(tc.Name)
			if !ok {
				errorMsg := fmt.Sprintf("unknown tool: %s", tc.Name)
				if !quiet {
					fmt.Printf("  ERROR: %s\n", errorMsg)
				}
				ctx.AddToolResult(tc.Name, false, "", errorMsg)
				stats.AddFailedToolCall()
				step.ToolResult = &ToolResult{Error: errorMsg}
				steps = append(steps, step)
				continue
			}

			result := tool.Execute(tc.Params)
			stats.AddToolCall()

			if !result.Success {
				stats.AddFailedToolCall()
				if !quiet {
					fmt.Printf("  Tool '%s' failed: %s\n", tc.Name, result.Error)
				}
				ctx.AddToolResult(tc.Name, false, result.Output, result.Error)
				step.ToolResult = &ToolResult{Output: result.Output, Error: result.Error}
			} else {
				if !quiet {
					fmt.Printf("  Tool '%s' executed successfully\n", tc.Name)
					// Truncate long output for display
					displayOutput := tools.TruncateOutput(result.Output, 500)
					if displayOutput != "" && verbose {
						fmt.Println("  " + displayOutput)
					}
				}
				ctx.AddToolResult(tc.Name, true, result.Output, result.Error)
				step.ToolResult = &ToolResult{Output: result.Output, Error: result.Error}
			}

			steps = append(steps, step)
		}

		// Continue loop to get response based on tool results
	}
}

// printResult prints the agent result
func printResult(result *AgentResult, verbose, quiet bool) error {
	if quiet {
		// Minimal output - just the final answer
		fmt.Println(result.FinalOutput)
		return nil
	}

	// Verbose output with tool calls
	if verbose {
		fmt.Println("=== Agent Execution Log ===")
		for i, step := range result.Steps {
			fmt.Printf("\n[Step %d] %s\n", i+1, step.Action)
			if step.ToolCall != nil {
				fmt.Printf("Tool: %s\n", step.ToolCall.Name)
				fmt.Printf("Parameters: %s\n", step.ToolCall.Parameters)
			}
			if step.ToolResult != nil {
				if step.ToolResult.Error != "" {
					fmt.Printf("Result: ERROR - %s\n", step.ToolResult.Error)
				} else {
					fmt.Printf("Result: %s\n", tools.TruncateOutput(step.ToolResult.Output, 200))
				}
			}
		}
		fmt.Println("\n=== Final Output ===")
	}

	fmt.Println(result.FinalOutput)

	// Summary statistics
	if verbose {
		fmt.Printf("\n=== Summary ===\n")
		fmt.Printf("Steps executed: %d\n", len(result.Steps))
		fmt.Printf("Tokens used: %d\n", result.TokenUsage)
		fmt.Printf("Duration: %s\n", result.Duration)
	}

	return nil
}

// writeToFile writes content to a file
func writeToFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// runInteractiveMode executes the agent in interactive mode
func runInteractiveMode(cfg *config.Config, noStream bool) error {
	// Create statistics tracker
	statsTracker := stats.NewStats()

	// Create TUI
	tui := tui.NewTUI(statsTracker, gitHash, gitDirty, buildTime)
	tui.DisplayWelcome()

	// Create tool registry
	toolRegistry := tools.NewToolRegistry()
	toolRegistry.Register(tools.NewBashTool())
	toolRegistry.Register(tools.NewReadFileTool())
	toolRegistry.Register(tools.NewWriteFileTool())
	toolRegistry.Register(tools.NewReadLinesTool())
	toolRegistry.Register(tools.NewInsertLinesTool())
	toolRegistry.Register(tools.NewReplaceLinesTool())

	// Create context
	ctx := ctxpkg.NewContext(systemPrompt, cfg.ContextSize)

	// Create inference client
	client := inference.NewInferenceClient(
		cfg.InferenceEndpoint,
		cfg.APIKey,
		cfg.Model,
		cfg.ContextSize,
		cfg.InitialTokenTimeout,
		cfg.ConnectionTimeout,
		cfg.ReadTimeout,
		!noStream,
		statsTracker,
	)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Main interaction loop
	fmt.Println("\nType your request below (or 'quit' to exit):")

	for {
		// Check for shutdown signal
		select {
		case <-sigChan:
			fmt.Println("\nShutting down...")
			tui.DisplayStats(ctx)
			return nil
		default:
		}

		// Display context info before prompt
		tui.DisplayContextInfo(ctx)
		tui.DisplayPrompt()

		// Read input with history and cancellation support
		input, canceled, err := tui.ReadInput("")
		if canceled {
			tui.AddOutput("Request cancelled")
			continue
		}
		if err != nil {
			if err.Error() == "EOF" {
				fmt.Println("\nGoodbye!")
				return nil
			}
			tui.DisplayError("Failed to read input: %v", err)
			continue
		}

		// Check for quit
		if input == "quit" || input == "exit" {
			fmt.Println("\nGoodbye!")
			return nil
		}

		// Add to history before processing
		tui.AddToHistory(input)

		// Process commands
		if !tui.ProcessCommand(input, ctx) {
			continue
		}

		// Add user message to context (for LLM)
		ctx.AddUserMessage(input)

		// Process the conversation
		if err := processConversation(ctx, client, toolRegistry, statsTracker, cfg.MaxIterations, tui); err != nil {
			tui.DisplayError("Conversation error: %v", err)
			// Remove the failed user message from context
			messages := ctx.GetMessages()
			if len(messages) > 0 {
				ctx = ctxpkg.NewContext(systemPrompt, cfg.ContextSize)
				// Restore system prompt and any successful conversation
			}
		}
	}
}

// processConversation handles a single user request, including tool calls
func processConversation(ctx *ctxpkg.Context, client *inference.InferenceClient,
	registry *tools.ToolRegistry, stats *stats.Stats, maxIterations int, t *tui.TUI,
) error {
	iterations := 0
	for {
		iterations++
		stats.IncrementIteration()

		// Check iteration limit
		if iterations > maxIterations {
			t.AddOutput("Maximum iterations reached. Please refine your request.")
			return nil
		}

		// Check context size
		if ctx.IsOverLimit() {
			t.AddOutput("Context size exceeded. Compressing context...")
			// For now, just clear and warn
			t.AddOutput("Warning: Context was too large. Starting fresh conversation.")
			ctx.Clear()
		}

		// Drain stdin before setting raw mode to clear any leftover bytes
		// This prevents buffer desynchronization between bufio.Reader and raw reads
		drainStdin()

		// Set terminal to raw mode for immediate ESC key detection
		if err := terminal.SetRawMode(); err != nil {
			// If we can't set raw mode, continue without ESC cancellation
			// This is a fallback for non-TTY environments
		}

		// Create a cancellable context for this inference request
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancelOnce := &sync.Once{}
		escCancelled := make(chan struct{})
		stopEscListener := make(chan struct{})
		escListenerDone := make(chan struct{})

		// Set up ESC key listener in a separate goroutine
		// This goroutine watches for ESC key press during inference
		go func() {
			defer close(escListenerDone)
			buf := make([]byte, 1)
			for {
				// Use a non-blocking check to see if we should stop
				select {
				case <-stopEscListener:
					return
				default:
				}

				// Set a short timeout for reading so we can check stop signal
				// We need to read with a timeout to allow checking stopEscListener
				os.Stdin.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				n, err := os.Stdin.Read(buf)
				os.Stdin.SetReadDeadline(time.Time{}) // Clear deadline

				if err != nil {
					// Timeout or other error - continue loop to check stop signal
					if ne, ok := err.(interface{ Timeout() bool }); ok && ne.Timeout() {
						continue
					}
					return
				}
				if n > 0 && buf[0] == 27 { // ESC key (ASCII 27)
					cancelOnce.Do(func() {
						cancel()
						close(escCancelled)
					})
					return
				}
			}
		}()

		// Send to inference client
		var assistantContent string
		var canceled bool
		err := client.ChatCompletion(inference.ChatCompletionRequest{
			Context:   ctx,
			CancelCtx: cancelCtx,
			OnToken: func(token string) {
				// Check for cancellation before adding token
				select {
				case <-cancelCtx.Done():
					return
				default:
				}
				assistantContent += token
				fmt.Print(token)
			},
			OnComplete: func() {
				fmt.Println()
				t.AddOutputf("[Assistant] %s", assistantContent)
			},
			OnError: func(err error) {
				t.DisplayError("Inference error: %v", err)
			},
			OnCancel: func() {
				canceled = true
				fmt.Println("\nRequest cancelled")
			},
		})

		// Signal the ESC listener to stop and wait for it to finish
		close(stopEscListener)
		<-escListenerDone
		cancel() // Clean up the cancel function
		terminal.RestoreMode() // Restore terminal mode after each inference call

		// Check if cancelled
		if canceled {
			// Don't add incomplete response to context
			// Remove the last user message that triggered this request
			messages := ctx.GetMessages()
			if len(messages) > 0 && messages[len(messages)-1].Role == "user" {
				ctx.RemoveLastMessage()
			}
			return nil
		}

		if err != nil {
			return fmt.Errorf("inference failed: %w", err)
		}

		// Check for tool calls in assistant response
		toolCalls, err := tools.ExtractToolCalls(assistantContent)
		if err != nil {
			t.DisplayError("Failed to parse tool calls: %v", err)
			// Failing to extract tools is a failure from LLM since it has probably generated bad call syntax
			stats.AddFailedToolCall()
			// Display error prominently
			t.AddOutputf("  \033[31mTool '%s' failed: %s\033[0m", "unknown", err)
			errorMsg := fmt.Sprintf("Failed to parse tool calls: %s", err)
			ctx.AddToolResult("unknown", false, errorMsg, errorMsg)
			continue
		}

		if len(toolCalls) == 0 {
			// No tool calls, conversation for this turn is complete
			break
		}

		// Execute tool calls
		for _, tc := range toolCalls {
			// Display tool call with key parameters (print immediately + add to buffer)
			keyParam := tools.GetRelevantParameter(tc.Name, tc.Params)
			if keyParam != "" {
				fmt.Printf("Calling tool: %s (%s)\n", tc.Name, keyParam)
				t.AddOutputf("Calling tool: %s (%s)", tc.Name, keyParam)
			} else {
				fmt.Printf("Calling tool: %s\n", tc.Name)
				t.AddOutputf("Calling tool: %s", tc.Name)
			}

			tool, ok := registry.Get(tc.Name)
			if !ok {
				errorMsg := fmt.Sprintf("unknown tool: %s", tc.Name)
				fmt.Printf("  \033[31mERROR: %s\033[0m\n", errorMsg)
				t.AddOutputf("  \033[31mERROR: %s\033[0m", errorMsg)
				ctx.AddToolResult(tc.Name, false, "", errorMsg)
				stats.AddFailedToolCall()
				continue
			}

			result := tool.Execute(tc.Params)
			stats.AddToolCall()

			if !result.Success {
				stats.AddFailedToolCall()
				// Display error prominently
				fmt.Printf("  \033[31mTool '%s' failed: %s\033[0m\n", tc.Name, result.Error)
				t.AddOutputf("  \033[31mTool '%s' failed: %s\033[0m", tc.Name, result.Error)
			} else {
				// Display success with formatted output
				fmt.Printf("  \033[32mTool '%s' executed successfully\033[0m\n", tc.Name)
				t.AddOutputf("  \033[32mTool '%s' executed successfully\033[0m", tc.Name)
				// Truncate long output for display
				displayOutput := tools.TruncateOutput(result.Output, 500)
				if displayOutput != "" {
					fmt.Println("  " + displayOutput)
					t.AddOutput("  " + displayOutput)
				}
			}

			// Add result to context (full output, not truncated)
			ctx.AddToolResult(tc.Name, result.Success, result.Output, result.Error)
		}

		// Continue loop to get response based on tool results
	}

	return nil
}

// drainStdin drains any pending bytes from stdin
// This is called before setting raw mode to prevent buffer desynchronization
// between the bufio.Reader used by TUI and the raw reads used by the ESC listener
func drainStdin() {
	buf := make([]byte, 1024)
	for {
		// Set a short timeout so we don't block indefinitely
		os.Stdin.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
		n, err := os.Stdin.Read(buf)
		os.Stdin.SetReadDeadline(time.Time{}) // Clear deadline

		if err != nil {
			// Timeout or EOF means no more pending input
			if ne, ok := err.(interface{ Timeout() bool }); ok && ne.Timeout() {
				break
			}
			if err == io.EOF {
				break
			}
			// Other errors - just continue
			break
		}
		// Discard any bytes read
		_ = n
	}
}

// printHelp prints the help message
func printHelp() {
	helpText := `Minimal Coding Agent Harness

Usage:
  coding-agent [OPTIONS] [COMMAND]

Options:
  -p, --prompt string      Prompt for one-shot mode (non-interactive)
      --stdin              Read prompt from stdin
      --prompt-file path   Read prompt from file
      --model string       Model to use (default: "llama-cpp")
      --temperature float  Inference temperature (default: 0.7)
      --max-tokens int     Maximum tokens to generate (default: 4096)
      --verbose            Enable verbose output
      --quiet              Suppress non-essential output
      --output file        Write results to file
      --no-stream          Disable streaming output
  -h, --help               Show this help message
  -v, --version            Show version information

Configuration:
      --config string      Path to configuration file
      --endpoint string    Inference endpoint URL
      --context-size int   Context size in tokens
      --timeout int        Initial token timeout in seconds
      --streaming int      Enable/disable streaming (-1 for default, 0=false, 1=true)
      --max-iterations int Maximum tool call iterations

Examples:
  coding-agent -p "Create a REST API in Go"
  coding-agent --prompt-file task.txt
  echo "Fix bug" | coding-agent --stdin
  coding-agent --verbose -p "Analyze the code in main.go"
  coding-agent --quiet -p "Summarize this" --output summary.txt

Exit Codes:
  0  - Success: Agent completed the task
  1  - Error: Agent failed or encountered an error
  2  - Usage error: Invalid arguments or flags
  3  - Authentication error: API key missing or invalid
  4  - Context limit: Task exceeded context limits
`
	fmt.Println(helpText)
}
