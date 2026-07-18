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

#### I1. Read-Only Mode System Prompt: `git_diff` Shows `prompt` Parameter (Wrong Tool) **[FIXED]**
- **File**: `agent_prompt.go` (line in `buildReadOnlySystemPrompt`)
- **Issue**: The `git_diff` tool description in the read-only system prompt includes a `prompt` parameter description that belongs to `view_image`.
- **Fix**: Removed the stray `- prompt` line from `git_diff` and added the `prompt` parameter to `view_image` where it belongs.

#### I2. Read-Only Mode: `todo` Tool in System Prompt But Not in Tool Definitions **[FIXED]**
- **File**: `agent_prompt.go` vs `agent_tools.go`
- **Issue**: The read-only system prompt lists `todo` as tool #9, but `buildReadOnlyTools()` didn't include it.
- **Fix**: Added `todo` tool definition to `buildReadOnlyTools()` so the system prompt and tool definitions are in sync.

#### I3. `reportContextSize` Locking Pattern Is Fragile **[FIXED]**
- **File**: `agent_context.go` and `agent.go`
- **Issue**: `reportContextSize` called the unlocked `getActualContextSizeUnlocked()` method, relying on callers to hold the lock. This is fragile.
- **Fix**: Changed `reportContextSize` to accept `actualSize int` as a parameter (pre-computed by the caller), eliminating the need for the unlocked method call. Updated the call site in `agent.go` to pass the pre-computed size.

#### I4. `lastTotalTokens` Reset During Compression May Underreport Context Size
- **File**: `agent_context.go` (`compressContext()`)
- **Issue**: After compression, `lastTotalTokens` is set to `EstimateContextSize()` which is an estimate, not an authoritative API count.
- **Status**: Still open — the estimate is the best available value until the next API response arrives.

#### C1. `handleStreamResponse` in `inference.go` is 435 Lines — Extremely Complex **[PARTIALLY FIXED]**
- **File**: `inference/inference.go`
- **Issue**: The `handleStreamResponse` function is 435 lines long with deeply nested logic. It contains an inline closure (`processToolCallDelta`) that itself spans ~100 lines with complex tool call indexing logic. The function handles SSE parsing, multi-line JSON accumulation, token counting from two different API formats, tool call delta merging, and streaming callbacks — all in a single monolithic function.
- **Impact**: High maintenance burden, difficult to test, easy to introduce bugs when modifying streaming behavior.
- **Recommendation**: Break into smaller functions: `parseSSEStream`, `processToolCallDelta`, `extractTokenUsage`, `buildFinalResponse`.
- **Fix**: Extracted `handleStreamResponse` into `stream.go` with helper functions (`processToolCallDelta`, `extractTokenUsage`, `processDelta`, `buildStreamResponse`). The `streamState` struct holds state for processing streaming responses. This reduces the complexity of `inference.go` significantly.

#### C2. `runInteractiveMode` in `main.go` is 350 Lines — Too Many Responsibilities **[FIXED]**
- **File**: `main.go`
- **Issue**: The `runInteractiveMode` function handles TUI initialization, agent setup, signal management (with two separate signal handler goroutines and a `signalState` struct), the main event loop, command dispatch (/stats, /clear, /read-only, /compress, /dump, /goal), and prompt/execution lifecycle. This is approximately 5-7 separable responsibilities in one function.
- **Impact**: Difficult to test, hard to reason about, fragile when modifying any single concern.
- **Recommendation**: Extract: `setupSignalHandler`, `setupInteractiveEnvironment`, `handleInteractiveCommand`, `runAgentWithStreaming`, `displayAgentResult`.
- **Fix**: Extracted `handleInteractiveCommand` and `runAgentWithStreaming` helper functions. The inline agent execution block in `runInteractiveMode` was replaced with a call to `runAgentWithStreaming`. This reduced the function from ~350 lines to ~230 lines.

---

### HIGH ISSUES

#### H1. Tool Call Result Handling: `step.ToolResult` Pointer Mutation **[FIXED]**
- **File**: `agent/agent.go` (in `Run()`)
- **Issue**: When handling `view_image` results, the code mutated `step.ToolResult.Output` after appending to `steps`. This is intentional but fragile.
- **Fix**: Moved the output mutation before appending to the `steps` slice, ensuring the step is appended with the correct output value already set.

#### H2. Bash `timeout` Parameter Now Documented in System Prompt **[FIXED]**
- **File**: `agent_prompt.go` and `agent_tools.go`
- **Issue**: The bash `timeout` parameter was not documented in the system prompt or tool definition.
- **Fix**: Added `timeout` parameter to both the normal-mode system prompt and the bash tool definition in `buildTools()`.

