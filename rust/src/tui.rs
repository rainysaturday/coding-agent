#![allow(dead_code)]
// TUI module - handles interactive prompt, history, navigation, and real-time display
// Implements requirements: 002, 017, 019, 020, 021, 022, 027, 038
use crate::colors;
use crate::inference::StreamingContentType;
use crate::tools::ToolResult;
use std::io::{self, Write, stdout};
use std::sync::{Arc, Mutex};
use std::collections::VecDeque;

use crossterm::{
    event::{self, Event, KeyCode, KeyEvent, KeyModifiers},
    terminal::{disable_raw_mode, enable_raw_mode},
};

/// Internal TUI state protected by a Mutex
struct TUIState {
    theme: colors::ColorTheme,
    history: VecDeque<String>,
    current_input: String,
    history_index: Option<usize>,
    max_history: usize,
    context_size: usize,
    max_context: usize,
    goal_active: bool,
    goal_prompt: String,
    show_help: bool,
}

/// TUI state for interactive mode (cloneable via Arc)
#[derive(Clone)]
pub struct TUI {
    state: Arc<Mutex<TUIState>>,
}

impl TUI {
    pub fn new() -> Self {
        TUI {
            state: Arc::new(Mutex::new(TUIState {
                theme: colors::ColorTheme::new(),
                history: VecDeque::new(),
                current_input: String::new(),
                history_index: None,
                max_history: 1000,
                context_size: 0,
                max_context: 128000,
                goal_active: false,
                goal_prompt: String::new(),
                show_help: false,
            })),
        }
    }

    pub fn with_max_context(self, max_context: usize) -> Self {
        {
            let mut state = self.state.lock().unwrap();
            state.max_context = max_context;
        }
        self
    }

    pub fn set_context_size(&self, size: usize) {
        let mut state = self.state.lock().unwrap();
        state.context_size = size;
    }

    pub fn enable_goal_mode(&self, goal: &str) {
        let mut state = self.state.lock().unwrap();
        state.goal_active = true;
        state.goal_prompt = goal.to_string();
        drop(state);
        self.display_goal_active();
    }

    pub fn disable_goal_mode(&self) {
        let mut state = self.state.lock().unwrap();
        state.goal_active = false;
        state.goal_prompt.clear();
        drop(state);
        self.display_goal_deactivated();
    }

    pub fn is_goal_active(&self) -> bool {
        let state = self.state.lock().unwrap();
        state.goal_active
    }

    pub fn get_goal_prompt(&self) -> String {
        let state = self.state.lock().unwrap();
        state.goal_prompt.clone()
    }

    pub fn set_show_help(&self, show: bool) {
        let mut state = self.state.lock().unwrap();
        state.show_help = show;
    }

    pub fn get_context_size(&self) -> usize {
        let state = self.state.lock().unwrap();
        state.context_size
    }

    pub fn get_max_context(&self) -> usize {
        let state = self.state.lock().unwrap();
        state.max_context
    }

    pub fn display_help(&self) {
        println!("{}", colors::get_color("cyan"));
        println!("Commands:");
        println!("  <text>            - Send a message to the agent");
        println!("  /stats            - Display runtime statistics");
        println!("  /context          - Show context information");
        println!("  /goal <prompt>    - Enter goal-directed mode");
        println!("  /goal-off         - Exit goal-directed mode");
        println!("  /clear            - Clear the screen");
        println!("  /clear-history    - Clear input history");
        println!("  exit, quit, :q    - Exit the application");
        println!("  help, :help, ?    - Show this help");
        println!("{}", colors::get_color("reset"));
    }

