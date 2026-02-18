# Feature Store — TODO

Status: in progress

## What exists
- **FG:** list, info, preview, features, create, delete, insert (Python SDK), stats
- **FV:** list, info, create (single FG only), delete
- **TD:** list, create, delete
- **Other:** update (self-update from GitHub releases), --version, init (Claude Code integration)

---

## Phase 1: Statistics — DONE
- [x] `pkg/client/statistics.go` — DTOs + client methods
- [x] `cmd/fg.go` — `hops fg stats <name> [--version N] [--features col1,col2]`
- [x] `cmd/fg.go` — `hops fg stats <name> --compute` (trigger Spark job)
- [x] Tested against live cluster (endpoint works, no data yet to show real stats)

## Blocker: Insert data to test stats end-to-end
- [ ] Get `hops fg insert --generate 50` working from terminal pod
  - Terminal JWT expires — need fresh session
  - Binary published at GitHub releases v0.2.0
  - From terminal: `curl -L .../hops-linux-amd64 -o ~/hops && chmod +x ~/hops`

---

## Phase 2: Storage Connectors + External FGs
> Unlocks on-demand feature groups (JDBC, S3, etc.)

- [ ] `pkg/client/connector.go` — StorageConnector DTOs + list
- [ ] `cmd/connector.go` — `hops connector list`
- [ ] `pkg/client/featuregroup.go` — add external FG support (onDemandFeaturegroupDTO)
- [ ] `cmd/fg.go` — `hops fg create-external <name> --connector <name> --query "SQL" --features "col:type,..."`
- [ ] Test against live cluster

## Phase 3: FV Create with Joins
> Core structural change — multi-FG feature views.

- [ ] `pkg/client/featureview.go` — QueryDTO, JoinDTO, update CreateFeatureView
- [ ] `cmd/fv.go` — `--join "fg:JOIN_TYPE:left=right:prefix"` flag (repeatable)
- [ ] `cmd/fv.go` — `--from query.json` for complex queries
- [ ] Backwards compatible: existing `--feature-group` still works
- [ ] Test against live cluster

## Phase 4: Transformations (list + reference only)
> List existing, reference in FV. No Python UDF creation from Go.

- [ ] `pkg/client/transformation.go` — TF DTOs + list
- [ ] `cmd/transformation.go` — `hops transformation list`
- [ ] `cmd/fv.go` — `--transformation "fn_name:feature"` flag
- [ ] Test against live cluster

## Phase 5: FV Query/Preview
> Inspect what a feature view actually produces.

- [ ] `cmd/fv.go` — `hops fv query <name> [--version N]`
- [ ] `cmd/fv.go` — `hops fv preview <name> [--version N] [--n 10]`
- [ ] Test against live cluster

---

## Known cluster issues
- **RSS CRD missing** — apply `charts/spark/crds/uniffle.apache.org_remoteshuffleservices.yaml --server-side`, recreate from ConfigMap `rss-crd`, restart `rss-controller`. See `docs/CLUSTER-OPS.md`.
- **DDL migration missing** — `terminal_session.dev_mode` column not in DB. Applied manually: `ALTER TABLE terminal_session ADD COLUMN dev_mode TINYINT(1) DEFAULT 0;`
- **Terminal JWT expiry** — tokens expire after ~8h, restart terminal session to refresh.
- **API key scope** — current key lacks SERVING scope, blocks Python SDK login locally. Use terminal pod instead.

## Dev loop
See `docs/DEV-LOOP.md`
