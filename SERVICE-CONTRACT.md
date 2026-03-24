// @metrics-project: metrics
// @metrics-path: SERVICE-CONTRACT.md
# SERVICE-CONTRACT.md — Metrics
# @version: 0.2.0-phase2
# @updated: 2026-03-25

**Port:** 8083 · **Domain:** Observer (read-only)

---

## Code

```
internal/collector/nexus.go    polls GET /metrics + GET /events every 5s
internal/collector/forge.go    polls GET /history?limit=50 every 10s
internal/collector/atlas.go    polls GET /workspace/projects every 30s
internal/snapshot/model.go     Snapshot struct
internal/api/handler/snapshot.go    GET /metrics/snapshot
internal/api/handler/prometheus.go  GET /metrics/prometheus
```

---

## Contract

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/health` | none | Liveness |
| GET | `/metrics/snapshot` | none | `{nexus, events, forge, atlas}` snapshot |
| GET | `/metrics/prometheus` | none | Prometheus text format |

Each sub-field carries `available: bool` — false when upstream is unreachable.

---

## Control

`collectAll()` assembles all four sources under a single `mt-<hex>` trace ID, then atomically replaces the stored snapshot. No partial state is ever served. Lost on restart.

---

## Context

Derived, non-authoritative. Never calls write endpoints.
