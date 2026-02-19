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
	fgExtConnector   string
	fgExtQuery       string
	fgExtPrimaryKey  string
	fgExtFeatures    string
	fgExtEventTime   string
	fgExtOnline      bool
	fgExtDescription string
	fgExtDatabase    string
	fgExtTable       string
	fgExtSchema      string
)

var fgCreateExternalCmd = &cobra.Command{
	Use:   "create-external <name>",
	Short: "Create an external feature group backed by a storage connector",
	Long: `Create an external feature group that reads from an external data source
via a storage connector (Snowflake, JDBC, S3, etc.).

Schema is auto-inferred from the connector when --database and --table are provided.
Otherwise, provide --features explicitly.

Examples:
  # Auto-infer schema from connector (recommended)
  hops fg create-external snowflake__orders \
    --connector my_sf \
    --query "SELECT * FROM MY_DB.SCHEMA.ORDERS" \
    --database MY_DB --table ORDERS --schema SCHEMA \
    --primary-key o_orderkey

  # Explicit schema
  hops fg create-external sales_fg \
    --connector my_sf \
    --query "SELECT id, amount FROM sales" \
    --features "id:bigint,amount:double" \
    --primary-key id`,
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

		c, err := mustClient()
		if err != nil {
			return err
		}

		sc, err := c.GetStorageConnector(fgExtConnector)
		if err != nil {
			return fmt.Errorf("connector '%s' not found: %w", fgExtConnector, err)
		}

		// Resolve features: explicit flag > auto-infer from connector > error
		var features []client.Feature
		pks := splitComma(fgExtPrimaryKey)
		pkSet := make(map[string]bool)
		for _, pk := range pks {
			pkSet[strings.ToLower(pk)] = true
		}

		if fgExtFeatures != "" {
			// Parse explicit features
			for _, spec := range splitComma(fgExtFeatures) {
				parts := splitStr(trimSpace(spec), ":")
				name := trimSpace(parts[0])
				typ := "string"
				if len(parts) > 1 {
					typ = trimSpace(parts[1])
				}
				features = append(features, client.Feature{
					Name:    name,
					Type:    typ,
					Primary: pkSet[strings.ToLower(name)],
				})
			}
		} else if fgExtTable != "" {
			// Auto-infer from connector's data endpoint
			if !output.JSONMode {
				output.Info("Inferring schema from connector...")
			}
			ds := &client.DataSource{
				Database: fgExtDatabase,
				Table:    fgExtTable,
				Group:    fgExtSchema,
			}
			result, err := c.GetConnectorData(fgExtConnector, ds)
			if err != nil {
				return fmt.Errorf("could not infer schema: %w", err)
			}
			if len(result.Features) == 0 {
				return fmt.Errorf("no features returned from connector — use --features to specify schema manually")
			}
			for _, f := range result.Features {
				typ := f.Type
				if typ == "" {
					typ = "string"
				}
				features = append(features, client.Feature{
					Name:    strings.ToLower(f.Name),
					Type:    typ,
					Primary: pkSet[strings.ToLower(f.Name)],
				})
			}
			if !output.JSONMode {
				output.Info("Inferred %d features from %s.%s", len(features), fgExtDatabase, fgExtTable)
			}
		} else {
			return fmt.Errorf("provide --features or --database/--table for schema inference")
		}

		if !output.JSONMode {
			output.Info("Creating external feature group '%s' using connector '%s' (%s)...",
				fgName, sc.Name, sc.StorageConnectorType)
		}

		pyScript := buildExternalFGScript(fgName, fgExtConnector, fgExtQuery,
			pks, features, fgExtEventTime, fgExtOnline, fgExtDescription)

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

func buildExternalFGScript(name, connector, query string, primaryKeys []string, features []client.Feature, eventTime string, online bool, description string) string {
	// Build primary_key list
	var pkParts []string
	for _, pk := range primaryKeys {
		pkParts = append(pkParts, fmt.Sprintf("%q", pk))
	}
	pkList := "[" + strings.Join(pkParts, ", ") + "]"

	// Build features list for Python
	var featureParts []string
	for _, f := range features {
		pk := "False"
		if f.Primary {
			pk = "True"
		}
		featureParts = append(featureParts, fmt.Sprintf(
			`        Feature(name=%q, type=%q, primary=%s)`, f.Name, f.Type, pk))
	}
	featuresBlock := "[\n" + strings.Join(featureParts, ",\n") + "\n    ]"

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
from hsfs.feature import Feature
import warnings
warnings.filterwarnings("ignore")

project = hopsworks.login()
fs = project.get_feature_store()

sc = fs.get_storage_connector(%q)
print(f"Using connector: {sc.name} ({sc.type})")

from hsfs import statistics_config as sc_mod
fg = fs.create_external_feature_group(
    name=%q,
    version=1,
    storage_connector=sc,
    query=%q,
    primary_key=%s,
    features=%s,
    statistics_config=sc_mod.StatisticsConfig(enabled=False),
%s%s%s)
fg.save()
print(f"Created external feature group '{fg.name}' v{fg.version} (ID: {fg.id})")
`, connector, name, query, pkList, featuresBlock, etLine, onlineLine, descLine)
}

func init() {
	fgCreateExternalCmd.Flags().StringVar(&fgExtConnector, "connector", "", "Storage connector name")
	fgCreateExternalCmd.Flags().StringVar(&fgExtQuery, "query", "", "SQL query for external data")
	fgCreateExternalCmd.Flags().StringVar(&fgExtPrimaryKey, "primary-key", "", "Primary key columns (comma-separated)")
	fgCreateExternalCmd.Flags().StringVar(&fgExtFeatures, "features", "", "Feature schema: name:type,... (explicit, skips auto-infer)")
	fgCreateExternalCmd.Flags().StringVar(&fgExtDatabase, "database", "", "Database for schema inference")
	fgCreateExternalCmd.Flags().StringVar(&fgExtTable, "table", "", "Table for schema inference")
	fgCreateExternalCmd.Flags().StringVar(&fgExtSchema, "schema", "", "Schema for schema inference")
	fgCreateExternalCmd.Flags().StringVar(&fgExtEventTime, "event-time", "", "Event time column")
	fgCreateExternalCmd.Flags().BoolVar(&fgExtOnline, "online", false, "Enable online storage")
	fgCreateExternalCmd.Flags().StringVar(&fgExtDescription, "description", "", "Description")
	fgCmd.AddCommand(fgCreateExternalCmd)
}
