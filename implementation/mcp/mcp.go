package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coding-agent/coding-agent/config"
	"github.com/coding-agent/coding-agent/debug"
	"github.com/coding-agent/coding-agent/inference"
	"github.com/coding-agent/coding-agent/tools"
	"github.com/coding-agent/coding-agent/version"
)

// Status describes the state of one MCP server connection.
type Status struct {
	Name      string   `json:"name"`
	Transport string   `json:"transport"`
	Connected bool     `json:"connected"`
	Tools     []string `json:"tools"`
	Error     string   `json:"error,omitempty"`
}

// Server is a single MCP server connection.
type Server struct {
	cfg            config.MCPServerConfig
	impl           Implementation
	tools          []Tool
	info           ClientInfo
	mu             sync.Mutex
	connected      bool
	lastErr        string
	onToolsChanged func() // invoked after tools/list_changed re-fetch
}

// Manager owns all MCP server connections.
type Manager struct {
	cfg        *config.Config
	servers    map[string]*Server
	order      []string
	mu         sync.Mutex
	reg        *tools.Registry     // set by RegisterTools; used for live re-registration
	registered map[string][]string // agent-visible tool names per server
}

// NewManager creates a manager for the configured servers.
func NewManager(cfg *config.Config) *Manager {
	m := &Manager{cfg: cfg, servers: map[string]*Server{}}
	for _, sc := range cfg.EnabledMCPServers() {
		name := sc.Name
		if name == "" {
			continue
		}
		sc := sc
		m.servers[name] = &Server{cfg: sc}
		m.order = append(m.order, name)
	}
	sort.Strings(m.order)
	return m
}

// Empty reports whether no MCP servers are configured.
func (m *Manager) Empty() bool { return len(m.servers) == 0 }

// ConnectAll connects to every configured server, retrying with exponential
// backoff. Failures are collected and returned; they never abort startup.
func (m *Manager) ConnectAll(ctx context.Context) []error {
	var errs []error
	for _, name := range m.order {
		srv := m.servers[name]
		if err := srv.connectWithRetry(ctx); err != nil {
			errs = append(errs, fmt.Errorf("MCP server %q: %w", name, err))
			continue
		}
		if dbg := debug.Default(); dbg != nil {
			names := make([]string, 0, len(srv.tools))
			for _, t := range srv.tools {
				names = append(names, t.Name)
			}
			dbg.Logf("[MCP:%s] connected (%s), tools: %s", name, srv.cfg.Transport, strings.Join(names, ", "))
		}
	}
	return errs
}

// connectWithRetry performs the connect handshake with exponential backoff.
func (s *Server) connectWithRetry(ctx context.Context) error {
	maxAttempts := s.cfg.MaxRetries
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	delay := time.Second
	const maxDelay = 30 * time.Second
	var last error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := s.Connect(ctx)
		if err == nil {
			return nil
		}
		last = err
		if attempt == maxAttempts {
			break
		}
		jitter := time.Duration(rand.Int63n(int64(delay / 4)))
		if dbg := debug.Default(); dbg != nil {
			dbg.Logf("[MCP:%s] connect attempt %d/%d failed (%v); retrying in %s", s.cfg.Name, attempt, maxAttempts, err, delay+jitter)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay + jitter):
		}
		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}
	return fmt.Errorf("failed after %d attempts: %w", maxAttempts, last)
}

// Connect opens the transport, performs the MCP handshake and lists tools.
func (s *Server) Connect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.impl != nil {
		_ = s.impl.Close()
		s.impl = nil
		s.connected = false
	}

	var impl Implementation
	switch strings.ToLower(strings.TrimSpace(s.cfg.Transport)) {
	case "", "stdio":
		st := newStdioTransport(s.cfg)
		if err := st.start(); err != nil {
			s.lastErr = err.Error()
			return err
		}
		impl = st
	case "sse":
		st := newSSETransport(s.cfg)
		if err := st.start(ctx); err != nil {
			s.lastErr = err.Error()
			return err
		}
		impl = st
	default:
		return fmt.Errorf("unsupported MCP transport %q (expected stdio or sse)", s.cfg.Transport)
	}

	initParams := InitializeParams{
		ProtocolVersion: ProtocolVersion,
		Capabilities: map[string]interface{}{
			"tools":     map[string]interface{}{},
			"resources": map[string]interface{}{},
			"prompts":   map[string]interface{}{},
		},
		ClientInfo: ClientInfo{Name: "coding-agent", Version: version.String()},
	}
	resp, err := impl.Send(ctx, &Request{Method: "initialize", Params: initParams})
	if err != nil {
		_ = impl.Close()
		s.lastErr = err.Error()
		return fmt.Errorf("initialize failed: %w", err)
	}
	if resp.Error != nil {
		_ = impl.Close()
		s.lastErr = resp.Error.Error()
		return resp.Error
	}
	var initRes InitializeResult
	_ = json.Unmarshal(resp.Result, &initRes)
	s.info = initRes.ServerInfo

	if err := impl.Notify(ctx, &Notification{Method: "notifications/initialized"}); err != nil {
		_ = impl.Close()
		s.lastErr = err.Error()
		return fmt.Errorf("initialized notification failed: %w", err)
	}

	impl.SetNotificationHandler(s.handleNotification)
	s.impl = impl
	s.connected = true

	listResp, err := impl.Send(ctx, &Request{Method: "tools/list", Params: map[string]interface{}{"cursor": nil}})
	if err != nil {
		s.lastErr = err.Error()
		return fmt.Errorf("tools/list failed: %w", err)
	}
	if listResp.Error != nil {
		// Servers without the tools capability are still usable.
		s.lastErr = listResp.Error.Error()
		s.tools = nil
		return nil
	}
	var list ToolsListResult
	if err := json.Unmarshal(listResp.Result, &list); err != nil {
		s.lastErr = err.Error()
		return fmt.Errorf("failed to decode tools/list: %w", err)
	}
	s.tools = list.Tools
	s.lastErr = ""
	return nil
}

