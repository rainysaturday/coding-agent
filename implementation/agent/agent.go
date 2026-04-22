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
	SystemPrompt string                     `json:"system_prompt"`
	Messages     []*inference.Message       `json:"messages"`
	Stats        *Stats                     `json:"stats"`
	Timestamp    string                     `json:"timestamp"`
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
	case "sub_agent":
		prompt := ""
		if p, ok := params["prompt"].(string); ok {
			prompt = p
			if len(prompt) > 50 {
				prompt = prompt[:50] + "..."
			}
		}
		msg = fmt.Sprintf("\n%s[Sub-agent] task: %s%s\n", ColorCyan, prompt, ColorReset)
	case "git_status":
		format := ""
		if f, ok := params["format"].(string); ok {
			format = f
		}
		msg = fmt.Sprintf("\n%s[Git] status (format: %s)%s\n", ColorCyan, format, ColorReset)
	case "git_diff":
		staged := false
		if s, ok := params["staged"].(bool); ok && s {
			staged = true
		}
		file := ""
		if f, ok := params["file"].(string); ok && f != "" {
			file = " file: " + f
		}
		mode := "unstaged"
		if staged {
			mode = "staged"
		}
		msg = fmt.Sprintf("\n%s[Git] diff (%s)%s%s\n", ColorCyan, mode, file, ColorReset)
	case "git_log":
		branches := ""
		if b, ok := params["branches"]; ok {
			branches = fmt.Sprintf("%v", b)
		}
		maxCount := 20
		if mc, ok := params["max_count"].(float64); ok {
			maxCount = int(mc)
		}
		msg = fmt.Sprintf("\n%s[Git] log (%d commits, branches: %s)%s\n", ColorCyan, maxCount, branches, ColorReset)
	case "git_show":
		ref := "HEAD"
		if r, ok := params["ref"].(string); ok {
			ref = r
		}
		path := ""
		if p, ok := params["path"].(string); ok {
			path = p
		}
		msg = fmt.Sprintf("\n%s[Git] show %s:%s%s\n", ColorCyan, ref, path, ColorReset)
	case "git_add":
		msg = fmt.Sprintf("\n%s[Git] add (staging files)%s\n", ColorCyan, ColorReset)
	case "git_commit":
		msg = fmt.Sprintf("\n%s[Git] commit%s\n", ColorCyan, ColorReset)
	case "git_branch":
		action := ""
		name := ""
		if a, ok := params["action"].(string); ok {
			action = a
		}
		if n, ok := params["name"].(string); ok {
			name = n
		}
		switch action {
		case "list":
			msg = fmt.Sprintf("\n%s[Git] list branches%s\n", ColorCyan, ColorReset)
		case "create":
			msg = fmt.Sprintf("\n%s[Git] create branch: %s%s\n", ColorCyan, name, ColorReset)
		case "checkout":
			msg = fmt.Sprintf("\n%s[Git] checkout branch: %s%s\n", ColorCyan, name, ColorReset)
		case "delete":
			msg = fmt.Sprintf("\n%s[Git] delete branch: %s%s\n", ColorCyan, name, ColorReset)
		case "rename":
			oldName := ""
			if o, ok := params["old_name"].(string); ok {
				oldName = o
			}
			newName := ""
			if n, ok := params["new_name"].(string); ok {
				newName = n
			}
			msg = fmt.Sprintf("\n%s[Git] rename branch: %s -> %s%s\n", ColorCyan, oldName, newName, ColorReset)
		case "set_upstream":
			msg = fmt.Sprintf("\n%s[Git] set upstream for: %s%s\n", ColorCyan, name, ColorReset)
		default:
			msg = fmt.Sprintf("\n%s[Git] branch (%s)%s\n", ColorCyan, action, ColorReset)
		}
	case "move_file":
		src := ""
		dst := ""
		if s, ok := params["source"].(string); ok {
			src = s
		}
		if d, ok := params["destination"].(string); ok {
			dst = d
		}
		msg = fmt.Sprintf("\n%s[Moving] '%s' -> '%s'%s\n", ColorCyan, src, dst, ColorReset)
	case "file_rename":
		src := ""
		dst := ""
		if s, ok := params["source"].(string); ok {
			src = s
		}
		if d, ok := params["destination"].(string); ok {
			dst = d
		}
		msg = fmt.Sprintf("\n%s[Rename] '%s' -> '%s'%s\n", ColorCyan, src, dst, ColorReset)
	case "copy_file":
		src := ""
		dst := ""
		if s, ok := params["source"].(string); ok {
			src = s
		}
		if d, ok := params["destination"].(string); ok {
			dst = d
		}
		msg = fmt.Sprintf("\n%s[Copying] '%s' -> '%s'%s\n", ColorCyan, src, dst, ColorReset)
	case "code_navigation":
		query := ""
		mode := "definitions"
		if q, ok := params["query"].(string); ok {
			query = q
		}
		if m, ok := params["mode"].(string); ok {
			mode = m
		}
		if len(query) > 40 {
			query = query[:40] + "..."
		}
		msg = fmt.Sprintf("\n%s[Code Navigation] %s: '%s'%s\n", ColorCyan, mode, query, ColorReset)
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
		case "sub_agent":
			exitCode := result.ExitCode
			if exitCode == 0 {
				output := result.Output
				lines := strings.Split(output, "\n")
				if len(lines) > 10 {
					lines = lines[:10]
					output = strings.Join(lines, "\n") + "\n... [output truncated]"
				}
				return fmt.Sprintf("%s[Success] sub-agent completed (exit code: %d)\nOutput:\n%s%s\n", ColorGreen, exitCode, output, ColorReset)
			}
			return fmt.Sprintf("%s[Success] sub-agent completed (exit code: %d)%s\n", ColorGreen, exitCode, ColorReset)
		case "git_status":
			staged := 0
			unstaged := 0
			untracked := 0
			if s, ok := result.Extra["stagedFiles"].(int); ok {
				staged = s
			}
			if u, ok := result.Extra["unstagedFiles"].(int); ok {
				unstaged = u
			}
			if u, ok := result.Extra["untrackedFiles"].(int); ok {
				untracked = u
			}
			parts := []string{}
			if staged > 0 {
				parts = append(parts, fmt.Sprintf("%d staged", staged))
			}
			if unstaged > 0 {
				parts = append(parts, fmt.Sprintf("%d unstaged", unstaged))
			}
			if untracked > 0 {
				parts = append(parts, fmt.Sprintf("%d untracked", untracked))
			}
			status := "no changes"
			if len(parts) > 0 {
				status = strings.Join(parts, ", ")
			}
			return fmt.Sprintf("%s[Success] %s%s\n", ColorGreen, status, ColorReset)
		case "git_diff":
			changedFiles := 0
			linesAdded := 0
			linesDeleted := 0
			if cf, ok := result.Extra["changedFiles"].(int); ok {
				changedFiles = cf
			}
			if la, ok := result.Extra["linesAdded"].(int); ok {
				linesAdded = la
			}
			if ld, ok := result.Extra["linesDeleted"].(int); ok {
				linesDeleted = ld
			}
			staged := ""
			if result.Extra["staged"] == true {
				staged = " (staged)"
			}
			return fmt.Sprintf("%s[Success] %d file(s) changed, +%d -%d%s%s\n", ColorGreen, changedFiles, linesAdded, linesDeleted, staged, ColorReset)
		case "git_log":
			commitCount := 0
			if cc, ok := result.Extra["commitCount"].(int); ok {
				commitCount = cc
			}
			branches := ""
			if b, ok := result.Extra["branches"].([]string); ok {
				branches = " " + strings.Join(b, ", ")
			}
			return fmt.Sprintf("%s[Success] %d commit(s) on branch(es)%s%s\n", ColorGreen, commitCount, branches, ColorReset)
		case "git_show":
			path := ""
			if p, ok := result.Extra["path"].(string); ok {
				path = p
			}
			ref := "HEAD"
			if r, ok := result.Extra["ref"].(string); ok {
				ref = r
			}
			contentLen := 0
			if cl, ok := result.Extra["contentLen"].(int); ok {
				contentLen = cl
			}
			return fmt.Sprintf("%s[Success] showed %s:%s (%d bytes)%s\n", ColorGreen, ref, path, contentLen, ColorReset)
		case "git_add":
			mode := ""
			if m, ok := result.Extra["mode"].(string); ok {
				if m == "update" {
					mode = "all tracked modified files"
				} else {
					files := []string{}
					if fs, ok := result.Extra["files"].([]string); ok {
						files = fs
					}
					mode = strings.Join(files, ", ")
				}
			}
			return fmt.Sprintf("%s[Success] staged %s%s\n", ColorGreen, mode, ColorReset)
		case "git_commit":
			amend := false
			if a, ok := result.Extra["amend"].(bool); ok {
				amend = a
			}
			if amend {
				return fmt.Sprintf("%s[Success] amended last commit%s\n", ColorGreen, ColorReset)
			}
			return fmt.Sprintf("%s[Success] committed changes%s\n", ColorGreen, ColorReset)
		case "git_branch":
			action := ""
			if a, ok := result.Extra["action"].(string); ok {
				action = a
			}
			switch action {
			case "list":
				localCount := 0
				if lc, ok := result.Extra["localCount"].(int); ok {
					localCount = lc
				}
				remoteCount := 0
				if rc, ok := result.Extra["remoteCount"].(int); ok {
					remoteCount = rc
				}
				currentBranch := ""
				if cb, ok := result.Extra["currentBranch"].(string); ok {
					currentBranch = cb
				}
				total := localCount + remoteCount
				msg := fmt.Sprintf("%s[Success] %d branch(es) listed%s", ColorGreen, total, ColorReset)
				if currentBranch != "" {
					msg += fmt.Sprintf(" (current: %s)", currentBranch)
				}
				return msg
			case "create":
				name := ""
				if n, ok := result.Extra["name"].(string); ok {
					name = n
				}
				return fmt.Sprintf("%s[Success] created branch '%s'%s\n", ColorGreen, name, ColorReset)
			case "checkout":
				created := false
				if c, ok := result.Extra["created"].(bool); ok {
					created = c
				}
				if created {
					name := ""
					if n, ok := result.Extra["name"].(string); ok {
						name = n
					}
					return fmt.Sprintf("%s[Success] created and checked out branch '%s'%s\n", ColorGreen, name, ColorReset)
				}
				newBranch := ""
				if nb, ok := result.Extra["newBranch"].(string); ok {
					newBranch = nb
				}
				return fmt.Sprintf("%s[Success] switched to branch '%s'%s\n", ColorGreen, newBranch, ColorReset)
			case "delete":
				forced := false
				if f, ok := result.Extra["forced"].(bool); ok {
					forced = f
				}
				name := ""
				if n, ok := result.Extra["name"].(string); ok {
					name = n
				}
				if forced {
					return fmt.Sprintf("%s[Success] force deleted branch '%s'%s\n", ColorGreen, name, ColorReset)
				}
				return fmt.Sprintf("%s[Success] deleted branch '%s'%s\n", ColorGreen, name, ColorReset)
			case "rename":
				oldName := ""
				if on, ok := result.Extra["old_name"].(string); ok {
					oldName = on
				}
				newName := ""
				if nn, ok := result.Extra["new_name"].(string); ok {
					newName = nn
				}
				return fmt.Sprintf("%s[Success] renamed '%s' -> '%s'%s\n", ColorGreen, oldName, newName, ColorReset)
			case "set_upstream":
				name := ""
				if n, ok := result.Extra["name"].(string); ok {
					name = n
				}
				upstream := ""
				if u, ok := result.Extra["upstream"].(string); ok {
					upstream = u
				}
				return fmt.Sprintf("%s[Success] set upstream for '%s' to '%s'%s\n", ColorGreen, name, upstream, ColorReset)
			default:
				return fmt.Sprintf("%s[Success] branch operation completed%s\n", ColorGreen, ColorReset)
			}
		case "move_file":
			source := ""
			destination := ""
			if s, ok := result.Extra["source"].(string); ok {
				source = s
			}
			if d, ok := result.Extra["destination"].(string); ok {
				destination = d
			}
			return fmt.Sprintf("%s[Success] moved '%s' -> '%s'%s\n", ColorGreen, source, destination, ColorReset)
		case "file_rename":
			source := ""
			destination := ""
			if s, ok := result.Extra["source"].(string); ok {
				source = s
			}
			if d, ok := result.Extra["destination"].(string); ok {
				destination = d
			}
			if refs, ok := result.Extra["referencesUpdated"].([]string); ok && len(refs) > 0 {
				return fmt.Sprintf("%s[Success] renamed '%s' -> '%s' (updated %d reference(s))%s\n", ColorGreen, source, destination, len(refs), ColorReset)
			}
			return fmt.Sprintf("%s[Success] renamed '%s' -> '%s'%s\n", ColorGreen, source, destination, ColorReset)
		case "copy_file":
			source := ""
			destination := ""
			if s, ok := result.Extra["source"].(string); ok {
				source = s
			}
			if d, ok := result.Extra["destination"].(string); ok {
				destination = d
			}
			if overwritten, ok := result.Extra["overwritten"].(bool); ok && overwritten {
				return fmt.Sprintf("%s[Success] overwrote '%s' with copy of '%s'%s\n", ColorGreen, destination, source, ColorReset)
			}
			return fmt.Sprintf("%s[Success] copied '%s' -> '%s'%s\n", ColorGreen, source, destination, ColorReset)
		case "code_navigation":
			query := ""
			mode := "definitions"
			if q, ok := result.Extra["query"].(string); ok {
				query = q
			}
			if m, ok := result.Extra["mode"].(string); ok {
				mode = m
			}
			matches := 0
			if mc, ok := result.Extra["matchesFound"].(int); ok {
				matches = mc
			}
			return fmt.Sprintf("%s[Success] %s: found %d result(s) for '%s'%s\n", ColorGreen, mode, matches, query, ColorReset)
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

