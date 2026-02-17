package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up Claude Code integration",
	Long: `Initialize hops for use with Claude Code.

Creates a /hops skill and configures permissions so Claude can use hops commands directly.
Run this in any directory where you use Claude Code.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, _ := os.UserHomeDir()

		// 1. Create .claude/commands/hops.md skill file
		skillDir := filepath.Join(home, ".claude", "commands")
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			return fmt.Errorf("create skill dir: %w", err)
		}

		skillContent := generateSkillContent()
		skillPath := filepath.Join(skillDir, "hops.md")
		if err := os.WriteFile(skillPath, []byte(skillContent), 0644); err != nil {
			return fmt.Errorf("write skill file: %w", err)
		}
		output.Success("Created skill: %s", skillPath)

		// 2. Update .claude/settings.json to allow hops commands
		settingsPath := filepath.Join(home, ".claude", "settings.json")
		if err := updateSettings(settingsPath); err != nil {
			output.Info("Note: Could not update settings.json: %v", err)
			output.Info("Add 'Bash(hops *)' to your allowed commands manually")
		} else {
			output.Success("Updated permissions: %s", settingsPath)
		}

		// 3. Detect environment
		if os.Getenv("REST_ENDPOINT") != "" {
			output.Success("Detected Hopsworks terminal environment")
			output.Info("  Host: %s", os.Getenv("REST_ENDPOINT"))
			output.Info("  Project: %s", os.Getenv("PROJECT_NAME"))
		}

		output.Info("")
		output.Success("Done! Use /hops in Claude Code to get started.")
		output.Info("Claude can now run hops commands directly.")
		return nil
	},
}

func generateSkillContent() string {
	return `You have access to the hops CLI for interacting with Hopsworks Feature Store.
Use hops commands via Bash to explore and manage feature store resources.

## Setup
First, check if hops is configured:
` + "```bash" + `
hops project list
` + "```" + `

If not configured, the user needs to run:
` + "```bash" + `
hops login
hops project use <project-name>
` + "```" + `

## Getting Context
Always start by understanding what's available:
` + "```bash" + `
hops context
` + "```" + `

## Quick Reference

### Feature Groups
` + "```bash" + `
hops fg list                              # List all feature groups
hops fg info <name> [--version N]         # Show details + schema
hops fg preview <name> [--n 10]           # Preview data rows
hops fg features <name>                   # List features with types
hops fg create <name> --version 1 --primary-key id  # Create new
` + "```" + `

### Feature Views
` + "```bash" + `
hops fv list                              # List all feature views
hops fv info <name> [--version N]         # Show details
hops fv create <name> --version 1 --feature-group <fg-name>  # Create new
` + "```" + `

### Training Datasets
` + "```bash" + `
hops td list <fv-name> <fv-version>       # List training datasets
hops td create <fv-name> <fv-version>     # Create training dataset
` + "```" + `

### Other
` + "```bash" + `
hops project list                         # List projects
hops project use <name>                   # Switch project
hops fs list                              # List feature stores
hops job list                             # List jobs
hops dataset list [path]                  # Browse files
` + "```" + `

## Tips
- Use --json flag when you need to parse output programmatically
- Run hops context first to understand the current state
- Feature group names are case-sensitive
`
}

func updateSettings(path string) error {
	var settings map[string]interface{}

	data, err := os.ReadFile(path)
	if err != nil {
		// Create new settings
		settings = make(map[string]interface{})
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse settings: %w", err)
		}
	}

	// Get or create permissions.allow array
	permissions, ok := settings["permissions"].(map[string]interface{})
	if !ok {
		permissions = make(map[string]interface{})
	}

	allow, ok := permissions["allow"].([]interface{})
	if !ok {
		allow = []interface{}{}
	}

	// Check if hops is already allowed
	hopsRule := "Bash(hops *)"
	for _, rule := range allow {
		if ruleStr, ok := rule.(string); ok && strings.Contains(ruleStr, "hops") {
			return nil // Already configured
		}
	}

	allow = append(allow, hopsRule)
	permissions["allow"] = allow
	settings["permissions"] = permissions

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, out, 0644)
}

func init() {
	rootCmd.AddCommand(initCmd)
}
