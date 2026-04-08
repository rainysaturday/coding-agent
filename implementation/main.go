//go:generate make build

// Minimal Coding Agent Harness
// A minimal coding agent harness written in Go with a basic TUI supporting an input prompt.

package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"sync"
	"time"

	"github.com/coding-agent/harness/agent"
	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/tui"
)

// Version information injected at build time
var (
	gitHash   string
	gitDirty  string
	buildTime string
)

func init() {
	if gitHash == "" {
		gitHash = "unknown"
	}
	if gitDirty == "" {
		gitDirty = "unknown"
	}
}

func main() {
	// Parse command-line arguments
	cfg, err := config.ParseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	// Handle version flag
	if cfg.ShowVersion {
		displayVersion()
		os.Exit(0)
	}

	// Handle help flag
	if cfg.ShowHelp {
		displayHelp()
		os.Exit(0)
	}

	// Detect run mode
	if cfg.Prompt != "" || cfg.PromptFile != "" || cfg.UseStdin {
		// One-shot mode
		err = runOneShotMode(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Interactive mode
	err = runInteractiveMode(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func displayVersion() {
	fmt.Println("============================================================")
	fmt.Println("  Minimal Coding Agent Harness")
	fmt.Println("============================================================")
	if gitDirty == "clean" {
		fmt.Printf("  Version: %s [clean]\n", gitHash)
	} else if gitDirty == "dirty" {
		fmt.Printf("  \033[33mVersion: %s [dirty]\033[0m\n", gitHash)
	} else {
		fmt.Printf("  Version: %s\n", gitHash)
	}
	if buildTime != "" {
		fmt.Printf("  Built: %s\n", buildTime)
	}
	fmt.Println()
}

func displayHelp() {
	fmt.Println("Minimal Coding Agent Harness")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  coding-agent [OPTIONS] [COMMAND]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -p, --prompt string      Prompt for one-shot mode (non-interactive)")
	fmt.Println("      --stdin              Read prompt from stdin")
	fmt.Println("      --prompt-file path   Read prompt from file")
	fmt.Println("      --config path        Load configuration from file")
	fmt.Println("      --model string       Model to use (default: \"llama3\")")
	fmt.Println("      --temperature float  Inference temperature (default: 0.7)")
	fmt.Println("      --max-tokens int     Maximum tokens to generate (default: 4096)")
	fmt.Println("      --context-size int   Context window size (default: 128000)")
	fmt.Println("      --max-iterations int Maximum iterations for loop protection (default: 1000)")
	fmt.Println("      --connection-timeout int  Connection timeout in seconds (default: 30)")
	fmt.Println("      --read-timeout int        Read timeout in seconds (default: 300)")
	fmt.Println("      --verbose            Enable verbose output")
	fmt.Println("      --quiet              Suppress non-essential output")
	fmt.Println("      --output file        Write results to file")
	fmt.Println("      --no-stream          Disable streaming output")
	fmt.Println("  -h, --help               Show this help message")
	fmt.Println("  -v, --version            Show version information")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  coding-agent -p \"Create a REST API in Go\"")
	fmt.Println("  coding-agent --prompt-file task.txt")
	fmt.Println("  coding-agent --config config.txt")
	fmt.Println("  echo \"Fix bug\" | coding-agent --stdin")
	fmt.Println("  coding-agent")
}

func runOneShotMode(cfg *config.Config) error {
	// Load prompt
	prompt, err := loadPrompt(cfg)
	if err != nil {
		return fmt.Errorf("failed to load prompt: %w", err)
	}

	if prompt == "" {
		return fmt.Errorf("no prompt provided")
	}

	// Initialize agent
	ag := agent.NewAgent(cfg)

	// Create context with cancellation support
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle SIGINT for cancellation
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Run agent
	startTime := time.Now()
	result, err := ag.Run(ctx, prompt)
	duration := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("agent execution failed: %w", err)
	}

	// Output result
	return outputResult(result, cfg, duration)
}

func loadPrompt(cfg *config.Config) (string, error) {
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
		var prompt string
		data, err := io.ReadAll(reader)
		if err != nil {
			return "", err
		}
		prompt = string(data)
		return prompt, nil
	}

	return "", fmt.Errorf("no prompt provided")
}

func outputResult(result *agent.Result, cfg *config.Config, duration time.Duration) error {
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
		fmt.Printf("\n=== Summary ===\n")
		fmt.Printf("Steps executed: %d\n", len(result.Steps))
		fmt.Printf("Tokens used: %d\n", result.TokenUsage)
		fmt.Printf("Duration: %s\n", duration)
	}

	// Write to file if specified
	if cfg.OutputFile != "" {
		err := os.WriteFile(cfg.OutputFile, []byte(result.FinalOutput), 0644)
		if err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
	}

	return nil
}

