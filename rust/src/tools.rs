#![allow(dead_code)]
// Tools module - implements all tool handlers for the coding agent
// Implements requirements: 014, 015, 016, 018, 028, 029, 030, 031, 032, 033, 034, 040, 041
use std::fs;
use std::io::Read;
use std::path::Path;
use std::process::Command;
use crate::config::Config;
use crate::tui::TUI;

/// Result of a tool execution
#[derive(Debug, Clone)]
pub struct ToolResult {
    pub success: bool,
    pub output: String,
    pub error: Option<String>,
    pub tool_name: String,
}

impl ToolResult {
    pub fn success(output: String, tool_name: &str) -> Self {
        ToolResult {
            success: true,
            output,
            error: None,
            tool_name: tool_name.to_string(),
        }
    }

    pub fn failure(error: String, tool_name: &str) -> Self {
        ToolResult {
            success: false,
            output: String::new(),
            error: Some(error),
            tool_name: tool_name.to_string(),
        }
    }
}

/// Documentation for a tool - used in the system prompt
pub struct ToolDoc {
    pub name: &'static str,
    pub description: &'static str,
    pub parameters: &'static str,
    pub how_to_call: &'static str,
    pub example_use_case: &'static str,
}

impl ToolDoc {
    pub fn to_prompt(&self) -> String {
        format!(
            "{}\n   Description: {}\n   Parameters:\n{}\n   How to call: {}\n   Example use case: \"{}\"",
            self.name, self.description, self.parameters, self.how_to_call, self.example_use_case
        )
    }
}

/// Available tools with their documentation
pub fn get_tool_docs() -> Vec<ToolDoc> {
    vec![
        ToolDoc {
            name: "bash",
            description: "Execute a bash command in the terminal",
            parameters: "     - command (string, required): The bash command to execute\n     - timeout (integer, optional): Timeout in milliseconds (default: 30000)",
            how_to_call: "Use the bash tool when you need to run shell commands, install packages, build projects, check file system, etc.",
            example_use_case: "ls -la",
        },
        ToolDoc {
            name: "read_file",
            description: "Read the contents of a file",
            parameters: "     - path (string, required): The path to the file to read",
            how_to_call: "Use read_file to view the contents of any file before making changes.",
            example_use_case: "Reading source files, configuration files, documentation",
        },
        ToolDoc {
            name: "read_lines",
            description: "Read a specific line range from a file",
            parameters: "     - path (string, required): The path to the file\n     - start (integer, required): The starting line number (1-indexed)\n     - end (integer, required): The ending line number (1-indexed)",
            how_to_call: "Use read_lines when you only need to view a portion of a large file.",
            example_use_case: "Viewing lines 1-50 of a large source file, checking specific sections",
        },
        ToolDoc {
            name: "write_file",
            description: "Write content to a file",
            parameters: "     - path (string, required): The path to the file to write\n     - content (string, required): The content to write to the file",
            how_to_call: "Use write_file to create new files or completely overwrite existing files.",
            example_use_case: "Creating new source files, writing configuration, saving output",
        },
        ToolDoc {
            name: "insert_lines",
            description: "Insert lines at a specific line number",
            parameters: "     - path (string, required): The path to the file\n     - line (integer, required): The line number where insertion should occur (1-indexed)\n     - lines (string, required): The lines to insert (use \\n for newlines)",
            how_to_call: "Use insert_lines to add new content without replacing existing content.",
            example_use_case: "Adding imports, inserting new functions, adding comments",
        },
        ToolDoc {
            name: "replace_text",
            description: "Find and replace text in a file by searching for a pattern",
            parameters: "     - path (string, required): File path to modify\n     - search (string, required): Text pattern to find (exact match, not regex)\n     - replace (string, required): Replacement text\n     - count (integer, optional): Number of occurrences to replace (default: 1, use -1 for all)",
            how_to_call: "Use replace_text when you know the text to find but not the line numbers.",
            example_use_case: "Renaming variables, updating function names, fixing typos throughout a file",
        },
        ToolDoc {
            name: "list_files",
            description: "List files in a directory",
            parameters: "     - path (string, required): The path to the directory to list",
            how_to_call: "Use list_files to see the contents of a directory.",
            example_use_case: "Checking what files exist in a directory before working with them",
        },
        ToolDoc {
            name: "grep",
            description: "Search for a pattern in files or directories",
            parameters: "     - pattern (string, required): The pattern to search for\n     - path (string, required): The file or directory to search in",
            how_to_call: "Use grep to find files or lines matching a specific pattern.",
            example_use_case: "grep 'fn main' src/",
        },
        ToolDoc {
            name: "git_log",
            description: "View git log history",
            parameters: "     - args (string, optional): Additional arguments to pass to git log",
            how_to_call: "Use git_log to view recent commit history.",
            example_use_case: "git_log",
        },
        ToolDoc {
            name: "git_show",
            description: "Show details of a git commit or diff",
            parameters: "     - ref (string, required): Git reference (commit hash, branch name, tag)",
            how_to_call: "Use git_show to examine a specific commit or diff.",
            example_use_case: "git_show HEAD",
        },
        ToolDoc {
            name: "git_diff",
            description: "Show git diff of uncommitted changes",
            parameters: "     - args (string, optional): Additional arguments to pass to git diff",
            how_to_call: "Use git_diff to view uncommitted changes.",
            example_use_case: "git_diff",
        },
        ToolDoc {
            name: "view_image",
            description: "Get information about an image file",
            parameters: "     - path (string, required): The path to the image file",
            how_to_call: "Use view_image to get details about an image file (name, type, size).",
            example_use_case: "view_image screenshot.png",
        },
        ToolDoc {
            name: "subagent",
            description: "Spawn a sub-agent to work on a task independently. The sub-agent runs as a separate process and returns only its conclusion/summary.",
            parameters: "     - prompt (string, required): The task description for the sub-agent\n     - persona (string, optional): A persona to give the sub-agent",
            how_to_call: "Use the subagent tool when you need to delegate a task to a specialized agent. The subagent will run independently and return its conclusion/summary.",
            example_use_case: "subagent",
        },
        ToolDoc {
            name: "patch",
            description: "Apply a patch (unified diff) to a file",
            parameters: "     - path (string, required): The path to the file to patch\n     - patch (string, required): The unified diff/patch content to apply",
            how_to_call: "Use patch when you have a unified diff and want to apply it to a file.",
            example_use_case: "patch with diff content",
        },
    ]
}

