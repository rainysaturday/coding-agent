// Command coding-agent is a minimal coding agent harness: a TUI (or web UI)
// driving an OpenAI-compatible inference backend with tool calling.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/coding-agent/coding-agent/agent"
	"github.com/coding-agent/coding-agent/colors"
	"github.com/coding-agent/coding-agent/config"
	"github.com/coding-agent/coding-agent/debug"
	"github.com/coding-agent/coding-agent/tui"
	"github.com/coding-agent/coding-agent/version"
	"github.com/coding-agent/coding-agent/web"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	cfg, err := config.Load(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		printUsage(os.Stderr)
		return 2
	}
	if cfg.ShowHelp {
		printUsage(os.Stdout)
		return 0
	}
	if cfg.ShowVersion {
		fmt.Println(version.Full())
		return 0
	}

	if err := colors.ApplyTheme(cfg.Theme); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v (falling back to %s)\n", err, colors.DefaultTheme)
		cfg.Theme = colors.DefaultTheme
		_ = colors.ApplyTheme(cfg.Theme)
	}

	if cfg.Debug {
		debug.Init(cfg.DebugLog, true)
		defer debug.Close()
	}
	for _, w := range cfg.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}

	// One-shot prompt sources.
	if cfg.PromptFile != "" {
		data, err := os.ReadFile(cfg.PromptFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot read prompt file: %v\n", err)
			return 2
		}
		cfg.Prompt = string(data)
	}
	if cfg.UseStdin {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot read stdin: %v\n", err)
			return 2
		}
		if cfg.Prompt != "" {
			cfg.Prompt += "\n"
		}
		cfg.Prompt += string(data)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cfg.Mode {
	case config.WebMode:
		srv := web.NewServer(cfg)
		if err := srv.Serve(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		return 0
	case config.OneShotMode:
		return runOneShot(ctx, cfg)
	default:
		a := newAgent(cfg)
		return tui.New(a, cfg).Run(ctx)
	}
}

// newAgent builds the agent and connects MCP servers (warnings are printed).
func newAgent(cfg *config.Config) *agent.Agent {
	a := agent.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, err := range a.ConnectMCP(ctx) {
		fmt.Fprintf(os.Stderr, "warning: MCP: %v\n", err)
	}
	return a
}

// ---------------------------------------------------------------------------
// one-shot mode (requirement 025)
// ---------------------------------------------------------------------------

func runOneShot(ctx context.Context, cfg *config.Config) int {
	a := newAgent(cfg)
	if cfg.Goal != "" {
		a.SetGoal(cfg.Goal)
	}

	// Load a previous context when requested.
	if cfg.ContextFile != "" {
		n, lastUser, err := a.LoadContext(cfg.ContextFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 2
		}
		fmt.Printf("Loaded %d messages from %s\n", n, cfg.ContextFile)
		if !cfg.Quiet {
			if lastUser != "" {
				fmt.Printf("Last user prompt: %s\n", truncate(lastUser, 200))
			}
		}
	}

	var final string
	a.SetEventCallback(func(e agent.Event) {
		switch e.Type {
		case agent.EventContent:
			if !cfg.Quiet {
				fmt.Print(e.Text)
			}
		case agent.EventReasoning:
			if cfg.Verbose {
				fmt.Print(colors.GetColor("dim") + e.Text + colors.GetColor("reset"))
			}
		case agent.EventToolStart:
			if !cfg.Quiet {
				fmt.Printf("\n[Tool Call] %s (%s)\n", e.Tool, agent.FlattenArgs(e.Args))
			}
		case agent.EventToolResult:
			if !cfg.Quiet {
				if e.Failed {
					fmt.Printf("Tool '%s' failed:\n%s\n", e.Tool, e.Result)
				} else {
					fmt.Printf("Tool '%s' executed successfully:\n%s\n", e.Tool, e.Result)
				}
			}
		case agent.EventError:
			fmt.Fprintf(os.Stderr, "\n%s\n", e.Text)
		case agent.EventGoal, agent.EventCompressed, agent.EventSubagent:
			if cfg.Verbose {
				fmt.Printf("\n%s\n", e.Text)
			}
		}
	})

	var err error
	if cfg.Prompt != "" {
		final, err = a.Submit(ctx, cfg.Prompt)
	} else {
		final, err = a.Continue(ctx)
	}
	if final != "" {
		fmt.Println()
	}

	if !cfg.Quiet {
		fmt.Println("\n=== Final Output ===")
		fmt.Println(final)
	} else {
		fmt.Println(final)
	}

	if cfg.Verbose {
		snap := a.Stats().Snapshot()
		fmt.Println("\n=== Summary ===")
		fmt.Printf("Iterations executed: %d\n", snap.Iterations)
		fmt.Printf("Tokens used: %d (input) + %d (output)\n", snap.InputTokens, snap.OutputTokens)
		fmt.Printf("Tool calls: %d (%d failed)\n", snap.ToolCalls, snap.FailedToolCalls)
		fmt.Printf("Duration: %s\n", agent.FormatDuration(time.Since(snap.StartTime)))
	}

	if cfg.OutputFile != "" {
		if err := os.WriteFile(cfg.OutputFile, []byte(final), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot write output file: %v\n", err)
			return 1
		}
	}

	if err != nil {
		return agent.ExitCodeFor(err)
	}
	return 0
}

func truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// ---------------------------------------------------------------------------
// usage
// ---------------------------------------------------------------------------

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `coding-agent %s — minimal coding agent harness

Usage:
  coding-agent [flags]
  coding-agent -p "prompt" [flags]
  echo "prompt" | coding-agent --stdin [flags]

Prompt input:
  -p, --prompt <text>        One-shot prompt
  --stdin                    Read the prompt from stdin
  --prompt-file <path>       Read the prompt from a file
  --output <path>            Write the final output to a file
  --verbose                  Show all tool interactions (one-shot)
  --quiet                    Only show the final answer (one-shot)

Inference backend:
  --api-endpoint <url>       API base URL (default: %s)
  --api-key <key>            API key (or CODING_AGENT_API_KEY)
  --model <name>             Model name (default: %s)
  --temperature <float>      Sampling temperature (only sent when set)
  --max-tokens <n>           Maximum response tokens (0 = backend default)
  --top-p <float>            Nucleus sampling parameter
  --stream / --no-stream     Streaming inference (default: on)
  --initial-token-timeout <s> Timeout for the first streamed token (default: %d)
  --connect-timeout <s>      Connection timeout in seconds (default: 30)
  --read-timeout <s>         Response-header timeout in seconds (0 = use initial token timeout)

Context management:
  --context-size <n>         Context window in tokens (default: %d)
  --context-threshold <f>    Compression threshold fraction (default: %.2f)
  --max-iterations <n>       Maximum agentic iterations (default: %d)
  --no-compress              Disable automatic context compression
  --load <path>              Load a dumped context and continue
  --no-dump-on-exit          Do not dump the context when exiting interactive mode

Behaviour:
  --read-only                Only enable read-only tools
  --persona <text>           Inject a persona into the system prompt
  --summary-only             Lightweight mode: return a summary, not the full log
  --goal <text>              Start in goal mode with the given goal
  --theme <name>             %s
  --debug                    Enable debug logging
  --debug-log <path>         Debug log file (default: %s)
  --config <path>            Configuration file (default: ~/.coding-agentrc)
  --cwd <path>               Working directory for tools
  --max-history <n>          Input history size (default: %d)
  --subagent-timeout <s>     Subagent timeout in seconds (default: %d)

Web UI:
  --web                      Serve the web UI instead of the TUI
  --web-addr <host>          Listen address (default: %s)
  --web-port <port>          Listen port (default: %d)

Skills:
  --skills / --no-skills     Enable skill discovery (default: on)
  --skills-dir <path>        Additional skill directory
  --trust                    Trust project-local skill directories

GitHub Copilot backend:
  --api-endpoint https://api.githubcopilot.com
  Token via GITHUB_TOKEN (a GitHub App user token from 'gh auth token',
  NOT a personal access token) or --api-key. Copilot-hosted models such as
  gpt-4o, claude-sonnet-4 and o3-mini work with --model.

Misc:
  -h, --help                 Show this help
  -v, --version              Show version information

Environment variables: CODING_AGENT_API_ENDPOINT, CODING_AGENT_API_KEY,
CODING_AGENT_MODEL, CODING_AGENT_CONTEXT_SIZE, CODING_AGENT_MAX_ITERATIONS,
CODING_AGENT_THEME, CODING_AGENT_DEBUG, CODING_AGENT_READ_ONLY,
CODING_AGENT_PERSONA, CODING_AGENT_GOAL, CODING_AGENT_STREAM and others.

Exit codes: 0 success, 1 runtime/inference failure, 2 usage error,
3 authentication failure, 4 context too large.
`, version.String(), config.DefaultAPIEndpoint, config.DefaultModel,
		config.DefaultInitialTokenTimeout, config.DefaultContextSize,
		config.DefaultContextThreshold, config.DefaultMaxIterations,
		strings.Join(colors.ListThemes(), " | "), config.DefaultDebugLogFile,
		config.DefaultHistorySize, config.DefaultSubagentTimeout,
		config.DefaultWebAddr, config.DefaultWebPort)
}
