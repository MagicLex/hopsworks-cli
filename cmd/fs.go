package cmd

import (
	"strconv"

	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var fsCmd = &cobra.Command{
	Use:   "fs",
	Short: "Manage feature stores",
}

var fsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List feature stores in the project",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		stores, err := c.ListFeatureStores()
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(stores)
			return nil
		}

		headers := []string{"NAME", "ID", "ACTIVE"}
		var rows [][]string
		for _, s := range stores {
			active := ""
			if s.FeaturestoreID == cfg.FeatureStoreID {
				active = "*"
			}
			rows = append(rows, []string{
				s.FeaturestoreName,
				strconv.Itoa(s.FeaturestoreID),
				active,
			})
		}
		output.Table(headers, rows)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(fsCmd)
	fsCmd.AddCommand(fsListCmd)
}
