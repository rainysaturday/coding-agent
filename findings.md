# Code Review Findings — Coding Agent Harness

This document lists bugs and quality issues identified during a review of the Go
codebase in `implementation/`. The project is a minimal coding-agent harness with
an agent loop, LLM inference client, tool execution system, TUI, and configuration.

All code compiles (`go build ./...`), passes `go vet ./...`, and all tests pass
(`go test ./...`). The issues below are correctness concerns, robustness gaps,
and maintainability improvements. Each item is tagged by severity.

**Severity legend**
- 🔴 **High** — likely to cause incorrect behavior, data loss, or a crash.
- 🟠 **Medium** — edge-case correctness or robustness concern.
- 🟢 **Low** — code quality, maintainability, or minor UX.

---

## Bugs & Correctness Issues

### 2. 🔴 `bufio.Scanner` 64KB default limit can abort large streaming responses
**File:** `inference/stream.go` — `handleStreamResponse()` (line ~292)

```go
scanner := bufio.NewScanner(body)
```

`bufio.Scanner` has a default maximum token size of **64KB**. A single SSE `data:`
line that exceeds 64KB (e.g., a large tool-call `arguments` JSON blob or a large
content chunk) makes the scanner fail with `bufio.ErrTooLong`, which is returned
as a stream error and aborts the whole response.

**Fix:** Increase the buffer, e.g. `scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)`.

---

### 3. 🔴 File write tools clobber the original file permission bits
**Files:** `tools/write_file.go`, `tools/replace_text.go`, `tools/insert_lines.go`,
`tools/move_text.go`

Every write path calls `os.WriteFile(path, data, FilePermWrite)` where
`FilePermWrite = 0644`. When a tool rewrites an existing file, the file's original
mode is lost. For example, an executable script (0755) edited via
`replace_text`/`insert_lines` silently becomes non-executable (0644).

**Fix:** Before writing, `os.Stat` the target; if it exists, preserve its `Mode()`
(and ownership if possible) instead of hard-coding `0644`.

---

### 4. 🟠 Vision requests are not included in token accounting or given a system prompt
**File:** `agent/agent.go` — `handleViewImage()` (lines ~644-696)

When `view_image` is used, a second inference request is made to describe the image:

```go
response, err := a.inference.InferenceRequest(ctx, []*inference.Message{msg}, "")
```

- The `systemPrompt` is passed as `""`, so the vision model receives no system
  context (tools, persona, read-only instructions, etc.).
- The token usage of this vision request is never added to `Stats.InputTokens` /
  `Stats.OutputTokens`, so reported token counts undercount actual API usage.
- The image message is never added to the main conversation `context`; only the
  text description is stored as a tool result.

**Fix:** Pass the real system prompt, and accumulate the vision request's
`InputTokens`/`OutputTokens` into `Stats` (and goal token counters).

---

### 5. 🟠 `DumpContext` produces inconsistent filenames
**File:** `agent/agent.go` — `DumpContext()` (lines ~723-772)

The first dump is written to `coding-agent-context.json`, but every subsequent
dump is written to `coding-agent-context-2.json`, `-3.json`, etc. (no `.json`
suffix on the numbered variants). The naming is inconsistent, and the counter
always starts at `2` regardless of what files already exist (it only skips
collisions by incrementing), which can leave stale/garbage files.

**Fix:** Use a consistent pattern (e.g. `coding-agent-context-1.json`, `-2.json` …)
and scan existing files to pick the next available number.

---

### 6. 🟠 Potential race/panic around `close(sigChan)` in interactive mode
**File:** `main.go` — `runInteractiveMode()` (lines ~688-715, 750-760)

The signal-handler goroutine ranges over `sigChan` while the main loop calls
`signal.Stop(sigChan)` followed by `close(sigChan)` when exiting. Because
`signal.Notify` delivers asynchronously, there is a small window where a pending
signal could be delivered after `signal.Stop` but before/after `close`, writing to
a closed channel and causing a panic. The `defer signal.Stop(sigChan)` inside the
`for range` loop body is also misleading (defers accumulate and run only when the
goroutine exits).

