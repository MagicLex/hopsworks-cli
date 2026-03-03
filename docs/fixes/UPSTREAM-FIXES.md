# Upstream Fixes — Discovered via CLI Development

All issues found while building/testing the Hopsworks CLI against a 4.8 cluster.

---

## 1. hopsworks-api (Python SDK)

**Repo upstream**: https://github.com/logicalclocks/hopsworks-api
**Fork**: https://github.com/MagicLex/hopsworks-api
**Branch**: `fix/cli-terminal` (based on upstream `main`)

### Fix 1.1: `commit_details` IndexError on first insert

**File**: `python/hsfs/feature_group.py` — `save()` (~line 3254) and `insert()` (~line 3457)
**Severity**: Critical — insert silently reports success while no data is written
**Status**: Fixed in fork. Needs PR to upstream.

**Problem**: On first insert into a Delta FG, no commits exist yet:
```python
commit_id = list(self.commit_details(limit=1))[0]  # IndexError
```
The CLI's generated Python script caught this with `except IndexError: pass` — masking total data loss.

**Fix**:
```python
commits = list(self.commit_details(limit=1))
if commits:
    self._statistics_engine.compute_and_save_statistics(
        metadata_instance=self,
        feature_dataframe=feature_dataframe,
        feature_group_commit_id=commits[0],
    )
```

### Fix 1.2: `delta_engine` skips PEMS_DIR for internal clients

**File**: `python/hsfs/core/delta_engine.py` — `_setup_delta_rs()`
**Severity**: High — blocks all Delta writes from terminal/Jupyter pods
**Status**: Fixed in fork. Needs PR to upstream.

**Problem**: Only sets `PEMS_DIR` and `LIBHDFS_DEFAULT_USER` when `_client._is_external()` is True. Internal clients (terminal pods, Jupyter) are skipped, so `libhdfs` can't do mTLS.

**Fix** — added `else` branch:
```python
else:
    material_dir = os.environ.get("MATERIAL_DIRECTORY", "")
    hadoop_user = os.environ.get("HADOOP_USER_NAME", "")
    if material_dir and "PEMS_DIR" not in os.environ:
        os.environ["PEMS_DIR"] = material_dir
    if hadoop_user and "LIBHDFS_DEFAULT_USER" not in os.environ:
        os.environ["LIBHDFS_DEFAULT_USER"] = hadoop_user
```

**Note**: Requires PEM files to exist in `MATERIAL_DIRECTORY`. If only JKS files are present (default today), the hopsworks-ee fix (2.1) is also needed.

### SDK Installation (temporary workaround)

System `hsfs` is root-owned. Applied via **user site-packages overlay**:
1. Copied system package to `~/.local/lib/python3.11/site-packages/hsfs/`
2. Patched `feature_group.py` (Fix 1.1) at 2 locations
3. Copied `core/delta_engine.py` from fork (Fix 1.2)

```bash
# Recreate from scratch:
cp -r /usr/local/lib/python3.11/dist-packages/hsfs/ ~/.local/lib/python3.11/site-packages/hsfs/
# Then apply patches manually or copy from fork
```

**Proper fix**: upstream PR + new release, or rebuild terminal image with patched SDK.

---

## 2. hopsworks-ee (Java backend)

**Repo**: https://github.com/MagicLex/hopsworks-ee
**Branch**: `feature/terminal-dev-mode`

### Fix 2.1: Terminal pod needs PEM certs for Delta/HDFS writes

**Where**: `KubeTerminalManager.java`
**Severity**: High — blocks all Python SDK inserts from terminal pods
**Status**: Open

**Problem**: Terminal pod mounts JKS keystores at `/srv/hops/certs/` but `hops-deltalake` (Rust, via `libhdfs` C) expects PEM files.

**Fix**: Add init container or startup script to extract PEM from JKS at pod boot:
```bash
PEMS_DIR=/srv/hops/pems
mkdir -p $PEMS_DIR
# Extract from keystore -> client_key.pem + client_cert.pem
# Extract from truststore -> ca_chain.pem
# Password from ${MATERIAL_DIRECTORY}/${HADOOP_USER_NAME}__cert.key
```

