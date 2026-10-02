// Package skills implements the Agent Skills (SKILL.md) format: discovery,
// stdlib-only frontmatter parsing, the system prompt catalog and activation.
package skills

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/coding-agent/coding-agent/config"
	"github.com/coding-agent/coding-agent/debug"
)

// Skill is a discovered skill.
type Skill struct {
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Location      string            `json:"location"`
	BaseDir       string            `json:"base_dir"`
	Body          string            `json:"-"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	License       string            `json:"license,omitempty"`
	Compatibility string            `json:"compatibility,omitempty"`
	AllowedTools  string            `json:"allowed_tools,omitempty"`
	Scope         string            `json:"scope"` // "project" | "user" | "custom"
}

// Diagnostic records a non-fatal problem found while discovering skills.
type Diagnostic struct {
	Level   string `json:"level"` // warn | error
	Path    string `json:"path"`
	Message string `json:"message"`
}

// Manager holds the skill catalog.
type Manager struct {
	mu          sync.Mutex
	byName      map[string]*Skill
	order       []string
	activated   map[string]bool
	diagnostics []Diagnostic
	scanned     []string
	skipped     []string
}

const (
	maxDepth    = 6
	maxDirsSeen = 2000
)

var skipDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true, "node_modules": true,
	".venv": true, "venv": true, "__pycache__": true, ".idea": true,
	".vscode": true, "dist": true, "build": true, ".next": true, "target": true,
}

// Discover scans the configured locations and returns a catalog.
func Discover(cfg *config.Config, cwd string) *Manager {
	m := &Manager{byName: map[string]*Skill{}, activated: map[string]bool{}}
	if cfg == nil || !cfg.SkillsEnabled {
		return m
	}

	projectTrusted := isTrusted(cfg, cwd)
	if !projectTrusted {
		m.addDiagnostic(Diagnostic{Level: "warn", Path: cwd, Message: "project skills skipped: the working directory is not trusted (pass --trust " + cwd + " to enable them)"})
	}

	type candidate struct {
		dir   string
		scope string
	}
	var dirs []candidate

	// Project scope: cross-client convention first, then client native, then
	// the compatibility location.
	if projectTrusted {
		for _, d := range []string{".agents/skills", "skills", ".claude/skills"} {
			dirs = append(dirs, candidate{dir: filepath.Join(cwd, d), scope: "project"})
		}
	}

	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		for _, d := range []string{".agents/skills", ".skills", ".claude/skills"} {
			dirs = append(dirs, candidate{dir: filepath.Join(home, filepath.FromSlash(d)), scope: "user"})
		}
	}

	for _, d := range cfg.SkillsDirs {
		scope := "custom"
		if projectTrusted && sameOrUnder(d, cwd) {
			scope = "project"
		}
		dirs = append(dirs, candidate{dir: d, scope: scope})
	}

	for _, c := range dirs {
		m.scanned = append(m.scanned, c.dir)
		m.scanDir(c.dir, c.scope)
	}
	return m
}

func sameOrUnder(dir, root string) bool {
	dir = filepath.Clean(dir)
	root = filepath.Clean(root)
	if dir == root {
		return true
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

// isTrusted reports whether the working directory is trusted for project skills.
func isTrusted(cfg *config.Config, cwd string) bool {
	for _, t := range cfg.TrustDirs {
		if sameOrUnder(cwd, t) {
			return true
		}
	}
	for _, t := range loadPersistedTrust() {
		if sameOrUnder(cwd, t) {
			return true
		}
	}
	return false
}

// TrustFile returns the path of the persisted trust list.
func TrustFile() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".coding-agent-trusted"
	}
	return filepath.Join(home, ".coding-agent-trusted")
}

func loadPersistedTrust() []string {
	data, err := os.ReadFile(TrustFile())
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// Trust marks a directory as trusted and persists it.
func Trust(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	existing := loadPersistedTrust()
	for _, e := range existing {
		if e == abs {
			return nil
		}
	}
	existing = append(existing, abs)
	return os.WriteFile(TrustFile(), []byte(strings.Join(existing, "\n")+"\n"), 0o600)
}

// scanDir walks a skills root and registers every directory containing SKILL.md.
func (m *Manager) scanDir(root, scope string) {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return
	}
	seen := 0
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		if depth > maxDepth || seen >= maxDirsSeen {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		hasSkillMD := false
		for _, e := range entries {
			if !e.IsDir() && e.Name() == "SKILL.md" {
				hasSkillMD = true
				break
			}
		}
		if depth > 0 && hasSkillMD {
			m.register(filepath.Join(dir, "SKILL.md"), scope)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if skipDirs[e.Name()] || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			seen++
			if seen >= maxDirsSeen {
				return
			}
			walk(filepath.Join(dir, e.Name()), depth+1)
		}
	}
	// The scope root itself never counts as a skill.
	for _, e := range mustReadDir(root) {
		if !e.IsDir() || skipDirs[e.Name()] || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		seen++
		if seen >= maxDirsSeen {
			return
		}
		walk(filepath.Join(root, e.Name()), 1)
	}
}

func mustReadDir(dir string) []os.DirEntry {
	entries, _ := os.ReadDir(dir)
	return entries
}

// register parses and stores a skill.
func (m *Manager) register(location, scope string) {
	meta, body, err := loadSkillMD(location)
	if err != nil {
		m.addDiagnostic(Diagnostic{Level: "error", Path: location, Message: err.Error()})
		if dbg := debug.Default(); dbg != nil {
			dbg.Logf("skill skipped: %s: %v", location, err)
		}
		return
	}
	dirName := filepath.Base(filepath.Dir(location))
	name := strings.TrimSpace(meta["name"])
	if name == "" {
		name = dirName
		m.addDiagnostic(Diagnostic{Level: "warn", Path: location, Message: "missing 'name' in frontmatter, using directory name"})
	}
	desc := strings.TrimSpace(meta["description"])
	if desc == "" {
		m.addDiagnostic(Diagnostic{Level: "error", Path: location, Message: "missing 'description' in frontmatter; skill skipped"})
		if dbg := debug.Default(); dbg != nil {
			dbg.Logf("skill skipped (no description): %s", location)
		}
		return
	}
	if name != dirName {
		m.addDiagnostic(Diagnostic{Level: "warn", Path: location, Message: fmt.Sprintf("skill name %q does not match parent directory %q", name, dirName)})
	}
	if len(name) > 64 {
		m.addDiagnostic(Diagnostic{Level: "warn", Path: location, Message: "skill name exceeds 64 characters"})
	}
	if len(desc) > 1024 {
		desc = desc[:1024]
		m.addDiagnostic(Diagnostic{Level: "warn", Path: location, Message: "skill description truncated to 1024 characters"})
	}

	sk := &Skill{
		Name:          name,
		Description:   desc,
		Location:      location,
		BaseDir:       filepath.Dir(location),
		Body:          body,
		Metadata:      meta,
		License:       meta["license"],
		Compatibility: meta["compatibility"],
		AllowedTools:  meta["allowed-tools"],
		Scope:         scope,
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if prev, ok := m.byName[name]; ok {
		// Project scope always wins; within a scope later scans win.
		if prev.Scope == "project" && sk.Scope != "project" {
			m.addDiagnosticLocked(Diagnostic{Level: "warn", Path: location, Message: fmt.Sprintf("skill %q shadowed by %s", name, prev.Location)})
			return
		}
		m.addDiagnosticLocked(Diagnostic{Level: "warn", Path: location, Message: fmt.Sprintf("skill %q from %s overrides %s", name, location, prev.Location)})
	} else {
		m.order = append(m.order, name)
	}
	m.byName[name] = sk
}

func (m *Manager) addDiagnostic(d Diagnostic) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addDiagnosticLocked(d)
}

// addDiagnosticLocked appends a diagnostic; the caller must hold m.mu.
func (m *Manager) addDiagnosticLocked(d Diagnostic) {
	m.diagnostics = append(m.diagnostics, d)
}

// List returns the catalog sorted by name.
func (m *Manager) List() []*Skill {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Skill, 0, len(m.order))
	for _, n := range m.order {
		out = append(out, m.byName[n])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Names returns the skill names.
func (m *Manager) Names() []string {
	out := []string{}
	for _, s := range m.List() {
		out = append(out, s.Name)
	}
	return out
}

// Count returns the number of discovered skills.
func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.byName)
}

// Diagnostics returns discovery diagnostics.
func (m *Manager) Diagnostics() []Diagnostic {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Diagnostic, len(m.diagnostics))
	copy(out, m.diagnostics)
	return out
}

// ScannedDirs returns the directories that were searched.
func (m *Manager) ScannedDirs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.scanned))
	copy(out, m.scanned)
	return out
}

// Catalog renders the AVAILABLE SKILLS section for the system prompt.
// It returns an empty string when no skills were discovered.
func (m *Manager) Catalog() string {
	skills := m.List()
	if len(skills) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("AVAILABLE SKILLS:\n")
	for _, s := range skills {
		b.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, s.Description))
		b.WriteString(fmt.Sprintf("  (SKILL.md: %s)\n", s.Location))
	}
	b.WriteString("\nThe following skills provide specialized instructions for specific tasks.\n")
	b.WriteString("When a task matches a skill's description, call the activate_skill tool with\n")
	b.WriteString("the skill's name to load its full instructions, or use read_file to load the\n")
	b.WriteString("SKILL.md at the listed location before proceeding.\n")
	b.WriteString("When a skill references relative paths, resolve them against the skill's\n")
	b.WriteString("directory (the parent of SKILL.md) and use absolute paths in tool calls.")
	return b.String()
}

// Activate returns the skill body wrapped in identifying tags, plus a listing of
// bundled resources. Re-activating an already loaded skill is a no-op.
func (m *Manager) Activate(name string) (string, error) {
	m.mu.Lock()
	sk, ok := m.byName[name]
	already := m.activated[name]
	if ok {
		m.activated[name] = true
	}
	m.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("skill %q is not available", name)
	}
	body := sk.Body
	if strings.TrimSpace(body) == "" {
		meta, b, err := loadSkillMD(sk.Location)
		if err != nil {
			return "", err
		}
		body = b
		_ = meta
	}
	var out strings.Builder
	out.WriteString(fmt.Sprintf("Skill '%s' activated (scope: %s).\n", sk.Name, sk.Scope))
	if already {
		out.WriteString("(this skill was already loaded earlier in the session; the instructions below are repeated for convenience)\n")
	}
	out.WriteString("--- BEGIN SKILL INSTRUCTIONS ---\n")
	out.WriteString(strings.TrimSpace(body) + "\n")
	out.WriteString("--- END SKILL INSTRUCTIONS ---\n")
	out.WriteString(fmt.Sprintf("Skill directory: %s\n", sk.BaseDir))
	out.WriteString("Relative paths in this skill are relative to the skill directory.\n")
	res := resources(sk.BaseDir)
	if len(res) > 0 {
		out.WriteString("Bundled resources (not loaded):\n")
		for _, r := range res {
			out.WriteString(r + "\n")
		}
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

// resources lists scripts/, references/ and assets/ without reading them.
func resources(base string) []string {
	var out []string
	for _, sub := range []string{"scripts", "references", "assets"} {
		dir := filepath.Join(base, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				out = append(out, sub+"/"+e.Name()+"/")
			} else {
				out = append(out, sub+"/"+e.Name())
			}
		}
	}
	sort.Strings(out)
	if len(out) > 100 {
		out = out[:100]
	}
	return out
}

// IsSkillContent reports whether a tool result carries skill instructions
// (used to protect it from context compression).
func IsSkillContent(s string) bool {
	return strings.Contains(s, "--- BEGIN SKILL INSTRUCTIONS ---")
}

// Body returns the cached body of a skill.
func (m *Manager) Body(name string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.byName[name]; ok {
		return s.Body
	}
	return ""
}

// ---------------------------------------------------------------------------
// SKILL.md parsing (stdlib only)
// ---------------------------------------------------------------------------

// loadSkillMD reads SKILL.md and returns the frontmatter map plus the body.
func loadSkillMD(path string) (map[string]string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > 20000 {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, "", err
	}
	if len(lines) == 0 {
		return nil, "", fmt.Errorf("empty file")
	}

	// The opening delimiter must be the first line.
	start := -1
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if trimmed == "---" {
			start = i
		}
		break
	}
	if start < 0 {
		// No frontmatter: the description is missing, so the skill will be
		// skipped by register() with a diagnostic (spec 4.3).
		body := strings.TrimSpace(strings.Join(lines, "\n"))
		return map[string]string{}, body, nil
	}
	end := -1
	for i := start + 1; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if t == "---" || t == "..." {
			end = i
			break
		}
	}
	if end < 0 {
		return nil, "", fmt.Errorf("unterminated frontmatter (missing closing ---)")
	}

	meta, diags := parseFrontmatter(lines[start+1 : end])
	if strings.TrimSpace(meta["name"]) == "" && strings.TrimSpace(meta["description"]) == "" {
		// Recoverable YAML issues: retry with a lenient repair pass.
		repaired := repairFrontmatter(lines[start+1 : end])
		meta2, _ := parseFrontmatter(repaired)
		if strings.TrimSpace(meta2["description"]) != "" {
			meta = meta2
		}
	}
	if len(diags) > 0 {
		if dbg := debug.Default(); dbg != nil {
			for _, d := range diags {
				dbg.Logf("skill frontmatter warning in %s: %s", path, d)
			}
		}
	}
	body := strings.TrimSpace(strings.Join(lines[end+1:], "\n"))
	return meta, body, nil
}

// parseFrontmatter parses the supported YAML subset: top-level scalars plus a
// flat metadata map. Values may be quoted and may span continuation lines.
func parseFrontmatter(lines []string) (map[string]string, []string) {
	meta := map[string]string{}
	var diags []string
	currentKey := ""
	inMetadata := false

	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		indented := strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
		trimmed := strings.TrimSpace(line)

		colon := strings.Index(trimmed, ":")
		if colon < 0 {
			// Continuation of a folded value.
			if currentKey != "" {
				meta[currentKey] = strings.TrimSpace(meta[currentKey] + " " + trimmed)
				continue
			}
			diags = append(diags, fmt.Sprintf("ignored unsupported line %q", trimmed))
			continue
		}
		key := strings.TrimSpace(trimmed[:colon])
		value := strings.TrimSpace(trimmed[colon+1:])

		if !indented && key == "metadata" {
			inMetadata = true
			currentKey = ""
			continue
		}
		if inMetadata && indented {
			meta["metadata."+key] = unquote(value)
			continue
		}
		inMetadata = false
		if !indented {
			currentKey = key
			meta[key] = unquote(value)
			continue
		}
		// Indented unknown key: ignore with a warning.
		diags = append(diags, fmt.Sprintf("ignored unsupported nested field %q", key))
	}
	return meta, diags
}

// repairFrontmatter quotes values that contain ": " (a common authoring mistake
// that strict YAML rejects) and retries.
func repairFrontmatter(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			out = append(out, l)
			continue
		}
		colon := strings.Index(trimmed, ":")
		if colon < 0 {
			out = append(out, l)
			continue
		}
		value := strings.TrimSpace(trimmed[colon+1:])
		if value != "" && !strings.HasPrefix(value, "\"") && !strings.HasPrefix(value, "'") &&
			(strings.Contains(value, ": ") || strings.HasSuffix(value, ":")) {
			indent := l[:len(l)-len(strings.TrimLeft(l, " \t"))]
			out = append(out, indent+trimmed[:colon+1]+" \""+strings.ReplaceAll(value, "\"", "\\\"")+"\"")
			continue
		}
		out = append(out, l)
	}
	return out
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return strings.ReplaceAll(s[1:len(s)-1], "\\\"", "\"")
		}
	}
	if s == "|" || s == ">" || s == "|-" || s == ">-" {
		return ""
	}
	return s
}
