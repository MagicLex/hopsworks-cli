package cmd

import (
	"fmt"
	"strconv"

	"github.com/MagicLex/hopsworks-cli/pkg/client"
	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

// --- Flag variables ---

var (
	chartURL         string
	chartDescription string
	chartTitle       string
	chartJobID       int
	chartWidth       int
	chartHeight      int
	chartX           int
	chartY           int
)

// --- Parent command ---

var chartCmd = &cobra.Command{
	Use:   "chart",
	Short: "Manage charts",
}

// --- List ---

var chartListCmd = &cobra.Command{
	Use:   "list",
	Short: "List charts",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		charts, err := c.ListCharts()
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(charts)
			return nil
		}

		headers := []string{"ID", "TITLE", "URL", "JOB", "DESCRIPTION"}
		var rows [][]string
		for _, ch := range charts {
			job := ""
			if ch.Job != nil {
				job = ch.Job.Name
			}
			rows = append(rows, []string{
				strconv.Itoa(ch.ID),
				ch.Title,
				truncate(ch.URL, 40),
				job,
				truncate(ch.Description, 40),
			})
		}
		output.Table(headers, rows)
		return nil
	},
}

// --- Info ---

var chartInfoCmd = &cobra.Command{
	Use:   "info <id>",
	Short: "Show chart details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid chart ID: %s", args[0])
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		ch, err := c.GetChart(id)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(ch)
			return nil
		}

		output.Info("Chart: %s (ID: %d)", ch.Title, ch.ID)
		output.Info("URL: %s", ch.URL)
		output.Info("Description: %s", ch.Description)
		if ch.Job != nil {
			output.Info("Job: %s (ID: %d)", ch.Job.Name, ch.Job.ID)
		}
		if ch.Width > 0 || ch.Height > 0 {
			output.Info("Size: %dx%d", ch.Width, ch.Height)
		}
		if ch.X > 0 || ch.Y > 0 {
			output.Info("Position: (%d, %d)", ch.X, ch.Y)
		}
		return nil
	},
}

// --- Create ---

var chartCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a chart",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if chartURL == "" {
			return fmt.Errorf("--url is required")
		}
		if chartDescription == "" {
			return fmt.Errorf("--description is required")
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		ch := &client.Chart{
			Title:       args[0],
			URL:         chartURL,
			Description: chartDescription,
			Width:       chartWidth,
			Height:      chartHeight,
			X:           chartX,
			Y:           chartY,
		}
		if chartJobID > 0 {
			ch.Job = &client.JobRef{ID: chartJobID}
		}

		created, err := c.CreateChart(ch)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(created)
			return nil
		}
		output.Success("Created chart '%s' (ID: %d)", created.Title, created.ID)
		return nil
	},
}

// --- Update ---

var chartUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a chart",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid chart ID: %s", args[0])
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		// GET current state
		ch, err := c.GetChart(id)
		if err != nil {
			return err
		}

		// Overlay changed flags
		if cmd.Flags().Changed("title") {
			ch.Title = chartTitle
		}
		if cmd.Flags().Changed("url") {
			ch.URL = chartURL
		}
		if cmd.Flags().Changed("description") {
			ch.Description = chartDescription
		}
		if cmd.Flags().Changed("job-id") {
			if chartJobID > 0 {
				ch.Job = &client.JobRef{ID: chartJobID}
			} else {
				ch.Job = nil
			}
		}
		if cmd.Flags().Changed("width") {
			ch.Width = chartWidth
		}
		if cmd.Flags().Changed("height") {
			ch.Height = chartHeight
		}
		if cmd.Flags().Changed("x") {
			ch.X = chartX
		}
		if cmd.Flags().Changed("y") {
			ch.Y = chartY
		}

		updated, err := c.UpdateChart(id, ch)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(updated)
			return nil
		}
		output.Success("Updated chart '%s' (ID: %d)", updated.Title, updated.ID)
		return nil
	},
}

// --- Delete ---

var chartDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a chart",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid chart ID: %s", args[0])
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		if err := c.DeleteChart(id); err != nil {
			return err
		}

		output.Success("Deleted chart %d", id)
		return nil
	},
}

// --- Registration ---

func init() {
	rootCmd.AddCommand(chartCmd)

	// List
	chartCmd.AddCommand(chartListCmd)

	// Info
	chartCmd.AddCommand(chartInfoCmd)

	// Create
	chartCreateCmd.Flags().StringVar(&chartURL, "url", "", "Chart URL (required)")
	chartCreateCmd.Flags().StringVar(&chartDescription, "description", "", "Chart description (required)")
	chartCreateCmd.Flags().IntVar(&chartJobID, "job-id", 0, "Associated job ID")
	chartCreateCmd.Flags().IntVar(&chartWidth, "width", 0, "Chart width")
	chartCreateCmd.Flags().IntVar(&chartHeight, "height", 0, "Chart height")
	chartCreateCmd.Flags().IntVar(&chartX, "x", 0, "Chart X position")
	chartCreateCmd.Flags().IntVar(&chartY, "y", 0, "Chart Y position")
	chartCmd.AddCommand(chartCreateCmd)

	// Update
	chartUpdateCmd.Flags().StringVar(&chartTitle, "title", "", "Chart title")
	chartUpdateCmd.Flags().StringVar(&chartURL, "url", "", "Chart URL")
	chartUpdateCmd.Flags().StringVar(&chartDescription, "description", "", "Chart description")
	chartUpdateCmd.Flags().IntVar(&chartJobID, "job-id", 0, "Associated job ID")
	chartUpdateCmd.Flags().IntVar(&chartWidth, "width", 0, "Chart width")
	chartUpdateCmd.Flags().IntVar(&chartHeight, "height", 0, "Chart height")
	chartUpdateCmd.Flags().IntVar(&chartX, "x", 0, "Chart X position")
	chartUpdateCmd.Flags().IntVar(&chartY, "y", 0, "Chart Y position")
	chartCmd.AddCommand(chartUpdateCmd)

	// Delete
	chartCmd.AddCommand(chartDeleteCmd)
}
