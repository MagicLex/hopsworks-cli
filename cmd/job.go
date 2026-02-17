package cmd

import (
	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var jobCmd = &cobra.Command{
	Use:   "job",
	Short: "Manage jobs",
}

var jobListCmd = &cobra.Command{
	Use:   "list",
	Short: "List jobs in the project",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		jobs, err := c.ListJobs()
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(jobs)
			return nil
		}

		headers := []string{"NAME", "TYPE", "CREATOR"}
		var rows [][]string
		for _, j := range jobs {
			jobType := j.JobType
			if jobType == "" && j.Config != nil {
				jobType = j.Config.Type
			}
			creator := ""
			if j.Creator != nil {
				creator = j.Creator.Email
			}
			rows = append(rows, []string{j.Name, jobType, creator})
		}
		output.Table(headers, rows)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(jobCmd)
	jobCmd.AddCommand(jobListCmd)
}
