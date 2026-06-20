package agent

import (
	"fmt"
	"testing"
)

func TestAuthError_Error(t *testing.T) {
	e := &AuthError{Message: "invalid API key"}
	result := e.Error()
	if result != "invalid API key" {
		t.Errorf("Expected 'invalid API key', got '%s'", result)
	}
}

func TestAuthError_Error_Default(t *testing.T) {
	e := &AuthError{}
	result := e.Error()
	if result != "authentication failed" {
		t.Errorf("Expected 'authentication failed', got '%s'", result)
	}
}

func TestContextLimitError_Error(t *testing.T) {
	e := &ContextLimitError{Message: "too many tokens"}
	result := e.Error()
	if result != "too many tokens" {
		t.Errorf("Expected 'too many tokens', got '%s'", result)
	}
}

func TestContextLimitError_Error_Default(t *testing.T) {
	e := &ContextLimitError{}
	result := e.Error()
	if result != "context size limit exceeded" {
		t.Errorf("Expected 'context size limit exceeded', got '%s'", result)
	}
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"auth failed", fmt.Errorf("API authentication failed"), true},
		{"401", fmt.Errorf("HTTP 401 Unauthorized"), true},
		{"403", fmt.Errorf("HTTP 403 Forbidden"), true},
		{"authorization", fmt.Errorf("Authorization required"), true},
		{"API key", fmt.Errorf("Invalid API key"), true},
		{"not auth", fmt.Errorf("some other error"), false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAuthError(tt.err)
			if got != tt.want {
				t.Errorf("isAuthError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsContextLimitError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"context size limit", fmt.Errorf("context size limit exceeded"), true},
		{"maximum context length", fmt.Errorf("maximum context length"), true},
		{"maximum context length exceeded", fmt.Errorf("maximum context length exceeded"), true},
		{"not context", fmt.Errorf("some other error"), false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isContextLimitError(tt.err)
			if got != tt.want {
				t.Errorf("isContextLimitError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWrapError_Nil(t *testing.T) {
	result := wrapError(nil)
	if result != nil {
		t.Errorf("Expected nil, got %v", result)
	}
}

func TestWrapError_AuthError(t *testing.T) {
	err := fmt.Errorf("API authentication failed (HTTP 401)")
	wrapped := wrapError(err)

	_, ok := wrapped.(*AuthError)
	if !ok {
		t.Errorf("Expected *AuthError, got %T", wrapped)
	}
}

func TestWrapError_ContextLimitError(t *testing.T) {
	err := fmt.Errorf("maximum context length exceeded")
	wrapped := wrapError(err)

	_, ok := wrapped.(*ContextLimitError)
	if !ok {
		t.Errorf("Expected *ContextLimitError, got %T", wrapped)
	}
}

func TestWrapError_OtherError(t *testing.T) {
	err := fmt.Errorf("some other error")
	wrapped := wrapError(err)

	if wrapped != err {
		t.Errorf("Expected same error, got %v", wrapped)
	}
}

