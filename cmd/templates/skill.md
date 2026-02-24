---
name: hops
description: Use when working with Hopsworks Feature Store — listing and managing feature groups, feature views, training datasets, storage connectors, models, deployments, projects, jobs, and datasets. Auto-invoke when the user discusses feature engineering, feature store operations, ML pipelines, model serving, external data sources, or needs to interact with Hopsworks.
allowed-tools: Bash(hops *)
---

# Hopsworks CLI

You have access to the `hops` CLI to interact with the user's Hopsworks Feature Store.

## Current Context

Project:
!`hops project info --json 2>/dev/null`

Feature Groups:
!`hops fg list --json 2>/dev/null`

Feature Views:
!`hops fv list --json 2>/dev/null`

Storage Connectors:
!`hops connector list --json 2>/dev/null`

Jobs:
!`hops job list --json 2>/dev/null`

## Commands

### Projects
```bash
hops project list                         # List projects
hops project use <name>                   # Switch active project
hops project info                         # Current project details
```

### Feature Groups
```bash
hops fg list                              # List all feature groups
hops fg info <name> [--version N]         # Show details + schema
hops fg preview <name> [--n 10]           # Preview data rows
hops fg features <name>                   # List features with types
hops fg stats <name> [--version N]        # Show/compute statistics
hops fg search <name> --vector "0.1,..."  # KNN similarity search
hops fg keywords <name>                   # List keywords (visual tags)
hops fg add-keyword <name> <kw> [kw...]  # Add keywords
hops fg remove-keyword <name> <keyword>  # Remove a keyword
hops fg delete <name> --version N         # Delete
```

#### Create
```bash
hops fg create <name> --primary-key <cols> [flags]
```
Flags:
- `--primary-key <cols>` — comma-separated primary key columns (required)
- `--features "name:type,..."` — schema spec (types: bigint, double, boolean, timestamp, string, array<float>)
- `--online` — enable online storage (creates stream FG with Kafka + materialization job)
- `--event-time <col>` — event time column for time-travel queries
- `--description <text>` — feature group description
- `--format <DELTA|NONE>` — time travel format (default: DELTA)
- `--version <n>` — version number (default: 1)
- `--embedding "name:dimension[:metric]"` — embedding column (repeatable, metrics: l2, cosine, dot_product)

Without `--online`: creates offline-only (cached) FG — direct Delta writes, no Kafka.
With `--online`: creates online+offline (stream) FG — writes go to Kafka→RonDB, then a Spark job materializes to Delta (~2min).
With `--embedding`: auto-enables online, creates OpenSearch vector index for similarity search.

#### Embeddings & Similarity Search
```bash
# Create FG with embedding column
hops fg create documents \
  --primary-key doc_id \
  --features "doc_id:bigint,title:string" \
  --embedding "text_embedding:384:cosine"

# Search for nearest neighbors
hops fg search documents --vector "0.1,0.2,..." --k 5
hops fg search documents --vector "[0.1, 0.2, ...]" --k 10 --col text_embedding
```
Flags for search:
- `--vector` — query vector (comma-separated floats or JSON array, required)
- `--k <n>` — number of neighbors (default: 10)
- `--col <name>` — embedding column (required if FG has multiple embeddings)
- `--version <n>` — feature group version

#### Insert
```bash
hops fg insert <name> --file data.csv     # Insert from CSV/JSON/Parquet
hops fg insert <name> --generate 100      # Insert generated sample data
cat data.json | hops fg insert <name>     # Insert from stdin
```
Flags:
- `--file <path>` — read from CSV, JSON, or Parquet file
- `--generate <n>` — generate n sample rows based on schema
- `--online-only` — write to online store (Kafka) only, skip Spark materialization job
- `--version <n>` — target version (default: 1)

For online-enabled FGs, insert triggers a Spark materialization job by default. Use `--online-only` to skip it.

