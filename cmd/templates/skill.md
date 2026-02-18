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
hops fg create <name> --version 1 --primary-key id  # Create new
hops fg delete <name> --version N         # Delete
hops fg insert <name> --file data.csv     # Insert data (via Python SDK)
hops fg insert <name> --generate 100      # Insert sample data
```

### Feature Views
```bash
hops fv list                              # List all feature views
hops fv info <name> [--version N]         # Show details
hops fv create <name> --version 1 --feature-group <fg>  # Create
hops fv delete <name> --version N         # Delete
```

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
