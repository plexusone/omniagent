package skills

import (
	"strings"
)

// InjectConfig controls how skills are injected into prompts.
type InjectConfig struct {
	MaxSkills       int    // Maximum skills to inject (0 = unlimited)
	IncludeDisabled bool   // Include skills with missing requirements
	Separator       string // Separator between skills
}

// DefaultInjectConfig returns sensible defaults.
func DefaultInjectConfig() InjectConfig {
	return InjectConfig{
		MaxSkills: 20,
		Separator: "\n\n---\n\n",
	}
}

// InjectIntoPrompt appends skill content to the system prompt.
// Skills with missing requirements are skipped unless IncludeDisabled is true.
func InjectIntoPrompt(systemPrompt string, skills []*Skill, cfg InjectConfig) string {
	if len(skills) == 0 {
		return systemPrompt
	}

	var sb strings.Builder
	sb.WriteString(systemPrompt)
	sb.WriteString("\n\n# Available Skills\n\n")
	sb.WriteString("The following skills provide guidance on using specific tools and capabilities.\n\n")

	count := 0
	for _, skill := range skills {
		if cfg.MaxSkills > 0 && count >= cfg.MaxSkills {
			break
		}

		// Skip skills with missing requirements unless configured otherwise
		if !cfg.IncludeDisabled && !skill.IsAvailable() {
			continue
		}

		// Write skill header
		sb.WriteString("## ")
		if emoji := skill.Emoji(); emoji != "" {
			sb.WriteString(emoji)
			sb.WriteString(" ")
		}
		sb.WriteString(skill.Name)
		sb.WriteString("\n\n")

		// Write skill content
		sb.WriteString(skill.Content)
		sb.WriteString(cfg.Separator)

		count++
	}

	return sb.String()
}

// FilterAvailable returns only skills that have all requirements met.
func FilterAvailable(skills []*Skill) []*Skill {
	var available []*Skill
	for _, s := range skills {
		if s.IsAvailable() {
			available = append(available, s)
		}
	}
	return available
}

// FilterByName returns skills matching any of the given names.
func FilterByName(skills []*Skill, names []string) []*Skill {
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}

	var filtered []*Skill
	for _, s := range skills {
		if nameSet[s.Name] {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// ExcludeByName returns skills not matching any of the given names.
func ExcludeByName(skills []*Skill, names []string) []*Skill {
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}

	var filtered []*Skill
	for _, s := range skills {
		if !nameSet[s.Name] {
			filtered = append(filtered, s)
		}
	}
	return filtered
}
