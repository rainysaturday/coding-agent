package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coding-agent/coding-agent/config"
)

// isolateHome points HOME at a temp dir so user-scope scanning and the trust
// file do not leak between tests or touch the real home directory.
func isolateHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func writeSkill(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testCfg(trust ...string) *config.Config {
	c := config.NewDefault()
	c.SkillsEnabled = true
	c.TrustDirs = trust
	return c
}

func TestDiscoverTrustedProject(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeSkill(t, filepath.Join(cwd, ".agents", "skills", "hello"),
		"---\nname: hello\ndescription: Says hello to the user\nlicense: MIT\n---\n\nDo the hello thing.\nStep two.\n")

	m := Discover(testCfg(cwd), cwd)
	if m.Count() != 1 {
		t.Fatalf("count: %d (%v)", m.Count(), m.Diagnostics())
	}
	s := m.List()[0]
	if s.Name != "hello" || s.Scope != "project" || s.License != "MIT" {
		t.Fatalf("skill: %+v", s)
	}
	if !strings.Contains(s.Description, "Says hello") {
		t.Fatalf("description: %q", s.Description)
	}

	cat := m.Catalog()
	if !strings.Contains(cat, "hello") || !strings.Contains(cat, "Says hello") {
		t.Fatalf("catalog: %q", cat)
	}

	out, err := m.Activate("hello")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Do the hello thing.") || !strings.Contains(out, "BEGIN SKILL INSTRUCTIONS") {
		t.Fatalf("activate: %q", out)
	}
	// Second activation notes repetition.
	out2, _ := m.Activate("hello")
	if !strings.Contains(out2, "already loaded") {
		t.Fatalf("repeat activate: %q", out2)
	}
	if _, err := m.Activate("missing"); err == nil {
		t.Fatal("expected error for unknown skill")
	}
}

func TestProjectSkillsSkippedWhenUntrusted(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeSkill(t, filepath.Join(cwd, ".agents", "skills", "hello"),
		"---\nname: hello\ndescription: d\n---\nbody\n")

	m := Discover(testCfg(), cwd) // no trust
	if m.Count() != 0 {
		t.Fatalf("untrusted project skills must be skipped, got %d", m.Count())
	}
	var warned bool
	for _, d := range m.Diagnostics() {
		if strings.Contains(d.Message, "not trusted") {
			warned = true
		}
	}
	if !warned {
		t.Fatal("expected a trust diagnostic")
	}
}

func TestTrustPersistence(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeSkill(t, filepath.Join(cwd, "skills", "t"), "---\nname: t\ndescription: d\n---\nx\n")

	if Discover(testCfg(), cwd).Count() != 0 {
		t.Fatal("should be untrusted initially")
	}
	if err := Trust(cwd); err != nil {
		t.Fatal(err)
	}
	if Discover(testCfg(), cwd).Count() != 1 {
		t.Fatal("persisted trust should enable project skills")
	}
}

func TestMissingDescriptionSkipped(t *testing.T) {
	// Spec 4.3 / requirement 047: a missing or empty description skips the
	// skill with a diagnostic; the body is NOT used as a fallback.
	isolateHome(t)
	cwd := t.TempDir()
	writeSkill(t, filepath.Join(cwd, ".agents", "skills", "nodesc"), "---\nname: nodesc\n---\nExplains things.\n\nDetails.\n")
	writeSkill(t, filepath.Join(cwd, ".agents", "skills", "emptydesc"), "---\nname: emptydesc\ndescription:\n---\nbody\n")
	m := Discover(testCfg(cwd), cwd)
	if m.Count() != 0 {
		t.Fatalf("skills without description must be skipped: %d", m.Count())
	}
	var found int
	for _, d := range m.Diagnostics() {
		if strings.Contains(d.Message, "description") {
			found++
		}
	}
	if found < 2 {
		t.Fatalf("expected description diagnostics for both skills: %v", m.Diagnostics())
	}
}

func TestEmptySkillSkipped(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeSkill(t, filepath.Join(cwd, ".agents", "skills", "empty"), "---\nname: empty\n---\n")
	m := Discover(testCfg(cwd), cwd)
	if m.Count() != 0 {
		t.Fatalf("skill with no description anywhere must be skipped: %d", m.Count())
	}
	var found bool
	for _, d := range m.Diagnostics() {
		if strings.Contains(d.Message, "description") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected description diagnostic")
	}
}

