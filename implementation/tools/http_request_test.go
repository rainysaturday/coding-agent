package tools

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExecuteHttpRequest_BasicGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	executor := NewToolExecutor()
	result := executor.Execute(&ToolCall{
		Name: "http_request",
		Parameters: map[string]interface{}{
			"url":    server.URL,
			"method": "GET",
		},
	})

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Extra["status_code"] != 200 {
		t.Errorf("expected status 200, got %v", result.Extra["status_code"])
	}
	if !strings.Contains(result.Output, "ok") {
		t.Errorf("expected 'ok' in output, got: %s", result.Output)
	}
}

func TestExecuteHttpRequest_PostJson(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": 1, "name": "test"}`))
	}))
	defer server.Close()

	executor := NewToolExecutor()
	result := executor.Execute(&ToolCall{
		Name: "http_request",
		Parameters: map[string]interface{}{
			"url":        server.URL,
			"method":     "POST",
			"body":       `{"name": "test"}`,
			"content_type": "application/json",
		},
	})

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.Extra["status_code"] != 201 {
		t.Errorf("expected status 201, got %v", result.Extra["status_code"])
	}
}

func TestExecuteHttpRequest_BearerAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token-123" {
			t.Errorf("expected Bearer auth, got: %s", auth)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"auth": true}`))
	}))
	defer server.Close()

	executor := NewToolExecutor()
	result := executor.Execute(&ToolCall{
		Name: "http_request",
		Parameters: map[string]interface{}{
			"url": server.URL,
			"auth": map[string]interface{}{
				"type":  "bearer",
				"token": "test-token-123",
			},
		},
	})

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
}

func TestExecuteHttpRequest_BasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Basic ") {
			t.Errorf("expected Basic auth, got: %s", auth)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"auth": "basic"}`))
	}))
	defer server.Close()

	executor := NewToolExecutor()
	result := executor.Execute(&ToolCall{
		Name: "http_request",
		Parameters: map[string]interface{}{
			"url": server.URL,
			"auth": map[string]interface{}{
				"type":     "basic",
				"username": "user",
				"password": "pass",
			},
		},
	})

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
}

func TestExecuteHttpRequest_Validation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "not found"}`))
	}))
	defer server.Close()

	executor := NewToolExecutor()

	// Without validation - should succeed
	result := executor.Execute(&ToolCall{
		Name: "http_request",
		Parameters: map[string]interface{}{
			"url": server.URL,
		},
	})
	if !result.Success {
		t.Fatalf("expected success without validation, got error: %s", result.Error)
	}

	// With validation - should fail
	result = executor.Execute(&ToolCall{
		Name: "http_request",
		Parameters: map[string]interface{}{
			"url":               server.URL,
			"expected_status":   200.0,
		},
	})
	if result.Success {
		t.Fatal("expected failure with status validation, got success")
	}
	if !strings.Contains(result.Error, "expected status 200") {
		t.Errorf("expected validation error message, got: %s", result.Error)
	}
}

func TestExecuteHttpRequest_InvalidUrl(t *testing.T) {
	executor := NewToolExecutor()
	result := executor.Execute(&ToolCall{
		Name: "http_request",
		Parameters: map[string]interface{}{
			"url": "file:///etc/passwd",
		},
	})
	if result.Success {
		t.Fatal("expected failure for file:// URL")
	}
	if !strings.Contains(result.Error, "blocked") && !strings.Contains(result.Error, "unsupported") {
		t.Errorf("expected blocked/unsupported error, got: %s", result.Error)
	}
}

func TestExecuteHttpRequest_MissingUrl(t *testing.T) {
	executor := NewToolExecutor()
	result := executor.Execute(&ToolCall{
		Name: "http_request",
		Parameters: map[string]interface{}{},
	})
	if result.Success {
		t.Fatal("expected failure for missing url")
	}
	if !strings.Contains(result.Error, "missing required parameter: url") {
		t.Errorf("expected missing url error, got: %s", result.Error)
	}
}

func TestExecuteHttpRequest_Headers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			t.Errorf("expected X-Custom-Header, got: %s", r.Header.Get("X-Custom-Header"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"headers": true}`))
	}))
	defer server.Close()

	executor := NewToolExecutor()
	result := executor.Execute(&ToolCall{
		Name: "http_request",
		Parameters: map[string]interface{}{
			"url": server.URL,
			"headers": map[string]interface{}{
				"X-Custom-Header": "custom-value",
			},
		},
	})

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
}

func TestExecuteHttpRequest_APIKeyAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "my-api-key" {
			t.Errorf("expected API key header, got: %s", r.Header.Get("X-API-Key"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	executor := NewToolExecutor()
	result := executor.Execute(&ToolCall{
		Name: "http_request",
		Parameters: map[string]interface{}{
			"url": server.URL,
			"auth": map[string]interface{}{
				"type":     "api_key",
				"api_key":  "my-api-key",
				"key_name": "X-API-Key",
			},
		},
	})

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
}

func TestMatchesStatusRange(t *testing.T) {
	tests := []struct {
		status int
		rangeStr string
		want bool
	}{
		{200, "2xx", true},
		{404, "4xx", true},
		{500, "5xx", true},
		{200, "4xx", false},
		{200, "200", true},
		{201, "200", false},
		{250, "200-299", true},
		{300, "200-299", false},
		{200, "2xx,4xx", true},
		{404, "2xx,4xx", true},
		{500, "2xx,4xx", false},
		{200, "200-200", true},
		{201, "200-200", false},
	}

	for _, tt := range tests {
		got := matchesStatusRange(tt.status, tt.rangeStr)
		if got != tt.want {
			t.Errorf("matchesStatusRange(%d, %q) = %v, want %v", tt.status, tt.rangeStr, got, tt.want)
		}
	}
}

func TestIsJSONBody(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{`{"key": "value"}`, true},
		{`[1, 2, 3]`, true},
		{`not json`, false},
		{`{"unclosed": true`, false},
		{``, false},
		{`   `, false},
	}

	for _, tt := range tests {
		got := isJSONBody(tt.input)
		if got != tt.want {
			t.Errorf("isJSONBody(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestExecuteHttpRequest_Delete(t *testing.T) {
	var method string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	executor := NewToolExecutor()
	result := executor.Execute(&ToolCall{
		Name: "http_request",
		Parameters: map[string]interface{}{
			"url":    server.URL,
			"method": "DELETE",
		},
	})

	if method != "DELETE" {
		t.Errorf("expected DELETE method, got: %s", method)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
}

func TestExecuteHttpRequest_Head(t *testing.T) {
	var method string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	executor := NewToolExecutor()
	result := executor.Execute(&ToolCall{
		Name: "http_request",
		Parameters: map[string]interface{}{
			"url":    server.URL,
			"method": "HEAD",
		},
	})

	if method != "HEAD" {
		t.Errorf("expected HEAD method, got: %s", method)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
}

func TestExecuteHttpRequest_ContentTypeValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"test": true}`))
	}))
	defer server.Close()

	executor := NewToolExecutor()
	result := executor.Execute(&ToolCall{
		Name: "http_request",
		Parameters: map[string]interface{}{
			"url":                   server.URL,
			"expected_content_type": "application/json",
		},
	})

	if !result.Success {
		t.Fatalf("expected success with content type validation, got error: %s", result.Error)
	}
}
