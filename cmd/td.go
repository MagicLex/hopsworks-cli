package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var tdCmd = &cobra.Command{
	Use:   "td",
	Short: "Manage training datasets",
}

var tdListCmd = &cobra.Command{
	Use:   "list <fv-name> <fv-version>",
	Short: "List training datasets for a feature view",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fvVer, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid version: %s", args[1])
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		tds, err := c.ListTrainingDatasets(args[0], fvVer)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(tds)
			return nil
		}

		headers := []string{"VERSION", "FORMAT", "DESCRIPTION", "CREATED"}
		var rows [][]string
		for _, td := range tds {
			rows = append(rows, []string{
				strconv.Itoa(td.Version),
				td.DataFormat,
				truncate(td.Description, 30),
				td.Created,
			})
		}
		output.Table(headers, rows)
		return nil
	},
}

var (
	tdCreateDesc   string
	tdCreateFormat string
)

var tdCreateCmd = &cobra.Command{
	Use:   "create <fv-name> <fv-version>",
	Short: "Create a training dataset",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fvVer, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid version: %s", args[1])
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		td, err := c.CreateTrainingDataset(args[0], fvVer, tdCreateDesc, tdCreateFormat)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(td)
			return nil
		}
		output.Success("Created training dataset v%d for '%s' v%d", td.Version, args[0], fvVer)
		return nil
	},
}

var tdDeleteCmd = &cobra.Command{
	Use:   "delete <fv-name> <fv-version> <td-version>",
	Short: "Delete a training dataset",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		fvVer, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid fv version: %s", args[1])
		}
		tdVer, err := strconv.Atoi(args[2])
		if err != nil {
			return fmt.Errorf("invalid td version: %s", args[2])
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		if err := c.DeleteTrainingDataset(args[0], fvVer, tdVer); err != nil {
			return err
		}

		output.Success("Deleted training dataset v%d from '%s' v%d", tdVer, args[0], fvVer)
		return nil
	},
}

// --- td compute ---

var (
	tdComputeDesc   string
	tdComputeFormat string
	tdComputeSplit  string
)

