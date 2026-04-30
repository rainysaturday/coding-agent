# Requirement 033: Read-Only Mode

## Description
The harness must support a `--read-only` flag that, when specified, restricts the coding agent to only read-only operations. In read-only mode, the agent has access only to the `read_file`, `read_lines`, `list_files`, `grep`, `git_log`, `git_show`, and `git_diff` tools. All write-modifying tools (such as `bash`, `write_file`, `insert_lines`, `replace_text`) are disabled and not available to the agent.

## Acceptance Criteria
- [ ] `--read-only` flag is available as a command-line option
- [ ] When `--read-only` is specified, only `read_file`, `read_lines`, `list_files`, `grep`, `git_log`, `git_show`, and `git_diff` tools are available
- [ ] All write-modifying tools are disabled when read-only mode is active
- [ ] System prompt is modified to only describe the available read-only tools
- [ ] Tool definitions sent to the LLM only include `read_file`, `read_lines`, `list_files`, `grep`, `git_log`, `git_show`, and `git_diff`
- [ ] Attempting to call a write tool returns an error indicating the tool is not available in read-only mode
- [ ] User is informed that read-only mode is active

## Command-Line Usage

```bash
# Normal mode (all tools available)
./coding-agent --prompt "What files are in this directory?"

# Read-only mode (only read_file, list_files, read_lines, grep, git_log, and git_show)
./coding-agent --read-only --prompt "What files are in this directory?"
```

## Tool Behavior in Read-Only Mode

### Available Tools
- `read_file` - Read file contents (available)
- `read_lines` - Read a specific line range from a file (available)
- `list_files` - List directory contents (available)
- `grep` - Search for patterns in files (available)
- `git_log` - View the commit history of a git repository (available)
- `git_show` - View detailed information about a specific git commit (available)
- `git_diff` - Compare changes between git commits, branches, or the working tree (available)

### Disabled Tools
All other tools are disabled when read-only mode is active:
- `bash` - Execute shell commands (disabled)
- `write_file` - Write to files (disabled)
- `insert_lines` - Insert lines in files (disabled)
- `replace_text` - Replace text in files (disabled)

When a disabled tool is attempted to be called, the tool executor should return an error indicating the tool is not available in read-only mode.

## System Prompt in Read-Only Mode

When `--read-only` is specified, the system prompt should:
1. Indicate that the agent is operating in read-only mode
2. Only list and describe the `read_file`, `read_lines`, `list_files`, `grep`, `git_log`, `git_show`, and `git_diff` tools
3. Explicitly state that no write operations are allowed
4. Inform the user that the agent can only read files, list directory contents, read specific line ranges, search for patterns, view git commit history, inspect commits, and compare changes

## Example System Prompt Content

When read-only mode is active, the "AVAILABLE TOOLS" section of the system prompt should only contain:

```
AVAILABLE TOOLS:

1. read_file
   Description: Read the contents of a file
   Parameters:
     - path (string, required): The path to the file to read

2. read_lines
   Description: Read a specific line range from a file
   Parameters:
     - path (string, required): Path to the file to read
     - start (integer, required): Starting line number (1-indexed)
     - end (integer, required): Ending line number (1-indexed)

3. list_files
   Description: List files and directories in a path, similar to the ls command
   Parameters:
     - path (string, optional): The path to the file or directory to list
     - flags (array, optional): List of ls-style flags to control output

4. grep
   Description: Search through file contents using grep-like pattern matching
   Parameters:
     - path (string, optional): Path to search within (file or directory, defaults to current directory)
     - pattern (string, required): Search pattern (regular expression)
     - flags (array, optional): List of grep-style flags (e.g., '-n' for line numbers, '-i' for case insensitive, '-r' for recursive)

5. git_log
   Description: View the commit history of a git repository
   Parameters:
     - path (string, optional): Path within the repository to view history for
     - reference (string, optional): Git reference to view log from (branch, tag, or commit hash)
     - count (integer, optional): Number of commits to display
     - flags (array, optional): List of git-log-style flags (e.g., '--oneline', '--stat', '--patch', '--follow', '--grep')

6. git_show
   Description: View detailed information about a specific git commit
   Parameters:
     - commit (string, optional): Git reference for the commit to show (branch, tag, or commit hash)
     - flags (array, optional): List of git-show-style flags (e.g., '-s' for short format, '--stat' for file statistics)

7. git_diff
   Description: Compare changes between git commits, branches, or the working tree
   Parameters:
     - reference1 (string, optional): First git reference for comparison
     - reference2 (string, optional): Second git reference for comparison
     - path (string, optional): Path within the repository to limit the diff
     - flags (array, optional): List of git-diff-style flags (e.g., '-p' for patch, '-s' for short format, '--stat' for file statistics)

IMPORTANT: This session is in read-only mode. Only the tools listed above are available.
You cannot modify, write, delete, or execute any files or commands.
```

## Return Values for Disabled Tools

When a tool that is not available in read-only mode is called, the tool executor should return:

```json
{
  "success": false,
  "error": "Tool 'tool_name' is not available in read-only mode",
  "tool_name": "bash"
}
```

## Implementation Notes

- The read-only mode should be checked when building the list of available tools
- The agent should be informed of read-only mode at startup
- The TUI should indicate when read-only mode is active (e.g., in the status bar)
- The read-only flag takes precedence over any other configuration that might enable write tools
- The `git_log` and `git_show` tools are read-only operations and are available in all modes including read-only mode
