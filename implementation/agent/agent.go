// Package agent implements the main agent logic.
package agent

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/debug"
	"github.com/coding-agent/harness/inference"
	"github.com/coding-agent/harness/tools"
)

// StreamCallback is a function type for handling streaming chunks.
// Using inference.StreamingCallbackWithType for typed streaming support.
type StreamCallback = inference.StreamingCallbackWithType

// ContextSizeCallback is a function called when context size changes.
type ContextSizeCallback func(size, max int)

// StreamCallbackString is a legacy type for backwards compatibility with string callbacks.
type StreamCallbackString func(chunk string)

// Agent represents the coding agent.
type Agent struct {
	config              *config.Config
	inference           *inference.InferenceClient
	toolExecutor        *tools.ToolExecutor
	context             []*inference.Message
	systemPrompt        string
	stats               *Stats
	maxIterations       int
	streamCallback      StreamCallback
	contextSizeCallback ContextSizeCallback
	maxContextSize      int
	compressionCount    int
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
	InputTokens     int       `json:"input_tokens"`
	OutputTokens    int       `json:"output_tokens"`
	ToolCalls       int       `json:"tool_calls"`
	FailedToolCalls int       `json:"failed_tool_calls"`
	Iterations      int       `json:"iterations"`
	StartTime       time.Time `json:"start_time"`
	TokensPerSecond float64   `json:"tokens_per_second"`
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

// Exit code constants for agent execution.
const (
	ExitSuccess        = 0
	ExitError          = 1
	ExitUsageError     = 2
	ExitAuthError      = 3
	ExitContextLimit   = 4
)

// AuthError represents an authentication error (exit code 3).
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "authentication failed"
}

// ContextLimitError represents a context size limit exceeded error (exit code 4).
type ContextLimitError struct {
	Message string
}

func (e *ContextLimitError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "context size limit exceeded"
}

// isAuthError checks if an error string indicates an authentication failure.
func isAuthError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "authentication failed") ||
		strings.Contains(msg, "401") ||
		strings.Contains(msg, "403") ||
		strings.Contains(msg, "Authorization") ||
		strings.Contains(msg, "API key")
}

// isContextLimitError checks if an error string indicates a context size limit.
func isContextLimitError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "context size limit") ||
		strings.Contains(msg, "maximum context length") ||
		strings.Contains(msg, "maximum context length exceeded") ||
		strings.Contains(msg, "maximum context length")
}

