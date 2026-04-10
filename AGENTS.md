# Agent Onboarding Guide

This guide helps agents quickly understand the project structure and find relevant information.

## Project Structure

```
/workspace/
├── requirements/      # All requirement specifications
│   ├── 001-go-runtime.md
│   ├── 002-tui-input-prompt.md
│   ├── ...            # 29 requirements total
│   └── 029-system-prompt-environment-info.md
├── implementation/    # Go source code
│   ├── agent/         # Core agent logic (agent.go)
│   ├── config/        # Configuration parsing (config.go)
│   ├── inference/     # LLM API communication (inference.go)
│   ├── tools/         # Tool implementations (tools.go)
│   ├── tui/           # Terminal UI (tui.go)
│   ├── debug/         # Debug logging (debug.go)
│   └── main.go        # Entry point
├── README.md          # User-facing documentation
├── AGENTS.md          # This file - agent onboarding
├── Makefile           # Build system
└── go.mod             # Go module definition
```

## Finding Requirements

All requirements are in the `requirements/` folder, numbered sequentially:

- **001-010**: Core features (Go runtime, TUI, statistics, tools, inference)
- **011-020**: Tool implementations (read_lines, insert_lines, replace_lines, etc.)
- **021-029**: Advanced features (versioning, one-shot mode, debug, environment info)

To find requirements for a feature:
1. List requirements: `ls requirements/`
2. Read specific requirement: `cat requirements/XXX-feature-name.md`
3. Search for keywords: `grep -r "keyword" requirements/`

## Key Implementation Files

### agent/agent.go
- Main agent logic and state management
- `buildSystemPrompt()` - Construct system prompt with tools and environment info
- `buildTools()` - Tool definitions for LLM
- `Run()` / `RunStream()` - Agent execution methods
- `Stats` struct - Runtime statistics tracking

### config/config.go
- Configuration parsing (env vars, CLI args, config file)
- `Config` struct - All configuration options
- Default values and validation

### inference/inference.go
- LLM API communication
- `InferenceClient` struct - API client
- `StreamingCallback` - Real-time token streaming

### tools/tools.go
- Tool executor implementation
- `ToolExecutor` struct - Execute tool calls
- Individual tool implementations (bash, read_file, write_file, etc.)

### tui/tui.go
- Terminal user interface
- Input handling, history navigation, output display

## System Prompt Structure

The system prompt (built in `agent/agent.go`) contains:

1. **Environment Information** (runtime):
   - Current working directory
   - Agent executable path
   - Operating system
   - Architecture

2. **Tool Calling Format**: How to use tools

3. **Available Tools**: Description of 7 tools (bash, read_file, write_file, read_lines, insert_lines, replace_lines, replace_text)

4. **Best Practices**: Verification requirements, tool calling guidelines

## Common Tasks

### Add a New Tool
1. Implement in `implementation/tools/tools.go`
2. Add to `buildTools()` in `implementation/agent/agent.go`
3. Add to system prompt in `buildSystemPrompt()`
4. Create requirement in `requirements/`

### Add a New Requirement
1. Create file: `requirements/XXX-description.md`
2. Include acceptance criteria with checkboxes
3. Reference in README if user-facing

### Modify Existing Feature
1. Check existing requirements in `requirements/`
2. Update implementation in `implementation/`
3. Update README if user-facing behavior changes

## Testing

```bash
# Run all tests
cd implementation
go test ./...

# Run with coverage
go test -cover ./...

# Build binary
make build
```

## Debugging

Enable debug logging:
```bash
./coding-agent --debug
# Logs saved to debug.log
```

Check debug log:
```bash
cat debug.log
```

## Quick Reference

| Need to find... | Look in... |
|-----------------|------------|
| Feature requirements | `requirements/XXX-*.md` |
| Tool implementation | `implementation/tools/tools.go` |
| Agent logic | `implementation/agent/agent.go` |
| Configuration | `implementation/config/config.go` |
| API communication | `implementation/inference/inference.go` |
| TUI code | `implementation/tui/tui.go` |
| User documentation | `README.md` |
| Build commands | `Makefile` |

## Environment Variables

Common environment variables:
- `CODING_AGENT_API_ENDPOINT` - LLM API URL
- `CODING_AGENT_API_KEY` - API key (optional)
- `CODING_AGENT_MODEL` - Model name (default: llama3)
- `CODING_AGENT_CONTEXT_SIZE` - Context window (default: 128000)
- `CODING_AGENT_MAX_ITERATIONS` - Max iterations (default: 1000)

## Sub-Agent Spawning

The agent can spawn sub-agents for parallel tasks:
```bash
coding-agent -p "Your task here"
```

The executable path and current working directory are included in the system prompt for this purpose.
