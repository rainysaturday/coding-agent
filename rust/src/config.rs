#![allow(dead_code)]
// Configuration module - handles environment variables, config files, and CLI overrides
// Implements requirements: 001, 010, 011, 012, 013, 025, 026, 039
use std::env;
use std::fs;
use std::path::Path;

/// Agent configuration
#[derive(Debug, Clone)]
pub struct Config {
    // Core settings
    pub api_key: String,
    pub model: String,
    pub endpoint: String,

    // Interaction modes
    pub goal_mode: bool,
    pub goal: Option<String>,
    pub one_shot_prompt: Option<String>,
    pub read_only: bool,
    pub summary_only: bool,

    // Persona settings
    pub persona: String,

    // Context settings
    pub max_context: usize,
    pub enable_compression: bool,
    pub compression_target_size: usize,

    // Display settings
    pub theme: String,

    // Behavior settings
    pub max_iterations: usize,
    pub disable_streaming: bool,
    pub show_all_errors: bool,

    // Debug settings
    pub debug: bool,
}

impl Default for Config {
    fn default() -> Self {
        Config {
            api_key: String::new(),
            model: "gpt-4".to_string(),
            endpoint: "https://api.openai.com/v1/chat/completions".to_string(),
            goal_mode: false,
            goal: None,
            one_shot_prompt: None,
            read_only: false,
            summary_only: false,
            persona: String::new(),
            max_context: 128000,
            enable_compression: false,
            compression_target_size: 32000,
            theme: "dark".to_string(),
            max_iterations: 50,
            disable_streaming: false,
            show_all_errors: false,
            debug: false,
        }
    }
}

impl Config {
    /// Create config from environment variables
    pub fn from_env() -> Self {
        let mut config = Config::default();

        if let Ok(api_key) = env::var("API_KEY") {
            config.api_key = api_key;
        }
        if let Ok(model) = env::var("MODEL") {
            config.model = model;
        }
        if let Ok(endpoint) = env::var("ENDPOINT") {
            config.endpoint = endpoint;
        }
        if let Ok(theme) = env::var("CODING_AGENT_THEME") {
            config.theme = theme;
        }

        config
    }

    /// Validate the configuration
    pub fn validate(&self) -> Result<(), String> {
        // API key is required unless running in one-shot mode without inference
        if self.api_key.is_empty() && self.one_shot_prompt.is_none() {
            return Err("No API key provided. Set the API_KEY environment variable or use --api-key".to_string());
        }

        // Validate theme
        let valid_themes = ["dark", "light", "solarized", "gruvbox", "darkula"];
        if !valid_themes.contains(&self.theme.as_str()) {
            return Err(format!(
                "Invalid theme '{}': valid themes are: {}",
                self.theme,
                valid_themes.join(", ")
            ));
        }

        // Validate max_context
        if self.max_context == 0 {
            return Err("max_context must be greater than 0".to_string());
        }

        // Validate max_iterations
        if self.max_iterations == 0 {
            return Err("max_iterations must be greater than 0".to_string());
        }

        // Validate compression target size
        if self.compression_target_size > self.max_context {
            return Err(
                "compression_target_size must not exceed max_context".to_string()
            );
        }

        Ok(())
    }

    /// List valid themes
    pub fn valid_themes() -> Vec<&'static str> {
        vec!["dark", "light", "solarized", "gruvbox", "darkula"]
    }

    /// Merge another config into this one (higher priority values override)
    pub fn merge(&mut self, other: Config) {
        // Only override if the other has non-default values
        if !other.api_key.is_empty() {
            self.api_key = other.api_key;
        }
        if other.model != "gpt-4" {
            self.model = other.model;
        }
        if other.endpoint != "https://api.openai.com/v1/chat/completions" {
            self.endpoint = other.endpoint;
        }
        if other.goal_mode {
            self.goal_mode = other.goal_mode;
            if other.goal.is_some() {
                self.goal = other.goal;
            }
        }
        if other.one_shot_prompt.is_some() {
            self.one_shot_prompt = other.one_shot_prompt;
        }
        if other.read_only {
            self.read_only = other.read_only;
        }
        if other.summary_only {
            self.summary_only = other.summary_only;
        }
        if !other.persona.is_empty() {
            self.persona = other.persona;
        }
        if other.max_context != 128000 {
            self.max_context = other.max_context;
        }
        if other.enable_compression {
            self.enable_compression = other.enable_compression;
            if other.compression_target_size != 32000 {
                self.compression_target_size = other.compression_target_size;
            }
        }
        if other.theme != "dark" {
            self.theme = other.theme;
        }
        if other.max_iterations != 50 {
            self.max_iterations = other.max_iterations;
        }
        if other.disable_streaming {
            self.disable_streaming = other.disable_streaming;
        }
        if other.show_all_errors {
            self.show_all_errors = other.show_all_errors;
        }
        if other.debug {
            self.debug = other.debug;
        }
    }
}

