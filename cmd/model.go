package cmd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var modelVersion int

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Manage models in the model registry",
}

var modelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List models",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		models, err := c.ListModels()
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(models)
			return nil
		}

		headers := []string{"NAME", "VERSION", "FRAMEWORK", "CREATED", "DESCRIPTION"}
		var rows [][]string
		for _, m := range models {
			rows = append(rows, []string{
				m.Name,
				strconv.Itoa(m.Version),
				m.Framework,
				fmtEpochMs(m.Created),
				truncate(m.Description, 40),
			})
		}
		output.Table(headers, rows)
		return nil
	},
}

var modelInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show model details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		m, err := c.GetModel(args[0], modelVersion)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(m)
			return nil
		}

		output.Info("Model: %s (v%d)", m.Name, m.Version)
		output.Info("ID: %s", m.ID)
		if m.Framework != "" {
			output.Info("Framework: %s", m.Framework)
		}
		if m.Description != "" {
			output.Info("Description: %s", m.Description)
		}
		if m.UserFullName != "" {
			output.Info("Creator: %s", m.UserFullName)
		}
		if m.Created > 0 {
			output.Info("Created: %s", fmtEpochMs(m.Created))
		}

		if len(m.Metrics) > 0 {
			output.Info("")
			output.Info("Metrics:")
			headers := []string{"METRIC", "VALUE"}
			var rows [][]string
			for k, v := range m.Metrics {
				rows = append(rows, []string{k, fmt.Sprintf("%.6g", v)})
			}
			output.Table(headers, rows)
		}

		return nil
	},
}

var modelDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a model version",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if modelVersion == 0 {
			return fmt.Errorf("--version is required for delete")
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		m, err := c.GetModel(args[0], modelVersion)
		if err != nil {
			return err
		}

		if err := c.DeleteModel(m.ID); err != nil {
			return err
		}

		output.Success("Deleted model '%s' v%d", args[0], modelVersion)
		return nil
	},
}

func fmtEpochMs(ms int64) string {
	if ms == 0 {
		return "-"
	}
	return time.UnixMilli(ms).Format("2006-01-02 15:04")
}

func init() {
	rootCmd.AddCommand(modelCmd)

	modelInfoCmd.Flags().IntVar(&modelVersion, "version", 0, "Model version (latest if omitted)")
	modelDeleteCmd.Flags().IntVar(&modelVersion, "version", 0, "Model version to delete")

	modelCmd.AddCommand(modelListCmd)
	modelCmd.AddCommand(modelInfoCmd)
	modelCmd.AddCommand(modelDeleteCmd)
}