Then add env var:
```java
envVars.add(new V1EnvVar().name("PEMS_DIR").value("/srv/hops/pems"));
```

**Alternative**: Mount PEM certs as a separate Secret/ConfigMap alongside the JKS ones.

### Fix 2.2: Set LIBHDFS_DEFAULT_USER env var on terminal pod

**Where**: `KubeTerminalManager.java`
**Severity**: Medium
**Status**: Open

**Fix**:
```java
envVars.add(new V1EnvVar().name("LIBHDFS_DEFAULT_USER").value(projectUser));
// projectUser = "lexterm__meb10000" format
```

`HADOOP_USER_NAME` is already set but `libhdfs` reads `LIBHDFS_DEFAULT_USER` specifically.

### Issue 2.3: REST `POST /featuregroups` does not provision Kafka topic or materialization job

**Severity**: High — blocks all REST/CLI-based data ingestion
**Status**: Open

**Problem**: Creating a FG via REST with `onlineEnabled: true` creates metadata only (Hive table, schema, flag) but does NOT:
1. Provision Kafka topic (`topicName` remains `null`)
2. Create materialization job (no Spark job for offline sync)
3. Initialize Hudi commit tracking

Any subsequent Kafka insert silently drops data. The FG looks correct in UI/API but is a dead end.

**Where to fix**:

| File | Change |
|------|--------|
| `FeaturegroupController.java` | Trigger Kafka topic provisioning if `onlineEnabled` after FG creation |
| Kafka topic provisioning service | Ensure it's called from REST create path, not just SDK path |
| Materialization job service | Create the offline materialization job on FG creation |

**Workaround**: Use Python SDK `get_or_create_feature_group()` for creation. The SDK's first `insert()` provisions everything. The CLI uses this workaround.

### Issue 2.4: No Spark shuffle coordinator in terminal pod

**Severity**: Medium — blocks offline materialization from terminal
**Status**: Known limitation

Terminal pod has no Spark shuffle coordinator. Offline materialization jobs fail with `Cannot find any Spark shuffle coordinator pods`.

**Workaround**: Use `--online-only` flag for inserts, or trigger materialization from Hopsworks UI/Jobs page.

### Issue 2.5: Brewer serving record required for chart/dashboard UI

**Severity**: Low — UI-only coupling
**Status**: Workaround applied

**Problem**: Frontend gates the "Add Dashboard" button behind `brewer_enabled` setting (`Dashboards/index.tsx` lines 61-69). When `brewer_enabled=true`, the backend's `ChatController.getProjectBrewerServing()` looks for a serving record named `brewer`. If missing:

```
Serving for LLM Assistant in <project> not found
```

Charts/dashboards have zero backend dependency on Brewer — this is pure frontend coupling.

**Workaround**: Create a stopped stub deployment named `brewer` via CLI:
```bash
mkdir /tmp/brewer_stub && echo '{}' > /tmp/brewer_stub/model.json
cat > /tmp/brewer_stub/predict.py << 'EOF'
class Predict:
    def __init__(self): pass
    def predict(self, inputs): return {"predictions": []}
EOF
hops model register brewerstub /tmp/brewer_stub --framework python --description "Stub for brewer serving record"
cp /tmp/brewer_stub/predict.py /hopsfs/Models/brewerstub/1/Files/predict.py
hops deployment create brewerstub --script predict.py --name brewer
rm -rf /tmp/brewer_stub
```

**Proper fix**: Decouple the frontend — remove the `brewerEnabled` gate from the dashboard "Add" button.

---

## 3. Arrow Flight Server

**Where**: ArrowFlight deployment pods, `/usr/src/app/src/`
**Status**: Applied as ConfigMap overlay

### Fix 3.1: `UnboundLocalError` in `query_engine.py`

**File**: `query_engine.py` — `read_query()` method
**Severity**: High — blocks TD materialization for any feature view with external FGs

**Problem**: `connectors` only assigned inside `if/elif` branches. If neither condition is met, variable is undefined when the `for` loop uses it.

**Fix**: Initialize `connectors = {}` before the conditional block.

### Fix 3.2: TD creation never decrypts encrypted connectors

