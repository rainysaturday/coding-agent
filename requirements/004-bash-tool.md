# Requirement 004: Bash Tool

## Description
The harness must support a `bash` tool that allows execution of shell commands.

## Acceptance Criteria
- [ ] Tool named `bash` is available
- [ ] Accepts command string as input parameter
- [ ] Supports multi-line scripts using raw mode markers
- [ ] Executes command in shell environment
- [ ] Returns command output (stdout and stderr)
- [ ] Returns exit code of executed command
- [ ] Handles command execution errors gracefully
- [ ] Tool call failures are tracked in statistics

## Tool Usage

### Standard Mode (single-line commands)
```
[tool:bash(command="ls -la /home")]
[tool:bash(command="echo 'Hello World'")]
[tool:bash(command="grep -r 'pattern' .")]
```

### Raw Mode (multi-line scripts)
```
[tool:bash(command=<<<RAW>>>
#!/bin/bash
# Multi-line script
echo "Starting..."
for i in {1..10}; do
    echo "Iteration $i"
done
echo "Done!"
<<<END_RAW>>>)]
```

### Parameters
- `command`: Shell command or script to execute (required)
  - Use standard mode for simple single-line commands
  - Use raw mode (`<<<RAW>>>`...`<<<END_RAW>>>`) for multi-line scripts, loops, and complex commands
  - Raw mode preserves exact formatting and special characters without escaping

### Examples

**Simple command:**
```
[tool:bash(command="pwd")]
```

**Command with quotes:**
```
[tool:bash(command="echo \"Hello World\"")]
```

**Multi-line script (raw mode):**
```
[tool:bash(command=<<<RAW>>>
#!/bin/bash
set -e
cd /tmp
cat > test.txt << EOF
line 1
line 2
EOF
cat test.txt
<<<END_RAW>>>)]
```
