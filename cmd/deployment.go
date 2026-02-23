package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/MagicLex/hopsworks-cli/pkg/client"
	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var (
	deployInstances int
	deployScript    string
	deployVersion   int
	deployName      string
	deployTail      int
	deployData      string
	deployComponent string
)

var deploymentCmd = &cobra.Command{
	Use:     "deployment",
	Aliases: []string{"deploy"},
	Short:   "Manage model deployments",
}

var deployListCmd = &cobra.Command{
	Use:   "list",
	Short: "List deployments",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		deployments, err := c.ListDeployments()
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(deployments)
			return nil
		}

		if len(deployments) == 0 {
			output.Info("No deployments")
			return nil
		}

		headers := []string{"ID", "NAME", "MODEL", "VERSION", "STATUS", "INSTANCES"}
		var rows [][]string
		for _, d := range deployments {
			rows = append(rows, []string{
				strconv.Itoa(d.ID),
				d.Name,
				d.ModelName,
				strconv.Itoa(d.ModelVersion),
				d.Status,
				strconv.Itoa(d.RequestedInstances),
			})
		}
		output.Table(headers, rows)
		return nil
	},
}

var deployInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show deployment details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		d, err := c.GetDeploymentByName(args[0])
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(d)
			return nil
		}

		output.Info("Deployment: %s", d.Name)
		output.Info("ID: %d", d.ID)
		output.Info("Model: %s (v%d)", d.ModelName, d.ModelVersion)
		output.Info("Status: %s", d.Status)
		if d.ModelServer != "" {
			output.Info("Server: %s", d.ModelServer)
		}
		if d.ServingTool != "" {
			output.Info("Serving Tool: %s", d.ServingTool)
		}
		output.Info("Instances: %d", d.RequestedInstances)
		if d.Created != nil {
			if ts, ok := d.Created.(float64); ok && ts > 0 {
				output.Info("Created: %s", fmtEpochMs(int64(ts)))
			} else if s, ok := d.Created.(string); ok && s != "" {
				output.Info("Created: %s", s)
			}
		}
		return nil
	},
}

var deployCreateCmd = &cobra.Command{
	Use:   "create <model-name>",
	Short: "Create a deployment from a registered model",
	Long: `Create a deployment via the REST API.

Examples:
  hops deployment create fraud_detector
  hops deployment create fraud_detector --version 1 --name fraud_serving --instances 1
  hops deployment create fraud_detector --script predictor.py`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modelName := args[0]

		c, err := mustClient()
		if err != nil {
			return err
		}

		// Resolve model version
		m, err := c.GetModel(modelName, deployVersion)
		if err != nil {
			return fmt.Errorf("model '%s' not found: %w", modelName, err)
		}

		name := deployName
		if name == "" {
			// Serving names must be alphanumeric only [a-zA-Z0-9]+
			name = strings.ReplaceAll(m.Name, "_", "")
			name = strings.ReplaceAll(name, "-", "")
		}

		// Map framework string to what the API expects
		framework := m.Framework
		if framework == "" {
			framework = "PYTHON"
		}

		// Model server: PYTHON for custom, TF_SERVING for tensorflow
		modelServer := "PYTHON"
		switch strings.ToUpper(framework) {
		case "TENSORFLOW":
			modelServer = "TF_SERVING"
		}

		// Resolve predictor script path (relative to model version Files dir)
		predictorPath := deployScript
		if predictorPath != "" && !strings.HasPrefix(predictorPath, "/") {
			predictorPath = fmt.Sprintf("/Projects/%s/Models/%s/%d/Files/%s",
				c.Config.Project, m.Name, m.Version, deployScript)
		}

		req := &client.DeploymentCreateRequest{
			Name:               name,
			ModelServer:        modelServer,
			ServingTool:        "KSERVE",
			ModelName:          m.Name,
			ModelPath:          fmt.Sprintf("/Projects/%s/Models/%s", c.Config.Project, m.Name),
			ModelVersion:       m.Version,
			ModelFramework:     framework,
			Predictor:          predictorPath,
			RequestedInstances: deployInstances,
			APIProtocol:        "REST",
			InferenceLogging:   "NONE",
			BatchingConfig:     &client.DeployBatchingConfig{BatchingEnabled: false},
			PredictorResources: &client.DeployResourceConfig{
				Requests: client.DeployResources{Cores: 0.2, Memory: 256, GPUs: 0},
				Limits:   client.DeployResources{Cores: 1.0, Memory: 1024, GPUs: 0},
			},
		}

		if !output.JSONMode {
			output.Info("Creating deployment '%s' for %s v%d...", name, m.Name, m.Version)
		}

		d, err := c.CreateDeployment(req)
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(d)
		} else {
			output.Success("Deployment '%s' created (id: %d)", d.Name, d.ID)
		}
		return nil
	},
}

