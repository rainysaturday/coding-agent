package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coding-agent/coding-agent/inference"
)

func newRun(t *testing.T) *RunContext {
	t.Helper()
	return &RunContext{CWD: t.TempDir(), MaxOutputLines: 200, BashTimeoutMS: 5000}
}

func call(t *testing.T, reg *Registry, name, args string) string {
	t.Helper()
	res, _ := reg.Execute(context.Background(), inference.ToolCall{
		ID:       "c1",
		Type:     "function",
		Function: inference.FunctionCall{Name: name, Arguments: args},
	})
	return res
}

// callFailed executes a call that is expected to fail and returns the raw
// error message.
func callFailed(t *testing.T, reg *Registry, name, args string) string {
	t.Helper()
	res, failed := reg.Execute(context.Background(), inference.ToolCall{
		ID:       "c1",
		Type:     "function",
		Function: inference.FunctionCall{Name: name, Arguments: args},
	})
	if !failed {
		t.Fatalf("expected %s(%s) to fail, got: %q", name, args, res)
	}
	return res
}

func TestBashTool(t *testing.T) {
	run := newRun(t)
	reg := NewRegistry(run)
	reg.Register(NewBashTool(run))

	out := call(t, reg, "bash", `{"command":"echo hello"}`)
	if strings.TrimSpace(out) != "hello" {
		t.Fatalf("echo: %q", out)
	}
	out = call(t, reg, "bash", `{"command":"echo err >&2"}`)
	if !strings.Contains(out, "stderr:") || !strings.Contains(out, "err") {
		t.Fatalf("stderr: %q", out)
	}
	out = callFailed(t, reg, "bash", `{"command":"exit 3"}`)
	if !strings.Contains(out, "exit status 3") {
		t.Fatalf("exit status: %q", out)
	}
	out = callFailed(t, reg, "bash", `{"command":"echo oops >&2; exit 3"}`)
	if !strings.Contains(out, "exit status 3") || !strings.Contains(out, "oops") {
		t.Fatalf("exit status with output: %q", out)
	}
	out = callFailed(t, reg, "bash", `{"command":"sleep 5","timeout":200}`)
	if !strings.Contains(out, "timed out after 200ms") || !strings.Contains(out, "Consider increasing the timeout") {
		t.Fatalf("timeout: %q", out)
	}
	// max_output_lines keeps the last N lines.
	out = call(t, reg, "bash", `{"command":"printf 'a\\nb\\nc\\nd\\n'","max_output_lines":2}`)
	if !strings.Contains(out, "showing last 2 of 4 lines") || !strings.HasSuffix(out, "c\nd") {
		t.Fatalf("truncation: %q", out)
	}
	// Command runs in the configured CWD.
	out = call(t, reg, "bash", `{"command":"pwd"}`)
	if strings.TrimSpace(out) != run.CWD {
		t.Fatalf("cwd: %q want %q", out, run.CWD)
	}
}

