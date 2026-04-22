package tools

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// commitSuggestion represents a generated commit message suggestion.
type commitSuggestion struct {
	Type       string  `json:"type"`
	Scope      string  `json:"scope,omitempty"`
	Subject    string  `json:"subject"`
	Body       string  `json:"body,omitempty"`
	FullMessage string `json:"full_message"`
	Confidence float64 `json:"confidence"`
}

// executeGitCommitMsg generates conventional commit messages from git changes.
func (te *ToolExecutor) executeGitCommitMsg(params map[string]interface{}) *ToolResult {
	action, ok := params["action"].(string)
	if !ok || action == "" {
		action = "generate" // Default to generating from staged changes
	}

	switch action {
	case "generate", "suggest":
		return te.gitCommitMsgGenerate(params)
	default:
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s. Valid actions: 'generate', 'suggest'", action),
		}
	}
}

// gitCommitMsgGenerate analyzes changes and generates a commit message.
func (te *ToolExecutor) gitCommitMsgGenerate(params map[string]interface{}) *ToolResult {
	maxDiffLines := 300
	if md, ok := params["max_diff_lines"].(float64); ok {
		maxDiffLines = int(md)
	}

	convention := "conventional"
	if c, ok := params["convention"].(string); ok {
		convention = c
	}

	// Get staged diff
	diffOutput, err := te.getStagedDiff(maxDiffLines)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to get staged diff: %v", err),
		}
	}

	if diffOutput == "" {
		return &ToolResult{
			Success: false,
			Error:   "no staged changes. Use git_add to stage files first, or specify a different diff source.",
		}
	}

	// Analyze the diff and generate a commit message
	suggestion := te.analyzeDiffAndGenerate(diffOutput, convention)

	// Build output
	var output strings.Builder
	output.WriteString("Generated commit message:\n\n")
	output.WriteString(suggestion.FullMessage)
	output.WriteString("\n\n")
	output.WriteString(fmt.Sprintf("Confidence: %.0f%%\n", suggestion.Confidence*100))

	if suggestion.Type != "" {
		output.WriteString(fmt.Sprintf("Type: %s\n", suggestion.Type))
	}
	if suggestion.Scope != "" {
		output.WriteString(fmt.Sprintf("Scope: %s\n", suggestion.Scope))
	}
	output.WriteString(fmt.Sprintf("Subject: %s\n", suggestion.Subject))
	if suggestion.Body != "" {
		output.WriteString(fmt.Sprintf("Body:\n%s\n", suggestion.Body))
	}

	// Count changed files
	changedFiles := strings.Count(diffOutput, "diff --git")
	changedLinesAdded := strings.Count(diffOutput, "^+")
	changedLinesDeleted := strings.Count(diffOutput, "^-")

	return &ToolResult{
		Success: true,
		Output:  output.String(),
		Extra: map[string]interface{}{
			"tool":             "git_commit_msg",
			"type":             suggestion.Type,
			"scope":            suggestion.Scope,
			"subject":          suggestion.Subject,
			"body":             suggestion.Body,
			"full_message":     suggestion.FullMessage,
			"confidence":       suggestion.Confidence,
			"convention":       convention,
			"changedFiles":     changedFiles,
			"linesAdded":       changedLinesAdded,
			"linesDeleted":     changedLinesDeleted,
		},
	}
}

// getStagedDiff retrieves the staged git diff.
func (te *ToolExecutor) getStagedDiff(maxLines int) (string, error) {
	// First check if there are staged changes
	checkCmd := exec.Command("git", "diff", "--cached", "--name-only")
	checkOutput, err := checkCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git check failed: %s", string(checkOutput))
	}

	stagedFiles := strings.TrimSpace(string(checkOutput))
	if stagedFiles == "" {
		return "", fmt.Errorf("no staged changes")
	}

	// Get the diff
	diffCmd := exec.Command("git", "diff", "--cached", "-U3")
	diffOutput, err := diffCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %s", string(diffOutput))
	}

	diffStr := string(diffOutput)

	// Truncate if too long
	if maxLines > 0 {
		lines := strings.Split(diffStr, "\n")
		if len(lines) > maxLines {
			diffStr = strings.Join(lines[:maxLines], "\n") + "\n... [output truncated, " + strconv.Itoa(len(lines)-maxLines) + " more lines]"
		}
	}

	return diffStr, nil
}

