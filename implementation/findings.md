# Code Findings

> This document catalogs issues discovered during a code review of the coding-agent harness.

---

## CRITICAL BUGS

### 1. Misplaced `replace_text` display line overwrites `git_diff` output in `agent_format.go`

**File:** `agent/agent_format.go`, lines 82-93

**Issue:** Line 92 contains `msg = fmt.Sprintf("\n%s[Replace] %s%s\n", ...)` which is positioned **inside** the `case "git_diff":` block. This means when `git_diff` is called, its properly formatted message is immediately overwritten with a `[Replace]` message, causing incorrect display output for `git_diff` tool calls.

```go
case "git_diff":
    ref1 := getToolParamStr("reference1", params)
    ref2 := getToolParamStr("reference2", params)
    if ref1 != "" && ref2 != "" {
        msg = fmt.Sprintf(...)  // Correct git_diff message
    } else if ref1 != "" {
        msg = fmt.Sprintf(...)  // Correct git_diff message
    } else {
        msg = fmt.Sprintf(...)  // Correct git_diff message
    }
    msg = fmt.Sprintf("\n%s[Replace] %s%s\n", ...)  // LINE 92: OVERWRITES git_diff message!
```

**Impact:** When `git_diff` is called, the user sees `[Replace]` instead of `[GitDiff]`.

**Fix:** Move line 92 to be the body of `case "replace_text":` (which is currently an empty case on line 57).

---

### 2. Empty `case "replace_text":` in `streamToolCallWithFullParams` (no display message)

**File:** `agent/agent_format.go`, line 57

**Issue:** The `case "replace_text":` on line 57 has no body — it's an empty case that does nothing. Go does not fall through from empty cases, so `replace_text` tool calls produce no display message during the "streamToolCallWithFullParams" function.

```go
case "replace_text":  // Empty — does nothing!
case "list_files":    // Falls here but Go doesn't auto-fallthrough
```

