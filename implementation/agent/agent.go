package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/coding-agent/coding-agent/config"
	"github.com/coding-agent/coding-agent/debug"
	"github.com/coding-agent/coding-agent/inference"
	"github.com/coding-agent/coding-agent/mcp"
	"github.com/coding-agent/coding-agent/skills"
	"github.com/coding-agent/coding-agent/tools"
)

// EventType classifies output produced by the agent.
type EventType string

// Event types emitted during a run.
const (
	EventContent    EventType = "content"
	EventReasoning  EventType = "reasoning"
	EventToolStart  EventType = "tool_start"
	EventToolResult EventType = "tool_result"
	EventError      EventType = "error"
	EventInfo       EventType = "info"
	EventGoal       EventType = "goal"
	EventCompressed EventType = "compressed"
	EventSubagent   EventType = "subagent"
	EventIteration  EventType = "iteration"
	EventToolArgs   EventType = "tool_args"
	EventDone       EventType = "done"
)

// Event is a single unit of agent output for the UI layer.
type Event struct {
	Type      EventType `json:"type"`
	Text      string    `json:"text,omitempty"`
	Tool      string    `json:"tool,omitempty"`
	Args      string    `json:"args,omitempty"`
	Result    string    `json:"result,omitempty"`
	Failed    bool      `json:"failed,omitempty"`
	Iteration int       `json:"iteration,omitempty"`
}

// Agent runs the agentic loop against an inference backend.
type Agent struct {
	cfg      *config.Config
	client   *inference.Client
	vision   *inference.Client
	registry *tools.Registry
	skills   *skills.Manager
	mcp      *mcp.Manager
	todo     *tools.TodoStore
	stats    *Stats
	run      *tools.RunContext

	mu               sync.Mutex
	context          []*inference.Message
	snapshots        []Iteration
	lastPromptTokens int
	goal             string
	goalStart        time.Time

	cbMu     sync.RWMutex
	callback func(Event)

	cancelMu      sync.Mutex
	currentCancel context.CancelFunc
}

// New creates an agent with the default tool set wired up.
func New(cfg *config.Config) *Agent {
	run := &tools.RunContext{
		CWD:               cfg.WorkingDir,
		ReadOnly:          cfg.ReadOnly,
		BashTimeoutMS:     config.DefaultBashTimeoutMs,
		MaxOutputLines:    config.DefaultBashMaxOutputLines,
		SubagentTimeoutMS: cfg.SubagentTimeout * 1000,
		MaxReadBytes:      20 * 1024,
		MaxImageBytes:     20 * 1024 * 1024,
	}
	reg := tools.NewRegistry(run)
	todoStore := tools.NewTodoStore()

	visionCfg := *cfg
	visionCfg.Stream = false

	a := &Agent{
		cfg:      cfg,
		client:   inference.New(cfg),
		vision:   inference.New(&visionCfg),
		registry: reg,
		todo:     todoStore,
		stats:    NewStats(),
		run:      run,
	}

	a.registerCoreTools(run, todoStore)

	if cfg.SkillsEnabled {
		a.skills = skills.Discover(cfg, cfg.WorkingDir)
		if a.skills.Count() > 0 {
			reg.SetSkillActivator(a.skills)
		}
	}

	if len(cfg.EnabledMCPServers()) > 0 {
		a.mcp = mcp.NewManager(cfg)
	}

	return a
}

// registerCoreTools wires the standard tool set into the registry.
func (a *Agent) registerCoreTools(run *tools.RunContext, todoStore *tools.TodoStore) {
	a.registry.Register(tools.NewBashTool(run))
	a.registry.Register(tools.NewReadFileTool(run))
	a.registry.Register(tools.NewReadLinesTool(run))
	a.registry.Register(tools.NewListFilesTool(run))
	a.registry.Register(tools.NewGrepTool(run))
	a.registry.Register(tools.NewGitLogTool(run))
	a.registry.Register(tools.NewGitShowTool(run))
	a.registry.Register(tools.NewGitDiffTool(run))
	a.registry.Register(tools.NewTodoTool(todoStore, run))
	a.registry.Register(tools.NewWriteFileTool(run))
	a.registry.Register(tools.NewInsertLinesTool(run))
	a.registry.Register(tools.NewReplaceTextTool(run))
	a.registry.Register(tools.NewMoveTextTool(run))
	a.registry.Register(tools.NewViewImageTool(run, a))
	a.registry.SetSubagentRunner(a)
}

// SetEventCallback installs the UI callback.
func (a *Agent) SetEventCallback(cb func(Event)) {
	a.cbMu.Lock()
	defer a.cbMu.Unlock()
	a.callback = cb
}

func (a *Agent) emit(e Event) {
	a.cbMu.RLock()
	cb := a.callback
	a.cbMu.RUnlock()
	if cb != nil {
		cb(e)
	}
}