// analyzeDiffAndGenerate analyzes a git diff and generates a conventional commit message.
func (te *ToolExecutor) analyzeDiffAndGenerate(diff string, convention string) commitSuggestion {
	// Analyze the diff to determine commit type and scope
	typeAnalysis := te.analyzeDiffType(diff)
	scopeAnalysis := te.analyzeDiffScope(diff)
	subjectAnalysis := te.generateSubject(diff, typeAnalysis, scopeAnalysis)

	// Build the full message based on convention
	var fullMessage string
	switch convention {
	case "conventional", "angular":
		scopeStr := ""
		if scopeAnalysis != "" {
			scopeStr = fmt.Sprintf("(%s): ", scopeAnalysis)
		}
		fullMessage = fmt.Sprintf("%s%s%s", typeAnalysis, scopeStr, subjectAnalysis)
	case "simple":
		fullMessage = subjectAnalysis
	default:
		fullMessage = fmt.Sprintf("%s%s", typeAnalysis, subjectAnalysis)
	}

	// Add body if there are multiple areas affected
	body := te.generateBody(diff, typeAnalysis, scopeAnalysis)
	if body != "" {
		fullMessage += "\n\n" + body
	}

	confidence := te.calculateConfidence(diff, typeAnalysis, subjectAnalysis)

	return commitSuggestion{
		Type:        typeAnalysis,
		Scope:       scopeAnalysis,
		Subject:     subjectAnalysis,
		Body:        body,
		FullMessage: fullMessage,
		Confidence:  confidence,
	}
}

// analyzeDiffType determines the commit type from the diff.
func (te *ToolExecutor) analyzeDiffType(diff string) string {
	// Count changes by type
	newFileCount := strings.Count(diff, "new file mode")
	testChanges := strings.Count(diff, "test/")
	testChanges += strings.Count(diff, "tests/")
	testChanges += strings.Count(diff, "_test.go")
	testChanges += strings.Count(diff, ".test.")
	testChanges += strings.Count(diff, ".spec.")

	// Check for specific patterns
	hasDocsChanges := te.hasPatternInDiff(diff, []string{
		"README", "CHANGELOG", "CONTRIBUTING",
		".md", ".rst", ".txt",
	})

	hasConfigChanges := te.hasPatternInDiff(diff, []string{
		".env", ".config", ".toml", ".yaml", ".yml",
		"package.json", "go.mod", "Cargo.toml",
		"Dockerfile", "Makefile", "justfile",
	})

	hasBuildChanges := te.hasPatternInDiff(diff, []string{
		"Makefile", "justfile", "Dockerfile",
		".github/", ".gitlab-ci.yml",
		"ci/", "cd/",
	})

	hasRevert := strings.Contains(diff, "Revert")

	// Determine type based on analysis
	if hasRevert {
		return "revert"
	}

	if hasBuildChanges {
		return "ci"
	}

	if hasConfigChanges {
		return "chore"
	}

	if testChanges > 0 {
		// Check if ONLY test files changed
		nonTestChanges := strings.Count(diff, "diff --git") - testChanges
		if nonTestChanges <= 0 {
			return "test"
		}
	}

	if hasDocsChanges && testChanges == 0 {
		return "docs"
	}

	if newFileCount > 0 && strings.Contains(diff, "index 0000000") {
		// New files - likely a feature
		return "feat"
	}

	// Default: analyze content for bug fixes vs features
	if te.detectBugFixPattern(diff) {
		return "fix"
	}

	if te.detectRefactoringPattern(diff) {
		return "refactor"
	}

	return "feat"
}

