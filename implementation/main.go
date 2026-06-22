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
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/coding-agent/harness/agent"
	"github.com/coding-agent/harness/colors"
	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
	"github.com/coding-agent/harness/tui"
)

// Version information injected at build time
// Terminal color codes for one-shot mode output.

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
		fmt.Fprintf(os.Stderr, "%sError: %v%s\n", colors.GetColor("red"), err, colors.GetColor("reset"))
		os.Exit(2)
	}

	// Apply theme (CLI flag overrides env var; default is "dark")
	if cfg.Theme != "" {
		if err := colors.ApplyTheme(cfg.Theme); err != nil {
			fmt.Fprintf(os.Stderr, "%sError: %v%s\n", colors.GetColor("red"), err, colors.GetColor("reset"))
			os.Exit(2)
		}
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
	if cfg.Prompt != "" || cfg.PromptFile != "" || cfg.UseStdin || cfg.Goal != "" {
		// One-shot mode
		err = runOneShotMode(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%sError: %v%s\n", colors.GetColor("red"), err, colors.GetColor("reset"))
			os.Exit(exitCodeForError(err))
		}
		os.Exit(0)
	}

	// Interactive mode
	err = runInteractiveMode(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError: %v%s\n", colors.GetColor("red"), err, colors.GetColor("reset"))
		os.Exit(1)
	}
}

