package tools

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

// Interactive session manager maintains state across tool calls.
var interactiveSessionManager = NewInteractiveSessionManager()

// InteractiveSessionManager manages multiple interactive terminal sessions.
type InteractiveSessionManager struct {
	sessions map[string]*InteractiveSession
	mu       sync.Mutex
}

// InteractiveSession represents a running interactive session.
type InteractiveSession struct {
	ID        string
	Cmd       *exec.Cmd
	Stdin     io.WriteCloser
	Stdout    io.ReadCloser
	Stderr    io.ReadCloser
	StartTime time.Time
	Running   bool
	mu        sync.Mutex
}

// NewInteractiveSessionManager creates a new session manager.
func NewInteractiveSessionManager() *InteractiveSessionManager {
	return &InteractiveSessionManager{
		sessions: make(map[string]*InteractiveSession),
	}
}

// nextSessionID generates a unique session ID.
func (m *InteractiveSessionManager) nextSessionID() string {
	id := fmt.Sprintf("session-%d", len(m.sessions)+1)
	if _, exists := m.sessions[id]; exists {
		return m.nextSessionID()
	}
	return id
}

// StartSession starts a new interactive session.
func (m *InteractiveSessionManager) StartSession(command string, args []string) (*InteractiveSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Build command
	var cmd *exec.Cmd
	if command == "bash" || command == "sh" || command == "zsh" || command == "fish" {
		// Use interactive mode for shells
		cmd = exec.Command("bash", "-c", fmt.Sprintf("exec %s", command))
	} else {
		cmd = exec.Command(command, args...)
	}

	// Set up pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	sessionID := m.nextSessionID()
	session := &InteractiveSession{
		ID:        sessionID,
		Cmd:       cmd,
		Stdin:     stdin,
		Stdout:    stdout,
		Stderr:    stderr,
		StartTime: time.Now(),
		Running:   true,
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("failed to start session '%s': %w", command, err)
	}

	m.sessions[sessionID] = session
	return session, nil
}

// StopSession stops a running session.
func (m *InteractiveSessionManager) StopSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session '%s' not found", id)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.Running {
		return fmt.Errorf("session '%s' is not running", id)
	}

	s.Running = false
	s.Stdin.Close()

	if s.Cmd != nil && s.Cmd.Process != nil {
		s.Cmd.Process.Kill()
		s.Cmd.Wait()
	}

	delete(m.sessions, id)
	return nil
}

// SendInput sends input to a session and captures output.
func (m *InteractiveSessionManager) SendInput(sessionID, input string, timeout time.Duration) (string, error) {
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	m.mu.Lock()
	s, ok := m.sessions[sessionID]
	m.mu.Unlock()

	if !ok {
		return "", fmt.Errorf("session '%s' not found", sessionID)
	}

	s.mu.Lock()
	if !s.Running {
		s.mu.Unlock()
		return "", fmt.Errorf("session '%s' is not running", sessionID)
	}
	stdin := s.Stdin
	s.mu.Unlock()

	// Write input to session
	_, err := stdin.Write([]byte(input + "\n"))
	if err != nil {
		return "", fmt.Errorf("failed to write to session '%s': %w", sessionID, err)
	}

	// Wait for output with timeout
	done := make(chan struct{})
	var output bytes.Buffer

	go func() {
		defer func() {
			recover()
			close(done)
		}()
		buf := make([]byte, 4096)
		for {
			n, err := s.Stdout.Read(buf)
			if n > 0 {
				output.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()

	select {
	case <-done:
		// Read finished
	case <-time.After(timeout):
		// Timeout - just return error, don't kill the session
		return "", fmt.Errorf("session '%s' timed out after %v", sessionID, timeout)
	}

	// Also capture stderr
	var stderrBuf bytes.Buffer
	io.Copy(&stderrBuf, s.Stderr)

	return output.String() + stderrBuf.String(), nil
}

// ListSessions returns info about all active sessions.
func (m *InteractiveSessionManager) ListSessions() []map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []map[string]interface{}
	for id, s := range m.sessions {
		s.mu.Lock()
		status := "stopped"
		if s.Running {
			status = "running"
		}
		result = append(result, map[string]interface{}{
			"id":       id,
			"status":   status,
			"started":  s.StartTime.Format(time.RFC3339),
			"uptime":   time.Since(s.StartTime).Round(time.Second).String(),
		})
		s.mu.Unlock()
	}
	return result
}

// executeInteractiveSession dispatches to the appropriate action handler.
func (te *ToolExecutor) executeInteractiveSession(params map[string]interface{}) *ToolResult {
	action, ok := params["action"].(string)
	if !ok || action == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: action (valid: start, send, stop, list)",
		}
	}

	switch action {
	case "start":
		return te.interactiveSessionStart(params)
	case "send":
		return te.interactiveSessionSend(params)
	case "stop":
		return te.interactiveSessionStop(params)
	case "list":
		return te.interactiveSessionList(params)
	default:
		return &ToolResult{
			Success: false,
			Error: fmt.Sprintf("unknown action: %s (valid: start, send, stop, list)", action),
		}
	}
}

