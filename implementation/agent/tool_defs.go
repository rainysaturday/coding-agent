package agent

import "github.com/coding-agent/harness/inference"

// ToolInfo holds all information about a tool: its API definition and its text description.
type ToolInfo struct {
	Definition  inference.ToolDefinition
	Description string
}

// AllToolDefinitions returns a map of all available tools keyed by name.
func AllToolDefinitions() map[string]ToolInfo {
	return map[string]ToolInfo{
		"bash": {
			Definition: inference.ToolDefinition{
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
							"timeout": {
								Type:        "integer",
								Description: "Timeout in milliseconds for the command (default: 30000). Use this for long-running commands.",
							},
						},
						Required: []string{"command"},
					},
				},
			},
			Description: `bash
   Description: Execute a bash command in the terminal
   Parameters:
     - command (string, required): The bash command to execute
     - timeout (integer, optional): Timeout in milliseconds for the command (default: 30000). Use this for long-running commands.
   How to call: Use the bash tool when you need to run shell commands, install packages, build projects, check file system, etc.
   Example use case: "ls -la", "cat file.txt", "npm install", "pip install -r requirements.txt"`,
		},
		"read_file": {
			Definition: inference.ToolDefinition{
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
			Description: `read_file
   Description: Read the contents of a file
   Parameters:
     - path (string, required): The path to the file to read
   How to call: Use read_file to view the contents of any file before making changes.
   Example use case: Reading source files, configuration files, documentation`,
		},
		"read_lines": {
			Definition: inference.ToolDefinition{
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
			Description: `read_lines
   Description: Read a specific line range from a file
   Parameters:
     - path (string, required): The path to the file
     - start (integer, required): The starting line number (1-indexed)
     - end (integer, required): The ending line number (1-indexed)
   How to call: Use read_lines when you only need to view a portion of a large file.
   Example use case: Viewing lines 1-50 of a large source file, checking specific sections`,
		},
		"write_file": {
			Definition: inference.ToolDefinition{
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
			Description: `write_file
   Description: Write content to a file
   Parameters:
     - path (string, required): The path to the file to write
     - content (string, required): The content to write to the file
   How to call: Use write_file to create new files or completely overwrite existing files.
   Example use case: Creating new source files, writing configuration, saving output
   Note: For multi-line content, use \n to represent newlines in the content parameter`,
		},
		"insert_lines": {
			Definition: inference.ToolDefinition{
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
			Description: `insert_lines
   Description: Insert lines at a specific line number
   Parameters:
     - path (string, required): The path to the file
     - line (integer, required): The line number where insertion should occur (1-indexed)
     - lines (string, required): The lines to insert (use \n for newlines)
   How to call: Use insert_lines to add new content without replacing existing content.
   Example use case: Adding imports, inserting new functions, adding comments
   Note: Inserting at line 1 adds at the beginning; inserting beyond file length appends`,
		},
		"replace_text": {
			Definition: inference.ToolDefinition{
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
			Description: `replace_text
   Description: Find and replace text in a file by searching for a pattern
   Parameters:
     - path (string, required): The path to the file to modify
     - search (string, required): Text pattern to find (exact match, not regex)
     - replace (string, required): Replacement text
     - count (integer, optional): Number of occurrences to replace (default: 1, use -1 for all)
   How to call: Use replace_text when you know the text to find but not the line numbers.
   Example use case: Renaming variables, updating function names, fixing typos throughout a file`,
		},
		"move_text": {
			Definition: inference.ToolDefinition{
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
			Description: `move_text
   Description: Move a block of text from one location to another. Extracts lines from a source file and inserts them at a target location (same file or different file). Automatically creates target file and directories if needed.
   Parameters:
     - source_path (string, required): Path to the source file to extract lines from
     - source_start (integer, required): Starting line number in source file (1-indexed)
     - source_end (integer, required): Ending line number in source file (1-indexed, inclusive)
     - target_path (string, required): Path to the target file to insert lines into
     - target_line (integer, required): Line number in target file to insert before (1-indexed)
   How to call: Use move_text to move code blocks or text between files or within the same file.
   Example use case: Moving a function from one file to another, reorganizing code sections`,
		},
		"list_files": {
			Definition: inference.ToolDefinition{
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
			Description: `list_files
   Description: List files and directories in a path, similar to the ls command
   Parameters:
     - path (string, optional): The path to the file or directory to list (defaults to current directory if not specified)
     - flags (array, optional): List of ls-style flags to control output (e.g., 'l' for long format, 'a' for all including hidden, 'h' for human-readable sizes, 't' for time-sorted, 'S' for size-sorted, 'R' for recursive)
   How to call: Use list_files to see files, folders, sizes, permissions, and other information formatted like a simple ls command.
   Example use case: Listing directory contents with details, checking file sizes, viewing hidden files`,
		},
		"grep": {
			Definition: inference.ToolDefinition{
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
			Description: `grep
   Description: Search through file contents using grep-like pattern matching
   Parameters:
     - path (string, optional): Path to search (defaults to current directory if not specified)
     - pattern (string, required): Pattern to search for (supports regex)
     - flags (array, optional): List of grep-style flags to control output (e.g., '-n' for line numbers, '-i' for case insensitive, '-r' for recursive, '-f' for pattern file, '-a' for all including hidden, '-c' for count, '-v' for invert match, '-l' for filenames only)
   How to call: Use grep to find specific patterns or text within files.
   Example use case: Finding where a function is defined, searching for error messages, locating configuration values`,
		},
		"git_log": {
			Definition: inference.ToolDefinition{
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
			Description: `git_log
   Description: Show commit logs from a git repository
   Parameters:
     - path (string, optional): Path to the git repository (defaults to current directory)
     - reference (string, optional): Git reference to view log from (branch name, tag, or commit hash; defaults to HEAD)
     - count (integer, optional): Number of commits to display (defaults to 10)
     - flags (array, optional): List of git log flags to control output (e.g., '--oneline', '--stat', '--patch', '--follow', '--grep')
   How to call: Use git_log to view commit history and understand changes in the repository.
   Example use case: Reviewing recent changes, finding when a bug was introduced, understanding project history`,
		},
		"git_show": {
			Definition: inference.ToolDefinition{
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
			Description: `git_show
   Description: Show information about a git commit
   Parameters:
     - path (string, optional): Path to the git repository (defaults to current directory)
     - commit (string, optional): Commit to show (defaults to HEAD)
     - flags (array, optional): List of git show flags to control output (e.g., '--stat', '--patch', '--name-status')
   How to call: Use git_show to examine the details of a specific commit, including its changes and metadata.
   Example use case: Examining a specific commit's changes, reviewing what was modified in a particular update`,
		},
		"git_diff": {
			Definition: inference.ToolDefinition{
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
			Description: `git_diff
   Description: Show changes between commits, commit and working tree, etc.
   Parameters:
     - path (string, optional): Path to the git repository (defaults to current directory)
     - reference1 (string, optional): First git reference for comparison (commit hash, branch, tag; omit for working tree)
     - reference2 (string, optional): Second git reference for comparison (commit hash, branch, tag; omit for index or working tree)
     - flags (array, optional): List of git diff flags to control output (e.g., '--stat', '--patch', '--name-status', '--numstat', '--summary', '--color')
   How to call: Use git_diff to compare different versions of files, branches, or commits.
   Example use case: Comparing changes between two branches, viewing modifications in a specific commit, checking differences in the working tree`,
		},
		"view_image": {
			Definition: inference.ToolDefinition{
				Type: "function",
				Function: inference.FunctionDefinition{
					Name:        "view_image",
					Description: "View a local image file. Reads the image from disk and sends it to a vision-capable model for analysis. Returns a description of the image contents. Supported formats: PNG, JPEG, WEBP, GIF.",
					Parameters: inference.ParameterSchema{
						Type:     "object",
						Required: []string{"path"},
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
			Description: `view_image
   Description: View a local image file. Reads the image from disk and sends it to a vision-capable model for analysis. Returns a description of the image contents.
   Parameters:
     - prompt (string, optional): Custom prompt or question to guide the vision analysis. When provided, this prompt is used instead of the default description prompt.
     - path (string, required): Path to the image file to view
   Supported formats: PNG, JPEG, WEBP, GIF
   How to call: Use view_image when you need to see what's in an image file, read text from screenshots, analyze diagrams, etc.
   Example use case: "What does this screenshot show?", "Read the text in this diagram"`,
		},
		"todo": {
			Definition: inference.ToolDefinition{
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
			Description: `todo
   Description: Manage a personal task list for tracking work-in-progress during development
   Parameters:
     - action (string, required): The action to perform (add, complete, remove, or list)
     - id (integer, optional): Item ID (required for complete/remove)
     - description (string, optional): Task description (required for add)
   How to call: Use the todo tool to break down complex tasks into tracked sub-items. This helps you remember what to do between turns.
   Example use case: Creating a checklist for a multi-step refactoring task`,
		},
		"subagent": {
			Definition: inference.ToolDefinition{
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
			},
			Description: `subagent
   Description: Spawn a sub-agent to work on a task independently. The sub-agent runs as a separate process and returns only its conclusion/summary.
   Parameters:
     - prompt (string, required): The task description for the sub-agent. Be specific and clear about what you want the sub-agent to accomplish.
     - persona (string, optional): A persona to give the sub-agent. For example: "Expert Go developer", "Code reviewer focused on security", "Documentation writer".
   How to call: Use the subagent tool when you need to spawn a sub-agent for parallel tasks.
   Example use case: Running independent research tasks in parallel, having code reviewed while you continue working`,
		},
		"activate_skill": {
			Definition: inference.ToolDefinition{
				Type: "function",
				Function: inference.FunctionDefinition{
					Name:        "activate_skill",
					Description: "Load a skill's instructions and bundled resources into context. Skills are discovered from the AVAILABLE SKILLS catalog.",
					Parameters: inference.ParameterSchema{
						Type: "object",
						Properties: map[string]inference.Property{
							"name": {
								Type:        "string",
								Description: "The name of the skill to activate (must be one of the available skills).",
							},
						},
						Required: []string{"name"},
					},
				},
			},
			Description: `activate_skill
   Description: Load a skill's instructions and bundled resources into context.
   Parameters:
     - name (string, required): The name of the skill to activate (must be one of the available skills).
   How to call: Use activate_skill when a listed AVAILABLE SKILL matches the current task, to gain its specialized knowledge and workflows.
   Example use case: Activating a "code-review" skill before reviewing a pull request`,
		},
	}
}
