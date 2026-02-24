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
	tdComputeDesc      string
	tdComputeFormat    string
	tdComputeSplit     string
	tdComputeFilter    string
	tdComputeStartTime string
	tdComputeEndTime   string
)

var tdComputeCmd = &cobra.Command{
	Use:   "compute <fv-name> <fv-version>",
	Short: "Materialize training data (Spark job)",
	Long: `Materialize training data from a feature view.

Examples:
  hops td compute my_view 1
  hops td compute my_view 1 --format csv
  hops td compute my_view 1 --split "train:0.8,test:0.2"
  hops td compute my_view 1 --filter "price > 100"
  hops td compute my_view 1 --filter "price > 50 AND product == Laptop"
  hops td compute my_view 1 --start-time "2026-01-01" --end-time "2026-02-01"
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

		script := buildTDComputeScript(args[0], fvVer, tdComputeFormat, tdComputeDesc, tdComputeSplit, tdComputeFilter, tdComputeStartTime, tdComputeEndTime)
		if err := runPython(script); err != nil {
			return fmt.Errorf("materialize training data: %w", err)
		}
		return nil
	},
}

func buildTDComputeScript(fvName string, fvVer int, format, desc, split, filter, startTime, endTime string) string {
	var sb strings.Builder
	sb.WriteString(buildFVPreamble(fvName, fvVer))

	// Build extra_filter from --filter flag
	if filter != "" {
		sb.WriteString(buildFilterSnippet(filter))
	}

	// Common kwargs for all create methods
	var kwargs []string
	kwargs = append(kwargs, fmt.Sprintf("data_format=%q", format))
	kwargs = append(kwargs, fmt.Sprintf("description=%q", desc))
	if startTime != "" {
		kwargs = append(kwargs, fmt.Sprintf("start_time=%q", startTime))
	}
	if endTime != "" {
		kwargs = append(kwargs, fmt.Sprintf("end_time=%q", endTime))
	}
	if filter != "" {
		kwargs = append(kwargs, "extra_filter=extra_filter")
	}
	kwargs = append(kwargs, `write_options={"wait_for_job": False}`)
	kwargsStr := strings.Join(kwargs, ",\n    ")

	if split != "" {
		// Parse split spec: "train:0.8,test:0.2" or "train:0.7,validation:0.15,test:0.15"
		testSize, valSize := parseSplitSpec(split)

		if valSize > 0 {
			sb.WriteString(fmt.Sprintf(`
td_version, job = fv.create_train_validation_test_split(
    validation_size=%.4f,
    test_size=%.4f,
    %s,
)
`, valSize, testSize, kwargsStr))
		} else {
			sb.WriteString(fmt.Sprintf(`
td_version, job = fv.create_train_test_split(
    test_size=%.4f,
    %s,
)
`, testSize, kwargsStr))
		}
	} else {
		sb.WriteString(fmt.Sprintf(`
td_version, job = fv.create_training_data(
    %s,
)
`, kwargsStr))
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

// buildFilterSnippet generates Python code that builds an hsfs Filter from a simple expression.
// Uses fv.query.featuregroups to get Feature objects with proper FG references.
// Supports: "price > 100", "price > 50 AND product == Laptop", "status != failed OR price >= 10"
func buildFilterSnippet(filter string) string {
	var sb strings.Builder
	// Build a lookup dict of feature name → Feature object from the FV's query
	sb.WriteString(`
_fg_features = {}
for _fg in fv.query.featuregroups:
    for _feat in _fg.features:
        _fg_features[_feat.name.lower()] = _fg[_feat.name]
`)
	// Split on AND/OR to support compound filters
	parts := splitFilterExpression(filter)
	for i, part := range parts {
		sb.WriteString(fmt.Sprintf("_f%d = _fg_features[%q] %s %s\n", i, strings.ToLower(part.feature), part.op, part.value))
	}

	// Combine with AND/OR
	sb.WriteString("extra_filter = _f0\n")
	for i := 1; i < len(parts); i++ {
		if strings.EqualFold(parts[i].conjunction, "OR") {
			sb.WriteString(fmt.Sprintf("extra_filter = extra_filter | _f%d\n", i))
		} else {
			sb.WriteString(fmt.Sprintf("extra_filter = extra_filter & _f%d\n", i))
		}
	}
	return sb.String()
}

type filterPart struct {
	conjunction string // AND or OR (empty for first)
	feature     string
	op          string
	value       string
}

// splitFilterExpression splits "price > 100 AND product == Laptop" into parts.
func splitFilterExpression(expr string) []filterPart {
	// Tokenize by splitting on AND/OR boundaries
	tokens := strings.Fields(expr)
	var parts []filterPart
	var conj string
	i := 0
	for i < len(tokens) {
		upper := strings.ToUpper(tokens[i])
		if upper == "AND" || upper == "OR" {
			conj = upper
			i++
			continue
		}
		// Expect: feature op value
		if i+2 >= len(tokens) {
			break
		}
		feature := tokens[i]
		op := tokens[i+1]
		value := tokens[i+2]

		// Map user-friendly ops to Python ops
		switch op {
		case "=":
			op = "=="
		case "!=":
			// keep as !=
		}

		// Quote string values (non-numeric)
		if !isNumeric(value) {
			value = fmt.Sprintf("%q", value)
		}

		parts = append(parts, filterPart{conjunction: conj, feature: feature, op: op, value: value})
		conj = ""
		i += 3
	}
	if len(parts) == 0 {
		// Fallback: treat entire expression as a single condition
		parts = append(parts, filterPart{feature: expr, op: "!=", value: "None"})
	}
	return parts
}

func isNumeric(s string) bool {
	for i, c := range s {
		if c == '-' && i == 0 {
			continue
		}
		if c == '.' {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
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
			return fmt.Errorf("read training data: %w", err)
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

// --- td stats ---

var (
	tdStatsVersion  int
	tdStatsFeatures string
	tdStatsCompute  bool
)

var tdStatsCmd = &cobra.Command{
	Use:   "stats <fv-name> <fv-version>",
	Short: "Show or compute training dataset statistics",
	Long: `Show computed statistics for a training dataset, or trigger computation.

Examples:
  # Show latest stats
  hops td stats snowflake_customer_orders 1 --td-version 4

  # Filter to specific features
  hops td stats my_view 1 --td-version 1 --features amount,age

  # Trigger stats computation
  hops td stats my_view 1 --td-version 1 --compute`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fvVer, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid version: %s", args[1])
		}
		if tdStatsVersion == 0 {
			return fmt.Errorf("--td-version is required")
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		if tdStatsCompute {
			if !output.JSONMode {
				output.Info("Computing statistics for '%s' v%d TD v%d...", args[0], fvVer, tdStatsVersion)
			}
			script := buildTDStatsComputeScript(args[0], fvVer, tdStatsVersion)
			rawOutput, err := runPythonCapture(script)
			if err != nil {
				return fmt.Errorf("compute statistics: %w", err)
			}
			// Extract JSON object from output (skip SDK log lines)
			statsJSON := extractJSON(rawOutput)
			if statsJSON == nil {
				return fmt.Errorf("no statistics JSON in Python output")
			}
			// Register via Go client
			if err := c.RegisterTrainingDatasetStatistics(args[0], fvVer, tdStatsVersion, statsJSON); err != nil {
				return fmt.Errorf("register statistics: %w", err)
			}
			if !output.JSONMode {
				output.Success("Statistics computed and registered for '%s' v%d TD v%d", args[0], fvVer, tdStatsVersion)
			}
			return nil
		}

		var featureNames []string
		if tdStatsFeatures != "" {
			featureNames = splitComma(tdStatsFeatures)
		}

		stats, err := c.GetTrainingDatasetStatistics(args[0], fvVer, tdStatsVersion, featureNames)
		if err != nil {
			return err
		}

		if stats == nil {
			if output.JSONMode {
				output.PrintJSON(struct{}{})
				return nil
			}
			output.Info("No statistics computed for TD v%d. Use --compute to trigger.", tdStatsVersion)
			return nil
		}

		if output.JSONMode {
			output.PrintJSON(stats)
			return nil
		}

		if stats.ComputationTime != nil {
			output.Info("Statistics for '%s' v%d TD v%d (computed: %d)", args[0], fvVer, tdStatsVersion, *stats.ComputationTime)
		} else {
			output.Info("Statistics for '%s' v%d TD v%d", args[0], fvVer, tdStatsVersion)
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

// extractJSON finds the first line starting with '{' in raw output and returns it as bytes.
func extractJSON(raw []byte) []byte {
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "{") {
			return []byte(trimmed)
		}
	}
	return nil
}

func buildTDStatsComputeScript(fvName string, fvVer, tdVer int) string {
	return fmt.Sprintf(`import hopsworks, warnings, logging, sys, os, contextlib
warnings.filterwarnings("ignore")
logging.getLogger("hsfs").setLevel(logging.WARNING)
logging.getLogger("hopsworks").setLevel(logging.WARNING)

# Suppress SDK login messages from stdout
with contextlib.redirect_stdout(open(os.devnull, "w")):
    project = hopsworks.login()
    fs = project.get_feature_store()
fv = fs.get_feature_view(name=%q, version=%d)

# Read materialized TD data
X, y = fv.get_training_data(training_dataset_version=%d)
import pandas as pd
if y is not None and len(y.columns) > 0:
    df = pd.concat([X, y], axis=1)
else:
    df = X

print(f"Read {len(df)} rows, {len(df.columns)} columns for stats", file=sys.stderr)

# Compute descriptive statistics
import json, time
from hsfs.core.statistics_api import StatisticsApi

stats_list = []
for col in df.columns:
    s = df[col]
    # Arrow Flight returns some numeric columns (e.g. DECIMAL) as object/strings — coerce them
    if s.dtype == "object":
        coerced = pd.to_numeric(s, errors="coerce")
        if coerced.notna().sum() > 0:
            s = coerced
            df[col] = s
    fs_dict = {"featureName": col, "featureType": str(s.dtype)}
    fs_dict["count"] = int(s.count())
    fs_dict["numNullValues"] = int(s.isna().sum())
    fs_dict["completeness"] = float(s.count() / len(s)) if len(s) > 0 else 0.0
    if s.dtype in ("int64", "float64", "int32", "float32"):
        fs_dict["min"] = float(s.min()) if s.count() > 0 else None
        fs_dict["max"] = float(s.max()) if s.count() > 0 else None
        fs_dict["mean"] = float(s.mean()) if s.count() > 0 else None
        fs_dict["stddev"] = float(s.std()) if s.count() > 0 else None
        fs_dict["sum"] = float(s.sum()) if s.count() > 0 else None
    fs_dict["approxNumDistinctValues"] = int(s.nunique())
    stats_list.append(fs_dict)

# Output stats as JSON for the Go client to register
result = {
    "computationTime": int(time.time() * 1000),
    "rowPercentage": 1.0,
    "beforeTransformation": False,
    "featureDescriptiveStatistics": stats_list,
}
print(json.dumps(result))
`, fvName, fvVer, tdVer)
}

func init() {
	rootCmd.AddCommand(tdCmd)

	tdCreateCmd.Flags().StringVar(&tdCreateDesc, "description", "", "Description")
	tdCreateCmd.Flags().StringVar(&tdCreateFormat, "format", "parquet", "Data format (parquet, csv, tfrecord)")

	tdComputeCmd.Flags().StringVar(&tdComputeFormat, "format", "parquet", "Data format (parquet, csv, tfrecord)")
	tdComputeCmd.Flags().StringVar(&tdComputeDesc, "description", "", "Description")
	tdComputeCmd.Flags().StringVar(&tdComputeSplit, "split", "", `Split spec: "train:0.8,test:0.2"`)
	tdComputeCmd.Flags().StringVar(&tdComputeFilter, "filter", "", `Filter rows: "price > 100", "price > 50 AND product == Laptop"`)
	tdComputeCmd.Flags().StringVar(&tdComputeStartTime, "start-time", "", "Start time filter (e.g. 2026-01-01)")
	tdComputeCmd.Flags().StringVar(&tdComputeEndTime, "end-time", "", "End time filter (e.g. 2026-02-01)")

	tdReadCmd.Flags().IntVar(&tdReadVersion, "td-version", 0, "Training dataset version (required)")
	tdReadCmd.Flags().StringVar(&tdReadOutput, "output", "", "Save to file (.parquet, .csv, .json)")
	tdReadCmd.Flags().StringVar(&tdReadSplit, "split", "", "Read specific split (train, test)")

	tdStatsCmd.Flags().IntVar(&tdStatsVersion, "td-version", 0, "Training dataset version (required)")
	tdStatsCmd.Flags().StringVar(&tdStatsFeatures, "features", "", "Filter features (comma-separated)")
	tdStatsCmd.Flags().BoolVar(&tdStatsCompute, "compute", false, "Trigger statistics computation")

	tdCmd.AddCommand(tdListCmd)
	tdCmd.AddCommand(tdCreateCmd)
	tdCmd.AddCommand(tdDeleteCmd)
	tdCmd.AddCommand(tdComputeCmd)
	tdCmd.AddCommand(tdReadCmd)
	tdCmd.AddCommand(tdStatsCmd)
}
