// Package version exposes build-time version information.
//
// Values are injected at build time via -ldflags (see Makefile):
//
//	-X github.com/coding-agent/coding-agent/version.GitHash=$(git rev-parse --short HEAD)
//	-X github.com/coding-agent/coding-agent/version.GitDirty=clean|dirty
//	-X github.com/coding-agent/coding-agent/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)
package version

import "fmt"

// Default values are used when the binary is built without ldflags
// (for example with a bare `go build` or `go test`).
var (
	// GitHash is the short (7 char) git commit hash the binary was built from.
	GitHash = "unknown"
	// GitDirty is "clean" or "dirty" depending on the working tree at build time.
	GitDirty = "unknown"
	// BuildTime is the UTC build timestamp (RFC3339).
	BuildTime = "unknown"
	// Version is the human readable release version.
	Version = "0.1.0"
)

// String returns the short version line used on the welcome screen:
//
//	<git-hash> [clean]
//	<git-hash> [dirty]
func String() string {
	hash := GitHash
	if hash == "" {
		hash = "unknown"
	}
	status := GitDirty
	if status != "clean" && status != "dirty" {
		status = "unknown"
	}
	return fmt.Sprintf("%s [%s]", hash, status)
}

// Full returns a multi-line friendly version banner value.
func Full() string {
	return fmt.Sprintf("%s (%s, built %s)", Version, String(), BuildTime)
}
