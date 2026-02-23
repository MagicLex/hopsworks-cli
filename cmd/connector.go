package cmd

import (
	"fmt"
	"strings"

	"github.com/MagicLex/hopsworks-cli/pkg/client"
	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

// --- Flag variables ---

var (
	// Snowflake
	connSFUrl       string
	connSFUser      string
	connSFPassword  string
	connSFToken     string
	connSFDatabase  string
	connSFSchema    string
	connSFWarehouse string
	connSFRole      string
	connSFTable     string
	// JDBC
	connJDBCConnStr string
	connJDBCArgs    string
	// S3
	connS3Bucket       string
	connS3AccessKey    string
	connS3SecretKey    string
	connS3IamRole      string
	connS3Region       string
	connS3Path         string
	connS3SessionToken string
	// BigQuery
	connBQKeyPath       string
	connBQParentProject string
	connBQDataset       string
	connBQQueryProject  string
	connBQQueryTable    string
	connBQMatDataset    string
	// Common
	connDescription string
	// Browse flags
	connDatabase string
	connTable    string
	connSchema   string
)

// --- Parent command ---

var connectorCmd = &cobra.Command{
	Use:     "connector",
	Aliases: []string{"conn"},
	Short:   "Manage storage connectors",
}

// --- List ---

var connectorListCmd = &cobra.Command{
	Use:   "list",
	Short: "List storage connectors",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		connectors, err := c.ListStorageConnectors()
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(connectors)
			return nil
		}

		headers := []string{"NAME", "TYPE", "DESCRIPTION"}
		var rows [][]string
		for _, sc := range connectors {
			rows = append(rows, []string{
				sc.Name,
				sc.StorageConnectorType,
				truncate(sc.Description, 50),
			})
		}
		output.Table(headers, rows)
		return nil
	},
}

// --- Info ---

var connectorInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show connector details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		sc, err := c.GetStorageConnector(args[0])
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(sc)
			return nil
		}

		output.Info("Connector: %s (%s)", sc.Name, sc.StorageConnectorType)
		if sc.Description != "" {
			output.Info("Description: %s", sc.Description)
		}

		switch sc.StorageConnectorType {
		case "SNOWFLAKE":
			output.Info("URL: %s", sc.URL)
			output.Info("Database: %s", sc.Database)
			output.Info("Schema: %s", sc.Schema)
			output.Info("Warehouse: %s", sc.Warehouse)
			if sc.User != "" {
				output.Info("User: %s", sc.User)
			}
			if sc.Role != "" {
				output.Info("Role: %s", sc.Role)
			}
		case "JDBC":
			output.Info("Connection String: %s", sc.ConnectionString)
			if len(sc.Arguments) > 0 {
				for _, arg := range sc.Arguments {
					if arg.Name == "password" {
						output.Info("  %s: ****", arg.Name)
					} else {
						output.Info("  %s: %s", arg.Name, arg.Value)
					}
				}
			}
		case "S3":
			output.Info("Bucket: %s", sc.Bucket)
			if sc.Region != "" {
				output.Info("Region: %s", sc.Region)
			}
			if sc.Path != "" {
				output.Info("Path: %s", sc.Path)
			}
			if sc.IamRole != "" {
				output.Info("IAM Role: %s", sc.IamRole)
			}
		case "BIGQUERY":
			output.Info("Parent Project: %s", sc.ParentProject)
			if sc.KeyPath != "" {
				output.Info("Key Path: %s", sc.KeyPath)
			}
			if sc.Dataset != "" {
				output.Info("Dataset: %s", sc.Dataset)
			}
			if sc.QueryProject != "" {
				output.Info("Query Project: %s", sc.QueryProject)
			}
			if sc.QueryTable != "" {
				output.Info("Query Table: %s", sc.QueryTable)
			}
			if sc.MaterializationDataset != "" {
				output.Info("Materialization Dataset: %s", sc.MaterializationDataset)
			}
		}

		return nil
	},
}

