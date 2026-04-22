# Feature #058: Changelog Generator Tool

## Description
A tool that generates changelog entries from git commit history. Parses conventional commit messages and groups them into categories (Features, Bug Fixes, Breaking Changes, etc.) following the Keep a Changelog format.

## Actions
- `generate` - Generate changelog from git commits
- `add` - Append generated entries to an existing CHANGELOG file

## Parameters

### generate
- `from_tag` (optional): Start from this git tag/commit (default: first commit)
- `to_tag` (optional): End at this git tag/commit (default: HEAD)
- `path` (optional): Path to output changelog (default: stdout)
- `unreleased` (optional): If true, include only commits without a tag (default: false)
- `header` (optional): Custom header text to prepend

### add
- `tag` (required): Git tag/version for this changelog entry
- `date` (optional): Date for this entry (default: today)
- `path` (optional): Path to CHANGELOG file (default: ./CHANGELOG.md)
- `unreleased` (optional): If true, move unreleased section to under the new tag

## Acceptance Criteria
- [ ] Tool registered in agent.go with correct tool definition
- [ ] `generate` outputs changelog grouped by commit type
- [ ] Conventional commit types recognized: feat, fix, chore, docs, style, refactor, perf, test, build, ci, revert
- [ ] Categories mapped: feat→Features, fix→Bug Fixes, breaking→Breaking Changes, etc.
- [ ] `unreleased=true` only includes commits without an associated tag
- [ ] `from_tag`/`to_tag` correctly limit commit range
- [ ] `add` appends entries to an existing CHANGELOG.md
- [ ] `add` with `unreleased=true` moves unreleased section to new tag
- [ ] Uses only Go stdlib (no external dependencies)
- [ ] Tool appears in system prompt with proper documentation
