# Contracts (ours) — planning lock

**Status:** LOCKED — the *meaning* of each contract. Wire formats are chosen when the first gateway is written.  
**Date:** 27 August 2026  
**Parent:** `ARCHITECTURE.md`  
**Go shapes:** `PORTS.md` (implementation freeze). **Build order:** `PLATFORM_FIRST.md`.

These ports do not change when a vendor changes. A **gateway** maps a vendor or edge API onto one port.

---

## Engine ports

### Listen (STT)

**We give:** audio (live stream or playback blob) at session canonical rate (8000–48000 Hz); optional language hint; `session_id`.  
**We need back:** text (partial and/or final), optional timestamps, optional confidence, optional detected language, end-of-stream, errors.  
**Must work with brain off** (captions, tapes).  
**Capabilities:** `streaming`, `batch`, `partials`, supported input rates.

### Speak (TTS)

**We give:** text (and later SSML if the profile asks), `session_id`, cancel/flush.  
**We need back:** audio at session canonical rate (gateway resamples from vendor format); optional **mark** (utterance fully delivered to sink); errors.  
**Must work with brain off** (read-outs).

**Gateways on this slot are equal.** First-party TTS-Engine, Next AI TTS, Sarvam, etc. each get one gateway. Wire quirks (PCMU 8 kHz for PSTN, gRPC vs HTTP, first-chunk latency) stay **inside** that gateway. The composer only sees canonical audio + cancel/flush + optional mark.

### Think (LLM)

**We give:** messages/text, persona, profile-derived instructions, **grounding context we assembled**, tool/skill descriptors we allow.  
**We need back:** response text (optionally streamed), optional structured skill proposal, errors.  
**Must not** be the source of truth for policy; we attach or withhold KB. Rules in the runtime may discard or override the response.

### Translate (MT)

**We give:** text, source/target language codes, `session_id`.  
**We need back:** translated text (stream or batch), errors.  
**Used when** profile `language.behaviour` is one-way or two-way interpret.

---

## Attach ports (edges)

### Feeder

**They give us:** audio and/or text, plus start metadata (peer sample rate, their stream id).  
**We need:** start, frames or chunks resampled to session canonical rate, stop/error. Optional DTMF or similar as events, not as engines.

### Sink

**We give them:** audio at session canonical rate (edge encodes to peer rate) and/or text, plus stop.  
**We need:** ability to **flush** unplayed audio (barge-in) and to know **playback finished** when Talk needs it.

`mod_audio_stream` implements feeder+sink for FreeSWITCH only. Its JSON/PCM dialect stays **inside that edge** (`EDGE_FS.md`).

---

## Knowledge and Skill ports

### Knowledge

**We give:** query text, profile collection ids, `session_id`.  
**We need back:** snippets + metadata | no-hit. Gateways: ingest, http_kb, hybrid, graph (later).

### Skill

**We give:** skill name, validated args, `session_id`.  
**We need back:** result | error. Side-effecting skills: at most one successful act per invocation; no blind retry.

---

## Optional extra (not default)

### Bundled Talk

Audio in → vendor audio out, hidden STT+LLM+TTS. Allowed only as an optional gateway. **Cannot** satisfy Listen-only, Speak-only, or Think with our RAG in the middle. Do not use this as the Next AI path; Next AI is separate engine gateways.

---

## Control (runtime API)

Create session, bind profile, attach feeder/sink, inject text, stop, health, subscribe events. Independent of vendor SDKs. OpenAPI published for Coral Java.

---

## Mapping rule

A **router** sits on each engine port (Listen / Think / Speak / Translate / Knowledge / Skill). The composer calls the port; the router picks the **active gateway** from the profile (payment-gateway style) plus failover and capability filters. A **gateway** maps one vendor’s API onto the port.

When we receive a vendor’s API documentation we **classify** it onto a port, then add a gateway. We do not change Talk.
