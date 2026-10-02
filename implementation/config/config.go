// Package config resolves the coding agent configuration from (in order of
// increasing precedence) built-in defaults, a config file, environment
// variables and command line flags.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/coding-agent/coding-agent/colors"
)

// RunMode selects the interactive or the one-shot execution mode.
type RunMode int

const (
	// InteractiveMode runs the TUI loop.
	InteractiveMode RunMode = iota
	// OneShotMode runs a single prompt and exits.
	OneShotMode
	// WebMode serves the browser based UI.
	WebMode
)

// Default values for the harness.
const (
	DefaultModel               = "llama3"
	DefaultAPIEndpoint         = "http://localhost:8080"
	DefaultContextSize         = 128000
	DefaultContextThreshold    = 0.8
	DefaultMaxIterations       = 1000
	DefaultMaxTokens           = 0 // 0 => omit max_tokens, let the backend decide
	DefaultInitialTokenTimeout = 86400
	DefaultConnectTimeout      = 30
	DefaultReadTimeout         = 0 // 0: derived from the initial token timeout
	DefaultHistorySize         = 100
	DefaultSubagentTimeout     = 1800
	DefaultWebAddr             = "127.0.0.1"
	DefaultWebPort             = 8080
	DefaultDebugLogFile        = "debug.log"
	DefaultBashTimeoutMs       = 30000
	DefaultBashMaxOutputLines  = 200
)

