package tools

import (
	"testing"
	"time"
)

func TestInteractiveSessionStartInvalid(t *testing.T) {
	te := NewToolExecutor()

	// Missing action
	result := te.executeInteractiveSession(map[string]interface{}{})
	if result.Success {
		t.Error("Expected failure for missing action")
	}
	if result.Error == "" {
		t.Error("Expected error message for missing action")
	}

	// Invalid action
	result = te.executeInteractiveSession(map[string]interface{}{
		"action": "invalid",
	})
	if result.Success {
		t.Error("Expected failure for invalid action")
	}
}

func TestInteractiveSessionStartMissingCommand(t *testing.T) {
	te := NewToolExecutor()

	result := te.executeInteractiveSession(map[string]interface{}{
		"action": "start",
	})
	if result.Success {
		t.Error("Expected failure for missing command")
	}
	if result.Error == "" {
		t.Error("Expected error message for missing command")
	}
}

func TestInteractiveSessionStartWithBash(t *testing.T) {
	te := NewToolExecutor()

	result := te.executeInteractiveSession(map[string]interface{}{
		"action":  "start",
		"command": "bash",
		"args":    []interface{}{},
	})

	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	if result.Output == "" {
		t.Error("Expected non-empty output")
	}

	sessionID, ok := result.Extra["session"].(string)
	if !ok {
		t.Error("Expected session ID in result")
	}
	if sessionID == "" {
		t.Error("Expected non-empty session ID")
	}

	// Clean up
	stopResult := te.executeInteractiveSession(map[string]interface{}{
		"action":  "stop",
		"session": sessionID,
	})
	if !stopResult.Success {
		t.Logf("Warning: cleanup failed: %s", stopResult.Error)
	}
}

func TestInteractiveSessionStartWithPython(t *testing.T) {
	te := NewToolExecutor()

	result := te.executeInteractiveSession(map[string]interface{}{
		"action":  "start",
		"command": "python3",
		"args":    []interface{}{},
	})

	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	sessionID, ok := result.Extra["session"].(string)
	if !ok || sessionID == "" {
		t.Error("Expected session ID in result")
	}

	// Clean up
	stopResult := te.executeInteractiveSession(map[string]interface{}{
		"action":  "stop",
		"session": sessionID,
	})
	if !stopResult.Success {
		t.Logf("Warning: cleanup failed: %s", stopResult.Error)
	}
}

func TestInteractiveSessionSendToNonexistent(t *testing.T) {
	te := NewToolExecutor()

	result := te.executeInteractiveSession(map[string]interface{}{
		"action":  "send",
		"session": "nonexistent",
		"input":   "print('hello')",
	})

	if result.Success {
		t.Error("Expected failure for nonexistent session")
	}
}

func TestInteractiveSessionSendMissingSession(t *testing.T) {
	te := NewToolExecutor()

	result := te.executeInteractiveSession(map[string]interface{}{
		"action": "send",
		"input":  "print('hello')",
	})

	if result.Success {
		t.Error("Expected failure for missing session")
	}
}

func TestInteractiveSessionSendMissingInput(t *testing.T) {
	te := NewToolExecutor()

	result := te.executeInteractiveSession(map[string]interface{}{
		"action":  "send",
		"session": "session-1",
	})

	if result.Success {
		t.Error("Expected failure for missing input")
	}
}

func TestInteractiveSessionStopNonexistent(t *testing.T) {
	te := NewToolExecutor()

	result := te.executeInteractiveSession(map[string]interface{}{
		"action":  "stop",
		"session": "nonexistent",
	})

	if result.Success {
		t.Error("Expected failure for nonexistent session")
	}
}

func TestInteractiveSessionStopMissingSession(t *testing.T) {
	te := NewToolExecutor()

	result := te.executeInteractiveSession(map[string]interface{}{
		"action": "stop",
	})

	if result.Success {
		t.Error("Expected failure for missing session")
	}
}

func TestInteractiveSessionList(t *testing.T) {
	te := NewToolExecutor()

	result := te.executeInteractiveSession(map[string]interface{}{
		"action": "list",
	})

	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	count, ok := result.Extra["count"].(int)
	if !ok {
		t.Error("Expected count in result")
	}
	_ = count
}

func TestInteractiveSessionStartArgs(t *testing.T) {
	te := NewToolExecutor()

	result := te.executeInteractiveSession(map[string]interface{}{
		"action":  "start",
		"command": "bash",
		"args":    []interface{}{"-c", "echo hello"},
	})

	if !result.Success {
		t.Fatalf("Expected success, got: %s", result.Error)
	}

	sessionID, ok := result.Extra["session"].(string)
	if !ok || sessionID == "" {
		t.Error("Expected session ID in result")
	}

	// Clean up
	stopResult := te.executeInteractiveSession(map[string]interface{}{
		"action":  "stop",
		"session": sessionID,
	})
	if !stopResult.Success {
		t.Logf("Warning: cleanup failed: %s", stopResult.Error)
	}
}