// Registry returns the tool registry.
func (a *Agent) Registry() *tools.Registry { return a.registry }

// Skills returns the skill manager (nil when skills are disabled).
func (a *Agent) Skills() *skills.Manager { return a.skills }

// MCP returns the MCP manager (nil when no servers are configured).
func (a *Agent) MCP() *mcp.Manager { return a.mcp }

// Todo returns the todo store.
func (a *Agent) Todo() *tools.TodoStore { return a.todo }

// SetReadOnly toggles read-only mode at runtime.
func (a *Agent) SetReadOnly(v bool) {
	a.cfg.ReadOnly = v
	a.registry.SetReadOnly(v)
}

// ReadOnly reports whether read-only mode is active.
func (a *Agent) ReadOnly() bool { return a.registry.ReadOnly() }

// Stats returns the session statistics.
func (a *Agent) Stats() *Stats { return a.stats }

// Client returns the inference client.
func (a *Agent) Client() *inference.Client { return a.client }

// ConnectMCP connects to the configured MCP servers and registers their tools.
func (a *Agent) ConnectMCP(ctx context.Context) []error {
	if a.mcp == nil {
		return nil
	}
	errs := a.mcp.ConnectAll(ctx)
	n := a.mcp.RegisterTools(a.registry)
	if dbg := debug.Default(); dbg != nil {
		dbg.Logf("registered %d MCP tools", n)
	}
	return errs
}

// ---------------------------------------------------------------------------
// goal mode
// ---------------------------------------------------------------------------

// Goal returns the active goal and when it was set.
func (a *Agent) Goal() (string, time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.goal, a.goalStart
}

// SetGoal activates goal mode and resets the goal timer.
func (a *Agent) SetGoal(goal string) {
	a.mu.Lock()
	a.goal = goal
	a.goalStart = time.Now()
	a.mu.Unlock()
}

// ClearGoal deactivates goal mode.
func (a *Agent) ClearGoal() {
	a.mu.Lock()
	a.goal = ""
	a.goalStart = time.Time{}
	a.mu.Unlock()
}

// ---------------------------------------------------------------------------
// cancellation
// ---------------------------------------------------------------------------

// Cancel aborts the in-flight inference and tool execution.
func (a *Agent) Cancel() {
	a.cancelMu.Lock()
	cancel := a.currentCancel
	a.cancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *Agent) setCancel(cancel context.CancelFunc) {
	a.cancelMu.Lock()
	a.currentCancel = cancel
	a.cancelMu.Unlock()
}

func (a *Agent) clearCancel() { a.setCancel(nil) }

// ---------------------------------------------------------------------------
// the agentic loop
// ---------------------------------------------------------------------------

// Submit adds a user message and runs the agentic loop until the model stops
// calling tools (or the goal is achieved / the iteration limit is hit).
func (a *Agent) Submit(ctx context.Context, prompt string) (string, error) {
	if strings.TrimSpace(prompt) != "" {
		a.AddMessage(inference.NewTextMessage("user", prompt))
	}
	return a.loop(ctx)
}

// Continue runs the loop without adding a user message (used after loading a
// dumped context).
func (a *Agent) Continue(ctx context.Context) (string, error) { return a.loop(ctx) }

func (a *Agent) loop(ctx context.Context) (string, error) {
	runCtx, cancel := context.WithCancel(ctx)
	a.setCancel(cancel)
	defer func() {
		cancel()
		a.clearCancel()
	}()

	maxIterations := a.cfg.MaxIterations
	iterations := 0
	lastContent := ""

	for {
		if runCtx.Err() != nil {
			return lastContent, runCtx.Err()
		}
		if maxIterations > 0 && iterations >= maxIterations {
			a.emit(Event{Type: EventError, Text: MaxIterationsMessage(maxIterations), Failed: true})
			return lastContent, fmt.Errorf("%s", MaxIterationsMessage(maxIterations))
		}
		if a.ShouldCompress() {
			if err := a.Compress(runCtx); err != nil {
				if dbg := debug.Default(); dbg != nil {
					dbg.Logf("automatic compression failed: %v", err)
				}
			}
		}

		iterations++
		a.stats.AddIteration()
		a.emit(Event{Type: EventIteration, Iteration: iterations})

		resp, err := a.complete(runCtx)
		if err != nil {
			a.emit(Event{Type: EventError, Text: formatInferenceError(err), Failed: true})
			return lastContent, err
		}

		lastContent = strings.TrimSpace(resp.Content)
		assistant := &inference.Message{
			Role:           "assistant",
			Reasoning:      resp.Reasoning,
			ReasoningField: a.client.ReasoningField(),
			ToolCalls:      resp.ToolCalls,
		}
		if resp.Content != "" {
			assistant.Content = resp.Content
		}
		a.AddMessage(assistant)

		if len(resp.ToolCalls) == 0 {
			// Natural end of an inference turn.
			goal, goalStart := a.Goal()
			if goal != "" {
				if strings.Contains(strings.ToLower(resp.Content), "goal achieved") {
					a.ClearGoal()
					elapsed := time.Since(goalStart)
					a.emit(Event{Type: EventGoal, Text: fmt.Sprintf("Goal achieved in %s", FormatDuration(elapsed))})
					return lastContent, nil
				}
				a.emit(Event{Type: EventGoal, Text: "Checking goal: " + goal})
				a.AddMessage(inference.NewTextMessage("user", GoalCheckPrompt(goal)))
				continue
			}
			a.emit(Event{Type: EventDone, Text: lastContent})
			return lastContent, nil
		}

		a.executeToolCalls(runCtx, resp.ToolCalls)
	}
}

