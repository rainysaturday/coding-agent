// Package tools implements the code review tool for the coding agent.
package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ReviewSeverity represents the severity level of a review finding.
type ReviewSeverity string

const (
	SeverityInfo     ReviewSeverity = "info"
	SeverityWarning  ReviewSeverity = "warning"
	SeverityError    ReviewSeverity = "error"
	SeverityCritical ReviewSeverity = "critical"
)

// Severity string constants for use in CodeReviewRule (plain string type)
const (
	SeverityStrInfo     = "info"
	SeverityStrWarning  = "warning"
	SeverityStrError    = "error"
	SeverityStrCritical = "critical"
)

// ReviewFinding represents a single finding from a code review.
type ReviewFinding struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Severity   string `json:"severity"`
	Category   string `json:"category"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

// ReviewResult represents the complete result of a code review.
type ReviewResult struct {
	Findings     []ReviewFinding `json:"findings"`
	TotalFiles   int             `json:"total_files"`
	Summary      map[string]int  `json:"summary"` // severity -> count
}

// executeCodeReview reviews code files for issues, quality problems, and potential bugs.
func (te *ToolExecutor) executeCodeReview(params map[string]interface{}) *ToolResult {
	path, ok := params["path"].(string)
	if !ok || path == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	// Determine review scope
	filesParam, hasFiles := params["files"]
	var targetFiles []string
	if hasFiles {
		switch v := filesParam.(type) {
		case []interface{}:
			targetFiles = make([]string, len(v))
			for i, f := range v {
				targetFiles[i] = fmt.Sprintf("%v", f)
			}
		case string:
			if strings.Contains(v, ",") {
				for _, f := range strings.Split(v, ",") {
					targetFiles = append(targetFiles, strings.TrimSpace(f))
				}
			} else {
				targetFiles = []string{v}
			}
		}
	}

	// Determine languages to check
	languagesParam, hasLanguages := params["languages"]
	var targetLangs []string
	if hasLanguages {
		switch v := languagesParam.(type) {
		case []interface{}:
			targetLangs = make([]string, len(v))
			for i, l := range v {
				targetLangs[i] = fmt.Sprintf("%v", l)
			}
		case string:
			if strings.Contains(v, ",") {
				for _, l := range strings.Split(v, ",") {
					targetLangs = append(targetLangs, strings.TrimSpace(l))
				}
			} else {
				targetLangs = []string{v}
			}
		}
	}

	// Default languages if none specified
	if len(targetLangs) == 0 {
		targetLangs = []string{"go", "python", "javascript", "typescript", "rust", "java", "c", "cpp", "csharp", "ruby", "php", "swift", "kotlin"}
	}

	// Max findings to return
	maxFindings := 100
	if mf, ok := params["max_findings"].(float64); ok {
		maxFindings = int(mf)
	} else if mf, ok := params["max_findings"].(int); ok {
		maxFindings = mf
	}

	// Check for specific rule filters
	rulesParam, hasRules := params["rules"]
	var filterRules []string
	if hasRules {
		switch v := rulesParam.(type) {
		case []interface{}:
			filterRules = make([]string, len(v))
			for i, r := range v {
				filterRules[i] = fmt.Sprintf("%v", r)
			}
		case string:
			if strings.Contains(v, ",") {
				for _, r := range strings.Split(v, ",") {
					filterRules = append(filterRules, strings.TrimSpace(r))
				}
			} else {
				filterRules = []string{v}
			}
		}
	}

	// Determine files to review
	if len(targetFiles) == 0 {
		// If path is a directory, find all source files
		info, err := os.Stat(path)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("path not found: %v", err),
			}
		}

		if info.IsDir() {
			// Walk directory to find source files
			var allFiles []string
			fileExtMap := getFileExtensionMap(targetLangs)
			err := filepath.WalkDir(path, func(walkPath string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					// Skip common non-source directories
					base := d.Name()
					if base == ".git" || base == "node_modules" || base == "vendor" || base == ".venv" || base == "__pycache__" {
						return filepath.SkipDir
					}
					return nil
				}
				if _, ok := fileExtMap[filepath.Ext(walkPath)]; ok {
					allFiles = append(allFiles, walkPath)
				}
				return nil
			})
			if err != nil {
				return &ToolResult{
					Success: false,
					Error:   fmt.Sprintf("failed to walk directory: %v", err),
				}
			}
			targetFiles = allFiles
		} else {
			// Path is a file
			targetFiles = []string{path}
		}
	}

	// Filter files by extension based on languages
	if len(targetLangs) > 0 {
		fileExtMap := getFileExtensionMap(targetLangs)
		var filtered []string
		for _, f := range targetFiles {
			if _, ok := fileExtMap[filepath.Ext(f)]; ok {
				filtered = append(filtered, f)
			}
		}
		targetFiles = filtered
	}

	if len(targetFiles) == 0 {
		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("No source files found for review in: %s", path),
			Extra: map[string]interface{}{
				"filesSearched": 0,
				"findingsFound": 0,
			},
		}
	}

	// Perform code review on each file
	var allFindings []ReviewFinding
	var filesSearched int

	for _, filePath := range targetFiles {
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue // Skip unreadable files
		}

		filesSearched++
		lines := strings.Split(string(content), "\n")
		// Remove trailing empty line from file with newline
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}

		fileFindings := reviewFile(filePath, lines, filterRules)
		allFindings = append(allFindings, fileFindings...)

		if len(allFindings) >= maxFindings {
			break
		}
	}

	// Sort findings by severity (critical first, then error, warning, info)
	sort.Slice(allFindings, func(i, j int) bool {
		severityOrder := map[string]int{
			"critical": 0,
			"error":    1,
			"warning":  2,
			"info":     3,
		}
		return severityOrder[allFindings[i].Severity] < severityOrder[allFindings[j].Severity]
	})

	// Build summary
	summary := map[string]int{
		"info":     0,
		"warning":  0,
		"error":    0,
		"critical": 0,
	}
	for _, f := range allFindings {
		summary[f.Severity]++
	}

	// Format output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Code review completed for %d file(s).\n\n", filesSearched))
	output.WriteString(fmt.Sprintf("Summary: %d critical, %d errors, %d warnings, %d info\n\n",
		summary["critical"], summary["error"], summary["warning"], summary["info"]))

	if len(allFindings) == 0 {
		output.WriteString("No issues found. Code looks good!\n")
	} else {
		// Limit output to max findings
		if len(allFindings) > maxFindings {
			output.WriteString(fmt.Sprintf("Showing %d of %d findings:\n\n", maxFindings, len(allFindings)))
			allFindings = allFindings[:maxFindings]
		} else {
			output.WriteString(fmt.Sprintf("Findings (%d):\n\n", len(allFindings)))
		}

		for i, f := range allFindings {
			severityLabel := strings.ToUpper(string(f.Severity))
			output.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, severityLabel, f.Message))
			if f.File != "" {
				output.WriteString(fmt.Sprintf("   File: %s", f.File))
				if f.Line > 0 {
					output.WriteString(fmt.Sprintf(":%d", f.Line))
				}
				output.WriteString("\n")
			}
			if f.Category != "" {
				output.WriteString(fmt.Sprintf("   Category: %s\n", f.Category))
			}
			if f.Suggestion != "" {
				output.WriteString(fmt.Sprintf("   Suggestion: %s\n", f.Suggestion))
			}
			output.WriteString("\n")
		}
	}

	result := &ToolResult{
		Success: true,
		Output:  output.String(),
		Extra: map[string]interface{}{
			"filesSearched": filesSearched,
			"findingsFound": len(allFindings),
			"summary":       summary,
		},
	}

	// Also provide structured JSON for programmatic use
	jsonData, _ := json.MarshalIndent(ReviewResult{
		Findings:  allFindings,
		TotalFiles: filesSearched,
		Summary:  summary,
	}, "", "  ")

	if result.Extra == nil {
		result.Extra = make(map[string]interface{})
	}
	result.Extra["json"] = string(jsonData)

	return result
}

// reviewFile performs a code review on a single file.
func reviewFile(filePath string, lines []string, filterRules []string) []ReviewFinding {
	var findings []ReviewFinding
	lang := getLanguageFromPath(filePath)

	// Apply language-specific and general rules
	generalRules := getGeneralRules()
	languageRules := getLanguageRules(lang)

	allRules := append(generalRules, languageRules...)

	// Apply rule filters if specified
	if len(filterRules) > 0 {
		var filtered []CodeReviewRule
		for _, rule := range allRules {
			for _, f := range filterRules {
				if f == rule.Category || f == rule.Name {
					filtered = append(filtered, rule)
					break
				}
			}
		}
		allRules = filtered
	}

	for _, rule := range allRules {
		if rule.Match != nil {
			matches := rule.Match(lines)
			for _, match := range matches {
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Line:       match.Line,
					Severity:   rule.Severity,
					Category:   rule.Category,
					Message:    rule.Description,
					Suggestion: rule.Suggestion,
				})
			}
		}
	}

	return findings
}

// CodeReviewRule defines a single review rule.
type CodeReviewRule struct {
	Name        string
	Category    string
	Severity    string
	Description string
	Suggestion  string
	Match       func(lines []string) []ReviewMatch
}

// ReviewMatch represents a match for a review rule.
type ReviewMatch struct {
	Line int
}

// getGeneralRules returns general code review rules applicable to all languages.
func getGeneralRules() []CodeReviewRule {
	return []CodeReviewRule{
		{
			Name:        "long-line",
			Category:    "style",
			Severity:    SeverityStrWarning,
			Description: "Line exceeds recommended length (120 characters)",
			Suggestion:  "Consider breaking the line into multiple shorter lines",
			Match: func(lines []string) []ReviewMatch {
				var matches []ReviewMatch
				for i, line := range lines {
					if len(line) > 120 && !strings.HasPrefix(strings.TrimSpace(line), "//") {
						matches = append(matches, ReviewMatch{Line: i + 1})
					}
				}
				return matches
			},
		},
		{
			Name:        "trailing-whitespace",
			Category:    "style",
			Severity:    SeverityStrInfo,
			Description: "Trailing whitespace detected",
			Suggestion:  "Remove trailing whitespace to keep diffs clean",
			Match: func(lines []string) []ReviewMatch {
				var matches []ReviewMatch
				for i, line := range lines {
					if len(line) > 0 && line != strings.TrimRight(line, " \t") {
						matches = append(matches, ReviewMatch{Line: i + 1})
					}
				}
				return matches
			},
		},
		{
			Name:        "empty-file",
			Category:    "quality",
			Severity:    SeverityStrInfo,
			Description: "File is empty",
			Suggestion:  "Consider removing empty files or adding content",
			Match: func(lines []string) []ReviewMatch {
				nonEmpty := 0
				for _, line := range lines {
					if strings.TrimSpace(line) != "" {
						nonEmpty++
					}
				}
				if nonEmpty == 0 {
					return []ReviewMatch{{Line: 0}}
				}
				return nil
			},
		},
		{
			Name:        "magic-number",
			Category:    "quality",
			Severity:    SeverityStrWarning,
			Description: "Magic number detected (unexplained numeric literal)",
			Suggestion:  "Consider using a named constant or variable instead",
			Match: func(lines []string) []ReviewMatch {
				magicNumberRe := regexp.MustCompile(`\b(?i)(?:if|while|for|switch|case|return|>=|<=|>|<|==|!=)\s*.*?\b(\d{2,})\b`)
				var matches []ReviewMatch
				for i, line := range lines {
					trimmed := strings.TrimSpace(line)
					// Skip comments and pure numeric lines
					if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "*") {
						continue
					}
					if magicNumberRe.MatchString(trimmed) {
						// Skip common non-magic numbers (like timeouts, small counts)
						nums := magicNumberRe.FindAllStringSubmatch(trimmed, -1)
						for _, numsGroup := range nums {
							if len(numsGroup) > 1 {
								num, err := strconv.Atoi(numsGroup[1])
								if err == nil && num >= 10 && num != 10 && num != 100 && num != 1000 && num != 10000 {
									matches = append(matches, ReviewMatch{Line: i + 1})
									break // One match per line
								}
							}
						}
					}
				}
				return matches
			},
		},
		{
			Name:        "hardcoded-credentials",
			Category:    "security",
			Severity:    SeverityStrCritical,
			Description: "Potential hardcoded credentials or secrets detected",
			Suggestion:  "Use environment variables or a secrets manager instead of hardcoding credentials",
			Match: func(lines []string) []ReviewMatch {
				credPattern := regexp.MustCompile(`(?i)(password|passwd|pwd|secret|api[_-]?key|access[_-]?key|token|private[_-]?key|auth[_-]?token)\s*[:=]\s*["'][^"']+["']`)
				var matches []ReviewMatch
				for i, line := range lines {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "*") {
						continue
					}
					if credPattern.MatchString(trimmed) {
						matches = append(matches, ReviewMatch{Line: i + 1})
					}
				}
				return matches
			},
		},
		{
			Name:        "todo-comment",
			Category:    "maintenance",
			Severity:    SeverityStrInfo,
			Description: "TODO/FIXME/HACK comment detected",
			Suggestion:  "Consider addressing the TODO/FIXME before merging",
			Match: func(lines []string) []ReviewMatch {
				todoRe := regexp.MustCompile(`(?i)\b(TODO|FIXME|HACK|XXX|WORKAROUND|TEMP|TEMPORARY)\b`)
				var matches []ReviewMatch
				for i, line := range lines {
					if todoRe.MatchString(line) {
						matches = append(matches, ReviewMatch{Line: i + 1})
					}
				}
				return matches
			},
		},
		{
			Name:        "unused-import",
			Category:    "quality",
			Severity:    SeverityStrWarning,
			Description: "Potential unused import detected",
			Suggestion:  "Remove unused imports to keep the code clean",
			Match: func(lines []string) []ReviewMatch {
				// This is a simplified check - full analysis requires parsing
				var matches []ReviewMatch
				// Look for import blocks in various languages
				inImportBlock := false
				var importedNames []string
				for _, line := range lines {
					trimmed := strings.TrimSpace(line)
					// Go imports
					if trimmed == "import (" {
						inImportBlock = true
						continue
					}
					if inImportBlock && trimmed == ")" {
						inImportBlock = false
						// Check if any of the imported packages are used
						// Skip for now - too complex without AST
						continue
					}
					if inImportBlock {
						// Extract package name from import line
						parts := strings.Fields(trimmed)
						if len(parts) >= 1 {
							// Could be "pkg" or "alias \"pkg\""
							if strings.Contains(parts[0], "\"") {
								// alias "package"
								name := strings.Fields(parts[0])[0]
								if len(name) > 1 && name[0] == '"' && name[len(name)-1] == '"' {
									name = name[1:]
								}
								importedNames = append(importedNames, name)
							} else if len(parts) >= 2 && strings.Contains(parts[1], "\"") {
								// "package" - extract package name
								pkg := strings.Trim(parts[1], "\"")
								lastSlash := strings.LastIndex(pkg, "/")
								if lastSlash >= 0 {
									importedNames = append(importedNames, pkg[lastSlash+1:])
								} else {
									importedNames = append(importedNames, pkg)
								}
							}
						}
					}
				}
				return matches // Simplified - skip for now to avoid false positives
			},
		},
		{
			Name:        "long-function",
			Category:    "quality",
			Severity:    SeverityStrWarning,
			Description: "Potentially long function detected (>50 lines)",
			Suggestion:  "Consider splitting into smaller, more focused functions",
			Match: func(lines []string) []ReviewMatch {
				return nil // Requires AST parsing, skip in basic mode
			},
		},
	}
}

// getLanguageRules returns language-specific review rules.
func getLanguageRules(lang string) []CodeReviewRule {
	lang = strings.ToLower(lang)

	switch lang {
	case "go":
		return getGoRules()
	case "python":
		return getPythonRules()
	case "javascript", "typescript":
		return getJSRules()
	case "rust":
		return getRustRules()
	case "java":
		return getJavaRules()
	default:
		return nil
	}
}

// getGoRules returns Go-specific review rules.
func getGoRules() []CodeReviewRule {
	return []CodeReviewRule{
		{
			Name:        "go-naked-return",
			Category:    "best-practices",
			Severity:    SeverityStrWarning,
			Description: "Naked return statement in multi-line function",
			Suggestion:  "Consider using named return values or explicit returns for clarity",
			Match: func(lines []string) []ReviewMatch {
				inMultiLineFn := false
				var matches []ReviewMatch
				for i, line := range lines {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "func ") {
						inMultiLineFn = false
					}
					if strings.Contains(trimmed, "{") {
						inMultiLineFn = true
					}
					if inMultiLineFn && strings.TrimSpace(line) == "return" {
						matches = append(matches, ReviewMatch{Line: i + 1})
					}
					if strings.TrimSpace(line) == "}" {
						inMultiLineFn = false
					}
				}
				return matches
			},
		},
		{
			Name:        "go-error-check",
			Category:    "best-practices",
			Severity:    SeverityStrWarning,
			Description: "Potential missing error check",
			Suggestion:  "Check the error return value instead of ignoring it",
			Match: func(lines []string) []ReviewMatch {
				errorIgnoreRe := regexp.MustCompile(`\b\w+\((.*?)\)\s*$`)
				var matches []ReviewMatch
				for i, line := range lines {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "//") {
						continue
					}
					// Check for function calls where the return is ignored
					if errorIgnoreRe.MatchString(trimmed) {
						// Check if this line is NOT part of an error check
						if i+1 < len(lines) {
							nextLine := strings.TrimSpace(lines[i+1])
							if strings.HasPrefix(nextLine, "if err") || strings.HasPrefix(nextLine, "if ok") {
								continue
							}
						}
						if strings.Contains(trimmed, " := ") && strings.Contains(trimmed, "(") {
							// Multi-return function call without error check
							parts := strings.Split(trimmed, ":=")
							if len(parts) == 2 {
								vars := strings.Split(parts[0], ",")
								hasErr := false
								for _, v := range vars {
									if strings.TrimSpace(v) == "err" {
										hasErr = true
										break
									}
								}
								// Check if next line checks the error
								checksError := false
								if i+1 < len(lines) {
									if strings.Contains(lines[i+1], "if err != nil") {
										checksError = true
									}
								}
								if !hasErr && !checksError {
									// Could be a multi-return call with ignored errors
									// Skip for now to avoid false positives
								}
							}
						}
					}
				}
				return matches
			},
		},
		{
			Name:        "go-interface-implementation",
			Category:    "best-practices",
			Severity:    SeverityStrInfo,
			Description: "Consider implementing error interface for custom error types",
			Suggestion:  "Add an Error() string method to implement the error interface",
			Match: func(lines []string) []ReviewMatch {
				var matches []ReviewMatch
				errTypeRe := regexp.MustCompile(`type\s+(\w*[Ee]rror)\s+struct`)
				for i, line := range lines {
					if errTypeRe.MatchString(line) {
						matches = append(matches, ReviewMatch{Line: i + 1})
					}
				}
				return matches
			},
		},
	}
}

// getPythonRules returns Python-specific review rules.
func getPythonRules() []CodeReviewRule {
	return []CodeReviewRule{
		{
			Name:        "python-global-import",
			Category:    "best-practices",
			Severity:    SeverityStrWarning,
			Description: "Wildcard import detected (from module import *)",
			Suggestion:  "Import specific names instead of using wildcard imports",
			Match: func(lines []string) []ReviewMatch {
				wildcardRe := regexp.MustCompile(`from\s+\w+\s+import\s+\*`)
				var matches []ReviewMatch
				for i, line := range lines {
					if wildcardRe.MatchString(line) {
						matches = append(matches, ReviewMatch{Line: i + 1})
					}
				}
				return matches
			},
		},
		{
			Name:        "python-bare-except",
			Category:    "best-practices",
			Severity:    SeverityStrError,
			Description: "Bare except clause catches all exceptions including SystemExit and KeyboardInterrupt",
			Suggestion:  "Use specific exception types instead of bare except",
			Match: func(lines []string) []ReviewMatch {
				bareExceptRe := regexp.MustCompile(`^\s*except\s*:`)
				var matches []ReviewMatch
				for i, line := range lines {
					if bareExceptRe.MatchString(line) {
						matches = append(matches, ReviewMatch{Line: i + 1})
					}
				}
				return matches
			},
		},
		{
			Name:        "python-literal-boolean",
			Category:    "best-practices",
			Severity:    SeverityStrWarning,
			Description: "Literal boolean used in conditional",
			Suggestion:  "Use True/False directly instead of comparing to boolean",
			Match: func(lines []string) []ReviewMatch {
				literalBoolRe := regexp.MustCompile(`\b(if|while)\s+\w+\s*(==|!=)\s*(True|False)\b`)
				var matches []ReviewMatch
				for i, line := range lines {
					if literalBoolRe.MatchString(line) {
						matches = append(matches, ReviewMatch{Line: i + 1})
					}
				}
				return matches
			},
		},
		{
			Name:        "python-unnecessary-lambda",
			Category:    "best-practices",
			Severity:    SeverityStrInfo,
			Description: "Lambda that could be a regular function",
			Suggestion:  "Consider using a named function instead of a lambda",
			Match: func(lines []string) []ReviewMatch {
				return nil // Complex to check without full AST
			},
		},
	}
}

// getJSRules returns JavaScript/TypeScript-specific review rules.
func getJSRules() []CodeReviewRule {
	return []CodeReviewRule{
		{
			Name:        "js-var-redeclaration",
			Category:    "best-practices",
			Severity:    SeverityStrWarning,
			Description: "Variable redeclaration detected",
			Suggestion:  "Use 'const' or 'let' consistently and avoid redeclaration",
			Match: func(lines []string) []ReviewMatch {
				var matches []ReviewMatch
				declared := make(map[string]int)
				for i, line := range lines {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "//") {
						continue
					}
					// Match var declarations
					varRe := regexp.MustCompile(`\bvar\s+(\w+)`)
					if m := varRe.FindStringSubmatch(trimmed); m != nil {
						name := m[1]
						if prev, exists := declared[name]; exists {
							if prev != i+1 {
								matches = append(matches, ReviewMatch{Line: i + 1})
							}
						}
						declared[name] = i + 1
					}
				}
				return matches
			},
		},
		{
			Name:        "js-console-log",
			Category:    "best-practices",
			Severity:    SeverityStrInfo,
			Description: "console.log statement detected (likely debugging code)",
			Suggestion:  "Remove console.log statements before committing",
			Match: func(lines []string) []ReviewMatch {
				consoleRe := regexp.MustCompile(`console\.(log|debug|info|warn|error)\(`)
				var matches []ReviewMatch
				for i, line := range lines {
					if consoleRe.MatchString(line) {
						matches = append(matches, ReviewMatch{Line: i + 1})
					}
				}
				return matches
			},
		},
		{
			Name:        "js-any-type",
			Category:    "best-practices",
			Severity:    SeverityStrWarning,
			Description: "'any' type used in TypeScript code",
			Suggestion:  "Consider using a more specific type instead of 'any'",
			Match: func(lines []string) []ReviewMatch {
				anyRe := regexp.MustCompile(`:\s*any\b|\bany\s*[<\[]`)
				var matches []ReviewMatch
				for i, line := range lines {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
						continue
					}
					if anyRe.MatchString(line) {
						matches = append(matches, ReviewMatch{Line: i + 1})
					}
				}
				return matches
			},
		},
		{
			Name:        "js-empty-block",
			Category:    "best-practices",
			Severity:    SeverityStrWarning,
			Description: "Empty block statement detected ({})",
			Suggestion:  "Add a comment or implement the block body",
			Match: func(lines []string) []ReviewMatch {
				var matches []ReviewMatch
				for i, line := range lines {
					trimmed := strings.TrimSpace(line)
					if trimmed == "{}" || trimmed == "{\n}" {
						matches = append(matches, ReviewMatch{Line: i + 1})
					}
				}
				return matches
			},
		},
	}
}

// getRustRules returns Rust-specific review rules.
func getRustRules() []CodeReviewRule {
	return []CodeReviewRule{
		{
			Name:        "rust-ignore-result",
			Category:    "best-practices",
			Severity:    SeverityStrWarning,
			Description: "Result/Option value is ignored (no error handling)",
			Suggestion:  "Handle the Result with unwrap/expect/? or use match",
			Match: func(lines []string) []ReviewMatch {
				// Skip this rule - requires full statement analysis
				return nil
			},
		},
		{
			Name:        "rust-unnecessary-lifetime",
			Category:    "best-practices",
			Severity:    SeverityStrInfo,
			Description: "Potentially unnecessary explicit lifetime",
			Suggestion:  "Consider using lifetime elision",
			Match: func(lines []string) []ReviewMatch {
				return nil // Too complex without AST
			},
		},
	}
}

// getJavaRules returns Java-specific review rules.
func getJavaRules() []CodeReviewRule {
	return []CodeReviewRule{
		{
			Name:        "java-print-stacktrace",
			Category:    "best-practices",
			Severity:    SeverityStrWarning,
			Description: "printStackTrace() call detected - prefer logging frameworks",
			Suggestion:  "Use a logging framework (SLF4J, Log4j) instead of printStackTrace()",
			Match: func(lines []string) []ReviewMatch {
				psRe := regexp.MustCompile(`\.printStackTrace\(\)`)
				var matches []ReviewMatch
				for i, line := range lines {
					if psRe.MatchString(line) {
						matches = append(matches, ReviewMatch{Line: i + 1})
					}
				}
				return matches
			},
		},
		{
			Name:        "java-generic-raw-type",
			Category:    "best-practices",
			Severity:    SeverityStrWarning,
			Description: "Raw type used for generic class",
			Suggestion:  "Specify type parameters (e.g., List<String> instead of List)",
			Match: func(lines []string) []ReviewMatch {
				return nil // Requires type analysis
			},
		},
	}
}

// getLanguageFromPath determines the programming language from a file path.
func getLanguageFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".py", ".pyi":
		return "python"
	case ".js", ".mjs":
		return "javascript"
	case ".ts", ".mts":
		return "typescript"
	case ".jsx":
		return "javascript"
	case ".tsx":
		return "typescript"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp", ".hh":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".kt", ".kts":
		return "kotlin"
	default:
		return "unknown"
	}
}

// getFileExtensionMap maps languages to their file extensions.
func getFileExtensionMap(languages []string) map[string]bool {
	extMap := map[string]bool{
		// Go
		".go": true,
		// Python
		".py": true, ".pyi": true,
		// JavaScript
		".js": true, ".mjs": true, ".jsx": true,
		// TypeScript
		".ts": true, ".mts": true, ".tsx": true,
		// Rust
		".rs": true,
		// Java
		".java": true,
		// C/C++
		".c": true, ".h": true, ".cpp": true, ".cc": true, ".cxx": true, ".hpp": true, ".hh": true,
		// C#
		".cs": true,
		// Ruby
		".rb": true,
		// PHP
		".php": true,
		// Swift
		".swift": true,
		// Kotlin
		".kt": true, ".kts": true,
	}

	if len(languages) == 0 {
		return extMap
	}

	languageToExt := map[string][]string{
		"go":         {".go"},
		"python":     {".py", ".pyi"},
		"javascript": {".js", ".mjs", ".jsx"},
		"typescript": {".ts", ".mts", ".tsx"},
		"js":         {".js", ".mjs", ".jsx"},
		"ts":         {".ts", ".mts", ".tsx"},
		"rust":       {".rs"},
		"java":       {".java"},
		"c":          {".c", ".h"},
		"cpp":        {".cpp", ".cc", ".cxx", ".hpp", ".hh"},
		"csharp":     {".cs"},
		"ruby":       {".rb"},
		"php":        {".php"},
		"swift":      {".swift"},
		"kotlin":     {".kt", ".kts"},
	}

	result := make(map[string]bool)
	for _, lang := range languages {
		lang = strings.ToLower(lang)
		if exts, ok := languageToExt[lang]; ok {
			for _, ext := range exts {
				result[ext] = true
			}
		}
	}
	return result
}

// formatCodeReviewOutput formats a code review result for display.
func formatCodeReviewOutput(result *ReviewResult) string {
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Code Review Results (%d files, %d findings)\n", result.TotalFiles, len(result.Findings)))
	output.WriteString(strings.Repeat("=", 60) + "\n\n")

	// Summary by severity
	if result.Summary != nil {
		output.WriteString("Summary:\n")
		for _, sev := range []string{"critical", "error", "warning", "info"} {
			if count, ok := result.Summary[sev]; ok && count > 0 {
				output.WriteString(fmt.Sprintf("  %s: %d\n", strings.ToUpper(sev), count))
			}
		}
		output.WriteString("\n")
	}

	// Findings grouped by file
	files := make(map[string][]ReviewFinding)
	for _, f := range result.Findings {
		files[f.File] = append(files[f.File], f)
	}

	for _, file := range getSortedFileNames(files) {
		findings := files[file]
		output.WriteString(fmt.Sprintf("\n%s:\n", file))
		for i, f := range findings {
			output.WriteString(fmt.Sprintf("  Line %d [%s] %s", f.Line, f.Severity, f.Message))
			if f.Suggestion != "" {
				output.WriteString(fmt.Sprintf(" -> %s", f.Suggestion))
			}
			output.WriteString("\n")
			_ = i
		}
	}

	return output.String()
}

// getSortedFileNames returns file names sorted alphabetically.
func getSortedFileNames(files map[string][]ReviewFinding) []string {
	var names []string
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// readLines reads lines from a file for review.
func readLines(filePath string) ([]string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	// Remove trailing empty line
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines, nil
}

// readLinesWithScanner reads lines from a file using a scanner.
func readLinesWithScanner(filePath string) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}
