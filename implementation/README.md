# Minimal Coding Agent Harness

A minimal coding agent harness written in Go with a basic TUI supporting an input prompt.

## Overview

This project implements a coding agent that can:

- Execute bash commands
- Read and write files
- Read and modify specific line ranges in files
- Connect to an LLM inference backend (llama.cpp server via OpenAI API)
- Provide a terminal user interface (TUI) for interactive use
- Support one-shot non-interactive mode

## Features

### Core Features

- **Minimal TUI** with input prompt for user interaction
- **Runtime statistics** tracking (tokens, tool calls, iterations)
- **Tool support** for common coding tasks

### Supported Tools

1. **bash** - Execute shell commands and scripts
2. **read_file** - Read entire file contents
3. **write_file** - Write content to files
4. **read_lines** - Read specific line ranges from files
5. **insert_lines** - Insert lines at specific positions
6. **replace_lines** - Replace lines by range or search-and-replace

### Technical Features

- **Zero external dependencies** - Uses only Go standard library
- **Cross-platform** - Works on Linux, macOS, and Windows
- **Streaming responses** - Real-time token display
- **Context management** - Configurable context size with compression
- **History navigation** - Up/down arrow keys for previous prompts
- **Cancellation** - Escape key to cancel ongoing operations
- **Version tracking** - Git commit hash and dirty status on startup

## Building

### Prerequisites

- Go 1.22 or later

### Build Commands

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Run tests
make test

# Run tests with coverage
make test-cover

# Run go vet
make vet
```

### Build with Version Information

Version information is automatically embedded at build time:

```bash
make build
# Git Hash: a1b2c3d [clean]
# Build Time: 2024-01-15T10:30:00Z
```

## Usage

### Interactive Mode

```bash
# Start the agent
./coding-agent

# Commands available in TUI:
# - Type your request and press Enter
# - Type 'stats' to view statistics
# - Type 'clear' to clear output
# - Type 'clear-history' to clear input history
# - Press Escape to cancel ongoing operations
# - Use Up/Down arrow keys for history navigation
# - Press Ctrl+C to exit
```

### One-Shot Mode

```bash
# With prompt flag
./coding-agent --prompt "Create a Go function that adds two numbers"

# With prompt from stdin
echo "Refactor utils.go" | ./coding-agent --stdin

# With prompt from file
./coding-agent --prompt-file task.txt

# With output file
./coding-agent --prompt "Create main.go" --output result.txt

# Verbose mode
./coding-agent --prompt "Create a file" --verbose

