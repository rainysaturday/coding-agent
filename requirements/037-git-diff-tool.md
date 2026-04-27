# Requirement 037: Git Diff Tool

## Description
The harness must support a `git_diff` tool that allows comparing changes between git commits, branches, files, or the working tree via OpenAI's tool calling interface. This tool enables the agent to inspect differences between any two git states, understand changes made in specific commits, and compare branches or tags.

## Acceptance Criteria
- [ ] Tool named `git_diff` is available
- [ ] Accepts two git references (commits, branches, tags) for comparison
- [ ] Supports comparing a working tree against a commit or index
- [ ] Supports comparing two files within a commit
- [ ] Defaults to showing unstaged changes in the working tree
- [ ] Supports diff format options (unified, raw, patch)
- [ ] Supports showing only filenames or summary statistics
- [ ] Supports showing rename detection
- [ ] Handles cases where the path is not a git repository gracefully
- [ ] Handles git errors gracefully (e.g., invalid references, non-existent files)
- [ ] Tool call failures are tracked in statistics

## Tool Definition (OpenAI Format)

```json
{
  "type": "function",
  "function": {
    "name": "git_diff",
    "description": "Compare changes between git commits, branches, or the working tree",
    "parameters": {
      "type": "object",
      "properties": {
        "reference1": {
          "type": "string",
          "description": "First git reference for comparison (commit hash, branch, tag; omit for working tree)"
        },
        "reference2": {
          "type": "string",
          "description": "Second git reference for comparison (commit hash, branch, tag; omit for index or working tree)"
        },
        "path": {
          "type": "string",
          "description": "Path within the repository to limit the diff (file or directory)"
        },
        "flags": {
          "type": "array",
          "items": {
            "type": "string"
          },
          "description": "List of git-diff-style flags to control output (e.g., 'p' for patch, 's' for short format, 'r' for rename detection, 'stat' for file statistics)"
        }
      },
      "required": []
    }
  }
}
```

## Tool Call Format

```json
{
  "id": "call_abc123",
  "type": "function",
  "function": {
    "name": "git_diff",
    "arguments": "{\"reference1\":\"HEAD~1\",\"reference2\":\"HEAD\",\"path\":\"./src\",\"flags\":[\"s\"]}"
  }
}
```

### Parameters
- `reference1`: First git reference (optional, string, defaults to working tree)
  - Can be a full or abbreviated commit hash
  - Can be a branch name or tag name
  - When omitted, defaults to the current working tree
  - Supports commit range notation (e.g., `HEAD~2`, `main..feature`)
- `reference2`: Second git reference (optional, string, defaults to index when reference1 is specified)
  - Can be a full or abbreviated commit hash
  - Can be a branch name or tag name
  - When omitted and reference1 is provided, compares against the index (staging area)
  - When both omitted, shows unstaged changes in the working tree
- `path`: Path within the repository (optional, string, defaults to repository root)
  - Can be a file or directory path
  - When specified, shows diff only for changes affecting that path
- `flags`: Array of git-diff-style flags (optional, array of strings)
  - `"p"`: Include unified patch/diff output
  - `"s"`: Short format (show file names and number of insertions/deletions)
  - `"r"`: Detect renames if files are renamed
  - `"M"`: Detect copies if files are copied
  - `"stat"`: Show file-level change statistics
  - `"name-only"`: Show only file names that changed
  - `"name-status"`: Show file names and status (Added, Modified, Deleted, Renamed)
  - `"numstat"`: Show numeric count of changes per file
  - `"summary"`: Show diffstat summary
  - `"ignore-space-change"`: Ignore whitespace changes
  - `"ignore-all-space"`: Ignore all whitespace changes
  - `"unified"`: Show unified diff format (default)
  - `"raw"`: Show raw diff format
  - `"patch"`: Show patch format (same as unified)
  - `"color"`: Colorize the output
  - Multiple flags can be combined (e.g., `["p", "r", "stat"]`)

### Examples

**Show unstaged changes in working tree (default behavior):**
```json
{
  "id": "call_001",
  "type": "function",
  "function": {
    "name": "git_diff",
    "arguments": "{}"
  }
}
```

**Show changes between two commits:**
```json
{
  "id": "call_002",
  "type": "function",
  "function": {
    "name": "git_diff",
    "arguments": "{\"reference1\":\"HEAD~1\",\"reference2\":\"HEAD\"}"
  }
}
```

**Show changes in a specific file:**
```json
{
  "id": "call_003",
  "type": "function",
  "function": {
    "name": "git_diff",
    "arguments": "{\"reference1\":\"main\",\"reference2\":\"feature-branch\",\"path\":\"./src/main.go\"}"
  }
}
```

