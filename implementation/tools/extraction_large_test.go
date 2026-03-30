package tools

import (
	"strconv"
	"strings"
	"testing"
)

// ==================== LARGE COMPLEX PROMPT TESTS ====================

// TestExtractToolCalls_LargePrompt_MultipleTools tests extraction from a large response with many tools
func TestExtractToolCalls_LargePrompt_MultipleTools(t *testing.T) {
	largePrompt := `
I'll help you analyze this codebase. Let me start by exploring the directory structure.

First, let's list the files:
[TOOL:{"name":"bash","parameters":{"command":"find . -type f -name '*.go' | head -20"}}]

Now I'll read the main configuration file:
[TOOL:{"name":"read_file","parameters":{"path":"./config/config.go"}}]

Based on the config, I need to check the implementation:
[TOOL:{"name":"read_lines","parameters":{"path":"./main.go","start":"1","end":"50"}}]

Let me also check the tools implementation:
[TOOL:{"name":"read_lines","parameters":{"path":"./tools/tools.go","start":"1","end":"100"}}]
`

	calls, err := ExtractToolCalls(largePrompt)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	if len(calls) != 4 {
		t.Errorf("Expected 4 tool calls, got %d", len(calls))
		for i, call := range calls {
			t.Logf("Call %d: %s", i, call.Name)
		}
	}

	expectedTools := []string{"bash", "read_file", "read_lines", "read_lines"}
	for i, expected := range expectedTools {
		if i < len(calls) && calls[i].Name != expected {
			t.Errorf("Expected call %d to be '%s', got '%s'", i, expected, calls[i].Name)
		}
	}
}

// TestExtractToolCalls_LargePrompt_ComplexJSON tests extraction when JSON contains complex structures
func TestExtractToolCalls_LargePrompt_ComplexJSON(t *testing.T) {
	complexPrompt := `
I'll create a comprehensive script with embedded JSON configuration.

Here's the script:
[TOOL:{"name":"write_file","parameters":{"path":"./config.json","content":"{\n  \"server\": {\n    \"host\": \"localhost\",\n    \"port\": 8080,\n    \"tls\": {\n      \"enabled\": true,\n      \"cert\": \"/path/to/cert\"\n    }\n  },\n  \"database\": {\n    \"connection\": \"postgres://user:pass@localhost/db\"\n  }\n}"}}]

Now let me verify it was created:
[TOOL:{"name":"bash","parameters":{"command":"cat ./config.json"}}]
`

	calls, err := ExtractToolCalls(complexPrompt)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	if len(calls) != 2 {
		t.Errorf("Expected 2 tool calls, got %d", len(calls))
	}

	if calls[0].Name != "write_file" {
		t.Errorf("Expected first call to be 'write_file', got '%s'", calls[0].Name)
	}

	// Verify the JSON content was preserved
	if !strings.Contains(calls[0].Params["content"], `"server"`) {
		t.Errorf("Expected content to contain 'server' key")
	}
	if !strings.Contains(calls[0].Params["content"], `"port": 8080`) {
		t.Errorf("Expected content to contain port configuration")
	}
}

// TestExtractToolCalls_LargePrompt_MultilineContent tests extraction with large multiline content
func TestExtractToolCalls_LargePrompt_MultilineContent(t *testing.T) {
	// Generate a large multiline content (100 lines) with proper JSON escaping
	var contentLines []string
	for i := 1; i <= 100; i++ {
		contentLines = append(contentLines, "Line "+strconv.Itoa(i)+": Some content here")
	}
	// Use JSON-escaped newlines (this is what LLMs should produce)
	largeContent := strings.Join(contentLines, "\\n")

	largePrompt := `
I'll write a large file with 100 lines of content:
[TOOL:{"name":"write_file","parameters":{"path":"./large.txt","content":"` + largeContent + `"}}]
`

	calls, err := ExtractToolCalls(largePrompt)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	if len(calls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(calls))
	}

	if calls[0].Name != "write_file" {
		t.Errorf("Expected 'write_file', got '%s'", calls[0].Name)
	}

	// Verify content was preserved (JSON unescape happens during parsing)
	if !strings.Contains(calls[0].Params["content"], "Line 1") {
		t.Errorf("Expected content to contain 'Line 1'")
	}
	if !strings.Contains(calls[0].Params["content"], "Line 100") {
		t.Errorf("Expected content to contain 'Line 100'")
	}
}

