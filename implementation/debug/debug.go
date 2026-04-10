// Package debug implements debug logging for the coding agent.
package debug

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"
)

// SessionSummary represents a summary of a debug session.
type SessionSummary struct {
	SessionID        string    `json:"session_id"`
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	TotalMessages    int       `json:"total_messages"`
	TotalInputTokens int       `json:"total_input_tokens"`
	TotalOutputTokens int      `json:"total_output_tokens"`
	TotalToolCalls   int       `json:"total_tool_calls"`
	FailedToolCalls  int       `json:"failed_tool_calls"`
	DurationSeconds  float64   `json:"duration_seconds"`
	Version          string    `json:"version"`
}

// DebugLogger handles debug logging for the coding agent.
type DebugLogger struct {
	filePath   string
	file       *os.File
	enabled    bool
	mu         sync.Mutex
	session    *SessionSummary
	version    string
}

// NewDebugLogger creates a new debug logger.
func NewDebugLogger(filePath string, version string) (*DebugLogger, error) {
	// If debug is disabled, return a no-op logger
	if filePath == "" {
		return &DebugLogger{
			enabled: false,
		}, nil
	}

	// Open or create the log file with restrictive permissions
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open debug log file: %w", err)
	}

	now := time.Now()
	logger := &DebugLogger{
		filePath: filePath,
		file:     file,
		enabled:  true,
		version:  version,
		session: &SessionSummary{
			SessionID:   fmt.Sprintf("sess_%d", now.UnixNano()),
			StartTime:   now,
			TotalMessages: 0,
			TotalInputTokens: 0,
			TotalOutputTokens: 0,
			TotalToolCalls: 0,
			FailedToolCalls: 0,
			Version:     version,
		},
	}

	// Write header
	logger.writeLog("================================================================================\n")
	logger.writeLog("CODING AGENT DEBUG LOG\n")
	logger.writeLog("Session: %s\n", now.Format(time.RFC3339))
	logger.writeLog("Version: %s\n", version)
	logger.writeLog("Log File: %s\n", filePath)
	logger.writeLog("================================================================================\n")
	logger.writeLog("\n")

	return logger, nil
}