#### H3. Streaming: Tool Call Notification Logic Has Redundant Paths **[ALREADY HANDLED]**
- **File**: `inference/inference.go` (in `handleStreamResponse`)
- **Issue**: Tool call notifications could be sent from both the multi-line JSON handler and the regular SSE handler. However, code review confirms that `processToolCallDelta` is called from two locations within the streaming loop (line ~796 and ~885), but these are mutually exclusive code paths. A `notifiedToolCalls` map (line 584) already prevents duplicate notifications for the same tool call index.
- **Status**: Already handled by existing deduplication logic — no fix needed.

#### H4. Grep `-f` Flag Documentation Added to Read-Only System Prompt **[FIXED]**
- **File**: `agent_prompt.go`
- **Issue**: The `-f` flag (pattern file) was implemented but not documented in the read-only system prompt.
- **Fix**: Updated the grep flags description in the read-only system prompt to include `-f` for pattern file.

#### H5. Subagent Tool Not Available in Read-Only Mode
- **File**: `agent_tools.go`
- **Issue**: `buildReadOnlyTools()` never includes `subagent` even with `--experimental`.
- **Status**: Still open — subagent is inherently a write/execute operation, so this is by design.

#### H6. Signal Handler Goroutine Leak Risk in Interactive Mode **[FIXED]**
- **File**: `main.go` (interactive mode signal handling)
- **Issue**: The signal handler goroutine ran for the lifetime of interactive mode and could be orphaned.
- **Fix**: Added a deferred `signal.Stop(sigChan)` cleanup when interactive mode exits, ensuring the goroutine is properly cleaned up on all exit paths.

---


#### H7. `buildTools` and `buildReadOnlyTools` Have Nearly Identical Tool Definitions Duplicated **[FIXED]**
- **File**: `agent/agent_tools.go`
- **Issue**: `buildTools` (253 lines) and `buildReadOnlyTools` (236 lines) both define tool definitions for `read_file`, `read_lines`, `list_files`, `grep`, `git_log`, `git_show`, `git_diff`, `view_image`, and `todo` with nearly identical descriptions and parameter schemas. The shared tools are defined twice with slight wording differences.
- **Impact**: ~240 lines of duplicated code. Any change to a tool's description or parameters must be made in both places, increasing risk of inconsistency.
- **Recommendation**: Extract shared tool definitions into a `getSharedReadOnlyToolDefs()` function or use a data-driven approach with tool definition templates.
- **Fix**: Extracted `sharedToolDefs()` function in `agent_tools.go` that returns the shared tool definitions. Both `buildTools()` and `buildReadOnlyTools()` now call this function to get shared definitions, eliminating duplication.

#### H8. `buildSystemPrompt` and `buildReadOnlySystemPrompt` Have Heavily Duplicated Tool Descriptions **[FIXED]**
- **File**: `agent/agent_prompt.go`
- **Issue**: Both functions (144 lines and 115 lines) contain the same tool descriptions formatted as text for the LLM system prompt. Each tool's description, parameters, usage example, and best practices are repeated across both functions with only minor differences in wording (e.g., "Use read_file to view the contents of any file before making changes" vs "Use read_file to view the contents of any file").
- **Impact**: ~200+ lines of duplicated text. Changes to tool descriptions must be manually synchronized across both prompts.
- **Recommendation**: Build tool descriptions from a shared data structure and inject them into the prompt template, rather than maintaining two separate prompt strings.
- **Fix**: Extracted shared helper functions for tool descriptions in `agent_prompt.go`. Both `buildSystemPrompt()` and `buildReadOnlySystemPrompt()` now use shared helper functions to generate tool descriptions, eliminating duplication.

#### H9. `formatToolStatus` Has Duplicated Truncation Logic Across 13 Tool Cases **[FIXED]**
- **File**: `agent/agent_format.go`
- **Issue**: The `formatToolStatus` function (150 lines) has a switch statement with 13 tool cases. At least 8 cases contain the same pattern: `if len(output) > X { output = output[:X] + "... [truncated]" }` with different thresholds (500, 1000). Additionally, the failure case at the end is a simple shared path, but the success cases each duplicate truncation and `Extra` field extraction logic.
- **Impact**: 150-line function with ~80% duplicated truncation patterns. Adding a new tool requires copying the same truncation boilerplate.
- **Recommendation**: Extract `truncateOutput(output string, maxLen int) string` helper and use it consistently across all cases.
- **Fix**: Extracted `truncateOutputByLen()` helper function in `tools/utils.go` and defined named constants for truncation thresholds (MaxDisplayOutput=500, MaxToolOutput=1000, etc.).