// handleNotification reacts to server-initiated notifications. For
// notifications/tools/list_changed the tool list is re-fetched and the
// agent-visible tools are updated (spec: Notification Handling).
func (s *Server) handleNotification(method string, params json.RawMessage) {
	if method != "notifications/tools/list_changed" {
		return
	}
	// Runs on the transport read loop; the re-fetch needs that loop free to
	// deliver the tools/list response, so do it asynchronously.
	go s.refetchTools()
}

func (s *Server) refetchTools() {
	s.mu.Lock()
	impl := s.impl
	connected := s.connected
	s.mu.Unlock()
	if !connected || impl == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := impl.Send(ctx, &Request{Method: "tools/list", Params: map[string]interface{}{"cursor": nil}})
	if err != nil || resp.Error != nil {
		if dbg := debug.Default(); dbg != nil {
			dbg.Logf("[MCP:%s] tools/list_changed re-fetch failed: %v", s.cfg.Name, err)
		}
		return
	}
	var list ToolsListResult
	if err := json.Unmarshal(resp.Result, &list); err != nil {
		return
	}
	s.mu.Lock()
	s.tools = list.Tools
	changed := s.onToolsChanged
	s.mu.Unlock()
	if dbg := debug.Default(); dbg != nil {
		dbg.Logf("[MCP:%s] tool list updated after list_changed (%d tools)", s.cfg.Name, len(list.Tools))
	}
	if changed != nil {
		changed()
	}
}

// Connected reports the connection state.
func (s *Server) Connected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connected
}

func (s *Server) lastError() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastErr
}

// Tools returns a copy of the tool definitions.
func (s *Server) Tools() []Tool {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Tool, len(s.tools))
	copy(out, s.tools)
	return out
}

// ToolNames returns the MCP tool names exposed by the server.
func (s *Server) ToolNames() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.tools))
	for _, t := range s.tools {
		out = append(out, t.Name)
	}
	return out
}

