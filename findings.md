# Codebase Analysis Findings

## Project Overview

This is a **Minimal Coding Agent Harness** written in Go (module `github.com/coding-agent/harness`, requires Go 1.25+). It provides a terminal-based coding assistant with a TUI, LLM inference support (including GitHub Copilot and GitHub Models), and a set of file/tool execution capabilities.

---

## 1. Architecture

### Package Layout
```
/workspace/
  implementation/
    main.go              - Entry point, CLI parsing, interactive & one-shot modes
    agent/               - Core agent loop (agent.go, agent_context.go, agent_errors.go, agent_format.go, agent_prompt.go, agent_tools.go)
    config/              - Configuration (config.go)
    inference/           - LLM API client (inference.go)
    tools/               - Tool execution system (bash, file ops, git, grep, subagent, todo, view_image, etc.)
    tui/                 - Terminal user interface (tui.go)
    colors/              - ANSI color codes and theme support (colors.go, theme.go)
    debug/               - Debug logging (debug.go)
  requirements/          - Requirements documents (one per feature)
  specifications/        - API specs (inference-api.md, mcp-server-integration.md)
```

### Dependencies
- `golang.org/x/term` (required)
- `golang.org/x/sys` (indirect)
- **No other external dependencies** — adheres to requirement 024 (zero external dependencies beyond Go stdlib + x/term)

### Build System
- Makefile with targets: `build`, `build-all`, `build-darwin-arm64`, `build-linux-amd64`, `build-windows-amd64`
- Version injection via linker flags: `gitHash`, `gitDirty`, `buildTime`
- Binary name: `coding-agent`

---

## 2. Configuration (config/)

### Config Structure
- `Config` struct with fields for all modes, inference, API, output, theme, persona, timeouts, debug, etc.
- Default values: model=`llama3`, maxTokens=64000, contextSize=128000, streaming=true, maxIterations=1000
- Timeouts default to 24 hours

