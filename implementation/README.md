# Minimal Coding Agent Harness

A minimal coding agent harness written in Go with a basic TUI supporting an input prompt.

## Features

- **Minimal TUI with input prompt**: Terminal user interface for user interaction (no redundant input echo)
- **Runtime statistics tracking**: Tracks tokens, tool calls, and performance metrics
- **Tool feedback display**: Brief, relevant feedback for tool calls in TUI
  - Tool name and key parameters shown before execution
  - Success messages with truncated output for long results
  - Error messages displayed prominently with color coding
- **LLM error reporting**: Failed tool calls reported back to LLM with actionable error messages
- **Basic tool support**: 
  - `bash`: Execute shell commands
  - `read_file`: Read file contents
  - `write_file`: Write contents to files
  - `read_lines`: Read specific line ranges from files
  - `insert_lines`: Insert lines at specified positions
  - `replace_lines`: Replace line ranges with new content

## Technical Requirements

- **Language**: Go (Golang)
- **Dependencies**: Minimal, no external dependencies
- **Cross-platform**: Supports Linux, macOS, and Windows

## Runtime Statistics

- Total input tokens
- Total output tokens
- Tokens per second
- Number of tool calls
- Number of failed tool calls

## Installation

### Using Make (recommended)

```bash
cd implementation
make build
```

This will embed version information (git hash, dirty status, build time) into the binary.

### Manual Build

```bash
cd implementation
go build -o coding-agent .
```

### Version Information

The binary embeds version information at build time:

| Variable | Description | Default |
|----------|-------------|---------|
| `gitHash` | Git commit hash (short) | `unknown` |
| `gitDirty` | Repository clean/dirty status | `unknown` |
| `buildTime` | UTC build timestamp | `` |

View version info before building:
```bash
make version
```

Build with version info:
```bash
make build
```

## Usage

```bash
# Run with default settings (shows version on startup)
./coding-agent
```

On startup, the agent displays version information:

```
============================================================
  Minimal Coding Agent Harness
============================================================
  Version: 3eda58c [dirty] ⚠
  Built: 2026-03-27T12:14:29Z

Type your request below. Use Ctrl+C to exit.
Type 'stats' to view statistics, 'clear' to clear output.
```

# Run with custom configuration
./coding-agent -config /path/to/config.json

# Run with custom endpoint
./coding-agent -endpoint http://localhost:8080/v1

# Run with custom context size
./coding-agent -context-size 65536

# Run with streaming disabled
./coding-agent -streaming 0
```

### Command-Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-config` | Path to configuration file | `~/.coding-agent-config.json` |
| `-endpoint` | Inference endpoint URL | `http://localhost:8080/v1` |
| `-context-size` | Context size in tokens | `128000` |
| `-timeout` | Initial token timeout (seconds) | `7200` |
| `-streaming` | Enable/disable streaming (-1=default, 0=false, 1=true) | `-1` |
| `-max-iterations` | Maximum tool call iterations | `50` |

### Interactive Commands

- `stats` - Display runtime statistics
- `clear` - Clear the output buffer
- `quit` / `exit` - Exit the application

## Configuration

Configuration can be set via:

1. **Configuration file** (JSON format)
2. **Environment variables**
3. **Command-line flags** (highest priority)

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CODING_AGENT_ENDPOINT` | Inference endpoint URL | `http://localhost:8080/v1` |
| `CODING_AGENT_API_KEY` | API key for authentication | `not-needed` |
| `CODING_AGENT_MODEL` | Model name | `llama-cpp` |
| `CODING_AGENT_CONTEXT_SIZE` | Context size in tokens | `128000` |
| `CODING_AGENT_INITIAL_TOKEN_TIMEOUT` | Initial token timeout (seconds) | `7200` |
| `CODING_AGENT_STREAMING` | Enable streaming | `true` |
| `CODING_AGENT_MAX_ITERATIONS` | Maximum iterations | `50` |

## Tool Calling Format

All tool calls use a standardized format with two modes:

### Standard Mode (for short values)
```
[tool:tool_name(param_name="param_value", ...)]
```

### Raw Mode (for multi-line content without escaping)
```
[tool:tool_name(path="file.txt", content=<<<RAW>>>
line 1
line 2
line 3
<<<END_RAW>>>)]
```

### Examples

**Standard Mode:**
```
[tool:bash(command="ls -la /home")]
[tool:read_file(path="/path/to/file.txt")]
[tool:write_file(path="/path/to/file.txt", content="Hello World")]
```

**Raw Mode (multi-line content):**
```
[tool:write_file(path="/path/to/script.sh", content=<<<RAW>>>
#!/bin/bash
echo "Hello World"
for i in {1..10}; do
    echo "Count: $i"
done
<<<END_RAW>>>)]

[tool:insert_lines(path="/path/to/file.txt", line=5, lines=<<<RAW>>>
new line 1
new line 2
new line 3
<<<END_RAW>>>)]
```

### When to Use Each Mode

| Mode | Use When |
|------|----------|
| Standard | Short values, single-line content, paths, simple commands |
| Raw | Multi-line content, code, scripts, documents with special characters |

## Project Structure

```
implementation/
├── main.go              # Entry point
├── config/              # Configuration management
│   ├── config.go
│   └── config_test.go
├── context/             # Conversation context management
│   ├── context.go
│   └── context_test.go
├── inference/           # Inference backend client
│   ├── inference.go
│   └── inference_test.go
├── stats/               # Runtime statistics
│   ├── stats.go
│   └── stats_test.go
├── tools/               # Tool implementations
│   ├── tools.go
│   ├── tools_test.go
│   ├── bash.go
│   ├── bash_test.go
│   ├── read_file.go
│   ├── read_file_test.go
│   ├── write_file.go
│   ├── write_file_test.go
│   ├── read_lines.go
│   ├── read_lines_test.go
│   ├── insert_lines.go
│   ├── insert_lines_test.go
│   ├── replace_lines.go
│   └── replace_lines_test.go
├── tui/                 # Terminal user interface
│   ├── tui.go
│   └── tui_test.go
└── go.mod
```

## Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Build the binary with version info |
| `make test` | Run all tests |
| `make clean` | Remove binary and clean build artifacts |
| `make version` | Display version information |
| `make run` | Build and run the agent |
| `make all` | Build and test (default) |

## Running Tests

```bash
cd implementation
go test ./... -v
```

## License

MIT License