// TestExtractToolCalls_LargePrompt_ReplaceLinesComplex tests replace_lines with complex code content
func TestExtractToolCalls_LargePrompt_ReplaceLinesComplex(t *testing.T) {
	// Complex code replacement - properly JSON-escaped (quotes escaped as \")
	codeReplacement := `func NewServer(config *Config) *Server {\n    s := &Server{\n        config: config,\n        router: mux.NewRouter(),\n        middleware: []Middleware{\n            LoggingMiddleware,\n            AuthMiddleware,\n            RateLimitMiddleware,\n        },\n        handlers: make(map[string]Handler),\n    }\n    \n    s.registerRoutes()\n    return s\n}\n\nfunc (s *Server) registerRoutes() {\n    s.router.HandleFunc(\"/api/v1/health\", s.healthCheck).Methods(\"GET\")\n    s.router.HandleFunc(\"/api/v1/users\", s.listUsers).Methods(\"GET\")\n    s.router.HandleFunc(\"/api/v1/users/{id}\", s.getUser).Methods(\"GET\")\n}`

	prompt := `
I'll update the server initialization code:
[TOOL:{"name":"replace_lines","parameters":{"path":"./server.go","start":"45","end":"70","lines":"` + codeReplacement + `"}}]

This should fix the issue with the server not starting properly.
`

	calls, err := ExtractToolCalls(prompt)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	if len(calls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(calls))
	}

	if calls[0].Name != "replace_lines" {
		t.Errorf("Expected 'replace_lines', got '%s'", calls[0].Name)
	}

	// Verify the complex code was preserved
	if !strings.Contains(calls[0].Params["lines"], "func NewServer") {
		t.Errorf("Expected lines to contain 'func NewServer'")
	}
	if !strings.Contains(calls[0].Params["lines"], "mux.NewRouter()") {
		t.Errorf("Expected lines to contain 'mux.NewRouter()'")
	}
	if !strings.Contains(calls[0].Params["lines"], "RateLimitMiddleware") {
		t.Errorf("Expected lines to contain 'RateLimitMiddleware'")
	}
}

// TestExtractToolCalls_LargePrompt_NestedBracesInJSON tests handling of deeply nested JSON structures
func TestExtractToolCalls_LargePrompt_NestedBracesInJSON(t *testing.T) {
	// Deeply nested JSON content - properly escaped for JSON
	// In Go raw strings: \" becomes \" in JSON (escaped quote), \n becomes \n in JSON (escaped newline)
	deeplyNestedJSON := `{\n  \"level1\": {\n    \"level2\": {\n      \"level3\": {\n        \"level4\": {\n          \"value\": \"deep\"\n        }\n      }\n    }\n  }\n}`

	prompt := `
Here's the deeply nested configuration:
[TOOL:{"name":"write_file","parameters":{"path":"./nested.json","content":"` + deeplyNestedJSON + `"}}]
`

	calls, err := ExtractToolCalls(prompt)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	if len(calls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(calls))
	}

	// After JSON unescaping, content should have actual quotes
	if !strings.Contains(calls[0].Params["content"], `"level1"`) {
		t.Errorf("Expected content to contain '\"level1\"', got: %s", calls[0].Params["content"])
	}
}