**Impact:** When `replace_text` is called, no status message is shown to the user. The misplaced line at line 92 (see issue #1) should have been here.

**Fix:** Add the proper `replace_text` formatting code (currently at line 92) to this case.

---

## PROMPT NUMBERING ISSUES

### 3. Inconsistent tool numbering in system prompt

**File:** `agent/agent_prompt.go`

**Issue:** The tool numbers in the system prompt are inconsistent and don't form a proper sequence.

In `sharedToolDescriptions()` (lines 41-73):
```
1. read_file
2. read_lines
8. view_image
9. todo
```

Numbers 3-7 are missing! They're defined in `readOnlyOnlyToolDescriptions()` (lines 77-121):
```
3. list_files
4. grep
5. git_log
6. git_show
7. git_diff
```

But these are only included in read-only mode prompts. In the non-read-only prompt (`buildSystemPrompt`, lines 134-226), the numbering becomes:
```
1. bash
2. sharedToolDescriptions() → 1.read_file, 2.read_lines, 8.view_image, 9.todo
3. write_file
5. insert_lines
6. replace_text
7. move_text
```

Resulting in this sequence for non-read-only mode: **1, 1, 2, 8, 9, 3, 5, 6, 7** — which is completely broken.

**Impact:** The LLM receives a confusing system prompt with incorrect numbering, which may affect tool selection behavior.

**Fix:** Restructure the tool descriptions to use proper sequential numbering in both modes:
- **Normal mode:** 1. bash, 2. read_file, 3. read_lines, 4. write_file, 5. insert_lines, 6. replace_text, 7. move_text, 8. view_image, 9. todo
- **Read-only mode:** 1. read_file, 2. read_lines, 3. list_files, 4. grep, 5. git_log, 6. git_show, 7. git_diff, 8. view_image, 9. todo

---

## DESIGN / CODE QUALITY ISSUES

### 4. `replace_text` has a re-searching issue when replacement contains search text

**File:** `tools/replace_text.go`, lines 98-106

**Issue:** When `count < totalOccurrences`, the loop-based replacement re-searches from the beginning of the string after each replacement. If the replacement text contains the search text, this can cause the same replacement to be applied multiple times to the same location.

```go
newContent = originalContent
for i := 0; i < count; i++ {
    idx := strings.Index(newContent, searchText)  // Re-searches from beginning!
    // ...
    newContent = newContent[:idx] + replaceText + newContent[idx+len(searchText):]
}
```

**Impact:** If replacing "foo" with "foobar" with count=1, after replacement "foobar" contains "foo" at position 0, so the next iteration finds it again.

**Fix:** Use `strings.Replace` with a limit (`n` parameter) instead of a manual loop, or track replacement positions to avoid re-processing.

### 5. `git_log.go` confusingly sets `grep` to `reference` when `grep` parameter is not provided

**File:** `tools/git_log.go`, lines 45-50

**Issue:** When the `grep` parameter is not provided but `reference` is, the code sets `grep = reference`:

```go
grep := ""
if gp, ok := params["grep"].(string); ok && gp != "" {
    grep = gp
} else if reference != "" {
    grep = reference  // ??? Why would grep default to reference?
}
```

This is confusing — `grep` and `reference` are independent parameters. The `reference` should not be used as a grep pattern.

**Impact:** If a user provides a `reference` without a `grep` parameter, the tool unexpectedly searches commit messages for the reference string, which is probably not what the user intended.

**Fix:** Remove the `else if` clause — `grep` should only be set from the `grep` parameter.

### 6. `config.go` validation comment is misleading

**File:** `config/config.go`, lines 155-156

**Issue:** The comment says "ConnectionTimeout and ReadTimeout have defaults, only validate if explicitly set (non-zero)" but the defaults are `24 * 60 * 60` (86400 seconds, non-zero), so the `!= 0` check never triggers for defaults.

```go
// ConnectionTimeout and ReadTimeout have defaults, only validate if explicitly set (non-zero)
if c.ConnectionTimeout != 0 && c.ConnectionTimeout < 5 {
```

**Impact:** Minor — the comment is misleading but the code works correctly because the defaults (86400) pass the `< 5` check.

**Fix:** Update the comment to accurately describe the validation logic, e.g., "only validate if explicitly set below safe minimums".

---

## MINOR / COSMETIC ISSUES

### 7. `insert_lines.go` and `move_text.go` use different approaches for handling trailing newlines

**File:** `tools/insert_lines.go` vs `tools/move_text.go`

**Issue:** `insert_lines.go` (lines 43-48) manually strips trailing empty lines, while `move_text.go` uses a dedicated `splitLines()` helper function (line 212-222) that does the same thing. This inconsistency should be unified.

### 8. `git_diff.go` supports fallback `commit1`/`commit2` parameter names but other git tools do not

**File:** `tools/git_diff.go`, lines 29-47

**Issue:** `git_diff.go` accepts both `reference1`/`reference2` and fallback `commit1`/`commit2` parameter names:

```go
reference1 := ""
if c, ok := params["reference1"].(string); ok && c != "" {
    reference1 = c
}
if reference1 == "" {
    if c, ok := params["commit1"].(string); ok && c != "" {
        reference1 = c
    }
}
```

But `git_log.go` and `git_show.go` do not provide similar fallbacks. This is inconsistent — either all should support fallbacks or none should.

### 9. `fileinfo_unix.go` and `fileinfo_windows.go` may have issues

**Files:** `tools/fileinfo_unix.go`, `tools/fileinfo_windows.go`

**Note:** These platform-specific files for ownership/link count information were not reviewed in detail, but the `formatFileLong` function (line 298 of `list_files.go`) calls `getFileInfoDetails(info.Sys())` which depends on platform-specific implementations that may differ.

### 10. `config.go` `loadConfigFile` silently ignores invalid config lines

**File:** `config/config.go`, lines 91-92

**Issue:** When a config line doesn't have a `=` separator, it's silently skipped:

```go
parts := strings.SplitN(line, "=", 2)
if len(parts) != 2 {
    continue  // Silently ignored
}
```

**Impact:** Users might think their config is being applied when it's actually being silently dropped due to formatting errors.

---

## SUMMARY

| # | Severity | File | Description |
|---|----------|------|-------------|
| 1 | **CRITICAL** | `agent/agent_format.go:92` | Misplaced `[Replace]` line overwrites `git_diff` display |
| 2 | **CRITICAL** | `agent/agent_format.go:57` | Empty `case "replace_text":` — no display message |
| 3 | **HIGH** | `agent/agent_prompt.go` | Broken tool numbering in system prompt |
| 4 | **MEDIUM** | `tools/replace_text.go:98-106` | Re-searching issue when replacement contains search text |
| 5 | **MEDIUM** | `tools/git_log.go:45-50` | `grep` incorrectly defaults to `reference` value |
| 6 | **LOW** | `config/config.go:155-156` | Misleading validation comment |
| 7 | **LOW** | `tools/insert_lines.go` vs `move_text.go` | Inconsistent trailing newline handling |
| 8 | **LOW** | `tools/git_diff.go:29-47` | Inconsistent fallback parameter support |
| 9 | **LOW** | `tools/fileinfo_*.go` | Platform-specific file info (unreviewed) |
| 10 | **LOW** | `config/config.go:91-92` | Silent config line skipping |

All tests pass (`go test ./...` succeeds) and `go vet` reports no issues. The critical bugs are in the display formatting layer (`agent_format.go`) and the prompt numbering, which affect user experience but don't break core tool execution functionality.
