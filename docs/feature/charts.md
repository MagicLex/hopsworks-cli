# Charts & Dashboards — CLI Findings

## How It Actually Works

The Hopsworks Chart/Dashboard feature is **not** an external URL embed or Grafana integration. It's a **file viewer grid**.

### Architecture

```
chart.url  =  HopsFS dataset path  (e.g. "Resources/charts/my_chart.html")
                     │
                     ▼
             UI file viewer component (EW)
                     │
                     ▼
         GET /project/{id}/dataset/{path}?type=DATASET&action=preview
                     │
                     ▼
              Rendered in a card on the dashboard grid
```

- **Chart** = a pointer to a file in HopsFS + layout metadata (width, height, x, y)
- **Dashboard** = a named collection of charts arranged on a 12-column grid
- The `url` field is a **project-relative dataset path**, not an external URL
- The UI renders file content using its built-in file viewer (`EW` component)
- HTML files with embedded JS (e.g. Plotly) render as interactive charts

### What Works Today

- `hops chart {list, info, create, update, delete}` — full CRUD
- `hops dashboard {list, info, create, delete, add-chart, remove-chart}` — full CRUD + chart association
- Alias: `hops dash`
- Dashboard grid layout via `--width`, `--height`, `--x`, `--y` flags
- 12-column grid, charts placed with x/y coordinates

### Backend Bugs (workarounds in CLI)

1. **NPE on null charts** — `DashboardDTO.getCharts().stream()` crashes if charts list is null. CLI always sends `"charts": []`. See `docs/fixes/hopsworks-ee-fixes.md` Fix 5a.

2. **NOT NULL layout columns** — `dashboard_chart.width/height/x/y` are non-nullable with no defaults. CLI defaults to width=12, height=8, x=0, y=0. See Fix 5b.

### Chart Content Pattern

To display actual visualizations, the pattern is:

1. Pull data from FG/FV via Python SDK
2. Generate standalone HTML with embedded Plotly (or any JS charting lib)
3. Upload to HopsFS (e.g. `Resources/charts/`)
4. Create a chart record pointing to the file path
5. Add the chart to a dashboard with layout coordinates

Example standalone chart HTML:
```html
<!DOCTYPE html>
<html><head>
<script src="https://cdn.plot.ly/plotly-2.27.0.min.js"></script>
<style>body{margin:0;background:#1a1a2e;height:100vh}
#chart{width:100%;height:100%}</style>
</head><body><div id="chart"></div>
<script>
const data = [/* JSON from FG */];
Plotly.newPlot('chart', [{x: ..., y: ..., type: 'bar'}], {
  paper_bgcolor: '#1a1a2e', plot_bgcolor: '#16213e',
  font: {color: '#eee'}
});
</script></body></html>
```

## Current Limitations

### 1. No auto-resize
Charts are rendered in fixed-size boxes by the UI file viewer. The Plotly chart inside doesn't auto-resize to fill the container. Needs `Plotly.Plots.resize()` on window resize, or responsive Plotly config.

### 2. No refresh on click
The file viewer has a refresh button but it only re-fetches the static HTML file. If the underlying FG data changes, the chart is stale until the HTML file is regenerated and re-uploaded. The `job` field on Chart exists but there's no UI trigger to run it.

### 3. `url` field naming is misleading
It's not a URL — it's a dataset path. The CLI flag `--url` should ideally be `--path`, but we keep `--url` to match the API field name.

## TODOs

### CLI: `hops chart generate` command
Auto-generate chart HTML from feature store data:
```
hops chart generate <fg-or-fv-name> \
  --feature <feature_name> \
  --type {bar,scatter,pie,line,histogram} \
  --group-by <feature_name> \
  --title "My Chart" \
  --dashboard <dashboard-id>   # optional: auto-add to dashboard
```

Flow: read FG/FV → aggregate → generate Plotly HTML → upload to `Resources/charts/` → create chart record → optionally add to dashboard.

This would be a Python delegation command (like `hops fg insert`) since it needs the hsfs SDK to read data.

### CLI: responsive chart template
The generated HTML should include:
```js
window.addEventListener('resize', () => Plotly.Plots.resize('chart'));
// + Plotly config: {responsive: true}
```
So charts fill their dashboard card properly.

### CLI: `hops chart refresh <id>`
Regenerate a chart's HTML from its source FG/FV. Requires storing the generation params (FG name, feature, chart type) somewhere — either in the chart description or a metadata field.

### Backend: chart `job` integration
The Chart entity has a `job` field. The intended pattern is probably:
1. Create a Hopsworks Job that regenerates the chart HTML
2. Link it to the chart via `--job-id`
3. UI could then offer "Run job to refresh" on each chart card

### Backend: Fix null guards + layout defaults
See `docs/fixes/hopsworks-ee-fixes.md` Fix 5a/5b. Should be fixed upstream so clients don't need workarounds.