#### Derive
```bash
# Join two FGs on a shared column
hops fg derive enriched --base transactions --join "products LEFT id" --primary-key id

# Different join keys + prefix
hops fg derive enriched --base transactions --join "products LEFT customer_id=id p_" --primary-key customer_id

# Multiple joins, online, with feature selection
hops fg derive full_view --base orders \
  --join "customers LEFT customer_id" \
  --join "products LEFT product_id=id p_" \
  --primary-key order_id --online --features "order_id,amount,name,p_category"
```
Join spec format: `"<fg>[:<version>] <INNER|LEFT|RIGHT|FULL> <on>[=<right_on>] [prefix]"`

Flags:
- `--base <fg>` — base feature group (name or name:version, required)
- `--join <spec>` — join spec (repeatable, at least one required)
- `--primary-key <cols>` — primary key for derived FG (comma-separated, required)
- `--online` — enable online storage
- `--event-time <col>` — event time column
- `--description <text>` — description
- `--features <cols>` — comma-separated columns to keep (post-query filter)

### Feature Views
```bash
hops fv list                              # List all feature views
hops fv info <name> [--version N]         # Show details + source FGs + joins
hops fv create <name> --feature-group <fg>  # Create from single FG
hops fv get <name> --entry "pk=val"       # Online feature vector lookup
hops fv read <name> [--n 100]             # Batch read (offline)
hops fv read <name> --output data.parquet # Save batch to file
hops fv delete <name> --version N         # Delete
```

#### Create with Joins
```bash
# Single FG
hops fv create my_view --feature-group transactions

# With join
hops fv create enriched_view \
  --feature-group transactions \
  --join "products LEFT product_id=id p_"

# Multiple joins + feature selection
hops fv create full_view \
  --feature-group orders \
  --join "customers LEFT customer_id" \
  --join "products LEFT product_id=id p_" \
  --features "order_id,amount,name,p_category"
```
Join spec format: `"<fg>[:<version>] <INNER|LEFT|RIGHT|FULL> <on>[=<right_on>] [prefix]"`

Flags:
- `--feature-group <fg>` — base feature group (required)
- `--join <spec>` — join spec (repeatable)
- `--features <cols>` — selected features (comma-separated)
- `--labels <cols>` — label columns (comma-separated)
- `--description <text>` — description
- `--version <n>` — version (default: 1)
- `--fg-version <n>` — base FG version (latest if omitted)
- `--transform <spec>` — transform spec (repeatable): `"fn_name:column"`

#### Online Feature Vector Lookup
```bash
hops fv get my_view --entry "id=42"
hops fv get my_view --entry "id=1" --entry "id=2" --entry "id=3"
```
Requires FV built from online-enabled FGs. Uses `--entry "key=value"` (repeatable).

#### Batch Read
```bash
hops fv read my_view                      # Print table
hops fv read my_view --n 100              # Limit rows
hops fv read my_view --output data.parquet
hops fv read my_view --output data.csv
hops fv read my_view --output data.json
```
Flags:
- `--output <path>` — save to file (format from extension: .parquet, .csv, .json)
- `--n <rows>` — limit rows
- `--version <n>` — feature view version

### Transformations
```bash
hops transformation list                         # List all transformation functions
hops transformation create --file scaler.py      # Register from Python file
hops transformation create --code '@udf(float)   # Register inline
def double_it(value):
    return value * 2'
```
Alias: `hops tf list`, `hops tf create`

Custom transforms are saved locally to `~/.hops/transformations/`.

### Training Datasets
```bash
hops td list <fv-name> <fv-version>       # List training datasets
hops td create <fv-name> <fv-version>     # Create training dataset (metadata only)
hops td compute <fv-name> <fv-version>    # Materialize training data (Spark job)
hops td compute <fv-name> <fv-version> --split "train:0.8,test:0.2"  # With splits
hops td compute <fv-name> <fv-version> --filter "price > 100"        # Filter rows
hops td compute <fv-name> <fv-version> --filter "price > 50 AND product == Laptop"
hops td compute <fv-name> <fv-version> --start-time "2026-01-01" --end-time "2026-02-01"
hops td read <fv-name> <fv-version> --td-version N  # Read training data
hops td read <fv-name> <fv-version> --td-version N --split train --output train.csv
hops td delete <fv-name> <fv-version> <td-version>  # Delete
```

