package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/inference"
	"github.com/coding-agent/harness/tools"
)

func TestBuildTools_AllToolsPresent(t *testing.T) {
	tools := buildTools(false, false)

	expectedNames := []string{
		"bash",
		"read_file",
		"write_file",
		"read_lines",
		"insert_lines",
		"replace_text",
	}

	for _, expected := range expectedNames {
		found := false
		for _, tool := range tools {
			if tool.Function.Name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Tool '%s' not found in buildTools()", expected)
		}
	}
}

func TestBuildTools_Parameters(t *testing.T) {
	toolDefs := buildTools(false, false)

	expectedParams := map[string][]string{
		"bash":         {"command"},
		"read_file":    {"path"},
		"write_file":   {"path", "content"},
		"read_lines":   {"path", "start", "end"},
		"insert_lines": {"path", "line", "lines"},
		"replace_text": {"path", "search", "replace"},
		"move_text":    {"source_path", "source_start", "source_end", "target_path", "target_line"},
		"view_image":   {"path"},
		"subagent":     {"prompt"},
		"todo":         {"action"},
	}

	for _, tool := range toolDefs {
		expected, ok := expectedParams[tool.Function.Name]
		if !ok {
			t.Errorf("Unexpected tool: %s", tool.Function.Name)
			continue
		}

		if len(tool.Function.Parameters.Required) != len(expected) {
			t.Errorf("Tool %s: expected %d required params, got %d",
				tool.Function.Name, len(expected), len(tool.Function.Parameters.Required))
		}

		for _, req := range expected {
			found := false
			for _, actual := range tool.Function.Parameters.Required {
				if actual == req {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Tool %s: missing required param %s", tool.Function.Name, req)
			}
		}
	}
}

func TestBuildTools_ViewImage_OptionalPrompt(t *testing.T) {
	toolDefs := buildTools(false, false)

	var viewImageTool *inference.ToolDefinition
	for _, tool := range toolDefs {
		if tool.Function.Name == "view_image" {
			viewImageTool = &tool
			break
		}
	}

	if viewImageTool == nil {
		t.Fatal("view_image tool not found")
	}

	// prompt should be in properties but NOT in required
	if _, ok := viewImageTool.Function.Parameters.Properties["prompt"]; !ok {
		t.Errorf("Expected 'prompt' in view_image properties, got: %v", viewImageTool.Function.Parameters.Properties)
	}

	for _, req := range viewImageTool.Function.Parameters.Required {
		if req == "prompt" {
			t.Errorf("'prompt' should not be in required parameters, got: %v", viewImageTool.Function.Parameters.Required)
		}
	}

	// path should still be required
	if _, ok := viewImageTool.Function.Parameters.Properties["path"]; !ok {
		t.Errorf("Expected 'path' in view_image properties, got: %v", viewImageTool.Function.Parameters.Properties)
	}
}

func TestBuildTools_ReadOnly_ViewImage_OptionalPrompt(t *testing.T) {
	toolDefs := buildTools(true, false)

	var viewImageTool *inference.ToolDefinition
	for _, tool := range toolDefs {
		if tool.Function.Name == "view_image" {
			viewImageTool = &tool
			break
		}
	}

	if viewImageTool == nil {
		t.Fatal("view_image tool not found in read-only mode")
	}

	// prompt should be in properties but NOT in required
	if _, ok := viewImageTool.Function.Parameters.Properties["prompt"]; !ok {
		t.Errorf("Expected 'prompt' in view_image properties, got: %v", viewImageTool.Function.Parameters.Properties)
	}

	for _, req := range viewImageTool.Function.Parameters.Required {
		if req == "prompt" {
			t.Errorf("'prompt' should not be in required parameters, got: %v", viewImageTool.Function.Parameters.Required)
		}
	}
}

func TestHandleViewImage_CustomPrompt(t *testing.T) {
	// Create a mock HTTP server to simulate vision model response
	var receivedPrompt string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err == nil {
			if messages, ok := reqBody["messages"].([]interface{}); ok && len(messages) > 0 {
				if msg, ok := messages[0].(map[string]interface{}); ok {
					if content, ok := msg["content"].([]interface{}); ok && len(content) > 0 {
						if textPart, ok := content[0].(map[string]interface{}); ok {
							if text, ok := textPart["text"].(string); ok {
								receivedPrompt = text
							}
						}
					}
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"Custom analysis result"}}]}`)
	}))
	defer mockServer.Close()

	cfg := config.DefaultConfig()
	cfg.APIEndpoint = mockServer.URL
	cfg.Streaming = false
	agent := NewAgent(cfg)

	result := &tools.ToolResult{
		Success: true,
		Output:  "Image loaded: test.png (image/png, 100 bytes)",
		Path:    "/tmp/test.png",
		Extra: map[string]interface{}{
			"data_uri":  "data:image/png;base64,iVBORw0KGgo=",
			"mime_type": "image/png",
			"size":      100,
			"prompt":    "What is the total revenue shown in this chart?",
		},
	}

	description := agent.handleViewImage(context.Background(), result)

	expectedDescription := "Tool 'view_image' executed successfully.\n\nImage description:\nCustom analysis result"
	if description != expectedDescription {
		t.Errorf("Expected description with custom analysis, got: %s", description)
	}
	if receivedPrompt != "What is the total revenue shown in this chart?" {
		t.Errorf("Expected custom prompt to be sent to vision model, got: %s", receivedPrompt)
	}
}

func TestHandleViewImage_DefaultPrompt(t *testing.T) {
	var receivedPrompt string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err == nil {
			if messages, ok := reqBody["messages"].([]interface{}); ok && len(messages) > 0 {
				if msg, ok := messages[0].(map[string]interface{}); ok {
					if content, ok := msg["content"].([]interface{}); ok && len(content) > 0 {
						if textPart, ok := content[0].(map[string]interface{}); ok {
							if text, ok := textPart["text"].(string); ok {
								receivedPrompt = text
							}
						}
					}
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"Default analysis result"}}]}`)
	}))
	defer mockServer.Close()

	cfg := config.DefaultConfig()
	cfg.APIEndpoint = mockServer.URL
	cfg.Streaming = false

	agent := NewAgent(cfg)

	result := &tools.ToolResult{
		Success: true,
		Output:  "Image loaded: test.png (image/png, 100 bytes)",
		Path:    "/tmp/test.png",
		Extra: map[string]interface{}{
			"data_uri":  "data:image/png;base64,iVBORw0KGgo=",
			"mime_type": "image/png",
			"size":      100,
			// No prompt - should use default
		},
	}

	description := agent.handleViewImage(context.Background(), result)

	expectedDescription := "Tool 'view_image' executed successfully.\n\nImage description:\nDefault analysis result"
	if description != expectedDescription {
		t.Errorf("Expected description with default analysis, got: %s", description)
	}
	if !strings.Contains(receivedPrompt, "Describe this image in detail") {
		t.Errorf("Expected default prompt to be sent to vision model, got: %s", receivedPrompt)
	}
}

