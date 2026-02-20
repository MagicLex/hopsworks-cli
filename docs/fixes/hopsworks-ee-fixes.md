# hopsworks-ee — Fixes Needed

Repo: https://github.com/MagicLex/hopsworks-ee
Branch: `feature/terminal-dev-mode`

## Fix 1: Terminal pod needs PEM certs for Delta/HDFS writes

**Problem**: The terminal pod mounts JKS keystores at `/srv/hops/certs/` but the `hops-deltalake` Rust library (via `libhdfs` C) expects PEM files in a `PEMS_DIR` directory.

**Impact**: Any Python SDK insert from a terminal pod fails with `Connection to HopsFS failed` because libhdfs can't do mTLS without PEM certs.

**Where to fix**: `KubeTerminalManager.java` — the class that sets up the terminal pod spec.

**What to do**:
Add an init container or startup script that extracts PEM from JKS at pod boot:

```bash
# Using openssl (or pyjks in Python — both available in terminal image)
# Password is in ${MATERIAL_DIRECTORY}/${HADOOP_USER_NAME}__cert.key

PEMS_DIR=/srv/hops/pems
mkdir -p $PEMS_DIR

# Extract from keystore → client_key.pem + client_cert.pem
# Extract from truststore → ca_chain.pem
# (exact conversion code exists in ~/hopsworks-cli/docs/fixes/sdk-fixes.md)
```

Then set env var on the terminal container:
```java
envVars.add(new V1EnvVar().name("PEMS_DIR").value("/srv/hops/pems"));
```

**Alternative**: Mount PEM certs directly as a separate Secret/ConfigMap alongside the JKS ones. The CA, client cert, and client key are already known at pod creation time.

## Fix 2: HDFS write permissions for project users — LOW PRIORITY

**Original problem**: After fixing TLS, writes to `/apps/hive/warehouse/<project>_featurestore.db/` fail with `Failed to create file`.

**Current status**: Not a blocker for online-enabled FGs. The terminal writes to Kafka (online store), then a Spark materialization job handles the Delta/HDFS write with proper permissions. The early "Failed to create file" error was from the cert debugging phase.

**Still relevant for**: Offline-only FGs (no online store) where the Python engine writes directly to Delta via libhdfs. Not currently tested or needed.

## Fix 3: Set LIBHDFS_DEFAULT_USER env var

**Problem**: `libhdfs` needs `LIBHDFS_DEFAULT_USER` to identify itself to the NameNode. Without it, the connection may use wrong identity.

**Where to fix**: `KubeTerminalManager.java` — add env var:
```java
envVars.add(new V1EnvVar().name("LIBHDFS_DEFAULT_USER").value(projectUser));
// projectUser = "lexterm__meb10000" format
```

The `HADOOP_USER_NAME` is already set but libhdfs reads `LIBHDFS_DEFAULT_USER` specifically.

## Fix 4: Arrow Flight Server — external FG connectors in training datasets

**Problem**: Two bugs in the Arrow Flight Server prevent training dataset materialization from external (on-demand) feature groups.

**Impact**: `td compute` on any feature view that includes external FGs (e.g. Snowflake, JDBC) fails. `fv read` works fine because it goes through a different code path.

**Where**: Arrow Flight Server pods (`arrowflight-deployment`), files in `/usr/src/app/src/`.

### Bug 4a: `UnboundLocalError` in `query_engine.py`

`read_query()` only assigns `connectors` inside `if/elif` branches. If neither condition is met, the variable is undefined when the `for` loop iterates it.

**File**: `query_engine.py`, `read_query()` method

```python
# BEFORE (buggy):
filters = query.get("filters", None)
if "connectors" in query and query["connectors"]:
    connectors = query["connectors"]
elif "connectors_encrypted" in query and query["connectors_encrypted"] and is_query_signed:
    connectors = json.loads(...)

# AFTER (fixed):
filters = query.get("filters", None)
connectors = {}  # <-- initialize before conditional
if "connectors" in query and query["connectors"]:
    connectors = query["connectors"]
elif "connectors_encrypted" in query and query["connectors_encrypted"] and is_query_signed:
    connectors = json.loads(...)
```

### Bug 4b: TD creation never decrypts encrypted connectors

`arrow_dataset_reader_writer.py` calls `read_query(query)` without `is_query_signed=True`, so encrypted connectors (used by external FGs) are never decrypted. The `do_get` path (used by `fv read`) works because it passes the Hopsworks signature, but the `do_action` path (used by `td compute`) doesn't.

**File**: `arrow_dataset_reader_writer.py`, `create_parquet_dataset()` method

```python
# BEFORE (buggy):
result_batches = self.query_engine.read_query(query)

# AFTER (fixed):
has_encrypted_connectors = bool(query.get("connectors_encrypted"))
result_batches = self.query_engine.read_query(query, is_query_signed=has_encrypted_connectors)
```

This is safe because TD creation is already authenticated via client certificates and peer identity validation.

### Applied as ConfigMap overlay

Both fixes are deployed as a ConfigMap mount on the ArrowFlight deployment:

```bash
# ConfigMap: arrowflight-query-engine-patch
# Mounts:
#   query_engine.py → /usr/src/app/src/query_engine.py
#   arrow_dataset_reader_writer.py → /usr/src/app/src/arrow_dataset_reader_writer.py
```

**To remove the patch** (e.g. after a backend upgrade that includes the fix):
```bash
# Remove volume mounts from deployment, then:
kubectl -n hopsworks delete configmap arrowflight-query-engine-patch
```

## Fix 5: Dashboard API — null charts NPE + non-nullable layout columns

**Problem**: Two issues in the Dashboard/Chart API:

### Bug 5a: `DashboardDTO.getCharts()` NPE on null charts
`DashboardController.createDashboard()` and `updateDashboard()` call `.getCharts().stream()` which NPEs if the charts list is null (e.g. when creating a dashboard without specifying charts).

**File**: `DashboardController.java`, line 56
**Workaround**: CLI always sends `"charts": []` (never omits or sends null).
**Proper fix**: Add null guard in `updateDashboard()`:
```java
List<ChartDTO> charts = dashboardDTO.getCharts();
if (charts == null) {
    charts = Collections.emptyList();
}
```

### Bug 5b: `dashboard_chart` layout columns are NOT NULL but no defaults
`DashboardChart` entity has `@Basic(optional = false)` on `width`, `height`, `x`, `y`. When a chart is added to a dashboard without explicit layout values, the `ChartDTO` fields are null → JPA constraint violation → transaction rollback.

**File**: `DashboardChart.java`, lines 51-65
**Workaround**: CLI defaults width=12, height=8, x=0, y=0 when adding charts.
**Proper fix**: Either set defaults in `DashboardController.updateDashboard()`:
```java
dashboardChart.setWidth(chartDto.getWidth() != null ? chartDto.getWidth() : 12);
dashboardChart.setHeight(chartDto.getHeight() != null ? chartDto.getHeight() : 8);
dashboardChart.setX(chartDto.getX() != null ? chartDto.getX() : 0);
dashboardChart.setY(chartDto.getY() != null ? chartDto.getY() : 0);
```
Or make the DB columns nullable / add column defaults in the migration.
