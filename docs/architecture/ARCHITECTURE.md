# Architecture — locked approach

**Status:** LOCKED (replaces earlier “Python V1 / phased stack” talk).  
**Date:** 26 August 2026  
**Product parent:** `docs/product/PRODUCT_DECISIONS.md` (product features still win if they conflict)

**Locks from this discussion**

1. **Language: Go** — this service is a Go process. `Ai_code` is a **reference**, not the base.  
2. **Vendors: payment-gateway style** — product says Listen / Think / Speak; a **router** sends the call to the **active** provider. Adding a provider is a new gateway, not a change to Talk.  
4. **Customer systems: connect, don’t absorb** — Profile / persona / skill *definitions* live **here**. KB/FAQ/RAG **content** and CRM/RDBMS stay **theirs** unless they **dump** into us or we **call their APIs** this turn. See `INTEGRATION.md`.  
6. **Identity: Coral user management** — operators and tenants come from the existing Coral directory. This service stores **references** (`coral_user_id`, tenant) and may add product flags later on that id. It is not a second IdP. Callers are ANI/channel + CRM skill unless they already have a Coral/customer id.

---

## 1. What we are building

A **Go runtime** that:

- Attaches live or playback audio/text  
- Runs a **named profile** (modes, rules, KB, skills, **which gateways are active**)  
- Talks to **engine gateways** (STT, LLM, TTS) the way a checkout talks to Razorpay vs PayU  
- Gives audio/text/actions back out  

FreeSWITCH + `mod_audio_stream` = **card machine / POS** (bytes in/out).  
TTS-Engine, Next AI TTS, Sarvam = **payment gateways** on the Speak rail.  
This service = **checkout + order logic** (session, VAD, barge-in, RAG, rules).  
Coral Java = **ERP** (ACD, CRM, tickets) via skills and control HTTP.

---

## 2. Payment-gateway pattern (engines)

Product-level code **never** names Next AI or TTS-Engine.

```
Talk composer:  “Speak(text, session)”
                     │
                     ▼
              Speak router
              (profile: active TTS + failover list)
                     │
        ┌────────────┼────────────┐
        ▼            ▼            ▼
   TTS-Engine    Next AI TTS    Sarvam
   (gateway)     (gateway)      (gateway)
```

Same **router family** (not only STT/LLM/TTS):

- **Knowledge router** — ingest store **or** customer KB HTTP (or both).  
- **Skill router** — Coral CRM / transfer **or** customer txn/ticket APIs.  
- **Translate router** — MT gateways for interpret profiles.  

Product says “ground this turn” / “get status”; the **active** knowledge/skill gateway runs. Same pattern as Speak.

| Checkout analogue | Us |
|---|---|
| “Pay ₹500” | “Speak this text” / “Transcribe this audio” / “Complete this prompt” |
| Active PG + MID | Profile: `speak.provider = tts-engine`, secrets in vault |
| PG adapter (ISO messages) | Gateway: their gRPC/HTTP/WS → our port |
| Enable/disable PG in admin | Activate/deactivate gateway; health probe |
| Fallback PG on decline | Failover list (no dead air: next gateway or canned clip) |
| Refund in our ledger, not in PG | Session/transcript/audit **here**, not only in the vendor |

**Capability filter:** live session → router **skips** gateways that cannot stream (same as not sending a UPI-only intent to a card-only PG).

**First-party is still a PG:** TTS-Engine is “our Razorpay,” not the checkout itself.

Edges (FS, file) are **not** payment gateways. They are **acquire / settle rails** (feeder/sink adapters). Different registry, same idea: product says “attach audio,” router does not care if it is FS or a file.

---

## 3. Media plane (the Kafka decision)

**PCM and live text-for-playout never enter a broker.**

Why this is the right architecture, not a shortcut:

- A turn is **one session, one ear, 20 ms**. Kafka/Redis copy, batch, and persist; they add jitter and still need a single consumer — the session that owns the socket.  
- Barge-in must **flush this process’s sink buffer now**. A broker cannot do that.  
- LiveKit, FS, every serious voice runtime keeps media **in the process that holds the socket**.

**What we use instead (complete system, not a later phase):**