### Models
```bash
hops model list                           # List models in registry
hops model info <name> [--version N]      # Show model details + metrics
hops model register <name> <path>         # Register model + upload artifacts
hops model download <name> [--output dir] # Download model artifacts
hops model delete <name> --version N      # Delete model version
```
Flags for register:
- `--framework <python|sklearn|tensorflow|torch>` — model framework (default: python)
- `--metrics "key=value,..."` — training metrics
- `--description <text>` — model description
- `--feature-view <name>` — link to feature view (provenance + auto schema inference)
- `--td-version <n>` — training dataset version (with --feature-view)
- `--input-example <file>` — sample input file (JSON or CSV)
- `--schema "in:name:type,... out:name:type,..."` — explicit input/output schema
- `--program <file>` — training script path (stored as metadata)
- `--version <n>` — model version (default: auto-increment)

When `--feature-view` + `--td-version` are both set, the SDK auto-infers input/output schema from the training dataset features/labels.

### Deployments
```bash
hops deployment list                      # List all deployments
hops deployment info <name>               # Show deployment details
hops deployment create <model-name>       # Create deployment from model
hops deployment start <name>              # Start a deployment
hops deployment stop <name>               # Stop a deployment
hops deployment predict <name> --data '{"instances": [...]}' # Send prediction
hops deployment logs <name>               # View deployment logs
hops deployment delete <name>             # Delete a deployment
```
Alias: `hops deploy list`, `hops deploy create`, etc.

Flags for create:
- `--version <n>` — model version (latest if omitted)
- `--name <name>` — deployment name (default: sanitized model name)
- `--instances <n>` — number of instances (default: 1)
- `--script <path>` — custom predictor script

Flags for logs:
- `--tail <n>` — number of log lines (default: 50)
- `--component <predictor|transformer>` — log component (default: predictor)

#### Deployment Guide — Important Gotchas

**Deployment names** must be alphanumeric only (`[a-zA-Z0-9]+`). No hyphens, underscores, or special characters.

**Predictor script** — KServe deployments need a `predict.py` with a `Predict` class:

```python
import os
import joblib
import numpy as np

class Predict:
    def __init__(self):
        # Model artifacts are in /mnt/models/ (NOT /mnt/artifacts/)
        # /mnt/artifacts/ only contains the predictor script itself
        model_dir = "/mnt/models"
        self.model = joblib.load(os.path.join(model_dir, "model.pkl"))

    def predict(self, inputs):
        # inputs is a LIST (instances), not {"instances": [...]} dict
        # KServe unwraps the payload before calling predict()
        if isinstance(inputs, list):
            instances = inputs
        else:
            instances = inputs.get("instances", [])
        predictions = self.model.predict(np.array(instances))
        return {"predictions": predictions.tolist()}
```

**sklearn version**: The serving environment uses **sklearn 1.3.2**, numpy 1.26.4, pandas 2.3.1. You MUST train with the same sklearn version — pickle is not backwards-compatible across major versions. Install with `pip install scikit-learn==1.3.2` before training.

**Artifact paths**: The `storage-initializer` init container downloads files from HopsFS:
- Predictor script → `/mnt/artifacts/predictor-<script>.py`
- Model files (from `Models/<name>/<version>/Files/`) → `/mnt/models/`

**Updating a deployment**:
- **Model version swap**: Update in-place via SDK — `deployment.model_version = N; deployment.save()` triggers a rolling update with zero downtime.
- **Predictor script changes**: Must `delete` then `create` a new deployment — `start`/`stop` does not refresh scripts.

