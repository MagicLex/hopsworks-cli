# The Plan

## What We're Building

A `hops` CLI in Go that works for humans and LLMs. No MCP server needed — just `Bash(hops *)`.

## What We Have

### Repos (cloned, authenticated, push access)

| Repo | Branch | Location | Push? |
|------|--------|----------|-------|
| [hopsworks-ee](https://github.com/MagicLex/hopsworks-ee) | `feature/terminal-4.8-clean` | `~/hopsworks-ee` | Yes |
| [hopsworks-front](https://github.com/MagicLex/hopsworks-front) | `feature/terminal-ui` | `~/hopsworks-front` | Yes (remote) |
| [hopsworks-api](https://github.com/MagicLex/hopsworks-api) | `new_mcp` | `~/hopsworks-api` | Yes |
| [hopsworks-cli](https://github.com/MagicLex/hopsworks-cli) | `main` | `~/hopsworks-cli` | Yes |

### What's Working Now

- `hops` CLI v0.1.0 — 14 commands, tested live against this Hopsworks cluster
- Feature Store: `fg list/info/preview/features/create/delete`, `fv`, `td`, `job`, `dataset`
- Claude integration: `hops init` creates `/hops` skill, `hops context` dumps project state
- kubectl installed with edit access to `hopsworks` + `lexterm` namespaces

## The `devMode` Flag

**Problem:** Terminal pods run with a locked-down service account. No kubectl, no internal service access. Good for users, bad for developers.

**Solution:** Add a `devMode` boolean to terminal sessions (like `persistentHome`). When true:

1. **RBAC** — Create RoleBindings granting `edit` on `hopsworks` + project namespace
2. **kubectl** — Pre-install kubectl in the terminal image (or download on first use)
3. **Env var** — Set `TERMINAL_DEV_MODE=true` so the `hops` CLI auto-detects it
4. **Network policy** — Relax `terminal-isolation` to allow direct access to internal services
5. **Admin-only** — Only `HOPS_ADMIN` users can enable devMode

### Changes Required

**hopsworks-ee** (new branch: `feature/terminal-dev-mode` from `feature/terminal-4.8-clean`):

| File | Change |
|------|--------|
| `TerminalSession.java` | Add `devMode` column (Boolean) |
| `TerminalDTO.java` | Add `devMode` field |
| `TerminalService.java` | Add `devMode` query param to `/start`, admin check |
| `TerminalController.java` | Pass `devMode` through to manager |
| `TerminalManager.java` | Add `devMode` to interface |
| `KubeTerminalManager.java` | Create RoleBindings, set env var, relax NetworkPolicy |
| `Constants.java` | Add dev mode constants |
| SQL migration | Add `dev_mode` column to `terminal_session` |

**hopsworks-cli** (on `main`):
- `hops cluster` commands (Phase 1 of internal mode) — only active when `TERMINAL_DEV_MODE=true`
- `hops status` — show internal mode detection

## The Self-Improving Loop

```
hops update
  → checks Hopsworks REST API for new endpoints
  → compares against implemented CLI commands
  → reports gaps
  → (future) auto-scaffolds new commands
```

Build the CLI, use it, improve it, repeat. Better for humans = better for LLMs.

## Execution Order

1. **Done** — `hops` CLI with Feature Store commands, Claude integration
2. **Next** — `feature/terminal-dev-mode` branch in hopsworks-ee (RBAC + env var + relaxed network)
3. **Then** — `hops cluster pods/logs/restart` commands (kubectl wrapper)
4. **Then** — `hops hdfs/rondb/kafka` commands (direct service clients)
5. **Later** — `hops model/deployment` (ML serving), `hops update` (self-improvement)
