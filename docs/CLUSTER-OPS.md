# Cluster Operations

## Access
- SSH: `ssh lex@dev0.devnet.hops.works`
- Kubeconfig: `KUBECONFIG=~/terraform/kubeconfig-lexterm`
- Namespace: `hopsworks`

## Common Commands
```bash
# All from dev0
export KUBECONFIG=~/terraform/kubeconfig-lexterm

# Pods
kubectl get pods -n hopsworks | grep <component>

# Logs
kubectl logs -n hopsworks <pod> --tail=50

# Payara admin password
kubectl get secret -n hopsworks hopsworks-users-secrets -o jsonpath='{.data.admin_password}' | base64 -d

# MySQL
MYSQL_PWD=$(kubectl get secret mysql-users-secrets -n hopsworks -o jsonpath='{.data.root}' | base64 -d)
kubectl exec -n hopsworks mysqlds-0 -- mysql -u root --password="$MYSQL_PWD" -D hopsworks -e "SHOW TABLES;"
```

## Spark / RSS (Remote Shuffle Service)
The RSS is Apache Uniffle-based. Components:
- `rss-controller` — manages RSS CRD
- `rss-webhook` — pod injection
- `rss-coordinator-rss-hops-{0,1}` — shuffle coordinators (2 instances)
- `rss-shuffle-server-rss-hops-{0..N}` — shuffle data servers

### If "Cannot find any Spark shuffle coordinator pods"
The CRD `RemoteShuffleService` (uniffle.apache.org/v1alpha1) may be missing or the resource stuck in Terminating.

Fix:
```bash
# 1. Check CRD exists
kubectl get crd | grep shuffle

# 2. If missing, apply from helm chart
kubectl apply -f charts/spark/crds/uniffle.apache.org_remoteshuffleservices.yaml --server-side

# 3. Check RSS resource
kubectl get remoteshuffleservice -n hopsworks

# 4. If missing/stuck, recreate from ConfigMap
kubectl get configmap rss-crd -n hopsworks -o jsonpath='{.data.rss\.yaml}' > /tmp/rss.yaml
# Remove unsupported fields: imagePullPolicy, envs
kubectl apply -f /tmp/rss.yaml

# 5. Restart controller to pick up changes
kubectl rollout restart deployment/rss-controller -n hopsworks

# 6. Verify
kubectl get pods -n hopsworks | grep rss
# Should see: rss-coordinator-rss-hops-{0,1} + rss-shuffle-server-rss-hops-0
```

## Missing DDL Migrations
If the deployed EAR is ahead of the DB schema, you get errors like `Unknown column 'X' in 'field list'`.

Fix: apply manually via kubectl exec into mysqlds-0.

Known missing migrations on this cluster:
```bash
# dev_mode column for terminal sessions (added in dev EAR, DDL not shipped)
ALTER TABLE terminal_session ADD COLUMN dev_mode TINYINT(1) DEFAULT 0;
```

## Ingress
- Istio ingress gateway: `172.20.153.241`
- Ports: 80 (HTTP), 443 (HTTPS), 4848 (Payara admin)
- Host routing: requests must have `Host: hopsworks.ai.local`
