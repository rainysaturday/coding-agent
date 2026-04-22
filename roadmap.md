# Roadmap - Minimal Coding Agent Harness

## Implemented Features

### Core Infrastructure
- **#001: Go Runtime** - Project uses Go modules, compiles without errors, cross-platform support
- **#007: Inference Backend** - OpenAI-compatible REST endpoint, API key auth, llama.cpp server support
- **#024: Zero External Dependencies** - No external Go modules, stdlib only
- **#023: Versioning** - Git commit hash, dirty status, build time embedding

### Session Management
- **#034: Conversation Save/Load** - Save and restore conversation sessions with /save and /load commands, JSON format

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

### Tools (9 Total)
- **#004: Bash Tool** - Execute shell commands
- **#005: Read File Tool** - Read file contents
- **#006: Write File Tool** - Write/overwrite files
- **#011: Read Lines Tool** - Read specific line ranges
- **#012: Insert Lines Tool** - Insert lines at position
- **#013: Replace Text Tool** - Find and replace text
- **#030: Patch Tool** - Apply unified diff patches (via system `patch` command)
- **#032: Replace Lines Tool** - Replace lines by number or search-and-replace
- **#035: Sub-Agent Tool** - Spawn parallel sub-agents with `-p "prompt"` for delegated tasks, configurable timeout

### Git Tools
- **#036: Git Integration Tool** - Five git tools for repository interaction:
  - `git_status` - Check staged, unstaged, and untracked files
  - `git_diff` - View staged/unstaged diffs with per-file support
  - `git_log` - View commit history with branch and count filters
  - `git_show` - View file content at specific git refs
  - `git_add` - Stage specific files or all tracked modified files
- **#037: Git Commit Tool** - Commit staged changes with descriptive messages, amend support

### Tool System
- **#014: Tool Calling Format** - OpenAI API specification format
- **#015: Tool Prefix Prompt** - Tool definitions in system prompt
- **#016: Tool Result Context** - Results added to conversation history
- **#017: TUI Tool Feedback** - Visual tool call/status feedback
- **#018: LLM Error Feedback** - Detailed error messages to LLM

### Content Search
- **#038: Find Tool** - Search file contents with regex patterns, structured output with file/line/content

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

**38 / 38 features implemented**
