# Code Findings

> This document catalogs issues discovered during a code review of the coding-agent harness.

---

## ALL ISSUES FIXED ✅

All 10 issues identified during the code review have been reviewed and addressed:

| # | Severity | File | Description | Status |
|---|----------|------|-------------|--------|
| 1 | **CRITICAL** | `agent/agent_format.go:92` | Misplaced `[Replace]` line overwrites `git_diff` display | ✅ Fixed |
| 2 | **CRITICAL** | `agent/agent_format.go:57` | Empty `case "replace_text":` — no display message | ✅ Fixed |
| 3 | **HIGH** | `agent/agent_prompt.go` | Broken tool numbering in system prompt | ✅ Fixed |
| 4 | **MEDIUM** | `tools/replace_text.go:98-106` | Re-searching issue when replacement contains search text | ✅ Fixed |
| 5 | **MEDIUM** | `tools/git_log.go:45-50` | `grep` incorrectly defaults to `reference` value | ✅ Fixed |
| 6 | **LOW** | `config/config.go:155-156` | Misleading validation comment | ✅ Fixed |
| 7 | **LOW** | `tools/insert_lines.go` vs `move_text.go` | Inconsistent trailing newline handling | ✅ Fixed |
| 8 | **LOW** | `tools/git_diff.go:29-47` | Inconsistent fallback parameter support | ✅ Fixed |
| 9 | **LOW** | `tools/fileinfo_*.go` | Platform-specific file info (reviewed - no issues found) | ✅ Reviewed |
| 10 | **LOW** | `config/config.go:91-92` | Silent config line skipping | ✅ Fixed |

All tests pass (`go test ./...` succeeds) and `go vet` reports no issues.
