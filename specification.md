# Minimal Coding Agent Harness

## Overview
A minimal coding agent harness written in Go with a basic TUI supporting an input prompt.

## Core Features
- Minimal TUI with input prompt
- Runtime statistics tracking
- Basic tool support (bash, read_file, write_file)

## Technical Requirements
- Language: Go (Golang)
- Minimal dependencies
- Cross-platform support

## Runtime Statistics
- Total input tokens
- Total output tokens
- Tokens per second
- Number of tool calls
- Number of failed tool calls

## Supported Tools

### Normal Mode
- `bash`: Execute bash commands/scripts
- `read_file`: Read file contents
- `write_file`: Write contents to files
- `read_lines`: Read specific line range from a file
- `insert_lines`: Insert lines at a specific line number
- `replace_text`: Find and replace text in a file

### Read-Only Mode
- `read_file`: Read file contents
- `read_lines`: Read specific line range from a file
- `list_files`: List files and directories
- `grep`: Search for patterns in files
- `git_log`: View commit history
- `git_show`: View commit details
- `git_diff`: Compare changes between commits
