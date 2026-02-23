package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/MagicLex/hopsworks-cli/pkg/client"
	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var (
	fgInsertFile     string
	fgInsertGenerate int
	fgInsertOnline   bool
)

var fgInsertCmd = &cobra.Command{
	Use:   "insert <name>",
	Short: "Insert data into a feature group",
	Long: `Insert data into a feature group from a JSON file, stdin, or generate sample data.

Examples:
  # Generate and insert 50 rows of sample data
  hops fg insert customer_transactions --generate 50

  # Insert from a JSON file
  hops fg insert customer_transactions --file data.json

  # Insert from stdin (pipe)
  cat data.json | hops fg insert customer_transactions`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fgName := args[0]

		// Get FG info first via the Go client to validate it exists
		c, err := mustClient()
		if err != nil {
			return err
		}

		fg, err := c.GetFeatureGroup(fgName, fgVersion)
		if err != nil {
			return fmt.Errorf("feature group '%s' not found: %w", fgName, err)
		}

		// Build the Python script
		var pyScript string
		if fgInsertGenerate > 0 {
			pyScript = buildGenerateScript(fg.Name, fg.Version, fg.Features, fgInsertGenerate, fgInsertOnline)
		} else if fgInsertFile != "" {
			pyScript = buildFileInsertScript(fg.Name, fg.Version, fgInsertFile, fgInsertOnline)
		} else {
			// Read from stdin
			pyScript = buildStdinInsertScript(fg.Name, fg.Version, fgInsertOnline)
		}

		if !output.JSONMode {
			output.Info("Inserting data into '%s' v%d (ID: %d)...", fg.Name, fg.Version, fg.ID)
		}

		// Execute via python3
		pyCmd := exec.Command("python3", "-c", pyScript)
		pyCmd.Stdout = os.Stdout
		pyCmd.Stderr = os.Stderr
		pyCmd.Stdin = os.Stdin

		// Set env vars for hops-deltalake mTLS (PEM certs + HDFS user identity).
		// PEMS_DIR must point to PEM files extracted from the pod's JKS keystores.
		// See docs/fixes/sdk-fixes.md for the full chain and why these are needed.
		pyCmd.Env = append(os.Environ(),
			"PEMS_DIR="+os.ExpandEnv("${HOME}/.hopsfs_pems"),
			"LIBHDFS_DEFAULT_USER="+os.Getenv("HADOOP_USER_NAME"),
		)

		if err := pyCmd.Run(); err != nil {
			return fmt.Errorf("insert into feature group: %w", err)
		}

		if output.JSONMode {
			out := map[string]interface{}{
				"status":        "success",
				"feature_group": fg.Name,
				"version":       fg.Version,
			}
			if fgInsertGenerate > 0 {
				out["rows_generated"] = fgInsertGenerate
			}
			output.PrintJSON(out)
		}
		return nil
	},
}

// writeOptionsSnippet returns the Python write_options dict depending on mode.
func writeOptionsSnippet(onlineOnly bool) string {
	if onlineOnly {
		return `{"start_offline_materialization": False}`
	}
	return `{"wait_for_job": True}`
}

// storageSnippet returns the storage kwarg for insert if needed.
func storageSnippet(onlineOnly bool) string {
	if onlineOnly {
		return `, storage="online"`
	}
	return ""
}

// buildGenerateScript creates a Python script that generates sample data
// based on the feature group schema and inserts it.
func buildGenerateScript(fgName string, fgVersion int, features []client.Feature, n int, onlineOnly bool) string {
	// Build column generators based on type
	var colGens []string
	for _, f := range features {
		gen := columnGenerator(f)
		colGens = append(colGens, fmt.Sprintf("    %q: %s,", f.Name, gen))
	}

	// Build PK list and event time for get_or_create
	var pkNames []string
	var eventTime string
	for _, f := range features {
		if f.Primary {
			pkNames = append(pkNames, fmt.Sprintf("%q", f.Name))
		}
	}
	for _, f := range features {
		if strings.Contains(strings.ToLower(f.Type), "timestamp") || strings.Contains(f.Name, "event_time") {
			eventTime = f.Name
			break
		}
	}

	pkList := "[" + strings.Join(pkNames, ", ") + "]"
	etLine := ""
	if eventTime != "" {
		etLine = fmt.Sprintf("    event_time=%q,", eventTime)
	}

	return fmt.Sprintf(`
import hopsworks
import pandas as pd
import numpy as np
from datetime import datetime, timedelta
import warnings, logging
warnings.filterwarnings("ignore")
logging.getLogger("hsfs").setLevel(logging.WARNING)
logging.getLogger("hopsworks").setLevel(logging.WARNING)

np.random.seed(42)
n = %d

project = hopsworks.login()
fs = project.get_feature_store()

# get_or_create ensures Kafka topic + materialization job exist
fg = fs.get_or_create_feature_group(
    name=%q,
    version=%d,
    primary_key=%s,
%s
    online_enabled=True,
)

data = {
%s
}
df = pd.DataFrame(data)
print(f"Generated {len(df)} rows, inserting...")
fg.insert(df, write_options=%s%s)
print(f"Successfully inserted {len(df)} rows into {fg.name} v{fg.version}")
`, n, fgName, fgVersion, pkList, etLine, strings.Join(colGens, "\n"), writeOptionsSnippet(onlineOnly), storageSnippet(onlineOnly))
}

