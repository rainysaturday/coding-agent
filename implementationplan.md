# Implementation Plan: Web UI for the Coding Agent CLI

## 1. Overview

Add a **Web UI** to the existing Go coding-agent harness that lets users perform the
**same interactions currently available in the terminal TUI**, but from a browser.
The web UI must be **beautiful** while satisfying the project's hard constraint of
**zero external dependencies** (stdlib-only Go, no CDN, no frontend framework, offline).

### What the terminal TUI currently offers (must be replicated)

| Terminal interaction | TUI implementation | Web UI equivalent |
|---|---|---|
| Enter a prompt / request | `tui.Prompt()` raw-mode line input | Chat input box, Enter to send |
| Stream assistant tokens live | `StreamChunkWithType` (Normal/Reasoning/Goal/Compression) | SSE push of tokens to the chat pane |
| Show reasoning (dim) | `StreamReasoningChunk` | Collapsible "Reasoning" block, dim styling |
| Show goal-mode messages (magenta) | `StreamGoalChunk` | Highlighted goal block |
| Show tool-call progress w/ params | `StreamChunkWithType` `[Tool Call]` in-place update | Live-updating "tool call" card showing tool + params |
| Context-size indicator w/ warning levels | `printContextSizeInternal` ✓/⚠/⚠⚠/⚠⚠⚠ | Top status bar / progress ring colored green→yellow→red |
| Runtime statistics | `/stats` → `DisplayStats` | `/stats` command + live stats panel |
| Input history navigation | Arrow keys / Ctrl-P/N | Up/down arrows in input, history dropdown |
| Cancel current operation | Ctrl+C during run | Cancel button / Escape |
| `/stats` command | `handleInteractiveCommand` | Slash command + button |
| `/clear` (clear output) | `ClearOutput` | Clear button |
| `/clear-history` | `ClearHistory` | Clear-history button |
| `/read-only` (toggle) | `SetReadOnly(true)` | Toggle switch |
| `/compress` (manual compression) | `CompressContext` | Compress button |
| `/goal <text>` / `/goal-off` | `SetGoal` / `ClearGoal` | Goal input + toggle |
| `/dump` (dump context) | `DumpContext` | Dump button / command |
| Theme support | `colors` package themes | CSS variables mapped from theme palette |
| Auto context dump on exit | `defer DumpContext` | (server-side, same behavior) |

---

## 2. Constraints & Design Principles

1. **Zero external dependencies (Requirement 024):**
   - Backend: **Go stdlib only** — `net/http`, `encoding/json`, `net/url`, `embed`,
     `sync`, `context`, `io`, `fmt`, `log`. No router, no websocket lib, no framework.
   - Streaming transport: **Server-Sent Events (SSE)** implemented with plain
     `net/http` + `http.Flusher` (stdlib). SSE is a one-way server→client stream —
     sufficient because the agent pushes tokens *to* the browser, while the browser
     only sends discrete commands (prompt / slash commands / cancel) which are plain
     `fetch()` POST requests. No WebSocket library is needed.
   - Frontend: **Embedded static assets** via `//go:embed` compiled into the binary.
     Vanilla **HTML/CSS/JS only** — no React/Vue, no CDN, no build step, no node/npm.
     Everything works offline and the binary is fully self-contained.
2. **Cross-platform:** `embed` + `net/http` work identically on Linux/macOS/Windows.
3. **Reuse the existing core:** The web UI is a **new front-end over the existing
   `agent`, `inference`, `tools`, and `colors` packages**. No changes to agent core
   logic beyond adding small, safe accessors where needed (e.g., exposing the resolved
   config/theme for the UI, and a cancellation handle).
4. **Single-user, local:** The web server binds to `127.0.0.1` by default (optionally
   configurable). One agent instance per active session, serialized by a mutex
   (the agent already uses an internal mutex for its state, but runs are serialized
   per session to preserve conversation context).

---

## 3. Architecture

