package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/coding-agent/harness/colors"
)

// formatSubagentResult formats the subagent result for display in the TUI.
func formatSubagentResult(result *ToolResult) string {
	if result.Success {
		output := result.Output
		if len(output) > 200 {
			output = output[:200] + "\n... [subagent output truncated]"
		}
		return fmt.Sprintf("%s[Subagent] Task completed\nOutput:\n%s%s\n", colors.GetColor("cyan"), output, colors.GetColor("reset"))
	}
	return fmt.Sprintf("%s[Subagent] Failed: %s%s\n", colors.GetColor("red"), result.Error, colors.GetColor("reset"))
}

// streamSubagentResult streams a subagent result status message.
func streamSubagentResult(result *ToolResult, callback func(chunk interface{})) {
	status := formatSubagentResult(result)
	if callback != nil {
		// Create a streaming chunk with the status
		chunk := struct {
			Text        string
			ContentType int
		}{
			Text:        status,
			ContentType: 0, // Normal content type
		}
		callback(chunk)
	} else {
		fmt.Print(status)
	}
}

// executeSubagent runs a subagent by spawning a subprocess of the coding-agent binary.
// It passes the prompt and persona to the subagent and captures only the summary output.
// Configuration is inherited from the parent process via environment variables and CLI flags.
func executeSubagent(params map[string]interface{}, binaryPath string) *ToolResult {
	prompt, ok := params["prompt"].(string)
	if !ok || prompt == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: prompt",
		}
	}

	persona := ""
	if p, ok := params["persona"].(string); ok && p != "" {
		persona = p
	}

	// Get the binary path if not provided
	if binaryPath == "" {
		binaryPath = getExecutablePath()
	}

	// Build the command to run the subagent
	// We use --summary-only to get just the conclusion
	args := []string{
		"--prompt-file", "-", // Read prompt from stdin
		"--summary-only", // Only return the summary
		"--no-stream",    // Disable streaming for cleaner output
		"--quiet",        // Minimize noise
	}

	// Add persona if specified
	if persona != "" {
		args = append(args, "--persona", persona)
	}

	// Inherit configuration from parent process via environment variables.
	// These are set by the parent and will be inherited by the subprocess automatically.
	// For CLI flags that are not environment-backed, we pass them explicitly.

	// Read-only mode: check environment variable
	if os.Getenv("CODING_AGENT_READ_ONLY") == "true" {
		args = append(args, "--read-only")
	}

	// Experimental mode: check environment variable
	if os.Getenv("CODING_AGENT_EXPERIMENTAL") == "true" {
		args = append(args, "--experimental")
	}

	// Theme: pass explicitly if set via env var (CLI flag override is harder to detect,
	// but the env var is inherited by the subprocess automatically)
	if theme := os.Getenv("CODING_AGENT_THEME"); theme != "" {
		args = append(args, "--theme", theme)
	}

	// Debug settings are inherited via environment variables automatically

	// Build the command with inherited environment
	cmd := exec.Command(binaryPath, args...)

	// Set working directory to current directory
	cwd, err := os.Getwd()
	if err == nil {
		cmd.Dir = cwd
	}

	// Write prompt to stdin
	cmd.Stdin = strings.NewReader(prompt)

	// Capture output
	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err = cmd.Run()

	// Extract the summary from the output
	output := stdout.String()

	// If there's an error, include it
	var errorMsg string
	if err != nil {
		errorMsg = stderr.String()
		if errorMsg == "" {
			errorMsg = err.Error()
		}
	}

	// Clean up the output - extract just the meaningful summary
	summary := extractSummary(output)

	// Build extra info
	extra := map[string]interface{}{
		"prompt":  prompt,
		"persona": persona,
		"summary": summary,
	}

	// If there was an error, return failure
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("subagent failed: %s", errorMsg),
			Extra:   extra,
		}
	}

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Subagent completed.\n\nSummary:\n%s", summary),
		Extra:   extra,
	}
}

// getExecutablePath returns the path to the current executable.
func getExecutablePath() string {
	exe, err := os.Executable()
	if err != nil {
		// Fallback: try to find coding-agent in PATH
		if path, err := exec.LookPath("coding-agent"); err == nil {
			return path
		}
		return "coding-agent"
	}
	return exe
}

