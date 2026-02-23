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
	fgDeriveBase     string
	fgDeriveJoins    []string
	fgDerivePK       string
	fgDeriveOnline   bool
	fgDeriveDesc     string
	fgDeriveEvtTime  string
	fgDeriveFeatures string
)

type joinSpec struct {
	fgName  string
	version int
	joinType string
	leftOn  string
	rightOn string
	prefix  string
}

// parseNameVersion splits "name:version" into (name, version).
// Default version is 1 if omitted.
func parseNameVersion(s string) (string, int) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 2 {
		v := 1
		fmt.Sscanf(parts[1], "%d", &v)
		return parts[0], v
	}
	return s, 1
}

// parseJoinSpec parses a join spec string:
// "<fg_name>[:<version>] <JOIN_TYPE> <on_col>[=<right_on_col>] [prefix]"
func parseJoinSpec(spec string) (joinSpec, error) {
	tokens := strings.Fields(spec)
	if len(tokens) < 3 {
		return joinSpec{}, fmt.Errorf("join spec needs at least 3 parts: \"<fg> <JOIN_TYPE> <on_col>\", got: %q", spec)
	}

	name, version := parseNameVersion(tokens[0])

	jt := strings.ToUpper(tokens[1])
	switch jt {
	case "INNER", "LEFT", "RIGHT", "FULL":
	default:
		return joinSpec{}, fmt.Errorf("invalid join type %q (must be INNER, LEFT, RIGHT, or FULL)", jt)
	}

	leftOn, rightOn := tokens[2], tokens[2]
	if eqIdx := strings.Index(tokens[2], "="); eqIdx > 0 {
		leftOn = tokens[2][:eqIdx]
		rightOn = tokens[2][eqIdx+1:]
	}

	prefix := ""
	if len(tokens) >= 4 {
		prefix = tokens[3]
	}

	return joinSpec{
		fgName:   name,
		version:  version,
		joinType: strings.ToLower(jt),
		leftOn:   leftOn,
		rightOn:  rightOn,
		prefix:   prefix,
	}, nil
}

var fgDeriveCmd = &cobra.Command{
	Use:   "derive <name>",
	Short: "Create a feature group by joining existing ones",
	Long: `Derive a new feature group by joining existing feature groups.

Join spec format: "<fg_name>[:<version>] <JOIN_TYPE> <on_col>[=<right_on_col>] [prefix]"

Examples:
  # Simple: join on shared column
  hops fg derive enriched_customers \
    --base customer_transactions \
    --join "product_metrics LEFT id" \
    --primary-key id

  # Different join keys + prefix
  hops fg derive enriched \
    --base customer_transactions \
    --join "product_metrics LEFT customer_id=id p_" \
    --primary-key customer_id

  # Multiple joins with online storage
  hops fg derive full_view \
    --base orders \
    --join "customers LEFT customer_id" \
    --join "products LEFT product_id=id p_" \
    --primary-key order_id \
    --online`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetName := args[0]

		if fgDeriveBase == "" {
			return fmt.Errorf("--base is required")
		}
		if fgDerivePK == "" {
			return fmt.Errorf("--primary-key is required")
		}
		if len(fgDeriveJoins) == 0 {
			return fmt.Errorf("at least one --join is required")
		}

		// Parse base name:version
		baseName, baseVersion := parseNameVersion(fgDeriveBase)

		// Parse all join specs
		var joins []joinSpec
		for _, raw := range fgDeriveJoins {
			j, err := parseJoinSpec(raw)
			if err != nil {
				return err
			}
			joins = append(joins, j)
		}

		// Validate all referenced FGs exist via Go REST client
		c, err := mustClient()
		if err != nil {
			return err
		}

		baseFG, err := c.GetFeatureGroup(baseName, baseVersion)
		if err != nil {
			return fmt.Errorf("base feature group '%s' not found: %w", baseName, err)
		}

		for _, j := range joins {
			_, err := c.GetFeatureGroup(j.fgName, j.version)
			if err != nil {
				return fmt.Errorf("join feature group '%s' not found: %w", j.fgName, err)
			}
		}

		if !output.JSONMode {
			output.Info("Deriving '%s' from '%s' v%d with %d join(s)...", targetName, baseFG.Name, baseFG.Version, len(joins))
		}

		pyScript := buildDeriveScript(targetName, baseName, baseVersion, joins)

		pyCmd := exec.Command("python3", "-c", pyScript)
		pyCmd.Stdout = os.Stdout
		pyCmd.Stderr = os.Stderr

		pyCmd.Env = append(os.Environ(),
			"PEMS_DIR="+os.ExpandEnv("${HOME}/.hopsfs_pems"),
			"LIBHDFS_DEFAULT_USER="+os.Getenv("HADOOP_USER_NAME"),
		)

		if err := pyCmd.Run(); err != nil {
			return fmt.Errorf("derive feature group: %w", err)
		}

		if output.JSONMode {
			output.PrintJSON(map[string]interface{}{
				"status":        "success",
				"feature_group": targetName,
				"base":          baseName,
				"joins":         len(joins),
			})
		}
		return nil
	},
}