// --- Test ---

var connectorTestCmd = &cobra.Command{
	Use:   "test <name>",
	Short: "Test a connector by listing databases",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		// First verify connector exists
		sc, err := c.GetStorageConnector(args[0])
		if err != nil {
			return err
		}

		// Try listing databases as a connection test
		dbs, err := c.GetConnectorDatabases(args[0])
		if err != nil {
			return fmt.Errorf("connection test failed for %s (%s): %w", sc.Name, sc.StorageConnectorType, err)
		}

		if output.JSONMode {
			output.PrintJSON(map[string]interface{}{
				"status":    "connected",
				"connector": sc.Name,
				"type":      sc.StorageConnectorType,
				"databases": dbs,
			})
			return nil
		}

		output.Success("Connected to %s (%s)", sc.Name, sc.StorageConnectorType)
		if len(dbs) > 0 {
			output.Info("Found %d databases: %s", len(dbs), strings.Join(dbs, ", "))
		}
		return nil
	},
}

// --- Databases ---

var connectorDatabasesCmd = &cobra.Command{
	Use:   "databases <name>",
	Short: "List databases from a connector",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		dbs, err := c.GetConnectorDatabases(args[0])
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(dbs)
			return nil
		}

		headers := []string{"DATABASE"}
		var rows [][]string
		for _, db := range dbs {
			rows = append(rows, []string{db})
		}
		output.Table(headers, rows)
		return nil
	},
}

// --- Tables ---

var connectorTablesCmd = &cobra.Command{
	Use:   "tables <name>",
	Short: "List tables from a connector",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		tables, err := c.GetConnectorTables(args[0], connDatabase)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(tables)
			return nil
		}

		headers := []string{"DATABASE", "SCHEMA", "TABLE"}
		var rows [][]string
		for _, t := range tables {
			rows = append(rows, []string{t.Database, t.Group, t.Table})
		}
		output.Table(headers, rows)
		return nil
	},
}

// --- Preview ---

var connectorPreviewCmd = &cobra.Command{
	Use:   "preview <name>",
	Short: "Preview data from a connector's table",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if connTable == "" {
			return fmt.Errorf("--table is required")
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		ds := &client.DataSource{
			Database: connDatabase,
			Table:    connTable,
			Group:    connSchema,
		}

		result, err := c.GetConnectorData(args[0], ds)
		if err != nil {
			return err
		}

		previewRows := result.PreviewRows()

		if output.JSONMode {
			output.PrintJSON(previewRows)
			return nil
		}

		if len(previewRows) == 0 {
			output.Info("No data returned")
			return nil
		}

		// Use keys from first preview row as headers (preserves actual casing)
		var headers []string
		if len(previewRows[0]) > 0 {
			// Maintain feature order from API if available
			if len(result.Features) > 0 {
				for _, f := range result.Features {
					headers = append(headers, strings.ToLower(f.Name))
				}
			} else {
				for k := range previewRows[0] {
					headers = append(headers, k)
				}
			}
		}

		var rows [][]string
		for _, row := range previewRows {
			var r []string
			for _, h := range headers {
				r = append(r, row[h])
			}
			rows = append(rows, r)
		}
		output.Table(headers, rows)
		return nil
	},
}

// --- Delete ---

var connectorDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a storage connector",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		if err := c.DeleteStorageConnector(args[0]); err != nil {
			return err
		}

		output.Success("Deleted connector '%s'", args[0])
		return nil
	},
}

// --- Create parent ---

var connectorCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a storage connector (use subcommand: snowflake, jdbc, s3, bigquery)",
}

// --- Create Snowflake ---

