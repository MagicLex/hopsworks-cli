# Dev Loop

## CLI
```bash
go build -o hops .
./hops <command>
```

## Backend (hopsworks-ee)
```bash
export JAVA_HOME=/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home
mvn clean install -P kube-cluster,remote-user-auth,kube,cloud -DskipTests
```
EAR output: `hopsworks-ear/target/hopsworks-ear.ear`

## Deploy to cluster
Option A (reliable): Port-forward Payara admin UI on 4848, upload EAR
Option B: `./scripts/kube_redeploy_ear.sh` (needs stable SSH)

**DO NOT** `kubectl rollout restart` after hot redeploy — reloads from Docker image, not your EAR.

## SSH tunnel to cluster
```bash
ssh -f -N -L 8443:172.20.153.241:443 lex@dev0.devnet.hops.works
```
Requires `/etc/hosts`: `127.0.0.1 hopsworks.ai.local`

Config: `~/.hops/config` → `host: https://hopsworks.ai.local:8443`

## Test
```bash
./hops fg list
./hops fg stats customer_transactions
```