// sanitizeName normalises a name for use inside a tool identifier.
func sanitizeName(s string) string {
	var b strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore && b.Len() > 0 {
				b.WriteRune('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

// PrefixedName returns the agent side tool name for an MCP tool.
func PrefixedName(server, tool string) string {
	return fmt.Sprintf("mcp_%s_%s", sanitizeName(server), sanitizeName(tool))
}

// RegisterTools registers every MCP tool with the agent tool registry using the
// mcp_{server}_{tool} naming convention.
func (m *Manager) RegisterTools(reg *tools.Registry) int {
	m.mu.Lock()
	m.reg = reg
	if m.registered == nil {
		m.registered = map[string][]string{}
	}
	m.mu.Unlock()
	count := 0
	for _, name := range m.order {
		srv := m.servers[name]
		srv.mu.Lock()
		srv.onToolsChanged = func() { m.reregister(name) }
		srv.mu.Unlock()
		count += m.registerServerTools(name, reg)
	}
	return count
}

// registerServerTools registers the current tool set of one server and records
// the registered names.
func (m *Manager) registerServerTools(name string, reg *tools.Registry) int {
	srv := m.servers[name]
	var names []string
	for _, t := range srv.Tools() {
		agentName := PrefixedName(name, t.Name)
		reg.Register(&mcpTool{
			reg:    reg,
			mgr:    m,
			server: name,
			tool:   t.Name,
			name:   agentName,
			desc:   t.Description,
			schema: t.InputSchema,
		})
		names = append(names, agentName)
	}
	m.mu.Lock()
	if m.registered == nil {
		m.registered = map[string][]string{}
	}
	m.registered[name] = names
	m.mu.Unlock()
	return len(names)
}

// reregister replaces a server's registered tools after a list_changed
// notification.
func (m *Manager) reregister(name string) {
	m.mu.Lock()
	reg := m.reg
	old := m.registered[name]
	m.mu.Unlock()
	if reg == nil {
		return
	}
	for _, n := range old {
		reg.Unregister(n)
	}
	m.registerServerTools(name, reg)
}

// mcpTool adapts an MCP tool to the agent tool interface.
type mcpTool struct {
	reg    *tools.Registry
	mgr    *Manager
	server string
	tool   string
	name   string
	desc   string
	schema map[string]interface{}
}

func (t *mcpTool) Name() string { return t.name }

func (t *mcpTool) Description() string {
	if t.desc == "" {
		return fmt.Sprintf("Call the %s tool provided by the MCP server %q", t.tool, t.server)
	}
	return t.desc
}

func (t *mcpTool) Schema() map[string]interface{} {
	if t.schema == nil || len(t.schema) == 0 {
		return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	}
	return t.schema
}

// ReadOnly is false: MCP servers may mutate state, so their tools are blocked
// in read-only mode.
func (t *mcpTool) ReadOnly() bool { return false }

func (t *mcpTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.mgr.CallTool(ctx, t.server, t.tool, args)
}

// Definitions returns the OpenAI definitions for all connected MCP tools.
func (m *Manager) Definitions() []inference.ToolDefinition {
	var defs []inference.ToolDefinition
	for _, name := range m.order {
		srv := m.servers[name]
		for _, t := range srv.Tools() {
			schema := t.InputSchema
			if schema == nil {
				schema = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
			}
			defs = append(defs, inference.ToolDefinition{
				Type:     "function",
				Function: inference.ToolFunction{Name: PrefixedName(name, t.Name), Description: t.Description, Parameters: schema},
			})
		}
	}
	return defs
}

// CallTool invokes a tool, reconnecting with backoff when the connection died.
func (m *Manager) CallTool(ctx context.Context, serverName, toolName string, args map[string]interface{}) (string, error) {
	m.mu.Lock()
	srv := m.servers[serverName]
	m.mu.Unlock()
	if srv == nil {
		return "", fmt.Errorf("unknown MCP server %q", serverName)
	}

	result, err := srv.call(ctx, toolName, args)
	if err != nil && srv.shouldReconnect() {
		if dbg := debug.Default(); dbg != nil {
			dbg.Logf("[MCP:%s] call failed (%v); attempting reconnect", serverName, err)
		}
		if rerr := srv.connectWithRetry(ctx); rerr == nil {
			return srv.call(ctx, toolName, args)
		} else {
			return "", fmt.Errorf("connection to MCP server %q failed: %v", serverName, rerr)
		}
	}
	if err != nil {
		return "", fmt.Errorf("Error executing %s: %v", PrefixedName(serverName, toolName), err)
	}
	return result, nil
}

func (s *Server) shouldReconnect() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.Reconnect
}

func (s *Server) call(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	s.mu.Lock()
	impl := s.impl
	connected := s.connected
	s.mu.Unlock()
	if !connected || impl == nil {
		return "", fmt.Errorf("MCP server %q is not connected", s.cfg.Name)
	}
	if args == nil {
		args = map[string]interface{}{}
	}
	if dbg := debug.Default(); dbg != nil {
		dbg.Logf("[MCP:%s] Calling tool: %s with args %s", s.cfg.Name, toolName, debug.Redact(mustJSON(args)))
	}
	resp, err := impl.Send(ctx, &Request{Method: "tools/call", Params: map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	}})
	if err != nil {
		s.mu.Lock()
		s.connected = false
		s.lastErr = err.Error()
		s.mu.Unlock()
		return "", err
	}
	if resp.Error != nil {
		return "", resp.Error
	}
	var res CallToolResult
	if err := json.Unmarshal(resp.Result, &res); err != nil {
		return "", fmt.Errorf("failed to decode tools/call result: %w", err)
	}
	text := TruncateResult(res.Text())
	if res.IsError {
		return "", fmt.Errorf("%s", text)
	}
	return text, nil
}

func mustJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}

// Status returns the state of every configured server.
func (m *Manager) Status() []Status {
	out := make([]Status, 0, len(m.order))
	for _, name := range m.order {
		srv := m.servers[name]
		st := Status{Name: name, Transport: srv.cfg.Transport, Connected: srv.Connected(), Tools: srv.ToolNames(), Error: srv.lastError()}
		for i := range st.Tools {
			st.Tools[i] = PrefixedName(name, st.Tools[i])
		}
		out = append(out, st)
	}
	return out
}

// Close shuts every connection down.
func (m *Manager) Close() {
	for _, name := range m.order {
		srv := m.servers[name]
		srv.mu.Lock()
		if srv.impl != nil {
			_ = srv.impl.Close()
			srv.impl = nil
		}
		srv.connected = false
		srv.mu.Unlock()
	}
}
