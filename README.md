# Minimal Coding Agent Harness

A minimal coding agent harness written in Go with a basic TUI supporting tool calling via OpenAI-compatible API endpoints.

## Features

- **Interactive TUI**: Terminal-based user interface with history navigation
- **Tool Calling**: Support for bash, file read/write, and text manipulation tools
- **Streaming Inference**: Real-time token streaming for better UX
- **Context Management**: Automatic context compression when limits are approached
- **One-Shot Mode**: Non-interactive mode for CI/CD integration
- **Zero External Dependencies**: Built entirely with Go standard library

## Quick Start

### Prerequisites

- Go 1.22 or higher
- A running llama.cpp server or compatible OpenAI API endpoint

### Building

```bash
# Clone the repository
git clone https://github.com/your-org/coding-agent.git
cd coding-agent

# Build the binary
make build

# Or build manually with version info
make build-with-version
```

### Configuration

Set up your environment variables:

```bash
export CODING_AGENT_API_ENDPOINT="http://localhost:8080"
export CODING_AGENT_API_KEY="your-api-key"  # Optional
export CODING_AGENT_MODEL="llama3"
export CODING_AGENT_CONTEXT_SIZE=128000
export CODING_AGENT_MAX_ITERATIONS=1000
```

Or use a config file:

```ini
# config.txt
api_endpoint=http://localhost:8080
api_key=your-api-key
model=llama3
context_size=128000
max_iterations=1000
```

### Running Interactive Mode

```bash
# Start the interactive TUI
./coding-agent

# With custom config
./coding-agent --config config.txt

# Disable streaming
./coding-agent --no-stream
```

### Running One-Shot Mode

```bash
# Using command-line prompt
./coding-agent -p "Create a Go function that adds two numbers"

# Using stdin
echo "List files in /tmp" | ./coding-agent --stdin

# Using prompt file
./coding-agent --prompt-file task.txt

# Verbose output
./coding-agent -p "Your task" --verbose

# Quiet mode (only final output)
./coding-agent -p "Your task" --quiet
```

### Available Commands (Interactive Mode)

- `stats` - Display runtime statistics
- `clear` - Clear the output display
- `clear-history` - Clear input history

## Usage Examples

### Example 1: File Operations

```bash
./coding-agent -p "Create a Go hello world program in main.go"
```

### Example 2: Code Review

```bash
./coding-agent -p "Review the code in src/ for security issues" --output review.txt
```

### Example 3: Refactoring

```bash
./coding-agent -p "Refactor legacy code to use modern Go patterns" --max-iterations 2000
```

### Example 4: CI/CD Integration

```bash
#!/bin/bash
coding-agent --prompt "$(cat task.txt)" --output result.txt
if [ $? -eq 0 ]; then
    echo "Task completed successfully"
else
    echo "Task failed"
    exit 1
fi
```

## Development

### Project Structure

```
/workspace/
├── implementation/
│   ├── agent/         # Agent core logic
│   ├── config/        # Configuration handling
│   ├── inference/     # LLM API communication
│   ├── tools/         # Tool implementations
│   ├── tui/           # Terminal user interface
│   └── main.go        # Entry point
├── requirements/      # Requirement specifications
├── README.md          # This file
└── LICENSE            # MIT License
```

### Building for Development

```bash
# Build with version information
make build

# Run tests
cd implementation
go test ./...

# Format code
gofmt -w .

# Vet code
go vet ./...
```

### Adding New Tools

1. Implement the tool in `implementation/tools/tools.go`
2. Add the tool definition to `buildTools()` in `implementation/agent/agent.go`
3. Add the tool description to `buildSystemPrompt()` in `implementation/agent/agent.go`
4. Update this README with the new tool documentation

### Adding Requirements

1. Create a new requirement file in `requirements/` folder
2. Number it sequentially (e.g., `027-new-feature.md`)
3. Include acceptance criteria with checkboxes
4. Update the implementation accordingly

### Configuration Options

| Option | Flag | Env Variable | Default |
|--------|------|--------------|---------|
| Model | `--model` | `CODING_AGENT_MODEL` | `llama3` |
| Temperature | `--temperature` | `CODING_AGENT_TEMPERATURE` | `0.7` |
| Max Tokens | `--max-tokens` | `CODING_AGENT_MAX_TOKENS` | `4096` |
| Context Size | `--context-size` | `CODING_AGENT_CONTEXT_SIZE` | `128000` |
| Max Iterations | `--max-iterations` | `CODING_AGENT_MAX_ITERATIONS` | `1000` |
| Streaming | `--no-stream` | `CODING_AGENT_STREAMING` | enabled |
| API Endpoint | - | `CODING_AGENT_API_ENDPOINT` | `http://localhost:8080` |
| API Key | - | `CODING_AGENT_API_KEY` | (none) |
| Verbose | `--verbose` | `CODING_AGENT_VERBOSE` | disabled |
| Quiet | `--quiet` | `CODING_AGENT_QUIET` | disabled |

### Available Tools

The agent supports the following tools:

1. **bash** - Execute shell commands
2. **read_file** - Read file contents
3. **write_file** - Write content to files
4. **read_lines** - Read specific line ranges
5. **insert_lines** - Insert lines at position
6. **replace_lines** - Replace lines by range
7. **replace_text** - Find and replace text

### Testing

```bash
# Run all tests
go test -v ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test -v ./agent
```

### Troubleshooting

#### Connection Errors

```
Error: failed to make request: connection refused
```

- Verify the llama.cpp server is running
- Check the API endpoint configuration
- Ensure the port is correct (default: 8080)

#### Context Size Warnings

```
[Context: 100000 / 128000 (78.1%) ⚠]
```

- Context compression will trigger automatically
- Consider increasing `CODING_AGENT_CONTEXT_SIZE` for complex tasks

#### Max Iterations Exceeded

```
Error: maximum iterations (1000) exceeded
```

- Increase with `--max-iterations 2000`
- Or set `CODING_AGENT_MAX_ITERATIONS=2000`
- Consider breaking complex tasks into smaller steps

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Support

For issues and questions:
- Open an issue on GitHub
- Check the requirements documentation in `requirements/`
- Review the implementation code for examples