/// Generate the system prompt with tool definitions
pub fn generate_system_prompt(config: &Config) -> String {
    let tool_docs = get_tool_docs();
    let mut prompt = String::from("You are a helpful coding assistant. You have access to the following tools.\n\nAVAILABLE TOOLS:\n\n");

    for (i, doc) in tool_docs.iter().enumerate() {
        prompt.push_str(&format!("\n{}. {}\n", i + 1, doc.to_prompt()));
    }

    prompt.push_str("\n\nTOOL CALLING BEST PRACTICES:\n1. Always read a file first (using read_file or read_lines) to understand its contents\n2. When modifying files, be precise about what you're changing\n3. For multi-line content, properly format with \\n for newlines\n4. Verify your changes by re-reading files after writing\n5. Test code by running appropriate commands (go build, go test, cargo test, etc.)\n\nVERIFICATION REQUIREMENTS:\n- ALWAYS double-check your work before considering a task complete\n- Verify that created/modified files exist and contain the expected content\n- Test code execution when possible\n- Validate that changes meet the user's requirements\n- If you make multiple changes, verify each one independently\n- Re-read files after writing to confirm content was written correctly\n- Run validation commands (e.g., go vet, gofmt -d, pylint, eslint, cat to view files)\n- If verification fails, fix the issue and re-verify\n- Provide a final verification summary before concluding the task\n\nVerification Checklist:\n1. Files exist at the expected paths\n2. File content matches the intended changes\n3. Code compiles without errors (for Go code)\n4. Code follows formatting standards (gofmt, black, prettier, rustfmt, etc.)\n5. Changes align with user requirements\n6. No unintended side effects or broken dependencies\n");

    // Add persona if configured
    if !config.persona.is_empty() {
        prompt.push_str(&format!("\nYOUR PERSONA:\n{}\n", config.persona));
    }

    // Add summary-only instruction if needed
    if config.summary_only {
        prompt.push_str("\n\nIMPORTANT OUTPUT INSTRUCTION: You are running in summary-only mode. Your final output should be a concise summary/conclusion of the work completed. Do NOT include verbose explanations, step-by-step details, or code. Only provide the essential outcome and any critical findings.\n");
    }

    // Add environment info
    if let Ok(cwd) = std::env::current_dir() {
        if let Some(cwd_str) = cwd.to_str() {
            prompt.push_str(&format!("\nCURRENT WORKING DIRECTORY: {}\n", cwd_str));
        }
    }

    prompt
}

/// The main tools handler
pub struct Tools {
    config: Config,
    tui: TUI,
}

impl Tools {
    pub fn new(config: Config, tui: TUI) -> Self {
        Tools { config, tui }
    }

