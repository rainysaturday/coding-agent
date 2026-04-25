# Requirement 035: Git Log Tool

## Description
The harness must support a `git_log` tool that allows viewing the commit history of a git repository via OpenAI's tool calling interface. This tool enables the agent to search through commit history, find specific changes, and understand the evolution of the codebase.

## Acceptance Criteria
- [ ] Tool named `git_log` is available
- [ ] Defaults to showing the repository's commit history
- [ ] Accepts an optional path or directory to limit the scope of the log
- [ ] Accepts an optional commit reference (branch, tag, or commit hash)
- [ ] Supports filtering by file path
- [ ] Supports limiting the number of entries returned
- [ ] Supports formatting options (short, medium, full, fuller)
- [ ] Supports showing patch/diff in each commit
- [ ] Handles cases where the path is not a git repository gracefully
- [ ] Handles git errors gracefully (e.g., no commits, detached HEAD)
- [ ] Tool call failures are tracked in statistics

## Tool Definition (OpenAI Format)

```json
{
  "type": "function",
  "function": {
    "name": "git_log",
    "description": "View the commit history of a git repository",
    "parameters": {
      "type": "object",
      "properties": {
        "path": {
          "type": "string",
          "description": "Path within the repository to view history for (defaults to repository root)"
        },
        "reference": {
          "type": "string",
          "description": "Git reference to view log from (branch name, tag, or commit hash; defaults to current HEAD)"
        },
        "count": {
          "type": "integer",
          "description": "Number of commits to display (defaults to 10)"
        },
        "flags": {
          "type": "array",
          "items": {
            "type": "string"
          },
          "description": "List of git-log-style flags to control output (e.g., 'p' for patch/diff, 's' for short format, 'm' to include merges, 'first-parent' to follow only first parent)"
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
    "name": "git_log",
    "arguments": "{\"path\":\"./src\",\"reference\":\"main\",\"count\":20,\"flags\":[\"s\"]}"
  }
}
```

### Parameters
- `path`: Path within the repository (optional, string, defaults to repository root)
  - Can be a file or directory path
  - When specified, shows history only for changes affecting that path
- `reference`: Git reference (optional, string, defaults to current HEAD)
  - Can be a branch name (e.g., `main`, `develop`), tag name, or commit hash
  - When specified, shows log starting from that reference
- `count`: Number of commits to display (optional, integer, defaults to 10)
  - Maximum number of log entries to return
  - Should be a positive integer
- `flags`: Array of git-log-style flags (optional, array of strings)
  - `"p"`: Include patch/diff for each commit
  - `"s"`: Short format (show only hash, date, and subject)
  - `"m"`: Include merge commits
  - `"first-parent"`: Follow only the first parent of merges
  - `"stat"`: Show file-level change statistics for each commit
  - `"oneline"`: Show each commit on a single line (hash and subject)
  - `"graph"`: Display a text-based graph of branch and merge history
  - `"decorate"`: Show branch and tag names next to commits
  - Multiple flags can be combined (e.g., `["s", "p"]`)

### Examples

**Show recent commits (default behavior):**
```json
{
  "id": "call_001",
  "type": "function",
  "function": {
    "name": "git_log",
    "arguments": "{}"
  }
}
```

**Show commits for a specific directory:**
```json
{
  "id": "call_002",
  "type": "function",
  "function": {
    "name": "git_log",
    "arguments": "{\"path\":\"./src/utils\",\"count\":5}"
  }
}
```

**Show commits from a specific branch:**
```json
{
  "id": "call_003",
  "type": "function",
  "function": {
    "name": "git_log",
    "arguments": "{\"reference\":\"feature-branch\",\"count\":15}"
  }
}
```

**Show short-form commit history:**
```json
{
  "id": "call_004",
  "type": "function",
  "function": {
    "name": "git_log",
    "arguments": "{\"flags\":[\"s\",\"decorate\"]}"
  }
}
```

**Show commit history with patches:**
```json
{
  "id": "call_005",
  "type": "function",
  "function": {
    "name": "git_log",
    "arguments": "{\"reference\":\"HEAD~5..HEAD\",\"flags\":[\"p\"]}"
  }
}
```

**Show branch graph history:**
```json
{
  "id": "call_006",
  "type": "function",
  "function": {
    "name": "git_log",
    "arguments": "{\"flags\":[\"graph\",\"decorate\",\"oneline\"]}"
  }
}
```

## Return Values

On success (default format):
```
commit a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t
Author: John Doe <john.doe@example.com>
Date:   Mon Apr 21 10:30:00 2025 +0000

    Fix authentication bug in login flow

    Resolves issue #42

commit b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0a
Author: Jane Smith <jane.smith@example.com>
Date:   Sun Apr 20 15:45:00 2025 +0000

    Add unit tests for payment processing

    - Test refund scenario
    - Test partial refund
    - Test invalid card number
```

On success with short format (`-s`):
```
a1b2c3d (HEAD -> main) Fix authentication bug in login flow
b2c3d4e Add unit tests for payment processing
c3d4e5f Update README with new API documentation
d4e5f6g Refactor database connection pooling
e5f6g7h Initial project setup
```

On success with patch flag (`-p`):
```
commit a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t
Author: John Doe <john.doe@example.com>
Date:   Mon Apr 21 10:30:00 2025 +0000

    Fix authentication bug in login flow

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

On failure:
- `error`: Description of the error (not a git repository, invalid reference, etc.)
- `success`: `false`

### Output Format Details

When no flags are provided (default):
- Full commit hash (40 characters)
- Author name and email
- Commit date
- Commit message body
- Blank line between commits

When `"s"` (short) flag is provided:
- Abbreviated commit hash (7 characters)
- Branch/tag annotations in parentheses
- Subject line only (no body)

When `"p"` (patch) flag is provided:
- Includes the full diff for each commit after the commit header
- Standard unified diff format

When `"graph"` flag is provided:
- Text-based graph showing branch structure
- Commits displayed with graph markers (e.g., `*`, `/`, `\`)
- Typically combined with `"oneline"` or `"s"` for readability

## Implementation Notes

- The tool should execute `git log` with appropriate flags
- Default behavior: show last 10 commits in the repository root
- When `path` is specified, use `-- <path>` argument to git log
- When `reference` is specified, use it as the starting point
- If not in a git repository, return an error explaining this
- Large repositories should limit output to the specified `count`
- When `count` exceeds a reasonable limit (e.g., 1000), warn the user and cap at 1000
- Patch output (`-p`) should be truncated if excessively large (> 50KB per commit)
- The tool should respect `.gitignore` settings
- For detached HEAD state, show the appropriate annotation
