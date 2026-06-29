package agent

import "github.com/coding-agent/harness/inference"

// buildTools builds the tool definitions for the OpenAI API.
// When readOnly is true, only read-only tools (read_file, read_lines, list_files, grep, git_log, git_show, git_diff) are returned.
// When experimental is false, the subagent tool is not included.
func buildTools(readOnly bool, experimental bool) []inference.ToolDefinition {
	if readOnly {
		return buildReadOnlyTools()
	}

	baseTools := []inference.ToolDefinition{
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "bash",
				Description: "Execute a bash command in the terminal",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"command": {
							Type:        "string",
							Description: "The bash command to execute",
						},
					},
					Required: []string{"command"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "read_file",
				Description: "Read the contents of a file",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the file to read",
						},
					},
					Required: []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "write_file",
				Description: "Write content to a file",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the file to write",
						},
						"content": {
							Type:        "string",
							Description: "Content to write to the file",
						},
					},
					Required: []string{"path", "content"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "read_lines",
				Description: "Read a specific line range from a file",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the file to read",
						},
						"start": {
							Type:        "integer",
							Description: "Starting line number (1-indexed)",
						},
						"end": {
							Type:        "integer",
							Description: "Ending line number (1-indexed)",
						},
					},
					Required: []string{"path", "start", "end"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "insert_lines",
				Description: "Insert lines at a specific line number in a file",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "File path to modify",
						},
						"line": {
							Type:        "integer",
							Description: "Line number to insert before (1-indexed)",
						},
						"lines": {
							Type:        "string",
							Description: "Lines to insert (use \\n for newlines)",
						},
					},
					Required: []string{"path", "line", "lines"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "replace_text",
				Description: "Find and replace text in a file by searching for a pattern",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "File path to modify",
						},
						"search": {
							Type:        "string",
							Description: "Text pattern to find (exact match, not regex)",
						},
						"replace": {
							Type:        "string",
							Description: "Replacement text",
						},
						"count": {
							Type:        "integer",
							Description: "Number of occurrences to replace (default: 1, use -1 for all)",
						},
					},
					Required: []string{"path", "search", "replace"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "move_text",
				Description: "Move a block of text from one location to another. Extracts lines from a source file and inserts them at a target location (same file or different file). Automatically creates target file and directories if needed.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"source_path": {
							Type:        "string",
							Description: "Path to the source file to extract lines from",
						},
						"source_start": {
							Type:        "integer",
							Description: "Starting line number in source file (1-indexed)",
						},
						"source_end": {
							Type:        "integer",
							Description: "Ending line number in source file (1-indexed, inclusive)",
						},
						"target_path": {
							Type:        "string",
							Description: "Path to the target file to insert lines into",
						},
						"target_line": {
							Type:        "integer",
							Description: "Line number in target file to insert before (1-indexed)",
						},
					},
					Required: []string{"source_path", "source_start", "source_end", "target_path", "target_line"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "todo",
				Description: "Manage a personal task list for tracking work-in-progress during development",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"action": {
							Type:        "string",
							Description: "The action to perform: add, complete, remove, or list",
						},
						"id": {
							Type:        "integer",
							Description: "The ID of the todo item (required for complete, remove; not for add or list)",
						},
						"description": {
							Type:        "string",
							Description: "The description of the todo item (required for add; not for complete, remove, or list)",
						},
					},
					Required: []string{"action"},
				},
			},
		},
	}

	tools := baseTools

	// Add view_image tool (always available, including read-only)
	tools = append(tools, inference.ToolDefinition{
		Type: "function",
		Function: inference.FunctionDefinition{
			Name:        "view_image",
			Description: "View a local image file. Reads the image from disk and sends it to a vision-capable model for analysis. Returns a description of the image contents. Supported formats: PNG, JPEG, WEBP, GIF.",
			Parameters: inference.ParameterSchema{
				Type: "object",
				Properties: map[string]inference.Property{
					"path": {
						Type:        "string",
						Description: "Path to the image file to view",
					},
					"prompt": {
						Type:        "string",
						Description: "Optional custom prompt or question to guide the vision analysis. When provided, this prompt is used instead of the default description prompt.",
					},
				},
				Required: []string{"path"},
			},
		},
	})

	if experimental {
		tools = append(tools, inference.ToolDefinition{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "subagent",
				Description: "Spawn a sub-agent to work on a task independently. The sub-agent runs as a separate process and returns only its conclusion/summary.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"prompt": {
							Type:        "string",
							Description: "The task description for the sub-agent. Be specific and clear about what you want the sub-agent to accomplish.",
						},
						"persona": {
							Type:        "string",
							Description: "A persona to give the sub-agent. For example: \"Expert Go developer\", \"Code reviewer focused on security\", \"Documentation writer\".",
						},
					},
					Required: []string{"prompt"},
				},
			},
		})
	}

	return tools
}

