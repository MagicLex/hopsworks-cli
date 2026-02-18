package cmd

import (
	"fmt"
	"time"

	"github.com/MagicLex/hopsworks-cli/pkg/client"
	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var (
	jobStatusWait bool
	jobStatusPoll int
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

var jobStatusCmd = &cobra.Command{
	Use:   "status <name>",
	Short: "Show latest execution status for a job",
	Long: `Show the latest execution status for a job, with optional polling.

Examples:
  # Check materialization job status
  hops job status customer_transactions_1_offline_fg_materialization

  # Wait for job to finish (poll every 10s)
  hops job status customer_transactions_1_offline_fg_materialization --wait

  # Poll every 5 seconds
  hops job status customer_transactions_1_offline_fg_materialization --wait --poll 5`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobName := args[0]

		c, err := mustClient()
		if err != nil {
			return err
		}

		for {
			exec, err := c.GetLatestExecution(jobName)
			if err != nil {
				return err
			}
			if exec == nil {
				output.Info("No executions found for '%s'", jobName)
				return nil
			}

			if output.JSONMode {
				output.PrintJSON(exec)
				if !jobStatusWait || client.IsExecutionTerminal(exec.State) {
					return nil
				}
				time.Sleep(time.Duration(jobStatusPoll) * time.Second)
				continue
			}

			dur := formatDuration(exec.Duration)
			fmt.Printf("  Job:      %s\n", jobName)
			fmt.Printf("  Exec:     #%d\n", exec.ID)
			fmt.Printf("  State:    %s\n", exec.State)
			fmt.Printf("  Status:   %s\n", exec.FinalStatus)
			fmt.Printf("  Duration: %s\n", dur)
			fmt.Printf("  Started:  %s\n", exec.SubmissionTime)

			if client.IsExecutionTerminal(exec.State) {
				if exec.FinalStatus == "SUCCEEDED" {
					output.Success("Job finished successfully in %s", dur)
				} else {
					output.Error("Job %s (%s)", exec.State, exec.FinalStatus)
				}
				return nil
			}

			if !jobStatusWait {
				return nil
			}

			fmt.Printf("  Polling every %ds...\n\n", jobStatusPoll)
			time.Sleep(time.Duration(jobStatusPoll) * time.Second)
		}
	},
}

func formatDuration(ms int64) string {
	if ms == 0 {
		return "-"
	}
	d := time.Duration(ms) * time.Millisecond
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}

func init() {
	rootCmd.AddCommand(jobCmd)
	jobCmd.AddCommand(jobListCmd)

	jobStatusCmd.Flags().BoolVar(&jobStatusWait, "wait", false, "Poll until job finishes")
	jobStatusCmd.Flags().IntVar(&jobStatusPoll, "poll", 10, "Poll interval in seconds")
	jobCmd.AddCommand(jobStatusCmd)
}