    /// Execute a tool by name with arguments
    pub async fn execute(&self, tool_name: &str, args: &str) -> ToolResult {
        match tool_name {
            "bash" => self.execute_bash(args),
            "read_file" => self.read_file(args),
            "read_lines" => self.read_lines(args),
            "write_file" => self.write_file(args),
            "insert_lines" => self.insert_lines(args),
            "replace_text" => self.replace_text(args),
            "list_files" => self.list_files(args),
            "grep" => self.grep(args),
            "git_log" => self.git_log(args),
            "git_show" => self.git_show(args),
            "git_diff" => self.git_diff(args),
            "view_image" => self.view_image(args),
            "subagent" => self.subagent(args),
            "patch" => self.patch(args),
            _ => ToolResult::failure(format!("Unknown tool: {}", tool_name), tool_name),
        }
    }

    /// Execute a bash/shell command
    fn execute_bash(&self, args: &str) -> ToolResult {
        let parts: Vec<&str> = args.splitn(2, ' ').collect();

        if self.config.read_only {
            // In read-only mode, only allow safe read-only commands
            let safe_commands = ["ls", "cat", "head", "tail", "grep", "wc", "find",
                "git log", "git show", "git diff", "git status", "pwd", "echo", "true", "false"];
            let first_word = parts.first().copied().unwrap_or("");
            let is_safe = safe_commands.iter().any(|&cmd| first_word == cmd || args.starts_with(cmd));

            if !is_safe {
                return ToolResult::failure(
                    "Read-only mode: file modifications and unsafe commands are not allowed".to_string(),
                    "bash",
                );
            }
        }

        // Use sh -c for proper shell parsing
        let output = match Command::new("sh")
            .arg("-c")
            .arg(args.trim())
            .output()
        {
            Ok(output) => {
                let stdout = String::from_utf8_lossy(&output.stdout).to_string();
                let stderr = String::from_utf8_lossy(&output.stderr).to_string();

                if !output.status.success() {
                    let combined = if stderr.is_empty() { stdout } else { stderr };
                    return ToolResult::failure(combined, "bash");
                }

                // Limit output to last 1000 lines
                let lines: Vec<&str> = stdout.lines().collect();
                let max_lines = 1000;
                if lines.len() > max_lines {
                    let start = lines.len() - max_lines;
                    let truncated = lines[start..].join("\n");
                    format!("... (output truncated, {} total lines)\n{}", lines.len(), truncated)
                } else {
                    stdout
                }
            }
            Err(e) => return ToolResult::failure(e.to_string(), "bash"),
        };

        ToolResult::success(output, "bash")
    }

    /// Read a file
    fn read_file(&self, args: &str) -> ToolResult {
        let parts: Vec<&str> = args.splitn(2, ':').collect();
        let file_path = parts[0].trim();
        let line_range = if parts.len() > 1 {
            parts[1].trim()
        } else {
            ""
        };

        if !Path::new(file_path).exists() {
            return ToolResult::failure(format!("File not found: {}", file_path), "read_file");
        }

        let mut file = match fs::File::open(file_path) {
            Ok(f) => f,
            Err(e) => return ToolResult::failure(e.to_string(), "read_file"),
        };

        let mut content = String::new();
        if file.read_to_string(&mut content).is_err() {
            return ToolResult::failure("Failed to read file".to_string(), "read_file");
        }

        // Apply line range if specified (e.g., "1:10" for lines 1-10)
        let final_content = if !line_range.is_empty() {
            let lines: Vec<&str> = content.lines().collect();
            let parsed: Vec<usize> = line_range.split('-').filter_map(|s| s.parse().ok()).collect();
            if parsed.len() == 2 {
                let start = (parsed[0] - 1).min(lines.len());
                let end = parsed[1].min(lines.len());
                lines[start..end].join("\n")
            } else if parsed.len() == 1 {
                let line_num = (parsed[0] - 1).min(lines.len());
                if line_num < lines.len() {
                    format!("{}\n", lines[line_num])
                } else {
                    String::new()
                }
            } else {
                content
            }
        } else {
            content
        };

        ToolResult::success(final_content, "read_file")
    }