var tdComputeCmd = &cobra.Command{
	Use:   "compute <fv-name> <fv-version>",
	Short: "Materialize training data (Spark job)",
	Long: `Materialize training data from a feature view.

Examples:
  hops td compute my_view 1
  hops td compute my_view 1 --format csv
  hops td compute my_view 1 --split "train:0.8,test:0.2"
  hops td compute my_view 1 --description "v1 training set"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fvVer, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid version: %s", args[1])
		}

		if !output.JSONMode {
			output.Info("Materializing training data from '%s' v%d...", args[0], fvVer)
		}

		script := buildTDComputeScript(args[0], fvVer, tdComputeFormat, tdComputeDesc, tdComputeSplit)
		if err := runPython(script); err != nil {
			return fmt.Errorf("training data materialization failed: %w", err)
		}
		return nil
	},
}

func buildTDComputeScript(fvName string, fvVer int, format, desc, split string) string {
	var sb strings.Builder
	sb.WriteString(buildFVPreamble(fvName, fvVer))

	if split != "" {
		// Parse split spec: "train:0.8,test:0.2" or "train:0.7,validation:0.15,test:0.15"
		testSize, valSize := parseSplitSpec(split)

		if valSize > 0 {
			sb.WriteString(fmt.Sprintf(`
td_version, job = fv.create_train_validation_test_split(
    validation_size=%.4f,
    test_size=%.4f,
    data_format=%q,
    description=%q,
    write_options={"wait_for_job": False},
)
`, valSize, testSize, format, desc))
		} else {
			sb.WriteString(fmt.Sprintf(`
td_version, job = fv.create_train_test_split(
    test_size=%.4f,
    data_format=%q,
    description=%q,
    write_options={"wait_for_job": False},
)
`, testSize, format, desc))
		}
	} else {
		sb.WriteString(fmt.Sprintf(`
td_version, job = fv.create_training_data(
    data_format=%q,
    description=%q,
    write_options={"wait_for_job": False},
)
`, format, desc))
	}

	sb.WriteString(`
print(f"Training dataset version: {td_version}", file=sys.stderr)
if job and hasattr(job, 'name'):
    print(f"Job: {job.name}", file=sys.stderr)
    print(f"Poll with: hops job status {job.name} --wait", file=sys.stderr)
else:
    print("Materialized (no Spark job needed)", file=sys.stderr)
`)

	return sb.String()
}

// parseSplitSpec parses "train:0.8,test:0.2" into (testSize, validationSize).
func parseSplitSpec(spec string) (testSize, valSize float64) {
	for _, part := range strings.Split(spec, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), ":", 2)
		if len(kv) != 2 {
			continue
		}
		name := strings.TrimSpace(kv[0])
		val := 0.0
		fmt.Sscanf(strings.TrimSpace(kv[1]), "%f", &val)

		switch strings.ToLower(name) {
		case "test":
			testSize = val
		case "validation", "val":
			valSize = val
		}
	}
	return
}

// --- td read ---

var (
	tdReadVersion int
	tdReadOutput  string
	tdReadSplit   string
)

var tdReadCmd = &cobra.Command{
	Use:   "read <fv-name> <fv-version>",
	Short: "Read training dataset data",
	Long: `Read a materialized training dataset.

Examples:
  hops td read my_view 1 --td-version 1
  hops td read my_view 1 --td-version 1 --output train.parquet
  hops td read my_view 1 --td-version 1 --split train --output train.csv
  hops td read my_view 1 --td-version 1 --split test --output test.csv`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fvVer, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid version: %s", args[1])
		}
		if tdReadVersion == 0 {
			return fmt.Errorf("--td-version is required")
		}

		if !output.JSONMode {
			output.Info("Reading training data from '%s' v%d (TD v%d)...", args[0], fvVer, tdReadVersion)
		}

		script := buildTDReadScript(args[0], fvVer, tdReadVersion, tdReadOutput, tdReadSplit, output.JSONMode)
		if err := runPython(script); err != nil {
			return fmt.Errorf("training data read failed: %w", err)
		}
		return nil
	},
}

func buildTDReadScript(fvName string, fvVer, tdVer int, outputPath, split string, jsonMode bool) string {
	var sb strings.Builder
	sb.WriteString(buildFVPreamble(fvName, fvVer))

	if split != "" {
		// Read specific split
		sb.WriteString(fmt.Sprintf(`
X_train, X_test, y_train, y_test = fv.get_train_test_split(training_dataset_version=%d)
`, tdVer))
		switch strings.ToLower(split) {
		case "train":
			sb.WriteString("X, y = X_train, y_train\n")
		case "test":
			sb.WriteString("X, y = X_test, y_test\n")
		default:
			sb.WriteString(fmt.Sprintf("raise ValueError('Unknown split: %s. Use train or test.')\n", split))
		}
	} else {
		sb.WriteString(fmt.Sprintf(`
X, y = fv.get_training_data(training_dataset_version=%d)
`, tdVer))
	}

	// Combine X and y into one DataFrame
	sb.WriteString(`
if y is not None and len(y.columns) > 0:
    df = pd.concat([X, y], axis=1)
else:
    df = X
print(f'Read {len(df)} rows, {len(df.columns)} columns', file=sys.stderr)
`)

	if outputPath != "" {
		ext := strings.ToLower(outputPath)
		if strings.HasSuffix(ext, ".csv") {
			sb.WriteString(fmt.Sprintf("df.to_csv(%q, index=False)\n", outputPath))
		} else if strings.HasSuffix(ext, ".json") {
			sb.WriteString(fmt.Sprintf("df.to_json(%q, orient='records', indent=2)\n", outputPath))
		} else {
			sb.WriteString(fmt.Sprintf("df.to_parquet(%q, index=False)\n", outputPath))
		}
		sb.WriteString(fmt.Sprintf("print('Saved to %s', file=sys.stderr)\n", outputPath))
	} else if jsonMode {
		sb.WriteString("print(df.to_json(orient='records', indent=2))\n")
	} else {
		sb.WriteString("print(df.to_string(index=False))\n")
	}

	return sb.String()
}

func init() {
	rootCmd.AddCommand(tdCmd)

	tdCreateCmd.Flags().StringVar(&tdCreateDesc, "description", "", "Description")
	tdCreateCmd.Flags().StringVar(&tdCreateFormat, "format", "parquet", "Data format (parquet, csv, tfrecord)")

	tdComputeCmd.Flags().StringVar(&tdComputeFormat, "format", "parquet", "Data format (parquet, csv, tfrecord)")
	tdComputeCmd.Flags().StringVar(&tdComputeDesc, "description", "", "Description")
	tdComputeCmd.Flags().StringVar(&tdComputeSplit, "split", "", `Split spec: "train:0.8,test:0.2"`)

	tdReadCmd.Flags().IntVar(&tdReadVersion, "td-version", 0, "Training dataset version (required)")
	tdReadCmd.Flags().StringVar(&tdReadOutput, "output", "", "Save to file (.parquet, .csv, .json)")
	tdReadCmd.Flags().StringVar(&tdReadSplit, "split", "", "Read specific split (train, test)")

	tdCmd.AddCommand(tdListCmd)
	tdCmd.AddCommand(tdCreateCmd)
	tdCmd.AddCommand(tdDeleteCmd)
	tdCmd.AddCommand(tdComputeCmd)
	tdCmd.AddCommand(tdReadCmd)
}
