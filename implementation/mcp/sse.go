package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coding-agent/coding-agent/config"
	"github.com/coding-agent/coding-agent/debug"
)

// sseTransport talks JSON-RPC to a remote MCP server: requests are POSTed to a
// messages endpoint, responses arrive on a Server-Sent Events stream.
type sseTransport struct {
	cfg      config.MCPServerConfig
	http     *http.Client
	base     *url.URL
	endpoint string
	endReady chan struct{}

	nextID  int64
	mu      sync.Mutex
	pend    map[int64]chan *Response
	closed  chan struct{}
	once    sync.Once
	endOnce sync.Once
	err     error

	noteHandler func(method string, params json.RawMessage)

	cancel context.CancelFunc
}

func newSSETransport(cfg config.MCPServerConfig) *sseTransport {
	t := &sseTransport{
		http:     &http.Client{},
		pend:     map[int64]chan *Response{},
		closed:   make(chan struct{}),
		endReady: make(chan struct{}),
	}
	t.cfg = cfg
	return t
}

func (t *sseTransport) applyAuth(req *http.Request) {
	for k, v := range t.cfg.Headers {
		req.Header.Set(k, v)
	}
	if t.cfg.Env != nil {
		if tok := t.cfg.Env["Authorization"]; tok != "" {
			req.Header.Set("Authorization", tok)
		}
		if tok := t.cfg.Env["BEARER_TOKEN"]; tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
	}
}

func (t *sseTransport) start(ctx context.Context) error {
	if strings.TrimSpace(t.cfg.URL) == "" {
		return fmt.Errorf("MCP server %q uses sse transport but no url is configured", t.cfg.Name)
	}
	u, err := url.Parse(t.cfg.URL)
	if err != nil {
		return fmt.Errorf("invalid MCP url %q: %w", t.cfg.URL, err)
	}
	t.base = u
	if t.cfg.Endpoint != "" {
		if eu, err := url.Parse(t.cfg.Endpoint); err == nil {
			t.endpoint = resolveRef(u, eu).String()
			t.markEndpointReady()
		}
	}

	streamCtx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	req, err := http.NewRequestWithContext(streamCtx, http.MethodGet, t.cfg.URL, nil)
	if err != nil {
		cancel()
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	t.applyAuth(req)

	resp, err := t.http.Do(req)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to connect to MCP SSE endpoint %s: %w", t.cfg.URL, err)
	}
	if resp.StatusCode != http.StatusOK {
		cancel()
		return fmt.Errorf("MCP SSE endpoint %s returned HTTP %d", t.cfg.URL, resp.StatusCode)
	}
	go t.readLoop(resp.Body)
	return nil
}

func resolveRef(base, ref *url.URL) *url.URL {
	out := base.ResolveReference(ref)
	return out
}

func (t *sseTransport) readLoop(body io.ReadCloser) {
	defer body.Close()
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var event string
	var data strings.Builder
	flush := func() {
		payload := strings.TrimSpace(data.String())
		ev := event
		event = ""
		data.Reset()
		if payload == "" {
			return
		}
		if ev == "endpoint" || (t.endpoint == "" && strings.HasPrefix(payload, "/")) {
			if ref, err := url.Parse(payload); err == nil {
				t.endpoint = resolveRef(t.base, ref).String()
				t.markEndpointReady()
			}
			return
		}
		var resp Response
		if err := json.Unmarshal([]byte(payload), &resp); err != nil {
			if dbg := debug.Default(); dbg != nil {
				dbg.Logf("[MCP:%s] failed to decode SSE payload: %v", t.cfg.Name, err)
			}
			return
		}
		t.dispatch(&resp)
	}

	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, ":"):
			// comment / keep alive
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			if data.Len() > 0 {
				data.WriteString("\n")
			}
			data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	t.fail(fmt.Errorf("MCP SSE stream for server %q closed", t.cfg.Name))
}