10. sub_agent
    Description: Spawn a parallel sub-agent to work on a delegated task. The sub-agent runs the coding-agent in one-shot mode with the provided prompt.
    Parameters:
      - prompt (string, required): The task/prompt to delegate to the sub-agent
      - timeout (integer, optional): Maximum execution time in seconds (default: 300)
    How to call: Use sub_agent when a task can be delegated to a parallel agent. This is useful for multi-step tasks where parts can be worked on independently.
    Example use case: Deletting code review of a specific file, researching a specific topic, or generating code for a separate module.
    Note: The sub-agent runs with the same working directory and has access to the same tools (bash, read_file, write_file, etc.).

11. git_status
    Description: Check the git status of the repository, showing staged, unstaged, and untracked files.
    Parameters:
      - format (string, optional): Output format - 'short', 'long', 'porcelain' (default: short)
      - include_untracked (boolean, optional): Include untracked files (default: true)
    How to call: Use git_status to check what changes exist in the working directory before making or committing changes.
    Example use case: Checking which files have been modified before committing, verifying staged changes

12. git_diff
    Description: Show changes between commits, commit and working tree, etc. Shows unstaged changes by default.
    Parameters:
      - staged (boolean, optional): Show staged changes instead of unstaged (default: false)
      - file (string, optional): Show diff for a specific file
      - max_lines (integer, optional): Maximum output lines (default: 200)
    How to call: Use git_diff to review changes before committing, or to verify specific file changes.
    Example use case: Reviewing unstaged changes, checking what's staged for commit, viewing changes to a specific file

