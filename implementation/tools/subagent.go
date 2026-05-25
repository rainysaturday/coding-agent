package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ANSI color codes for tool feedback
const (
	ColorReset   = "\033[0m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorRed     = "\033[31m"
	ColorCyan    = "\033[36m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
)

// formatSubagentResult formats the subagent result for display in the TUI.
func formatSubagentResult(result *ToolResult) string {
	if result.Success {
		output := result.Output
		if len(output) > 200 {
			output = output[:200] + "\n... [subagent output truncated]"
		}
		return fmt.Sprintf("%s[Subagent] Task completed\nOutput:\n%s%s\n", ColorCyan, output, ColorReset)
	}
	return fmt.Sprintf("%s[Subagent] Failed: %s%s\n", ColorRed, result.Error, ColorReset)
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

	// Determine if we're in read-only mode by checking the environment
	readOnly := os.Getenv("CODING_AGENT_READ_ONLY") == "true"

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

	// Add read-only flag if needed
	if readOnly {
		args = append(args, "--read-only")
	}

	// Build the command
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

	// Try to extract the "Final Output" section if present
	if idx := strings.Index(output, "=== Final Output ==="); idx != -1 {
		// Find the text after "=== Final Output ==="
		summaryStart := idx + len("=== Final Output ===")
		summary := strings.TrimSpace(output[summaryStart:])
		if summary != "" {
			return summary
		}
	}

	// Try to extract text after "[Final Output]" or "[Result]" markers
	for _, marker := range []string{"[Final Output]", "[Result]", "[Output]"} {
		if idx := strings.Index(output, marker); idx != -1 {
			summary := strings.TrimSpace(output[idx+len(marker):])
			if summary != "" && len(summary) < 10000 {
				return summary
			}
		}
	}

	// Look for the last substantial text block (after any tool output)
	// This handles the case where the output has multiple sections
	lines := strings.Split(output, "\n")

	// Find the last non-empty line that's not a separator or header
	var lastSignificantLine string
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		// Skip separator lines
		if strings.HasPrefix(line, "===") || strings.HasPrefix(line, "---") {
			continue
		}
		// Skip short lines that are likely headers
		if len(line) < 10 {
			continue
		}
		lastSignificantLine = line
		break
	}

	// If we found a significant line, return everything from that line backward
	if lastSignificantLine != "" {
		for i, line := range lines {
			if strings.TrimSpace(line) == lastSignificantLine {
				return strings.Join(lines[i:], "\n")
			}
		}
	}

	// Fall back to the raw output (trimmed)
	// But limit the length to avoid overwhelming the main agent
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
