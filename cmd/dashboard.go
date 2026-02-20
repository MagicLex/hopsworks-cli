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
	dashChartID int
	dashWidth   int
	dashHeight  int
	dashX       int
	dashY       int
)

// --- Parent command ---

var dashboardCmd = &cobra.Command{
	Use:     "dashboard",
	Aliases: []string{"dash"},
	Short:   "Manage dashboards",
}

// --- List ---

var dashboardListCmd = &cobra.Command{
	Use:   "list",
	Short: "List dashboards",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		dashboards, err := c.ListDashboards()
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(dashboards)
			return nil
		}

		headers := []string{"ID", "NAME", "CHARTS"}
		var rows [][]string
		for _, d := range dashboards {
			rows = append(rows, []string{
				strconv.Itoa(d.ID),
				d.Name,
				strconv.Itoa(len(d.Charts)),
			})
		}
		output.Table(headers, rows)
		return nil
	},
}

// --- Info ---

var dashboardInfoCmd = &cobra.Command{
	Use:   "info <id>",
	Short: "Show dashboard details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid dashboard ID: %s", args[0])
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		d, err := c.GetDashboard(id)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(d)
			return nil
		}

		output.Info("Dashboard: %s (ID: %d)", d.Name, d.ID)
		output.Info("Charts: %d", len(d.Charts))

		if len(d.Charts) > 0 {
			fmt.Println()
			headers := []string{"ID", "TITLE", "SIZE", "POSITION"}
			var rows [][]string
			for _, ch := range d.Charts {
				size := fmt.Sprintf("%dx%d", ch.Width, ch.Height)
				pos := fmt.Sprintf("(%d, %d)", ch.X, ch.Y)
				rows = append(rows, []string{
					strconv.Itoa(ch.ID),
					ch.Title,
					size,
					pos,
				})
			}
			output.Table(headers, rows)
		}
		return nil
	},
}

// --- Create ---

var dashboardCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create an empty dashboard",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		d := &client.Dashboard{Name: args[0]}

		created, err := c.CreateDashboard(d)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(created)
			return nil
		}
		output.Success("Created dashboard '%s' (ID: %d)", created.Name, created.ID)
		return nil
	},
}

// --- Delete ---

var dashboardDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a dashboard",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid dashboard ID: %s", args[0])
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		if err := c.DeleteDashboard(id); err != nil {
			return err
		}

		output.Success("Deleted dashboard %d", id)
		return nil
	},
}

// --- Add Chart ---

var dashboardAddChartCmd = &cobra.Command{
	Use:   "add-chart <dashboard-id>",
	Short: "Add a chart to a dashboard",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dashID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid dashboard ID: %s", args[0])
		}
		if dashChartID == 0 {
			return fmt.Errorf("--chart-id is required")
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		// GET current dashboard
		d, err := c.GetDashboard(dashID)
		if err != nil {
			return err
		}

		// Check if chart already in dashboard
		for _, ch := range d.Charts {
			if ch.ID == dashChartID {
				return fmt.Errorf("chart %d is already in dashboard %d", dashChartID, dashID)
			}
		}

		// Fetch full chart to populate fields (API may require full objects)
		ch, err := c.GetChart(dashChartID)
		if err != nil {
			return err
		}

		// Set layout — defaults if not provided (DB columns are NOT NULL)
		if cmd.Flags().Changed("width") {
			ch.Width = dashWidth
		} else if ch.Width == 0 {
			ch.Width = 12
		}
		if cmd.Flags().Changed("height") {
			ch.Height = dashHeight
		} else if ch.Height == 0 {
			ch.Height = 8
		}
		if cmd.Flags().Changed("x") {
			ch.X = dashX
		}
		if cmd.Flags().Changed("y") {
			ch.Y = dashY
		}

		d.Charts = append(d.Charts, *ch)

		updated, err := c.UpdateDashboard(dashID, d)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(updated)
			return nil
		}
		output.Success("Added chart %d to dashboard '%s'", dashChartID, updated.Name)
		return nil
	},
}

// --- Remove Chart ---

var dashboardRemoveChartCmd = &cobra.Command{
	Use:   "remove-chart <dashboard-id>",
	Short: "Remove a chart from a dashboard",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dashID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid dashboard ID: %s", args[0])
		}
		if dashChartID == 0 {
			return fmt.Errorf("--chart-id is required")
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		// GET current dashboard
		d, err := c.GetDashboard(dashID)
		if err != nil {
			return err
		}

		// Filter out the chart
		found := false
		var filtered []client.Chart
		for _, ch := range d.Charts {
			if ch.ID == dashChartID {
				found = true
				continue
			}
			filtered = append(filtered, ch)
		}
		if !found {
			return fmt.Errorf("chart %d not found in dashboard %d", dashChartID, dashID)
		}

		d.Charts = filtered

		updated, err := c.UpdateDashboard(dashID, d)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(updated)
			return nil
		}
		output.Success("Removed chart %d from dashboard '%s'", dashChartID, updated.Name)
		return nil
	},
}

// --- Registration ---

func init() {
	rootCmd.AddCommand(dashboardCmd)

	// List
	dashboardCmd.AddCommand(dashboardListCmd)

	// Info
	dashboardCmd.AddCommand(dashboardInfoCmd)

	// Create
	dashboardCmd.AddCommand(dashboardCreateCmd)

	// Delete
	dashboardCmd.AddCommand(dashboardDeleteCmd)

	// Add chart
	dashboardAddChartCmd.Flags().IntVar(&dashChartID, "chart-id", 0, "Chart ID to add (required)")
	dashboardAddChartCmd.Flags().IntVar(&dashWidth, "width", 0, "Chart width in dashboard")
	dashboardAddChartCmd.Flags().IntVar(&dashHeight, "height", 0, "Chart height in dashboard")
	dashboardAddChartCmd.Flags().IntVar(&dashX, "x", 0, "Chart X position in dashboard")
	dashboardAddChartCmd.Flags().IntVar(&dashY, "y", 0, "Chart Y position in dashboard")
	dashboardCmd.AddCommand(dashboardAddChartCmd)

	// Remove chart
	dashboardRemoveChartCmd.Flags().IntVar(&dashChartID, "chart-id", 0, "Chart ID to remove (required)")
	dashboardCmd.AddCommand(dashboardRemoveChartCmd)
}