    pub fn display_prompt(&self) {
        let state = self.state.lock().unwrap();
        let pct = if state.max_context > 0 {
            (state.context_size as f64 / state.max_context as f64) * 100.0
        } else {
            0.0
        };

        let context_color = if pct < 50.0 {
            colors::get_color("green")
        } else if pct < 75.0 {
            colors::get_color("yellow")
        } else if pct < 90.0 {
            colors::get_color("yellow")
        } else {
            colors::get_color("red")
        };

        let context_indicator = if pct >= 90.0 { " ⚠" } else { "" };

        let goal_indicator = if state.goal_active {
            format!(" {}[GOAL]{}", colors::get_color("magenta"), colors::get_color("reset"))
        } else {
            String::new()
        };

        print!("\x1b[2K\r");
        print!("{}[Tokens: {}/{}]{}{} > {}",
            context_color, state.context_size, state.max_context,
            context_indicator, goal_indicator,
            colors::get_color("reset"));

        if let Some(idx) = state.history_index {
            if idx < state.history.len() {
                let hist: Vec<String> = state.history.iter().cloned().collect();
                let pos = hist.len().saturating_sub(1).saturating_sub(idx);
                if pos < hist.len() {
                    print!("{}", hist[pos]);
                }
            }
        } else {
            print!("{}", state.current_input);
        }

        io::stdout().flush().ok();
    }

    pub fn read_line(&self) -> Option<String> {
        let mut buffer = String::new();
        match io::stdin().read_line(&mut buffer) {
            Ok(n) if n > 0 => {
                if buffer.ends_with('\n') {
                    buffer.pop();
                }
                if buffer.ends_with('\r') {
                    buffer.pop();
                }
                if buffer.is_empty() {
                    return None;
                }
                Some(buffer)
            }
            _ => None,
        }
    }

    /// Read a line with raw terminal input (supports arrow key history navigation)
    pub fn read_line_raw(&self) -> Option<String> {
        // Enter raw mode
        if let Err(e) = enable_raw_mode() {
            eprintln!("Warning: Could not enable raw mode: {}", e);
            return self.read_line(); // Fallback to regular mode
        }

        let mut buffer = String::new();
        let result = self.read_line_internal(&mut buffer);

        // Exit raw mode
        let _ = disable_raw_mode();

        result
    }

    fn read_line_internal(&self, buffer: &mut String) -> Option<String> {
        loop {
            match event::read() {
                Ok(Event::Key(KeyEvent {
                    code: KeyCode::Enter,
                    modifiers: KeyModifiers::NONE,
                    ..
                })) => {
                    println!(); // Move to next line
                    io::stdout().flush().ok();

                    if buffer.is_empty() {
                        return None;
                    }
                    return Some(buffer.clone());
                }
                Ok(Event::Key(KeyEvent {
                    code: KeyCode::Char('c'),
                    modifiers: KeyModifiers::CONTROL,
                    ..
                })) => {
                    println!("\n^C");
                    io::stdout().flush().ok();
                    return None;
                }
                Ok(Event::Key(KeyEvent {
                    code: KeyCode::Up,
                    modifiers: KeyModifiers::NONE,
                    ..
                })) => {
                    self.navigate_history(1);
                    self.display_prompt();
                    buffer.clear();
                    return self.read_line_internal(buffer);
                }
                Ok(Event::Key(KeyEvent {
                    code: KeyCode::Down,
                    modifiers: KeyModifiers::NONE,
                    ..
                })) => {
                    self.navigate_history(-1);
                    self.display_prompt();
                    buffer.clear();
                    return self.read_line_internal(buffer);
                }
                Ok(Event::Key(KeyEvent {
                    code: KeyCode::Char(c),
                    modifiers: KeyModifiers::NONE,
                    ..
                })) => {
                    buffer.push(c);
                    print!("{}", c);
                    io::stdout().flush().ok();
                }
                Ok(Event::Key(KeyEvent {
                    code: KeyCode::Backspace,
                    modifiers: KeyModifiers::NONE,
                    ..
                })) => {
                    if !buffer.is_empty() {
                        buffer.pop();
                        // Redraw the prompt with the modified input
                        self.redraw_prompt(&buffer);
                    }
                }
                Ok(Event::Key(KeyEvent {
                    code: KeyCode::Left,
                    modifiers: KeyModifiers::NONE,
                    ..
                })) => {
                    // Move cursor left (visual only, since we don't have a cursor buffer)
                    // For simplicity, we'll just ignore cursor movement
                }
                Ok(Event::Key(KeyEvent {
                    code: KeyCode::Right,
                    modifiers: KeyModifiers::NONE,
                    ..
                })) => {
                    // Move cursor right
                }
                _ => {
                    // Ignore other key events
                }
            }
        }
    }