/// Load config from a TOML file
pub fn load_config_file(path: &str) -> Result<Config, String> {
    let content = fs::read_to_string(path)
        .map_err(|e| format!("Could not read config file: {}", e))?;

    // Parse TOML manually (simple key=value and [section] parsing)
    let mut config = Config::default();
    let mut current_section = String::new();

    for line in content.lines() {
        let line = line.trim();

        // Skip empty lines and comments
        if line.is_empty() || line.starts_with('#') || line.starts_with(';') {
            continue;
        }

        // Section headers
        if line.starts_with('[') && line.ends_with(']') {
            current_section = line[1..line.len()-1].trim().to_string();
            continue;
        }

        // Key-value pairs
        if let Some((key, value)) = line.split_once('=') {
            let key = key.trim();
            let value = value.trim().trim_matches('"').trim_matches('\'');

            let key = if !current_section.is_empty() {
                format!("{}.{}", current_section, key)
            } else {
                key.to_string()
            };

            match key.as_str() {
                "api_key" | "core.api_key" => config.api_key = value.to_string(),
                "model" | "core.model" => config.model = value.to_string(),
                "endpoint" | "core.endpoint" => config.endpoint = value.to_string(),
                "goal_mode" | "core.goal_mode" => {
                    config.goal_mode = value.to_lowercase() == "true" || value == "1";
                }
                "goal" | "core.goal" => config.goal = Some(value.to_string()),
                "read_only" | "core.read_only" => {
                    config.read_only = value.to_lowercase() == "true" || value == "1";
                }
                "summary_only" | "core.summary_only" => {
                    config.summary_only = value.to_lowercase() == "true" || value == "1";
                }
                "persona" | "core.persona" => config.persona = value.to_string(),
                "max_context" | "core.max_context" => {
                    if let Ok(size) = value.parse::<usize>() {
                        config.max_context = size;
                    }
                }
                "enable_compression" | "core.enable_compression" => {
                    config.enable_compression = value.to_lowercase() == "true" || value == "1";
                }
                "compression_target_size" | "core.compression_target_size" => {
                    if let Ok(size) = value.parse::<usize>() {
                        config.compression_target_size = size;
                    }
                }
                "theme" | "tui.theme" => config.theme = value.to_string(),
                "max_iterations" | "core.max_iterations" => {
                    if let Ok(n) = value.parse::<usize>() {
                        config.max_iterations = n;
                    }
                }
                "disable_streaming" | "core.disable_streaming" => {
                    config.disable_streaming = value.to_lowercase() == "true" || value == "1";
                }
                "debug" | "core.debug" => {
                    config.debug = value.to_lowercase() == "true" || value == "1";
                }
                "show_all_errors" | "core.show_all_errors" => {
                    config.show_all_errors = value.to_lowercase() == "true" || value == "1";
                }
                _ => {}
            }
        }
    }

    Ok(config)
}

/// Get default config file paths
pub fn default_config_paths() -> Vec<String> {
    let mut paths = Vec::new();

    if let Some(home) = dirs::home_dir() {
        paths.push(home.join(".coding-agent").join("config.toml").to_string_lossy().to_string());
        paths.push(home.join(".config").join("coding-agent").join("config.toml").to_string_lossy().to_string());
    }

    // Current directory
    paths.push("config.toml".to_string());
    paths.push(".coding-agent.toml".to_string());

    paths
}

/// Set goal mode via the TUI (called from tui module)
pub fn set_goal_mode(tui: &crate::tui::TUI, active: bool) {
    if active {
        // Already handled by TUI
    } else {
        tui.disable_goal_mode();
    }
}

/// Check if a config file exists at any default path
pub fn config_file_exists() -> Option<String> {
    for path in default_config_paths() {
        if Path::new(&path).exists() {
            return Some(path);
        }
    }
    None
}