// hasPatternInDiff checks if any of the given patterns appear in file paths of the diff.
func (te *ToolExecutor) hasPatternInDiff(diff string, patterns []string) bool {
	inDiffHeader := false
	diffLines := strings.Split(diff, "\n")

	for _, line := range diffLines {
		// Track if we're in a diff header
		if strings.HasPrefix(line, "diff --git") {
			inDiffHeader = true
			continue
		}

		if inDiffHeader {
			// Check if this line contains a file path (a/... or b/...)
			if (strings.HasPrefix(line, "a/") || strings.HasPrefix(line, "b/")) && len(line) > 3 {
				// Get the actual path (without a/ or b/ prefix)
				path := line[2:]
				for _, pattern := range patterns {
					if strings.Contains(path, pattern) {
						return true
					}
				}
			}
			// End of header
			if strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "--- ") {
				inDiffHeader = false
			}
		}
	}

	return false
}

// detectBugFixPattern looks for bug fix indicators in the diff.
func (te *ToolExecutor) detectBugFixPattern(diff string) bool {
	patterns := []string{
		"fix", "bug", "issue", "error", "panic", "crash",
		"segfault", "race condition", "leak", "memory leak",
		"null", "nil", "undefined",
	}

	for _, pattern := range patterns {
		if strings.Contains(strings.ToLower(diff), pattern) {
			return true
		}
	}

	return false
}

// detectRefactoringPattern looks for refactoring indicators in the diff.
func (te *ToolExecutor) detectRefactoringPattern(diff string) bool {
	// Check for large reorganizations without new functionality
	patterns := []string{
		"rename", "reorganize", "restructure", "cleanup",
		"extract", "extract_", "reformat",
	}

	for _, pattern := range patterns {
		if strings.Contains(strings.ToLower(diff), pattern) {
			return true
		}
	}

	return false
}

// analyzeDiffScope extracts the scope (subsystem/component) from the diff.
func (te *ToolExecutor) analyzeDiffScope(diff string) string {
	// Extract unique directory/file scopes from the diff
	scopeCounts := make(map[string]int)

	inDiffHeader := false
	diffLines := strings.Split(diff, "\n")

	for _, line := range diffLines {
		if strings.HasPrefix(line, "diff --git") {
			inDiffHeader = true
			continue
		}

		if inDiffHeader {
			if (strings.HasPrefix(line, "a/") || strings.HasPrefix(line, "b/")) && len(line) > 3 {
				path := line[2:]
				// Extract the top-level directory or file name
				parts := strings.Split(path, "/")
				if len(parts) > 1 {
					scope := parts[0]
					// Skip common non-scope directories
					if scope != "vendor" && scope != "node_modules" && scope != ".git" &&
						scope != "third_party" && scope != "extern" && scope != "pkg" {
						scopeCounts[scope]++
					}
				} else {
					// Single file in root
					ext := ""
					if idx := strings.LastIndex(parts[0], "."); idx > 0 {
						ext = parts[0][idx:]
					}
					if ext == "" || ext == ".go" || ext == ".py" || ext == ".js" || ext == ".ts" ||
						ext == ".rs" || ext == ".java" || ext == ".c" || ext == ".cpp" || ext == ".h" {
						scopeCounts[parts[0]]++
					}
				}
			}
			if strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "--- ") {
				inDiffHeader = false
			}
		}
	}

	if len(scopeCounts) == 0 {
		return ""
	}

	// Find the most common scope
	maxCount := 0
	mostCommonScope := ""
	for scope, count := range scopeCounts {
		if count > maxCount {
			maxCount = count
			mostCommonScope = scope
		}
	}

	// Only return scope if it accounts for more than 50% of changes
	if float64(maxCount)/float64(len(strings.Split(diff, "diff --git"))) > 0.5 {
		return mostCommonScope
	}

	return ""
}

