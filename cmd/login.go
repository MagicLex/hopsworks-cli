package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/MagicLex/hopsworks-cli/pkg/client"
	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Hopsworks",
	Long:  `Login to a Hopsworks instance. Validates the API key and saves config to ~/.hops/config.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.Internal {
			output.Success("Internal mode — already authenticated via JWT")
			output.Info("  Host: %s", cfg.Host)
			output.Info("  Project: %s", cfg.Project)
			return nil
		}

		// Prompt for host if not set
		if cfg.Host == "" {
			fmt.Print("Hopsworks host (e.g. app.hopsworks.ai): ")
			reader := bufio.NewReader(os.Stdin)
			host, _ := reader.ReadString('\n')
			cfg.Host = strings.TrimSpace(host)
		}

		// Prompt for API key if not set
		if cfg.APIKey == "" {
			fmt.Print("API key: ")
			reader := bufio.NewReader(os.Stdin)
			key, _ := reader.ReadString('\n')
			cfg.APIKey = strings.TrimSpace(key)
		}

		// Validate by fetching projects
		c, err := client.New(cfg)
		if err != nil {
			return err
		}

		data, err := c.Get("/hopsworks-api/api/project")
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		var projects []json.RawMessage
		if err := json.Unmarshal(data, &projects); err != nil {
			return fmt.Errorf("unexpected response: %w", err)
		}

		// Save config
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		output.Success("Logged in to %s (%d projects accessible)", cfg.Host, len(projects))

		// If no project set and there's exactly one, auto-select it
		if cfg.Project == "" && len(projects) > 0 {
			var first struct {
				ProjectName string `json:"projectName"`
				ProjectId   int    `json:"projectId"`
			}
			json.Unmarshal(projects[0], &first)
			if len(projects) == 1 {
				cfg.Project = first.ProjectName
				cfg.ProjectID = first.ProjectId
				// Fetch feature store ID
				if err := resolveFeatureStoreID(c); err == nil {
					cfg.Save()
				}
				output.Success("Auto-selected project: %s", cfg.Project)
			} else {
				output.Info("Run 'hops project use <name>' to select a project")
			}
		}

		return nil
	},
}

func resolveFeatureStoreID(c *client.Client) error {
	data, err := c.Get(fmt.Sprintf("/hopsworks-api/api/project/%d/featurestores", cfg.ProjectID))
	if err != nil {
		return err
	}
	var stores []struct {
		FeaturestoreId int    `json:"featurestoreId"`
		Name           string `json:"featurestoreName"`
	}
	if err := json.Unmarshal(data, &stores); err != nil {
		return err
	}
	for _, s := range stores {
		if strings.Contains(s.Name, cfg.Project) || len(stores) == 1 {
			cfg.FeatureStoreID = s.FeaturestoreId
			return nil
		}
	}
	if len(stores) > 0 {
		cfg.FeatureStoreID = stores[0].FeaturestoreId
	}
	return nil
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
