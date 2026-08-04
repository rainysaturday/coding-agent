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

## Robustness & Edge Cases



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
| Fix first | #1 duplicate output ✅ fixed, #2 scanner 64KB limit ✅ fixed,  #3 file-permission loss ✅ fixed |
| Next |  #4 vision token/system prompt ✅ fixed,  #5 dump filename ✅ fixed,  #6 signal close race ✅ fixed,  #7 compression first-message ✅ fixed |
| Clean up |  #8 UTF-8 truncation ✅ fixed,  #9 subagent config ✅ fixed,  #10 robustness,  #11-18 maintainability |

---

*Review performed against `implementation/` as of the current working tree.
Build: `go build ./...` OK · `go vet ./...` OK · `go test ./...` all passing.*
