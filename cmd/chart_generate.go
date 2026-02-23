package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MagicLex/hopsworks-cli/pkg/client"
	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

// --- Flag variables (prefixed chartGen to avoid collision with chart.go) ---

var (
	chartGenFG        string
	chartGenFV        string
	chartGenX         string
	chartGenY         string
	chartGenType      string
	chartGenTitle     string
	chartGenN         int
	chartGenVersion   int
	chartGenDashboard int
)

var validChartTypes = map[string]bool{
	"bar": true, "line": true, "scatter": true, "histogram": true, "pie": true,
}

// --- Command ---

var chartGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a chart from a feature group or feature view",
	Long: `Generate an interactive Plotly chart from feature store data.

Examples:
  hops chart generate --fg products --x brand --type bar
  hops chart generate --fg orders --x category --y revenue --type bar
  hops chart generate --fv my_view --x age --type histogram --n 1000
  hops chart generate --fg orders --x status --type pie --dashboard 5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate: exactly one of --fg / --fv
		if (chartGenFG == "") == (chartGenFV == "") {
			return fmt.Errorf("exactly one of --fg or --fv is required")
		}
		if chartGenX == "" {
			return fmt.Errorf("--x is required")
		}
		if !validChartTypes[chartGenType] {
			return fmt.Errorf("--type must be one of: bar, line, scatter, histogram, pie")
		}
		if (chartGenType == "line" || chartGenType == "scatter") && chartGenY == "" {
			return fmt.Errorf("--y is required for %s charts", chartGenType)
		}

		// Source name for title / filename
		source := chartGenFG
		if source == "" {
			source = chartGenFV
		}

		// Auto-generate title
		title := chartGenTitle
		if title == "" {
			if chartGenY != "" {
				title = fmt.Sprintf("%s: %s vs %s", source, chartGenX, chartGenY)
			} else {
				title = fmt.Sprintf("%s: %s", source, chartGenX)
			}
		}

		if !output.JSONMode {
			output.Info("Generating %s chart from '%s'...", chartGenType, source)
		}

		// Build and run Python script
		script := buildChartGenerateScript(source, chartGenFG != "", chartGenX, chartGenY, chartGenType, title, chartGenN, chartGenVersion)
		raw, err := runPythonCapture(script)
		if err != nil {
			return fmt.Errorf("generate chart: %w", err)
		}

		// Parse JSON result — SDK prints login/progress to stdout, so extract the JSON line
		var result struct {
			Path string `json:"path"`
			Rows int    `json:"rows"`
		}
		var jsonLine []byte
		for _, line := range bytes.Split(raw, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if len(line) > 0 && line[0] == '{' {
				jsonLine = line
			}
		}
		if jsonLine == nil {
			return fmt.Errorf("no JSON output from Python script\nraw: %s", string(raw))
		}
		if err := json.Unmarshal(jsonLine, &result); err != nil {
			return fmt.Errorf("parse Python output: %w\nraw: %s", err, string(raw))
		}

		if !output.JSONMode {
			output.Success("Generated HTML (%d rows) → %s", result.Rows, result.Path)
		}

		// Register chart via API
		c, err := mustClient()
		if err != nil {
			return err
		}

		ch := &client.Chart{
			Title:       title,
			URL:         result.Path,
			Description: fmt.Sprintf("%s chart of %s", chartGenType, source),
			Width:       12,
			Height:      8,
		}
		created, err := c.CreateChart(ch)
		if err != nil {
			return fmt.Errorf("register chart: %w", err)
		}

		if !output.JSONMode {
			output.Success("Registered chart '%s' (ID: %d)", created.Title, created.ID)
		}

		// Optionally add to dashboard
		if chartGenDashboard > 0 {
			d, err := c.GetDashboard(chartGenDashboard)
			if err != nil {
				return fmt.Errorf("get dashboard %d: %w", chartGenDashboard, err)
			}

			d.Charts = append(d.Charts, *created)

			updated, err := c.UpdateDashboard(chartGenDashboard, d)
			if err != nil {
				return fmt.Errorf("add chart to dashboard: %w", err)
			}

			if !output.JSONMode {
				output.Success("Added to dashboard '%s' (ID: %d)", updated.Name, updated.ID)
			}
		}

		if output.JSONMode {
			output.PrintJSON(created)
		}
		return nil
	},
}

// buildChartGenerateScript produces a Python script that reads data from a FG or FV,
// transforms it per chart type, generates standalone Plotly HTML, writes to /hopsfs/,
// and prints JSON {path, rows} to stdout.
func buildChartGenerateScript(name string, isFG bool, x, y, chartType, title string, n, version int) string {
	var sb strings.Builder

	// Preamble
	sb.WriteString(`import hopsworks, warnings, logging, json, sys, os
