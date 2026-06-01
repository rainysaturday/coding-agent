# Requirement 042: Theme Support

## Description
The coding agent harness must support configurable color themes for the TUI. Users must be able to select a color theme via an environment variable (`CODING_AGENT_THEME`) and a `--theme` CLI flag. The implementation must provide five built-in themes: dark, light, solarized, gruvbox, and darkula.

## Background
Terminal applications are used across a wide variety of terminal emulators and user preferences. Some users prefer dark backgrounds, others prefer light backgrounds, and some use specialized color schemes like Solarized or Gruvbox. Providing theme support allows the TUI to render legible and aesthetically appropriate colors regardless of the user's terminal configuration.

## Acceptance Criteria
- [ ] A `--theme` CLI flag accepts a theme name as its argument
- [ ] The `CODING_AGENT_THEME` environment variable sets the theme
- [ ] The CLI flag overrides the environment variable
- [ ] Five built-in themes are available: `dark`, `light`, `solarized`, `gruvbox`, `darkula`
- [ ] An invalid theme name produces a clear error message listing valid options
- [ ] Theme selection is applied before any colored output is produced
- [ ] All existing color usage in the TUI respects the active theme
- [ ] The default theme is `dark` when no theme is specified
- [ ] The `--help` output documents the `--theme` flag and `CODING_AGENT_THEME` variable
- [ ] Unit tests cover theme parsing, theme validation, theme application, and color retrieval

## Theme Definitions

Each theme defines the following semantic color slots:

| Slot | Purpose |
|------|---------|
| `reset` | Reset all attributes |
| `dim` | Dimmed text (reasoning/thinking content) |
| `red` | Errors, warnings |
| `green` | Success indicators, context OK |
| `yellow` | Caution indicators, context warning |
| `blue` | Informational elements |
| `magenta` | Goal messages, compression messages |
| `cyan` | Informational highlights |

### Built-in Themes

#### `dark` (default)
Optimized for dark terminal backgrounds with vibrant 256-color palette.
```
reset:   \033[0m
dim:     \033[90m
red:     \033[38;5;196m   # Bright red
green:   \033[38;5;46m    # Bright green
yellow:  \033[38;5;226m   # Bright yellow
blue:    \033[38;5;33m    # Strong blue
magenta: \033[38;5;171m   # Purple-magenta
cyan:    \033[38;5;51m    # Bright cyan
```

#### `light`
Optimized for light terminal backgrounds (darker colors for readability).
```
reset:   \033[0m
dim:     \033[38;5;242m   # Gray
red:     \033[38;5;160m   # Darker red
green:   \033[38;5;28m    # Darker green
yellow:  \033[38;5;136m   # Darker yellow
blue:    \033[38;5;25m    # Darker blue
magenta: \033[38;5;125m   # Darker magenta
cyan:    \033[38;5;30m    # Darker cyan
```

#### `solarized`
Based on the Solarized color scheme by Ethan Schoonover (256-color approximation).
```
reset:   \033[0m
dim:     \033[38;5;244m   # Solarized base2 gray
red:     \033[38;5;166m   # Solarized red (#dc322f)
green:   \033[38;5;106m   # Solarized green (#859900)
yellow:  \033[38;5;179m   # Solarized yellow (#b58900)
blue:    \033[38;5;33m    # Solarized blue (#268bd2)
magenta: \033[38;5;162m   # Solarized magenta (#d33682)
cyan:    \033[38;5;37m    # Solarized cyan (#2aa198)
```

#### `gruvbox`
Based on the Gruvbox color scheme (warm, retro colors).
```
reset:   \033[0m
dim:     \033[38;5;244m   # Gruvbox gray
red:     \033[38;5;203m   # Gruvbox red (#fb4934)
green:   \033[38;5;142m   # Gruvbox green (#b8bb26)
yellow:  \033[38;5;214m   # Gruvbox yellow (#fabd2f)
blue:    \033[38;5;109m   # Gruvbox blue (#83a598)
magenta: \033[38;5;175m   # Gruvbox purple (#d3869b)
cyan:    \033[38;5;108m   # Gruvbox cyan (#8ec07c)
```

#### `darkula`
Based on the Darcula/Darkula IDE color scheme (JetBrains style).
```
reset:   \033[0m
dim:     \033[38;5;245m   # Darcula gray
red:     \033[38;5;204m   # Darcula red (#ff79c6)
green:   \033[38;5;120m   # Darcula green (#50fa7b)
yellow:  \033[38;5;220m   # Darcula yellow (#f1fa8c)
blue:    \033[38;5;81m    # Darcula blue (#6272a4)
magenta: \033[38;5;212m   # Darcula purple (#bd93f9)
cyan:    \033[38;5;146m   # Darcula cyan (#8be9fd)
```

## Implementation Details

### New Files
- `implementation/colors/theme.go` — Theme type definition and built-in theme registry

### Modified Files
- `implementation/colors/colors.go` — Add theme management: `SetTheme()`, `GetColor()`, `GetCurrentTheme()`, `ListThemes()`
- `implementation/config/config.go` — Add `Theme` field to `Config`, parse `--theme` flag, load `CODING_AGENT_THEME` env var
- `implementation/main.go` — Call `colors.ApplyTheme()` after config parsing, update `--help` output
- `implementation/tui/tui.go` — Use `colors.GetColor()` for all color references instead of direct constants

### Theme API
```go
// colors/theme.go
type Theme struct {
    Name   string
    Reset  string
    Dim    string
    Red    string
    Green  string
    Yellow string
    Blue   string
    Magenta string
    Cyan   string
}

var BuiltInThemes = map[string]Theme{...}

// colors/colors.go
var currentTheme *Theme

func SetTheme(name string) error
func GetColor(slot string) string  // e.g. "reset", "dim", "red", "green", etc.
func GetCurrentTheme() *Theme
func ListThemes() []string
func ApplyTheme(name string) error  // Convenience: SetTheme + returns error
```

### Config Integration
```go
// config/config.go — Config struct
Theme string  // Theme name (default: "dark")

// CLI flag: --theme <name>
// Env var:    CODING_AGENT_THEME=<name>
```

### Validation
Invalid theme names must produce a clear error:
```
Error: invalid theme "foobar": valid themes are: dark, light, solarized, gruvbox, darkula
```

## Examples

### CLI
```bash
coding-agent --theme gruvbox
coding-agent --theme solarized -p "Write a Go program"
```

### Environment Variable
```bash
export CODING_AGENT_THEME=light
coding-agent
```

### Override Priority
CLI flag (`--theme`) > Environment variable (`CODING_AGENT_THEME`) > Default (`dark`)

### Config File
```
# .coding-agentrc
theme=gruvbox
```

## Testing Requirements

- Unit tests for each built-in theme: verify all color slots are non-empty and distinct
- Unit tests for `SetTheme()` / `ApplyTheme()`: valid names succeed, invalid names return error
- Unit tests for `GetColor()`: returns correct ANSI codes for each slot under each theme
- Unit tests for `ListThemes()`: returns all five built-in theme names
- Config tests: `--theme` flag parsing, `CODING_AGENT_THEME` env var, flag-overrides-env behavior
- Config tests: invalid theme via flag and env var produces descriptive error
- Integration test: theme is applied at startup before any colored output

## Security and Privacy Considerations
- Theme selection is a purely cosmetic setting; no security implications
- Theme names are validated against a whitelist of built-in themes; no code injection via theme name
- No external resources are loaded for themes; all definitions are compiled into the binary