13. git_log
    Description: Show commit history with subject, body, author, and date information.
    Parameters:
      - branches (array, optional): Branch names to show (default: HEAD)
      - max_count (integer, optional): Maximum commits to show (default: 20)
      - format (string, optional): 'short', 'medium', 'full', 'fuller', 'raw', 'oneline' (default: medium)
    How to call: Use git_log to review recent commits, understand project history, or find a specific commit.
    Example use case: Checking recent commit history, finding when a bug was introduced

14. git_show
    Description: Show the contents of a file at a specific git ref (commit, branch, tag, or HEAD).
    Parameters:
      - ref (string, optional): Git reference (commit hash, branch, tag, HEAD) (default: HEAD)
      - path (string, required): Path to the file in the repository
      - max_lines (integer, optional): Maximum output lines (default: 200)
    How to call: Use git_show to view file content at a previous commit, compare versions, or view a file from a branch.
    Example use case: Viewing how a file looked before recent changes, comparing HEAD to a branch

15. git_add
    Description: Stage files for commit. Can stage specific files or all tracked modified files.
    Parameters:
      - files (array, optional): List of file paths to stage. If omitted, stages all tracked modified files.
    How to call: Use git_add to stage files before committing. Without arguments, stages all tracked modified files.
    Example use case: Staging specific files for commit, staging all changes after reviewing them

16. git_commit
    Description: Commit staged changes with a descriptive message. Use this after git_add to save changes to the repository.
    Parameters:
      - message (string, required): Commit message describing the changes made
      - amend (boolean, optional): Amend the most recent commit instead of creating a new one (default: false)
    How to call: Use git_commit after git_add to save changes. Provide a clear, descriptive commit message.
    Example use case: "git_commit message='Add new authentication module'" or "git_commit message='Fix login bug' amend=true"

16. git_branch
    Description: Manage git branches with six actions: list, create, checkout, delete, rename, and set_upstream.
    Parameters:
      - action (string, required): One of: 'list', 'create', 'checkout', 'delete', 'rename', 'set_upstream'
      - name (string, required for create/checkout/delete/set_upstream): The branch name to operate on
      - old_name (string, required for rename): The current branch name
      - new_name (string, required for rename): The new branch name
      - start_point (string, optional): Base branch to create from (for create/checkout)
      - create (boolean, optional): Create branch if it doesn't exist (for checkout)
      - force (boolean, optional): Force delete even if not merged (for delete)
      - remote (string, optional): Remote name for set_upstream (e.g., 'origin')
      - branch (string, optional): Remote branch name for set_upstream
    How to call: Use git_branch to manage branches. List all branches with action='list', create a new branch with action='create' name='feature-xyz', or switch branches with action='checkout' name='main'.
    Example use case: git_branch(action='list'), git_branch(action='create', name='feature/auth'), git_branch(action='checkout', name='feature/auth'), git_branch(action='delete', name='feature/old')

17. git_stash
    Description: Manage git stashes: list, save (push), pop, apply, and drop. Use stash to temporarily set aside uncommitted changes.
    Parameters:
      - action (string, required): One of: 'list', 'save', 'pop', 'apply', 'drop'
      - message (string, optional): Stash message/description (for save action)
      - stash_ref (string, optional): Stash reference like 'stash@{0}' (for pop, apply, drop). Default is the most recent stash.
      - include_untracked (boolean, optional): Include untracked files in the stash (for save action, default: false)
      - include_ignored (boolean, optional): Include ignored files in the stash (for save action, default: false)
      - pathspec (string, optional): Restrict stashing to specific file(s) or directory (for save action)
    How to call: Use git_stash to temporarily save uncommitted changes. List stashes with action='list', save changes with action='save', or restore with action='pop'.
    Example use case: git_stash(action='list'), git_stash(action='save', message='WIP auth changes'), git_stash(action='pop'), git_stash(action='drop', stash_ref='stash@{1}')

18. git_tag
    Description: Manage git tags: list, create, delete, and show. Tags mark specific points in git history (e.g., release versions).
    Parameters:
      - action (string, required): One of: 'list', 'create', 'delete', 'show'
      - name (string, required for create/delete/show): The tag name
      - pattern (string, optional): Glob pattern to filter tag list (e.g., 'v1.*', 'release-*')
      - message (string, optional): Tag message/description (for create action, creates an annotated tag)
      - annotated (boolean, optional): Create annotated tag (true) or lightweight tag (false). Default: true for annotated.
      - force (boolean, optional): Force create/delete even if tag already exists / delete failed (default: false)
      - max_results (integer, optional): Maximum tags to return when listing (default: 50)
      - sort (string, optional): Sort order for listing: 'version' (default), 'committerdate', or 'creatordate'
    How to call: Use git_tag to create release tags, list existing tags, delete outdated tags, or inspect tag details.
    Example use case: git_tag(action='list'), git_tag(action='create', name='v1.2.0', message='Release 1.2.0'), git_tag(action='delete', name='v1.2.0'), git_tag(action='show', name='v1.2.0')

19. find
    Description: Search file contents for matches to a regex pattern. Returns file paths, line numbers, and matching content.
    Parameters:
      - pattern (string, required): Regular expression pattern to search for (Go regex syntax)
      - paths (array, optional): Glob patterns to restrict search (e.g., ['*.go', 'src/**']). If omitted, searches all files.
      - case_insensitive (boolean, optional): Ignore case when matching (default: false)
      - max_results (integer, optional): Maximum results to return (default: 50)
    How to call: Use find to search for patterns across the codebase. Returns structured results with file, line, and content.
    Example use case: find(pattern='func.*HandleRequest', paths=['*.go']) or find(pattern='TODO', case_insensitive=true)

19. web_fetch
    Description: Fetch content from a URL using HTTP GET. Useful for looking up documentation, API specs, or any publicly accessible web resource.
    Parameters:
      - url (string, required): The URL to fetch (http or https only)
      - timeout (integer, optional): Maximum time in seconds to wait for the request (default: 30)
      - max_size (integer, optional): Maximum response size in bytes (default: 10240)
    How to call: Use web_fetch to retrieve content from the web, such as documentation or API references.
    Example use case: web_fetch(url='https://example.com/docs')

20. move_file
    Description: Move or rename a file from source to destination path. Creates parent directories for the destination if they don't already exist.
    Parameters:
      - source (string, required): Source file path to move/rename
      - destination (string, required): Destination file path (new location and/or name)
    How to call: Use move_file when you need to relocate a file within the filesystem or rename it.
    Example use case: move_file(source='old_name.go', destination='new_name.go') or move_file(source='temp/foo.txt', destination='docs/foo.txt')

20. copy_file
    Description: Copy a file from source to destination path. Creates parent directories for the destination if they don't already exist. Does not remove the source file.
    Parameters:
      - source (string, required): Source file path to copy from
      - destination (string, required): Destination file path (new location and/or name)
      - overwrite (boolean, optional): Allow overwriting existing destination file (default: false)
    How to call: Use copy_file when you need to duplicate a file within the filesystem. The source file is preserved.
    Example use case: copy_file(source='config.go', destination='config_backup.go') or copy_file(source='app.go', destination='dist/app.go') overwrite=true