import pandas as pd
warnings.filterwarnings("ignore")
logging.getLogger("hsfs").setLevel(logging.WARNING)
logging.getLogger("hopsworks").setLevel(logging.WARNING)

project = hopsworks.login()
fs = project.get_feature_store()
`)

	// Get data source
	if isFG {
		if version > 0 {
			sb.WriteString(fmt.Sprintf("src = fs.get_feature_group(name=%q, version=%d)\n", name, version))
		} else {
			sb.WriteString(fmt.Sprintf("src = fs.get_feature_group(name=%q, version=1)\n", name))
		}
		sb.WriteString("print('Reading feature group...', file=sys.stderr)\n")
		sb.WriteString("df = src.read()\n")
	} else {
		if version > 0 {
			sb.WriteString(fmt.Sprintf("src = fs.get_feature_view(name=%q, version=%d)\n", name, version))
		} else {
			sb.WriteString(fmt.Sprintf("src = fs.get_feature_view(name=%q, version=1)\n", name))
		}
		sb.WriteString("print('Reading feature view...', file=sys.stderr)\n")
		sb.WriteString("df = src.get_batch_data()\n")
	}

	// Row limit
	if n > 0 {
		sb.WriteString(fmt.Sprintf("df = df.head(%d)\n", n))
	}

	// Coerce Decimal columns to float (Snowflake DECIMAL → Python Decimal, not JSON-serializable)
	sb.WriteString("from decimal import Decimal as _Dec\n")
	sb.WriteString("for _c in df.columns:\n")
	sb.WriteString("    if df[_c].dropna().apply(type).eq(_Dec).any():\n")
	sb.WriteString("        df[_c] = df[_c].astype(float)\n")

	sb.WriteString("rows = len(df)\n")
	sb.WriteString(fmt.Sprintf("print(f'Processing {rows} rows for %s chart...', file=sys.stderr)\n", chartType))

	// Data transform + trace per chart type
	switch chartType {
	case "bar":
		if y != "" {
			sb.WriteString(fmt.Sprintf("agg = df.groupby(%q)[%q].mean().reset_index()\n", x, y))
			sb.WriteString(fmt.Sprintf("trace = {'type': 'bar', 'x': agg[%q].astype(str).tolist(), 'y': agg[%q].tolist()}\n", x, y))
		} else {
			sb.WriteString(fmt.Sprintf("vc = df[%q].value_counts().reset_index()\n", x))
			sb.WriteString(fmt.Sprintf("vc.columns = [%q, 'count']\n", x))
			sb.WriteString(fmt.Sprintf("trace = {'type': 'bar', 'x': vc[%q].astype(str).tolist(), 'y': vc['count'].tolist()}\n", x))
		}
	case "line":
		sb.WriteString(fmt.Sprintf("df = df.sort_values(%q)\n", x))
		sb.WriteString(fmt.Sprintf("trace = {'type': 'scatter', 'mode': 'lines', 'x': df[%q].astype(str).tolist(), 'y': df[%q].tolist()}\n", x, y))
	case "scatter":
		sb.WriteString(fmt.Sprintf("trace = {'type': 'scatter', 'mode': 'markers', 'x': df[%q].tolist(), 'y': df[%q].tolist()}\n", x, y))
	case "histogram":
		sb.WriteString(fmt.Sprintf("trace = {'type': 'histogram', 'x': df[%q].tolist()}\n", x))
	case "pie":
		sb.WriteString(fmt.Sprintf("vc = df[%q].value_counts().reset_index()\n", x))
		sb.WriteString(fmt.Sprintf("vc.columns = [%q, 'count']\n", x))
		sb.WriteString(fmt.Sprintf("trace = {'type': 'pie', 'labels': vc[%q].astype(str).tolist(), 'values': vc['count'].tolist()}\n", x))
	}

	// Build HTML
	sb.WriteString(fmt.Sprintf(`
