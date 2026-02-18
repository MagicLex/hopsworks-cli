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
# (exact conversion code exists in ~/hopsworks-cli/docs/SDK-FIXES.md)
```

Then set env var on the terminal container:
```java
envVars.add(new V1EnvVar().name("PEMS_DIR").value("/srv/hops/pems"));
```

**Alternative**: Mount PEM certs directly as a separate Secret/ConfigMap alongside the JKS ones. The CA, client cert, and client key are already known at pod creation time.

## Fix 2: HDFS write permissions for project users

**Problem**: After fixing TLS, writes to `/apps/hive/warehouse/<project>_featurestore.db/` fail with `Failed to create file`. The HDFS user `lexterm__meb10000` likely doesn't have write ACLs on the warehouse directory.

**Impact**: Delta table initialization (first insert) fails. This blocks all offline feature group writes from terminal pods.

**Where to investigate**:
- Check HDFS ACLs: `hdfs dfs -getfacl /apps/hive/warehouse/lexterm_featurestore.db/`
- The Hive warehouse dirs are typically owned by `hive` or `glassfish` — project users may need explicit ACLs
- Compare with what Spark jobs get (they run as `lexterm__meb10000` too but via YARN which may have different permissions)

**Possible fix in KubeTerminalManager**: Ensure the terminal pod's HDFS user has the same write permissions as Spark executors for the project's warehouse directory.

## Fix 3: Set LIBHDFS_DEFAULT_USER env var

**Problem**: `libhdfs` needs `LIBHDFS_DEFAULT_USER` to identify itself to the NameNode. Without it, the connection may use wrong identity.

**Where to fix**: `KubeTerminalManager.java` — add env var:
```java
envVars.add(new V1EnvVar().name("LIBHDFS_DEFAULT_USER").value(projectUser));
// projectUser = "lexterm__meb10000" format
```

The `HADOOP_USER_NAME` is already set but libhdfs reads `LIBHDFS_DEFAULT_USER` specifically.
