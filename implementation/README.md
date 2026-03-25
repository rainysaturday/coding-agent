# Coding Agent Harness

A minimal coding agent harness written in Go with a basic TUI and support for various tools.

## Features

- **Minimal TUI**: Simple terminal interface with input prompt
- **Runtime Statistics**: Tracks input/output tokens, tokens/second, tool calls, and failures
- **Inference Backend**: Connects to llama.cpp server via OpenAI API compatible REST endpoint
- **Configurable Context**: Adjustable context size (default: 128000 tokens)
- **Context Compression**: Automatic summarization when context exceeds limit
- **Streaming Inference**: Real-time token display with configurable timeout (default: 2 hours)
- **Supported Tools**:
  - `bash`: Execute shell commands
    - Format: `[tool:bash(command="ls -la")]`
  - `read_file`: Read entire file contents
    - Format: `[tool:read_file(path="/path/to/file.txt")]`
  - `write_file`: Write content to files
    - Format: `[tool:write_file(path="/path/to/file.txt", content="Hello")]`
  - `read_lines`: Read specific line ranges
    - Format: `[tool:read_lines(path="/path/to/file.txt", start=1, end=10)]`
  - `insert_lines`: Insert lines at specific positions
    - Format: `[tool:insert_lines(path="/path/to/file.txt", line=5, lines="new line")]`
  - `replace_lines`: Replace line ranges
    - Format: `[tool:replace_lines(path="/path/to/file.txt", start=1, end=5, lines="new content")]`

- **Tool Calling Format**: Standardized `[tool:tool_name(param="value")]` format
- **System Prompt**: Complete tool list and format always prefixed to context

## Installation

```bash
cd implementation
go build ./cmd/main.go
```

## Configuration

### Environment Variables

- `INFERENCE_URL`: URL to inference server (default: `http://localhost:8080/v1`)
- `API_KEY`: API key for inference server (default: `sk-no-key-required`)
- `CONTEXT_SIZE`: Context size in tokens (default: `128000`)
- `INITIAL_TOKEN_TIMEOUT`: Timeout in seconds for initial token (default: `7200`)
- `CONFIG_PATH`: Path to config file (default: `config.yaml`)

### Command Line Flags

```bash
./main -no-stream    # Disable streaming mode
./main -help         # Show help message
```

## Usage

```bash
./main
```

The agent will start with an interactive prompt. Type your message and press Enter to send.

### Commands

- `stats`: Show runtime statistics
- `clear`: Clear the conversation context
- `exit`/`quit`: Exit the agent

## Example Usage

```
> Write a Python function to calculate factorial
> [Agent responds with code]
> [Agent uses write_file tool to save the code]
> stats
=== Runtime Statistics ===
Total Input Tokens:  1250
Total Output Tokens: 890
Tokens/Second:       45.23
Tool Calls:          1
Failed Tool Calls:   0
Elapsed Time:        0h0m15s
========================
```

## Architecture

```
implementation/
├── cmd/
│   └── main.go          # Main entry point
├── pkg/
│   ├── config/          # Configuration management
│   ├── context/         # Context management with compression
│   ├── inference/       # Inference client (OpenAI API compatible)
│   ├── stats/           # Runtime statistics tracking
│   ├── tools/           # Tool implementations
│   └── tui/             # Terminal user interface
└── go.mod               # Go module file
```

## Requirements

- Go 1.21 or later
- A running llama.cpp server with OpenAI API compatibility

## License

MIT License