layout = {
    'title': {'text': %q, 'font': {'color': '#e0e0e0'}},
    'paper_bgcolor': '#1a1a2e',
    'plot_bgcolor': '#16213e',
    'font': {'color': '#c0c0c0'},
    'xaxis': {'gridcolor': '#2a2a4a'},
    'yaxis': {'gridcolor': '#2a2a4a'},
    'margin': {'t': 60, 'b': 50, 'l': 60, 'r': 30},
}
`, title))

	// Sanitize filename
	sb.WriteString(fmt.Sprintf(`
safe_name = %q.replace(' ', '_').replace(':', '').lower()
filename = f"{safe_name}_%s.html"
chart_dir = "/hopsfs/Resources/charts"
os.makedirs(chart_dir, exist_ok=True)
filepath = os.path.join(chart_dir, filename)
`, name, chartType))

	// HTML template
	sb.WriteString(`
html = f"""<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<script src="https://cdn.plot.ly/plotly-2.27.0.min.js"></script>
<style>
  body {{ margin: 0; padding: 0; background: #1a1a2e; overflow: hidden; }}
  #chart {{ width: 100vw; height: 100vh; }}
</style>
</head>
<body>
<div id="chart"></div>
<script>
var trace = {json.dumps(trace)};
var layout = {json.dumps(layout)};
var config = {{responsive: true, displayModeBar: false}};
Plotly.newPlot('chart', [trace], layout, config);
window.addEventListener('resize', function() {{
  Plotly.Plots.resize(document.getElementById('chart'));
}});
</script>
</body>
</html>"""

with open(filepath, 'w') as f:
    f.write(html)

rel_path = f"Resources/charts/{filename}"
print(f"Wrote {filepath}", file=sys.stderr)
print(json.dumps({"path": rel_path, "rows": rows}))
`)

	return sb.String()
}

// --- Registration ---

func init() {
	chartGenerateCmd.Flags().StringVar(&chartGenFG, "fg", "", "Feature group name")
	chartGenerateCmd.Flags().StringVar(&chartGenFV, "fv", "", "Feature view name")
	chartGenerateCmd.Flags().StringVar(&chartGenX, "x", "", "X-axis / category column (required)")
	chartGenerateCmd.Flags().StringVar(&chartGenY, "y", "", "Y-axis / value column")
	chartGenerateCmd.Flags().StringVar(&chartGenType, "type", "bar", "Chart type: bar, line, scatter, histogram, pie")
	chartGenerateCmd.Flags().StringVar(&chartGenTitle, "title", "", "Chart title (auto-generated if omitted)")
	chartGenerateCmd.Flags().IntVar(&chartGenN, "n", 0, "Row limit (0 = all)")
	chartGenerateCmd.Flags().IntVar(&chartGenVersion, "version", 0, "FG/FV version (default: 1)")
	chartGenerateCmd.Flags().IntVar(&chartGenDashboard, "dashboard", 0, "Dashboard ID to auto-add chart")

	chartCmd.AddCommand(chartGenerateCmd)
}