var deployStartCmd = &cobra.Command{
	Use:   "start <name>",
	Short: "Start a deployment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		d, err := c.GetDeploymentByName(args[0])
		if err != nil {
			return err
		}

		if err := c.DeploymentAction(d.ID, "START"); err != nil {
			return err
		}

		output.Success("Started deployment '%s'", args[0])
		return nil
	},
}

var deployStopCmd = &cobra.Command{
	Use:   "stop <name>",
	Short: "Stop a deployment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		d, err := c.GetDeploymentByName(args[0])
		if err != nil {
			return err
		}

		if err := c.DeploymentAction(d.ID, "STOP"); err != nil {
			return err
		}

		output.Success("Stopped deployment '%s'", args[0])
		return nil
	},
}

var deployPredictCmd = &cobra.Command{
	Use:   "predict <name>",
	Short: "Send a prediction request",
	Long: `Send prediction data to a running deployment.

Examples:
  hops deployment predict fraud_serving --data '{"instances": [[1, 2, 3]]}'
  hops deployment predict fraud_serving --data '[1.0, 2.0, 3.0]'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if deployData == "" {
			return fmt.Errorf("--data is required")
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		// Validate JSON
		var payload json.RawMessage
		if err := json.Unmarshal([]byte(deployData), &payload); err != nil {
			return fmt.Errorf("invalid JSON data: %w", err)
		}

		result, err := c.Predict(args[0], payload)
		if err != nil {
			return err
		}

		if output.JSONMode {
			fmt.Println(string(result))
		} else {
			// Pretty-print
			var pretty json.RawMessage
			if json.Unmarshal(result, &pretty) == nil {
				formatted, _ := json.MarshalIndent(pretty, "", "  ")
				fmt.Println(string(formatted))
			} else {
				fmt.Println(string(result))
			}
		}
		return nil
	},
}

var deployLogsCmd = &cobra.Command{
	Use:   "logs <name>",
	Short: "View deployment logs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		d, err := c.GetDeploymentByName(args[0])
		if err != nil {
			return err
		}

		logs, err := c.DeploymentLogs(d.ID, deployComponent, deployTail)
		if err != nil {
			return err
		}

		fmt.Println(logs)
		return nil
	},
}

var deployDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a deployment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		d, err := c.GetDeploymentByName(args[0])
		if err != nil {
			return err
		}

		if err := c.DeleteDeployment(d.ID); err != nil {
			return err
		}

		output.Success("Deleted deployment '%s'", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deploymentCmd)

	deployCreateCmd.Flags().IntVar(&deployVersion, "version", 0, "Model version (latest if omitted)")
	deployCreateCmd.Flags().StringVar(&deployName, "name", "", "Deployment name (default: model name)")
	deployCreateCmd.Flags().IntVar(&deployInstances, "instances", 1, "Number of instances")
	deployCreateCmd.Flags().StringVar(&deployScript, "script", "", "Predictor script file")

	deployPredictCmd.Flags().StringVar(&deployData, "data", "", "Prediction data (JSON)")

	deployLogsCmd.Flags().IntVar(&deployTail, "tail", 50, "Number of log lines")
	deployLogsCmd.Flags().StringVar(&deployComponent, "component", "predictor", "Log component (predictor, transformer)")

	deploymentCmd.AddCommand(deployListCmd)
	deploymentCmd.AddCommand(deployInfoCmd)
	deploymentCmd.AddCommand(deployCreateCmd)
	deploymentCmd.AddCommand(deployStartCmd)
	deploymentCmd.AddCommand(deployStopCmd)
	deploymentCmd.AddCommand(deployPredictCmd)
	deploymentCmd.AddCommand(deployLogsCmd)
	deploymentCmd.AddCommand(deployDeleteCmd)
}
