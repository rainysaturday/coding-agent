# Code Review — Minimal Coding Agent Harness

**Scope:** full Go source under `implementation/` (~34k LOC) — `agent/`, `inference/`, `tools/`, `config/`, `debug/`, `tui/`, `webui/`, `skills/`, `colors/`, `main.go` — plus `Makefile` and the test suite. Every package was read; findings below were checked against the code, and the ones marked "verified" were reproduced by running the built binary or calling the function directly.
**Reviewed at:** `f3204a8` (`style(tui,webui,colors)…`).
**Focus:** correctness, and especially *things that make this code hard to debug* — silent failures, misleading errors, missing observability, and drift between documentation and behaviour.

## Baseline (measured, not assumed)

| Check | Result |
|---|---|
| `go build ./...` | ✅ clean |
| `go vet ./...` | ✅ clean |
| `go test ./...` | ❌ **1 failure** — `agent.TestNewAgent_SkillsDiscovered` |
| `go test -race ./...` | ✅ no data races reported |
| `gofmt -l .` | ❌ 4 files unformatted |
| Coverage | `config` **49.2%**, `main` **29.6%**, `tui` 62.9%, `webui` 71.1%, `inference` 76.4%, `tools` 82.9%, `debug` 96.6% |

Note: `ISSUE_SUMMARY.md` and `implementation/findings.md` claim *"no outstanding findings remain"*. That is not true — the suite is red on a clean checkout, and the issues below are all reproducible.

**Severity:** 🔴 High · 🟠 Medium · 🟡 Low

---

## Fix status

All 11 **A** (high severity) and **B** (silent failure) issues are fixed and committed; each fix
was reproduced before and verified after. **C**, **D** and **E** remain open.

