# Analytics, post-call, and disposition — architecture lock

**Status:** LOCKED  
**Date:** 27 August 2026  
**Parents:** product lock, CC vertical, `INTEGRATION.md`

---

## 1. Three signals (unchanged)

| Signal | Store | Retention |
|---|---|---|
| **Audit** | Append-only PostgreSQL | Tenant policy (compliance) |
| **Traces/logs** | OTel + structured logs | Short ops retention |
| **Analytics** | PostgreSQL rollups + optional export | Product/dashboard retention |

Analytics answers ROI and ops questions; audit answers disputes. Overlap is minimal by design.

---

## 2. Analytics events

### Per-turn / per-session counters (locked set)

| Metric | When emitted |
|---|---|
| `session_started` | Running |
| `session_completed` | Terminal success |
| `session_failed` | Terminal failure |
| `turn_completed` | After each Talk turn |
| `no_grounding_hit` | Grounded profile, Knowledge miss |
| `handoff` | warm_transfer skill success |
| `contained` | Session ended without handoff (profile-defined) |
| `barge_in` | Composer barge-in |
| `hop_latency_ms` | Per hop: vad, listen, retrieve, think, speak, skill, translate |
| `hop_cost` | Optional if gateway reports usage |

Stored in `analytics_event` (append-only) and nightly **rollups** per tenant/profile (`analytics_daily`).

### Export

- HTTP webhook or batch export to **Coral telemetry** (existing estate service).  
- Schema: JSON lines with `tenant_id`, `profile_id`, `session_id`, metric name, value, dimensions.  
- Dashboards in Coral GUI consume export — orchestrator is not the BI warehouse.

---

## 3. Post-call pipeline

Triggered on session **Terminal** (live) or playback job **Completed**.

```
Session Terminal
  → enqueue postcall_job (same PG worker pool as playback, or inline if < SLA)
  → optional Listen batch on recording URI (if not already transcribed)
  → Think + disposition template (if profile.analytics / templates.disposition)
  → write audit + analytics_event
  → Skill: push disposition to Coral CRM/CDR
  → emit recording_ref + transcript link correlation
```

| Output | Destination |
|---|---|
| Transcript | Audit + optional Coral CDR field |
| AI summary | Screen-pop already sent on warm transfer; post-call updates CRM |
| Disposition tags | `resolved` \| `unresolved` \| `escalated` + agent override via Coral UI |
| Recording link | `recording_ref` from FS metadata passed at session create |

**Agent override:** Coral agent desktop owns manual disposition; orchestrator stores AI suggestion + final outcome when CRM skill callbacks or webhook confirms.

---

## 4. Captions / text sink delivery

For Listen-only and live captions:

| Transport | Use |
|---|---|
| `GET /sessions/{id}/events` (SSE) | Partial/final text, state changes |
| WebSocket on control port | Same event stream for thick clients |
| Text sink attachment | Batch or stream to customer webhook |

Event shape:

```json
{
  "type": "caption",
  "session_id": "...",
  "partial": false,
  "text": "...",
  "language": "en-IN",
  "ts_ms": 1234567890
}
```

Backpressure: slow clients drop **partial** captions first; finals are buffered briefly then marked dropped in audit if undeliverable.

---

## 5. Containment KPIs (contact-center vertical)

Derived from analytics rollups:

- Containment rate = sessions without `handoff` / completed sessions  
- Transfer rate = `handoff` / completed  
- No-grounding rate = `no_grounding_hit` / turns with grounding on  
- Latency p50/p95 from `hop_latency_ms`

CC vertical dashboards read Coral telemetry export; definitions align with `mod_audio_stream-1/docs/AI_Call_Center_Product_Decisions.md`.
