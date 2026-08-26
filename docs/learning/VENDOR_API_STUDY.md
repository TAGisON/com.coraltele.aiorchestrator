# Vendor API study notes

**Purpose:** Learn from a **working** voice+AI product’s documentation.  
**Not an integration:** we are not implementing Exotel, and we are not sitting in Next AI’s telephony seat.  
**Source studied (session):** *Exotel Voice Streaming & Voicebot Integration*, AI NXT Technologies, v1.0, August 2026 (local PDF).

---

## Seats (locked in discussion)

- **We** are the client of engine vendors (Next AI, later others): STT, TTS, LLM as **separate services**.  
- **Exotel** in that PDF is a telephony client of a **duplex audio** brain. That is how *they* work; it is not how we consume Next AI.  
- The PDF is a specimen of how a live product specifies **sessions, audio, interruption, and lifecycle**.

---

## What their guide specifies well (steal the discipline)

1. **Audio is a table, not a vibe** — encoding, bit depth, channels, endianness, sample rate, transport wrapping.  
2. **Session id on every message** — correlate chunks to one conversation.  
3. **Lifecycle** — connected / start (metadata + format) / media / stop.  
4. **Two controls besides bytes** — flush unplayed audio (barge-in); mark when playback finished.  
5. **Small media chunks** — large blobs make flush useless.  
6. **Auth and routing as config** — URL, sample-rate query, optional params, Basic or IP allowlist.  
7. **Field tables** — required vs optional, who sends what.

Our **contracts** (`../architecture/CONTRACTS.md`) should be documented with the same strictness when we lock wire formats.

---

## What that PDF does *not* contain (we still need from Next AI)

It does not document:

- STT: voice in → text out (partials/finals)  
- TTS: text in → voice out (cancel/flush)  
- LLM: texts + persona + profile + KB context → response  

Those three are how **we** will call Next AI. Until those service docs exist, we design our contracts; we do not wrap their Voicebot WebSocket as our brain.

---

## How we will use Next AI (when engine docs arrive)

Classify each API as Listen, Speak, or Think. Write three translators. Orchestration, profiles, RAG assembly, rules, feeders stay in `com.coraltele.aiorchestrator`.
