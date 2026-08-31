# Runtime — deep architecture

**Status:** LOCKED depth. Complements `ARCHITECTURE.md`. Does not override `docs/product/PRODUCT_DECISIONS.md`.  
**Date:** 27 August 2026

This file is how the orchestrator **runs**. Contracts stay in `CONTRACTS.md`. Product stays in the product lock.

---

## 1. Objects (do not collapse these)

| Object | What it is | Lifetime |
|---|---|---|
| **Profile** | Named, versioned configuration: modes, clock allowed, gateways by router, KB ids, rules, skills, language behaviour, memory policy, canonical audio rate | Until someone publishes a new version |
| **Session** | One running job: profile version **pinned at start**, attachments, turn state, transcript buffer, analytics | Until stop / feeder end / failure policy |
| **Attachment** | One feeder or one sink bound to a session | Subset of session |
| **Turn** | One user utterance → (optional) think → (optional) speak, or one playback pass | Inside a live Talk session |
| **Gateway** | Implementation of exactly one port for one vendor or one edge dialect | Process lifetime; chosen per session from profile |
| **Edge** | Feeder/sink gateway bound to a transport (`mod_audio_stream`, file, WS) | Same as gateway; lives under `internal/edge/` |

A profile is not a process. A session is not a vendor connection. A vendor connection is not a session (one session may open Listen, Think, and Speak connections as the turn machine requires).

**Pin the profile version at session start.** Mid-session edits apply only to fields in `hot_swap_allowed` (language switch, skill unlock after verify). Money, PII, and escalation rules do not hot-swap on a live Talk.

---

## 2. Why the product is a bus, not a pipe

STT → LLM → TTS is a **Talk template**, not the runtime kernel. If the kernel is that chain, Listen-only and Speak-only and “LLM off” are hacks.

Kernel:

```
                    ┌─────────── control plane (session API) ───────────┐
                    │  create / attach / inject text / stop / health     │
                    └────────────────────┬──────────────────────────────┘
                                         │
   feeder edge → normalize →  SESSION BUS (session canonical PCM rate)
                                         │
              ┌──────────────────────────┼──────────────────────────┐
              ▼                          ▼                          ▼
         audio tap                   text tap                    events
         (VAD, Listen)               (Think, captions sink)      (DTMF, stop)
              │                          │
              └──────── Talk composer ───┘
                         │
                         ▼
                    Speak tap → sink edge
```

**Modes are subscribers.**  
Listen on: audio tap → Listen router → text events.  
Think on: text events (finals, or a whole playback transcript) → playbook (optional) → grounding → rules → Think router → response events.  
Speak on: response events or injected text → Speak router → sink.  
Talk on: composer + VAD + barge-in **uses** the three taps; it is not a fourth engine.

Language behaviour uses the **Translate router** on the text path when profile requires it — not a second kernel.

---

## 3. Two clocks (never one loop)

| | **Live** | **Playback** |
|---|---|---|
| Input | Unbounded, jitter, loss | Finite (file or captured blob) |
| Feeder | Jitter buffer, resample to canonical rate | Read at our pace (faster than real time allowed) |
| Sink | **Paced** to sample clock (or the edge paces, e.g. FS 20 ms) | Write file / dump; pacing optional |
| VAD / barge-in | Required for Talk | Off unless we simulate |
| Engine class | Prefer streaming Listen/Speak | Batch Listen/Speak allowed |
| End | Feeder stop, control stop, policy | Input exhausted + Think/Speak finished |

A session declares **one** clock at create. Mixing live audio with a batch LLM on every 20 ms frame is how you get dead air and cost explosions.

**Capability gate at start:** if clock=live and Listen is on, the chosen Listen gateway must advertise `streaming`. If it only has batch, refuse the session (profile invalid for live).

---

## 4. Canonical audio rate (session-scoped)

Each session runs at **`profile.audio.canonical_sample_rate_hz`** (8000–48000), pinned at create. See `PROFILE_SCHEMA.md` §2.

- Bus frames: s16le mono, ~20 ms, size derived from rate.  
- Feeder/sink edges resample to/from peer rate (FS often 8 kHz; meetings may use 16–48 kHz).  
- Speak/Listen gateways resample to/from canonical inside the gateway package.

---

## 5. Session lifecycle

```
Created → Attached (feeder and/or sink as the profile needs)
       → Running
       → Draining (stop requested: finish current Speak or drop per profile)
       → Terminal (Completed | Cancelled | Failed)
```

