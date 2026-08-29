# Architecture Solution — com.coraltele.aiorchestrator

**Document type:** Architecture Solution (not a product definition)  
**Artifact:** `com.coraltele.aiorchestrator`  
**Date:** 27 August 2026  
**Status:** LOCKED  

**Product source of truth:** [`docs/product/PRODUCT_DECISIONS.md`](product/PRODUCT_DECISIONS.md)  
**Companion architecture notes:** [`docs/architecture/`](architecture/) (ARCHITECTURE, RUNTIME, CONTRACTS, PORTS, PLATFORM_FIRST, CONTROL_API, SERVICE, INTEGRATION, TECH_CHOICES, BUILD, PROFILE_SCHEMA, RULES_AND_SKILLS, EDGE_FS, ANALYTICS_AND_POSTCALL, OPERATIONS)

This document answers: **how we build and run the locked product** — system shape, service boundaries, media/control planes, gateway routers, data, identity, audit, failure, latency, and Coral estate fit. It does **not** redefine what we sell; it implements it.

If this file and the product lock disagree after review, update one of them so they stay aligned.

---

## 1. Product summary (reference only)

Full product lock: **`docs/product/PRODUCT_DECISIONS.md`**. Vertical (call center): `mod_audio_stream-1/docs/AI_Call_Center_Product_Decisions.md`.

| Item | Summary |
|---|---|
| **What** | Configurable speech-and-agent **runtime**: attach audio/text in → named **profile** → audio/text/action out |
| **Not** | PBX, ACD, CPaaS, meeting app, CRM of record, IdP, model farm |
| **Modes** | Listen, Speak, Think, Talk (independent; LLM not required for Listen/Speak) |
| **Clocks** | Live and/or playback (same modes, different scheduler) |
| **Agent** | Persona, knowledge, skills, rules, memory, analytics; **rules > skills > grounding > LLM** |
| **Modularity** | STT / LLM / TTS / feeders / sinks / knowledge / skills behind **routers + gateways** |
| **Jobs** | Contact agent, meeting pack, grounded copilot, captions/translate, two-way interpret — all **profiles**, not separate products |

Architecture below assumes that lock. Do not invent product rules here.

---

## 2. Problem this architecture solves

Today’s POC path is:

```
FreeSWITCH → mod_audio_stream → Ai_code (Python) → OpenAI Realtime (fused)
```

That couples **telephony give/take**, **orchestration**, and **one vendor’s duplex brain**. It cannot cleanly do Listen-only, Speak-only, our RAG between STT and TTS, swap TTS to TTS-Engine, or attach a customer’s CRM without rewriting the bridge.

**Target path:**

```
FreeSWITCH → mod_audio_stream ──WS──► aiorchestrator (Go)
                                         │ routers
                                         ├► Listen / Think / Speak / Translate gateways (Next AI, TTS-Engine, …)
                                         ├► Knowledge / Skill gateways (ingest, their APIs, Coral CRM)
                                         └► Coral Java (control, transfer, CRM, users)
```

`mod_audio_stream` stays **bytes only**. Intelligence and policy live in **this service**. Engines and customer systems are **gateways**, not the kernel.

---

## 3. Architecture principles (locked)

| Principle | Meaning in this solution |
|---|---|
| **Hexagonal** | Core never imports vendor SDKs. Ports = contracts. Gateways = adapters. |
| **Payment-gateway routing** | Composer says `Speak(text)`. Router picks **active** gateway + failover + capability. Same for Listen, Think, Speak, Translate, Knowledge, Skill. |
| **Modular monolith** | One Go process owns the turn. No microservice hop between VAD and Speak. Scale by **more instances**, not by splitting the turn machine. |
| **In-memory media plane** | PCM and in-flight turn audio stay in **goroutine channels**. No Kafka/Redis/NATS for audio. |
| **Durable control plane** | Profiles, jobs, ingest KB, audit in **PostgreSQL**. Blobs by URI in object store/FS. |
| **Connect, don’t absorb** | Profile/persona/skill *definitions* here. Their KB/CRM stay theirs unless dump or live API. |
| **Coral identity** | No second user DB. `coral_user_id` + `tenant_id` on control and audit. |
| **First-party ≠ special path** | TTS-Engine is a Speak **gateway**, same slot as Next AI TTS. |
| **Ai_code = reference** | Use for FS WS dialect and 20 ms behaviour. **Do not** extend Python as the kernel. |

