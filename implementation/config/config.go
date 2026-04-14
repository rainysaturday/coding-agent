package config

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	APIEndpoint   string
	APIKey        string
	Model         string
	ContextSize   int
	MaxIterations int
	HistorySize   int
	Temperature   float64
	MaxTokens     int
	NoStream      bool
	Prompt        string
	UseStdin      bool
	PromptFile    string
	Verbose       bool
	Quiet         bool
	OutputFile    string
	Debug         bool
	DebugLog      string
	Version       bool
	ConfigFile    string
}

func Load(version string) (Config, error) {
	cfg := Config{}

	prompt := flag.String("prompt", "", "Prompt for one-shot mode")
	flag.StringVar(prompt, "p", "", "Prompt for one-shot mode")
	stdin := flag.Bool("stdin", false, "Read prompt from stdin")
	promptFile := flag.String("prompt-file", "", "Read prompt from file")
	debug := flag.Bool("debug", false, "Enable debug logging")
	debugLog := flag.String("debug-log", "", "Path to debug log file")
	model := flag.String("model", "", "Model to use")
	temperature := flag.Float64("temperature", -1, "Inference temperature")
	maxTokens := flag.Int("max-tokens", -1, "Maximum tokens to generate")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	quiet := flag.Bool("quiet", false, "Suppress non-essential output")
	output := flag.String("output", "", "Write results to file")
	noStream := flag.Bool("no-stream", false, "Disable streaming output")
	maxIterations := flag.Int("max-iterations", -1, "Maximum iterations")
	historySize := flag.Int("history-size", -1, "Maximum prompt history entries")
	contextSize := flag.Int("context-size", -1, "Context window size")
	apiEndpoint := flag.String("api-endpoint", "", "OpenAI-compatible endpoint")
	apiKey := flag.String("api-key", "", "API key")
	configFile := flag.String("config", "", "Path to config file")
	versionFlag := flag.Bool("version", false, "Show version")
	flag.BoolVar(versionFlag, "v", false, "Show version")

	flag.Usage = func() {
		fmt.Println("Minimal Coding Agent Harness")
		fmt.Println("\nUsage:")
		fmt.Println("  coding-agent [OPTIONS]")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		fmt.Println("\nGitHub Copilot setup:")
		fmt.Println("  export CODING_AGENT_API_ENDPOINT=\"https://api.githubcopilot.com\"")
		fmt.Println("  export GITHUB_TOKEN=\"ghu_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\"")
		fmt.Println("  coding-agent --model gpt-4o")
		fmt.Println("\nGitHub Models setup (official API):")
		fmt.Println("  export CODING_AGENT_API_ENDPOINT=\"https://models.github.ai\"")
		fmt.Println("  export CODING_AGENT_API_KEY=\"github_pat_xxxxxxxxxxxxxxxxxxxx\"")
		fmt.Println("  coding-agent --model openai/gpt-4.1")
		fmt.Printf("\nVersion: %s\n", version)
	}

	flag.Parse()

	cfg.ConfigFile = firstNonEmpty(*configFile, os.Getenv("CODING_AGENT_CONFIG"))
	kv := map[string]string{}
	if cfg.ConfigFile != "" {
		loaded, err := parseConfigFile(cfg.ConfigFile)
		if err != nil {
			return cfg, err
		}
		kv = loaded
	}

	cfg.APIEndpoint = firstNonEmpty(*apiEndpoint, os.Getenv("CODING_AGENT_API_ENDPOINT"), kv["api_endpoint"], "http://localhost:8080")
	cfg.APIKey = firstNonEmpty(*apiKey, os.Getenv("CODING_AGENT_API_KEY"), os.Getenv("GITHUB_TOKEN"), kv["api_key"])
	cfg.Model = firstNonEmpty(*model, os.Getenv("CODING_AGENT_MODEL"), kv["model"], "llama3")
	cfg.ContextSize = firstNonEmptyInt(*contextSize, atoiEnv("CODING_AGENT_CONTEXT_SIZE"), atoiSafe(kv["context_size"]), 128000)
	cfg.MaxIterations = firstNonEmptyInt(*maxIterations, atoiEnv("CODING_AGENT_MAX_ITERATIONS"), atoiSafe(kv["max_iterations"]), 1000)
	cfg.HistorySize = firstNonEmptyInt(*historySize, atoiEnv("CODING_AGENT_HISTORY_SIZE"), atoiSafe(kv["history_size"]), 100)
	cfg.Temperature = firstNonEmptyFloat(*temperature, atofEnv("CODING_AGENT_TEMPERATURE"), atofSafe(kv["temperature"]), 0.7)
	cfg.MaxTokens = firstNonEmptyInt(*maxTokens, atoiEnv("CODING_AGENT_MAX_TOKENS"), atoiSafe(kv["max_tokens"]), 4096)
	cfg.Debug = *debug || strings.EqualFold(os.Getenv("CODING_AGENT_DEBUG"), "true")
	cfg.DebugLog = firstNonEmpty(*debugLog, os.Getenv("CODING_AGENT_DEBUG_LOG"), kv["debug_log"], "debug.log")
	cfg.NoStream = *noStream
	cfg.Prompt = *prompt
	cfg.UseStdin = *stdin
	cfg.PromptFile = *promptFile
	cfg.Verbose = *verbose
	cfg.Quiet = *quiet
	cfg.OutputFile = *output
	cfg.Version = *versionFlag

	if cfg.ContextSize <= 0 {
		return cfg, fmt.Errorf("context-size must be positive")
	}
	if cfg.MaxIterations <= 0 {
		return cfg, fmt.Errorf("max-iterations must be positive")
	}
	if cfg.HistorySize <= 0 {
		return cfg, fmt.Errorf("history-size must be positive")
	}

	return cfg, nil
}

func parseConfigFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := map[string]string{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		out[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func firstNonEmptyInt(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

func firstNonEmptyFloat(values ...float64) float64 {
	for _, v := range values {
		if v >= 0 {
			return v
		}
	}
	return 0
}

func atoiEnv(key string) int {
	return atoiSafe(os.Getenv(key))
}

func atoiSafe(v string) int {
	i, _ := strconv.Atoi(strings.TrimSpace(v))
	return i
}

func atofEnv(key string) float64 {
	return atofSafe(os.Getenv(key))
}

func atofSafe(v string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
	return f
}

func ReadPrompt(cfg Config) (string, bool, error) {
	if strings.TrimSpace(cfg.Prompt) != "" {
		return cfg.Prompt, true, nil
	}
	if cfg.PromptFile != "" {
		b, err := os.ReadFile(cfg.PromptFile)
		return string(b), true, err
	}
	if cfg.UseStdin {
		b, err := ioReadAll(os.Stdin)
		return string(b), true, err
	}
	return "", false, nil
}

func ioReadAll(f *os.File) ([]byte, error) {
	buf := new(strings.Builder)
	s := bufio.NewScanner(f)
	for s.Scan() {
		buf.WriteString(s.Text())
		buf.WriteString("\n")
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}

func DefaultHTTPTimeout() time.Duration {
	return 120 * time.Second
}
