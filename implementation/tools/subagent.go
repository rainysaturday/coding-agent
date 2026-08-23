package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// defaultSubagentTimeout is the maximum time a subagent may run before it is
// killed. Subagents can legitimately take a while (multiple LLM round-trips),
// but they must be bounded so a hung subprocess cannot block the parent forever.
const defaultSubagentTimeout = 5 * time.Minute

// executeSubagent runs a subagent by spawning a subprocess of the coding-agent binary.
// It passes the prompt and persona to the subagent and captures only the summary output.
// Configuration is inherited from the parent process via environment variables and CLI flags.
// The provided ctx is propagated to the child so cancellation or timeout kills it.
func executeSubagent(ctx context.Context, params map[string]interface{}, binaryPath string) *ToolResult {
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

	// Tools: pass explicitly if set via env var
	if toolsEnv := os.Getenv("CODING_AGENT_TOOLS"); toolsEnv != "" {
		args = append(args, "--tools", toolsEnv)
	}

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

	// Build the command with inherited environment. Use a child context that
	// respects both the parent cancellation and the subagent timeout so a hung
	// subagent cannot block the parent forever and is killed when cancelled.
	childCtx, cancel := context.WithTimeout(ctx, defaultSubagentTimeout)
	defer cancel()
	cmd := exec.CommandContext(childCtx, binaryPath, args...)

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
		// Surface timeout/cancellation clearly so it isn't mistaken for a normal failure.
		if childCtx.Err() == context.DeadlineExceeded {
			errorMsg = "subagent timed out after " + defaultSubagentTimeout.String() + ": " + errorMsg
		} else if childCtx.Err() == context.Canceled {
			errorMsg = "subagent was cancelled: " + errorMsg
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

// ansiEscapeRe matches ANSI CSI escape sequences (e.g. "\x1b[31m") so color
// codes from the subagent's piped stdout do not leak into the parent's context.
var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripANSI removes ANSI escape sequences from s.
func stripANSI(s string) string {
	return ansiEscapeRe.ReplaceAllString(s, "")
}

// extractSummary extracts the meaningful summary from the subagent output.
// It tries to find the final output or the last meaningful text block.
func extractSummary(output string) string {
	// If output is empty, return placeholder
	if output == "" {
		return "(No output from subagent)"
	}

	// Strip ANSI escape sequences so color codes don't leak into the summary.
	cleaned := stripANSI(output)

	// In --quiet mode the child's stdout is "[Reasoning]\n<reasoning>\n\n<final
	// answer>". The reasoning is the subagent's internal deliberation, not its
	// conclusion, so drop that section and keep only what follows the blank line
	// after it (the final answer).
	if idx := strings.Index(cleaned, "[Reasoning]"); idx != -1 {
		after := cleaned[idx+len("[Reasoning]"):]
		if blank := strings.Index(after, "\n\n"); blank != -1 {
			cleaned = after[blank+2:]
		}
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
		if idx := strings.Index(cleaned, sm.marker); idx != -1 {
			after := cleaned[idx+len(sm.marker):]
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
	lines := strings.Split(cleaned, "\n")

	// Find the LAST significant paragraph — defined as a block of 3+ related lines
	// that are not separators, headers, or tool output artifacts. Only the final
	// block is returned (scanning backwards and stopping at the first one found),
	// so an earlier block such as a reasoning section is not mistaken for the
	// subagent's conclusion.
	var lastParagraph []string
	currentBlock := make([]string, 0, 10)

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])

		// Skip empty lines, separators, and very short lines (headers/artifacts).
		if line == "" || strings.HasPrefix(line, "===") || strings.HasPrefix(line, "---") || len(line) < 5 {
			if len(currentBlock) >= 3 {
				// Found the last substantial block — stop scanning.
				lastParagraph = append(lastParagraph, currentBlock...)
				break
			}
			currentBlock = make([]string, 0, 10)
			continue
		}

		currentBlock = append(currentBlock, line)
	}

	// Check the final accumulated block (output ending directly in a block).
	if len(lastParagraph) == 0 && len(currentBlock) >= 3 {
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

	// Strategy 3: Fall back to the cleaned output (trimmed) with length limit.
	cleaned = strings.TrimSpace(cleaned)
	if len(cleaned) > 5000 {
		return cleaned[:5000] + "\n... [output truncated]"
	}

	return cleaned
}

// ExecuteSubagent is the main entry point for the subagent tool.
// It's called by the tool executor and handles getting the binary path.
// The provided ctx is propagated to the subprocess so cancellation or timeout
// kills it rather than leaving an orphaned process.
func ExecuteSubagent(ctx context.Context, params map[string]interface{}) *ToolResult {
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

	return executeSubagent(ctx, params, binaryPath)
}
