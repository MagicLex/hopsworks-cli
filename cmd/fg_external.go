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
	fgExtConnector   string
	fgExtQuery       string
	fgExtPrimaryKey  string
	fgExtFeatures    string
	fgExtEventTime   string
	fgExtOnline      bool
	fgExtDescription string
)

var fgCreateExternalCmd = &cobra.Command{
	Use:   "create-external <name>",
	Short: "Create an external feature group backed by a storage connector",
	Long: `Create an external feature group that reads from an external data source
via a storage connector (Snowflake, JDBC, S3, etc.).

Examples:
  hops fg create-external sales_fg \
    --connector my_snowflake \
    --query "SELECT id, name, amount FROM MY_DB.PUBLIC.sales" \
    --primary-key id

  hops fg create-external customer_features \
    --connector my_jdbc \
    --query "SELECT * FROM customers" \
    --primary-key customer_id \
    --event-time updated_at \
    --online`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fgName := args[0]

		if fgExtConnector == "" {
			return fmt.Errorf("--connector is required")
		}
		if fgExtQuery == "" {
			return fmt.Errorf("--query is required")
		}
		if fgExtPrimaryKey == "" {
			return fmt.Errorf("--primary-key is required")
		}

		// Validate connector exists via Go REST client
		c, err := mustClient()
		if err != nil {
			return err
		}

		sc, err := c.GetStorageConnector(fgExtConnector)
		if err != nil {
			return fmt.Errorf("connector '%s' not found: %w", fgExtConnector, err)
		}

		if !output.JSONMode {
			output.Info("Creating external feature group '%s' using connector '%s' (%s)...",
				fgName, sc.Name, sc.StorageConnectorType)
		}

		pyScript := buildExternalFGScript(fgName, fgExtConnector, fgExtQuery,
			splitComma(fgExtPrimaryKey), fgExtEventTime, fgExtOnline, fgExtDescription)

		pyCmd := exec.Command("python3", "-c", pyScript)
		pyCmd.Stdout = os.Stdout
		pyCmd.Stderr = os.Stderr
		pyCmd.Env = os.Environ()

		if err := pyCmd.Run(); err != nil {
			return fmt.Errorf("create external feature group failed: %w", err)
		}

		if output.JSONMode {
			output.PrintJSON(map[string]interface{}{
				"status":        "success",
				"feature_group": fgName,
				"connector":     sc.Name,
				"type":          "external",
			})
		}
		return nil
	},
}

func buildExternalFGScript(name, connector, query string, primaryKeys []string, eventTime string, online bool, description string) string {
	// Build primary_key list
	var pkParts []string
	for _, pk := range primaryKeys {
		pkParts = append(pkParts, fmt.Sprintf("%q", pk))
	}
	pkList := "[" + strings.Join(pkParts, ", ") + "]"

	etLine := ""
	if eventTime != "" {
		etLine = fmt.Sprintf("    event_time=%q,\n", eventTime)
	}

	onlineLine := ""
	if online {
		onlineLine = "    online_enabled=True,\n"
	}

	descLine := ""
	if description != "" {
		descLine = fmt.Sprintf("    description=%q,\n", description)
	}

	return fmt.Sprintf(`
import hopsworks
import warnings
warnings.filterwarnings("ignore")

project = hopsworks.login()
fs = project.get_feature_store()

sc = fs.get_storage_connector(%q)
print(f"Using connector: {sc.name} ({sc.connector_type})")

fg = fs.create_external_feature_group(
    name=%q,
    version=1,
    storage_connector=sc,
    query=%q,
    primary_key=%s,
%s%s%s)
fg.save()
print(f"Created external feature group '{fg.name}' v{fg.version} (ID: {fg.id})")
`, connector, name, query, pkList, etLine, onlineLine, descLine)
}

func init() {
	fgCreateExternalCmd.Flags().StringVar(&fgExtConnector, "connector", "", "Storage connector name")
	fgCreateExternalCmd.Flags().StringVar(&fgExtQuery, "query", "", "SQL query for external data")
	fgCreateExternalCmd.Flags().StringVar(&fgExtPrimaryKey, "primary-key", "", "Primary key columns (comma-separated)")
	fgCreateExternalCmd.Flags().StringVar(&fgExtFeatures, "features", "", "Feature schema: name:type,... (auto-inferred if omitted)")
	fgCreateExternalCmd.Flags().StringVar(&fgExtEventTime, "event-time", "", "Event time column")
	fgCreateExternalCmd.Flags().BoolVar(&fgExtOnline, "online", false, "Enable online storage")
	fgCreateExternalCmd.Flags().StringVar(&fgExtDescription, "description", "", "Description")
	fgCmd.AddCommand(fgCreateExternalCmd)
}
