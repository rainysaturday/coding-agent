package agent

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/coding-agent/coding-agent/config"
	"github.com/coding-agent/coding-agent/tools"
)

// SystemPrompt builds the fixed system prompt: environment information, the
// tool catalogue, best practices, verification requirements and the optional
// skills catalog, persona and summary-only instructions.
func (a *Agent) SystemPrompt() string {
	var b strings.Builder
	b.WriteString("You are a helpful coding assistant. You have access to the following tools.\n")

	// Environment information (requirement 029).
	cwd := a.cfg.WorkingDir
	if cwd == "" {
		cwd = "unknown"
	}
	exe := a.cfg.ExecutablePath
	if exe == "" {
		if p, err := os.Executable(); err == nil {
			exe = p
		} else {
			exe = "unknown"
		}
	}
	b.WriteString("\nENVIRONMENT INFORMATION:\n")
	b.WriteString(fmt.Sprintf("- Current Working Directory: %s\n", cwd))
	b.WriteString(fmt.Sprintf("- Agent Executable: %s\n", exe))
	b.WriteString(fmt.Sprintf("- Operating System: %s\n", runtime.GOOS))
	b.WriteString(fmt.Sprintf("- Architecture: %s\n", runtime.GOARCH))
	if !a.cfg.ReadOnly {
		b.WriteString("\nYou can use the agent executable to spawn sub-agents for parallel tasks using the subagent tool.\n")
		b.WriteString("When you need to run a subagent, use the 'subagent' tool with a clear task description.\n")
		b.WriteString("The subagent will run independently and return its conclusion/summary.\n")
		b.WriteString(fmt.Sprintf("(The subagent is a recursive invocation of: %s -p \"Your task here\".)\n", filepathBase(exe)))
	}

	b.WriteString("\nTOOL CALLING FORMAT:\n")
	b.WriteString("- Tools are provided natively in the request's `tools` field (OpenAI function calling format).\n")
	b.WriteString("- Call a tool by emitting one or more tool calls; each tool call names a tool and provides JSON arguments.\n")
	b.WriteString("- You may call several tools in one turn; their results are returned as tool messages before you continue.\n")
	b.WriteString("- When you have no more tool calls, answer the user in plain text.\n")

	// Tool catalogue (requirement 015).
	defs := a.registry.Definitions()
	if len(defs) > 0 {
		b.WriteString("\nAVAILABLE TOOLS:\n")
		for i, d := range defs {
			b.WriteString(fmt.Sprintf("\n%d. %s\n", i+1, d.Function.Name))
			b.WriteString(fmt.Sprintf("   Description: %s\n", d.Function.Description))
			b.WriteString("   Parameters:\n")
			for _, line := range parameterLines(d.Function.Parameters) {
				b.WriteString(line + "\n")
			}
			how, example := tools.UsageGuide(d.Function.Name)
			if how != "" {
				b.WriteString(fmt.Sprintf("   How to call: %s\n", how))
			}
			if example != "" {
				b.WriteString(fmt.Sprintf("   Example use case: %s\n", example))
			}
		}
	}

	b.WriteString("\nTOOL CALLING BEST PRACTICES:\n")
	b.WriteString("1. Always read a file first (using read_file or read_lines) to understand its contents\n")
	b.WriteString("2. When modifying files, be precise about what you're changing\n")
	b.WriteString("3. For multi-line content, properly format with \\n for newlines\n")
	b.WriteString("4. Verify your changes by re-reading files after writing\n")
	b.WriteString("5. Test code by running appropriate commands (go build, npm test, pytest, etc.)\n")

	b.WriteString("\nVERIFICATION REQUIREMENTS:\n")
	b.WriteString("- ALWAYS double-check your work before considering a task complete\n")
	b.WriteString("- Verify that created/modified files exist and contain the expected content\n")
	b.WriteString("- Test code execution when possible (e.g., go build, npm test, pytest, cargo test, etc.)\n")
	b.WriteString("- Validate that changes meet the user's requirements\n")
	b.WriteString("- Re-read files after writing to confirm content was written correctly\n")
	b.WriteString("- Run validation commands (e.g., go vet, gofmt -d, pylint, eslint, cat to view files)\n")
	b.WriteString("- If verification fails, fix the issue and re-verify\n")
	b.WriteString("- Provide a final verification summary before concluding the task\n")

	if a.cfg.ReadOnly {
		b.WriteString("\nREAD-ONLY MODE:\n")
		b.WriteString("The session runs in read-only mode. Only tools that inspect state are available (read_file, read_lines, list_files, grep, git_log, git_show, git_diff and other read-only tools); write-modifying tools such as bash, write_file, insert_lines and replace_text are disabled. Never attempt to modify files or system state; if a task requires changes, explain what the user must run in normal mode.\n")
	}

	if a.skills != nil {
		if catalog := a.skills.Catalog(); catalog != "" {
			b.WriteString("\n" + catalog + "\n")
		}
	}

	if a.cfg.Persona != "" {
		b.WriteString("\nYOUR PERSONA:\n" + strings.TrimSpace(a.cfg.Persona) + "\n")
	}

	if a.cfg.SummaryOnly {
		b.WriteString("\nSUMMARY ONLY MODE:\n")
		b.WriteString("When finished, respond with a concise summary of what was accomplished, the conclusion and any critical findings. Do not include step-by-step logs, tool call transcripts or reasoning.\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

func filepathBase(p string) string {
	if p == "" {
		return "coding-agent"
	}
	if i := strings.LastIndexAny(p, `/\`); i >= 0 && i < len(p)-1 {
		return p[i+1:]
	}
	return p
}

// parameterLines renders a JSON schema's properties for the system prompt.
func parameterLines(schema map[string]interface{}) []string {
	if len(schema) == 0 {
		return []string{"     (none)"}
	}
	props, _ := schema["properties"].(map[string]interface{})
	if len(props) == 0 {
		return []string{"     (none)"}
	}
	required := toStringSlice(schema["required"])
	isRequired := func(name string) bool {
		for _, r := range required {
			if r == name {
				return true
			}
		}
		return false
	}
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		ri, rj := isRequired(names[i]), isRequired(names[j])
		if ri != rj {
			return ri
		}
		return names[i] < names[j]
	})
	out := make([]string, 0, len(names))
	for _, name := range names {
		p, _ := props[name].(map[string]interface{})
		typ := "any"
		if t, ok := p["type"].(string); ok {
			typ = t
			if t == "array" {
				if items, ok := p["items"].(map[string]interface{}); ok {
					if it, ok := items["type"].(string); ok {
						typ = "array of " + it
					}
				}
			}
		}
		desc := ""
		if d, ok := p["description"].(string); ok {
			desc = d
		}
		req := "optional"
		if isRequired(name) {
			req = "required"
		}
		line := fmt.Sprintf("     - %s (%s, %s)", name, typ, req)
		if desc != "" {
			line += ": " + desc
		}
		out = append(out, line)
	}
	return out
}

func toStringSlice(v interface{}) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, e := range t {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// GoalCheckPrompt is injected when a goal is active and the model stopped
// calling tools (requirement 038).
func GoalCheckPrompt(goal string) string {
	return fmt.Sprintf(`GOAL CHECK (automatic):
The active goal is: %q

Review the work done so far against this goal. If the goal is fully achieved and verified, reply with a short summary that includes the exact phrase "goal achieved". If anything is still missing, continue working on the goal using your tools and do not use that phrase.`, goal)
}

// SummaryRequestPrompt asks the model to summarise the conversation.
func SummaryRequestPrompt() string {
	return "CONTEXT COMPRESSION: Summarize the conversation above so work can continue with a shorter context. Preserve: the original user request, key decisions, file paths and names touched, tool results that matter, current progress and the exact next step. Reply with the summary only."
}

// MaxIterationsMessage is shown when the iteration limit is reached.
func MaxIterationsMessage(n int) string {
	return fmt.Sprintf("Maximum number of iterations (%d) reached; stopping the agentic loop.", n)
}

// ModeName returns a human readable mode name.
func ModeName(mode config.RunMode) string {
	switch mode {
	case config.OneShotMode:
		return "one-shot"
	case config.WebMode:
		return "web"
	default:
		return "interactive"
	}
}
