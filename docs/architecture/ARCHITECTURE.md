# Architecture — approach (planning lock)

**Status:** Approach locked for planning. Not an implementation spec (no class list, no codec matrix, no vendor SDK).  
**Date:** 26 August 2026  
**Product parent:** `docs/product/PRODUCT_DECISIONS.md`  
**Contracts:** `docs/architecture/CONTRACTS.md`

If this file and the product lock disagree, **the product lock wins**.

---

## 1. Goal of the architecture

We own **orchestration and control**. Vendors own **engines we can replace**. Edges own **streams we can attach**.

Future integrations (another STT, a meeting file, a mail skill, telephony) must plug in **without rewriting sessions, profiles, or modes**.

---

## 2. Three layers

```
Edges (not this product)          Runtime (this product)           Consumers
─────────────────────            ──────────────────────           ──────────
Feeder: file, WSS, FS, …    →    Session + named profile     →    Call center profile
Sink:   file, WSS, FS, …    ←    Listen / Speak / Talk / Think    Meeting pack, copilot, …
                                 Agent bundle (KB, rules, skills)
                                 Engine translators (STT, LLM, TTS)
```

**Runtime** (`com.coraltele.aiorchestrator`) is a process we install. It does not embed FreeSWITCH, Zoom, or a vendor’s full agent product.

**Edges** only give and take bytes (or files).  
`mod_audio_stream` is the telephony edge: capture + inject. **No OpenAI, no Next AI, no profile inside that module.** The old FreeSWITCH → Realtime bridge is not the architecture of this product.

**Consumers** are verticals: they pick a profile and attach feeders/sinks/skills.

---

## 3. Full control (what we never give away)

| We own | Vendor may see | Vendor must not own |
|---|---|---|
| Session id, profile, clock (live/playback) | Audio or text for **one** call to STT or TTS or LLM | When to listen vs speak vs think |
| When STT / LLM / TTS is invoked | Persona + context **we** put on an LLM request | Our KB as the only copy of truth |
| Rules, skills, memory, analytics | Engine-specific ids (mapped in the translator) | Barge-in policy, handoff, feeders |
| Canonical audio inside the runtime | Their wire format (translator converts) | Product modes |

Next AI (and any later engine vendor) is a **service we call**:

- STT: we give voice → they give text  
- TTS: we give text → they give voice  
- LLM: we give texts + persona + profile + grounding context → they give a response  

We do **not** send them a raw duplex “black box call” as the product path.

---

## 4. Contracts vs translators (the only extension mechanism)

**Contract (ours, stable):** what the runtime needs. See `CONTRACTS.md`.  
**Translator (theirs, disposable):** maps one vendor or one edge onto a contract.

Adding Next AI = one STT translator + one LLM translator + one TTS translator.  
Adding a second STT vendor = a fourth translator, **zero** change to Talk or call center.  
Adding FreeSWITCH = a feeder/sink translator that speaks `mod_audio_stream`’s existing stream dialect.

We never “if Next AI then different session.” We never “if Exotel then different product.” External API docs are used to **write a translator**, or to **learn**, not to reshape the runtime.

Fused speech-to-speech (audio in, audio out, hidden STT+LLM+TTS) is an **optional** extra contract (`bundled Talk`). It is not the default and must not be required for captions, Speak-only, or our RAG-in-the-middle.

---

## 5. Control plane vs data plane

**Control:** create/destroy session, bind profile, attach feeder/sink, inject text, stop Talk, health.  
**Data:** audio frames and/or text streams for that session.

Do not make HTTP POST of live PCM the realtime path. Do not make FreeSWITCH JSON the native platform protocol — that would force every non-call feeder to fake telephony.

---

## 6. Canonical media (direction, not a codec SOW)

Decode / resample at the **edge translator**. Inside the runtime, one canonical PCM family. Encode again at the sink translator.

Published allowlists at the edge; not “every codec on the live path” in V1.

Turn-taking (VAD, barge-in, flush TTS) lives in the **runtime**, not in `mod_audio_stream` and not inside a vendor unless we explicitly call their cancel API from the TTS translator.

---

## 7. Scope for future integrations

Named slots (product §9). A future integration is valid when it fills a slot:

| Slot | First uses (not a closed list) |
|---|---|
| Feeder / sink | File, generic WSS, `mod_audio_stream` |
| STT / LLM / TTS | Next AI first; others behind the same contracts |
| Skill | One audited action in V1 (mail or equivalent is a project choice) |
| Knowledge | Upload / files first; graph later |

Chat as a feeder, two-way interpret, on-prem engines, diarization: **in-category**, not required to specify wire format now.

---

## 8. V1 build order

1. Runtime: session + profile + canonical audio + Listen and Speak against **contracts** (one translator each is enough). File feeder + file sink (proves we are not call-center-only).  
2. Think: LLM translator + rules + document context we attach. LLM off still runs Listen/Speak.  
3. Talk: compose Listen + Think + Speak + barge-in in the runtime.  
4. Telephony: point `mod_audio_stream` at this runtime (feeder translator), not at a vendor.  
5. Further feeders and vendors: more translators only.

Playback jobs never require FreeSWITCH. Live CC never requires engines inside the FS module.

---

## 9. What we are not doing

- Implementing Exotel Voicebot as a product path.  
- Treating a duplex vendor WSS as our STT+LLM+TTS.  
- Putting profile/KB/orchestration in Next AI or OpenAI.  
- Growing `mod_audio_stream` into an AI module.
