// Agent module - core agent logic, message management, tool execution
// Implements requirements: 003, 004, 015, 018, 024, 027, 034, 035
use std::time::Instant;
use crate::config::Config;
use crate::inference::{InferenceEngine, Message};
use crate::tools::{Tools, ToolResult};
use crate::tui::TUI;

/// Runtime statistics
#[derive(Debug, Clone)]
#[allow(dead_code)]
pub struct AgentStats {
    pub input_tokens: usize,
    pub output_tokens: usize,
    pub tool_calls: usize,
    pub failed_calls: usize,
    pub iterations: usize,
    pub elapsed_secs: f64,
    pub context_size: usize,
    pub max_context: usize,
    pub start_time: Instant,
}

impl AgentStats {
    pub fn new() -> Self {
        AgentStats {
            input_tokens: 0,
            output_tokens: 0,
            tool_calls: 0,
            failed_calls: 0,
            iterations: 0,
            elapsed_secs: 0.0,
            context_size: 0,
            max_context: 128000,
            start_time: Instant::now(),
        }
    }
}

/// Response from the agent after processing a prompt
#[derive(Debug)]
pub struct AgentResponse {
    pub final_answer: String,
    pub reasoning: String,
    pub input_tokens: usize,
    pub output_tokens: usize,
    pub tool_calls: usize,
    pub failed_tool_calls: usize,
    pub iterations: usize,
}

/// The main agent struct
pub struct Agent {
    config: Config,
    tools: Tools,
    engine: InferenceEngine,
    tui: TUI,
    messages: Vec<Message>,
}

impl Agent {
    pub fn new(config: Config, tui: TUI) -> Result<Self, String> {
        let engine = InferenceEngine::new(config.clone());
        let tools = Tools::new(config.clone(), tui.clone());

        Ok(Agent {
            config,
            tools,
            engine,
            tui,
            messages: Vec::new(),
        })
    }

    /// Run the agent once with a single user prompt (async)
    pub async fn run_once(&mut self, user_prompt: &str) -> Result<AgentResponse, String> {
        let _start = Instant::now();
        let mut stats = AgentStats::new();
        stats.max_context = self.config.max_context;

        // Add user message
        self.messages.push(Message {
            role: "user".to_string(),
            content: user_prompt.to_string(),
        });

        // Run the main loop
        let response = self.run_loop(&mut stats).await?;

        Ok(response)
    }

    /// Run the agent in goal-directed mode (async)
    pub async fn run_goal_mode(
        &mut self,
        goal: &str,
        stats: &mut AgentStats,
    ) {
        let start = Instant::now();

        // Add goal as first message
        self.messages.push(Message {
            role: "user".to_string(),
            content: format!("GOAL: {}\n\nPlease work towards achieving this goal. You have access to tools for file operations, shell execution, git commands, and image viewing. Work autonomously and use tools as needed.", goal),
        });

        // Run loop with goal mode
        loop {
            stats.iterations += 1;
            if stats.iterations > self.config.max_iterations {
                println!("{}[Max iterations reached: {}]{}",
                    crate::colors::get_color("yellow"), self.config.max_iterations,
                    crate::colors::get_color("reset"));
                break;
            }

            // Check context compression
            if self.config.enable_compression {
                let estimated = self.engine.estimate_tokens(&self.messages);
                if estimated > self.config.max_context {
                    println!("{}[Context full: {} tokens] Compressing...{}",
                        crate::colors::get_color("yellow"), estimated,
                        crate::colors::get_color("reset"));

                    match self.engine.compress_context(
                        &self.messages,
                        self.config.compression_target_size,
                    ).await {
                        Ok(compressed) => {
                            self.messages = compressed;
                            println!("{}[Context compressed]{}",
                                crate::colors::get_color("green"),
                                crate::colors::get_color("reset"));
                        }
                        Err(e) => {
                            println!("{}[Compression failed: {}]{}",
                                crate::colors::get_color("red"), e,
                                crate::colors::get_color("reset"));
                        }
                    }
                }
            }

            // Get LLM response
            let response = match if self.config.disable_streaming {
                self.engine.infer(&self.messages).await
            } else {
                self.engine.infer_streaming(&self.messages, self.tui.clone()).await
            } {
                Ok(r) => r,
                Err(e) => {
                    println!("{}[API Error: {}]{}",
                        crate::colors::get_color("red"), e,
                        crate::colors::get_color("reset"));

                    self.messages.push(Message {
                        role: "assistant".to_string(),
                        content: format!("Error: {}. Please retry.", e),
                    });
                    continue;
                }
            };

            // Update stats
            stats.input_tokens += response.token_usage.input_tokens;
            stats.output_tokens += response.token_usage.output_tokens;
            stats.context_size = self.engine.estimate_tokens(&self.messages);

            // Display reasoning with better visibility
            if !response.reasoning.is_empty() {
                println!("\n{}", "─".repeat(50));
                println!("{}[Thought Process]{}",
                    crate::colors::get_color("cyan"), crate::colors::get_color("reset"));
                println!("{}", response.reasoning);
                println!("{}", "─".repeat(50));
            }

            // Display final answer
            println!("{}", response.content);

            // Add assistant response to history
            self.messages.push(Message {
                role: "assistant".to_string(),
                content: response.content.clone(),
            });

            // Check for goal achievement
            if response.content.to_lowercase().contains("goal achieved") {
                println!("\n{}[Goal Achieved] ✓{}",
                    crate::colors::get_color("magenta"), crate::colors::get_color("reset"));
                break;
            }

            stats.elapsed_secs = start.elapsed().as_secs_f64();
        }

        stats.elapsed_secs = start.elapsed().as_secs_f64();
    }