**Create** needs: `profile_id` + `profile_version` (or “latest” resolved then pinned), `clock`, tenant, canonical rate from profile.  
**Attach** may happen at create or immediately after. A captions-only live session needs feeder + text sink (or control-plane SSE). A Speak-only session needs text in (inject or feeder) + audio sink. A Talk live session needs audio feeder + audio sink.

**Our `session_id`** is the only id the rest of Coral/analytics should store. Feeder ids (FS uuid) and vendor ids live in gateway session maps.

---

## 6. Talk composer — turn machine

Only when profile has Talk (or live Listen+Think+Speak used as conversation). Listen-only never enters this machine.

```
        ┌─────────────┐
        │  Listening  │◄──────────────────────────────┐
        └──────┬──────┘                               │
               │ VAD speech / energy                  │
               ▼                                      │
        ┌─────────────┐     barge-in (speech while    │
        │  Capturing  │     Speaking)                 │
        └──────┬──────┘                               │
               │ endpoint (silence, STT final,        │
               │ max utterance)                       │
               ▼                                      │
        ┌─────────────┐     barge-in cancel           │
        │  Thinking   │─────────────────────────────►│
        └──────┬──────┘                               │
               │ response text (after rules)          │
               ▼                                      │
        ┌─────────────┐     flush sink + cancel Speak │
        │  Speaking   │───────────────────────────────┘
        └──────┬──────┘
               │ mark / playout complete
               ▼
             Listening
```

**Endpointing is ours.** The Listen gateway may emit `final`; VAD may emit “silence for N ms.” The composer decides the turn, not Next AI.

**Barge-in (ours):**

1. VAD (or STT partials that look like speech) while state=Speaking.  
2. Sink **flush** (unplayed audio gone).  
3. Speak gateway **cancel**.  
4. Think gateway **cancel** if still Thinking.  
5. State → Capturing (new user audio). Do not wait for a vendor mark.

If the Speak vendor cannot cancel, the gateway still must stop **delivering** frames to the sink; leftover vendor audio is discarded. The sink flush is what the human hears.

**Speculative Speak** (start TTS on first sentence of streamed LLM): composer optimization when implemented; barge-in must still work.

---

## 7. Think path (where the agent actually is)

Order is locked by product: **rules > skills > grounding > free LLM**. Details: `RULES_AND_SKILLS.md`.

```
inbound text (STT final, inject, or playback transcript)
    → session memory (append user)
    → PII redact for model/logs (policy)
    → playbook step (if profile uses playbook grounding)
    → retrieve grounding (Knowledge router: docs / template / graph)
    → if grounded profile and no hit → refuse or escalate (no LLM invent)
    → rules pre-check (may block Think entirely)
    → Think gateway (persona + instructions + memory window + retrieved chunks + allowed skill descriptors)
    → rules post-check (strip, rewrite, force escalate)
    → if act + skill → authority + confirmation + audit → Skill router
    → session memory (append assistant)
    → Translate router (if language behaviour requires)
    → emit response text (to Speak and/or text sink)
```

**We assemble the LLM payload.** The vendor does not hold our KB. Chunks in the request are copies for that call.

Skills are not LLM magic: the model may *propose* a skill; the runtime *executes* only if the profile allows it and rules pass. Inform vs decide vs act is enforced here.

Playback Think: often one shot over the full transcript + template (MoM). No turn machine. Same Think path, different trigger (input exhausted).

---

## 8. Gateways — construction rules

A gateway:

- Implements **one** port contract.  
- Converts canonical audio/text ↔ vendor wire.  
- Maps `session_id` ↔ vendor request/stream ids.  
- Surfaces vendor errors as **our** error codes (timeout, auth, rate-limit, bad-audio, cancelled).  
- Declares **capabilities**: `streaming`, `batch`, `partials`, `cancel`, `ssml`, supported sample rates (8000–48000).  
- Holds **secrets** from our vault, never from profile JSON.

A gateway must not: choose the next mode, run RAG, own barge-in policy, or read the whole profile.

**Next AI** = separate **gateways** (Listen, Think, Speak, Translate), registered on routers. Duplex WSS if they only offer it = Bundled Talk, not the default Talk path.

**TTS-Engine** = one Speak **gateway** (first-party, same router as Next AI TTS). PCMU 8 kHz / gRPC stay inside that gateway.

**`mod_audio_stream` edge** = feeder + sink in `internal/edge/modaudiostream`. Dialect (binary PCM + `streamAudio` JSON) stays inside. See `EDGE_FS.md`.

