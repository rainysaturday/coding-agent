// Package colors provides ANSI color constants and theme support for the coding agent.
package colors

// Theme defines a set of ANSI color codes for a specific visual style.
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

// BuiltInThemes is the registry of all built-in color themes.
var BuiltInThemes = map[string]Theme{
	"dark": {
		Name:    "dark",
		Reset:   "\033[0m",
		Dim:     "\033[90m",
		Red:     "\033[38;5;196m", // Bright red
		Green:   "\033[38;5;46m",  // Bright green
		Yellow:  "\033[38;5;226m", // Bright yellow
		Blue:    "\033[38;5;33m",  // Strong blue
		Magenta: "\033[38;5;171m", // Purple-magenta
		Cyan:    "\033[38;5;51m",  // Bright cyan
	},
	"light": {
		Name:    "light",
		Reset:   "\033[0m",
		Dim:     "\033[38;5;242m", // Gray for light backgrounds
		Red:     "\033[38;5;160m", // Darker red for light backgrounds
		Green:   "\033[38;5;28m",  // Darker green for light backgrounds
		Yellow:  "\033[38;5;136m", // Darker yellow for light backgrounds
		Blue:    "\033[38;5;25m",  // Darker blue for light backgrounds
		Magenta: "\033[38;5;125m", // Darker magenta for light backgrounds
		Cyan:    "\033[38;5;30m",  // Darker cyan for light backgrounds
	},
	"solarized": {
		Name:    "solarized",
		Reset:   "\033[0m",
		Dim:     "\033[38;5;244m", // Solarized base2 gray
		Red:     "\033[38;5;166m", // Solarized red (#dc322f)
		Green:   "\033[38;5;106m", // Solarized green (#859900)
		Yellow:  "\033[38;5;179m", // Solarized yellow (#b58900)
		Blue:    "\033[38;5;33m",  // Solarized blue (#268bd2)
		Magenta: "\033[38;5;162m", // Solarized magenta (#d33682)
		Cyan:    "\033[38;5;37m",  // Solarized cyan (#2aa198)
	},
	"gruvbox": {
		Name:    "gruvbox",
		Reset:   "\033[0m",
		Dim:     "\033[38;5;244m", // Gruvbox gray
		Red:     "\033[38;5;203m", // Gruvbox red (#fb4934)
		Green:   "\033[38;5;142m", // Gruvbox green (#b8bb26)
		Yellow:  "\033[38;5;214m", // Gruvbox yellow (#fabd2f)
		Blue:    "\033[38;5;109m", // Gruvbox blue (#83a598)
		Magenta: "\033[38;5;175m", // Gruvbox purple (#d3869b)
		Cyan:    "\033[38;5;108m", // Gruvbox cyan (#8ec07c)
	},
	"darkula": {
		Name:    "darkula",
		Reset:   "\033[0m",
		Dim:     "\033[38;5;245m", // Darcula gray
		Red:     "\033[38;5;204m", // Darcula red (#ff79c6)
		Green:   "\033[38;5;120m", // Darcula green (#50fa7b)
		Yellow:  "\033[38;5;220m", // Darcula yellow (#f1fa8c)
		Blue:    "\033[38;5;81m",  // Darcula blue (#6272a4)
		Magenta: "\033[38;5;212m", // Darcula purple (#bd93f9)
		Cyan:    "\033[38;5;146m", // Darcula cyan (#8be9fd)
	},
}

// ErrInvalidTheme is returned when an unknown theme name is provided.
var ErrInvalidTheme = &themeError{}

type themeError struct{}

func (e *themeError) Error() string {
	names := make([]string, 0, len(BuiltInThemes))
	for name := range BuiltInThemes {
		names = append(names, name)
	}
	return "valid themes are: " + joinThemeNames(names)
}