---

## 4. System context (Coral estate)

```
┌──────────────────────────────────────────────────────────────────────────┐
│                         Coral estate (existing)                           │
│  User management │ ACD / FreeSWITCH │ CRM / tickets │ Config │ Telemetry │
│  Agent / admin UI                                                     │
└───────────────┬──────────────────────────┬───────────────────────────────┘
                │ HTTP control / skills    │ SIP, queues, transfer
                │ Coral auth token         │
                ▼                          ▼
┌───────────────────────────────┐   ┌─────────────────────────────────────┐
│  aiorchestrator (THIS)        │   │  FreeSWITCH                         │
│  Go · session · routers · PG  │◄──│  mod_audio_stream (give/take only)  │
└───────┬───────────┬───────────┘   │  uuid_audio_stream → WSS URL here   │
        │           │               └─────────────────────────────────────┘
        │           │
        │           ├──── gRPC/HTTP ────► TTS-Engine (Speak gateway)
        │           ├──── HTTP/WS ──────► Next AI STT / LLM / TTS gateways
        │           ├──── HTTP ─────────► Customer KB / CRM APIs
        │           └──── HTTP ─────────► Coral CRM / transfer (skill gateways)
        │
        └──── file URI / object store (playback inputs, optional recordings)
```

### 4.1 Boundary table

| Component | Owns | Does not own |
|---|---|---|
| **aiorchestrator** | Session, clocks, VAD, Talk composer, Think policy, routers, ingest store, audit, playback jobs | SIP, RTP, ACD, waveform models, LLM/STT clouds, CRM tables, user passwords |
| **mod_audio_stream** | Capture caller PCM → WS; inject AI PCM onto call; Speex; 20 ms framing | STT, LLM, TTS, profiles, RAG |
| **FreeSWITCH / Coral CC** | Call routing, queues, warm transfer, PBX recording | Agent brain |
| **TTS-Engine** | Synthesis (gRPC/WS), PCMU 8 kHz for PSTN, Indic voices | When to speak, barge-in policy, session |
| **Next AI** | STT / LLM / TTS **as separate services we call** | Our profile store, our only KB copy |
| **Coral user mgmt** | Users, orgs, roles, auth | Profile content (that lives in orchestrator PG, keyed by Coral ids) |
| **Customer systems** | Their FAQ/search/CRM truth | Our session lifecycle |

### 4.2 Dialplan change (call center)

Existing mechanism: `uuid_audio_stream` starts a WebSocket to a URL.

**Change:** point that URL at **aiorchestrator’s** `mod_audio_stream` edge, not at `Ai_code` / OpenAI Realtime.

**Do not change:** inbound binary PCM from FS; outbound JSON `streamAudio` inject schema that the C module already expects. That dialect stays **inside** `internal/edge/modaudiostream`.

---

## 5. Logical architecture (inside aiorchestrator)

```
                    ┌─────────────────────────────────────────┐
                    │           CONTROL PLANE (HTTP)           │
                    │  create session · attach · inject · stop │
                    │  enqueue playback job · health · OpenAPI │
                    │  Coral auth → tenant_id, coral_user_id   │
                    └───────────────────┬─────────────────────┘
                                        │
┌───────────────────────────────────────▼───────────────────────────────────────┐
│                         SESSION ACTOR (per session)                            │
│  profile version pinned · clock · attachments · turn state · memory buffer     │
│                                                                                │
│   feeder edge ──normalize──► SESSION BUS (in-memory channels)                  │
│                                   │                                            │
│                    ┌──────────────┼──────────────┐                             │
│                    ▼              ▼              ▼                             │
│               audio tap      text tap        events                            │
│               (VAD)          (captions)      (DTMF, stop)                      │
│                    │              │                                            │
│                    └────── Talk composer (if Talk) ──────┐                     │
│                           Listen / Think / Speak taps     │                     │
│                                                           ▼                     │
│                                                    sink edge                    │
└───────────────┬───────────────────┬───────────────────────┬───────────────────┘
                │                   │                       │
                ▼                   ▼                       ▼
         Listen router       Think router            Speak router
         Knowledge router    Skill router            Translate router
                │                   │                       │
                ▼                   ▼                       ▼
         gateways…           gateways…              gateways…
```

