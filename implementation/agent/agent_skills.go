package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/coding-agent/harness/config"
	"github.com/coding-agent/harness/skills"
)

// discoverSkills discovers Agent Skills (Requirement 047) and appends the
// AVAILABLE SKILLS catalog section to the base system prompt. When skill
// discovery is disabled or no skills are found, the catalog is empty and no
// section is added. Project-level skills are trust-gated.
func (a *Agent) discoverSkills(cfg *config.Config) {
	if !cfg.Skills {
		a.skillCatalog = skills.NewCatalog()
		return
	}

	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	trusted := isDirTrusted(cwd, cfg.TrustDirs)

	catalog, warnings := skills.Discover(skills.DiscoverOptions{
		WorkingDir: cwd,
		HomeDir:    home,
		SkillsDirs: cfg.SkillsDirs,
		Trusted:    trusted,
	})
	a.skillCatalog = catalog

	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}

	if section := skills.BuildCatalogSection(catalog); section != "" {
		a.systemPrompt += "\n\n" + section
	}
}

// isDirTrusted reports whether the given directory is covered by any entry in
// the trust list. A directory is trusted if it equals a trust entry or is a
// subdirectory of one (so trusting a repo root covers its subdirectories).
func isDirTrusted(dir string, trustDirs []string) bool {
	if dir == "" || len(trustDirs) == 0 {
		return false
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = filepath.Clean(dir)
	}
	for _, td := range trustDirs {
		if td == "" {
			continue
		}
		absTD, err := filepath.Abs(td)
		if err != nil {
			absTD = filepath.Clean(td)
		}
		rel, err := filepath.Rel(absTD, absDir)
		if err != nil {
			continue
		}
		// Trusted if the directory is the trust root or under it.
		if rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)) {
			return true
		}
	}
	return false
}

// ActivateSkill loads a skill's instructions into the agent's context. It is
// the handler backing both the activate_skill tool and the /skill slash
// command. Activating an already-active skill is a no-op (deduplication). The
// instructions are appended to the skill prompt, which rides along with the
// base system prompt on every inference call and is therefore exempt from
// context compaction.
func (a *Agent) ActivateSkill(name string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.skillCatalog == nil {
		return "", fmt.Errorf("skill discovery is disabled (use --skills to enable)")
	}
	skill := a.skillCatalog.Get(name)
	if skill == nil {
		return "", fmt.Errorf("unknown skill %q (use /skills to list available skills)", name)
	}

	if a.activatedSkills[name] {
		return fmt.Sprintf("Skill %q is already active.", name), nil
	}
	a.activatedSkills[name] = true

	// Append the instructions to the skill prompt so they survive compaction.
	if a.skillPrompt != "" {
		a.skillPrompt += "\n"
	}
	a.skillPrompt += fmt.Sprintf("\nACTIVE SKILL: %s\n%s\n", name, skill.Instructions)

	return formatActivationResult(skill), nil
}

// formatActivationResult builds the concise confirmation returned to the model
// after a skill is activated. The full instructions are carried in the skill
// prompt; the result lists the bundled resources without reading their content.
func formatActivationResult(skill *skills.Skill) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Skill %q activated. Its instructions are now active in your context.\n", skill.Name))
	if len(skill.Resources) > 0 {
		sb.WriteString("Bundled resources (read them with read_file or run them as needed):\n")
		for _, r := range skill.Resources {
			sb.WriteString(fmt.Sprintf("  - %s\n", r))
		}
	} else {
		sb.WriteString("This skill bundles no additional resources.\n")
	}
	return strings.TrimSpace(sb.String())
}

// ListSkills returns a human-readable listing of the available skills.
func (a *Agent) ListSkills() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.skillCatalog == nil || a.skillCatalog.Len() == 0 {
		return "No skills are available."
	}
	var sb strings.Builder
	sb.WriteString("Available skills:\n")
	for _, name := range a.skillCatalog.Names() {
		skill := a.skillCatalog.Get(name)
		if skill == nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("  %s: %s\n", name, skill.Description))
	}
	return strings.TrimSpace(sb.String())
}

// GetSkillCatalog returns the agent's skill catalog (may be empty).
func (a *Agent) GetSkillCatalog() *skills.Catalog {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.skillCatalog
}

// currentSystemPromptUnlocked returns the full system prompt including any
// activated skill instructions. The caller must hold a.mu.
func (a *Agent) currentSystemPromptUnlocked() string {
	return a.systemPrompt + a.skillPrompt
}
