// Package skills implements the open Agent Skills (SKILL.md) format for the
// coding agent. It discovers skill directories from the cross-client
// `.agents/skills/` convention (plus client-native `skills/` locations) at
// project and user scope, parses SKILL.md frontmatter using only the Go
// standard library, and builds the compact catalog surfaced to the model in
// the system prompt.
package skills

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skill represents a single discovered skill.
type Skill struct {
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	License       string            `json:"license,omitempty"`
	Compatibility string            `json:"compatibility,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	// AllowedTools is the optional set of tools the skill is permitted to use.
	// It is advisory only; the coding agent does not enforce it at execution
	// time, but surfaces it to the model.
	AllowedTools []string `json:"allowed_tools,omitempty"`
	// Path is the absolute path to the skill's SKILL.md file.
	Path string `json:"path"`
	// Dir is the absolute path to the skill directory (containing scripts/,
	// references/, assets/, etc.).
	Dir string `json:"dir"`
	// Instructions is the markdown body after the frontmatter closing `---`.
	Instructions string `json:"instructions"`
	// Resources lists the bundled resource files (scripts/, references/,
	// assets/) without their contents being read eagerly into context.
	Resources []string `json:"resources,omitempty"`
	// Scope is "project" or "user" (used for trust gating and shadowing logs).
	Scope string `json:"scope"`
}

// Catalog is an in-memory collection of discovered skills keyed by name.
type Catalog struct {
	Skills map[string]*Skill
	Order  []string // Deterministic display order (sorted by name)
}

// NewCatalog returns an empty catalog.
func NewCatalog() *Catalog {
	return &Catalog{Skills: make(map[string]*Skill)}
}

// Has reports whether a skill with the given name exists in the catalog.
func (c *Catalog) Has(name string) bool {
	_, ok := c.Skills[name]
	return ok
}

// Get returns the skill with the given name, or nil if absent.
func (c *Catalog) Get(name string) *Skill {
	return c.Skills[name]
}

// Names returns the skill names in deterministic (sorted) order.
func (c *Catalog) Names() []string {
	out := make([]string, len(c.Order))
	copy(out, c.Order)
	return out
}

// Len returns the number of skills in the catalog.
func (c *Catalog) Len() int {
	return len(c.Skills)
}

// add inserts a skill, returning a warning if it shadows an existing one.
// Later-inserted skills take precedence (so higher-precedence scopes are
// added last).
func (c *Catalog) add(s *Skill) (shadowed bool) {
	if _, exists := c.Skills[s.Name]; exists {
		// A higher-precedence scope wins: replace the existing (shadowed) entry
		// so the newly discovered skill takes precedence.
		c.Skills[s.Name] = s
		return true
	}
	c.Skills[s.Name] = s
	c.Order = append(c.Order, s.Name)
	sort.Strings(c.Order)
	return false
}

// Add inserts a skill into the catalog, returning true if it shadowed (and
// replaced) an existing skill of the same name. It is exported for tests and
// for programmatic catalog construction.
func (c *Catalog) Add(s *Skill) bool {
	return c.add(s)
}

// DiscoverOptions controls skill discovery.
type DiscoverOptions struct {
	// WorkingDir is the project working directory (project scope).
	WorkingDir string
	// HomeDir is the user's home directory (user scope).
	HomeDir string
	// SkillsDirs are additional custom search directories (--skills-dir).
	SkillsDirs []string
	// Trusted indicates whether the project directory is trusted. When false,
	// project-level skills are skipped (but user-level skills are always loaded).
	Trusted bool
}

// Discover scans the configured scopes and returns a catalog. It never fails
// hard: individual malformed skills are skipped with a warning collected in
// the returned slice. The catalog is always non-nil (possibly empty).
func Discover(opts DiscoverOptions) (*Catalog, []string) {
	catalog := NewCatalog()
	var warnings []string

	// User scope is always trusted and loaded first (lower precedence).
	if opts.HomeDir != "" {
		discoverScope(catalog, opts.HomeDir, "user", true, &warnings)
	}

	// Project scope is loaded only when the working directory is trusted.
	if opts.WorkingDir != "" {
		if !opts.Trusted {
			if hasSkillDirs(opts.WorkingDir) {
				warnings = append(warnings, fmt.Sprintf("skills: project scope skipped because the working directory is not trusted (use --trust to enable)"))
			}
		} else {
			discoverScope(catalog, opts.WorkingDir, "project", true, &warnings)
		}
	}

	// Custom --skills-dir paths are treated as trusted user-supplied base
	// directories that directly contain skill subdirectories.
	for _, dir := range opts.SkillsDirs {
		if dir == "" {
			continue
		}
		scanBaseDir(catalog, dir, "custom", &warnings)
	}

	return catalog, warnings
}

// discoverScope scans a single scope root for skill directories. The scope
// root may contain skills under `<root>/skills/`, `<root>/.agents/skills/`,
// and (for compatibility) `<root>/.claude/skills/`. The `.agents/` convention
// is scanned first so it is the interoperable default.
func discoverScope(catalog *Catalog, root, scope string, trusted bool, warnings *[]string) {
	// Ordered candidate directories: cross-client convention first.
	baseDirs := []string{
		filepath.Join(root, ".agents", "skills"),
		filepath.Join(root, "skills"),
		filepath.Join(root, ".claude", "skills"),
	}
	for _, base := range baseDirs {
		scanBaseDir(catalog, base, scope, warnings)
	}
}

// hasSkillDirs reports whether the given root contains any candidate skill
// base directory (e.g. <root>/.agents/skills/, <root>/skills/). It is used to
// decide whether to warn about an untrusted project scope that actually
// bundles skills.
func hasSkillDirs(root string) bool {
	for _, base := range []string{
		filepath.Join(root, ".agents", "skills"),
		filepath.Join(root, "skills"),
		filepath.Join(root, ".claude", "skills"),
	} {
		if info, err := os.Stat(base); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// scanBaseDir scans a single skill base directory (e.g. .agents/skills/) for
// subdirectories that are skills (contain a SKILL.md).
func scanBaseDir(catalog *Catalog, baseDir, scope string, warnings *[]string) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		// Not an error: the directory simply doesn't exist or isn't readable.
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if isSkippedDir(name) {
			continue
		}
		skillDir := filepath.Join(baseDir, name)
		skill, warn := loadSkill(skillDir, scope)
		if warn != "" {
			*warnings = append(*warnings, warn)
		}
		if skill == nil {
			continue
		}
		if catalog.add(skill) {
			*warnings = append(*warnings, fmt.Sprintf("skills: skill %q shadowed by a higher-precedence scope (path: %s)", skill.Name, skill.Path))
		}
	}
}

// isSkippedDir reports whether a directory should be skipped during scanning.
func isSkippedDir(name string) bool {
	switch name {
	case ".git", "node_modules", ".svn", ".hg", ".DS_Store", "vendor":
		return true
	}
	return strings.HasPrefix(name, ".")
}

// loadSkill loads a single skill directory. It returns the parsed Skill, or
// nil (plus a warning) if the directory is not a valid skill.
func loadSkill(dir, scope string) (*Skill, string) {
	skillPath := filepath.Join(dir, "SKILL.md")
	info, err := os.Stat(skillPath)
	if err != nil || info.IsDir() {
		// Not a skill: a directory without a SKILL.md is ignored silently.
		return nil, ""
	}

	skill, err := ParseSKILLMD(skillPath)
	if err != nil {
		return nil, fmt.Sprintf("skills: skipping %s: %v", skillPath, err)
	}
	skill.Dir = dir
	skill.Scope = scope
	skill.Resources = listResources(dir)
	return skill, ""
}

// listResources lists bundled resource files under the standard subdirectories
// (scripts/, references/, assets/) without reading their contents.
func listResources(dir string) []string {
	var resources []string
	for _, sub := range []string{"scripts", "references", "assets"} {
		subDir := filepath.Join(dir, sub)
		entries, err := os.ReadDir(subDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			resources = append(resources, filepath.Join(sub, entry.Name()))
		}
	}
	sort.Strings(resources)
	return resources
}

// ParseSKILLMD reads and parses a SKILL.md file. It returns the parsed skill
// with its instructions (the markdown body after frontmatter). A skill is
// rejected (returning an error) when its frontmatter is unparseable or its
// description is missing/empty.
func ParseSKILLMD(path string) (*Skill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read SKILL.md: %w", err)
	}
	return ParseSKILLMDContent(string(content), path)
}

// ParseSKILLMDContent parses SKILL.md content. The path is used only for
// reporting the skill location; it may be empty in tests.
func ParseSKILLMDContent(content, path string) (*Skill, error) {
	frontmatter, body, hasFrontmatter := splitFrontmatter(content)
	if !hasFrontmatter {
		return nil, fmt.Errorf("missing frontmatter (no leading '---' block)")
	}

	fields := parseFrontmatter(frontmatter)

	name := strings.TrimSpace(fields["name"])
	if name == "" {
		// Fall back to the directory name as a best-effort default.
		if dir := filepath.Dir(path); dir != "." && dir != "" {
			name = filepath.Base(dir)
		}
	}
	if name == "" {
		return nil, fmt.Errorf("missing required 'name' in frontmatter")
	}

	description := strings.TrimSpace(fields["description"])
	if description == "" {
		return nil, fmt.Errorf("missing or empty 'description' in frontmatter")
	}

	skill := &Skill{
		Name:          name,
		Description:   description,
		License:       strings.TrimSpace(fields["license"]),
		Compatibility: strings.TrimSpace(fields["compatibility"]),
		Metadata:      collectMetadata(fields),
		AllowedTools:  splitTools(fields["allowed-tools"]),
		Path:          path,
		Instructions:  strings.TrimSpace(body),
	}
	return skill, nil
}

// collectMetadata merges the inline `metadata: {...}` value with any indented
// `metadata.<key>: <value>` sub-fields into a single flat map.
func collectMetadata(fields map[string]string) map[string]string {
	m := parseMetadata(fields["metadata"])
	for k, v := range fields {
		if strings.HasPrefix(k, "metadata.") {
			key := strings.TrimPrefix(k, "metadata.")
			if m == nil {
				m = make(map[string]string)
			}
			m[key] = v
		}
	}
	return m
}

// splitFrontmatter splits SKILL.md content into the frontmatter block and the
// body. It returns hasFrontmatter=false if there is no leading `---` delimiter.
func splitFrontmatter(content string) (frontmatter, body string, hasFrontmatter bool) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", content, false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), true
		}
	}
	// No closing delimiter: treat the whole thing as frontmatter with no body.
	return strings.Join(lines[1:], "\n"), "", true
}

// parseFrontmatter parses the lenient YAML-subset frontmatter block into a
// flat key/value map. It handles top-level scalar fields and a flat
// `metadata` map. Values may be single- or double-quoted; unquoted values are
// taken verbatim after the first `:` so values containing colons (a common
// cross-client authoring mistake) still parse.
func parseFrontmatter(frontmatter string) map[string]string {
	fields := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(frontmatter))
	var currentKey string
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Determine indentation level (spaces).
		indent := len(line) - len(strings.TrimLeft(line, " "))

		if indent == 0 {
			key, val, ok := splitKeyValue(trimmed)
			if !ok {
				continue
			}
			fields[key] = val
			currentKey = key
			continue
		}

		// Indented line: a metadata sub-field under the previous top-level key.
		if currentKey == "metadata" {
			key, val, ok := splitKeyValue(trimmed)
			if ok {
				fields["metadata."+key] = val
			}
		}
	}
	return fields
}

// splitKeyValue splits a `key: value` line into a normalized key and value.
// The value has surrounding quotes stripped. It returns ok=false for lines
// that are not a key/value pair.
func splitKeyValue(line string) (key, value string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])
	value = strings.Trim(value, "\"'")
	return key, value, true
}

// parseMetadata flattens the parsed `metadata` block (both `metadata: {a: b}`
// inline and indented `a: b` lines) into a map.
func parseMetadata(raw string) map[string]string {
	if raw == "" {
		return nil
	}
	m := make(map[string]string)
	// Inline map form: `{a: b, c: d}`.
	inline := strings.TrimSpace(raw)
	inline = strings.TrimPrefix(inline, "{")
	inline = strings.TrimSuffix(inline, "}")
	for _, part := range strings.Split(inline, ",") {
		key, val, ok := splitKeyValue(part)
		if ok {
			m[key] = val
		}
	}
	return m
}

// splitTools splits an `allowed-tools` value on whitespace or commas.
func splitTools(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ' ' || r == '\t' || r == ',' || r == '\n'
	})
	if len(fields) == 0 {
		return nil
	}
	return fields
}

// BuildCatalogSection builds the "AVAILABLE SKILLS" section added to the
// system prompt. It returns an empty string when no skills are discovered, so
// the section (and its behavioral instructions) is omitted entirely.
func BuildCatalogSection(catalog *Catalog) string {
	if catalog == nil || catalog.Len() == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("AVAILABLE SKILLS:\n")
	sb.WriteString("You may load a skill to gain specialized knowledge and workflows. ")
	sb.WriteString("To use a skill, read its SKILL.md at the given path (or call activate_skill), then follow its instructions. ")
	sb.WriteString("Only load a skill that is relevant to the current task; do not load unrelated skills.\n\n")

	for _, name := range catalog.Names() {
		skill := catalog.Skills[name]
		if skill == nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("- %s: %s\n", skill.Name, skill.Description))
	}

	return sb.String()
}
