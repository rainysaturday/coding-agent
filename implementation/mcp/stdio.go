package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/coding-agent/coding-agent/config"
	"github.com/coding-agent/coding-agent/debug"
)

// stdioTransport talks JSON-RPC to a subprocess over stdin/stdout.
type stdioTransport struct {
	cfg         config.MCPServerConfig
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	nextID      int64
	mu          sync.Mutex
	pend        map[int64]chan *Response
	closed      chan struct{}
	once        sync.Once
	err         error
	noteHandler func(method string, params json.RawMessage)
}

func newStdioTransport(cfg config.MCPServerConfig) *stdioTransport {
	return &stdioTransport{cfg: cfg, pend: map[int64]chan *Response{}, closed: make(chan struct{})}
}

func (t *stdioTransport) start() error {
	if strings.TrimSpace(t.cfg.Command) == "" {
		return fmt.Errorf("MCP server %q uses stdio transport but no command is configured", t.cfg.Name)
	}
	cmd := exec.Command(t.cfg.Command, t.cfg.Args...)
	if t.cfg.CWD != "" {
		cmd.Dir = t.cfg.CWD
	}
	env := os.Environ()
	for k, v := range t.cfg.Env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server %q: %w", t.cfg.Name, err)
	}
	t.cmd = cmd
	t.stdin = stdin

	go t.readLoop(stdout)
	go func() {
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			if dbg := debug.Default(); dbg != nil {
				dbg.Logf("[MCP:%s stderr] %s", t.cfg.Name, sc.Text())
			}
		}
	}()
	return nil
}

func (t *stdioTransport) readLoop(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var resp Response
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			if dbg := debug.Default(); dbg != nil {
				dbg.Logf("[MCP:%s] failed to decode message: %v", t.cfg.Name, err)
			}
			continue
		}
		t.dispatch(&resp)
	}
	t.fail(fmt.Errorf("MCP server %q closed its output stream", t.cfg.Name))
}

func (t *stdioTransport) SetNotificationHandler(h func(method string, params json.RawMessage)) {
	t.mu.Lock()
	t.noteHandler = h
	t.mu.Unlock()
}

func (t *stdioTransport) dispatch(resp *Response) {
	id, ok := responseID(resp.ID)
	if !ok {
		// Server-initiated notification; nothing to correlate.
		if dbg := debug.Default(); dbg != nil {
			dbg.Logf("[MCP:%s] notification %s", t.cfg.Name, resp.Method)
		}
		t.mu.Lock()
		h := t.noteHandler
		t.mu.Unlock()
		if h != nil {
			h(resp.Method, resp.Params)
		}
		return
	}
	t.mu.Lock()
	ch := t.pend[id]
	delete(t.pend, id)
	t.mu.Unlock()
	if ch != nil {
		ch <- resp
	}
}

func responseID(v interface{}) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case int64:
		return t, true
	case int:
		return int64(t), true
	case json.Number:
		n, err := t.Int64()
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

func (t *stdioTransport) fail(err error) {
	t.once.Do(func() { t.err = err; close(t.closed) })
	t.mu.Lock()
	for id, ch := range t.pend {
		ch <- &Response{Error: &RPCError{Code: -32000, Message: err.Error()}}
		delete(t.pend, id)
	}
	t.mu.Unlock()
}

func (t *stdioTransport) Send(ctx context.Context, req *Request) (*Response, error) {
	t.mu.Lock()
	if t.stdin == nil {
		t.mu.Unlock()
		return nil, errors.New("MCP transport is not connected")
	}
	t.nextID++
	id := t.nextID
	ch := make(chan *Response, 1)
	t.pend[id] = ch
	req.ID = id
	req.JSONRPC = "2.0"
	payload, err := json.Marshal(req)
	if err != nil {
		delete(t.pend, id)
		t.mu.Unlock()
		return nil, err
	}
	_, err = t.stdin.Write(append(payload, '\n'))
	t.mu.Unlock()
	if err != nil {
		t.mu.Lock()
		delete(t.pend, id)
		t.mu.Unlock()
		return nil, fmt.Errorf("failed to write to MCP server: %w", err)
	}
	if dbg := debug.Default(); dbg != nil {
		dbg.Logf("[MCP:%s] --> %s %s", t.cfg.Name, req.Method, debug.Redact(string(payload)))
	}

	timeout := time.Duration(t.cfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case resp := <-ch:
		if dbg := debug.Default(); dbg != nil {
			dbg.Logf("[MCP:%s] <-- %s", t.cfg.Name, debug.Redact(fmt.Sprintf("%s", resp.Result)))
		}
		return resp, nil
	case <-timer.C:
		t.mu.Lock()
		delete(t.pend, id)
		t.mu.Unlock()
		return nil, fmt.Errorf("timeout after %ds waiting for MCP server %q", int(timeout.Seconds()), t.cfg.Name)
	case <-ctx.Done():
		t.mu.Lock()
		delete(t.pend, id)
		t.mu.Unlock()
		return nil, ctx.Err()
	case <-t.closed:
		t.mu.Lock()
		delete(t.pend, id)
		t.mu.Unlock()
		if t.err != nil {
			return nil, t.err
		}
		return nil, errors.New("MCP connection closed")
	}
}

func (t *stdioTransport) Notify(ctx context.Context, note *Notification) error {
	note.JSONRPC = "2.0"
	payload, err := json.Marshal(note)
	if err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stdin == nil {
		return errors.New("MCP transport is not connected")
	}
	_, err = t.stdin.Write(append(payload, '\n'))
	return err
}

func (t *stdioTransport) Close() error {
	t.mu.Lock()
	stdin := t.stdin
	cmd := t.cmd
	t.stdin = nil
	t.mu.Unlock()
	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd != nil {
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	}
	t.once.Do(func() { close(t.closed) })
	return nil
}
