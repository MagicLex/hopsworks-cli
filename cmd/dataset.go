package cmd

import (
	"fmt"
	"strconv"

	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var datasetCmd = &cobra.Command{
	Use:   "dataset",
	Short: "Browse project files",
}

var datasetListCmd = &cobra.Command{
	Use:   "list [path]",
	Short: "List files in a dataset path",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := ""
		if len(args) > 0 {
			path = args[0]
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		files, err := c.ListDatasets(path)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(files)
			return nil
		}

		headers := []string{"NAME", "TYPE", "SIZE"}
		var rows [][]string
		for _, f := range files {
			t := "file"
			if f.Dir {
				t = "dir"
			}
			rows = append(rows, []string{f.Name, t, humanSize(f.Size)})
		}
		output.Table(headers, rows)
		return nil
	},
}

var datasetMkdirCmd = &cobra.Command{
	Use:   "mkdir <path>",
	Short: "Create a directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		if err := c.MkDir(args[0]); err != nil {
			return err
		}

		output.Success("Created directory: %s", args[0])
		return nil
	},
}

func humanSize(bytes int64) string {
	if bytes == 0 {
		return "-"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	size := float64(bytes)
	i := 0
	for size >= 1024 && i < len(units)-1 {
		size /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", bytes)
	}
	s := strconv.FormatFloat(size, 'f', 1, 64)
	return s + " " + units[i]
}

func init() {
	rootCmd.AddCommand(datasetCmd)
	datasetCmd.AddCommand(datasetListCmd)
	datasetCmd.AddCommand(datasetMkdirCmd)
}