// complete performs one chat completion with the current context.
func (a *Agent) complete(ctx context.Context) (*inference.Response, error) {
	req := &inference.Request{
		Model:      a.cfg.Model,
		Messages:   a.messagesForRequest(),
		MaxTokens:  a.cfg.MaxTokens,
		ToolChoice: "auto",
	}
	if a.cfg.Temperature != nil {
		req.Temperature = a.cfg.Temperature
	}
	if a.cfg.TopP != nil {
		req.TopP = a.cfg.TopP
	}
	if defs := a.registry.Definitions(); len(defs) > 0 {
		req.Tools = defs
	}

	start := time.Now()
	accum := map[int]*toolAccum{}
	cb := &inference.StreamCallbacks{
		Content:   func(s string) { a.emit(Event{Type: EventContent, Text: s}) },
		Reasoning: func(s string) { a.emit(Event{Type: EventReasoning, Text: s}) },
		ToolCallDelta: func(index int, d inference.ToolCallDelta) {
			if dbg := debug.Default(); dbg != nil {
				dbg.Logf("tool call delta #%d %s %s", index, d.Function.Name, d.Function.Arguments)
			}
			// Accumulate so the UI can show parameters as they stream in
			// (requirement 017).
			acc := &toolAccum{}
			if existing, ok := accum[index]; ok {
				acc = existing
			}
			if d.ID != "" {
				acc.id = d.ID
			}
			if d.Function != nil {
				if d.Function.Name != "" {
					acc.name += d.Function.Name
				}
				acc.args += d.Function.Arguments
			}
			accum[index] = acc
			a.emit(Event{Type: EventToolArgs, Tool: acc.name, Args: FlattenArgs(acc.args)})
		},
	}
	resp, err := a.client.ChatCompletion(ctx, req, cb)
	elapsed := time.Since(start)
	if err != nil {
		a.stats.AddInferenceTime(elapsed)
		return nil, err
	}
	// In non-streaming mode no content deltas were emitted; surface the full
	// answer as a single content event so consumers stay uniform.
	if !a.cfg.Stream && resp.Content != "" {
		a.emit(Event{Type: EventContent, Text: resp.Content})
	}
	a.stats.AddInferenceTime(elapsed)
	a.stats.AddUsage(resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.CachedTokens)
	a.mu.Lock()
	if resp.Usage.PromptTokens > 0 {
		a.lastPromptTokens = resp.Usage.PromptTokens
	}
	a.mu.Unlock()
	if dbg := debug.Default(); dbg != nil {
		dbg.Logf("iteration finished in %s: finish=%q tool_calls=%d content=%d chars", elapsed, resp.FinishReason, len(resp.ToolCalls), len(resp.Content))
	}
	return resp, nil
}

// executeToolCalls runs every requested tool call and appends the results.
func (a *Agent) executeToolCalls(ctx context.Context, calls []inference.ToolCall) {
	for _, call := range calls {
		a.emit(Event{Type: EventToolStart, Tool: call.Function.Name, Args: call.Function.Arguments})
		start := time.Now()
		result, failed := a.registry.Execute(ctx, call)
		elapsed := time.Since(start)
		a.stats.AddToolCall(failed)
		a.stats.AddToolTime(elapsed)
		a.emit(Event{Type: EventToolResult, Tool: call.Function.Name, Result: result, Failed: failed})
		if dbg := debug.Default(); dbg != nil {
			dbg.Logf("tool %s (%s) -> %d bytes", call.Function.Name, elapsed, len(result))
		}
		// Requirement 016: tool messages carry a standard wrapper so the model
		// can always tell success from failure.
		content := result
		if failed {
			content = fmt.Sprintf("Tool '%s' failed: %s", call.Function.Name, result)
		} else {
			content = fmt.Sprintf("Tool '%s' executed successfully:\n%s", call.Function.Name, result)
		}
		a.AddMessage(&inference.Message{Role: "tool", Content: content, ToolCallID: call.ID, Name: call.Function.Name})
	}
}

