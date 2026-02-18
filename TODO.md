# Feature Store — TODO

Status: in progress

## What exists
- **FG:** list, info, preview, features, create, delete, insert (Python SDK)
- **FV:** list, info, create (single FG only), delete
- **TD:** list, create, delete

---

## Phase 1: Statistics
> No dependencies, immediate value.

- [ ] `pkg/client/statistics.go` — DTOs + client methods (GET stats, POST compute)
- [ ] `cmd/fg.go` — add `hops fg stats <name> [--version N] [--features col1,col2]`
- [ ] `cmd/fg.go` — add `hops fg stats <name> --compute` (trigger Spark job)
- [ ] Test against live cluster

REST: `GET/POST {FSPath}/featuregroups/{id}/statistics`

## Phase 2: Storage Connectors + External FGs
> Unlocks on-demand feature groups (JDBC, S3, etc.)

- [ ] `pkg/client/connector.go` — StorageConnector DTOs + list
- [ ] `cmd/connector.go` — `hops connector list`
- [ ] `pkg/client/featuregroup.go` — add external FG support (onDemandFeaturegroupDTO)
- [ ] `cmd/fg.go` — `hops fg create-external <name> --connector <name> --query "SQL" --features "col:type,..."`
- [ ] Test against live cluster

REST: `GET {FSPath}/storageconnectors`, `POST {FSPath}/featuregroups` (type=onDemandFeaturegroupDTO)

## Phase 3: FV Create with Joins
> Core structural change — multi-FG feature views.

- [ ] `pkg/client/featureview.go` — QueryDTO, JoinDTO, update CreateFeatureView
- [ ] `cmd/fv.go` — `--join "fg:JOIN_TYPE:left=right:prefix"` flag (repeatable)
- [ ] `cmd/fv.go` — `--from query.json` for complex queries
- [ ] Backwards compatible: existing `--feature-group` still works
- [ ] Test against live cluster

REST: `POST {FSPath}/featureview` with QueryDTO containing joins list

## Phase 4: Transformations (list + reference only)
> Creating Python UDFs from Go is impractical. List existing, reference in FV.

- [ ] `pkg/client/transformation.go` — TF DTOs + list
- [ ] `cmd/transformation.go` — `hops transformation list`
- [ ] `cmd/fv.go` — `--transformation "fn_name:feature"` flag
- [ ] `pkg/client/featureview.go` — add TF refs to create request
- [ ] Test against live cluster

REST: `GET {FSPath}/transformationfunctions`

## Phase 5: FV Query/Preview
> Inspect what a feature view actually produces.

- [ ] `pkg/client/featureview.go` — GetFeatureViewQuery, PreviewFeatureView
- [ ] `cmd/fv.go` — `hops fv query <name> [--version N]`
- [ ] `cmd/fv.go` — `hops fv preview <name> [--version N] [--n 10]`
- [ ] Test against live cluster

REST: `GET {FSPath}/featureview/{name}/version/{version}/query`

---

## Backend concerns
1. Stats compute needs Spark — reading existing stats works, compute may need cluster resources
2. External FG — same endpoint, different type field — needs live testing
3. FV with QueryDTO — Python SDK does this already, no backend changes expected
4. FV preview endpoint — verify it exists, fallback = show query

## Dev loop
1. Code (CLI / API / backend)
2. `go build -o hops .` for CLI
3. `mvn clean install -P kube-cluster,remote-user-auth,kube,cloud -DskipTests` for backend
4. `./scripts/kube_redeploy_ear.sh` or Payara UI for deploy
5. Test `hops` against real cluster
6. Repeat