**Modes are subscribers on the bus**, not a hard-wired STT→LLM→TTS pipe.

| Mode on | What subscribes |
|---|---|
| Listen | audio tap → Listen router → text events |
| Think | text finals / playback transcript → Knowledge → rules → Think router → response / skill |
| Speak | response or inject text → Speak router → sink |
| Talk | composer drives Listen + Think + Speak + VAD + barge-in |

LLM off: Listen and Speak still run. Captions and read-outs do not require Think.

---

## 6. Process and concurrency model

| Concern | Design |
|---|---|
| **Language** | Go |
| **Deployable** | Single binary `cmd/aiorchestrator` (modular monolith) |
| **Live session** | One session actor (goroutine + WS read/write) per live call. **No shared STT/TTS socket across sessions.** |
| **Cancel** | `context.Context` per session and per turn; barge-in cancels Speak/Think contexts |
| **Playback** | Same binary; worker loop leases jobs from Postgres (`FOR UPDATE SKIP LOCKED`) |
| **Live scale-out** | N identical instances behind LB with **WebSocket sticky affinity**. Call does not migrate mid-stream. |
| **Shared durable state** | Postgres (profiles, jobs, audit, ingest). Not Redis for session media. |
| **Why not Java kernel** | 20 ms playout + GC is the wrong default; Coral Java remains **client** (control/skills). |
| **Why not Python kernel** | `Ai_code` is reference only; Go matches TTS-Engine estate and session-per-goroutine. |

---

## 7. Tech stack (this service)

| Layer | Choice | Role |
|---|---|---|
| Runtime | Go | Session actors, WS, timers |
| Control API | `net/http` + chi (or equivalent) + OpenAPI | Coral Java integration |
| Live media | WebSocket | FS edge + optional generic feeder |
| Session bus | Go channels | Audio/text/events in-process |
| Persistence | PostgreSQL | Profiles, ingest KB, jobs, audit |
| Blobs | FS or S3-compatible object store | File URI in PG |
| VAD | Local ONNX (Silero-class) | Barge-in / endpointing without vendor RTT |
| Speak (first-party) | TTS-Engine gRPC client gateway | Same Speak port as cloud TTS |
| Engines | Next AI HTTP/WS/gRPC per their engine docs | Separate Listen / Think / Speak gateways |
| Observability | OpenTelemetry (async export) | Per-hop traces; must not block playout |
| Secrets | Vault / env, tenant-scoped | Gateways read; not stored in profile JSON |
| Tests | Fake gateways | Composer/clock tests without vendors |

---

## 8. Package layout (application architecture)

```
cmd/aiorchestrator/          # main, config load, wire routers
internal/
  control/                   # HTTP handlers, OpenAPI, Coral auth middleware
  runtime/
    session/                 # actor, lifecycle, id maps
    clock/                   # live vs playback schedulers
    bus/                     # in-memory taps
    composer/                # Talk state machine, barge-in
    vad/                     # ONNX wrapper
    thinkpath/               # redact → knowledge → rules → LLM → skills
  port/                      # interfaces: Listen, Speak, Think, Translate, Feeder, Sink, Knowledge, Skill
  router/                    # payment-gateway selection, failover, capability filter
  gateway/
    ttsengine/               # Speak
    nextai_stt/              # Listen
    nextai_llm/              # Think
    nextai_tts/              # Speak
    nextai_mt/               # Translate
    ingest_kb/               # Knowledge (local store)
    http_kb/                 # Knowledge (customer search API)
    coral_crm/               # Skill
    coral_transfer/          # Skill (warm handoff)
    http_crm/                # Skill (customer API)
    ...
  edge/
    modaudiostream/          # FS WS dialect ↔ canonical PCM
    file/                    # playback feeder / file sink
  store/                     # Postgres repositories
  obs/                       # OTel, structured logs
```