// wrapError wraps errors with appropriate typed errors for exit codes.
func wrapError(err error) error {
	if err == nil {
		return nil
	}
	if isAuthError(err) {
		return &AuthError{Message: err.Error()}
	}
	if isContextLimitError(err) {
		return &ContextLimitError{Message: err.Error()}
	}
	return err
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
		config:                   cfg,
		inference:                inference.NewInferenceClient(cfg),
		toolExecutor:             tools.NewToolExecutor(),
		context:                  make([]*inference.Message, 0),
		toolResultMsgsSinceLastAPI: make(map[int]bool),
		stats: &Stats{
			StartTime: time.Now(),
		},
		maxIterations:  cfg.MaxIterations,
		maxContextSize: cfg.ContextSize,
		debugLogger:    debugLogger,
	}

	// Build system prompt and tools based on read-only mode
	agent.systemPrompt = buildSystemPrompt(cfg.ReadOnly)

	// Set read-only mode on tool executor
	agent.toolExecutor.SetReadOnly(cfg.ReadOnly)

	// Log system prompt if debug is enabled
	if agent.debugLogger != nil {
		agent.debugLogger.LogSystemPrompt(agent.systemPrompt, inference.EstimateTokens(agent.systemPrompt))
	}

	// Register tools with inference client
	agent.inference.SetTools(buildTools(cfg.ReadOnly))

	// Display read-only mode warning
	if cfg.ReadOnly {
		fmt.Fprintln(os.Stderr, "============================================================")
		fmt.Fprintln(os.Stderr, "  READ-ONLY MODE ACTIVE")
		fmt.Fprintln(os.Stderr, "============================================================")
		fmt.Fprintln(os.Stderr, "  Only read_file, read_lines, list_files, grep, git_log, git_show, and git_diff tools are available.")
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
				// Log compression failure but continue
				if a.streamCallback != nil {
					a.streamCallback(inference.StreamingChunk{
						Text:        fmt.Sprintf("\n[Warning] Context compression failed: %v\n", err),
						ContentType: inference.StreamingContentTypeNormal,
					})
				}
			}
		}

		// Get response from LLM (now supports streaming)
		response, err := a.getInferenceResponse(ctx)
		if err != nil {
			return nil, err
		}

		// Add assistant response to context for continuity
		a.mu.Lock()
		a.context = append(a.context, &inference.Message{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.APIToolCalls,
		})
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
				// Stream tool call status to user
				// In streaming mode, the inference client already sent "[Tool Call] X" notification.
				// We send "[Running] X" here with parameter details (e.g., the command being executed).
				streamStatus(tc.Name, tc.Parameters, a.streamCallback)

				step := Step{
					Action:   fmt.Sprintf("Calling tool: %s", tc.Name),
					ToolCall: tc,
				}

				// Execute the tool
				result := a.toolExecutor.Execute(tc)
				step.ToolResult = result

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
					// Use full output for LLM context (not truncated)
					resultMessage = fmt.Sprintf("Tool '%s' executed successfully:\n%s", tc.Name, result.Output)
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

		// No tool calls - this is the final response
		return &Result{
			FinalOutput: response.Content,
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

// reportContextSize calculates and reports the current actual context size.
// It calls the unlocked version directly since it is always called while
// the caller already holds a.mu. This avoids deadlocking.
func (a *Agent) reportContextSize(callback ContextSizeCallback, maxContextSize int) {
	if callback != nil {
		actualSize := a.getActualContextSizeUnlocked()
		callback(actualSize, maxContextSize)
	}
}

// GetStats returns the current statistics.
func (a *Agent) GetStats() *Stats {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Get tool executor stats
	toolStats := a.toolExecutor.Stats()

	// Calculate tokens per second
	tokensPerSecond := 0.0
	elapsed := time.Since(a.stats.StartTime).Seconds()
	if elapsed > 0 {
		totalTokens := a.stats.InputTokens + a.stats.OutputTokens
		tokensPerSecond = float64(totalTokens) / elapsed
	}

	return &Stats{
		InputTokens:     a.stats.InputTokens,
		OutputTokens:    a.stats.OutputTokens,
		ToolCalls:       toolStats.TotalCalls,
		FailedToolCalls: toolStats.FailedCalls,
		Iterations:      a.stats.Iterations,
		StartTime:       a.stats.StartTime,
		TokensPerSecond: tokensPerSecond,
	}
}

// ClearContext clears the conversation context.
func (a *Agent) ClearContext() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.context = make([]*inference.Message, 0)
	a.lastTotalTokens = 0
	a.toolResultMsgsSinceLastAPI = make(map[int]bool)
}

// AddUserMessage adds a user message to the context.
func (a *Agent) AddUserMessage(message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.context = append(a.context, &inference.Message{
		Role:    "user",
		Content: message,
	})
}

// AddAssistantMessage adds an assistant message to the context.
func (a *Agent) AddAssistantMessage(message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.context = append(a.context, &inference.Message{
		Role:    "assistant",
		Content: message,
	})
}

// GetContextSize returns the current context size.
// Uses total_tokens from the last API response as the authoritative count.
func (a *Agent) GetContextSize() int {
	return a.GetActualContextSize()
}

// GetActualContextSize returns the exact total_tokens from the last API response,
// which is the authoritative count of everything the API processed:
// system prompt + messages + tools + completion.
// Then adds estimated tokens for any new messages added after the response
// (e.g., tool results that were appended during tool execution).
func (a *Agent) GetActualContextSize() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.getActualContextSizeUnlocked()
}

// getActualContextSizeUnlocked is the internal, unlocked version.
// Must be called while holding a.mu.
func (a *Agent) getActualContextSizeUnlocked() int {
	if a.lastTotalTokens > 0 {
		// Only count tool messages added AFTER the last API call.
		// Messages before the API call were already included in total_tokens.
		delta := 0
		for idx, msg := range a.context {
			if msg.Role == "tool" && a.toolResultMsgsSinceLastAPI[idx] {
				delta += 3 + inference.EstimateTokens(msg.Content)
			}
		}
		return a.lastTotalTokens + delta
	}
	// No API response yet, estimate from scratch
	return inference.EstimateContextSize(a.context, a.inference.GetTools(), a.systemPrompt)
}

// shouldCompress checks if context compression is needed based on actual context window usage.
func (a *Agent) shouldCompress() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	total := a.getActualContextSizeUnlocked()
	maxSize := a.maxContextSize
	return total > int(float64(maxSize)*0.8)
}

