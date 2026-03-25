package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"coding-agent-harness/pkg/config"
	agentcontext "coding-agent-harness/pkg/context"
	"coding-agent-harness/pkg/inference"
	"coding-agent-harness/pkg/stats"
	"coding-agent-harness/pkg/tools"
)

func main() {
	// Parse command line flags
	noStream := flag.Bool("no-stream", false, "Disable streaming mode")
	help := flag.Bool("help", false, "Show help message")
	flag.Parse()

	if *help {
		fmt.Println("Coding Agent Harness - A minimal coding agent")
		fmt.Println()
		fmt.Println("Usage: coding-agent-harness [options]")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -no-stream")
		fmt.Println("    Disable streaming mode")
		fmt.Println("  -help")
		fmt.Println("    Show help message")
		fmt.Println()
		fmt.Println("Environment Variables:")
		fmt.Println("  INFERENCE_URL    URL to inference server (default \"http://localhost:8080/v1\")")
		fmt.Println("  API_KEY          API key for inference server (default \"sk-no-key-required\")")
		fmt.Println("  CONTEXT_SIZE     Context size in tokens (default 128000)")
		fmt.Println("  INITIAL_TOKEN_TIMEOUT Timeout in seconds for initial token (default 7200)")
		os.Exit(0)
	}

	// Load configuration
	cfg := config.LoadConfig()

	// Initialize components
	statsTracker := stats.NewStats()
	ctxManager := agentcontext.NewContext(cfg.ContextSize)
	toolsManager := tools.NewTools()
	inferenceClient := inference.NewClient(cfg)

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		fmt.Println(statsTracker.String())
		os.Exit(0)
	}()

	// Print welcome message
	fmt.Println("🤖 Coding Agent Harness")
	fmt.Println("======================")
	fmt.Println("Type your message and press Enter to send.")
	fmt.Println("Type 'stats' to show statistics.")
	fmt.Println("Type 'exit' or 'quit' to exit.")
	fmt.Println()

	// Main input loop
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle commands
		switch input {
		case "exit", "quit":
			fmt.Println(statsTracker.String())
			return
		case "stats":
			fmt.Println(statsTracker.String())
			continue
		case "clear":
			ctxManager.Reset()
			fmt.Println("Context cleared.")
			continue
		}

		// Process user input
		if _, err := processUserInput(input, ctxManager, toolsManager, inferenceClient, statsTracker, *noStream); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}

		fmt.Println()
	}
}

// processUserInput processes user input and returns response
func processUserInput(userMsg string, ctxManager *agentcontext.Context, toolsManager *tools.Tools, inferenceClient *inference.Client, statsTracker *stats.Stats, noStream bool) (string, error) {
	// Add user message to context
	ctxManager.AddUserMessage(userMsg)

	// Check if context needs compression
	if ctxManager.NeedsCompression() {
		fmt.Println("Compressing context...")
		if err := ctxManager.Compress(inferenceClient); err != nil {
			return "", fmt.Errorf("context compression failed: %w", err)
		}
	}

	// Get messages for inference
	messages := ctxManager.GetMessages()

	// Check for tool calls in the user message
	toolCalls := tools.ExtractToolCalls(userMsg)
	if len(toolCalls) > 0 {
		return processToolCalls(toolCalls, ctxManager, toolsManager, statsTracker, inferenceClient)
	}

	// Send to inference engine
	var response string
	var err error

	if noStream {
		resp, err := inferenceClient.Chat(context.Background(), messages, ctxManager.GetContextSize())
		if err != nil {
			return "", err
		}
		response = resp.Choices[0].Message.Content
	} else {
		err = inferenceClient.ChatStream(context.Background(), messages, ctxManager.GetContextSize(), func(content string, done bool) {
			fmt.Print(content)
			if done {
				fmt.Println()
			}
		})
		if err != nil {
			return "", err
		}
	}

	// Add assistant response to context
	ctxManager.AddAssistantMessage(response)

	return response, nil
}

// processToolCalls handles tool call messages and adds results to context
func processToolCalls(toolCalls []tools.ToolCall, ctxManager *agentcontext.Context, toolsManager *tools.Tools, statsTracker *stats.Stats, inferenceClient *inference.Client) (string, error) {
	var results []string
	var formattedResults []string

	for _, tc := range toolCalls {
		result, err := toolsManager.CallTool(tc.Name, tc.Args)
		if err != nil {
			statsTracker.RecordFailedToolCall()
			formattedResults = append(formattedResults, tools.FormatToolResult(tc.Name, nil, err))
			results = append(results, fmt.Sprintf("Tool '%s' failed: %v", tc.Name, err))
		} else {
			statsTracker.RecordToolCall()
			formattedResults = append(formattedResults, tools.FormatToolResult(tc.Name, result, nil))
			results = append(results, fmt.Sprintf("Tool '%s' result: %v", tc.Name, result))
		}
	}

	// Add tool results to context as user message so AI can see and iterate
	toolResultMsg := "Tool results:\n" + strings.Join(formattedResults, "\n")
	ctxManager.AddUserMessage(toolResultMsg)

	// AI can now see the results and iterate further
	resp, err := inferenceClient.Chat(context.Background(), ctxManager.GetMessages(), ctxManager.GetContextSize())
	if err != nil {
		return "", err
	}

	// Add AI's response to context for next iteration
	ctxManager.AddAssistantMessage(resp.Choices[0].Message.Content)
	return resp.Choices[0].Message.Content, nil
}

// toolCall represents a tool call
type toolCall struct {
	Name string
	Args map[string]interface{}
}