// formatInferenceError turns an inference error into actionable output
// (requirement 018).
func formatInferenceError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return "Inference cancelled."
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "Inference timed out."
	}
	var apiErr *inference.APIError
	if errors.As(err, &apiErr) {
		msg := "Inference error: " + apiErr.Message
		if apiErr.Hint != "" {
			msg += "\n" + apiErr.Hint
		}
		return msg
	}
	return "Inference error: " + err.Error()
}

// ExitCodeFor maps an agent error onto the documented process exit code.
func ExitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, context.Canceled) {
		return 1
	}
	var apiErr *inference.APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == 401 || apiErr.StatusCode == 403 {
			return 3
		}
		if apiErr.StatusCode == 400 || apiErr.StatusCode == 404 {
			if strings.Contains(strings.ToLower(apiErr.Message), "context") || strings.Contains(strings.ToLower(apiErr.Message), "too long") {
				return 4
			}
		}
		return 1
	}
	if strings.Contains(strings.ToLower(err.Error()), "maximum number of iterations") {
		return 1
	}
	if strings.Contains(strings.ToLower(err.Error()), "context") {
		return 4
	}
	return 1
}

// ---------------------------------------------------------------------------
// subagents (requirement 040)
// ---------------------------------------------------------------------------

// RunSubagent spawns a separate coding-agent process and returns only its
// summary.
func (a *Agent) RunSubagent(ctx context.Context, prompt, persona string) (string, error) {
	exe := a.cfg.ExecutablePath
	if exe == "" {
		p, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("cannot locate the coding-agent executable: %w", err)
		}
		exe = p
	}
	args := []string{
		"--prompt-file", "-",
		"--summary-only",
		"--no-stream",
		"--quiet",
		"--api-endpoint", a.cfg.APIEndpoint,
		"--model", a.cfg.Model,
		"--context-size", fmt.Sprintf("%d", a.cfg.ContextSize),
		"--max-iterations", fmt.Sprintf("%d", a.cfg.MaxIterations),
	}
	if persona != "" {
		args = append(args, "--persona", persona)
	} else if a.cfg.Persona != "" {
		args = append(args, "--persona", a.cfg.Persona)
	}
	if a.cfg.ReadOnly {
		args = append(args, "--read-only")
	}
	if a.cfg.MaxTokens > 0 {
		args = append(args, "--max-tokens", fmt.Sprintf("%d", a.cfg.MaxTokens))
	}
	if a.cfg.NoDumpOnExit {
		args = append(args, "--no-dump-on-exit")
	}

	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = a.cfg.WorkingDir
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	env := os.Environ()
	if a.cfg.APIKey != "" {
		env = append(env, "CODING_AGENT_API_KEY="+a.cfg.APIKey)
	}
	cmd.Env = env

	a.emit(Event{Type: EventSubagent, Text: "Starting subagent task..."})
	err := cmd.Run()
	out := stdout.String()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		a.emit(Event{Type: EventSubagent, Text: "Failed: " + msg, Failed: true})
		return "", fmt.Errorf("subagent failed: %s", msg)
	}
	summary := extractSummary(out)
	a.emit(Event{Type: EventSubagent, Text: "Task completed\nSummary:\n" + summary})
	return "Subagent completed.\n\nSummary:\n" + summary, nil
}

// extractSummary pulls the conclusion out of a subagent's output.
func extractSummary(output string) string {
	if idx := strings.Index(output, "=== Final Output ==="); idx != -1 {
		return strings.TrimSpace(output[idx+len("=== Final Output ==="):])
	}
	if len(output) > 5000 {
		return output[:5000] + "\n... [output truncated]"
	}
	return strings.TrimSpace(output)
}

// ---------------------------------------------------------------------------
// image viewing (requirement 041)
// ---------------------------------------------------------------------------

// DescribeImage sends an image to the configured model and returns the answer.
func (a *Agent) DescribeImage(ctx context.Context, dataURI, prompt string) (string, error) {
	req := &inference.Request{
		Model: a.cfg.Model,
		Messages: []*inference.Message{{
			Role: "user",
			Content: []inference.ContentPart{
				{Type: "text", Text: prompt},
				{Type: "image_url", ImageURL: &inference.ImageURL{URL: dataURI, Detail: "auto"}},
			},
		}},
	}
	resp, err := a.vision.ChatCompletion(ctx, req, nil)
	if err != nil {
		return "", fmt.Errorf("loaded the image but vision analysis failed: %v", err)
	}
	a.stats.AddUsage(resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.CachedTokens)
	return strings.TrimSpace(resp.Content), nil
}
