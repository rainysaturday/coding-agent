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
- **#049: Git Branch Tool** - Unified branch management with list, create, checkout, delete, rename, and set_upstream actions; branch name validation; upstream tracking

### Git Tools (Extended)
- **#050: Git Stash Tool** - Manage git stashes with list, save, pop, apply, and drop actions; stash message support; stashed file metadata

### Git Tools (Extended)
- **#059: Git Tag Management Tool** - Manage git tags with four actions: list (with pattern filtering and sorting), create (annotated/lightweight), delete, and show (with tagger/date/message/object ref details); tag name validation

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

### External Access
- **#039: Web Fetch Tool** - HTTP GET requests to fetch web content, configurable timeout and max response size

### File Management
- **#040: Move File Tool** - Move or rename files within the filesystem, with directory creation support and path validation
- **#042: Copy File Tool** - Copy files from source to destination with path validation, directory creation, permission preservation, and overwrite protection

### Directory Management
- **#041: List Directory Tool** - List directory contents with metadata (file type, size, modification time), recursive mode, hidden files support
- **#043: Delete File Tool** - Delete files from the filesystem with path validation and error handling

### Code Scaffolding
- **#044: Scaffold Tool** - Generate code from built-in templates with variable substitution. Templates: go_struct, go_handler, go_service, python_class, python_dataclass, proto_message, openapi_schema, go_test

### Project Understanding
- **#046: Project Tree Tool** - Generate a visual directory tree with file metadata (type, size, permissions), depth limiting, glob filtering, and hidden file toggling

### Test Execution
- **#045: Run Tests Tool** - Execute tests for the current project with auto-detection of project type (Go, Node.js, Python). Supports custom commands, arguments, and timeouts. Returns structured results with pass/fail status, exit code, and failure summaries.

### Code Navigation
- **#047: Code Navigation Tool** - Navigate code to find definitions, references, or implementations of symbols across the codebase. Supports multiple languages (Go, Python, JavaScript/TypeScript, Rust, Java, etc.) with grep-based pattern matching. Features three modes: `definitions` (find where defined), `references` (find all usages), `implementations` (find interface implementations). Supports language-specific file filtering, path scoping, and configurable result limits.

### Link Validation
- **#048: Broken Link Checker Tool** - Scan Markdown and HTML files for broken links (both internal file links and external URLs). Detects relative paths, image references, and HTTP/HTTPS URLs. Returns structured results with summaries of valid and broken links, with details for each broken link.

### Data/Format Tools
- **#051: JSON Transformer Tool** - Transform JSON data with multiple operations: extract value by path using dot notation, set/modify fields at arbitrary paths, merge multiple JSON files/sources, validate required fields, format/beautify with configurable indentation, convert to YAML, and convert to environment variable format. Uses only Go stdlib (encoding/json) - zero external dependencies.

### Project Quality
- **#052: Project Diagnostics Tool** - Scan a codebase for common issues and quality problems. Detects TODO/FIXME/HACK/WARN/XXX markers, empty files, large files (>500 lines), hardcoded secrets/keys patterns. Returns structured report with severity levels (low, medium, high, critical) and recommendations. Supports configurable scan depth and path scoping.

### Code Quality
- **#053: Run Lint Tool** - Run linters for the current project and report structured results. Auto-detects project type (Go, Python, Node.js, Shell) and runs appropriate linters (gofmt, go vet, flake8, pylint, eslint, shellcheck). Supports custom commands, arguments, and timeouts. Returns pass/fail status with formatted output.

### File Management
- **#054: File Rename with Reference Update** - Rename or move a file and automatically update all code references (imports, includes, requires, etc.) across the codebase. Supports custom search paths to limit the scope of reference updates. Uses only Go stdlib (no external dependencies).

### Process Management
- **#055: Process Management Tool** - Manage running processes and check system resources: process_list (list/filter processes), process_kill (kill by PID or name), port_check (check TCP/UDP ports), system_info (CPU/memory/disk usage).

### Environment Variables
- **#056: Environment Variable Manager Tool** - Read and manage environment variables with five actions: get (read a variable), set (set a variable), unset (remove a variable), list (list all or filtered variables), and source (load variables from a .env file). Uses only Go stdlib - zero external dependencies.

### File Comparison
- **#057: File Compare Tool** - Compare two text files and return a structured unified diff showing added, removed, and unchanged lines with line numbers and context. Supports configurable context lines, binary file detection, and directory traversal protection. Uses only Go stdlib (LCS algorithm) - zero external dependencies.

### Changelog Generation
- **#058: Changelog Generator Tool** - Generate changelog entries from git commit history. Groups conventional commits by category (Features, Bug Fixes, Breaking Changes, etc.) following Keep a Changelog format. Supports 'generate' (output to stdout) and 'add' (append to CHANGELOG.md) actions. Handles tag ranges, unreleased commits, and custom headers. Uses only Go stdlib - zero external dependencies.

### Build Execution
- **#060: Run Build Tool** - Execute project build commands with auto-detection of project type (Go, Node.js, Rust, Java/Maven, Java/Gradle, Python). Supports custom command override, arguments, and configurable timeout. Returns structured results with exit code and output.

### Test Coverage
- **#061: Test Coverage Analysis Tool** - Run project tests with coverage analysis and return structured coverage results. Auto-detects project type (Go, Node.js, Python) and reports overall coverage percentage, per-file coverage, and highlights low-coverage files (<50%) and files with no coverage (0%). Uses only Go stdlib for parsing - zero external dependencies.

## Completed Feature Count

**61 / 61 features implemented**

## Upcoming Features