    fn redraw_prompt(&self, input: &str) {
        // Clear the current line and redraw with new input
        print!("\x1b[2K\r");
        print!("{}[Tokens: {}/{}]{}{} > {}",
            colors::get_color("green"), 0, 128000, "",
            colors::get_color("reset"), input);
        io::stdout().flush().ok();
    }

    fn navigate_history(&self, direction: i32) {
        let mut state = self.state.lock().unwrap();

        match direction {
            1 => { // Up - go to older entries
                let idx = state.history_index.map_or(0, |i| i + 1);
                if idx < state.history.len() {
                    state.history_index = Some(idx);
                    // Store current input before navigating away
                    if state.current_input.is_empty() {
                        // Only store if we're starting navigation
                    }
                }
            }
            -1 => { // Down - go to newer entries
                match state.history_index {
                    Some(0) => {
                        state.history_index = None;
                        state.current_input = String::new();
                    }
                    Some(i) => {
                        if i > 0 {
                            state.history_index = Some(i - 1);
                        } else {
                            state.history_index = None;
                            state.current_input = String::new();
                        }
                    }
                    None => {}
                }
            }
            _ => {}
        }

        // Update current_input based on history position
        if let Some(idx) = state.history_index {
            let hist: Vec<String> = state.history.iter().cloned().collect();
            if idx < hist.len() {
                let pos = hist.len().saturating_sub(1).saturating_sub(idx);
                if pos < hist.len() {
                    state.current_input = hist[pos].clone();
                }
            }
        } else {
            state.current_input = String::new();
        }
    }

    pub fn add_to_history(&self, input: &str) {
        let mut state = self.state.lock().unwrap();
        state.history.push_back(input.to_string());
        if state.history.len() > state.max_history {
            state.history.pop_front();
        }
        state.history_index = None;
        state.current_input = input.to_string();
    }

    pub fn display_no_echo(&self) {
        println!();
        io::stdout().flush().ok();
    }

    pub fn display_tool_feedback(&self, result: &ToolResult) {
        if result.success {
            let output = &result.output;
            let lines: Vec<&str> = output.lines().collect();
            let display_lines = if lines.len() > 5 {
                let start = lines.len() - 5;
                let mut display = vec![format!("... [output truncated - {} total lines]", lines.len())];
                display.extend(lines[start..].iter().map(|&l| l.to_string()));
                display.join("\n")
            } else {
                output.clone()
            };
            println!("{}Tool '{}' executed successfully:{}\n{}",
                colors::get_color("green"), result.output.chars().next().unwrap_or(' '),
                colors::get_color("reset"), display_lines);
        } else {
            println!("{}Tool '{}' failed: {}{}",
                colors::get_color("red"),
                result.output.chars().next().unwrap_or(' '),
                result.error.as_deref().unwrap_or("unknown error"),
                colors::get_color("reset"));
        }
    }

    pub fn display_streaming_chunk(&self, content: &str, content_type: StreamingContentType) {
        match content_type {
            StreamingContentType::Normal => {
                print!("{}", content);
            }
            StreamingContentType::Reasoning => {
                print!("{}{}", colors::get_color("dim"), content);
            }
            StreamingContentType::Goal => {
                print!("{}{}", colors::get_color("magenta"), content);
            }
            StreamingContentType::Compression => {
                print!("{}{}", colors::get_color("yellow"), content);
            }
        }
        io::stdout().flush().ok();
    }