// TestExtractToolCalls_LargePrompt_SequentialSameTool tests multiple sequential calls to the same tool
func TestExtractToolCalls_LargePrompt_SequentialSameTool(t *testing.T) {
	prompt := `
I'll read multiple sections of the file:
[TOOL:{"name":"read_lines","parameters":{"path":"./main.go","start":"1","end":"50"}}]
[TOOL:{"name":"read_lines","parameters":{"path":"./main.go","start":"51","end":"100"}}]
[TOOL:{"name":"read_lines","parameters":{"path":"./main.go","start":"101","end":"150"}}]
[TOOL:{"name":"read_lines","parameters":{"path":"./main.go","start":"151","end":"200"}}]
`

	calls, err := ExtractToolCalls(prompt)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	if len(calls) != 4 {
		t.Errorf("Expected 4 tool calls, got %d", len(calls))
	}

	for i, call := range calls {
		if call.Name != "read_lines" {
			t.Errorf("Call %d: Expected 'read_lines', got '%s'", i, call.Name)
		}
	}

	// Verify line ranges
	expectedRanges := []struct{ start, end string }{
		{"1", "50"},
		{"51", "100"},
		{"101", "150"},
		{"151", "200"},
	}

	for i, expected := range expectedRanges {
		if calls[i].Params["start"] != expected.start {
			t.Errorf("Call %d: Expected start '%s', got '%s'", i, expected.start, calls[i].Params["start"])
		}
		if calls[i].Params["end"] != expected.end {
			t.Errorf("Call %d: Expected end '%s', got '%s'", i, expected.end, calls[i].Params["end"])
		}
	}
}

// ==================== EDGE CASE TESTS ====================

// TestExtractToolCalls_EdgeCase_EmptyText tests extraction from empty text
func TestExtractToolCalls_EdgeCase_EmptyText(t *testing.T) {
	calls, err := ExtractToolCalls("")
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	if len(calls) != 0 {
		t.Errorf("Expected 0 tool calls, got %d", len(calls))
	}
}

// TestExtractToolCalls_EdgeCase_OnlyTextNoTools tests extraction from text with no tools
func TestExtractToolCalls_EdgeCase_OnlyTextNoTools(t *testing.T) {
	text := `This is just regular text without any tool calls.
It might contain words like tool, call, or even [TOOL: but not in the right format.
The end.`

	calls, err := ExtractToolCalls(text)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	if len(calls) != 0 {
		t.Errorf("Expected 0 tool calls, got %d", len(calls))
	}
}

// TestExtractToolCalls_EdgeCase_MalformedToolCall tests handling of malformed tool calls
func TestExtractToolCalls_EdgeCase_MalformedToolCall(t *testing.T) {
	// Missing closing bracket
	text := `Here's a tool call:
[TOOL:{"name":"bash","parameters":{"command":"ls"}}`

	calls, err := ExtractToolCalls(text)
	// Should not error, just skip malformed calls
	if err != nil {
		t.Logf("Note: ExtractToolCalls returned error: %v", err)
	}

	// Should return empty or partial results
	t.Logf("Extracted %d calls from malformed input", len(calls))
}

// TestExtractToolCalls_EdgeCase_BracketInText tests handling of brackets in regular text
func TestExtractToolCalls_EdgeCase_BracketInText(t *testing.T) {
	text := `
I need to check the array syntax [TOOL: not this one] and then:
[TOOL:{"name":"bash","parameters":{"command":"ls -la"}}]
Also note that [foo] and [bar] are not tool calls.
`

	calls, err := ExtractToolCalls(text)
	// Should not error - malformed calls should be skipped
	if err != nil {
		t.Logf("Note: ExtractToolCalls returned error (may be expected): %v", err)
	}

	// Should extract at least the valid tool call
	if len(calls) < 1 {
		t.Errorf("Expected at least 1 tool call, got %d", len(calls))
	}

	// Find the bash call
	var bashCall *ToolCall
	for _, call := range calls {
		if call.Name == "bash" {
			bashCall = call
			break
		}
	}
	if bashCall == nil {
		t.Errorf("Expected to find 'bash' tool call")
	}
}

