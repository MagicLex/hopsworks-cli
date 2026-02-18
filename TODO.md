# Feature Store — TODO

Status: in progress

## What exists
- **FG:** list, info, preview, features, create (with embeddings), delete, insert (Python SDK), stats, search (KNN), derive (join + provenance)
- **FV:** list, info (shows source FGs + joins), create (multi-FG joins + transforms), get (online vectors), read (batch offline), delete
- **Transformations:** list, create (file + inline @udf)
- **TD:** list, create, compute (materialize with splits), read (retrieve with splits), delete
- **Job:** list, status (with --wait polling)
- **Model:** list, info, delete, register (Python SDK, with provenance + schema + input example), download (Python SDK)
- **Deployment:** list, info, create (REST), start, stop, delete, predict, logs
- **Other:** update (self-update from GitHub releases), --version, init (Claude Code integration)

---

## Phase 1: Statistics + Insert Pipeline — DONE
- [x] `pkg/client/statistics.go` — DTOs + client methods
- [x] `cmd/fg.go` — `hops fg stats <name> [--version N] [--features col1,col2]`
- [x] `cmd/fg.go` — `hops fg stats <name> --compute` (trigger Spark job)
- [x] FG recreated as DELTA (was HUDI, incompatible with 4.8 backend)
- [x] JKS certs extracted to PEM at ~/.hopsfs_pems/
- [x] SDK patches applied via user site-packages overlay (see docs/SDK-FIXES.md)
- [x] CLI sets PEMS_DIR + LIBHDFS_DEFAULT_USER on subprocess, removed except IndexError hack
- [x] Fixed preview API parsing (row vs rows mismatch)
- [x] `hops job status <name> [--wait] [--poll N]` — track materialization jobs
- [x] End-to-end tested: insert → preview → stats (all working)

---

## Phase 1.5: FG Derive — DONE
- [x] `cmd/fg_derive.go` — `hops fg derive <name> --base <fg> --join "<spec>" --primary-key <cols>`
- [x] Join spec parser: `"<fg>[:<ver>] <INNER|LEFT|RIGHT|FULL> <on>[=<right_on>] [prefix]"`
- [x] Multiple `--join` flags, optional `--online`, `--event-time`, `--features`, `--description`
- [x] Provenance: passes `parents=[base_fg, join_fgs...]` for Hopsworks lineage graph
- [x] Auto-generated description with derivation lineage when `--description` omitted
- [x] Validates all source FGs exist via Go REST before running Python
- [x] End-to-end tested: derive → preview → provenance graph (all working)

---

## Phase 2: Storage Connectors + External FGs
> Unlocks on-demand feature groups (JDBC, S3, etc.)

- [ ] `pkg/client/connector.go` — StorageConnector DTOs + list
- [ ] `cmd/connector.go` — `hops connector list`
- [ ] `pkg/client/featuregroup.go` — add external FG support (onDemandFeaturegroupDTO)
- [ ] `cmd/fg.go` — `hops fg create-external <name> --connector <name> --query "SQL" --features "col:type,..."`
- [ ] Test against live cluster

## Phase 3: FV Create with Joins — DONE
> Core structural change — multi-FG feature views.

- [x] `pkg/client/featureview.go` — FVJoinSpec, nested query DTO, buildFGRef/buildFeatureList helpers
- [x] `cmd/fv.go` — `--join "fg LEFT on[=right_on] [prefix]"` flag (repeatable, reuses parseJoinSpec)
- [x] Backwards compatible: existing `--feature-group` still works (no joins = same as before)
- [x] `fv info` shows source FGs + joins via `/query` sub-endpoint
- [x] `GetFeatureViewQuery` client method parses query response
- [x] End-to-end tested: single FG create, joined FV create, info display (all working)

## Phase 4: Transformations — DONE
> List, create custom, attach to feature views.

