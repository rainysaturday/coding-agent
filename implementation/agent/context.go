package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coding-agent/coding-agent/debug"
	"github.com/coding-agent/coding-agent/inference"
	"github.com/coding-agent/coding-agent/skills"
)

// EstimateTokens approximates the token count of a message list. The heuristic
// is roughly four characters per token plus a small per-message overhead; it is
// replaced by server reported usage whenever that is available.
func EstimateTokens(msgs []*inference.Message) int {
	total := 0
	for _, m := range msgs {
		total += 4 // role and framing overhead
		total += len(m.Text()) / 4
		total += len(m.Reasoning) / 4
		for _, tc := range m.ToolCalls {
			total += len(tc.Function.Name) / 4
			total += len(tc.Function.Arguments) / 4
			total += 8
		}
	}
	return total
}

// ContextTokens returns the best known token count for the current context.
func (a *Agent) ContextTokens() int {
	a.mu.Lock()
	msgs := append([]*inference.Message(nil), a.context...)
	last := a.lastPromptTokens
	a.mu.Unlock()
	if last > 0 {
		return last
	}
	return EstimateTokens(msgs)
}

// ContextLimit returns the configured context window size.
func (a *Agent) ContextLimit() int {
	if a.cfg.ContextSize > 0 {
		return a.cfg.ContextSize
	}
	return 128000
}

// ContextPercent returns the percentage of the context window in use.
func (a *Agent) ContextPercent() float64 {
	return 100 * float64(a.ContextTokens()) / float64(a.ContextLimit())
}

// ShouldCompress reports whether the context crossed the compression threshold.
func (a *Agent) ShouldCompress() bool {
	if !a.cfg.AutoCompress {
		return false
	}
	threshold := a.cfg.ContextThreshold
	if threshold <= 0 {
		threshold = 0.8
	}
	return float64(a.ContextTokens()) >= threshold*float64(a.ContextLimit())
}

// AddMessage appends a message to the conversation context.
func (a *Agent) AddMessage(m *inference.Message) {
	a.mu.Lock()
	a.context = append(a.context, m)
	a.mu.Unlock()
}

// Context returns a copy of the conversation context (without the system
// prompt, which is injected per request).
func (a *Agent) Context() []*inference.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]*inference.Message, len(a.context))
	copy(out, a.context)
	return out
}

// SetContext replaces the conversation context.
func (a *Agent) SetContext(msgs []*inference.Message) {
	a.mu.Lock()
	a.context = msgs
	a.lastPromptTokens = 0
	a.mu.Unlock()
}

// ClearContext empties the conversation context.
func (a *Agent) ClearContext() {
	a.SetContext(nil)
}

// messagesForRequest builds the message array sent to the backend: the system
// prompt followed by the stored context.
func (a *Agent) messagesForRequest() []*inference.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.messagesForRequestLocked()
}

// messagesForRequestLocked is the lock-free variant; the caller must hold a.mu.
func (a *Agent) messagesForRequestLocked() []*inference.Message {
	sys := inference.NewTextMessage("system", a.SystemPrompt())
	ctx := append([]*inference.Message(nil), a.context...)
	return append([]*inference.Message{sys}, ctx...)
}

// Compress summarises the middle of the conversation and rebuilds the context
// as [first user message, assistant summary, last messages...] (requirement 009).
func (a *Agent) Compress(ctx context.Context) error {
	a.mu.Lock()
	msgs := append([]*inference.Message(nil), a.context...)
	a.mu.Unlock()

	if len(msgs) <= 5 {
		return nil
	}

	// The absolute first user message is always preserved verbatim.
	firstIdx := -1
	for i, m := range msgs {
		if m.Role == "user" {
			firstIdx = i
			break
		}
	}
	if firstIdx < 0 {
		firstIdx = 0
	}

	// Preserve the last three messages, extending backwards so that preserved
	// tool messages keep the assistant message that requested the call.
	keep := 3
	start := len(msgs) - keep
	if start <= firstIdx+1 {
		return nil
	}
	for start < len(msgs) && msgs[start].Role == "tool" {
		start--
	}
	if start <= firstIdx+1 {
		return nil
	}

	middle := msgs[firstIdx+1 : start]
	preserved := msgs[start:]

	summary, err := a.summarize(ctx, middle)
	if err != nil {
		if dbg := debug.Default(); dbg != nil {
			dbg.Logf("compression failed: %v", err)
		}
		// Fall back to a mechanical truncation so the context still shrinks.
		summary = mechanicalSummary(middle)
	}

	// Skill instructions survive compression (requirement 047).
	var skillNotes []string
	for _, m := range middle {
		if m.Role == "tool" && skills.IsSkillContent(m.Text()) {
			skillNotes = append(skillNotes, m.Text())
		}
	}

	a.mu.Lock()
	next := []*inference.Message{msgs[firstIdx]}
	next = append(next, inference.NewTextMessage("assistant", "Conversation summary: "+summary))
	if len(skillNotes) > 0 {
		next = append(next, inference.NewTextMessage("assistant", "Loaded skill instructions (preserved through compression):\n\n"+strings.Join(skillNotes, "\n\n")))
	}
	next = append(next, preserved...)
	a.context = next
	a.lastPromptTokens = 0
	a.mu.Unlock()

	a.stats.AddCompression()
	a.recordSnapshot() // snapshots are recorded on compression (requirement 044)
	if dbg := debug.Default(); dbg != nil {
		dbg.Logf("compressed context: %d messages -> %d messages", len(msgs), len(next))
	}
	a.emit(Event{Type: EventCompressed, Text: fmt.Sprintf("Context compressed (%d messages -> %d)", len(msgs), len(next))})
	return nil
}

// summarize asks the inference backend for a summary of the given messages.
func (a *Agent) summarize(ctx context.Context, msgs []*inference.Message) (string, error) {
	req := &inference.Request{
		Model:    a.cfg.Model,
		Messages: append([]*inference.Message{inference.NewTextMessage("system", "You summarize conversations for a coding agent. Produce a compact, information dense summary.")}, msgs...),
	}
	req.Messages = append(req.Messages, inference.NewTextMessage("user", SummaryRequestPrompt()))
	resp, err := a.client.ChatCompletion(ctx, req, nil)
	if err != nil {
		return "", err
	}
	a.stats.AddUsage(resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.CachedTokens)
	text := strings.TrimSpace(resp.Content)
	if text == "" {
		return "", fmt.Errorf("the inference backend returned an empty summary")
	}
	return text, nil
}

// mechanicalSummary builds a summary without the inference backend.
func mechanicalSummary(msgs []*inference.Message) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%d earlier messages were removed to fit the context window.\n", len(msgs)))
	for _, m := range msgs {
		switch m.Role {
		case "assistant":
			for _, tc := range m.ToolCalls {
				b.WriteString(fmt.Sprintf("- called %s %s\n", tc.Function.Name, truncate(tc.Function.Arguments, 160)))
			}
			if t := strings.TrimSpace(m.Text()); t != "" {
				b.WriteString("- said: " + truncate(t, 160) + "\n")
			}
		case "user":
			b.WriteString("- user: " + truncate(strings.TrimSpace(m.Text()), 160) + "\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// jsonCompact renders a value compactly for logging.
func jsonCompact(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}