### Config Sources (priority order: CLI > env > config file > defaults)
1. CLI flags parsed in `ParseArgs()`
2. Environment variables via `loadEnv()` (CODING_AGENT_* prefix)
3. Config file (simple KEY=VALUE format, # comments)
4. Defaults

### Notable CLI Flags
- `-p`/`--prompt`, `--stdin`, `--goal`, `--prompt-file`
- `--model`, `--temperature`, `--max-tokens`, `--context-size`, `--max-iterations`
- `--api-endpoint`, `--api-key`
- `--read-only`, `--experimental`, `--debug`, `--persona`, `--theme`
- `--summary-only`, `--no-stream`, `--load` (context loading), `--no-dump-on-exit`

---

## 3. Agent (agent/)

### Core Loop (agent.go)
- `Agent` struct manages context (messages), inference client, tool executor, stats, goals
- `Run()` method: main loop with iteration limit, tool call execution, response handling
- `RunStream()`: wraps `Run()` with streaming callback
- Goal mode: injects goal check prompts, tracks goal achievement tokens/timing
- Context compression: when context exceeds 80% of max, summarizes conversation

### Context Management (agent_context.go)
- `getActualContextSizeUnlocked()`: Uses `lastTotalTokens` from API response + estimates for new tool results
- `shouldCompress()`: triggers at 80% context usage
- `compressContext()`: summarizes middle messages, preserves first user message + last 3
- `LoadContext()`/`DumpContext()`: JSON serialization of conversation state
- `recordIteration()`: snapshots full context for dump/history

### System Prompt (agent_prompt.go)
- Two variants: normal mode and read-only mode
- Includes environment info (cwd, executable path, OS, architecture)
- Lists all available tools with parameters, examples, best practices
- Includes verification checklist for the LLM
- Persona and summary-only mode instructions appended when configured

### Tool Definitions (agent_tools.go)
- Tool definitions in OpenAI `tools` format for LLM function calling
- Normal mode: bash, read_file, write_file, read_lines, insert_lines, replace_text, move_text, todo, view_image
- Read-only mode: read_file, read_lines, list_files, grep, git_log, git_show, git_diff, view_image
- Experimental mode adds `subagent` tool

### Error Handling (agent_errors.go)
- Exit codes: 0=success, 1=error, 2=usage, 3=auth, 4=context limit
- `AuthError` and `ContextLimitError` types
- Error classification by message content

### Formatting (agent_format.go)
- Tool call display formatting with ANSI colors
- Streaming status messages for each tool
- Result truncation for display (e.g., bash output shows last 5 lines)

---

## 4. Inference (inference/)

### Client Design
- OpenAI-compatible chat completions API
- Supports streaming (SSE) and non-streaming modes
- Retry logic (3 attempts) with backoff
- Special handling for GitHub Copilot and GitHub Models endpoints

### Endpoint Detection
- Copilot: `githubcopilot.com` → `/chat/completions`
- GitHub Models: `models.github.ai` → `/inference/chat/completions`
- Default: `*/v1/chat/completions`

### Message Handling
- `Message` struct with Role, Content, Reasoning, ToolCalls, ToolCallId
- Multi-modal support (images via ContentParts)
- Custom `MarshalJSON()` for conditional content field (string vs array)
- Tool call normalization: ensures `type` field is always populated

### Streaming
- SSE parsing with `data:` prefix and `[DONE]` terminator
- Multi-line JSON blob accumulation
- Tool call delta accumulation by index (merging partial deltas)
- Token usage from both OpenAI format and llama.cpp timings format
- Reasoning content split into separate stream

### Response Parsing
- Token usage from API response: `prompt_tokens`, `completion_tokens`, `total_tokens`
- Fallback to llama.cpp `timings` format: `cache_n`, `prompt_n`, `predicted_n`
- Tool call JSON argument parsing with error handling

### Token Estimation
- Heuristic-based: word count × 1.3, with code content multiplier (×1.2)
- Used for context size estimation when no API response available

---

## 5. Tools (tools/)

### Architecture
- `ToolExecutor` struct with dispatch via `Execute()` switch statement
- `ToolCall` and `ToolResult` types
- `Stats` tracking (total/failed calls)
- Read-only mode enforcement via `isReadOnlyTool()` map
- Todo store (in-memory task list)

### Implemented Tools (13 total)
| Tool | File | Read-Only? | Description |
|------|------|------------|-------------|
| `bash` | bash.go | No | Execute commands with timeout/cancellation |
| `read_file` | read_file.go | Yes | Read file contents |
| `write_file` | write_file.go | No | Write content to files |
| `read_lines` | read_lines.go | Yes | Read specific line range |
| `insert_lines` | insert_lines.go | No | Insert lines at position |
| `replace_text` | replace_text.go | No | Find and replace text |
| `move_text` | move_text.go | No | Move text between files/locations |
| `list_files` | list_files.go | Yes | Directory listing (ls-like) |
| `grep` | grep.go | Yes | Pattern search in files |
| `git_log` | git_log.go | Yes | View commit history |
| `git_show` | git_show.go | Yes | View commit details |
| `git_diff` | git_diff.go | Yes | Compare changes |
| `subagent` | subagent.go | Conditional | Spawn subagent process |
| `view_image` | view_image.go | Yes | Load image for vision analysis |
| `todo` | todo.go | Partial (list/remove only) | Task management |

---

## 6. TUI (tui/)

### Features
- Input prompt with raw mode character-by-character reading
- History navigation (arrow keys, Ctrl+P/N)
- Context size display with color-coded indicator
- Streaming output with content type coloring (reasoning dimmed, goal magenta)
- Tool call parameter updates in-place (ANSI cursor positioning)
- Stats display
- Command support (/stats, /clear, /clear-history, /read-only, /compress, /dump, /goal, /goal-off)

### Signal Handling
- Ctrl+C during input: cancels operation
- Ctrl+C during execution: forwarded via cancelSignal channel
- Context cancellation propagation

---

## 7. Colors & Theme (colors/)

- ANSI color constants (reset, red, green, yellow, blue, magenta, cyan, dim)
- Theme support: dark (default), light, solarized, gruvbox, darkula (defined in theme.go)
- `GetColor()` function with theme-aware lookup + fallback to defaults
- `SetTheme()`/`ApplyTheme()` at startup

---

## 8. Debug Logging (debug/)

- `--debug` flag enables logging of all LLM conversation to file
- `SessionSummary` tracks total messages, tokens, tool calls
- Sensitive data redaction (API keys, tokens, secrets) via regex patterns
- Timestamps on all log entries
- Structured log format

---

## 9. Issues Found

### CRITICAL ISSUES

#### I4. `lastTotalTokens` Reset During Compression May Underreport Context Size
- **File**: `agent_context.go` (`compressContext()`)
- **Issue**: After compression, `lastTotalTokens` is set to `EstimateContextSize()` which is an estimate, not an authoritative API count.
- **Status**: Still open — the estimate is the best available value until the next API response arrives.



---

### HIGH ISSUES

#### H5. Subagent Tool Not Available in Read-Only Mode
- **File**: `agent_tools.go`
- **Issue**: `buildReadOnlyTools()` never includes `subagent` even with `--experimental`.
- **Status**: Still open — subagent is inherently a write/execute operation, so this is by design.

---






### MEDIUM ISSUES

#### M7. Config File Loading: Unknown Keys Print Warning but Continue
- **File**: `config/config.go`
- **Issue**: Unknown config file keys print a warning to stderr but don't return an error.
- **Status**: Still open — by design, allows forward-compatibility with future config keys.

---






### LOW ISSUES

#### L6. `config.go` — `loadConfigFile` Reads File Twice (Once in ParseArgs, Once in LoadConfigFile)
- **File**: `config/config.go`
- **Issue**: `ParseArgs()` reads the config file path from args, then `loadConfigFile()` reads and parses it. But `ParseArgs()` already iterates through all args including `--config`, so there's no double-read. This is fine.
- **No actual issue here** — misidentified initially.



---


## 10. Requirements Coverage

### Fully Implemented (45/45 requirements)
All 45 requirements files in `/workspace/requirements/` have been implemented:

1. **001** - Go runtime ✓
2. **002** - TUI input prompt ✓
3. **003** - Runtime statistics ✓
4. **004** - Bash tool ✓
5. **005** - Read file tool ✓
6. **006** - Write file tool ✓
7. **007** - Inference backend ✓
8. **008** - Context size ✓
9. **009** - Context compression ✓
10. **010** - Streaming inference ✓
11. **011** - Read lines tool ✓
12. **012** - Insert lines tool ✓
13. **013** - Replace text tool ✓
14. **014** - Tool calling format ✓
15. **015** - Tool prefix prompt ✓
16. **016** - Tool result context ✓
17. **017** - TUI tool feedback ✓
18. **018** - LLM error feedback ✓
19. **019** - TUI history navigation ✓
20. **020** - TUI Ctrl+C cancellation ✓
21. **021** - TUI context size display ✓
22. **022** - No user input echo ✓
23. **023** - Versioning ✓
24. **024** - Zero external dependencies ✓
25. **025** - Non-interactive one-shot mode ✓
26. **026** - Configurable max iterations ✓
27. **027** - TUI reasoning token coloring ✓
28. **028** - Debug flag ✓
29. **029** - System prompt environment info ✓
30. **030** - Patch tool (DEPRECATED - marked as removed) ✓
31. **031** - GitHub Copilot backend ✓
32. **032** - List files tool ✓
33. **033** - Read-only mode ✓
34. **034** - Grep tool ✓
35. **035** - Git log tool ✓
36. **036** - Git show tool ✓
37. **037** - Git diff tool ✓
38. **038** - Goal mode ✓
39. **039** - Persona configuration ✓
40. **040** - Subagent tool ✓
41. **041** - View image tool ✓
42. **042** - Theme support ✓
43. **043** - Todo tool ✓
44. **044** - Context dump/load ✓
45. **045** - Move text tool ✓

### Additional Observations
- **MCP Server Integration** (`specifications/mcp-server-integration.md`) exists as a specification but is NOT implemented in code. This is a future feature.
- **`--initial-token-timeout`** flag is parsed but not documented in `--help` output.

---

## 11. Code Quality Summary

### Strengths
- Clean package separation with clear responsibilities
- Good use of Go idioms (context, mutexes, channels)
- Comprehensive streaming support with content type separation
- Well-structured tool execution system
- Thorough error handling with typed errors
- Debug logging with sensitive data redaction
- Cross-platform build support (macOS, Linux, Windows)
- Extensive test coverage (multiple test files per package)

### Weaknesses
- **Streaming output duplication**: Tool call notification duplication is already handled by existing `notifiedToolCalls` deduplication map.
- **Context compression accuracy**: Token counting after compression uses estimates (inherent design limitation).
