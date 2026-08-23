# Code Review — Full Diff `origin/master` → `HEAD`

> **Reviewed:** 78 commits, 65 files changed, +3273 / −3167 lines
> **Range:** `origin/master..HEAD` (merge-base `47fc764`, HEAD `39b928f`)
> All paths below are relative to the repository root; the Go code lives under
> `implementation/` on both sides of the diff.

## Method

1. Enumerated every commit in `origin/master..HEAD` (`git log --oneline`, 78 commits)
   and inspected each one-by-one with `git show`.
2. For every code change, re-read the **final state** of the affected file at HEAD to
   confirm the fix is actually present and correct (not just present in the diff).
3. Checked provenance of each candidate issue against the merge-base
   (`git show 47fc764:<file>`) to distinguish **introduced-by-branch** bugs from
   **pre-existing** bugs that are still live in HEAD.
4. Reproduced each confirmed bug with temporary Go tests against the current tree
   (all removed after confirmation).
5. Ran `go build ./...`, `go vet ./...`, `go test ./...` — **all pass**.

## Overall Assessment

The branch is a large batch of fixes for a prior review (13 findings), plus refactors
(helper extraction M11), a `--tools`/`--list-tools` feature, and docs. Most fixes are
correct and verified: slice-bounds clamp in `compressContext` (`1874124`), subagent
timeout/cancellation (`df9b90d`), full config export to subagents (`e4d2ed3`), vision
requests without tools/system prompt (`b7d8d7b`), `buildURL` normalization (`5c18f3a`),
`GITHUB_TOKEN` fallback for GitHub Models (`c4ea7d3`), explicit body close (`600bb36`),
`content: null` marshaling (`e1e00e2`), `ContentParts` deep copy (`d08a804`), centralized
token accounting (`953147e`), scanner buffer increase (`a2dc3c3`), permission-preserving
writes (`3adc3e9`), host-based endpoint detection (`0b04a39`), rune-safe truncation
(`5e3a134`), `--initial-token-timeout` (`e3d7259`), and the M11 helper extractions
(`0711cf0`, `cb62fba`, `5ed7937`, `56af63b`, `0e91d05`, `c5fc5ca`) which are
behavior-preserving.

However, the review found **10 issues**, including one **incomplete fix** (N1), one
**behavioral regression** (N2), a **live correctness bug in context compression** (N3),
and a **live functional bug in the grep tool** (N4). Details below.

---

## Issues Found

### N1. High — Streaming refusal is still dropped in the common single-line SSE case (incomplete fix)

- **File:** `implementation/inference/stream.go`
- **Introduced by:** `da87c16` ("surface model refusals instead of silently dropping them")
  — the fix is incomplete relative to its own commit message.
- **Problem:** `da87c16` added refusal handling to the non-streaming path
  (`handleResponse`, `inference/inference.go:543/605`) and to
  `streamState.processDelta` (`inference/stream.go:186-232`). But
  `handleStreamResponse` has **two** delta-accumulation paths:
  - the **buffered** multi-line-JSON path calls `ss.processDelta(...)`
    (`stream.go:354`) — refusal handled ✅;
  - the **inline** single-line-JSON path accumulates `delta.Content`,
    `delta.Reasoning`, `delta.ReasoningContent`, and `delta.ToolCalls` directly
    (`stream.go:400-448`) but **never reads `delta.Refusal`** ❌.

  Standard SSE delivers one `data: {json}` per line, so the inline path is the common
  case. There, `Response.Refusal` is left empty, and the agent cannot show the
  "[Model refusal]" message the fix added (`agent/agent.go:398-401`) — the refusal is
  silently dropped again.
- **Reproduced:** a single-line SSE chunk carrying `delta.refusal` yields
  `resp.Refusal == ""` on the current tree.
- **Fix:** In the inline path, mirror `processDelta`: append `delta.Refusal` to
  `ss.fullRefusal` and emit a `StreamingChunk` for it — or route inline deltas through
  `processDelta` so the two paths can't diverge again. Add a streaming-refusal test.

### N2. High — `extractSummary` returns the subagent's reasoning instead of its final answer (regression)

- **File:** `implementation/tools/subagent.go` (`extractSummary`, lines 166-273)
- **Introduced by:** `4d06709` (M5 "Improve extractSummary with more markers and paragraph parsing").
- **Problem:** The subagent is invoked with `--quiet --summary-only --no-stream`
  (`subagent.go:44-50`), and in `main.go`'s `outputResult` the **Quiet branch is
  evaluated before SummaryOnly** (pre-existing ordering, `main.go:354-367`), so the
  child's stdout is: `[Reasoning] <reasoning lines> <blank> <final answer>` (ANSI-colored).
  The rewritten "Strategy 2" paragraph logic scans backwards and appends **every**
  block of 3+ consecutive significant lines (lines 225/237/250), not just the last one:
  - if the **final answer is < 3 lines**, the only 3+ line block is the reasoning block,
    so the "summary" handed to the parent is **the reasoning — the actual answer is dropped**;
  - if the answer is 3+ lines, the returned summary is **reasoning + answer merged**.

  The parent agent therefore receives the subagent's internal reasoning rather than
  (or mixed with) its conclusion.
