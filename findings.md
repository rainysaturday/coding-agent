# Code Review Findings

> **STATUS: ALL FINDINGS RESOLVED ✅**
>
> All 13 findings below have been fixed and committed. Each fix is verified with
> `go build ./...`, `go vet ./...`, and `go test ./...` (all pass). The individual
> commits are referenced in each section. See the "Resolution Log" section at the
> bottom for a summary of the fixes and their commits.

This document summarizes bugs and quality issues found during a review of the
coding-agent codebase (under `implementation/`). Findings are grouped by
severity. Line numbers refer to the current state of the code.

---

## Critical / Confirmed Bugs

### 1. ✅ `compressContext` can panic with a slice-bounds error (crashes the agent)
- **File:** `implementation/agent/agent_context.go:169`
- **Code:**
  ```go
  summaryMessages := messages[firstUserIdx+1 : len(messages)-preserveCount]
  ```
- **Problem:** If the first `user` message in the conversation appears within the
  last `preserveCount` (3) messages, then `firstUserIdx+1 > len(messages)-preserveCount`,
  which makes `start > end` and triggers a runtime `panic: slice bounds out of range`.
  The existing guard only checks `len(a.context) <= preserveCount+1`, which does not
  prevent this case.
- **Trigger:** A context loaded via `--load` (or a goal-injected context) whose first
  message is not a user message and whose first user message sits near the end of the
  list. This panic is **not** recovered, so it crashes the whole process.
- **Confirmed:** A reproduction test produced
  `panic: runtime error: slice bounds out of range [3:2]` at `agent_context.go:169`.
- **Fix:** Clamp the slice bounds before slicing, e.g.:
  ```go
  end := len(messages) - preserveCount
  if firstUserIdx+1 > end {
      end = firstUserIdx + 1
  }
  summaryMessages := messages[firstUserIdx+1 : end]
  ```
  and guard against an empty summary range.

---

## Robustness / Reliability Issues

### 2. ✅ `subagent` tool has no timeout and no cancellation support
- **File:** `implementation/tools/subagent.go` (`executeSubagent`, `ExecuteSubagent`)
- **Problem:** The tool spawns a subprocess via `cmd.Run()` with no timeout and no
  `context.Context`. If the subagent hangs (a long-running LLM call or a blocking tool),
  the parent agent blocks indefinitely. There is also no way to kill the subprocess when
  the parent request is cancelled, which can leave orphaned processes.
- **Fix:** Add a `ctx` parameter (propagate the parent context), apply a
  `context.WithTimeout`, and use `exec.CommandContext` so cancellation/timeout kill the child.

### 3. ✅ `exportResolvedConfigToEnv` omits several config values that subagents read
- **File:** `implementation/main.go:130-161` (compare `implementation/config/config.go:441-529`)
- **Problem:** The function exports resolved config so spawned subagents inherit the
  same settings, but it misses variables the config parser does read:
  `CODING_AGENT_CONTEXT_SIZE`, `CODING_AGENT_MAX_ITERATIONS`,
  `CODING_AGENT_INITIAL_TOKEN_TIMEOUT`, `CODING_AGENT_CONNECTION_TIMEOUT`,
  `CODING_AGENT_READ_TIMEOUT`, `CODING_AGENT_STREAMING`, `CODING_AGENT_DEBUG`,
  `CODING_AGENT_SUMMARY_ONLY`, `CODING_AGENT_GOAL`.
- **Impact:** If these were provided via CLI flags (not environment), a subagent will fall
  back to defaults — e.g., a parent configured with a large `--context-size` will spawn
  subagents with the default context size, producing inconsistent behavior.
- **Fix:** Export all config values that the parser reads (or document the exclusion).

### 4. ✅ Vision request sends the full tool list and agent system prompt
- **File:** `implementation/agent/agent.go:683` (`handleViewImage`)
- **Problem:** The image-analysis request goes through `InferenceRequest`, which appends
  *all* registered tool definitions (from `a.inference.SetTools(...)`) and the full agent
  system prompt (which describes tools/persona). This wastes tokens on every image, and
  some endpoints/models reject requests that include a `tools` array when they don't
  support function calling — turning a valid image analysis into a hard failure.
