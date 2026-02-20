package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MagicLex/hopsworks-cli/pkg/client"
	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

// --- Flag variables ---

var (
	jobStatusWait bool
	jobStatusPoll int

	// create flags
	jobCreateType        string
	jobCreateAppPath     string
	jobCreateArgs        string
	jobCreateEnvName     string
	jobCreateDriverMem   int
	jobCreateDriverCores int
	jobCreateExecMem     int
	jobCreateExecCores   int
	jobCreateExecutors   int
	jobCreateDynamic     bool
	jobCreateMemory      int
	jobCreateCores       int
	jobCreateGPUs        int
	jobCreateWorkerMem   int
	jobCreateWorkerCores int
	jobCreateWorkersMin  int
	jobCreateWorkersMax  int
	jobCreateMainClass   string

	// run flags
	jobRunArgs string
	jobRunWait bool

	// stop flags
	jobStopExecID int

	// logs flags
	jobLogsExecID int
	jobLogsType   string

	// history flags
	jobHistoryLimit int
)

var jobCmd = &cobra.Command{
	Use:   "job",
	Short: "Manage jobs",
}

// --- list ---

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
			if jobType == "" {
				jobType = j.ConfigType()
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

// --- info ---

var jobInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show job details and configuration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		job, err := c.GetJob(args[0])
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(job)
			return nil
		}

		jobType := job.JobType
		if jobType == "" {
			jobType = job.ConfigType()
		}
		creator := ""
		if job.Creator != nil {
			creator = job.Creator.Email
		}

		fmt.Printf("  Name:    %s\n", job.Name)
		fmt.Printf("  ID:      %d\n", job.ID)
		fmt.Printf("  Type:    %s\n", jobType)
		fmt.Printf("  Created: %s\n", job.CreationTime)
		fmt.Printf("  Creator: %s\n", creator)

		// Pretty-print config
		if job.Config != nil {
			var cfg map[string]interface{}
			if json.Unmarshal(job.Config, &cfg) == nil {
				fmt.Println("\n  Config:")
				for k, v := range cfg {
					fmt.Printf("    %-24s %v\n", k+":", v)
				}
			}
		}
		return nil
	},
}

// --- create ---

var jobCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new job",
	Long: `Create a new job with the given type and script path.

Examples:
  hops job create my_job --type python --app-path Resources/jobs/script.py
  hops job create spark_job --type pyspark --app-path Resources/jobs/etl.py --driver-mem 2048 --executors 4
  hops job create ray_job --type ray --app-path Resources/jobs/train.py --workers-max 4`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if jobCreateType == "" {
			return fmt.Errorf("--type is required (python, pyspark, spark, ray)")
		}
		if jobCreateAppPath == "" {
			return fmt.Errorf("--app-path is required")
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		// Fetch default config for this job type
		defaultCfg, err := c.GetDefaultConfig(jobCreateType)
		if err != nil {
			return fmt.Errorf("get default config for %s: %w", jobCreateType, err)
		}

		// Unmarshal to map so we can overlay user flags
		var cfg map[string]interface{}
		if err := json.Unmarshal(defaultCfg, &cfg); err != nil {
			return fmt.Errorf("parse default config: %w", err)
		}

		// Common fields
		cfg["appName"] = name
		cfg["appPath"] = jobCreateAppPath
		cfg["jobType"] = strings.ToUpper(jobCreateType)
		if cmd.Flags().Changed("args") {
			cfg["defaultArgs"] = jobCreateArgs
		}
		if cmd.Flags().Changed("env-name") {
			cfg["environmentName"] = jobCreateEnvName
		}

		// Type-specific overlays
		switch jobCreateType {
		case "python":
			// type discriminator already set by default config
		case "pyspark":
			cfg["mainClass"] = "org.apache.spark.deploy.PythonRunner"
			if cmd.Flags().Changed("driver-mem") {
				cfg["spark.driver.memory"] = jobCreateDriverMem
			}
			if cmd.Flags().Changed("driver-cores") {
				cfg["spark.driver.cores"] = jobCreateDriverCores
			}
			if cmd.Flags().Changed("executor-mem") {
				cfg["spark.executor.memory"] = jobCreateExecMem
			}
			if cmd.Flags().Changed("executor-cores") {
				cfg["spark.executor.cores"] = jobCreateExecCores
			}
			if cmd.Flags().Changed("executors") {
				cfg["spark.executor.instances"] = jobCreateExecutors
			}
			if cmd.Flags().Changed("dynamic") {
				cfg["spark.dynamicAllocation.enabled"] = jobCreateDynamic
			}
		case "spark":
			if jobCreateMainClass != "" {
				cfg["mainClass"] = jobCreateMainClass
			}
			if cmd.Flags().Changed("driver-mem") {
				cfg["spark.driver.memory"] = jobCreateDriverMem
			}
			if cmd.Flags().Changed("driver-cores") {
				cfg["spark.driver.cores"] = jobCreateDriverCores
			}
			if cmd.Flags().Changed("executor-mem") {
				cfg["spark.executor.memory"] = jobCreateExecMem
			}
			if cmd.Flags().Changed("executor-cores") {
				cfg["spark.executor.cores"] = jobCreateExecCores
			}
			if cmd.Flags().Changed("executors") {
				cfg["spark.executor.instances"] = jobCreateExecutors
			}
			if cmd.Flags().Changed("dynamic") {
				cfg["spark.dynamicAllocation.enabled"] = jobCreateDynamic
			}
		case "ray":
			if cmd.Flags().Changed("driver-mem") {
				cfg["driverMemory"] = jobCreateDriverMem
			}
			if cmd.Flags().Changed("driver-cores") {
				cfg["driverCores"] = jobCreateDriverCores
			}
			if cmd.Flags().Changed("worker-mem") {
				cfg["workerMemory"] = jobCreateWorkerMem
			}
			if cmd.Flags().Changed("worker-cores") {
				cfg["workerCores"] = jobCreateWorkerCores
			}
			if cmd.Flags().Changed("workers-min") {
				cfg["workerMinInstances"] = jobCreateWorkersMin
			}
			if cmd.Flags().Changed("workers-max") {
				cfg["workerMaxInstances"] = jobCreateWorkersMax
			}
		}

		cfgJSON, err := json.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("marshal config: %w", err)
		}

		job, err := c.CreateJob(name, cfgJSON)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(job)
			return nil
		}
		output.Success("Created job '%s' (ID: %d, type: %s)", job.Name, job.ID, jobCreateType)
		return nil
	},
}

// --- run ---

var jobRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Run a job (start new execution)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobName := args[0]

		c, err := mustClient()
		if err != nil {
			return err
		}

		exec, err := c.RunJob(jobName, jobRunArgs)
		if err != nil {
			return err
		}

		if output.JSONMode && !jobRunWait {
			output.PrintJSON(exec)
			return nil
		}

		if !output.JSONMode {
			output.Success("Started execution #%d for '%s'", exec.ID, jobName)
		}

		if !jobRunWait {
			return nil
		}

		// Poll until terminal
		return pollExecution(c, jobName, jobStatusPoll)
	},
}

// --- stop ---

var jobStopCmd = &cobra.Command{
	Use:   "stop <name>",
	Short: "Stop a running job execution",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobName := args[0]

		c, err := mustClient()
		if err != nil {
			return err
		}

		execID := jobStopExecID
		if execID == 0 {
			// Get latest execution
			latest, err := c.GetLatestExecution(jobName)
			if err != nil {
				return err
			}
			if latest == nil {
				return fmt.Errorf("no executions found for '%s'", jobName)
			}
			if client.IsExecutionTerminal(latest.State) {
				return fmt.Errorf("latest execution #%d is already %s", latest.ID, latest.State)
			}
			execID = latest.ID
		}

		exec, err := c.StopExecution(jobName, execID)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(exec)
			return nil
		}
		output.Success("Stopped execution #%d (%s)", exec.ID, exec.State)
		return nil
	},
}