---

## 9. Language behaviour

| Profile setting | Runtime |
|---|---|
| `auto_detect` | First confident Listen final → lock `session.detected_language` + `active_language` (see `docs/product/LANGUAGE_POLICY.md`); ambient re-detect ignored after lock |
| `mid_call_switch` | Control API hot field (`PATCH …/profile-fields` `language.primary`) → set `active_language` → restart Listen with new hint → flush partial STT |
| `one_way` | Translate router on outbound text (captions or pre-Speak) |
| `two_way` | Dual attachments; parallel Listen/Speak/Translate chains per leg |

Think + Speak consume `active_language` after lock. Contact Agent defaults: `LANGUAGE_POLICY.md`.

---

## 10. Attachments: N, not 1+1 forever

Typical sessions: one audio feeder and one audio sink. The **model** is a list of attachments so later we can do:

- Two-way interpret: two audio pairs, two languages, two Speak paths, **no** shared Think copilot unless the profile says so.  
- Captions + audio: audio sink + text sink.  
- Supervisor later: extra sink (listen/whisper) as another attachment, not a fork of the kernel.

Do not hard-code “the” feeder in the session struct.

---

## 11. Failure and degradation (industrial)

Every hop has a **budget** (timeout). On expiry: fail that hop, not the process.

| Failure | Live Talk default (profile may override) |
|---|---|
| Feeder gone | Terminal; analytics |
| Listen down | No dead air: canned audio or text apology + optional transfer skill |
| Think down | Profile `fallback.think_down`: canned clip + escalate skill when session engines are pinned (`gateway_binding`); do not improvise from empty or hop Think vendors mid-session (cc-4) |
| Speak down | Skip Speak; push text sink if any; or canned clip |
| Skill down | Fail the act; do not retry side effects blindly |
| Partial STT only | Do not Think on garbage; wait for final or timeout the turn |

Graceful fallback is a **product must**. The composer owns it, not the gateway.

---

## 12. Isolation and scale

One process, **N sessions** (in-process gateways).

- No shared Listen websocket across sessions.  
- Backpressure: cap queued TTS PCM per session; if full, pause feeding LLM tokens or drop oldest *unplayed* (never drop already handed to a live sink without flush policy).  
- Limit concurrent sessions per tenant (`OPERATIONS.md`).  
- Later: out-of-process GPU Listen without changing ports.

---

## 13. Observability

One **trace per turn** (or per playback job): retrieve, listen, think, speak, skill, translate. Spans named by port, not by vendor (vendor is an attribute).

Counters: ran / failed / no-grounding-hit / handoff / barge-in / hop latency / hop cost if the gateway reports usage. Export: `ANALYTICS_AND_POSTCALL.md`.

Logs never store raw PII if the profile says redact; redact **before** Think and **before** persist.

---

## 14. Control vs data (shape, not a framework)

**Control** (reliable, small): create session, attach, inject text, stop, get status, subscribe to captions/events (SSE).  
**Data** (lossy-tolerant, large): audio frames on WSS.

Live audio is not REST POST of a wav every 20 ms. Playback **may** be upload-then-job (file feeder).

The platform-native data dialect is **ours**. FS and files adapt to it.

---

## 15. Config vs runtime

| Store | Contents |
|---|---|
| Profile registry | Versioned profiles, router → gateway ids, canonical audio rate |
| Knowledge store | Documents, templates; graphs later |
| Secret store | Vendor API keys |
| Memory store | Session buffer (hot); customer memory (PG, consent + TTL when enabled) |
| Runtime | Only live sessions and maps |

Gateways register in the process (plugin table). Profiles pick them by id (`listen: nextai`, `speak: tts-engine`). Swapping vendor = profile change + gateway present, not a code change in the composer.

---

## 16. Companion docs (implementation detail lives here)

| Topic | Doc |
|---|---|
| Profile JSON schema, audio rates | `PROFILE_SCHEMA.md` |
| Rules, skills, playbooks, warm transfer | `RULES_AND_SKILLS.md` |
| FS WSS auth, resampling | `EDGE_FS.md` |
| Analytics, post-call, captions SSE | `ANALYTICS_AND_POSTCALL.md` |
| Deploy, drain, limits | `OPERATIONS.md` |
| Next AI wire formats | Open until engine docs arrive; ports unchanged |

Those are choices **under** this runtime. They must not invent a second orchestration path.
