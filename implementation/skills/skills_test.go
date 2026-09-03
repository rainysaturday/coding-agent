package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSKILLMDContent_Valid(t *testing.T) {
	content := `---
name: pdf-processor
description: Extract and summarize PDF documents.
license: MIT
allowed-tools: bash, read_file
metadata:
  category: document
  version: 1.2
---
# PDF Processing

Use pdftotext to extract text, then summarize.
`
	skill, err := ParseSKILLMDContent(content, "/skills/pdf-processor/SKILL.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.Name != "pdf-processor" {
		t.Errorf("Name = %q, want %q", skill.Name, "pdf-processor")
	}
	if skill.Description != "Extract and summarize PDF documents." {
		t.Errorf("Description = %q", skill.Description)
	}
	if skill.License != "MIT" {
		t.Errorf("License = %q", skill.License)
	}
	if len(skill.AllowedTools) != 2 || skill.AllowedTools[0] != "bash" || skill.AllowedTools[1] != "read_file" {
		t.Errorf("AllowedTools = %v", skill.AllowedTools)
	}
	if skill.Metadata["category"] != "document" || skill.Metadata["version"] != "1.2" {
		t.Errorf("Metadata = %v", skill.Metadata)
	}
	if !strings.Contains(skill.Instructions, "# PDF Processing") {
		t.Errorf("Instructions missing body: %q", skill.Instructions)
	}
	if strings.Contains(skill.Instructions, "---") {
		t.Errorf("Instructions should not include frontmatter")
	}
}

