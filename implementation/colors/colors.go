// Package colors provides theme-aware ANSI color helpers for the TUI and
// the web UI. It is dependency free (stdlib only).
package colors

import (
	"fmt"
	"sort"
	"strings"
)

// Theme holds the semantic color slots used by the UI layers.
type Theme struct {
	Name    string
	Reset   string
	Dim     string
	Red     string
	Green   string
	Yellow  string
	Blue    string
	Magenta string
	Cyan    string
}

// Slots returns every semantic slot name of the theme.
func (t *Theme) Slots() []string {
	return []string{"reset", "dim", "red", "green", "yellow", "blue", "magenta", "cyan"}
}

// Get returns the ANSI escape sequence for a semantic slot.
// Unknown slots return the reset sequence.
func (t *Theme) Get(slot string) string {
	switch strings.ToLower(strings.TrimSpace(slot)) {
	case "reset":
		return t.Reset
	case "dim":
		return t.Dim
	case "red":
		return t.Red
	case "green":
		return t.Green
	case "yellow":
		return t.Yellow
	case "blue":
		return t.Blue
	case "magenta":
		return t.Magenta
	case "cyan":
		return t.Cyan
	default:
		return t.Reset
	}
}

// BuiltInThemes contains the five built-in themes required by the spec.
var BuiltInThemes = map[string]Theme{
	"dark": {
		Name:    "dark",
		Reset:   "\033[0m",
		Dim:     "\033[90m",
		Red:     "\033[38;5;196m",
		Green:   "\033[38;5;46m",
		Yellow:  "\033[38;5;226m",
		Blue:    "\033[38;5;33m",
		Magenta: "\033[38;5;171m",
		Cyan:    "\033[38;5;51m",
	},
	"light": {
		Name:    "light",
		Reset:   "\033[0m",
		Dim:     "\033[38;5;242m",
		Red:     "\033[38;5;160m",
		Green:   "\033[38;5;28m",
		Yellow:  "\033[38;5;136m",
		Blue:    "\033[38;5;25m",
		Magenta: "\033[38;5;125m",
		Cyan:    "\033[38;5;30m",
	},
	"solarized": {
		Name:    "solarized",
		Reset:   "\033[0m",
		Dim:     "\033[38;5;244m",
		Red:     "\033[38;5;166m",
		Green:   "\033[38;5;106m",
		Yellow:  "\033[38;5;179m",
		Blue:    "\033[38;5;33m",
		Magenta: "\033[38;5;162m",
		Cyan:    "\033[38;5;37m",
	},
	"gruvbox": {
		Name:    "gruvbox",
		Reset:   "\033[0m",
		Dim:     "\033[38;5;244m",
		Red:     "\033[38;5;203m",
		Green:   "\033[38;5;142m",
		Yellow:  "\033[38;5;214m",
		Blue:    "\033[38;5;109m",
		Magenta: "\033[38;5;175m",
		Cyan:    "\033[38;5;108m",
	},
	"darkula": {
		Name:    "darkula",
		Reset:   "\033[0m",
		Dim:     "\033[38;5;245m",
		Red:     "\033[38;5;204m",
		Green:   "\033[38;5;120m",
		Yellow:  "\033[38;5;220m",
		Blue:    "\033[38;5;81m",
		Magenta: "\033[38;5;212m",
		Cyan:    "\033[38;5;146m",
	},
}

// DefaultTheme is used when no theme is configured.
const DefaultTheme = "dark"

var currentTheme = BuiltInThemes[DefaultTheme]

// SetTheme activates a built-in theme by name.
func SetTheme(name string) error {
	theme, ok := BuiltInThemes[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return fmt.Errorf("invalid theme %q: valid themes are: %s", name, strings.Join(ListThemes(), ", "))
	}
	currentTheme = theme
	return nil
}

// ApplyTheme is a convenience wrapper around SetTheme.
func ApplyTheme(name string) error {
	return SetTheme(name)
}

// GetCurrentTheme returns the active theme.
func GetCurrentTheme() *Theme {
	t := currentTheme
	return &t
}

// GetColor returns the ANSI sequence for a semantic slot of the active theme.
func GetColor(slot string) string {
	return currentTheme.Get(slot)
}

// ListThemes returns the sorted list of built-in theme names.
func ListThemes() []string {
	names := make([]string, 0, len(BuiltInThemes))
	for name := range BuiltInThemes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ValidTheme reports whether name is a built-in theme.
func ValidTheme(name string) bool {
	_, ok := BuiltInThemes[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// Paint wraps s with the given semantic slot colors.
func Paint(slot, s string) string {
	return GetColor(slot) + s + GetColor("reset")
}

// CSSVariables returns the theme as CSS custom properties for the web UI.
func (t *Theme) CSSVariables() string {
	return fmt.Sprintf(
		"--dim:%s;--red:%s;--green:%s;--yellow:%s;--blue:%s;--magenta:%s;--cyan:%s;",
		ansiToCSS(t.Dim), ansiToCSS(t.Red), ansiToCSS(t.Green), ansiToCSS(t.Yellow),
		ansiToCSS(t.Blue), ansiToCSS(t.Magenta), ansiToCSS(t.Cyan),
	)
}

// ansiToCSS converts an ANSI 256-color escape into a css rgb() color so the
// web UI can mirror the terminal palette without a color library.
func ansiToCSS(seq string) string {
	if idx := strings.Index(seq, "38;5;"); idx >= 0 {
		var n int
		if _, err := fmt.Sscanf(seq[idx:], "38;5;%d", &n); err == nil {
			r, g, b := xterm256ToRGB(n)
			return fmt.Sprintf("rgb(%d,%d,%d)", r, g, b)
		}
	}
	switch seq {
	case "\033[90m":
		return "rgb(128,128,128)"
	case "\033[2m":
		return "rgb(128,128,128)"
	}
	return "inherit"
}

// xterm256ToRGB maps an xterm 256 color index to an RGB triple.
func xterm256ToRGB(n int) (int, int, int) {
	if n < 0 || n > 255 {
		return 255, 255, 255
	}
	if n < 8 {
		base := [][3]int{{0, 0, 0}, {170, 0, 0}, {0, 170, 0}, {170, 85, 0},
			{0, 0, 170}, {170, 0, 170}, {0, 170, 170}, {170, 170, 170}}
		return base[n][0], base[n][1], base[n][2]
	}
	if n < 16 {
		base := [][3]int{{85, 85, 85}, {255, 85, 85}, {85, 255, 85}, {255, 255, 85},
			{85, 85, 255}, {255, 85, 255}, {85, 255, 255}, {255, 255, 255}}
		return base[n-8][0], base[n-8][1], base[n-8][2]
	}
	if n < 232 {
		i := n - 16
		r := (i / 36) * 51
		g := ((i / 6) % 6) * 51
		b := (i % 6) * 51
		return r, g, b
	}
	v := 8 + (n-232)*10
	return v, v, v
}