    /// Read a specific line range from a file
    fn read_lines(&self, args: &str) -> ToolResult {
        // Parse JSON-like arguments: {"path": "...", "start": 1, "end": 10}
        // Or simple format: "file_path:start:end"
        let args_trimmed = args.trim();

        let (file_path, start, end) = if args_trimmed.contains('"') {
            // Try to parse as JSON-like format
            let path = extract_json_field(args_trimmed, "path").unwrap_or_else(|| args_trimmed.to_string());
            let start_val = extract_json_field(args_trimmed, "start")
                .and_then(|s| s.trim().parse::<usize>().ok())
                .unwrap_or(1);
            let end_val = extract_json_field(args_trimmed, "end")
                .and_then(|s| s.trim().parse::<usize>().ok())
                .unwrap_or(10);
            (path, start_val, end_val)
        } else {
            // Simple format: "file_path:start:end"
            let parts: Vec<&str> = args_trimmed.splitn(3, ':').collect();
            if parts.len() < 3 {
                return ToolResult::failure(
                    "Usage: read_lines <file_path>:<start>:<end>".to_string(),
                    "read_lines",
                );
            }
            let path = parts[0].trim().to_string();
            let start_val = parts[1].trim().parse::<usize>()
                .unwrap_or(1);
            let end_val = parts[2].trim().parse::<usize>()
                .unwrap_or(10);
            (path, start_val, end_val)
        };

        let path = Path::new(&file_path);
        if !path.exists() {
            return ToolResult::failure(format!("File not found: {}", file_path), "read_lines");
        }

        let mut file = match fs::File::open(path) {
            Ok(f) => f,
            Err(e) => return ToolResult::failure(e.to_string(), "read_lines"),
        };

        let mut content = String::new();
        if file.read_to_string(&mut content).is_err() {
            return ToolResult::failure("Failed to read file".to_string(), "read_lines");
        }

        let lines: Vec<&str> = content.lines().collect();
        let total_lines = lines.len();

        if start > total_lines {
            return ToolResult::success(
                format!("File has {} lines, requested start line {} is out of range",
                    total_lines, start),
                "read_lines",
            );
        }

        let end = end.min(total_lines);
        let selected: Vec<&str> = lines[start - 1..end].to_vec();

        // Add line numbers
        let numbered: Vec<String> = selected.iter().enumerate()
            .map(|(i, line)| format!("{:>4}  {}", start + i, line))
            .collect();

        ToolResult::success(numbered.join("\n"), "read_lines")
    }

    /// Write to a file
    fn write_file(&self, args: &str) -> ToolResult {
        if self.config.read_only {
            return ToolResult::failure(
                "Read-only mode: file writes are not allowed".to_string(),
                "write_file",
            );
        }

        let parts: Vec<&str> = args.splitn(2, ':').collect();
        let file_path = parts[0].trim();
        let content = if parts.len() > 1 {
            parts[1].trim()
        } else {
            ""
        };

        // Sanitize the path - prevent directory traversal
        let sanitized = sanitize_path(file_path);
        let path = Path::new(&sanitized);

        // Ensure parent directory exists
        if let Some(parent) = path.parent() {
            if let Err(e) = fs::create_dir_all(parent) {
                return ToolResult::failure(format!("Failed to create directory: {}", e), "write_file");
            }
        }

        // Write the file
        match fs::write(path, content) {
            Ok(()) => ToolResult::success(
                format!("File written successfully: {}", sanitized),
                "write_file",
            ),
            Err(e) => ToolResult::failure(e.to_string(), "write_file"),
        }
    }

    /// Insert lines at a specific line number
    fn insert_lines(&self, args: &str) -> ToolResult {
        if self.config.read_only {
            return ToolResult::failure(
                "Read-only mode: file modifications are not allowed".to_string(),
                "insert_lines",
            );
        }

        // Format: "file_path:line_number:content"
        let parts: Vec<&str> = args.splitn(3, ':').collect();
        if parts.len() < 3 {
            return ToolResult::failure(
                "Usage: insert_lines <file_path>:<line_number>:<content>".to_string(),
                "insert_lines",
            );
        }

        let file_path = parts[0].trim();
        let line_num: usize = match parts[1].trim().parse() {
            Ok(n) => n,
            Err(_) => return ToolResult::failure("Invalid line number".to_string(), "insert_lines"),
        };
        let content = parts[2].trim();

        let sanitized = sanitize_path(file_path);
        let path = Path::new(&sanitized);

        let mut file = match fs::File::open(path) {
            Ok(f) => f,
            Err(e) => return ToolResult::failure(e.to_string(), "insert_lines"),
        };

        let mut existing = String::new();
        if file.read_to_string(&mut existing).is_err() {
            return ToolResult::failure("Failed to read file".to_string(), "insert_lines");
        }

        let lines: Vec<String> = existing.lines().map(|l| l.to_string()).collect();
        let insert_at = line_num.min(lines.len());

        let mut new_lines: Vec<String> = Vec::new();
        for (i, line) in lines.iter().enumerate() {
            if i == insert_at {
                new_lines.push(content.to_string());
            }
            new_lines.push(line.clone());
        }
        if insert_at == lines.len() {
            new_lines.push(content.to_string());
        }

        let new_content = new_lines.join("\n");

        match fs::write(path, new_content) {
            Ok(()) => ToolResult::success(
                format!("Lines inserted at line {} in {}", line_num, sanitized),
                "insert_lines",
            ),
            Err(e) => ToolResult::failure(e.to_string(), "insert_lines"),
        }
    }

