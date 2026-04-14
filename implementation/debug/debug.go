package debug

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"coding-agent/implementation/types"
)

type Logger struct {
	mu   sync.Mutex
	file *os.File
	on   bool
}

func New(enabled bool, path, version string) (*Logger, error) {
	l := &Logger{on: enabled}
	if !enabled {
		return l, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	l.file = f
	_, _ = fmt.Fprintf(l.file, "================================================================================\n")
	_, _ = fmt.Fprintf(l.file, "CODING AGENT DEBUG LOG\nSession: %s\nVersion: %s\n", time.Now().UTC().Format(time.RFC3339), version)
	_, _ = fmt.Fprintf(l.file, "================================================================================\n\n")
	return l, nil
}

func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

func (l *Logger) LogMessage(title string, m types.Message) {
	if !l.on || l.file == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	content := m.Content
	if m.ReasoningContent != "" {
		content += "\n[reasoning]\n" + m.ReasoningContent
	}
	content = redact(content)
	_, _ = fmt.Fprintf(l.file, "[%s] %s\n--------------------------------------------------------------------------------\n%s\n\n", time.Now().UTC().Format(time.RFC3339), title, content)
	if len(m.ToolCalls) > 0 {
		b, _ := json.MarshalIndent(m.ToolCalls, "", "  ")
		_, _ = fmt.Fprintf(l.file, "Tool Calls:\n%s\n\n", redact(string(b)))
	}
}

func (l *Logger) LogToolResult(toolName, callID string, success bool, output string) {
	if !l.on || l.file == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	status := "success"
	if !success {
		status = "error"
	}
	_, _ = fmt.Fprintf(l.file, "[%s] TOOL RESULT: %s\n--------------------------------------------------------------------------------\nTool ID: %s\nStatus: %s\nOutput:\n%s\n\n", time.Now().UTC().Format(time.RFC3339), toolName, callID, status, redact(output))
}

func (l *Logger) LogUsage(u types.Usage) {
	if !l.on || l.file == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintf(l.file, "[%s] TOKEN USAGE prompt=%d completion=%d total=%d\n\n", time.Now().UTC().Format(time.RFC3339), u.PromptTokens, u.CompletionTokens, u.TotalTokens)
}

func redact(s string) string {
	patterns := []string{"github_pat_", "ghp_", "sk-"}
	for _, p := range patterns {
		for {
			i := strings.Index(strings.ToLower(s), strings.ToLower(p))
			if i < 0 {
				break
			}
			j := i
			for j < len(s) {
				c := s[j]
				if c == ' ' || c == '\n' || c == '\t' || c == '"' {
					break
				}
				j++
			}
			s = s[:i] + "[REDACTED]" + s[j:]
		}
	}
	return s
}