| ID | Issue | Commit |
|----|-------|--------|
| A1 | `--debug-verbose` prints the API key to stderr (and the comment claims it doesn't) | `0e74216` |
| A2 | Sub-agents truncate the parent's `debug.log` and inherit the parent's goal | `69f5e45` |
| A3 | The test suite is red on a clean machine (non-hermetic test) | `d53b391` |
| A4 | No upper bound on tool output entering the LLM context | `737e75b` |
| B1 | Unrecognised git flags are dropped without any error | `2b13981` |
| B2 | Multi-character `flags` entries are silently discarded for `grep`/`list_files` | `4bedf7a` |
| B3 | Bad numeric config values are ignored silently (file + env), but reported for CLI flags | `9d41d14` |
| B4 | Lenient parameter parsing turns type errors into misleading "missing parameter" messages | `25928a4` |
| B5 | `move_text` silently clamps an out-of-range `source_end` | `e05d866` |
| B6 | Errors compared by string | `4380f96` |
| B7 | `colors.GetColor` returns a reset code for an unknown slot name | `fb89cda` |

Notes on two fixes worth knowing about:

- **A4** bounds output at the *tool* layer rather than per-tool, so a future tool cannot forget to
  truncate. The marker reads `[... N of M bytes shown — re-run with a narrower range]`.
- **B1** deliberately keeps a *deny*-by-default posture: an unrecognised flag is an error rather
  than a pass-through, because `git log --help` would otherwise be sent to the LLM as if it worked.


## A. High severity

### A1. 🔴 `--debug-verbose` prints the API key to stderr (and the comment claims it doesn't) — ✅ FIXED (`0e74216`)

`inference/inference.go:352-366`

```go
for k, v := range req.Header {
    fmt.Fprintf(os.Stderr, "%s=%s", k, strings.Join(v, ";"))   // includes Authorization
}
...
// Log body (redact authorization)
fmt.Fprintf(os.Stderr, "[DEBUG-VERBOSE] >>> Body (%d bytes): %s\n", ...)
```

Verified empirically (once per retry attempt):

```
$ CODING_AGENT_DEBUG_VERBOSE=true CODING_AGENT_API_KEY="sk-SECRETDO_NOTLOG1234567890" ./agent -p "hi"
[DEBUG-VERBOSE] >>> Headers: Authorization=Bearer sk-SECRETDO_NOTLOG1234567890, Content-Type=application/json
```

Every request logs `Authorization=Bearer sk-…` in plaintext. It lands in the terminal scrollback, CI logs, and anything piped from stderr. The comment *"redact authorization"* is false — **there is no redaction helper anywhere in the `inference` package** (`grep -n redact inference/*.go` → only that comment). This is the worst kind of misleading comment: it tells the next debugger the output is safe.

**Fix:** precompile a header allowlist (`Content-Type`, `Copilot-Integration-Id`, `Editor-Version`) or mask any header whose name matches `(?i)auth|token|key|cookie` to `Bearer sk-…xxx`. Move the redaction into a shared helper and reuse `debug.redactSensitiveData`.

### A2. 🔴 Sub-agents truncate the parent's `debug.log` and inherit the parent's goal — ✅ FIXED (`69f5e45`)

`main.go:195-206` exports the resolved config into the process environment so child processes inherit it:

```go
os.Setenv("CODING_AGENT_DEBUG", strconv.FormatBool(cfg.Debug))
os.Setenv("CODING_AGENT_DEBUG_LOG", cfg.DebugLog)   // default "debug.log"
...
if cfg.Goal != "" { os.Setenv("CODING_AGENT_GOAL", cfg.Goal) }
```

`tools/subagent.go:88` then spawns the child with **no environment filtering**:

```go
cmd := exec.CommandContext(childCtx, binaryPath, args...)   // cmd.Env == nil → full parent env
```

Two consequences:

1. `debug/debug.go:47` opens the log with **`os.O_TRUNC`**. The sub-agent process re-opens `debug.log` and **erases everything the parent logged so far**, mid-run. You run with `--debug`, spawn one sub-agent, and the log you were relying on is gone — replaced by the child's session. This makes debugging sub-agent behaviour nearly impossible, and it is invisible to the user.
2. `CODING_AGENT_GOAL` leaks into the child, so a sub-agent started from a `--goal` run silently starts its **own** goal loop instead of doing the single task it was asked to do.

Verified empirically — a child process run with the inherited env wipes the parent's log:

```
$ printf 'PARENT SESSION DATA - useful debug output that must survive\n' > debug.log
$ wc -c debug.log                       # 60
$ CODING_AGENT_DEBUG=true CODING_AGENT_DEBUG_LOG=debug.log ./agent -p "hi"
$ wc -c debug.log                       # 17013  ← parent's line is gone
```

`grep -n "cmd.Env" tools/subagent.go` → no matches, confirming the child inherits the full parent environment. The goal leak is equally direct: `CODING_AGENT_GOAL` → `cfg.Goal` (`config/config.go:600`) → goal-mode branch (`main.go:112`).

**Fix:** build an explicit allowlist of env vars to propagate to sub-agents (model, endpoint, key, tools, read-only, theme, context size) and deliberately exclude `CODING_AGENT_DEBUG*`, `CODING_AGENT_GOAL`, `CODING_AGENT_WEB*`, `CODING_AGENT_MAX_ITERATIONS`. Separately, change the debug log to `os.O_APPEND` and/or derive a per-PID filename (`debug.log` → `debug.<pid>.log`) so concurrent processes never fight over one file.

### A3. 🔴 The test suite is red on a clean machine (non-hermetic test) — ✅ FIXED (`d53b391`)

`agent/agent_skills_test.go:52`

```
--- FAIL: TestNewAgent_SkillsDiscovered
    agent_skills_test.go:52: expected 1 skill, got 3
```

The test seeds one skill in a temp dir and sets `cfg.SkillsDirs`, but `skills.Discover` (`skills/skills.go:116-144`) *also* scans user scope (`$HOME/.agents/skills`, `$HOME/skills`, `$HOME/.claude/skills`) and project scope. On this machine `$HOME/.agents/skills` contains 2 skills → 3 discovered. It fails identically at the parent commit, so it is pre-existing, not a regression.

**Why it matters for debuggability:** a permanently-red suite trains everyone to ignore `go test` output, so real regressions hide in the noise.

**Fix:** `t.Setenv("HOME", t.TempDir())` (and `cfg.WorkingDir`/`Trusted` pinned) inside the test, or assert `catalog.Has("review")` plus `Len() >= 1` instead of an exact count.

### A4. 🔴 No upper bound on tool output entering the LLM context — ✅ FIXED (`737e75b`)

`agent/agent.go:626-627`

```go
// Use full output for LLM context (not truncated)
resultMessage = fmt.Sprintf("Tool '%s' executed successfully:\n%s", tc.Name, result.Output)
```

`read_file` is capped at 20 KB (`tools/read_file.go:14`), but `bash` is not: its schema advertises `max_output_lines: 0` = *"disable truncation and return the full output"* (`agent/tool_defs.go:31-33`, `tools/bash.go:73-75`). A single `cat bigfile`, `go test -v`, or a verbose build can inject megabytes into the context in one step. The only backstop is `compressContext`, which then silently rewrites history — so the visible symptom is *"the agent forgot what it was doing"*, with no error anywhere pointing at the offending tool call.

**Fix:** apply a global byte cap (e.g. 100–200 KB) when converting a `ToolResult` into a context message, with an explicit `[… N bytes elided; re-run with narrower scope]` marker so the model knows the data is incomplete.

---

## B. Silent failures — the main source of "why is this behaving oddly?"

### B1. 🟠 Unrecognised git flags are dropped without any error — ✅ FIXED (`2b13981`)

`tools/git_log.go:62-91`, `tools/git_show.go`, `tools/git_diff.go` — all three build arguments with an allowlist `switch flag { case "stat": … }` and **no `default:` branch**. Anything unmatched vanishes silently.

Verified by calling `buildGitLogArgs` / `buildGitShowArgs` directly:

```
flags = ["--oneline", "oneline", "--stat", "stat", "numstat"]
  git_log  args = [log --max-count=5 --oneline --stat]   ← 3 of 5 inputs vanished
flags = ["numstat", "stat-numstat"]
  git_show args = [show HEAD --numstat]                  ← "numstat" vanished
```

`grep -n "default:" tools/git_log.go tools/git_show.go tools/git_diff.go` → **no matches in any of the three**.

Worse, the tool's own documentation teaches the wrong form. The schema property says `'s' for short format, 'm' for merges…` (bare) while the human-readable description says `'--oneline', '--stat', '--patch'` (dashed). The dashed form never matches the switch.

Concrete traps:
- `git_log(flags=["--oneline"])` → full-format log, `Success: true`, no hint the flag was ignored.
- `git_show(flags=["numstat"])` → dropped (git_show only matches `"stat-numstat"`, while git_diff accepts both `"stat-numstat"` and `"numstat"`).
- Value-taking flags only match degenerate spellings, and then emit a hardcoded value: `case "unified", "unified="` → `--unified=3` (so `unified=10` is dropped, and `unified=` ignores whatever number you typed); `case "stat-width", "stat-width=0"` → `--stat-width=0`.
- `case "grep": if gp.grep != ""` — asking for the `grep` flag without the `grep` parameter is a silent no-op.
- Conflict detection (`hasFlag(gp.flags, "oneline")`, `git_log.go:110-118`) compares bare names, so `--oneline` also bypasses the "conflicting flags" guard.

**Fix:** normalise input once (strip leading `-`), keep the allowlist, and **return an error listing unknown flags** (`unknown flag "numstat" for git_show; did you mean "stat-numstat"?`). Then make the schema and the description list the identical canonical form. The correct pattern already exists in this codebase — `tools/todo.go:156-160` and `tools/schedule.go:574-578` both have a `default:` that names the valid options; the git tools simply omit it.

### B2. 🟠 Multi-character `flags` entries are silently discarded for `grep`/`list_files` — ✅ FIXED (`4bedf7a`)

`tools/utils.go:246-269`

```go
name := strings.TrimPrefix(flagStr, "-")
if len(name) == 1 {
    flags[name] = true      // anything longer is dropped, no error
}
```

Verified directly:

```
parseFlagsParamToMap({flags: ["r", "-i", "--recursive", "recursive"]})
  → map[i:true r:true]        ← 2 of 4 entries kept, no error, no warning
```

`grep(flags=["recursive"])` or `["--recursive"]` → dropped. The result looks like a normal successful search.

**Fix:** collect rejected entries and surface them in the tool result (`ignored flags: recursive`), or map long aliases to their short form.

### B3. 🟠 Bad numeric config values are ignored silently (file + env), but reported for CLI flags — ✅ FIXED (`9d41d14`)

`config/config.go` — CLI path errors properly (`config.go:224-227`: `invalid max-tokens: …`), but the config-file path (`loadConfigFile`, `config.go:409`, blocks at `:437-503`) and the env path (`loadEnv`, `config.go:515`, blocks at `:520-623`) are all of the shape:

```go
if v, err := strconv.Atoi(value); err == nil {
    cfg.MaxTokens = v
}
```

So `CODING_AGENT_MAX_TOKENS=64k`, `max_tokens = "one hundred"`, or `CODING_AGENT_TEMPERATURE=0,5` (comma decimal) keep the default with **zero output**. The user believes their setting is active; the agent behaves as if it isn't. This is one of the most likely ways someone wastes an hour here.

**Fix:** collect parse errors and print `warning: ignoring CODING_AGENT_MAX_TOKENS="64k": invalid integer` (or fail). One helper used by all three sources would also remove the ~30 duplicated blocks.

### B4. 🟠 Lenient parameter parsing turns type errors into misleading "missing parameter" messages — ✅ FIXED (`25928a4`)

- `tools/utils.go:133-147` `parseIntParam` returns the **default** when the value is the wrong type or unparseable. `bash(timeout="5s")` → silently 30 000 ms.
- `tools/schedule.go:822-827` `stringParam` returns `""` for a non-string, so a numeric `path` produces `missing required parameter: path`.

A strict variant already exists (`parseIntParamStrict`, `utils.go:154-170`, added for I-13) and produces the right message — but only `schedule`, `todo` and `move_text` use it. The rest of the package still uses the lenient form.

**Fix:** use the strict helpers everywhere; when a value is ignored, say so in the tool result.

### B5. 🟡 `move_text` silently clamps an out-of-range `source_end` — ✅ FIXED (`e05d866`)

`tools/move_text.go:103-105` — `source_start` out of range errors, but `source_end > len(lines)` is quietly clamped. Asking for lines 10–500 of a 20-line file moves 10–20 and reports success. (The rest of `move_text` is genuinely careful — target written first, rollback on source failure — so this is the one rough edge.)

**Fix:** error, or state the clamp in the success message (`requested 10-500, moved 10-20`).

### B6. 🟡 Errors compared by string — ✅ FIXED (`4380f96`)

`main.go:1004-1005`

```go
if err.Error() == "cancelled" || err.Error() == "EOF" {
```

against `fmt.Errorf("cancelled")` produced at `tui/tui.go:96` and `:183`, and a raw `io.EOF` bubbling up from `os.Stdin.Read`. Any rewording or wrapping of those errors silently changes the exit path (a Ctrl+C at the prompt would become a hard error return).

**Fix:** `var ErrCancelled = errors.New("cancelled")` in `tui`, and `errors.Is(err, io.EOF)`.

### B7. 🟡 `colors.GetColor` returns a reset code for an unknown slot name — ✅ FIXED (`fb89cda`)

`colors/colors.go:86-141` — after two `switch slot` blocks, the function ends with:

```go
    return ColorReset      // unknown slot: no error, no warning
}
```

So `GetColor("inf")`, `GetColor("sucess")`, or a slot that exists in the theme struct but is missing from the switch, emits a bare ANSI reset. Output renders as plain text — visually "slightly off", never an error. Given there are ~10 valid slot names duplicated across two switch blocks plus a `Theme` struct with the same fields, a typo is a realistic and invisible failure.

**Fix:** return `("", false)` or log once per unknown slot; better, drive the lookup from a `map[string]func(*Theme) string` so the theme struct is the single source of truth (the two hand-maintained switch blocks are themselves a drift risk, cf. D1).

---

## C. Observability gaps — ⬜ OPEN — the debug tooling doesn't cover the interesting parts

### C1. 🟠 `debug.log` records none of the things you actually need to debug

`debug/debug.go` exposes 8 log methods; `grep` shows what is actually called:

| Method | Call sites |
|---|---|
| `LogSystemPrompt` / `LogUserMessage` / `LogAssistantMessage` / `LogToolCall` / `LogToolResult` | 1–2 each ✅ |
| `LogStreamingChunk` | **0 — dead code** |
| `LogStreamingComplete` | **0 — dead code** |

Missing entirely: HTTP request/response status, retry attempts, 429/5xx bodies, context-compression events, cancellation, and token-usage deltas. So when a run misbehaves at the API boundary — the most common failure mode — `debug.log` shows a clean conversation and no clue. Meanwhile the retry details only exist as `lastErr` strings on stderr (`inference/inference.go:403-405`).

**Fix:** add `LogAPIRequest(status, attempt, dur, tokens, err)` and `LogCompression(before, after, kept)`; wire or delete the two dead streaming methods.

### C2. 🟠 The debug log reports a fabricated token count

`agent/agent.go:556`

```go
a.debugLogger.LogAssistantMessage(response.Content, response.TokenUsage/2)
```

`TokenUsage` is the **total** (in+out), so the log claims output tokens = total/2 — a made-up number. Anyone reconciling `debug.log` against provider billing will chase a phantom discrepancy. `LogSystemPrompt`/`LogUserMessage` also add *estimated* tokens (`EstimateTokens` is a word-count heuristic, ~1.3×words) into the same `TotalInputTokens` counter that is meant to reflect real usage, so the session summary mixes estimates and authoritative counts with no distinction.

**Fix:** log `CompletionTokens` when the API provides it, and label estimated values explicitly (`tokens≈`).

### C3. 🟠 Redaction rewrites the log's content, so the log lies

`debug/debug.go:259-297` — `redactSensitiveData` runs **17 `regexp.MustCompile` calls per log entry** (uncached; every message, every tool result) and ends with a catch-all:

```go
{`\b[a-zA-Z0-9]{40,}\b`, "[REDACTED_LONG_STRING]"}
```

That pattern matches **every git commit SHA**, every base64 blob, every hash and every long identifier. Debugging a git-tool bug through `debug.log` gives you `[REDACTED_LONG_STRING]` where the commit hash should be. Combined with C1, the log is both incomplete and actively wrong in places.

**Fix:** compile the regexes once at package level; drop the 40-char catch-all or restrict it to contexts that look like key/value assignments.

### C4. 🟠 No panic recovery and no crash artifact

`grep -rn "recover()" --include=*.go` → **0 occurrences**. A panic in the scheduler goroutine (`agent/schedule.go:170,378`), the SSE keep-alive (`webui/handlers.go:115`), or a tool kills the whole process. `tui/tui.go:128,135` does `defer term.Restore`, so the terminal recovers, but there is no crash log, no stack capture in `debug.log`, and no version/context banner — a user reporting "it crashed" leaves no evidence behind.

**Fix:** wrap the top-level run and each long-lived goroutine in a `recover()` that writes the stack + build version (`getBuildVersion()` already exists) to stderr *and* `debug.log`.

### C5. 🟡 Retry policy is invisible and unadjustable

`inference/inference.go:206-207` — `maxRetries: 3`, `retryDelay: 1 * time.Second` are hardcoded; there is no exponential backoff for 5xx and no flag/env to change either. When a provider is flaky, the only signal is three identical 1-second-apart failures.

### C6. 🟡 `io.ReadAll(resp.Body)` errors discarded in the error path

`inference/inference.go:398` and `:418` — `body, _ := io.ReadAll(resp.Body)`; the body is then interpolated into the error message, so a failed read produces `API rate limited (429, attempt 2):  - retrying in 1s` with an empty reason.

---

## D. Drift between schema, description and implementation — ⬜ OPEN

### D1. 🟠 Every tool's parameters are documented twice, and they already disagree

`agent/tool_defs.go` holds a JSON Schema **and** a hand-written `Description` string that re-lists the same parameters (the `Description` is what feeds the system prompt and `--list-tools`). A diff of the two shows live drift:

- `git_log`: schema declares a `grep` property (`tool_defs.go:352`); the `Description` bullet list omits it entirely.
- `git_log` flags: schema examples use bare names (`'s' for short format`), description uses dashed names (`'--oneline'`) — and the implementation accepts neither reliably (see B1).

Two hand-maintained copies of one contract will keep drifting, and the model gets the drifted copy.

**Fix:** generate the `Description` text from the `ParameterSchema` at startup (one function, ~30 lines) and delete the duplicated strings.

### D2. 🟠 `git_show` accepts a parameter that isn't in its schema

`tools/git_show.go:32-35` falls back to `params["reference"]` when `commit` is unset — but `git_show`'s schema declares `commit`, not `reference` (`git_log` declares `reference`). So the same concept has two names across sibling tools, one of which is undocumented. A model (or human) passing `reference` to `git_show` gets a silently different code path.

**Fix:** accept both, document both, and prefer one canonical name across `git_log`/`git_show`/`git_diff`.

### D3. 🟡 Two different parameter-extraction idioms in one package

Older tools (`read_file`, `write_file`, `bash`, `git_*`) inline `params["path"].(string)` with bespoke error strings; the newest file (`tools/schedule.go:798-827`) introduces `stringParam` / `optionalIntParamStrict` / `parseIntParamStrict` — helpers that live in `schedule.go` rather than `utils.go`, so nobody finds them. New tools keep re-inventing the parsing and the error wording.

**Fix:** move the helpers to `tools/utils.go`, migrate every tool to them, and reject unknown parameters so typos (`max_output_line`) fail loudly.

### D4. 🟡 Misleading comment + unclosed context in the signal machinery

`main.go:922-926`

```go
// recreateSignalState creates a fresh root context … Must be called with sigMu held.
recreateSignalState := func() {
    sigMu.Lock()
    defer sigMu.Unlock()
```

The comment says the caller must hold the mutex; the function takes it itself. The call site (`main.go:985-989`) therefore does a lock → unlock → call dance to avoid a self-deadlock. Harmless today (only the main goroutine calls it) but exactly the kind of comment that sends the next debugger down a wrong path. Also, the previous `st.rootCancel` is dropped without being called when the context is recreated — a `context.WithCancel` leak (and the pattern `go vet`'s `lostcancel` check exists for).

---

## E. Robustness & hygiene — ⬜ OPEN

| # | Severity | Location | Issue |
|---|---|---|---|
| E1 | 🟡 | `gofmt -l` | 4 files unformatted: `agent/streaming_test.go`, `tools/grep_test.go`, `tools/utils_test.go`, `webui/sse_test.go`. Add `gofmt -l` to `make vet` so it can't regress. |
| E2 | 🟡 | `config` 49.2%, `main` 29.6% | The least-tested code is the riskiest: config precedence (three sources) and the interactive signal/prompt loop. `-race` reports nothing today precisely because those paths are unexercised. |
| E3 | 🟡 | `main.go:146-225` | `exportResolvedConfigToEnv` writes ~25 env vars into the *current* process. Anything later that re-parses config (tests, sub-agents, a future `--load`) now sees mutated state rather than the user's original config. Prefer passing an explicit `SubagentConfig` struct. |
| E4 | 🟡 | `tools/list_files.go:58-66` | `iInfo, _ := filtered[i].Info()` — a stat failure yields a zero `FileInfo` and silently sorts entries as size 0 / epoch time. |
| E5 | 🟡 | `tools/git_log.go:93-94` | The flag allowlist does stop arbitrary flags reaching `git` (good), but free-text values are appended verbatim: `args = append(args, gp.reference)`. A `reference` (or `commit`) beginning with `-` becomes a flag rather than a revision — argument injection via a value field. Use `git log -- <rev>` / validate the leading character. |
| E6 | 🟡 | `webui/session.go:394` | `SetStreamCallback(broadcastChunk)` is installed for non-streaming runs and never cleared, so the agent keeps holding a reference to a finished session's broadcast closure. |
| E7 | 🟡 | `debug/debug.go:90` | `d.file.Sync()` on **every** `writeLog` call. Fine today (few calls), but it becomes a bottleneck the moment the dead `LogStreamingChunk` is wired up per chunk. |
| E8 | 🟡 | 144 `err != nil` sites, 42 bare `return nil`, 11 `, _ :=` | Most are legitimate, but the `_ :=` discards cluster in exactly the places you'd want truth (error-body reads, `Info()`, `Getwd`). Worth a `errcheck` pass in `make lint`. |
| E9 | 🟡 | `colors/colors.go:31,36` | `currentTheme` and `enabled` are unsynchronised package-level globals read from TUI rendering, agent output and web-UI handlers. Safe *today* — `ApplyTheme` runs once at startup (`main.go:80`) before any goroutine starts — but `webui/session.go:73,340` already carries a per-session theme, so the obvious next step (a runtime `/theme` command) would introduce a data race that `-race` will not catch until something exercises it. Guard with a mutex or make the theme an explicit parameter. |

---

## F. What's done well (keep it)

Worth recording, because several areas are noticeably more careful than the average agent harness and the fixes above shouldn't disturb them:

- **Cancellation correctness** — when a run is cancelled mid-tool-loop, tool results are synthesised for *every* outstanding `tool_call`, so the next request can't 400 on orphaned tool calls. This is a bug most harnesses ship.
- **`move_text` atomicity** (`tools/move_text.go:165-227`) — target written first, source rolled back if its write fails, with the rollback failure reported. Text can't vanish from both files.
- **Truncation is announced, not silent** — `bash`, `grep`, `git_*`, `read_lines`, `subagent` all append an explicit `[output truncated: showing last N of M lines]` marker.
- **The schedule subsystem is solid** — `ScheduleStore` returns snapshots rather than live pointers (no cross-goroutine mutation), `Due()` is atomic re-arm + retire, one-shot `limit` is rejected with a clear message rather than silently forced, and the cron `*/n` day-field bug and the swallowed-delivery bug from `ISSUE_SUMMARY.md` are fixed with regression tests.
- **`WriteFilePreservePerm`** — tools don't strip the execute bit when rewriting files in place.
- **`parseIntParamStrict`** (`utils.go:149-170`) — the right pattern, with a comment explaining *why* lenient parsing produced misleading errors. It just needs to be the default everywhere (B4).
- **No data races** under `go test -race`, and the web UI tracks dropped SSE events (`sub.dropped`) instead of silently losing them.

---

## G. Suggested order of work

1. **A3** — make the suite green again (one `t.Setenv("HOME", …)`). Everything else is easier to land with a working safety net.
2. **A1** — redact the `Authorization` header; delete the false "redact authorization" comment.
3. **A2** — env allowlist for sub-agents + `O_APPEND`/per-PID debug log.
4. **A4** — global cap on tool output into context.
5. **B1–B5** — make every silent drop loud: unknown git flags, dropped `flags` entries, ignored config numbers, lenient param parsing, `move_text` clamp. This is the single biggest improvement to debuggability per line changed.
6. **C1–C4** — log API/retry/compression events, fix the fabricated token count, cache the redaction regexes, add panic recovery with a crash artifact.
7. **D1** — generate tool descriptions from the schema so documentation can't drift again.
8. **E1–E2** — `gofmt` in CI; tests for config precedence and the interactive signal loop.

---

## Appendix — verification commands

```bash
cd implementation
go build ./... && go vet ./...          # clean
go test ./...                            # FAIL: agent.TestNewAgent_SkillsDiscovered
go test ./agent -run TestNewAgent_SkillsDiscovered -v
go test -race ./...                      # no races
gofmt -l .                               # 4 files
go test -cover ./...                     # config 49.2%, main 29.6%
grep -rn "recover()" --include=*.go .    # 0 hits
grep -n "O_TRUNC" debug/debug.go         # line 47
grep -n "Authorization" inference/inference.go   # line 342 (set), 355-360 (logged)
grep -n "cmd.Env" tools/subagent.go              # no matches → child inherits env
grep -n "default:" tools/git_log.go tools/git_show.go tools/git_diff.go   # no matches

# Reproduce A1 (API key leak)
CODING_AGENT_DEBUG_VERBOSE=true CODING_AGENT_API_KEY="sk-SECRET…" \
  CODING_AGENT_API_ENDPOINT="http://127.0.0.1:9/v1" ./agent -p "hi" 2>&1 | grep Headers

# Reproduce A2 (debug.log truncation by a child process)
printf 'PARENT SESSION DATA\n' > debug.log
CODING_AGENT_DEBUG=true CODING_AGENT_DEBUG_LOG=debug.log ./agent -p "hi"
cat debug.log        # parent's line is gone, replaced by the child's session
```

A1, A2, B1 and B2 were each reproduced by running the built binary or calling the builder/parser functions directly from a throwaway test (since removed). A3 reproduces from a clean checkout. The remaining findings are established by the cited code.