- **Reproduced:** for input `[Reasoning]\n<3 reasoning lines>\n\nDone.` the function
  returns the 4-line `[Reasoning]` block and not `Done.`; for a 3-line final answer it
  returns reasoning + answer concatenated.
- **Fix:** Strategy 2 should return only the **last** substantial block (stop at the
  first 3+ line block reached scanning from the end), and/or strip the `[Reasoning]`
  section and ANSI escapes before extraction. Add a unit test covering the actual
  `--quiet` output shape.

### N3. High — `compressContext` leaves an orphaned tool message at the head of the preserved window

- **File:** `implementation/agent/agent_context.go` (`groupAssistantToolMessages`, lines 322-363)
- **Provenance:** **Pre-existing** (byte-identical at merge-base `47fc764`), but live in
  HEAD and directly adjacent to this branch's compression work: `4d06709` (M4) added an
  orphan-tool filter for the **summary** range (`agent_context.go:227-242`) but not for
  the **preserved** window.
- **Problem:** `preserved = messages[len-3:]`. When that window **starts** with a `tool`
  message — the *normal* mid-loop shape, since the context then ends
  `…assistant(tool_calls), tool` and `len(messages)-3` lands on a `tool` result — that
  tool's assistant message (carrying the matching `tool_calls`) is in the
  summarized/dropped range. `groupAssistantToolMessages` seeds `currentGroup` with
  `messages[0]` (line 330) and only skips a tool message that *follows* a non-assistant
  message (lines 340-344); a **leading** tool message is kept as a one-element group.
  The rebuilt context becomes `[user, summary, tool, assistant(tc), tool]` — an orphaned
  tool result with no preceding assistant `tool_calls`, which OpenAI-compatible APIs
  reject with a 400 on the next call.
- **Reproduced:** `groupAssistantToolMessages([tool, assistant, tool])` returns the
  leading `tool` unchanged (confirmed by test on the current tree). The existing test
  (`TestGroupAssistantToolMessages`) only covers nil/empty input.
- **Fix:** Drop (or re-home) a leading tool message whose assistant is not in the
  preserved window — symmetric to the summary-side filter added in M4.

### N4. Medium — `grep` flags are silently dropped (documented flags never applied)

- **File:** `implementation/tools/utils.go` (`parseFlagsParamToMap`, lines 212-234)
- **Provenance:** **Pre-existing** at merge-base, still live in HEAD (unchanged by this
  branch, but it fully defeats the documented tool behavior).
- **Problem:** The parser only accepts flags with `len(flagStr) == 1` (lines 219/226).
  But the grep tool schema (`agent/tool_defs.go:307`) and the system prompt document the
  flags as `-n`, `-i`, `-r`, `-f`, `-a`, `-c`, `-v`, `-l` — all length 2, dash-prefixed.
  The model emits e.g. `["-n","-r"]`, **every** flag is discarded, and grep runs without
  line numbers, case-insensitivity, and — critically — recursion (`-r`), so it never
  searches subdirectories. The grep implementation (`tools/grep.go`) fully supports all
  of these flags; only the parser drops them.
- **Reproduced:** `parseGrepParams({flags: ["-n","-r"]})` yields all flag keys `false`.
- **Fix:** Accept the documented `-x` forms (strip a leading `-`) or change the schema/
  prompt to single-char flags; keep the two in sync. Add a parser unit test.

### N5. Low — Subagent output carries ANSI color codes into the parent's context

- **Files:** `implementation/main.go` (`outputResult` Quiet branch, ~354-361); `implementation/colors/colors.go` (`GetColor`)
- **Provenance:** Pre-existing; made more likely to surface by the N2 rewrite.
- **Problem:** There is no TTY/`NO_COLOR` detection anywhere — `colors.GetColor` always
  returns ANSI escapes. A subagent's piped stdout (captured by `executeSubagent`)
  therefore contains escape sequences around `[Reasoning]` and the final answer, and
  `extractSummary` passes that text (escapes included) back into the parent agent's
  message history as a tool result — injecting `\x1b[…m` noise into the LLM context.