// generateSubject creates a short subject line from the diff.
func (te *ToolExecutor) generateSubject(diff, commitType string, scope string) string {
	// Analyze the diff content to generate a meaningful subject
	// Extract meaningful change descriptions from the diff

	// Look for the most significant changes (added lines that are meaningful)
	changes := te.extractMeaningfulChanges(strings.Split(strings.TrimSpace(diff), "\n"))

	if len(changes) == 0 {
		// Fallback: generate a generic message
		return fmt.Sprintf("%s changes", commitType)
	}

	// Combine top changes into a subject line
	subject := te.buildSubjectFromChanges(changes, commitType, scope)

	// Ensure subject is concise (under 72 chars for conventional commits)
	if len(subject) > 72 {
		subject = subject[:69] + "..."
	}

	return subject
}

// extractMeaningfulChanges extracts meaningful change descriptions from diff lines.
func (te *ToolExecutor) extractMeaningfulChanges(lines []string) []string {
	var changes []string
	addedLines := 0
	skippedLines := 0

	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			content := strings.TrimPrefix(line, "+")
			content = strings.TrimSpace(content)

			// Skip empty lines, comments-only changes, and boilerplate
			if content == "" {
				skippedLines++
				continue
			}
			if strings.HasPrefix(content, "//") || strings.HasPrefix(content, "#") ||
				strings.HasPrefix(content, "/*") || strings.HasPrefix(content, "*") {
				skippedLines++
				continue
			}
			// Skip pure whitespace
			if strings.TrimSpace(content) == "" {
				skippedLines++
				continue
			}

			addedLines++
			changes = append(changes, content)

			// Only consider the first 20 meaningful changes
			if addedLines >= 20 {
				break
			}
		}

		// Stop after we've seen enough
		if addedLines >= 30 && skippedLines > 20 {
			break
		}
	}

	return changes
}

// buildSubjectFromChanges constructs a subject line from extracted changes.
func (te *ToolExecutor) buildSubjectFromChanges(changes []string, commitType string, scope string) string {
	if len(changes) == 0 {
		return fmt.Sprintf("%s changes", commitType)
	}

	// Analyze the changes to find the primary action
	actionVerbs := te.extractActionVerbs(changes)
	nouns := te.extractNouns(changes)

	// Build a descriptive subject
	subject := te.describeChanges(actionVerbs, nouns, changes)

	if subject == "" {
		return fmt.Sprintf("%s changes", commitType)
	}

	// Ensure the subject starts with a verb
	if !strings.Contains(subject, " ") || strings.HasPrefix(subject, "Add ") ||
		strings.HasPrefix(subject, "Update ") || strings.HasPrefix(subject, "Fix ") ||
		strings.HasPrefix(subject, "Remove ") || strings.HasPrefix(subject, "Add ") ||
		strings.HasPrefix(subject, "Implement ") || strings.HasPrefix(subject, "Add ") {
		// Good - starts with a verb
	} else {
		// Try to add a verb
		subject = "Update " + subject
	}

	// Clean up
	subject = strings.ReplaceAll(subject, "  ", " ")
	subject = strings.TrimSpace(subject)

	return subject
}

