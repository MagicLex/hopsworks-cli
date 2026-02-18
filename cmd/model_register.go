package cmd

import (
	"fmt"
	"strings"

	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var (
	modelRegFramework    string
	modelRegMetrics      string
	modelRegDesc         string
	modelRegFV           string
	modelRegTDVersion    int
	modelRegVersion      int
	modelRegInputExample string
	modelRegSchema       string
	modelRegProgram      string
	modelDownloadOutput  string
	modelDownloadVer     int
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
    --feature-view my_view --td-version 1
  hops model register my_model ./model_dir \
    --input-example sample.json \
    --schema "in:age:int,salary:float out:prediction:float"
  hops model register my_model ./model_dir \
    --feature-view my_view --td-version 1 \
    --input-example sample.json --program train.py`,
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

	// Schema imports + setup
	needsSchema := modelRegSchema != "" || modelRegInputExample != ""
	if needsSchema {
		sb.WriteString("from hsml.schema import Schema\n")
		sb.WriteString("from hsml.model_schema import ModelSchema\n")
	}

	// Parse explicit schema: "in:name:type,name:type out:name:type"
	if modelRegSchema != "" {
		inSchema, outSchema := parseModelSchema(modelRegSchema)
		if len(inSchema) > 0 {
			sb.WriteString(fmt.Sprintf("_input_schema = Schema(%s)\n", schemaToListLiteral(inSchema)))
		}
		if len(outSchema) > 0 {
			sb.WriteString(fmt.Sprintf("_output_schema = Schema(%s)\n", schemaToListLiteral(outSchema)))
		}
		sb.WriteString("_model_schema = ModelSchema(")
		if len(inSchema) > 0 {
			sb.WriteString("input_schema=_input_schema")
		}
		if len(inSchema) > 0 && len(outSchema) > 0 {
			sb.WriteString(", ")
		}
		if len(outSchema) > 0 {
			sb.WriteString("output_schema=_output_schema")
		}
		sb.WriteString(")\n")
	}

	// Load input example from file
	if modelRegInputExample != "" {
		if strings.HasSuffix(modelRegInputExample, ".csv") {
			sb.WriteString("import pandas as pd\n")
			sb.WriteString(fmt.Sprintf("_input_example = pd.read_csv(%q)\n", modelRegInputExample))
		} else {
			sb.WriteString(fmt.Sprintf("with open(%q) as _f:\n", modelRegInputExample))
			sb.WriteString("    _input_example = json.load(_f)\n")
		}
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
	if modelRegInputExample != "" {
		sb.WriteString("    input_example=_input_example,\n")
	}
	if modelRegSchema != "" {
		sb.WriteString("    model_schema=_model_schema,\n")
	}
	if modelRegProgram != "" {
		sb.WriteString(fmt.Sprintf("    program=%q,\n", modelRegProgram))
	}
	sb.WriteString(")\n")

	sb.WriteString(fmt.Sprintf("\nmodel.save(%q)\n", path))
	sb.WriteString(`print(f"Model '{model.name}' v{model.version} registered successfully", file=sys.stderr)
`)
	return sb.String()
}

// parseModelSchema parses "in:name:type,name:type out:name:type" into input/output field lists.
func parseModelSchema(spec string) (input, output []schemaField) {
	for _, part := range strings.Fields(spec) {
		if strings.HasPrefix(part, "in:") {
			input = parseSchemaFields(strings.TrimPrefix(part, "in:"))
		} else if strings.HasPrefix(part, "out:") {
			output = parseSchemaFields(strings.TrimPrefix(part, "out:"))
		}
	}
	return
}

type schemaField struct {
	Name string
	Type string
}

func parseSchemaFields(s string) []schemaField {
	var fields []schemaField
	for _, f := range splitComma(s) {
		parts := strings.SplitN(trimSpace(f), ":", 2)
		if len(parts) == 2 {
			fields = append(fields, schemaField{Name: trimSpace(parts[0]), Type: trimSpace(parts[1])})
		}
	}
	return fields
}

func schemaToListLiteral(fields []schemaField) string {
	var items []string
	for _, f := range fields {
		items = append(items, fmt.Sprintf(`{"name": %q, "type": %q}`, f.Name, f.Type))
	}
	return "[" + strings.Join(items, ", ") + "]"
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
	modelRegisterCmd.Flags().StringVar(&modelRegFV, "feature-view", "", "Link to feature view (provenance + auto schema)")
	modelRegisterCmd.Flags().IntVar(&modelRegTDVersion, "td-version", 0, "Training dataset version (with --feature-view)")
	modelRegisterCmd.Flags().IntVar(&modelRegVersion, "version", 0, "Model version (default: auto-increment)")
	modelRegisterCmd.Flags().StringVar(&modelRegInputExample, "input-example", "", "Input example file (JSON or CSV)")
	modelRegisterCmd.Flags().StringVar(&modelRegSchema, "schema", "", `Model schema: "in:name:type,... out:name:type,..."`)
	modelRegisterCmd.Flags().StringVar(&modelRegProgram, "program", "", "Training script path (stored as metadata)")

	modelDownloadCmd.Flags().IntVar(&modelDownloadVer, "version", 0, "Model version (latest if omitted)")
	modelDownloadCmd.Flags().StringVar(&modelDownloadOutput, "output", "", "Download directory")

	modelCmd.AddCommand(modelRegisterCmd)
	modelCmd.AddCommand(modelDownloadCmd)
}
