// Package agent implements the main agent logic.
package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"encoding/json"
	"github.com/coding-agent/harness/colors"
	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/debug"
	"github.com/coding-agent/harness/inference"
	"github.com/coding-agent/harness/tools"
	"path/filepath"
)

// StreamCallback is a function type for handling streaming chunks.
// Using inference.StreamingCallbackWithType for typed streaming support.
type StreamCallback = inference.StreamingCallbackWithType

// ContextSizeCallback is a function called when context size changes.
type ContextSizeCallback func(size, max int)

// Agent represents the coding agent.
type Agent struct {
	config              *config.Config
	inference           *inference.InferenceClient
	toolExecutor        *tools.ToolExecutor
	context             []*inference.Message
	iterationHistory    []Iteration // History of full contexts and stats after each turn
	systemPrompt        string
	stats               *Stats
	maxIterations       int
	streamCallback      StreamCallback
	contextSizeCallback ContextSizeCallback
	maxContextSize      int
	compressionCount    int
	goal                string    // Current goal prompt (empty string means goal mode is off)
	goalActive          bool      // Whether goal mode is currently active
	goalStartTime       time.Time // When the current goal was started (for timing)
	debugLogger         *debug.DebugLogger
	mu                  sync.Mutex
	// lastTotalTokens is the exact total_tokens from the last API response.
	// This is the authoritative count of everything the API processed:
	// system prompt + messages + tools + completion.
	// We use this as the baseline and only estimate deltas for messages
	// added after the response (e.g., tool results).
	lastTotalTokens int
	// toolResultMsgsSinceLastAPI is the set of indices in context that
	// are tool-result messages added AFTER the last API call.
	// Messages before this index were already included in lastTotalTokens.
	toolResultMsgsSinceLastAPI map[int]bool
}

// Stats represents agent statistics.
type Stats struct {
	InputTokens      int       `json:"input_tokens"`
	OutputTokens     int       `json:"output_tokens"`
	ToolCalls        int       `json:"tool_calls"`
	FailedToolCalls  int       `json:"failed_tool_calls"`
	Iterations       int       `json:"iterations"`
	CompressionCount int       `json:"compression_count"`
	StartTime        time.Time `json:"start_time"`
	TokensPerSecond  float64   `json:"tokens_per_second"`
}

// Step represents an execution step.
type Step struct {
	Action     string
	ToolCall   *tools.ToolCall
	ToolResult *tools.ToolResult
	StreamMsg  string // Status message streamed to user
}

// Result represents the final result of an agent run.
type Result struct {
	FinalOutput string
	Reasoning   string
	Steps       []Step
	TokenUsage  int
}