**Dependency rule:** `runtime` and `composer` import `port` and `router` only. They **never** import `gateway/*` or vendor SDKs. Gateways implement ports; routers select among registered gateways.

---

## 9. Ports (contracts) — technical meaning

Wire formats per vendor are written when that gateway is implemented. The **semantics** below are stable.

### 9.1 Listen (STT)

**Input:** canonical PCM stream or blob; optional language hint; `session_id`.  
**Output:** partial and/or final text; optional timestamps/confidence; end-of-utterance / end-of-stream; typed errors.  
**Capabilities:** `streaming` | `batch` (live requires streaming).  
**Cancel:** abandon in-flight recognition on barge-in / session stop.

### 9.2 Speak (TTS)

**Input:** text (SSML later if profile asks); `session_id`; **cancel/flush**.  
**Output:** canonical PCM (gateway converts from vendor format / PCMU); optional **mark** (utterance fully delivered to sink).  
**Equal gateways:** TTS-Engine, Next AI TTS, Sarvam — composer cannot tell which.  
**TTS-Engine specifics stay in gateway:** gRPC stream, PCMU 8 kHz for PSTN → resample to/from session canonical rate as needed for the sink edge.

### 9.3 Think (LLM)

**Input:** messages; persona + profile instructions; **grounding chunks we assembled**; allowed skill descriptors.  
**Output:** text (prefer streamed tokens); optional structured fields.  
**Policy:** runtime may discard/rewrite after rules. LLM is not source of truth for policy.

### 9.4 Feeder / Sink (edges)

**Feeder:** start (format, peer stream id) → frames → stop/error. Optional DTMF as events.  
**Sink:** frames out; **flush** unplayed; signal playback finished when Talk needs mark semantics.  
**mod_audio_stream edge:** maps FS binary PCM + `streamAudio` JSON ↔ these ports. Platform-native generic WS is **not** required to speak FS JSON.

### 9.6 Knowledge / Skill

**Knowledge:** query → snippets | no-hit. Gateways: ingest store, customer HTTP search, hybrid.  
**Skill:** name + args → result | error. **No blind retry** of side-effecting skills. Gateways: Coral CRM/transfer, customer txn/ticket HTTP; direct RDBMS last-resort only.

### 9.7 Translate (MT)

**Input:** text; source/target language; `session_id`.  
**Output:** translated text (stream or batch); errors.  
**When:** profile `language.behaviour` is `one_way` or `two_way`. Same router/failover pattern as Listen/Speak.

### 9.8 Bundled Talk (optional, non-default)

Audio in → vendor audio out. Optional gateway only. Cannot satisfy Listen-only, Speak-only, or RAG-in-the-middle. **Not** the Next AI integration path.

---

## 10. Router behaviour (payment-gateway style)

```
Speak(text, session):
  candidates = profile.routers.speak.providers   # ordered: primary, failover…
  filter by: registered ∧ healthy ∧ capabilities.fit(session.clock)
  for each candidate:
    try gateway.Speak(...) until success or non-retryable error
  if all fail:
    profile.fallback → canned clip and/or escalate skill
    audit failure
```

Same algorithm for Listen, Think, Speak, Translate, Knowledge, Skill (skill side-effects: **at most one successful act**, no automatic re-pay).

**Profile stores provider ids**, not URLs with secrets:

```text
speak:
  providers: [tts-engine, nextai-tts]
knowledge:
  providers: [ingest-default, customer-kb-http]
skills:
  get_transaction_status: { gateway: customer-crm-http, confirm: false }
```

Gateway registry maps id → implementation + health probe + capability flags.

---

## 11. Control plane API (shape)

Authenticated with **Coral** credentials (estate’s existing token/header — open detail, not a new IdP).