func TestNoFrontmatterSkipped(t *testing.T) {
	// A SKILL.md without frontmatter has no description: skip with a
	// diagnostic (spec 4.3: unparseable/missing frontmatter -> skip).
	isolateHome(t)
	cwd := t.TempDir()
	writeSkill(t, filepath.Join(cwd, ".agents", "skills", "plain"), "Just a heading paragraph.\n\nMore text.\n")
	m := Discover(testCfg(cwd), cwd)
	if m.Count() != 0 {
		t.Fatalf("skill without frontmatter must be skipped: %d", m.Count())
	}
	var found bool
	for _, d := range m.Diagnostics() {
		if strings.Contains(d.Message, "description") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected description diagnostic")
	}
}

func TestSkipDirsNotScanned(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeSkill(t, filepath.Join(cwd, ".agents", "skills", "node_modules", "evil"),
		"---\nname: evil\ndescription: d\n---\nx\n")
	m := Discover(testCfg(cwd), cwd)
	if m.Count() != 0 {
		t.Fatalf("node_modules must be skipped: %d", m.Count())
	}
}

func TestProjectScopeWinsShadowing(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeSkill(t, filepath.Join(cwd, ".agents", "skills", "dup"),
		"---\nname: dup\ndescription: project copy\n---\np\n")
	custom := filepath.Join(cwd, "..", "customskills")
	writeSkill(t, filepath.Join(custom, "dup"),
		"---\nname: dup\ndescription: custom copy\n---\nc\n")

	cfg := testCfg(cwd)
	cfg.SkillsDirs = []string{custom}
	m := Discover(cfg, cwd)
	if m.Count() != 1 {
		t.Fatalf("count: %d", m.Count())
	}
	if m.List()[0].Description != "project copy" {
		t.Fatalf("project scope should win: %q", m.List()[0].Description)
	}
}

func TestQuotedAndMultilineFrontmatter(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	writeSkill(t, filepath.Join(cwd, ".agents", "skills", "q"),
		"---\nname: q\ndescription: \"A quoted: description that continues\n  on another line\"\nallowed-tools: read_file, grep\n---\nbody\n")
	m := Discover(testCfg(cwd), cwd)
	if m.Count() != 1 {
		t.Fatalf("count: %d (%v)", m.Count(), m.Diagnostics())
	}
	s := m.List()[0]
	if !strings.Contains(s.Description, "A quoted: description") || !strings.Contains(s.Description, "another line") {
		t.Fatalf("multiline description: %q", s.Description)
	}
	if s.AllowedTools == "" {
		t.Fatal("allowed-tools not captured")
	}
}

func TestIsSkillContent(t *testing.T) {
	if !IsSkillContent("Skill 'x' activated.\n--- BEGIN SKILL INSTRUCTIONS ---\nbody\n--- END SKILL INSTRUCTIONS ---") {
		t.Fatal("activated skill output should be detected")
	}
	if IsSkillContent("just some text") {
		t.Fatal("plain text is not skill content")
	}
}

func TestBundledResourcesListed(t *testing.T) {
	isolateHome(t)
	cwd := t.TempDir()
	dir := filepath.Join(cwd, ".agents", "skills", "withres")
	writeSkill(t, dir, "---\nname: withres\ndescription: d\n---\nbody\n")
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts", "run.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := Discover(testCfg(cwd), cwd).Activate("withres")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "scripts/run.sh") || !strings.Contains(out, "Bundled resources") {
		t.Fatalf("resources: %q", out)
	}
}

func TestLenientUnquotedColon(t *testing.T) {
	// Spec 4.2: unquoted values containing a colon are technically invalid
	// YAML but must be recovered by the lenient repair pass.
	isolateHome(t)
	cwd := t.TempDir()
	writeSkill(t, filepath.Join(cwd, ".agents", "skills", "pdf"),
		"---\nname: pdf\ndescription: Use this skill when: the user asks about PDFs\n---\nbody\n")
	m := Discover(testCfg(cwd), cwd)
	if m.Count() != 1 {
		t.Fatalf("lenient parse should recover the skill: %d (%v)", m.Count(), m.Diagnostics())
	}
	if !strings.Contains(m.List()[0].Description, "the user asks about PDFs") {
		t.Fatalf("recovered description: %q", m.List()[0].Description)
	}
}