**File**: `arrow_dataset_reader_writer.py` — `create_parquet_dataset()` method
**Severity**: High — `td compute` fails for feature views with external FGs (Snowflake, JDBC)

**Problem**: `read_query(query)` is called without `is_query_signed=True`, so encrypted connectors are never decrypted. The `do_get` path (`fv read`) works, but `do_action` (`td compute`) doesn't.

**Fix**:
```python
has_encrypted_connectors = bool(query.get("connectors_encrypted"))
result_batches = self.query_engine.read_query(query, is_query_signed=has_encrypted_connectors)
```

### Fix 3.3: External FG source queries don't push down LIMIT

**Files**: `query_engine.py` (caller) + `engine/sql_query_engine.py` (source fetch)
**Severity**: High — preview/read on large external tables (e.g. 5.4B rows Snowflake) hangs indefinitely

**Problem**: `sql_query_engine.py` fetches the entire external table before DuckDB applies `FETCH NEXT N ROWS ONLY`. Arrow Flight tries to download everything from Snowflake/BigQuery/etc.

**Fix**: Extract the LIMIT from the outer DuckDB query and push it down to the source connector query.

In `query_engine.py`, `read_query()`:
```python
import re

# Before _register_featuregroups:
source_limit = None
if external_featuregroup_connectors:
    m = re.search(r'FETCH\s+NEXT\s+(\d+)\s+ROWS?\s+ONLY', query_string, re.IGNORECASE)
    if not m:
        m = re.search(r'LIMIT\s+(\d+)', query_string, re.IGNORECASE)
    if m:
        source_limit = int(m.group(1))

# Pass to _register_featuregroups -> sql_engine.register(source_limit=source_limit)
```

In `engine/sql_query_engine.py`, `register()` and `_ibis()`/`_redshift()`:
```python
def register(self, ..., source_limit=None):
    # pass source_limit to _ibis/_redshift

def _ibis(self, ..., source_limit=None):
    query = f"SELECT {feature_string} FROM ({connector_query})"
    if source_limit is not None:
        query += f" LIMIT {source_limit}"
```

**Result**: Preview of 5.4B-row Snowflake table goes from infinite timeout to ~5 seconds. Affects both CLI and UI preview.

### Deployment (ConfigMap overlay)

All three fixes (3.1, 3.2, 3.3) deployed as a ConfigMap mount on the ArrowFlight deployment:

```bash
# ConfigMap: arrowflight-query-engine-patch
# Mounts patched files over originals:
#   query_engine.py -> /usr/src/app/src/query_engine.py
#   arrow_dataset_reader_writer.py -> /usr/src/app/src/arrow_dataset_reader_writer.py
#   sql_query_engine.py -> /usr/src/app/src/engine/sql_query_engine.py

# To remove after backend upgrade includes fixes:
kubectl -n hopsworks delete configmap arrowflight-query-engine-patch
# + remove volume mounts from deployment
```

---

## Reference: FG Types & Write Paths

| | `cachedFeaturegroupDTO` | `streamFeatureGroupDTO` |
|---|---|---|
| Python engine write | Direct Delta to HDFS | Kafka -> online + Spark job -> offline |
| Online store | NOT populated | Populated via Kafka |
| Offline store | Written directly (fast) | Written by materialization job (~2min) |
| Use when | Offline-only (training data) | Online+offline (serving + training) |

CLI picks automatically: `hops fg create --online` -> stream, `hops fg create` -> cached.

## Reference: Insert Architecture (terminal pod)

```
hops fg insert (Go CLI)
  -> generates Python script
    -> hsfs Python SDK (feature_group.insert)
      -> hops-deltalake (Python wheel, Rust binary)
        -> hopsfs-native-object-store (Rust)
          -> libhdfs (HopsFS C library, bundled in wheel)
            -> mTLS to NameNode (hdfs://namenode.hopsworks.svc.cluster.local:8020)
```

## Reference: Repos Investigated

| Repo | Purpose |
|------|---------|
| logicalclocks/hopsworks-api | Python SDK (`hsfs`) |
| logicalclocks/delta-rs | `hops-deltalake` wheel build |
| logicalclocks/hopsfs-native-object-store | HDFS object store layer (branch 1.1.0) |
