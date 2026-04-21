# Roadmap - Minimal Coding Agent Harness

## Implemented Features

### Core Infrastructure
- **#001: Go Runtime** - Project uses Go modules, compiles without errors, cross-platform support
- **#007: Inference Backend** - OpenAI-compatible REST endpoint, API key auth, llama.cpp server support
- **#024: Zero External Dependencies** - No external Go modules, stdlib only
- **#023: Versioning** - Git commit hash, dirty status, build time embedding

### File System Tools
- **#033: File Search Tool** - Glob pattern file discovery with recursive ** support, metadata output

### Terminal User Interface
- **#002: TUI Input Prompt** - Terminal UI with input area, command processing
- **#019: TUI History Navigation** - Arrow key history navigation
- **#020: TUI Ctrl+C Cancellation** - Graceful cancellation
- **#021: TUI Context Size Display** - Real-time context size display
- **#022: No User Input Echo** - Clean input handling
- **#027: TUI Reasoning Token Coloring** - Dimmed reasoning content display

### Agent Logic
- **#003: Runtime Statistics** - Token counts, tool call stats, throughput
- **#008: Context Size** - Configurable via env, CLI, config file (default 128k)
- **#009: Context Compression** - Automatic summarization when limit approached
- **#025: One-Shot Mode** - Non-interactive mode for CI/CD
- **#026: Configurable Max Iterations** - Loop protection
- **#028: Debug Flag** - Full conversation logging to file
- **#029: System Prompt Environment Info** - Working directory, executable path, OS/arch

### Tools (8 Total)
- **#004: Bash Tool** - Execute shell commands
- **#005: Read File Tool** - Read file contents
- **#006: Write File Tool** - Write/overwrite files
- **#011: Read Lines Tool** - Read specific line ranges
- **#012: Insert Lines Tool** - Insert lines at position
- **#013: Replace Text Tool** - Find and replace text
- **#030: Patch Tool** - Apply unified diff patches (via system `patch` command)
- **#032: Replace Lines Tool** - Replace lines by number or search-and-replace

### Tool System
- **#014: Tool Calling Format** - OpenAI API specification format
- **#015: Tool Prefix Prompt** - Tool definitions in system prompt
- **#016: Tool Result Context** - Results added to conversation history
- **#017: TUI Tool Feedback** - Visual tool call/status feedback
- **#018: LLM Error Feedback** - Detailed error messages to LLM

### Inference
- **#010: Streaming Inference** - Real-time token streaming, configurable timeout

### GitHub Copilot Support
- **#031: GitHub Copilot Backend** - Full Copilot API support including:
  - Configurable endpoint (`https://api.githubcopilot.com`)
  - `GITHUB_TOKEN` environment variable fallback
  - Copilot-specific headers (`Copilot-Integration-Id`, `Editor-Version`)
  - Correct request paths for Copilot and GitHub Models endpoints
  - 429 rate limit retry with backoff
  - Clear authentication error messages
  - Streaming tool-call assembly (merge by index, normalize type)
  - GitHub Models API support (`https://models.github.ai`)

## Upcoming Features

_None - all planned features have been implemented._

## Completed Feature Count

**33 / 33 features implemented**
