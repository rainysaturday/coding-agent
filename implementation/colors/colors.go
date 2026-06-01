// Package colors provides ANSI color constants shared across the codebase.
package colors

import "fmt"

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
)

// Semantic color aliases for specific use cases.
const (
	GoalColor = ColorMagenta // Goal-related messages use magenta
)

// currentTheme holds the currently active theme; nil means defaults are in use.
var currentTheme *Theme

// SetTheme sets the active theme by name. Returns an error if the theme is unknown.
func SetTheme(name string) error {
	theme, ok := BuiltInThemes[name]
	if !ok {
		names := make([]string, 0, len(BuiltInThemes))
		for n := range BuiltInThemes {
			names = append(names, n)
		}
		return fmt.Errorf("invalid theme %q: valid themes are: %s", name, joinThemeNames(names))
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

// GetColor returns the ANSI code for the given semantic color slot under the active theme.
// Supported slots: "reset", "dim", "red", "green", "yellow", "blue", "magenta", "cyan".
// If no theme is active or the slot is unknown, the built-in default is returned.
func GetColor(slot string) string {
	if currentTheme != nil {
		switch slot {
		case "reset":
			return currentTheme.Reset
		case "dim":
			return currentTheme.Dim
		case "red":
			return currentTheme.Red
		case "green":
			return currentTheme.Green
		case "yellow":
			return currentTheme.Yellow
		case "blue":
			return currentTheme.Blue
		case "magenta":
			return currentTheme.Magenta
		case "cyan":
			return currentTheme.Cyan
		}
	}
	// Fall back to built-in defaults
	switch slot {
	case "reset":
		return ColorReset
	case "dim":
		return ColorDim
	case "red":
		return ColorRed
	case "green":
		return ColorGreen
	case "yellow":
		return ColorYellow
	case "blue":
		return ColorBlue
	case "magenta":
		return ColorMagenta
	case "cyan":
		return ColorCyan
	}
	return ColorReset
}

func joinThemeNames(names []string) string {
	result := ""
	for i, n := range names {
		if i > 0 {
			result += ", "
		}
		result += n
	}
	return result
}

// ListThemes returns the names of all built-in themes.
func ListThemes() []string {
	names := make([]string, 0, len(BuiltInThemes))
	for name := range BuiltInThemes {
		names = append(names, name)
	}
	return names
}
