package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"coding-agent/implementation/types"
)

func TestReplaceTextCount(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("a a a"), 0o644); err != nil {
		t.Fatal(err)
	}

	args, _ := json.Marshal(map[string]interface{}{
		"path": p, "find": "a", "replace": "b", "count": 1,
	})
	call := types.ToolCall{Function: types.ToolCallFunction{Name: "replace_text", Arguments: string(args)}}
	_, err := NewExecutor().Execute(context.Background(), call)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "b a a" {
		t.Fatalf("unexpected content: %q", string(b))
	}
}

func TestReadLinesRange(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(p, []byte("1\n2\n3\n4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]interface{}{
		"path": p, "start": 2, "end": 3,
	})
	call := types.ToolCall{Function: types.ToolCallFunction{Name: "read_lines", Arguments: string(args)}}
	out, err := NewExecutor().Execute(context.Background(), call)
	if err != nil {
		t.Fatal(err)
	}
	if out != "2\n3" {
		t.Fatalf("unexpected output: %q", out)
	}
}
