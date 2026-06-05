#![allow(dead_code)]
// Inference engine - LLM API calls, streaming, and context management
// Implements requirements: 005, 006, 007, 008, 009, 037
use serde::{Deserialize, Serialize};
use futures::StreamExt;
use crate::config::Config;

/// Content type for streaming responses
#[derive(Debug, Clone)]
pub enum StreamingContentType {
    Normal,
    Reasoning,
    Goal,
    Compression,
}

/// Message in a conversation
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Message {
    pub role: String,
    pub content: String,
}

/// Token usage information
#[derive(Debug, Clone, Default)]
pub struct TokenUsage {
    pub input_tokens: usize,
    pub output_tokens: usize,
}

impl TokenUsage {
    pub fn total(&self) -> usize {
        self.input_tokens + self.output_tokens
    }
}

/// Response from LLM inference
#[derive(Debug, Clone, Default)]
pub struct Response {
    pub content: String,
    pub reasoning: String,
    pub token_usage: TokenUsage,
    pub tool_calls: Option<Vec<String>>,
}

/// LLM inference engine
pub struct InferenceEngine {
    config: Config,
}

impl InferenceEngine {
    pub fn new(config: Config) -> Self {
        InferenceEngine { config }
    }

    /// Run inference with a list of messages (non-streaming)
    pub async fn infer(&self, messages: &[Message]) -> Result<Response, String> {
        let system_prompt = self.build_system_prompt();

        let mut api_messages = vec![Message {
            role: "system".to_string(),
            content: system_prompt,
        }];
        api_messages.extend(messages.iter().cloned());

        let body = serde_json::json!({
            "model": self.config.model,
            "messages": api_messages,
            "max_tokens": 4096,
            "temperature": 0.7,
            "stream": false,
        });

        let client = reqwest::Client::new();
        let response = client.post(&self.config.endpoint)
            .bearer_auth(&self.config.api_key)
            .header("Content-Type", "application/json")
            .json(&body)
            .send()
            .await
            .map_err(|e| format!("API request failed: {}", e))?;

        if !response.status().is_success() {
            let status = response.status();
            let body_text = response.text().await.unwrap_or_default();
            return Err(format!("API error {}: {}", status, body_text));
        }

        let api_response: serde_json::Value = response.json().await
            .map_err(|e| format!("Failed to parse response: {}", e))?;

        let content = api_response["choices"][0]["message"]["content"]
            .as_str().unwrap_or("").to_string();

        let mut reasoning = String::new();
        if let Some(rc) = api_response["choices"][0]["message"]["reasoning"].as_str() {
            reasoning = rc.to_string();
        } else if let Some(tc) = api_response["choices"][0]["message"]["tool_calls"].as_array() {
            reasoning = format!("[Tool calls detected: {} tool(s)]", tc.len());
        }

        let mut token_usage = TokenUsage::default();
        let usage_value = &api_response["usage"];
        if usage_value.is_object() {
            token_usage.input_tokens = usage_value["prompt_tokens"].as_u64().unwrap_or(0) as usize;
            token_usage.output_tokens = usage_value["completion_tokens"].as_u64().unwrap_or(0) as usize;
        }

        Ok(Response { content, reasoning, token_usage, tool_calls: None })
    }

