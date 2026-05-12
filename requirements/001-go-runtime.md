# Requirement 001: Go Runtime

## Description
The coding agent harness must be written in Go (Golang) and compile/run as a standalone binary.

## Acceptance Criteria
- [ ] Project uses Go modules (go.mod)
- [ ] Binary compiles without errors using `go build`
- [ ] Binary runs on Linux, macOS, and Windows
- [ ] Project has no external dependencies that prevent cross-compilation

## Cross-Platform Build Requirements

### `make all` Must Build for All Platforms
When running `make all`, the build system must compile binaries for **all** of the following target platforms:

| Platform | GOOS | GOARCH | Description |
|----------|------|--------|-------------|
| arm64-darwin | darwin | arm64 | Apple Silicon macOS |
| x86_64-linux | linux | amd64 | 64-bit Linux |
| x86_64-windows | windows | amd64 | 64-bit Windows |

### Build Output
Each platform target must produce a separate binary with appropriate naming:
- **darwin/arm64**: `coding-agent-darwin-arm64` (or platform-appropriate naming)
- **linux/amd64**: `coding-agent-linux-amd64` (or platform-appropriate naming)
- **windows/amd64**: `coding-agent-windows-amd64.exe` (or platform-appropriate naming)

### Makefile Integration
The Makefile must include:
```makefile
.PHONY: all

all: build-all

.PHONY: build-all

build-all: build-darwin-arm64 build-linux-amd64 build-windows-amd64

.PHONY: build-darwin-arm64 build-linux-amd64 build-windows-amd64

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o coding-agent-darwin-arm64 .

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o coding-agent-linux-amd64 .

build-windows-amd64:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o coding-agent-windows-amd64.exe .
```

## Rationale

### Why All Platforms on `make all`
1. **Consistent Delivery**: Ensures all platform binaries are always built together
2. **CI/CD Simplicity**: Single command produces all artifacts for distribution
3. **Regression Prevention**: Changes that break one platform are caught immediately
4. **Developer Convenience**: No need to remember separate build commands per platform