var connectorCreateSnowflakeCmd = &cobra.Command{
	Use:   "snowflake <name>",
	Short: "Create a Snowflake storage connector",
	Long: `Create a Snowflake storage connector.

Examples:
  hops connector create snowflake my_sf \
    --url "https://xyz.snowflakecomputing.com" \
    --user admin --password secret123 \
    --database MY_DB --schema PUBLIC --warehouse MY_WH

  # With OAuth token instead of password
  hops connector create snowflake my_sf \
    --url "https://xyz.snowflakecomputing.com" \
    --token "eyJ..." \
    --database MY_DB --schema PUBLIC --warehouse MY_WH`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if connSFUrl == "" {
			return fmt.Errorf("--url is required")
		}
		if connSFDatabase == "" {
			return fmt.Errorf("--database is required")
		}
		if connSFSchema == "" {
			return fmt.Errorf("--schema is required")
		}
		if connSFWarehouse == "" {
			return fmt.Errorf("--warehouse is required")
		}
		if connSFPassword == "" && connSFToken == "" {
			return fmt.Errorf("--password or --token is required")
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		sc := &client.StorageConnector{
			Type:                 "featurestoreSnowflakeConnectorDTO",
			Name:                 args[0],
			Description:          connDescription,
			StorageConnectorType: "SNOWFLAKE",
			URL:                  connSFUrl,
			User:                 connSFUser,
			Password:             connSFPassword,
			Token:                connSFToken,
			Database:             connSFDatabase,
			Schema:               connSFSchema,
			Warehouse:            connSFWarehouse,
			Role:                 connSFRole,
			Table:                connSFTable,
		}

		created, err := c.CreateStorageConnector(sc)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(created)
			return nil
		}
		output.Success("Created Snowflake connector '%s' (ID: %d)", created.Name, created.ID)
		return nil
	},
}

// --- Create JDBC ---

var connectorCreateJDBCCmd = &cobra.Command{
	Use:   "jdbc <name>",
	Short: "Create a JDBC storage connector",
	Long: `Create a JDBC storage connector.

Examples:
  hops connector create jdbc my_db \
    --connection-string "jdbc:mysql://host:3306/mydb" \
    --arguments "user=admin,password=secret"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if connJDBCConnStr == "" {
			return fmt.Errorf("--connection-string is required")
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		var arguments []client.OptionDTO
		if connJDBCArgs != "" {
			for _, pair := range splitComma(connJDBCArgs) {
				parts := splitStr(pair, "=")
				if len(parts) != 2 {
					return fmt.Errorf("invalid argument %q: expected key=value", pair)
				}
				arguments = append(arguments, client.OptionDTO{
					Name:  trimSpace(parts[0]),
					Value: trimSpace(parts[1]),
				})
			}
		}

		sc := &client.StorageConnector{
			Type:                 "featurestoreJdbcConnectorDTO",
			Name:                 args[0],
			Description:          connDescription,
			StorageConnectorType: "JDBC",
			ConnectionString:     connJDBCConnStr,
			Arguments:            arguments,
		}

		created, err := c.CreateStorageConnector(sc)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(created)
			return nil
		}
		output.Success("Created JDBC connector '%s' (ID: %d)", created.Name, created.ID)
		return nil
	},
}

// --- Create S3 ---

var connectorCreateS3Cmd = &cobra.Command{
	Use:   "s3 <name>",
	Short: "Create an S3 storage connector",
	Long: `Create an S3 storage connector.

Examples:
  # With access keys
  hops connector create s3 my_bucket \
    --bucket my-data-bucket \
    --access-key AKIA... --secret-key wJal... \
    --region us-west-2

  # With IAM role
  hops connector create s3 my_bucket \
    --bucket my-data-bucket \
    --iam-role "arn:aws:iam::123:role/MyRole"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if connS3Bucket == "" {
			return fmt.Errorf("--bucket is required")
		}
		if connS3AccessKey == "" && connS3IamRole == "" {
			return fmt.Errorf("--access-key or --iam-role is required")
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		sc := &client.StorageConnector{
			Type:                 "featurestoreS3ConnectorDTO",
			Name:                 args[0],
			Description:          connDescription,
			StorageConnectorType: "S3",
			Bucket:               connS3Bucket,
			AccessKey:            connS3AccessKey,
			SecretKey:            connS3SecretKey,
			IamRole:              connS3IamRole,
			Region:               connS3Region,
			Path:                 connS3Path,
			SessionToken:         connS3SessionToken,
		}

		created, err := c.CreateStorageConnector(sc)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(created)
			return nil
		}
		output.Success("Created S3 connector '%s' (ID: %d)", created.Name, created.ID)
		return nil
	},
}

