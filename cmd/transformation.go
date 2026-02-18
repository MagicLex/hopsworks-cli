package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/MagicLex/hopsworks-cli/pkg/client"
	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

var tfCmd = &cobra.Command{
	Use:     "transformation",
	Aliases: []string{"tf"},
	Short:   "Manage transformation functions",
}

var tfListCmd = &cobra.Command{
	Use:   "list",
	Short: "List transformation functions",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := mustClient()
		if err != nil {
			return err
		}

		tfs, err := c.ListTransformationFunctions()
		if err != nil {
			return err
		}

		if output.JSONMode {
			output.PrintJSON(tfs)
			return nil
		}

		headers := []string{"NAME", "VERSION", "INPUTS", "OUTPUT", "MODE"}
		var rows [][]string
		for _, tf := range tfs {
			inputs := strings.Join(tf.HopsworksUdf.TransformationFunctionArgumentNames, ", ")
			outputType := strings.Join(tf.HopsworksUdf.OutputTypes, ", ")
			rows = append(rows, []string{
				tf.HopsworksUdf.Name,
				strconv.Itoa(tf.Version),
				inputs,
				outputType,
				tf.HopsworksUdf.ExecutionMode,
			})
		}
		output.Table(headers, rows)
		return nil
	},
}

var (
	tfCreateFile    string
	tfCreateCode    string
	tfCreateVersion int
)

var tfCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Register a custom transformation function",
	Long: `Register a custom transformation function from a Python file or inline code.

The function must use the @udf decorator from hopsworks.

Examples:
  # From file
  hops transformation create --file my_scaler.py

  # Inline
  hops transformation create --code '@udf(float)
  def double_it(value):
      return value * 2'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if tfCreateFile == "" && tfCreateCode == "" {
			return fmt.Errorf("--file or --code is required")
		}

		var pythonSource string
		if tfCreateFile != "" {
			data, err := os.ReadFile(tfCreateFile)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
			pythonSource = string(data)
		} else {
			pythonSource = tfCreateCode
		}

		// Use Python to parse the UDF and extract metadata
		metadata, err := parseUdfMetadata(pythonSource)
		if err != nil {
			return fmt.Errorf("parse UDF: %w", err)
		}

		c, err := mustClient()
		if err != nil {
			return err
		}

		version := tfCreateVersion
		if version == 0 {
			version = 1
		}

		tf := &client.TransformationFunction{
			Version: version,
			HopsworksUdf: client.HopsworksUdf{
				SourceCode:                          pythonSource,
				Name:                                metadata.Name,
				OutputTypes:                         metadata.OutputTypes,
				TransformationFeatures:              []string{},
				TransformationFunctionArgumentNames: metadata.ArgNames,
				ExecutionMode:                       "DEFAULT",
			},
		}

		result, err := c.CreateTransformationFunction(tf)
		if err != nil {
			return err
		}

		// Save a local copy for reference
		saveTransformLocally(metadata.Name, pythonSource)

		if output.JSONMode {
			output.PrintJSON(result)
			return nil
		}
		output.Success("Registered transformation '%s' v%d (ID: %d)", metadata.Name, result.Version, result.ID)
		return nil
	},
}

type udfMetadata struct {
	Name        string   `json:"name"`
	ArgNames    []string `json:"arg_names"`
	OutputTypes []string `json:"output_types"`
}

// parseUdfMetadata runs a Python script to introspect the @udf decorated function.
func parseUdfMetadata(source string) (*udfMetadata, error) {
	// Python script that parses the UDF source and extracts metadata
	parseScript := fmt.Sprintf(`
import json, ast, re, sys

source = %q

# Parse AST to find function definitions
tree = ast.parse(source)
func_def = None
decorator_args = None

for node in ast.walk(tree):
    if isinstance(node, ast.FunctionDef):
        func_def = node
        # Check decorators for @udf(...)
        for dec in node.decorator_list:
            if isinstance(dec, ast.Call):
                dec_name = ""
                if isinstance(dec.func, ast.Name):
                    dec_name = dec.func.id
                elif isinstance(dec.func, ast.Attribute):
                    dec_name = dec.func.attr
                if dec_name == "udf":
                    decorator_args = dec.args
        break

if func_def is None:
    print(json.dumps({"error": "no function definition found"}))
    sys.exit(1)

# Extract function name and arguments
name = func_def.name
arg_names = [a.arg for a in func_def.args.args]

# Extract return types from @udf decorator
type_map = {"float": "double", "int": "bigint", "str": "string", "bool": "boolean"}
output_types = ["double"]  # default

if decorator_args:
    types = []
    for arg in decorator_args:
        if isinstance(arg, ast.Name):
            types.append(type_map.get(arg.id, arg.id))
        elif isinstance(arg, ast.Constant):
            types.append(type_map.get(str(arg.value), str(arg.value)))
    if types:
        output_types = types

print(json.dumps({"name": name, "arg_names": arg_names, "output_types": output_types}))
`, source)

	pyCmd := exec.Command("python3", "-c", parseScript)
	out, err := pyCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("python parse failed: %w", err)
	}

	var meta udfMetadata
	if err := json.Unmarshal(out, &meta); err != nil {
		return nil, fmt.Errorf("parse metadata JSON: %w (output: %s)", err, string(out))
	}
	if meta.Name == "" {
		return nil, fmt.Errorf("could not extract function name from UDF source")
	}
	return &meta, nil
}

// saveTransformLocally saves a copy of the UDF source to ~/.hops/transformations/
func saveTransformLocally(name, source string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := home + "/.hops/transformations"
	os.MkdirAll(dir, 0755)
	path := dir + "/" + name + ".py"
	if err := os.WriteFile(path, []byte(source), 0644); err == nil {
		output.Info("Saved to %s", path)
	}
}

func init() {
	rootCmd.AddCommand(tfCmd)

	tfCreateCmd.Flags().StringVar(&tfCreateFile, "file", "", "Python file with @udf decorated function")
	tfCreateCmd.Flags().StringVar(&tfCreateCode, "code", "", "Inline Python code with @udf decorated function")
	tfCreateCmd.Flags().IntVar(&tfCreateVersion, "version", 1, "Transformation function version")

	tfCmd.AddCommand(tfListCmd)
	tfCmd.AddCommand(tfCreateCmd)
}
