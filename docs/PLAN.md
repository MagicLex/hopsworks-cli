# The Plan

## What We're Building

A `hops` CLI in Go that works for humans and LLMs. No MCP server needed — just `Bash(hops *)`.

## What We Have

### Repos (cloned, authenticated, push access)

| Repo | Branch | Location | Push? |
|------|--------|----------|-------|
| [hopsworks-ee](https://github.com/MagicLex/hopsworks-ee) | `feature/terminal-dev-mode` | `~/hopsworks-ee` | Yes |
| [hopsworks-front](https://github.com/MagicLex/hopsworks-front) | `feature/terminal-ui` | `~/hopsworks-front` | Yes (remote) |
| [hopsworks-api](https://github.com/MagicLex/hopsworks-api) | `new_mcp` | `~/hopsworks-api` | Yes |
| [hopsworks-cli](https://github.com/MagicLex/hopsworks-cli) | `main` | `~/hopsworks-cli` | Yes |

### What's Working Now

- `hops` CLI v0.1.0 — 14 commands, tested live against this Hopsworks cluster
- Feature Store: `fg list/info/preview/features/create/delete`, `fv`, `td`, `job`, `dataset`
- Claude integration: `hops init` creates `/hops` skill, `hops context` dumps project state
- kubectl installed with edit access to `hopsworks` + `lexterm` namespaces

---

## The `devMode` Flag

**Problem:** Terminal pods run with a locked-down service account. No kubectl, no internal service access. Good for users, bad for developers.

**Solution:** Add a `devMode` boolean to terminal sessions (like `persistentHome`). When true:

1. **RBAC** — RoleBindings granting `edit` on `hopsworks` + project namespace
2. **kubectl** — Pre-install kubectl in the terminal image (or download on first use)
3. **Env var** — `TERMINAL_DEV_MODE=true` so the `hops` CLI auto-detects it
4. **Network policy** — Separate `terminal-dev-mode` NetworkPolicy (additive, doesn't touch `terminal-isolation`)
5. **Admin-only** — Only `HOPS_ADMIN` users can enable devMode

### Status: CODE COMPLETE, NEEDS BUILD

The Java backend changes are written on `feature/terminal-dev-mode` (branched from `feature/terminal-4.8-clean`). **Not yet committed** — changes are staged in the working tree at `~/hopsworks-ee`.

No JDK/Maven in the terminal environment. These changes must be **compiled and tested outside** (CI pipeline or local dev machine).

### Files Changed (9 files)

| File | Change | Status |
|------|--------|--------|
| `V55__[HWORKS-XXXX]_terminal_dev_mode.sql` | `ALTER TABLE terminal_session ADD COLUMN dev_mode` | NEW |
| `TerminalSession.java` | `@Column(name = "dev_mode") Boolean devMode` + getter/setter | Modified |
| `TerminalDTO.java` | `Boolean devMode` field, new 10-arg constructor | Modified |
| `TerminalService.java` | `@QueryParam("devMode")`, admin gate via `BbcGroup` check | Modified |
| `TerminalController.java` | `boolean devMode` param threaded to manager + facade | Modified |
| `TerminalSessionFacade.java` | New `save()` overload with `devMode` | Modified |
| `TerminalManager.java` | New `start()` overload with `devMode` | Modified |
| `KubeTerminalManager.java` | `ensureDevModeRbac()`, `ensureDevModeNetworkPolicy()`, `TERMINAL_DEV_MODE` env var, `terminal-dev-mode` pod label | Modified |
| `Constants.java` | `LABEL_DEV_MODE`, `DEV_MODE_NETWORK_POLICY`, `DEV_MODE_ROLE_BINDING_SUFFIX` | Modified |

### Build & Deploy Steps

```bash
# 1. On a machine with JDK 17+ and Maven:
cd ~/hopsworks-ee
git checkout feature/terminal-dev-mode

# 2. Commit the changes (not committed yet!)
git add -A
git commit -m "feat(terminal): Add devMode flag for elevated RBAC + kubectl access"

# 3. Compile
mvn -pl hopsworks-persistence,hopsworks-common,hopsworks-api,hopsworks-kube -am compile

# 4. Full build (produces the Payara EAR)
mvn clean package -DskipTests

# 5. Deploy to Hopsworks cluster
#    - Apply SQL migration V55 to the database
#    - Deploy the new EAR to Payara/GlassFish
#    - Restart the Hopsworks service

# 6. Test
#    POST /hopsworks-api/api/project/{id}/terminal/start?devMode=true
#    → should return DTO with devMode=true
#    → terminal pod should have TERMINAL_DEV_MODE=true env var
#    → kubectl should work inside the terminal
```

### What devMode Creates in Kubernetes

When `devMode=true`, the `KubeTerminalManager` does three extra things:

1. **RoleBindings** (parallel async):
   - `{user}-terminal-dev-edit` in project namespace → ClusterRole `edit`
   - `{user}-terminal-dev-edit` in hopsworks namespace → ClusterRole `edit`

2. **NetworkPolicy** `terminal-dev-mode`:
   - Selects pods with label `terminal-dev-mode=true`
   - Allows all ingress (additive — doesn't weaken `terminal-isolation` for non-dev pods)

3. **Environment variable**: `TERMINAL_DEV_MODE=true` on the terminal container

---

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
2. **Done (code)** — `feature/terminal-dev-mode` Java changes (needs build + deploy)
3. **Next** — `hops cluster pods/logs/restart` commands (kubectl wrapper, can develop NOW — this terminal already has access)
4. **Then** — `hops hdfs/rondb/kafka` commands (direct service clients)
5. **Later** — `hops model/deployment` (ML serving), `hops update` (self-improvement)

> **Note:** Steps 3-5 don't depend on step 2 being deployed. This terminal already has kubectl + edit RBAC. The devMode flag just productionizes that access for other admin users.