// columnGenerator returns a numpy expression to generate sample data for a feature type.
func columnGenerator(f client.Feature) string {
	name := f.Name
	typ := strings.ToLower(f.Type)

	// Primary key: sequential integers
	if f.Primary {
		return "list(range(1, n+1))"
	}

	// Event time / timestamp columns
	if strings.Contains(typ, "timestamp") || strings.Contains(name, "time") || strings.Contains(name, "date") {
		return "[datetime.now() - timedelta(days=int(x)) for x in np.random.randint(0, 30, n)]"
	}

	// Boolean
	if strings.Contains(typ, "bool") {
		return "np.random.choice([True, False], n).tolist()"
	}

	// Float / double
	if strings.Contains(typ, "double") || strings.Contains(typ, "float") || strings.Contains(typ, "decimal") {
		return "np.round(np.random.uniform(1.0, 1000.0, n), 2).tolist()"
	}

	// Integer types
	if strings.Contains(typ, "int") || strings.Contains(typ, "bigint") || strings.Contains(typ, "smallint") {
		return "np.random.randint(1, 500, n).tolist()"
	}

	// String
	if strings.Contains(typ, "string") || strings.Contains(typ, "varchar") {
		return fmt.Sprintf("[f'%s_{i}' for i in range(n)]", name)
	}

	// Default: random integers
	return "np.random.randint(1, 100, n).tolist()"
}

func buildFileInsertScript(fgName string, fgVersion int, filePath string, onlineOnly bool) string {
	return fmt.Sprintf(`
import hopsworks
import pandas as pd
import json
import warnings
warnings.filterwarnings("ignore")

project = hopsworks.login()
fs = project.get_feature_store()
fg = fs.get_or_create_feature_group(name=%q, version=%d)

file_path = %q
if file_path.endswith('.csv'):
    df = pd.read_csv(file_path)
elif file_path.endswith('.parquet'):
    df = pd.read_parquet(file_path)
else:
    with open(file_path) as f:
        data = json.load(f)
    if isinstance(data, list):
        df = pd.DataFrame(data)
    else:
        df = pd.DataFrame([data])

print(f"Read {len(df)} rows from {file_path}, inserting...")
fg.insert(df, write_options=%s%s)
print(f"Successfully inserted {len(df)} rows into {fg.name} v{fg.version}")
`, fgName, fgVersion, filePath, writeOptionsSnippet(onlineOnly), storageSnippet(onlineOnly))
}

func buildStdinInsertScript(fgName string, fgVersion int, onlineOnly bool) string {
	return fmt.Sprintf(`
import hopsworks
import pandas as pd
import json
import sys
import warnings
warnings.filterwarnings("ignore")

project = hopsworks.login()
fs = project.get_feature_store()
fg = fs.get_or_create_feature_group(name=%q, version=%d)

raw = sys.stdin.read()
data = json.loads(raw)
if isinstance(data, list):
    df = pd.DataFrame(data)
else:
    df = pd.DataFrame([data])

print(f"Read {len(df)} rows from stdin, inserting...")
fg.insert(df, write_options=%s%s)
print(f"Successfully inserted {len(df)} rows into {fg.name} v{fg.version}")
`, fgName, fgVersion, writeOptionsSnippet(onlineOnly), storageSnippet(onlineOnly))
}

func init() {
	fgInsertCmd.Flags().IntVar(&fgVersion, "version", 0, "Feature group version (latest if omitted)")
	fgInsertCmd.Flags().StringVar(&fgInsertFile, "file", "", "JSON/CSV/Parquet file to insert")
	fgInsertCmd.Flags().IntVar(&fgInsertGenerate, "generate", 0, "Generate N rows of sample data")
	fgInsertCmd.Flags().BoolVar(&fgInsertOnline, "online-only", false, "Write to online store only (skip offline materialization)")
	fgCmd.AddCommand(fgInsertCmd)
}
