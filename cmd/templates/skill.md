---
name: hops
description: Use when working with Hopsworks Feature Store — listing and managing feature groups, feature views, training datasets, models, deployments, projects, jobs, and datasets. Auto-invoke when the user discusses feature engineering, feature store operations, ML pipelines, model serving, or needs to interact with Hopsworks.
allowed-tools: Bash(hops *)
---

# Hopsworks CLI

You have access to the `hops` CLI to interact with the user's Hopsworks Feature Store.

## Current Context

Project:
!`hops project info --json 2>/dev/null || echo "No project selected — run: hops project use <name>"`

Feature Groups:
!`hops fg list --json 2>/dev/null || echo "[]"`

Feature Views:
!`hops fv list --json 2>/dev/null || echo "[]"`

Jobs:
!`hops job list --json 2>/dev/null || echo "[]"`

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

### Jobs
```bash
hops job list                             # List jobs
hops job status <name>                    # Latest execution status
hops job status <name> --wait             # Poll until finished (10s default)
hops job status <name> --wait --poll 5    # Poll every 5s
```

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
