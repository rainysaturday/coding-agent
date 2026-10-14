// Package colors provides ANSI color constants shared across the codebase.
package colors

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

// ANSI color codes (default/dark theme — used as fallback before any theme is applied)
const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorDim     = "\033[90m" // Dim/bright black for reasoning content
	ColorBold    = "\033[1m"  // Bold attribute, theme-independent
)

// Semantic color aliases for specific use cases.
const (
	GoalColor = ColorMagenta // Goal-related messages use magenta
)

// currentTheme holds the currently active theme; nil means defaults are in use.
var currentTheme *Theme

// enabled controls whether GetColor returns ANSI escape sequences. Colors are
// disabled when stdout is not a terminal (e.g. a subagent's piped stdout), so
// escape codes do not leak into captured output such as subagent summaries.
var enabled = true

// SetEnabled controls whether color output is enabled.
func SetEnabled(e bool) { enabled = e }

// AutoDetect enables colors only when stdout is a terminal and the NO_COLOR
// convention is not set. It should be called once at startup before any output.
func AutoDetect() {
	enabled = term.IsTerminal(int(os.Stdout.Fd())) && os.Getenv("NO_COLOR") == ""
}

// SetTheme sets the active theme by name. Returns an error if the theme is unknown.
func SetTheme(name string) error {
	theme, ok := BuiltInThemes[name]
	if !ok {
		names := make([]string, 0, len(BuiltInThemes))
		for n := range BuiltInThemes {
			names = append(names, n)
		}
		return fmt.Errorf("invalid theme %q: valid themes are: %s", name, strings.Join(names, ", "))
	}
	currentTheme = &theme
	return nil
}

// ApplyTheme is a convenience wrapper: it sets the theme and returns any error.
// It is intended to be called once at startup.
func ApplyTheme(name string) error {
	return SetTheme(name)
}

// ResetTheme clears the active theme, returning colors to their built-in defaults.
func ResetTheme() {
	currentTheme = nil
}

// GetCurrentTheme returns a copy of the currently active theme, or nil if none has been set.
func GetCurrentTheme() *Theme {
	if currentTheme == nil {
		return nil
	}
	cp := *currentTheme
	return &cp
}

// colorSlots lists every semantic slot GetColor understands. Keep it in sync
// with themeColor/defaultColor below: it is what an unknown-slot warning
// advertises.
var colorSlots = []string{"reset", "dim", "red", "green", "yellow", "blue", "magenta", "cyan", "border", "bold"}

// Slots returns the semantic color slot names accepted by GetColor.
func Slots() []string {
	out := make([]string, len(colorSlots))
	copy(out, colorSlots)
	return out
}

// themeColor resolves a slot against the active theme.
func themeColor(t *Theme, slot string) (string, bool) {
	switch slot {
	case "reset":
		return t.Reset, true
	case "dim":
		return t.Dim, true
	case "red":
		return t.Red, true
	case "green":
		return t.Green, true
	case "yellow":
		return t.Yellow, true
	case "blue":
		return t.Blue, true
	case "magenta":
		return t.Magenta, true
	case "cyan":
		return t.Cyan, true
	case "border":
		return t.Border, true
	}
	return "", false
}

// defaultColor resolves a slot against the built-in palette.
func defaultColor(slot string) (string, bool) {
	switch slot {
	case "reset":
		return ColorReset, true
	case "dim":
		return ColorDim, true
	case "red":
		return ColorRed, true
	case "green":
		return ColorGreen, true
	case "yellow":
		return ColorYellow, true
	case "blue":
		return ColorBlue, true
	case "magenta":
		return ColorMagenta, true
	case "cyan":
		return ColorCyan, true
	case "border":
		return ColorDim, true
	}
	return "", false
}

// warnedSlots records which unknown slot names have already been reported, so
// a typo in a hot rendering path warns once instead of on every call.
var warnedSlots sync.Map

// warnUnknownSlot reports a slot name that no theme can satisfy. Without this,
// GetColor("grey") quietly returned the reset code and the text simply lost
// its styling -- indistinguishable from a theme that legitimately has no such
// colour.
func warnUnknownSlot(slot string) {
	if _, loaded := warnedSlots.LoadOrStore(slot, true); loaded {
		return
	}
	fmt.Fprintf(os.Stderr, "colors: unknown color slot %q; valid slots are: %s\n",
		slot, strings.Join(colorSlots, ", "))
}

// LookupColor returns the ANSI code for a semantic slot and whether the slot is
// known. Use it when the caller wants to handle a bad slot itself instead of
// relying on the warning.
func LookupColor(slot string) (string, bool) {
	if slot == "bold" {
		return ColorBold, true
	}
	if currentTheme != nil {
		if code, ok := themeColor(currentTheme, slot); ok {
			return code, true
		}
	}
	code, ok := defaultColor(slot)
	if !ok {
		return "", false
	}
	return code, true
}

// GetColor returns the ANSI code for the given semantic color slot under the active theme.
// Supported slots: "reset", "dim", "red", "green", "yellow", "blue", "magenta", "cyan",
// "border" (muted tone for box drawing), plus the theme-independent attribute
// slot "bold".
//
// An unknown slot is reported once on stderr and rendered with the reset code.
// It used to fall through to ColorReset silently, which made a typo in a slot
// name look like a deliberate choice to print unstyled text.
func GetColor(slot string) string {
	// When color output is disabled, return an empty string for every slot so
	// callers emit plain text with no ANSI escape sequences. The slot name is
	// still worth reporting: it is wrong either way.
	if !enabled {
		if slot != "bold" {
			if _, ok := defaultColor(slot); !ok {
				warnUnknownSlot(slot)
			}
		}
		return ""
	}
	// "bold" is a text attribute rather than a palette color, so it is the
	// same under every theme.
	if slot == "bold" {
		return ColorBold
	}
	code, ok := defaultColor(slot)
	if !ok {
		warnUnknownSlot(slot)
		return ColorReset
	}
	if currentTheme != nil {
		if themed, ok := themeColor(currentTheme, slot); ok {
			return themed
		}
	}
	return code
}

func joinThemeNames(names []string) string {
	return strings.Join(names, ", ")
}

// ListThemes returns the names of all built-in themes.
func ListThemes() []string {
	names := make([]string, 0, len(BuiltInThemes))
	for name := range BuiltInThemes {
		names = append(names, name)
	}
	return names
}
