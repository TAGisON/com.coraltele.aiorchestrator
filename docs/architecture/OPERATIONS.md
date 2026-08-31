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

---

## 10. Sarvam lab gateways (Phase F)

Lab realtime Talk can use Sarvam Listen / Think / Speak with fakes as ordered failover. Secrets never go in git.

### Secrets

1. Preferred: `PUT /v1/tenant/credentials/sarvam` with `{ "api_key": "…" }` (Postgres; masked on GET / config).  
2. Lab fallback only: copy `.agent/secrets.example.json` → `.agent/secrets.local.json`, or export `SARVAM_API_KEY`.  
3. Optional URL overrides: `stt_rest_url`, `stt_ws_url`, `chat_url`, `tts_url` (defaults match public Sarvam hosts in the example file).

`cmd/aiorchestrator` always registers `sarvam-stt`, `sarvam-llm`, and `sarvam-tts`; calls fail until a key is present in DB (or lab env). Boot properties in `conf/aiorchestrator.properties` hold listen address (**`:8011`**), database DSN, and engine seeds — not vendor keys.

### Profile provider lists (example)

```yaml
routers:
  listen:
    providers: [sarvam-stt, fake-listen]
  think:
    providers: [sarvam-llm, fake-think]
  speak:
    providers: [sarvam-tts, fake-speak]
```

Prefer session canonical PCM at **16 kHz** for lab (Sarvam STT streaming supports 8 kHz / 16 kHz; other rates are resampled in-gateway to 16 kHz).

Defaults: STT `saaras:v3` + `en-IN`; LLM `sarvam-105b-conversations`; TTS Bulbul `bulbul:v3` speaker `shubh`.

### Tests

```text
# Always (no key / no network to Sarvam required)
go test ./... -count=1

# Optional live (operator machine only)
$env:SARVAM_API_KEY = "<from secrets.local.json>"
go test -tags=sarvam_live ./internal/gateway/sarvamstt -count=1 -v
```

Never commit `.agent/secrets.local.json` or print API keys in logs/artifacts.

---

## 11. Product Validation V1 (Control + memory + audit)

Validation V1 proves the A–F product surface **without FreeSWITCH**: Control API, fakes, audit/analytics, postcall disposition. It is a **parallel track** to `coral-phase` product coding.

### How to run Tier A (always)

```powershell
cd C:\Users\user\Documents\GitHub\com.coraltele.aiorchestrator
go test ./internal/validation -count=1
# or:
.\scripts\validation\Run-ValidationV1.ps1
```

Scenarios and assertion table: `.agent/work/validation-v1/scenarios.md` (`V1-A01`…`V1-A08`).

Agent pipeline (quiet dispatch):

```powershell
$env:AGENT_NO_MAIL = "1"
.\tools\agent-runner\agent.ps1 start -Pipeline validation-v1 -From validation-v1
.\tools\agent-runner\agent.ps1 monitor-start
.\tools\agent-runner\agent.ps1 assign-role
```

Roles use existing `coral-validation-*` skills (aliases: `coral-validation-planner` / `runner` / `auditor` / `summarizer`). Artifacts land in `.agent/work/validation-v1/`.

Per-feature deep wave remains `product-validation` (`docs/VALIDATION_PIPELINE.md`).

### Tier B — Sarvam live (key required)

1. Put a rotated key in `.agent/secrets.local.json` as `SARVAM_API_KEY`, or export `$env:SARVAM_API_KEY`.  
2. Prefer Admin `PUT /v1/tenant/credentials/sarvam` for long-lived lab.  
3. Re-run `go test ./internal/validation -count=1` — Tier B subtests run when a key is found; otherwise they **skip**.  
4. Optional package live tests (same as §10):

```powershell
$env:SARVAM_API_KEY = "<from secrets.local.json>"
go test -tags=sarvam_live ./internal/gateway/sarvamstt -count=1 -v
```

Never commit secrets. Tier B soft-skip without a key does **not** fail Validation V1.

### Tier C — later (not V1)

Postgres durable audit under `DATABASE_URL`, FreeSWITCH / `mod_audio_stream` round-trip, multi-tenant auth limits — flip when human provides deploy topology / call-server endpoint (`F-edge-fs-live`).
