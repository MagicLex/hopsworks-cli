# hops internal mode — Roadmap

## What is Internal Mode?

When `hops` detects it's running inside a Hopsworks terminal pod, it unlocks a superset of commands that leverage direct access to the cluster internals: kubectl, internal services, HDFS, RonDB, Kafka, Prometheus, logs, and more.

Detection is automatic — if `REST_ENDPOINT` and `SECRETS_DIR` are set and kubectl can reach the cluster, internal mode is on.

```bash
hops status    # Shows: "Internal mode: active" + accessible services
```

## Available Infrastructure (from inside the pod)

### Direct Service Access

| Service | Endpoint | Protocol |
|---------|----------|----------|
| Hopsworks API | `hopsworks.glassfish.service.consul:8182` | HTTPS (JWT) |
| Hopsworks HTTP | `hopsworks-http:28080` | HTTP |
| RonDB MySQL | `mysqld:3306` | MySQL protocol |
| RonDB REST | `rdrs:4406` | HTTP REST |
| Arrow Flight | `arrowflight-server:5005` | gRPC |
| HDFS NameNode | `namenode-cluster-ip:8020` | RPC |
| Hive Metastore | `metastore:9083` | Thrift |
| OpenSearch | `opensearch:9200` | HTTP REST |
| Prometheus | `hopsworks-prometheus-server:80` | HTTP REST |
| Grafana | `hopsworks-grafana:80` | HTTP |
| MinIO (S3) | `minio:9000` | S3 API |
| Kafka | via Strimzi CRDs | Kafka protocol |
| Airflow | `airflow-webserver:12358` | HTTP |
| Spark History | `sparkhistoryserver:80` | HTTP |
| Docker Registry | `docker-registry:30443` | HTTPS |

### kubectl Access

With RBAC rolebindings (`edit` on `hopsworks` + project namespace), the CLI can:
- List/describe/restart any deployment or statefulset
- View logs from any pod
- Scale replicas
- Read configmaps and secrets
- Manage Kafka topics via Strimzi CRDs
- Inspect network policies

---

## Roadmap — Phased

### Phase 1: Cluster Navigation (`hops cluster`)

```bash
hops status                        # Internal mode detection, service health summary
hops cluster pods [namespace]      # List pods (default: hopsworks + project ns)
hops cluster services              # List reachable internal services
hops cluster logs <pod> [--tail N] # Stream/tail pod logs
hops cluster restart <deployment>  # Restart a deployment (rollout restart)
hops cluster describe <resource>   # Describe any k8s resource
```

**Why:** The most basic need — see what's running, read logs, restart things. Every operator needs this.

### Phase 2: Infrastructure Inspection (`hops infra`)

```bash
# HDFS
hops hdfs status                   # NameNode status, capacity, live datanodes
hops hdfs ls <path>                # Browse HDFS (alternative to /hopsfs mount)
hops hdfs du <path>                # Disk usage

# RonDB / Online Feature Store
hops rondb status                  # Cluster health, data nodes, free memory
hops rondb query "SELECT ..."      # Direct SQL against RonDB (via mysqld:3306)

# Kafka
hops kafka topics                  # List Kafka topics (from Strimzi CRDs)
hops kafka describe <topic>        # Topic config, partitions, consumer groups
hops kafka lag <consumer-group>    # Consumer lag

# OpenSearch
hops search indices                # List OpenSearch indices
hops search query <index> <query>  # Quick search
```

**Why:** When debugging feature pipelines, you need to check if data landed in Kafka, if RonDB has the online features, if HDFS has the offline data. This closes the loop.

### Phase 3: Monitoring & Metrics (`hops metrics`)

```bash
hops metrics cpu [pod|deployment]   # CPU usage from Prometheus
hops metrics memory [pod|deploy]    # Memory usage
hops metrics fg <name>              # Feature group ingestion metrics
hops metrics query <promql>         # Raw PromQL query
hops metrics alerts                 # Active Alertmanager alerts
```

**Why:** Understanding why a job is slow, why a feature group isn't updating, or why a model serving endpoint is degraded — without leaving the terminal.

### Phase 4: Job & Pipeline Operations (`hops run`)

```bash
# Spark
hops spark status                  # Running Spark apps
hops spark logs <app-id>           # Spark driver/executor logs
hops spark history [--app <id>]    # Spark History Server data
hops spark submit <jar|py> [args]  # Submit Spark job

# Airflow
hops airflow dags                  # List DAGs
hops airflow trigger <dag>         # Trigger DAG run
hops airflow status <dag>          # DAG run status

# Feature materialization
hops fg materialize <name>         # Trigger offline materialization job
hops fg materialize-online <name>  # Trigger online materialization
```

