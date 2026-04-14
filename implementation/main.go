package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"coding-agent/implementation/agent"
	"coding-agent/implementation/config"
	"coding-agent/implementation/debug"
	"coding-agent/implementation/tui"
)

var (
	version = "dev"
	gitHash = "unknown"
	gitTree = "dirty"
)

func main() {
	cfg, err := config.Load(fullVersion())
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	if cfg.Version {
		fmt.Println(fullVersion())
		return
	}

	logger, err := debug.New(cfg.Debug, cfg.DebugLog, fullVersion())
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	defer logger.Close()

	ag := agent.New(cfg, logger)

	prompt, oneShot, err := config.ReadPrompt(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	if oneShot {
		if strings.TrimSpace(prompt) == "" {
			fmt.Fprintln(os.Stderr, "Error: empty one-shot prompt")
			os.Exit(1)
		}
		ag.AddUserMessage(strings.TrimSpace(prompt))
		writer := tui.New(cfg.HistorySize)
		out, err := ag.RunOnce(context.Background(), writer)
		fmt.Println()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		if cfg.OutputFile != "" {
			if err := os.WriteFile(cfg.OutputFile, []byte(out), 0o644); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
		}
		if !cfg.Quiet {
			fmt.Println(out)
		}
		return
	}

	ui := tui.New(cfg.HistorySize)
	if err := ui.Run(context.Background(), ag); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func fullVersion() string {
	return fmt.Sprintf("%s %s [%s]", version, gitHash, gitTree)
}
