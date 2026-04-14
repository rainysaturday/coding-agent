package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"

	"coding-agent/implementation/config"
	"coding-agent/implementation/debug"
	"coding-agent/implementation/inference"
	"coding-agent/implementation/tools"
	"coding-agent/implementation/types"
)

type StreamWriter interface {
	StreamReasoningChunk(text string)
	StreamNormalChunk(text string)
	StreamToolEvent(text string)
	Println(text string)
}

type Agent struct {
	cfg      config.Config
	client   *inference.Client
	tools    *tools.Executor
	logger   *debug.Logger
	stats    *types.Stats
	messages []types.Message
}

func New(cfg config.Config, logger *debug.Logger) *Agent {
	a := &Agent{
		cfg:    cfg,
		client: inference.New(cfg),
		tools:  tools.NewExecutor(),
		logger: logger,
		stats:  types.NewStats(),
	}
	a.messages = append(a.messages, types.Message{Role: "system", Content: buildSystemPrompt()})
	return a
}

func (a *Agent) Stats() *types.Stats { return a.stats }

func buildSystemPrompt() string {
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	return strings.TrimSpace(`You are a coding assistant with tool access.

Tool calling format:
- Return tool calls via OpenAI tool_calls JSON function format.
- Prefer tools over guessing filesystem state.
- Verify your work when possible.

Environment information:
- Current working directory: ` + cwd + `
- Executable path: ` + exe + `
- OS: ` + runtime.GOOS + `
- Architecture: ` + runtime.GOARCH + `

Available tools:
- bash(command)
- read_file(path)
- write_file(path, content)
- read_lines(path, start, end)
- insert_lines(path, line, text)
- replace_text(path, find, replace, count)
- patch(patch)
`)
}

func (a *Agent) AddUserMessage(prompt string) {
	a.messages = append(a.messages, types.Message{Role: "user", Content: prompt})
	a.logger.LogMessage("USER MESSAGE", types.Message{Role: "user", Content: prompt})
}

func (a *Agent) ContextStatus() string {
	used := estimateTokens(a.messages)
	pct := float64(used) / float64(a.cfg.ContextSize) * 100
	warn := ""
	if pct >= 90 {
		warn = " !!"
	} else if pct >= 75 {
		warn = " !"
	}
	return fmt.Sprintf("[Context: %d / %d (%.1f%%)%s]", used, a.cfg.ContextSize, pct, warn)
}

func (a *Agent) RunOnce(ctx context.Context, stream StreamWriter) (string, error) {
	for i := 0; i < a.cfg.MaxIterations; i++ {
		if estimateTokens(a.messages) > int(float64(a.cfg.ContextSize)*0.85) {
			a.compressContext(ctx)
		}

		var msg types.Message
		if a.cfg.NoStream {
			resp, err := a.client.Infer(ctx, a.messages, tools.Definitions())
			if err != nil {
				return "", err
			}
			msg = resp.Message
			a.stats.RecordUsage(resp.Usage)
			a.logger.LogUsage(resp.Usage)
		} else {
			var builder strings.Builder
			var reason strings.Builder
			var gatheredCalls []types.ToolCall
			usage, err := a.client.InferStream(ctx, a.messages, tools.Definitions(), func(delta types.StreamDelta) {
				if delta.ReasoningContent != "" {
					reason.WriteString(delta.ReasoningContent)
					stream.StreamReasoningChunk(delta.ReasoningContent)
				}
				if delta.Content != "" {
					builder.WriteString(delta.Content)
					stream.StreamNormalChunk(delta.Content)
				}
				if len(delta.ToolCalls) > 0 {
					gatheredCalls = delta.ToolCalls
				}
			})
			if err != nil {
				return "", err
			}
			msg = types.Message{Role: "assistant", Content: builder.String(), ReasoningContent: reason.String(), ToolCalls: gatheredCalls}
			a.stats.RecordUsage(usage)
			a.logger.LogUsage(usage)
		}

		a.messages = append(a.messages, msg)
		a.logger.LogMessage("ASSISTANT MESSAGE", msg)

		if len(msg.ToolCalls) == 0 {
			return msg.Content, nil
		}

		for _, call := range msg.ToolCalls {
			stream.StreamToolEvent(fmt.Sprintf("[tool] %s", call.Function.Name))
			out, err := a.tools.Execute(ctx, call)
			success := err == nil
			a.stats.RecordToolCall(success)
			if err != nil {
				out = formatToolError(call, err)
			}
			a.logger.LogToolResult(call.Function.Name, call.ID, success, out)
			a.messages = append(a.messages, types.Message{Role: "tool", ToolCallID: call.ID, Name: call.Function.Name, Content: out})
		}
	}
	return "", fmt.Errorf("maximum iterations (%d) exceeded", a.cfg.MaxIterations)
}

func formatToolError(call types.ToolCall, err error) string {
	return fmt.Sprintf("tool %s failed: %v", call.Function.Name, err)
}

func estimateTokens(messages []types.Message) int {
	total := 0
	for _, m := range messages {
		total += len(m.Content)/4 + len(m.ReasoningContent)/4 + 8
		if len(m.ToolCalls) > 0 {
			b, _ := json.Marshal(m.ToolCalls)
			total += len(b) / 4
		}
	}
	return total
}

func (a *Agent) compressContext(ctx context.Context) {
	if len(a.messages) < 6 {
		return
	}
	system := a.messages[0]
	tail := a.messages[len(a.messages)-4:]
	var b strings.Builder
	for _, m := range a.messages[1 : len(a.messages)-4] {
		if m.Role == "tool" {
			continue
		}
		b.WriteString(m.Role)
		b.WriteString(": ")
		b.WriteString(m.Content)
		b.WriteString("\n")
	}
	prompt := "Summarize this conversation compactly while preserving key technical facts and decisions:\n" + b.String()
	sumMessages := []types.Message{{Role: "system", Content: "You summarize conversations."}, {Role: "user", Content: prompt}}
	resp, err := a.client.Infer(ctx, sumMessages, nil)
	if err != nil || strings.TrimSpace(resp.Message.Content) == "" {
		return
	}
	a.messages = []types.Message{system, {Role: "assistant", Content: "[context summary]\n" + resp.Message.Content}}
	a.messages = append(a.messages, tail...)
}