func runInteractiveMode(cfg *config.Config) error {
	// Display welcome screen
	displayVersion()
	fmt.Println("Type your request below. Use Ctrl+C to exit.")
	fmt.Println("Commands start with '/': /stats, /clear, /clear-history")
	fmt.Println()

	// Initialize TUI
	tuiInstance := tui.NewTUI(cfg)

	// Initialize agent
	ag := agent.NewAgent(cfg)

	// Set context size callback
	ag.SetContextSizeCallback(func(size, max int) {
		tuiInstance.SetContextSize(size, max)
	})

	// Set up cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Handle signals
	go func() {
		for {
			<-sigChan
			cancel()
			// Re-create context for next operation
			ctx, cancel = context.WithCancel(context.Background())
			tuiInstance.CancelOperation()
		}
	}()

	// Wait group to track running agent operations
	var wg sync.WaitGroup

	// Main event loop
	for {
		// Wait for any previous operation to complete
		wg.Wait()

		input, err := tuiInstance.Prompt()
		if err != nil {
			if err.Error() == "cancelled" {
				// Handle cancellation
				continue
			}
			if err.Error() == "EOF" {
				// End of input (Ctrl+D), exit gracefully
				fmt.Println("\nGoodbye!")
				return nil
			}
			return err
		}

		if input == "" {
			continue
		}

		// Handle commands (with / prefix)
		if strings.HasPrefix(input, "/") {
			command := strings.TrimPrefix(input, "/")

			switch command {
			case "stats":
				stats := ag.GetStats()
				tuiInstance.DisplayStats(stats)
				continue
			case "clear":
				tuiInstance.ClearOutput()
				continue
			case "clear-history":
				tuiInstance.ClearHistory()
				continue
			default:
				// Unknown command - show error
				fmt.Printf("Unknown command: /%s\n", command)
				fmt.Println("Available commands: /stats, /clear, /clear-history")
				continue
			}
		}

		// Reset cancellation for new request
		cancel()
		ctx, cancel = context.WithCancel(context.Background())

		// Show waiting indicator
		fmt.Println()
		fmt.Print("Thinking...")

		// Add to wait group
		wg.Add(1)

		// Run agent with the prompt
		go func(userInput string) {
			defer wg.Done()

			var result *agent.Result
			var err error

			if cfg.Streaming {
				// Use streaming mode - tokens appear as they arrive
				result, err = ag.RunStream(ctx, userInput, func(chunk string) {
					// Stream each chunk immediately through TUI
					tuiInstance.StreamChunk(chunk)
				})
			} else {
				// Non-streaming mode
				result, err = ag.Run(ctx, userInput)
			}

			// Check for cancellation
			select {
			case <-ctx.Done():
				fmt.Println("\n[Cancelled]")
				return
			default:
			}

			if err != nil {
				fmt.Printf("\nError: %v\n", err)
				return
			}

			// End streaming session - ensures proper newline
			if cfg.Streaming {
				tuiInstance.StreamEnd()
			}

			// Display final output if not already streamed
			if !cfg.Streaming && result.FinalOutput != "" {
				tuiInstance.AddOutputf("\n[Assistant] %s", result.FinalOutput)
			}

			// Display summary if verbose
			if cfg.Verbose {
				tuiInstance.AddOutputf("\n--- Summary ---")
				tuiInstance.AddOutputf("Steps: %d, Tokens: %d", len(result.Steps), result.TokenUsage)
			}
		}(input)

		// Loop continues, but wg.Wait() at top will block until done
	}
}
