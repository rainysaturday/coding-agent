package tools

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// executeGitBlame shows git blame for files or lists most recently modified files.
func (te *ToolExecutor) executeGitBlame(params map[string]interface{}) *ToolResult {
	action, ok := params["action"].(string)
	if !ok || action == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: action (use 'blame' or 'recent')",
		}
	}

	switch action {
	case "blame":
		return te.gitBlameBlame(params)
	case "recent":
		return te.gitBlameRecent(params)
	default:
		return &ToolResult{
			Success: false,
			Error: fmt.Sprintf("unknown action: %s. Valid actions: 'blame', 'recent'", action),
		}
	}
}

// gitBlameBlame shows git blame for specific files/paths.
func (te *ToolExecutor) gitBlameBlame(params map[string]interface{}) *ToolResult {
	path, hasPath := params["path"].(string)
	if !hasPath || path == "" {
		return &ToolResult{
			Success: false,
			Error: "missing required parameter: path (file or directory to show blame for)",
		}
	}

	// Optional: start/end line range
	startLine, hasStart := params["start"].(float64)
	endLine, hasEnd := params["end"].(float64)

	// Optional: reverse blame order (newest first)
	reverse := false
	if r, ok := params["reverse"].(bool); ok {
		reverse = r
	}

	// Optional: date format for blame output
	dateFormat := "relative"
	if df, ok := params["date"].(string); ok {
		dateFormat = df
	}

	// Optional: line range (e.g., 1-10 for first 10 lines)
	lineRange := ""
	if hasStart && hasEnd {
		lineRange = fmt.Sprintf("%d-%d", int(startLine), int(endLine))
	}

	// Build git blame arguments
	args := []string{"blame", "--porcelain"}
	if reverse {
		args = append(args, "--reverse")
	}
	if dateFormat != "" {
		args = append(args, "--date", dateFormat)
	}

	// Add path range if specified
	if lineRange != "" {
		args = append(args, lineRange, "--", path)
	} else {
		args = append(args, "--", path)
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error: fmt.Sprintf("git blame failed: %s\nOutput: %s", err, string(output)),
		}
	}

	// Parse porcelain output
	result := te.parseBlamePorcelain(string(output), path)

	return result
}

// gitBlameRecent lists the most recently modified files in the repository.
func (te *ToolExecutor) gitBlameRecent(params map[string]interface{}) *ToolResult {
	// Optional: limit number of files shown
	maxResults := 20
	if mr, ok := params["max_results"].(float64); ok {
		maxResults = int(mr)
	}

	// Optional: path pattern to filter
	pathFilter, hasPathFilter := params["path"].(string)

	// Get recent commits with files changed
	// Use pipe delimiter to avoid space issues in author names
	logArgs := []string{"log", "-n", strconv.Itoa(maxResults * 3), "--name-only", "--format=%H|%ct|%an|%ae|%s"}
	if hasPathFilter && pathFilter != "" {
		logArgs = append(logArgs, "--", pathFilter)
	}

	cmd := exec.Command("git", logArgs...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &ToolResult{
			Success: false,
			Error: fmt.Sprintf("git log failed: %s\nOutput: %s", err, string(output)),
		}
	}

	// Parse the log output to extract commits and files
	type commitInfo struct {
		hash      string
		timestamp int64
		author    string
		email     string
		message   string
		files     []string
	}

	var commits []*commitInfo
	var currentCommit *commitInfo

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if this is a commit header line
		// Format: hash|timestamp|author_name|author_email|subject
		parts := strings.SplitN(line, "|", 5)
		if len(parts) >= 5 && len(parts[0]) == 40 && isHex(parts[0]) {
			timestamp, tsErr := strconv.ParseInt(parts[1], 10, 64)
			if tsErr == nil {
				currentCommit = &commitInfo{
					hash:      parts[0],
					timestamp: timestamp,
					author:    parts[2],
					email:     parts[3],
					message:   parts[4],
				}
				commits = append(commits, currentCommit)
				continue
			}
		}

		// This is a file name
		if currentCommit != nil {
			// Skip .git and other git metadata
			if !strings.HasPrefix(line, ".git/") && line != ".git" {
				currentCommit.files = append(currentCommit.files, line)
			}
		}
	}

	// Sort commits by timestamp (most recent first)
	sort.Slice(commits, func(i, j int) bool {
		return commits[i].timestamp > commits[j].timestamp
	})

	// Build the output
	var outputStr strings.Builder
	outputStr.WriteString("Most recently modified files (by last commit):\n\n")

	shown := 0
	for _, c := range commits {
		if shown >= maxResults {
			break
		}
		if len(c.files) == 0 {
			continue
		}
		shown++

		// Format timestamp as readable date
		dateStr := formatTimestamp(c.timestamp)

		outputStr.WriteString(fmt.Sprintf("Commit %s (%s)\n", c.hash, dateStr))
		outputStr.WriteString(fmt.Sprintf("  Author: %s <%s>\n", c.author, c.email))
		outputStr.WriteString(fmt.Sprintf("  Message: %s\n", c.message))
		outputStr.WriteString("  Files changed:\n")
		for _, f := range c.files {
			outputStr.WriteString(fmt.Sprintf("    - %s\n", f))
		}
		outputStr.WriteString("\n")
	}

	if shown == 0 {
		outputStr.WriteString("No recently modified files found.\n")
	}

	return &ToolResult{
		Success: true,
		Output:  outputStr.String(),
		Extra: map[string]interface{}{
			"tool":         "git_blame",
			"action":       "recent",
			"commitsShown": shown,
		},
	}
}