// writeLog writes a log message to the file.
func (d *DebugLogger) writeLog(format string, args ...interface{}) {
	if !d.enabled {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	fmt.Fprintf(d.file, format, args...)
	d.file.Sync()
}

// LogSystemPrompt logs the system prompt.
func (d *DebugLogger) LogSystemPrompt(prompt string, tokenCount int) {
	if !d.enabled {
		return
	}

	d.mu.Lock()
	d.session.TotalMessages++
	d.session.TotalInputTokens += tokenCount
	d.mu.Unlock()

	d.writeLog("[%s] SYSTEM PROMPT (tokens: %d)\n",
		time.Now().Format(time.RFC3339), tokenCount)
	d.writeLog("--------------------------------------------------------------------------------\n")
	d.writeLog("%s\n", redactSensitiveData(prompt))
	d.writeLog("\n")
}

// LogUserMessage logs a user message.
func (d *DebugLogger) LogUserMessage(content string, tokenCount int) {
	if !d.enabled {
		return
	}

	d.mu.Lock()
	d.session.TotalMessages++
	d.session.TotalInputTokens += tokenCount
	d.mu.Unlock()

	d.writeLog("[%s] USER MESSAGE (tokens: %d)\n",
		time.Now().Format(time.RFC3339), tokenCount)
	d.writeLog("--------------------------------------------------------------------------------\n")
	d.writeLog("%s\n", redactSensitiveData(content))
	d.writeLog("\n")
}

// LogAssistantMessage logs an assistant response.
func (d *DebugLogger) LogAssistantMessage(content string, tokenCount int) {
	if !d.enabled {
		return
	}

	d.mu.Lock()
	d.session.TotalMessages++
	d.session.TotalOutputTokens += tokenCount
	d.mu.Unlock()

	d.writeLog("[%s] ASSISTANT RESPONSE (tokens: %d)\n",
		time.Now().Format(time.RFC3339), tokenCount)
	d.writeLog("--------------------------------------------------------------------------------\n")
	d.writeLog("%s\n", redactSensitiveData(content))
	d.writeLog("\n")
}

// LogToolCall logs a tool call.
func (d *DebugLogger) LogToolCall(toolID, toolName string, parameters map[string]interface{}) {
	if !d.enabled {
		return
	}

	d.mu.Lock()
	d.session.TotalMessages++
	d.session.TotalToolCalls++
	d.mu.Unlock()

	d.writeLog("[%s] TOOL CALL: %s\n",
		time.Now().Format(time.RFC3339), toolName)
	d.writeLog("--------------------------------------------------------------------------------\n")
	d.writeLog("Tool ID: %s\n", toolID)
	d.writeLog("Parameters:\n")

	// Format parameters as JSON
	if paramsJSON, err := json.MarshalIndent(parameters, "", "  "); err == nil {
		d.writeLog("%s\n", redactSensitiveData(string(paramsJSON)))
	} else {
		d.writeLog("%v\n", parameters)
	}
	d.writeLog("\n")
}

// LogToolResult logs a tool result.
func (d *DebugLogger) LogToolResult(toolID, toolName string, success bool, output string) {
	if !d.enabled {
		return
	}

	d.mu.Lock()
	d.session.TotalMessages++
	if !success {
		d.session.FailedToolCalls++
	}
	d.mu.Unlock()

	status := "success"
	if !success {
		status = "failed"
	}

	d.writeLog("[%s] TOOL RESULT: %s\n",
		time.Now().Format(time.RFC3339), toolName)
	d.writeLog("--------------------------------------------------------------------------------\n")
	d.writeLog("Tool ID: %s\n", toolID)
	d.writeLog("Status: %s\n", status)
	d.writeLog("Output: %s\n", redactSensitiveData(output))
	d.writeLog("\n")
}

// LogStreamingChunk logs a streaming chunk.
func (d *DebugLogger) LogStreamingChunk(content string, chunkType string) {
	if !d.enabled {
		return
	}

	d.writeLog("[%s] STREAMING CHUNK (%s)\n",
		time.Now().Format(time.RFC3339), chunkType)
	d.writeLog("%s", content) // No newline for streaming chunks
}

// LogStreamingComplete marks the end of streaming.
func (d *DebugLogger) LogStreamingComplete() {
	if !d.enabled {
		return
	}
	d.writeLog("\n[STREAMING COMPLETE]\n")
}

// LogSessionSummary logs the session summary.
func (d *DebugLogger) LogSessionSummary() {
	if !d.enabled {
		return
	}

	d.mu.Lock()
	d.session.EndTime = time.Now()
	d.session.DurationSeconds = d.session.EndTime.Sub(d.session.StartTime).Seconds()
	sessionCopy := *d.session
	d.mu.Unlock()

	d.writeLog("[%s] SESSION SUMMARY\n",
		time.Now().Format(time.RFC3339))
	d.writeLog("--------------------------------------------------------------------------------\n")
	d.writeLog("Session ID: %s\n", sessionCopy.SessionID)
	d.writeLog("Total Messages: %d\n", sessionCopy.TotalMessages)
	d.writeLog("Total Input Tokens: %d\n", sessionCopy.TotalInputTokens)
	d.writeLog("Total Output Tokens: %d\n", sessionCopy.TotalOutputTokens)
	d.writeLog("Total Tool Calls: %d\n", sessionCopy.TotalToolCalls)
	d.writeLog("Failed Tool Calls: %d\n", sessionCopy.FailedToolCalls)
	d.writeLog("Duration: %.1fs\n", sessionCopy.DurationSeconds)
	d.writeLog("================================================================================\n")
}

// Close closes the debug logger.
func (d *DebugLogger) Close() error {
	if !d.enabled {
		return nil
	}

	d.LogSessionSummary()

	if d.file != nil {
		return d.file.Close()
	}
	return nil
}

// redactSensitiveData redacts sensitive information from log content.
func redactSensitiveData(content string) string {
	// Redact API keys (various patterns)
	patterns := []struct {
		pattern string
		replacement string
	}{
		// API key patterns with is/are/was/were
		{`(?i)(api[_-]?key|apikey)\s+is\s+["']?[a-zA-Z0-9_\-\.]{16,}["']?`, "$1 is [REDACTED]"},
		{`(?i)(api[_-]?key|apikey)\s+[:=]\s*["']?[a-zA-Z0-9_\-\.]{16,}["']?`, "$1: [REDACTED]"},
		{`(?i)bearer\s+[a-zA-Z0-9_\-\.]{20,}`, "bearer [REDACTED]"},
		{`(?i)(token|auth[_-]?token|access[_-]?token)\s+is\s+["']?[a-zA-Z0-9_\-\.]{8,}["']?`, "$1 is [REDACTED]"},
		{`(?i)(token|auth[_-]?token|access[_-]?token)\s*[:=]\s*["']?[a-zA-Z0-9_\-\.]{8,}["']?`, "$1: [REDACTED]"},
		{`(?i)(secret|password|private[_-]?key)\s+is\s+["']?[a-zA-Z0-9_\-\.]{8,}["']?`, "$1 is [REDACTED]"},
		{`(?i)(secret|password|private[_-]?key)\s*[:=]\s*["']?[a-zA-Z0-9_\-\.]{8,}["']?`, "$1: [REDACTED]"},
		// Common API key patterns in JSON
		{`"api[_-]?key"\s*:\s*"[^"]{16,}"`, `"api_key": "[REDACTED]"`},
		{`"apikey"\s*:\s*"[^"]{16,}"`, `"apikey": "[REDACTED]"`},
		{`"token"\s*:\s*"[^"]{8,}"`, `"token": "[REDACTED]"`},
		{`"secret"\s*:\s*"[^"]{8,}"`, `"secret": "[REDACTED]"`},
		{`"password"\s*:\s*"[^"]+"`, `"password": "[REDACTED]"`},
		{`"auth[_-]?token"\s*:\s*"[^"]+"`, `"auth_token": "[REDACTED]"`},
		{`"access[_-]?token"\s*:\s*"[^"]+"`, `"access_token": "[REDACTED]"`},
		{`"private[_-]?key"\s*:\s*"[^"]+"`, `"private_key": "[REDACTED]"`},
		// Sk- prefixed keys (OpenAI style)
		{`sk-[a-zA-Z0-9]{20,}`, "[REDACTED_API_KEY]"},
		// Generic long alphanumeric strings that look like keys
		{`\b[a-zA-Z0-9]{32,}\b`, "[REDACTED]"},
	}

	result := content
	for _, p := range patterns {
		re := regexp.MustCompile(p.pattern)
		result = re.ReplaceAllString(result, p.replacement)
	}

	return result
}