- **Regression vs. June 30 build (confirmed):** At commit `47fc764` (June 30),
  `handleViewImage` called the vision request with an **empty** system prompt
  (`systemPrompt = ""`), so `buildMessages` emitted **no system message** and the request
  body was just `[user(image)]` plus the tool definitions. Commit `e439ec1` (Aug 4,
  "Vision requests not in token accounting or given a system prompt") changed it to pass
  `a.systemPrompt`, so the request now prepends the **full coding-agent system prompt**
  (~2000+ tokens describing all tools, the persona, and verification requirements) to every
  vision call. This is the most likely cause of the "view_image stopped working after the
  June 30 build" reports: a large, tool-centric system prompt can push the image out of the
  model's context budget or cause a pure vision model to attempt tool calling / emit
  tool-call JSON instead of describing the image.
- **Confirmed by test:** A request-body dump of `handleViewImage` showed
  `has system msg: true`, `has tools array: true`, `has image_url: true` — i.e. the request
  carries the full system prompt AND all 14 tool definitions alongside the image.
- **Fix:** Perform the vision call with an empty tool set (a dedicated "no-tools" request
  path) and a minimal/empty system prompt, so the request body is essentially
  `[user(image)]` (as it was on June 30).

### 5. ✅ `buildURL` can produce a doubled `/v1` path segment
- **File:** `implementation/inference/inference.go:236-244`
- **Problem:** `buildURL` always appends `/v1/chat/completions` (or the Copilot/Models
  variant). If a user sets an endpoint that already ends in `/v1` (e.g.
  `https://api.openai.com/v1`), the request goes to
  `https://api.openai.com/v1/v1/chat/completions`. The existing test
  (`copilot_test.go:99-109`) *documents* this behavior as expected, but it is a footgun
  that silently breaks non-default base URLs.
- **Fix:** Strip a trailing `/v1` from the endpoint before appending, or normalize the
  base URL and clearly document that callers must omit the version prefix.

### 6. ✅ `GITHUB_TOKEN` fallback only applies to Copilot, not GitHub Models
- **File:** `implementation/config/config.go:537-543`
- **Problem:** The automatic API key from `GITHUB_TOKEN` is only applied when the endpoint
  is a Copilot URL. The code elsewhere (inference.go:392) actively recommends
  `https://models.github.ai` as an alternative that accepts PAT/OAuth tokens, so
  `GITHUB_TOKEN` should also be honored for the GitHub Models endpoint.
- **Fix:** Extend the condition to `IsGitHubCopilotEndpoint(cfg.APIEndpoint) || IsGitHubModelsEndpoint(cfg.APIEndpoint)`.

---

## Quality / Maintainability Issues

### 7. ✅ `defer resp.Body.Close()` inside the retry loop
- **File:** `implementation/inference/inference.go:409`
- **Problem:** `defer` is function-scoped, so on each retry iteration a new deferred close
  is registered. It works today only because the success path returns immediately, but it
  is fragile and easy to break if the loop is restructured. Also, error paths close the body
  manually while the success path relies on `defer` — inconsistent.
- **Fix:** Close the body explicitly on the success path (or use a helper) rather than
  `defer` inside the loop.

### 8. ✅ Assistant messages with only tool calls serialize `content: ""`
- **File:** `implementation/inference/inference.go:97-115` (`Message.MarshalJSON`)
- **Problem:** For an assistant message that only contains `tool_calls` (no text), `Content`
  is `""` and is emitted as `"content": ""` instead of `null`/omitted. Several
  OpenAI-compatible servers expect `content: null` for tool-call-only assistant messages and
  may reject or misbehave on an empty string.
- **Fix:** Emit `null` (or omit `content`) when there is no content and no content parts.

### 9. ✅ `buildMessages` shallow-copies `ContentParts`
- **File:** `implementation/inference/inference.go:440-451`
- **Problem:** The deep-copy comment covers only `ToolCalls`; `ContentParts` is a slice that
  is shallow-copied (`normalized := *msg`), sharing the backing array. It is currently safe
  because nothing mutates the parts, but the no-mutation guarantee is undocumented for this
  field and any future mutation would corrupt the caller's message.
- **Fix:** Document the invariant or deep-copy `ContentParts`.

### 10. ✅ Context-size delta undercounts the assistant tool-call message
- **File:** `implementation/agent/agent_context.go:101-116`; `implementation/agent/agent.go:400,522`
- **Problem:** `getActualContextSizeUnlocked` adds a delta only for tracked `tool` messages
  added after the last API response. The assistant message containing the `tool_calls`
  (appended at agent.go:400) is not tracked, so between API calls the estimate is slightly
  low. This can delay context compression near the 80% threshold.