| Operation | Purpose |
|---|---|
| `POST /sessions` | Create: `profile_id`, pin version, `clock`, `tenant_id`, optional caller metadata, optional `recording_ref` |
| `POST /sessions/{id}/attachments` | Bind feeder/sink (or imply FS when WS connects with session token) |
| `POST /sessions/{id}/inject` | Text in (Speak-only / push prompt) |
| `PATCH /sessions/{id}/profile-fields` | Hot-swap allowed fields (language, skill unlock) |
| `GET /sessions/{id}/events` | SSE: captions, state, analytics-friendly events |
| `POST /sessions/{id}/stop` | Drain → terminal |
| `GET /sessions/{id}` | Status, clock, profile version |
| `POST /jobs/playback` | Enqueue file URI + profile; returns job id |
| `GET /jobs/{id}` | Job state |
| `GET /health` | Process + optional gateway probes |
| Profile CRUD | Versioned profiles (API-first; admin UI later) |

**Live CC option:** FS connects WSS with query/path carrying tenant/profile (or pre-created session id). Control may be called by Coral Java before answer, or session created on first WS — choose one operational pattern and document it in edge config; both are valid as long as profile is pinned before Running.

OpenAPI published for Coral Java clients — route and body shapes: `architecture/CONTROL_API.md`. Build order (Coral before vendors): `architecture/PLATFORM_FIRST.md`. Port Go shapes: `architecture/PORTS.md`.

---

## 12. Data plane — live media

### 12.1 Canonical audio (configurable per profile/session)

| Property | Value |
|---|---|
| Encoding | Signed linear PCM |
| Bit depth | 16-bit |
| Channels | Mono |
| Byte order | Little-endian |
| Sample rate | **8000–48000 Hz**, set by profile `audio.canonical_sample_rate_hz` and pinned on session |
| Live frame | **~20 ms** fixed; `frame_bytes = rate × 2 × 0.02` (e.g. 640 B @ 16 kHz, 320 B @ 8 kHz) |

Common rates: 8000 (PSTN), 16000 (wideband), 24000/48000 (high-quality playback).  
Feeder edge: decode/resample **into** session canonical.  
Sink edge: encode/resample **from** canonical to peer (FS dialplan rate, file rate, etc.).  
Speak/Listen gateways: vendor format ↔ canonical **inside that gateway**.  
Full schema: `architecture/PROFILE_SCHEMA.md` §2.

### 12.2 Live path (call center)

```
Caller RTP → FS → mod_audio_stream
  → WSS binary PCM (~20 ms)
  → edge/modaudiostream → canonical frames → session bus
  → VAD / Listen router → text
  → Think path (if on)
  → Speak router → PCM frames
  → edge → JSON streamAudio { audioDataType: raw, sampleRate, channels, audioData: base64 }
  → mod_audio_stream inject → caller
```

### 12.3 Why no broker for PCM

- One ear, 20 ms: broker adds copy + jitter and still needs a single consumer (the session holding the socket).  
- Barge-in must flush **this process’s** sink buffer immediately.  
- Industrial voice runtimes keep media with the socket owner.

**Non-PCM durable work** uses Postgres jobs (file URI), not audio messages.

### 12.4 Playback path

```
POST /jobs/playback { profile, file_uri }
  → row in jobs
  → any instance leases job
  → file edge reads blob
  → Listen (batch OK) → Think (+ template) → optional Skill / file text sink
  → job Completed | Failed + audit
```

No FreeSWITCH required. No Talk state machine unless profile asks for simulated turns.

---

## 13. Session lifecycle (detailed)

```
Created
  → Attached   (feeder and/or sink as profile requires)
  → Running
  → Draining   (stop: finish current Speak or drop per profile policy)
  → Terminal   (Completed | Cancelled | Failed)
```

**Create requires:** tenant, profile id, resolved then **pinned** profile version, clock.  
**Ids:**

| Id | Owner | Use |
|---|---|---|
| `session_id` | Orchestrator | Audit, Coral, all control |
| Feeder peer id (e.g. FS uuid) | Edge map | Debug, correlation |
| Vendor stream/request ids | Gateway map | Cancel, vendor support |

Never expose vendor ids as the Coral primary key.

---

## 14. Talk composer (live turn machine)

```
Listening ──VAD speech──► Capturing ──endpoint──► Thinking ──text──► Speaking ──mark──► Listening
                ▲                         │                │
                │         barge-in        │                │ barge-in
                └─────────────────────────┴────────────────┘
                     flush sink + cancel Speak (+ cancel Think if active)
```