// --- logs ---

var jobLogsCmd = &cobra.Command{
	Use:   "logs <name>",
	Short: "Show logs for a job execution",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobName := args[0]

		c, err := mustClient()
		if err != nil {
			return err
		}

		execID := jobLogsExecID
		if execID == 0 {
			latest, err := c.GetLatestExecution(jobName)
			if err != nil {
				return err
			}
			if latest == nil {
				return fmt.Errorf("no executions found for '%s'", jobName)
			}
			execID = latest.ID
		}

		log, err := c.GetExecutionLogs(jobName, execID, jobLogsType)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(log)
			return nil
		}

		if log.Log == "" {
			output.Info("No %s logs for execution #%d", jobLogsType, execID)
			return nil
		}
		fmt.Print(log.Log)
		return nil
	},
}

// --- history ---

var jobHistoryCmd = &cobra.Command{
	Use:   "history <name>",
	Short: "List executions for a job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobName := args[0]

		c, err := mustClient()
		if err != nil {
			return err
		}

		execs, err := c.GetExecutions(jobName, jobHistoryLimit)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(execs)
			return nil
		}

		headers := []string{"ID", "STATE", "STATUS", "DURATION", "SUBMITTED"}
		var rows [][]string
		for _, e := range execs {
			rows = append(rows, []string{
				strconv.Itoa(e.ID),
				e.State,
				e.FinalStatus,
				formatDuration(e.Duration),
				e.SubmissionTime,
			})
		}
		output.Table(headers, rows)
		return nil
	},
}

// --- delete ---

var jobDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobName := args[0]

		c, err := mustClient()
		if err != nil {
			return err
		}

		if err := c.DeleteJob(jobName); err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(map[string]string{"deleted": jobName})
			return nil
		}
		output.Success("Deleted job '%s'", jobName)
		return nil
	},
}

// --- schedule ---

var (
	jobSchedCron  string
	jobSchedStart string
	jobSchedEnd   string
)