```
                    ┌──────────────────────────────────────────────────┐
                    │                    coding-agent                  │
                    │                                                  │
 Browser  ──HTTP──▶ │  webui package (NEW)                             │
 (vanilla           │  ┌───────────────────────────────────────────┐  │
  SPA)              │  │ http.Server (net/http, stdlib)            │  │
                    │  │  • static assets (go:embed index.html,    │  │
                    │  │    app.js, styles.css)                    │  │
                    │  │  • REST handlers                          │  │
                    │  │     POST /api/chat        (send prompt)   │  │
                    │  │     POST /api/command     (/stats etc.)   │  │
                    │  │     POST /api/cancel      (cancel run)    │  │
                    │  │     GET  /api/state       (context/stats) │  │
                    │  │  • SSE handler                             │  │
                    │  │     GET  /api/events      (stream push)   │  │
                    │  └───────────────┬───────────────────────────┘  │
                    │                  │  calls                       │
                    │                  ▼                              │
                    │  ┌───────────────────────────────────────────┐  │
                    │  │ SessionManager (NEW)                      │  │
                    │  │  • session registry (id → *Session)       │  │
                    │  │  • per-session *agent.Agent               │  │
                    │  │  • per-session SSE hub (chan of events)   │  │
                    │  └───────────────────┬───────────────────────┘  │
                    │                      │                          │
                    │                      ▼                          │
                    │  ┌───────────────────────────────────────────┐  │
                    │  │ existing: agent / inference / tools /     │  │
                    │  │ colors / config packages (reused as-is)   │  │
                    │  └───────────────────────────────────────────┘  │
                    └──────────────────────────────────────────────────┘
```

### Data flow for a chat turn

1. Browser opens `GET /api/events` → registers an SSE subscription on the session hub.
2. User types a prompt → browser `POST /api/chat` `{prompt, session}`.
3. Server looks up (or creates) the session, starts a goroutine:
   `session.agent.RunStream(ctx, prompt, callback)`.
4. The stream `callback` receives `inference.StreamingChunk` and publishes a JSON
   `chunk` event to the session hub → SSE flushes it to the browser in real time.
5. On completion, server publishes a `result` event (`agent.Result` + `Stats`).
6. On error/cancel, server publishes an `error`/`done` event.

---

## 4. New Files & Changes

### 4.1 New package `implementation/webui/`

| File | Purpose |
|---|---|
| `webui/server.go` | `http.Server` setup, route registration, `Serve()` |
| `webui/session.go` | `Session` type: owns an `*agent.Agent`, run mutex, SSE hub, history, current run state |
| `webui/handlers.go` | HTTP handlers for `/api/chat`, `/api/command`, `/api/cancel`, `/api/state`, `/` |
| `webui/sse.go` | SSE writer helpers (`writeEvent`, flush, keep-alive ping) and event types |
| `webui/events.go` | JSON event structs shared with the frontend (`ChunkEvent`, `ResultEvent`, `StatsEvent`, `StateEvent`, …) |
| `webui/static.go` | `//go:embed` of `static/` assets |
| `webui/static/index.html` | Single-page app shell |
| `webui/static/app.js` | Vanilla JS: chat renderer, SSE client, slash commands, history, cancel |
| `webui/static/styles.css` | Vanilla CSS: modern, responsive, theme-aware |
| `webui/server_test.go` | httptest-based tests (see §8) |

### 4.2 Changes to existing files

| File | Change |
|---|---|
| `implementation/config/config.go` | Add `Web`, `WebPort`, `WebAddr` fields + `--web`, `--web-port`, `--web-addr` flags |
| `implementation/main.go` | Branch to `webui.Serve(cfg)` when `cfg.Web` is set (mirrors the interactive-mode setup: create agent, set context-size callback, debug logger close, auto dump on exit) |
| `implementation/main.go` | Export new web flags in `exportResolvedConfigToEnv` (so subagents stay consistent) |
| `implementation/README.md` | Document web UI usage + new flags |
| `requirements/024-zero-external-dependencies.md` | Add explicit acceptance criterion that the web UI uses stdlib `net/http` + `embed` only (no websocket lib, no CDN) |
| `requirements/046-web-ui.md` | **New requirement** specifying the web UI feature (created below) |

### 4.3 Optional small additions to the agent package (safe, additive)

- `agent.Agent.GetContextSize()` / `GetMaxContextSize()` (currently only pushed via
  `SetContextSizeCallback`) so `/api/state` can return current values on demand.
