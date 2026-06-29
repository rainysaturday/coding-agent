package agent

import "strings"

// Exit code constants for agent execution.
const (
	ExitSuccess      = 0
	ExitError        = 1
	ExitUsageError   = 2
	ExitAuthError    = 3
	ExitContextLimit = 4
)

// AuthError represents an authentication error (exit code 3).
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "authentication failed"
}

// ContextLimitError represents a context size limit exceeded error (exit code 4).
type ContextLimitError struct {
	Message string
}

func (e *ContextLimitError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "context size limit exceeded"
}

// isAuthError checks if an error string indicates an authentication failure.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "authentication failed") ||
		strings.Contains(msg, "401") ||
		strings.Contains(msg, "403") ||
		strings.Contains(msg, "Authorization") ||
		strings.Contains(msg, "API key")
}

// isContextLimitError checks if an error string indicates a context size limit.
func isContextLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "context size limit") ||
		strings.Contains(msg, "maximum context length") ||
		strings.Contains(msg, "maximum context length exceeded") ||
		strings.Contains(msg, "maximum context length")
}

// wrapError wraps errors with appropriate typed errors for exit codes.
func wrapError(err error) error {
	if err == nil {
		return nil
	}
	if isAuthError(err) {
		return &AuthError{Message: err.Error()}
	}
	if isContextLimitError(err) {
		return &ContextLimitError{Message: err.Error()}
	}
	return err
}
