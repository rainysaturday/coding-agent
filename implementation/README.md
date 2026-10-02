# coding-agent

A **minimal coding agent harness** — a terminal coding assistant with tool
calling, built from scratch in Go with **zero external dependencies** (Go
standard library only).

It talks to any **OpenAI-compatible chat-completions API** (llama.cpp server,
Ollama, LM Studio, vLLM, OpenAI, GitHub Models, GitHub Copilot API), gives the
model a set of coding tools, and loops until the task is done.

```
$ coding-agent "fix the off-by-one in parse.go"
```

## Features

- **Interactive TUI** with ANSI styling, themes, spinners, live token stats,
  and slash commands (`/help`, `/stats`, `/goal`, `/compress`, `/dump`, …)
- **Agentic tool loop**: the model calls tools; results are fed back; the loop
  runs until the model stops or `--max-iterations` is reached
- **Tools**: `bash`, `read_file`, `write_file`, `read_lines`, `insert_lines`,
  `replace_text`, `move_text`, `list_files`, `grep`, `git_log`, `git_show`,
  `git_diff`, `todo`, `view_image`, `activate_skill`, and `subagent`
  (nested agent)
- **Skills**: `SKILL.md` files (YAML front matter + instructions) discovered
  from bundled, user (`~/.config/coding-agent/skills`) and project scopes;
  project skills require directory trust
- **MCP servers**: stdio JSON-RPC Model Context Protocol clients; server tools
  are exposed to the model as `mcp_<server>_<tool>`
- **Context compression**: when the estimated context exceeds a threshold, the
  middle of the conversation is replaced by a model-written summary
  (first and last messages are preserved)
- **Goal mode**: `/goal <text>` keeps the agent looping until the model reports
  `GOAL_ACHIEVED`
- **Session dumps**: the full context is written to
  `coding-agent-context.json` in the system temp directory (unique suffixes on
  collision) on exit or via `/dump`; `--load <file>` resumes a session
- **Web UI**: `--web` serves an HTML/JS chat interface with SSE streaming
- **Read-only mode** (`--read-only`) removes all mutating tools
- **One-shot mode** (`-p "prompt"`) for scripting; `--quiet` prints only the
  answer; exit codes are meaningful (see below)

## Requirements

- Go 1.22 or newer to build
- An OpenAI-compatible inference endpoint (e.g. `llama-server --port 8080`)

## Build

```sh
make build          # or: go build -o coding-agent .
make test           # go test ./...
make lint           # go vet + gofmt check
```

The binary embeds git hash / dirty state / build time via `-ldflags`
(`make build` does this automatically; a bare `go build` shows `unknown`).

## Quick start

```sh
# llama.cpp server on localhost:8080 (default endpoint)
llama-server -m your-model.gguf --port 8080
./coding-agent

# one-shot
./coding-agent -p "list the go files here and summarise them"

# GitHub Models
export CODING_AGENT_API_ENDPOINT=https://models.inference.ai.azure.com
export CODING_AGENT_API_KEY=ghp_...
./coding-agent --model gpt-4o
```

## Configuration

Priority (lowest → highest): **defaults → config file → environment → CLI
flags**.

### Config file

`~/.coding-agentrc` (INI style; override with `--config` or
`CODING_AGENT_CONFIG`). Global `key = value` pairs plus optional
`[mcp_server.<name>]` sections:

```ini
api_endpoint = http://localhost:8080/v1
model        = qwen2.5-coder-7b-instruct
temperature  = 0.7
max_tokens   = 4096
stream       = true
context_size = 8192
context_threshold = 0.75
max_iterations = 25
theme        = dark
persona      = You are a terse senior engineer.

[mcp_server.myserver]
enabled = true
command = path/to/server
args    = --stdio
env.TOKEN = secret
```

### Environment variables

