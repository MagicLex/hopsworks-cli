package cmd

import (
	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for updates and improvements",
	Long: `Check the hops CLI against the latest Hopsworks API surface.

Future: will identify gaps between CLI commands and available API endpoints,
and suggest or auto-generate new command stubs.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		output.Info("hops CLI v0.1.0")
		output.Info("")
		output.Info("Current commands: login, project, fs, fg, fv, td, job, dataset, init, context")
		output.Info("")
		output.Info("Planned improvements:")
		output.Info("  - hops fg insert (data ingestion)")
		output.Info("  - hops fg stats (compute/view statistics)")
		output.Info("  - hops fv serve (online feature serving)")
		output.Info("  - hops model list/info/deploy (model registry)")
		output.Info("  - hops deployment list/create (model serving)")
		output.Info("  - Auto-update from GitHub releases")
		output.Info("")
		output.Info("Run 'hops update' again after updating the binary.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
