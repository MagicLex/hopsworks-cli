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
var fgStatsFeatures string
var fgStatsCompute bool

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

		if fg.EmbeddingIndex != nil && len(fg.EmbeddingIndex.Features) > 0 {
			output.Info("")
			output.Info("Embeddings:")
			headers := []string{"COLUMN", "DIMENSION", "METRIC"}
			var rows [][]string
			for _, ef := range fg.EmbeddingIndex.Features {
				rows = append(rows, []string{ef.Name, strconv.Itoa(ef.Dimension), ef.SimilarityFunctionType})
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
	fgCreatePK         string
	fgCreateOnline     bool
	fgCreateEvtTime    string
	fgCreateDesc       string
	fgCreateFeatures   string
	fgCreateFormat     string
	fgCreateEmbeddings []string
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
    --description "Customer transaction features"

  # With embeddings (auto-enables online store)
  hops fg create documents \
    --primary-key doc_id \
    --features "doc_id:bigint,title:string" \
    --embedding "text_embedding:384:cosine"`,
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

		// Parse embeddings
		var embeddingIndex *client.EmbeddingIndex
		if len(fgCreateEmbeddings) > 0 {
			embFeatures, embSchemaFeatures, err := parseEmbeddingSpecs(fgCreateEmbeddings)
			if err != nil {
				return err
			}

			// Merge embedding columns into feature list (skip if already declared)
			existing := make(map[string]bool)
			for _, f := range features {
				existing[f.Name] = true
			}
			for _, ef := range embSchemaFeatures {
				if !existing[ef.Name] {
					features = append(features, ef)
				}
			}

			embeddingIndex = &client.EmbeddingIndex{Features: embFeatures}
			fgCreateOnline = true // embeddings require online store
		}

		req := &client.CreateFeatureGroupRequest{
			Name:             args[0],
			Version:          fgVersion,
			OnlineEnabled:    fgCreateOnline,
			EventTime:        fgCreateEvtTime,
			Description:      fgCreateDesc,
			Features:         features,
			TimeTravelFormat: fgCreateFormat,
			EmbeddingIndex:   embeddingIndex,
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

var fgStatsCmd = &cobra.Command{
	Use:   "stats <name>",
	Short: "Show or compute feature group statistics",
	Long: `Show computed statistics for a feature group, or trigger computation.

Examples:
  # Show latest stats
  hops fg stats transactions

  # Filter to specific features
  hops fg stats transactions --features amount,age

  # Trigger stats computation (Spark job)
  hops fg stats transactions --compute`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		fg, err := c.GetFeatureGroup(args[0], fgVersion)
		if err != nil {
			return err
		}

		if fgStatsCompute {
			job, err := c.ComputeFeatureGroupStatistics(fg.ID)
			if err != nil {
				return err
			}
			if output.JSONMode {
				output.PrintJSON(job)
				return nil
			}
			output.Success("Statistics computation job started (job: %s, id: %d)", job.Name, job.ID)
			return nil
		}

		var featureNames []string
		if fgStatsFeatures != "" {
			featureNames = splitComma(fgStatsFeatures)
		}

		stats, err := c.GetFeatureGroupStatistics(fg.ID, featureNames)
		if err != nil {
			return err
		}

		if stats == nil {
			if output.JSONMode {
				output.PrintJSON(struct{}{})
				return nil
			}
			output.Info("No statistics computed for '%s' v%d. Use --compute to trigger.", fg.Name, fg.Version)
			return nil
		}

		if output.JSONMode {
			output.PrintJSON(stats)
			return nil
		}

		if stats.ComputationTime != nil {
			output.Info("Statistics for '%s' v%d (computed: %d)", fg.Name, fg.Version, *stats.ComputationTime)
		} else {
			output.Info("Statistics for '%s' v%d", fg.Name, fg.Version)
		}
		output.Info("")

		if len(stats.FeatureDescriptiveStatistics) == 0 {
			output.Info("No feature statistics available")
			return nil
		}

		headers := []string{"FEATURE", "TYPE", "COUNT", "MEAN", "MIN", "MAX", "STDDEV", "NULLS", "COMPLETENESS"}
		var rows [][]string
		for _, fs := range stats.FeatureDescriptiveStatistics {
			rows = append(rows, []string{
				fs.FeatureName,
				fs.FeatureType,
				fmtInt64(fs.Count),
				fmtFloat64(fs.Mean),
				fmtFloat64(fs.Min),
				fmtFloat64(fs.Max),
				fmtFloat64(fs.Stddev),
				fmtInt64(fs.NumNullValues),
				fmtFloat32Pct(fs.Completeness),
			})
		}
		output.Table(headers, rows)
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

// parseEmbeddingSpecs parses --embedding flags into EmbeddingFeatures + schema Features.
// Format: "col_name:dimension[:metric]" where metric is l2, cosine, or dot_product.
func parseEmbeddingSpecs(specs []string) ([]client.EmbeddingFeature, []client.Feature, error) {
	var embFeatures []client.EmbeddingFeature
	var schemaFeatures []client.Feature

	for _, spec := range specs {
		parts := splitStr(trimSpace(spec), ":")
		if len(parts) < 2 || len(parts) > 3 {
			return nil, nil, fmt.Errorf("invalid --embedding %q: expected \"name:dimension[:metric]\"", spec)
		}

		name := trimSpace(parts[0])
		dim := 0
		if _, err := fmt.Sscanf(trimSpace(parts[1]), "%d", &dim); err != nil || dim <= 0 {
			return nil, nil, fmt.Errorf("invalid dimension in --embedding %q: must be positive integer", spec)
		}

		metric := "l2_norm"
		if len(parts) == 3 {
			switch trimSpace(parts[2]) {
			case "l2", "l2_norm":
				metric = "l2_norm"
			case "cosine":
				metric = "cosine"
			case "dot_product":
				metric = "dot_product"
			default:
				return nil, nil, fmt.Errorf("invalid metric in --embedding %q: must be l2, cosine, or dot_product", spec)
			}
		}

		embFeatures = append(embFeatures, client.EmbeddingFeature{
			Name:                   name,
			Dimension:              dim,
			SimilarityFunctionType: metric,
		})
		schemaFeatures = append(schemaFeatures, client.Feature{
			Name: name,
			Type: "array<float>",
		})
	}
	return embFeatures, schemaFeatures, nil
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
	fgCreateCmd.Flags().StringVar(&fgCreateFormat, "format", "DELTA", "Time travel format: DELTA or NONE")
	fgCreateCmd.Flags().StringArrayVar(&fgCreateEmbeddings, "embedding", nil, `Embedding column: "name:dimension[:metric]" (l2, cosine, dot_product)`)
	fgDeleteCmd.Flags().IntVar(&fgVersion, "version", 0, "Feature group version to delete")
	fgStatsCmd.Flags().IntVar(&fgVersion, "version", 0, "Feature group version")
	fgStatsCmd.Flags().StringVar(&fgStatsFeatures, "features", "", "Filter to specific features (comma-separated)")
	fgStatsCmd.Flags().BoolVar(&fgStatsCompute, "compute", false, "Trigger statistics computation (Spark job)")

	fgCmd.AddCommand(fgListCmd)
	fgCmd.AddCommand(fgInfoCmd)
	fgCmd.AddCommand(fgPreviewCmd)
	fgCmd.AddCommand(fgFeaturesCmd)
	fgCmd.AddCommand(fgCreateCmd)
	fgCmd.AddCommand(fgDeleteCmd)
	fgCmd.AddCommand(fgStatsCmd)
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

func fmtFloat64(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.4g", *v)
}

func fmtFloat32Pct(v *float32) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", *v*100)
}

func fmtInt64(v *int64) string {
	if v == nil {
		return "-"
	}
	return strconv.FormatInt(*v, 10)
}
