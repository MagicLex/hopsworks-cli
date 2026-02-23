# The Plan

## What We're Building

A `hops` CLI in Go that works for humans and LLMs. No MCP server needed — just `Bash(hops *)`.

## Current State (v0.8.5)

### Commands Implemented

| Domain | Commands | Status |
|--------|----------|--------|
| Projects | `list`, `use`, `info` | Done |
| Feature Groups | `list`, `info`, `preview`, `features`, `stats`, `create`, `create-external`, `delete`, `insert`, `derive`, `search` | Done |
| Feature Views | `list`, `info`, `create`, `get`, `read`, `delete` | Done |
| Training Datasets | `list`, `create`, `compute`, `read`, `delete` | Done |
| Transformations | `list`, `create` | Done |
| Storage Connectors | `list`, `info`, `test`, `databases`, `tables`, `preview`, `create`, `delete` | Done |
| Models | `list`, `info`, `register`, `download`, `delete` | Done |
| Deployments | `list`, `info`, `create`, `start`, `stop`, `predict`, `logs`, `delete` | Done |
| Jobs | `list`, `info`, `create`, `run`, `stop`, `status`, `logs`, `history`, `delete`, `schedule`, `schedule-info`, `unschedule` | Done |
| Charts | `list`, `info`, `create`, `update`, `delete`, `generate` | Done |
| Dashboards | `list`, `info`, `create`, `delete`, `add-chart`, `remove-chart` | Done |
| Datasets | `list`, `mkdir` | Done |
| Other | `init`, `context`, `fs list` | Done |

### Repos

| Repo | Branch | Location |
|------|--------|----------|
| [hopsworks-cli](https://github.com/MagicLex/hopsworks-cli) | `main` | `~/hopsworks-cli` |
| [hopsworks-ee](https://github.com/MagicLex/hopsworks-ee) | reference | `~/hopsworks-ee` |
| [hopsworks-api](https://github.com/MagicLex/hopsworks-api) | `fix/cli-terminal` | `~/hopsworks-api` |

## What's Next

- Wire `chart generate` to create PYTHON jobs (refreshable charts from UI)
- Internal mode: `hops cluster`, `hops infra` (see `INTERNAL-MODE-ROADMAP.md`)
- `hops update` self-improvement loop