- **Fix:** Disable colors when stdout is not a TTY (or in `--quiet`/`--summary-only`
  piped mode), and/or strip ANSI escapes in `extractSummary`.

### N6. Low — `insert_lines` splits the inserted text inconsistently with `splitLines`

- **File:** `implementation/tools/insert_lines.go` (~line 39)
- **Provenance:** Pre-existing; `d9701af` ("Use shared splitLines() helper in
  insert_lines.go") only converted the **existing-content** read to `splitLines`,
  leaving the **inserted-text** parse as a raw `strings.Split(insertLines, "\n")`.
- **Problem:** `splitLines` strips a trailing empty element; the raw split does not. If
  the model passes text ending in `\n` (e.g. `"a\nb\n"`), an extra empty line is
  inserted, inconsistent with how the file's existing content is handled.
- **Fix:** Use `splitLines` (or equivalent trailing-empty strip) for the inserted text.

### N7. Low — `replace_text` with `count=0` replaces all occurrences but reports "0 time(s)"

- **File:** `implementation/tools/replace_text.go` (~lines 78-90)
- **Provenance:** Pre-existing at merge-base.
- **Problem:** The branch is `if count < 0 || count > totalOccurrences { ReplaceAll }
  else { strings.Replace(..., count) }`. For `count == 0` the condition is false, so it
  calls `strings.Replace(..., 0)` — which replaces **all** occurrences — but sets
  `replacementsMade = count` = 0. The file is fully rewritten while the tool reports
  "Replaced … 0 time(s)".
- **Fix:** Treat `count == 0` as "replace all" (or reject it) and set
  `replacementsMade` to the actual number.

### N8. Low — `read_lines` no longer bounds output by byte size (size guard removed)

- **File:** `implementation/tools/read_lines.go` (`maxReadLinesBlock`, lines 12, 66)
- **Introduced by:** `979b140` ("remove file size limit, add line block limit").
- **Problem:** The old `fileInfo.Size() > maxReadFileSize` guard was replaced by a limit
  on the **number of requested lines** (5000). `os.ReadFile` still loads the *entire*
  file into memory regardless of the requested range, and the returned output is not
  byte-truncated. A file with ≤5000 lines but very long lines (minified/JSON, wide logs)
  can return an unbounded number of bytes, blowing up memory and the model context. This
  is now asymmetric with `read_file`, which still caps at 20 KB (`read_file.go:14`).
- **Fix:** Keep a byte-size cap on the returned output (and/or the file read), or
  document the trade-off explicitly.

### N9. Low — `TodoStore.List()` returns the live internal slice

- **File:** `implementation/tools/todo.go` (lines 77-82)
- **Provenance:** Pre-existing.
- **Problem:** `List()` locks, then returns `ts.items` directly — the same slice the
  store owns. A caller that appends to or mutates the returned slice does so without
  holding the lock, racing subsequent store operations and potentially corrupting the
  store's backing array.
- **Fix:** Return a copy (or expose items read-only).

### N10. Low — Compression summary prompt drops tool-call information

- **File:** `implementation/agent/agent_context.go` (lines 245-248)
- **Provenance:** Pre-existing.
- **Problem:** The summary prompt is built from `msg.Content` only. Assistant messages
  that carry `tool_calls` have empty `Content`, so the summarizer sees a bare
  `assistant: ` line with no indication that a tool was called (or which tool), while the
  corresponding `tool` result is included. The model summarizing the history loses the
  tool-call structure.
- **Fix:** Include tool-call names/arguments in the summary serialization for assistant
  messages.

---

## Commits Verified — No Issues Found

Spot-checked in final state (behavior-preserving or correct fixes):

| Commit(s) | Change | Verdict |
|---|---|---|
| `1874124` | Clamp summary slice bounds in `compressContext` | Correct clamp present (agent_context.go:220-225); related gap documented as N3 |
| `df9b90d` | Subagent timeout + cancellation (`exec.CommandContext`, 5 min) | Correct; timeout/cancel surfaced in error text |
| `e4d2ed3`, `6d4f285` | Export resolved config to subagent env | `exportResolvedConfigToEnv` now covers all parsed config values |
| `b7d8d7b`, `e439ec1` | Vision requests without tools/system prompt | `InferenceRequestNoTools` path used; no-tools + empty system prompt verified |
| `5c18f3a` | `buildURL` normalization | Correct for default/Copilot/Models and `/v1`-suffixed endpoints |
| `c4ea7d3` | `GITHUB_TOKEN` fallback for GitHub Models | Condition extended correctly, endpoint-gated |
| `600bb36` | Explicit body close in retry loop | Closes present at lines 390/410/458; no `defer` in loop |
| `e1e00e2` | `content: null` for tool-call-only messages | `MarshalJSON` verified; test covers omission |
| `d08a804` | Deep-copy `ContentParts` in `buildMessages` | Present |
| `953147e` | Centralized token accounting | `recordTokenUsageUnlocked` centralized; assistant tool-call msg counted |
| `0b04a39` | Host-based endpoint detection | `isHostOrSubdomain` correct (scheme/path ignored, malformed → false) |
| `a2dc3c3` | Scanner buffer 10 MB | Present in `handleStreamResponse` |
| `3adc3e9`, `be90a84` | Permission-preserving writes + shared constants | `WriteFilePreservePerm` used by write/insert/replace/move |
| `5e3a134` | Rune-safe truncation | `TruncateRunes` correct |
| `979b140` | read_lines line-block limit | Works as described; size-guard trade-off documented as N8 |
| `43aa976`, `9cd0cfc`, `c79fd0a` | `--tools` / `--list-tools` / validation | `ValidateToolNames` rejects unknown names; read-only still enforced at dispatch |
| `d5284e4` | One-shot duplicate output | Quiet/SummaryOnly/verbose branches mutually exclusive in output |
| `1f70a0a` | sigChan race | Done-channel refactor correct |
| `2c7db17` | DumpContext filename | Consistent naming |
| `4c436ce` | First-user-message handling in compression | Correct search + fallback |
| `6453d3a`, `0ddfc7f`, `fad876b` | git_log/git_show/git_diff param fixes | `commit`/`reference`/`grep` params wired in schema and parser |
| `982b049` | replace_text re-search fix | `strings.Replace` n-occurrences correct; edge case documented as N7 |
| `a470dfb`, `cd4e77c`, `c2f9fdc` | System prompt tool numbering / display | Consistent numbered list; no redundant params |
| `0198cf1`, `ad4ebe9` | Tool definition/description dedup | Single source in `tools` package |
| `daa32a7` | `parseFlagsParam` helper extraction | Behavior-preserving (parser limitation documented as N4) |
| `0711cf0`, `cb62fba`, `5ed7937`, `56af63b`, `0e91d05`, `c5fc5ca` | M11 helper extraction (git_log/show/diff, move_text, list_files, grep) | Behavior-preserving refactors |
| `b96eb68` | Fresh `sseChunk` per iteration | Present; prevents stale-field leaks |
| `c89a6f1`, `3a7577e`, `37e9e6f`, `6cde7ec` | strconv / dead code / gofmt | Trivial, verified |
| `b1fea26` | `isContextLimitError` simplification | Correct |
| `e3d7259` | `--initial-token-timeout` | Flag, config, env export, and inference wiring all present |
| `928ce23` | `view_image` size limit 20 MB → 10 MB + clearer error | Correct; `maxImageSize = 10 MB` with explicit error verified |
| `451fb5a` | Context-size callback outside mutex | Callback dispatched after unlock |
| `bfa1229` | `agent.WrapError` centralization | Single classification point |
| `49d369e`, `19587d4`, `54846d0`, `a51a69f`, `d4c3396` | write_file output, constants, misc fixes | Correct |
| Docs-only (`39b928f`, `776b0d8`, `ec6a6c3`, `d5686ab`, `40897c4`, `a51d604`, `176c6a7`, `f96a3e5`, `12a2e09`, `76ad3ac`, `f7a1cc3`, `769bc2c`, `ab03fcb`, `bcf33dc`, `d535f53`) | findings.md status updates | Consistent with the code state |

## Verification

- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./...` — PASS (all packages)
- N1, N2, N3, N4 each reproduced with temporary tests on the current tree (tests removed
  after confirmation; tree left clean).

## Summary

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| N1 | High | Streaming refusal dropped in inline SSE path — `da87c16` fix incomplete | Open |
| N2 | High | `extractSummary` returns reasoning instead of final answer — regression from `4d06709` | Open |
| N3 | High | Orphaned leading tool message after compression (M4 only fixed summary side) | Open |
| N4 | Medium | Documented grep flags silently dropped by `len==1` parser | Open |
| N5 | Low | ANSI color codes leak from subagent output into parent context | Open |
| N6 | Low | `insert_lines` inserted-text split inconsistent with `splitLines` | Open |
| N7 | Low | `replace_text count=0` replaces all but reports 0 | Open |
| N8 | Low | `read_lines` byte-size guard removed; output bytes unbounded | Open |
| N9 | Low | `TodoStore.List()` returns live internal slice | Open |
| N10 | Low | Compression summary prompt omits tool-call info | Open |

The same issues are also tracked in `findings.md` (section "New Findings — Review of
the Fix Commits"), where they are documented alongside the 13 previously resolved
findings.
