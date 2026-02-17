package cmd

import (
	"fmt"
	"strings"

	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Dump project context (for LLMs)",
	Long:  `Outputs a structured summary of the current project state — feature groups, feature views, and schemas. Designed for LLM consumption.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		var sb strings.Builder

		// Project info
		sb.WriteString(fmt.Sprintf("# Hopsworks Project: %s\n", cfg.Project))
		sb.WriteString(fmt.Sprintf("Host: %s | Project ID: %d | Feature Store ID: %d\n\n", cfg.Host, cfg.ProjectID, cfg.FeatureStoreID))

		// Feature Groups
		fgs, err := c.ListFeatureGroups()
		if err != nil {
			sb.WriteString(fmt.Sprintf("Feature Groups: error fetching (%v)\n\n", err))
		} else {
			sb.WriteString(fmt.Sprintf("## Feature Groups (%d)\n\n", len(fgs)))
			for _, fg := range fgs {
				online := ""
				if fg.OnlineEnabled {
					online = " [online]"
				}
				sb.WriteString(fmt.Sprintf("### %s v%d%s\n", fg.Name, fg.Version, online))
				if fg.Description != "" {
					sb.WriteString(fmt.Sprintf("%s\n", fg.Description))
				}
				if len(fg.Features) > 0 {
					sb.WriteString("Schema: ")
					var cols []string
					for _, f := range fg.Features {
						pk := ""
						if f.Primary {
							pk = " (PK)"
						}
						cols = append(cols, fmt.Sprintf("%s:%s%s", f.Name, f.Type, pk))
					}
					sb.WriteString(strings.Join(cols, ", "))
					sb.WriteString("\n")
				}
				sb.WriteString("\n")
			}
		}

		// Feature Views
		fvs, err := c.ListFeatureViews()
		if err != nil {
			sb.WriteString(fmt.Sprintf("Feature Views: error fetching (%v)\n\n", err))
		} else {
			sb.WriteString(fmt.Sprintf("## Feature Views (%d)\n\n", len(fvs)))
			for _, fv := range fvs {
				sb.WriteString(fmt.Sprintf("### %s v%d\n", fv.Name, fv.Version))
				if fv.Description != "" {
					sb.WriteString(fmt.Sprintf("%s\n", fv.Description))
				}
				if len(fv.Features) > 0 {
					var cols []string
					for _, f := range fv.Features {
						cols = append(cols, fmt.Sprintf("%s:%s", f.Name, f.Type))
					}
					sb.WriteString(fmt.Sprintf("Features: %s\n", strings.Join(cols, ", ")))
				}
				if len(fv.Labels) > 0 {
					sb.WriteString(fmt.Sprintf("Labels: %s\n", strings.Join(fv.Labels, ", ")))
				}
				sb.WriteString("\n")
			}
		}

		// Jobs
		jobs, err := c.ListJobs()
		if err == nil && len(jobs) > 0 {
			sb.WriteString(fmt.Sprintf("## Jobs (%d)\n\n", len(jobs)))
			for _, j := range jobs {
				sb.WriteString(fmt.Sprintf("- %s (%s)\n", j.Name, j.JobType))
			}
			sb.WriteString("\n")
		}

		result := sb.String()

		if output.JSONMode {
			output.PrintJSON(map[string]interface{}{
				"project":        cfg.Project,
				"project_id":     cfg.ProjectID,
				"host":           cfg.Host,
				"feature_groups": fgs,
				"feature_views":  fvs,
				"jobs":           jobs,
			})
			return nil
		}

		fmt.Print(result)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(contextCmd)
}