// TestExtractToolCalls_EdgeCase_UnclosedJSONBrace tests handling of unclosed JSON braces
func TestExtractToolCalls_EdgeCase_UnclosedJSONBrace(t *testing.T) {
	text := `
This has an unclosed brace: [TOOL:{"name":"bash","parameters":{
And then a valid one:
[TOOL:{"name":"read_file","parameters":{"path":"./test.txt"}}]
`

	calls, err := ExtractToolCalls(text)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	// Should extract the valid one
	if len(calls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(calls))
	}

	if calls[0].Name != "read_file" {
		t.Errorf("Expected 'read_file', got '%s'", calls[0].Name)
	}
}

// TestExtractToolCalls_EdgeCase_StrippedNewlines tests handling of content that might have stripped newlines
func TestExtractToolCalls_EdgeCase_StrippedNewlines(t *testing.T) {
	// Some LLMs might strip or alter newlines
	text := `Here's the tool call: [TOOL:{"name":"bash","parameters":{"command":"echo hello"}}] Done.`

	calls, err := ExtractToolCalls(text)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	if len(calls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(calls))
	}
}

// ==================== SPECIAL CHARACTER TESTS ====================

// TestExtractToolCalls_SpecialChars_Unicode tests extraction with unicode in content
func TestExtractToolCalls_SpecialChars_Unicode(t *testing.T) {
	// JSON allows unicode characters directly - quotes must be escaped as \" in outer JSON
	unicodeContent := `{\n  \"message\": \"hello\",\n  \"emoji\": \"🚀\",\n  \"greek\": \"αβγδ\"\n}`
	prompt := `[TOOL:{"name":"write_file","parameters":{"path":"./unicode.txt","content":"` + unicodeContent + `"}}]`

	calls, err := ExtractToolCalls(prompt)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	if len(calls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(calls))
	}

	if !strings.Contains(calls[0].Params["content"], "hello") {
		t.Errorf("Expected content to contain 'hello'")
	}
	if !strings.Contains(calls[0].Params["content"], "🚀") {
		t.Errorf("Expected content to contain emoji")
	}
}

// TestExtractToolCalls_SpecialChars_Backslashes tests extraction with backslashes in content
func TestExtractToolCalls_SpecialChars_Backslashes(t *testing.T) {
	// Windows paths need escaped backslashes in JSON
	content := `C:\\Users\\test\\node_modules\\package.json`
	prompt := `[TOOL:{"name":"read_file","parameters":{"path":"` + content + `"}}]`

	calls, err := ExtractToolCalls(prompt)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	if len(calls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(calls))
	}

	// The path should be preserved (JSON unescapes the backslashes)
	if !strings.Contains(calls[0].Params["path"], "Users") {
		t.Errorf("Expected path to contain 'Users', got: %s", calls[0].Params["path"])
	}
}

// TestExtractToolCalls_SpecialChars_Quotes tests extraction with quotes in content
func TestExtractToolCalls_SpecialChars_Quotes(t *testing.T) {
	// Quotes need to be escaped in JSON strings
	content := `echo \"Hello \"World\"\"`
	prompt := `[TOOL:{"name":"bash","parameters":{"command":"` + content + `"}}]`

	calls, err := ExtractToolCalls(prompt)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	if len(calls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(calls))
	}

	if !strings.Contains(calls[0].Params["command"], "Hello") {
		t.Errorf("Expected command to contain 'Hello'")
	}
}

// ==================== PERFORMANCE TESTS ====================

// TestExtractToolCalls_Performance_LargeText tests extraction performance with very large text
func TestExtractToolCalls_Performance_LargeText(t *testing.T) {
	// Create a very large text (simulating a long LLM response)
	var textParts []string
	textParts = append(textParts, "This is a very long response from the LLM.\n")
	
	// Add 50 tool calls
	for i := 0; i < 50; i++ {
		textParts = append(textParts, "\nLet me check file "+string(rune(i/10+48))+string(rune(i%10+48))+"\n")
		textParts = append(textParts, `[TOOL:{"name":"read_file","parameters":{"path":"./file`+string(rune(i/10+48))+string(rune(i%10+48))+`.txt"}}]`)
		textParts = append(textParts, "\n")
	}
	
	textParts = append(textParts, "\nAll files have been checked.")
	largeText := strings.Join(textParts, "")

	calls, err := ExtractToolCalls(largeText)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	if len(calls) != 50 {
		t.Errorf("Expected 50 tool calls, got %d", len(calls))
	}
}

