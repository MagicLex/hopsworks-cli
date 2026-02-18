package cmd

import (
	"fmt"
	"strings"

	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var (
	modelRegFramework   string
	modelRegMetrics     string
	modelRegDesc        string
	modelRegFV          string
	modelRegTDVersion   int
	modelRegVersion     int
	modelDownloadOutput string
	modelDownloadVer    int
)

var modelRegisterCmd = &cobra.Command{
	Use:   "register <name> <path>",
	Short: "Register a model and upload artifacts",
	Long: `Register a model in the model registry and upload artifacts.

Examples:
  hops model register fraud_detector ./model_dir
  hops model register fraud_detector ./model_dir \
    --framework sklearn \
    --metrics "accuracy=0.95,f1=0.88" \
    --description "Fraud detection v1"
  hops model register my_model ./model_dir \
    --feature-view my_view --td-version 1`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		path := args[1]

		if !output.JSONMode {
			output.Info("Registering model '%s' from %s...", name, path)
		}

		script := buildModelRegisterScript(name, path)
		if err := runPython(script); err != nil {
			return fmt.Errorf("model registration failed: %w", err)
		}
		return nil
	},
}

func buildModelRegisterScript(name, path string) string {
	var sb strings.Builder
	sb.WriteString(`import hopsworks, warnings, logging, json, sys
warnings.filterwarnings("ignore")
logging.getLogger("hsfs").setLevel(logging.WARNING)
logging.getLogger("hopsworks").setLevel(logging.WARNING)

project = hopsworks.login()
mr = project.get_model_registry()
`)

	if modelRegFV != "" {
		sb.WriteString("fs = project.get_feature_store()\n")
	}

	// Determine framework create method
	createMethod := "python.create_model"
	switch modelRegFramework {
	case "sklearn":
		createMethod = "sklearn.create_model"
	case "tensorflow", "tf":
		createMethod = "tensorflow.create_model"
	case "torch", "pytorch":
		createMethod = "torch.create_model"
	}

	// Parse metrics
	var metricsStr string
	if modelRegMetrics != "" {
		var pairs []string
		for _, kv := range splitComma(modelRegMetrics) {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				pairs = append(pairs, fmt.Sprintf("%q: float(%q)", trimSpace(parts[0]), trimSpace(parts[1])))
			}
		}
		metricsStr = "{" + strings.Join(pairs, ", ") + "}"
	}

	// Build create_model call
	sb.WriteString(fmt.Sprintf("\nmodel = mr.%s(\n", createMethod))
	sb.WriteString(fmt.Sprintf("    name=%q,\n", name))
	if modelRegVersion > 0 {
		sb.WriteString(fmt.Sprintf("    version=%d,\n", modelRegVersion))
	}
	if metricsStr != "" {
		sb.WriteString(fmt.Sprintf("    metrics=%s,\n", metricsStr))
	}
	if modelRegDesc != "" {
		sb.WriteString(fmt.Sprintf("    description=%q,\n", modelRegDesc))
	}
	if modelRegFV != "" {
		sb.WriteString(fmt.Sprintf("    feature_view=fs.get_feature_view(name=%q),\n", modelRegFV))
	}
	if modelRegTDVersion > 0 {
		sb.WriteString(fmt.Sprintf("    training_dataset_version=%d,\n", modelRegTDVersion))
	}
	sb.WriteString(")\n")

	sb.WriteString(fmt.Sprintf("\nmodel.save(%q)\n", path))
	sb.WriteString(`print(f"Model '{model.name}' v{model.version} registered successfully", file=sys.stderr)
`)
	return sb.String()
}

var modelDownloadCmd = &cobra.Command{
	Use:   "download <name>",
	Short: "Download model artifacts",
	Long: `Download model artifacts to a local directory.

Examples:
  hops model download fraud_detector
  hops model download fraud_detector --version 1 --output ./local_dir`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if !output.JSONMode {
			output.Info("Downloading model '%s'...", name)
		}

		script := buildModelDownloadScript(name)
		if err := runPython(script); err != nil {
			return fmt.Errorf("model download failed: %w", err)
		}
		return nil
	},
}

func buildModelDownloadScript(name string) string {
	var sb strings.Builder
	sb.WriteString(`import hopsworks, warnings, logging, sys
warnings.filterwarnings("ignore")
logging.getLogger("hsfs").setLevel(logging.WARNING)
logging.getLogger("hopsworks").setLevel(logging.WARNING)

project = hopsworks.login()
mr = project.get_model_registry()
`)

	if modelDownloadVer > 0 {
		sb.WriteString(fmt.Sprintf("\nmodel = mr.get_model(name=%q, version=%d)\n", name, modelDownloadVer))
	} else {
		sb.WriteString(fmt.Sprintf("\n_models = mr.get_models(name=%q)\n", name))
		sb.WriteString("if not _models:\n")
		sb.WriteString(fmt.Sprintf("    raise ValueError('No model found with name %q')\n", name))
		sb.WriteString("model = sorted(_models, key=lambda m: m.version, reverse=True)[0]\n")
	}

	if modelDownloadOutput != "" {
		sb.WriteString(fmt.Sprintf("path = model.download(%q)\n", modelDownloadOutput))
	} else {
		sb.WriteString("path = model.download()\n")
	}
	sb.WriteString("print(f'Downloaded to {path}', file=sys.stderr)\n")

	return sb.String()
}

func init() {
	modelRegisterCmd.Flags().StringVar(&modelRegFramework, "framework", "python", "Model framework (python, sklearn, tensorflow, torch)")
	modelRegisterCmd.Flags().StringVar(&modelRegMetrics, "metrics", "", `Training metrics: "key=value,..."`)
	modelRegisterCmd.Flags().StringVar(&modelRegDesc, "description", "", "Model description")
	modelRegisterCmd.Flags().StringVar(&modelRegFV, "feature-view", "", "Link to feature view (provenance)")
	modelRegisterCmd.Flags().IntVar(&modelRegTDVersion, "td-version", 0, "Training dataset version (with --feature-view)")
	modelRegisterCmd.Flags().IntVar(&modelRegVersion, "version", 0, "Model version (default: auto-increment)")

	modelDownloadCmd.Flags().IntVar(&modelDownloadVer, "version", 0, "Model version (latest if omitted)")
	modelDownloadCmd.Flags().StringVar(&modelDownloadOutput, "output", "", "Download directory")

	modelCmd.AddCommand(modelRegisterCmd)
	modelCmd.AddCommand(modelDownloadCmd)
}
