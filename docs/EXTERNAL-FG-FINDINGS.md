# External Feature Groups — Findings & Peculiarities

Notes from building end-to-end external FG support (Snowflake TPC-H).
Covers feature view creation, training dataset materialization, and statistics.

## 1. Joins Are Trees, Not Lists

The Hopsworks REST API models feature view joins as **nested query objects**, not a flat array.

For a 3-way join `orders → customers → nations`, the nations join must be nested _inside_ the customers join query:

```json
{
  "leftFeatureGroup": "orders",
  "joins": [{
    "query": {
      "leftFeatureGroup": "customers",
      "joins": [{
        "query": { "leftFeatureGroup": "nations" },
        "leftOn": ["c_nationkey"],
        "rightOn": ["n_nationkey"]
      }]
    },
    "leftOn": ["o_custkey"],
    "rightOn": ["c_custkey"]
  }]
}
```

The CLI was putting all joins flat — fixed in `CreateFeatureView` to auto-nest when `leftOn` belongs to a previously-joined FG.

## 2. PK Constraint on Joins

The right side of every join **must be the primary key** of the right FG. This means:

- `orders → customers` works: `o_custkey = c_custkey` (c_custkey is PK of customers)
- `customers → orders` does NOT work: `c_custkey = o_custkey` (o_custkey is NOT PK of orders)

The base FG choice matters — pick the FG whose FK points to other FGs' PKs.

## 3. FG Type Discriminator

`buildFGRef()` was guessing the Jackson type from `OnlineEnabled`:
- `true` → `streamFeatureGroupDTO`
- `false` → `cachedFeaturegroupDTO`

This misses `onDemandFeaturegroupDTO` for external FGs. Fixed to use the actual `fg.Type` field from the API response. The server happened to accept the wrong type on create, but this could break silently on a backend upgrade.

## 4. Arrow Flight Server — Two Bugs

### 4a. `connectors` variable uninitialized (`query_engine.py`)

`read_query()` only assigns `connectors` inside `if/elif` branches. If neither matches (common for TD creation path), the `for` loop hits `UnboundLocalError`.

**Fix**: `connectors = {}` before the conditional.

### 4b. TD creation never decrypts encrypted connectors (`arrow_dataset_reader_writer.py`)

The `do_get` path (used by `fv read`) passes the Hopsworks signature, which gates connector decryption. The `do_action` path (used by `td compute`) calls `read_query()` without `is_query_signed=True`, so encrypted connectors are never decrypted.

**Fix**: Pass `is_query_signed=True` when `connectors_encrypted` exists. TD creation is already authenticated via client certificates.

Both fixes deployed as ConfigMap overlay on `arrowflight-deployment`. See `hopsworks-ee-fixes.md` Fix 4.

## 5. Spark Stats Job Fails for External FG-backed TDs

`POST .../statistics/compute` triggers a Spark job that runs `hsfs_utils.py:compute_stats()`. For external FG-backed feature views, `fv` is `None`:

```
File "/srv/hops/artifacts/hsfs_utils.py", line 156, in compute_stats
    entity = fv._feature_view_engine._get_training_dataset_metadata(
AttributeError: 'NoneType' object has no attribute '_feature_view_engine'
```

**Not fixed server-side.** Workaround: `hops td stats --compute` reads the materialized parquet via Python, computes stats with pandas, and POSTs them via the REST API.

## 6. Statistics POST Requires `beforeTransformation`

The backend `StatisticsDTO.getBeforeTransformation()` returns `Boolean` (not `boolean`). When the field is missing from the request, it's `null`, and the Java auto-unboxing NPEs:

```
Cannot invoke "java.lang.Boolean.booleanValue()" because the return value of
"StatisticsDTO.getBeforeTransformation()" is null
```

**Fix**: Always include `"beforeTransformation": false` in the stats POST body.

## 7. SDK stdout Pollution

`hopsworks.login()` and `connection.close()` print messages to stdout:
```
Logged in to project, explore it here https://...
Connection closed.
```

When capturing Python output for JSON, these pollute the result. Handled two ways:
- `contextlib.redirect_stdout(os.devnull)` around login
- `extractJSON()` in Go to find the first `{`-prefixed line

## 8. Arrow Flight Returns DECIMAL Columns as Strings

**Upstream bug.** The Arrow Flight server (or Snowflake connector) returns `DECIMAL`/`NUMBER` columns as Python `object` (string) dtype instead of `float64`. Affected columns in TPC-H: `c_acctbal` (DECIMAL(15,2)), `o_totalprice` (DECIMAL(15,2)).

**Impact**: Without mitigation, pandas sees these as strings and skips numeric statistics (min/max/mean/stddev).

**Root cause**: Likely in the Arrow Flight query engine's type mapping for external connectors — Snowflake `NUMBER`/`DECIMAL` types aren't being mapped to Arrow decimal or float types before serialization.

**CLI workaround** (in `buildTDStatsComputeScript`): Before computing stats, object columns are coerced with `pd.to_numeric(errors="coerce")`. If any values convert successfully, the column is promoted to `float64`. This is safe — truly non-numeric columns (like `o_orderstatus`) fail coercion and stay as `object`.

**Proper fix**: Should be in the Arrow Flight server's type conversion layer, mapping Snowflake DECIMAL → Arrow Decimal128 or Float64. This would fix all downstream consumers, not just our stats script.

## 9. External FGs and the `StatisticsConfig`

When creating external FGs, stats must be disabled: `StatisticsConfig(enabled=False)`. If enabled, the system tries to run a Spark job to compute stats on the external data source, which fails because Spark can't reach the external DB.

Training dataset stats are different — the data is already materialized as parquet on HopsFS, so stats can be computed from the local copy.