// compressContext compresses the conversation history while preserving system prompt.
// After compression, the context is: [first_user_message, assistant_summary, ...last_3_messages...]
func (a *Agent) compressContext(ctx context.Context) error {
	a.mu.Lock()
	if len(a.context) <= 2 {
		a.mu.Unlock()
		return nil // Nothing to compress
	}

	// Keep the first user message and last few messages
	preserveCount := 3
	if len(a.context) <= preserveCount+1 {
		a.mu.Unlock()
		return nil
	}

	messages := make([]*inference.Message, len(a.context))
	copy(messages, a.context)
	a.mu.Unlock()

	// First user message is always preserved
	firstUserMsg := messages[0]
	// Ensure we actually have a user message as the first message.
	// If the first message is not a user message (e.g., assistant added programmatically),
	// find the first user message or use the first message as fallback.
	if firstUserMsg.Role != "user" {
		for _, msg := range messages {
			if msg.Role == "user" {
				firstUserMsg = msg
				break
			}
		}
	}

	// Messages to summarize: everything between first user msg and last N preserved messages
	summaryMessages := messages[1 : len(messages)-preserveCount]

	// Build summary prompt
	summaryReq := fmt.Sprintf("Summarize the following conversation history concisely, preserving key information, decisions, and results:\n\n")
	for _, msg := range summaryMessages {
		summaryReq += fmt.Sprintf("%s: %s\n\n", msg.Role, msg.Content)
	}
	summaryReq += "\nProvide a concise summary that captures all essential information."

	// Get summary from LLM
	summaryMsg := &inference.Message{Role: "user", Content: summaryReq}
	response, err := a.inference.InferenceRequest(ctx, []*inference.Message{summaryMsg}, "You are a conversation summarizer.")
	if err != nil {
		return fmt.Errorf("failed to compress context: %w", err)
	}

	// Rebuild context: first_user_message + assistant_summary + preserved messages
	// Note: we do NOT add the system prompt here because buildMessages() in the
	// inference client prepends it on every API call. Adding it here would cause
	// duplicate system prompts.
	a.mu.Lock()
	newContext := make([]*inference.Message, 0, preserveCount+2)
	newContext = append(newContext, firstUserMsg) // Preserve the original user prompt

	// Summary is stored as an assistant message to maintain conversation flow
	summaryContent := "Conversation summary: " + response.Content
	newContext = append(newContext, &inference.Message{Role: "assistant", Content: summaryContent})

	// Preserve the last N messages, but reorder to maintain conversation integrity.
	// Group consecutive tool results with their preceding assistant message so
	// that we never have a tool message without its corresponding assistant message.
	preserved := messages[len(messages)-preserveCount:]
	grouped := a.groupAssistantToolMessages(preserved)
	newContext = append(newContext, grouped...)
	a.context = newContext
	a.compressionCount++

	// Reset lastTotalTokens so context size is recalculated from scratch on next
	// context size check. This prevents the old (pre-compression) total from
	// inflating the reported size after compression.
	a.lastTotalTokens = 0

	// Reset tool result tracking since the context has been rebuilt.
	a.toolResultMsgsSinceLastAPI = make(map[int]bool)

	// Preserve cumulative token stats. The stats represent the total tokens used
	// across the entire session. We should NOT overwrite them with the compressed
	// context size; instead, the next API call will add its own token usage on top
	// of the existing cumulative totals.
	// (No change to a.stats.InputTokens / a.stats.OutputTokens)
	a.mu.Unlock()

	return nil
}

// groupAssistantToolMessages takes a slice of messages and reorders them so that
// tool results are always immediately after their corresponding assistant message.
// This prevents broken sequences like: [user_summary, tool_result, assistant, user].
func (a *Agent) groupAssistantToolMessages(messages []*inference.Message) []*inference.Message {
	if len(messages) == 0 {
		return messages
	}

	// Build groups: each group starts with a non-tool message, followed by its
	// tool results (if the preceding message was an assistant with tool calls).
	var groups [][]*inference.Message
	currentGroup := []*inference.Message{messages[0]}

	for i := 1; i < len(messages); i++ {
		msg := messages[i]
		if msg.Role == "tool" {
			// Tool results should be grouped with the preceding assistant message.
			// If the current group's last message is an assistant, keep it here.
			// Otherwise, we need to move it.
			if len(currentGroup) > 0 && currentGroup[len(currentGroup)-1].Role == "assistant" {
				currentGroup = append(currentGroup, msg)
			} else {
				// This tool result's assistant is not in the preserved set.
				// Skip it to avoid broken causality.
				continue
			}
		} else {
			// Non-tool message starts a new group.
			if len(currentGroup) > 0 {
				groups = append(groups, currentGroup)
			}
			currentGroup = []*inference.Message{msg}
		}
	}
	if len(currentGroup) > 0 {
		groups = append(groups, currentGroup)
	}

	// Flatten groups, preserving order within each group.
	result := make([]*inference.Message, 0, len(messages))
	for _, group := range groups {
		result = append(result, group...)
	}
	return result
}