// buildReadOnlyTools returns only the read-only tool definitions.
func buildReadOnlyTools() []inference.ToolDefinition {
	return []inference.ToolDefinition{
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "read_file",
				Description: "Read the contents of a file",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the file to read",
						},
					},
					Required: []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "read_lines",
				Description: "Read a specific line range from a file",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the file to read",
						},
						"start": {
							Type:        "integer",
							Description: "Starting line number (1-indexed)",
						},
						"end": {
							Type:        "integer",
							Description: "Ending line number (1-indexed)",
						},
					},
					Required: []string{"path", "start", "end"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "list_files",
				Description: "List files and directories in a path, similar to the ls command",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the file or directory to list (defaults to current directory if not specified)",
						},
						"flags": {
							Type:        "array",
							Description: "List of ls-style flags to control output (e.g., 'l' for long format, 'a' for all including hidden, 'h' for human-readable sizes, 't' for time-sorted, 'S' for size-sorted, 'R' for recursive)",
							Items: &inference.Property{
								Type: "string",
							},
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "grep",
				Description: "Search through file contents using grep-like pattern matching",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to search (defaults to current directory if not specified)",
						},
						"pattern": {
							Type:        "string",
							Description: "Pattern to search for (supports regex)",
						},
						"flags": {
							Type:        "array",
							Description: "List of grep-style flags to control output (e.g., '-n' for line numbers, '-i' for case insensitive, '-r' for recursive, '-f' for pattern file, '-a' for all including hidden, '-c' for count, '-v' for invert match, '-l' for filenames only)",
							Items: &inference.Property{
								Type: "string",
							},
						},
					},
					Required: []string{"pattern"},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "git_log",
				Description: "Show commit logs from a git repository",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the git repository (defaults to current directory)",
						},
						"reference": {
							Type:        "string",
							Description: "Git reference to view log from (branch name, tag, or commit hash; defaults to HEAD)",
						},
						"count": {
							Type:        "integer",
							Description: "Number of commits to display (defaults to 10)",
						},
						"grep": {
							Type:        "string",
							Description: "Search commit messages for this pattern (used with '--grep' flag)",
						},
						"flags": {
							Type:        "array",
							Description: "List of git log flags to control output (e.g., 's' for short format, 'm' for merges, 'no-merges', 'stat', 'patch', 'oneline', 'shortstat', 'follow', 'grep' to search commit messages, 'decorate', 'graph', 'first-parent')",
							Items: &inference.Property{
								Type: "string",
							},
						},
					},
					Required: []string{},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "git_show",
				Description: "Show information about a git commit",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the git repository (defaults to current directory)",
						},
						"commit": {
							Type:        "string",
							Description: "Commit to show (defaults to HEAD)",
						},
						"flags": {
							Type:        "array",
							Description: "List of git show flags to control output (e.g., 'stat', 'patch', 'p', 'name-status', 'name-only', 'shortstat', 'numstat', 'oneline', 's' for short format, 'no-patch', 'summary', 'r' for rename detection, 'M' for copy detection)",
							Items: &inference.Property{
								Type: "string",
							},
						},
					},
					Required: []string{},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "git_diff",
				Description: "Show changes between commits, commit and working tree, etc.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the git repository (defaults to current directory)",
						},
						"reference1": {
							Type:        "string",
							Description: "First git reference for comparison (commit hash, branch, tag; omit for working tree)",
						},
						"reference2": {
							Type:        "string",
							Description: "Second git reference for comparison (commit hash, branch, tag; omit for index or working tree)",
						},
						"flags": {
							Type:        "array",
							Description: "List of git diff flags to control output (e.g., 'stat', 'patch', 'p', 'name-status', 'name-only', 'shortstat', 'numstat', 'color', 'summary', 'compact-summary', 'stat-width', 'ignore-space-at-eol', 'ignore-space-change', 'ignore-all-space', 'unified', 'raw', 'r' for rename detection, 'M' for copy detection, 'patience', 'minimal')",
							Items: &inference.Property{
								Type: "string",
							},
						},
					},
					Required: []string{},
				},
			},
		},
		{
			Type: "function",
			Function: inference.FunctionDefinition{
				Name:        "view_image",
				Description: "View a local image file. Reads the image from disk and sends it to a vision-capable model for analysis. Returns a description of the image contents. Supported formats: PNG, JPEG, WEBP, GIF.",
				Parameters: inference.ParameterSchema{
					Type: "object",
					Properties: map[string]inference.Property{
						"path": {
							Type:        "string",
							Description: "Path to the image file to view",
						},
						"prompt": {
							Type:        "string",
							Description: "Optional custom prompt or question to guide the vision analysis. When provided, this prompt is used instead of the default description prompt.",
						},
					},
				},
			},
		},
	}
}