20. file_rename
    Description: Rename or move a file and automatically update all code references (imports, includes, requires, etc.) across the codebase. This tool combines file renaming with intelligent reference detection and replacement.
    Parameters:
      - source (string, required): Source file path to rename/move
      - destination (string, required): Destination file path (new location and/or name)
      - search_paths (array, optional): Glob patterns or paths to limit where references are searched. If omitted, searches all code files recursively from the current directory.
    How to call: Use file_rename when you need to rename a file AND update all import/reference statements that point to it. This is the preferred tool over move_file when references need updating.
    Example use case: file_rename(source='utils/helper.go', destination='utils/helpers.go') to rename and update imports, file_rename(source='src/old_module.py', destination='src/new_module.py', search_paths=['src/**/*.py']) to limit search scope.
    Note: The tool automatically detects and replaces references in common source file types (Go, Python, JS/TS, C/C++, etc.).

21. list_dir
    Description: List directory contents with metadata (file type, size, modification time). Supports recursive listing and hidden file filtering.
    Parameters:
      - path (string, optional): Directory path to list (default: current directory)
      - recursive (boolean, optional): List contents recursively (default: false)
      - max_results (integer, optional): Maximum entries to return (default: 100)
      - show_hidden (boolean, optional): Include hidden files (dotfiles) (default: false)
    How to call: Use list_dir to explore directory structure and find files.
    Example use case: list_dir(path='src/') or list_dir(path='src/', recursive=true, show_hidden=true)

22. run_tests
    Description: Execute tests for the current project and report structured results. Auto-detects project type (Go, Node.js, Python) and runs the appropriate test command.
    Parameters:
      - command (string, optional): Custom test command to run (e.g., 'go test ./pkg/...', 'npm test -- --coverage'). If omitted, auto-detects from project files.
      - args (array, optional): Additional arguments for the test command (e.g., ['-v', '-run', 'TestFoo'] or '-v').
      - timeout (integer, optional): Maximum execution time in seconds (default: 60).
    How to call: Use run_tests to verify code changes by running the project's test suite. Returns structured results including exit code, pass/fail status, and a summary of failures.
    Example use case: run_tests() to run all tests, or run_tests(command='go test ./pkg/...', args=['-v']) to run specific packages with verbose output.
    Note: Auto-detection runs 'go test ./...' for Go projects, 'npm test' for Node.js projects, and 'python -m pytest' for Python projects.

23. project_tree
    Description: Generate a visual directory tree showing the project structure with file metadata. Displays tree-drawing characters (├──, └──, │), file type icons (📁 for directories, 🐹 for Go, 🐍 for Python, etc.), file sizes, and symlink indicators.
    Parameters:
      - path (string, optional): Directory path to show (default: current directory)
      - max_depth (integer, optional): Maximum depth to traverse (default: 3)
      - show_hidden (boolean, optional): Include hidden files (dotfiles) (default: true)
      - max_entries (integer, optional): Maximum entries to show per level (default: 100)
    How to call: Use project_tree to get a quick overview of the project structure without navigating through list_dir repeatedly.
    Example use case: project_tree() to see current directory, project_tree(path='src/', max_depth=2) to see a specific subdirectory.
    Note: The tree uses Unicode box-drawing characters for clean visual presentation. File icons are based on file extensions.

24. code_navigation
    Description: Navigate code to find definitions, references, or implementations of a symbol across the codebase. Supports Go, Python, JavaScript/TypeScript, Rust, Java, and more.
    Parameters:
      - query (string, required): Symbol name to search for (function name, class name, type, variable, etc.)
      - mode (string, optional): Search mode - 'definitions' (find where defined), 'references' (find all usages), 'implementations' (find interface implementations). Default: definitions
      - file_type (string, optional): Limit search to a specific language (e.g., 'go', 'python', 'typescript', 'rust'). If omitted, searches all source files.
      - paths (array, optional): Directories to search within. If omitted, searches from current directory.
      - max_results (integer, optional): Maximum results to return (default: 30)
    How to call: Use code_navigation to understand code structure, find where functions are defined, track usage of a symbol, or find which types implement an interface.
    Example use case: code_navigation(query='HandleRequest', mode='definitions') to find function definitions, code_navigation(query='User', mode='references') to find all usages of 'User', code_navigation(query='ReadWriter', mode='implementations') to find types implementing ReadWriter.
    Note: This tool uses grep-based pattern matching optimized for different programming languages. For the best results, specify the file_type when searching in a specific project.

25. check_links
    Description: Scan Markdown and HTML files for broken links (both internal file links and external URLs). Detects relative paths, image references, and HTTP/HTTPS URLs. Returns structured results with summaries of valid and broken links.
    Parameters:
      - paths (array, optional): Glob patterns to restrict search to specific files/directories (e.g., ['docs/**/*.md', 'README.md']). If omitted, searches all .md and .html files recursively from the current directory.
      - file_types (array, optional): File extensions to scan for links. Default: ['.md', '.html', '.htm']. Can be a list of extensions with or without the leading dot.
      - timeout (integer, optional): Timeout in seconds for checking external URLs (default: 10).
    How to call: Use check_links to verify that all links in documentation and web files are valid. Useful during code reviews or documentation updates.
    Example use case: check_links() to scan all .md and .html files, check_links(paths=['docs/**/*.md']) to check only documentation files, check_links(file_types=['.md'], timeout=15) to scan only markdown files with a longer timeout.
    Note: Internal links are resolved relative to the file's directory. External links are verified via HTTP HEAD request (falls back to GET). Rate-limited to 5 simultaneous requests.

26. json_transformer
    Description: Transform JSON data with multiple operations. Supports extract, set, merge, validate, format, convert_to_yaml, and convert_to_env commands.
    Parameters:
      - command (string, required): Operation to perform: 'extract', 'set', 'merge', 'validate', 'format', 'convert_to_yaml', 'convert_to_env'
      - file_path (string, optional): Path to a JSON file to operate on (alternative to json_string)
      - json_string (string, optional): Raw JSON string to operate on (alternative to file_path)
      - path (string, optional): JSON path for extract/set operations using dot notation (e.g., '.foo.bar' or 'foo.bar[0]'). Required for 'extract' and 'set' commands.
      - value (string, optional): Value to set for 'set' command. Can be a JSON value, number, boolean, or 'null'.
      - files (array, optional): List of JSON file paths to merge (for 'merge' command).
      - json_strings (array, optional): List of raw JSON strings to merge (for 'merge' command).
      - required_fields (array, optional): List of required JSON field paths to validate (for 'validate' command).
      - indent (integer, optional): Number of spaces for formatting (default: 2).
    How to call: Use json_transformer to work with JSON data. Extract a specific field, set a value at a path, merge multiple JSON sources, validate structure, format for readability, or convert between JSON and YAML/ENV formats.
    Example use case: json_transformer(command='extract', file_path='config.json', path='.database.host') to extract a field, json_transformer(command='set', file_path='config.json', path='.database.port', value='5432') to set a value, json_transformer(command='format', file_path='data.json', indent=4) to beautify JSON, json_transformer(command='convert_to_yaml', file_path='config.json') to convert to YAML.

27. project_diagnostics
    Description: Scan a codebase for common issues and quality problems. Detects TODO/FIXME/HACK/WARN/XXX markers, empty files, large files (>500 lines), hardcoded secrets/keys, and more. Returns a structured report with severity levels and recommendations.
    Parameters:
      - paths (array, optional): Paths to scan (glob patterns or directory paths). If omitted, scans the current directory.
      - max_depth (integer, optional): Maximum directory depth to traverse (default: 10).
      - mode (string, optional): Scan mode: 'full' (default, all checks) or 'basic' (TODOs and empty files only).
    How to call: Use project_diagnostics to get a quick overview of code quality issues in a project. Useful before starting refactoring work, code reviews, or when onboarding to a new codebase.
    Example use case: project_diagnostics() to scan the current directory, project_diagnostics(paths=['src/']) to scan only the source directory, project_diagnostics(mode='full') for a comprehensive scan.
    Note: Checks include TODO/FIXME/HACK/WARN/XXX markers, empty files, large files, hardcoded secrets, and more. Severity levels: low, medium, high, critical.

