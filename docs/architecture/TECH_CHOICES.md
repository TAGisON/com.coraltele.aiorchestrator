# Tech choices — why this, why not something else

**Status:** Review with **locks**: kernel **Go**; engines **payment-gateway routers**; audio **in-memory only**.  
**Date:** 26 August 2026

Scoring in this file: **best for live 20 ms + N sessions** vs **best given Coral’s existing code**. Those are not always the same.

---

## 1. Language of *this* service (the big one)

The work is: many concurrent WebSockets, 20 ms timers, streaming HTTP/gRPC to vendors, local VAD (CPU), almost no “business CRUD.”

### Python + asyncio

| For | Against |
|---|---|
| We already have the FS 20 ms framer, inject JSON, playout loop in `Ai_code` | GIL: many sessions doing ONNX VAD + resample on one process will **contend** |
| Fastest path to *correct architecture* (contracts, not Realtime) | Packaging/on-prem (venv, wheels) is messier than a Go binary |
| STT/LLM vendor SDKs are Python-first | Tail latency under load worse than Go |
| Team already shipped a live POC in it | Not how TTS-Engine or Coral Java are operated |

**Best at:** time-to-kernel, vendor glue.  
**Not best at:** high concurrency live Talk on one box.

### Go

| For | Against |
|---|---|
| Goroutine ≈ one session; no GIL | Rewrite of `Ai_code` (weeks, not days) |
| One static binary next to TTS-Engine | Fewer “official” STT/LLM SDKs (HTTP/gRPC still fine) |
| gRPC to TTS-Engine is idiomatic | Team’s live-audio POC is Python, not Go |
| Predictable p99 for timers + WS | Slightly slower first gateway to a new Python-only SDK |

**Best at:** the **actual** realtime job (N live sessions, streaming, ops with TTS-Engine).  
This is the strongest *technical* kernel language.

### Java (Spring / Netty)

| For | Against |
|---|---|
| Matches `com.coraltele.*`, hiring, monitoring, config culture | GC on a 20 ms inject loop is the wrong default (mitigable with ZGC + dedicated threads, still fighting the platform) |
| Control API and Coral skills are natural | Telemetry today is **Java 8** Spring — not a modern low-pause audio runtime |
| | They already **left** Java for this exact path (`Ai_code`) |

**Best at:** control plane, skill adapters *calling* Coral, admin APIs.  
**Not best at:** being the media kernel **in the same JVM**.

A **split** (Java control + Go media) is technically clean and **ops-heavy**. We rejected it so we do not add a hop or two deployables before the kernel exists.

### Also-rans

| | Why not |
|---|---|
| **Node** | Event loop is fine; numeric/VAD/ONNX story is weaker; not in the estate |
| **Rust** | Best safety/latency; slowest delivery; no evidence we staff it |
| **C/C++** | Belongs in `mod_audio_stream`, not the agent runtime |
| **.NET** | Not in the estate |

### Verdict (locked)

**Kernel language: Go.** `Ai_code` is a reference for the FS WS dialect and 20 ms behaviour, not the codebase we extend.

Java remains Coral (control client, skills, ACD). Python is not the orchestrator.

---

## 2. One service vs microservices

**Why a modular monolith:** every extra hop is milliseconds we do not have. STT→LLM→TTS already crosses the internet. Adding “VAD service” and “composer service” is how you miss 1.2 s.

**Why not microservices:** independent scale sounds good; live Talk is **one sticky session** that needs VAD+composer+WS together. You would still colocate them.

**When to split:** GPU STT farm, or Java admin UI, as **adapters/clients**, not as the turn machine.

---

## 3. Hexagonal / ports-adapters vs “just call the SDK”

**Why ports:** product lock (swap Next AI / TTS-Engine / Sarvam without rewriting Talk).  
**Why not a switch/case on vendor in the composer:** that *is* vendor lock with extra steps.  
**Why not a generic “AI gateway” product (Kong, etc.):** they do not do barge-in, clocks, or profiles.

---

## 4. Concurrency model (locked: Go goroutines)

**Go:** one goroutine per session actor; `context` for cancel. Network wait dominates; VAD runs on CPU without GIL contention.

Historical note: if this service were Python, asyncio would be the right I/O model — but the kernel is **Go**, not Python threads or process-per-call.

---

## 5. FastAPI for control vs gRPC vs raw aiohttp

