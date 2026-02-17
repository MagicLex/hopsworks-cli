package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var fvVersion int
var fvPreviewN int

var fvCmd = &cobra.Command{
	Use:   "fv",
	Short: "Manage feature views",
}

var fvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List feature views",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		fvs, err := c.ListFeatureViews()
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(fvs)
			return nil
		}

		headers := []string{"NAME", "VERSION", "FEATURES", "DESCRIPTION"}
		var rows [][]string
		for _, fv := range fvs {
			rows = append(rows, []string{
				fv.Name,
				strconv.Itoa(fv.Version),
				strconv.Itoa(len(fv.Features)),
				truncate(fv.Description, 40),
			})
		}
		output.Table(headers, rows)
		return nil
	},
}

var fvInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show feature view details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		fv, err := c.GetFeatureView(args[0], fvVersion)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(fv)
			return nil
		}

		output.Info("Feature View: %s (v%d)", fv.Name, fv.Version)
		output.Info("ID: %d", fv.ID)
		if fv.Description != "" {
			output.Info("Description: %s", fv.Description)
		}
		if len(fv.Labels) > 0 {
			output.Info("Labels: %s", strings.Join(fv.Labels, ", "))
		}
		output.Info("")

		if len(fv.Features) > 0 {
			headers := []string{"FEATURE", "TYPE"}
			var rows [][]string
			for _, f := range fv.Features {
				rows = append(rows, []string{f.Name, f.Type})
			}
			output.Table(headers, rows)
		}
		return nil
	},
}

var (
	fvCreateFG       string
	fvCreateFGVer    int
	fvCreateFeatures string
	fvCreateLabels   string
	fvCreateDesc     string
)

var fvCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a feature view",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if fvCreateFG == "" {
			return fmt.Errorf("--feature-group is required")
		}
		if fvVersion == 0 {
			fvVersion = 1
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		// Resolve feature group ID
		fg, err := c.GetFeatureGroup(fvCreateFG, fvCreateFGVer)
		if err != nil {
			return fmt.Errorf("feature group '%s' not found: %w", fvCreateFG, err)
		}

		var features []string
		if fvCreateFeatures != "" {
			features = splitComma(fvCreateFeatures)
		} else {
			// Default: include all features from the FG
			for _, f := range fg.Features {
				features = append(features, f.Name)
			}
		}

		var labels []string
		if fvCreateLabels != "" {
			labels = splitComma(fvCreateLabels)
		}

		fv, err := c.CreateFeatureView(args[0], fvVersion, fvCreateDesc, fg, features, labels)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(fv)
			return nil
		}
		output.Success("Created feature view '%s' v%d (ID: %d)", fv.Name, fv.Version, fv.ID)
		return nil
	},
}

var fvDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a feature view",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		if err := c.DeleteFeatureView(args[0], fvVersion); err != nil {
			return err
		}

		if fvVersion > 0 {
			output.Success("Deleted feature view '%s' v%d", args[0], fvVersion)
		} else {
			output.Success("Deleted all versions of feature view '%s'", args[0])
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(fvCmd)

	fvInfoCmd.Flags().IntVar(&fvVersion, "version", 0, "Feature view version")
	fvCreateCmd.Flags().IntVar(&fvVersion, "version", 1, "Feature view version")
	fvCreateCmd.Flags().StringVar(&fvCreateFG, "feature-group", "", "Source feature group name")
	fvCreateCmd.Flags().IntVar(&fvCreateFGVer, "fg-version", 0, "Source feature group version")
	fvCreateCmd.Flags().StringVar(&fvCreateFeatures, "features", "", "Selected features (comma-separated)")
	fvCreateCmd.Flags().StringVar(&fvCreateLabels, "labels", "", "Label columns (comma-separated)")
	fvCreateCmd.Flags().StringVar(&fvCreateDesc, "description", "", "Description")
	fvDeleteCmd.Flags().IntVar(&fvVersion, "version", 0, "Version to delete (all if omitted)")

	fvCmd.AddCommand(fvListCmd)
	fvCmd.AddCommand(fvInfoCmd)
	fvCmd.AddCommand(fvCreateCmd)
	fvCmd.AddCommand(fvDeleteCmd)
}
