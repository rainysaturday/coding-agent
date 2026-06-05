// Main module - orchestrates the Coding Agent Harness
// Implements requirements: 001, 002, 003, 004, 010, 011, 012, 013, 015, 018, 025, 026, 034, 035, 039, 040
//
// Usage:
//   cargo run -- --model gpt-4 --endpoint https://api.openai.com/v1
//   cargo run -- --goal "Write a web scraper"
//   cargo run -- --one-shot "What is the capital of France?"
//   cargo run -- --read-only
//   cargo run -- --debug --show-all-errors
//   cargo run -- --help

mod agent;
mod colors;
mod config;
mod inference;
mod tools;
mod tui;

use std::env;
use std::process;

/// Check if stdin is a TTY
fn is_tty() -> bool {
    atty::is(atty::Stream::Stdin)
}

/// Print version information
fn print_version() {
    println!("Coding Agent Harness v0.1.0");
    println!("Rust implementation with full tool support");
}

/// Print usage information
fn print_usage() {
    println!("Usage: coding-agent [OPTIONS]");
    println!();
    println!("Options:");
    println!("  --model <MODEL>            Model name (e.g., gpt-4, claude-3-opus)");
    println!("  --endpoint <URL>           OpenAI-compatible endpoint URL");
    println!("  --max-context <TOKENS>     Maximum context size (default: 128000)");
    println!("  --goal <PROMPT>            Goal-directed mode with prompt");
    println!("  --one-shot <PROMPT>        Non-interactive mode with single prompt");
    println!("  --read-only                Read-only mode (no file modifications)");
    println!("  --summary-only             Summary-only mode (concise output only)");
    println!("  --persona <TEXT>           Custom agent persona (influences tone/expertise)");
    println!("  --max-iterations <N>       Maximum iterations (default: 50)");
    println!("  --compress-on-overflow     Enable context compression when full");
    println!("  --compress-size <TOKENS>   Context compression target size");
    println!("  --no-streaming             Disable streaming");
    println!("  --no-stream                Disable streaming output");
    println!("  --debug                    Enable debug logging");
    println!("  --show-all-errors          Show all errors (not just tool errors)");
    println!("  --theme <THEME>            Set TUI theme (dark, light, solarized, gruvbox, darkula)");
    println!("  --config <FILE>            Config file path");
    println!("  --version                  Display version information");
    println!("  --help                     Display this help message");
    println!();
    println!("Environment Variables:");
    println!("  API_KEY                    OpenAI-compatible API key");
    println!("  MODEL                      Default model name");
    println!("  ENDPOINT                   OpenAI-compatible endpoint URL");
    println!("  CODING_AGENT_THEME         Set TUI theme (dark, light, solarized, gruvbox, darkula)");
    println!("  GOAL_MODE                  Enable goal-directed mode (true/false)");
    println!("  GOAL                       Goal prompt text");
    println!("  CONTEXT_COMPRESSION        Enable context compression (true/false)");
}