# Quiet mode
./coding-agent --prompt "Create a file" --quiet
```

### Command-Line Options

| Option           | Description                   | Default |
| ---------------- | ----------------------------- | ------- |
| `-p, --prompt`   | Prompt for one-shot mode      | -       |
| `--stdin`        | Read prompt from stdin        | false   |
| `--prompt-file`  | Read prompt from file         | -       |
| `--model`        | Model to use                  | llama3  |
| `--temperature`  | Inference temperature         | 0.7     |
| `--max-tokens`   | Maximum tokens to generate    | 4096    |
| `--context-size` | Context window size           | 128000  |
| `--no-stream`    | Disable streaming             | false   |
| `--verbose`      | Enable verbose output         | false   |
| `--quiet`        | Suppress non-essential output | false   |
| `--output`       | Write results to file         | -       |
| `-h, --help`     | Show help message             | -       |
| `-v, --version`  | Show version information      | -       |

### Environment Variables

| Variable                             | Description                           |
| ------------------------------------ | ------------------------------------- |
| `CODING_AGENT_MODEL`                 | Model to use                          |
| `CODING_AGENT_TEMPERATURE`           | Inference temperature                 |
| `CODING_AGENT_MAX_TOKENS`            | Maximum tokens                        |
| `CODING_AGENT_CONTEXT_SIZE`          | Context window size                   |
| `CODING_AGENT_API_ENDPOINT`          | API endpoint URL                      |
| `CODING_AGENT_API_KEY`               | API key for authentication            |
| `CODING_AGENT_INITIAL_TOKEN_TIMEOUT` | Initial token timeout (seconds)       |
| `CODING_AGENT_STREAMING`             | Enable/disable streaming (true/false) |
| `CODING_AGENT_MAX_HISTORY`           | Maximum input history entries         |

## Tool Calling Format

All tool calls use a standardized JSON format:

```
[TOOL:{"name":"tool_name","parameters":{...}}]
```

### Examples

**Bash command:**

```
[TOOL:{"name":"bash","parameters":{"command":"ls -la"}}]
```

**Read file:**

```
[TOOL:{"name":"read_file","parameters":{"path":"/path/to/file.txt"}}]
```

**Write file with multi-line content:**

```
[TOOL:{"name":"write_file","parameters":{"path":"script.sh","content":"#!/bin/bash\necho hello"}}]
```

**Read lines:**

```
[TOOL:{"name":"read_lines","parameters":{"path":"file.txt","start":1,"end":10}}]
```

**Insert lines:**

```
[TOOL:{"name":"insert_lines","parameters":{"path":"file.txt","line":5,"lines":"new line"}}]
```

**Replace lines (by range):**

```
[TOOL:{"name":"replace_lines","parameters":{"path":"file.txt","start":1,"end":5,"lines":"replacement"}}]
```

**Replace lines (search-and-replace):**

```
[TOOL:{"name":"replace_lines","parameters":{"path":"main.go","search":"oldName","replace":"newName"}}]
```

## Runtime Statistics

The agent tracks and displays:

- Input tokens processed
- Output tokens generated
- Tool calls (total and failed)
- Iterations
- Uptime
- Current context size

View statistics with the `stats` command in TUI mode.

## Configuration

### Context Size

Default: 128000 tokens

Can be configured via:

- Environment variable: `CODING_AGENT_CONTEXT_SIZE`
- Command-line flag: `--context-size`
- Config file (future)

### Initial Token Timeout

Default: 7200 seconds (2 hours)

Minimum: 10 seconds

Can be configured via:

- Environment variable: `CODING_AGENT_INITIAL_TOKEN_TIMEOUT`
- Command-line flag: `--initial-token-timeout` (future)

## Architecture

```
implementation/
├── main.go           # Entry point, CLI handling
├── go.mod            # Go module definition
├── Makefile          # Build automation
├── README.md         # This file
├── agent/            # Agent logic
│   ├── agent.go      # Main agent implementation
│   └── agent_test.go # Agent tests
├── config/           # Configuration handling
│   ├── config.go     # Config parsing and validation
│   └── config_test.go # Config tests
├── inference/        # LLM backend communication
│   ├── inference.go  # Inference client
│   └── inference_test.go # Inference tests
├── tools/            # Tool implementations
│   ├── tools.go      # Tool executor and definitions
│   └── tools_test.go # Tool tests
└── tui/              # Terminal user interface
    ├── tui.go        # TUI implementation
    └── tui_test.go   # TUI tests
```

## Requirements Coverage

This implementation covers all 25 requirements:

1. ✅ **Go Runtime** - Go modules, cross-platform binary
2. ✅ **TUI Input Prompt** - Minimal TUI with input
3. ✅ **Runtime Statistics** - Token and tool tracking
4. ✅ **Bash Tool** - Command execution
5. ✅ **Read File Tool** - File reading
6. ✅ **Write File Tool** - File writing
7. ✅ **Inference Backend** - OpenAI API compatible
8. ✅ **Context Size** - Configurable context window
9. ✅ **Context Compression** - Auto-compression support
10. ✅ **Streaming Inference** - Real-time token display
11. ✅ **Read Lines Tool** - Line range reading
12. ✅ **Insert Lines Tool** - Line insertion
13. ✅ **Replace Lines Tool** - Line replacement (range and search)
14. ✅ **Tool Calling Format** - JSON-based format
15. ✅ **Tool Prefix Prompt** - System prompt with tools
16. ✅ **Tool Result Context** - Results added to context
17. ✅ **TUI Tool Feedback** - Tool call display
18. ✅ **LLM Error Feedback** - Error messages to LLM
19. ✅ **TUI History Navigation** - Up/down arrow support
20. ✅ **TUI Escape Cancellation** - Escape key cancellation
21. ✅ **TUI Context Size Display** - Context size indicator
22. ✅ **No User Input Echo** - No redundant input display
23. ✅ **Version Information** - Git hash display
24. ✅ **Zero External Dependencies** - Standard library only
25. ✅ **One-Shot Mode** - Non-interactive CLI mode

## Testing

Run all tests:

```bash
make test
```

Run with coverage:

```bash
make test-cover
```

Run with race detector:

```bash
make test-race
```

## License

See LICENSE
