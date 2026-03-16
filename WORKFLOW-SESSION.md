# WORKFLOW-SESSION.md
# Session: MT-phase1-metrics-observer
# Date: 2026-03-17

## What changed — Metrics Phase 1 (ADR-011)

New observer service. Polls Nexus, Atlas, Forge and exposes
GET /metrics/snapshot as a single platform health endpoint.

## New project: ~/workspace/projects/apps/metrics

Files:
- cmd/metrics/main.go
- internal/config/env.go
- internal/snapshot/model.go
- internal/collector/nexus.go
- internal/collector/forge.go
- internal/collector/atlas.go
- internal/api/handler/snapshot.go
- internal/api/server.go
- go.mod
- nexus.yaml

## Setup and run

cd ~/workspace/projects/apps/metrics && \
go mod tidy && \
go build ./... && \
METRICS_SERVICE_TOKEN=7d5fcbe4-44b9-4a8f-8b79-f80925c1330e metrics &

## Verify

curl -s http://127.0.0.1:8083/health
curl -s http://127.0.0.1:8083/metrics/snapshot | jq '{nexus:.data.nexus.available, atlas:.data.atlas.available, forge:.data.forge.available}'
curl -s http://127.0.0.1:8083/metrics/snapshot | jq '.data.nexus'
curl -s http://127.0.0.1:8083/metrics/snapshot | jq '.data.atlas'

## Commit

git init && git add . && \
git commit -m "feat: metrics observer phase 1 (ADR-011)" && \
git tag v0.1.0-phase1 && \
git remote add origin git@github.com:Harshmaury/Metrics.git && \
git push -u origin main --tags
