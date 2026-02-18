# SDK Fixes — Terminal Pod Insert Pipeline

Status: **working** — insert → preview → stats pipeline tested end-to-end
Date: 2026-02-18

## Problem Statement

`hops fg insert` calls the Python SDK (`hsfs`) which needs to write to both:
1. **Online store** (RonDB via Kafka)
2. **Offline store** (Delta Lake on HopsFS/HDFS)

From a terminal pod, the offline write path fails because:
- The Python SDK uses `hops-deltalake` (Rust wheel) which links `libhdfs` (HopsFS C library)
- `libhdfs` connects to the NameNode over **mTLS** (mutual TLS)
- The terminal pod has certs in JKS format at `/srv/hops/certs/` but `libhdfs` needs **PEM files**

## Architecture (the chain)

```
hops fg insert (Go CLI)
  → generates Python script
    → hsfs Python SDK (feature_group.insert)
      → hops-deltalake (Python wheel, Rust binary inside)
        → hopsfs-native-object-store (Rust, commit 9c57f45)
          → libhdfs-006a46f7.so (HopsFS C library, bundled in wheel)
            → mTLS to NameNode (hdfs://namenode.hopsworks.svc.cluster.local:8020)
```

Source repos:
- `hops-deltalake` wheel built from: https://github.com/logicalclocks/delta-rs
- Object store layer: https://github.com/logicalclocks/hopsfs-native-object-store (branch 1.1.0, commit 9c57f45)
- `libhdfs` bundled at: `deltalake.libs/libhdfs-006a46f7.so`

## What We Found

### Bug 1: `commit_details` IndexError on first insert
- **File**: `hsfs/feature_group.py` — `save()` line ~3254 and `insert()` line ~3457
- **Code**: `commit_id = list(self.commit_details(limit=1))[0]`
- **Problem**: On first insert, no commits exist yet → IndexError. The `except IndexError: pass` in the CLI's Python script silently swallows this and reports success when nothing was written.
- **Impact**: CRITICAL — insert appears to succeed but no data is written
- **Fix**: Guard with `commits = list(self.commit_details(limit=1)); if commits: ...`
- **Status**: Fixed in fork `MagicLex/hopsworks-api` branch `fix/cli-terminal`
- **Upstream**: Bug exists in latest `main` of `logicalclocks/hopsworks-api`

