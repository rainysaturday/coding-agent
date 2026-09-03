package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/skills"
)

// writeTempSkill writes a SKILL.md into a temp skills directory and returns the
// directory to pass as --skills-dir.
func writeTempSkill(t *testing.T, name, description, body string) string {
	t.Helper()
	dir := t.TempDir()
	skillDir := filepath.Join(dir, name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n" + body
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestNewAgent_SkillsDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Skills = false
	ag := NewAgent(cfg)
	if ag.GetSkillCatalog() == nil {
		t.Fatal("expected a non-nil (empty) catalog when skills disabled")
	}
	if ag.GetSkillCatalog().Len() != 0 {
		t.Errorf("expected empty catalog, got %d", ag.GetSkillCatalog().Len())
	}
	if strings.Contains(ag.GetSystemPrompt(), "AVAILABLE SKILLS") {
		t.Error("system prompt should not contain AVAILABLE SKILLS when disabled")
	}
}

func TestNewAgent_SkillsDiscovered(t *testing.T) {
	dir := writeTempSkill(t, "review", "Review code for issues", "# Review\nDo a thorough review.")
	cfg := config.DefaultConfig()
	cfg.Skills = true
	cfg.SkillsDirs = []string{dir}
	ag := NewAgent(cfg)

	if ag.GetSkillCatalog().Len() != 1 {
		t.Fatalf("expected 1 skill, got %d", ag.GetSkillCatalog().Len())
	}
	if !ag.GetSkillCatalog().Has("review") {
		t.Error("review skill not discovered")
	}
	if !strings.Contains(ag.GetSystemPrompt(), "AVAILABLE SKILLS") {
		t.Error("system prompt should contain AVAILABLE SKILLS section")
	}
	if !strings.Contains(ag.GetSystemPrompt(), "review: Review code for issues") {
		t.Error("system prompt should list the review skill")
	}
}

func TestActivateSkill_Unknown(t *testing.T) {
	ag := &Agent{skillCatalog: skills.NewCatalog(), activatedSkills: make(map[string]bool)}
	if _, err := ag.ActivateSkill("nope"); err == nil {
		t.Error("expected error for unknown skill")
	}
}

func TestActivateSkill_DisabledCatalog(t *testing.T) {
	ag := &Agent{skillCatalog: nil, activatedSkills: make(map[string]bool)}
	if _, err := ag.ActivateSkill("x"); err == nil {
		t.Error("expected error when skill discovery is disabled")
	}
}

func TestActivateSkill_ValidAndDedup(t *testing.T) {
	ag := &Agent{
		skillCatalog:    skills.NewCatalog(),
		activatedSkills: make(map[string]bool),
		systemPrompt:    "base prompt",
	}
	ag.skillCatalog.Add(&skills.Skill{
		Name:         "review",
		Description:  "Review code",
		Instructions: "Do a thorough review.",
		Resources:    []string{"scripts/review.sh"},
	})

	out, err := ag.ActivateSkill("review")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "review") {
		t.Errorf("activation output should mention skill: %q", out)
	}
	if !strings.Contains(out, "scripts/review.sh") {
		t.Errorf("activation output should list resources: %q", out)
	}
	if !strings.Contains(ag.skillPrompt, "Do a thorough review.") {
		t.Error("skillPrompt should contain the instructions")
	}

	// Dedup: activating again should be a no-op.
	out2, err := ag.ActivateSkill("review")
	if err != nil {
		t.Fatalf("unexpected error on second activation: %v", err)
	}
	if !strings.Contains(out2, "already active") {
		t.Errorf("second activation should report already active: %q", out2)
	}
	if strings.Count(ag.skillPrompt, "Do a thorough review.") != 1 {
		t.Error("skill instructions should not be duplicated on repeated activation")
	}
}

func TestCurrentSystemPromptIncludesSkill(t *testing.T) {
	ag := &Agent{
		systemPrompt:    "base",
		skillPrompt:     "\nACTIVE SKILL: review\nInstructions here.",
		activatedSkills: make(map[string]bool),
		skillCatalog:    skills.NewCatalog(),
	}
	if got := ag.currentSystemPromptUnlocked(); !strings.Contains(got, "Instructions here.") || !strings.Contains(got, "base") {
		t.Errorf("currentSystemPromptUnlocked = %q, want base + skill", got)
	}
}

func TestListSkills(t *testing.T) {
	ag := &Agent{skillCatalog: skills.NewCatalog(), activatedSkills: make(map[string]bool)}
	ag.skillCatalog.Add(&skills.Skill{Name: "alpha", Description: "A skill"})
	ag.skillCatalog.Add(&skills.Skill{Name: "beta", Description: "B skill"})
	listing := ag.ListSkills()
	if !strings.Contains(listing, "alpha: A skill") || !strings.Contains(listing, "beta: B skill") {
		t.Errorf("ListSkills = %q", listing)
	}
}

func TestListSkills_Empty(t *testing.T) {
	ag := &Agent{skillCatalog: skills.NewCatalog(), activatedSkills: make(map[string]bool)}
	if got := ag.ListSkills(); !strings.Contains(got, "No skills") {
		t.Errorf("ListSkills empty = %q", got)
	}
}

func TestIsDirTrusted(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if !isDirTrusted(root, []string{root}) {
		t.Error("root should be trusted when it matches")
	}
	if !isDirTrusted(sub, []string{root}) {
		t.Error("subdir should be trusted when under a trusted root")
	}
	if isDirTrusted(sub, nil) {
		t.Error("no trust dirs should mean untrusted")
	}
	if isDirTrusted(sub, []string{"/nonexistent"}) {
		t.Error("unrelated trust dir should not mark sub trusted")
	}
}

// TestActivateSkill_CompactionExemption verifies that activated skill content
// rides along in the system prompt, which is never pruned by compaction.
func TestActivateSkill_CompactionExemption(t *testing.T) {
	ag := &Agent{
		skillCatalog:    skills.NewCatalog(),
		activatedSkills: make(map[string]bool),
		systemPrompt:    "base",
	}
	ag.skillCatalog.Add(&skills.Skill{Name: "s", Description: "d", Instructions: "protected instructions"})
	if _, err := ag.ActivateSkill("s"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	full := ag.currentSystemPromptUnlocked()
	if !strings.Contains(full, "protected instructions") {
		t.Error("activated skill instructions should be part of the system prompt")
	}
}