// parseBlamePorcelain parses git blame porcelain format output.
func (te *ToolExecutor) parseBlamePorcelain(output, path string) *ToolResult {
	type blameEntry struct {
		FullHash   string
		Suffix     int
		LineNumber int
		Author     string
		Date       string
		Content    string
	}

	var entries []blameEntry
	var currentEntry *blameEntry

	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}

		// Check if this is a hash line: <hash> <suffix> <lno> [original-lno]
		// Hash is always 40 hex characters at the start
		if len(line) >= 40 && isHex(line[:40]) && line[40] == ' ' {
			// Save previous entry if exists
			if currentEntry != nil && currentEntry.Content == "" && len(lines) > i+1 {
				// The entry has no content line (empty file line), add it
				entries = append(entries, *currentEntry)
			}

			parts := strings.Fields(line)
			if len(parts) >= 2 {
				hash := parts[0]
				suffix := 0
				if n, err := strconv.Atoi(parts[1]); err == nil {
					suffix = n
				}

				lno := 0
				if len(parts) >= 3 {
					if n, err := strconv.Atoi(parts[2]); err == nil {
						lno = n
					}
				}

				currentEntry = &blameEntry{
					FullHash:   hash,
					Suffix:     suffix,
					LineNumber: lno,
				}
			}
			continue
		}

		// Metadata lines start with known keys
		if strings.HasPrefix(line, "author ") {
			if currentEntry != nil {
				currentEntry.Author = strings.TrimPrefix(line, "author ")
			}
		} else if strings.HasPrefix(line, "author-mail ") || strings.HasPrefix(line, "author-time ") || strings.HasPrefix(line, "author-tz ") {
			if currentEntry != nil && strings.HasPrefix(line, "author-time ") {
				currentEntry.Date = strings.TrimPrefix(line, "author-time ")
			}
		} else if strings.HasPrefix(line, "committer ") || strings.HasPrefix(line, "committer-mail ") || strings.HasPrefix(line, "committer-time ") || strings.HasPrefix(line, "committer-tz ") {
			// Skip committer metadata
		} else if strings.HasPrefix(line, "summary ") || strings.HasPrefix(line, "filename ") {
			// Skip summary and filename
		} else if strings.HasPrefix(line, "previous ") || strings.HasPrefix(line, "rename ") {
			// Skip these metadata lines
		} else if strings.HasPrefix(line, "boundary") {
			// Skip boundary markers for merges
		} else if currentEntry != nil {
			// This is a content line (may start with tab in porcelain format)
			currentEntry.Content = strings.TrimPrefix(line, "\t")

			// Convert timestamp to human-readable date
			if currentEntry.Date != "" {
				if ts, err := strconv.ParseInt(currentEntry.Date, 10, 64); err == nil {
					currentEntry.Date = formatTimestamp(ts)
				}
			}

			// Add entry immediately
			entries = append(entries, *currentEntry)
			currentEntry = nil
		}
	}

	// Handle last entry if it wasn't followed by a content line
	if currentEntry != nil && currentEntry.Content == "" {
		entries = append(entries, *currentEntry)
	}

	// Sort by line number
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LineNumber < entries[j].LineNumber
	})

	// Format output
	var outputStr strings.Builder
	outputStr.WriteString(fmt.Sprintf("Git blame for: %s\n", path))
	outputStr.WriteString(fmt.Sprintf("Total lines: %d\n\n", len(entries)))

	// Limit output to avoid context overflow
	maxLines := 100
	if len(entries) > maxLines {
		entries = entries[:maxLines]
		outputStr.WriteString(fmt.Sprintf("(Showing first %d of %d lines)\n\n", maxLines, len(entries)))
	}

	for _, e := range entries {
		// Format: <short_hash> (<author> <date> <line_num>) <content>
		shortHash := e.FullHash
		if len(e.FullHash) > 7 {
			shortHash = e.FullHash[:7]
		}

		// Truncate content if too long
		content := e.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}

		outputStr.WriteString(fmt.Sprintf("%s (%s %s %d) %s\n", shortHash, e.Author, e.Date, e.LineNumber, content))
	}

	// Summary statistics
	authorCount := make(map[string]int)
	for _, e := range entries {
		authorCount[e.Author]++
	}

	// Sort authors by contribution count
	type authorStat struct {
		Author string
		Count  int
	}
	var authors []authorStat
	for a, c := range authorCount {
		authors = append(authors, authorStat{a, c})
	}
	sort.Slice(authors, func(i, j int) bool {
		return authors[i].Count > authors[j].Count
	})

	outputStr.WriteString("\n---\nAuthor contributions:\n")
	for _, a := range authors {
		outputStr.WriteString(fmt.Sprintf("  %s: %d lines\n", a.Author, a.Count))
	}

	return &ToolResult{
		Success: true,
		Output:  outputStr.String(),
		Extra: map[string]interface{}{
			"tool":          "git_blame",
			"path":          path,
			"totalLines":    len(entries),
			"uniqueAuthors": len(authorCount),
			"topAuthors":    authors,
		},
	}
}

// isHex checks if a string contains only hex characters.
func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return len(s) > 0
}

// formatTimestamp converts a unix timestamp to a human-readable date string.
func formatTimestamp(ts int64) string {
	t := time.Unix(ts, 0)
	return t.Format("2006-01-02 15:04:05")
}