// interactiveSessionStart starts a new interactive terminal session.
func (te *ToolExecutor) interactiveSessionStart(params map[string]interface{}) *ToolResult {
	command, ok := params["command"].(string)
	if !ok || command == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: command (e.g., 'python3', 'node', 'sqlite3', 'bash')",
		}
	}

	// Get arguments
	var args []string
	if argsParam, ok := params["args"]; ok {
		switch v := argsParam.(type) {
		case []interface{}:
			for _, a := range v {
				if s, ok := a.(string); ok {
					args = append(args, s)
				}
			}
		case string:
			if v != "" {
				args = append(args, v)
			}
		}
	}

	session, err := interactiveSessionManager.StartSession(command, args)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   err.Error(),
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Started interactive session '%s' running: %s", session.ID, command),
		Extra: map[string]interface{}{
			"action":  "start",
			"session": session.ID,
			"command": command,
			"args":    args,
		},
	}
}

// interactiveSessionSend sends input to a running session.
func (te *ToolExecutor) interactiveSessionSend(params map[string]interface{}) *ToolResult {
	sessionID, ok := params["session"].(string)
	if !ok || sessionID == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: session (session ID, e.g., 'session-1')",
		}
	}

	input, ok := params["input"].(string)
	if !ok || input == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: input (text to send to the session)",
		}
	}

	timeout := 10
	if timeoutParam, exists := params["timeout"]; exists {
		switch v := timeoutParam.(type) {
		case float64:
			timeout = int(v)
		case int:
			timeout = v
		}
	}

	output, err := interactiveSessionManager.SendInput(sessionID, input, time.Duration(timeout)*time.Second)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   err.Error(),
			Extra: map[string]interface{}{
				"action":  "send",
				"session": sessionID,
			},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  output,
		Extra: map[string]interface{}{
			"action":  "send",
			"session": sessionID,
		},
	}
}

// interactiveSessionStop stops a running session.
func (te *ToolExecutor) interactiveSessionStop(params map[string]interface{}) *ToolResult {
	sessionID, ok := params["session"].(string)
	if !ok || sessionID == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: session (session ID to stop)",
		}
	}

	err := interactiveSessionManager.StopSession(sessionID)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   err.Error(),
			Extra: map[string]interface{}{
				"action":  "stop",
				"session": sessionID,
			},
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Session '%s' stopped and cleaned up", sessionID),
		Extra: map[string]interface{}{
			"action":  "stop",
			"session": sessionID,
		},
	}
}

// interactiveSessionList lists all active sessions.
func (te *ToolExecutor) interactiveSessionList(params map[string]interface{}) *ToolResult {
	sessions := interactiveSessionManager.ListSessions()

	output := fmt.Sprintf("Active interactive sessions (%d):\n", len(sessions))
	for _, s := range sessions {
		output += fmt.Sprintf("  [%s] %s, uptime: %s\n",
			s["id"], s["status"], s["uptime"])
	}

	if len(sessions) == 0 {
		output = "No active interactive sessions."
	}

	return &ToolResult{
		Success: true,
		Output:  output,
		Extra: map[string]interface{}{
			"action":   "list",
			"count":    len(sessions),
			"sessions": sessions,
		},
	}
}