    /// Find and replace text in a file
    fn replace_text(&self, args: &str) -> ToolResult {
        if self.config.read_only {
            return ToolResult::failure(
                "Read-only mode: file modifications are not allowed".to_string(),
                "replace_text",
            );
        }

        // Format: "file_path:search_pattern:replace_text:count"
        let parts: Vec<&str> = args.splitn(4, ':').collect();
        if parts.len() < 3 {
            return ToolResult::failure(
                "Usage: replace_text <file_path>:<search>:<replace>:<count>".to_string(),
                "replace_text",
            );
        }

        let file_path = parts[0].trim();
        let search = parts[1].trim();
        let replace = parts[2].trim();
        let count: i32 = if parts.len() > 3 {
            parts[3].trim().parse().unwrap_or(-1)
        } else {
            -1
        };

        let sanitized = sanitize_path(file_path);
        let path = Path::new(&sanitized);

        let content = match fs::read_to_string(path) {
            Ok(c) => c,
            Err(e) => return ToolResult::failure(e.to_string(), "replace_text"),
        };

        let new_content = if count == -1 {
            content.replace(search, replace)
        } else {
            let mut result = content.clone();
            let mut occurrences = 0;
            let mut pos = 0;
            while let Some(found) = result[pos..].find(search) {
                if occurrences >= count && count > 0 {
                    break;
                }
                let abs_pos = pos + found;
                result.replace_range(abs_pos..abs_pos + search.len(), replace);
                pos = abs_pos + replace.len();
                occurrences += 1;
            }
            result
        };

        if content == new_content {
            return ToolResult::success(
                format!("No occurrences of '{}' found in {}", search, sanitized),
                "replace_text",
            );
        }

        match fs::write(path, new_content) {
            Ok(()) => ToolResult::success(
                format!("Text replaced in {}", sanitized),
                "replace_text",
            ),
            Err(e) => ToolResult::failure(e.to_string(), "replace_text"),
        }
    }

    /// List files in a directory
    fn list_files(&self, args: &str) -> ToolResult {
        let dir_path = args.trim();
        let path = Path::new(dir_path);

        if !path.exists() {
            return ToolResult::failure(format!("Directory not found: {}", dir_path), "list_files");
        }

        if !path.is_dir() {
            return ToolResult::failure(format!("Not a directory: {}", dir_path), "list_files");
        }

        let entries = match fs::read_dir(path) {
            Ok(e) => e,
            Err(e) => return ToolResult::failure(e.to_string(), "list_files"),
        };

        let mut files = Vec::new();
        for entry in entries {
            if let Ok(e) = entry {
                let path = e.path();
                let entry_type = if e.file_type().map(|t| t.is_dir()).unwrap_or(false) {
                    "dir"
                } else {
                    "file"
                };
                let name = path.file_name()
                    .map(|n| n.to_string_lossy().to_string())
                    .unwrap_or_else(|| "?".to_string());
                files.push(format!("  [{}/{}] {}", entry_type,
                    if e.metadata().map(|m| m.len()).unwrap_or(0) > 1024 {
                        format!("{}K", e.metadata().map(|m| m.len()).unwrap_or(0) / 1024)
                    } else {
                        "0".to_string()
                    }, name));
            }
        }

        if files.is_empty() {
            ToolResult::success(format!("Directory '{}' is empty", dir_path), "list_files")
        } else {
            ToolResult::success(files.join("\n"), "list_files")
        }
    }

    /// Grep for a pattern in files
    fn grep(&self, args: &str) -> ToolResult {
        let parts: Vec<&str> = args.splitn(2, ' ').collect();
        if parts.len() < 2 {
            return ToolResult::failure("Usage: grep <pattern> <file_or_dir>".to_string(), "grep");
        }

        let pattern = parts[0].trim();
        let search_path = parts[1].trim();

        // Check if path is a directory or file
        let path = Path::new(search_path);
        let mut results = Vec::new();

        if path.is_dir() {
            // Recursively search directory
            for entry in walk_dir(path).unwrap_or_default() {
                if entry.is_file() {
                    if let Ok(content) = fs::read_to_string(&entry) {
                        for (i, line) in content.lines().enumerate() {
                            if line.contains(pattern) {
                                results.push(format!("{}:{}:{}",
                                    entry.display(), i + 1, line.trim()));
                            }
                        }
                    }
                }
            }
        } else if path.is_file() {
            if let Ok(content) = fs::read_to_string(path) {
                for (i, line) in content.lines().enumerate() {
                    if line.contains(pattern) {
                        results.push(format!("{}:{}:{}",
                            path.display(), i + 1, line.trim()));
                    }
                }
            }
        } else {
            return ToolResult::failure(format!("Path not found: {}", search_path), "grep");
        }

        if results.is_empty() {
            ToolResult::success(format!("No matches found for '{}' in {}", pattern, search_path), "grep")
        } else if results.len() > 100 {
            let truncated: Vec<&str> = results.iter().take(100).map(|s| s.as_str()).collect();
            ToolResult::success(format!("{} matches found (showing first 100):\n{}",
                results.len(), truncated.join("\n")), "grep")
        } else {
            ToolResult::success(results.join("\n"), "grep")
        }
    }

