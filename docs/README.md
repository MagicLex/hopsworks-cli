# hops CLI — Documentation

## Feature Docs

How each CLI domain works, API quirks, and implementation notes.

| Doc | Covers |
|-----|--------|
| [feature/jobs.md](feature/jobs.md) | Job lifecycle — create, run, stop, logs, schedule. Config endpoints, type discriminators, PySpark gotchas. |
| [feature/external-fgs.md](feature/external-fgs.md) | External (on-demand) feature groups — Snowflake/JDBC, joins, Arrow Flight, TD materialization, model deployment. |
| [feature/charts.md](feature/charts.md) | Charts & dashboards — HopsFS file viewer grid, Plotly pattern, layout system. |

## Fixes

Active patches and workarounds applied to the cluster. **Do not remove these** — they document real bugs with real workarounds in production.

| Doc | Covers |
|-----|--------|
| [fixes/sdk-fixes.md](fixes/sdk-fixes.md) | Python SDK insert pipeline — commit_details bug, Delta mTLS certs, JKS→PEM, env vars. Fork status. |
| [fixes/hopsworks-ee-fixes.md](fixes/hopsworks-ee-fixes.md) | Backend (Java) fixes — PEM extraction, Arrow Flight ConfigMap overlay, Dashboard NPE. |
| [fixes/hopsworks-api-fixes.md](fixes/hopsworks-api-fixes.md) | Python SDK fixes — fork branch `fix/cli-terminal`, 2 patches, installation via site-packages overlay. |
| [fixes/backend-issues.md](fixes/backend-issues.md) | Discovered backend issues — REST FG creation missing Kafka topic, no Spark shuffle in terminal. |

## Reference

API specs, test accounts, cluster ops.

| Doc | Covers |
|-----|--------|
| [reference/feature-store-api.md](reference/feature-store-api.md) | Feature Store REST API endpoint reference. |
| [reference/snowflake-test.md](reference/snowflake-test.md) | Snowflake test account credentials and verified CLI flow. |
| [reference/cluster-ops.md](reference/cluster-ops.md) | kubectl commands, SSH, RSS troubleshooting, DDL migrations, ingress. |

## Project

| Doc | Covers |
|-----|--------|
| [plan.md](plan.md) | Roadmap and current state (v0.8.2). |
| [dev-loop.md](dev-loop.md) | Development workflow inside the cluster. |
| [internal-mode-roadmap.md](internal-mode-roadmap.md) | Future: `hops cluster`, `hops infra`, `hops metrics`. |
