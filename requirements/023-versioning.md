# Requirement 023: Version Information Display

## Description
The coding agent harness must display version information on startup, including the Git commit hash and whether the repository was clean (no uncommitted changes) at build time. This enables traceability and debugging of specific builds.

## Acceptance Criteria
- [ ] Git commit hash is displayed on startup/title screen
- [ ] Dirty status is shown (clean/dirty indicator)
- [ ] Version information is embedded at build time
- [ ] Format is consistent and human-readable
- [ ] Build timestamp can optionally be included
- [ ] No runtime dependency on Git (information is embedded)

## Display Format

### Welcome Screen Version Line
```
============================================================
  Minimal Coding Agent Harness
============================================================
  Version: <git-hash> [clean]
  or
  Version: <git-hash> [dirty]
```

### Git Hash Format
- Short format: First 7 characters of full commit hash
- Example: `a1b2c3d`
- If unavailable: Display "unknown"

### Dirty Status
- `[clean]`: Repository had no uncommitted changes at build time
- `[dirty]`: Repository had uncommitted changes at build time
- Displayed in brackets after the hash

## Implementation Details

### Build-Time Variables

Version information must be injected at build time using Go ldflags:

```bash
# Build with version information
go build -ldflags "\
  -X main.gitHash=$(git rev-parse --short HEAD) \
  -X main.gitDirty=$(if git diff --quiet; then echo 'clean'; else echo 'dirty'; fi) \
  -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  " -o coding-agent .
```

### Go Variables

```go
// Version information injected at build time
var (
    gitHash   string
    gitDirty  string
    buildTime string
)

// Default values (if not set at build time)
func init() {
    if gitHash == "" {
        gitHash = "unknown"
    }
    if gitDirty == "" {
        gitDirty = "unknown"
    }
}
```

### Display Function

```go
func displayVersion() {
    status := gitDirty
    if status == "clean" {
        fmt.Printf("Version: %s [clean]\n", gitHash)
    } else if status == "dirty" {
        fmt.Printf("\033[33mVersion: %s [dirty]\033[0m\n", gitHash)
    } else {
        fmt.Printf("Version: %s\n", gitHash)
    }
}
```

## Example Output

### Clean Build
```
============================================================
  Minimal Coding Agent Harness
============================================================
  Version: a1b2c3d [clean]
  Built: 2024-01-15T10:30:00Z

Type your request below. Use Ctrl+C to exit.
Type 'stats' to view statistics, 'clear' to clear output.
```

### Dirty Build (with warning indicator)
```
============================================================
  Minimal Coding Agent Harness
============================================================
  Version: a1b2c3d [dirty] ⚠
  Built: 2024-01-15T10:30:00Z

Type your request below. Use Ctrl+C to exit.
Type 'stats' to view statistics, 'clear' to clear output.
```

## Makefile Integration

### Recommended Makefile Target
```makefile
GIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_DIRTY := $(shell if git diff --quiet 2>/dev/null; then echo "clean"; else echo "dirty"; fi)
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X main.gitHash=$(GIT_HASH) \
           -X main.gitDirty=$(GIT_DIRTY) \
           -X main.buildTime=$(BUILD_TIME)

build:
	go build -ldflags "$(LDFLAGS)" -o coding-agent .

version:
	@echo "Git Hash: $(GIT_HASH)"
	@echo "Dirty: $(GIT_DIRTY)"
	@echo "Built: $(BUILD_TIME)"
```

## Debugging Use Cases

### Identifying Build Source
When debugging issues, the git hash allows developers to:
- Identify the exact commit that produced the binary
- Check out that commit for reproduction
- Review the code at that point in time

### Dirty Build Detection
Dirty builds indicate:
- Local modifications that aren't committed
- Potential testing or development version
- Should typically not be used in production

### CI/CD Integration
- CI systems should build with clean repository
- Dirty builds should be flagged or rejected in production pipelines
- Version info enables traceability in logs

## Fallback Behavior

| Scenario | Git Hash | Dirty Status |
|----------|----------|--------------|
| Normal build | Actual hash | clean/dirty |
| Not a git repo | "unknown" | "unknown" |
| Build script error | "unknown" | "unknown" |
| Hash too short | As provided | As provided |

## Security Considerations

- Version info is displayed to users (not sensitive)
- Git hash can be used to identify specific builds
- Dirty status reveals development state
- No security implications from displaying this information

## Testing Requirements

- Verify hash is displayed correctly
- Verify dirty status is accurate
- Verify fallback values when git unavailable
- Verify format is consistent

## Related Requirements

- **002-tui-input-prompt.md**: Welcome screen display
- **001-go-runtime.md**: Binary compilation