// extractSummary extracts the meaningful summary from the subagent output.
// It tries to find the final output or the last meaningful text block.
func extractSummary(output string) string {
	// If output is empty, return placeholder
	if output == "" {
		return "(No output from subagent)"
	}

	// Strategy 1: Try to extract text after known section markers.
	// These markers appear in various output formats (summary-only, verbose, etc.).
	// Check in order of specificity (more specific markers first).
	type sectionMarker struct {
		marker   string
		minLines int // minimum non-empty lines required after marker
	}
	markers := []sectionMarker{
		{marker: "=== Final Output ===", minLines: 1},
		{marker: "[Final Output]", minLines: 1},
		{marker: "[Summary]", minLines: 1},
		{marker: "[Result]", minLines: 1},
		{marker: "[Output]", minLines: 1},
		{marker: "## Summary", minLines: 2},
		{marker: "Summary:", minLines: 2},
		{marker: "Conclusion:", minLines: 2},
	}

	// Try markers first — these give the most precise extraction
	for _, sm := range markers {
		if idx := strings.Index(output, sm.marker); idx != -1 {
			after := output[idx+len(sm.marker):]
			summary := strings.TrimSpace(after)

			// Count non-empty lines to verify this is a real section, not a false match
			lines := strings.Split(summary, "\n")
			nonEmpty := 0
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					nonEmpty++
				}
			}

			if nonEmpty >= sm.minLines && len(summary) < 10000 {
				return summary
			}
		}
	}

	// Strategy 2: Look for the last substantial paragraph (multiple lines).
	// This handles output without explicit markers.
	lines := strings.Split(output, "\n")

	// Find the last significant paragraph — defined as a block of 3+ related lines
	// that are not separators, headers, or tool output artifacts.
	var lastParagraph []string
	currentBlock := make([]string, 0, 10)

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])

		// Skip empty lines and separators
		if line == "" || strings.HasPrefix(line, "===") || strings.HasPrefix(line, "---") {
			if len(currentBlock) >= 3 {
				// Found a substantial block, save it
				lastParagraph = append(lastParagraph, currentBlock...)
				currentBlock = make([]string, 0, 10)
			} else {
				currentBlock = make([]string, 0, 10)
			}
			continue
		}

		// Skip very short lines (likely headers or artifacts)
		if len(line) < 5 {
			if len(currentBlock) >= 3 {
				lastParagraph = append(lastParagraph, currentBlock...)
				currentBlock = make([]string, 0, 10)
			} else {
				currentBlock = make([]string, 0, 10)
			}
			continue
		}

		currentBlock = append(currentBlock, line)
	}

	// Check the last accumulated block
	if len(currentBlock) >= 3 {
		lastParagraph = append(lastParagraph, currentBlock...)
	}

	if len(lastParagraph) > 0 {
		// Reverse the block (since we collected backwards) and join
		for i, j := 0, len(lastParagraph)-1; i < j; i, j = i+1, j-1 {
			lastParagraph[i], lastParagraph[j] = lastParagraph[j], lastParagraph[i]
		}
		result := strings.Join(lastParagraph, "\n")
		if len(result) < 10000 {
			return result
		}
	}

	// Strategy 3: Fall back to the raw output (trimmed) with length limit.
	if len(output) > 5000 {
		return output[:5000] + "\n... [output truncated]"
	}

	return output
}

// executeSubagentFromTool is the main entry point for the subagent tool.
// It's called by the tool executor and handles getting the binary path.
func ExecuteSubagent(params map[string]interface{}) *ToolResult {
	// Try to find the coding-agent binary
	binaryPath := getExecutablePath()

	// Also check for common locations
	candidates := []string{
		binaryPath,
		"coding-agent",
		filepath.Join(os.Getenv("HOME"), "go", "bin", "coding-agent"),
		filepath.Join(os.Getenv("GOPATH"), "bin", "coding-agent"),
	}

	for _, path := range candidates {
		if path != "" {
			// Check if the binary exists and is executable
			if _, err := os.Stat(path); err == nil {
				binaryPath = path
				break
			}
			// Try as a command in PATH
			if p, err := exec.LookPath(filepath.Base(path)); err == nil {
				binaryPath = p
				break
			}
		}
	}

	return executeSubagent(params, binaryPath)
}
