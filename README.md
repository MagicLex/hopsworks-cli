# hops — Hopsworks CLI for Humans and LLMs

A Go CLI for the Hopsworks Feature Store. Works as a standalone tool for humans and as an AI tool interface for LLMs — no MCP server needed.

## Philosophy

Build a great CLI, and both humans and LLMs benefit. `hops` is the simplest way to give an AI agent access to Hopsworks: just allow `Bash(hops *)` and it works. No protocol, no server, no overhead.

## Install

```bash
# From source
go build -o hops . && sudo mv hops /usr/local/bin/

# Or via go install
go install github.com/MagicLex/hopsworks-cli@latest && mv ~/go/bin/hopsworks-cli ~/go/bin/hops
```

## Quick Start

```bash
# Login (prompted for host + API key)
hops login

# Or if inside Hopsworks terminal, it auto-configures
hops project list
hops project use myproject

# Explore feature groups
hops fg list
hops fg info customer_transactions
hops fg preview customer_transactions --n 5
hops fg features customer_transactions

# Feature views (single FG or multi-FG joins + transforms)
hops fv list
hops fv create my_view --feature-group transactions
hops fv create enriched --feature-group transactions \
  --join "products LEFT product_id=id p_" \
  --transform "standard_scaler:amount"

# Online feature vector lookup
hops fv get my_view --entry "id=42"

# Batch read from feature view
hops fv read my_view --n 100
hops fv read my_view --output data.parquet

# Insert data
hops fg insert customer_transactions --file data.csv
hops fg insert customer_transactions --generate 100

# Derive new FG from joins (with provenance tracking)
hops fg derive enriched --base transactions \
  --join "products LEFT id" --primary-key id

# Embeddings + similarity search
hops fg create documents --primary-key doc_id \
  --features "doc_id:bigint,title:string" \
  --embedding "text_embedding:384:cosine"
hops fg search documents --vector "0.1,0.2,..." --k 5

# Training datasets (materialize + retrieve)
hops td compute my_view 1
hops td compute my_view 1 --split "train:0.8,test:0.2"
hops td read my_view 1 --td-version 1 --output train.parquet

# Transformations
hops transformation list
hops transformation create --file my_scaler.py

# Browse files
hops dataset list

# Context dump (for LLMs)
hops context
```

## Commands

| Command | Description |
|---------|------------|
| `hops login` | Authenticate with Hopsworks |
| `hops project list\|use\|info` | Manage projects |
| `hops fs list` | List feature stores |
| `hops fg list\|info\|preview\|features\|create\|delete\|insert\|derive\|search` | Feature groups (with embeddings + KNN) |
| `hops fv list\|info\|create\|get\|read\|delete` | Feature views (joins + transforms + online/batch read) |
| `hops transformation list\|create` | Transformation functions |
| `hops td list\|create\|compute\|read\|delete` | Training datasets (materialize + retrieve with splits) |
| `hops job list\|status` | Jobs (with `--wait` polling) |
| `hops dataset list\|mkdir` | Browse project files |
| `hops init` | Set up Claude Code integration |
| `hops context` | Dump project state for LLMs |

## Claude Code Integration

```bash
hops init
```

This creates `.claude/skills/hops/SKILL.md` with:
- Frontmatter for auto-invocation when discussing feature stores
- Dynamic context (feature groups, feature views, jobs load automatically)
- Full CLI reference

It also adds `Bash(hops *)` to `.claude/settings.local.json`.

Open Claude Code in the project directory — type `/hops` or just ask about your feature store.

## Output Modes

```bash
# Human-readable tables (default)
hops fg list

# JSON for programmatic use / LLMs
hops fg list --json
```

## Authentication

- **Inside Hopsworks terminal**: Auto-detects `REST_ENDPOINT`, `PROJECT_NAME`, and JWT token. Zero config.
- **Outside**: Run `hops login` or set `HOPSWORKS_API_KEY` and `REST_ENDPOINT` env vars.

Config saved to `~/.hops/config`.

## Global Flags

```
--host <url>       Override Hopsworks host
--api-key <key>    Override API key
--project <name>   Override active project
--json             JSON output
```

## Architecture

```
hopsworks-cli/
├── main.go              # Entry point
├── cmd/                 # Cobra commands (one file per resource)
│   └── templates/       # Embedded SKILL.md template
├── pkg/
│   ├── client/          # HTTP client for Hopsworks REST API
│   ├── config/          # Config management (~/.hops/config)
│   └── output/          # Table vs JSON formatting
└── docs/                # Plans & known issues
```