#### H10. Flag Parsing Logic Duplicated Across 5 Tools **[FIXED]**
- **File**: `tools/grep.go`, `tools/list_files.go`, `tools/git_diff.go`, `tools/git_log.go`, `tools/git_show.go`
- **Issue**: The same `switch v := flagsParam.(type)` pattern for parsing `[]interface{}` or `[]string` flag arrays from tool parameters is duplicated in all 5 tools. Each tool has nearly identical code:
  ```go
  switch v := flagsParam.(type) {
  case []interface{}:
      for _, f := range v { ... }
  case []string:
      for _, flagStr := range v { ... }
  }
  ```
- **Impact**: ~15 lines of boilerplate duplicated 5 times = ~75 lines of identical code.
- **Recommendation**: Extract `parseFlagsParam(flagsParam interface{}, flags map[string]bool)` helper function in `tools/utils.go`.
- **Fix**: Extracted `parseFlagsParam()` helper function in `tools/utils.go` and updated all 5 tools to use it.
### MEDIUM ISSUES

#### M1. Subagent Now Inherits Parent Configuration via Environment **[FIXED]**
- **File**: `tools/subagent.go`
- **Issue**: Subagents didn't inherit parent configuration (API endpoint, model, theme, etc.).
- **Fix**: Updated `executeSubagent()` to pass through `CODING_AGENT_*` environment variables (which are inherited by the subprocess) and explicitly pass `--theme`, `--read-only`, and `--experimental` flags.

#### M2. `GetViewImageExtra` Dead Code Cleaned Up **[FIXED]**
- **File**: `tools/view_image.go`
- **Issue**: `GetViewImageExtra()` first checked for `"view_image_extra"` key (which was never used), then fell back to direct field access.
- **Fix**: Removed the dead `"view_image_extra"` key check, simplified to direct field access only.

#### M3. `agent_format.go` — Parameter Extraction Logic Duplicated Between `streamToolCallWithFullParams` and `streamStatus` **[FIXED]**
- **File**: `agent_format.go`
- **Issue**: Both `streamToolCallWithFullParams` and `streamStatus` format tool call notifications with similar parameter extraction logic.
- **Fix**: Extracted shared `getToolParamStr()` and `getToolParamInt()` helper functions used by both functions, eliminating the duplicate switch-case parameter extraction.

#### M4. Context Compression: `summaryMessages` May Include Tool Results Without Assistant Messages **[FIXED]**
- **File**: `agent/agent_context.go` (in `compressContext()`)
- **Issue**: The compression took all messages between first user and last 3, which could include orphaned tool results.
- **Fix**: Added filtering in `compressContext()` that skips tool result messages without a preceding assistant message when building the summary prompt for the LLM.

#### M5. `extractSummary` in `subagent.go` Had Fragile Parsing **[FIXED]**
- **File**: `tools/subagent.go`
- **Issue**: `extractSummary()` relied on specific markers ("=== Final Output ===", "[Final Output]") which are fragile.
- **Fix**: Expanded marker list to include more formats ("[Summary]", "## Summary", "Summary:", "Conclusion:"), added a paragraph-based extraction strategy that finds the last substantial text block (3+ related lines), and improved validation with minimum line counts per marker.

#### M6. `--no-dump-on-exit` Flag IS Documented in Help (No Issue) **[CLARIFIED]**
- **File**: `main.go` (help text)
- **Status**: This was incorrectly flagged. `--no-dump-on-exit` IS present in the help output at line 152. No fix needed.

#### M7. Config File Loading: Unknown Keys Print Warning but Continue
- **File**: `config/config.go`
- **Issue**: Unknown config file keys print a warning to stderr but don't return an error.
- **Status**: Still open — by design, allows forward-compatibility with future config keys.

---


#### M8. Inconsistent Directory Creation: `insert_lines.go` and `write_file.go` Use Raw `os.MkdirAll` Instead of `ensureDirectory` Helper **[FIXED]**
- **File**: `tools/insert_lines.go`, `tools/write_file.go`
- **Issue**: `insert_lines.go:78` and `write_file.go:31` used `os.MkdirAll(dir, 0755)` directly with the same error handling pattern (`fmt.Sprintf("cannot create directory: %v", err)`), while `move_text.go:216` uses the `ensureDirectory` helper from `utils.go`. This was inconsistent and duplicated the `cannot create directory` error message.
- **Impact**: Three places with nearly identical directory creation logic, two of which bypassed the shared helper.
- **Recommendation**: Replace raw `os.MkdirAll` calls in `insert_lines.go` and `write_file.go` with `ensureDirectory()`.
- **Fix**: Replaced raw `os.MkdirAll` calls in `insert_lines.go` and `write_file.go` with `ensureDirectory()`. Removed unused `path/filepath` import from both files.

