---
name: hops
description: Use when working with Hopsworks Feature Store — listing and managing feature groups, feature views, training datasets, projects, jobs, and datasets. Auto-invoke when the user discusses feature engineering, feature store operations, ML pipelines, or needs to interact with Hopsworks.
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
hops fg delete <name> --version N         # Delete
```

#### Create
```bash
hops fg create <name> --primary-key <cols> [flags]
```
Flags:
- `--primary-key <cols>` — comma-separated primary key columns (required)
- `--features "name:type,..."` — schema spec (types: bigint, double, boolean, timestamp, string)
- `--online` — enable online storage (creates stream FG with Kafka + materialization job)
- `--event-time <col>` — event time column for time-travel queries
- `--description <text>` — feature group description
- `--format <DELTA|NONE>` — time travel format (default: DELTA)
- `--version <n>` — version number (default: 1)

Without `--online`: creates offline-only (cached) FG — direct Delta writes, no Kafka.
With `--online`: creates online+offline (stream) FG — writes go to Kafka→RonDB, then a Spark job materializes to Delta (~2min).

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
hops td create <fv-name> <fv-version>     # Create training dataset
hops td delete <fv-name> <fv-version> <td-version>  # Delete
```

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
