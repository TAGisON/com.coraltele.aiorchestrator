# Service, tech stack, and architecture style

**Status:** Planning lock for *this* deployable (`com.coraltele.aiorchestrator`).  
**Date:** 26 August 2026  
**Parents:** `BUILD.md`, `RUNTIME.md`, `CONTRACTS.md`, product lock.

---

## 1. What this service is

One **long-running application** we install (on-prem or our cloud). It is the Speech-and-Agent **runtime**.

It is **not** FreeSWITCH, **not** TTS-Engine, **not** telemetry, **not** the agent desktop, **not** Next AI.

In Coral’s estate it is a **peer service**: FS talks to it on the media WS; Java services talk to it on control HTTP (and later events); it talks **out** to Speak/Listen/Think vendors (TTS-Engine, Next AI, …).

---

## 2. Architecture styles we use (and do not)

| Style | How we use it |
|---|---|
| **Hexagonal (ports and adapters)** | Core = session, profile, composer, rules. **Ports** = Listen, Speak, Think, Translate, Feeder, Sink, Skill, Knowledge. **Gateways** = adapters. This is the only extension style. |
| **Modular monolith** | **One process / one deployable.** Plugins in-process. Not 8 microservices (STT svc, TTS svc, VAD svc…) — that would add hops and kill live latency. |
| **Session as unit of isolation** | Each session is its own **goroutine** (actor-like). No shared Listen socket across sessions. |
| **Split APIs, same process** | **Control plane** (small, reliable): HTTP. **Data plane** (audio): WebSocket (and file for playback). Not two products. |
| **Internal event bus** | In-memory session bus (audio tap / text tap / events). **Not** Kafka/Rabbit for PCM. |
| **Plugin / strategy** | Gateway registry: profile says `listen: nextai`, `speak: tts-engine`. |

**We do not use:** serverless per turn, ESB, Java media kernel, Kafka/Redis for PCM, service mesh between STT and TTS.

**Scale-out:** more **instances** of this same Go service + sticky live sessions (WS affinity). Playback: any instance that **leases a PG job**.

---

## 3. Tech stack (this service)

| Concern | Choice | Why |
|---|---|---|
| Language | **Go** | Session-per-goroutine, no GIL, binary next to TTS-Engine, gRPC |
| Control API | `net/http` + chi (or equivalent) + OpenAPI | Coral Java |
| Live media | WebSocket | `mod_audio_stream` dialect in an **edge** package |
| Session bus | In-memory channels | Locked: no broker for PCM |
| Durable state / playback jobs | PostgreSQL | Profiles, KB, audit, job lease (`SKIP LOCKED`) |
| Blobs | Filesystem / object store | Recordings, playback inputs (URI in PG) |
| VAD | Local ONNX | Barge-in without vendor RTT |
| Observability | OpenTelemetry | Hop traces; async export |
| Speak to TTS-Engine | gRPC **gateway** | Same Speak port as Next AI TTS |
| Secrets | Vault / env, not profile rows | Gateway reads keys |
| Tests | Fake gateways | Composer without vendors |

`Ai_code` is reference for 20 ms / FS JSON only.

---

## 4. What this service is responsible for — and how

### Owns (must)

