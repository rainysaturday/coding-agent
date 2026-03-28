# Requirement 004: Bash Tool

## Description
The harness must support a `bash` tool that allows execution of shell commands.

## Acceptance Criteria
- [ ] Tool named `bash` is available
- [ ] Accepts command string as input parameter
- [ ] Executes command in shell environment
- [ ] Returns command output (stdout and stderr)
- [ ] Returns exit code of executed command
- [ ] Handles command execution errors gracefully
- [ ] Tool call failures are tracked in statistics

## Tool Usage

### Single-line Commands
```
[TOOL:{"name":"bash","parameters":{"command":"ls -la /home"}}]
[TOOL:{"name":"bash","parameters":{"command":"echo \"Hello World\""}}]
[TOOL:{"name":"bash","parameters":{"command":"grep -r \"pattern\" ."}}]
```

### Multi-line Scripts
```
[TOOL:{"name":"bash","parameters":{"command":"#!/bin/bash\n# Multi-line script\necho \"Starting...\"\nfor i in {1..10}; do\n    echo \"Iteration $i\"\ndone\necho \"Done!\""}}]
```

### Parameters
- `command`: Shell command or script to execute (required, string)
  - Single-line commands use regular JSON strings
  - Multi-line scripts use `\n` escape sequences
  - All special characters must be JSON-escaped

### Examples

**Simple command:**
```
[TOOL:{"name":"bash","parameters":{"command":"pwd"}}]
```

**Command with quotes:**
```
[TOOL:{"name":"bash","parameters":{"command":"echo \"Hello World\""}}]
```

**Multi-line script:**
```
[TOOL:{"name":"bash","parameters":{"command":"#!/bin/bash\nset -e\ncd /tmp\ncat > test.txt << EOF\nline 1\nline 2\nEOF\ncat test.txt"}}]
```

**Complex command with special characters:**
```
[TOOL:{"name":"bash","parameters":{"command":"echo \"Price: $100 \\\"special\\\" items\""}}]
```

## Return Values

On success:
- `output`: Combined stdout and stderr from command execution
- `success`: `true`

On failure:
- `error`: Description of the error
- `success`: `false`
