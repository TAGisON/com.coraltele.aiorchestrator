# L3 — S.3 Supervisor light aggregates

| Field | Value |
|---|---|
| **id** | `S.3` |
| **title** | Supervisor: light session/analytics aggregates |
| **status** | **Closed** (`eea05cf`) |
| **parent_plan** | [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) Wave S; OD-13-8 |
| **depends_on** | S.2 Closed (`c158232`); existing analytics_event + sessions |

## architecture_refs

- [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) — OD-13-8 light aggregates only
- [09_EVIDENCE_AND_RECORDING.md](../09_EVIDENCE_AND_RECORDING.md) — analytics not transcript SoT
- Per-session `GET /v1/sessions/{id}/analytics`
- Locked metrics: `handoff`, `contained`, `session_completed`, …

## goal

Give supervisors a **light** rollup over recent sessions (counts by state/clock, disposition finals, sum of key analytics metrics) — not an executive/QM dashboard.

## in_scope

- `GET /v1/analytics/summary?limit=N` (default 100, max 500):
  - `sessions_total`, `by_state`, `by_clock`, `with_recording`
  - `disposition_final` counts (when disposition present)
  - `metrics` sums for allowlisted metric names observed in window (`handoff`, `contained`, `session_completed`, `session_failed`, `session_started` as present)
  - `window` / `limit` echo
- Compute from existing `ListSessions` + disposition + per-session analytics (no new DDL)
- Supervisor UI: Summary card + Refresh; per-session analytics list on detail
- `OrchAPI.analyticsSummary`, `getAnalytics`
- Tests + this phase file + README

## out_scope

- Supervisor soak (**S.4**)
- Time-range BI / charts / QM scoring
- New analytics tables or materialized views
- Cross-tenant product dashboards ([01] Later)

## forbidden

- Claiming WFO / executive dashboard
- New DDL
- Write paths from Supervisor

## exit_criteria

- [x] `GET /v1/analytics/summary` returns counts for recent sessions
- [x] Supervisor shows summary card
- [x] Session detail shows per-session analytics events (or empty)
- [x] `go test ./web/... ./internal/control/...` green

## edge_cases

1. Empty lab → zeros / empty maps, HTTP 200.
2. Disposition missing → skip that session in disposition counts.
3. Analytics missing → metric sums may be empty; session counts still work.

## verification

```text
go test ./web/... ./internal/control/... -count=1 -timeout 180s
go build ./cmd/aiorchestrator/
```

## rollback

Remove summary handler + Supervisor summary/analytics panels.

## handoff

Next: **S.4** Supervisor soak checklist.