| Data | Store |
|---|---|
| Live audio / turn text in flight | **Goroutine + in-memory channels** (session bus) |
| Profiles, KB metadata, session/audit, playback **jobs** (file URI, not samples) | **PostgreSQL** |
| Recording / playback blobs | **Filesystem or object store** (URI in PG) |
| Hop traces / metrics | **OpenTelemetry** (async) |
| Coral notify (disposition, transfer) | **HTTP** skill/control (they already are HTTP services) |

No Redis on the media path. No Kafka. Playback **workers** are more Go processes (or the same binary) that **lease jobs from Postgres** (`FOR UPDATE SKIP LOCKED`). That is distribution without treating audio as messages.

Live scale-out: **more Go instances + WS affinity**. A call never migrates mid-stream. That is correct, not incomplete.

---

## 4. System architecture

```
                         Coral Java (ACD, CRM, config, telemetry)
                                    │  HTTP control / skills
                                    ▼
┌───────────────────────────────────────────────────────────┐
│  aiorchestrator (Go)                                      │
│  HTTP control │ session actors │ routers │ in-memory bus  │
│  PG: profiles, KB, jobs, audit                            │
└──────────┬─────────────────┬──────────────────┬───────────┘
           │ WS              │ gRPC/HTTP        │ HTTP
           ▼                 ▼                  ▼
    mod_audio_stream    Speak/Listen/Think    Next AI
    (FreeSWITCH)        gateways:             STT / LLM / TTS
                        TTS-Engine, …         (gateways)
```

**This service owns:** session, clocks, VAD, composer, routers, RAG/rules, skill dispatch, job lease, audit.  
**This service does not own:** SIP/ACD, waveform models (TTS-Engine process), LLM/STT clouds, CRM tables.

---

## 5. Application architecture (inside Go)

Hexagonal:

- **Core:** session actor, Talk state machine, clocks, rules, retrieve  
- **Ports:** Listen, Think, Speak, Translate, Feeder, Sink, Skill, Knowledge  
- **Routers:** pick active gateway by profile + health + capabilities  
- **Gateways:** one package per provider (`internal/gateway/ttsengine`, `internal/gateway/nextai`, …)  
- **Rails:** `internal/edge/modaudiostream`, `internal/edge/file`

Core **must not** import gateway SDKs.

**Session actor:** one goroutine (plus its WS read/write). Channels for audio frames. Cancel via `context`.

**Control:** Go HTTP (stdlib or chi) + OpenAPI for Coral.

**TTS-Engine:** gRPC client **inside the Speak gateway**, PCMU 8 kHz converted there to session canonical PCM (8000–48000 Hz per profile).

---

## 6. Live vs playback (same service, two clocks)

Same routers, same gateways, same PG.

| | Live | Playback |
|---|---|---|
| Trigger | FS WS connect (or generic WS) | Job row in PG (file URI) |
| Clock | 20 ms paced, VAD, barge-in | Run as fast as quality allows |
| Gateways | Streaming-capable only | Streaming or batch |
| Scale | Sticky instance | Any instance that leases the job |

---

## 7. Failure

Router: primary gateway timeout → failover gateway → profile fallback (clip / escalate skill).  
No dead air. Skills with side effects are not blindly retried (like a double charge).

---

## 8. Latency (unchanged physics)

p50 &lt; ~1.2 s live turn. Budget: our VAD + vendor STT + vendor LLM + Speak gateway TTFB (TTS-Engine ~50 ms) + 40–100 ms playout. Stream every live hop. Speculative TTS when we implement Talk fully — it belongs in the composer, not in a broker.

---

## 9. What we explicitly discarded

- Python kernel (reference only)  
- Java media loop  
- `if nextai` in Talk  
- Kafka/Redis **audio**  
- Microservice-per-engine  
- Embedding TTS-Engine as a library  
- Bundled duplex vendor as the default Talk path  

Companion notes: `TECH_CHOICES.md`, `SERVICE.md`, `RUNTIME.md`, `CONTRACTS.md`, `BUILD.md`, `PROFILE_SCHEMA.md`, `RULES_AND_SKILLS.md`, `EDGE_FS.md`, `ANALYTICS_AND_POSTCALL.md`, `OPERATIONS.md` (aligned to this lock).