    /// Run the main agent loop: infer -> check for tools -> execute -> repeat
    async fn run_loop(&mut self, stats: &mut AgentStats) -> Result<AgentResponse, String> {
         let max_iterations = self.config.max_iterations;
        let mut tool_calls_in_turn = 0;
        let max_tool_calls_per_turn = 10;

        loop {
            stats.iterations += 1;
            if stats.iterations > max_iterations {
                return Err(format!("Maximum iterations ({}) reached", max_iterations));
            }

            // Check context compression
            if self.config.enable_compression {
                let estimated = self.engine.estimate_tokens(&self.messages);
                stats.context_size = estimated;

                if estimated > self.config.max_context {
                    println!("{}[Context full: {} tokens] Compressing...{}",
                        crate::colors::get_color("yellow"), estimated,
                        crate::colors::get_color("reset"));

                    match self.engine.compress_context(
                        &self.messages,
                        self.config.compression_target_size,
                    ).await {
                        Ok(compressed) => {
                            self.messages = compressed;
                            println!("{}[Context compressed]{}",
                                crate::colors::get_color("green"),
                                crate::colors::get_color("reset"));
                        }
                        Err(e) => {
                            println!("{}[Compression failed: {}]{}",
                                crate::colors::get_color("red"), e,
                                crate::colors::get_color("reset"));
                        }
                    }
                }
            }

            // Get LLM response
            let response = if self.config.disable_streaming {
                self.engine.infer(&self.messages).await?
            } else {
                self.engine.infer_streaming(&self.messages, self.tui.clone()).await?
            };

            // Update stats
            stats.input_tokens += response.token_usage.input_tokens;
            stats.output_tokens += response.token_usage.output_tokens;
            stats.context_size = self.engine.estimate_tokens(&self.messages);

          // Display reasoning with better visibility
            if !response.reasoning.is_empty() {
                println!("\n{}", "─".repeat(50));
                println!("{}[Thought Process]{}",
                    crate::colors::get_color("cyan"), crate::colors::get_color("reset"));
                println!("{}", response.reasoning);
                println!("{}", "─".repeat(50));
            }

            // Add assistant response to history
            self.messages.push(Message {
                role: "assistant".to_string(),
                content: response.content.clone(),
            });

            // Check for tool calls
            if self.has_tool_calls(&response.content) && tool_calls_in_turn < max_tool_calls_per_turn {
                // Parse and execute tool calls
                let tool_results = self.parse_and_execute_tools(&response.content);

                for result in &tool_results {
                    if result.success {
                        stats.tool_calls += 1;
                    } else {
                        stats.failed_calls += 1;
                        if self.config.show_all_errors {
                            if let Some(ref err) = result.error {
                                println!("{}[Tool Error] {}{}",
                                    crate::colors::get_color("red"), err,
                                    crate::colors::get_color("reset"));
                            }
                        }
                    }

                    // Display tool feedback
                    self.tui.display_tool_feedback(result);
                }

                // Add tool results to conversation
                for result in &tool_results {
                    self.messages.push(Message {
                        role: "user".to_string(),
                        content: format!("[Tool: {}] {}", result.tool_name,
                            if result.output.len() > 500 {
                                format!("{}... (truncated)", &result.output[..500])
                            } else {
                                result.output.clone()
                            }),
                    });
                }

                tool_calls_in_turn += 1;
                continue;
            }

            // No tool calls - return final answer
            return Ok(AgentResponse {
                final_answer: response.content,
                reasoning: response.reasoning,
                input_tokens: stats.input_tokens,
                output_tokens: stats.output_tokens,
                tool_calls: stats.tool_calls,
                failed_tool_calls: stats.failed_calls,
                iterations: stats.iterations,
            });
        }
    }

    /// Check if the response contains tool call instructions
    fn has_tool_calls(&self, content: &str) -> bool {
        let tool_keywords = [
            "tool_call(",
            "read_file(", "write_file(",
            "bash(", "list_files(", "grep(",
            "git_log(", "git_show(",
            "git_diff(", "insert_lines(",
            "replace_text(", "view_image(",
        ];
        tool_keywords.iter().any(|&k| content.contains(k))
    }

    /// Parse tool calls from response content
    fn parse_and_execute_tools(&self, content: &str) -> Vec<ToolResult> {
        let mut results = Vec::new();

        let tool_names = [
            "bash", "read_file", "write_file",
            "insert_lines", "replace_text", "list_files",
            "grep", "git_log", "git_show",
            "git_diff", "view_image",
        ];

        for line in content.lines() {
            let trimmed = line.trim();
            if trimmed.is_empty() {
                continue;
            }

            for tool_name in &tool_names {
                let pattern = format!("{}(", tool_name);
                if trimmed.starts_with(&pattern) || trimmed.starts_with(&format!("{}, ", tool_name)) {
                    // Extract arguments: everything after the opening paren
                    let args = if let Some(start) = trimmed.find('(') {
                        &trimmed[start + 1..trimmed.rfind(')').unwrap_or(trimmed.len())]
                    } else {
                        &trimmed[tool_name.len()..]
                    };

                    let result = futures::executor::block_on(self.tools.execute(tool_name, args));
                    results.push(result);
                    break;
                }
            }
        }

        results
    }
}