// markEndpointReady signals that the message endpoint has been discovered.
func (t *sseTransport) markEndpointReady() {
	t.endOnce.Do(func() { close(t.endReady) })
}

func (t *sseTransport) SetNotificationHandler(h func(method string, params json.RawMessage)) {
	t.mu.Lock()
	t.noteHandler = h
	t.mu.Unlock()
}

func (t *sseTransport) dispatch(resp *Response) {
	id, ok := responseID(resp.ID)
	if !ok {
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

func (t *sseTransport) fail(err error) {
	t.once.Do(func() { t.err = err; close(t.closed) })
	t.mu.Lock()
	for id, ch := range t.pend {
		ch <- &Response{Error: &RPCError{Code: -32000, Message: err.Error()}}
		delete(t.pend, id)
	}
	t.mu.Unlock()
}

func (t *sseTransport) timeout() time.Duration {
	d := time.Duration(t.cfg.Timeout) * time.Second
	if d <= 0 {
		d = 30 * time.Second
	}
	return d
}

func (t *sseTransport) Send(ctx context.Context, req *Request) (*Response, error) {
	select {
	case <-t.endReady:
	case <-time.After(t.timeout()):
		return nil, fmt.Errorf("timed out waiting for the MCP message endpoint from %s", t.cfg.URL)
	case <-t.closed:
		return nil, t.connErr()
	}

	t.mu.Lock()
	t.nextID++
	id := t.nextID
	ch := make(chan *Response, 1)
	t.pend[id] = ch
	req.ID = id
	req.JSONRPC = "2.0"
	payload, err := json.Marshal(req)
	t.mu.Unlock()
	if err != nil {
		return nil, err
	}

	postCtx, cancel := context.WithTimeout(ctx, t.timeout())
	defer cancel()
	httpReq, err := http.NewRequestWithContext(postCtx, http.MethodPost, t.endpoint, strings.NewReader(string(payload)))
	if err != nil {
		t.dropPending(id)
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	t.applyAuth(httpReq)

	resp, err := t.http.Do(httpReq)
	if err != nil {
		t.dropPending(id)
		return nil, fmt.Errorf("failed to POST to MCP endpoint: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		t.dropPending(id)
		return nil, fmt.Errorf("MCP endpoint returned HTTP %d", resp.StatusCode)
	}

	// Some servers answer directly on the POST response instead of the stream.
	dec := json.NewDecoder(resp.Body)
	var direct Response
	if err := dec.Decode(&direct); err == nil && direct.JSONRPC != "" {
		if rid, ok := responseID(direct.ID); ok && rid == id {
			return &direct, nil
		}
		t.dispatch(&direct)
	}

	timer := time.NewTimer(t.timeout())
	defer timer.Stop()
	select {
	case r := <-ch:
		return r, nil
	case <-timer.C:
		t.dropPending(id)
		return nil, fmt.Errorf("timeout after %ds waiting for MCP server %q", int(t.timeout().Seconds()), t.cfg.Name)
	case <-ctx.Done():
		t.dropPending(id)
		return nil, ctx.Err()
	case <-t.closed:
		t.dropPending(id)
		return nil, t.connErr()
	}
}

func (t *sseTransport) connErr() error {
	if t.err != nil {
		return t.err
	}
	return errors.New("MCP connection closed")
}

func (t *sseTransport) dropPending(id int64) {
	t.mu.Lock()
	delete(t.pend, id)
	t.mu.Unlock()
}

func (t *sseTransport) Notify(ctx context.Context, note *Notification) error {
	note.JSONRPC = "2.0"
	payload, err := json.Marshal(note)
	if err != nil {
		return err
	}
	endpoint := t.endpoint
	if endpoint == "" {
		return errors.New("MCP message endpoint is not available yet")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	t.applyAuth(req)
	resp, err := t.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("MCP endpoint returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func (t *sseTransport) Close() error {
	if t.cancel != nil {
		t.cancel()
	}
	t.once.Do(func() { close(t.closed) })
	return nil
}