**Show diff between working tree and index (staged changes):**
```json
{
  "id": "call_004",
  "type": "function",
  "function": {
    "name": "git_diff",
    "arguments": "{\"reference1\":\"HEAD\"}"
  }
}
```

**Show short format with rename detection:**
```json
{
  "id": "call_005",
  "type": "function",
  "function": {
    "name": "git_diff",
    "arguments": "{\"reference1\":\"a1b2c3d\",\"reference2\":\"b2c3d4e\",\"flags\":[\"s\",\"r\"]}"
  }
}
```

**Show only file names that changed:**
```json
{
  "id": "call_006",
  "type": "function",
  "function": {
    "name": "git_diff",
    "arguments": "{\"reference1\":\"main..feature\",\"flags\":[\"name-only\"]}"
  }
}
```

**Show file statistics:**
```json
{
  "id": "call_007",
  "type": "function",
  "function": {
    "name": "git_diff",
    "arguments": "{\"reference1\":\"HEAD~3\",\"reference2\":\"HEAD\",\"flags\":[\"stat\"]}"
  }
}
```

**Show diff ignoring whitespace:**
```json
{
  "id": "call_008",
  "type": "function",
  "function": {
    "name": "git_diff",
    "arguments": "{\"reference1\":\"main\",\"reference2\":\"develop\",\"flags\":[\"ignore-all-space\"]}"
  }
}
```

## Return Values

On success (default format with unified diff):
```
diff --git a/src/auth/login.go b/src/auth/login.go
index 1234567..abcdefg 100644
--- a/src/auth/login.go
+++ b/src/auth/login.go
@@ -42,7 +42,7 @@ func Login(username, password string) (*User, error) {
 	if err != nil {
-		return nil, errors.New("invalid credentials")
+		return nil, fmt.Errorf("invalid credentials: %w", err)
 	}
```

On success with short format (`-s`):
```
src/auth/login.go | 2 +-
src/auth/utils.go | 5 +++--
2 files changed, 4 insertions(+), 3 deletions(-)
```

On success with name-status flag:
```
M	src/auth/login.go
M	src/auth/utils.go
R	src/lib/old.go	src/lib/new.go
A	src/lib/new_module.go
D	src/lib/deprecated.go
```

On success with stat flag:
```
 src/auth/login.go   | 2 +-
 src/auth/utils.go   | 5 +++--
 src/lib/old.go      | 0
 src/lib/new.go      | 0
 src/lib/new_module.go | 3 +++
 src/lib/deprecated.go | 4 ----
 6 files changed, 7 insertions(+), 7 deletions(-)
```

On success with name-only flag:
```
src/auth/login.go
src/auth/utils.go
src/lib/old.go
src/lib/new.go
src/lib/new_module.go
src/lib/deprecated.go
```

On failure:
- `error`: Description of the error (not a git repository, invalid reference, commit not found, path not found, etc.)
- `success`: `false`

### Output Format Details

When no flags are provided (default):
- Unified diff format
- Full path with `a/` and `b/` prefixes for old and new paths
- File mode changes shown in index line
- Hunk headers showing line numbers
- `+` lines for additions, `-` lines for deletions, ` ` lines for context

When `"s"` (short) flag is provided:
- `filename | N +-, M +-` format
- One line per file showing file name and count of insertions/deletions
- Summary line at the end

When `"name-status"` flag is provided:
- `STATUS\tfilename` format
- Status codes: `M` (Modified), `A` (Added), `D` (Deleted), `R` (Renamed), `C` (Copied), `T` (Type changed)

When `"stat"` flag is provided:
- `filename | N +-, M +-` format
- Detailed file-level statistics
- Summary line with total changes

When `"name-only"` flag is provided:
- One filename per line
- No additional information about changes

When `"numstat"` flag is provided:
- `insertions\tdeletions\tfilename` format
- Tab-separated values

When `"r"` (rename detection) flag is provided:
- Detects and shows renamed files
- Shows similarity percentage for renames
- Can be combined with other format flags

## Implementation Notes

- The tool should execute `git diff` with appropriate flags
- Default behavior: show unstaged changes in the working tree
- When `reference1` is provided without `reference2`, compare working tree against index
- When both `reference1` and `reference2` are provided, compare them directly
- If not in a git repository, return an error explaining this
- If a specified reference is not found, return an error with available references
- When diff output is excessively large (> 50KB), truncate and warn the user
- The tool should handle submodules gracefully (skip by default unless explicitly configured)
- For binary files, show a message indicating binary files differ
- The tool should handle detached HEAD state gracefully
- When `path` is specified, use `-- <path>` argument to git diff
- Support for abbreviated commit hashes (minimum 4 characters)
- Large diffs should be paginated or truncated with a note about the truncation