| Concern | Owner |
|---|---|
| Endpointing | Us (silence ms, STT final, max utterance) — **profile knobs** |
| Barge-in | Us: flush sink → cancel Speak gateway → cancel Think → Capturing |
| Mark | Sink/Speak gateway; composer waits only when not barged |
| Listen-only | Never enters this machine |
| Playback Think | One-shot; no barge-in |

VAD: local ONNX. Endpointing delay trades cut-off vs perceived latency (often 200–400 ms of the turn budget).

---

## 15. Think path (technical pipeline)

```
inbound text (STT final | inject | playback transcript)
  → append session memory
  → PII redact (if profile)
  → playbook step (if profile.grounding.type = playbook) — intent/slots/state
  → Knowledge router.Retrieve(...)
  → if profile.grounding.required && no-hit → refuse/escalate (no LLM invent)
  → rules pre-check (may block Think)
  → Think router.Complete(persona, instructions, memory window, chunks, skill descriptors)
  → rules post-check (strip / force escalate)
  → if act requested → Skill router (authority + confirm + audit)  // no blind retry
  → append assistant memory
  → if language.behaviour ≠ none → Translate router (optional, on text path)
  → emit text → Speak tap and/or text sink
  → audit event
```

Grounding chunks are **copies for this call**. Default: do not persist customer API snippets as our KB.  
Rules, skills, playbooks: `architecture/RULES_AND_SKILLS.md`.

---

## 16. Knowledge and Skill integration (architecture)

### 16.1 Knowledge gateways

| Gateway | Behaviour | Persist |
|---|---|---|
| **ingest** | Chunk/embed uploaded docs into our store | Versioned copy they chose to publish |
| **http_kb** | Call their search/KB API this turn | Ephemeral (unless audit policy keeps redacted snippet) |
| **hybrid** | Profile lists both; rules/intent pick which | Per gateway |

### 16.2 Skill gateways

| Gateway | Behaviour |
|---|---|
| **http_crm / http_txn** | Their REST for “status of txn 4412” |
| **coral_crm / transfer** | First-party Coral APIs / FS transfer trigger |
| **sql_readonly** | Last resort: allowlisted queries only |

Credentials: tenant-scoped in vault. Orchestrator is **HTTP client**, not CRM replica.

### 16.3 Identity binding

| Actor | Binding |
|---|---|
| Operator | Coral auth → `coral_user_id`, `tenant_id` on control + audit |
| Caller | `from` / channel id; optional CRM skill resolution; optional existing Coral/customer id **reference** |

Extension tables (preferences, allowed profiles) **key by Coral id** — never a parallel user master.

---

## 17. Data model (logical)

PostgreSQL (illustrative entities — not a migration SOW):

| Entity | Purpose |
|---|---|
| `profile` / `profile_version` | Immutable versions; session pins one |
| `gateway_registration` | Optional DB view of enabled gateways per tenant (or config file + vault) |
| `kb_document` / `kb_chunk` | Ingest store |
| `session` | id, tenant, profile_version, clock, state, owner_instance, canonical_sample_rate_hz, coral_user optional, caller metadata, recording_ref |
| `audit_event` | Append-only; turn-correlated |
| `playback_job` | file_uri, profile_version, state, lease_owner, leased_until |
| `postcall_job` | session_id, profile_version, state, lease (disposition/summary pipeline) |
| `analytics_event` | Append-only metrics per session/turn (see ANALYTICS_AND_POSTCALL.md) |
| `customer_memory` | Optional keyed by tenant + caller/customer id; consent + TTL |
| `tenant_extension` | Optional flags keyed by Coral tenant/user |

**Blobs:** object store; only URI in PG.

**Not in PG:** live PCM buffers.

---

## 18. Audit vs logging vs telemetry

| Signal | Store | Purpose |
|---|---|---|
| **Audit** | Append-only PG | Dispute / compliance: profile version, clock, gateway ids, STT/Think/Speak/skill outcomes (policy/redaction), barge-in, errors |
| **Logs** | Structured logs | Ops debug; shorter retention; **not** legal record |
| **Traces** | OTel | Hop latency (VAD, STT, retrieve, LLM, TTS, skill); async export |