    pub fn flush(&self) {
        io::stdout().flush().ok();
    }

    pub fn display_goal_active(&self) {
        let state = self.state.lock().unwrap();
        println!("{}[Goal mode activated: \"{}\"]{}",
            colors::get_color("magenta"), state.goal_prompt, colors::get_color("reset"));
    }

    pub fn display_goal_deactivated(&self) {
        println!("{}[Goal mode deactivated]{}",
            colors::get_color("magenta"), colors::get_color("reset"));
    }

    pub fn display_stats(&self, _agent: &crate::agent::Agent, stats: &crate::agent::AgentStats) {
        println!("{}", colors::get_color("cyan"));
        println!("==================================================");
        println!("  Runtime Statistics");
        println!("=================================================={}", colors::get_color("reset"));
        println!("  Input Tokens:      {}", stats.input_tokens);
        println!("  Output Tokens:     {}", stats.output_tokens);
        println!("  Current Context:   {}/{} tokens ({:.1}%)",
            stats.context_size, stats.max_context,
            if stats.max_context > 0 {
                (stats.context_size as f64 / stats.max_context as f64) * 100.0
            } else {
                0.0
            });
        if stats.elapsed_secs > 0.0 {
            let tps = (stats.input_tokens + stats.output_tokens) as f64 / stats.elapsed_secs;
            println!("  Tokens/Second:     {:.1}", tps);
        }
        println!("  Tool Calls:        {}", stats.tool_calls);
        println!("  Failed Calls:      {}", stats.failed_calls);
        println!("  Iterations:        {}", stats.iterations);
        println!("  Uptime:            {}", format_duration(stats.elapsed_secs));
        println!("==================================================");
    }

    pub fn display_context_info(&self) {
        println!("{}", colors::get_color("cyan"));
        println!("==================================================");
        println!("  Context Information");
        println!("=================================================={}", colors::get_color("reset"));
        let state = self.state.lock().unwrap();
        let pct = if state.max_context > 0 {
            (state.context_size as f64 / state.max_context as f64) * 100.0
        } else {
            0.0
        };
        println!("  Current: {}/{} tokens ({:.1}%)",
            state.context_size, state.max_context, pct);
        let indicator = if pct >= 90.0 { " ⚠⚠⚠ CRITICAL" }
            else if pct >= 75.0 { " ⚠⚠" }
            else if pct >= 50.0 { " ⚠" }
            else { "" };
        println!("  Status:{}", indicator);
        println!("==================================================");
    }

    pub fn handle_command(
        &self,
        input: &str,
        agent: &mut crate::agent::Agent,
        stats: &mut crate::agent::AgentStats,
    ) -> bool {
        let parts: Vec<&str> = input[1..].splitn(2, ' ').collect();
        let cmd = parts[0];
        let args = if parts.len() > 1 { parts[1].trim() } else { "" };

        match cmd {
            "stats" | ":stats" => {
                self.display_stats(agent, stats);
            }
            "clear" | ":clear" => {
                print!("\x1b[2J\x1b[H");
                stdout().flush().ok();
            }
            "clear-history" | ":clear-history" => {
                let mut state = self.state.lock().unwrap();
                state.history.clear();
                println!("{}History cleared.{}", colors::get_color("yellow"), colors::get_color("reset"));
            }
            "exit" | "quit" | ":q" | ":q!" | ":wq" => {
                return true;
            }
            "help" | ":help" | "/help" | "?" => {
                self.display_help();
            }
            "goal" => {
                if args.is_empty() {
                    println!("{}Usage: /goal <prompt>{}", colors::get_color("red"), colors::get_color("reset"));
                } else {
                    self.enable_goal_mode(args);
                    crate::config::set_goal_mode(self, true);
                }
            }
            "goal-off" | ":goal-off" => {
                self.disable_goal_mode();
                crate::config::set_goal_mode(self, false);
            }
            "context" | ":context" => {
                self.display_context_info();
            }
            _ => {
                println!("{}Unknown command: {}. Type 'help' for available commands.{}",
                    colors::get_color("red"), cmd, colors::get_color("reset"));
            }
        }
        false
    }

