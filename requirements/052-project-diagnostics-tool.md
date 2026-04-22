# #052: Project Diagnostics Tool

## Description
A tool that scans a codebase for common issues and quality problems, producing a structured diagnostic report with severity levels and recommendations.

## Acceptance Criteria

- [x] Tool is named `project_diagnostics` and registered in the agent's tool set
- [x] Accepts optional `paths` parameter to restrict scan to specific paths/glob patterns. If omitted, scans the current directory.
- [x] Accepts optional `max_depth` parameter to limit directory traversal depth (default: 10)
- [x] Accepts optional `mode` parameter: 'full' (default, all checks) or 'basic' (TODOs and empty files only)
- [x] Detects TODO, FIXME, HACK, WARN, XXX, DEPRECATED, and TEMP markers in source code
- [x] Detects empty or near-empty files (< 20 bytes)
- [x] Warns about large files (> 500 lines)
- [x] Detects hardcoded secrets/keys patterns (API keys, tokens, passwords, AWS credentials, GitHub tokens, bearer tokens)
- [x] Filters out binary files and common non-source directories (vendor, node_modules, .git, etc.)
- [x] Returns structured results with:
  - `tool` - tool name identifier
  - `summary` - total issues, files scanned, breakdowns by severity and category
  - `issues` - detailed issue list with file path, line number, severity, category, message, and recommendation
  - `files_scanned` - number of files examined
  - `scan_duration` - time taken for the scan
- [x] Issues are categorized with severity levels: low, medium, high, critical
- [x] Output is human-readable with sections organized by severity
- [x] No false positives for common placeholder values (e.g., "your_", "changeme", "TODO", example values)
- [x] Works in non-interactive mode (no TUI required)
- [x] Is added to the system prompt with proper tool calling format and description
- [x] Uses only Go stdlib - zero external dependencies

## Diagnostic Checks

### Severity Levels
- **Critical**: Hardcoded secrets, credentials, tokens, API keys
- **High**: XXX markers, potential bearer tokens, credentials in URLs
- **Medium**: FIXME markers, HACK markers, deprecated code, large files
- **Low**: TODO markers, WARN markers, empty files, TEMP code

### Checks Performed
1. **TODOs and markers** - Finds TODO, FIXME, HACK, WARN, XXX, DEPRECATED, TEMP in comments
2. **Empty files** - Flags files that are 0 bytes or whitespace-only
3. **Large files** - Warns about files exceeding 500 lines
4. **Hardcoded secrets** - Detects potential API keys, tokens, passwords, AWS credentials in source files

### Scanning Scope
- Scans all text-based source files (.go, .py, .js, .ts, .java, .rs, .c, .cpp, .h, .yaml, .yml, .json, .toml, .env, etc.)
- Skips binary files and non-source directories (.git, vendor, node_modules, dist, build, .venv, __pycache__)
- Supports recursive directory traversal with configurable depth

## Notes
- Uses regex-based pattern matching for marker and secret detection
- Secret detection includes common patterns like GitHub PATs (`ghp_...`), OpenAI keys (`sk-...`), and AWS credentials
- False positive filtering skips placeholder values and template strings
- Results are deduplicated to avoid reporting the same issue multiple times
