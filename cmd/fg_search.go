package cmd

import (
	"fmt"
	"strings"

	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var (
	fgSearchVector string
	fgSearchK      int
	fgSearchCol    string
)

var fgSearchCmd = &cobra.Command{
	Use:   "search <name>",
	Short: "Similarity search on embedding features",
	Long: `Find nearest neighbors using vector similarity search.

Requires an online-enabled feature group with an embedding index.

Examples:
  hops fg search documents --vector "0.1,0.2,0.3,0.4"
  hops fg search documents --vector "0.1,0.2,0.3,0.4" --k 5
  hops fg search documents --vector "[0.1, 0.2, 0.3, 0.4]" --col text_embedding`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if fgSearchVector == "" {
			return fmt.Errorf("--vector is required (comma-separated floats or JSON array)")
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		fg, err := c.GetFeatureGroup(args[0], fgVersion)
		if err != nil {
			return fmt.Errorf("feature group '%s' not found: %w", args[0], err)
		}

		if !output.JSONMode {
			output.Info("Searching '%s' v%d (k=%d)...", fg.Name, fg.Version, fgSearchK)
		}

		script := buildSearchScript(fg.Name, fg.Version, fgSearchVector, fgSearchK, fgSearchCol, output.JSONMode)
		if err := runPython(script); err != nil {
			return fmt.Errorf("similarity search failed: %w", err)
		}
		return nil
	},
}

func buildSearchScript(fgName string, fgVersion int, vector string, k int, col string, jsonMode bool) string {
	var sb strings.Builder

	sb.WriteString(`import hopsworks, warnings, logging, json, sys
import pandas as pd
warnings.filterwarnings("ignore")
logging.getLogger("hsfs").setLevel(logging.WARNING)
logging.getLogger("hopsworks").setLevel(logging.WARNING)

project = hopsworks.login()
fs = project.get_feature_store()
`)

	sb.WriteString(fmt.Sprintf("fg = fs.get_feature_group(%q, version=%d)\n", fgName, fgVersion))

	// Parse vector: handle both "0.1,0.2,0.3" and "[0.1,0.2,0.3]"
	sb.WriteString(fmt.Sprintf("\nvector_str = %q\n", vector))
	sb.WriteString(`vector_str = vector_str.strip()
if vector_str.startswith("["):
    embedding = json.loads(vector_str)
else:
    embedding = [float(x.strip()) for x in vector_str.split(",")]
`)

	// Build find_neighbors call
	sb.WriteString(fmt.Sprintf("\nresults = fg.find_neighbors(embedding, k=%d", k))
	if col != "" {
		sb.WriteString(fmt.Sprintf(", col=%q", col))
	}
	sb.WriteString(")\n")

	// Format output
	sb.WriteString("feature_names = [f.name for f in fg.features]\n\n")

	if jsonMode {
		sb.WriteString(`rows = []
for score, values in results:
    row = {"_score": score}
    row.update(dict(zip(feature_names, values)))
    rows.append(row)
print(json.dumps(rows, default=str))
`)
	} else {
		sb.WriteString(`if not results:
    print("No results found")
else:
    rows = []
    for score, values in results:
        row = dict(zip(feature_names, values))
        row["_score"] = f"{score:.6f}"
        rows.append(row)
    df = pd.DataFrame(rows)
    cols = ["_score"] + [c for c in df.columns if c != "_score"]
    df = df[cols]
    print(df.to_string(index=False))
`)
	}

	return sb.String()
}

func init() {
	fgSearchCmd.Flags().IntVar(&fgVersion, "version", 0, "Feature group version (latest if omitted)")
	fgSearchCmd.Flags().StringVar(&fgSearchVector, "vector", "", "Query vector (comma-separated floats or JSON array)")
	fgSearchCmd.Flags().IntVar(&fgSearchK, "k", 10, "Number of nearest neighbors")
	fgSearchCmd.Flags().StringVar(&fgSearchCol, "col", "", "Embedding column (required if multiple embeddings)")
	fgCmd.AddCommand(fgSearchCmd)
}