#### Typical Deploy Flow
```bash
# 1. Train with matching sklearn version
pip install scikit-learn==1.3.2
python train.py  # saves model.pkl to a local dir

# 2. Register model (uploads artifacts to HopsFS Models/<name>/<version>/Files/)
hops model register mymodel ./model_dir \
  --framework sklearn --feature-view my_fv --td-version 1 \
  --metrics "mae=100,r2=0.85"

# 3. Upload predictor script to model dir
cp predict.py /hopsfs/Models/mymodel/1/Files/predict.py

# 4. Create and start deployment
hops deployment create mymodel --script predict.py --name mymodel
hops deployment start mymodel

# 5. Test
hops deployment predict mymodel --data '{"instances": [...]}'
```

### Jobs
```bash
hops job list                             # List jobs
hops job info <name>                      # Show job config details
hops job create <name> --type <type> --app-path <path>  # Create job
hops job run <name> [--wait] [--args "..."]             # Start execution
hops job stop <name> [--exec ID]          # Stop running execution
hops job status <name> [--wait] [--poll 5]              # Latest execution status
hops job logs <name> [--exec ID] [--type out|err]       # Execution logs
hops job history <name> [--limit N]       # List executions
hops job delete <name>                    # Delete job
hops job schedule <name> "<cron>"         # Set cron schedule
hops job schedule-info <name>             # Show schedule
hops job unschedule <name>                # Remove schedule
```

Flags for create:
- `--type <python|pyspark|spark|ray>` — job type (required)
- `--app-path <path>` — script/JAR path (required). Python: relative (`Resources/jobs/x.py`), Spark: HDFS (`hdfs:///Projects/...`)
- `--main-class <class>` — main class for Spark JARs
- `--args <string>` — default arguments
- `--env-name <name>` — conda environment
- `--driver-mem`, `--driver-cores`, `--executor-mem`, `--executor-cores`, `--executors`, `--dynamic` — Spark resources
- `--memory`, `--cores`, `--gpus` — Python resources
- `--worker-mem`, `--worker-cores`, `--workers-min`, `--workers-max` — Ray resources

Schedule uses Quartz 6-field cron: `SEC MIN HOUR DAY MONTH WEEKDAY` (use `?` for unspecified).
Examples: `"0 0 * * * ?"` (every hour), `"0 */15 * * * ?"` (every 15 min), `"0 0 8 * * MON-FRI"` (weekdays at 8am).

### Storage Connectors
```bash
hops connector list                       # List all connectors
hops connector info <name>                # Show connector details
hops connector test <name>                # Test connection (lists databases)
hops connector databases <name>           # List databases
hops connector tables <name> --database X # List tables in database
hops connector preview <name> --database X --table Y [--schema Z]  # Preview data
hops connector delete <name>              # Delete connector
```
Alias: `hops conn list`, etc.

#### Create Connectors
```bash
# Snowflake
hops connector create snowflake <name> \
  --url <url> --user <user> --password <pw> \
  --database <db> --schema <schema> --warehouse <wh> \
  [--role <role>] [--token <token>] [--description <text>]

# JDBC
hops connector create jdbc <name> \
  --connection-string "jdbc:..." \
  [--arguments "key=val,key=val"] [--description <text>]

# S3
hops connector create s3 <name> \
  --bucket <bucket> --access-key <ak> --secret-key <sk> \
  [--region <region>] [--iam-role <arn>] [--path <prefix>] [--description <text>]

# BigQuery
hops connector create bigquery <name> \
  --key-path <hdfs-path-to-key.json> --parent-project <gcp-project-id> \
  --materialization-dataset <dataset> [--description <text>]
  # OR with explicit query target:
  --query-project <proj> --dataset <ds> --query-table <tbl>
```
The key file must be uploaded to HopsFS first (e.g. `/Projects/<project>/Resources/key.json`).

