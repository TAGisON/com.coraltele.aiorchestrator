# Contracts (ours) — planning lock

**Status:** Planning lock — the *meaning* of each contract. Wire formats are chosen when the first translator is written.  
**Date:** 26 August 2026  
**Parent:** `ARCHITECTURE.md`

These sockets do not change when a vendor changes. A translator maps a vendor or edge API onto one socket.

---

## Engine contracts

### Listen (STT)

**We give:** audio (live stream or playback blob), language hint if the profile has one, session correlation id.  
**We need back:** text (partial and/or final), optional timestamps, optional confidence, end-of-stream, errors.  
**Must work with brain off** (captions, tapes).

### Speak (TTS)

**We give:** text (and later SSML if the profile asks), session correlation id, cancel/flush.  
**We need back:** audio (stream or blob) in a declared format, optional “utterance finished” (mark), errors.  
**Must work with brain off** (read-outs).

### Think (LLM)

**We give:** messages/text, persona, profile-derived instructions, **grounding context we assembled**, tool/skill descriptors we allow.  
**We need back:** response text (optionally streamed), optional structured fields, errors.  
**Must not** be the source of truth for policy; we attach or withhold KB. Rules in the runtime may discard or override the response.

---

## Attach contracts (edges)

### Feeder

**They give us:** audio and/or text, plus start metadata (format, their stream id).  
**We need:** start, frames or chunks, stop/error. Optional DTMF or similar as events, not as engines.

### Sink

**We give them:** audio and/or text, plus stop.  
**We need:** ability to **flush** unplayed audio (barge-in) and to know **playback finished** when Talk needs it.

`mod_audio_stream` implements feeder+sink for FreeSWITCH only. Its JSON/PCM dialect stays **inside that translator**.

---

## Optional extra (not default)

### Bundled Talk

Audio in → vendor audio out, hidden STT+LLM+TTS. Allowed only as an optional translator. **Cannot** satisfy Listen-only, Speak-only, or Think with our RAG in the middle. Do not use this as the Next AI path; Next AI is three engine contracts.

---

## Control (runtime API, later)

Create session, bind profile, attach feeder/sink, inject text, stop, health. Independent of vendor SDKs.

---

## Mapping rule

When we receive a vendor’s API documentation we **classify** it: Listen, Speak, Think, Feeder, Sink, Skill, Knowledge, or Bundled Talk. Then we write a translator. If it does not fit a contract, we do not bend the product — we reject the API or we extend **contracts** (and product lock) on purpose.
