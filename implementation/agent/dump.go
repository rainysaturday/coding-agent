package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/coding-agent/coding-agent/inference"
)

// DumpVersion is the current context dump format version.
const DumpVersion = 1

// DefaultDumpFileName is the base name for context dumps.
const DefaultDumpFileName = "coding-agent-context.json"

// StatsInfo holds cumulative statistics at a point in time.
type StatsInfo struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	ToolCalls       int `json:"tool_calls"`
	FailedToolCalls int `json:"failed_tool_calls"`
}

// Iteration is a full snapshot of the context at a point in time. The first
// message is the system prompt.
type Iteration struct {
	Index    int                  `json:"index"`
	Messages []*inference.Message `json:"messages"`
	Stats    StatsInfo            `json:"stats"`
}

// SessionInfo holds session metadata.
type SessionInfo struct {
	StartTime        time.Time `json:"start_time"`
	Iterations       int       `json:"iterations"`
	CompressionCount int       `json:"compression_count"`
	Stats            StatsInfo `json:"stats"`
}

// ContextDump is a serializable conversation context (requirement 044).
type ContextDump struct {
	Version    int         `json:"version"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
	Session    SessionInfo `json:"session"`
	Iterations []Iteration `json:"iterations"`
}

func (s *Stats) info() StatsInfo {
	snap := s.Snapshot()
	return StatsInfo{
		InputTokens:     snap.InputTokens,
		OutputTokens:    snap.OutputTokens,
		ToolCalls:       snap.ToolCalls,
		FailedToolCalls: snap.FailedToolCalls,
	}
}

// recordSnapshot appends a context snapshot; snapshots are recorded on agent
// exit and on context compression.
func (a *Agent) recordSnapshot() {
	a.mu.Lock()
	defer a.mu.Unlock()
	snap := Iteration{
		Index:    a.stats.Snapshot().Iterations,
		Messages: a.messagesForRequestLocked(),
		Stats:    a.stats.info(),
	}
	a.snapshots = append(a.snapshots, snap)
}

// DumpContext writes the conversation context to the temp directory using a
// unique file name and returns the path.
func (a *Agent) DumpContext() (string, error) {
	a.recordSnapshot()
	dump := a.buildDump()
	path := uniqueDumpPath()
	data, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize context: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("failed to write %s: %w", path, err)
	}
	return path, nil
}

func (a *Agent) buildDump() *ContextDump {
	a.mu.Lock()
	defer a.mu.Unlock()
	snap := a.stats.Snapshot()
	return &ContextDump{
		Version:   DumpVersion,
		CreatedAt: a.stats.StartTime,
		UpdatedAt: time.Now(),
		Session: SessionInfo{
			StartTime:        a.stats.StartTime,
			Iterations:       snap.Iterations,
			CompressionCount: snap.CompressionCount,
			Stats:            a.stats.info(),
		},
		Iterations: a.snapshots,
	}
}

// uniqueDumpPath returns coding-agent-context.json in the system temp dir,
// adding a numeric suffix when the file already exists.
func uniqueDumpPath() string {
	dir := os.TempDir()
	path := filepath.Join(dir, DefaultDumpFileName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(DefaultDumpFileName)
	base := DefaultDumpFileName[:len(DefaultDumpFileName)-len(ext)]
	for i := 2; ; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

// LoadContext restores the conversation from a dump file, using only the last
// recorded iteration. It returns the number of restored messages and the last
// user prompt found.
func (a *Agent) LoadContext(path string) (int, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, "", fmt.Errorf("context file not found: %s", path)
		}
		return 0, "", fmt.Errorf("cannot read context file %s: %w", path, err)
	}
	var dump ContextDump
	if err := json.Unmarshal(data, &dump); err != nil {
		return 0, "", fmt.Errorf("malformed context file %s: %w", path, err)
	}
	if len(dump.Iterations) == 0 {
		return 0, "", fmt.Errorf("context file %s contains no iterations", path)
	}
	last := dump.Iterations[len(dump.Iterations)-1]
	if len(last.Messages) == 0 {
		return 0, "", fmt.Errorf("last iteration in %s contains no messages", path)
	}

	// Drop the leading system prompt: the current binary regenerates it.
	msgs := last.Messages
	if len(msgs) > 0 && msgs[0].Role == "system" {
		msgs = msgs[1:]
	}
	a.SetContext(msgs)

	lastUser := ""
	for _, m := range msgs {
		if m.Role == "user" {
			lastUser = m.Text()
		}
	}
	return len(msgs), lastUser, nil
}
