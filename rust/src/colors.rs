#![allow(dead_code)]
// ANSI color codes and theme support
// Implements 256-color themes for the TUI as specified in req 042
use std::sync::Mutex;
use std::sync::OnceLock;
use std::collections::HashMap;

/// Color codes for each semantic slot
#[derive(Debug, Clone)]
pub struct ColorCodes {
    pub reset: &'static str,
    pub dim: &'static str,
    pub red: &'static str,
    pub green: &'static str,
    pub yellow: &'static str,
    pub blue: &'static str,
    pub magenta: &'static str,
    pub cyan: &'static str,
}

impl ColorCodes {
    /// Create a new set of color codes from a theme name
    fn from_theme(name: &str) -> Self {
        match name {
            "dark" => ColorCodes {
                reset: "\x1b[0m",
                dim: "\x1b[90m",        // Bright black (dim)
                red: "\x1b[38;5;196m",   // Bright red
                green: "\x1b[38;5;46m",  // Bright green
                yellow: "\x1b[38;5;226m",// Bright yellow
                blue: "\x1b[38;5;33m",   // Strong blue
                magenta: "\x1b[38;5;171m",// Purple-magenta
                cyan: "\x1b[38;5;51m",   // Bright cyan
            },
            "light" => ColorCodes {
                reset: "\x1b[0m",
                dim: "\x1b[38;5;242m",   // Gray
                red: "\x1b[38;5;160m",   // Darker red
                green: "\x1b[38;5;28m",  // Darker green
                yellow: "\x1b[38;5;136m",// Darker yellow
                blue: "\x1b[38;5;25m",   // Darker blue
                magenta: "\x1b[38;5;125m",// Darker magenta
                cyan: "\x1b[38;5;30m",   // Darker cyan
            },
            "solarized" => ColorCodes {
                reset: "\x1b[0m",
                dim: "\x1b[38;5;244m",   // Solarized base2 gray
                red: "\x1b[38;5;166m",   // Solarized red
                green: "\x1b[38;5;106m", // Solarized green
                yellow: "\x1b[38;5;179m",// Solarized yellow
                blue: "\x1b[38;5;33m",   // Solarized blue
                magenta: "\x1b[38;5;162m",// Solarized magenta
                cyan: "\x1b[38;5;37m",   // Solarized cyan
            },
            "gruvbox" => ColorCodes {
                reset: "\x1b[0m",
                dim: "\x1b[38;5;244m",   // Gruvbox gray
                red: "\x1b[38;5;203m",   // Gruvbox red
                green: "\x1b[38;5;142m", // Gruvbox green
                yellow: "\x1b[38;5;214m",// Gruvbox yellow
                blue: "\x1b[38;5;109m",  // Gruvbox blue
                magenta: "\x1b[38;5;175m",// Gruvbox purple
                cyan: "\x1b[38;5;108m",  // Gruvbox cyan
            },
            "darkula" => ColorCodes {
                reset: "\x1b[0m",
                dim: "\x1b[38;5;245m",   // Darcula gray
                red: "\x1b[38;5;204m",   // Darcula red
                green: "\x1b[38;5;120m", // Darcula green
                yellow: "\x1b[38;5;220m",// Darcula yellow
                blue: "\x1b[38;5;81m",   // Darcula blue
                magenta: "\x1b[38;5;212m",// Darcula purple
                cyan: "\x1b[38;5;146m",  // Darcula cyan
            },
            _ => ColorCodes::default(),
        }
    }
}

impl Default for ColorCodes {
    fn default() -> Self {
        ColorCodes {
            reset: "\x1b[0m",
            dim: "\x1b[90m",
            red: "\x1b[31m",
            green: "\x1b[32m",
            yellow: "\x1b[33m",
            blue: "\x1b[34m",
            magenta: "\x1b[35m",
            cyan: "\x1b[36m",
        }
    }
}

/// Built-in themes
pub const BUILTIN_THEMES: [&str; 5] = ["dark", "light", "solarized", "gruvbox", "darkula"];

/// Theme registry
pub struct ThemeRegistry {
    themes: HashMap<String, ColorCodes>,
    current_theme: String,
}

impl ThemeRegistry {
    pub fn new() -> Self {
        let mut registry = ThemeRegistry {
            themes: HashMap::new(),
            current_theme: "dark".to_string(),
        };
        for &name in &BUILTIN_THEMES {
            registry.themes.insert(name.to_string(), ColorCodes::from_theme(name));
        }
        registry
    }

