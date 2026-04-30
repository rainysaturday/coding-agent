# Requirement 036: Git Show Tool

## Description
The harness must support a `git_show` tool that allows viewing detailed information about a specific git commit via OpenAI's tool calling interface. This tool enables the agent to inspect individual commits, view their diffs, and understand specific changes made to the codebase.

**Mode:** This tool is available in **read-only mode** only. It is not available in normal (interactive) mode.

## Acceptance Criteria
- [ ] Tool named `git_show` is available
- [ ] Accepts a commit reference (branch, tag, or commit hash)
- [ ] Defaults to showing the current HEAD commit
- [ ] Shows commit metadata (author, date, message)
- [ ] Shows diff/patch of the commit by default
- [ ] Supports diff format options (unified, raw, patch)
- [ ] Supports showing only commit metadata without diff
- [ ] Supports showing commit with stat summary
- [ ] Handles cases where the reference is not found gracefully
- [ ] Handles cases where the path is not a git repository gracefully
- [ ] Tool call failures are tracked in statistics

## Tool Definition (OpenAI Format)

```json
{
  "type": "function",
  "function": {
    "name": "git_show",
    "description": "View detailed information about a specific git commit",
    "parameters": {
      "type": "object",
      "properties": {
        "commit": {
          "type": "string",
          "description": "Git reference for the commit to show (branch name, tag, commit hash; defaults to HEAD)"
        },
        "flags": {
          "type": "array",
          "items": {
            "type": "string"
          },
          "description": "List of git-show-style flags to control output (e.g., 'm' to include merges, 'r' to show rename detection, 's' for short format, 'no-patch' to suppress diff, 'stat' for file statistics)"
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
    "name": "git_show",
    "arguments": "{\"commit\":\"a1b2c3d\",\"flags\":[\"stat\"]}"
  }
}
```

### Parameters
- `commit`: Git reference for the commit (optional, string, defaults to HEAD)
  - Can be a full or abbreviated commit hash
  - Can be a branch name (will show the latest commit on that branch)
  - Can be a tag name
  - Supports commit range notation (e.g., `HEAD~1` for previous commit)
- `flags`: Array of git-show-style flags (optional, array of strings)
  - `"s"`: Short format (show abbreviated hash, date, and subject only)
  - `"no-patch"`: Suppress the patch/diff output
  - `"stat"`: Show file-level change statistics for the commit
  - `"name-status"`: Show file names and status (Added, Modified, Deleted, Renamed)
  - `"r"`: Detect renames if files are renamed
  - `"M"`: Detect copies if files are copied
  - `"summary"`: Show diffstat summary
  - `"patch"`: Show full patch/diff (default behavior)
  - `"stat-name-only"`: Show only file names that changed
  - `"stat-numstat"`: Show numeric count of changes per file
  - Multiple flags can be combined (e.g., `["s", "no-patch"]`)

### Examples

**Show current HEAD commit (default behavior):**
```json
{
  "id": "call_001",
  "type": "function",
  "function": {
    "name": "git_show",
    "arguments": "{}"
  }
}
```

**Show a specific commit by hash:**
```json
{
  "id": "call_002",
  "type": "function",
  "function": {
    "name": "git_show",
    "arguments": "{\"commit\":\"a1b2c3d\"}"
  }
}
```

**Show the previous commit:**
```json
{
  "id": "call_003",
  "type": "function",
  "function": {
    "name": "git_show",
    "arguments": "{\"commit\":\"HEAD~1\"}"
  }
}
```

**Show commit with file statistics only (no diff):**
```json
{
  "id": "call_004",
  "type": "function",
  "function": {
    "name": "git_show",
    "arguments": "{\"commit\":\"main\",\"flags\":[\"stat\"]}"
  }
}
```

**Show only commit metadata (no diff):**
```json
{
  "id": "call_005",
  "type": "function",
  "function": {
    "name": "git_show",
    "arguments": "{\"flags\":[\"s\",\"no-patch\"]}"
  }
}
```

**Show commit with file name and status:**
```json
{
  "id": "call_006",
  "type": "function",
  "function": {
    "name": "git_show",
    "arguments": "{\"commit\":\"abc1234\",\"flags\":[\"name-status\"]}"
  }
}
```

**Show commit with rename detection:**
```json
{
  "id": "call_007",
  "type": "function",
  "function": {
    "name": "git_show",
    "arguments": "{\"commit\":\"HEAD\",\"flags\":[\"r\",\"stat\"]}"
  }
}
```

## Return Values

On success (default format with patch):
```
commit a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t
Author: John Doe <john.doe@example.com>
Date:   Mon Apr 21 10:30:00 2025 +0000

    Fix authentication bug in login flow

    Resolves issue #42

diff --git a/src/auth/login.go b/src/auth/login.go
index 1234567..abcdefg 100644
--- a/src/auth/login.go
+++ b/src/auth/login.go
@@ -42,7 +42,7 @@ func Login(username, password string) (*User, error) {
        if err != nil {
-               return nil, errors.New("invalid credentials")
+               return nil, fmt.Errorf("invalid credentials: %w", err)
        }
```

On success with short format (`-s`) and no patch:
```
a1b2c3d (HEAD -> main) Fix authentication bug in login flow
```

On success with stat flag:
```
commit a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t
Author: John Doe <john.doe@example.com>
Date:   Mon Apr 21 10:30:00 2025 +0000

    Fix authentication bug in login flow

    Resolves issue #42

 src/auth/login.go   | 2 +-
 src/auth/utils.go   | 5 +++--
 2 files changed, 4 insertions(+), 3 deletions(-)
```

On success with name-status flag:
```
commit a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t
Author: John Doe <john.doe@example.com>
Date:   Mon Apr 21 10:30:00 2025 +0000

    Fix authentication bug in login flow

    Resolves issue #42

M       src/auth/login.go
M       src/auth/utils.go
```

On failure:
- `error`: Description of the error (not a git repository, invalid reference, commit not found, etc.)
- `success`: `false`

### Output Format Details

When no flags are provided (default):
- Full commit hash (40 characters)
- Author name and email
- Commit date
- Commit message (subject line and body if present)
- Full unified diff patch

When `"s"` (short) flag is provided:
- Abbreviated commit hash (7 characters)
- Branch/tag annotations in parentheses
- Subject line only (no body)
- No patch/diff by default unless explicitly requested

When `"no-patch"` flag is provided:
- Commit header information only
- No diff or patch output
- Useful when only metadata is needed

When `"stat"` flag is provided:
- Commit header information
- File-level change statistics
- Summary line showing files changed, insertions, and deletions
- No actual diff content

When `"name-status"` flag is provided:
- Commit header information
- List of affected files with their status:
  - `M` - Modified
  - `A` - Added
  - `D` - Deleted
  - `R` - Renamed
  - `C` - Copied
  - `T` - Type changed

When `"r"` (rename detection) flag is provided:
- Detects renamed files in addition to normal modifications
- Shows rename percentage similarity
- Can be combined with `"stat"` for rename statistics

## Implementation Notes

- The tool should execute `git show` with appropriate flags
- Default behavior: show current HEAD commit with full details and patch
- If `commit` is not specified, default to `HEAD`
- If not in a git repository, return an error explaining this
- If the specified commit reference is not found, return an error with available references
- When patch output is excessively large (> 50KB), truncate and warn the user
- For merge commits, include the merge information and all parents
- When showing a branch name, resolve it to the HEAD of that branch
- Support for abbreviated commit hashes (minimum 4 characters)
- The tool should handle detached HEAD state gracefully
- For very old or deeply nested repositories, ensure the tool doesn't timeout