// ANSI color codes for tool feedback
const (
	ColorReset  = "\033[0m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorRed    = "\033[31m"
	ColorCyan   = "\033[36m"
	ColorBlue   = "\033[34m"
)

// streamStatus streams a tool call status message with color.
// If callback is nil, prints to stdout instead.
func streamStatus(toolName string, params map[string]interface{}, callback StreamCallback) {
	var msg string
	switch toolName {
	case "bash":
		cmd := ""
		if p, ok := params["command"].(string); ok {
			cmd = p
			if len(cmd) > 50 {
				cmd = cmd[:50] + "..."
			}
		}
		msg = fmt.Sprintf("\n%s[Running] bash: %s%s\n", ColorCyan, cmd, ColorReset)
	case "read_file":
		path := ""
		if p, ok := params["path"].(string); ok {
			path = p
		}
		msg = fmt.Sprintf("\n%s[Reading] file: %s%s\n", ColorCyan, path, ColorReset)
	case "read_lines":
		path := ""
		start, end := 0, 0
		if p, ok := params["path"].(string); ok {
			path = p
		}
		if p, ok := params["start"].(float64); ok {
			start = int(p)
		}
		if p, ok := params["end"].(float64); ok {
			end = int(p)
		}
		msg = fmt.Sprintf("\n%s[Reading] lines %d-%d from: %s%s\n", ColorCyan, start, end, path, ColorReset)
	case "write_file":
		path := ""
		if p, ok := params["path"].(string); ok {
			path = p
		}
		msg = fmt.Sprintf("\n%s[Writing] file: %s%s\n", ColorCyan, path, ColorReset)
	case "insert_lines":
		path := ""
		line := 0
		if p, ok := params["path"].(string); ok {
			path = p
		}
		if p, ok := params["line"].(float64); ok {
			line = int(p)
		}
		msg = fmt.Sprintf("\n%s[Inserting] at line %d in: %s%s\n", ColorCyan, line, path, ColorReset)
	case "replace_text":
		path := ""
		search := ""
		if p, ok := params["path"].(string); ok {
			path = p
		}
		if p, ok := params["search"].(string); ok {
			search = p
			if len(search) > 30 {
				search = search[:30] + "..."
			}
		}
		msg = fmt.Sprintf("\n%s[Replacing] '%s' in: %s%s\n", ColorCyan, search, path, ColorReset)
	default:
		msg = fmt.Sprintf("\n%s[Running] tool: %s%s\n", ColorCyan, toolName, ColorReset)
	}

	if callback != nil {
		callback(inference.StreamingChunk{
			Text:        msg,
			ContentType: inference.StreamingContentTypeNormal,
		})
	} else {
		fmt.Print(msg)
	}
}

// streamResult streams a tool result status message with color.
// If callback is nil, prints to stdout instead.
func streamResult(toolName string, result *tools.ToolResult, callback StreamCallback) {
	status := formatToolStatus(toolName, result)
	if callback != nil {
		callback(inference.StreamingChunk{
			Text:        status,
			ContentType: inference.StreamingContentTypeNormal,
		})
	} else {
		fmt.Print(status)
	}
}

// formatToolStatus formats a tool status message for display with colors.
func formatToolStatus(toolName string, result *tools.ToolResult) string {
	if result.Success {
		switch toolName {
		case "bash":
			// Show exit code and truncated output
			output := result.Output
			lines := strings.Split(output, "\n")
			if len(lines) > 5 {
				lines = lines[:5]
				output = strings.Join(lines, "\n") + "\n... [output truncated]"
			}
			exitCode := ""
			if result.ExitCode != 0 {
				exitCode = fmt.Sprintf(" (exit code: %d)", result.ExitCode)
			}
			return fmt.Sprintf("%s[Success] bash completed%s\nOutput:\n%s%s\n", ColorGreen, exitCode, output, ColorReset)
		case "read_file":
			output := result.Output
			lines := strings.Split(output, "\n")
			if len(lines) > 10 {
				lines = lines[:10]
				output = strings.Join(lines, "\n") + "\n... [content truncated]"
			}
			// Use accurate line count from Extra if available
			linesRead := len(strings.Split(result.Output, "\n"))
			if result.Extra != nil {
				if lr, ok := result.Extra["linesRead"].(int); ok {
					linesRead = lr
				}
			}
			return fmt.Sprintf("%s[Success] read %d lines\nContent:\n%s%s\n", ColorGreen, linesRead, output, ColorReset)
		case "write_file":
			if msg, ok := result.Extra["message"].(string); ok {
				return fmt.Sprintf("%s[Success] %s%s\n", ColorGreen, msg, ColorReset)
			}
			return fmt.Sprintf("%s[Success] file written%s\n", ColorGreen, ColorReset)
		case "read_lines":
			// Show the lines that were read, truncated if too long
			output := result.Output
			lines := strings.Split(output, "\n")
			linesRead := len(lines)
			if linesRead > 10 {
				lines = lines[:10]
				output = strings.Join(lines, "\n") + "\n... [output truncated]"
				linesRead = len(lines)
			}
			return fmt.Sprintf("%s[Success] read %d lines\nContent:\n%s%s\n", ColorGreen, linesRead, output, ColorReset)
		case "insert_lines":
			count := 0
			if c, ok := result.Extra["linesInserted"].(int); ok {
				count = c
			}
			return fmt.Sprintf("%s[Success] inserted %d line(s)%s\n", ColorGreen, count, ColorReset)
		case "replace_text":
			count := 0
			if c, ok := result.Extra["replacementsMade"].(int); ok {
				count = c
			}
			search := ""
			if s, ok := result.Extra["search"].(string); ok {
				search = s
				if len(search) > 20 {
					search = search[:20] + "..."
				}
			}
			return fmt.Sprintf("%s[Success] replaced '%s' %d time(s)%s\n", ColorGreen, search, count, ColorReset)
		case "list_files":
			entries := 0
			if e, ok := result.Extra["entriesListed"].(int); ok {
				entries = e
			}
			return fmt.Sprintf("%s[Success] listed %d entries%s\n", ColorGreen, entries, ColorReset)
		case "grep":
			return fmt.Sprintf("%s[Success] grep completed%s\n", ColorGreen, ColorReset)
		case "git_log":
			reference := "HEAD"
			if r, ok := result.Extra["reference"].(string); ok && r != "" {
				reference = r
			}
			count := 0
			if c, ok := result.Extra["count"].(int); ok {
				count = c
			}
			return fmt.Sprintf("%s[Success] git log: %d commits from %s%s\n", ColorGreen, count, reference, ColorReset)
		case "git_show":
			commit := "HEAD"
			if c, ok := result.Extra["commitReference"].(string); ok && c != "" {
				commit = c
			}
			return fmt.Sprintf("%s[Success] git show %s%s\n", ColorGreen, commit, ColorReset)
		case "git_diff":
			ref1 := ""
			if r, ok := result.Extra["reference1"].(string); ok && r != "" {
				ref1 = r
			}
			ref2 := ""
			if r, ok := result.Extra["reference2"].(string); ok && r != "" {
				ref2 = r
			}
			msg := fmt.Sprintf("%s[Success] git diff", ColorGreen)
			if ref1 != "" {
				msg += fmt.Sprintf(" %s", ref1)
			}
			if ref2 != "" {
				msg += fmt.Sprintf(" %s", ref2)
			}
			msg += ColorReset
			return msg
		default:
			return fmt.Sprintf("%s[Success] tool completed%s\n", ColorGreen, ColorReset)
		}
	}
	return fmt.Sprintf("%s[Failed] %s\nError: %s%s\n", ColorRed, toolName, result.Error, ColorReset)
}

// getEnvironmentInfo gathers runtime environment information.
func getEnvironmentInfo() string {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "unknown"
	}

	// Get executable path
	exePath, err := os.Executable()
	if err != nil {
		exePath = "unknown"
	}

	// Get OS and architecture
	osInfo := runtime.GOOS
	archInfo := runtime.GOARCH

	return fmt.Sprintf(`ENVIRONMENT INFORMATION:
- Current Working Directory: %s
- Agent Executable: %s
- Operating System: %s
- Architecture: %s

You can use the agent executable to spawn sub-agents for parallel tasks using the -p parameter:
  coding-agent -p "Your task here"
`, cwd, exePath, osInfo, archInfo)
}