func displayVersion() {
	fmt.Printf("%s============================================================%s\n", colors.GetColor("blue"), colors.GetColor("reset"))
	fmt.Printf("%s  Minimal Coding Agent Harness%s\n", colors.GetColor("blue"), colors.GetColor("reset"))
	fmt.Printf("%s============================================================%s\n", colors.GetColor("blue"), colors.GetColor("reset"))
	if gitDirty == "clean" {
		fmt.Printf("  %sVersion:%s %s [clean]\n", colors.GetColor("cyan"), colors.GetColor("reset"), gitHash)
	} else if gitDirty == "dirty" {
		fmt.Printf("  %sVersion:%s %s [dirty]%s\n", colors.GetColor("cyan"), colors.GetColor("reset"), gitHash, colors.GetColor("reset"))
	} else {
		fmt.Printf("  %sVersion:%s %s\n", colors.GetColor("cyan"), colors.GetColor("reset"), gitHash)
	}
	if buildTime != "" {
		fmt.Printf("  %sBuilt:%s %s\n", colors.GetColor("cyan"), colors.GetColor("reset"), buildTime)
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
	fmt.Println("      --goal string        Set a goal for one-shot mode (non-interactive, activates goal mode)")
	fmt.Println("      --prompt-file path   Read prompt from file")
	fmt.Println("      --load path           Load conversation context from JSON file")
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
	fmt.Println("      --connection-timeout int  Connection timeout in seconds (default: 24 hours)")
	fmt.Println("      --read-timeout int        Read timeout in seconds (default: 24 hours)")
	fmt.Println("      --api-endpoint string  API endpoint URL (default: \"http://localhost:8080\")")
	fmt.Println("      --api-key string       API key for authentication")
	fmt.Println("      --verbose            Enable verbose output")
	fmt.Println("      --quiet              Suppress non-essential output")
	fmt.Println("      --read-only          Enable read-only mode (only read_file, read_lines, list_files, grep, git_log, git_show, git_diff available)")
	fmt.Println("      --experimental       Enable experimental features (e.g., subagent tool)")
	fmt.Println("      --persona string     Set a persona for the agent (e.g., \"Expert Go developer\", \"Security code reviewer\")")
	fmt.Println("      --theme string       Color theme for the TUI (dark, light, solarized, gruvbox, darkula). Overrides CODING_AGENT_THEME env var (default: \"dark\")")
	fmt.Println("      --summary-only       Only output the final summary (used by subagents)")
	fmt.Println("      --output file        Write results to file")
	fmt.Println("      --no-dump-on-exit     Disable automatic context dump on exit")
	fmt.Println("      --no-stream          Disable streaming output")
	fmt.Println("  -h, --help               Show this help message")
	fmt.Println("  -v, --version            Show version information")
	fmt.Println()
	fmt.Println("Interactive Commands:")
	fmt.Println("  /stats       - Display runtime statistics")
	fmt.Println("  /clear       - Clear the output display")
	fmt.Println("  /clear-history - Clear input history")
	fmt.Println("  /read-only   - Enable read-only mode")
	fmt.Println("  /dump         - Dump current context to JSON file")
	fmt.Println("  /compress    - Manually trigger context compression")
	fmt.Println("  /goal <prompt>    - Set a goal to guide the agent")
	fmt.Println("  /goal-off         - Deactivate goal mode")
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
	fmt.Println()
	fmt.Println("Theme customization:")
	fmt.Println("  coding-agent --theme gruvbox")
	fmt.Println("  export CODING_AGENT_THEME=solarized")
	fmt.Println("  coding-agent --theme light")
}

func runOneShotMode(cfg *config.Config) error {
	var prompt string
	var err error

	// If --goal is specified, use it as the prompt and enable goal mode
	if cfg.Goal != "" {
		prompt = cfg.Goal
	} else {
		// Load prompt from other sources
		prompt, err = loadPrompt(cfg)
		if err != nil {
			return fmt.Errorf("failed to load prompt: %w", err)
		}

		if prompt == "" {
			return fmt.Errorf("no prompt provided")
		}
	}

	// Initialize agent
	ag := agent.NewAgent(cfg)

	if cfg.ContextFile != "" {
		if err := ag.LoadContext(cfg.ContextFile); err != nil {
			return fmt.Errorf("failed to load context: %w", err)
		}
	}

	// If --goal is specified, set the goal on the agent
	if cfg.Goal != "" {
		ag.SetGoal(cfg.Goal)
	}


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
	lastContentWasToolCall := false
	result, err := ag.RunStream(ctx, prompt, func(chunk inference.StreamingChunk) {
		// Stream LLM responses to stdout with appropriate coloring
		switch chunk.ContentType {
		case inference.StreamingContentTypeReasoning:
			fmt.Printf("%s%s%s", colors.GetColor("dim"), chunk.Text, colors.GetColor("reset"))
		case inference.StreamingContentTypeGoal:
			// Goal messages are displayed in magenta to stand out
			fmt.Printf("%s%s%s", colors.GetColor("magenta"), chunk.Text, colors.GetColor("reset"))
		default:
			// Add separator when transitioning from reasoning to normal content
			if !transitionedFromReasoning && chunk.Text != "" {
				fmt.Println()
				fmt.Printf("%s--- Thinking Complete ---%s\n\n", colors.GetColor("dim"), colors.GetColor("reset"))
				transitionedFromReasoning = true
			}
			// Handle tool call parameter updates in-place using ANSI cursor positioning,
			// matching the behavior of interactive mode.
			if strings.HasPrefix(chunk.Text, "[Tool Call] ") {
				fmt.Print("\033[2K\r" + chunk.Text)
				lastContentWasToolCall = true
			} else {
				if lastContentWasToolCall {
					fmt.Print("\n")
				}
				fmt.Print(chunk.Text)
				lastContentWasToolCall = false
			}
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
		// Handle "-" as stdin
		if cfg.PromptFile == "-" {
			reader := bufio.NewReader(os.Stdin)
			var prompt string
			data, err := io.ReadAll(reader)
			if err != nil {
				return "", err
			}
			prompt = string(data)
			return prompt, nil
		}
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
			fmt.Printf("%s[Reasoning]\n%s%s\n\n", colors.GetColor("dim"), result.Reasoning, colors.GetColor("reset"))
		}
		fmt.Printf("%s%s%s\n", colors.GetColor("cyan"), result.FinalOutput, colors.GetColor("reset"))
		return nil
	}

	// Summary-only mode - only output the final answer, minimal formatting
	if cfg.SummaryOnly {
		fmt.Printf("%s%s%s\n", colors.GetColor("cyan"), result.FinalOutput, colors.GetColor("reset"))
		return nil
	}

	// Verbose output with tool calls
	if cfg.Verbose {
		fmt.Printf("%s=== Agent Execution Log ===%s\n", colors.GetColor("blue"), colors.GetColor("reset"))
		for _, step := range result.Steps {
			fmt.Printf("\n%s[Step]%s %s\n", colors.GetColor("blue"), colors.GetColor("reset"), step.Action)
			if step.ToolCall != nil {
				fmt.Printf("  %sTool:%s %s\n", colors.GetColor("cyan"), colors.GetColor("reset"), step.ToolCall.Name)
				fmt.Printf("  %sParameters:%s %s\n", colors.GetColor("cyan"), colors.GetColor("reset"), step.ToolCall.Parameters)
			}
			if step.ToolResult != nil {
				fmt.Printf("  %sResult:%s %s\n", colors.GetColor("green"), colors.GetColor("reset"), step.ToolResult.Output)
			}
		}
		fmt.Printf("\n%s=== Reasoning ===%s\n", colors.GetColor("blue"), colors.GetColor("reset"))
		if result.Reasoning != "" {
			fmt.Printf("%s%s%s\n\n", colors.GetColor("dim"), result.Reasoning, colors.GetColor("reset"))
		} else {
			fmt.Printf("%s(No reasoning provided)%s\n", colors.GetColor("dim"), colors.GetColor("reset"))
		}
		fmt.Printf("\n%s=== Final Output ===%s\n", colors.GetColor("blue"), colors.GetColor("reset"))
	}

	// Display reasoning first if present
	if result.Reasoning != "" {
		fmt.Printf("%s[Reasoning]\n%s%s\n\n", colors.GetColor("dim"), result.Reasoning, colors.GetColor("reset"))
	}

	fmt.Printf("%s%s%s\n", colors.GetColor("cyan"), result.FinalOutput, colors.GetColor("reset"))

	// Summary statistics
	if cfg.Verbose {
		fmt.Printf("\n%s=== Summary ===%s\n", colors.GetColor("blue"), colors.GetColor("reset"))
		fmt.Printf("  %sSteps executed:%s %d\n", colors.GetColor("cyan"), colors.GetColor("reset"), len(result.Steps))
		fmt.Printf("  %sTokens used:%s %d\n", colors.GetColor("cyan"), colors.GetColor("reset"), result.TokenUsage)
		if result.Reasoning != "" {
			fmt.Printf("  %sReasoning:%s %d chars\n", colors.GetColor("cyan"), colors.GetColor("reset"), len(result.Reasoning))
		}
		fmt.Printf("  %sDuration:%s %s\n", colors.GetColor("cyan"), colors.GetColor("reset"), duration)
	}

	return nil
}

func runInteractiveMode(cfg *config.Config) error {
	// Display welcome screen
	displayVersion()
	fmt.Printf("%sType your request below. Use Ctrl+C to exit.%s\n", colors.GetColor("dim"), colors.GetColor("reset"))
	fmt.Printf("%sCommands start with '/': /stats, /clear, /clear-history, /read-only, /compress, /goal, /goal-off%s\n", colors.GetColor("dim"), colors.GetColor("reset"))
	fmt.Println()

	// Initialize TUI
	tuiInstance := tui.NewTUI(cfg)

	// Initialize agent
	ag := agent.NewAgent(cfg)
	if cfg.ContextFile != "" {
		if err := ag.LoadContext(cfg.ContextFile); err != nil {
			return fmt.Errorf("failed to load context: %w", err)
		}
	}


	// Detect terminal width and set max display width for tool call arguments
	if fd := int(os.Stdin.Fd()); term.IsTerminal(fd) {
		width, _, err := term.GetSize(fd)
		if err == nil && width > 0 {
			// Reserve space for UI elements and use remaining width for arguments
			// Typically we want to leave ~30 chars for other UI elements (tool name, brackets, etc.)
			ag.SetMaxDisplayWidth(width - 30)
		}
	}

	// Ensure debug logger is closed on exit
	// Automatic context dump on exit
	defer func() {
		if !cfg.NoDumpOnExit {
			path, err := ag.DumpContext()
			if err != nil {
				fmt.Printf("%s[Automatic dump failed: %v]%s\n", colors.GetColor("red"), err, colors.GetColor("reset"))
			} else {
				fmt.Printf("%s[Context automatically dumped to: %s]%s\n", colors.GetColor("green"), path, colors.GetColor("reset"))
			}
		}
	}()

	if cfg.Debug {
		defer func() {
			ag.CloseDebugLogger()
		}()
	}

	// Set context size callback
	ag.SetContextSizeCallback(func(size, max int) {
		tuiInstance.SetContextSize(size, max)
	})

	// signalState holds the current signal handling state, protected by a mutex.
	// This prevents race conditions during signal handler recreation.
	type signalState struct {
		rootCtx     context.Context
		rootCancel  context.CancelFunc
		sigChan     chan os.Signal
		handlerDone chan struct{} // Closed when the signal handler goroutine exits
	}

	var sigMu sync.Mutex
	st := &signalState{
		handlerDone: make(chan struct{}),
	}

	// initSignalHandler sets up the root context and signal handler goroutine.
	// Must be called with sigMu held.
	initSignalHandler := func(s *signalState) {
		s.rootCtx, s.rootCancel = context.WithCancel(context.Background())
		s.sigChan = make(chan os.Signal, 1)
		signal.Notify(s.sigChan, syscall.SIGINT, syscall.SIGTERM)
		s.handlerDone = make(chan struct{})
	}
	initSignalHandler(st)

	// recreateSignalState shuts down the old signal handler and creates a fresh
	// context and handler. Must be called when rootCtx is cancelled. Thread-safe.
	recreateSignalState := func() {
		sigMu.Lock()
		defer sigMu.Unlock()

		// Stop signal delivery to the old channel and close it.
		signal.Stop(st.sigChan)
		close(st.sigChan)

		// Wait briefly for the old handler to exit
		select {
		case <-st.handlerDone:
		default:
			time.Sleep(10 * time.Millisecond)
		}

		// Create fresh state
		initSignalHandler(st)
	}

	// promptState tracks whether we're at the input prompt.
	// When true, signals are ignored (TUI handles Ctrl+C in raw mode).
	// When false, signals trigger cancellation.
	var promptMu sync.Mutex
	atPrompt := false

	// cancelSignal is sent when Ctrl+C should cancel the current operation.
	cancelSignal := make(chan struct{}, 1)

	// Start the prompt-aware signal handler goroutine.
	// This goroutine runs for the lifetime of interactive mode.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for range sigChan {
			promptMu.Lock()
			isPrompting := atPrompt
			promptMu.Unlock()

			if isPrompting {
				// Signal during prompt input - let TUI handle it via its raw mode
				// The TUI will detect the Ctrl+C byte directly
				continue
			}

			// Signal during agent execution - cancel the operation
			select {
			case cancelSignal <- struct{}{}:
			default:
				// Already signalling cancellation
			}
		}
	}()

	// Wait group to track running agent operations
	var wg sync.WaitGroup

	// Main event loop
	for {
		// Wait for any previous operation to complete
		wg.Wait()

		// Ensure we have a valid root context (recreate if cancelled)
		sigMu.Lock()
		select {
		case <-st.rootCtx.Done():
			// Root context was cancelled, recreate everything
			sigMu.Unlock()
			recreateSignalState()
		default:
			sigMu.Unlock()
		}

		// Mark that we're at the prompt - signal handler will skip cancellation
		promptMu.Lock()
		atPrompt = true
		promptMu.Unlock()

		input, err := tuiInstance.Prompt()

		// Mark that we're no longer at the prompt
		promptMu.Lock()
		atPrompt = false
		promptMu.Unlock()

		// Handle prompt errors
		if err != nil {
			if err.Error() == "cancelled" || err.Error() == "EOF" {
				if err.Error() == "EOF" {
					fmt.Printf("%sGoodbye!%s\n", colors.GetColor("dim"), colors.GetColor("reset"))
				}
				signal.Stop(sigChan)
				close(sigChan)
				return nil
			}
			signal.Stop(sigChan)
			close(sigChan)
			return err
		}

		if input == "" {
			continue
		}

		// Handle commands (with / prefix)
		if strings.HasPrefix(input, "/") {
			// Extract the full command string after "/"
			fullCommand := strings.TrimPrefix(input, "/")
			// Split to get the command name and arguments
			parts := strings.SplitN(fullCommand, " ", 2)
			command := parts[0]

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
				fmt.Printf("%s[Read-only mode enabled: write operations disabled]%s\n", colors.GetColor("yellow"), colors.GetColor("reset"))
				continue
			case "compress":
				fmt.Print("\n[Compressing context...]")
				timeout := time.Duration(cfg.ReadTimeout) * time.Second
				if timeout == 0 {
					timeout = 24 * 60 * 60 * time.Second // Default to 24 hours if not set
				}
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				err := ag.CompressContext(ctx)
				cancel()
				if err != nil {
					fmt.Printf("%s[Compression failed: %v]%s\n", colors.GetColor("red"), err, colors.GetColor("reset"))
				} else {
					fmt.Printf("%s[Context compressed successfully]%s\n", colors.GetColor("green"), colors.GetColor("reset"))
				}
				continue
			case "goal":
				// Extract the goal prompt (everything after "/goal ")
				var goalPrompt string
				if len(parts) > 1 {
					goalPrompt = strings.TrimSpace(parts[1])
				}
				if goalPrompt == "" {
					fmt.Printf("%sUsage: /goal <your goal here>%s\n", colors.GetColor("yellow"), colors.GetColor("reset"))
					continue
				}
				ag.SetGoal(goalPrompt)
				fmt.Printf("%s[Goal mode activated: %q]%s\n", colors.GetColor("magenta"), goalPrompt, colors.GetColor("reset"))
				// Set input to goalPrompt so the agent starts working immediately
				// with the goal as the first user prompt
				input = goalPrompt
			case "goal-off":
				ag.ClearGoal()
				fmt.Printf("%s[Goal mode deactivated]%s\n", colors.GetColor("dim"), colors.GetColor("reset"))
				continue
			case "dump":
				path, err := ag.DumpContext()
				if err != nil {
					fmt.Printf("%s[Dump failed: %v]%s\n", colors.GetColor("red"), err, colors.GetColor("reset"))
				} else {
					fmt.Printf("%s[Context dumped to: %s]%s\n", colors.GetColor("green"), path, colors.GetColor("reset"))
				}
				continue

			default:
				// Unknown command - show error
				fmt.Printf("%sUnknown command: /%s%s\n", colors.GetColor("red"), command, colors.GetColor("reset"))
				fmt.Printf("%sAvailable commands: /stats, /clear, /clear-history, /read-only, /compress, /dump, /goal, /goal-off%s\n", colors.GetColor("dim"), colors.GetColor("reset"))
				continue
			}
		}

		// Get current root context for this request
		sigMu.Lock()
		currentRootCtx := st.rootCtx
		sigMu.Unlock()

		// Create a cancellable context for this request derived from rootCtx.
		// The context can be cancelled by:
		// 1. Ctrl+C during agent execution (via cancelSignal channel)
		// 2. Root context cancellation
		ctx, cancel := context.WithCancel(currentRootCtx)

		// Start a goroutine to forward cancelSignal to this context's cancel
		go func() {
			select {
			case <-cancelSignal:
				cancel()
			case <-ctx.Done():
				// Context already cancelled (e.g., by root or completion)
			}
		}()


		// Show waiting indicator
		fmt.Println()
		fmt.Printf("%sThinking...%s", colors.GetColor("dim"), colors.GetColor("reset"))

		// Add to wait group
		wg.Add(1)

		// Run agent with the prompt
		go func(userInput string) {
			defer wg.Done()
			defer cancel()

			var result *agent.Result
			var err error

			if cfg.Streaming {
				// Use streaming mode - tokens appear as they arrive
				result, err = ag.RunStream(ctx, userInput, func(chunk inference.StreamingChunk) {
					// Stream each chunk immediately through TUI with appropriate coloring
					switch chunk.ContentType {
					case inference.StreamingContentTypeReasoning:
						tuiInstance.StreamReasoningChunk(chunk.Text)
					case inference.StreamingContentTypeGoal:
						tuiInstance.StreamGoalChunk(chunk.Text)
					default:
						tuiInstance.StreamNormalChunk(chunk.Text)
					}
				})
				// Ensure streaming session is ended even on error
				defer tuiInstance.StreamEnd()
			} else {
				// Non-streaming mode
				result, err = ag.Run(ctx, userInput)
			}

			// Check if we were cancelled
			if err == context.Canceled || err == context.DeadlineExceeded {
				fmt.Printf("%s[Cancelled]%s\n", colors.GetColor("yellow"), colors.GetColor("reset"))
				return
			}

			if err != nil {
				fmt.Printf("%sError: %v%s\n", colors.GetColor("red"), err, colors.GetColor("reset"))
				return
			}

			// Display final output if not already streamed
			if !cfg.Streaming && result.FinalOutput != "" {
				if result.Reasoning != "" {
					tuiInstance.AddOutputf("\n%s[Assistant]%s %s[Reasoning]\n%s%s", colors.GetColor("blue"), colors.GetColor("reset"), colors.GetColor("dim"), result.Reasoning, colors.GetColor("reset"))
				} else {
					tuiInstance.AddOutputf("\n%s[Assistant]%s %s%s%s", colors.GetColor("blue"), colors.GetColor("reset"), colors.GetColor("cyan"), result.FinalOutput, colors.GetColor("reset"))
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