If it is not in audit, we cannot defend the call.

---

## 19. Failure, degradation, latency

### 19.1 Failure matrix

| Failure | Behaviour |
|---|---|
| Feeder disconnect | Terminal + audit |
| Listen/Think/Speak timeout | Next gateway in failover list → profile fallback (clip / escalate). **No dead air** |
| Knowledge no-hit (grounded) | Refuse or escalate; no invent |
| Skill error | Fail the act; do not auto-retry side effects |
| Instance crash (live) | Call drops; sticky session not reconstructed mid-call (honest). Restart + new call. |

### 19.2 Live Talk latency budget

Target: **p50 &lt; ~1.2 s**, p95 &lt; ~2 s (user endpoint → first reply audio). POC baseline ~0.8–1.0 s on current FS path.

| Hop | Typical order | Notes |
|---|---|---|
| Endpointing (VAD silence) | 200–400 ms | Profile knob; largest delay we own |
| STT finalize | 100–400 ms | Streaming required when live |
| LLM TTFB | 200–600 ms | Prefer token stream |
| TTS first chunk | 50–300 ms | TTS-Engine Piper ~50 ms class |
| Playout prebuffer | 40–100 ms | 2–5 × 20 ms |
| FS + RTP | ~20–40 ms | Edge |

**Rules:** stream every live hop; capability-gate batch engines off live; speculative TTS (Speak before full LLM) is a composer optimization when implemented — not a broker.

Playback: job duration SLA only; batch engines allowed.

---

## 20. Security

- TLS for HTTP and WSS.  
- Tenant isolation on all PG rows and gateway credentials.  
- PII redaction before Think and before audit when profile requires.  
- Recording/AI disclosure: profile greeting **rule**, not an engine.  
- Live clock cannot select non-streaming Listen/Speak gateways.  
- Secrets never in profile documents or audit payloads.

---

## 21. Deployment topology

```
                  LB (WS sticky + HTTP)
                        │
          ┌─────────────┼─────────────┐
          ▼             ▼             ▼
     orch-1          orch-2        orch-N     (same Go binary)
          │             │             │
          └─────────────┼─────────────┘
                        ▼
                   PostgreSQL
                        │
              object store (blobs)
```

| Traffic | Affinity |
|---|---|
| Live WSS | Sticky to instance holding session |
| Control HTTP | Any instance (session row in PG; live media still on sticky node — control that needs live state must route to owner or only mutate durable fields) |
| Playback workers | Any instance; lease in PG |

Operational note: prefer **create session on the instance that will take the WS**, or store `owner_instance` on the session row for control routing. Document the chosen rule in ops runbook when coding starts.

---

## 22. End-to-end sequences

### 22.1 Live contact Talk

1. Coral/FS answers call; dialplan starts `uuid_audio_stream` → orch WSS.  
2. Edge authenticates tenant/profile (or attaches to pre-created session).  
3. Session Running; audio frames → VAD → Listen gateway → finals.  
4. Think path (knowledge + rules + LLM + optional skill to their CRM).  
5. Speak gateway (e.g. TTS-Engine) → inject to FS.  
6. Barge-in: flush + cancel → new Capturing.  
7. Transfer skill → Coral HTTP payload (summary, intent, transcript, recording_ref); Coral/FS completes queue transfer; session Draining.  
8. Post-call job → disposition template + analytics export.  
9. Audit events written throughout.

### 22.2 Playback MoM

1. `POST /jobs/playback` with recording URI + meeting profile.  
2. Worker leases job; file Listen; Think + MoM template; optional mail skill.  
3. Job complete; audit; no FS.

### 22.3 Captions only

1. Live or playback session; Listen on; Think/Speak off.  
2. Text events to control SSE/WS or text sink.  
3. No Talk machine.

---

## 23. What this architecture explicitly excludes

