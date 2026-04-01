// Package agent implements the main agent logic.
package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
	"github.com/coding-agent/harness/tools"
)

// StreamCallback is a function type for handling streaming chunks.
// Using inference.StreamingCallback for type compatibility.
type StreamCallback = inference.StreamingCallback

// Agent represents the coding agent.
type Agent struct {
	config        *config.Config
	inference     *inference.InferenceClient
	toolExecutor  *tools.ToolExecutor
	context       []*inference.Message
	systemPrompt  string
	stats         *Stats
	maxIterations int
	streamCallback StreamCallback
	mu            sync.Mutex
}

// Stats represents agent statistics.
type Stats struct {
	InputTokens      int       `json:"input_tokens"`
	OutputTokens     int       `json:"output_tokens"`
	ToolCalls        int       `json:"tool_calls"`
	FailedToolCalls  int       `json:"failed_tool_calls"`
	Iterations       int       `json:"iterations"`
	StartTime        time.Time `json:"start_time"`
}

// Step represents an execution step.
type Step struct {
	Action      string
	ToolCall    *tools.ToolCall
	ToolResult  *tools.ToolResult
	StreamMsg   string // Status message streamed to user
}

// Result represents the final result of an agent run.
type Result struct {
	FinalOutput string
	Steps       []Step
	TokenUsage  int
}

// NewAgent creates a new agent.
func NewAgent(cfg *config.Config) *Agent {
	agent := &Agent{
		config:       cfg,
		inference:    inference.NewInferenceClient(cfg),
		toolExecutor: tools.NewToolExecutor(),
		context:      make([]*inference.Message, 0),
		stats: &Stats{
			StartTime: time.Now(),
		},
		maxIterations: 50,
	}

	// Build system prompt
	agent.systemPrompt = buildSystemPrompt()

	return agent
}

