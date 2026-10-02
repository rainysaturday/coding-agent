package tools

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// list_files
// ---------------------------------------------------------------------------

// ListFilesTool lists files and directories like a simple ls command.
type ListFilesTool struct{ run *RunContext }

// NewListFilesTool creates the list_files tool.
func NewListFilesTool(run *RunContext) *ListFilesTool { return &ListFilesTool{run: run} }

func (t *ListFilesTool) Name() string { return "list_files" }

func (t *ListFilesTool) Description() string {
	return "List files and directories in a path, similar to the ls command"
}

func (t *ListFilesTool) Schema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"path":  prop("string", "Path to the file or directory to list (defaults to current directory if not specified)"),
		"flags": arrayProp("string", "List of ls-style flags to control output (e.g., 'l' for long format, 'a' for all including hidden, 'h' for human-readable sizes, 't' for time-sorted, 'S' for size-sorted, 'R' for recursive)"),
	}, nil)
}

func (t *ListFilesTool) ReadOnly() bool { return true }

func hasFlag(flags []string, name string) bool {
	for _, f := range flags {
		f = strings.TrimLeft(f, "-")
		if f == name {
			return true
		}
		if len(name) == 1 && strings.Contains(f, name) {
			return true
		}
	}
	return false
}

func (t *ListFilesTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	pathArg := getString(args, "path")
	if pathArg == "" {
		pathArg = "."
	}
	flags := getStringSlice(args, "flags")
	long := hasFlag(flags, "l")
	all := hasFlag(flags, "a")
	human := hasFlag(flags, "h")
	byTime := hasFlag(flags, "t")
	bySize := hasFlag(flags, "S")
	recursive := hasFlag(flags, "R")

	abs := t.run.resolvePath(pathArg)
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return formatEntry(pathArg, info, long, human), nil
	}

	var b strings.Builder
	if err := t.listDir(&b, abs, pathArg, flags, long, all, human, byTime, bySize, recursive, 0); err != nil {
		return "", err
	}
	out := strings.TrimRight(b.String(), "\n")
	if out == "" {
		return "(empty directory)", nil
	}
	return truncateLines(out, t.run.MaxOutputLines), nil
}

