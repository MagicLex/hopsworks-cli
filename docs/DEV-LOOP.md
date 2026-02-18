# Dev Loop

Two contexts: **local** (your laptop) and **internal** (Hopsworks terminal pod).

## 1. Code (local)
```bash
# CLI changes
vim cmd/fg.go
go build -o hops .
./hops fg list                    # quick local test via tunnel

# Backend changes (hopsworks-ee)
export JAVA_HOME=/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home
cd /path/to/hopsworks-ee
mvn clean install -P kube-cluster,remote-user-auth,kube,cloud -DskipTests
```

## 2. Deploy backend (if backend changed)
```bash
# Option A: Payara admin UI (reliable)
kubectl port-forward -n hopsworks deployment/hopsworks-admin 4848:4848
# Open https://localhost:4848, upload hopsworks-ear/target/hopsworks-ear.ear

# Option B: script (needs stable SSH)
./scripts/kube_redeploy_ear.sh
```
**DO NOT** `kubectl rollout restart` after hot redeploy — reloads from Docker image.

## 3. Test locally (via SSH tunnel)
```bash
# Open tunnel (one-time)
ssh -f -N -L 8443:172.20.153.241:443 lex@dev0.devnet.hops.works

# Requires /etc/hosts: 127.0.0.1 hopsworks.ai.local
# Config: ~/.hops/config → host: https://hopsworks.ai.local:8443

./hops fg list
./hops fg stats customer_transactions
```
Local testing covers: all REST operations (list, info, create, delete, stats).
Local testing CANNOT do: `fg insert` (needs Python SDK), anything needing Kafka.

## 4. Publish CLI
```bash
# Build both platforms
GOOS=linux GOARCH=amd64 go build -o hops-linux-amd64 .
GOOS=darwin GOARCH=arm64 go build -o hops-darwin-arm64 .

# Commit + push
git add . && git commit -m "feat: ..." && git push origin main

# Release (or update existing)
gh release create vX.Y.Z ./hops-linux-amd64 ./hops-darwin-arm64 --title "vX.Y.Z - ..."
# or update:
gh release upload vX.Y.Z ./hops-linux-amd64 ./hops-darwin-arm64 --clobber
```

## 5. Test from Hopsworks terminal (internal mode)
Open terminal from Hopsworks UI, then:
```bash
# First time
curl -L https://github.com/MagicLex/hopsworks-cli/releases/download/vX.Y.Z/hops-linux-amd64 -o ~/hops
chmod +x ~/hops

# After a release update
~/hops update

# Internal mode is auto-detected (REST_ENDPOINT + SECRETS_DIR set)
# No config needed — JWT + project auto-discovered
~/hops fg list
~/hops fg insert customer_transactions --generate 50   # needs Python SDK (available here)
~/hops fg stats customer_transactions --compute         # triggers Spark job
~/hops fg stats customer_transactions                   # view results
```
Internal testing covers: everything, including `fg insert` (Python SDK available), Spark jobs, Kafka writes.

**Terminal JWT expires after ~8h** — close and reopen terminal session to refresh.

## What to test where

| Operation | Local (tunnel) | Internal (terminal) |
|-----------|:-:|:-:|
| fg list/info/features/preview | yes | yes |
| fg create/delete | yes | yes |
| fg stats (read) | yes | yes |
| fg stats --compute | yes | yes |
| fg insert | no | yes |
| fv/td operations | yes | yes |
| connector list | yes | yes |

## Typical flow
1. Code + build locally
2. Quick test via tunnel (`fg list`, `fg stats`, etc.)
3. If it works, commit + push + release
4. Pull in terminal pod, test insert/compute operations
5. If backend change needed: build EAR, redeploy, repeat