// ==================== REAL-WORLD SCENARIO TESTS ====================

// TestExtractToolCalls_RealWorld_CodeRefactoring tests a realistic code refactoring scenario
func TestExtractToolCalls_RealWorld_CodeRefactoring(t *testing.T) {
	realisticPrompt := `
I'll help you refactor this code. Let me first understand the current structure.

First, let's see what files we're working with:
[TOOL:{"name":"bash","parameters":{"command":"ls -la src/"}}]

Now I'll read the main module:
[TOOL:{"name":"read_file","parameters":{"path":"src/main.go"}}]

Let me also check the utilities:
[TOOL:{"name":"read_lines","parameters":{"path":"src/utils/helpers.go","start":"1","end":"100"}}]

After reviewing the code, I'll update the configuration:
[TOOL:{"name":"write_file","parameters":{"path":"src/config/settings.go","content":"package config\n\nconst (\n\tDebug = true\n\tPort  = 8080\n)"}}]

Finally, let's verify the changes:
[TOOL:{"name":"bash","parameters":{"command":"go build ./src/"}}]
`

	calls, err := ExtractToolCalls(realisticPrompt)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	expectedTools := []string{"bash", "read_file", "read_lines", "write_file", "bash"}
	if len(calls) != len(expectedTools) {
		t.Errorf("Expected %d tool calls, got %d", len(expectedTools), len(calls))
	}

	for i, expected := range expectedTools {
		if i < len(calls) && calls[i].Name != expected {
			t.Errorf("Call %d: Expected '%s', got '%s'", i, expected, calls[i].Name)
		}
	}
}

// TestExtractToolCalls_RealWorld_DatabaseMigration tests a database migration scenario
func TestExtractToolCalls_RealWorld_DatabaseMigration(t *testing.T) {
	// SQL with JSON-escaped newlines
	migrationSQL := `-- Migration: Add user preferences table\nCREATE TABLE user_preferences (\n    id SERIAL PRIMARY KEY,\n    user_id INTEGER NOT NULL REFERENCES users(id),\n    key VARCHAR(255) NOT NULL,\n    value TEXT,\n    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,\n    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,\n    UNIQUE(user_id, key)\n);\n\nCREATE INDEX idx_user_preferences_user_id ON user_preferences(user_id);\nCREATE INDEX idx_user_preferences_key ON user_preferences(key);`

	migrationPrompt := `
I'll create a database migration for user preferences.

First, let's check the current migrations:
[TOOL:{"name":"bash","parameters":{"command":"ls -la db/migrations/"}}]

Now I'll create the migration file:
[TOOL:{"name":"write_file","parameters":{"path":"db/migrations/002_add_user_preferences.sql","content":"` + migrationSQL + `"}}]

Let's apply the migration:
[TOOL:{"name":"bash","parameters":{"command":"psql -d mydb -f db/migrations/002_add_user_preferences.sql"}}]

Verify the table was created:
[TOOL:{"name":"bash","parameters":{"command":"psql -d mydb -c \"\\dt user_preferences\""}}]
`

	calls, err := ExtractToolCalls(migrationPrompt)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	if len(calls) != 4 {
		t.Errorf("Expected 4 tool calls, got %d", len(calls))
	}

	// Verify the SQL content was preserved
	if !strings.Contains(calls[1].Params["content"], "CREATE TABLE") {
		t.Errorf("Expected content to contain 'CREATE TABLE'")
	}
	if !strings.Contains(calls[1].Params["content"], "user_preferences") {
		t.Errorf("Expected content to contain 'user_preferences'")
	}
}

