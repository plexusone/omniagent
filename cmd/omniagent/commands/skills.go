package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/plexusone/omniagent/skills"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage skills",
	Long: `Manage skills that teach your agent how to use tools and services.

Skills are loaded from:
  1. ~/.omniagent/skills/
  2. ./skills/
  3. ./.skills/

Each skill is a directory containing a SKILL.md file with instructions.`,
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available skills",
	Long:  `List all discovered skills and their availability status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		discovered, err := skills.Discover(skills.DefaultSearchPaths())
		if err != nil {
			return fmt.Errorf("discovering skills: %w", err)
		}

		if len(discovered) == 0 {
			fmt.Println("No skills found.")
			fmt.Println("\nSearched directories:")
			for _, p := range skills.DefaultSearchPaths() {
				fmt.Printf("  - %s\n", p)
			}
			return nil
		}

		fmt.Printf("Found %d skills:\n\n", len(discovered))

		for _, skill := range discovered {
			status := "✓"
			errs := skill.CheckRequirements()
			if len(errs) > 0 {
				status = "✗"
			}

			emoji := skill.Emoji()
			if emoji == "" {
				emoji = "📦"
			}

			// Truncate description if too long
			desc := skill.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}

			fmt.Printf("%s %s %s\n", status, emoji, skill.Name)
			if desc != "" {
				fmt.Printf("    %s\n", desc)
			}

			if len(errs) > 0 {
				for _, e := range errs {
					if reqErr, ok := e.(*skills.RequirementError); ok {
						fmt.Printf("    ⚠ Missing %s: %s\n", reqErr.Type, reqErr.Name)
						if hint := reqErr.InstallHint(); hint != "" {
							fmt.Printf("      Install: %s\n", hint)
						}
					}
				}
			}
		}

		return nil
	},
}

var skillsInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show skill details",
	Long:  `Show detailed information about a specific skill.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		skillName := args[0]

		discovered, err := skills.Discover(skills.DefaultSearchPaths())
		if err != nil {
			return fmt.Errorf("discovering skills: %w", err)
		}

		var skill *skills.Skill
		for _, s := range discovered {
			if s.Name == skillName {
				skill = s
				break
			}
		}

		if skill == nil {
			return fmt.Errorf("skill not found: %s", skillName)
		}

		// Print skill info
		emoji := skill.Emoji()
		if emoji != "" {
			fmt.Printf("%s %s\n", emoji, skill.Name)
		} else {
			fmt.Printf("%s\n", skill.Name)
		}
		fmt.Println(strings.Repeat("=", len(skill.Name)+3))

		if skill.Description != "" {
			fmt.Printf("\n%s\n", skill.Description)
		}

		if skill.Homepage != "" {
			fmt.Printf("\nHomepage: %s\n", skill.Homepage)
		}

		fmt.Printf("\nPath: %s\n", skill.Path)

		// Status
		errs := skill.CheckRequirements()
		if len(errs) == 0 {
			fmt.Println("\nStatus: ✓ Available")
		} else {
			fmt.Println("\nStatus: ✗ Unavailable")
			fmt.Println("\nMissing requirements:")
			for _, e := range errs {
				if reqErr, ok := e.(*skills.RequirementError); ok {
					fmt.Printf("  - %s: %s\n", reqErr.Type, reqErr.Name)
					if hint := reqErr.InstallHint(); hint != "" {
						fmt.Printf("    Install: %s\n", hint)
					}
				}
			}
		}

		// Features
		if skill.HasHooks || skill.HasScripts {
			fmt.Println("\nFeatures:")
			if skill.HasHooks {
				fmt.Println("  - Has hooks (TypeScript)")
			}
			if skill.HasScripts {
				fmt.Println("  - Has scripts (shell)")
			}
		}

		return nil
	},
}

var skillsCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check skill requirements",
	Long:  `Validate all skills and report missing requirements.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		discovered, err := skills.Discover(skills.DefaultSearchPaths())
		if err != nil {
			return fmt.Errorf("discovering skills: %w", err)
		}

		if len(discovered) == 0 {
			fmt.Println("No skills found.")
			return nil
		}

		available := 0
		unavailable := 0

		for _, skill := range discovered {
			errs := skill.CheckRequirements()
			if len(errs) == 0 {
				available++
			} else {
				unavailable++
			}
		}

		fmt.Printf("Skills: %d available, %d unavailable\n", available, unavailable)

		if unavailable > 0 {
			fmt.Println("\nUnavailable skills:")
			for _, skill := range discovered {
				errs := skill.CheckRequirements()
				if len(errs) > 0 {
					fmt.Printf("\n  %s:\n", skill.Name)
					for _, e := range errs {
						if reqErr, ok := e.(*skills.RequirementError); ok {
							fmt.Printf("    - Missing %s: %s\n", reqErr.Type, reqErr.Name)
							if hint := reqErr.InstallHint(); hint != "" {
								fmt.Printf("      Install: %s\n", hint)
							}
						}
					}
				}
			}
		}

		return nil
	},
}

func init() {
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsInfoCmd)
	skillsCmd.AddCommand(skillsCheckCmd)
}
