package cmd

import (
	"fmt"
	"strconv"

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

func init() {
	rootCmd.AddCommand(tdCmd)

	tdCreateCmd.Flags().StringVar(&tdCreateDesc, "description", "", "Description")
	tdCreateCmd.Flags().StringVar(&tdCreateFormat, "format", "parquet", "Data format (parquet, csv, tfrecord)")

	tdCmd.AddCommand(tdListCmd)
	tdCmd.AddCommand(tdCreateCmd)
	tdCmd.AddCommand(tdDeleteCmd)
}