// --- Create BigQuery ---

var connectorCreateBigQueryCmd = &cobra.Command{
	Use:   "bigquery <name>",
	Short: "Create a BigQuery storage connector",
	Long: `Create a BigQuery storage connector.

The key file must already be uploaded to HopsFS (e.g. /Projects/<project>/Resources/key.json).
Use "hops fs upload" or copy it to /hopsfs/Resources/ first.

Examples:
  # With materialization dataset (most common)
  hops connector create bigquery my_bq \
    --key-path Resources/bq-key.json \
    --parent-project hops-20 \
    --materialization-dataset my_views

  # With explicit query target
  hops connector create bigquery my_bq \
    --key-path Resources/bq-key.json \
    --parent-project hops-20 \
    --query-project hops-20 --dataset my_dataset --query-table my_table`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if connBQKeyPath == "" {
			return fmt.Errorf("--key-path is required (HDFS path to service account JSON)")
		}
		if connBQParentProject == "" {
			return fmt.Errorf("--parent-project is required (GCP project ID)")
		}
		// Validate: either (queryProject+dataset+queryTable) or materializationDataset
		hasQuery := connBQQueryProject != "" || connBQDataset != "" || connBQQueryTable != ""
		hasMat := connBQMatDataset != ""
		if !hasQuery && !hasMat {
			return fmt.Errorf("--materialization-dataset or --query-project/--dataset/--query-table is required")
		}
		if hasQuery && (connBQQueryProject == "" || connBQDataset == "" || connBQQueryTable == "") {
			return fmt.Errorf("--query-project, --dataset, and --query-table must all be provided together")
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		sc := &client.StorageConnector{
			Type:                   "featurestoreBigqueryConnectorDTO",
			Name:                   args[0],
			Description:            connDescription,
			StorageConnectorType:   "BIGQUERY",
			KeyPath:                connBQKeyPath,
			ParentProject:          connBQParentProject,
			Dataset:                connBQDataset,
			QueryProject:           connBQQueryProject,
			QueryTable:             connBQQueryTable,
			MaterializationDataset: connBQMatDataset,
		}

		created, err := c.CreateStorageConnector(sc)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(created)
			return nil
		}
		output.Success("Created BigQuery connector '%s' (ID: %d)", created.Name, created.ID)
		return nil
	},
}

// --- Registration ---

