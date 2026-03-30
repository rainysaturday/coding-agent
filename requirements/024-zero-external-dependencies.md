# Requirement 024: Zero External Dependencies

## Description
The coding agent harness must have zero external dependencies in the codebase. All functionality must be implemented using only the Go standard library.

## Acceptance Criteria
- [ ] No external Go modules in go.mod (only standard library packages)
- [ ] No vendor directory with external packages
- [ ] `go mod tidy` produces a clean go.mod with no external dependencies
- [ ] All HTTP clients, JSON parsing, file operations use stdlib only
- [ ] No third-party TUI libraries - implement terminal UI with stdlib
- [ ] No third-party logging libraries - use fmt or custom logging
- [ ] Project can be built in isolated environments without network access
- [ ] CI/CD pipeline does not require fetching external dependencies

## Implementation Guidelines

### Allowed (Standard Library Only)
```go
// HTTP/REST clients
import "net/http"
import "net/url"

// JSON handling
import "encoding/json"

// File operations
import "io/ioutil"
import "os"
import "path/filepath"

// String/text processing
import "strings"
import "regexp"
import "bufio"

// Terminal/TTY operations
import "os"
import "syscall"
import "golang.org/x/term" // Only if absolutely necessary for raw mode

// Context/cancellation
import "context"
import "time"

// Logging
import "fmt"
import "log"
```

### Not Allowed (External Dependencies)
```go
// TUI libraries
import "github.com/mattn/go-tview"
import "github.com/gdamore/tcell"
import "github.com/nsf/termbox-go"

// HTTP clients
import "github.com/go-resty/resty/v2"
import "github.com/hashicorp/go-retryablehttp"

// JSON libraries
import "github.com/json-iterator/go"
import "github.com/goccy/go-json"

// Logging
import "github.com/sirupsen/logrus"
import "go.uber.org/zap"
import "github.com/rs/zerolog"

// Configuration
import "github.com/spf13/viper"
import "github.com/mitchellh/go-homedir"
```

## Rationale

### Benefits
1. **Portability**: Binary works anywhere Go works without dependency resolution
2. **Security**: Smaller attack surface, no supply chain risks from third-party packages
3. **Build Reliability**: Builds succeed even without network access
4. **Simplicity**: Easier to understand, maintain, and audit the codebase
5. **Size**: Smaller binary size without unnecessary dependencies

### Trade-offs
1. **Development Time**: More code to write from scratch (e.g., TUI implementation)
2. **Features**: May need to implement features that libraries provide
3. **Maintenance**: More code to maintain internally

## TUI Implementation

Since no external TUI libraries are allowed, implement terminal UI using:
- ANSI escape codes for colors, cursor positioning, screen clearing
- `os.Stdin` with raw mode for input handling
- `bufio.Scanner` for line input
- Manual screen buffer management for output

### Example: ANSI Escape Sequences
```go
const (
    // Colors
    ColorReset  = "\033[0m"
    ColorRed    = "\033[31m"
    ColorGreen  = "\033[32m"
    ColorYellow = "\033[33m"
    ColorBlue   = "\033[34m"
    ColorCyan   = "\033[36m"
    
    // Cursor/Screen
    ClearScreen = "\033[2J"
    CursorHome  = "\033[H"
    HideCursor  = "\033[?25l"
    ShowCursor  = "\033[?25h"
)

func printColored(color, text string) {
    fmt.Printf("%s%s%s", color, text, ColorReset)
}

func clearScreen() {
    fmt.Print(ClearScreen + CursorHome)
}
```

## HTTP Client Implementation

Use `net/http` for all REST API calls:

```go
func makeInferenceRequest(endpoint string, payload []byte) (*http.Response, error) {
    req, err := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+apiKey)
    
    client := &http.Client{
        Timeout: 30 * time.Second,
    }
    return client.Do(req)
}
```

## JSON Handling

Use `encoding/json` for all serialization:

```go
type InferenceRequest struct {
    Model   string    `json:"model"`
    Messages []Message `json:"messages"`
    Stream  bool      `json:"stream"`
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

func marshalJSON(v interface{}) ([]byte, error) {
    return json.MarshalIndent(v, "", "  ")
}

func unmarshalJSON(data []byte, v interface{}) error {
    return json.Unmarshal(data, v)
}
```

## Verification Requirements

### Build Verification
```bash
# Verify no external dependencies
go mod tidy
go list -m all | grep -v "^$" | grep -v "go.dev" | grep -v "internal"
# Should only show standard library modules

# Verify clean build without network
go mod download -x 2>&1 | grep -v "^$"
# Should complete without downloading anything
```

### Code Review Checklist
- [ ] No `go get` commands in build scripts
- [ ] No import paths starting with `github.com`, `golang.org/x/` (except term if needed), `go.uber.org`, etc.
- [ ] All dependencies listed in go.mod are from `stdlib` or `go.dev` (internal)
- [ ] `go build` succeeds in clean environment

## Related Requirements
- **001-go-runtime.md**: Go runtime requirements
- **007-inference-backend.md**: Inference backend connection
- **015-tool-prefix-prompt.md**: System prompt requirements