// SetStreamCallback sets the callback function for streaming chunks.
func (a *Agent) SetStreamCallback(callback StreamCallback) {
	a.mu.Lock()
	a.streamCallback = callback
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

// Run runs the agent with the given prompt.
func (a *Agent) Run(ctx context.Context, prompt string) (*Result, error) {
	// Add user message to context
	a.mu.Lock()
	a.context = append(a.context, &inference.Message{
		Role:    "user",
		Content: prompt,
	})
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

		// Get response from LLM (now supports streaming)
		response, err := a.getInferenceResponse(ctx)
		if err != nil {
			return nil, err
		}

		// Update token stats
		a.mu.Lock()
		a.stats.InputTokens += response.TokenUsage / 2 // Rough estimate
		a.stats.OutputTokens += response.TokenUsage / 2
		a.mu.Unlock()

		// Check if there are tool calls
		if len(response.ToolCalls) > 0 {
			// Execute tool calls
			for _, tc := range response.ToolCalls {
				// Stream tool call status to user
				streamStatus(tc.Name, tc.Parameters, a.streamCallback)

				step := Step{
					Action:   fmt.Sprintf("Calling tool: %s", tc.Name),
					ToolCall: tc,
				}

				// Execute the tool
				result := a.toolExecutor.Execute(tc)
				step.ToolResult = result

				// Stream tool result status to user
				streamResult(tc.Name, result, a.streamCallback)
				step.StreamMsg = formatToolStatus(tc.Name, result)

				steps = append(steps, step)

				// Add tool result to context
				var resultMessage string
				if result.Success {
					resultMessage = fmt.Sprintf("Tool '%s' executed successfully:\n%s", tc.Name, formatResult(result))
				} else {
					resultMessage = fmt.Sprintf("Tool '%s' failed: %s", tc.Name, result.Error)
				}

				a.mu.Lock()
				a.context = append(a.context, &inference.Message{
					Role:    "user",
					Content: resultMessage,
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

// GetStats returns the current statistics.
func (a *Agent) GetStats() *Stats {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Get tool executor stats
	toolStats := a.toolExecutor.Stats()

	return &Stats{
		InputTokens:     a.stats.InputTokens,
		OutputTokens:    a.stats.OutputTokens,
		ToolCalls:       toolStats.TotalCalls,
		FailedToolCalls: toolStats.FailedCalls,
		Iterations:      a.stats.Iterations,
		StartTime:       a.stats.StartTime,
	}
}

// ClearContext clears the conversation context.
func (a *Agent) ClearContext() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.context = make([]*inference.Message, 0)
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

// GetContextSize returns the current context size in estimated tokens.
func (a *Agent) GetContextSize() int {
	a.mu.Lock()
	defer a.mu.Unlock()

	total := 0
	for _, msg := range a.context {
		total += inference.EstimateTokens(msg.Content)
	}
	return total
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

// streamStatus streams a tool call status message.
func streamStatus(toolName string, params map[string]interface{}, callback StreamCallback) {
	if callback == nil {
		return
	}
	
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
		msg = fmt.Sprintf("\n[Running] bash: %s\n", cmd)
	case "read_file":
		path := ""
		if p, ok := params["path"].(string); ok {
			path = p
		}
		msg = fmt.Sprintf("\n[Reading] file: %s\n", path)
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
		msg = fmt.Sprintf("\n[Reading] lines %d-%d from: %s\n", start, end, path)
	case "write_file":
		path := ""
		if p, ok := params["path"].(string); ok {
			path = p
		}
		msg = fmt.Sprintf("\n[Writing] file: %s\n", path)
	case "insert_lines":
		path := ""
		line := 0
		if p, ok := params["path"].(string); ok {
			path = p
		}
		if p, ok := params["line"].(float64); ok {
			line = int(p)
		}
		msg = fmt.Sprintf("\n[Inserting] at line %d in: %s\n", line, path)
	case "replace_lines":
		path := ""
		if p, ok := params["path"].(string); ok {
			path = p
		}
		msg = fmt.Sprintf("\n[Replacing] in file: %s\n", path)
	default:
		msg = fmt.Sprintf("\n[Running] tool: %s\n", toolName)
	}
	callback(msg)
}

// streamResult streams a tool result status message.
func streamResult(toolName string, result *tools.ToolResult, callback StreamCallback) {
	if callback == nil {
		return
	}
	
	status := formatToolStatus(toolName, result)
	callback(status)
}

// formatToolStatus formats a tool status message for display.
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
			return fmt.Sprintf("[Success] bash completed\nOutput:\n%s\n", output)
		case "read_file":
			output := result.Output
			lines := strings.Split(output, "\n")
			if len(lines) > 10 {
				lines = lines[:10]
				output = strings.Join(lines, "\n") + "\n... [content truncated]"
			}
			return fmt.Sprintf("[Success] read %d lines\nContent:\n%s\n", len(lines), output)
		case "read_lines":
			output := result.Output
			lines := strings.Split(output, "\n")
			if len(lines) > 10 {
				lines = lines[:10]
				output = strings.Join(lines, "\n") + "\n... [content truncated]"
			}
			return fmt.Sprintf("[Success] read %d lines\nContent:\n%s\n", len(lines), output)
		case "write_file":
			if msg, ok := result.Extra["message"].(string); ok {
				return fmt.Sprintf("[Success] %s\n", msg)
			}
			return "[Success] file written\n"
		case "insert_lines":
			count := 0
			if c, ok := result.Extra["linesInserted"].(int); ok {
				count = c
			}
			return fmt.Sprintf("[Success] inserted %d line(s)\n", count)
		case "replace_lines":
			count := 0
			if c, ok := result.Extra["replacementsMade"].(int); ok {
				count = c
			} else if c, ok := result.Extra["linesReplaced"].(int); ok {
				count = c
			}
			return fmt.Sprintf("[Success] replaced %d line(s)\n", count)
		default:
			return "[Success] tool completed\n"
		}
	}
	return fmt.Sprintf("[Failed] %s\nError: %s\n", toolName, result.Error)
}

// buildSystemPrompt builds the system prompt with tool definitions.
func buildSystemPrompt() string {
	return `You are a helpful coding assistant. You have access to the following tools:

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

- replace_lines: Replace a line range with new lines (line-number mode)
  Format: [TOOL:{"name":"replace_lines","parameters":{"path":"file path","start":line_number,"end":line_number,"lines":"replacement lines"}}]
  Example: [TOOL:{"name":"replace_lines","parameters":{"path":"/path/to/file.txt","start":1,"end":5,"lines":"new content"}}]

- replace_lines: Replace content by searching (search-and-replace mode)
  Format: [TOOL:{"name":"replace_lines","parameters":{"path":"file path","search":"text to find","replace":"replacement text","count":1}}]
  Example: [TOOL:{"name":"replace_lines","parameters":{"path":"./main.go","search":"oldVariable","replace":"newVariable"}}]

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

VERIFICATION REQUIREMENTS:
- ALWAYS double-check your work before considering a task complete
- Verify that created/modified files exist and contain the expected content
- Test code execution when possible (e.g., run go build, go test)
- Validate that changes meet the user's requirements
- If you make multiple changes, verify each one independently
- Re-read files after writing to confirm content was written correctly
- Run validation commands (e.g., go vet, gofmt -d, cat to view files)
- If verification fails, fix the issue and re-verify
- Provide a final verification summary before concluding the task

Verification Checklist:
1. Files exist at the expected paths
2. File content matches the intended changes
3. Code compiles without errors (for Go code)
4. Code follows Go formatting standards (gofmt)
5. Changes align with user requirements
6. No unintended side effects or broken dependencies`
}