/// Main entry point
#[tokio::main]
async fn main() {
    let args: Vec<String> = env::args().collect();

    // Parse CLI flags
    let mut config = config::Config::default();

    let mut i = 1;
    while i < args.len() {
        match args[i].as_str() {
            "--help" | "-h" => {
                print_usage();
                process::exit(0);
            }
            "--version" | "-V" => {
                print_version();
                process::exit(0);
            }
            "--model" => {
                if let Some(val) = args.get(i + 1) {
                    config.model = val.clone();
                    i += 1;
                }
            }
            "--endpoint" => {
                if let Some(val) = args.get(i + 1) {
                    config.endpoint = val.clone();
                    i += 1;
                }
            }
            "--max-context" => {
                if let Some(val) = args.get(i + 1) {
                    if let Ok(size) = val.parse::<usize>() {
                        config.max_context = size;
                        i += 1;
                    }
                }
            }
            "--goal" => {
                if let Some(val) = args.get(i + 1) {
                    config.goal = Some(val.clone());
                    config.goal_mode = true;
                    i += 1;
                }
            }
            "--one-shot" => {
                if let Some(val) = args.get(i + 1) {
                    config.one_shot_prompt = Some(val.clone());
                    i += 1;
                }
            }
            "--read-only" | "-R" => {
                config.read_only = true;
            }
            "--summary-only" => {
                config.summary_only = true;
            }
            "--persona" => {
                if let Some(val) = args.get(i + 1) {
                    config.persona = val.clone();
                    i += 1;
                }
            }
            "--max-iterations" => {
                if let Some(val) = args.get(i + 1) {
                    if let Ok(n) = val.parse::<usize>() {
                        config.max_iterations = n;
                        i += 1;
                    }
                }
            }
            "--compress-on-overflow" => {
                config.enable_compression = true;
            }
            "--compress-size" => {
                if let Some(val) = args.get(i + 1) {
                    if let Ok(size) = val.parse::<usize>() {
                        config.compression_target_size = size;
                        i += 1;
                    }
                }
            }
            "--no-streaming" => {
                config.disable_streaming = true;
            }
            "--debug" | "-d" => {
                config.debug = true;
            }
            "--show-all-errors" => {
                config.show_all_errors = true;
            }
            "--theme" => {
                if let Some(val) = args.get(i + 1) {
                    config.theme = val.clone();
                    i += 1;
                }
            }
            "--no-stream" => {
                config.disable_streaming = true;
            }
            "--config" => {
                if let Some(val) = args.get(i + 1) {
                    if let Ok(c) = config::load_config_file(val) {
                        config.merge(c);
                    } else {
                        eprintln!("Error: Could not read config file '{}'", val);
                        process::exit(1);
                    }
                    i += 1;
                }
            }
            "--" => {
                // End of options; remaining args are ignored for this CLI
                break;
            }
            arg if arg.starts_with('-') => {
                eprintln!("Error: Unknown option '{}'", arg);
                print_usage();
                process::exit(1);
            }
            _ => {
                // Treat as positional prompt for one-shot mode
                config.one_shot_prompt = Some(args[i..].join(" "));
                break;
            }
        }
        i += 1;
    }

    // Load environment variables
    if let Ok(api_key) = env::var("API_KEY") {
        config.api_key = api_key;
    }
    if let Ok(model) = env::var("MODEL") {
        config.model = model;
    }
    if let Ok(endpoint) = env::var("ENDPOINT") {
        config.endpoint = endpoint;
    }
    if let Ok(goal_mode_str) = env::var("GOAL_MODE") {
        config.goal_mode = goal_mode_str.to_lowercase() == "true";
    }
    if let Ok(goal) = env::var("GOAL") {
        config.goal = Some(goal);
    }
    if let Ok(compression_str) = env::var("CONTEXT_COMPRESSION") {
        if compression_str.to_lowercase() == "true" && !config.enable_compression {
            config.enable_compression = true;
        }
    }

    // Validate configuration
    if let Err(e) = config.validate() {
        eprintln!("Error: {}", e);
        process::exit(1);
    }

    // Check theme validity
    if !config::Config::valid_themes().contains(&config.theme.as_str()) {
        eprintln!("Error: Invalid theme '{}'", config.theme);
        eprintln!("Valid themes: {}", config::Config::valid_themes().join(", "));
        process::exit(1);
    }

    // Debug mode logging
    if config.debug {
        eprintln!("[DEBUG] Starting Coding Agent Harness v0.1.0");
        eprintln!("[DEBUG] Model: {}", config.model);
        eprintln!("[DEBUG] Endpoint: {}", config.endpoint);
        eprintln!("[DEBUG] Mode: {}", if config.goal_mode { "goal-directed" } else if config.summary_only { "summary-only" } else { "interactive" });
        eprintln!("[DEBUG] Read-only: {}", config.read_only);
        eprintln!("[DEBUG] Debug: {}", config.debug);
        eprintln!("[DEBUG] API Key: {}", redact_key(&config.api_key));
        eprintln!("[DEBUG] Theme: {}", config.theme);
        if !config.persona.is_empty() {
            eprintln!("[DEBUG] Persona: {}", config.persona);
        }
    }

    // Initialize TUI
    let tui = tui::TUI::new().with_max_context(config.max_context);

    // Apply theme if specified
    let _ = colors::apply_theme(&config.theme);

    // Check read-only mode
    if config.read_only {
        println!("{}[Read-Only Mode] No file modifications will be performed.{}",
            colors::get_color("yellow"), colors::get_color("reset"));
    }

    // Check TTY
    if !is_tty() {
        // If stdin is not a TTY, fall back to one-shot mode if no prompt given
        if config.one_shot_prompt.is_none() && config.goal.is_none() {
            eprintln!("Error: No prompt provided and stdin is not a TTY.");
            eprintln!("Pipe text to stdin or provide a prompt with --one-shot");
            process::exit(1);
        }
    }

    // Create the agent
    let mut agent = match agent::Agent::new(config.clone(), tui.clone()) {
        Ok(a) => a,
        Err(e) => {
            eprintln!("Error: Failed to initialize agent: {}", e);
            process::exit(1);
        }
    };

    // Initialize stats
    let mut stats = agent::AgentStats::new();

    // Determine mode
    if let Some(prompt) = &config.one_shot_prompt {
        // One-shot mode
        if config.debug {
            eprintln!("[DEBUG] Running in one-shot mode");
        }

            match agent.run_once(prompt).await {
            Ok(response) => {
                if !response.reasoning.is_empty() {
                    println!("\n{}[Thought Process]{}\n{}",
                        colors::get_color("cyan"), colors::get_color("reset"),
                        response.reasoning);
                }
                println!("{}", response.final_answer);

                // Update stats
                stats.input_tokens += response.input_tokens;
                stats.output_tokens += response.output_tokens;
                stats.tool_calls += response.tool_calls;
                stats.failed_calls += response.failed_tool_calls;
                stats.iterations += response.iterations;

                // One-shot mode ends after first response
                if config.debug {
                    eprintln!("[DEBUG] One-shot complete");
                    eprintln!("[DEBUG] Tokens: input={}, output={}, total={}",
                        response.input_tokens, response.output_tokens,
                        response.input_tokens + response.output_tokens);
                }
            }
            Err(e) => {
                eprintln!("Error: {}", e);
                process::exit(1);
            }
        }
    } else if config.goal_mode {
        // Goal-directed mode
        if let Some(goal) = &config.goal {
            if config.debug {
                eprintln!("[DEBUG] Running in goal-directed mode: '{}'", goal);
            }

            tui.enable_goal_mode(goal);
            stats.max_context = config.max_context;

            agent.run_goal_mode(goal, &mut stats).await;
        }
    } else if is_tty() {
        // Interactive mode
        if config.debug {
            eprintln!("[DEBUG] Running in interactive mode");
        }

        tui.set_context_size(stats.context_size);
        stats.max_context = config.max_context;

        // Show help on first run
        tui.set_show_help(true);
        tui.display_help();
        tui.set_show_help(false);

        loop {
            tui.display_prompt();
            let input = match tui.read_line_raw() {
                Some(line) => line.trim().to_string(),
                None => break, // EOF
            };

            if input.is_empty() {
                continue;
            }

            // Handle commands
            if input.starts_with('/') {
                if tui.handle_command(&input, &mut agent, &mut stats) {
                    break;
                }
                continue;
            }

            // Normal input
            tui.add_to_history(&input);
            tui.display_no_echo();
            tui.process_input(&input, &mut agent, &mut stats);

            // Update context size
            tui.set_context_size(stats.context_size);
        }
    } else {
        // Pipe mode - read from stdin
        if config.debug {
            eprintln!("[DEBUG] Running in pipe mode");
        }

        let mut input = String::new();
        std::io::stdin().read_line(&mut input).ok();

        if !input.is_empty() {
            input = input.trim().to_string();
            if let Ok(response) = agent.run_once(&input).await {
                if !response.reasoning.is_empty() {
                    println!("\n{}[Thought Process]{}\n{}",
                        colors::get_color("cyan"), colors::get_color("reset"),
                        response.reasoning);
                }
                println!("{}", response.final_answer);

                stats.input_tokens += response.input_tokens;
                stats.output_tokens += response.output_tokens;
                stats.tool_calls += response.tool_calls;
                stats.failed_calls += response.failed_tool_calls;
                stats.iterations += response.iterations;
            } else {
                eprintln!("Error: Agent encountered an error");
                process::exit(1);
            }
        }
    }

    // Print final stats
    if config.debug || !stats.input_tokens + stats.output_tokens == 0 {
        let total_tokens = stats.input_tokens + stats.output_tokens;
        if total_tokens > 0 {
            println!("\n{}[Session Summary]{}",
                colors::get_color("cyan"), colors::get_color("reset"));
            println!("  Input Tokens:  {}", stats.input_tokens);
            println!("  Output Tokens: {}", stats.output_tokens);
            println!("  Total Tokens:  {}", total_tokens);
            println!("  Tool Calls:    {}", stats.tool_calls);
            println!("  Failed Calls:  {}", stats.failed_calls);
            println!("  Iterations:    {}", stats.iterations);
            if stats.elapsed_secs > 0.0 {
                println!("  Duration:      {}", format_duration(stats.elapsed_secs));
                let tps = total_tokens as f64 / stats.elapsed_secs;
                println!("  Tokens/Second: {:.1}", tps);
            }
        }
    }

    process::exit(0);
}

fn format_duration(secs: f64) -> String {
    if secs < 60.0 {
        format!("{:.0}s", secs)
    } else if secs < 3600.0 {
        format!("{:.0}m {:.0}s", secs / 60.0, secs % 60.0)
    } else {
        format!("{:.0}h {:.0}m {:.0}s", secs / 3600.0, (secs % 3600.0) / 60.0, secs % 60.0)
    }
}

/// Redact API key for logging
fn redact_key(key: &str) -> String {
    if key.len() <= 8 {
        "***".to_string()
    } else {
        format!("{}...{}", &key[..4], &key[key.len()-4..])
    }
}
