// Package tools implements HTTP request testing functionality.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// executeHttpRequest handles HTTP request testing with full method support,
// custom headers, authentication, request body handling, and response validation.
func (te *ToolExecutor) executeHttpRequest(params map[string]interface{}) *ToolResult {
	urlStr, ok := params["url"].(string)
	if !ok || urlStr == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: url",
		}
	}

	// Validate URL - only allow http and https schemes
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid URL: %v", err),
		}
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unsupported URL scheme: %s (only http and https are allowed)", parsedURL.Scheme),
		}
	}

	// Check for dangerous schemes
	if strings.HasPrefix(urlStr, "file://") || strings.HasPrefix(urlStr, "ftp://") ||
		strings.HasPrefix(urlStr, "gopher://") || strings.HasPrefix(urlStr, "data:") {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("blocked dangerous URL scheme: %s", parsedURL.Scheme),
		}
	}

	// Determine HTTP method (default: GET)
	method := "GET"
	if m, ok := params["method"].(string); ok {
		method = strings.ToUpper(m)
	}

	// Validate method
	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
		"PATCH": true, "HEAD": true, "OPTIONS": true,
	}
	if !validMethods[method] {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unsupported HTTP method: %s", method),
		}
	}

	// Build headers
	headers := map[string]string{}
	if h, ok := params["headers"].(map[string]interface{}); ok {
		for k, v := range h {
			if vs, ok := v.(string); ok {
				headers[k] = vs
			}
		}
	}

	// Set default headers
	headers["User-Agent"] = "coding-agent/1.0"

	// Apply authentication if provided
	if auth, ok := params["auth"].(map[string]interface{}); ok {
		switch authType := auth["type"].(string); authType {
		case "bearer":
			if token, ok := auth["token"].(string); ok && token != "" {
				headers["Authorization"] = "Bearer " + token
			}
		case "basic":
			username := ""
			password := ""
			if u, ok := auth["username"].(string); ok {
				username = u
			}
			if p, ok := auth["password"].(string); ok {
				password = p
			}
			if username != "" || password != "" {
				headers["Authorization"] = "Basic " + basicAuth(username, password)
			}
		case "api_key":
			keyName := "X-API-Key"
			if kn, ok := auth["key_name"].(string); ok && kn != "" {
				keyName = kn
			}
			if apiKey, ok := auth["api_key"].(string); ok {
				headers[keyName] = apiKey
			}
		}
	}

	// Build request body for methods that support it
	var bodyReader io.Reader
	if method == "POST" || method == "PUT" || method == "PATCH" {
		body, hasBody := params["body"]
		if hasBody && body != nil {
			bodyStr, _ := body.(string)
			if bodyStr != "" {
				// Determine content type
				contentType := headers["Content-Type"]
				if contentType == "" {
					if ct, ok := params["content_type"].(string); ok {
						contentType = ct
					} else {
						// Default to JSON if body looks like JSON
						if isJSONBody(bodyStr) {
							contentType = "application/json"
						} else {
							contentType = "text/plain"
						}
					}
				}
				headers["Content-Type"] = contentType
				bodyReader = strings.NewReader(bodyStr)
			}
		}
	} else if body, ok := params["body"].(string); ok && body != "" {
		headers["Content-Type"] = "text/plain"
		bodyReader = strings.NewReader(body)
	}

	// Determine timeout (default: 30 seconds)
	timeoutSeconds := 30
	if timeoutParam, hasTimeout := params["timeout"]; hasTimeout {
		switch v := timeoutParam.(type) {
		case float64:
			timeoutSeconds = int(v)
		case int:
			timeoutSeconds = v
		case string:
			if n, err := strconv.Atoi(v); err == nil {
				timeoutSeconds = n
			}
		}
	}

	// Determine max response size (default: 10MB)
	maxSize := 10 * 1024 * 1024 // 10MB
	if maxParam, hasMax := params["max_size"]; hasMax {
		switch v := maxParam.(type) {
		case float64:
			maxSize = int(v)
		case int:
			maxSize = v
		case string:
			if n, err := strconv.Atoi(v); err == nil {
				maxSize = n
			}
		}
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(timeoutSeconds) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Limit redirects
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects (max 10)")
			}
			return nil
		},
	}

	// Create request
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create request: %v", err),
		}
	}

	// Set headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Perform request
	startTime := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("request failed: %v", err),
		}
	}
	defer resp.Body.Close()

	// Read response body with size limit
	var body []byte
	if resp.ContentLength > 0 && resp.ContentLength < int64(maxSize) {
		body, _ = io.ReadAll(io.LimitReader(resp.Body, resp.ContentLength+1))
	} else {
		body, err = io.ReadAll(io.LimitReader(resp.Body, int64(maxSize)+1))
	}
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read response body: %v", err),
		}
	}

	// Check if response was truncated
	truncated := len(body) > maxSize
	if truncated {
		body = body[:maxSize]
	}

	// Collect response headers
	respHeaders := map[string]string{}
	for k, vals := range resp.Header {
		if len(vals) > 0 {
			respHeaders[k] = vals[0]
		}
	}

	// Validate response if expected status provided
	validationErrors := []string{}
	if expectedStatus, ok := params["expected_status"].(float64); ok {
		if int(expectedStatus) != resp.StatusCode {
			validationErrors = append(validationErrors,
				fmt.Sprintf("expected status %d, got %d", int(expectedStatus), resp.StatusCode))
		}
	}
	if expectedStatusRange, ok := params["expected_status_range"].(string); ok {
		if !matchesStatusRange(resp.StatusCode, expectedStatusRange) {
			validationErrors = append(validationErrors,
				fmt.Sprintf("expected status matching range '%s', got %d", expectedStatusRange, resp.StatusCode))
		}
	}
	if expectedContentType, ok := params["expected_content_type"].(string); ok {
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, expectedContentType) {
			validationErrors = append(validationErrors,
				fmt.Sprintf("expected content-type containing '%s', got '%s'", expectedContentType, contentType))
		}
	}

	// Build result
	result := &ToolResult{
		Success: len(validationErrors) == 0,
		Extra: map[string]interface{}{
			"method":         method,
			"url":            urlStr,
			"status_code":    resp.StatusCode,
			"status_text":    resp.Status,
			"content_type":   resp.Header.Get("Content-Type"),
			"content_length": len(body),
			"duration_ms":    duration.Milliseconds(),
			"headers":        respHeaders,
		},
	}

	// Format output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("HTTP %s %s\n", method, urlStr))
	output.WriteString(fmt.Sprintf("Status: %d %s\n", resp.StatusCode, resp.Status))
	output.WriteString(fmt.Sprintf("Content-Type: %s\n", resp.Header.Get("Content-Type")))
	output.WriteString(fmt.Sprintf("Content-Length: %d\n", len(body)))
	output.WriteString(fmt.Sprintf("Duration: %v\n", duration.Round(time.Millisecond)))

	// Add response headers (limit to important ones)
	importantHeaders := []string{"Content-Type", "Content-Length", "Set-Cookie", "Location",
		"Cache-Control", "X-RateLimit-Remaining", "X-RateLimit-Limit", "X-RateLimit-Reset"}
	for _, h := range importantHeaders {
		if v := resp.Header.Get(h); v != "" {
			output.WriteString(fmt.Sprintf("%s: %s\n", h, v))
		}
	}

	// Add body
	bodyStr := string(body)
	if truncated {
		bodyStr += "\n... [response truncated, exceeded max_size limit]"
	}
	output.WriteString(fmt.Sprintf("\n%s", bodyStr))

	result.Output = output.String()

	// Add validation errors if any
	if len(validationErrors) > 0 {
		result.Success = false
		result.Error = "Response validation failed:\n" + strings.Join(validationErrors, "\n")
	}

	return result
}

