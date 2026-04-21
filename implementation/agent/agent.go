// Package agent implements the main agent logic.
package agent

import (
	"context"
	"encoding/json"
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
	Steps       []Step
	TokenUsage  int
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
		config:       cfg,
		inference:    inference.NewInferenceClient(cfg),
		toolExecutor: tools.NewToolExecutor(),
		context:      make([]*inference.Message, 0),
		stats: &Stats{
			StartTime: time.Now(),
		},
		maxIterations:  cfg.MaxIterations,
		maxContextSize: cfg.ContextSize,
		debugLogger:    debugLogger,
	}

	// Build system prompt
	agent.systemPrompt = buildSystemPrompt()

	// Log system prompt if debug is enabled
	if agent.debugLogger != nil {
		agent.debugLogger.LogSystemPrompt(agent.systemPrompt, inference.EstimateTokens(agent.systemPrompt))
	}

	// Register tools with inference client
	agent.inference.SetTools(buildTools())

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
		a.mu.Unlock()

		// Report context size to TUI with real token count
		a.reportContextSize(a.contextSizeCallback, a.maxContextSize)

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
				// In streaming mode, the tool call notification was already sent during inference
				// So we only need to stream the execution status
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
				a.context = append(a.context, &inference.Message{
					Role:       "tool",
					Content:    resultMessage,
					ToolCallId: tc.ID, // Preserve the original tool call ID
				})
				a.mu.Unlock()
			}
			continue // Loop for next iteration
		}

		// No tool calls - this is the final response
		return &Result{
			FinalOutput: response.Content,
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

// reportContextSize calculates and reports the current actual context size using message-level token estimation.
func (a *Agent) reportContextSize(callback ContextSizeCallback, maxContextSize int) {
	if callback != nil {
		actualSize := a.GetActualContextSize()
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
}

// SessionData represents a saved conversation session.
type SessionData struct {
	SystemPrompt string              `json:"system_prompt"`
	Messages     []*inference.Message `json:"messages"`
	Stats        *Stats              `json:"stats"`
	Timestamp    string              `json:"timestamp"`
	ToolDefs     []inference.ToolDefinition `json:"tool_definitions,omitempty"`
}

// SaveSession saves the current conversation session to a file.
func (a *Agent) SaveSession(filepath string) error {
	a.mu.Lock()
	messages := make([]*inference.Message, len(a.context))
	for i, msg := range a.context {
		msgCopy := *msg
		messages[i] = &msgCopy
	}
	systemPrompt := a.systemPrompt
	statsCopy := *a.stats
	toolsCopy := make([]inference.ToolDefinition, len(a.inference.GetTools()))
	for i, t := range a.inference.GetTools() {
		toolsCopy[i] = t
	}
	a.mu.Unlock()

	session := &SessionData{
		SystemPrompt: systemPrompt,
		Messages:     messages,
		Stats:        &statsCopy,
		Timestamp:    time.Now().Format(time.RFC3339),
		ToolDefs:     toolsCopy,
	}

	jsonData, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(filepath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// LoadSession loads a conversation session from a file.
func (a *Agent) LoadSession(filepath string) error {
	jsonData, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read session file: %w", err)
	}

	var session SessionData
	if err := json.Unmarshal(jsonData, &session); err != nil {
		return fmt.Errorf("failed to parse session file: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Restore system prompt
	a.systemPrompt = session.SystemPrompt

	// Restore messages
	a.context = make([]*inference.Message, len(session.Messages))
	copy(a.context, session.Messages)

	// Restore stats
	a.stats = session.Stats
	if a.stats == nil {
		a.stats = &Stats{}
	}

	// Restore tool definitions
	if len(session.ToolDefs) > 0 {
		a.inference.SetTools(session.ToolDefs)
	}

	// Recalculate input tokens from the restored context
	a.stats.InputTokens = inference.EstimateContextSize(a.context, a.inference.GetTools(), a.systemPrompt)
	a.stats.OutputTokens = 0

	return nil
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

// GetContextSize returns the current actual context size using message-level token estimation.
// This reflects the real context window usage, suitable for display to users and compression checks.
func (a *Agent) GetContextSize() int {
	return a.GetActualContextSize()
}

// GetActualContextSize calculates the current context size by summing token estimates
// of all messages in the context. This reflects the actual context window usage,
// not cumulative API token counts.
func (a *Agent) GetActualContextSize() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.getActualContextSizeUnlocked()
}

// getActualContextSizeUnlocked is the internal, unlocked version.
// Must be called while holding a.mu.
func (a *Agent) getActualContextSizeUnlocked() int {
	// Include tool definitions in the count since they are sent with every API request
	return inference.EstimateContextSize(a.context, a.inference.GetTools(), a.systemPrompt)
}

// shouldCompress checks if context compression is needed based on actual context window usage.
func (a *Agent) shouldCompress() bool {
	a.mu.Lock()
	total := a.getActualContextSizeUnlocked()
	maxSize := a.maxContextSize
	a.mu.Unlock()
	return total > int(float64(maxSize)*0.8)
}

// compressContext compresses the conversation history while preserving system prompt.
func (a *Agent) compressContext(ctx context.Context) error {
	a.mu.Lock()
	if len(a.context) <= 2 {
		a.mu.Unlock()
		return nil // Nothing to compress
	}

	// Keep system prompt and last few messages
	preserveCount := 3
	if len(a.context) <= preserveCount {
		a.mu.Unlock()
		return nil
	}

	messages := make([]*inference.Message, len(a.context))
	copy(messages, a.context)
	systemPrompt := a.systemPrompt
	a.mu.Unlock()

	// Create summary request - summarize all but system prompt and last messages
	summaryMessages := messages[1 : len(messages)-preserveCount] // Skip system prompt, keep last messages

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

	// Rebuild context: summary + preserved messages
	// Note: we do NOT add the system prompt here because buildMessages() in the
	// inference client prepends it on every API call. Adding it here would cause
	// duplicate system prompts.
	a.mu.Lock()
	newContext := make([]*inference.Message, 0, preserveCount+1)
	newContext = append(newContext, &inference.Message{Role: "user", Content: "Conversation summary: " + response.Content})
	newContext = append(newContext, messages[len(messages)-preserveCount:]...)
	a.context = newContext
	a.compressionCount++

	// Update token stats to reflect the compressed context size.
	// This prevents the infinite compression loop: the cumulative stats would
	// otherwise still be high even though the actual context is now small.
	// We set InputTokens to the estimated size of the compressed context,
	// which is approximately what the next API request will consume.
	compressedTokens := inference.EstimateContextSize(newContext, a.inference.GetTools(), systemPrompt)
	a.stats.InputTokens = compressedTokens
	a.stats.OutputTokens = 0 // reset output tokens; they'll accumulate from new API calls
	a.mu.Unlock()

	return nil
}

// formatResult formats a tool result for display with truncation.
func formatResult(result *tools.ToolResult) string {
	if result.Extra != nil {
		if msg, ok := result.Extra["message"].(string); ok {
			return msg
		}
	}

	// Truncate output if too long (max 10 lines)
	output := result.Output
	lines := strings.Split(output, "\n")
	if len(lines) > 10 {
		lines = lines[:10]
		output = strings.Join(lines, "\n") + "\n... [output truncated]"
	}
	return output
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
	case "replace_lines":
		path := ""
		if p, ok := params["path"].(string); ok {
			path = p
		}
		msg = fmt.Sprintf("\n%s[Replacing lines] in: %s%s\n", ColorCyan, path, ColorReset)
	case "patch":
		path := ""
		if p, ok := params["path"].(string); ok {
			path = p
		}
		msg = fmt.Sprintf("\n%s[Patching] file: %s%s\n", ColorCyan, path, ColorReset)
	case "glob":
		pattern := ""
		if p, ok := params["pattern"].(string); ok {
			pattern = p
			if len(pattern) > 50 {
				pattern = pattern[:50] + "..."
			}
		}
		msg = fmt.Sprintf("\n%s[Searching] pattern: %s%s\n", ColorCyan, pattern, ColorReset)
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
		case "replace_lines":
			start := 0
			end := 0
			if s, ok := result.Extra["start"].(int); ok {
				start = s
			}
			if e, ok := result.Extra["end"].(int); ok {
				end = e
			}
			search := ""
			if s, ok := result.Extra["search"].(string); ok {
				search = s
				if len(search) > 20 {
					search = search[:20] + "..."
				}
			}
			if start > 0 {
				return fmt.Sprintf("%s[Success] replaced lines %d-%d%s\n", ColorGreen, start, end, ColorReset)
			}
			count := 0
			if c, ok := result.Extra["replacementsMade"].(int); ok {
				count = c
			}
			return fmt.Sprintf("%s[Success] replaced '%s' %d time(s)%s\n", ColorGreen, search, count, ColorReset)
		case "patch":
			hunks := 0
			if h, ok := result.Extra["patches_applied"].(int); ok {
				hunks = h
			}
			return fmt.Sprintf("%s[Success] applied %d hunk(s)%s\n", ColorGreen, hunks, ColorReset)
		case "glob":
			pattern := ""
			if p, ok := result.Extra["pattern"].(string); ok {
				pattern = p
			}
			matchCount := 0
			if mc, ok := result.Extra["matchesFound"].(int); ok {
				matchCount = mc
			}
			return fmt.Sprintf("%s[Success] found %d file(s) matching '%s'%s\n", ColorGreen, matchCount, pattern, ColorReset)
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
func buildSystemPrompt() string {
	// Get environment information
	envInfo := getEnvironmentInfo()

	return fmt.Sprintf(`You are a helpful coding assistant. You have access to the following tools.

%s

## Session Management
In interactive mode, you can use these commands:
- '/save [filename]' - Save the current conversation session (default: session.json)
- '/load <filename>' - Load a previous conversation session to continue where you left off

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
 7. replace_lines
    Description: Replace lines in a file. Supports two modes:
      - Line-number mode: Provide start and end line numbers with replacement content
      - Search-and-replace mode: Provide search text and replacement text (like replace_text)
    Parameters:
      - path (string, required): The path to the file
      - start (integer, required for line-number mode): Starting line number (1-indexed)
      - end (integer, required for line-number mode): Ending line number (1-indexed)
      - lines (string, required for line-number mode): Replacement lines (use \n for newlines)
      - search (string, required for search mode): Text pattern to find (exact match)
      - replace (string, required for search mode): Replacement text
      - count (integer, optional, search mode only): Number of occurrences to replace (default: 1, -1 for all)
    How to call: Use replace_lines for bulk line replacements by number, or for search-and-replace. Prefer replace_text for simple search-and-replace since it supports the "count" parameter.
    Example use case: Replacing a multi-line block of code with new content, or replacing all occurrences of a pattern across a file.
    Note: When using line-number mode, if start > end, the tool returns an error. If lines is empty, it effectively deletes those lines.


8. patch
   Description: Apply a unified diff patch to a file
   Parameters:
     - path (string, required): The path to the file to patch
     - diff (string, required): Unified diff content to apply
   How to call: Use the patch tool when you need to apply a unified diff to a file.
   Example use case: Applying code changes, fixing bugs, updating file content


9. glob
   Description: Search for files matching a glob pattern (e.g., "*.go", "src/**/*.ts", "**/test.js")
   Parameters:
     - pattern (string, required): Glob pattern to match (supports *, ?, **)
     - max_results (integer, optional): Maximum number of results to return (default: 100)
   How to call: Use glob to discover files in the codebase. Supports recursive matching with ** patterns.
   Example use case: Finding all Go files in a project with "go/**/*.go", or finding test files with "**/*_test.go"

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

// buildTools builds the tool definitions for the OpenAI API.
func buildTools() []inference.ToolDefinition {
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
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "replace_lines",
				Description: "Replace lines in a file. Supports two modes: line-number mode (start/end) or search-and-replace mode (search/replace)",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the file to modify",
						},
						"start": {
							Type:        "integer",
							Description: "Starting line number (1-indexed) for line-number mode",
						},
						"end": {
							Type:        "integer",
							Description: "Ending line number (1-indexed) for line-number mode",
						},
						"lines": {
							Type:        "string",
							Description: "Replacement lines for line-number mode (use \\n for newlines). Empty string effectively deletes the lines.",
						},
						"search": {
							Type:        "string",
							Description: "Text pattern to find (exact match, not regex) for search-and-replace mode",
						},
						"replace": {
							Type:        "string",
							Description: "Replacement text for search-and-replace mode",
						},
						"count": {
							Type:        "integer",
							Description: "Number of occurrences to replace in search-and-replace mode (default: 1, -1 for all)",
						},
					},
					Required: []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "patch",
				Description: "Apply a unified diff patch to a file",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "File path to patch",
						},
						"diff": {
							Type:        "string",
							Description: "Unified diff content to apply",
						},
					},
					Required: []string{"path", "diff"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "glob",
				Description: "Search for files matching a glob pattern (e.g., \"*.go\", \"src/**/*.ts\", \"**/test.js\")",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"pattern": {
							Type:        "string",
							Description: "Glob pattern to match (supports *, ?, ** for recursive matching)",
						},
						"max_results": {
							Type:        "integer",
							Description: "Maximum number of results to return (default: 100)",
						},
					},
					Required: []string{"pattern"},
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
