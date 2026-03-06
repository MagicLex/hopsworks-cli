# Command Backends — REST vs Python SDK

How each `hops` command fetches/sends data under the hood.

- **REST** — Direct HTTP calls from Go (`pkg/client/*.go`), no external deps
- **Python SDK** — Shells out to `python3 -c` using `hopsworks`/`hsfs`/`hsml` packages
- **Other** — File I/O, browser open, `go install`, etc.

## REST API (Go HTTP)

Zero external deps. Works anywhere with network access to the Hopsworks API.

| Domain | Commands |
|--------|----------|
| Feature Store | `fs list` |
| Feature Groups | `fg list`, `info`, `preview`, `features`, `stats`, `keywords`, `add-keyword`, `remove-keyword`, `create`, `delete` |
| Feature Views | `fv list`, `info`, `create`, `delete` |
| Connectors | `connector list`, `info`, `test`, `databases`, `tables`, `preview`, `create` (snowflake/jdbc/s3/bigquery), `delete` |
| Jobs | `job list`, `info`, `create`, `run`, `stop`, `logs`, `history`, `status`, `delete`, `schedule`, `schedule-info`, `unschedule` |
| Models | `model list`, `info`, `delete`, `download` |
| Deployments | `deployment list`, `info`, `create`, `start`, `stop`, `delete` |
| Charts | `chart list`, `info`, `create`, `update`, `delete` |
| Dashboards | `dashboard list`, `info`, `create`, `delete`, `add-chart`, `remove-chart` |
| Projects | `project list`, `use`, `info` |
| Transformations | `transformation list` |

## Python SDK (shell-out to `python3`)

Requires `hopsworks`, `hsfs`, `hsml` installed. Used for operations that need
the SDK's data pipeline logic (Spark/Hive reads, embedding search, model artifacts).

All Python commands call `hopsworks.login()` (auto-detects JWT or API key from env),
then get the feature store/registry handle.

| Domain | Commands | SDK packages |
|--------|----------|--------------|
| Feature Groups | `fg insert`, `derive`, `search`, `create-external` | hsfs, hopsworks |
| Feature Views | `fv get`, `read` | hsfs, hopsworks |
| Training Datasets | `td compute`, `read`, `stats` | hsfs, hopsworks |
| Models | `model register` | hsml, hopsworks |
| Transformations | `transformation create` | hsfs, hopsworks |
| Charts | `chart generate` | hsfs, hopsworks, plotly |

### Python env setup (in-cluster)

When running inside the terminal pod, the Python commands also set mTLS env vars
for HDFS access:

```
PEMS_DIR=${HOME}/.hopsfs_pems
LIBHDFS_DEFAULT_USER=$HADOOP_USER_NAME
```

## Other

| Command | Mechanism |
|---------|-----------|
| `login` | REST API for auth + `open`/`xdg-open` for browser |
| `init` | Local file I/O only (writes `.claude/skills/hops/SKILL.md`) |
| `update` | Shells out to `go install` |
| `context` | REST API (context dump) |