    pub fn process_input(&self, input: &str, agent: &mut crate::agent::Agent, stats: &mut crate::agent::AgentStats) {
        let response = futures::executor::block_on(agent.run_once(input));
        if let Ok(response) = response {
            // Display reasoning with better visibility
            if !response.reasoning.is_empty() {
                println!("\n{}[Thought Process]{}", colors::get_color("cyan"), colors::get_color("reset"));
                println!("{}", response.reasoning);
            }
            println!("{}", response.final_answer);
            self.update_stats(&response, stats);
            self.set_context_size(stats.context_size);

            if self.is_goal_active() {
                self.run_goal_check(agent, stats);
            }
        } else {
            println!("{}Error:{} Agent encountered an error",
                colors::get_color("red"), colors::get_color("reset"));
        }
    }

    pub fn process_goal_input(&self, input: &str, agent: &mut crate::agent::Agent, stats: &mut crate::agent::AgentStats) {
        let response = futures::executor::block_on(agent.run_once(input));
        if let Ok(response) = response {
            // Display reasoning with better visibility
            if !response.reasoning.is_empty() {
                println!("{}[Thought Process]{}\n{}",
                    colors::get_color("cyan"), colors::get_color("reset"),
                    response.reasoning);
            }
            if !response.final_answer.is_empty() {
                println!("{}", response.final_answer);
            }
            self.update_stats(&response, stats);
        }
    }

    pub fn run_goal_check(&self, agent: &mut crate::agent::Agent, stats: &mut crate::agent::AgentStats) {
        if !self.is_goal_active() {
            return;
        }
        let goal = self.get_goal_prompt();

        loop {
            println!("{}[Goal Check] Checking if goal is achieved: \"{}\"{}",
                colors::get_color("magenta"), goal, colors::get_color("reset"));

            let check_prompt = format!(
                "Please review the current state of your work. Have you achieved the following goal?\n\nGoal: {}\n\nIf you have achieved the goal, respond with 'goal achieved'.\nIf you have not achieved the goal, explain what remains to be done and continue working.",
                goal
            );

            let response = futures::executor::block_on(agent.run_once(&check_prompt));
            if let Ok(response) = response {
                if response.final_answer.to_lowercase().contains("goal achieved") {
                    println!("\n{}[Goal Achieved] ✓ Goal has been achieved!{}",
                        colors::get_color("magenta"), colors::get_color("reset"));
                    self.disable_goal_mode();
                    crate::config::set_goal_mode(self, false);
                    self.update_stats(&response, stats);
                    return;
                } else {
                    if !response.reasoning.is_empty() {
                        println!("{}[Reasoning]{}\n{}",
                            colors::get_color("dim"), colors::get_color("reset"),
                            response.reasoning);
                    }
                    println!("{}", response.final_answer);
                    self.update_stats(&response, stats);
                    self.set_context_size(stats.context_size);
                }
            } else {
                println!("{}Error: Goal check failed{}",
                    colors::get_color("red"), colors::get_color("reset"));
                break;
            }
        }
    }

    fn update_stats(&self, response: &crate::agent::AgentResponse, stats: &mut crate::agent::AgentStats) {
        stats.input_tokens += response.input_tokens;
        stats.output_tokens += response.output_tokens;
        stats.tool_calls += response.tool_calls;
        stats.failed_calls += response.failed_tool_calls;
        stats.iterations += response.iterations;
    }
}

impl Default for TUI {
    fn default() -> Self {
        Self::new()
    }
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

/// Check if stdin is a TTY
pub fn is_tty() -> bool {
    atty::is(atty::Stream::Stdin)
}