func buildDeriveScript(targetName, baseName string, baseVersion int, joins []joinSpec) string {
	var sb strings.Builder

	// Preamble
	sb.WriteString(`
import hopsworks, pandas as pd, warnings, logging
warnings.filterwarnings("ignore")
logging.getLogger("hsfs").setLevel(logging.WARNING)
logging.getLogger("hopsworks").setLevel(logging.WARNING)

project = hopsworks.login()
fs = project.get_feature_store()

`)

	// Get base FG
	sb.WriteString(fmt.Sprintf("base_fg = fs.get_feature_group(%q, version=%d)\n", baseName, baseVersion))

	// Get join FGs
	for i, j := range joins {
		sb.WriteString(fmt.Sprintf("join%d_fg = fs.get_feature_group(%q, version=%d)\n", i, j.fgName, j.version))
	}

	// Build query
	sb.WriteString("\nquery = base_fg.select_all()\n")
	for i, j := range joins {
		sb.WriteString(fmt.Sprintf("query = query.join(join%d_fg.select_all(), %s)\n", i, buildJoinKwargs(j)))
	}

	// Execute
	sb.WriteString(`
print("Executing join query...")
df = query.read()
print(f"Query returned {len(df)} rows, {len(df.columns)} columns")
`)

	// Optional feature filter
	if fgDeriveFeatures != "" {
		cols := splitComma(fgDeriveFeatures)
		var quoted []string
		for _, col := range cols {
			quoted = append(quoted, fmt.Sprintf("%q", col))
		}
		sb.WriteString(fmt.Sprintf("df = df[[%s]]\n", strings.Join(quoted, ", ")))
		sb.WriteString(fmt.Sprintf("print(f\"Filtered to %d columns\")\n", len(cols)))
	}

	// Primary keys
	pks := splitComma(fgDerivePK)
	var quotedPKs []string
	for _, pk := range pks {
		quotedPKs = append(quotedPKs, fmt.Sprintf("%q", pk))
	}
	pkList := "[" + strings.Join(quotedPKs, ", ") + "]"

	// Event time
	etKwarg := ""
	if fgDeriveEvtTime != "" {
		etKwarg = fmt.Sprintf(",\n    event_time=%q", fgDeriveEvtTime)
	}

	// Description — auto-generate provenance if not provided
	desc := fgDeriveDesc
	if desc == "" {
		desc = fmt.Sprintf("Derived from %s v%d", baseName, baseVersion)
		for _, j := range joins {
			onClause := j.leftOn
			if j.leftOn != j.rightOn {
				onClause = j.leftOn + "=" + j.rightOn
			}
			desc += fmt.Sprintf(" %s JOIN %s v%d ON %s", strings.ToUpper(j.joinType), j.fgName, j.version, onClause)
		}
	}
	descKwarg := ""
	if desc != "" {
		descKwarg = fmt.Sprintf(",\n    description=%q", desc)
	}

	// Parents list for provenance graph
	parentsList := "base_fg"
	for i := range joins {
		parentsList += fmt.Sprintf(", join%d_fg", i)
	}

	// Create target FG and insert
	sb.WriteString(fmt.Sprintf(`
target_fg = fs.get_or_create_feature_group(
    name=%q,
    version=1,
    primary_key=%s,
    online_enabled=%s%s%s,
    parents=[%s],
)
print(f"Inserting {len(df)} rows into {target_fg.name} v{target_fg.version}...")
target_fg.insert(df, write_options=%s%s)
print(f"Successfully derived '{target_fg.name}' v{target_fg.version} ({len(df)} rows)")
`, targetName, pkList, pythonBool(fgDeriveOnline), etKwarg, descKwarg, parentsList,
		writeOptionsSnippet(fgDeriveOnline), storageSnippet(fgDeriveOnline)))

	return sb.String()
}

func buildJoinKwargs(j joinSpec) string {
	var parts []string

	if j.leftOn == j.rightOn {
		parts = append(parts, fmt.Sprintf("on=[%q]", j.leftOn))
	} else {
		parts = append(parts, fmt.Sprintf("left_on=[%q], right_on=[%q]", j.leftOn, j.rightOn))
	}

	parts = append(parts, fmt.Sprintf("join_type=%q", j.joinType))

	if j.prefix != "" {
		parts = append(parts, fmt.Sprintf("prefix=%q", j.prefix))
	}

	return strings.Join(parts, ", ")
}

func pythonBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

func init() {
	fgDeriveCmd.Flags().StringVar(&fgDeriveBase, "base", "", "Base feature group (name or name:version)")
	fgDeriveCmd.Flags().StringArrayVar(&fgDeriveJoins, "join", nil, `Join spec: "<fg>[:<ver>] <INNER|LEFT|RIGHT|FULL> <on>[=<right_on>] [prefix]"`)
	fgDeriveCmd.Flags().StringVar(&fgDerivePK, "primary-key", "", "Primary key columns for target (comma-separated)")
	fgDeriveCmd.Flags().BoolVar(&fgDeriveOnline, "online", false, "Enable online storage for derived FG")
	fgDeriveCmd.Flags().StringVar(&fgDeriveDesc, "description", "", "Description for derived FG")
	fgDeriveCmd.Flags().StringVar(&fgDeriveEvtTime, "event-time", "", "Event time column for derived FG")
	fgDeriveCmd.Flags().StringVar(&fgDeriveFeatures, "features", "", "Columns to keep (comma-separated, applied post-query)")
	fgCmd.AddCommand(fgDeriveCmd)
}