// buildSystemPrompt builds the system prompt with tool definitions.
// When readOnly is true, only read-only tools are included.
func buildSystemPrompt(readOnly bool) string {
	// Get environment information
	envInfo := getEnvironmentInfo()

	if readOnly {
		return buildReadOnlySystemPrompt(envInfo)
	}

	return fmt.Sprintf(`You are a helpful coding assistant. You have access to the following tools.

%s

TOOL CALLING FORMAT:
- When you need to use a tool, the API will return a response containing tool calls
- Execute each tool call and report the result back as a tool message
- You do NOT need to construct JSON manually - the tool calling API handles the formatting
- Each tool has specific parameters that must be provided (marked as "required")

EXAMPLE workflow:
1. User asks you to list files in a directory
2. The API returns a tool call: {"name": "bash", "arguments": {"command": "ls -la /path"}}
3. Execute the tool and report the result back as a tool message with the matching tool_call_id
4. The API processes the result and may return another tool call or your final answer

AVAILABLE TOOLS:

1. bash
   Description: Execute a bash command in the terminal
   Parameters:
     - command (string, required): The bash command to execute
   How to call: Use the bash tool when you need to run shell commands, install packages, build projects, check file system, etc.
   Example use case: "ls -la", "cat file.txt", "npm install", "pip install -r requirements.txt"

2. read_file
   Description: Read the contents of a file
   Parameters:
     - path (string, required): The path to the file to read
   How to call: Use read_file to view the contents of any file before making changes.
   Example use case: Reading source files, configuration files, documentation

3. write_file
   Description: Write content to a file
   Parameters:
     - path (string, required): The path to the file to write
     - content (string, required): The content to write to the file
   How to call: Use write_file to create new files or completely overwrite existing files.
   Example use case: Creating new source files, writing configuration, saving output
   Note: For multi-line content, use \n to represent newlines in the content parameter

4. read_lines
   Description: Read a specific line range from a file
   Parameters:
     - path (string, required): The path to the file
     - start (integer, required): The starting line number (1-indexed)
     - end (integer, required): The ending line number (1-indexed)
   How to call: Use read_lines when you only need to view a portion of a large file.
   Example use case: Viewing lines 1-50 of a large source file, checking specific sections

5. insert_lines
   Description: Insert lines at a specific line number
   Parameters:
     - path (string, required): The path to the file
     - line (integer, required): The line number where insertion should occur (1-indexed)
     - lines (string, required): The lines to insert (use \n for newlines)
   How to call: Use insert_lines to add new content without replacing existing content.
   Example use case: Adding imports, inserting new functions, adding comments
   Note: Inserting at line 1 adds at the beginning; inserting beyond file length appends

6. replace_text
   Description: Find and replace text in a file by searching for a pattern
   Parameters:
     - path (string, required): The path to the file
     - search (string, required): Text pattern to find (exact match, not regex)
     - replace (string, required): Replacement text
     - count (integer, optional): Number of occurrences to replace (default: 1, use -1 for all)
   How to call: Use replace_text when you know the text to find but not the line numbers.
   Example use case: Renaming variables, updating function names, fixing typos throughout a file


TOOL CALLING BEST PRACTICES:
1. Always read a file first (using read_file or read_lines) to understand its contents
2. When modifying files, be precise about what you're changing
3. For multi-line content, properly format with \n for newlines
4. Verify your changes by re-reading files after writing
5. Test code by running appropriate commands for the language (e.g., go build, npm test, pytest, etc.)

VERIFICATION REQUIREMENTS:
- ALWAYS double-check your work before considering a task complete
- Verify that created/modified files exist and contain the expected content
- Test code execution when possible (e.g., run go build, npm test, pytest, cargo test, etc.)
- Validate that changes meet the user's requirements
- If you make multiple changes, verify each one independently
- Re-read files after writing to confirm content was written correctly
- Run validation commands (e.g., go vet, gofmt -d, pylint, eslint, cat to view files)
- If verification fails, fix the issue and re-verify
- Provide a final verification summary before concluding the task

Verification Checklist:
1. Files exist at the expected paths
2. File content matches the intended changes
3. Code compiles/builds without errors (for compiled languages)
4. Code formatting and linting (e.g., gofmt, black, prettier, rustfmt, etc.)
5. Changes align with user requirements
6. No unintended side effects or broken dependencies`, envInfo)
}