#### External Feature Groups
```bash
# Auto-infer schema from connector (recommended)
hops fg create-external <name> \
  --connector <connector-name> \
  --query "SELECT COL1, COL2 FROM DB.SCHEMA.TABLE" \
  --database <db> --table <table> --schema <schema> \
  --primary-key <cols> \
  [--event-time <col>] [--online] [--description <text>]

# Explicit schema (no auto-inference)
hops fg create-external <name> \
  --connector <connector-name> \
  --query "SELECT ..." \
  --features "col1:bigint,col2:string" \
  --primary-key <cols>
```
Creates an on-demand feature group backed by a storage connector. The connector must exist first.

Flags:
- `--connector <name>` — storage connector name (required)
- `--query <sql>` — SQL query for the external data source (required)
- `--primary-key <cols>` — primary key columns, comma-separated (required)
- `--database <db>` + `--table <tbl>` + `--schema <sch>` — auto-infer features from connector
- `--features "name:type,..."` — explicit schema (skips auto-inference)
- `--event-time <col>`, `--online`, `--description <text>` — optional

**Snowflake note**: Use UPPERCASE column names in `--query` and `--features`. Snowflake identifiers are case-sensitive and default to uppercase.

### Charts
```bash
hops chart list                           # List all charts
hops chart info <id>                      # Show chart details
hops chart generate --fg <name> --x <col> --type <type>  # Generate from FG data
hops chart generate --fv <name> --x <col> --y <col> --type bar --dashboard <id>
hops chart create <title> --url <url> --description <desc>  # Register external chart
hops chart update <id> --title <new-title>
hops chart delete <id>                    # Delete chart
```
Chart types: `bar`, `line`, `scatter`, `histogram`, `pie`

Flags for generate:
- `--fg <name>` or `--fv <name>` — data source (required, one of)
- `--x <col>` — X-axis / category column (required)
- `--y <col>` — Y-axis / value column (optional, depends on chart type)
- `--type <type>` — chart type (default: bar)
- `--n <rows>` — row limit (0 = all)
- `--title <text>` — chart title (auto-generated if omitted)
- `--dashboard <id>` — auto-add to dashboard with grid layout
- `--version <n>` — FG/FV version

### Dashboards
```bash
hops dashboard list                       # List all dashboards
hops dashboard info <id>                  # Show dashboard with chart layout
hops dashboard create <name>              # Create empty dashboard
hops dashboard add-chart <id> --chart-id <id>  # Add chart to dashboard
hops dashboard remove-chart <id> --chart-id <id>  # Remove chart
hops dashboard delete <id>               # Delete dashboard
```
Alias: `hops dash list`, etc.

Flags for add-chart:
- `--chart-id <id>` — chart to add (required)
- `--width <n>`, `--height <n>` — size in grid units (default: 12x8)
- `--x <n>`, `--y <n>` — position in grid

### Other
```bash
hops fs list                              # List feature stores
hops dataset list [path]                  # Browse project files
hops dataset mkdir <path>                 # Create directory
hops context                              # Dump full schema (for LLM context)
```

### Global Flags
```bash
--json                                    # Output as JSON (for parsing)
--host <url>                              # Override Hopsworks host
--api-key <key>                           # Override API key
--project <name>                          # Override project
```

## Working with Hopsworks

1. Start with `hops project list` then `hops project use <name>`
2. Use `hops fg list` and `hops fv list` to discover available resources
3. Use `hops fg info <name>` to understand schemas before working with data
4. Use `hops context` for a full markdown dump of the feature store state
5. Use `--json` when you need to parse output programmatically
6. Feature group and feature view names are case-sensitive

## Environment Variables

`HOPSWORKS_API_KEY`, `REST_ENDPOINT`, `PROJECT_NAME`, `HOPSWORKS_PROJECT_ID` override config file values.
Inside a Hopsworks terminal pod, authentication is automatic via JWT.
