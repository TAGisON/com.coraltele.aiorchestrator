# Platform-first build — Coral complete before vendors

**Status:** LOCKED  
**Date:** 27 August 2026  
**Parents:** `SOLUTION.md`, `CONTRACTS.md`, `PORTS.md`

**Goal:** Finish Coral’s orchestrator **end-to-end** with frozen ports, routers, fakes, first-party adapters, and control APIs. Add external vendors **one gateway at a time** without changing the composer, think path, or profile model.

---

## 1. Principle

```
Core (session, composer, thinkpath, routers)
        │
        │  calls ONLY port interfaces
        ▼
   Gateway registry
        │
   ┌────┼────┬────────────┐
   ▼    ▼    ▼            ▼
 Fake  First-party    External
       (TTS-Engine,   (Next AI, …)
        ingest,
        coral-*)
```

- Composer never imports a vendor package.  
- Profile stores **gateway ids**, not URLs or SDKs.  
- Adding Next AI STT = new package + registry entry + conformance test. **No kernel rewrite.**

---

## 2. Definition of done — “Coral side complete”

All of the following must pass **without** Next AI / Sarvam / any public cloud STT-LLM-TTS:

| # | Criterion |
|---|---|
| 1 | Go packages under `internal/port` match `PORTS.md`; contract tests green for every port |
| 2 | Gateway registry + routers select by profile id, capability, health |
| 3 | Fake gateways for Listen, Think, Speak, Translate, Knowledge, Skill |
| 4 | Control API OpenAPI published; Coral Java can create session / stop / subscribe events |
| 5 | Profile CRUD + validation rejects unknown gateway ids at publish time |
| 6 | Live path: FS edge → session → composer → fake Listen/Think → Speak (fake or TTS-Engine) → FS inject; audit rows written |
| 7 | Captions: Listen-only → SSE events |
| 8 | Playback job: file feeder → fake Listen → Think + template → text sink |
| 9 | Ingest Knowledge: upload → index → retrieve in Think path |
| 10 | Skill: `coral-transfer` and/or `fake-skill` execute with audit (Coral HTTP may be stubbed in lab) |
| 11 | Post-call job enqueues on Terminal; disposition template + analytics_event |
| 12 | Adding a new gateway requires **no** change to `composer` or `thinkpath` |

Until 1–12 hold, do **not** start external vendor gateways except TTS-Engine (first-party Speak).

---

## 3. Build phases

### Phase A — Freeze contracts (no FS, no vendors)

| Deliverable | Location |
|---|---|
| Port interfaces + types + `GatewayError` + `Capability` | `internal/port` (spec: `PORTS.md`) |
| Routers + registry | `internal/router` |
| Fake gateways (all ports) | `internal/gateway/fake` |
| Contract tests | `internal/port/contract` or `*_test.go` |

**Exit:** composer can be unit-tested against fakes only.

### Phase B — Durable platform

| Deliverable | Notes |
|---|---|
| Postgres: profile, profile_version, session, audit_event, playback_job, postcall_job | Migrations |
| Profile validate against registry | 422 on bad gateway id |
| Control HTTP + OpenAPI | `CONTROL_API.md` → `api/openapi.yaml` when coding |

**Exit:** create session, pin profile, stop, health — no audio yet.

### Phase C — Runtime kernel

| Deliverable | Notes |
|---|---|
| Session actor, bus, clocks | Live + playback |
| Talk composer + local VAD | Barge-in with fake Speak |
| Think path | Rules JSON, playbook stub, Knowledge call, Skill act |

**Exit:** in-process Talk with fakes; no FS.

### Phase D — Coral edges and first-party adapters

| Deliverable | Gateway / edge id |
|---|---|
| `mod_audio_stream` edge | Feeder + Sink |
| File edge | Playback |
| TTS-Engine Speak gateway | `tts-engine` |
| Ingest KB Knowledge gateway | `ingest-default` |
| Coral transfer / CRM skills | `coral-transfer`, `coral-crm` (Coral may stub HTTP) |

**Exit:** live call on lab FS with fake Listen/Think + real Speak (or all fakes).

### Phase E — Observability and post-call

| Deliverable | Notes |
|---|---|
| Audit write path complete | Per turn |
| `analytics_event` + SSE event catalog | `ANALYTICS_AND_POSTCALL.md` |
| Postcall worker | Disposition template |

**Exit:** one completed call leaves audit + analytics + optional disposition.

### Phase F — External vendors (one at a time)

Order (recommended):

1. `nextai-stt` (Listen)  
2. `nextai-llm` (Think)  
3. `nextai-tts` (Speak) — failover beside TTS-Engine  
4. `nextai-mt` (Translate) if interpret profiles  
5. Tenant `http_kb` / `http_crm` as needed  

Each vendor: implement port → register → pass **same** contract tests as fakes → enable in one profile → measure latency.

---

## 4. First-party vs fake vs external

| Kind | Examples | When |
|---|---|---|
| **Fake** | `fake-listen`, `fake-think`, `fake-speak`, … | Always in CI; lab without engines |
| **First-party** | `tts-engine`, `ingest-default`, `coral-transfer`, `coral-crm`, `file` edge, `modaudiostream` edge | Phase D–E |
| **External** | `nextai-*`, Sarvam, customer HTTP | Phase F only |

TTS-Engine is **not** a special Speak path. Same `Speak` port as `nextai-tts`.

---

## 5. Vendor plug-in checklist (Phase F)

Before merging a vendor gateway:

- [ ] Implements exactly one port from `PORTS.md`  
- [ ] Declares `Capability` (streaming/batch, cancel, sample rates)  
- [ ] Maps vendor errors → `GatewayError` codes  
- [ ] Secrets from vault/env only  
- [ ] Registered under a stable id (`nextai-stt`)  
- [ ] Contract tests pass  
- [ ] Profile can select it; failover list works  
- [ ] No import from `internal/runtime/composer` or `thinkpath` into the gateway  
- [ ] OTel span attribute `gateway.id` set; hop latency recorded  

If any item fails, the gateway is incomplete — do not patch the composer to compensate.

---

## 6. Explicit non-goals until Coral side is done

- Next AI duplex / bundled Talk as default  
- Customer Salesforce gateway before `coral-transfer` works  
- Admin GUI (API-first is enough)  
- Graph knowledge, diarization, chat feeder (later SKUs; ports may extend later without breaking existing ones)

---

## 7. Document map for this strategy

| Doc | Role |
|---|---|
| **This file** | Build order + definition of done |
| `PORTS.md` | Frozen Go-shaped port interfaces |
| `CONTROL_API.md` | Coral-facing HTTP/SSE |
| `CONTRACTS.md` | Semantic meaning of ports |
| `PROFILE_SCHEMA.md` | Profile selects gateway ids |
| `EDGE_FS.md` | First-party telephony edge |
| `SOLUTION.md` | Full architecture |

After Phase A–E, update this file’s checklist dates; do not invent a second orchestration path for vendors.