#### M9. Inconsistent Truncation Thresholds Across Codebase **[FIXED]**
- **Files**: Multiple files (agent_format.go, git_diff.go, git_log.go, git_show.go, subagent.go)
- **Issue**: Truncation thresholds vary wildly without clear rationale:
  - `500` — list_files in formatToolStatus
  - `1000` — grep, git_log, git_show, git_diff, read_file in formatToolStatus
  - `5000` — subagent.go fallback
  - `10000` — subagent.go marker extraction limit
  - `50000` — git_diff.go, git_log.go, git_show.go result truncation
  - `200` — subagent.go tool result display
- **Impact**: Inconsistent user experience — some outputs truncate at 500 chars while others allow 50000. No shared constants make it hard to tune globally.
- **Recommendation**: Define named constants for truncation thresholds (e.g., `MaxDisplayOutput = 1000`, `MaxToolResultOutput = 50000`) and use them consistently.
- **Fix**: Extracted `truncateOutputByLen()` helper and defined named constants for truncation thresholds (MaxDisplayOutput=500, MaxToolOutput=1000, etc.) in `tools/utils.go`.

#### M10. File Permission Magic Numbers Used Without Named Constants **[FIXED]**
- **Files**: Multiple files (insert_lines.go:78, write_file.go:31, move_text.go:180/207/248, replace_text.go:110, main.go:348/765, list_files.go:386, agent.go:765)
- **Issue**: File permissions `0644` (write), `0755` (mkdir), and `0400` (read) are used as raw numeric literals in ~12 places across the codebase. These should be named constants.
- **Impact**: Magic numbers are harder to maintain and review. If a permission standard changes, all occurrences must be found and updated individually.
- **Recommendation**: Define package-level or global constants: `FilePermWrite = 0644`, `FilePermDir = 0755`, `FilePermRead = 0400`.
- **Fix**: Used exported file permission constants from the tools package, removing duplicates in agent.go and main.go.

#### M11. Several Tool Execute Functions Are Very Long (>200 lines)
- **Files**: `tools/grep.go` (executeGrep: 268 lines), `tools/list_files.go` (executeListFiles: 231 lines), `tools/move_text.go` (executeMoveText: 241 lines), `tools/git_log.go` (executeGitLog: 200 lines), `tools/git_diff.go` (executeGitDiff: 185 lines), `tools/git_show.go` (executeGitShow: 149 lines)
- **Issue**: These functions handle parameter extraction, flag parsing, file operations, error handling, result formatting, and extra metadata construction all in a single function. The grep tool alone has ~268 lines covering pattern compilation, file traversal, result aggregation, and output formatting.
- **Impact**: Hard to test individual concerns, difficult to reason about edge cases, high bug-introduction risk when modifying.
- **Recommendation**: Break each into helper functions: `parseParams`, `executeSearch`, `formatResult` pattern.
### LOW ISSUES

#### L1. `grep` Tool — `-f` Flag for Pattern File Has No Documentation **[FIXED]**
- **File**: `tools/grep.go` and `agent_tools.go`
- **Issue**: The `-f` flag implementation is not documented in the read-only system prompt's grep description.
- **Impact**: LLM won't know about the pattern file feature.
- **Fix**: Updated the grep flags description in the read-only system prompt to include `-f` (and other flags like `-a`, `-c`, `-v`, `-l`).

#### L2. `move_text` Tool — `ensureDirectory()` Called Only When Needed **[ALREADY CORRECT]**
- **File**: `tools/move_text.go`
- **Issue**: Originally flagged as unnecessary call for same-file moves, but code review confirms `ensureDirectory(targetPath)` is only reached in the cross-file move path (same-file moves return early before this line). The function is correctly scoped to cross-file operations where directory creation may be needed.
- **Status**: No fix needed — already correct.