// TestErrorCategories checks the canonical error message categories from
// requirement 018: missing parameters, file not found, permission errors,
// invalid input and command execution errors.
func TestErrorCategories(t *testing.T) {
	run := newRun(t)
	reg := NewRegistry(run)
	reg.Register(NewBashTool(run))
	reg.Register(NewReadFileTool(run))
	reg.Register(NewReadLinesTool(run))

	// Missing parameters.
	if msg := callFailed(t, reg, "read_file", `{}`); !strings.Contains(msg, "missing required parameter: path") {
		t.Fatalf("missing param: %q", msg)
	}
	if msg := callFailed(t, reg, "bash", `{}`); !strings.Contains(msg, "missing required parameter: command") {
		t.Fatalf("missing param: %q", msg)
	}
	// File not found.
	if msg := callFailed(t, reg, "read_file", `{"path":"nope/missing.txt"}`); !strings.Contains(msg, "no such file") {
		t.Fatalf("file not found: %q", msg)
	}
	// Permission denied (skip when running as root).
	if os.Geteuid() != 0 {
		path := filepath.Join(run.CWD, "locked.txt")
		if err := os.WriteFile(path, []byte("secret"), 0o000); err != nil {
			t.Fatal(err)
		}
		msg := callFailed(t, reg, "read_file", `{"path":"locked.txt"}`)
		if !strings.Contains(msg, "permission denied") {
			t.Fatalf("permission: %q", msg)
		}
	}
	// Invalid input.
	if err := os.WriteFile(filepath.Join(run.CWD, "range.txt"), []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if msg := callFailed(t, reg, "read_lines", `{"path":"range.txt","start":0}`); !strings.Contains(msg, "start") {
		t.Fatalf("invalid start: %q", msg)
	}
	// Command execution error: shell reports "command not found" (exit 127).
	msg := callFailed(t, reg, "bash", `{"command":"nonexistent_command_xyz"}`)
	if !strings.Contains(msg, "exit status 127") || !strings.Contains(msg, "not found") {
		t.Fatalf("command not found: %q", msg)
	}
	// Unknown tool names are reported with the available tool list.
	if msg := callFailed(t, reg, "no_such_tool", `{}`); !strings.Contains(msg, "unknown tool") {
		t.Fatalf("unknown tool: %q", msg)
	}
}

func TestFileToolsRoundTrip(t *testing.T) {
	run := newRun(t)
	reg := NewRegistry(run)
	reg.Register(NewWriteFileTool(run))
	reg.Register(NewReadFileTool(run))
	reg.Register(NewReadLinesTool(run))
	reg.Register(NewInsertLinesTool(run))
	reg.Register(NewReplaceTextTool(run))
	reg.Register(NewMoveTextTool(run))

	path := filepath.Join(run.CWD, "f.txt")

	out := call(t, reg, "write_file", `{"path":"`+path+`","content":"line1\nline2\nline3\n"}`)
	if !strings.Contains(out, "Successfully") {
		t.Fatalf("write: %q", out)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "line1\nline2\nline3\n" {
		t.Fatalf("file on disk: %q %v", string(data), err)
	}

	out = call(t, reg, "read_file", `{"path":"`+path+`"}`)
	if !strings.Contains(out, "line2") {
		t.Fatalf("read: %q", out)
	}
	out = call(t, reg, "read_lines", `{"path":"`+path+`","start":2,"end":2}`)
	if strings.TrimSpace(out) != "2: line2" {
		t.Fatalf("read_lines: %q", out)
	}

	out = call(t, reg, "insert_lines", `{"path":"`+path+`","line":2,"lines":"inserted"}`)
	if !strings.Contains(out, "inserted") {
		t.Fatalf("insert: %q", out)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "line1\ninserted\nline2\nline3\n" {
		t.Fatalf("after insert: %q", string(data))
	}

	out = call(t, reg, "replace_text", `{"path":"`+path+`","search":"line2","replace":"LINE2","count":1}`)
	if !strings.Contains(out, "Replaced") {
		t.Fatalf("replace: %q", out)
	}
	data, _ = os.ReadFile(path)
	if !strings.Contains(string(data), "LINE2") {
		t.Fatalf("after replace: %q", string(data))
	}

	// move_text moves lines to another file (created on demand).
	target := filepath.Join(run.CWD, "sub", "moved.txt")
	out = call(t, reg, "move_text", `{"source_path":"`+path+`","source_start":1,"source_end":1,"target_path":"`+target+`","target_line":1}`)
	if !strings.Contains(out, "Moved") {
		t.Fatalf("move: %q", out)
	}
	data, err = os.ReadFile(target)
	if err != nil || !strings.Contains(string(data), "line1") {
		t.Fatalf("moved file: %q %v", string(data), err)
	}
	data, _ = os.ReadFile(path)
	if strings.Contains(string(data), "line1") {
		t.Fatalf("source still has moved line: %q", string(data))
	}
}

func TestReadFileRejectsBinary(t *testing.T) {
	run := newRun(t)
	reg := NewRegistry(run)
	reg.Register(NewWriteFileTool(run))
	reg.Register(NewReadFileTool(run))
	path := filepath.Join(run.CWD, "bin.dat")
	if err := os.WriteFile(path, []byte{0, 1, 2, 3, 'h', 0, 0}, 0o644); err != nil {
		t.Fatal(err)
	}
	out := callFailed(t, reg, "read_file", `{"path":"`+path+`"}`)
	if !strings.Contains(out, "binary") {
		t.Fatalf("binary read: %q", out)
	}
}

func TestListFilesAndGrep(t *testing.T) {
	run := newRun(t)
	reg := NewRegistry(run)
	reg.Register(NewListFilesTool(run))
	reg.Register(NewGrepTool(run))

	if err := os.MkdirAll(filepath.Join(run.CWD, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(run.CWD, "a.txt"), []byte("needle here\nsecond\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(run.CWD, "d", "b.txt"), []byte("no match\nneedle there\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := call(t, reg, "list_files", `{"path":"`+run.CWD+`"}`)
	if !strings.Contains(out, "a.txt") || !strings.Contains(out, "d") {
		t.Fatalf("list: %q", out)
	}

	out = call(t, reg, "grep", `{"pattern":"needle","path":"`+run.CWD+`","flags":["-r","-n"]}`)
	if !strings.Contains(out, "a.txt") || !strings.Contains(out, "b.txt") {
		t.Fatalf("recursive grep: %q", out)
	}
	out = call(t, reg, "grep", `{"pattern":"needle","path":"`+run.CWD+`","flags":["-l"]}`)
	if strings.Contains(out, "needle here") {
		t.Fatalf("-l should not print content: %q", out)
	}
	out = call(t, reg, "grep", `{"pattern":"nope","path":"`+run.CWD+`"}`)
	if !strings.Contains(out, "no match") && !strings.Contains(out, "No matches") {
		t.Fatalf("no-match message: %q", out)
	}
}

func TestTodoStore(t *testing.T) {
	s := NewTodoStore()
	a := s.Add("first")
	b := s.Add("second")
	if a.ID != 1 || b.ID != 2 {
		t.Fatalf("ids: %d %d", a.ID, b.ID)
	}
	if _, err := s.Complete(a.ID); err != nil {
		t.Fatal(err)
	}
	if s.List()[0].Status != "completed" {
		t.Fatalf("status: %s", s.List()[0].Status)
	}
	if _, err := s.Complete(99); err == nil {
		t.Fatal("expected error for unknown id")
	}
	if _, err := s.Remove(b.ID); err != nil {
		t.Fatal(err)
	}
	if len(s.List()) != 1 {
		t.Fatalf("after remove: %d", len(s.List()))
	}
	rendered := s.Render()
	if !strings.Contains(rendered, "first") {
		t.Fatalf("render: %q", rendered)
	}
}

func TestRegistryReadOnly(t *testing.T) {
	run := newRun(t)
	run.ReadOnly = true
	reg := NewRegistry(run)
	reg.Register(NewBashTool(run))
	reg.Register(NewReadFileTool(run))

	// Non-read-only tools must not be advertised.
	for _, d := range reg.Definitions() {
		if d.Function.Name == "bash" {
			t.Fatal("bash should be filtered from definitions in read-only mode")
		}
	}
	out := call(t, reg, "bash", `{"command":"echo hi"}`)
	if !strings.Contains(out, "read-only mode") {
		t.Fatalf("read-only execute: %q", out)
	}
	// Read-only tools still work.
	path := filepath.Join(run.CWD, "x.txt")
	_ = os.WriteFile(path, []byte("ok"), 0o644)
	out = call(t, reg, "read_file", `{"path":"`+path+`"}`)
	if out != "ok" {
		t.Fatalf("read_file in read-only mode: %q", out)
	}
}

func TestRegistryUnknownToolAndBadArgs(t *testing.T) {
	run := newRun(t)
	reg := NewRegistry(run)
	reg.Register(NewBashTool(run))
	out := call(t, reg, "nope", `{}`)
	if !strings.Contains(out, "unknown tool") || !strings.Contains(out, "bash") {
		t.Fatalf("unknown tool: %q", out)
	}
	out = call(t, reg, "bash", `{invalid`)
	if !strings.Contains(out, "invalid JSON") {
		t.Fatalf("bad args: %q", out)
	}
}

func TestGitTools(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	run := newRun(t)
	repo := run.CWD
	mustGit(t, repo, "init", "-q")
	mustGit(t, repo, "config", "user.email", "t@t")
	mustGit(t, repo, "config", "user.name", "T")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "add", "a.txt")
	mustGit(t, repo, "commit", "-q", "-m", "initial commit")

	reg := NewRegistry(run)
	reg.Register(NewGitLogTool(run))
	reg.Register(NewGitShowTool(run))
	reg.Register(NewGitDiffTool(run))

	out := call(t, reg, "git_log", `{"path":"`+repo+`"}`)
	if !strings.Contains(out, "initial commit") {
		t.Fatalf("git_log: %q", out)
	}
	out = call(t, reg, "git_show", `{"path":"`+repo+`","commit":"HEAD"}`)
	if !strings.Contains(out, "initial commit") {
		t.Fatalf("git_show: %q", out)
	}
	// Dirty working tree shows in diff.
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out = call(t, reg, "git_diff", `{"path":"`+repo+`"}`)
	if !strings.Contains(out, "+two") {
		t.Fatalf("git_diff: %q", out)
	}
}

func TestGitFlagSanitization(t *testing.T) {
	if _, err := sanitizeFlags([]string{"stat", "output=/tmp/pwn"}); err == nil {
		t.Fatal("expected unsafe flag rejection")
	}
	flags, err := sanitizeFlags([]string{"stat", "no-color"})
	if err != nil {
		t.Fatal(err)
	}
	if len(flags) != 2 || flags[0] != "-stat" {
		t.Fatalf("flags: %v", flags)
	}
}

func TestArgHelpers(t *testing.T) {
	var m map[string]interface{}
	_ = json.Unmarshal([]byte(`{"s":"x","i":7,"b":true,"arr":["a","b"]}`), &m)
	if getString(m, "s") != "x" {
		t.Fatal("getString")
	}
	if getInt(m, "i", 0) != 7 || getInt(m, "missing", 5) != 5 {
		t.Fatal("getInt")
	}
	// JSON numbers decode as float64.
	if getInt(map[string]interface{}{"i": float64(9)}, "i", 0) != 9 {
		t.Fatal("getInt float64")
	}
	if !getBool(m, "b", false) {
		t.Fatal("getBool")
	}
	if got := getStringSlice(m, "arr"); len(got) != 2 || got[0] != "a" {
		t.Fatalf("getStringSlice: %v", got)
	}
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

type fakeActivator struct{ names []string }

func (f *fakeActivator) Activate(name string) (string, error) {
	for _, n := range f.names {
		if n == name {
			return "--- BEGIN SKILL INSTRUCTIONS ---\nbody of " + name, nil
		}
	}
	return "", fmt.Errorf("unknown skill %q", name)
}
func (f *fakeActivator) Catalog() string { return "AVAILABLE SKILLS:\n" }
func (f *fakeActivator) Names() []string { return f.names }

func TestActivateSkillSchemaEnum(t *testing.T) {
	reg := NewRegistry(&RunContext{CWD: t.TempDir()})
	reg.SetSkillActivator(&fakeActivator{names: []string{"alpha", "beta"}})
	tool, ok := reg.Get("activate_skill")
	if !ok {
		t.Fatal("activate_skill not registered")
	}
	props, _ := tool.Schema()["properties"].(map[string]interface{})
	nameProp, _ := props["name"].(map[string]interface{})
	enum, ok := nameProp["enum"].([]string)
	if !ok || len(enum) != 2 || enum[0] != "alpha" || enum[1] != "beta" {
		t.Fatalf("schema enum constraint missing: %v", nameProp)
	}
	// Without an activator the tool is unregistered entirely.
	reg.SetSkillActivator(nil)
	if _, ok := reg.Get("activate_skill"); ok {
		t.Fatal("activate_skill should be unregistered when no skills exist")
	}
}

// Requirement 040: subagent tool takes prompt (required) + persona (optional).
type fakeSubagentRunner struct {
	gotPrompt  string
	gotPersona string
	out        string
	err        error
}

func (f *fakeSubagentRunner) RunSubagent(ctx context.Context, prompt, persona string) (string, error) {
	f.gotPrompt, f.gotPersona = prompt, persona
	return f.out, f.err
}

func TestSubagentTool(t *testing.T) {
	reg := NewRegistry(newRun(t))
	fr := &fakeSubagentRunner{out: "Subagent completed.\n\nSummary:\nall done"}
	reg.SetSubagentRunner(fr)

	out := call(t, reg, "subagent", `{"prompt":"do the work","persona":"Security specialist"}`)
	if !strings.Contains(out, "all done") {
		t.Fatalf("subagent output: %q", out)
	}
	if fr.gotPrompt != "do the work" || fr.gotPersona != "Security specialist" {
		t.Fatalf("args: %q %q", fr.gotPrompt, fr.gotPersona)
	}
	// Schema requires prompt, accepts persona.
	defs := reg.Definitions()
	var found bool
	for _, d := range defs {
		if d.Function.Name == "subagent" {
			found = true
			props := d.Function.Parameters["properties"].(map[string]interface{})
			if _, ok := props["prompt"]; !ok {
				t.Fatal("subagent schema missing prompt")
			}
			if _, ok := props["persona"]; !ok {
				t.Fatal("subagent schema missing persona")
			}
			req := d.Function.Parameters["required"].([]string)
			if len(req) != 1 || req[0] != "prompt" {
				t.Fatalf("subagent required: %v", req)
			}
		}
	}
	if !found {
		t.Fatal("subagent tool not registered")
	}
	// Missing prompt.
	if msg := callFailed(t, reg, "subagent", `{}`); !strings.Contains(msg, "missing required parameter: prompt") {
		t.Fatalf("missing prompt: %q", msg)
	}
	// Runner errors surface clearly.
	fr2 := &fakeSubagentRunner{err: errors.New("subagent failed: boom")}
	reg.SetSubagentRunner(fr2)
	if msg := callFailed(t, reg, "subagent", `{"prompt":"x"}`); !strings.Contains(msg, "boom") {
		t.Fatalf("runner error: %q", msg)
	}
}

// Requirement 011/012/013 edge cases.
func TestFileToolEdgeCases(t *testing.T) {
	run := newRun(t)
	reg := NewRegistry(run)
	reg.Register(NewWriteFileTool(run))
	reg.Register(NewReadLinesTool(run))
	reg.Register(NewInsertLinesTool(run))
	reg.Register(NewReplaceTextTool(run))

	path := filepath.Join(run.CWD, "edge.txt")
	call(t, reg, "write_file", `{"path":"`+path+`","content":"a\nb\nc\n"}`)

	// read_lines: end beyond EOF reads to end.
	out := call(t, reg, "read_lines", `{"path":"`+path+`","start":2,"end":99}`)
	if strings.TrimSpace(out) != "2: b\n3: c" {
		t.Fatalf("end beyond EOF: %q", out)
	}
	// read_lines: start beyond EOF is graceful.
	out = call(t, reg, "read_lines", `{"path":"`+path+`","start":50,"end":60}`)
	if !strings.Contains(out, "beyond end of file") {
		t.Fatalf("start beyond EOF: %q", out)
	}
	// read_lines: start > end is an error.
	callFailed(t, reg, "read_lines", `{"path":"`+path+`","start":3,"end":1}`)
	// read_lines: missing file.
	callFailed(t, reg, "read_lines", `{"path":"`+filepath.Join(run.CWD, "nope.txt")+`","start":1,"end":2}`)

	// insert_lines: beyond EOF appends; missing file creates it.
	out = call(t, reg, "insert_lines", `{"path":"`+path+`","line":99,"lines":"d"}`)
	data, _ := os.ReadFile(path)
	if !strings.Contains(out, "append") && !strings.Contains(out, "99") {
		t.Logf("insert beyond EOF message: %q", out)
	}
	if string(data) != "a\nb\nc\nd\n" {
		t.Fatalf("append beyond EOF: %q", string(data))
	}
	newPath := filepath.Join(run.CWD, "created", "by-insert.txt")
	call(t, reg, "insert_lines", `{"path":"`+newPath+`","line":1,"lines":"first"}`)
	data, err := os.ReadFile(newPath)
	if err != nil || string(data) != "first\n" {
		t.Fatalf("insert creates file: %q %v", string(data), err)
	}

	// replace_text: count -1 replaces all; missing pattern errors.
	call(t, reg, "write_file", `{"path":"`+path+`","content":"x x x\n"}`)
	out = call(t, reg, "replace_text", `{"path":"`+path+`","search":"x","replace":"y","count":-1}`)
	data, _ = os.ReadFile(path)
	if string(data) != "y y y\n" {
		t.Fatalf("replace all: %q (%q)", string(data), out)
	}
	callFailed(t, reg, "replace_text", `{"path":"`+path+`","search":"zzz","replace":"q"}`)
	// replace_text default count is 1.
	call(t, reg, "write_file", `{"path":"`+path+`","content":"x x\n"}`)
	call(t, reg, "replace_text", `{"path":"`+path+`","search":"x","replace":"y"}`)
	data, _ = os.ReadFile(path)
	if string(data) != "y x\n" {
		t.Fatalf("default count: %q", string(data))
	}
}

// Requirement 005: read_file rejects files over 20KB with a clear message.
func TestReadFileSizeLimit(t *testing.T) {
	run := newRun(t)
	reg := NewRegistry(run)
	reg.Register(NewReadFileTool(run))
	path := filepath.Join(run.CWD, "big.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("a", 20481)), 0o644); err != nil {
		t.Fatal(err)
	}
	msg := callFailed(t, reg, "read_file", `{"path":"`+path+`"}`)
	if !strings.Contains(msg, "too large") || !strings.Contains(msg, "read_lines") {
		t.Fatalf("size limit: %q", msg)
	}
	// Exactly at the limit is fine.
	if err := os.WriteFile(path, []byte(strings.Repeat("a", 20480)), 0o644); err != nil {
		t.Fatal(err)
	}
	out := call(t, reg, "read_file", `{"path":"`+path+`"}`)
	if len(out) != 20480 {
		t.Fatalf("at limit: len=%d", len(out))
	}
}

// Requirement 034: grep filters by file extension/pattern.
func TestGrepIncludeFilter(t *testing.T) {
	run := newRun(t)
	reg := NewRegistry(run)
	reg.Register(NewGrepTool(run))
	for name, content := range map[string]string{"a.go": "needle go\n", "b.md": "needle md\n"} {
		if err := os.WriteFile(filepath.Join(run.CWD, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := call(t, reg, "grep", `{"pattern":"needle","path":"`+run.CWD+`","include":["*.go"]}`)
	if !strings.Contains(out, "a.go") || strings.Contains(out, "b.md") {
		t.Fatalf("include param: %q", out)
	}
	out = call(t, reg, "grep", `{"pattern":"needle","path":"`+run.CWD+`","flags":["include=*.md"]}`)
	if !strings.Contains(out, "b.md") || strings.Contains(out, "a.go") {
		t.Fatalf("include flag: %q", out)
	}
	out = call(t, reg, "grep", `{"pattern":"needle","path":"`+run.CWD+`"}`)
	if !strings.Contains(out, "a.go") || !strings.Contains(out, "b.md") {
		t.Fatalf("no filter: %q", out)
	}
}
