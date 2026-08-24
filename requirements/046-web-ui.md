# Requirement 046: Web UI

## Description
The coding agent must provide an optional **Web UI** that lets a user perform the
same interactions available in the terminal TUI from a browser. The web UI must be
visually polished and must satisfy the project's **zero external dependencies**
constraint (Requirement 024): stdlib-only Go backend and dependency-free embedded
frontend assets.

## Motivation
- Provide an accessible, graphical alternative to the terminal TUI for interactive use.
- Allow the same prompts, streaming output, tool-call visibility, slash commands,
  context monitoring, and cancellation the terminal UI already supports.

## Acceptance Criteria
- [ ] A `--web` flag starts the web UI server instead of the terminal UI.
- [ ] The web UI supports entering a prompt and streams assistant tokens live.
- [ ] Reasoning, goal-mode, and normal content are visually distinct (mirroring the TUI's
      dim reasoning and magenta goal/compression styling).
- [ ] Tool calls are shown as live-updating cards with the tool name and parameters.
- [ ] The context-size indicator is displayed with the same warning levels as the TUI
      (green → yellow → red as usage rises).
- [ ] Runtime statistics are available (tokens, tokens/second, tool calls, failed calls,
      iterations, compressions, uptime) — via `/stats` and a live panel.
- [ ] Input history navigation is supported (Up/Down arrows) and persists across refresh.
- [ ] The current operation can be cancelled (Cancel button).
- [ ] The following slash commands work and match TUI behavior:
      `/stats`, `/clear`, `/clear-history`, `/read-only`, `/compress`, `/goal <text>`,
      `/goal-off`, `/dump`.
- [ ] Theme support matches the `colors` package themes (dark, light, solarized,
      gruvbox, darkula) via CSS variables.
- [ ] Backend uses **only Go standard library** (`net/http`, `encoding/json`, `embed`).
      No websocket library, no router, no HTTP framework.
- [ ] Frontend uses **only embedded vanilla HTML/CSS/JS**. No CDN, no npm/node build
      step, no external fonts or icon libraries.
- [ ] The server binds to `127.0.0.1` by default.
- [ ] `go mod tidy` keeps go.mod free of any new external dependencies.
- [ ] `go build`, `go vet`, `go test` all pass; the project builds offline.

## Design Summary

### CLI Flags
```
--web              Start the web UI server instead of the terminal UI
--web-addr <host>  Listen address (default: 127.0.0.1)
--web-port <port>  Listen port (default: 8080)
```

### Transport
- Streaming output from the agent to the browser uses **Server-Sent Events (SSE)**
  implemented with stdlib `net/http` + `http.Flusher`. SSE is one-way
  (server → client), which is sufficient: the agent pushes tokens to the browser,
  while the browser sends discrete commands (prompt, slash command, cancel) via
  `fetch()` POST requests. No WebSocket library is used, preserving the
  zero-dependency constraint.

### API Endpoints
| Endpoint | Method | Purpose |
|---|---|---|
| `/` | GET | Serve the embedded single-page app |
| `/assets/*` | GET | Serve embedded JS/CSS |
| `/api/chat` | POST | Send a prompt / run the agent |
| `/api/events` | GET | SSE stream of chunks, results, stats, state, errors |
| `/api/command` | POST | Slash commands (`/stats`, `/clear`, `/goal`, …) |
| `/api/cancel` | POST | Cancel the current operation |
| `/api/state` | GET | Current context size, read-only, goal, stats |

### Session Model
- One `*agent.Agent` per session, created lazily from the resolved config.
- A per-session run mutex serializes agent runs (the agent is not safe for concurrent
  runs; this mirrors the single-threaded TUI loop).
- Sessions are GC'd after an idle timeout to bound memory.

### Security
- Binds to `127.0.0.1` by default. Binding to a non-loopback address with
  `--web-addr` exposes the agent (which can execute tools) to the network and is
  intended for trusted environments only.

## Implementation Guidelines (Allowed — Stdlib Only)
```go
import "net/http"          // HTTP server
import "encoding/json"     // JSON serialization
import "embed"             // embed static frontend assets
import "net/http/httptest" // tests
```

## Related Requirements
- **024-zero-external-dependencies.md**: the web UI must not add external dependencies.
- **002-tui-input-prompt.md**: input handling parity.
- **017-tui-tool-feedback.md**: tool-call visibility parity.
- **021-tui-context-size-display.md**: context-size indicator parity.
- **003-runtime-statistics.md**: statistics parity.
- **019-tui-history-navigation.md**: input history parity.
- **020-tui-ctrl-c-cancellation.md**: cancellation parity.
- **042-theme-support.md**: theme parity.