// buildReadOnlySystemPrompt builds a system prompt for read-only mode.
func buildReadOnlySystemPrompt(envInfo string) string {
	return fmt.Sprintf(`You are a helpful coding assistant operating in READ-ONLY MODE. You have access only to the following read-only tools.

%s

IMPORTANT: This session is in read-only mode. You can ONLY read files and list directories.
You CANNOT modify, write, delete, execute, or make any changes to files or the system.

TOOL CALLING FORMAT:
- When you need to use a tool, the API will return a response containing tool calls
- Execute each tool call and report the result back as a tool message
- You do NOT need to construct JSON manually - the tool calling API handles the formatting
- Each tool has specific parameters that must be provided (marked as "required")

AVAILABLE TOOLS:

1. read_file
   Description: Read the contents of a file
   Parameters:
     - path (string, required): The path to the file to read
   How to call: Use read_file to view the contents of any file.
   Example use case: Reading source files, configuration files, documentation

2. read_lines
   Description: Read a specific line range from a file
   Parameters:
     - path (string, required): The path to the file
     - start (integer, required): The starting line number (1-indexed)
     - end (integer, required): The ending line number (1-indexed)
   How to call: Use read_lines when you only need to view a portion of a large file.
   Example use case: Viewing lines 1-50 of a large source file, checking specific sections

3. list_files
   Description: List files and directories in a path, similar to the ls command
   Parameters:
     - path (string, optional): The path to the file or directory to list (defaults to current directory if not specified)
     - flags (array, optional): List of ls-style flags to control output (e.g., 'l' for long format, 'a' for all including hidden, 'h' for human-readable sizes, 't' for time-sorted, 'S' for size-sorted, 'R' for recursive)
   How to call: Use list_files to see files, folders, sizes, permissions, and other information formatted like a simple ls command.
   Example use case: Listing directory contents with details, checking file sizes, viewing hidden files
4. grep
    Description: Search through file contents using grep-like pattern matching
    Parameters:
      - path (string, optional): Path to search (defaults to current directory if not specified)
      - pattern (string, required): Pattern to search for (supports regex)
      - flags (array, optional): List of grep-style flags to control output (e.g., '-n' for line numbers, '-i' for case insensitive, '-r' for recursive)
    How to call: Use grep to find specific patterns or text within files.
    Example use case: Finding where a function is defined, searching for error messages, locating configuration values

5. git_log
    Description: Show commit logs from a git repository
    Parameters:
      - path (string, optional): Path to the git repository (defaults to current directory)
      - reference (string, optional): Git reference to view log from (branch name, tag, or commit hash; defaults to HEAD)
      - count (integer, optional): Number of commits to display (defaults to 10)
      - flags (array, optional): List of git log flags to control output (e.g., '--oneline', '--stat', '--patch', '--follow', '--grep')
    How to call: Use git_log to view commit history and understand changes in the repository.
    Example use case: Reviewing recent changes, finding when a bug was introduced, understanding project history

6. git_show
    Description: Show information about a git commit
    Parameters:
      - path (string, optional): Path to the git repository (defaults to current directory)
      - commit (string, optional): Commit to show (defaults to HEAD)
      - flags (array, optional): List of git show flags to control output (e.g., '--stat', '--patch', '--name-status')
    How to call: Use git_show to examine the details of a specific commit, including its changes and metadata.
    Example use case: Examining a specific commit's changes, reviewing what was modified in a particular update

7. git_diff
    Description: Show changes between commits, commit and working tree, etc.
    Parameters:
      - path (string, optional): Path to the git repository (defaults to current directory)
      - reference1 (string, optional): First git reference for comparison (commit hash, branch, tag; omit for working tree)
      - reference2 (string, optional): Second git reference for comparison (commit hash, branch, tag; omit for index or working tree)
      - flags (array, optional): List of git diff flags to control output (e.g., '--stat', '--patch', '--name-status', '--numstat', '--summary', '--color')
    How to call: Use git_diff to compare different versions of files, branches, or commits.
    Example use case: Comparing changes between two branches, viewing modifications in a specific commit, checking differences in the working tree



TOOL CALLING BEST PRACTICES:
1. Use read_file, read_lines, and list_files to explore and read files
2. Use grep to search for patterns and text within files
3. Use git_log, git_show, and git_diff to explore git history and changes
4. Remember: you cannot modify any files or execute commands

NOTE: If the user asks you to write, modify, delete, or execute anything, explain that you are in read-only mode and cannot perform write operations.`, envInfo)
}

