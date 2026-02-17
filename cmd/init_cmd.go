package cmd

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

//go:embed templates/skill.md
var skillTemplate string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up Claude Code integration",
	Long: `Initialize hops for use with Claude Code.

Creates .claude/skills/hops/SKILL.md with feature store context and CLI reference,
so Claude Code can query your data, explore feature groups, and manage the feature store.

Run this in any project directory where you use Claude Code.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Write skill file to .claude/skills/hops/SKILL.md (project-level)
		skillDir := filepath.Join(".claude", "skills", "hops")
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			return fmt.Errorf("create skill dir: %w", err)
		}

		skillPath := filepath.Join(skillDir, "SKILL.md")
		if _, err := os.Stat(skillPath); err == nil {
			output.Info("Overwriting existing skill at %s", skillPath)
		}

		if err := os.WriteFile(skillPath, []byte(skillTemplate), 0644); err != nil {
			return fmt.Errorf("write skill: %w", err)
		}
		output.Success("Installed skill: %s", skillPath)

		// 2. Add Bash(hops *) permission to .claude/settings.local.json (project-level)
		if err := ensureHopsPermission(); err != nil {
			output.Info("Could not update settings: %v — add 'Bash(hops *)' manually", err)
		} else {
			output.Success("Updated permissions: .claude/settings.local.json")
		}

		// 3. Detect environment
		if os.Getenv("REST_ENDPOINT") != "" {
			output.Success("Detected Hopsworks terminal environment")
			output.Info("  Host: %s", os.Getenv("REST_ENDPOINT"))
			output.Info("  Project: %s", os.Getenv("PROJECT_NAME"))
		}

		output.Info("")
		output.Success("Done! Open Claude Code in this directory — type /hops or ask about your feature store.")
		return nil
	},
}

// ensureHopsPermission adds Bash(hops *) to .claude/settings.local.json if not already present.
func ensureHopsPermission() error {
	settingsPath := filepath.Join(".claude", "settings.local.json")

	const hopsPerm = "Bash(hops *)"

	settings := make(map[string]interface{})
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse %s: %w", settingsPath, err)
		}
	}

	perms, _ := settings["permissions"].(map[string]interface{})
	if perms == nil {
		perms = make(map[string]interface{})
		settings["permissions"] = perms
	}

	allowList, _ := perms["allow"].([]interface{})

	for _, item := range allowList {
		if s, ok := item.(string); ok && s == hopsPerm {
			return nil
		}
	}

	allowList = append(allowList, hopsPerm)
	perms["allow"] = allowList

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(settingsPath, append(data, '\n'), 0644)
}

func init() {
	rootCmd.AddCommand(initCmd)
}