func init() {
	rootCmd.AddCommand(connectorCmd)

	// List
	connectorCmd.AddCommand(connectorListCmd)

	// Info
	connectorCmd.AddCommand(connectorInfoCmd)

	// Test
	connectorCmd.AddCommand(connectorTestCmd)

	// Databases
	connectorCmd.AddCommand(connectorDatabasesCmd)

	// Tables
	connectorTablesCmd.Flags().StringVar(&connDatabase, "database", "", "Filter by database")
	connectorCmd.AddCommand(connectorTablesCmd)

	// Preview
	connectorPreviewCmd.Flags().StringVar(&connDatabase, "database", "", "Database name")
	connectorPreviewCmd.Flags().StringVar(&connTable, "table", "", "Table name")
	connectorPreviewCmd.Flags().StringVar(&connSchema, "schema", "", "Schema name")
	connectorCmd.AddCommand(connectorPreviewCmd)

	// Delete
	connectorCmd.AddCommand(connectorDeleteCmd)

	// Create parent
	connectorCmd.AddCommand(connectorCreateCmd)

	// Create Snowflake
	connectorCreateSnowflakeCmd.Flags().StringVar(&connSFUrl, "url", "", "Snowflake account URL")
	connectorCreateSnowflakeCmd.Flags().StringVar(&connSFUser, "user", "", "Snowflake user")
	connectorCreateSnowflakeCmd.Flags().StringVar(&connSFPassword, "password", "", "Snowflake password")
	connectorCreateSnowflakeCmd.Flags().StringVar(&connSFToken, "token", "", "OAuth/SSO token")
	connectorCreateSnowflakeCmd.Flags().StringVar(&connSFDatabase, "database", "", "Database name")
	connectorCreateSnowflakeCmd.Flags().StringVar(&connSFSchema, "schema", "", "Schema name")
	connectorCreateSnowflakeCmd.Flags().StringVar(&connSFWarehouse, "warehouse", "", "Warehouse name")
	connectorCreateSnowflakeCmd.Flags().StringVar(&connSFRole, "role", "", "Snowflake role")
	connectorCreateSnowflakeCmd.Flags().StringVar(&connSFTable, "table", "", "Default table")
	connectorCreateSnowflakeCmd.Flags().StringVar(&connDescription, "description", "", "Connector description")
	connectorCreateCmd.AddCommand(connectorCreateSnowflakeCmd)

	// Create JDBC
	connectorCreateJDBCCmd.Flags().StringVar(&connJDBCConnStr, "connection-string", "", "JDBC connection string")
	connectorCreateJDBCCmd.Flags().StringVar(&connJDBCArgs, "arguments", "", "Key=value pairs (comma-separated)")
	connectorCreateJDBCCmd.Flags().StringVar(&connDescription, "description", "", "Connector description")
	connectorCreateCmd.AddCommand(connectorCreateJDBCCmd)

	// Create S3
	connectorCreateS3Cmd.Flags().StringVar(&connS3Bucket, "bucket", "", "S3 bucket name")
	connectorCreateS3Cmd.Flags().StringVar(&connS3AccessKey, "access-key", "", "AWS access key")
	connectorCreateS3Cmd.Flags().StringVar(&connS3SecretKey, "secret-key", "", "AWS secret key")
	connectorCreateS3Cmd.Flags().StringVar(&connS3IamRole, "iam-role", "", "IAM role ARN")
	connectorCreateS3Cmd.Flags().StringVar(&connS3Region, "region", "", "AWS region")
	connectorCreateS3Cmd.Flags().StringVar(&connS3Path, "path", "", "Path prefix in bucket")
	connectorCreateS3Cmd.Flags().StringVar(&connS3SessionToken, "session-token", "", "Temporary session token")
	connectorCreateS3Cmd.Flags().StringVar(&connDescription, "description", "", "Connector description")
	connectorCreateCmd.AddCommand(connectorCreateS3Cmd)

	// Create BigQuery
	connectorCreateBigQueryCmd.Flags().StringVar(&connBQKeyPath, "key-path", "", "HDFS path to service account key JSON")
	connectorCreateBigQueryCmd.Flags().StringVar(&connBQParentProject, "parent-project", "", "GCP project ID")
	connectorCreateBigQueryCmd.Flags().StringVar(&connBQDataset, "dataset", "", "BigQuery dataset")
	connectorCreateBigQueryCmd.Flags().StringVar(&connBQQueryProject, "query-project", "", "Query project")
	connectorCreateBigQueryCmd.Flags().StringVar(&connBQQueryTable, "query-table", "", "Query table")
	connectorCreateBigQueryCmd.Flags().StringVar(&connBQMatDataset, "materialization-dataset", "", "Materialization dataset")
	connectorCreateBigQueryCmd.Flags().StringVar(&connDescription, "description", "", "Connector description")
	connectorCreateCmd.AddCommand(connectorCreateBigQueryCmd)
}