// MCPServerConfig configures a single MCP server connection.
type MCPServerConfig struct {
	Name       string            `json:"name"`
	Transport  string            `json:"transport"` // stdio | sse
	Command    string            `json:"command,omitempty"`
	Args       []string          `json:"args,omitempty"`
	CWD        string            `json:"cwd,omitempty"`
	URL        string            `json:"url,omitempty"`
	Endpoint   string            `json:"endpoint,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Enabled    bool              `json:"enabled"`
	Timeout    int               `json:"timeout,omitempty"` // seconds
	Reconnect  bool              `json:"reconnect,omitempty"`
	MaxRetries int               `json:"maxRetries,omitempty"`
}

// Config is the fully resolved runtime configuration.
type Config struct {
	// Run mode / entry points
	Mode           RunMode
	Prompt         string
	PromptFile     string
	UseStdin       bool
	ContextFile    string
	NoDumpOnExit   bool
	Verbose        bool
	Quiet          bool
	OutputFile     string
	ShowHelp       bool
	ShowVersion    bool
	WorkingDir     string
	ExecutablePath string

	// Inference backend
	APIEndpoint         string
	APIKey              string
	Model               string
	Temperature         *float64
	MaxTokens           int
	TopP                *float64
	Stream              bool
	InitialTokenTimeout int
	ConnectTimeout      int
	ReadTimeout         int
	ContextSize         int
	ContextThreshold    float64
	MaxIterations       int
	AutoCompress        bool

	// Behaviour
	ReadOnly        bool
	Persona         string
	SummaryOnly     bool
	Goal            string
	Theme           string
	Debug           bool
	DebugLog        string
	ConfigFile      string
	MaxHistory      int
	SubagentTimeout int

	// Web UI
	Web     bool
	WebAddr string
	WebPort int

	// Skills
	SkillsEnabled bool
	SkillsDirs    []string
	TrustDirs     []string

	// MCP servers keyed by name
	MCPServers map[string]MCPServerConfig

	// Errors collects non fatal configuration problems (surfaced in help output).
	Warnings []string
}

// NewDefault returns the configuration with built-in defaults applied.
func NewDefault() *Config {
	cwd, _ := os.Getwd()
	exe, _ := os.Executable()
	return &Config{
		Mode:                InteractiveMode,
		APIEndpoint:         DefaultAPIEndpoint,
		Model:               DefaultModel,
		MaxTokens:           DefaultMaxTokens,
		Stream:              true,
		InitialTokenTimeout: DefaultInitialTokenTimeout,
		ConnectTimeout:      DefaultConnectTimeout,
		ReadTimeout:         DefaultReadTimeout,
		ContextSize:         DefaultContextSize,
		ContextThreshold:    DefaultContextThreshold,
		MaxIterations:       DefaultMaxIterations,
		AutoCompress:        true,
		Theme:               colors.DefaultTheme,
		DebugLog:            DefaultDebugLogFile,
		MaxHistory:          DefaultHistorySize,
		SubagentTimeout:     DefaultSubagentTimeout,
		WebAddr:             DefaultWebAddr,
		WebPort:             DefaultWebPort,
		SkillsEnabled:       true,
		WorkingDir:          cwd,
		ExecutablePath:      exe,
		MCPServers:          map[string]MCPServerConfig{},
	}
}

// Load builds the configuration from defaults, config file, environment and
// command line arguments.
func Load(args []string) (*Config, error) {
	cfg := NewDefault()

	// A config file can be provided via env or flag; probe the flags first so
	// that --config is honoured before environment variables are applied.
	probe, err := probeFlags(args)
	if err != nil {
		return nil, err
	}
	if probe.ConfigFile != "" {
		if err := cfg.LoadFile(probe.ConfigFile); err != nil {
			return nil, err
		}
	} else if path := DefaultConfigPath(); path != "" {
		_ = cfg.LoadFile(path) // optional, ignore missing
	}

	cfg.LoadEnv()

	if err := cfg.ParseFlags(args); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// DefaultConfigPath returns the optional per-user config file path.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".coding-agentrc")
}

// LoadFile applies an INI style configuration file.
func (c *Config) LoadFile(path string) error {
	sections, global, err := parseINI(path)
	if err != nil {
		return err
	}
	for key, value := range global {
		if err := c.applyKV(key, value); err != nil {
			return fmt.Errorf("config file %s: %w", path, err)
		}
	}
	for name, kv := range sections {
		if !strings.HasPrefix(name, "mcp_server.") {
			continue
		}
		serverName := strings.TrimPrefix(name, "mcp_server.")
		if serverName == "" {
			continue
		}
		srv, ok := c.MCPServers[serverName]
		if !ok {
			srv = MCPServerConfig{Name: serverName, Transport: "stdio", Timeout: 30, MaxRetries: 5, Reconnect: true}
		}
		for key, value := range kv {
			switch strings.ToLower(key) {
			case "enabled":
				srv.Enabled = parseBool(value)
			case "transport":
				srv.Transport = strings.ToLower(value)
			case "command":
				srv.Command = value
			case "args":
				srv.Args = splitArgs(value)
			case "cwd":
				srv.CWD = value
			case "url":
				srv.URL = value
			case "endpoint":
				srv.Endpoint = value
			case "timeout":
				srv.Timeout = atoiDefault(value, srv.Timeout)
			case "reconnect":
				srv.Reconnect = parseBool(value)
			case "max_retries", "maxretries":
				srv.MaxRetries = atoiDefault(value, srv.MaxRetries)
			default:
				if strings.HasPrefix(key, "env.") {
					if srv.Env == nil {
						srv.Env = map[string]string{}
					}
					srv.Env[strings.TrimPrefix(key, "env.")] = value
				} else if strings.HasPrefix(key, "headers.") {
					if srv.Headers == nil {
						srv.Headers = map[string]string{}
					}
					srv.Headers[strings.TrimPrefix(key, "headers.")] = value
				}
			}
		}
		c.MCPServers[serverName] = srv
	}
	return nil
}

// applyKV applies a single key/value pair from a config file.
func (c *Config) applyKV(key, value string) error {
	switch strings.ToLower(key) {
	case "api_endpoint":
		c.APIEndpoint = strings.TrimRight(value, "/")
	case "api_key":
		c.APIKey = value
	case "model":
		c.Model = value
	case "temperature":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid temperature %q", value)
		}
		c.Temperature = &f
	case "max_tokens":
		c.MaxTokens = atoiDefault(value, c.MaxTokens)
	case "top_p":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid top_p %q", value)
		}
		c.TopP = &f
	case "stream":
		c.Stream = parseBool(value)
	case "no_stream":
		c.Stream = !parseBool(value)
	case "initial_token_timeout":
		c.InitialTokenTimeout = atoiDefault(value, c.InitialTokenTimeout)
	case "connect_timeout":
		c.ConnectTimeout = atoiDefault(value, c.ConnectTimeout)
	case "read_timeout":
		c.ReadTimeout = atoiDefault(value, c.ReadTimeout)
	case "context_size":
		c.ContextSize = atoiDefault(value, c.ContextSize)
	case "context_threshold":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid context_threshold %q", value)
		}
		c.ContextThreshold = f
	case "max_iterations":
		c.MaxIterations = atoiDefault(value, c.MaxIterations)
	case "auto_compress":
		c.AutoCompress = parseBool(value)
	case "read_only":
		c.ReadOnly = parseBool(value)
	case "persona":
		c.Persona = value
	case "summary_only":
		c.SummaryOnly = parseBool(value)
	case "goal":
		c.Goal = value
	case "theme":
		c.Theme = value
	case "debug":
		c.Debug = parseBool(value)
	case "debug_log":
		c.DebugLog = value
	case "verbose":
		c.Verbose = parseBool(value)
	case "quiet":
		c.Quiet = parseBool(value)
	case "output":
		c.OutputFile = value
	case "load":
		c.ContextFile = value
	case "no_dump_on_exit":
		c.NoDumpOnExit = parseBool(value)
	case "max_history":
		c.MaxHistory = atoiDefault(value, c.MaxHistory)
	case "subagent_timeout":
		c.SubagentTimeout = atoiDefault(value, c.SubagentTimeout)
	case "web":
		c.Web = parseBool(value)
	case "web_addr":
		c.WebAddr = value
	case "web_port":
		c.WebPort = atoiDefault(value, c.WebPort)
	case "skills":
		c.SkillsEnabled = parseBool(value)
	case "skills_dir":
		for _, p := range filepath.SplitList(value) {
			if p != "" {
				c.SkillsDirs = append(c.SkillsDirs, p)
			}
		}
	case "trust":
		c.TrustDirs = append(c.TrustDirs, value)
	case "prompt":
		c.Prompt = value
	case "prompt_file":
		c.PromptFile = value
	default:
		c.Warnings = append(c.Warnings, fmt.Sprintf("unknown config key %q", key))
	}
	return nil
}

// LoadEnv applies CODING_AGENT_* environment variables.
func (c *Config) LoadEnv() {
	setStr := func(name string, target *string) {
		if v, ok := os.LookupEnv(name); ok && v != "" {
			*target = v
		}
	}
	setInt := func(name string, target *int) {
		if v, ok := os.LookupEnv(name); ok && v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				*target = n
			}
		}
	}
	setFloat := func(name string, target **float64) {
		if v, ok := os.LookupEnv(name); ok && v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				*target = &f
			}
		}
	}
	setBool := func(name string, target *bool) {
		if v, ok := os.LookupEnv(name); ok && v != "" {
			*target = parseBool(v)
		}
	}

	if v, ok := os.LookupEnv("CODING_AGENT_API_ENDPOINT"); ok && v != "" {
		c.APIEndpoint = strings.TrimRight(v, "/")
	}
	setStr("CODING_AGENT_API_KEY", &c.APIKey)
	setStr("CODING_AGENT_MODEL", &c.Model)
	setFloat("CODING_AGENT_TEMPERATURE", &c.Temperature)
	setInt("CODING_AGENT_MAX_TOKENS", &c.MaxTokens)
	setFloat("CODING_AGENT_TOP_P", &c.TopP)
	setBool("CODING_AGENT_STREAM", &c.Stream)
	setInt("CODING_AGENT_INITIAL_TOKEN_TIMEOUT", &c.InitialTokenTimeout)
	setInt("CODING_AGENT_CONNECT_TIMEOUT", &c.ConnectTimeout)
	setInt("CODING_AGENT_MAX_HISTORY", &c.MaxHistory)
	setInt("CODING_AGENT_READ_TIMEOUT", &c.ReadTimeout)
	setInt("CODING_AGENT_CONTEXT_SIZE", &c.ContextSize)
	if v, ok := os.LookupEnv("CODING_AGENT_CONTEXT_THRESHOLD"); ok && v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.ContextThreshold = f
		}
	}
	setInt("CODING_AGENT_MAX_ITERATIONS", &c.MaxIterations)
	setStr("CODING_AGENT_THEME", &c.Theme)
	setBool("CODING_AGENT_DEBUG", &c.Debug)
	setStr("CODING_AGENT_DEBUG_LOG", &c.DebugLog)
	setBool("CODING_AGENT_READ_ONLY", &c.ReadOnly)
	setStr("CODING_AGENT_PERSONA", &c.Persona)
	setBool("CODING_AGENT_SUMMARY_ONLY", &c.SummaryOnly)
	setStr("CODING_AGENT_GOAL", &c.Goal)
	setStr("CODING_AGENT_CONFIG", &c.ConfigFile)
	setBool("CODING_AGENT_NO_DUMP_ON_EXIT", &c.NoDumpOnExit)
	setInt("CODING_AGENT_SUBAGENT_TIMEOUT", &c.SubagentTimeout)
	setBool("CODING_AGENT_SKILLS", &c.SkillsEnabled)
	if v, ok := os.LookupEnv("CODING_AGENT_SKILLS_DIR"); ok && v != "" {
		for _, p := range filepath.SplitList(v) {
			if p != "" {
				c.SkillsDirs = append(c.SkillsDirs, p)
			}
		}
	}

	// GITHUB_TOKEN is a fallback API key (never overrides an explicit key).
	if c.APIKey == "" {
		if v := os.Getenv("GITHUB_TOKEN"); v != "" {
			c.APIKey = v
		}
	}

	c.loadMCPServerEnv()
}

// loadMCPServerEnv reads CODING_AGENT_MCP_<NAME>_<FIELD> variables.
func (c *Config) loadMCPServerEnv() {
	const prefix = "CODING_AGENT_MCP_"
	fields := []string{"ENABLED", "TRANSPORT", "COMMAND", "ARGS", "URL", "ENDPOINT", "TIMEOUT", "CWD", "RECONNECT", "MAX_RETRIES"}
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || !strings.HasPrefix(parts[0], prefix) {
			continue
		}
		rest := strings.TrimPrefix(parts[0], prefix)
		if rest == "" {
			continue
		}
		name, field := "", ""
		if idx := strings.Index(rest, "_ENV_"); idx >= 0 {
			name = rest[:idx]
			field = "ENV"
		} else {
			for _, f := range fields {
				if strings.HasSuffix(rest, "_"+f) {
					name = rest[:len(rest)-len(f)-1]
					field = f
					break
				}
			}
		}
		if name == "" {
			continue
		}
		name = strings.ToLower(name)
		srv, ok := c.MCPServers[name]
		if !ok {
			srv = MCPServerConfig{Name: name, Transport: "stdio", Timeout: 30, MaxRetries: 5, Reconnect: true}
		}
		switch field {
		case "ENABLED":
			srv.Enabled = parseBool(parts[1])
		case "TRANSPORT":
			srv.Transport = strings.ToLower(parts[1])
		case "COMMAND":
			srv.Command = parts[1]
		case "ARGS":
			srv.Args = splitArgs(parts[1])
		case "URL":
			srv.URL = parts[1]
		case "ENDPOINT":
			srv.Endpoint = parts[1]
		case "TIMEOUT":
			srv.Timeout = atoiDefault(parts[1], srv.Timeout)
		case "CWD":
			srv.CWD = parts[1]
		case "RECONNECT":
			srv.Reconnect = parseBool(parts[1])
		case "MAX_RETRIES":
			srv.MaxRetries = atoiDefault(parts[1], srv.MaxRetries)
		case "ENV":
			key := strings.TrimPrefix(strings.TrimPrefix(rest, name), "_ENV_")
			if key != "" {
				if srv.Env == nil {
					srv.Env = map[string]string{}
				}
				srv.Env[key] = parts[1]
			}
		}
		c.MCPServers[name] = srv
	}
}

// probeFlags performs a minimal scan for flags that influence config loading.
func probeFlags(args []string) (*Config, error) {
	probe := &Config{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--config":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--config requires an argument")
			}
			i++
			probe.ConfigFile = args[i]
		case strings.HasPrefix(arg, "--config="):
			probe.ConfigFile = strings.TrimPrefix(arg, "--config=")
		}
	}
	return probe, nil
}

// ParseFlags applies command line flags on top of the current configuration.
func (c *Config) ParseFlags(args []string) error {
	need := func(i *int, name string) (string, error) {
		if *i+1 >= len(args) {
			return "", fmt.Errorf("%s requires an argument", name)
		}
		*i++
		return args[*i], nil
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		value := ""
		inline := false
		if idx := strings.Index(arg, "="); idx > 0 && strings.HasPrefix(arg, "--") {
			value = arg[idx+1:]
			arg = arg[:idx]
			inline = true
		}
		take := func(name string) (string, error) {
			if inline {
				return value, nil
			}
			return need(&i, name)
		}

		switch arg {
		case "-p", "--prompt":
			v, err := take(arg)
			if err != nil {
				return err
			}
			c.Prompt = v
		case "--stdin":
			c.UseStdin = true
		case "--prompt-file":
			v, err := take("--prompt-file")
			if err != nil {
				return err
			}
			c.PromptFile = v
		case "--output":
			v, err := take("--output")
			if err != nil {
				return err
			}
			c.OutputFile = v
		case "--verbose":
			c.Verbose = true
		case "--quiet":
			c.Quiet = true
		case "--api-endpoint":
			v, err := take("--api-endpoint")
			if err != nil {
				return err
			}
			c.APIEndpoint = strings.TrimRight(v, "/")
		case "--api-key":
			v, err := take("--api-key")
			if err != nil {
				return err
			}
			c.APIKey = v
		case "--model":
			v, err := take("--model")
			if err != nil {
				return err
			}
			c.Model = v
		case "--temperature":
			v, err := take("--temperature")
			if err != nil {
				return err
			}
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return fmt.Errorf("invalid --temperature value %q", v)
			}
			c.Temperature = &f
		case "--max-tokens":
			v, err := take("--max-tokens")
			if err != nil {
				return err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid --max-tokens value %q", v)
			}
			c.MaxTokens = n
		case "--top-p":
			v, err := take("--top-p")
			if err != nil {
				return err
			}
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return fmt.Errorf("invalid --top-p value %q", v)
			}
			c.TopP = &f
		case "--stream":
			c.Stream = true
		case "--no-stream":
			c.Stream = false
		case "--initial-token-timeout":
			v, err := take("--initial-token-timeout")
			if err != nil {
				return err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid --initial-token-timeout value %q", v)
			}
			c.InitialTokenTimeout = n
		case "--connect-timeout":
			v, err := take("--connect-timeout")
			if err != nil {
				return err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid --connect-timeout value %q", v)
			}
			c.ConnectTimeout = n
		case "--read-timeout":
			v, err := take("--read-timeout")
			if err != nil {
				return err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid --read-timeout value %q", v)
			}
			c.ReadTimeout = n
		case "--context-size":
			v, err := take("--context-size")
			if err != nil {
				return err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid --context-size value %q", v)
			}
			c.ContextSize = n
		case "--context-threshold":
			v, err := take("--context-threshold")
			if err != nil {
				return err
			}
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return fmt.Errorf("invalid --context-threshold value %q", v)
			}
			c.ContextThreshold = f
		case "--max-iterations":
			v, err := take("--max-iterations")
			if err != nil {
				return err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid --max-iterations value %q", v)
			}
			c.MaxIterations = n
		case "--no-compress":
			c.AutoCompress = false
		case "--read-only":
			c.ReadOnly = true
		case "--persona":
			v, err := take("--persona")
			if err != nil {
				return err
			}
			c.Persona = v
		case "--summary-only":
			c.SummaryOnly = true
		case "--goal":
			v, err := take("--goal")
			if err != nil {
				return err
			}
			c.Goal = v
		case "--theme":
			v, err := take("--theme")
			if err != nil {
				return err
			}
			c.Theme = v
		case "--debug":
			c.Debug = true
		case "--debug-log":
			v, err := take("--debug-log")
			if err != nil {
				return err
			}
			c.DebugLog = v
		case "--config":
			if !inline {
				if _, err := need(&i, "--config"); err != nil {
					return err
				}
			}
		case "--load":
			v, err := take("--load")
			if err != nil {
				return err
			}
			c.ContextFile = v
		case "--no-dump-on-exit":
			c.NoDumpOnExit = true
		case "--cwd":
			v, err := take("--cwd")
			if err != nil {
				return err
			}
			c.WorkingDir = v
		case "--max-history":
			v, err := take("--max-history")
			if err != nil {
				return err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid --max-history value %q", v)
			}
			c.MaxHistory = n
		case "--subagent-timeout":
			v, err := take("--subagent-timeout")
			if err != nil {
				return err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid --subagent-timeout value %q", v)
			}
			c.SubagentTimeout = n
		case "--web":
			c.Web = true
		case "--web-addr":
			v, err := take("--web-addr")
			if err != nil {
				return err
			}
			c.WebAddr = v
		case "--web-port":
			v, err := take("--web-port")
			if err != nil {
				return err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid --web-port value %q", v)
			}
			c.WebPort = n
		case "--skills":
			c.SkillsEnabled = true
		case "--no-skills":
			c.SkillsEnabled = false
		case "--skills-dir":
			v, err := take("--skills-dir")
			if err != nil {
				return err
			}
			c.SkillsDirs = append(c.SkillsDirs, v)
		case "--trust":
			v, err := take("--trust")
			if err != nil {
				return err
			}
			c.TrustDirs = append(c.TrustDirs, v)
		case "-h", "--help":
			c.ShowHelp = true
		case "-v", "--version":
			c.ShowVersion = true
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown flag: %s", arg)
			}
			// Positional argument: treat as a one-shot prompt when nothing else was given.
			if c.Prompt == "" {
				c.Prompt = arg
			}
		}
	}
	c.DetectMode()
	return nil
}

// Validate checks the resolved configuration and applies the theme.
func (c *Config) Validate() error {
	if c.ContextSize <= 0 {
		return fmt.Errorf("context size must be positive (got %d)", c.ContextSize)
	}
	if c.ContextThreshold <= 0 || c.ContextThreshold > 1 {
		return fmt.Errorf("context threshold must be in (0, 1] (got %v)", c.ContextThreshold)
	}
	if c.MaxIterations <= 0 {
		return fmt.Errorf("max iterations must be positive (got %d)", c.MaxIterations)
	}
	if c.InitialTokenTimeout < 10 {
		return fmt.Errorf("initial token timeout must be at least 10 seconds (got %d)", c.InitialTokenTimeout)
	}
	if c.ConnectTimeout <= 0 {
		return fmt.Errorf("connect timeout must be positive (got %d)", c.ConnectTimeout)
	}
	if c.ReadTimeout < 0 {
		return fmt.Errorf("read timeout must not be negative (got %d)", c.ReadTimeout)
	}
	if c.Temperature != nil && (*c.Temperature < 0 || *c.Temperature > 2) {
		return fmt.Errorf("temperature must be between 0 and 2 (got %v)", *c.Temperature)
	}
	if !colors.ValidTheme(c.Theme) {
		return fmt.Errorf("invalid theme %q: valid themes are: %s", c.Theme, strings.Join(colors.ListThemes(), ", "))
	}
	if err := colors.SetTheme(c.Theme); err != nil {
		return err
	}
	if c.WebPort <= 0 || c.WebPort > 65535 {
		return fmt.Errorf("web port must be between 1 and 65535 (got %d)", c.WebPort)
	}
	if c.APIEndpoint == "" {
		return fmt.Errorf("api endpoint must not be empty")
	}
	if c.Model == "" {
		return fmt.Errorf("model must not be empty")
	}
	c.DetectMode()
	return nil
}

// DetectMode resolves the run mode from the provided options.
func (c *Config) DetectMode() {
	switch {
	case c.Web:
		c.Mode = WebMode
	case c.Prompt != "" || c.PromptFile != "" || c.UseStdin || c.ContextFile != "":
		c.Mode = OneShotMode
	default:
		c.Mode = InteractiveMode
	}
}

// IsCopilotEndpoint reports whether a base URL points at GitHub Copilot.
func IsCopilotEndpoint(base string) bool {
	u := strings.ToLower(strings.TrimRight(base, "/"))
	return strings.Contains(u, "api.githubcopilot.com")
}

// IsGitHubModelsEndpoint reports whether a base URL points at GitHub Models.
func IsGitHubModelsEndpoint(base string) bool {
	u := strings.ToLower(strings.TrimRight(base, "/"))
	return strings.Contains(u, "models.github.ai") || strings.Contains(u, "inference.github.ai")
}

// IsCopilotEndpoint reports whether the configured endpoint is GitHub Copilot.
func (c *Config) IsCopilotEndpoint() bool { return IsCopilotEndpoint(c.APIEndpoint) }

// IsGitHubModelsEndpoint reports whether the endpoint is GitHub Models.
func (c *Config) IsGitHubModelsEndpoint() bool { return IsGitHubModelsEndpoint(c.APIEndpoint) }

// EnabledMCPServers returns the MCP servers that are enabled.
func (c *Config) EnabledMCPServers() []MCPServerConfig {
	out := make([]MCPServerConfig, 0, len(c.MCPServers))
	for _, srv := range c.MCPServers {
		if srv.Enabled {
			out = append(out, srv)
		}
	}
	return out
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func atoiDefault(v string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
		return n
	}
	return def
}

// splitArgs splits a command argument string, honouring simple quoting.
func splitArgs(v string) []string {
	var out []string
	var cur strings.Builder
	quote := rune(0)
	for _, r := range v {
		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote != 0:
			cur.WriteRune(r)
		case r == '"' || r == '\'':
			quote = r
		case r == ' ' || r == '\t':
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}