// basicAuth returns the Base64 encoding of username:password for Basic auth.
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64Encode([]byte(auth))
}

// base64Encode implements Base64 encoding using only the standard library.
func base64Encode(src []byte) string {
	const base64Table = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var result strings.Builder
	result.Grow((len(src) + 2) / 3 * 4)

	i := 0
	for i < len(src)-2 {
		a, b, c := src[i], src[i+1], src[i+2]
		result.WriteByte(base64Table[a>>2])
		result.WriteByte(base64Table[((a&3)<<4)|(b>>4)])
		result.WriteByte(base64Table[((b&15)<<2)|(c>>6)])
		result.WriteByte(base64Table[c&63])
		i += 3
	}

	if i < len(src) {
		a := src[i]
		result.WriteByte(base64Table[a>>2])
		if i+1 < len(src) {
			b := src[i+1]
			result.WriteByte(base64Table[((a&3)<<4)|(b>>4)])
			result.WriteByte(base64Table[(b&15)<<2])
		} else {
			result.WriteByte(base64Table[(a&3)<<4])
		}
		result.WriteByte('=')
		result.WriteByte('=')
	}

	return result.String()
}

// isJSONBody checks if a string looks like JSON.
func isJSONBody(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if (s[0] == '{' && s[len(s)-1] == '}') ||
		(s[0] == '[' && s[len(s)-1] == ']') {
		// Try to parse as JSON
		var data interface{}
		return json.Unmarshal([]byte(s), &data) == nil
	}
	return false
}

// matchesStatusRange checks if a status code matches a range pattern.
// Supports: "2xx", "4xx", "5xx", exact numbers, comma-separated ranges.
func matchesStatusRange(status int, rangeStr string) bool {
	// Handle comma-separated ranges
	parts := strings.Split(rangeStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Check if it's a range like "2xx"
		if len(part) == 3 && part[2] == 'x' {
			prefix := part[0]
			if status/100 == int(prefix-'0') {
				return true
			}
			continue
		}
		// Check if it's an exact status code
		if n, err := strconv.Atoi(part); err == nil {
			if status == n {
				return true
			}
		}
		// Check if it's a range like "200-299"
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) == 2 {
				if low, err1 := strconv.Atoi(rangeParts[0]); err1 == nil {
					if high, err2 := strconv.Atoi(rangeParts[1]); err2 == nil {
						if status >= low && status <= high {
							return true
						}
					}
				}
			}
		}
	}
	return false
}