    /// Run streaming inference - sends chunks to TUI as they arrive
    pub async fn infer_streaming(
        &self,
        messages: &[Message],
        tui: crate::tui::TUI,
    ) -> Result<Response, String> {
        let system_prompt = self.build_system_prompt();

        let mut api_messages = vec![Message {
            role: "system".to_string(),
            content: system_prompt,
        }];
        api_messages.extend(messages.iter().cloned());

        let body = serde_json::json!({
            "model": self.config.model,
            "messages": api_messages,
            "max_tokens": 4096,
            "temperature": 0.7,
            "stream": true,
            "stream_options": { "include_usage": true },
        });

        let client = reqwest::Client::new();
        let response = client.post(&self.config.endpoint)
            .bearer_auth(&self.config.api_key)
            .header("Content-Type", "application/json")
            .json(&body)
            .send()
            .await
            .map_err(|e| format!("API request failed: {}", e))?;

        if !response.status().is_success() {
            let status = response.status();
            let body_text = response.text().await.unwrap_or_default();
            return Err(format!("API error {}: {}", status, body_text));
        }

        // Show streaming indicator
        tui.display_streaming_chunk("\n[Streaming...]\n", StreamingContentType::Normal);

        // Streaming body - accumulate SSE lines and process them
        let mut full_content = String::new();
        let mut full_reasoning = String::new();
        let mut current_in_reasoning = false;
        let mut total_input_tokens: usize = 0;
        let mut total_output_tokens: usize = 0;
        let mut buffer = Vec::new();

        let mut stream = response.bytes_stream();
        while let Some(chunk_result) = stream.next().await {
            let chunk = chunk_result
                .map_err(|e| format!("Stream error: {}", e))?;

            buffer.extend_from_slice(&chunk);

            // Process all complete lines in the buffer
            loop {
                // Find the position of the next newline
                let newline_pos = match buffer.iter().position(|&b| b == b'\n') {
                    Some(pos) => pos,
                    None => break, // No complete line yet, wait for more data
                };

                let line = String::from_utf8_lossy(&buffer[..newline_pos]).trim().to_string();

                // Remove the processed line (including the newline)
                let end = if newline_pos + 1 < buffer.len() && buffer[newline_pos + 1] == b'\r' {
                    newline_pos + 2
                } else {
                    newline_pos + 1
                };
                buffer.drain(..end);

                if line.is_empty() {
                    continue;
                }

                if line.starts_with("data: ") {
                    let data = &line[6..];
                    if data == "[DONE]" {
                        break;
                    }

                    if let Ok(json) = serde_json::from_str::<serde_json::Value>(data) {
                        // Check for reasoning content (OpenAI o1/o3 models)
                        if let Some(reasoning_text) = json["delta"]["reasoning"].as_str() {
                            if !reasoning_text.is_empty() {
                                full_reasoning.push_str(reasoning_text);
                                tui.display_streaming_chunk(reasoning_text, StreamingContentType::Reasoning);
                            }
                            continue;
                        }

                        // Check for content
                        if let Some(delta_content) = json["delta"]["content"].as_str() {
                            if delta_content.is_empty() {
                                continue;
                            }

                            // Check for thinking/reasoning tags
                            if delta_content.contains("<thinking>") || delta_content.contains("<reasoning>") {
                                current_in_reasoning = true;
                                continue;
                            }
                            if delta_content.contains("</thinking>") || delta_content.contains("</reasoning>") {
                                current_in_reasoning = false;
                                continue;
                            }

                            if current_in_reasoning {
                                full_reasoning.push_str(delta_content);
                                tui.display_streaming_chunk(delta_content, StreamingContentType::Reasoning);
                            } else {
                                full_content.push_str(delta_content);
                                tui.display_streaming_chunk(delta_content, StreamingContentType::Normal);
                            }
                        }

                        // Check for token usage
                        if let Some(usage) = json["usage"].as_object() {
                            if let Some(inp) = usage["prompt_tokens"].as_u64() {
                                total_input_tokens = inp as usize;
                            }
                            if let Some(out) = usage["completion_tokens"].as_u64() {
                                total_output_tokens = out as usize;
                            }
                        }
                    }
                }
            }
        }

        let token_usage = if total_input_tokens > 0 || total_output_tokens > 0 {
            TokenUsage {
                input_tokens: total_input_tokens,
                output_tokens: total_output_tokens,
            }
        } else {
            TokenUsage {
                input_tokens: messages.iter().map(|m| m.content.chars().count() / 4).sum(),
                output_tokens: full_content.chars().count() / 4,
            }
        };

        // Flush any remaining content
        tui.flush();

        Ok(Response { content: full_content, reasoning: full_reasoning, token_usage, tool_calls: None })
    }


    /// Compress context by summarizing older messages
    pub async fn compress_context(
        &self,
        messages: &[Message],
        _target_size: usize,
    ) -> Result<Vec<Message>, String> {
        if messages.len() <= 3 {
            return Ok(messages.to_vec());
        }

        let system = messages.first().cloned();
        let keep_last = 3;
        let keep_first_user = 1;

        let mut compressed = Vec::new();
        if let Some(sys) = system {
            compressed.push(sys);
        }

        if keep_first_user < messages.len() - keep_last {
            let middle = &messages[keep_first_user..messages.len() - keep_last];
            let summary = self.summarize_messages(middle).await?;
            compressed.push(Message {
                role: "system".to_string(),
                content: format!("[Context Summary: {}]", summary),
            });
        }

        if messages.len() > keep_last {
            compressed.extend(messages[messages.len() - keep_last..].to_vec());
        }

        Ok(compressed)
    }

    async fn summarize_messages(&self, messages: &[Message]) -> Result<String, String> {
        let summary_prompt = format!(
            "Summarize the following conversation exchange in 2-3 sentences:\n\n{}",
            messages.iter().map(|m| format!("[{}]: {}", m.role, m.content)).collect::<Vec<_>>().join("\n")
        );

        let response = self.infer(&[
            Message {
                role: "system".to_string(),
                content: "You are a conversation summarizer. Provide concise summaries of conversations.".to_string(),
            },
            Message {
                role: "user".to_string(),
                content: summary_prompt,
            }
        ]).await?;

        Ok(response.content)
    }

    fn build_system_prompt(&self) -> String {
        if self.config.goal_mode {
            format!(
                "You are a coding agent with access to file system tools, git commands, and shell execution.\n\n\
                GOAL MODE ACTIVE: Your goal is: {}\n\
                You should work toward achieving this goal autonomously.\n\
                When you believe the goal is achieved, respond with 'Goal achieved'.",
                self.config.goal.as_deref().unwrap_or("none")
            )
        } else if self.config.read_only {
            "You are a coding agent in READ-ONLY MODE. You can read files, run commands, and view git history,\n\
            but you CANNOT write, create, edit, or delete any files. Only respond to read-only operations.".to_string()
        } else {
            "You are a coding agent with access to file system tools, git commands, and shell execution.\n\
            You can read files, write files, execute shell commands, use git, and view images.\n\
            Use tools when appropriate to accomplish the user's request.".to_string()
        }
    }

    pub fn estimate_tokens(&self, messages: &[Message]) -> usize {
        let total_chars: usize = messages.iter().map(|m| m.content.chars().count()).sum();
        total_chars / 4
    }
}
