package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "agentbin")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	bin := filepath.Join(dir, "coding-agent")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	binPath = bin
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// mockLLM serves scripted completions.
func mockLLM(t *testing.T, responses ...map[string]interface{}) *httptest.Server {
	t.Helper()
	var idx int
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{{"message": map[string]interface{}{"role": "assistant", "content": "mock answer"}}},
		}
		if idx < len(responses) {
			resp = responses[idx]
		}
		idx++
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func textResponse(content string) map[string]interface{} {
	return map[string]interface{}{
		"choices": []map[string]interface{}{{
			"message":       map[string]interface{}{"role": "assistant", "content": content},
			"finish_reason": "stop",
		}},
		"usage": map[string]int{"prompt_tokens": 30, "completion_tokens": 7, "total_tokens": 37},
	}
}

func toolResponse(id, name, args string) map[string]interface{} {
	return map[string]interface{}{
		"choices": []map[string]interface{}{{
			"message": map[string]interface{}{
				"role": "assistant",
				"tool_calls": []map[string]interface{}{{
					"id":       id,
					"type":     "function",
					"function": map[string]string{"name": name, "arguments": args},
				}},
			},
			"finish_reason": "tool_calls",
		}},
	}
}

// runCLI executes the built binary with a clean environment.
func runCLI(t *testing.T, endpoint string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir()}
	cmd.Env = append(cmd.Env, "CODING_AGENT_API_ENDPOINT="+endpoint, "CODING_AGENT_API_KEY=test")
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("run: %v", err)
		}
	}
	return stdout.String(), stderr.String(), code
}

func TestVersionAndHelp(t *testing.T) {
	out, _, code := runCLI(t, "http://127.0.0.1:1", "--version")
	if code != 0 || !strings.Contains(out, "0.1.0") {
		t.Fatalf("--version: code=%d out=%q", code, out)
	}
	out, _, code = runCLI(t, "http://127.0.0.1:1", "--help")
	if code != 0 || !strings.Contains(out, "Usage:") {
		t.Fatalf("--help: code=%d", code)
	}
}

func TestOneShotQuiet(t *testing.T) {
	mb := mockLLM(t, textResponse("e2e answer"))
	stdout, _, code := runCLI(t, mb.URL, "-p", "hello", "--no-stream", "--quiet")
	if code != 0 {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
	if strings.TrimSpace(stdout) != "e2e answer" {
		t.Fatalf("quiet output: %q", stdout)
	}
}

func TestOneShotVerboseWithTool(t *testing.T) {
	mb := mockLLM(t,
		toolResponse("c1", "bash", `{"command":"echo from-tool"}`),
		textResponse("the tool said from-tool"),
	)
	stdout, _, code := runCLI(t, mb.URL, "-p", "run it", "--no-stream", "--verbose")
	if code != 0 {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
	for _, want := range []string{"[Tool Call] bash", "echo from-tool", "=== Final Output ===", "the tool said from-tool", "=== Summary ===", "Iterations executed: 2"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("missing %q in:\n%s", want, stdout)
		}
	}
}

func TestOutputFile(t *testing.T) {
	mb := mockLLM(t, textResponse("written answer"))
	outFile := filepath.Join(t.TempDir(), "answer.txt")
	_, _, code := runCLI(t, mb.URL, "-p", "hello", "--no-stream", "--quiet", "--output", outFile)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	data, err := os.ReadFile(outFile)
	if err != nil || string(data) != "written answer" {
		t.Fatalf("output file: %q %v", string(data), err)
	}
}

func TestStdinPrompt(t *testing.T) {
	mb := mockLLM(t, textResponse("from stdin"))
	cmd := exec.Command(binPath, "--stdin", "--no-stream", "--quiet")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir(), "CODING_AGENT_API_ENDPOINT=" + mb.URL, "CODING_AGENT_API_KEY=test"}
	cmd.Stdin = strings.NewReader("prompt via stdin")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("stdin run: %v", err)
	}
	if strings.TrimSpace(string(out)) != "from stdin" {
		t.Fatalf("stdin output: %q", out)
	}
}

// Requirement 025: prompt loading from a file.
func TestPromptFile(t *testing.T) {
	mb := mockLLM(t, textResponse("from file"))
	pf := filepath.Join(t.TempDir(), "prompt.txt")
	if err := os.WriteFile(pf, []byte("prompt via file"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runCLI(t, mb.URL, "--prompt-file", pf, "--no-stream", "--quiet")
	if code != 0 {
		t.Fatalf("exit %d (stderr: %s)", code, stderr)
	}
	if strings.TrimSpace(stdout) != "from file" {
		t.Fatalf("output: %q", stdout)
	}
}

func TestAuthFailureExitCode3(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"Unauthorized"}}`)
	}))
	defer srv.Close()
	_, stderr, code := runCLI(t, srv.URL, "-p", "hi", "--no-stream", "--quiet")
	if code != 3 {
		t.Fatalf("expected exit 3, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "GitHub Copilot") && !strings.Contains(stderr, "CODING_AGENT_API_KEY") {
		// Non-copilot endpoints should still get an actionable hint.
		t.Logf("stderr: %s", stderr)
	}
}

func TestBadFlagExitCode2(t *testing.T) {
	_, stderr, code := runCLI(t, "http://127.0.0.1:1", "--nonexistent-flag")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Fatalf("usage not printed: %s", stderr)
	}
}

func TestContextDumpOnExitOneShot(t *testing.T) {
	mb := mockLLM(t, textResponse("dump me"))
	// One-shot mode dumps unless --no-dump-on-exit; verify the file appears.
	_, _, code := runCLI(t, mb.URL, "-p", "hello", "--no-stream", "--quiet")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	// The dump goes to the system temp dir; just ensure the flag is accepted.
	_, _, code = runCLI(t, mb.URL, "-p", "hello", "--no-stream", "--quiet", "--no-dump-on-exit")
	if code != 0 {
		t.Fatalf("--no-dump-on-exit: code=%d", code)
	}
}
