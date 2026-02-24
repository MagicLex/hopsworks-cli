# Backend Issues — Discovered via CLI Testing

## ISSUE-1: REST `POST /featuregroups` does not provision Kafka topic or materialization job

**Status:** Open
**Severity:** High — blocks all REST/CLI-based data ingestion
**Repo:** `hopsworks-ee` (Java backend)

### Problem

Creating a Feature Group via `POST /hopsworks-api/api/project/{id}/featurestores/{id}/featuregroups` with `onlineEnabled: true` creates **metadata only**:

- Hive table created
- Feature schema stored
- `onlineEnabled` flag set

But it does NOT:

1. **Provision Kafka topic** — `topicName` remains `null`
2. **Create materialization job** — no Spark job for offline sync
3. **Initialize Hudi commit tracking**

This means any subsequent data insert via Kafka silently drops data. The FG looks correct in the UI and API but is a dead end.

### Reproduction

```bash
# 1. Create FG via REST (or CLI which uses REST)
curl -X POST .../featurestores/67/featuregroups \
  -d '{"name":"test","version":1,"onlineEnabled":true,"type":"cachedFeaturegroupDTO",...}'
# → 200 OK, id: 18

# 2. Check topic
python3 -c "fg = fs.get_feature_group('test',1); print(fg.topic_name)"
# → None

# 3. Insert via SDK
fg.insert(df)  # Writes to Kafka topic None → data lost
```

### Expected behavior

`POST /featuregroups` with `onlineEnabled: true` should:
1. Create the Kafka topic (same as what the SDK's `save_feature_group_metadata` triggers)
2. Create the materialization job
3. Return a FG DTO with a valid `topicName`

### Where to fix

Key files in `hopsworks-ee`:

| File | What to change |
|------|---------------|
| `FeaturegroupController.java` | After creating FG, trigger Kafka topic provisioning if `onlineEnabled` |
| Kafka topic provisioning service | Ensure it's called from the REST create path, not just SDK path |
| Materialization job service | Create the offline materialization job on FG creation |

### Workaround

Use the Python SDK `get_or_create_feature_group()` for creation instead of raw REST. The SDK's first `insert()` call provisions everything. The CLI's `fg insert` command uses this workaround.

---

## ISSUE-2: No Spark shuffle coordinator in terminal pod

**Status:** Known limitation
**Severity:** Medium — blocks offline materialization from terminal

The terminal pod has no Spark shuffle coordinator, so offline materialization jobs fail with:
```
Cannot find any Spark shuffle coordinator pods
```

This means HUDI offline writes (which need a Spark job) cannot run from the terminal. Only online-only inserts (Kafka → RonDB) work.

### Workaround

Use `--online-only` flag for inserts, or trigger materialization jobs from the Hopsworks UI/Jobs page where Spark is available.

---

## ISSUE-3: Brewer serving record required for chart/dashboard UI

**Status:** Workaround applied
**Severity:** Low — UI-only coupling

### Problem

The frontend gates the "Add Dashboard" button behind `brewer_enabled` setting (`Dashboards/index.tsx` lines 61-69). When `brewer_enabled=true` (needed for dashboard UI), the backend's `ChatController.getProjectBrewerServing()` looks for a serving record named `brewer` in the project. If missing, it throws:

```
Serving for LLM Assistant in <project> not found
```

This error appears in the UI even though charts/dashboards have zero backend dependency on Brewer.

### Root cause

Frontend coupling in `hopsworks-front/src/pages/project/view/Dashboards/index.tsx`:
```typescript
const { data: brewerEnabled } = useGetBoolVariableQuery('brewer_enabled');
{brewerEnabled && (<Button>+ Add Dashboard</Button>)}
```

Backend lookup in `ChatController.java` (line 188-196):
```java
Serving serving = servingFacade.findByProjectAndName(project, settings.getString(BREWER_SERVING_NAME));
if (serving == null) throw new BrewerException(BREWER_WORKER_NOT_FOUND, ...);
```

### Workaround

Create a stopped stub deployment named `brewer` via the CLI:
```bash
# 1. Register a dummy model
mkdir /tmp/brewer_stub && echo '{}' > /tmp/brewer_stub/model.json
cat > /tmp/brewer_stub/predict.py << 'EOF'
class Predict:
    def __init__(self): pass
    def predict(self, inputs): return {"predictions": []}
EOF
hops model register brewerstub /tmp/brewer_stub --framework python --description "Stub for brewer serving record"
cp /tmp/brewer_stub/predict.py /hopsfs/Models/brewerstub/1/Files/predict.py

# 2. Create deployment (don't start it)
hops deployment create brewerstub --script predict.py --name brewer

# 3. Clean up
rm -rf /tmp/brewer_stub
```

The serving record satisfies the DB lookup. Dashboard UI is unblocked. Brewer chat won't work (no actual LLM worker) but that's fine.

### Proper fix

Decouple the frontend: remove the `brewerEnabled` gate from the dashboard "Add" button. Charts/dashboards are independent backend features.