// TestExtractToolCalls_RealWorld_APIIntegration tests an API integration scenario
func TestExtractToolCalls_RealWorld_APIIntegration(t *testing.T) {
	apiPrompt := `
I'll help you integrate with the external API.

First, let's check if we have any existing API clients:
[TOOL:{"name":"bash","parameters":{"command":"find . -name '*api*' -type f"}}]

Let me read the existing client implementation:
[TOOL:{"name":"read_file","parameters":{"path":"./internal/api/client.go"}}]

Now I'll create a new API client with proper error handling:
[TOOL:{"name":"write_file","parameters":{"path":"./internal/api/newclient.go","content":"package api\n\nimport (\n    \"context\"\n    \"encoding/json\"\n    \"net/http\"\n)\n\ntype NewClient struct {\n    baseURL string\n    client  *http.Client\n}\n\nfunc NewNewClient(baseURL string) *NewClient {\n    return &NewClient{\n        baseURL: baseURL,\n        client:  &http.Client{},\n    }\n}\n\nfunc (c *NewClient) Fetch(ctx context.Context, path string) (*Response, error) {\n    req, err := http.NewRequestWithContext(ctx, \"GET\", c.baseURL+path, nil)\n    if err != nil {\n        return nil, err\n    }\n    \n    resp, err := c.client.Do(req)\n    if err != nil {\n        return nil, err\n    }\n    defer resp.Body.Close()\n    \n    var result Response\n    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {\n        return nil, err\n    }\n    \n    return &result, nil\n}"}}]

Test the new client:
[TOOL:{"name":"bash","parameters":{"command":"go test ./internal/api/..."}}]
`

	calls, err := ExtractToolCalls(apiPrompt)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	if len(calls) != 4 {
		t.Errorf("Expected 4 tool calls, got %d", len(calls))
	}

	// Verify the Go code content was preserved
	if !strings.Contains(calls[2].Params["content"], "package api") {
		t.Errorf("Expected content to contain 'package api'")
	}
	if !strings.Contains(calls[2].Params["content"], "func NewNewClient") {
		t.Errorf("Expected content to contain 'func NewNewClient'")
	}
}

// TestExtractToolCalls_RealWorld_DocumentationGeneration tests documentation generation scenario
func TestExtractToolCalls_RealWorld_DocumentationGeneration(t *testing.T) {
	docPrompt := `
I'll generate API documentation for your project.

First, let's find all Go files with documentation:
[TOOL:{"name":"bash","parameters":{"command":"grep -r \"// \" --include=\"*.go\" src/ | head -50"}}]

Read the main package documentation:
[TOOL:{"name":"read_lines","parameters":{"path":"src/main.go","start":"1","end":"30"}}]

Read the API package documentation:
[TOOL:{"name":"read_lines","parameters":{"path":"src/api/handlers.go","start":"1","end":"50"}}]

Now I'll create the documentation markdown:
[TOOL:{"name":"write_file","parameters":{"path":"docs/API.md","content":"# API Documentation\n\n## Overview\n\nThis document describes the API endpoints.\n\n## Endpoints\n\n### GET /api/v1/health\n\nReturns the health status of the server.\n\n**Response:**\n{\"status\": \"ok\"}\n\n### GET /api/v1/users\n\nReturns a list of users.\n\n**Response:**\n{\"users\": []}"}}]

Verify the documentation:
[TOOL:{"name":"read_file","parameters":{"path":"docs/API.md"}}]
`

	calls, err := ExtractToolCalls(docPrompt)
	if err != nil {
		t.Fatalf("Failed to extract tool calls: %v", err)
	}

	if len(calls) != 5 {
		t.Errorf("Expected 5 tool calls, got %d", len(calls))
	}

	// Verify markdown content was preserved
	if !strings.Contains(calls[3].Params["content"], "# API Documentation") {
		t.Errorf("Expected content to contain '# API Documentation'")
	}
	if !strings.Contains(calls[3].Params["content"], "### GET /api/v1/health") {
		t.Errorf("Expected content to contain health endpoint documentation")
	}
}