func TestParseSKILLMDContent_MissingFrontmatter(t *testing.T) {
	_, err := ParseSKILLMDContent("# Just a body\n", "")
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestParseSKILLMDContent_MissingDescription(t *testing.T) {
	content := "---\nname: foo\n---\nbody"
	_, err := ParseSKILLMDContent(content, "")
	if err == nil {
		t.Fatal("expected error for missing description")
	}
}

func TestParseSKILLMDContent_NameFallbackToDir(t *testing.T) {
	content := "---\ndescription: has no name\n---\nbody"
	skill, err := ParseSKILLMDContent(content, "/some/dir/my-skill/SKILL.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.Name != "my-skill" {
		t.Errorf("Name = %q, want %q", skill.Name, "my-skill")
	}
}

func TestParseSKILLMDContent_UnquotedValueWithColon(t *testing.T) {
	// Cross-client authoring mistake: unquoted value containing a colon.
	content := `---
name: foo
description: Handles time: 10:30 and other cases
---
body`
	skill, err := ParseSKILLMDContent(content, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.Description != "Handles time: 10:30 and other cases" {
		t.Errorf("Description = %q", skill.Description)
	}
}

func TestParseSKILLMDContent_InlineMetadata(t *testing.T) {
	content := `---
name: foo
description: desc
metadata: {category: doc, priority: high}
---`
	skill, err := ParseSKILLMDContent(content, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.Metadata["category"] != "doc" || skill.Metadata["priority"] != "high" {
		t.Errorf("Metadata = %v", skill.Metadata)
	}
}

func TestParseSKILLMDContent_AllowedToolsCommaSpace(t *testing.T) {
	content := `---
name: foo
description: desc
allowed-tools: bash,grep read_file
---`
	skill, err := ParseSKILLMDContent(content, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skill.AllowedTools) != 3 {
		t.Errorf("AllowedTools = %v", skill.AllowedTools)
	}
}

func TestBuildCatalogSection_Empty(t *testing.T) {
	catalog := NewCatalog()
	if got := BuildCatalogSection(catalog); got != "" {
		t.Errorf("empty catalog should produce empty section, got %q", got)
	}
	if got := BuildCatalogSection(nil); got != "" {
		t.Errorf("nil catalog should produce empty section, got %q", got)
	}
}

func TestBuildCatalogSection_NonEmpty(t *testing.T) {
	catalog := NewCatalog()
	catalog.add(&Skill{Name: "alpha", Description: "A skill"})
	catalog.add(&Skill{Name: "beta", Description: "B skill"})
	section := BuildCatalogSection(catalog)
	if !strings.Contains(section, "AVAILABLE SKILLS:") {
		t.Errorf("section missing header: %q", section)
	}
	if !strings.Contains(section, "alpha: A skill") {
		t.Errorf("section missing alpha: %q", section)
	}
	if !strings.Contains(section, "beta: B skill") {
		t.Errorf("section missing beta: %q", section)
	}
}

// TestDiscover_Shadowing verifies that a higher-precedence scope (project)
// overrides a lower-precedence scope (user) on a name collision, with a
// warning about the shadowed skill.
func TestDiscover_Shadowing(t *testing.T) {
	dir := t.TempDir()
	userDir := filepath.Join(dir, "user")
	projDir := filepath.Join(dir, "proj")

	writeSkill(t, filepath.Join(userDir, ".agents", "skills", "dup", "SKILL.md"),
		"---\nname: dup\ndescription: user version\n---\nuser body")
	writeSkill(t, filepath.Join(projDir, ".agents", "skills", "dup", "SKILL.md"),
		"---\nname: dup\ndescription: project version\n---\nproj body")

	catalog, warnings := Discover(DiscoverOptions{
		WorkingDir: projDir,
		HomeDir:    userDir,
		Trusted:    true,
	})
	if catalog.Len() != 1 {
		t.Fatalf("expected 1 skill, got %d", catalog.Len())
	}
	skill := catalog.Get("dup")
	if skill == nil {
		t.Fatal("dup skill not found")
	}
	if skill.Description != "project version" {
		t.Errorf("expected project version to win, got %q", skill.Description)
	}
	if skill.Scope != "project" {
		t.Errorf("Scope = %q, want project", skill.Scope)
	}
	foundWarning := false
	for _, w := range warnings {
		if strings.Contains(w, "shadowed") {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected shadowed warning, got %v", warnings)
	}
}

// TestDiscover_TrustGating verifies that project skills are skipped when the
// working directory is not trusted, while user skills still load.
func TestDiscover_TrustGating(t *testing.T) {
	dir := t.TempDir()
	userDir := filepath.Join(dir, "user")
	projDir := filepath.Join(dir, "proj")

	writeSkill(t, filepath.Join(userDir, ".agents", "skills", "user-skill", "SKILL.md"),
		"---\nname: user-skill\ndescription: user\n---\nuser body")
	writeSkill(t, filepath.Join(projDir, ".agents", "skills", "proj-skill", "SKILL.md"),
		"---\nname: proj-skill\ndescription: proj\n---\nproj body")

	catalog, warnings := Discover(DiscoverOptions{
		WorkingDir: projDir,
		HomeDir:    userDir,
		Trusted:    false,
	})
	if !catalog.Has("user-skill") {
		t.Error("user skill should load even when project untrusted")
	}
	if catalog.Has("proj-skill") {
		t.Error("project skill should be skipped when untrusted")
	}
	foundWarning := false
	for _, w := range warnings {
		if strings.Contains(w, "not trusted") {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected not-trusted warning, got %v", warnings)
	}
}

// TestDiscover_MalformedSkipped verifies that a skill with a missing
// description is skipped with a warning rather than aborting discovery.
func TestDiscover_MalformedSkipped(t *testing.T) {
	dir := t.TempDir()
	userDir := filepath.Join(dir, "user")

	writeSkill(t, filepath.Join(userDir, ".agents", "skills", "bad", "SKILL.md"),
		"---\nname: bad\n---\nno description")
	writeSkill(t, filepath.Join(userDir, ".agents", "skills", "good", "SKILL.md"),
		"---\nname: good\ndescription: ok\n---\nbody")

	catalog, warnings := Discover(DiscoverOptions{HomeDir: userDir, Trusted: true})
	if !catalog.Has("good") {
		t.Error("good skill should load")
	}
	if catalog.Has("bad") {
		t.Error("bad skill should be skipped")
	}
	foundWarning := false
	for _, w := range warnings {
		if strings.Contains(w, "bad") {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected warning for malformed skill, got %v", warnings)
	}
}

func TestDiscover_Resources(t *testing.T) {
	dir := t.TempDir()
	userDir := filepath.Join(dir, "user")
	skillDir := filepath.Join(userDir, ".agents", "skills", "res", "SKILL.md")
	writeSkill(t, skillDir, "---\nname: res\ndescription: resources\n---\nbody")

	// Create a resource file.
	if err := os.MkdirAll(filepath.Join(userDir, ".agents", "skills", "res", "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, ".agents", "skills", "res", "scripts", "run.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	catalog, _ := Discover(DiscoverOptions{HomeDir: userDir, Trusted: true})
	skill := catalog.Get("res")
	if skill == nil {
		t.Fatal("res skill not found")
	}
	if len(skill.Resources) != 1 || skill.Resources[0] != filepath.Join("scripts", "run.sh") {
		t.Errorf("Resources = %v", skill.Resources)
	}
}

func writeSkill(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
