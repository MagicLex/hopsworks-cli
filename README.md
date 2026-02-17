# hops — Hopsworks CLI for Humans and LLMs

A Go CLI for the Hopsworks Feature Store. Works as a standalone tool for humans and as an AI tool interface for LLMs — no MCP server needed.

## Philosophy

Build a great CLI, and both humans and LLMs benefit. `hops` is the simplest way to give an AI agent access to Hopsworks: just allow `Bash(hops *)` and it works. No protocol, no server, no overhead.

## Install

```bash
# Build from source
go build -o hops .

# Or inside Hopsworks terminal (auto-detects environment)
./hops init    # Sets up Claude Code integration
```

## Quick Start

```bash
# Login (prompted for host + API key)
hops login

# Or if inside Hopsworks terminal, it auto-configures
hops project list
hops project use myproject

# Explore
hops fg list
hops fg info customer_transactions
hops fg preview customer_transactions --n 5
hops fg features customer_transactions

# Feature views
hops fv list
hops fv create my_view --version 1 --feature-group customer_transactions

# Context dump (for LLMs)
hops context
```

## Commands

| Command | Description |
|---------|------------|
| `hops login` | Authenticate with Hopsworks |
| `hops project list\|use\|info` | Manage projects |
| `hops fs list` | List feature stores |
| `hops fg list\|info\|preview\|features\|create\|delete` | Feature groups |
| `hops fv list\|info\|create\|delete` | Feature views |
| `hops td list\|create\|delete` | Training datasets |
| `hops job list` | List jobs |
| `hops dataset list\|mkdir` | Browse project files |
| `hops init` | Set up Claude Code integration |
| `hops context` | Dump project state for LLMs |
| `hops update` | Check for updates |

## Claude Code Integration

Run `hops init` to:
1. Create a `/hops` skill in `~/.claude/commands/hops.md`
2. Add `Bash(hops *)` to allowed permissions
3. Auto-detect Hopsworks terminal environment

Then in Claude Code, type `/hops` to activate — Claude knows how to use every command.

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

Config is saved to `~/.hops/config`.

## Architecture

```
hopsworks-cli/
├── main.go              # Entry point
├── cmd/                 # Cobra commands (one file per resource)
├── pkg/
│   ├── client/          # HTTP client for Hopsworks REST API
│   ├── config/          # Config management (~/.hops/config)
│   └── output/          # Table vs JSON formatting
└── templates/           # Skill templates
```

## Roadmap

- [ ] `hops fg insert` — Data ingestion
- [ ] `hops fg stats` — Compute/view statistics
- [ ] `hops fv serve` — Online feature serving
- [ ] `hops model list|info|deploy` — Model registry
- [ ] `hops deployment list|create` — Model serving
- [ ] `hops update` — Self-improving against API docs
- [ ] Cross-platform binary releases
