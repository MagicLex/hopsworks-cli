package cmd

import (
	"fmt"
	"strconv"

	"github.com/MagicLex/hopsworks-cli/pkg/client"
	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects",
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List accessible projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New(cfg)
		if err != nil {
			return err
		}
		projects, err := c.ListProjects()
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(projects)
			return nil
		}

		headers := []string{"NAME", "ID", "ACTIVE"}
		var rows [][]string
		for _, p := range projects {
			active := ""
			if p.ProjectName == cfg.Project {
				active = "*"
			}
			rows = append(rows, []string{
				p.ProjectName,
				strconv.Itoa(p.ProjectID),
				active,
			})
		}
		output.Table(headers, rows)
		return nil
	},
}

var projectUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set active project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		c, err := client.New(cfg)
		if err != nil {
			return err
		}

		project, err := c.GetProjectByName(name)
		if err != nil {
			return err
		}

		cfg.Project = project.ProjectName
		cfg.ProjectID = project.ProjectID

		// Resolve feature store ID
		if err := resolveFeatureStoreID(c); err != nil {
			return fmt.Errorf("resolve feature store: %w", err)
		}

		// Internal mode: don't persist (env vars are the source of truth)
		if !cfg.Internal {
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
		}

		output.Success("Active project: %s (ID: %d, FS: %d) [%s]", cfg.Project, cfg.ProjectID, cfg.FeatureStoreID, cfg.Mode())
		return nil
	},
}

var projectInfoCmd = &cobra.Command{
	Use:   "info [name]",
	Short: "Show project details",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.New(cfg)
		if err != nil {
			return err
		}

		var project *client.Project
		if len(args) > 0 {
			project, err = c.GetProjectByName(args[0])
		} else if cfg.ProjectID > 0 {
			project, err = c.GetProject(cfg.ProjectID)
		} else {
			return fmt.Errorf("no project specified. Use 'hops project use <name>' first")
		}
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(project)
			return nil
		}

		auth := "api-key"
		if cfg.Internal {
			auth = "jwt"
		}
		output.Table(
			[]string{"FIELD", "VALUE"},
			[][]string{
				{"Name", project.ProjectName},
				{"ID", strconv.Itoa(project.ProjectID)},
				{"Description", project.Description},
				{"Created", project.Created},
				{"Mode", cfg.Mode()},
				{"Auth", auth},
			},
		)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectUseCmd)
	projectCmd.AddCommand(projectInfoCmd)
}