- `agent.Agent.Cancel()` or reuse of an existing cancel path via context — the TUI
  cancels by cancelling the run context; the web session will hold a cancellable
  context and expose `POST /api/cancel` to call `cancel()`.
- `agent.Agent.History()` accessor so `/api/command /clear-history` and history
  navigation are backed by the same store as the TUI (or keep history in the
  `Session` to avoid touching the agent).

> These are intentionally tiny, non-behavior-changing additions. The core agent loop
> (`Run`/`RunStream`) is reused unchanged.

---

## 5. API Design (REST + SSE)

All request/response bodies are `application/json`. SSE events are `text/event-stream`.

### 5.1 `GET /` and `GET /assets/*`
Serve the embedded `index.html`, `app.js`, `styles.css` (and favicon) with proper
content types and `Cache-Control`.

### 5.2 `POST /api/chat`
```
Request:  { "session": "<id>", "prompt": "refactor utils.go" }
Response: 202 Accepted { "ok": true, "session": "<id>" }
```
Starts `agent.RunStream` in a goroutine (or `agent.Run` if `Streaming==false`).
If `prompt` starts with `/`, it is dispatched to the slash-command handler instead.

### 5.3 `GET /api/events?session=<id>` (SSE)
Opens a streaming channel. Server flushes:
- `event: chunk`  data: `{ "text": "...", "contentType": 0|1|2|3, "isToolCall": bool }`
- `event: result` data: `{ "finalOutput": "...", "reasoning": "...", "tokenUsage": n,
   "steps": [...], "stats": {...} }`
- `event: stats`  data: `{ "inputTokens":n, "outputTokens":n, "tps":f, "toolCalls":n, ... }`
- `event: state`  data: `{ "contextSize":n, "maxContextSize":n, "readOnly":bool,
   "goal":"", "goalActive":bool }`
- `event: error`  data: `{ "message":"..." }`
- `event: done`   data: `{ "ok":true }`  (marks end of a run)
- periodic `event: ping` comment lines to keep the connection alive.

ContentType mapping reuses `inference.StreamingContentType` (Normal=0, Reasoning=1,
Goal=2, Compression=3). The frontend styles each accordingly.

### 5.4 `POST /api/command`
```
Request:  { "session": "<id>", "command": "/stats" }
Response: { "ok": true, "output": "<formatted>", "state": {...} }
```
Supports: `/stats`, `/clear`, `/clear-history`, `/read-only`, `/compress`,
`/goal <text>`, `/goal-off`, `/dump`. Mirrors `handleInteractiveCommand` in `main.go`.