28. run_lint
    Description: Run linters for the current project and report structured results. Auto-detects project type (Go, Python, Node.js, Shell) and runs appropriate linters (gofmt, go vet, flake8, pylint, eslint, shellcheck).
    Parameters:
      - command (string, optional): Custom lint command to run (e.g., 'go vet ./...', 'flake8 src/', 'eslint .'). If omitted, auto-detects from project files.
      - args (array, optional): Additional arguments for the lint command (e.g., ['-v', '--max-line-length=120'] or '-v').
      - timeout (integer, optional): Maximum execution time in seconds (default: 60).
    How to call: Use run_lint to check code for style issues, potential bugs, and quality problems. Returns structured results including exit code, pass/fail status, and a summary of issues.
    Example use case: run_lint() to auto-detect and lint the current project, run_lint(command='go vet ./...') to run go vet specifically, run_lint(command='eslint .', args=['--fix']) to auto-fix ESLint issues.
    Note: Auto-detection runs 'go vet ./...' or 'gofmt -l .' for Go projects, 'flake8 .' or 'pylint .' for Python projects, 'eslint .' for Node.js projects.

29. process_management
    Description: Manage running processes and check system resources. Supports four actions: process_list, process_kill, port_check, and system_info.
    Parameters:
      - action (string, required): Action to perform: 'process_list', 'process_kill', 'port_check', or 'system_info'
      - filter (string, optional): Regex pattern to filter process names (for process_list)
      - user (string, optional): Filter by username (for process_list, Linux only)
      - limit (integer, optional): Maximum number of results (for process_list, default: 50)
      - sort (string, optional): Sort by 'pid', 'cpu', or 'memory' (for process_list, default: 'pid')
      - pid (integer, optional): Process ID to kill (for process_kill)
      - name (string, optional): Process name to kill (for process_kill)
      - force (boolean, optional): Use SIGKILL instead of SIGTERM (for process_kill, default: false)
      - port (integer, optional): Port number to check (for port_check)
      - protocol (string, optional): Protocol to check: 'tcp' or 'udp' (for port_check, default: 'tcp')
      - format (string, optional): Output format: 'short' or 'detailed' (for system_info, default: 'short')
    How to call: Use process_management when you need to manage running processes, check if ports are in use, or view system resource usage.
    Example use case: process_management(action='process_list', filter='python') to find Python processes, process_management(action='port_check', port=8080) to check if port 8080 is in use, process_management(action='system_info') to view CPU/memory/disk usage.

30. env_var
    Description: Manage environment variables. Supports five actions: get (read a variable), set (set a variable), unset (remove a variable), list (list all or filtered variables), and source (load variables from a .env file).
    Parameters:
      - action (string, required): Action to perform: 'get', 'set', 'unset', 'list', or 'source'
      - name (string, optional): Environment variable name (required for get, set, unset)
      - value (string, optional): Value to set (required for set)
      - prefix (string, optional): Filter environment variables by prefix (for list)
      - show_all (boolean, optional): Include empty/unset variables in list output (for list, default: false)
      - path (string, optional): Path to .env file to source (required for source)
      - overwrite (boolean, optional): Overwrite existing variables when sourcing .env file (for source, default: false)
    How to call: Use env_var to read, set, or manage environment variables. Useful for configuring tools, checking configuration, or loading environment files.
    Example use case: env_var(action='get', name='HOME') to read the HOME variable, env_var(action='set', name='MY_VAR', value='hello') to set a variable, env_var(action='list', prefix='GOPATH') to list GOPATH-related variables, env_var(action='source', path='.env', overwrite=true) to load a .env file.

31. git_merge
    Description: Manage git merge operations with five actions: merge (standard merge), abort (abort in-progress merge), status (check merge status), squash (squash merge), and merge_pr (merge a GitHub pull request).
    Parameters:
      - action (string, required): One of: 'merge', 'abort', 'status', 'squash', 'merge_pr'
      - source (string, optional): Source branch to merge (required for merge and squash actions)
      - target (string, optional): Target branch to merge into (default: current branch)
      - commit_message (string, optional): Custom commit message (for merge and squash actions)
      - github_token (string, optional): GitHub API token for merge_pr action (also reads GITHUB_TOKEN env var)
      - repo (string, optional): Repository in 'owner/repo' format (required for merge_pr action)
      - pr_number (integer, optional): Pull request number to merge (required for merge_pr action)
      - merge_method (string, optional): Merge method for PR: 'merge', 'squash', or 'rebase' (for merge_pr action only)
    How to call: Use git_merge for branch merging operations. Merge a branch with action='merge', check merge conflicts with action='status', abort a bad merge with action='abort', squash all commits into one with action='squash', or merge a GitHub PR with action='merge_pr'.
    Example use case: git_merge(action='merge', source='feature/auth'), git_merge(action='status') to check for conflicts, git_merge(action='abort') to cancel a merge, git_merge(action='squash', source='feature/new-api'), git_merge(action='merge_pr', pr_number=42, repo='owner/myproject', merge_method='squash')