### Bug 2: HUDI format on 4.8 backend
- **Problem**: FGs created before migration were in HUDI format. Backend 4.8 uses Delta.
- **Impact**: All HUDI operations fail (Python Engine can't write HUDI without Spark)
- **Fix**: Delete and recreate FG (it defaults to DELTA on 4.8)
- **Status**: Done

### Bug 3: HDFS mTLS certs not configured for internal clients
- **File**: `hsfs/core/delta_engine.py` — `_setup_delta_rs()`
- **Problem**: Only sets `PEMS_DIR` and `LIBHDFS_DEFAULT_USER` for external clients. Internal clients (terminal pods) are skipped.
- **Fix**: Add `else` branch that reads `MATERIAL_DIRECTORY` and `HADOOP_USER_NAME` env vars
- **Status**: Fixed in fork `MagicLex/hopsworks-api` branch `fix/cli-terminal`
- **Upstream**: Bug exists in latest `main` of `logicalclocks/hopsworks-api`

### Issue 4: JKS → PEM conversion needed
- **Problem**: Terminal pod certs are JKS at `/srv/hops/certs/`. `libhdfs` expects PEM files in `PEMS_DIR`.
- **Files needed** (names hardcoded in libhdfs):
  - `client_key.pem` — private key
  - `client_cert.pem` — cert chain (client + intermediate CA)
  - `ca_chain.pem` — root CA
- **Source JKS files**:
  - `lexterm__meb10000__kstore.jks` → contains private key + cert chain
  - `lexterm__meb10000__tstore.jks` → contains root CA
  - `lexterm__meb10000__cert.key` → password for both JKS files
- **Conversion**: Done via `pyjks` + `cryptography` Python libs. PEM files extracted to `~/.hopsfs_pems/`
- **Status**: Working — connection goes through after setting PEMS_DIR

### Issue 5: HDFS write permissions — NOT A BLOCKER
- **Original error**: `Failed to create file` (during early cert debugging)
- **Resolution**: For online-enabled FGs, the terminal pod writes to Kafka (online store), then a Spark materialization job writes to Delta on HDFS with proper permissions. The terminal pod never writes directly to HDFS in this flow.
- **Note**: Direct HDFS writes (offline-only FGs without online store) may still hit permission issues. Not tested since all current FGs are online-enabled.

## Environment Variables Required

```bash
# Already set in terminal pod:
HADOOP_CONF_DIR=/srv/hops/hadoop/etc/hadoop
MATERIAL_DIRECTORY=/srv/hops/certs
HADOOP_USER_NAME=lexterm__meb10000

# Must be set for hops-deltalake:
PEMS_DIR=/home/terminal/.hopsfs_pems        # PEM certs extracted from JKS
LIBHDFS_DEFAULT_USER=lexterm__meb10000       # HDFS user identity

# JDK (for libhdfs if it uses JNI — our libhdfs is C-native, not JNI):
# JAVA_HOME=/home/terminal/jdk-17.0.18+8    # NOT needed for this libhdfs
```

## Fork Status

### MagicLex/hopsworks-api — branch `fix/cli-terminal`
Based on `upstream/main` (logicalclocks/hopsworks-api).

Changes:
1. `python/hsfs/feature_group.py` — fix commit_details IndexError (2 locations)
2. `python/hsfs/core/delta_engine.py` — add internal client PEMS_DIR/LIBHDFS_DEFAULT_USER setup

## Applied Patches (user site-packages overlay)

The system `hsfs` at `/usr/local/lib/python3.11/dist-packages/hsfs/` is root-owned. We applied patches via Python's **user site-packages** (`~/.local/lib/python3.11/site-packages/hsfs/`), which takes import priority over system packages.

**What was done:**
1. Copied system `hsfs` package to `~/.local/lib/python3.11/site-packages/hsfs/`
2. `feature_group.py` — applied commit_details guard (Fix 1) at 2 locations (lines ~3181, ~3379)
3. `core/delta_engine.py` — copied patched file from fork (installed version == upstream/main, no conflicts)

**To recreate from scratch:**
```bash
# Copy system package
cp -r /usr/local/lib/python3.11/dist-packages/hsfs/ ~/.local/lib/python3.11/site-packages/hsfs/

# Option A: copy delta_engine from fork (if available)
cp ~/hopsworks-api/python/hsfs/core/delta_engine.py ~/.local/lib/python3.11/site-packages/hsfs/core/

# Option B: apply feature_group.py patch manually
# In save() and insert(), replace:
#   commit_id = list(self.commit_details(limit=1))[0]
#   self._statistics_engine.compute_and_save_statistics(...)
# With:
#   commits = list(self.commit_details(limit=1))
#   if commits:
#       self._statistics_engine.compute_and_save_statistics(...)
```

**Verify:**
```bash
python3 -c "import hsfs; print(hsfs.__file__)"
# Should print: /home/terminal/.local/lib/python3.11/site-packages/hsfs/__init__.py
```

**CLI changes (`cmd/fg_insert.go`):**
- Sets `PEMS_DIR` and `LIBHDFS_DEFAULT_USER` on the Python subprocess env
- Removed `except IndexError: pass` hack (SDK now handles first insert correctly)

**This is a temporary workaround.** The proper fix is:
- Upstream PR to `logicalclocks/hopsworks-api` for the 2 SDK fixes
- Terminal image rebuild to pre-extract PEM certs at pod boot (see `docs/hopsworks-ee-fixes.md`)

## Next Steps

1. **PR to upstream** `logicalclocks/hopsworks-api` for the two SDK fixes
2. **Update terminal image** to pre-extract PEM certs at startup (see `docs/hopsworks-ee-fixes.md`)

## Repos Cloned for Investigation

| Repo | Location | Purpose |
|------|----------|---------|
| logicalclocks/hopsworks-api | /tmp/hopsworks-api-latest | Latest SDK source |
| logicalclocks/delta-rs | ~/delta-rs | hops-deltalake build source |
| logicalclocks/hopsfs-native-object-store | ~/hopsfs-native-object-store | HDFS object store (commit 9c57f45) |
| hdfs-native v0.13.2 | ~/hdfs-native-src | Upstream HDFS crate (NOT used — hopsfs uses libhdfs C) |