All config keys have a `CODING_AGENT_*` equivalent:
`CODING_AGENT_API_ENDPOINT`, `CODING_AGENT_API_KEY`, `CODING_AGENT_MODEL`,
`CODING_AGENT_TEMPERATURE`, `CODING_AGENT_MAX_TOKENS`, `CODING_AGENT_TOP_P`,
`CODING_AGENT_STREAM`, `CODING_AGENT_INITIAL_TOKEN_TIMEOUT`,
`CODING_AGENT_CONTEXT_SIZE`, `CODING_AGENT_CONTEXT_THRESHOLD`,
`CODING_AGENT_MAX_ITERATIONS`, `CODING_AGENT_THEME`, `CODING_AGENT_DEBUG`,
`CODING_AGENT_DEBUG_LOG`, `CODING_AGENT_READ_ONLY`, `CODING_AGENT_PERSONA`,
`CODING_AGENT_SUMMARY_ONLY`, `CODING_AGENT_GOAL`, `CODING_AGENT_CONFIG`.
MCP servers can also be configured via
`CODING_AGENT_MCP_<NAME>_<FIELD>` variables (e.g.
`CODING_AGENT_MCP_GITHUB_COMMAND`).

### Flags

Run `coding-agent --help` for the full list. Highlights:

| Flag | Meaning |
|---|---|
| `-p, --prompt` | one-shot prompt (non-interactive) |
| `--stdin` | read prompt from stdin (auto-detected when piped) |
| `--model` | model name |
| `--api-endpoint` | API base URL |
| `--api-key` | API key |
| `--no-stream` | disable streaming |
| `--max-iterations N` | cap on tool-loop iterations |
| `--context-size N` | model context window (tokens) |
| `--context-threshold F` | compress above F × context-size |
| `--goal TEXT` | enable goal mode |
| `--read-only` | disable write/execute tools |
| `--web [--web-port N]` | serve the web UI |
| `--load FILE` | resume from a session dump |
| `--output FILE` | write final answer to a file |
| `--quiet` | print only the final answer (one-shot) |
| `--verbose` | print tool calls and a summary |
| `--no-dump-on-exit` | skip the exit context dump |
| `--theme NAME` | dark (default), light, solarized, gruvbox, darkula |

## Exit codes

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | runtime error |
| 2 | invalid arguments / configuration |
| 3 | authentication error (bad or missing API key) |
| 4 | inference timeout |

## Skills

A skill is a directory containing `SKILL.md` with YAML front matter:

```markdown
---
name: my-skill
description: What this skill does
---
Instructions injected when the model calls activate_skill("my-skill").
```

Search order (project scope first, so project skills win on name clashes):

- **Project** (only if the directory is trusted): `./.agents/skills`,
  `./skills`, `./.claude/skills`
- **User**: `~/.agents/skills`, `~/.skills`, `~/.claude/skills`
- **Custom**: additional directories via `--skills <dir>` (repeatable)

Trust a project directory with `--trust <dir>` (persisted in
`~/.coding-agent-trusted`; also settable via `trust = <dir>` in the config
file). Disable skills entirely with `--no-skills`.

## MCP servers

Configure servers in the config file:

```ini
[mcp_server.github]
enabled = true
command = mcp-github
```

or via environment: `CODING_AGENT_MCP_GITHUB_ENABLED=1`,
`CODING_AGENT_MCP_GITHUB_COMMAND=mcp-github`,
`CODING_AGENT_MCP_GITHUB_ARGS=--stdio`.

On startup the agent launches each server over stdio, performs the MCP
initialize handshake, and registers its tools as `mcp_github_<tool>`.
`--read-only` disables MCP tools (they may mutate state).

## Web UI

```sh
./coding-agent --web --web-port 8081
```

Open <http://127.0.0.1:8081>. The UI streams assistant tokens over SSE, shows
tool calls, stats, and supports the same slash commands.

## Development

```
.
├── main.go            # entry point, one-shot & interactive orchestration
├── config/            # flags, env, config file, precedence
├── inference/         # OpenAI-compatible client, SSE streaming, retries
├── agent/             # agentic loop, context mgmt, compression, dumps, goal mode
├── tools/             # tool registry + implementations
├── skills/            # SKILL.md discovery, trust, injection
├── mcp/               # stdio JSON-RPC MCP client
├── tui/               # terminal UI, key reader, themes, spinners
├── web/               # embedded web UI (net/http + SSE)
├── colors/            # ANSI helpers + themes
├── debug/             # optional debug logging
└── version/           # build info (ldflags-injected)
```

Tests use only the standard library (`go test ./...`).