### 5.5 `POST /api/cancel`
Cancels the current run for the given session (calls the session's run-cancel func).

### 5.6 `GET /api/state?session=<id>`
Returns the current `StateEvent` (context size, read-only, goal, stats) — used on
page load and after every command.

---

## 6. Frontend Design (beautiful, dependency-free)

Embedded `index.html` + `app.js` + `styles.css`, vanilla only.

### 6.1 Layout
- **Top bar:** app title, theme selector, context-size ring (green→yellow→red with
  warning levels matching the TUI ✓/⚠/⚠⚠/⚠⚠⚠), read-only toggle, Cancel button.
- **Main pane:** chat transcript — alternating user / assistant messages.
  - Assistant messages render streaming tokens; reasoning is a collapsible
    `<details>`-style block; goal messages are highlighted.
  - **Tool-call cards** appear inline: tool name, parameter key/value list, and a
    spinner while running; they update in place as the model emits tool-call params.
  - Tool results are shown in a monospace block with a copy button.
- **Right/side panel (responsive, collapsible):** live stats (tokens, TPS, tool calls,
  iterations, uptime, compressions), and input history list.
- **Bottom bar:** input textarea (Enter to send, Shift+Enter for newline, Up/Down for
  history), Send button, slash-command hint (`/` autocompletes known commands).

### 6.2 Styling
- Pure CSS with CSS custom properties; **theme-aware** by mapping the active
  `colors` theme (dark/light/solarized/gruvbox/darkula) to a palette of CSS variables
  served via `/api/state` or injected into the HTML at serve time.
- Modern look: subtle gradients, rounded cards, soft shadows, smooth transitions,
  monospace for code/tool output, responsive breakpoints, light/dark modes.
- No external fonts/icons — use system font stack and inline SVG/emoji.

### 6.3 JS behavior
- `new EventSource('/api/events?session=...')` to receive streamed chunks.
- `fetch('/api/chat', POST)` to send prompts.
- `fetch('/api/command')`, `fetch('/api/cancel')`, `fetch('/api/state')`.
- Client keeps the session id in `sessionStorage` so a refresh resumes the same agent
  conversation.
- Handles reconnection on SSE drop (EventSource auto-reconnects; server replays the
  current `state` event on reconnect).

---

## 7. Session & Concurrency Model

- `SessionManager` holds `map[string]*Session` guarded by a mutex; a session is
  created lazily on first chat request (each session builds its own `agent.NewAgent(cfg)`).
- Each `Session` has a **run mutex** so at most one `Run`/`RunStream` executes at a
  time (preserving the conversation context, mirroring the single-threaded TUI loop).
- Each `Session` has an **SSE hub**: a registry of active subscribers (channels). All
  run events are broadcast to all subscribers of that session (supports multiple tabs).
- A session holds a **cancellable context**; `/api/cancel` cancels it, and the run
  goroutine observes cancellation via the same `context.Context` passed to
  `agent.RunStream`.
- Idle sessions are GC'd after a configurable timeout (default e.g. 24h) to avoid
  unbounded memory, matching the agent's long-lived interactive model.

---

## 8. CLI Changes

New flags (added to `config.Config` and `config.ParseArgs`):

```
--web              Start the web UI server instead of the terminal UI
--web-addr <host>  Listen address (default: 127.0.0.1)
--web-port <port>  Listen port (default: 8080)
```

Example:
```bash
./coding-agent --web --web-port 8080 --model llama3 --api-endpoint http://localhost:8080/v1
# prints: Web UI running at http://127.0.0.1:8080  (Ctrl+C to stop)
```

`main.go` web branch mirrors `runInteractiveMode`'s setup:
- `colors.AutoDetect()` stays (already at top of `main`).
- Apply theme.
- Create agent, load `ContextFile` if given, set context-size callback, close debug
  logger on exit, auto `DumpContext` on exit (unless `--no-dump-on-exit`).
- Start `http.Server` with graceful shutdown on SIGINT/SIGTERM.

---

## 9. Requirements Updates

### 9.1 New requirement: `requirements/046-web-ui.md`
Captures: purpose, acceptance criteria (parity with the TUI interactions table above),
zero-dependency constraint, SSE transport rationale, CLI flags, security note
(local-only binding by default).

### 9.2 Update `requirements/024-zero-external-dependencies.md`
Add acceptance criteria / explicit note:
- Web UI backend uses only stdlib `net/http` + `embed` (no websocket library).
- Frontend assets are embedded and served locally (no CDN, no npm, no node build step).
- The project remains buildable offline with no network fetches.

---

## 10. Phased Implementation

### Phase 1 — Backend scaffolding (no UI yet) ✅ COMPLETE
- [x] Add `--web`, `--web-addr`, `--web-port` config + parsing + tests.
- [x] Create `webui` package: `Session`, `SessionManager`, SSE hub, event structs.
- [x] Wire `main.go` web branch (agent setup, context callback, graceful shutdown).
- [x] Add `GET /api/state` and a trivial `GET /` returning a placeholder.
- [x] **Milestone:** `curl /api/state` returns JSON; server starts/stops cleanly.

### Phase 2 — Chat + streaming (SSE) ✅ COMPLETE
- [x] `POST /api/chat`, `POST /api/cancel`, `GET /api/events`.
- [x] Run `agent.RunStream` in a goroutine; publish `chunk`/`result`/`error`/`done`/`stats`.
- [x] Handle non-streaming (`agent.Run`) path by emitting the result as a single chunk.
- [x] **Milestone:** an SSE client (e.g. `curl -N`) receives live tokens.

### Phase 3 — Slash commands & state ✅ COMPLETE
- [x] `/api/command` handler mirroring `handleInteractiveCommand`:
  `/stats`, `/clear`, `/clear-history`, `/read-only`, `/compress`, `/goal`, `/goal-off`, `/dump`.
- [x] Publish `state` events on every change.
- [x] **Milestone:** full command parity with the TUI.

### Phase 4 — Frontend SPA ✅ COMPLETE
- [x] `index.html`, `app.js`, `styles.css` embedded via `go:embed`.
- [x] Chat renderer (user/assistant/reasoning/goal/tool-call cards), SSE client,
  input with history + slash autocomplete, cancel button, clear buttons,
  stats panel, context ring, read-only toggle, goal input, dump button.
- [x] Theme mapping from `colors` to CSS variables.
- [x] **Milestone:** full visual parity; beautiful, responsive, offline.

### Phase 5 — Polish, tests, docs ✅ COMPLETE
- [x] `httptest` unit tests for handlers, session manager, SSE framing, command dispatch.
- [x] README + requirement 046 finalized.
- [x] `go build ./...`, `go vet ./...`, `go test ./...` all green.
- [x] Manual test across browsers (Chrome/Firefox) and themes.

---

## 11. Testing Strategy

- **Unit tests (stdlib `net/http/httptest`):**
  - `SessionManager`: create/get/GC sessions, per-session run serialization.
  - SSE writer: correct `event:`/`data:` framing, flush, ping.
  - Handlers: `POST /api/chat` returns 202; `/api/command` dispatches each command;
    `/api/cancel` cancels a running context; `/api/state` returns current values.
  - Config: parsing `--web`, `--web-port`, `--web-addr` and validation.
  - `go:embed` assets are non-empty and served with the right content type.
- **Integration test:** a fake inference client streams known chunks → assert the SSE
  channel receives matching `chunk` events and a final `result`/`done`.
- **Verification commands:** `go build ./...`, `go vet ./...`, `go test ./...`,
  `go mod tidy` (go.mod must remain stdlib-only apart from the existing `x/term`).
- **Manual:** run `--web`, open browser, exercise every TUI interaction listed in §1.

---

## 12. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| SSE vs WebSocket limitations (browser-only one-way) | All browser→server actions are discrete POSTs; SSE covers streaming output. No websocket lib required (stdlib constraint). |
| Agent state not thread-safe across concurrent runs | Per-session run mutex serializes runs (matches single-threaded TUI). Multiple tabs share the same session agent but runs are serialized. |
| Memory growth from many sessions | Session GC timeout + per-session history cap (mirrors `CODING_AGENT_MAX_HISTORY`). |
| Security (exposing tools to network) | Bind to `127.0.0.1` by default; document that `--web-addr 0.0.0.0` is for trusted networks only. |
| Frontend complexity without a framework | Keep SPA modest; vanilla JS with clear modules; reuse the exact content-type constants already in `inference`. |
| SSE reconnection losing context | On reconnect, server re-sends the latest `state` event; client re-fetches `/api/state`. |

---

## 13. Open Questions (to confirm during Phase 1)

1. Should history live in the `Session` (frontend-only) or reuse the TUI's history
   store? **Recommended:** keep a `Session.history` slice so it survives a refresh.
2. Should multiple browser tabs share one conversation (one agent) or get independent
   sessions? **Recommended:** share per-session id stored in `sessionStorage`, so tabs
   opened from the same browser share; a "New session" button generates a fresh id.
3. Default port 8080 vs a fixed default — configurable via `--web-port` (default 8080).
4. Should the web UI be available in `--web` mode only, or also auto-start when the
   terminal is non-interactive? **Recommended:** explicit `--web` flag only.

---

## 14. Deliverables & Definition of Done

- [x] `webui` package implements server, sessions, SSE, and all `/api/*` endpoints.
- [x] Embedded vanilla SPA provides parity with every TUI interaction in §1.
- [x] Theme-aware, responsive, visually polished UI (no external assets).
- [x] `requirements/046-web-ui.md` added; `requirements/024` updated.
- [x] `--web` / `--web-addr` / `--web-port` flags documented in README and `--help`.
- [x] Unit + integration tests pass; `go build`, `go vet`, `go test`, `go mod tidy` clean.
- [x] go.mod remains stdlib-only (plus existing `x/term`); buildable offline.