// extractActionVerbs finds action verbs in the changes.
func (te *ToolExecutor) extractActionVerbs(changes []string) []string {
	verbPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\b(add|added|added|create|created|implement|implemented|remove|removed|delete|deleted|update|updated|fix|fixed|change|changed|add|remove)\b`),
		regexp.MustCompile(`\b(parse|handle|process|validate|check|check|read|write|send|receive|load|save|connect|disconnect|initialize|setup)\b`),
	}

	verbCount := make(map[string]int)
	for _, change := range changes {
		lower := strings.ToLower(change)
		for _, pattern := range verbPatterns {
			matches := pattern.FindAllString(lower, -1)
			for _, match := range matches {
				match = strings.ToLower(match)
				verbCount[match]++
			}
		}
	}

	// Return top verbs sorted by count
	// Sort verbs by count (most common first)
	type verbStat struct {
		Verb  string
		Count int
	}
	var verbs []verbStat
	for verb, count := range verbCount {
		verbs = append(verbs, verbStat{verb, count})
	}
	for i := 0; i < len(verbs); i++ {
		for j := i + 1; j < len(verbs); j++ {
			if verbs[j].Count > verbs[i].Count {
				verbs[i], verbs[j] = verbs[j], verbs[i]
			}
		}
	}

	// Return the most common verb
	if len(verbs) > 0 {
		return []string{verbs[0].Verb}
	}
	return []string{}
}

// extractNouns finds key nouns/entities in the changes.
func (te *ToolExecutor) extractNouns(changes []string) []string {
	var nouns []string
	for _, change := range changes {
		// Look for identifiers (camelCase, UPPER_CASE, etc.)
		identifiers := regexp.MustCompile(`[A-Z][a-zA-Z0-9]*`).FindAllString(change, -1)
		for _, ident := range identifiers {
			if len(ident) > 2 && !isCommonWord(ident) {
				nouns = append(nouns, ident)
			}
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, noun := range nouns {
		if !seen[noun] {
			seen[noun] = true
			unique = append(unique, noun)
		}
		if len(unique) >= 3 {
			break
		}
	}
	return unique
}

// isCommonWord checks if a word is too common to be meaningful.
func isCommonWord(word string) bool {
	common := []string{"the", "and", "for", "with", "from", "this", "that", "these", "those",
		"which", "their", "there", "about", "into", "over", "after", "before",
		"between", "through", "during", "without", "within", "along", "across"}
	for _, c := range common {
		if strings.ToLower(word) == c {
			return true
		}
	}
	return false
}

// describeChanges creates a human-readable description of the changes.
func (te *ToolExecutor) describeChanges(actionVerbs []string, nouns []string, changes []string) string {
	if len(actionVerbs) == 0 && len(nouns) == 0 {
		return ""
	}

	var parts []string
	if len(actionVerbs) > 0 {
		parts = append(parts, actionVerbs[0])
	}
	if len(nouns) > 0 {
		parts = append(parts, nouns...)
	}

	if len(parts) == 0 {
		return ""
	}

	// Format: "add User model" or "fix handleRequest logic"
	subject := strings.Join(parts, " ")
	return subject
}

// generateBody creates a detailed body for the commit message.
func (te *ToolExecutor) generateBody(diff, commitType, scope string) string {
	// Count changed files and lines
	changedFiles := strings.Count(diff, "diff --git")
	added := 0
	deleted := 0

	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			deleted++
		}
	}

	var body strings.Builder
	body.WriteString(fmt.Sprintf("%d file(s) changed, %d line(s) added, %d line(s) deleted.\n\n",
		changedFiles, added, deleted))

	// List files changed
	inDiffHeader := false
	files := []string{}
	diffLines := strings.Split(diff, "\n")

	for _, line := range diffLines {
		if strings.HasPrefix(line, "diff --git") {
			inDiffHeader = true
			continue
		}

		if inDiffHeader {
			if (strings.HasPrefix(line, "a/") || strings.HasPrefix(line, "b/")) && len(line) > 3 {
				path := line[2:]
				if !containsInSlice(files, path) {
					files = append(files, path)
				}
			}
			if strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "--- ") {
				inDiffHeader = false
			}
		}
	}

	if len(files) > 0 {
		body.WriteString("Files changed:\n")
		for _, f := range files {
			body.WriteString(fmt.Sprintf("  - %s\n", f))
		}
	}

	if body.Len() == 0 {
		return ""
	}

	return body.String()
}

// containsInSlice checks if a slice contains a string.
func containsInSlice(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// calculateConfidence estimates how confident we are in the generated message.
func (te *ToolExecutor) calculateConfidence(diff, commitType, subject string) float64 {
	confidence := 0.5 // Base confidence

	// Higher confidence if diff is clear and substantial
	diffLines := strings.Split(diff, "\n")
	addedLines := 0
	for _, line := range diffLines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			addedLines++
		}
	}

	// Confidence based on amount of changes
	if addedLines > 0 {
		confidence += 0.1
	}
	if addedLines > 5 {
		confidence += 0.1
	}
	if addedLines > 20 {
		confidence += 0.1
	}

	// Higher confidence if subject is descriptive
	if len(subject) > 10 && len(subject) < 72 {
		confidence += 0.1
	}

	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}