| | |
|---|---|
| **FastAPI** | Coral Java needs a boring HTTP API + OpenAPI. Session create is CRUD. Good. |
| **gRPC control** | Better if only Go/Java talk; worse for curl, GUI, first integration. Can add later. |
| **Same WS for control+media** | Possible (Exotel style); mixes small commands with 20 ms PCM; harder to RBAC. Split is cleaner. |

**Media data plane stays WebSocket** because **`mod_audio_stream` already is WS**. Changing that is a C-module project, not an orchestrator style choice.

**Speak to TTS-Engine:** **gRPC or their WS** inside the **Speak gateway**, not as the FS protocol.

---

## 6. In-process plugins vs sidecar per vendor

**In-process:** no extra hop, one deploy.  
**Sidecar:** crash isolation (bad TTS client does not kill Talk); extra latency and ops. Use later if a vendor SDK is toxic.  
**Not:** Envoy in the 20 ms path.

---

## 7. In-memory session bus vs Kafka / Redis Streams (audio)

**Locked: in-process memory (channels) for PCM and in-flight turn audio/text.**

Kafka/Redis as an **audio bus** are the wrong architecture: extra copy, extra jitter, no barge-in flush of *this* sink, still one consumer (the session).

**Not a substitute later — this is the media plane.**

Durable **non-audio**: PostgreSQL (profiles, KB, session audit, playback **jobs as file URIs**). Workers lease jobs with `SKIP LOCKED`. Coral: HTTP. Traces: OTel.

No Redis on the media path. No Kafka.

---

## 8. Local VAD (Silero-class ONNX) vs vendor VAD vs energy-only vs WebRTC VAD

| | |
|---|---|
| **Local ONNX** | Barge-in cannot wait on Next AI. ~ms, works if vendor is slow. CPU cost (Go handles better at scale). |
| **Vendor VAD only** | Extra RTT; barge-in tied to their outage. |
| **Energy threshold** | Cheap; false barge-in on noise. OK as fallback. |
| **WebRTC VAD** | Light, weaker on Indic/noise. Possible first step. |

**Best:** local neural VAD + profile endpointing knob. **Not** VAD-as-a-service.

---

## 9. Sticky live sessions vs shared session state (Redis)

**Sticky WS:** the live path **holds** sockets and playout buffers. Sharing that via Redis is a science project.  
**Redis for profile cache / playback jobs:** fine later.  
**Best for live:** LB affinity. **Not:** “stateless live Talk.”

---

## 10. YAML profiles in the service vs only `com.coraltele.config`

**YAML/JSON in orchestrator PG:** kernel can start without a Java dependency.  
**Config service later:** distribution, UI, tenancy — **client** of the same profile schema (`PROFILE_SCHEMA.md`).  
**Not:** Spring XML inside the media loop.

---

## 11. OpenTelemetry vs only logs vs Prometheus-only

**OTel:** TTS-Engine already has it; hop traces are how we defend 1.2 s.  
**Logs only:** cannot budget STT vs LLM vs TTS.  
**Prometheus-only:** good for rates; weak for one slow turn. Use **both** (OTel traces + metrics).

Must **not** block the playout task (async export).

---

## 12. Fake gateways in tests vs only vendor sandboxes

**Fakes:** composer/barge-in/clock tests without API keys or 200 ms RTT.  
**Vendor sandboxes:** needed for gateway conformance, not for the kernel.  
**Best:** both. Kernel CI never requires Next AI.

---

## 13. TTS-Engine as Speak vendor vs “library linked in”

**Gateway:** matches product lock; can swap to Next AI TTS without rewriting Talk.  
**Link as a library:** would couple Go TTS into Python/Go orchestrator process, mix GPU/CPU, and destroy swap. TTS-Engine is already a **server** (gRPC :50051). Calling it **is** the vendor pattern.  
**Best:** remote Speak vendor, first-party. **Not:** embed Piper inside aiorchestrator.

---

## Summary table

| Decision | Locked |
|---|---|
| Kernel language | **Go** |
| Process shape | Modular monolith (more instances + WS affinity for live) |
| Extension | Ports + **payment-gateway routers** |
| Control API | HTTP + OpenAPI |
| FS media | Existing WS (edge adapter) |
| VAD | Local ONNX |
| Live scale | Sticky instances |
| Audio bus | **Memory only** |
| Jobs / config / audit | **PostgreSQL** |
| TTS-Engine | Speak **gateway** (gRPC), not embedded |
