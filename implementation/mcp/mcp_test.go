package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coding-agent/coding-agent/config"
	"github.com/coding-agent/coding-agent/inference"
	"github.com/coding-agent/coding-agent/tools"
)

var fakeServerBin string

func TestMain(m *testing.M) {
	// Build the fake MCP server once for all tests.
	dir, err := os.MkdirTemp("", "fakemcp")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	bin := filepath.Join(dir, "fakemcp")
	build := exec.Command("go", "build", "-o", bin, "./testdata/fakemcp")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build fake mcp server: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	fakeServerBin = bin
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func stdioCfg(name string) config.MCPServerConfig {
	return config.MCPServerConfig{
		Name:      name,
		Transport: "stdio",
		Command:   fakeServerBin,
		Enabled:   true,
		Timeout:   10,
	}
}

func TestConnectAndListTools(t *testing.T) {
	cfg := config.NewDefault()
	cfg.MCPServers = map[string]config.MCPServerConfig{"fake": stdioCfg("fake")}
	mgr := NewManager(cfg)
	defer mgr.Close()

	errs := mgr.ConnectAll(context.Background())
	if len(errs) != 0 {
		t.Fatalf("connect errors: %v", errs)
	}
	st := mgr.Status()
	if len(st) != 1 || !st[0].Connected {
		t.Fatalf("status: %+v", st)
	}
	if len(st[0].Tools) != 3 {
		t.Fatalf("tools: %v", st[0].Tools)
	}
	// Status lists agent-side prefixed names.
	if st[0].Tools[0] != "mcp_fake_echo" {
		t.Fatalf("prefixed tool name: %v", st[0].Tools)
	}
}

func TestCallTool(t *testing.T) {
	cfg := config.NewDefault()
	cfg.MCPServers = map[string]config.MCPServerConfig{"fake": stdioCfg("fake")}
	mgr := NewManager(cfg)
	defer mgr.Close()
	if errs := mgr.ConnectAll(context.Background()); len(errs) != 0 {
		t.Fatalf("connect: %v", errs)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := mgr.CallTool(ctx, "fake", "echo", map[string]interface{}{"text": "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "echo: hi" {
		t.Fatalf("call result: %q", out)
	}

	// isError results surface as errors.
	if _, err := mgr.CallTool(ctx, "fake", "boom", nil); err == nil || !strings.Contains(err.Error(), "exploded") {
		t.Fatalf("boom error: %v", err)
	}

	// Unknown server.
	if _, err := mgr.CallTool(ctx, "nope", "echo", nil); err == nil {
		t.Fatal("expected unknown server error")
	}
}

func TestResultTruncation(t *testing.T) {
	cfg := config.NewDefault()
	cfg.MCPServers = map[string]config.MCPServerConfig{"fake": stdioCfg("fake")}
	mgr := NewManager(cfg)
	defer mgr.Close()
	if errs := mgr.ConnectAll(context.Background()); len(errs) != 0 {
		t.Fatalf("connect: %v", errs)
	}
	out, err := mgr.CallTool(context.Background(), "fake", "big", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Response truncated") || len(out) > MaxResultChars+200 {
		t.Fatalf("truncation failed: len=%d", len(out))
	}
}

func TestRegisterToolsInRegistry(t *testing.T) {
	cfg := config.NewDefault()
	cfg.MCPServers = map[string]config.MCPServerConfig{"fake": stdioCfg("fake")}
	mgr := NewManager(cfg)
	defer mgr.Close()
	if errs := mgr.ConnectAll(context.Background()); len(errs) != 0 {
		t.Fatalf("connect: %v", errs)
	}
	reg := tools.NewRegistry(&tools.RunContext{CWD: t.TempDir()})
	if n := mgr.RegisterTools(reg); n != 3 {
		t.Fatalf("registered: %d", n)
	}
	if _, ok := reg.Get("mcp_fake_echo"); !ok {
		t.Fatal("mcp_fake_echo not registered")
	}
	// Execute through the registry.
	res, _ := reg.Execute(context.Background(), inferenceToolCall("mcp_fake_echo", `{"text":"via registry"}`))
	if res != "echo: via registry" {
		t.Fatalf("registry execute: %q", res)
	}
}

func TestReadOnlyBlocksMCP(t *testing.T) {
	cfg := config.NewDefault()
	cfg.MCPServers = map[string]config.MCPServerConfig{"fake": stdioCfg("fake")}
	mgr := NewManager(cfg)
	defer mgr.Close()
	if errs := mgr.ConnectAll(context.Background()); len(errs) != 0 {
		t.Fatalf("connect: %v", errs)
	}
	reg := tools.NewRegistry(&tools.RunContext{CWD: t.TempDir(), ReadOnly: true})
	mgr.RegisterTools(reg)
	res, _ := reg.Execute(context.Background(), inferenceToolCall("mcp_fake_echo", `{"text":"x"}`))
	if !strings.Contains(res, "read-only mode") {
		t.Fatalf("read-only should block MCP tools: %q", res)
	}
}

func TestSanitizeAndPrefix(t *testing.T) {
	if got := PrefixedName("My Server!", "do_it-2"); got != "mcp_my_server_do_it_2" {
		t.Fatalf("prefix: %q", got)
	}
}

func TestEmptyManager(t *testing.T) {
	mgr := NewManager(config.NewDefault())
	if !mgr.Empty() {
		t.Fatal("expected empty manager")
	}
	if errs := mgr.ConnectAll(context.Background()); len(errs) != 0 {
		t.Fatalf("no servers: %v", errs)
	}
}

func inferenceToolCall(name, args string) inference.ToolCall {
	return inference.ToolCall{ID: "c1", Type: "function", Function: inference.FunctionCall{Name: name, Arguments: args}}
}

func TestToolsListChangedNotification(t *testing.T) {
	cfg := config.NewDefault()
	srvCfg := stdioCfg("fake")
	srvCfg.Env = map[string]string{"FAKE_MCP_LIST_CHANGED": "1"}
	cfg.MCPServers = map[string]config.MCPServerConfig{"fake": srvCfg}
	mgr := NewManager(cfg)
	defer mgr.Close()
	if errs := mgr.ConnectAll(context.Background()); len(errs) != 0 {
		t.Fatalf("connect: %v", errs)
	}
	reg := tools.NewRegistry(&tools.RunContext{CWD: t.TempDir()})
	if n := mgr.RegisterTools(reg); n != 3 {
		t.Fatalf("initial registration: %d", n)
	}
	// The server sends notifications/tools/list_changed after the first
	// tool call; the client must re-fetch and swap the registered set.
	if res, _ := reg.Execute(context.Background(), inferenceToolCall("mcp_fake_echo", `{"text":"trigger"}`)); res != "echo: trigger" {
		t.Fatalf("trigger call: %q", res)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, ok := reg.Get("mcp_fake_newtool"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("newtool never registered; names: %v", reg.Names())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, ok := reg.Get("mcp_fake_boom"); ok {
		t.Fatal("stale tool mcp_fake_boom still registered after list_changed")
	}
	if _, ok := reg.Get("mcp_fake_echo"); !ok {
		t.Fatal("mcp_fake_echo should remain registered")
	}
}

func TestSSETransport(t *testing.T) {
	// Minimal MCP server over HTTP: SSE stream announces the message
	// endpoint; requests are POSTed and answered directly.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sse":
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "event: endpoint\ndata: /messages\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		case "/messages":
			var req struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      json.RawMessage `json:"id"`
				Method  string          `json:"method"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if req.Method == "notifications/initialized" {
				w.WriteHeader(http.StatusAccepted)
				return
			}
			var result interface{}
			switch req.Method {
			case "initialize":
				result = map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
					"serverInfo":      map[string]string{"name": "ssesrv", "version": "1.0.0"},
				}
			case "tools/list":
				result = map[string]interface{}{
					"tools": []map[string]interface{}{{
						"name":        "sse_echo",
						"description": "Echo over SSE",
						"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"text": map[string]string{"type": "string"}}},
					}},
				}
			case "tools/call":
				result = map[string]interface{}{
					"content": []map[string]interface{}{{"type": "text", "text": "sse: ok"}},
				}
			default:
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, mustJSON(result))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	cfg := config.NewDefault()
	cfg.MCPServers = map[string]config.MCPServerConfig{"ssesrv": {
		Name: "ssesrv", Transport: "sse", URL: srv.URL + "/sse", Enabled: true, Timeout: 5,
	}}
	mgr := NewManager(cfg)
	defer mgr.Close()
	if errs := mgr.ConnectAll(context.Background()); len(errs) != 0 {
		t.Fatalf("sse connect: %v", errs)
	}
	reg := tools.NewRegistry(&tools.RunContext{CWD: t.TempDir()})
	if n := mgr.RegisterTools(reg); n != 1 {
		t.Fatalf("registered: %d", n)
	}
	res, _ := reg.Execute(context.Background(), inferenceToolCall("mcp_ssesrv_sse_echo", `{"text":"hi"}`))
	if res != "sse: ok" {
		t.Fatalf("sse call result: %q", res)
	}
	st := mgr.Status()
	if len(st) != 1 || !st[0].Connected {
		t.Fatalf("status: %+v", st)
	}
}
