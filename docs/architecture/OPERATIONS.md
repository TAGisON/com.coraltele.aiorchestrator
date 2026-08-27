# Operations — deployment, scale, and runbooks

**Status:** LOCKED assumptions  
**Date:** 27 August 2026  
**Parent:** `SOLUTION.md`

---

## 1. Deployment unit

Single Go binary `cmd/aiorchestrator`:

- HTTP control API (port configurable, default 8080)  
- WSS edge for FS (default 8765 or path on same listener)  
- In-process playback/postcall workers (goroutine pool)

Dependencies: PostgreSQL, object store or shared FS for blobs, vault/env for secrets, optional OTel collector.

---

## 2. Topology

```
                    LB (HTTP + WSS sticky)
                           │
         ┌─────────────────┼─────────────────┐
         ▼                 ▼                 ▼
    orch-instance-1   orch-instance-2   orch-instance-N
         │                 │                 │
         └─────────────────┼─────────────────┘
                           ▼
                      PostgreSQL (HA pair)
                           │
                    object store / NFS (blob URIs)
```

| Traffic | Rule |
|---|---|
| Live WSS | **Sticky** to instance that owns session |
| Control API | Any instance; mutations that need live state route to owner via `session.owner_instance` |
| Playback/postcall jobs | Any instance; `FOR UPDATE SKIP LOCKED` lease |

---

## 3. Graceful shutdown

On `SIGTERM`:

1. Stop accepting new sessions and WS connections.  
2. Mark instance draining in health check.  
3. Wait up to `drain_timeout_sec` (default 120) for live sessions to reach Terminal.  
4. Release playback job leases back to queue.  
5. Exit.

Mid-call instance death: call drops (honest). No cross-instance media reconstruction.

---

## 4. Config and profile reload

| Data | Reload |
|---|---|
| Gateway registry | Process restart or SIGHUP reload of config file |
| Profile versions | Immediate read from PG on **new** session create |
| Running sessions | Pinned version; hot fields only via `PATCH` API |
| Secrets | Vault rotation; gateways pick up on next connection |

---

## 5. Limits and fairness

| Limit | Scope |
|---|---|
| `max_concurrent_sessions_per_tenant` | Enforced at session create → `429` |
| `max_concurrent_sessions_global` | Per instance |
| TTS PCM queue cap | Per session (backpressure) |
| Skill timeout | Per skill definition |
| Listen/Think/Speak hop timeout | Profile or global default |

---

## 6. Health

`GET /health`:

- Process up  
- PostgreSQL reachable  
- Optional: gateway probe summary (last success per registered gateway)

LB uses health for routing; sticky sessions stay on instance until drain.

---

## 7. Backup and DR

| Asset | Approach |
|---|---|
| PostgreSQL | Standard Coral DB backup (profiles, audit, KB metadata, jobs) |
| Object store blobs | Tenant-scoped lifecycle policies |
| Live calls | Not durable; DR = failover instance + new calls |

RPO/RTO for config/audit: estate standard. Live media is ephemeral by design.

---

## 8. On-prem vs cloud deploy

Same binary. **Deployment profile** (config, not code) selects:

| Pack | Gateways |
|---|---|
| Cloud contact-agent | Next AI STT/LLM/TTS + TTS-Engine failover |
| On-prem talking IVR | Local/cpu STT/LLM/TTS gateways + ingest KB; no outbound to public cloud if air-gapped |

Profile schema unchanged; router provider lists differ.

---

## 9. Observability runbook

- **Slow turn:** OTel trace → identify hop (VAD vs STT vs LLM vs TTS).  
- **No audio:** Check edge WS, sink flush logs, Speak gateway health.  
- **Wrong KB answer:** Audit row → gateway ids + retrieval hash; not logs.  
- **Stuck session:** Control `GET /sessions/{id}` + force `stop`.