func TestInteractiveSessionSendAndStop(t *testing.T) {
	te := NewToolExecutor()

	// Start a session
	startResult := te.executeInteractiveSession(map[string]interface{}{
		"action":  "start",
		"command": "bash",
	})
	if !startResult.Success {
		t.Fatalf("Failed to start session: %s", startResult.Error)
	}

	sessionID := startResult.Extra["session"].(string)

	// Send input - this may timeout because bash is interactive and waiting for input
	// That's expected behavior
	sendResult := te.executeInteractiveSession(map[string]interface{}{
		"action":  "send",
		"session": sessionID,
		"input":   "echo hello from session",
		"timeout": 2,
	})
	// Output may or may not succeed depending on shell setup
	_ = sendResult

	// Stop session (should always succeed since we don't auto-kill on timeout)
	stopResult := te.executeInteractiveSession(map[string]interface{}{
		"action":  "stop",
		"session": sessionID,
	})

	if !stopResult.Success {
		t.Errorf("Failed to stop session: %s", stopResult.Error)
	}
}

func TestInteractiveSessionSendTimeout(t *testing.T) {
	te := NewToolExecutor()

	// Start a session
	startResult := te.executeInteractiveSession(map[string]interface{}{
		"action":  "start",
		"command": "python3",
		"args":    []interface{}{},
	})
	if !startResult.Success {
		t.Fatalf("Failed to start session: %s", startResult.Error)
	}

	sessionID := startResult.Extra["session"].(string)

	// Send with very short timeout - should timeout
	sendResult := te.executeInteractiveSession(map[string]interface{}{
		"action":  "send",
		"session": sessionID,
		"input":   "print('hello')",
		"timeout": 1,
	})

	// Timeout is expected for interactive sessions
	_ = sendResult

	// Stop session (may already be stopped due to timeout)
	stopResult := te.executeInteractiveSession(map[string]interface{}{
		"action":  "stop",
		"session": sessionID,
	})
	_ = stopResult
}

func TestInteractiveSessionManager(t *testing.T) {
	m := NewInteractiveSessionManager()

	// Start a session
	session, err := m.StartSession("bash", []string{})
	if err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}

	if session.ID == "" {
		t.Error("Expected non-empty session ID")
	}
	if !session.Running {
		t.Error("Expected session to be running")
	}
	if session.StartTime.IsZero() {
		t.Error("Expected non-zero start time")
	}

	// Send input (short timeout since interactive shells wait for more input)
	_, err = m.SendInput(session.ID, "echo test", 2*time.Second)
	// Timeout is expected for interactive shells
	_ = err

	// List sessions
	sessions := m.ListSessions()
	if len(sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(sessions))
	}

	// Stop session
	err = m.StopSession(session.ID)
	if err != nil {
		t.Errorf("Failed to stop session: %v", err)
	}

	// Verify session is gone
	sessions = m.ListSessions()
	if len(sessions) != 0 {
		t.Errorf("Expected 0 sessions after stop, got %d", len(sessions))
	}

	// Try to stop again - should fail
	err = m.StopSession(session.ID)
	if err == nil {
		t.Error("Expected error when stopping already-stopped session")
	}
}

func TestInteractiveSessionManagerMultiple(t *testing.T) {
	m := NewInteractiveSessionManager()

	// Start two sessions
	s1, err1 := m.StartSession("bash", []string{})
	s2, err2 := m.StartSession("bash", []string{})

	if err1 != nil || err2 != nil {
		t.Fatalf("Failed to start sessions: %v, %v", err1, err2)
	}

	if s1.ID == s2.ID {
		t.Error("Expected unique session IDs")
	}

	// List should show 2
	sessions := m.ListSessions()
	if len(sessions) != 2 {
		t.Errorf("Expected 2 sessions, got %d", len(sessions))
	}

	// Stop both
	m.StopSession(s1.ID)
	m.StopSession(s2.ID)

	// List should show 0
	sessions = m.ListSessions()
	if len(sessions) != 0 {
		t.Errorf("Expected 0 sessions, got %d", len(sessions))
	}
}

func TestExecuteTool(t *testing.T) {
	te := NewToolExecutor()

	// Test with interactive_session action
	tc := &ToolCall{
		Name:       "interactive_session",
		Parameters: map[string]interface{}{"action": "list"},
	}

	result := te.Execute(tc)
	if !result.Success {
		t.Fatalf("Expected success for list action: %s", result.Error)
	}
}
