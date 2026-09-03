# Agent Skills Specification

> This document specifies how the **coding agent** implements support for the open
> **Agent Skills** format (SKILL.md). It is the detailed technical counterpart to
> `requirements/047-skills-support.md` and references the upstream **Agent Skills**
> specification (https://agentskills.io/specification).

## 1. Overview

Agent Skills are a lightweight, open format for extending agent capabilities with
specialized knowledge and workflows. At its core, a **skill is a directory containing a
`SKILL.md` file**, which holds YAML frontmatter metadata (`name`, `description`, and
optional fields) followed by Markdown instructions. A skill may also bundle `scripts/`,
`references/`, and `assets/` resources.

The coding agent integrates skills using the spec's **progressive disclosure** model:

| Tier | What's loaded | When | Cost |
|---|---|---|---|
| 1. Catalog | `name` + `description` | Session start | ~50-100 tokens/skill |
| 2. Instructions | Full `SKILL.md` body | When activated | <5000 tokens recommended |
| 3. Resources | `scripts/`, `references/`, `assets/` | On demand | Varies |

## 2. SKILL.md format (as consumed)

A `SKILL.md` file has two parts: YAML frontmatter between `---` delimiters, then a
Markdown body.

```markdown
---
name: pdf-processing
description: Extracts text and tables from PDFs, fills forms, merges files. Use when handling PDFs.
license: Apache-2.0
compatibility: Requires Python 3.14+ and uv
metadata:
  author: example-org
  version: "1.0"
allowed-tools: Bash(git:*) Read
---

# PDF Processing

## When to use this skill
...
```

### Frontmatter fields
| Field | Required | Constraints (from upstream spec) |
|---|---|---|
| `name` | Yes | 1-64 chars; lowercase alphanumerics + hyphens; no leading/trailing/consecutive hyphens; should match the parent directory |
| `description` | Yes | 1-1024 chars; describes what + when; should contain trigger keywords |
| `license` | No | Short license name or bundled license file reference |
| `compatibility` | No | 1-500 chars; environment requirements |
| `metadata` | No | map[string]string of arbitrary key-value pairs |
| `allowed-tools` | No | Space-separated pre-approved tools (experimental) |

### Body content
The Markdown body after the closing `---` is the skill's instructions. There are no
format restrictions. Recommended sections: step-by-step instructions, examples, edge
cases. The agent loads this entire body on activation.

## 3. Discovery

### 3.1 Search paths
| Scope | Client-native | Cross-client convention | Compatibility |
|---|---|---|---|
| Project | `<cwd>/skills/` | `<cwd>/.agents/skills/` | `<cwd>/.claude/skills/` |
| User | `~/.skills/` | `~/.agents/skills/` | `~/.claude/skills/` |

Additional user-configurable paths may be appended via `--skills-dir <path>`.

Within each scope, scan the cross-client `.agents/skills/` convention first (it is the
interoperable default), then the client-native `skills/`, then `.claude/skills/`.

### 3.2 Scanning algorithm
For each skills directory, walk it and treat a **subdirectory containing a file named
exactly `SKILL.md`** as a skill. Practical rules:

- Skip directories that cannot contain skills: `.git/`, `node_modules/`, `.venv/`, etc.
- Bound the walk (e.g. max depth 6, max 2000 directories) to prevent runaway scanning.
- A `SKILL.md` directly inside a scope root is not a skill (skills are directories);
  ignore stray `README.md`/`LICENSE.md` files in the scope root.

### 3.3 Name collisions
Skills are indexed by `name` in an in-memory map. On a collision, apply the universal
precedence rule: **project-level skills override user-level skills**. Within the same
scope, scan order determines precedence (deterministic: later scans override earlier).
Log a warning whenever a skill is shadowed so the user knows.

### 3.4 Trust
Project-level skills come from the repository being worked on, which may be untrusted.
Gate project-level skill loading on a trust check: only load them if the working
directory is trusted (via a `--trust <dir>` flag or a persisted trust list). Untrusted
project skills are skipped with a warning. User-level skills are always loaded.

## 4. Parsing (stdlib only)

### 4.1 Frontmatter extraction
1. Read `SKILL.md` (streamed via `bufio.Scanner` to bound memory).
2. Find the opening `---` at the start of the file and the closing `---` after it.
3. Parse the YAML block between them; everything after the closing `---` (trimmed) is
   the body.

Because the project has **zero external dependencies** (Requirement 024), implement a
minimal parser for the SKILL.md frontmatter YAML subset:

- Top-level scalar fields (`name`, `description`, `license`, `compatibility`,
  `allowed-tools`): `key: value`.
- A flat `metadata` map (`key: value` lines, optionally indented).
- No nested arrays/objects beyond `metadata`; unsupported constructs are ignored with a
  warning.

### 4.2 Lenient parsing (cross-client compatibility)
Skills authored for other clients may contain technically invalid YAML that other
parsers accept. The most common issue is unquoted values containing a colon:

```yaml
# Technically invalid YAML — the colon breaks parsing
description: Use this skill when: the user asks about PDFs
```

Implement a fallback that wraps such values in quotes (or converts them to block
scalars) before retrying. Do not block skill loading on cosmetic issues.

### 4.3 Validation policy
| Condition | Behavior |
|---|---|
| Name doesn't match the parent directory | Warn, load anyway |
| Name exceeds 64 chars | Warn, load anyway |
| `description` missing/empty | **Skip** the skill, log the error |
| Frontmatter completely unparseable | **Skip** the skill, log the error |
| YAML has recoverable issues | Warn, load anyway |

Record diagnostics so they can be surfaced (debug log, `/skills` output) without
blocking startup.

## 5. Catalog and disclosure

### 5.1 Catalog record
```go
type Skill struct {
    Name        string // from frontmatter `name`
    Description string // from frontmatter `description`
    Location    string // absolute path to the SKILL.md file
    BaseDir     string // parent of Location; used to resolve relative paths
    Body        string // optional: body cached at discovery, or read on activation
    Metadata    map[string]string
    AllowedTools string
    Scope       string // "project" | "user"
}
```

Store the catalog in `map[string]*Skill` keyed by `name` for fast activation lookup.
Cache the `Body` at discovery (faster activation) or read it from `Location` at
activation time (lower memory, picks up edits). `BaseDir` is derived from `Location`
for resolving relative references.

### 5.2 System prompt section
When at least one skill exists, append an `AVAILABLE SKILLS` section to the system
prompt (after the tools section), listing `name` + `description` per skill, plus the
`SKILL.md` path so the model can read it. Follow with a concise behavioral block:

```
The following skills provide specialized instructions for specific tasks.
When a task matches a skill's description, use read_file to load the SKILL.md
at the listed location before proceeding.
When a skill references relative paths, resolve them against the skill's
directory (the parent of SKILL.md) and use absolute paths in tool calls.
```

If the model activates skills via the optional dedicated tool instead, use:
```
When a task matches a skill's description, call the activate_skill tool with
the skill's name to load its full instructions.
```

When **no skills** are discovered, omit the catalog and behavioral block entirely.

## 6. Activation

### 6.1 Model-driven (default)
The model reads the `SKILL.md` at the catalog path using its existing `read_file` tool.
No new infrastructure is required. The model receives the file content (frontmatter
included) as a tool result.

### 6.2 Dedicated `activate_skill` tool (optional)
Register a tool `activate_skill` that takes a skill `name` and returns:

- The frontmatter-stripped body (or the full file, configurable).
- A listing of bundled resources (`scripts/`, `references/`, `assets/`) **without**
  reading them eagerly.

Constrain the `name` parameter to the set of valid skill names (as an enum in the tool
schema) so the model cannot hallucinate a skill name. If no skills are available, do not
register the tool.

Wrap returned skill content in identifying tags so the harness can distinguish it
during context compaction:

```text
# PDF Processing
## When to use this skill
... [SKILL.md body]
Skill directory: /home/user/.agents/skills/pdf-processing
Relative paths in this skill are relative to the skill directory.
scripts/extract.py
scripts/merge.py
references/pdf-spec-summary.md
```

### 6.3 User-explicit activation
- `/skill <name>`: harness looks up the skill and injects its content directly, without
  the model taking an action.
- `/skills`: lists the available catalog (name + description).
- Autocomplete of available skill names as the user types is optional.

### 6.4 Permission allowlisting
If a permission system gates file access, allowlist skill directories so the model can
read bundled resources (`scripts/`, `references/`, `assets/`) without prompting.

## 7. Context management

- **Exempt from compaction:** flag skill tool outputs as protected so context
  compression (Requirement 009) does not prune them. Use the structured tags from §6.2
  to identify skill content and preserve it.
- **Deduplicate:** track skills activated in the session; re-activation is a no-op.
- **Subagent delegation (optional):** run complex skills in a subagent (Requirement
  040) that receives the skill instructions and returns a summary to the main
  conversation.

## 8. CLI and configuration

```
--skills            Enable skill discovery (default: on, unless --no-skills)
--no-skills         Disable skill discovery
--skills-dir <path> Add a custom skill search directory (repeatable)
--trust <dir>       Mark a directory as trusted for project-level skills
```

Add `SkillsEnabled bool`, `SkillsDirs []string`, and `TrustDirs []string` to
`config.Config`. `SkillsEnabled` defaults to true.

## 9. Read-only mode

Skill discovery and activation remain available in read-only mode (Requirement 033).
Skills only add instructions to context; they never grant write access. A skill that
requires mutating tools simply instructs the model to ask the user to re-run in normal
mode. The `allowed-tools` frontmatter field is informational only in read-only mode.

## 10. Testing

- Discovery: skills found in project `.agents/skills/`, user `~/.agents/skills/`, and
  `.claude/skills/`; non-skill directories ignored; scan bounds respected.
- Precedence: project skill overrides same-named user skill; warning logged.
- Parsing: valid frontmatter; missing/empty description → skip; unparseable YAML →
  skip; unquoted colon value → recovered via fallback; name/dir mismatch → warn.
- Disclosure: catalog appears in system prompt; empty catalog → section omitted.
- Activation: `activate_skill` returns body + resource listing; `name` constrained;
  `/skill` injects content; deduplication on repeat.
- Compaction: skill content survives a compression cycle.
- Read-only: skills discovered and activatable; no write tools granted.
- `go build`, `go vet`, `go test` all pass; `go mod tidy` adds no new dependencies.

## 11. References

- Upstream Agent Skills specification: https://agentskills.io/specification
- Client implementation guide: https://agentskills.io/client-implementation/adding-skills-support
- Related project requirements: 024, 009, 015, 016, 029, 033, 040, 044.