// buildTools builds the tool definitions for the OpenAI API.
// When readOnly is true, only read-only tools (read_file, read_lines, list_files, grep, git_log, git_show, git_diff) are returned.
func buildTools(readOnly bool) []inference.ToolDefinition {
	if readOnly {
		return buildReadOnlyTools()
	}

	return []inference.ToolDefinition{
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "bash",
				Description: "Execute a bash command in the terminal",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"command": {
							Type:        "string",
							Description: "The bash command to execute",
						},
					},
					Required: []string{"command"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "read_file",
				Description: "Read the contents of a file",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the file to read",
						},
					},
					Required: []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "write_file",
				Description: "Write content to a file",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the file to write",
						},
						"content": {
							Type:        "string",
							Description: "Content to write to the file",
						},
					},
					Required: []string{"path", "content"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "read_lines",
				Description: "Read a specific line range from a file",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the file to read",
						},
						"start": {
							Type:        "integer",
							Description: "Starting line number (1-indexed)",
						},
						"end": {
							Type:        "integer",
							Description: "Ending line number (1-indexed)",
						},
					},
					Required: []string{"path", "start", "end"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "insert_lines",
				Description: "Insert lines at a specific line number in a file",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "File path to modify",
						},
						"line": {
							Type:        "integer",
							Description: "Line number to insert before (1-indexed)",
						},
						"lines": {
							Type:        "string",
							Description: "Lines to insert (use \\n for newlines)",
						},
					},
					Required: []string{"path", "line", "lines"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "replace_text",
				Description: "Find and replace text in a file by searching for a pattern",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "File path to modify",
						},
						"search": {
							Type:        "string",
							Description: "Text pattern to find (exact match, not regex)",
						},
						"replace": {
							Type:        "string",
							Description: "Replacement text",
						},
						"count": {
							Type:        "integer",
							Description: "Number of occurrences to replace (default: 1, use -1 for all)",
						},
					},
					Required: []string{"path", "search", "replace"},
				},
			},
		},
	}
}