    /// Git log
    fn git_log(&self, args: &str) -> ToolResult {
        let output = match Command::new("git")
            .arg("log")
            .arg("--oneline")
            .arg("--max-count=20")
            .arg(args.trim())
            .output()
        {
            Ok(o) => String::from_utf8_lossy(&o.stdout).to_string(),
            Err(e) => return ToolResult::failure(e.to_string(), "git_log"),
        };

        if output.is_empty() {
            ToolResult::success("No git log available".to_string(), "git_log")
        } else {
            ToolResult::success(output.trim().to_string(), "git_log")
        }
    }

    /// Git show
    fn git_show(&self, args: &str) -> ToolResult {
        let output = match Command::new("git")
            .arg("show")
            .arg(args.trim())
            .output()
        {
            Ok(o) => String::from_utf8_lossy(&o.stdout).to_string(),
            Err(e) => return ToolResult::failure(e.to_string(), "git_show"),
        };

        if output.is_empty() {
            ToolResult::success("No git show output".to_string(), "git_show")
        } else {
            ToolResult::success(output.trim().to_string(), "git_show")
        }
    }

    /// Git diff
    fn git_diff(&self, args: &str) -> ToolResult {
        let output = match Command::new("git")
            .arg("diff")
            .arg(args.trim())
            .output()
        {
            Ok(o) => String::from_utf8_lossy(&o.stdout).to_string(),
            Err(e) => return ToolResult::failure(e.to_string(), "git_diff"),
        };

        if output.is_empty() {
            ToolResult::success("No changes".to_string(), "git_diff")
        } else {
            ToolResult::success(output.trim().to_string(), "git_diff")
        }
    }

    /// View image (basic info)
    fn view_image(&self, args: &str) -> ToolResult {
        let path = args.trim();
        let image_path = Path::new(path);

        if !image_path.exists() {
            return ToolResult::failure(format!("Image not found: {}", path), "view_image");
        }

        let metadata = match image_path.metadata() {
            Ok(m) => m,
            Err(e) => return ToolResult::failure(e.to_string(), "view_image"),
        };

        let file_name = image_path.file_name()
            .map(|n| n.to_string_lossy().to_string())
            .unwrap_or_else(|| "?".to_string());

        let extension = image_path.extension()
            .map(|e| e.to_string_lossy().to_string())
            .unwrap_or_default();

        let size_bytes = metadata.len();
        let size_kb = size_bytes / 1024;

        ToolResult::success(
            format!("Image: {}\nExtension: {}\nSize: {} KB",
                file_name, extension, size_kb),
            "view_image",
        )
    }

    /// Subagent tool - spawn a subprocess
    fn subagent(&self, args: &str) -> ToolResult {
        // Parse prompt and optional persona from args
        // Format: "prompt_text" or "prompt_text" "persona"
        let args_trimmed = args.trim();

        let (prompt, persona) = if args_trimmed.contains('"') {
            // Try to parse JSON-like format: {"prompt": "...", "persona": "..."}
            let prompt_text = extract_json_field(args_trimmed, "prompt").unwrap_or_default();
            let persona_text = extract_json_field(args_trimmed, "persona").unwrap_or_default();
            (prompt_text, persona_text)
        } else {
            // Simple format: "prompt" "persona" or just "prompt"
            // Try to find the persona after the prompt
            let mut prompt_text = String::new();
            let mut persona_text = String::new();
            let mut quote_count = 0;

            for ch in args_trimmed.chars() {
                if ch == '"' {
                    quote_count += 1;
                    continue;
                }
                if quote_count >= 2 && quote_count < 4 {
                    persona_text.push(ch);
                } else if quote_count < 2 {
                    prompt_text.push(ch);
                }
            }

            (prompt_text, persona_text)
        };

        if prompt.is_empty() {
            return ToolResult::failure(
                "Missing required parameter: prompt".to_string(),
                "subagent",
            );
        }

        // Build the command to spawn the subagent
        let binary = std::env::current_exe().unwrap_or_else(|_| std::path::PathBuf::from("coding-agent"));

        let mut cmd_args = vec![
            "--prompt-file".to_string(),
            "-".to_string(),
            "--summary-only".to_string(),
            "--no-stream".to_string(),
            "--quiet".to_string(),
        ];

        if !persona.is_empty() {
            cmd_args.push("--persona".to_string());
            cmd_args.push(persona);
        }

        if self.config.read_only {
            cmd_args.push("--read-only".to_string());
        }

        let output = match Command::new(&binary)
            .args(&cmd_args)
            .stdin(std::process::Stdio::piped())
            .stdout(std::process::Stdio::piped())
            .stderr(std::process::Stdio::piped())
            .spawn()
        {
            Ok(mut child) => {
                // Send prompt via stdin
                if let Some(mut stdin) = child.stdin.take() {
                    use std::io::Write;
                    let _ = stdin.write_all(prompt.as_bytes());
                    drop(stdin);
                }
                match child.wait_with_output() {
                    Ok(o) => String::from_utf8_lossy(&o.stdout).to_string(),
                    Err(_) => String::new(),
                }
            }
            Err(e) => return ToolResult::failure(
                format!("Failed to spawn subagent: {}", e),
                "subagent",
            ),
        };

        // Extract summary from output
        let summary = extract_subagent_summary(&output);

        ToolResult::success(
            format!("Subagent completed.\n\nSummary:\n{}", summary),
            "subagent",
        )
    }

