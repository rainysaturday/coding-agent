package agent

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// getEnvironmentInfo gathers runtime environment information.
func getEnvironmentInfo() string {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "unknown"
	}

	// Get executable path
	exePath, err := os.Executable()
	if err != nil {
		exePath = "unknown"
	}

	// Get OS and architecture
	osInfo := runtime.GOOS
	archInfo := runtime.GOARCH

	return fmt.Sprintf(`ENVIRONMENT INFORMATION:
- Current Working Directory: %s
- Agent Executable: %s
- Operating System: %s
- Architecture: %s

You can use the coding-agent to spawn sub-agents for parallel tasks using the subagent tool.
When you need to run a subagent, use the 'subagent' tool with a clear task description.
The subagent will run independently and return its conclusion/summary.
`, cwd, exePath, osInfo, archInfo)
}

// toolDescription returns the text description for a tool by name.
func toolDescription(name string) string {
	allDefs := AllToolDefinitions()
	if info, ok := allDefs[name]; ok {
		return info.Description
	}
	return ""
}

// toolDescriptions returns a numbered list of tool descriptions for the given tool names.
func toolDescriptions(names []string) string {
	var parts []string
	for i, name := range names {
		desc := toolDescription(name)
		if desc != "" {
			parts = append(parts, fmt.Sprintf("%d. %s", i+1, desc))
		}
	}
	return strings.Join(parts, "\n\n")
}

// buildToolListSection creates the "AVAILABLE TOOLS" section for the system prompt.
func buildToolListSection(names []string, readOnly bool) string {
	if readOnly {
		return fmt.Sprintf(`AVAILABLE TOOLS:

%s`, toolDescriptions(names))
	}
	return fmt.Sprintf(`AVAILABLE TOOLS:

%s`, toolDescriptions(names))
}

// buildSystemPrompt builds the system prompt with tool definitions.
// When toolsList is non-empty, it overrides the defaults for the given mode.
func buildSystemPrompt(readOnly bool, persona string, summaryOnly bool, toolsList []string) string {
	// Get environment information
	envInfo := getEnvironmentInfo()
	toolNames := getToolNames(readOnly, false, toolsList)

	if readOnly {
		return buildReadOnlySystemPrompt(envInfo, toolNames, persona, summaryOnly)
	}

	return buildNormalSystemPrompt(envInfo, toolNames, persona, summaryOnly)
}

// buildNormalSystemPrompt builds the system prompt for normal (non-read-only) mode.
func buildNormalSystemPrompt(envInfo string, toolNames []string, persona string, summaryOnly bool) string {
	toolsSection := buildToolListSection(toolNames, false)

	basePrompt := fmt.Sprintf(`You are a helpful coding assistant. You have access to the following tools.

%s

TOOL CALLING FORMAT:
- When you need to use a tool, the API will return a response containing tool calls
- Execute each tool call and report the result back as a tool message
- You do NOT need to construct JSON manually - the tool calling API handles the formatting
- Each tool has specific parameters that must be provided (marked as "required")

EXAMPLE workflow:
1. User asks you to list files in a directory
2. The API returns a tool call: {"name": "bash", "arguments": {"command": "ls -la /path"}}
3. Execute the tool and report the result back as a tool message with the matching tool_call_id
4. The API processes the result and may return another tool call or your final answer

%s

TOOL CALLING BEST PRACTICES:
1. Always read a file first (using read_file or read_lines) to understand its contents
2. When modifying files, be precise about what you're changing
3. For multi-line content, properly format with \n for newlines
4. Verify your changes by re-reading files after writing
5. Test code by running appropriate commands for the language (e.g., go build, npm test, pytest, etc.)

VERIFICATION REQUIREMENTS:
- ALWAYS double-check your work before considering a task complete
- Verify that created/modified files exist and contain the expected content
- Test code execution when possible (e.g., run go build, npm test, pytest, cargo test, etc.)
- Validate that changes meet the user's requirements
- If you make multiple changes, verify each one independently
- Re-read files after writing to confirm content was written correctly
- Run validation commands (e.g., go vet, gofmt -d, pylint, eslint, cat to view files)
- If verification fails, fix the issue and re-verify
- Provide a final verification summary before concluding the task

Verification Checklist:
1. Files exist at the expected paths
2. File content matches the intended changes
3. Code compiles/builds without errors (for compiled languages)
4. Code formatting and linting (e.g., gofmt, black, prettier, rustfmt, etc.)
5. Changes align with user requirements
6. No unintended side effects or broken dependencies`, envInfo, toolsSection)

	// Add persona section if provided
	if persona != "" {
		basePrompt += fmt.Sprintf("\n\nYOUR PERSONA:\n%s\n", persona)
	}

	// Add summary-only instruction if needed
	if summaryOnly {
		basePrompt += "\n\nIMPORTANT OUTPUT INSTRUCTION: You are running in summary-only mode. Your final output should be a concise summary/conclusion of the work completed. Do NOT include verbose explanations, step-by-step details, or code. Only provide the essential outcome and any critical findings."
	}

	return basePrompt
}

// buildReadOnlySystemPrompt builds a system prompt for read-only mode.
func buildReadOnlySystemPrompt(envInfo string, toolNames []string, persona string, summaryOnly bool) string {
	toolsSection := buildToolListSection(toolNames, true)

	basePrompt := fmt.Sprintf(`You are a helpful coding assistant operating in READ-ONLY MODE. You have access only to the following read-only tools.

%s

IMPORTANT: This session is in read-only mode. You can ONLY read files and list directories.
You CANNOT modify, write, delete, execute, or make any changes to files or the system.

TOOL CALLING FORMAT:
- When you need to use a tool, the API will return a response containing tool calls
- Execute each tool call and report the result back as a tool message
- You do NOT need to construct JSON manually - the tool calling API handles the formatting
- Each tool has specific parameters that must be provided (marked as "required")

%s

TOOL CALLING BEST PRACTICES:
1. Use read_file, read_lines, and list_files to explore and read files
2. Use grep to search for patterns and text within files
3. Use git_log, git_show, and git_diff to explore git history and changes
4. Remember: you cannot modify any files or execute commands

NOTE: If the user asks you to write, modify, delete, or execute anything, explain that you are in read-only mode and cannot perform write operations.`,
		envInfo, toolsSection)

	// Add persona section if provided
	if persona != "" {
		basePrompt += fmt.Sprintf("\n\nYOUR PERSONA:\n%s\n", persona)
	}

	// Add summary-only instruction if needed
	if summaryOnly {
		basePrompt += "\n\nIMPORTANT OUTPUT INSTRUCTION: You are running in summary-only mode. Your final output should be a concise summary/conclusion of the work completed. Do NOT include verbose explanations, step-by-step details, or code. Only provide the essential outcome and any critical findings."
	}

	return basePrompt
}
