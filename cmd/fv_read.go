package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var (
	fvGetEntries []string
	fvReadOutput string
	fvReadN      int
)

// buildFVPreamble returns the Python preamble that logs in and gets a feature view.
func buildFVPreamble(fvName string, version int) string {
	return fmt.Sprintf(`import hopsworks, warnings, logging, json, sys
import pandas as pd
warnings.filterwarnings("ignore")
logging.getLogger("hsfs").setLevel(logging.WARNING)
logging.getLogger("hopsworks").setLevel(logging.WARNING)

project = hopsworks.login()
fs = project.get_feature_store()
fv = fs.get_feature_view(name=%q, version=%d)
`, fvName, version)
}

// runPython executes a Python script with mTLS env vars for HDFS access.
func runPython(script string) error {
	pyCmd := exec.Command("python3", "-c", script)
	pyCmd.Stdout = os.Stdout
	pyCmd.Stderr = os.Stderr
	pyCmd.Stdin = os.Stdin
	pyCmd.Env = append(os.Environ(),
		"PEMS_DIR="+os.ExpandEnv("${HOME}/.hopsfs_pems"),
		"LIBHDFS_DEFAULT_USER="+os.Getenv("HADOOP_USER_NAME"),
	)
	return pyCmd.Run()
}

// runPythonCapture executes a Python script and captures stdout (stderr goes to terminal).
func runPythonCapture(script string) ([]byte, error) {
	pyCmd := exec.Command("python3", "-c", script)
	pyCmd.Stderr = os.Stderr
	pyCmd.Stdin = os.Stdin
	pyCmd.Env = append(os.Environ(),
		"PEMS_DIR="+os.ExpandEnv("${HOME}/.hopsfs_pems"),
		"LIBHDFS_DEFAULT_USER="+os.Getenv("HADOOP_USER_NAME"),
	)
	return pyCmd.Output()
}

// pythonLiteral converts a string to a Python literal (number or quoted string).
func pythonLiteral(s string) string {
	isNum := true
	dotSeen := false
	for i, c := range s {
		if c == '-' && i == 0 {
			continue
		}
		if c == '.' && !dotSeen {
			dotSeen = true
			continue
		}
		if c < '0' || c > '9' {
			isNum = false
			break
		}
	}
	if isNum && len(s) > 0 && s != "-" && s != "." {
		return s
	}
	return fmt.Sprintf("%q", s)
}

// --- fv get ---

var fvGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Get feature vectors (online lookup)",
	Long: `Look up feature vectors from the online feature store.

Examples:
  hops fv get my_view --entry "id=42"
  hops fv get my_view --entry "id=1" --entry "id=2"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(fvGetEntries) == 0 {
			return fmt.Errorf("at least one --entry is required (format: \"key=value\")")
		}

		ver := fvVersion
		if ver == 0 {
			ver = 1
		}

		if !output.JSONMode {
			output.Info("Looking up feature vectors from '%s' v%d...", args[0], ver)
		}

		script := buildFVGetScript(args[0], ver, fvGetEntries, output.JSONMode)
		if err := runPython(script); err != nil {
			return fmt.Errorf("feature vector lookup failed: %w", err)
		}
		return nil
	},
}

func buildFVGetScript(fvName string, version int, entries []string, jsonMode bool) string {
	var sb strings.Builder
	sb.WriteString(buildFVPreamble(fvName, version))

	// Parse entries into Python dicts
	var pyEntries []string
	for _, e := range entries {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		pyEntries = append(pyEntries, fmt.Sprintf("{%q: %s}", key, pythonLiteral(val)))
	}

	sb.WriteString("\nfv.init_serving()\n")
	sb.WriteString("feature_names = [f.name for f in fv.features]\n\n")

	if len(pyEntries) == 1 {
		sb.WriteString(fmt.Sprintf("result = fv.get_feature_vector(entry=%s)\n", pyEntries[0]))
		sb.WriteString("row = dict(zip(feature_names, result))\n")
		if jsonMode {
			sb.WriteString("print(json.dumps(row, default=str))\n")
		} else {
			sb.WriteString("for k, v in row.items():\n")
			sb.WriteString("    print(f'{k}: {v}')\n")
		}
	} else {
		sb.WriteString(fmt.Sprintf("entries = [%s]\n", strings.Join(pyEntries, ", ")))
		sb.WriteString("results = fv.get_feature_vectors(entry=entries)\n")
		sb.WriteString("rows = [dict(zip(feature_names, r)) for r in results]\n")
		if jsonMode {
			sb.WriteString("print(json.dumps(rows, default=str))\n")
		} else {
			sb.WriteString("df = pd.DataFrame(rows)\n")
			sb.WriteString("print(df.to_string(index=False))\n")
		}
	}

	return sb.String()
}

// --- fv read ---

var fvReadCmd = &cobra.Command{
	Use:   "read <name>",
	Short: "Read batch data from a feature view",
	Long: `Read offline data from a feature view.

Examples:
  hops fv read my_view
  hops fv read my_view --n 100
  hops fv read my_view --output data.parquet
  hops fv read my_view --output data.csv`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ver := fvVersion
		if ver == 0 {
			ver = 1
		}

		if !output.JSONMode {
			if fvReadOutput != "" {
				output.Info("Reading batch data from '%s' v%d → %s...", args[0], ver, fvReadOutput)
			} else {
				output.Info("Reading batch data from '%s' v%d...", args[0], ver)
			}
		}

		script := buildFVReadScript(args[0], ver, fvReadOutput, fvReadN, output.JSONMode)
		if err := runPython(script); err != nil {
			return fmt.Errorf("batch read failed: %w", err)
		}
		return nil
	},
}

func buildFVReadScript(fvName string, version int, outputPath string, n int, jsonMode bool) string {
	var sb strings.Builder
	sb.WriteString(buildFVPreamble(fvName, version))

	sb.WriteString("\ndf = fv.get_batch_data()\n")

	if n > 0 {
		sb.WriteString(fmt.Sprintf("df = df.head(%d)\n", n))
	}

	sb.WriteString("print(f'Read {len(df)} rows, {len(df.columns)} columns', file=sys.stderr)\n")

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
	fvGetCmd.Flags().StringArrayVar(&fvGetEntries, "entry", nil, `Primary key entry: "key=value" (repeatable)`)
	fvGetCmd.Flags().IntVar(&fvVersion, "version", 0, "Feature view version (default: 1)")

	fvReadCmd.Flags().StringVar(&fvReadOutput, "output", "", "Save to file (.parquet, .csv, .json)")
	fvReadCmd.Flags().IntVar(&fvReadN, "n", 0, "Limit rows")
	fvReadCmd.Flags().IntVar(&fvVersion, "version", 0, "Feature view version (default: 1)")

	fvCmd.AddCommand(fvGetCmd)
	fvCmd.AddCommand(fvReadCmd)
}
