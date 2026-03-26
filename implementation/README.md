# Minimal Coding Agent Harness

A minimal coding agent harness written in Go with a basic TUI supporting an input prompt.

## Features

- **Minimal TUI with input prompt**: Terminal user interface for user interaction
- **Runtime statistics tracking**: Tracks tokens, tool calls, and performance metrics
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

```bash
cd implementation
go build -o coding-agent .
```

## Usage

```bash
# Run with default settings
./coding-agent

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

All tool calls use a standardized format:

```
[tool:tool_name(param_name="param_value", ...)]
```

### Examples

```
[tool:bash(command="ls -la /home")]
[tool:read_file(path="/path/to/file.txt")]
[tool:write_file(path="/path/to/file.txt", content="Hello World")]
[tool:read_lines(path="/path/to/file.txt", start=1, end=10)]
[tool:insert_lines(path="/path/to/file.txt", line=5, lines="new line")]
[tool:replace_lines(path="/path/to/file.txt", start=1, end=5, lines="replacement")]
```

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

## Running Tests

```bash
cd implementation
go test ./... -v
```

## License

MIT License