func (t *ListFilesTool) listDir(b *strings.Builder, abs, display string, flags []string, long, all, human, byTime, bySize, recursive bool, depth int) error {
	entries, err := os.ReadDir(abs)
	if err != nil {
		return err
	}
	type entry struct {
		name string
		info fs.FileInfo
	}
	var list []entry
	for _, e := range entries {
		if !all && strings.HasPrefix(e.Name(), ".") {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		list = append(list, entry{name: e.Name(), info: fi})
	}
	switch {
	case byTime:
		sort.Slice(list, func(i, j int) bool { return list[i].info.ModTime().After(list[j].info.ModTime()) })
	case bySize:
		sort.Slice(list, func(i, j int) bool { return list[i].info.Size() > list[j].info.Size() })
	default:
		sort.Slice(list, func(i, j int) bool { return list[i].name < list[j].name })
	}
	if long {
		b.WriteString(fmt.Sprintf("total %d\n", len(list)))
	}
	for _, e := range list {
		b.WriteString(formatEntry(e.name, e.info, long, human) + "\n")
	}
	if recursive {
		for _, e := range list {
			if e.info.IsDir() {
				sub := filepath.Join(abs, e.name)
				subDisplay := filepath.Join(display, e.name)
				b.WriteString("\n" + subDisplay + ":\n")
				if err := t.listDir(b, sub, subDisplay, flags, long, all, human, byTime, bySize, recursive, depth+1); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func formatEntry(name string, info fs.FileInfo, long, human bool) string {
	if !long {
		if info.IsDir() {
			return name + "/"
		}
		return name
	}
	mode := info.Mode()
	size := fmt.Sprintf("%8d", info.Size())
	if human {
		size = fmt.Sprintf("%8s", humanSize(info.Size()))
	}
	suffix := ""
	if info.IsDir() {
		suffix = "/"
	}
	return fmt.Sprintf("%s %s %s %s %s%s", mode, "-", info.ModTime().Format("2006-01-02 15:04"), size, name, suffix)
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%c", float64(n)/float64(div), "KMGTPE"[exp])
}

// ---------------------------------------------------------------------------
// grep
// ---------------------------------------------------------------------------

// GrepTool searches file contents with a regular expression.
type GrepTool struct{ run *RunContext }

// NewGrepTool creates the grep tool.
func NewGrepTool(run *RunContext) *GrepTool { return &GrepTool{run: run} }

func (t *GrepTool) Name() string { return "grep" }

func (t *GrepTool) Description() string {
	return "Search through file contents using grep-like pattern matching"
}

func (t *GrepTool) Schema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"path":    prop("string", "Path to search (defaults to current directory if not specified)"),
		"pattern": prop("string", "Pattern to search for (supports regex)"),
		"flags":   arrayProp("string", "List of grep-style flags to control output (e.g., '-n' for line numbers, '-i' for case insensitive, '-r' for recursive, '-f' for pattern file, '-a' for all including hidden, '-c' for count, '-v' for invert match, '-l' for filenames only)"),
		"include": arrayProp("string", "Optional glob patterns filtering files by name or extension (e.g., '*.go', '*.md'); also accepted as 'include=<glob>' flags"),
	}, []string{"pattern"})
}

func (t *GrepTool) ReadOnly() bool { return true }

var skipDirs = map[string]bool{".git": true, ".hg": true, ".svn": true, "node_modules": true, "__pycache__": true, ".idea": true, ".vscode": true}

func (t *GrepTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	pattern := getString(args, "pattern")
	if pattern == "" {
		return "", errors.New("missing required parameter: pattern")
	}
	pathArg := getString(args, "path")
	if pathArg == "" {
		pathArg = "."
	}
	flags := getStringSlice(args, "flags")
	caseIns := hasFlag(flags, "i")
	showLines := hasFlag(flags, "n") || hasFlag(flags, "r")
	namesOnly := hasFlag(flags, "l")
	counts := hasFlag(flags, "c")
	invert := hasFlag(flags, "v")
	includeHidden := hasFlag(flags, "a")
	patternFile := hasFlag(flags, "f")
	allFiles := hasFlag(flags, "a")

	// File extension / name filtering (requirement 034): globs from the
	// include parameter plus include=<glob> style flags.
	var globs []string
	globs = append(globs, getStringSlice(args, "include")...)
	for _, f := range flags {
		f = strings.TrimLeft(f, "-")
		if strings.HasPrefix(f, "include=") {
			globs = append(globs, strings.TrimPrefix(f, "include="))
		}
	}
	matchFile := func(path string) bool {
		if len(globs) == 0 {
			return true
		}
		for _, g := range globs {
			if g == "" {
				continue
			}
			target := filepath.Base(path)
			check := path
			if strings.Contains(g, "/") {
				target = path
			} else {
				check = target
			}
			if ok, err := filepath.Match(g, check); err == nil && ok {
				return true
			}
		}
		return false
	}

	expr := pattern
	if patternFile {
		abs := t.run.resolvePath(pattern)
		data, err := os.ReadFile(abs)
		if err != nil {
			return "", fmt.Errorf("failed to read pattern file: %w", err)
		}
		var parts []string
		for _, l := range splitLines(string(data)) {
			if strings.TrimSpace(l) != "" {
				parts = append(parts, regexp.QuoteMeta(l))
			}
		}
		if len(parts) == 0 {
			return "", errors.New("pattern file is empty")
		}
		expr = strings.Join(parts, "|")
		pattern = strings.Join(strings.Split(pattern, "\n"), ", ")
	}
	re, err := regexp.Compile(expr)
	if caseIns {
		re, err = regexp.Compile("(?i)" + expr)
	}
	if err != nil {
		return "", fmt.Errorf("invalid pattern %q: %w", pattern, err)
	}

	root := t.run.resolvePath(pathArg)
	var b strings.Builder
	limit := t.run.MaxOutputLines
	matched := 0
	truncated := false

	walk := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && (skipDirs[d.Name()] || (!includeHidden && strings.HasPrefix(d.Name(), "."))) {
				return filepath.SkipDir
			}
			return nil
		}
		if !includeHidden && strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		if !matchFile(path) {
			return nil
		}
		if !allFiles {
			if bin, err := sniffBinary(path); err == nil && bin {
				return nil
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(t.run.CWD, path)
		if err != nil {
			rel = path
		}
		lines := splitLines(string(data))
		var hits []string
		fileMatches := 0
		for i, line := range lines {
			if re.MatchString(line) != invert {
				fileMatches++
				if namesOnly || counts {
					continue
				}
				if showLines {
					hits = append(hits, fmt.Sprintf("%s:%d:%s", rel, i+1, line))
				} else {
					hits = append(hits, fmt.Sprintf("%s:%s", rel, line))
				}
			}
		}
		if fileMatches == 0 {
			return nil
		}
		matched++
		if namesOnly {
			b.WriteString(rel + "\n")
		} else if counts {
			b.WriteString(fmt.Sprintf("%s:%d\n", rel, fileMatches))
		} else {
			for _, h := range hits {
				if countLines(b.String()) >= limit {
					truncated = true
					return filepath.SkipAll
				}
				b.WriteString(h + "\n")
			}
		}
		return nil
	}

	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		if err := filepath.WalkDir(root, walk); err != nil {
			return "", err
		}
	} else {
		if err := walk(root, dirEntryFromFileInfo{info}, nil); err != nil {
			return "", err
		}
	}

	out := strings.TrimRight(b.String(), "\n")
	if out == "" {
		return "(no matches)", nil
	}
	if truncated {
		out = fmt.Sprintf("[output truncated at %d matching lines]\n%s", limit, out)
	}
	return out, nil
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

type dirEntryFromFileInfo struct{ fi os.FileInfo }

func (d dirEntryFromFileInfo) Name() string               { return d.fi.Name() }
func (d dirEntryFromFileInfo) IsDir() bool                { return d.fi.IsDir() }
func (d dirEntryFromFileInfo) Type() fs.FileMode          { return d.fi.Mode().Type() }
func (d dirEntryFromFileInfo) Info() (fs.FileInfo, error) { return d.fi, nil }