**Why:** The full feature engineering loop — write features, materialize them, check jobs, debug failures — without switching to the Hopsworks UI.

### Phase 5: Model Serving (`hops serve`)

```bash
hops model list                    # List models in registry
hops model info <name>             # Model details, versions, artifacts
hops deployment list               # List model deployments (KServe)
hops deployment status <name>      # Deployment health, replicas, endpoint
hops deployment logs <name>        # Inference server logs
hops deployment predict <name> <json>  # Send prediction request
hops deployment create <model> [--replicas N] [--resources ...]
```

**Why:** Completing the ML lifecycle — from features to model to deployment to prediction.

### Phase 6: Advanced Internal Operations

```bash
# Docker registry
hops registry images               # List images in internal registry
hops registry tags <image>         # List tags for an image

# Certificates
hops certs status                  # TLS cert expiry status
hops certs renew                   # Trigger cert renewal

# Consul
hops consul services               # List registered services
hops consul health <service>       # Service health checks

# Config management
hops config dump                   # Full resolved config (host, project, fs, services)
hops config services               # Auto-discovered service endpoints
```

---

## Internal Mode Architecture

```
hops (internal mode)
│
├── Direct REST (existing)
│   └── Hopsworks API → feature groups, views, jobs, datasets
│
├── kubectl (new)
│   ├── Pod lifecycle (list, logs, restart, describe)
│   ├── Strimzi CRDs (Kafka topics)
│   └── KServe CRDs (model deployments)
│
├── Direct service clients (new)
│   ├── RonDB → mysqld:3306 (SQL queries)
│   ├── OpenSearch → opensearch:9200 (log search)
│   ├── Prometheus → prometheus-server:80 (metrics)
│   ├── HDFS → namenode:8020 (file ops)
│   └── Arrow Flight → arrowflight-server:5005 (feature vectors)
│
└── CLI helpers
    ├── Auto-detection (env vars, service discovery)
    ├── Service health checks
    └── Context-aware defaults
```

## Implementation Notes

### Detection Logic

```go
func IsInternalMode() bool {
    // Check 1: env vars
    if os.Getenv("REST_ENDPOINT") == "" || os.Getenv("SECRETS_DIR") == "" {
        return false
    }
    // Check 2: kubectl access
    _, err := exec.Command("kubectl", "auth", "can-i", "get", "pods").Output()
    return err == nil
}
```

### kubectl Integration

Two options:
1. **Shell out** to `kubectl` binary — simple, works now, handles auth automatically via service account
2. **client-go** — Go native K8s client — more powerful, no binary dependency, but heavier

Recommendation: Start with shelling out to `kubectl` (Phase 1-2), migrate to `client-go` when we need CRD handling (Phase 3+).

### Service Discovery

Services can be discovered via:
- `kubectl get services -n hopsworks` (reliable)
- Consul DNS (`*.service.consul`) (already configured)
- Hardcoded well-known endpoints as fallback

### Security

- Internal mode commands should be clearly marked (e.g., `[internal]` in help text)
- Destructive operations (restart, scale) require `--confirm` flag
- Log access respects namespace RBAC
- No secrets are ever printed to stdout (masked by default, `--show-secrets` to override)

---

## Priority Order

| Phase | Commands | Effort | Impact |
|-------|----------|--------|--------|
| 1 | `cluster pods/logs/restart` | Small | High — every debugging session starts here |
| 2 | `hdfs/rondb/kafka/search` | Medium | High — closes the data pipeline debugging loop |
| 3 | `metrics` | Small | Medium — quick health checks without Grafana |
| 4 | `spark/airflow/materialize` | Medium | High — full feature engineering from CLI |
| 5 | `model/deployment` | Medium | High — completes the ML lifecycle |
| 6 | `registry/certs/consul` | Small | Low — nice to have for ops |

## The Self-Improving Loop

With `hops update`, the CLI can:
1. Query the Hopsworks REST API to discover new endpoints
2. Query Prometheus for available metrics
3. Query kubectl for available CRDs (Strimzi, KServe, Spark)
4. Compare against implemented commands
5. Generate a gap report or auto-scaffold new commands

This is the rightful loop: the CLI gets better as the platform evolves.
