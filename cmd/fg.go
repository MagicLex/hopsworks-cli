package cmd

import (
	"fmt"
	"strconv"

	"github.com/MagicLex/hopsworks-cli/pkg/client"
	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var fgVersion int
var fgPreviewN int

var fgCmd = &cobra.Command{
	Use:   "fg",
	Short: "Manage feature groups",
}

var fgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List feature groups",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		fgs, err := c.ListFeatureGroups()
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(fgs)
			return nil
		}

		headers := []string{"NAME", "VERSION", "ONLINE", "FEATURES", "DESCRIPTION"}
		var rows [][]string
		for _, fg := range fgs {
			online := "no"
			if fg.OnlineEnabled {
				online = "yes"
			}
			rows = append(rows, []string{
				fg.Name,
				strconv.Itoa(fg.Version),
				online,
				strconv.Itoa(len(fg.Features)),
				truncate(fg.Description, 40),
			})
		}
		output.Table(headers, rows)
		return nil
	},
}

var fgInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show feature group details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		fg, err := c.GetFeatureGroup(args[0], fgVersion)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(fg)
			return nil
		}

		output.Info("Feature Group: %s (v%d)", fg.Name, fg.Version)
		output.Info("ID: %d", fg.ID)
		output.Info("Online: %v", fg.OnlineEnabled)
		if fg.Description != "" {
			output.Info("Description: %s", fg.Description)
		}
		if fg.EventTime != "" {
			output.Info("Event Time: %s", fg.EventTime)
		}
		output.Info("")

		if len(fg.Features) > 0 {
			headers := []string{"FEATURE", "TYPE", "PRIMARY"}
			var rows [][]string
			for _, f := range fg.Features {
				pk := ""
				if f.Primary {
					pk = "yes"
				}
				rows = append(rows, []string{f.Name, f.Type, pk})
			}
			output.Table(headers, rows)
		}
		return nil
	},
}

var fgPreviewCmd = &cobra.Command{
	Use:   "preview <name>",
	Short: "Preview feature group data",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		fg, err := c.GetFeatureGroup(args[0], fgVersion)
		if err != nil {
			return err
		}

		rows, err := c.PreviewFeatureGroup(fg.ID, fgPreviewN)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(rows)
			return nil
		}

		if len(rows) == 0 {
			output.Info("No data in feature group")
			return nil
		}

		// Build table from first row's keys
		var headers []string
		for _, f := range fg.Features {
			headers = append(headers, f.Name)
		}
		// If no features metadata, use keys from first row
		if len(headers) == 0 {
			for k := range rows[0] {
				headers = append(headers, k)
			}
		}

		var tableRows [][]string
		for _, row := range rows {
			var r []string
			for _, h := range headers {
				r = append(r, fmt.Sprintf("%v", row[h]))
			}
			tableRows = append(tableRows, r)
		}
		output.Table(headers, tableRows)
		return nil
	},
}

var fgFeaturesCmd = &cobra.Command{
	Use:   "features <name>",
	Short: "List features with types",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		fg, err := c.GetFeatureGroup(args[0], fgVersion)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(fg.Features)
			return nil
		}

		headers := []string{"NAME", "TYPE", "PRIMARY", "DESCRIPTION"}
		var rows [][]string
		for _, f := range fg.Features {
			pk := ""
			if f.Primary {
				pk = "yes"
			}
			rows = append(rows, []string{f.Name, f.Type, pk, f.Description})
		}
		output.Table(headers, rows)
		return nil
	},
}

var (
	fgCreatePK       string
	fgCreateOnline   bool
	fgCreateEvtTime  string
	fgCreateDesc     string
	fgCreateFeatures string
	fgCreateFormat   string
)

var fgCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a feature group",
	Long: `Create a feature group with schema.

Examples:
  # Minimal (PK only, string types)
  hops fg create users --primary-key user_id

  # Full schema with types
  hops fg create transactions \
    --primary-key customer_id \
    --features "customer_id:bigint,age:bigint,total_spent:double,is_premium:boolean,event_time:timestamp" \
    --event-time event_time \
    --online \
    --description "Customer transaction features"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if fgVersion == 0 {
			fgVersion = 1
		}
		if fgCreatePK == "" {
			return fmt.Errorf("--primary-key is required")
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		pks := splitComma(fgCreatePK)
		pkSet := make(map[string]bool)
		for _, pk := range pks {
			pkSet[pk] = true
		}

		var features []client.Feature
		if fgCreateFeatures != "" {
			// Parse "name:type,name:type,..." format
			for _, spec := range splitComma(fgCreateFeatures) {
				parts := splitStr(trimSpace(spec), ":")
				name := trimSpace(parts[0])
				typ := "string"
				if len(parts) > 1 {
					typ = trimSpace(parts[1])
				}
				features = append(features, client.Feature{
					Name:    name,
					Type:    typ,
					Primary: pkSet[name],
				})
			}
		} else {
			// Minimal: just primary keys
			for _, pk := range pks {
				features = append(features, client.Feature{
					Name:    pk,
					Type:    "string",
					Primary: true,
				})
			}
		}

		req := &client.CreateFeatureGroupRequest{
			Name:             args[0],
			Version:          fgVersion,
			OnlineEnabled:    fgCreateOnline,
			EventTime:        fgCreateEvtTime,
			Description:      fgCreateDesc,
			Features:         features,
			TimeTravelFormat: fgCreateFormat,
		}

		fg, err := c.CreateFeatureGroup(req)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(fg)
			return nil
		}
		output.Success("Created feature group '%s' v%d (ID: %d)", fg.Name, fg.Version, fg.ID)
		return nil
	},
}

var fgDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a feature group",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if fgVersion == 0 {
			return fmt.Errorf("--version is required for delete")
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		fg, err := c.GetFeatureGroup(args[0], fgVersion)
		if err != nil {
			return err
		}

		if err := c.DeleteFeatureGroup(fg.ID); err != nil {
			return err
		}

		output.Success("Deleted feature group '%s' v%d", args[0], fgVersion)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(fgCmd)

	// Version flag for subcommands
	fgInfoCmd.Flags().IntVar(&fgVersion, "version", 0, "Feature group version (latest if omitted)")
	fgPreviewCmd.Flags().IntVar(&fgVersion, "version", 0, "Feature group version")
	fgPreviewCmd.Flags().IntVar(&fgPreviewN, "n", 10, "Number of rows to preview")
	fgFeaturesCmd.Flags().IntVar(&fgVersion, "version", 0, "Feature group version")
	fgCreateCmd.Flags().IntVar(&fgVersion, "version", 1, "Feature group version")
	fgCreateCmd.Flags().StringVar(&fgCreatePK, "primary-key", "", "Primary key columns (comma-separated)")
	fgCreateCmd.Flags().StringVar(&fgCreateFeatures, "features", "", "Feature schema: name:type,name:type,...")
	fgCreateCmd.Flags().BoolVar(&fgCreateOnline, "online", false, "Enable online storage")
	fgCreateCmd.Flags().StringVar(&fgCreateEvtTime, "event-time", "", "Event time column")
	fgCreateCmd.Flags().StringVar(&fgCreateDesc, "description", "", "Description")
	fgCreateCmd.Flags().StringVar(&fgCreateFormat, "format", "", "Time travel format: HUDI or DELTA (default: server decides)")
	fgDeleteCmd.Flags().IntVar(&fgVersion, "version", 0, "Feature group version to delete")

	fgCmd.AddCommand(fgListCmd)
	fgCmd.AddCommand(fgInfoCmd)
	fgCmd.AddCommand(fgPreviewCmd)
	fgCmd.AddCommand(fgFeaturesCmd)
	fgCmd.AddCommand(fgCreateCmd)
	fgCmd.AddCommand(fgDeleteCmd)
}

// Helper: create client with project validation
func mustClient() (*client.Client, error) {
	c, err := client.New(cfg)
	if err != nil {
		return nil, err
	}
	if cfg.ProjectID == 0 {
		return nil, fmt.Errorf("no project selected. Run 'hops project use <name>' first")
	}
	if cfg.FeatureStoreID == 0 {
		// Try to resolve
		if err := resolveFeatureStoreID(c); err != nil {
			return nil, fmt.Errorf("could not resolve feature store: %w", err)
		}
		cfg.Save()
	}
	return c, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func splitComma(s string) []string {
	var result []string
	for _, part := range splitStr(s, ",") {
		trimmed := trimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func splitStr(s, sep string) []string {
	var parts []string
	for {
		i := indexOf(s, sep)
		if i < 0 {
			parts = append(parts, s)
			break
		}
		parts = append(parts, s[:i])
		s = s[i+len(sep):]
	}
	return parts
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n') {
		s = s[:len(s)-1]
	}
	return s
}