32. generate_docs
    Description: Generate documentation for code files. Auto-detects language from file extension (Go, Python, JavaScript/TypeScript, Java, Rust, C/C++, Ruby, PHP, C#, Swift, Kotlin) and supports markdown or inline docstring output formats.
    Parameters:
      - path (string, required): Path to a file or directory to generate documentation for
      - format (string, optional): Output format - 'markdown' (default) or 'inline' (docstrings/comments)
      - detail (string, optional): Detail level - 'basic' (signatures only) or 'detailed' (default, includes comments and fields)
      - include_comments (boolean, optional): Include source comments in documentation (default: true)
    How to call: Use generate_docs to create documentation for source files or entire directories. For markdown format, returns structured docs with type and function sections. For inline format, returns code with generated comments.
    Example use case: generate_docs(path='src/main.go'), generate_docs(path='src/', format='markdown', detail='detailed'), generate_docs(path='app.py', format='inline')

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
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "sub_agent",
				Description: "Spawn a parallel sub-agent to work on a delegated task. The sub-agent runs the coding-agent in one-shot mode with the provided prompt.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"prompt": {
							Type:        "string",
							Description: "The task/prompt to delegate to the sub-agent",
						},
						"timeout": {
							Type:        "integer",
							Description: "Maximum execution time in seconds (default: 300)",
						},
					},
					Required: []string{"prompt"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "git_status",
				Description: "Check the git status of the repository (staged, unstaged, and untracked files)",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"format": {
							Type:        "string",
							Description: "Output format: 'short', 'long', 'porcelain', or 'porcelain=v2' (default: short)",
						},
						"include_untracked": {
							Type:        "boolean",
							Description: "Include untracked files in output (default: true)",
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
				Description: "Show changes between commits, commit and working tree, etc. Can show staged (--staged=true) or unstaged changes",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"staged": {
							Type:        "boolean",
							Description: "Show staged changes (git diff --cached). Default: false (shows unstaged changes)",
						},
						"file": {
							Type:        "string",
							Description: "Show diff for a specific file path",
						},
						"max_lines": {
							Type:        "integer",
							Description: "Maximum number of output lines to return (default: 200)",
						},
					},
					Required: []string{},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "git_log",
				Description: "Show commit history with optional branch and count filters",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"branches": {
							Type:        "array",
							Description: "Branch names to show log for (default: ['HEAD'])",
						},
						"max_count": {
							Type:        "integer",
							Description: "Maximum number of commits to show (default: 20)",
						},
						"format": {
							Type:        "string",
							Description: "Output format: 'short', 'medium', 'full', 'fuller', 'raw', 'oneline' (default: medium)",
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
				Description: "Show the contents of a file at a specific commit/ref in git",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"ref": {
							Type:        "string",
							Description: "Git ref (commit hash, branch name, tag, or HEAD) (default: HEAD)",
						},
						"path": {
							Type:        "string",
							Description: "Path to the file within the repository",
						},
						"max_lines": {
							Type:        "integer",
							Description: "Maximum number of output lines to return (default: 200)",
						},
					},
					Required: []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "git_add",
				Description: "Stage files for commit. Can stage specific files or all tracked modified files",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"files": {
							Type:        "array",
							Description: "List of file paths to stage. If not provided, stages all tracked modified files (git add -u)",
						},
					},
					Required: []string{},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "git_commit",
				Description: "Commit staged changes with a descriptive message. Use this after git_add to save changes to the repository.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"message": {
							Type:        "string",
							Description: "Commit message describing the changes",
						},
						"amend": {
							Type:        "boolean",
							Description: "Amend the most recent commit instead of creating a new one (default: false)",
						},
					},
					Required: []string{},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "git_branch",
				Description: "Manage git branches: list, create, checkout, delete, rename, and set upstream tracking.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"action": {
							Type:        "string",
							Description: "Branch action: 'list', 'create', 'checkout', 'delete', 'rename', or 'set_upstream'",
						},
						"name": {
							Type:        "string",
							Description: "Branch name (required for create, checkout, delete, set_upstream)",
						},
						"old_name": {
							Type:        "string",
							Description: "Current branch name (required for rename)",
						},
						"new_name": {
							Type:        "string",
							Description: "New branch name (required for rename)",
						},
						"start_point": {
							Type:        "string",
							Description: "Base branch to create from (for create and checkout with create=true)",
						},
						"create": {
							Type:        "boolean",
							Description: "Create the branch if it doesn't exist (for checkout action)",
						},
						"force": {
							Type:        "boolean",
							Description: "Force delete even if not merged (for delete action)",
						},
						"remote": {
							Type:        "string",
							Description: "Remote name for set_upstream (e.g., 'origin')",
						},
						"branch": {
							Type:        "string",
							Description: "Remote branch name for set_upstream",
						},
					},
					Required: []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "git_stash",
				Description: "Manage git stashes: list, save (push), pop, apply, and drop. Use stash to temporarily set aside uncommitted changes.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"action": {
							Type:        "string",
							Description: "Stash action: 'list', 'save', 'pop', 'apply', or 'drop'",
						},
						"message": {
							Type:        "string",
							Description: "Stash message/description (for save action)",
						},
						"stash_ref": {
							Type:        "string",
							Description: "Stash reference like 'stash@{0}' (for pop, apply, drop). Default is the most recent stash.",
						},
						"include_untracked": {
							Type:        "boolean",
							Description: "Include untracked files in the stash (for save action, default: false)",
						},
						"include_ignored": {
							Type:        "boolean",
							Description: "Include ignored files in the stash (for save action, default: false)",
						},
						"pathspec": {
							Type:        "string",
							Description: "Restrict stashing to specific file(s) or directory (for save action)",
						},
					},
					Required: []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "git_tag",
				Description: "Manage git tags: list, create, delete, and show. Tags mark specific points in git history (e.g., release versions).",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"action": {
							Type:        "string",
							Description: "Tag action: 'list', 'create', 'delete', or 'show'",
						},
						"name": {
							Type:        "string",
							Description: "Tag name (required for create/delete/show actions)",
						},
						"pattern": {
							Type:        "string",
							Description: "Glob pattern to filter tags when listing (e.g., 'v1.*', 'release-*')",
						},
						"message": {
							Type:        "string",
							Description: "Tag message/description (for create action; creates an annotated tag)",
						},
						"annotated": {
							Type:        "boolean",
							Description: "Create annotated tag (true) or lightweight tag (false). Default: true for annotated.",
						},
						"force": {
							Type:        "boolean",
							Description: "Force create/delete even if tag already exists or delete fails (default: false)",
						},
						"max_results": {
							Type:        "integer",
							Description: "Maximum tags to return when listing (default: 50)",
						},
						"sort": {
							Type:        "string",
							Description: "Sort order for listing: 'version' (default), 'committerdate', or 'creatordate'",
						},
					},
					Required: []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "find",
				Description: "Search file contents for matches to a regex pattern. Returns file paths, line numbers, and matching content.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"pattern": {
							Type:        "string",
							Description: "Regular expression pattern to search for (Go regex syntax)",
						},
						"paths": {
							Type:        "array",
							Description: "Glob patterns to restrict search to specific files/directories (e.g., ['*.go', 'src/**']). If not provided, searches all files recursively.",
						},
						"case_insensitive": {
							Type:        "boolean",
							Description: "Ignore case when matching (default: false)",
						},
						"max_results": {
							Type:        "integer",
							Description: "Maximum number of results to return (default: 50)",
						},
					},
					Required: []string{"pattern"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "web_fetch",
				Description: "Fetch content from a URL using HTTP GET. Useful for looking up documentation, API specs, or any publicly accessible web resource.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"url": {
							Type:        "string",
							Description: "The URL to fetch (http or https only)",
						},
						"timeout": {
							Type:        "integer",
							Description: "Maximum time in seconds to wait for the request (default: 30)",
						},
						"max_size": {
							Type:        "integer",
							Description: "Maximum response size in bytes (default: 10240)",
						},
					},
					Required: []string{"url"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "move_file",
				Description: "Move or rename a file from source to destination path. Creates parent directories for destination if they don't exist.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"source": {
							Type:        "string",
							Description: "Source file path to move/rename",
						},
						"destination": {
							Type:        "string",
							Description: "Destination file path (new location and/or name)",
						},
					},
					Required: []string{"source", "destination"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "file_rename",
				Description: "Rename or move a file and automatically update all code references (imports, includes, requires, etc.) across the codebase. Combines file system rename with reference detection and replacement.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"source": {
							Type:        "string",
							Description: "Source file path to rename/move",
						},
						"destination": {
							Type:        "string",
							Description: "Destination file path (new location and/or name)",
						},
						"search_paths": {
							Type:        "array",
							Description: "Glob patterns or paths to limit where references are searched. If omitted, searches all code files recursively.",
						},
					},
					Required: []string{"source", "destination"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "copy_file",
				Description: "Copy a file from source to destination path. Creates parent directories for destination if they don't exist. Does not remove the source file.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"source": {
							Type:        "string",
							Description: "Source file path to copy from",
						},
						"destination": {
							Type:        "string",
							Description: "Destination file path (new location and/or name)",
						},
						"overwrite": {
							Type:        "boolean",
							Description: "Allow overwriting existing destination file (default: false)",
						},
					},
					Required: []string{"source", "destination"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "list_dir",
				Description: "List directory contents with metadata including file type, size, and modification time. Supports recursive listing.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Directory path to list (default: current directory)",
						},
						"recursive": {
							Type:        "boolean",
							Description: "List directory contents recursively (default: false)",
						},
						"max_results": {
							Type:        "integer",
							Description: "Maximum number of entries to return (default: 100)",
						},
						"show_hidden": {
							Type:        "boolean",
							Description: "Include hidden files (dotfiles) in output (default: false)",
						},
					},
					Required: []string{},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "delete_file",
				Description: "Delete a file from the filesystem. Removes the specified file permanently. Does not work on directories.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the file to delete",
						},
					},
					Required: []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "scaffold",
				Description: "Generate code from built-in templates. Templates support variable substitution with Go template syntax. Available templates: go_struct, go_handler, go_service, python_class, python_dataclass, proto_message, openapi_schema, go_test",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"template": {
							Type:        "string",
							Description: "Template name to use (e.g., 'go_struct', 'go_handler', 'python_class', 'proto_message')",
						},
						"name": {
							Type:        "string",
							Description: "Name for the generated entity (struct, class, etc.)",
						},
						"package": {
							Type:        "string",
							Description: "Package name for Go templates or module name",
						},
						"description": {
							Type:        "string",
							Description: "Description of the generated entity",
						},
						"fields": {
							Type:        "array",
							Description: "Fields for the generated entity. Each field is an object with 'name', 'type', and optional 'json_tag' fields",
						},
						"method": {
							Type:        "string",
							Description: "HTTP method for go_handler template (GET, POST, etc.)",
						},
						"body": {
							Type:        "string",
							Description: "Custom method body for go_handler or go_service templates",
						},
						"request_type": {
							Type:        "string",
							Description: "Request type name for go_handler template",
						},
						"response_type": {
							Type:        "string",
							Description: "Response type name for go_handler template",
						},
						"method_name": {
							Type:        "string",
							Description: "Method name for go_service template",
						},
						"go_package": {
							Type:        "string",
							Description: "Full Go import path for proto_message template",
						},
						"plural_name": {
							Type:        "string",
							Description: "Plural name for openapi_schema template",
						},
					},
					Required: []string{"template"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "run_tests",
				Description: "Execute tests for the current project and report structured results. Auto-detects project type (Go, Node.js, Python) and runs appropriate test commands.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"command": {
							Type:        "string",
							Description: "Custom test command to run (e.g., 'go test ./pkg/...', 'npm test -- --coverage'). If omitted, auto-detects from project files.",
						},
						"args": {
							Type:        "array",
							Description: "Additional arguments for the test command. Can be an array of strings or a single space-separated string.",
						},
						"timeout": {
							Type:        "integer",
							Description: "Maximum execution time in seconds (default: 60)",
						},
					},
					Required: []string{},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "project_tree",
				Description: "Generate a visual directory tree showing the project structure with file metadata including icons, sizes, and type indicators. Supports depth limiting and hidden file filtering.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Directory path to show tree for (default: current directory)",
						},
						"max_depth": {
							Type:        "integer",
							Description: "Maximum depth levels to traverse (default: 3)",
						},
						"show_hidden": {
							Type:        "boolean",
							Description: "Include hidden files (dotfiles) in the tree (default: true)",
						},
						"max_entries": {
							Type:        "integer",
							Description: "Maximum entries to show per level (default: 100)",
						},
					},
					Required: []string{},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "code_navigation",
				Description: "Navigate code to find definitions, references, or implementations of a symbol. Supports multiple languages including Go, Python, JavaScript/TypeScript, Rust, Java, and more.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"query": {
							Type:        "string",
							Description: "Symbol name to search for (function, class, type, variable name, etc.)",
						},
						"mode": {
							Type:        "string",
							Description: "Search mode: 'definitions' (find where it's defined), 'references' (find all usages), or 'implementations' (find interface implementations). Default: definitions",
						},
						"file_type": {
							Type:        "string",
							Description: "Limit search to a specific language (e.g., 'go', 'python', 'typescript', 'rust', 'java'). If omitted, searches all known source files.",
						},
						"paths": {
							Type:        "array",
							Description: "Directories to search within. If omitted, searches the current directory recursively.",
						},
						"max_results": {
							Type:        "integer",
							Description: "Maximum results to return (default: 30)",
						},
					},
					Required: []string{"query"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "check_links",
				Description: "Scan Markdown and HTML files for broken links (both internal file links and external URLs). Detects relative paths, image references, and HTTP/HTTPS URLs. Returns structured results with summaries of valid and broken links.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"paths": {
							Type:        "array",
							Description: "Glob patterns to restrict search to specific files/directories (e.g., ['docs/**/*.md', 'README.md']). If omitted, searches all .md and .html files recursively from the current directory.",
						},
						"file_types": {
							Type:        "array",
							Description: "File extensions to scan for links. Default: ['.md', '.html', '.htm']. Can be a list of extensions with or without the leading dot.",
						},
						"timeout": {
							Type:        "integer",
							Description: "Timeout in seconds for checking external URLs (default: 10)",
						},
					},
					Required: []string{},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "json_transformer",
				Description: "Transform JSON data with multiple operations: extract, set, merge, validate, format, convert_to_yaml, convert_to_env. Operates on JSON from a file path or raw JSON string.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"command": {
							Type:        "string",
							Description: "The operation to perform: 'extract', 'set', 'merge', 'validate', 'format', 'convert_to_yaml', 'convert_to_env'",
						},
						"file_path": {
							Type:        "string",
							Description: "Path to a JSON file to operate on (alternative to json_string)",
						},
						"json_string": {
							Type:        "string",
							Description: "Raw JSON string to operate on (alternative to file_path)",
						},
						"path": {
							Type:        "string",
							Description: "JSON path for extract/set operations using dot notation (e.g., '.foo.bar' or 'foo.bar[0]')",
						},
						"value": {
							Type:        "string",
							Description: "Value to set (for 'set' command). Can be a JSON string, number, boolean, or null.",
						},
						"files": {
							Type:        "array",
							Description: "List of JSON file paths to merge (for 'merge' command)",
						},
						"json_strings": {
							Type:        "array",
							Description: "List of raw JSON strings to merge (for 'merge' command)",
						},
						"required_fields": {
							Type:        "array",
							Description: "List of required JSON field paths to validate (for 'validate' command)",
						},
						"indent": {
							Type:        "integer",
							Description: "Number of spaces for formatting (default: 2, for 'format' command)",
						},
					},
					Required: []string{"command"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "project_diagnostics",
				Description: "Scan a codebase for common issues and quality problems. Detects TODO/FIXME/HACK/WARN/XXX markers, empty files, large files (>500 lines), hardcoded secrets/keys, and more. Returns a structured report with severity levels and recommendations.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"paths": {
							Type:        "array",
							Description: "Paths to scan (glob patterns or directory paths). If omitted, scans the current directory.",
						},
						"max_depth": {
							Type:        "integer",
							Description: "Maximum directory depth to traverse (default: 10)",
						},
						"mode": {
							Type:        "string",
							Description: "Scan mode: 'full' (default, all checks), 'basic' (TODOs and empty files only). Currently supported: full, basic.",
						},
					},
					Required: []string{},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "run_lint",
				Description: "Run linters for the current project and report structured results. Auto-detects project type (Go, Python, Node.js) and runs appropriate linters (gofmt, go vet, flake8, pylint, eslint, shellcheck).",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"command": {
							Type:        "string",
							Description: "Custom lint command to run (e.g., 'go vet ./...', 'flake8 src/', 'eslint .'). If omitted, auto-detects from project files.",
						},
						"args": {
							Type:        "array",
							Description: "Additional arguments for the lint command. Can be an array of strings or a single space-separated string.",
						},
						"timeout": {
							Type:        "integer",
							Description: "Maximum execution time in seconds (default: 60)",
						},
					},
					Required: []string{},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "process_management",
				Description: "Manage running processes and check system resources. Supports four actions: process_list (list/filter processes), process_kill (kill by PID or name), port_check (check if a TCP/UDP port is in use), and system_info (CPU/memory/disk usage).",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"action": {
							Type:        "string",
							Description: "Action to perform: 'process_list', 'process_kill', 'port_check', or 'system_info'",
						},
						"filter": {
							Type:        "string",
							Description: "Regex pattern to filter process names (for process_list)",
						},
						"user": {
							Type:        "string",
							Description: "Filter by username (for process_list, Linux only)",
						},
						"limit": {
							Type:        "integer",
							Description: "Maximum number of results (for process_list, default: 50)",
						},
						"sort": {
							Type:        "string",
							Description: "Sort order: 'pid', 'cpu', or 'memory' (for process_list, default: 'pid')",
						},
						"pid": {
							Type:        "integer",
							Description: "Process ID to kill (for process_kill)",
						},
						"name": {
							Type:        "string",
							Description: "Process name to kill (for process_kill, kills first match)",
						},
						"force": {
							Type:        "boolean",
							Description: "Use SIGKILL instead of SIGTERM (for process_kill, default: false)",
						},
						"port": {
							Type:        "integer",
							Description: "Port number to check (for port_check)",
						},
						"protocol": {
							Type:        "string",
							Description: "Protocol to check: 'tcp' or 'udp' (for port_check, default: 'tcp')",
						},
						"format": {
							Type:        "string",
							Description: "Output format: 'short' or 'detailed' (for system_info, default: 'short')",
						},
					},
					Required: []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "env_var",
				Description: "Manage environment variables. Supports five actions: get (read a variable), set (set a variable), unset (remove a variable), list (list all or filtered variables), and source (load variables from a .env file).",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"action": {
							Type:        "string",
							Description: "Action to perform: 'get', 'set', 'unset', 'list', or 'source'",
						},
						"name": {
							Type:        "string",
							Description: "Environment variable name (required for get, set, unset)",
						},
						"value": {
							Type:        "string",
							Description: "Value to set (required for set)",
						},
						"prefix": {
							Type:        "string",
							Description: "Filter environment variables by prefix (for list)",
						},
						"show_all": {
							Type:        "boolean",
							Description: "Include empty/unset variables in list output (for list, default: false)",
						},
						"path": {
							Type:        "string",
							Description: "Path to .env file to source (required for source)",
						},
						"overwrite": {
							Type:        "boolean",
							Description: "Overwrite existing variables when sourcing .env file (for source, default: false)",
						},
					},
					Required: []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "file_compare",
				Description: "Compare two text files and return a structured diff showing added, removed, and unchanged lines with line numbers and context.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"file1": {
							Type:        "string",
							Description: "Path to the first (original) file",
						},
						"file2": {
							Type:        "string",
							Description: "Path to the second (modified) file",
						},
						"context": {
							Type:        "integer",
							Description: "Number of context lines around changes (default: 3)",
						},
					},
					Required: []string{"file1", "file2"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "changelog",
				Description: "Generate changelog entries from git commit history. Groups conventional commits by category (Features, Bug Fixes, Breaking Changes, etc.) following Keep a Changelog format. Actions: 'generate' outputs changelog to stdout, 'add' appends to an existing CHANGELOG.md file.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"action": {
							Type:        "string",
							Description: "Action to perform: 'generate' (output changelog) or 'add' (append to CHANGELOG.md)",
						},
						"from_tag": {
							Type:        "string",
							Description: "Start commit/tag for changelog range (default: first commit)",
						},
						"to_tag": {
							Type:        "string",
							Description: "End commit/tag for changelog range (default: HEAD)",
						},
						"unreleased": {
							Type:        "boolean",
							Description: "Include only commits without an associated tag (for generate), or move unreleased section to new tag (for add)",
						},
						"tag": {
							Type:        "string",
							Description: "Git tag/version for this changelog entry (required for 'add' action)",
						},
						"date": {
							Type:        "string",
							Description: "Date for this entry in YYYY-MM-DD format (default: today)",
						},
						"path": {
							Type:        "string",
							Description: "Path to CHANGELOG.md file (default: ./CHANGELOG.md)",
						},
						"header": {
							Type:        "string",
							Description: "Custom header text to prepend to the changelog output",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "run_build",
				Description: "Execute a build command for the current project. Auto-detects project type (Go, Node.js, Rust, Java/Maven, Java/Gradle, Python) and runs appropriate build commands. Supports custom command override.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"command": {
							Type:        "string",
							Description: "Custom build command to run (e.g., 'go build ./cmd/...', 'npm run build', 'cargo build'). If omitted, auto-detects from project files.",
						},
						"args": {
							Type:        "array",
							Description: "Additional arguments for the build command. Can be an array of strings or a single space-separated string.",
						},
						"timeout": {
							Type:        "integer",
							Description: "Maximum execution time in seconds (default: 120)",
						},
					},
					Required: []string{},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "run_coverage",
				Description: "Run project tests with coverage analysis and return structured coverage results. Auto-detects project type (Go, Node.js, Python) and runs appropriate coverage commands. Reports overall coverage percentage, per-file coverage, highlights low-coverage files (<50%), and files with no coverage (0%). Supports custom commands, arguments, and timeout.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"command": {
							Type:        "string",
							Description: "Custom coverage command to run (e.g., 'go test -coverprofile=coverage.out ./...', 'npx c8', 'pytest --cov=.'). If omitted, auto-detects from project files.",
						},
						"args": {
							Type:        "array",
							Description: "Additional arguments for the coverage command. Can be an array of strings or a single space-separated string.",
						},
						"timeout": {
							Type:        "integer",
							Description: "Maximum execution time in seconds (default: 120)",
						},
					},
					Required: []string{},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "git_merge",
				Description: "Manage git merge operations: merge (standard), abort, status check, squash merge, and merge GitHub PRs.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"action": {
							Type:        "string",
							Description: "Merge action: 'merge' (standard merge), 'abort' (abort in-progress merge), 'status' (check merge status), 'squash' (squash merge), or 'merge_pr' (merge a GitHub pull request)",
						},
						"source": {
							Type:        "string",
							Description: "Source branch to merge (required for merge and squash actions)",
						},
						"target": {
							Type:        "string",
							Description: "Target branch to merge into (default: HEAD/current branch)",
						},
						"commit_message": {
							Type:        "string",
							Description: "Custom commit message (for merge and squash actions)",
						},
						"github_token": {
							Type:        "string",
							Description: "GitHub API token for merge_pr action (also reads GITHUB_TOKEN env var as fallback)",
						},
						"repo": {
							Type:        "string",
							Description: "Repository in 'owner/repo' format (required for merge_pr action)",
						},
						"pr_number": {
							Type:        "integer",
							Description: "Pull request number to merge (required for merge_pr action)",
						},
						"merge_method": {
							Type:        "string",
							Description: "Merge method for PR: 'merge' (default), 'squash', or 'rebase' (for merge_pr action only)",
						},
					},
					Required: []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "generate_docs",
				Description: "Generate documentation for code files. Auto-detects language from file extension and supports multiple output formats.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to a file or directory to generate documentation for",
						},
						"format": {
							Type:        "string",
							Description: "Output format: 'markdown' (default) or 'inline' (docstrings)",
						},
						"detail": {
							Type:        "string",
							Description: "Detail level: 'basic' (signatures only) or 'detailed' (default, includes comments and field info)",
						},
						"include_comments": {
							Type:        "boolean",
							Description: "Include source comments/docstrings in output (default: true)",
						},
					},
					Required: []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "code_metrics",
				Description: "Analyze source code files for metrics: lines of code (total/blank/comment/code), cyclomatic complexity, function/method counts, and language breakdown. Supports Go, Python, JavaScript/TypeScript, Java, Rust, and C/C++.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "File or directory path to analyze",
						},
						"language": {
							Type:        "string",
							Description: "Force language detection (e.g., 'go', 'python', 'javascript'). Auto-detected from extension if not specified.",
						},
						"max_depth": {
							Type:        "integer",
							Description: "Maximum directory recursion depth (default: 5, use 0 for current file only)",
						},
						"glob": {
							Type:        "string",
							Description: "Glob pattern to filter files (e.g., '*.go', 'src/**/*.py')",
						},
					},
					Required: []string{"path"},
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