    /// Apply a patch to a file
    fn patch(&self, args: &str) -> ToolResult {
        if self.config.read_only {
            return ToolResult::failure(
                "Read-only mode: file modifications are not allowed".to_string(),
                "patch",
            );
        }

        // Parse JSON-like arguments: {"path": "...", "patch": "..."}
        // Or simple format: "file_path:patch_content"
        let args_trimmed = args.trim();

        let (file_path, patch_content) = if args_trimmed.contains('"') {
            let path = extract_json_field(args_trimmed, "path").unwrap_or_default();
            let patch = extract_json_field(args_trimmed, "patch").unwrap_or_default();
            (path, patch)
        } else {
            // Simple format: "file_path:patch_content"
            let parts: Vec<&str> = args_trimmed.splitn(2, ':').collect();
            if parts.len() < 2 {
                return ToolResult::failure(
                    "Usage: patch <file_path>:<patch_content>".to_string(),
                    "patch",
                );
            }
            (parts[0].trim().to_string(), parts[1].trim().to_string())
        };

        if file_path.is_empty() || patch_content.is_empty() {
            return ToolResult::failure(
                "Missing required parameters: path and patch content".to_string(),
                "patch",
            );
        }

        let path = Path::new(&file_path);
        if !path.exists() {
            return ToolResult::failure(format!("File not found: {}", file_path), "patch");
        }

        // Read original content
        let original = match fs::read_to_string(path) {
            Ok(c) => c,
            Err(e) => return ToolResult::failure(e.to_string(), "patch"),
        };

        // Apply the patch (unified diff format)
        match apply_unified_diff(&original, &patch_content) {
            Ok(new_content) => {
                match fs::write(path, new_content) {
                    Ok(()) => ToolResult::success(
                        format!("Patch applied successfully to {}", file_path),
                        "patch",
                    ),
                    Err(e) => ToolResult::failure(e.to_string(), "patch"),
                }
            }
            Err(e) => ToolResult::failure(format!("Failed to apply patch: {}", e), "patch"),
        }
    }
}

/// Extract a JSON field value from a JSON-like string
fn extract_json_field(json: &str, field: &str) -> Option<String> {
    let pattern = format!("\"{}\"", field);
    if let Some(start) = json.find(&pattern) {
        let value_start = start + pattern.len() + 1; // +1 for ':'
        if value_start >= json.len() {
            return None;
        }
        // Skip whitespace and colon
        let rest = &json[value_start..];
        let rest = rest.trim_start();
        if rest.starts_with(':') {
            let rest = &rest[1..];
            let rest = rest.trim_start();
            if rest.starts_with('"') {
                let rest = &rest[1..];
                if let Some(end) = rest.find('"') {
                    return Some(rest[..end].to_string());
                }
            } else {
                // Non-string value
                let end = rest.find(|c: char| c == ',' || c == '}' || c.is_whitespace()).unwrap_or(rest.len());
                return Some(rest[..end].trim().to_string());
            }
        }
    }
    None
}

/// Extract summary from subagent output
fn extract_subagent_summary(output: &str) -> String {
    // Try to find "=== Final Output ===" marker
    if let Some(idx) = output.find("=== Final Output ===") {
        let summary = output[idx + "=== Final Output ===".len()..].trim().to_string();
        if !summary.is_empty() {
            return summary;
        }
    }

    // Fall back to last substantial text block
    let output = if output.len() > 5000 {
        format!("{}... [output truncated]", &output[..5000])
    } else {
        output.to_string()
    };

    output
}