var jobScheduleCmd = &cobra.Command{
	Use:   "schedule <name> <cron>",
	Short: "Create or update a job schedule (cron)",
	Long: `Schedule a job using a cron expression.

Uses Quartz 6-field cron format: SEC MIN HOUR DAY MONTH WEEKDAY

Examples:
  hops job schedule my_job "0 0 * * * ?"             # every hour
  hops job schedule my_job "0 30 8 * * ?"             # daily at 08:30
  hops job schedule my_job "0 0 0 * * MON-FRI"        # midnight on weekdays
  hops job schedule my_job "0 */15 * * * ?"            # every 15 min
  hops job schedule my_job "0 0 6 * * ?" --start "2026-03-01T00:00:00Z"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobName := args[0]
		cron := args[1]

		c, err := mustClient()
		if err != nil {
			return err
		}

		now := time.Now().UnixMilli()
		enabled := true
		sched := &client.JobSchedule{
			CronExpression: cron,
			StartDateTime:  &now,
			Enabled:        &enabled,
		}

		if jobSchedStart != "" {
			t, err := time.Parse(time.RFC3339, jobSchedStart)
			if err != nil {
				return fmt.Errorf("--start must be RFC3339 (e.g. 2026-03-01T00:00:00Z): %w", err)
			}
			ms := t.UnixMilli()
			sched.StartDateTime = &ms
		}
		if jobSchedEnd != "" {
			t, err := time.Parse(time.RFC3339, jobSchedEnd)
			if err != nil {
				return fmt.Errorf("--end must be RFC3339 (e.g. 2026-12-31T23:59:59Z): %w", err)
			}
			ms := t.UnixMilli()
			sched.EndDateTime = &ms
		}

		// Try create first; if schedule exists, fetch ID and update
		result, err := c.CreateSchedule(jobName, sched)
		if err != nil {
			// Already exists — fetch to get the ID, then update
			existing, getErr := c.GetSchedule(jobName)
			if getErr != nil {
				return err // return original create error
			}
			sched.ID = existing.ID
			result, err = c.UpdateSchedule(jobName, sched)
			if err != nil {
				return err
			}
		}

		if output.JSONMode {
			output.PrintJSON(result)
			return nil
		}
		next := "-"
		if result.NextExecutionDateTime != nil {
			next = time.UnixMilli(*result.NextExecutionDateTime).UTC().Format(time.RFC3339)
		}
		output.Success("Scheduled '%s': %s (next: %s)", jobName, cron, next)
		return nil
	},
}

var jobScheduleInfoCmd = &cobra.Command{
	Use:   "schedule-info <name>",
	Short: "Show a job's schedule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		sched, err := c.GetSchedule(args[0])
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(sched)
			return nil
		}

		fmt.Printf("  Job:    %s\n", args[0])
		fmt.Printf("  Cron:   %s\n", sched.CronExpression)
		if sched.Enabled != nil {
			fmt.Printf("  Active: %v\n", *sched.Enabled)
		}
		if sched.StartDateTime != nil {
			fmt.Printf("  Start:  %s\n", time.UnixMilli(*sched.StartDateTime).UTC().Format(time.RFC3339))
		}
		if sched.EndDateTime != nil {
			fmt.Printf("  End:    %s\n", time.UnixMilli(*sched.EndDateTime).UTC().Format(time.RFC3339))
		}
		if sched.NextExecutionDateTime != nil {
			fmt.Printf("  Next:   %s\n", time.UnixMilli(*sched.NextExecutionDateTime).UTC().Format(time.RFC3339))
		}
		return nil
	},
}

var jobUnscheduleCmd = &cobra.Command{
	Use:   "unschedule <name>",
	Short: "Remove a job's schedule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		if err := c.DeleteSchedule(args[0]); err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(map[string]string{"unscheduled": args[0]})
			return nil
		}
		output.Success("Removed schedule for '%s'", args[0])
		return nil
	},
}

// --- status (existing) ---

var jobStatusCmd = &cobra.Command{
	Use:   "status <name>",
	Short: "Show latest execution status for a job",
	Long: `Show the latest execution status for a job, with optional polling.

Examples:
  hops job status my_job
  hops job status my_job --wait
  hops job status my_job --wait --poll 5`,
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

// --- helpers ---

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

func pollExecution(c *client.Client, jobName string, pollSec int) error {
	if pollSec <= 0 {
		pollSec = 10
	}
	for {
		exec, err := c.GetLatestExecution(jobName)
		if err != nil {
			return err
		}
		if exec == nil {
			return fmt.Errorf("execution disappeared")
		}

		if output.JSONMode {
			output.PrintJSON(exec)
		} else {
			fmt.Printf("  #%d  %s  %s  %s\n", exec.ID, exec.State, exec.FinalStatus, formatDuration(exec.Duration))
		}

		if client.IsExecutionTerminal(exec.State) {
			if exec.FinalStatus == "SUCCEEDED" {
				output.Success("Job finished successfully")
			} else {
				output.Error("Job %s (%s)", exec.State, exec.FinalStatus)
			}
			return nil
		}

		time.Sleep(time.Duration(pollSec) * time.Second)
	}
}

// --- registration ---

func init() {
	rootCmd.AddCommand(jobCmd)
	jobCmd.AddCommand(jobListCmd)
	jobCmd.AddCommand(jobInfoCmd)

	// create
	jobCreateCmd.Flags().StringVar(&jobCreateType, "type", "", "Job type: python, pyspark, spark, ray (required)")
	jobCreateCmd.Flags().StringVar(&jobCreateAppPath, "app-path", "", "Script path in HopsFS (required)")
	jobCreateCmd.Flags().StringVar(&jobCreateArgs, "args", "", "Default arguments")
	jobCreateCmd.Flags().StringVar(&jobCreateEnvName, "env-name", "", "Conda environment name")
	// python
	jobCreateCmd.Flags().IntVar(&jobCreateMemory, "memory", 2048, "Memory in MB (python)")
	jobCreateCmd.Flags().IntVar(&jobCreateCores, "cores", 1, "CPU cores (python)")
	jobCreateCmd.Flags().IntVar(&jobCreateGPUs, "gpus", 0, "GPUs (python)")
	// pyspark/spark
	jobCreateCmd.Flags().IntVar(&jobCreateDriverMem, "driver-mem", 2048, "Driver memory in MB")
	jobCreateCmd.Flags().IntVar(&jobCreateDriverCores, "driver-cores", 1, "Driver cores")
	jobCreateCmd.Flags().IntVar(&jobCreateExecMem, "executor-mem", 4096, "Executor memory in MB")
	jobCreateCmd.Flags().IntVar(&jobCreateExecCores, "executor-cores", 1, "Executor cores")
	jobCreateCmd.Flags().IntVar(&jobCreateExecutors, "executors", 2, "Number of executors")
	jobCreateCmd.Flags().BoolVar(&jobCreateDynamic, "dynamic", true, "Enable dynamic allocation (spark)")
	// ray
	jobCreateCmd.Flags().IntVar(&jobCreateWorkerMem, "worker-mem", 2048, "Worker memory in MB (ray)")
	jobCreateCmd.Flags().IntVar(&jobCreateWorkerCores, "worker-cores", 1, "Worker cores (ray)")
	jobCreateCmd.Flags().IntVar(&jobCreateWorkersMin, "workers-min", 0, "Min workers (ray)")
	jobCreateCmd.Flags().IntVar(&jobCreateWorkersMax, "workers-max", 2, "Max workers (ray)")
	jobCreateCmd.Flags().StringVar(&jobCreateMainClass, "main-class", "", "Main class (spark JARs)")
	jobCmd.AddCommand(jobCreateCmd)

	// run
	jobRunCmd.Flags().StringVar(&jobRunArgs, "args", "", "Execution arguments (overrides default)")
	jobRunCmd.Flags().BoolVar(&jobRunWait, "wait", false, "Wait for execution to finish")
	jobRunCmd.Flags().IntVar(&jobStatusPoll, "poll", 10, "Poll interval in seconds (with --wait)")
	jobCmd.AddCommand(jobRunCmd)

	// stop
	jobStopCmd.Flags().IntVar(&jobStopExecID, "exec", 0, "Specific execution ID (default: latest)")
	jobCmd.AddCommand(jobStopCmd)

	// logs
	jobLogsCmd.Flags().IntVar(&jobLogsExecID, "exec", 0, "Specific execution ID (default: latest)")
	jobLogsCmd.Flags().StringVar(&jobLogsType, "type", "out", "Log type: out or err")
	jobCmd.AddCommand(jobLogsCmd)

	// history
	jobHistoryCmd.Flags().IntVar(&jobHistoryLimit, "limit", 10, "Max executions to show")
	jobCmd.AddCommand(jobHistoryCmd)

	// delete
	jobCmd.AddCommand(jobDeleteCmd)

	// schedule
	jobScheduleCmd.Flags().StringVar(&jobSchedStart, "start", "", "Start time (RFC3339, default: now)")
	jobScheduleCmd.Flags().StringVar(&jobSchedEnd, "end", "", "End time (RFC3339, optional)")
	jobCmd.AddCommand(jobScheduleCmd)
	jobCmd.AddCommand(jobScheduleInfoCmd)
	jobCmd.AddCommand(jobUnscheduleCmd)

	// status (existing)
	jobStatusCmd.Flags().BoolVar(&jobStatusWait, "wait", false, "Poll until job finishes")
	jobStatusCmd.Flags().IntVar(&jobStatusPoll, "poll", 10, "Poll interval in seconds")
	jobCmd.AddCommand(jobStatusCmd)
}
