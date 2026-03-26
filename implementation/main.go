package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"coding-agent/config"
	"coding-agent/context"
	"coding-agent/inference"
	"coding-agent/stats"
	"coding-agent/tools"
	"coding-agent/tui"
)

// System prompt with all tools
const systemPrompt = `You are a helpful coding assistant. You have access to the following tools:

AVAILABLE TOOLS:
- bash: Execute a bash command
  Format: [tool:bash(command="command string")]
  Example: [tool:bash(command="ls -la")]
  
- read_file: Read the contents of a file
  Format: [tool:read_file(path="file path")]
  Example: [tool:read_file(path="/path/to/file.txt")]
  
- write_file: Write content to a file
  Format: [tool:write_file(path="file path", content="file content")]
  Example: [tool:write_file(path="/path/to/file.txt", content="Hello")]
  
- read_lines: Read a specific line range from a file
  Format: [tool:read_lines(path="file path", start=line_number, end=line_number)]
  Example: [tool:read_lines(path="/path/to/file.txt", start=1, end=10)]
  
- insert_lines: Insert lines at a specific line number
  Format: [tool:insert_lines(path="file path", line=line_number, lines="lines to insert")]
  Example: [tool:insert_lines(path="/path/to/file.txt", line=5, lines="new line")]
  
- replace_lines: Replace a line range with new lines
  Format: [tool:replace_lines(path="file path", start=line_number, end=line_number, lines="replacement lines")]
  Example: [tool:replace_lines(path="/path/to/file.txt", start=1, end=5, lines="new content")]

TOOL CALLING RULES:
- Use the exact format shown above for tool calls
- Tool calls must be enclosed in square brackets
- Tool name must match exactly (case-sensitive)
- Parameters must be properly quoted
- Multi-line content uses \n for line breaks

Instructions:
- Analyze the user's request and determine if tools are needed
- Use tools when they can help complete the task
- Always explain your reasoning before calling tools
- Provide clear explanations of tool results
- Continue the conversation after tool execution`

func main() {
	// Parse command-line flags
	configFile := flag.String("config", "", "Path to configuration file")
	endpoint := flag.String("endpoint", "", "Inference endpoint URL")
	contextSize := flag.Int("context-size", 0, "Context size in tokens")
	initialTokenTimeout := flag.Int("timeout", 0, "Initial token timeout in seconds")
	streaming := flag.Int("streaming", -1, "Enable/disable streaming (-1 for default, 0=false, 1=true)")
	maxIterations := flag.Int("max-iterations", 0, "Maximum tool call iterations")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
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

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Create statistics tracker
	statsTracker := stats.NewStats()

	// Create TUI
	tui := tui.NewTUI(statsTracker)
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
	ctx := context.NewContext(systemPrompt, cfg.ContextSize)

	// Create inference client
	client := inference.NewInferenceClient(
		cfg.InferenceEndpoint,
		cfg.APIKey,
		cfg.Model,
		cfg.ContextSize,
		cfg.InitialTokenTimeout,
		cfg.ConnectionTimeout,
		cfg.ReadTimeout,
		cfg.StreamingEnabled,
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
			return
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
				return
			}
			tui.DisplayError("Failed to read input: %v", err)
			continue
		}

		// Check for quit
		if input == "quit" || input == "exit" {
			fmt.Println("\nGoodbye!")
			return
		}

		// Add to history before processing
		tui.AddToHistory(input)

		// Process commands
		if !tui.ProcessCommand(input, ctx) {
			continue
		}

		// Add user message to context
		ctx.AddUserMessage(input)
		tui.AddOutputf("[User] %s", input)

		// Process the conversation
		if err := processConversation(ctx, client, toolRegistry, statsTracker, cfg.MaxIterations, tui); err != nil {
			tui.DisplayError("Conversation error: %v", err)
			// Remove the failed user message from context
			messages := ctx.GetMessages()
			if len(messages) > 0 {
				ctx = context.NewContext(systemPrompt, cfg.ContextSize)
				// Restore system prompt and any successful conversation
			}
		}
	}
}

// processConversation handles a single user request, including tool calls
func processConversation(ctx *context.Context, client *inference.InferenceClient,
	registry *tools.ToolRegistry, stats *stats.Stats, maxIterations int, t *tui.TUI) error {

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

		// Send to inference client
		var assistantContent string
		err := client.ChatCompletion(inference.ChatCompletionRequest{
			Context: ctx,
			OnToken: func(token string) {
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
		})

		if err != nil {
			return fmt.Errorf("inference failed: %w", err)
		}

		// Check for tool calls in assistant response
		toolCalls, err := tools.ExtractToolCalls(assistantContent)
		if err != nil {
			t.DisplayError("Failed to parse tool calls: %v", err)
			continue
		}

		if len(toolCalls) == 0 {
			// No tool calls, conversation for this turn is complete
			break
		}

		// Execute tool calls
		for _, tc := range toolCalls {
			// Display tool call with key parameters
			keyParam := tools.GetRelevantParameter(tc.Name, tc.Params)
			if keyParam != "" {
				t.AddOutputf("Calling tool: %s (%s)", tc.Name, keyParam)
			} else {
				t.AddOutputf("Calling tool: %s", tc.Name)
			}

			tool, ok := registry.Get(tc.Name)
			if !ok {
				errorMsg := fmt.Sprintf("unknown tool: %s", tc.Name)
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
				t.AddOutputf("  \033[31mTool '%s' failed: %s\033[0m", tc.Name, result.Error)
			} else {
				// Display success with formatted output
				t.AddOutputf("  \033[32mTool '%s' executed successfully\033[0m", tc.Name)
				// Truncate long output for display
				displayOutput := tools.TruncateOutput(result.Output, 500)
				if displayOutput != "" {
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
