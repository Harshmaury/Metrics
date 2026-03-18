# SERVICE-CONTRACT.md — Metrics

**Service:** metrics
**Domain:** Observer
**Port:** 8083
**ADRs:** ADR-011 (metrics observer), ADR-020 (governance)
**Version:** 0.1.0-phase1
**Updated:** 2026-03-18

---

## Role

Platform health snapshot observer. Polls Nexus, Atlas, and Forge at
staggered intervals and exposes a single aggregated health snapshot.
The simplest observer — build this first when adding new observers.

---

## Inputs

- `Nexus GET /metrics` — runtime counters (every 5s)
- `Nexus GET /events?since=<id>` — event activity (every 5s)
- `Forge GET /history?limit=50` — execution stats (every 10s)
- `Atlas GET /workspace/projects` — project counts (every 30s)

---

## Outputs

- `GET /health`
- `GET /metrics/snapshot` — full platform health snapshot (no auth required)

Snapshot shape: `{nexus: NexusMetrics, events: EventMetrics, forge: ForgeMetrics, atlas: AtlasMetrics}`

---

## Dependencies

| Service | Used for              | Auth required   |
|---------|-----------------------|-----------------|
| Nexus   | Runtime metrics + events | X-Service-Token |
| Forge   | Execution history stats | X-Service-Token |
| Atlas   | Workspace project counts | X-Service-Token |

---

## Guarantees

- `collectAll()` assembles all four metric sources under a single `mt-<hex>`
  trace ID and atomically replaces the stored snapshot — no partial state served.
- Graceful degradation — each collector returns an `Available: false` metric
  on upstream failure. The snapshot is always complete.
- One full collection pass before HTTP server starts (ADR-020 Rule 6).

## Non-Responsibilities

- Metrics never calls start/stop on Nexus.
- Metrics never writes to any platform database.
- Metrics does not own execution history, event log, or workspace state.
  It aggregates derived counts from services that own those.

## Data Authority

Derived, non-authoritative. Snapshot reflects data at collection time.
Lost on restart — no persistence.

## Concurrency Model

- `SnapshotStore` protected by `sync.RWMutex`. `Set()` takes write lock,
  `Get()` takes read lock.
- Single polling goroutine with three staggered tickers owns all collection.
- `collectAll()` assembles the full `Snapshot` struct locally then calls
  `Set()` once — atomic replacement, no partial visibility.