// buildReadOnlyTools returns only the read-only tool definitions.
func buildReadOnlyTools() []inference.ToolDefinition {
	return []inference.ToolDefinition{
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "read_file",
				Description: "Read the contents of a file",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the file to read",
						},
					},
					Required: []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "read_lines",
				Description: "Read a specific line range from a file",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the file to read",
						},
						"start": {
							Type:        "integer",
							Description: "Starting line number (1-indexed)",
						},
						"end": {
							Type:        "integer",
							Description: "Ending line number (1-indexed)",
						},
					},
					Required: []string{"path", "start", "end"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "list_files",
				Description: "List files and directories in a path, similar to the ls command",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the file or directory to list (defaults to current directory if not specified)",
						},
						"flags": {
							Type:        "array",
							Description: "List of ls-style flags to control output (e.g., 'l' for long format, 'a' for all including hidden, 'h' for human-readable sizes, 't' for time-sorted, 'S' for size-sorted, 'R' for recursive)",
							Items: &inference.Property{
								Type: "string",
							},
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "grep",
				Description: "Search through file contents using grep-like pattern matching",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to search (defaults to current directory if not specified)",
						},
						"pattern": {
							Type:        "string",
							Description: "Pattern to search for (supports regex)",
						},
						"flags": {
							Type:        "array",
							Description: "List of grep-style flags to control output (e.g., '-n' for line numbers, '-i' for case insensitive, '-r' for recursive, '-f' for pattern file, '-a' for all including hidden, '-c' for count, '-v' for invert match, '-l' for filenames only)",
							Items: &inference.Property{
								Type: "string",
							},
						},
					},
					Required: []string{"pattern"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "git_log",
				Description: "Show commit logs from a git repository",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the git repository (defaults to current directory)",
						},
						"reference": {
							Type:        "string",
							Description: "Git reference to view log from (branch name, tag, or commit hash; defaults to HEAD)",
						},
						"count": {
							Type:        "integer",
							Description: "Number of commits to display (defaults to 10)",
						},
						"grep": {
							Type:        "string",
							Description: "Search commit messages for this pattern (used with '--grep' flag)",
						},
						"flags": {
							Type:        "array",
							Description: "List of git log flags to control output (e.g., 's' for short format, 'm' for merges, 'no-merges', 'stat', 'patch', 'oneline', 'shortstat', 'follow', 'grep' to search commit messages, 'decorate', 'graph', 'first-parent')",
							Items: &inference.Property{
								Type: "string",
							},
						},
					},
					Required: []string{},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "git_show",
				Description: "Show information about a git commit",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the git repository (defaults to current directory)",
						},
						"commit": {
							Type:        "string",
							Description: "Commit to show (defaults to HEAD)",
						},
						"flags": {
							Type:        "array",
							Description: "List of git show flags to control output (e.g., 'stat', 'patch', 'p', 'name-status', 'name-only', 'shortstat', 'numstat', 'oneline', 's' for short format, 'no-patch', 'summary', 'r' for rename detection, 'M' for copy detection)",
							Items: &inference.Property{
								Type: "string",
							},
						},
					},
					Required: []string{},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "git_diff",
				Description: "Show changes between commits, commit and working tree, etc.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the git repository (defaults to current directory)",
						},
						"reference1": {
							Type:        "string",
							Description: "First git reference for comparison (commit hash, branch, tag; omit for working tree)",
						},
						"reference2": {
							Type:        "string",
							Description: "Second git reference for comparison (commit hash, branch, tag; omit for index or working tree)",
						},
						"flags": {
							Type:        "array",
							Description: "List of git diff flags to control output (e.g., 'stat', 'patch', 'p', 'name-status', 'name-only', 'shortstat', 'numstat', 'color', 'summary', 'compact-summary', 'stat-width', 'ignore-space-at-eol', 'ignore-space-change', 'ignore-all-space', 'unified', 'raw', 'r' for rename detection, 'M' for copy detection, 'patience', 'minimal')",
							Items: &inference.Property{
								Type: "string",
							},
						},
					},
					Required: []string{},
				},
			},
		},
	}
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
