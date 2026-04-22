# Feature #069: Code Review Tool

## Description
A tool that reviews code files for issues, quality problems, potential bugs, security concerns, and best practices. Supports multiple languages and returns structured findings with severity levels and improvement suggestions.

## Acceptance Criteria

- [x] Tool is callable via `code_review` in the system
- [x] Accepts `path` parameter (file or directory)
- [x] Accepts optional `files` parameter for specific file paths
- [x] Accepts optional `languages` parameter for language filtering
- [x] Accepts optional `max_findings` parameter for result limiting
- [x] Accepts optional `rules` parameter for rule filtering
- [x] Scans Go files for issues (naked returns, error handling, magic numbers)
- [x] Scans Python files for issues (bare except, wildcard imports, literal booleans)
- [x] Scans JavaScript/TypeScript files for issues (console.log, any types, var redeclaration)
- [x] Scans Rust files for issues
- [x] Scans Java files for issues (printStackTrace)
- [c] General checks (trailing whitespace, hardcoded credentials, TODO comments)
- [x] Returns structured results with severity levels (info, warning, error, critical)
- [x] Uses only Go stdlib - zero external dependencies
- [x] Integrated into system prompt with proper documentation
- [x] Streaming feedback shown in TUI when tool is called
- [x] Success message shown with file count and findings count

## Implementation Files
- `implementation/tools/code_review.go` - Tool implementation
- `implementation/tools/tools.go` - Tool registration in Execute switch
- `implementation/agent/agent.go` - Tool definition, system prompt, streaming/status formatting
- `implementation/agent/agent_test.go` - Updated test expectations

## Severity Levels
- **info**: Informational findings (trailing whitespace, TODO comments)
- **warning**: Code quality concerns (magic numbers, long lines, unused imports)
- **error**: Best practice violations (bare except, printStackTrace)
- **critical**: Security issues (hardcoded credentials, secrets)

## Categories
- **style**: Formatting and style issues
- **quality**: Code quality and maintainability
- **security**: Security vulnerabilities
- **best-practices**: Language-specific best practices
- **maintenance**: Technical debt indicators