**Fix:** Avoid closing a channel that `signal.Notify` may still target; use a
`context`/done channel to shut down the handler goroutine and stop signals
cleanly, and remove the in-loop `defer`.

---

### 7. 🟠 `compressContext` assumes the first context message is the user prompt
**File:** `agent/agent_context.go` — `compressContext()` (lines ~150-164)

```go
firstUserMsg := messages[0]
...
summaryMessages := messages[1 : len(messages)-preserveCount]
```

The code tries to find the "first user message" (looping through if `messages[0]`
isn't a user message), but the summary slice is always computed from `messages[1:]`.
After a context load/restore (or a goal-injected message), the first real user
message may not be `messages[0]`; it could then be *both* preserved (as
`firstUserMsg`) and included in the summarized range — or dropped from the summary.

**Fix:** Compute the summary slice based on the index of the message actually
selected as `firstUserMsg`.

---

## Robustness & Edge Cases

### 8. 🟠 `read_lines`/preview code splits on byte length, not runes
**Files:** `tools/utils.go` (`truncateString`, `truncateOutput`, `TruncateOutputByLen`),
`agent/agent_format.go`

Several truncation helpers slice `text[:maxLen]` by byte count. For UTF-8 content
(e.g. CJK, emoji), this can split a multi-byte rune in the middle, producing
invalid UTF-8 in output and tool results sent back to the LLM.

**Fix:** Use `[]rune`-aware truncation or guard against cutting mid-rune.

---

### 9. 🟠 Subagent does not fully inherit parent configuration
**File:** `tools/subagent.go` — `executeSubagent()` (lines ~46-107)

Only settings that exist as `CODING_AGENT_*` env vars (or `GITHUB_TOKEN`) are
inherited. Configuration provided purely via CLI flags (e.g. `--model`,
`--api-endpoint`, `--api-key`, `--max-tokens`, `--temperature`, `--persona` from
the parent, `--tools`) is **not** forwarded to the subprocess. A subagent can
therefore silently run against a different model/endpoint than its parent.

**Fix:** Forward the parent's resolved config (model, endpoint, key, max tokens,
tools, persona, etc.) explicitly to the child process, or document the limitation.

---

### 10. 🟠 Endpoint detection relies on `strings.Contains`
**Files:** `config/config.go` (`IsGitHubCopilotEndpoint`),
`inference/inference.go` (`isGitHubCopilotEndpoint`, `isGitHubModelsEndpoint`)

Endpoint type is inferred by substring matching on the URL. A custom proxy or
gateway whose URL merely contains `githubcopilot.com` / `models.github.ai` would
be misclassified and sent incompatible headers or a wrong path.

**Fix:** Compare against known hostnames/URL prefixes (or normalize the URL) rather
than substring matching.

---

## Code Quality & Maintainability

### 11. 🟠 Tool lists are duplicated in three places
**Files:** `agent/agent_tools.go` (`getToolNames`), `tools/tools.go`
(`DefaultNormalTools`, `DefaultReadOnlyTools`), `main.go` (`displayTools`)

The set of default/read-only/experimental tool names is maintained independently in
at least three locations (plus the read-only banner string in `agent.go`). Adding
or removing a tool requires updating every copy, risking drift between the system
prompt, the registered tools, and the help text.

**Fix:** Centralize the canonical tool lists in one place (e.g. in `tools`) and have
`agent` and `main` reference it.

---

### 12. 🟢 `buildToolListSection` has a redundant `readOnly` parameter
**File:** `agent/agent_prompt.go` — `buildToolListSection()` (lines ~62-71)

Both branches of the `if readOnly` produce identical output; the parameter is dead.

**Fix:** Remove the parameter and collapse the two branches.

---

### 13. 🟢 Dead code
- `agent/agent_format.go` — `streamStatus()` (lines ~160-237) is defined but never called.
- `tools/subagent.go` — `formatSubagentResult()` / `streamSubagentResult()`
  (lines ~14-41) are never called (the subagent status is handled by
  `formatToolStatus` in `agent_format.go`).