| Excluded | Reason |
|---|---|
| Python/Java as media kernel | Locked Go; Ai_code reference only |
| Kafka/Redis audio bus | Wrong for 20 ms + barge-in |
| Microservice-per-engine | Extra hops kill live SLA |
| `if nextai` in composer | Breaks gateway pattern |
| Embedding TTS-Engine in-process | Destroy swap; it is already a server |
| Duplex vendor WSS as default Talk | Blocks Listen-only / our RAG |
| Second user directory | Coral owns identity |
| Absorbing customer CRM into PG | Connect via Skill/Knowledge gateways |

---

## 24. Language and translation

| Concern | Design |
|---|---|
| Auto-detect | Listen gateway capability or dedicated detect on first utterance; writes `session.detected_language` |
| Mid-call switch | Hot field via control API; restart Listen with new hint; flush partial STT |
| One-way interpret | Translate router on caption or Speak text path |
| Two-way interpret | Dual attachments + dual Listen/Speak/Translate chains (see RUNTIME.md) |
| MT engines | **Translate router** — same payment-gateway pattern as Listen/Speak |

---

## 25. Open items (honest, bounded)

| Item | Why open | Constraint when closed |
|---|---|---|
| Next AI STT/LLM/TTS/MT wire formats | Engine docs not in hand | Must map to §9 ports; separate gateways; **Phase F lab uses Sarvam** until Next AI docs arrive |
| Sarvam STT/LLM/TTS wire formats | Public docs (REST + STT WS) | Phase F first real adapters: `sarvam-stt`, `sarvam-llm`, `sarvam-tts` |
| Coral auth header/token exact shape | Estate convention | Middleware only; no new IdP |
| Coral warm-transfer HTTP path | Estate endpoint naming | Payload shape locked in RULES_AND_SKILLS.md |
| Ingest vector backend | pgvector vs external | Knowledge **port** unchanged |
| Session create vs WS-first for FS | Ops preference (both documented in EDGE_FS.md) | Profile pinned before Running |
| Admin GUI | Product in-category | API + PROFILE_SCHEMA enough to operate |
| Graph knowledge gateway | Later SKU | Same Knowledge router |

---

## 26. Architecture review checklist

Say **yes** to this architecture solution if:

1. Product lock stays in `PRODUCT_DECISIONS.md`; this file only implements it.  
2. Captions (Listen only) and Speak-only work without Talk / without LLM.  
3. Speak can switch TTS-Engine ↔ Next AI TTS by profile/router only.  
4. FS path is give/take only; dialplan URL retarget; inject schema preserved in edge.  
5. Customer txn status is a Skill gateway HTTP call, not a copied CRM.  
6. Coral users are the directory; audit is append-only and distinct from logs.  
7. PCM never enters Kafka/Redis; live scale is sticky instances.  
8. Composer never names a vendor.  
9. Open items above are acknowledged, not hidden.

---

## 27. Document map

| Doc | Role |
|---|---|
| **This file (`SOLUTION.md`)** | Full **architecture solution** for review |
| `product/PRODUCT_DECISIONS.md` | **Product** lock |
| `architecture/ARCHITECTURE.md` | Locked approach summary |
| `architecture/RUNTIME.md` | Session/bus/composer depth |
| `architecture/CONTRACTS.md` | Port semantics |
| `architecture/PORTS.md` | Frozen Go-shaped port interfaces |
| `architecture/PLATFORM_FIRST.md` | Build order: Coral complete before vendors |
| `architecture/CONTROL_API.md` | Coral-facing HTTP/SSE API shape |
| `architecture/SERVICE.md` | Service responsibilities / stack |
| `architecture/INTEGRATION.md` | KB/CRM/identity/audit |
| `architecture/TECH_CHOICES.md` | Why Go / why not Kafka / etc. |
| `architecture/BUILD.md` | Coral fit + latency notes |
| `architecture/PROFILE_SCHEMA.md` | Profile JSON shape, audio rates, versioning |
| `architecture/RULES_AND_SKILLS.md` | Rules engine, skills, playbooks, warm transfer |
| `architecture/EDGE_FS.md` | mod_audio_stream WSS auth and resampling |
| `architecture/ANALYTICS_AND_POSTCALL.md` | KPIs, disposition, captions SSE |
| `architecture/OPERATIONS.md` | Deploy, drain, limits, on-prem packs |

After review, change **this file** (then companions) where you disagree — do not start a parallel architecture story.
