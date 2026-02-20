# hopsworks-api — Fixes Needed

Repo upstream: https://github.com/logicalclocks/hopsworks-api
Fork: https://github.com/MagicLex/hopsworks-api
Branch: `fix/cli-terminal` (based on upstream `main`)

## Fix 1: commit_details IndexError on first insert

**File**: `python/hsfs/feature_group.py`
**Lines**: ~3254 (in `save()`) and ~3457 (in `insert()`)

**Problem**:
```python
commit_id = list(self.commit_details(limit=1))[0]  # IndexError when no commits exist
```
On the first insert into a Delta FG, there are no commits yet. This crashes with `IndexError`. The CLI's generated Python script catches this with `except IndexError: pass` — which means the insert silently reports success while NO data is written.

**Fix applied in fork**:
```python
commits = list(self.commit_details(limit=1))
if commits:
    self._statistics_engine.compute_and_save_statistics(
        metadata_instance=self,
        feature_dataframe=feature_dataframe,
        feature_group_commit_id=commits[0],
    )
```

**Status**: Committed on `fix/cli-terminal`. Needs PR to upstream.

## Fix 2: delta_engine skips PEMS_DIR for internal clients

**File**: `python/hsfs/core/delta_engine.py`
**Method**: `_setup_delta_rs()`

**Problem**: Only sets `PEMS_DIR` and `LIBHDFS_DEFAULT_USER` env vars when `_client._is_external()` is True. Internal clients (terminal pods, Jupyter notebooks) are skipped, so `libhdfs` can't find the TLS certs.

**Fix applied in fork** — added `else` branch:
```python
else:
    material_dir = os.environ.get("MATERIAL_DIRECTORY", "")
    hadoop_user = os.environ.get("HADOOP_USER_NAME", "")
    if material_dir and "PEMS_DIR" not in os.environ:
        os.environ["PEMS_DIR"] = material_dir
    if hadoop_user and "LIBHDFS_DEFAULT_USER" not in os.environ:
        os.environ["LIBHDFS_DEFAULT_USER"] = hadoop_user
```

**Note**: This assumes PEM files exist in `MATERIAL_DIRECTORY`. If only JKS files are there (as is the case today), this alone won't fix it — the hopsworks-ee fix (PEM extraction at pod boot) is also needed.

**Status**: Committed on `fix/cli-terminal`. Needs PR to upstream.

## Installation status

System `hsfs` is root-owned. Fork's `main` depends on newer `hopsworks_common` (SinkJobConfiguration) — can't `pip install -e`.

**Applied via user site-packages overlay** (see `docs/fixes/sdk-fixes.md` for full details):
- Copied system `hsfs` to `~/.local/lib/python3.11/site-packages/hsfs/`
- Applied Fix 1 (feature_group.py) as a patch to the installed version copy
- Applied Fix 2 (delta_engine.py) by copying the fork's file (installed == upstream/main)

**Proper fix (later)**: rebuild terminal image with patched SDK, or upstream PR + new release.