- `tools/utils.go` — constants `MaxSubagentOutput`, `MaxSubagentMarkerLimit`,
  `MaxSubagentResultDisplay` are unused.

**Fix:** Remove or wire up the dead code so it doesn't rot.

---

### 14. 🟢 Error classification logic is duplicated
**Files:** `agent/agent_errors.go` (`isAuthError`, `isContextLimitError`,
`wrapError`) and `main.go` (`exitCodeForError`)

Both perform the same fragile string matching against error messages (e.g.
`"401"`, `"403"`, `"context size limit"`). If one is updated, the other can drift,
causing inconsistent exit codes.

**Fix:** Have `exitCodeForError` use the typed errors (`AuthError`,
`ContextLimitError`) produced by `wrapError` rather than re-string-matching.

---

### 15. 🟢 Invalid `--tools` names are silently ignored
**Files:** `config/config.go`, `agent/agent_tools.go` (`buildTools`), `main.go`

If a user passes `--tools "read_file,not_a_tool"`, the unknown name is silently
skipped when building tool definitions, so the model is told about a tool that
isn't registered (or a tool is missing) with no warning.

**Fix:** Validate tool names against `tools.AllToolNames()` and return an error
(or at least warn) for unknown entries.

---

### 16. 🟢 Re-entrancy risk: callbacks invoked while holding `Agent.mu`
**File:** `agent/agent.go` — `Run()` (e.g. `reportContextSize` at line ~437, and
`a.stats.Iterations` updates)

Several callbacks (context-size reporting, stream callbacks) are dispatched while
`a.mu` is held. If a callback ever re-enters the agent (e.g. the TUI triggering a
`/compress`), it could deadlock. The code is mostly careful to copy state before
dispatching, but the pattern is fragile.

**Fix:** Capture the needed values under the lock, release the lock, then invoke
callbacks.

---

### 17. 🟢 Custom `formatInt` reimplements `strconv`
**File:** `tools/read_file.go` — `formatInt()` (lines ~115-129)

`formatInt` is a hand-rolled decimal formatter used only by `formatFileSize`.
This duplicates the stdlib (`strconv.FormatInt` / `strconv.Itoa`) for no benefit.

**Fix:** Use `strconv.FormatInt(n, 10)` and delete `formatInt`.

---

### 18. 🟢 `handleStreamResponse` reuses a `chunk` struct with manual field reset
**File:** `inference/stream.go` (lines ~372-380)

The `chunk` variable is reused across loop iterations, and fields are reset
manually before each `json.Unmarshal`. Adding a new field to the chunk struct
later risks stale-data bugs (the reset list must be kept in sync).

**Fix:** Declare a fresh struct per iteration (or reset via `chunk = struct{...}{}`)
so unmarshalling always starts clean.

---

## Minor UX / Notes

- **One-shot mode "Thinking Complete" separator** (`main.go` ~291-295): prints the
  `--- Thinking Complete ---` banner before the first normal chunk even when the
  model never produced reasoning, so it can appear spuriously.
- **`view_image` 20MB limit**: base64 encoding inflates the data URI ~1.33× and it
  is sent inline in the request; large images can produce very large requests and
  token estimates. Consider downscaling or an explicit size cap on the data URI.
- **`git_*` tools pass user-supplied `reference`/`commit` strings directly to git**
  (no shell, so no injection), but unusual references can still cause surprising
  git behavior; consider validating the reference format.

---

## Summary of Recommended Priorities

| Priority | Item(s) |
|----------|---------|
| Fix first | ~~#1 duplicate output~~ ✅ fixed, #2 scanner 64KB limit, #3 file-permission loss |
| Next | #4 vision token/system prompt, #5 dump filename, #6 signal close race, #7 compression first-message |
| Clean up | #8-10 robustness, #11-18 maintainability |

---

*Review performed against `implementation/` as of the current working tree.
Build: `go build ./...` OK · `go vet ./...` OK · `go test ./...` all passing.*
