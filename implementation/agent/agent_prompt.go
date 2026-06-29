package agent

import (
	"fmt"
	"os"
	"runtime"
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

// buildSystemPrompt builds the system prompt with tool definitions.
// When readOnly is true, only read-only tools are included.
func buildSystemPrompt(readOnly bool, persona string, summaryOnly bool) string {
	// Get environment information
	envInfo := getEnvironmentInfo()

	if readOnly {
		return buildReadOnlySystemPrompt(envInfo, persona, summaryOnly)
	}

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

AVAILABLE TOOLS:

1. bash
   Description: Execute a bash command in the terminal
   Parameters:
     - command (string, required): The bash command to execute
   How to call: Use the bash tool when you need to run shell commands, install packages, build projects, check file system, etc.
   Example use case: "ls -la", "cat file.txt", "npm install", "pip install -r requirements.txt"

2. read_file
   Description: Read the contents of a file
   Parameters:
     - path (string, required): The path to the file to read
   How to call: Use read_file to view the contents of any file before making changes.
   Example use case: Reading source files, configuration files, documentation

3. write_file
   Description: Write content to a file
   Parameters:
     - path (string, required): The path to the file to write
     - content (string, required): The content to write to the file
   How to call: Use write_file to create new files or completely overwrite existing files.
   Example use case: Creating new source files, writing configuration, saving output
   Note: For multi-line content, use \n to represent newlines in the content parameter

4. read_lines
   Description: Read a specific line range from a file
   Parameters:
     - path (string, required): The path to the file
     - start (integer, required): The starting line number (1-indexed)
     - end (integer, required): The ending line number (1-indexed)
   How to call: Use read_lines when you only need to view a portion of a large file.
   Example use case: Viewing lines 1-50 of a large source file, checking specific sections

5. insert_lines
   Description: Insert lines at a specific line number
   Parameters:
     - path (string, required): The path to the file
     - line (integer, required): The line number where insertion should occur (1-indexed)
     - lines (string, required): The lines to insert (use \n for newlines)
   How to call: Use insert_lines to add new content without replacing existing content.
   Example use case: Adding imports, inserting new functions, adding comments
   Note: Inserting at line 1 adds at the beginning; inserting beyond file length appends

6. replace_text
   Description: Find and replace text in a file by searching for a pattern
   Parameters:
     - path (string, required): The path to the file to modify
     - search (string, required): Text pattern to find (exact match, not regex)
     - replace (string, required): Replacement text
     - count (integer, optional): Number of occurrences to replace (default: 1, use -1 for all)
   How to call: Use replace_text when you know the text to find but not the line numbers.
   Example use case: Renaming variables, updating function names, fixing typos throughout a file

7. view_image
    Description: View a local image file. Reads the image from disk and sends it to a vision-capable model for analysis. Returns a description of the image contents.
    Parameters:
      - prompt (string, optional): Custom prompt or question to guide the vision analysis. When provided, this prompt is used instead of the default description prompt.
      - path (string, required): Path to the image file to view
    Supported formats: PNG, JPEG, WEBP, GIF
    How to call: Use view_image when you need to see what's in an image file, read text from screenshots, analyze diagrams, etc.
    Example use case: "What does this screenshot show?", "Read the text in this diagram"

8. todo
    Description: Manage a personal task list for tracking work-in-progress during development
    Parameters:
      - action (string, required): The action to perform (add, complete, remove, or list)
      - id (integer, optional): Item ID (required for complete/remove)
      - description (string, optional): Task description (required for add)
    How to call: Use the todo tool to break down complex tasks into tracked sub-items. This helps you remember what to do between turns.
    Example use case: Creating a checklist for a multi-step refactoring task


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
6. No unintended side effects or broken dependencies`, envInfo)

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
func buildReadOnlySystemPrompt(envInfo string, persona string, summaryOnly bool) string {
	basePrompt := fmt.Sprintf(`You are a helpful coding assistant operating in READ-ONLY MODE. You have access only to the following read-only tools.

%s

IMPORTANT: This session is in read-only mode. You can ONLY read files and list directories.
You CANNOT modify, write, delete, execute, or make any changes to files or the system.

TOOL CALLING FORMAT:
- When you need to use a tool, the API will return a response containing tool calls
- Execute each tool call and report the result back as a tool message
- You do NOT need to construct JSON manually - the tool calling API handles the formatting
- Each tool has specific parameters that must be provided (marked as "required")

AVAILABLE TOOLS:

1. read_file
   Description: Read the contents of a file
   Parameters:
     - path (string, required): The path to the file to read
   How to call: Use read_file to view the contents of any file.
   Example use case: Reading source files, configuration files, documentation

2. read_lines
   Description: Read a specific line range from a file
   Parameters:
     - path (string, required): The path to the file
     - start (integer, required): The starting line number (1-indexed)
     - end (integer, required): The ending line number (1-indexed)
   How to call: Use read_lines when you only need to view a portion of a large file.
   Example use case: Viewing lines 1-50 of a large source file, checking specific sections

3. list_files
   Description: List files and directories in a path, similar to the ls command
   Parameters:
     - path (string, optional): The path to the file or directory to list (defaults to current directory if not specified)
     - flags (array, optional): List of ls-style flags to control output (e.g., 'l' for long format, 'a' for all including hidden, 'h' for human-readable sizes, 't' for time-sorted, 'S' for size-sorted, 'R' for recursive)
   How to call: Use list_files to see files, folders, sizes, permissions, and other information formatted like a simple ls command.
   Example use case: Listing directory contents with details, checking file sizes, viewing hidden files
4. grep
    Description: Search through file contents using grep-like pattern matching
    Parameters:
      - path (string, optional): Path to search (defaults to current directory if not specified)
      - pattern (string, required): Pattern to search for (supports regex)
      - flags (array, optional): List of grep-style flags to control output (e.g., '-n' for line numbers, '-i' for case insensitive, '-r' for recursive)
    How to call: Use grep to find specific patterns or text within files.
    Example use case: Finding where a function is defined, searching for error messages, locating configuration values

5. git_log
    Description: Show commit logs from a git repository
    Parameters:
      - path (string, optional): Path to the git repository (defaults to current directory)
      - reference (string, optional): Git reference to view log from (branch name, tag, or commit hash; defaults to HEAD)
      - count (integer, optional): Number of commits to display (defaults to 10)
      - flags (array, optional): List of git log flags to control output (e.g., '--oneline', '--stat', '--patch', '--follow', '--grep')
    How to call: Use git_log to view commit history and understand changes in the repository.
    Example use case: Reviewing recent changes, finding when a bug was introduced, understanding project history

6. git_show
    Description: Show information about a git commit
    Parameters:
      - path (string, optional): Path to the git repository (defaults to current directory)
      - commit (string, optional): Commit to show (defaults to HEAD)
      - flags (array, optional): List of git show flags to control output (e.g., '--stat', '--patch', '--name-status')
    How to call: Use git_show to examine the details of a specific commit, including its changes and metadata.
    Example use case: Examining a specific commit's changes, reviewing what was modified in a particular update

7. git_diff
    Description: Show changes between commits, commit and working tree, etc.
    Parameters:
      - path (string, optional): Path to the git repository (defaults to current directory)
      - reference1 (string, optional): First git reference for comparison (commit hash, branch, tag; omit for working tree)
      - reference2 (string, optional): Second git reference for comparison (commit hash, branch, tag; omit for index or working tree)
      - flags (array, optional): List of git diff flags to control output (e.g., '--stat', '--patch', '--name-status', '--numstat', '--summary', '--color')
     - prompt (string, optional): Custom prompt or question to guide the vision analysis. When provided, this prompt is used instead of the default description prompt.
    How to call: Use git_diff to compare different versions of files, branches, or commits.
    Example use case: Comparing changes between two branches, viewing modifications in a specific commit, checking differences in the working tree

8. view_image
    Description: View a local image file. Reads the image from disk and sends it to a vision-capable model for analysis. Returns a description of the image contents.
    Parameters:
      - path (string, required): Path to the image file to view
    Supported formats: PNG, JPEG, WEBP, GIF
    How to call: Use view_image when you need to see what's in an image file, read text from screenshots, analyze diagrams, etc.
    Example use case: "What does this screenshot show?", "Read the text in this diagram"

9. todo
    Description: Manage a personal task list for tracking work-in-progress during development
    Parameters:
      - action (string, required): The action to perform (add, complete, remove, or list)
      - id (integer, optional): Item ID (required for complete/remove)
      - description (string, optional): Task description (required for add)
    How to call: Use the todo tool to track tasks. In read-only mode, only list and remove actions are available.
    Example use case: Listing current tasks or removing completed items


TOOL CALLING BEST PRACTICES:
1. Use read_file, read_lines, and list_files to explore and read files
2. Use grep to search for patterns and text within files
3. Use git_log, git_show, and git_diff to explore git history and changes
4. Remember: you cannot modify any files or execute commands

NOTE: If the user asks you to write, modify, delete, or execute anything, explain that you are in read-only mode and cannot perform write operations.`, envInfo)

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
