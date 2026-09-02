# FreeSWITCH edge — mod_audio_stream architecture

**Status:** LOCKED  
**Date:** 27 August 2026  
**Reference implementation:** sibling repo [`mod_audio_stream-1`](https://github.com/TAGisON/mod_audio_stream-1) (C module + WS dialect; clone next to this repo, not as a submodule)

The FS edge is **`internal/edge/modaudiostream`**. It implements Feeder + Sink ports. FS JSON/binary dialect never leaks past this package.

---

## 1. Connection model

```
Dialplan: uuid_audio_stream <fs-call-uuid> start wss://orch-host/edge/fs?token=<signed>
       │
       ▼
Orchestrator WSS endpoint
  → validate token (tenant, profile or session_id, expiry)
  → optional IP allowlist (tenant/deploy config)
  → attach to session (create if WS-first pattern)
  → Running
```

### Auth (locked)

| Mechanism | Use |
|---|---|
| **Signed token in WSS URL** | HMAC or JWT: `tenant_id`, `profile_id` or `session_id`, `exp` (~5 min). Issued by control API or Coral Java before answer. |
| **IP allowlist** | Optional per-tenant; FS hosts only. |
| **Session binding** | Token’s `session_id` must match; one WS per live audio session. |

No anonymous FS connections in production.

### Operational patterns (choose per deploy; both valid)

1. **Control-first:** Coral Java `POST /sessions` → returns `session_id` + edge token → dialplan uses token.  
2. **WS-first:** FS connects with profile token → edge creates session and pins profile on first frame.

Profile version must be pinned before **Running**.

---

## 2. Wire protocol (unchanged from C module)

### Inbound (FS → orchestrator)

- Binary frames: PCM s16le, sample rate declared at stream start (often 8000 or 16000 on dialplan).  
- Optional JSON events: DTMF, stop (per module capabilities).

### Outbound (orchestrator → FS)

JSON `streamAudio` inject (schema preserved):

```json
{
  "type": "streamAudio",
  "data": {
    "audioDataType": "raw",
    "sampleRate": 8000,
    "channels": 1,
    "audioData": "<base64 pcm>"
  }
}
```

Edge sets `sampleRate` to the **sink peer rate** (from FS attach metadata), resampling from session canonical PCM.

### Framing

~**20 ms** inject cadence aligned with module playout loop. Frame size at peer rate: `peer_rate × 2 × 0.02` bytes.

**Pacing is a hard contract, not an optimisation.** The module's inject buffer is a 500 ms
jitter buffer that drops the *oldest* audio on overflow. Speak produces a whole utterance at
once; the edge queues it and releases exactly one frame per 20 ms tick. If the edge ever
bursts — because its write loop stalled and then caught up — the module discards the excess
and the caller hears a jump mid-sentence, with no error anywhere in the orchestrator.

### Call control (module 2.1.0)

Sinks bound to a real telephony leg implement `port.CallControl`:

| Method | Wire | Behaviour |
|---|---|---|
| `Hangup(ctx, cause)` | `{"type":"hangup","cause":…,"drainMs":…}` | Release once playout drains |
| `Transfer(ctx, req)` | `{"type":"transfer","dest":…,"dialplan":"XML","context":"calltransfer","drainMs":…}` | Blind transfer once playout drains |

Both are **armed, not immediate** — the module plays out what is still queued first, so a
closing prompt or "connecting you now" is never truncated. Only one action is accepted per
connection; a second returns an error rather than racing the first.

`Transfer` is the in-process equivalent of
`uuid_transfer <call-uuid> <dest> <dialplan> <context>`.

Sinks that are not a call leg (file, browser, tests) simply do not implement the interface,
so callers must type-assert.

---

## 3. Resampling

| Direction | Action |
|---|---|
| FS → bus | Feeder edge: peer rate → session `canonical_sample_rate_hz` (Speex or equivalent) |
| Bus → FS | Sink edge: canonical → peer rate for `streamAudio` |
| Speak gateway | Vendor format → canonical (inside gateway); edge does not know PCMU vs PCM |

Profile declares canonical rate (8–48 kHz). PSTN deployments commonly use **8000 Hz** canonical or 16 kHz canonical with 8 kHz edge — both supported.

---

## 4. DTMF and events

DTMF digits arrive as **feeder events** on the session bus (not Listen input). Composer/playbook may consume them for IVR-style confirmation (`confirm: true` skills).

---

## 5. Failure

| Event | Behaviour |
|---|---|
| WS disconnect | Feeder gone → session **Terminal** + audit |
| Inject backpressure | Cap queued PCM; drop oldest unplayed per profile or pause Speak |
| Auth failure | Close WS; no session created |
| Listen consumer behind | **Drop the uplink frame**, count it, never block the reader |
| Downlink write error | Count it, release the mark, log; close the leg after 5 consecutive |
| Pipeline failure (engine down / no credits / timeout) | Play the operator fallback prompt, then `hangup` |

### Why uplink drops instead of blocking

`mod_audio_stream` links `libwsc`, which services **send and receive on a single libevent
thread**. If the orchestrator stops reading the socket, TCP back-pressure blocks the
module's uplink send, which blocks that one thread, which also stops it delivering
downlink audio. The call then freezes in both directions and floods on recovery,
overflowing the 500 ms inject buffer and permanently losing agent speech.

So the edge read loop must never block: `onPCM` drops the frame and increments
`uplink_dropped_frames` instead. One lost 20 ms uplink frame is far cheaper than a
multi-second bidirectional stall.

### Why every popped frame releases its mark

`WaitMark` is what Speak (and the fallback path) use to know playout finished. Every frame
removed from the queue must release its pending count whether the write succeeded or not —
otherwise a single write error leaves `WaitMark` blocked until its context expires, and the
hangup that should follow a closing prompt never fires.

---

## 6. Explicit non-goals

- Orchestrator does not speak SIP/RTP.  
- Do not change C module JSON schema without a coordinated module release.  
- Generic platform WS feeder is a **separate** edge package (`internal/edge/ws`); it does not replace FS dialect.

---

## 7. Live talk answer gate (Contact Desk)

FS Lua (`../mod_audio_stream-1/fs/ai_voice_bot.lua` in the sibling repo) calls `POST /v1/sessions/{id}/answer` after a short media settle (`session:sleep`, ~1500 ms) once `uuid_audio_stream` is up — do **not** poll `media_phase` from Lua (curl keep-alive can stall on voip) and do **not** `uuid_broadcast silence_stream` (blocks Lua until hangup). Full phase machine: `docs/product/LIVE_TALK_CX_AND_INDIA_LANGUAGE.md` §2–§3.