- **Fix:** Track/count the assistant tool-call message in the same delta computation, or
  include it when building the post-response estimate.

### 11. ✅ Token split heuristic `TokenUsage / 2` for input/output
- **File:** `implementation/agent/agent.go:414-424` and `698-705` (`handleViewImage`)
- **Problem:** When the API returns only a total token count, the code assumes input and
  output are each half of the total. This is a rough heuristic that can misreport usage for
  models with very asymmetric prompt/completion sizes (and is duplicated in two places).
- **Fix:** Centralize the fallback logic in one helper and, where possible, request
  per-token usage from the API.

### 12. ✅ Very large image limit for `view_image` + vision
- **File:** `implementation/tools/view_image.go` (`maxImageSize = 20MB`)
- **Problem:** A 20 MB image becomes a ~27 MB base64 string sent inline in the vision
  request. Many inference servers enforce request-body size limits, so a large-but-allowed
  image can be rejected at request time. The size limit is generous compared to typical
  vision APIs (which often downscale / limit to a few MB).
- **Fix:** Consider a smaller default limit or downscaling images before base64 encoding,
  and surface a clearer error if the request is too large.

### 13. ✅ No handling of `message.refusal` / refusal content
- **File:** `implementation/inference/inference.go` (`handleResponse`, `handleStreamResponse`)
- **Problem:** If a model returns a `refusal` (e.g., content-policy refusal), the code does
  not surface it; the refusal text is silently dropped and the response appears empty.
- **Fix:** Parse `refusal` and propagate it into the `Response` so the UI/agent can display
  a meaningful message.

---

## Verification Notes

- `go build ./...`, `go vet ./...`, and `go test ./...` all pass on the current tree.
- Finding #1 was verified with a temporary reproduction test that triggered
  `panic: slice bounds out of range [3:2]` in `agent_context.go:169`.
- The vision path (`view_image` → `handleViewImage` → `InferenceRequest`) was verified
  end-to-end against a mock server and works in both non-streaming and streaming modes; the
  image base64 encoding and message marshaling are correct.
- A request-body dump confirmed the vision regression in finding #4: the request carries
  the full agent system prompt and all 14 tool definitions alongside the image (June 30
  sent only the image with no system message).
- The `view_image` mechanism itself (image decoding, base64, `image_url` content part,
  text-delta handling) is intact; the behavioral regression vs. June 30 is the added system
  prompt (commit `e439ec1`, Aug 4), described in finding #4.
- Temporary test files used for verification (`zz_vision_*_test.go`) were removed after
  confirming the findings.

---

## Resolution Log

All findings have been fixed and committed (one commit per finding). Each commit
was verified with `go build ./...`, `go vet ./...`, and `go test ./...`.

| # | Fix summary | Commit |
|---|-------------|--------|
| 1 | Clamp summary slice bounds in `compressContext` to prevent panic | `1874124` |
| 2 | Added timeout + cancellation to the subagent tool (`exec.CommandContext` + `context.WithTimeout`) | `df9b90d` |
| 3 | Exported all resolved config values to subagents (context size, timeouts, streaming, debug, goal, etc.) | `e4d2ed3` |
| 4 | Vision requests now sent without tools and without the agent system prompt (dedicated no-tools path) | `b7d8d7b` |
| 5 | `buildURL` normalizes base URL to avoid a doubled `/v1` path (test updated) | `5c18f3a` |
| 6 | `GITHUB_TOKEN` fallback now also applies to the GitHub Models endpoint (test added) | `c4ea7d3` |
| 7 | Response body closed explicitly (no `defer` in the retry loop) | `600bb36` |
| 8 | Empty message content serialized as `null` instead of `""` for tool-call-only messages (test added) | `e1e00e2` |
| 9 | `ContentParts` now deep-copied in `buildMessages` | `d08a804` |
| 10 | Centralized token accounting helper; assistant tool-call message now counted in the context-size delta | `953147e` |
| 11 | Token-split fallback logic centralized into one helper (`recordTokenUsageUnlocked`) | `953147e` |
| 12 | Lowered `view_image` size limit and clarified the error | `928ce23` |
| 13 | Model refusals surfaced via a new `Refusal` field (streaming + non-streaming, agent UI) | `da87c16` |

### Verification

- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./...` — PASS