    pub fn apply(&mut self, name: &str) -> Result<(), String> {
        if !BUILTIN_THEMES.contains(&name) {
            let names: Vec<String> = BUILTIN_THEMES.iter().map(|s| s.to_string()).collect();
            return Err(format!(
                "invalid theme {:?}: valid themes are: {}",
                name,
                names.join(", ")
            ));
        }
        self.current_theme = name.to_string();
        Ok(())
    }

    pub fn get_color(&self, slot: &str) -> &'static str {
        if let Some(colors) = self.themes.get(&self.current_theme) {
            return match slot {
                "reset" => colors.reset,
                "dim" => colors.dim,
                "red" => colors.red,
                "green" => colors.green,
                "yellow" => colors.yellow,
                "blue" => colors.blue,
                "magenta" => colors.magenta,
                "cyan" => colors.cyan,
                _ => colors.reset,
            };
        }
        ColorCodes::default().reset
    }

    pub fn get_current_theme(&self) -> &str {
        &self.current_theme
    }

    pub fn get_theme_names(&self) -> Vec<String> {
        BUILTIN_THEMES.iter().map(|s| s.to_string()).collect()
    }
}

impl Default for ThemeRegistry {
    fn default() -> Self {
        Self::new()
    }
}

// Thread-safe global color manager
static COLOR_MANAGER: OnceLock<Mutex<ThemeRegistry>> = OnceLock::new();

fn get_color_manager() -> &'static Mutex<ThemeRegistry> {
    COLOR_MANAGER.get_or_init(|| Mutex::new(ThemeRegistry::new()))
}

/// Get a color by slot name
pub fn get_color(slot: &str) -> &'static str {
    get_color_manager()
        .lock()
        .unwrap()
        .get_color(slot)
}

/// Apply a theme by name
pub fn apply_theme(name: &str) -> Result<(), String> {
    get_color_manager().lock().unwrap().apply(name)
}

/// Get the currently active theme name
pub fn get_current_theme() -> String {
    get_color_manager().lock().unwrap().get_current_theme().to_string()
}

/// Get all available theme names
pub fn list_themes() -> Vec<String> {
    get_color_manager().lock().unwrap().get_theme_names()
}

/// Format text with a color slot
pub fn colored(text: &str, slot: &str) -> String {
    format!("{}{}{}", get_color(slot), text, get_color("reset"))
}

/// Format text as bold
pub fn bold(text: &str) -> String {
    format!("\x1b[1m{}\x1b[0m", text)
}

/// Format text as italic
pub fn italic(text: &str) -> String {
    format!("\x1b[3m{}\x1b[0m", text)
}

/// Format text as underlined
pub fn underlined(text: &str) -> String {
    format!("\x1b[4m{}\x1b[0m", text)
}

/// Format text with foreground color
pub fn fg(slot: &str, text: &str) -> String {
    format!("{}{}{}", get_color(slot), text, get_color("reset"))
}

/// Color theme for TUI display
#[derive(Debug, Clone)]
pub struct ColorTheme {
    pub reset: String,
    pub red: String,
    pub green: String,
    pub yellow: String,
    pub blue: String,
    pub magenta: String,
    pub cyan: String,
    pub dim: String,
    pub tool_call: String,
    pub cursor: String,
    pub prompt: String,
    pub reasoning: String,
    pub goal: String,
    pub compression: String,
}

impl ColorTheme {
    pub fn new() -> Self {
        ColorTheme {
            reset: get_color("reset").to_string(),
            red: get_color("red").to_string(),
            green: get_color("green").to_string(),
            yellow: get_color("yellow").to_string(),
            blue: get_color("blue").to_string(),
            magenta: get_color("magenta").to_string(),
            cyan: get_color("cyan").to_string(),
            dim: get_color("dim").to_string(),
            tool_call: get_color("cyan").to_string(),
            cursor: get_color("green").to_string(),
            prompt: get_color("yellow").to_string(),
            reasoning: get_color("dim").to_string(),
            goal: get_color("magenta").to_string(),
            compression: get_color("yellow").to_string(),
        }
    }

    pub fn from_config(config: &crate::config::Config) -> Self {
        let _ = apply_theme(&config.theme);
        ColorTheme::new()
    }
}

impl Default for ColorTheme {
    fn default() -> Self {
        Self::new()
    }
}
