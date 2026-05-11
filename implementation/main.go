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
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/coding-agent/harness/agent"
	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
	"github.com/coding-agent/harness/tui"
)

// Version information injected at build time
// Terminal color codes for one-shot mode output.
const (
	ColorReset  = "\033[0m"
	ColorDim    = "\033[90m"
)

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

	// Set build version for agent debug logging
	version := gitHash
	if gitDirty == "dirty" {
		version = version + " [dirty]"
	}
	agent.SetBuildVersion(version)

	// Detect run mode
	if cfg.Prompt != "" || cfg.PromptFile != "" || cfg.UseStdin {
		// One-shot mode
		err = runOneShotMode(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(exitCodeForError(err))
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
	fmt.Println("      --debug              Enable debug logging (saves conversation to file)")
	fmt.Println("      --debug-log path     Path to debug log file (default: debug.log)")
	fmt.Println("      --debug-verbose             Print every request/response to stderr during inference")
	fmt.Println("      --debug-verbose-verbose     Print raw SSE body for every streaming chunk to stderr")
	fmt.Println("      --model string       Model to use (default: \"llama3\")")
	fmt.Println("      --temperature float  Inference temperature (omitted when not set, uses model default)")
	fmt.Println("      --max-tokens int     Maximum tokens to generate (default: 64000)")
	fmt.Println("      --context-size int   Context window size (default: 128000)")
	fmt.Println("      --max-iterations int Maximum iterations for loop protection (default: 1000)")
	fmt.Println("      --connection-timeout int  Connection timeout in seconds (default: 7200)")
	fmt.Println("      --read-timeout int        Read timeout in seconds (default: 7200)")
	fmt.Println("      --api-endpoint string  API endpoint URL (default: \"http://localhost:8080\")")
	fmt.Println("      --api-key string       API key for authentication")
	fmt.Println("      --verbose            Enable verbose output")
	fmt.Println("      --quiet              Suppress non-essential output")
	fmt.Println("      --read-only          Enable read-only mode (only read_file, read_lines, list_files, grep, git_log, git_show, git_diff available)")
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
	fmt.Println("  coding-agent --debug")
	fmt.Println("  coding-agent --debug --debug-log /tmp/agent-debug.log")
	fmt.Println("  coding-agent -p \"Task\" --debug")
	fmt.Println()
	fmt.Println("GitHub Copilot setup:")
	fmt.Println("  export CODING_AGENT_API_ENDPOINT=\"https://api.githubcopilot.com\"")
	fmt.Println("  export GITHUB_TOKEN=\"ghu_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\"")
	fmt.Println("  coding-agent --model gpt-4o")
	fmt.Println()
	fmt.Println("GitHub Models setup (official API):")
	fmt.Println("  export CODING_AGENT_API_ENDPOINT=\"https://models.github.ai\"")
	fmt.Println("  export CODING_AGENT_API_KEY=\"github_pat_xxxxxxxxxxxxxxxxxxxx\"")
	fmt.Println("  coding-agent --model openai/gpt-4.1")
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

	// Run agent with streaming to show LLM responses in real-time
	startTime := time.Now()
	transitionedFromReasoning := false
	result, err := ag.RunStream(ctx, prompt, func(chunk inference.StreamingChunk) {
		// Stream LLM responses to stdout with appropriate coloring
		switch chunk.ContentType {
		case inference.StreamingContentTypeReasoning:
			fmt.Printf("%s%s%s", ColorDim, chunk.Text, ColorReset)
		default:
			// Add separator when transitioning from reasoning to normal content
			if !transitionedFromReasoning && chunk.Text != "" {
				fmt.Println()
				fmt.Printf("%s--- Thinking Complete ---%s\n\n", ColorDim, ColorReset)
				transitionedFromReasoning = true
			}
			fmt.Print(chunk.Text)
		}
	})
	duration := time.Since(startTime)

	// Close debug logger at the end of one-shot mode
	if cfg.Debug {
		ag.CloseDebugLogger()
	}

	if err != nil {
		return fmt.Errorf("agent execution failed: %w", err)
	}

	// Output result
	return outputResult(result, cfg, duration)
}

// exitCodeForError returns the appropriate exit code for an error.
func exitCodeForError(err error) int {
	if err == nil {
		return agent.ExitSuccess
	}
	msg := err.Error()
	// Check for context size limit errors
	if strings.Contains(msg, "context size limit") ||
		strings.Contains(msg, "maximum context length") {
		return agent.ExitContextLimit
	}
	// Check for authentication errors
	if strings.Contains(msg, "authentication failed") ||
		strings.Contains(msg, "401") ||
		strings.Contains(msg, "403") ||
		strings.Contains(msg, "API authentication") {
		return agent.ExitAuthError
	}
	return agent.ExitError
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
	// Write to file if specified (do this first, before any early return)
	if cfg.OutputFile != "" {
		err := os.WriteFile(cfg.OutputFile, []byte(result.FinalOutput), 0644)
		if err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
	}

	if cfg.Quiet {
		// Minimal output - just the final answer
		if result.Reasoning != "" {
			fmt.Printf("%s[Reasoning]\n%s%s\n\n", ColorDim, result.Reasoning, ColorReset)
		}
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
		fmt.Println("\n=== Reasoning ===")
		if result.Reasoning != "" {
			fmt.Printf("%s%s%s\n\n", ColorDim, result.Reasoning, ColorReset)
		} else {
			fmt.Println("(No reasoning provided)")
		}
		fmt.Println("\n=== Final Output ===")
	}

	// Display reasoning first if present
	if result.Reasoning != "" {
		fmt.Printf("%s[Reasoning]\n%s%s\n\n", ColorDim, result.Reasoning, ColorReset)
	}

	fmt.Println(result.FinalOutput)

	// Summary statistics
	if cfg.Verbose {
		fmt.Printf("\n=== Summary ===\n")
		fmt.Printf("Steps executed: %d\n", len(result.Steps))
		fmt.Printf("Tokens used: %d\n", result.TokenUsage)
		if result.Reasoning != "" {
			fmt.Printf("Reasoning: %d chars\n", len(result.Reasoning))
		}
		fmt.Printf("Duration: %s\n", duration)
	}

	return nil
}

func runInteractiveMode(cfg *config.Config) error {
	// Display welcome screen
	displayVersion()
	fmt.Println("Type your request below. Use Ctrl+C to exit.")
	fmt.Println("Commands start with '/': /stats, /clear, /clear-history, /read-only")
	fmt.Println()

	// Initialize TUI
	tuiInstance := tui.NewTUI(cfg)

	// Initialize agent
	ag := agent.NewAgent(cfg)

	// Ensure debug logger is closed on exit
	if cfg.Debug {
		defer func() {
			ag.CloseDebugLogger()
		}()
	}

	// Set context size callback
	ag.SetContextSizeCallback(func(size, max int) {
		tuiInstance.SetContextSize(size, max)
	})

	// Set up cancellation with a root context that signal handler controls.
	// Each request gets a child context derived from this, so cancelling the
	// root cancels the currently running agent operation.
	// These are re-created after each cancellation so subsequent Ctrl+C works.
	rootCtx, rootCancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Track whether we're waiting at the prompt. The signal handler checks this
	// flag and skips cancellation when we're waiting for user input. This prevents
	// Ctrl+C pressed during prompt input from immediately cancelling the next request.
	var isPrompting atomic.Bool

	// Signal handler goroutine - listens for SIGINT/SIGTERM and cancels root context
	// when NOT waiting at the prompt. Restarted after context recreation to use new rootCancel.
	sigHandlerDone := make(chan struct{})
	startSigHandler := func() {
		go func() {
			defer close(sigHandlerDone)
			for range sigChan {
				// If we're currently waiting for user input at the prompt,
				// skip cancellation - the TUI handles Ctrl+C internally.
				if isPrompting.Load() {
					continue
				}
				rootCancel()
			}
		}()
	}
	startSigHandler()

	// Wait group to track running agent operations
	var wg sync.WaitGroup

	// Main event loop
	for {
		// Wait for any previous operation to complete
		wg.Wait()

		// Mark that we're waiting at the prompt - signal handler will skip
		// cancellation during this time, letting the TUI handle Ctrl+C internally
		isPrompting.Store(true)
		input, err := tuiInstance.Prompt()
		isPrompting.Store(false)

		if err != nil {
			if err.Error() == "cancelled" {
				// Handle cancellation - wait for old signal handler to finish
				// (so it doesn't interfere with the new context), drain any
				// pending SIGINT signals, and recreate a fresh root context.
				<-sigHandlerDone
				select {
				case <-sigChan:
				default:
				}
				rootCtx, rootCancel = context.WithCancel(context.Background())
				startSigHandler()
				continue
			}
			if err.Error() == "EOF" {
				// End of input (Ctrl+D), exit gracefully
				fmt.Println("\nGoodbye!")
				rootCancel()
				return nil
			}
			rootCancel()
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
			case "read-only":
				ag.GetToolExecutor().SetReadOnly(true)
				fmt.Println("\n[Read-only mode enabled: write operations disabled]")
				continue
			default:
				// Unknown command - show error
				fmt.Printf("Unknown command: /%s\n", command)
				fmt.Println("Available commands: /stats, /clear, /clear-history, /read-only")
				continue
			}
		}

		// Create a new child context for this request (derived from rootCtx)
		// Cancelling rootCtx (via signal handler) will cancel this context too
		ctx, cancel := context.WithCancel(rootCtx)

		// Show waiting indicator
		fmt.Println()
		fmt.Print("Thinking...")

		// Add to wait group
		wg.Add(1)

		// Run agent with the prompt
		go func(userInput string) {
			defer wg.Done()
			defer cancel() // Clean up child context when goroutine completes

			var result *agent.Result
			var err error

			if cfg.Streaming {
				// Use streaming mode - tokens appear as they arrive
				result, err = ag.RunStream(ctx, userInput, func(chunk inference.StreamingChunk) {
					// Stream each chunk immediately through TUI with appropriate coloring
					switch chunk.ContentType {
					case inference.StreamingContentTypeReasoning:
						tuiInstance.StreamReasoningChunk(chunk.Text)
					default:
						tuiInstance.StreamNormalChunk(chunk.Text)
					}
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
				if result.Reasoning != "" {
					tuiInstance.AddOutputf("\n[Assistant] %s[Reasoning]\n%s%s", ColorDim, result.Reasoning, ColorReset)
				} else {
					tuiInstance.AddOutputf("\n[Assistant] %s", result.FinalOutput)
				}
			}

			// Display summary if verbose
			if cfg.Verbose {
				tuiInstance.AddOutputf("\n--- Summary ---")
				tuiInstance.AddOutputf("Steps: %d, Tokens: %d", len(result.Steps), result.TokenUsage)
				if result.Reasoning != "" {
					tuiInstance.AddOutputf("Reasoning: %d chars", len(result.Reasoning))
				}
			}
		}(input)

		// Loop continues, but wg.Wait() at top will block until done
	}
}