func TestBuildTools_ReadOnly(t *testing.T) {
	tools := buildTools(true, false)

	// In read-only mode, should only have read_file, read_lines, list_files, grep, git_log, git_show, git_diff, and view_image
	expectedNames := []string{"read_file", "read_lines", "list_files", "grep", "git_log", "git_show", "git_diff", "view_image"}

	if len(tools) != len(expectedNames) {
		t.Errorf("Expected %d tools in read-only mode, got %d", len(expectedNames), len(tools))
	}

	for _, expected := range expectedNames {
		found := false
		for _, tool := range tools {
			if tool.Function.Name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Tool '%s' not found in read-only buildTools()", expected)
		}
	}

	// Verify that bash is NOT in read-only mode
	for _, tool := range tools {
		if tool.Function.Name == "bash" {
			t.Error("bash should not be in read-only mode")
		}
		if tool.Function.Name == "write_file" {
			t.Error("write_file should not be in read-only mode")
		}
	}
}

func TestBuildTools_ExperimentalGating(t *testing.T) {
	// Without experimental flag, subagent should NOT be present
	toolsNoExp := buildTools(false, false)
	for _, tool := range toolsNoExp {
		if tool.Function.Name == "subagent" {
			t.Error("subagent should NOT be present when experimental=false")
		}
	}

	// With experimental flag, subagent SHOULD be present
	toolsWithExp := buildTools(false, true)
	found := false
	for _, tool := range toolsWithExp {
		if tool.Function.Name == "subagent" {
			found = true
			break
		}
	}
	if !found {
		t.Error("subagent should be present when experimental=true")
	}
}

