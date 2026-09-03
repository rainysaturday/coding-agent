# Requirement 047: Agent Skills Support

## Description
The coding agent must support the open **Agent Skills** format (SKILL.md): reusable
packages of instructions, scripts, and resources that extend the agent's capabilities
with specialized knowledge and workflows. Skills live in **skill directories** and are
discovered from the `.agents/skills/` cross-client convention (plus a client-native
`skills/` location) at **project** and **user** scope. The agent discloses available
skills to the model as a compact catalog, loads a skill's full instructions into
context when it is activated, and keeps that content effective over a long session.

This must satisfy the project's **zero external dependencies** constraint
(Requirement 024): discovery, SKILL.md frontmatter parsing, and catalog construction
must be implemented with the Go standard library only.

## Motivation
- Let users and teams ship domain-specific expertise (PDF processing, code review,
  deployment, release workflows) as drop-in folders the agent picks up automatically.
- Interoperate with the wider ecosystem: skills installed by other compliant clients
  (Claude Code, Cursor, Codex CLI, etc.) under `.agents/skills/` are visible to the
  coding agent, and vice versa.
- Keep the base prompt small while giving the model access to specialized knowledge on
  demand, via the spec's three-tier **progressive disclosure** model.

## Acceptance Criteria
- [ ] The agent discovers skill directories from `.agents/skills/` and `skills/` at
      **project** scope (relative to the working directory) and **user** scope
      (relative to the home directory), plus `.claude/skills/` for compatibility.
- [ ] A skill is recognized as a subdirectory containing a file named exactly `SKILL.md`.
- [ ] `SKILL.md` frontmatter (`name`, `description`, and optional `license`,
      `compatibility`, `metadata`, `allowed-tools`) is parsed with **stdlib only**;
      the markdown body after the closing `---` is the skill's instructions.
- [ ] Parsing is **lenient**: name/description issues produce warnings, not failures;
      a skill with a missing/empty `description` or unparseable frontmatter is skipped
      with a log line rather than aborting startup.
- [ ] The agent builds an in-memory catalog keyed by skill name, storing at minimum
      `name`, `description`, and the absolute `SKILL.md` path.
- [ ] Project-level skills override user-level skills on a `name` collision, with a
      warning logged about the shadowed skill.
- [ ] A compact **AVAILABLE SKILLS** section (name + description per skill, ~50-100
      tokens each) is added to the system prompt, together with brief behavioral
      instructions on how and when to load a skill.
- [ ] When no skills are discovered, the catalog and its instructions are omitted
      entirely (no empty section, no dummy tool).
- [ ] Activation is available to the **model** (file-read activation via the existing
      `read_file` tool, and/or a dedicated `activate_skill` tool) and to the **user**
      (a `/skill <name>` slash command and a `/skills` listing command).
- [ ] When a skill is activated, the model receives the skill's instructions and the
      list of bundled resources (`scripts/`, `references/`, `assets/`) without those
      resources being eagerly read into context.
- [ ] Skill content loaded into context is **exempt from context compaction** so it is
      not silently pruned mid-session, and repeated activation of the same skill is
      deduplicated.
- [ ] Project-level skill loading is gated on a **trust check** so an untrusted
      repository cannot silently inject instructions.
- [ ] A `--skills` / `--no-skills` flag controls whether skills are loaded, and a
      `--skills-dir <path>` flag adds a custom search path.
- [ ] `go build`, `go vet`, `go test` all pass; `go mod tidy` keeps `go.mod` free of
      new external dependencies.

## Design Summary

### CLI Flags
```
--skills            Enable skill discovery (default: on, unless --no-skills)
--no-skills         Disable skill discovery
--skills-dir <path> Add a custom skill search directory (repeatable)
```

### Discovery scopes and paths
| Scope | Client-native | Cross-client convention |
|---|---|---|
| Project | `<cwd>/skills/` | `<cwd>/.agents/skills/` |
| User | `~/.skills/` | `~/.agents/skills/` |
| Compatibility | — | `~/.claude/skills/`, `<cwd>/.claude/skills/` |

Scanning rules: only subdirectories containing exactly `SKILL.md` are skills; skip
`.git/`, `node_modules/` and other non-skill directories; bound the scan (e.g. max
depth 6, max 2000 directories) to avoid runaway walking. Within each scope, both the
client-native and `.agents/` conventions are scanned, and `.agents/` is scanned first
so it is the interoperable default.

### Progressive disclosure (matching the Agent Skills spec)
1. **Catalog** (~50-100 tokens/skill): `name` + `description` loaded at startup and
   placed in the system prompt.
2. **Instructions** (<5000 tokens recommended): the full `SKILL.md` body is loaded only
   when a skill is activated.
3. **Resources**: `scripts/`, `references/`, `assets/` files are loaded only when the
   instructions reference them (via `read_file`/`bash`).

### Frontmatter parsing (stdlib only)
The Agent Skills frontmatter is a small, well-defined YAML subset. The coding agent
implements a minimal parser for it (top-level scalar fields, a flat `metadata` map, and
a space-separated `allowed-tools` string) using only the Go standard library. A lenient
fallback wraps unquoted values containing `:` (a common cross-client authoring mistake)
before parsing so skills authored for other clients still load.

### Activation
- **Model-driven (default):** the model reads the `SKILL.md` at the catalog path with
  its existing `read_file` tool. No new tool is required.
- **Dedicated tool (optional):** an `activate_skill <name>` tool returns the frontmatter-
  stripped body plus a listing of bundled resources, wraps it in identifying tags for
  context management, and constrains `name` to the set of valid skills.
- **User-explicit:** `/skill <name>` loads a skill directly (harness-side lookup +
  injection), and `/skills` lists the available catalog.

### Context management
- Skill tool outputs are flagged as protected so context compression does not prune them.
- Activating an already-loaded skill is a no-op (deduplication).

### Trust
- Project-level skills are only loaded when the working directory is marked trusted
  (e.g. `--trust <dir>` or a trust list); otherwise they are skipped with a warning.
  User-level skills are always loaded.

## Implementation Guidelines (Allowed — Stdlib Only)
```go
import "os"            // file system scanning
import "path/filepath" // path resolution, Rel comparison
import "strings"       // frontmatter splitting / parsing
import "bufio"         // streaming SKILL.md reads
```

## Related Requirements
- **024-zero-external-dependencies.md**: skills support must not add external deps.
- **015-tool-prefix-prompt.md** / **016-tool-result-context.md**: how tools and their
  results are surfaced; the skill catalog and activation reuse these patterns.
- **009-context-compression.md**: skill content must be exempt from compaction.
- **033-read-only-mode.md**: read-only mode must still allow skill discovery and
  activation (skills only add instructions, not write access).
- **029-system-prompt-environment-info.md**: the system prompt gains the skills section.
- **040-subagent-tool.md**: optional subagent-delegation pattern for complex skills.
- **044-context-dump-load.md**: skill state should round-trip with context snapshots.
