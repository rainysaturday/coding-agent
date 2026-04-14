package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"coding-agent/implementation/types"
)

type Executor struct{}

func NewExecutor() *Executor { return &Executor{} }

func Definitions() []types.ToolDefinition {
	return []types.ToolDefinition{
		def("bash", "Execute bash commands", map[string]any{"command": map[string]any{"type": "string"}}),
		def("read_file", "Read file contents", map[string]any{"path": map[string]any{"type": "string"}}),
		def("write_file", "Write file contents", map[string]any{"path": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"}}),
		def("read_lines", "Read a line range from a file", map[string]any{"path": map[string]any{"type": "string"}, "start": map[string]any{"type": "integer"}, "end": map[string]any{"type": "integer"}}),
		def("insert_lines", "Insert lines into file before a 1-indexed line", map[string]any{"path": map[string]any{"type": "string"}, "line": map[string]any{"type": "integer"}, "text": map[string]any{"type": "string"}}),
		def("replace_text", "Replace text in file", map[string]any{"path": map[string]any{"type": "string"}, "find": map[string]any{"type": "string"}, "replace": map[string]any{"type": "string"}, "count": map[string]any{"type": "integer"}}),
		def("patch", "Apply a unified diff patch", map[string]any{"patch": map[string]any{"type": "string"}}),
	}
}

func def(name, desc string, props map[string]any) types.ToolDefinition {
	return types.ToolDefinition{Type: "function", Function: map[string]any{
		"name":        name,
		"description": desc,
		"parameters": map[string]any{
			"type":       "object",
			"properties": props,
		},
	}}
}

func (e *Executor) Execute(ctx context.Context, call types.ToolCall) (string, error) {
	_ = ctx
	switch call.Function.Name {
	case "bash":
		var p struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &p); err != nil {
			return "", err
		}
		cmd := exec.Command("bash", "-lc", p.Command)
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		err := cmd.Run()
		if err != nil {
			if ee := (&exec.ExitError{}); errors.As(err, &ee) {
				return fmt.Sprintf("exit code %d\n%s", ee.ExitCode(), buf.String()), nil
			}
			return "", err
		}
		return buf.String(), nil
	case "read_file":
		var p struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &p); err != nil {
			return "", err
		}
		b, err := os.ReadFile(p.Path)
		if err != nil {
			return "", err
		}
		return string(b), nil
	case "write_file":
		var p struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &p); err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(p.Path), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(p.Path, []byte(p.Content), 0o644); err != nil {
			return "", err
		}
		return "ok", nil
	case "read_lines":
		var p struct {
			Path  string `json:"path"`
			Start int    `json:"start"`
			End   int    `json:"end"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &p); err != nil {
			return "", err
		}
		if p.Start <= 0 || p.End < p.Start {
			return "", fmt.Errorf("invalid range")
		}
		b, err := os.ReadFile(p.Path)
		if err != nil {
			return "", err
		}
		lines := splitLines(string(b))
		if p.Start > len(lines) {
			return "", nil
		}
		if p.End > len(lines) {
			p.End = len(lines)
		}
		return strings.Join(lines[p.Start-1:p.End], "\n"), nil
	case "insert_lines":
		var p struct {
			Path string `json:"path"`
			Line int    `json:"line"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &p); err != nil {
			return "", err
		}
		if p.Line <= 0 {
			return "", fmt.Errorf("line must be >= 1")
		}
		b, _ := os.ReadFile(p.Path)
		lines := splitLines(string(b))
		idx := p.Line - 1
		if idx > len(lines) {
			idx = len(lines)
		}
		insert := splitLines(p.Text)
		out := append([]string{}, lines[:idx]...)
		out = append(out, insert...)
		out = append(out, lines[idx:]...)
		if err := os.MkdirAll(filepath.Dir(p.Path), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(p.Path, []byte(strings.Join(out, "\n")), 0o644); err != nil {
			return "", err
		}
		return "ok", nil
	case "replace_text":
		var p struct {
			Path    string `json:"path"`
			Find    string `json:"find"`
			Replace string `json:"replace"`
			Count   int    `json:"count"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &p); err != nil {
			return "", err
		}
		if p.Count == 0 {
			p.Count = -1
		}
		b, err := os.ReadFile(p.Path)
		if err != nil {
			return "", err
		}
		orig := string(b)
		newText := strings.Replace(orig, p.Find, p.Replace, p.Count)
		if err := os.WriteFile(p.Path, []byte(newText), 0o644); err != nil {
			return "", err
		}
		replaced := strings.Count(orig, p.Find)
		if p.Count > 0 && replaced > p.Count {
			replaced = p.Count
		}
		return fmt.Sprintf("replaced=%d", replaced), nil
	case "patch":
		var p struct {
			Patch string `json:"patch"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &p); err != nil {
			return "", err
		}
		return applyUnifiedPatch(p.Patch)
	default:
		return "", fmt.Errorf("unknown tool: %s", call.Function.Name)
	}
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return []string{}
	}
	return strings.Split(s, "\n")
}

func applyUnifiedPatch(patch string) (string, error) {
	lines := splitLines(patch)
	if len(lines) == 0 {
		return "", fmt.Errorf("empty patch")
	}
	var file string
	for _, l := range lines {
		if strings.HasPrefix(l, "+++ ") {
			parts := strings.Fields(l)
			if len(parts) >= 2 {
				file = strings.TrimPrefix(parts[1], "b/")
				break
			}
		}
	}
	if file == "" {
		return "", fmt.Errorf("missing target file in patch")
	}
	origBytes, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	orig := splitLines(string(origBytes))
	result := make([]string, 0, len(orig)+16)
	i := 0

	for idx := 0; idx < len(lines); idx++ {
		line := lines[idx]
		if !strings.HasPrefix(line, "@@") {
			continue
		}
		for idx+1 < len(lines) {
			next := lines[idx+1]
			if strings.HasPrefix(next, "@@") {
				break
			}
			if strings.HasPrefix(next, "--- ") || strings.HasPrefix(next, "+++ ") {
				idx++
				continue
			}
			idx++
			switch {
			case strings.HasPrefix(next, " "):
				want := strings.TrimPrefix(next, " ")
				if i >= len(orig) || orig[i] != want {
					return "", fmt.Errorf("context mismatch at line %d", i+1)
				}
				result = append(result, orig[i])
				i++
			case strings.HasPrefix(next, "-"):
				want := strings.TrimPrefix(next, "-")
				if i >= len(orig) || orig[i] != want {
					return "", fmt.Errorf("delete mismatch at line %d", i+1)
				}
				i++
			case strings.HasPrefix(next, "+"):
				result = append(result, strings.TrimPrefix(next, "+"))
			case strings.HasPrefix(next, "\\"):
				// no-op metadata line
			default:
				break
			}
		}
	}
	result = append(result, orig[i:]...)
	if err := os.WriteFile(file, []byte(strings.Join(result, "\n")), 0o644); err != nil {
		return "", err
	}
	return "patch applied", nil
}

func readFileLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	var lines []string
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	return lines, s.Err()
}
