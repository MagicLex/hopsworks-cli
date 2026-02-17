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