- [x] `pkg/client/transformation.go` — DTOs + list/get/create methods
- [x] `cmd/transformation.go` — `hops transformation list` + `create --file/--code`
- [x] Custom UDF parsing via Python AST (extracts name, args, return types)
- [x] Local save: custom transforms saved to `~/.hops/transformations/`
- [x] `cmd/fv.go` — `--transform "fn_name:column"` (repeatable), resolves TF by name
- [x] `pkg/client/featureview.go` — FVTransformSpec, serializes transformationFunctions array
- [x] End-to-end tested: list built-ins, create custom (file + inline), attach to FV

## Phase 5: FV Read/Get + TD Compute/Read — DONE
> Pre-training data pipeline: online lookup, batch read, TD materialization + retrieval.

- [x] `cmd/fv_read.go` — `hops fv get <name> --entry "pk=value"` (online feature vector lookup)
- [x] `cmd/fv_read.go` — `hops fv read <name> [--n N] [--output file]` (batch offline read)
- [x] `cmd/td.go` — `hops td compute <fv> <ver> [--split "train:0.8,test:0.2"]` (materialize via SDK)
- [x] `cmd/td.go` — `hops td read <fv> <ver> --td-version N [--split train] [--output file]` (retrieve TD)
- [x] Shared helpers: `buildFVPreamble()`, `runPython()`, `pythonLiteral()` in `fv_read.go`
- [x] End-to-end tested: fv get (online), fv read (batch+file+json), td compute (no-split + split), td read (full+split+file)

## Phase 6: Embeddings & Similarity Search — DONE
> Vector embeddings in feature groups + KNN similarity search.

- [x] `pkg/client/featuregroup.go` — `EmbeddingFeature`, `EmbeddingIndex` structs in request + response
- [x] `cmd/fg.go` — `--embedding "name:dim[:metric]"` flag on `fg create`, auto-adds `array<float>` feature, auto-enables online
- [x] `cmd/fg.go` — `fg info` displays embedding index (column, dimension, metric)
- [x] `cmd/fg_search.go` — `hops fg search <name> --vector "..." --k N` via Python SDK `find_neighbors()`
- [x] End-to-end tested: create with embedding, insert vectors, KNN search (cosine), JSON mode

## Phase 7: Model Registry & Serving — DONE
> Full ML lifecycle: register models, deploy, predict, manage.

- [x] `pkg/client/model.go` — Model DTOs + list/get/delete methods
- [x] `cmd/model.go` — `hops model list`, `info <name> [--version]`, `delete <name> --version`
- [x] `cmd/model_register.go` — `hops model register <name> <path> --framework --metrics --description --feature-view --td-version --input-example --schema --program`
- [x] `cmd/model_register.go` — `hops model download <name> [--version] [--output dir]`
- [x] `pkg/client/deployment.go` — Deployment DTOs + list/get/create/delete/action/logs/predict
- [x] `cmd/deployment.go` — `hops deployment list/info/create/start/stop/delete/predict/logs`
- [x] Deployment create via Go REST (SDK `model.deploy()` is broken for PYTHON framework)
- [x] Fixed `get_best_model` → `get_models` fallback (SDK requires metric+direction args)
- [x] End-to-end tested: model list/info, deployment create/start/info (all working)

---

## Known cluster issues
- **RSS CRD missing** — apply `charts/spark/crds/uniffle.apache.org_remoteshuffleservices.yaml --server-side`, recreate from ConfigMap `rss-crd`, restart `rss-controller`. See `docs/CLUSTER-OPS.md`.
- **DDL migration missing** — `terminal_session.dev_mode` column not in DB. Applied manually: `ALTER TABLE terminal_session ADD COLUMN dev_mode TINYINT(1) DEFAULT 0;`
- **Terminal JWT expiry** — tokens expire after ~8h, restart terminal session to refresh.
- **API key scope** — current key lacks SERVING scope, blocks Python SDK login locally. Use terminal pod instead.

## Dev loop
See `docs/DEV-LOOP.md`
