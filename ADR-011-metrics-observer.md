# ADR-011 — Metrics Observer Service

**Status:** Accepted
**Date:** 2026-03-17
**Author:** Harsh Maury
**Scope:** New observer service — Metrics
**Port:** 8083
**Depends on:** Nexus Phase 15 (enriched /events), Forge Phase 4 (/history)

---

## Context

The platform now has three stable services with rich HTTP APIs. Nexus emits
structured events with component/outcome/trace_id fields. Forge persists
execution history with duration and status. Atlas exposes a verified project
graph. There is no service that aggregates these signals into a single
operational health view.

Developers currently must query three endpoints manually to understand platform
health. A lightweight read-only observer service can aggregate these signals,
compute derived metrics, and expose a single health dashboard endpoint that
tools, scripts, and future UI surfaces can consume.

ADR-003 permits this because Metrics communicates with existing services via
HTTP/JSON only — it never writes to Nexus, Atlas, or Forge state.

---

## Decision

### 1. Metrics is a read-only observer service

Metrics never issues control commands. It never calls:
- POST /projects/:id/start|stop
- POST /commands
- POST /workflows
- POST /triggers

It only reads from:
- Nexus GET /events?since=<id>
- Nexus GET /metrics
- Forge GET /history
- Atlas GET /workspace/projects

### 2. What Metrics computes

**From Nexus /events (polled every 5s):**
- Events per minute by component (nexus / drop / system)
- Events per minute by outcome (success / failure / deferred)
- Recent crash events (SERVICE_CRASHED in last 10 min)
- Recent file drop activity (FILE_DROPPED, FILE_ROUTED)

**From Nexus /metrics (polled every 10s):**
- Service uptime
- Reconcile cycles total
- Services running / in maintenance
- Services started/stopped/crashed totals

**From Forge /history (polled every 10s):**
- Command executions in last hour (total, success, failure, denied)
- Most active targets
- Average execution duration

**From Atlas /workspace/projects (polled every 30s):**
- Total projects
- Verified vs unverified count
- Language distribution

### 3. Single snapshot endpoint

GET /metrics/snapshot — returns all computed metrics as a JSON object.
This is the stable contract. Fields may be added; removing fields requires
a new ADR.

GET /health — always exempt from auth, returns {"ok":true}.

### 4. Authentication

Metrics uses X-Service-Token on all outbound calls to Nexus, Atlas, Forge.
Metrics does NOT require inbound authentication on GET /metrics/snapshot —
it is a read-only dashboard endpoint, safe to expose locally.

### 5. Polling model

Metrics polls each upstream service independently on its own interval.
It stores the latest snapshot in memory — no SQLite DB needed.
On startup it performs one immediate poll of all sources.

### 6. No SSE, no WebSockets

Metrics uses HTTP polling for data collection. ADR-015 (SSE) is a future
enhancement. Metrics is intentionally simple.

---

## Implementation scope

### New project: ~/workspace/projects/apps/metrics

```
metrics/
├── cmd/metrics/main.go
├── internal/
│   ├── config/env.go
│   ├── collector/
│   │   ├── nexus.go    — polls Nexus /events + /metrics
│   │   ├── forge.go    — polls Forge /history
│   │   └── atlas.go    — polls Atlas /workspace/projects
│   ├── api/
│   │   ├── handler/snapshot.go
│   │   └── server.go
│   └── snapshot/
│       └── model.go    — Snapshot struct (the aggregated view)
├── go.mod
└── nexus.yaml
```

---

## Consequences

**Positive:**
- Single endpoint for platform health — no manual multi-service querying
- Foundation for Navigator (topology) and Guardian (policy) observers
- Zero risk to control plane — strictly read-only

**Negative:**
- New binary to manage in the platform startup sequence
- Polling adds minor load to Nexus/Atlas/Forge APIs (negligible at local scale)

---

## Compliance

| ADR | Status |
|-----|--------|
| ADR-001 | ✅ Never maintains its own project list |
| ADR-003 | ✅ HTTP/JSON only — no new protocols |
| ADR-005 | ✅ Never calls start/stop on Nexus |
| ADR-008 | ✅ X-Service-Token on all outbound calls |

---

## Next ADR

ADR-012 — Navigator observer (port 8084).
Depends on Metrics being tagged and stable.