| Responsibility | How |
|---|---|
| **Session lifecycle** | Control API: create (pin profile version, clock), attach feeder/sink, drain, terminal. **Coral user/tenant on the session** (from existing user management). Maps `session_id` ↔ feeder/vendor ids. |
| **Interpret the profile** | Modes, **active gateways** + failover, KB, rules, skills, language. Live ⇒ skip non-streaming gateways. |
| **Invoke vendors** | Composer → **router** → active gateway. Never `if nextai` in Talk. |
| **Canonical audio + clocks** | Live: jitter buffer, ~20 ms frames at session canonical rate (8000–48000 Hz from profile). Playback: read blob, no barge-in. One clock per session. |
| **VAD / endpointing / barge-in** | Local VAD; composer; sink flush + cancel Speak/Think **gateways**. |
| **Mode composition** | Listen/Speak/Think as subscribers. Talk = composer using **routers**. LLM off still Listen/Speak. |
| **Think policy** | Retrieve **our** KB, rules before/after LLM, refuse if grounded and no hit, skill execute only if allowed + audited. |
| **Playback jobs** | Lease from Postgres (`SKIP LOCKED`); file URI feeder; same kernel, playback clock. |
| **Degradation** | No dead air: fallback clip / escalate skill per profile. |
| **Emit telemetry** | Turn traces, hop latency, no-grounding-hit, barge-in, session outcome — for Coral/ops. |
| **Audit trail** | Append-only PG events per turn (gateways, skill calls, redacted payloads). Logs/OTel are **not** the legal record. |

### Does not own (must not grow into)

| Out of this service | Who |
|---|---|
| SIP, RTP, queues, hunt, recording files as PBX | FreeSWITCH / Coral CC |
| PCM capture/inject on the call | `mod_audio_stream` (edge gateway talks to it) |
| Waveform synthesis | Speak **vendor** (TTS-Engine, Next AI TTS, …) |
| Hosted STT/LLM models | Listen/Think gateways (on-prem engine pack = different gateway registry) |
| CRM records, agent UI, config UI as product | Coral Java / GUIs |
| Sending mail, tickets | **Skill adapters** calling those systems |
| Being Exotel or Next AI’s duplex WSS brain | Not our path |

---

## 5. System view (where it sits)

```
                    Coral Java (ACD, CRM, config, telemetry)
                         │ control HTTP / later events
                         ▼
┌──────────────────────────────────────────┐
│  aiorchestrator  (this service)          │
│  control API │ session kernel │ plugins  │
└───────┬──────────────┬─────────────┬─────┘
        │ data WS      │             │ vendor APIs
        ▼              ▼             ▼
 mod_audio_stream    files      Next AI STT/LLM/TTS
 (FS)                           TTS-Engine (Speak vendor)
```

Live CC: FS is **client** of our data WS (today’s `uuid_audio_stream` URL).  
Playback: Java or a job client **creates a session** with `clock=playback` and a file attachment — FS is absent.

---

## 6. Application structure (inside the repo, later)

Logical packages (names illustrative):

- `cmd/aiorchestrator` — process  
- `internal/runtime` — session actor, clock, bus, composer, VAD  
- `internal/port` — Listen, Speak, Think, Translate, Feeder, Sink, Skill, Knowledge (see `PORTS.md`)  
- `internal/router` — payment-gateway style; active provider + failover  
- `internal/gateway` — `ttsengine`, `nextai`, …  
- `internal/edge` — `modaudiostream`, `file`  
- `internal/store` — Postgres  
- `internal/obs` — OTel  

Core **never** imports gateway SDKs. Composer calls **routers** only.

---

## 7. Multi-instance (future, designed now)

Live sessions are **sticky** to the instance that holds the WS. Load balancer: WS affinity.  
Playback: any instance.  
Shared profile/KB store (files NFS or later DB) so all instances see the same profile versions.  
No shared in-memory session table across hosts — do not start Redis for audio.

---

## 8. How a request runs (responsibility in motion)

**Live Talk:** FS connects data WS → edge adapter → session Running → audio tap → VAD → Listen gateway → text → Think (playbook, RAG, rules → Think gateway) → Speak gateway (e.g. TTS-Engine) → sink adapter → FS inject. Barge-in stays inside runtime.

**Playback MoM:** control create session + file feeder → Listen (batch OK) → Think + template → skill adapter (mail/CRM) or text sink. No Talk machine.

**Captions only:** Listen on, Think/Speak off, text events on control stream or text sink.

That is the whole service: **run profiles on attached streams**, with vendors behind ports, with live and playback as clocks, with Coral around it not inside it.
