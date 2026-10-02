// Package mcp implements a Model Context Protocol client (JSON-RPC 2.0 over
// stdio or SSE) that exposes MCP server tools to the agent.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ProtocolVersion is the MCP protocol version requested by the client.
const ProtocolVersion = "2024-11-05"

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Notification is a JSON-RPC 2.0 message that expects no response.
type Notification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// RPCError is a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message)
}

// Implementation is the transport independent message channel to a server.
type Implementation interface {
	Send(ctx context.Context, req *Request) (*Response, error)
	Notify(ctx context.Context, note *Notification) error
	// SetNotificationHandler installs a callback invoked for server-initiated
	// notifications (messages without a response id).
	SetNotificationHandler(h func(method string, params json.RawMessage))
	Close() error
}

// InitializeParams are the initialize request parameters.
type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
}

// ClientInfo identifies the client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is the initialize response.
type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      ClientInfo             `json:"serverInfo"`
}

// Tool is an MCP tool definition.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolsListResult is the tools/list response.
type ToolsListResult struct {
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// ContentBlock is one item of a tools/call result.
type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// CallToolResult is the tools/call response.
type CallToolResult struct {
	Content           []ContentBlock `json:"content"`
	IsError           bool           `json:"isError,omitempty"`
	StructuredContent interface{}    `json:"structuredContent,omitempty"`
}

// Text flattens the content blocks into a single string.
func (r *CallToolResult) Text() string {
	var b strings.Builder
	for _, c := range r.Content {
		switch c.Type {
		case "text":
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(c.Text)
		case "image":
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(fmt.Sprintf("[image: %s, %d bytes base64]", c.MimeType, len(c.Data)))
		case "resource":
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString("[resource content]")
		default:
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(fmt.Sprintf("[%s content]", c.Type))
		}
	}
	if len(r.Content) == 0 && r.StructuredContent != nil {
		if data, err := json.Marshal(r.StructuredContent); err == nil {
			b.Write(data)
		}
	}
	return b.String()
}

// MaxResultChars is the maximum number of characters kept from an MCP result.
const MaxResultChars = 8192

// TruncateResult trims oversized MCP tool results.
func TruncateResult(s string) string {
	if len(s) <= MaxResultChars {
		return s
	}
	truncated := s[:MaxResultChars]
	return fmt.Sprintf("%s\n\n[Note: Response truncated. %d total characters, showing first %d.]", truncated, len(s), MaxResultChars)
}
