# Dev Loop

We develop **inside** the Hopsworks cluster. Build, test, fix — same session.

## Setup

```bash
# CLI code lives here
cd ~/hopsworks-cli

# Binary goes here (aliased to `hops`)
go build -o ~/hops-bin .
alias hops=~/hops-bin

# Always test as a real user would
hops --version
hops fg list
```

## Build + Test Cycle

```bash
# 1. Edit code
vim cmd/job.go

# 2. Build
go build -o ~/hops-bin .

# 3. Test live against the cluster
hops job create testjob --type python --app-path Resources/jobs/test.py
hops job run testjob --wait
hops job logs testjob
hops job delete testjob

# 4. If it works, commit
git add cmd/job.go pkg/client/job.go
git commit -m "feat: add job create command"
```

## Backend Access

We have full access to backend/API source and can kubectl the cluster:

```bash
# Backend source (Java)
ls ~/hopsworks-ee

# API/SDK source (Python)
ls ~/hopsworks-api

# Kubernetes
kubectl get pods -n hopsworks
kubectl logs -n hopsworks <pod>
```

If a backend fix is needed:
1. Make the fix
2. Document in the appropriate `docs/*.md` (e.g. `hopsworks-ee-fixes.md`)
3. Port upstream later
4. **No untracked monkey patches**

## What's Available

- Go compiler, Python 3.11, pip
- kubectl with edit access to `hopsworks` + `lexterm` namespaces
- HopsFS via FUSE mount at `/hopsfs/`
- Hopsworks REST API (auto-authenticated via JWT)
- Python SDK (`import hopsworks; project = hopsworks.login()`)

## Error Format Convention

All `fmt.Errorf()` messages follow these patterns (Go convention: lowercase, no trailing period):

| Category | Pattern | Example |
|----------|---------|---------|
| Missing flag | `--flag is required` | `--url is required` |
| Not found | `<thing> '<name>' not found: %w` | `feature group 'fg1' not found: %w` |
| Operation | `<verb> <noun>: %w` | `register model: %w` |
| Invalid input | `invalid <thing>: <detail>` | `invalid chart ID: abc` |
| Conditional | `--flag-a or --flag-b is required` | `--password or --token is required` |

**Never**: `"X failed: %w"`, `"failed to X: %w"`, `"could not X: %w"`, capitalized starts, trailing periods.

## Rules

- Always use `hops` (the alias), not `~/hops-bin`
- Clean up temp files after testing
- Document everything — findings, quirks, fixes
- Test like a real user would