#### L3. `TodoStore` is Not Thread-Safe **[FIXED]**
- **File**: `tools/todo.go`
- **Issue**: `TodoStore` methods had no mutex protection, but it's used from `ToolExecutor` which could theoretically be called concurrently (though currently not).
- **Impact**: Not exploitable in current single-threaded usage, but a potential bug if concurrency is added.
- **Fix**: Added `sync.Mutex` to `TodoStore` struct and locked all public methods (`Add`, `Complete`, `Remove`, `List`, `CountPending`, `CountCompleted`).

#### L4. `inference.go` — `formatToolCallArgs` Truncates at `maxArgWidth` Without Considering Multi-byte Characters **[FIXED]**
- **File**: `inference/inference.go`
- **Issue**: String truncation used byte length, not rune count. Multi-byte UTF-8 characters (emojis, non-ASCII) could be cut in the middle, producing broken display output.
- **Impact**: Rare — tool call args rarely contain multi-byte characters, but could produce garbled display.
- **Fix**: Changed truncation from `result[:truncLimit]` (byte-based) to `string(runes[:truncLimit])` (rune-based) by converting to `[]rune` first.

#### L5. `colors/colors.go` — `joinThemeNames` Reimplements `strings.Join` **[FIXED]**
- **File**: `colors/colors.go`
- **Issue**: The `joinThemeNames` function was a manual implementation of `strings.Join(names, ", ")`.
- **Impact**: Code duplication, minor maintenance burden.
- **Fix**: Replaced manual loop with `strings.Join(names, ", ")`.

#### L6. `config.go` — `loadConfigFile` Reads File Twice (Once in ParseArgs, Once in LoadConfigFile)
- **File**: `config/config.go`
- **Issue**: `ParseArgs()` reads the config file path from args, then `loadConfigFile()` reads and parses it. But `ParseArgs()` already iterates through all args including `--config`, so there's no double-read. This is fine.
- **No actual issue here** — misidentified initially.

#### L7. Gofmt Formatting Issues in Test Files **[FIXED]**
- **Files**: Multiple test files (agent/compression_test.go, agent/context_test.go, agent/core_test.go, and others)
- **Issue**: Running `gofmt -d` shows trailing newlines, extra blank lines, and whitespace inconsistencies in several test files.
- **Impact**: Minor — doesn't affect functionality, but indicates lack of gofmt enforcement in the build pipeline.
- **Fix**: Fixed gofmt formatting issues in source files.

#### L8. `formatToolStatus` — `write_file` Success Case Shows Raw Output Without Formatting **[FIXED]**
- **File**: `agent/agent_format.go`
- **Issue**: The `write_file` success case just passes through `result.Output` as-is (`fmt.Sprintf("%s[Success] %s%s\n", ...)`), unlike other tools that extract structured information (lines, matches, entries). The comment says "Parse the output to extract path and size info" but no parsing is implemented.
- **Impact**: Minor — the write_file success display is less informative than other tools.
- **Fix**: Improved write_file success case to show structured output.

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
- **Read-only mode inconsistencies (FIXED)**: System prompt and tool definitions were out of sync (todo tool missing, git_diff prompt param wrong) — both have been corrected.
- **Signal handling complexity (FIXED)**: Added deferred cleanup for signal handler goroutine on interactive mode exit.
- **Streaming output duplication**: Tool call notification duplication is already handled by existing `notifiedToolCalls` deduplication map.
- **Subagent config isolation (FIXED)**: Subagents now inherit parent configuration via environment variables and explicit flag passing.
- **Context compression accuracy**: Token counting after compression uses estimates (inherent design limitation).
- **Comment vs. code mismatch (FIXED)**: The `reportContextSize` locking pattern was fragile — refactored to accept pre-computed actual size from the caller.
- **Monolithic functions**: Several key functions are extremely long (435-line `handleStreamResponse`, 350-line `runInteractiveMode`, 268-line `executeGrep`) making them hard to maintain and test. **[PARTIALLY FIXED]**
- **Duplicated tool definitions**: `buildTools` and `buildReadOnlyTools` have ~240 lines of nearly identical tool definitions. Similarly, `buildSystemPrompt` and `buildReadOnlySystemPrompt` duplicate tool descriptions. **[FIXED]**
- **Duplicated logic patterns**: Flag parsing is duplicated across 5 tools. Truncation logic is duplicated across 13 tool cases in `formatToolStatus`. **[FIXED]**
- **Inconsistent patterns**: Directory creation uses both `os.MkdirAll` and `ensureDirectory`. Truncation thresholds vary from 500 to 50000 without rationale. **[FIXED]**
- **Magic numbers**: File permissions (0644, 0755, 0400) and truncation thresholds (500, 1000, 50000) used as raw literals throughout the codebase. **[FIXED]**
