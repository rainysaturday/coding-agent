// Command fakemcp is a minimal MCP server used by the mcp package tests.
// It speaks newline-delimited JSON-RPC 2.0 over stdin/stdout.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type msg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcErr         `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func main() {
	listChanged := os.Getenv("FAKE_MCP_LIST_CHANGED") == "1"
	var listCount, callCount int
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	out := os.Stdout
	for in.Scan() {
		line := strings.TrimSpace(in.Text())
		if line == "" {
			continue
		}
		var m msg
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		if m.Method == "" {
			continue
		}
		var result interface{}
		switch m.Method {
		case "initialize":
			result = map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
				"serverInfo":      map[string]string{"name": "fakemcp", "version": "1.0.0"},
			}
		case "notifications/initialized":
			continue // notifications get no response
		case "tools/list":
			if listChanged && listCount > 0 {
				// Updated list after a list_changed notification.
				result = map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "echo",
							"description": "Echo the input text back",
							"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"text": map[string]string{"type": "string"}}, "required": []string{"text"}},
						},
						{
							"name":        "newtool",
							"description": "Appears after list changed",
							"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
						},
					},
				}
				data, _ := json.Marshal(msg{JSONRPC: "2.0", ID: m.ID, Result: result})
				fmt.Fprintf(out, "%s\n", data)
				continue
			}
			listCount++
			result = map[string]interface{}{
				"tools": []map[string]interface{}{
					{
						"name":        "echo",
						"description": "Echo the input text back",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"text": map[string]string{"type": "string"},
							},
							"required": []string{"text"},
						},
					},
					{
						"name":        "boom",
						"description": "Always fails",
						"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
					},
					{
						"name":        "big",
						"description": "Returns a very large text result",
						"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
					},
				},
			}
		case "tools/call":
			var p struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments"`
			}
			_ = json.Unmarshal(m.Params, &p)
			switch p.Name {
			case "echo":
				text, _ := p.Arguments["text"].(string)
				result = map[string]interface{}{
					"content": []map[string]interface{}{{"type": "text", "text": "echo: " + text}},
				}
			case "boom":
				result = map[string]interface{}{
					"content": []map[string]interface{}{{"type": "text", "text": "tool exploded"}},
					"isError": true,
				}
			case "big":
				result = map[string]interface{}{
					"content": []map[string]interface{}{{"type": "text", "text": strings.Repeat("x", 20000)}},
				}
			default:
				fmt.Fprintf(out, `{"jsonrpc":"2.0","id":%s,"error":{"code":-32601,"message":"unknown tool %s"}}`+"\n", m.ID, p.Name)
				continue
			}
		default:
			fmt.Fprintf(out, `{"jsonrpc":"2.0","id":%s,"error":{"code":-32601,"message":"method not found"}}`+"\n", m.ID)
			continue
		}
		data, _ := json.Marshal(msg{JSONRPC: "2.0", ID: m.ID, Result: result})
		fmt.Fprintf(out, "%s\n", data)
		if m.Method == "tools/call" && listChanged && callCount == 0 {
			// After the first tool call, announce that the tool set changed;
			// subsequent tools/list responses carry the updated list.
			callCount++
			fmt.Fprint(out, `{"jsonrpc":"2.0","method":"notifications/tools/list_changed"}`+"\n")
		}
	}
}
