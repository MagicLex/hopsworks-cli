package cmd

import (
	"fmt"
	"os"

	"github.com/MagicLex/hopsworks-cli/pkg/config"
	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags
var Version = "0.8.6"

var (
	cfg       *config.Config
	flagHost   string
	flagAPIKey string
	flagProject string
	flagJSON   bool
)

var rootCmd = &cobra.Command{
	Use:     "hops",
	Short:   "Hopsworks CLI — for humans and LLMs",
	Version: Version,
	Long: `hops is a CLI for the Hopsworks Feature Store platform.

Works for both humans (pretty tables) and LLMs (--json).
Run 'hops init' to set up Claude Code integration.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		output.JSONMode = flagJSON

		// Load config
		var err error
		cfg, err = config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load config: %v\n", err)
			cfg = &config.Config{}
		}

		// CLI flags override config/env
		if flagHost != "" {
			cfg.Host = flagHost
		}
		if flagAPIKey != "" {
			cfg.APIKey = flagAPIKey
		}
		if flagProject != "" {
			cfg.Project = flagProject
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagHost, "host", "", "Hopsworks host URL")
	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "Hopsworks API key")
	rootCmd.PersistentFlags().StringVar(&flagProject, "project", "", "Project name")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output as JSON (for LLMs)")
}