/// Apply a unified diff patch to content
#[allow(unused_assignments, unused_variables)]
fn apply_unified_diff(original: &str, patch: &str) -> Result<String, String> {
    let original_lines: Vec<&str> = original.lines().collect();
    let result: Vec<&str> = original_lines.clone().to_vec();
    let mut file_path = String::new();
    let mut line_offset = 0;

    for patch_line in patch.lines() {
        if patch_line.starts_with("--- ") {
            continue; // Skip the "--- " line
        } else if patch_line.starts_with("+++ ") {
            // Extract file path from "+++ b/path/to/file"
            let path_part = &patch_line[4..];
            if path_part.starts_with("b/") {
                file_path = path_part[2..].to_string();
            } else {
                file_path = path_part.to_string();
            }
            continue;
        } else if patch_line.starts_with("@@") {
            // Parse hunk header: @@ -old_start,old_count +new_start,new_count @@
            let hunk_start = patch_line.find("-").ok_or("Invalid hunk header")? + 1;
            let old_range = &patch_line[hunk_start..];
            let plus_idx = old_range.find("+").ok_or("Invalid hunk header")?;
            let new_range = &old_range[plus_idx + 1..];

            let _old_start: usize = old_range[..plus_idx].trim().trim_start_matches(',').parse()
                .map_err(|_| "Invalid hunk header")
                .unwrap_or(0);
            let new_start = new_range[..new_range.find(',').unwrap_or(new_range.len())].trim().trim_start_matches(',').parse::<usize>()
                .map_err(|_| "Invalid hunk header")?;

            // Calculate line offset from previous hunks
            let offset = new_start as isize - (result.len() as isize - line_offset as isize) as isize;
            line_offset = offset;

            let _remaining = &patch_line[new_range[new_range.find(',').unwrap_or(new_range.len())..].find('@').unwrap_or(0) + 1..];
            // Parse the remaining hunk lines
            continue;
        }
    }

    // Simplified patch application - apply line by line
    // For a more robust implementation, use a proper diff parsing library
    let patch_lines: Vec<&str> = patch.lines().collect();
    let mut target_lines: Vec<String> = original.lines().map(|l| l.to_string()).collect();
    let mut i = 0;
    let _j = 0;

    while i < patch_lines.len() {
        let line = patch_lines[i];

        if line.starts_with("@@") {
            // Parse hunk
            let parts: Vec<&str> = line.split_whitespace().collect();
            if parts.len() < 3 {
                i += 1;
                continue;
            }

            let old_range = parts[1];
            let new_range = parts[2];

            let _old_start: usize = old_range[1..].split(',').next().unwrap_or("1").parse().unwrap_or(1);
            let new_start: usize = new_range[1..].split(',').next().unwrap_or("1").parse().unwrap_or(1);

            i += 1;

            let mut target_idx = if new_start > 1 { new_start - 1 } else { 0 };

            // Skip context lines that match, process additions/deletions
            while i < patch_lines.len() {
                let hunk_line = patch_lines[i];

                if hunk_line.starts_with("@@") || hunk_line.starts_with("---") || hunk_line.starts_with("+++") {
                    break;
                }

                if hunk_line.starts_with(' ') || hunk_line.is_empty() {
                    // Context line - advance both
                    target_idx += 1;
                } else if hunk_line.starts_with('-') {
                    // Deletion
                    target_lines.remove(target_idx.saturating_sub(1));
                } else if hunk_line.starts_with('+') {
                    // Addition
                    let content = &hunk_line[1..];
                    target_lines.insert(target_idx, content.to_string());
                    target_idx += 1;
                }
                i += 1;
            }
        } else {
            i += 1;
        }
    }

    Ok(target_lines.join("\n"))
}

/// Recursively walk a directory
fn walk_dir(path: &Path) -> Result<Vec<std::path::PathBuf>, String> {
    let mut result = Vec::new();

    for entry in fs::read_dir(path)
        .map_err(|e| format!("Failed to read directory: {}", e))?
    {
        let entry = entry.map_err(|e| format!("Failed to read entry: {}", e))?;
        let path = entry.path();

        if path.is_dir() {
            result.extend(walk_dir(&path)?);
        } else {
            result.push(path);
        }
    }

    Ok(result)
}

/// Sanitize a file path to prevent directory traversal
fn sanitize_path(path: &str) -> String {
    // Remove null bytes
    let cleaned: String = path.chars().filter(|&c| c != '\0').collect();

    // Check for directory traversal
    if cleaned.contains("..") {
        // Resolve to absolute path
        if let Ok(abs) = std::env::current_dir().and_then(|c| c.join(&cleaned).canonicalize()) {
            return abs.to_string_lossy().to_string();
        }
    }

    // Return the path as-is if it's relative and safe
    if path.starts_with('/') {
        path.to_string()
    } else {
        // Make it relative to current directory
        format!("{}/{}", std::env::current_dir()
            .map(|c| c.to_string_lossy().to_string())
            .unwrap_or_else(|_| ".".to_string()),
            path)
    }
}