// ContextDump represents a serializable conversation context.
type ContextDump struct {
	Version    int         `json:"version"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
	Session    SessionInfo `json:"session"`
	Iterations []Iteration `json:"iterations"`
}

// StatsInfo holds runtime statistics.
type StatsInfo struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	ToolCalls       int `json:"tool_calls"`
	FailedToolCalls int `json:"failed_tool_calls"`
}

// Iteration represents a full snapshot of the context at a point in time.
type Iteration struct {
	Index    int                  `json:"index"`
	Messages []*inference.Message `json:"messages"` // System prompt + conversation
	Stats    StatsInfo            `json:"stats"`
}

// SessionInfo holds session metadata.
type SessionInfo struct {
	StartTime        time.Time `json:"start_time"`
	Iterations       int       `json:"iterations"`
	CompressionCount int       `json:"compression_count"`
	Stats            StatsInfo `json:"stats"`
}

// NewAgent creates a new agent.
func NewAgent(cfg *config.Config) *Agent {
	// Create debug logger if enabled
	var debugLogger *debug.DebugLogger
	if cfg.Debug {
		var err error
		debugLogger, err = debug.NewDebugLogger(cfg.DebugLog, getBuildVersion())
		if err != nil {
			// Log error but continue without debug logging
			fmt.Fprintf(os.Stderr, "Warning: failed to initialize debug logger: %v\n", err)
			debugLogger = nil
		} else {
			// Warn user about debug logging
			fmt.Fprintf(os.Stderr, "[WARNING] Debug logging enabled. All conversation data will be saved to:\n")
			fmt.Fprintf(os.Stderr, "  %s\n", cfg.DebugLog)
			fmt.Fprintf(os.Stderr, "This may include sensitive information. Ensure the log file is protected.\n\n")
		}
	}

	agent := &Agent{
		config:                     cfg,
		inference:                  inference.NewInferenceClient(cfg),
		toolExecutor:               tools.NewToolExecutor(),
		context:                    make([]*inference.Message, 0),
		toolResultMsgsSinceLastAPI: make(map[int]bool),
		stats: &Stats{
			StartTime: time.Now(),
		},
		maxIterations:  cfg.MaxIterations,
		maxContextSize: cfg.ContextSize,
		debugLogger:    debugLogger,
	}

	// Build system prompt, tools, and persona based on configuration
	agent.systemPrompt = buildSystemPrompt(cfg.ReadOnly, cfg.Persona, cfg.SummaryOnly)

	// Set read-only mode on tool executor
	agent.toolExecutor.SetReadOnly(cfg.ReadOnly)

	// Log system prompt if debug is enabled
	if agent.debugLogger != nil {
		agent.debugLogger.LogSystemPrompt(agent.systemPrompt, inference.EstimateTokens(agent.systemPrompt))
	}

	// Register tools with inference client
	agent.inference.SetTools(buildTools(cfg.ReadOnly, cfg.Experimental))

	// Display read-only mode warning
	if cfg.ReadOnly {
		fmt.Fprintln(os.Stderr, "============================================================")
		fmt.Fprintln(os.Stderr, "  READ-ONLY MODE ACTIVE")
		fmt.Fprintln(os.Stderr, "============================================================")
		fmt.Fprintln(os.Stderr, "  Only read_file, read_lines, list_files, grep, git_log, git_show, git_diff, and view_image tools are available.")
		fmt.Fprintln(os.Stderr, "  All write operations are disabled.")
		fmt.Fprintln(os.Stderr, "============================================================")
		fmt.Fprintln(os.Stderr)
	}

	return agent
}

// SetStreamCallback sets the callback function for streaming chunks.
func (a *Agent) SetStreamCallback(callback StreamCallback) {
	a.mu.Lock()
	a.streamCallback = callback
	a.mu.Unlock()
}

// recordIteration takes a snapshot of the current context (system prompt + messages)
// and adds it to the iteration history.

// SetContextSizeCallback sets the callback function for context size updates.
func (a *Agent) SetContextSizeCallback(callback ContextSizeCallback) {
	a.mu.Lock()
	a.contextSizeCallback = callback
	a.mu.Unlock()
}

// SetAPIEndpoint sets the API endpoint.
func (a *Agent) SetAPIEndpoint(endpoint string) {
	a.inference.SetEndpoint(endpoint)
}

// SetAPIKey sets the API key.
func (a *Agent) SetAPIKey(key string) {
	a.inference.SetAPIKey(key)
}

// SetMaxDisplayWidth sets the maximum display width for tool call arguments.
// This is used to truncate long parameter values to prevent terminal line wrapping.
func (a *Agent) SetMaxDisplayWidth(width int) {
	a.inference.SetMaxDisplayWidth(width)
}

// GetSystemPrompt returns the current system prompt.
func (a *Agent) GetSystemPrompt() string {
	return a.systemPrompt
}

// GetTools returns the registered tools.
func (a *Agent) GetTools() []inference.ToolDefinition {
	return a.inference.GetTools()
}

// GetToolExecutor returns the tool executor.
func (a *Agent) GetToolExecutor() *tools.ToolExecutor {
	return a.toolExecutor
}

// SetGoal sets a goal for the agent. When goal mode is active, the agent will
// check if the goal has been achieved after each inference response that has
// no tool calls (a natural end).
func (a *Agent) SetGoal(goal string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if goal == "" {
		a.goalActive = false
		a.goal = ""
		a.goalStartTime = time.Time{}
		return
	}
	a.goal = goal
	a.goalActive = true
	a.goalStartTime = time.Now()
}

// ClearGoal deactivates goal mode.
func (a *Agent) ClearGoal() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.goalActive = false
	a.goal = ""
	a.goalStartTime = time.Time{}
}

// IsGoalActive returns whether goal mode is currently active.
func (a *Agent) IsGoalActive() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.goalActive
}

// GetGoal returns the current goal prompt (empty string if goal mode is off).
func (a *Agent) GetGoal() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.goal
}

// GetGoalStartTime returns when the current goal was started.
func (a *Agent) GetGoalStartTime() time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.goalStartTime
}

// formatGoalDuration formats a duration for goal achievement reporting.
// It shows the raw seconds as well as hours, minutes, and seconds.
func formatGoalDuration(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%ds (%dh %dm %ds)", totalSeconds, hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%ds (%dm %ds)", totalSeconds, minutes, seconds)
	}
	return fmt.Sprintf("%ds", totalSeconds)
}

// shouldCheckGoal returns true if goal mode is active and a goal is set.
func (a *Agent) shouldCheckGoal() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.goalActive && a.goal != ""
}

// injectGoalCheck injects a goal check user message into the context.
// This is treated as an automatically injected user prompt that will go through
// the full agentic loop on the next iteration. The LLM can run tools to verify
// its work before responding with "goal achieved" or continuing.
// It also streams a goal check notification to the TUI.
func (a *Agent) injectGoalCheck() {
	goalCheckPrompt := fmt.Sprintf(`Please review the current state of your work. Have you achieved the following goal?

Goal: %s

If you have achieved the goal, respond with "goal achieved".
If you have not achieved the goal, explain what remains to be done and continue working.`, a.goal)

	// Create and add a user message for the goal check, capture goal and callback while holding lock
	a.mu.Lock()
	a.context = append(a.context, &inference.Message{
		Role:    "user",
		Content: goalCheckPrompt,
	})
	goal := a.goal
	streamCallback := a.streamCallback
	a.mu.Unlock()

	// Stream goal check notification to TUI with magenta color
	goalCheckMsg := fmt.Sprintf("\n%s[Goal Check] Checking if goal is achieved: %q%s\n", colors.GetColor("magenta"), goal, colors.GetColor("reset"))
	if streamCallback != nil {
		streamCallback(inference.StreamingChunk{
			Text:        goalCheckMsg,
			ContentType: inference.StreamingContentTypeGoal,
		})
	} else {
		fmt.Print(goalCheckMsg)
	}
}

// Run runs the agent with the given prompt.
func (a *Agent) Run(ctx context.Context, prompt string) (*Result, error) {
	// Log user message if debug is enabled
	if a.debugLogger != nil {
		a.debugLogger.LogUserMessage(prompt, inference.EstimateTokens(prompt))
	}

	// Add user message to context
	a.mu.Lock()
	a.context = append(a.context, &inference.Message{
		Role:    "user",
		Content: prompt,
	})
	// Context size not available until first API response, TUI will show 0 until then
	a.mu.Unlock()

	// Track steps
	var steps []Step

	// Main execution loop
	iteration := 0
	for {
		iteration++
		if iteration > a.maxIterations {
			return nil, fmt.Errorf("maximum iterations (%d) exceeded", a.maxIterations)
		}

		a.mu.Lock()
		a.stats.Iterations = iteration
		a.mu.Unlock()

		// Check if context compression is needed
		if a.shouldCompress() {
			if err := a.compressContext(ctx); err != nil {
				// Failure is reported inside compressContext via streamCallback
			}
		}

		// Get response from LLM (now supports streaming)
		response, err := a.getInferenceResponse(ctx)
		if err != nil {
			return nil, err
		}

		// Add assistant response to context for continuity
		a.mu.Lock()
		assistantMsg := &inference.Message{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.APIToolCalls,
		}
		// Preserve reasoning content in the same property used by the inference server
		// to maintain consistency in the conversation context
		if response.Reasoning != "" {
			if response.ReasoningContentType == "reasoning_content" {
				assistantMsg.ReasoningContent = response.Reasoning
			} else {
				// Default to "reasoning" for "reasoning" or any other/empty type
				assistantMsg.Reasoning = response.Reasoning
			}
		}
		a.context = append(a.context, assistantMsg)
		a.mu.Unlock()

		// Update token stats with accurate values from API
		a.mu.Lock()
		if response.InputTokens > 0 && response.OutputTokens > 0 {
			// Use actual API input/output token counts
			a.stats.InputTokens += response.InputTokens
			a.stats.OutputTokens += response.OutputTokens
		} else if response.TokenUsage > 0 {
			// API didn't split input/output — use total token count from API
			// TokenUsage comes from total_tokens or predicted_n (both from the API)
			a.stats.InputTokens += response.TokenUsage / 2
			a.stats.OutputTokens += response.TokenUsage - response.TokenUsage/2
		}

		// Store total_tokens as the authoritative baseline for context size.
		// This is the exact count the API used: system prompt + messages + tools + completion.
		// We only estimate deltas for new messages added after this response (e.g., tool results).
		a.lastTotalTokens = response.TokenUsage

		// Reset tool result tracking since the API response already accounts for
		// all messages currently in context. Keeping stale indices would cause
		// context size to be overestimated on subsequent calls.
		a.toolResultMsgsSinceLastAPI = make(map[int]bool)

		// Report context size to TUI with real token count (while holding lock)
		a.reportContextSize(a.contextSizeCallback, a.maxContextSize)
		a.mu.Unlock()

		// Log assistant response if debug is enabled
		if a.debugLogger != nil {
			// Log the assistant's text response (if any)
			if response.Content != "" {
				a.debugLogger.LogAssistantMessage(response.Content, response.TokenUsage/2)
			}

			// Log tool calls
			for _, tc := range response.ToolCalls {
				a.debugLogger.LogToolCall(tc.ID, tc.Name, tc.Parameters)
			}
		}

		// Check if there are tool calls
		if len(response.ToolCalls) > 0 {
			// Execute tool calls
			for _, tc := range response.ToolCalls {
				// Check for cancellation before executing each tool
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
				}

				// Show full tool parameters to user before execution
				// The streaming display showed truncated args, but we show the complete
				// command/parameters when the tool is actually called
				streamToolCallWithFullParams(tc, a.streamCallback)

				step := Step{
					Action:   fmt.Sprintf("Calling tool: %s", tc.Name),
					ToolCall: tc,
				}

				// Execute the tool with context support for cancellation
				result := a.toolExecutor.Execute(ctx, tc)
				step.ToolResult = result

				a.mu.Lock()
				a.stats.ToolCalls++
				if !result.Success {
					a.stats.FailedToolCalls++
				}
				a.mu.Unlock()

				// Log tool result if debug is enabled
				if a.debugLogger != nil {
					a.debugLogger.LogToolResult(tc.ID, tc.Name, result.Success, result.Output)
				}

				// Stream tool result status to user
				streamResult(tc.Name, result, a.streamCallback)
				step.StreamMsg = formatToolStatus(tc.Name, result)

				// Add step to the list
				steps = append(steps, step)

				// Add tool result to context with tool_call_id (OpenAI format)
				var resultMessage string
				if result.Success {
					// Special handling for view_image: send image to LLM for vision analysis
					if tc.Name == "view_image" {
						visionResult := a.handleViewImage(ctx, result)
						resultMessage = visionResult
						// Update step with vision result
						step.ToolResult.Output = visionResult
					} else {
						// Use full output for LLM context (not truncated)
						resultMessage = fmt.Sprintf("Tool '%s' executed successfully:\n%s", tc.Name, result.Output)
					}
				} else {
					resultMessage = fmt.Sprintf("Tool '%s' failed: %s", tc.Name, result.Error)
				}

				a.mu.Lock()
				resultIdx := len(a.context)
				a.context = append(a.context, &inference.Message{
					Role:       "tool",
					Content:    resultMessage,
					ToolCallId: tc.ID, // Preserve the original tool call ID
				})
				// Track this tool result so getActualContextSizeUnlocked only
				// counts tool messages added AFTER the last API call.
				a.toolResultMsgsSinceLastAPI[resultIdx] = true
				a.mu.Unlock()
			}
			continue // Loop for next iteration
		}

		// No tool calls - this is a natural end of the agentic loop
		// First, save the assistant response
		a.mu.Lock()
		assistantResponse := response.Content
		streamCallback := a.streamCallback
		a.mu.Unlock()

		// Check if goal mode is active
		if a.shouldCheckGoal() {
			// Check if goal was achieved (case-insensitive) in this natural end response
			if strings.Contains(strings.ToLower(assistantResponse), "goal achieved") {
				// Calculate elapsed time since goal was started
				a.mu.Lock()
				elapsed := time.Since(a.goalStartTime)
				a.mu.Unlock()

				// Stream goal achieved confirmation to TUI with magenta color
				goalAchievedMsg := fmt.Sprintf("\n%s[Goal Achieved] ✓ Goal has been achieved! Time: %s%s\n", colors.GetColor("magenta"), formatGoalDuration(elapsed), colors.GetColor("reset"))
				if streamCallback != nil {
					streamCallback(inference.StreamingChunk{
						Text:        goalAchievedMsg,
						ContentType: inference.StreamingContentTypeGoal,
					})
				} else {
					fmt.Print(goalAchievedMsg)
				}

				// Goal achieved - automatically clear the goal and return the result
				a.ClearGoal()
				a.recordIteration()
				return &Result{
					FinalOutput: assistantResponse,
					Reasoning:   response.Reasoning,
					Steps:       steps,
					TokenUsage:  a.stats.InputTokens + a.stats.OutputTokens,
				}, nil
			}

			// Goal not yet achieved - inject a goal check message as an automatically
			// generated user prompt and continue the loop. The LLM will go through
			// the full agentic loop: it can run tools to verify its work, continue
			// working, or respond with "goal achieved".
			a.injectGoalCheck()

			continue // Continue the loop with the goal check message
		}

		// No tool calls and no goal active - this is the final response
		a.recordIteration()
		return &Result{
			FinalOutput: assistantResponse,
			Reasoning:   response.Reasoning,
			Steps:       steps,
			TokenUsage:  a.stats.InputTokens + a.stats.OutputTokens,
		}, nil
	}
}

// CloseDebugLogger closes the debug logger and writes the session summary.
// This should be called when the session ends (in interactive mode, when exiting).
func (a *Agent) CloseDebugLogger() error {
	if a.debugLogger != nil {
		return a.debugLogger.Close()
	}
	return nil
}

// RunStream runs the agent with streaming support.
func (a *Agent) RunStream(ctx context.Context, prompt string, onChunk StreamCallback) (*Result, error) {
	// Set the stream callback temporarily
	a.mu.Lock()
	savedCallback := a.streamCallback
	a.streamCallback = onChunk
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.streamCallback = savedCallback
		a.mu.Unlock()
	}()

	return a.Run(ctx, prompt)
}

// getInferenceResponse gets a response from the inference backend.
func (a *Agent) getInferenceResponse(ctx context.Context) (*inference.Response, error) {
	a.mu.Lock()
	messages := make([]*inference.Message, len(a.context))
	copy(messages, a.context)
	systemPrompt := a.systemPrompt
	streamCallback := a.streamCallback
	a.mu.Unlock()

	// Use streaming version if callback is set
	if streamCallback != nil {
		return a.inference.InferenceRequestStream(ctx, messages, systemPrompt, streamCallback)
	}

	return a.inference.InferenceRequest(ctx, messages, systemPrompt)
}

// handleViewImage processes the result of a view_image tool call by sending the image
// to a vision-capable model for analysis. Returns the LLM's description of the image.
func (a *Agent) handleViewImage(ctx context.Context, result *tools.ToolResult) string {
	// Extract the data URI from the tool result
	viewExtra := tools.GetViewImageExtra(result)
	if viewExtra == nil || viewExtra.DataURI == "" {
		return fmt.Sprintf("Tool 'view_image' executed but no image data returned:\n%s", result.Output)
	}

	// Stream status to user
	if a.streamCallback != nil {
		a.streamCallback(inference.StreamingChunk{
			Text:        fmt.Sprintf("\n[Viewing image: %s]", result.Path),
			ContentType: inference.StreamingContentTypeNormal,
		})
	} else {
		fmt.Printf("\n%s[Viewing image: %s]%s", colors.GetColor("cyan"), result.Path, colors.GetColor("reset"))
	}

	// Use custom prompt if provided, otherwise use default description prompt
	visionPrompt := "Describe this image in detail. Include any text visible in the image, objects, colors, layout, and any other relevant details."
	if viewExtra.Prompt != "" {
		visionPrompt = viewExtra.Prompt
	}

	// Create a vision message with the image
	msg := &inference.Message{
		Role:    "user",
		Content: visionPrompt,
	}
	msg.SetImageContent(visionPrompt, viewExtra.DataURI, "auto")

	// Send to inference for vision analysis
	response, err := a.inference.InferenceRequest(ctx, []*inference.Message{msg}, "")
	if err != nil {
		return fmt.Sprintf("Tool 'view_image' loaded the image but vision analysis failed: %v", err)
	}

	description := response.Content
	if description == "" {
		description = "(The model did not provide a description of the image)"
	}

	// Stream the description to user
	if a.streamCallback != nil {
		a.streamCallback(inference.StreamingChunk{
			Text:        fmt.Sprintf("\n\nImage description:\n%s", description),
			ContentType: inference.StreamingContentTypeNormal,
		})
	} else {
		fmt.Printf("\n\n%sImage description:\n%s%s", colors.GetColor("blue"), description, colors.GetColor("reset"))
	}

	return fmt.Sprintf("Tool 'view_image' executed successfully.\n\nImage description:\n%s", description)
}

// GetStats returns the current statistics.
func (a *Agent) GetStats() *Stats {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Calculate tokens per second
	tokensPerSecond := 0.0
	elapsed := time.Since(a.stats.StartTime).Seconds()
	if elapsed > 0 {
		totalTokens := a.stats.InputTokens + a.stats.OutputTokens
		tokensPerSecond = float64(totalTokens) / elapsed
	}

	return &Stats{
		InputTokens:      a.stats.InputTokens,
		OutputTokens:     a.stats.OutputTokens,
		ToolCalls:        a.stats.ToolCalls,
		FailedToolCalls:  a.stats.FailedToolCalls,
		Iterations:       a.stats.Iterations,
		CompressionCount: a.compressionCount,
		StartTime:        a.stats.StartTime,
		TokensPerSecond:  tokensPerSecond,
	}
}

// ClearContext clears the conversation context.

// buildTools builds the tool definitions for the OpenAI API.
// It returns the path to the created file or an error if dumping fails.
func (a *Agent) DumpContext() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Create a dump object with current session state
	dump := &ContextDump{
		Version:   1,
		CreatedAt: a.stats.StartTime,
		UpdatedAt: time.Now(),
		Session: SessionInfo{
			StartTime:        a.stats.StartTime,
			Iterations:       a.stats.Iterations,
			CompressionCount: a.compressionCount,
			Stats: StatsInfo{
				InputTokens:     a.stats.InputTokens,
				OutputTokens:    a.stats.OutputTokens,
				ToolCalls:       a.stats.ToolCalls,
				FailedToolCalls: a.stats.FailedToolCalls,
			},
		},
		Iterations: a.iterationHistory,
	}
	// Determine filename in temp directory
	basePath := filepath.Join(os.TempDir(), "coding-agent-context.json")
	filePath := basePath

	// Ensure unique filename if it already exists
	counter := 2
	for {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			break
		}
		filePath = fmt.Sprintf("%s-%d.json", filepath.Join(os.TempDir(), "coding-agent-context"), counter)
		counter++
	}

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal context: %w", err)
	}

	// Write to file with read/write permissions for the owner
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write context file: %w", err)
	}

	return filePath, nil
}

// LoadContext loads a conversation context from the specified JSON file.
// It restores the system prompt, messages, and session statistics.
func (a *Agent) LoadContext(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read context file: %w", err)
	}

	var dump ContextDump
	if err := json.Unmarshal(data, &dump); err != nil {
		return fmt.Errorf("failed to parse context file: %w", err)
	}

	if dump.Version != 1 {
		return fmt.Errorf("unsupported context version: %d", dump.Version)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if len(dump.Iterations) == 0 {
		return fmt.Errorf("context file contains no iterations")
	}

	// Restore from the last iteration snapshot
	lastSnapshot := dump.Iterations[len(dump.Iterations)-1]

	// Restore system prompt from the first message of the last snapshot
	if len(lastSnapshot.Messages) > 0 && lastSnapshot.Messages[0].Role == "system" {
		a.systemPrompt = lastSnapshot.Messages[0].Content
		// Restore messages excluding the system prompt
		a.context = make([]*inference.Message, 0, len(lastSnapshot.Messages)-1)
		for _, msg := range lastSnapshot.Messages[1:] {
			cpy := *msg
			a.context = append(a.context, &cpy)
		}
	} else {
		// Fallback: use messages as is and keep current system prompt
		a.context = make([]*inference.Message, 0, len(lastSnapshot.Messages))
		for _, msg := range lastSnapshot.Messages {
			cpy := *msg
			a.context = append(a.context, &cpy)
		}
	}

	// Restore session metadata
	a.stats.StartTime = dump.Session.StartTime
	a.stats.Iterations = dump.Session.Iterations
	a.compressionCount = dump.Session.CompressionCount

	// Restore token counts
	a.stats.InputTokens = dump.Session.Stats.InputTokens
	a.stats.OutputTokens = dump.Session.Stats.OutputTokens
	a.stats.ToolCalls = dump.Session.Stats.ToolCalls
	a.stats.FailedToolCalls = dump.Session.Stats.FailedToolCalls

	// Reset token tracking baseline
	a.lastTotalTokens = inference.EstimateContextSize(a.context, a.inference.GetTools(), a.systemPrompt)
	a.toolResultMsgsSinceLastAPI = make(map[int]bool)

	return nil
}

// This is a public wrapper around the internal compressContext method,
// allowing the TUI to trigger compression on demand via the /compress command.
func (a *Agent) CompressContext(ctx context.Context) error {
	return a.compressContext(ctx)
}

// getBuildVersion returns the build version string.
// This is injected at build time via linker flags.
var buildVersion = "unknown"

func getBuildVersion() string {
	return buildVersion
}

// SetBuildVersion sets the build version (called from main at startup).
func SetBuildVersion(version string) {
	buildVersion = version
}
