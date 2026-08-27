# FreeSWITCH edge — mod_audio_stream architecture

**Status:** LOCKED  
**Date:** 27 August 2026  
**Reference implementation:** `mod_audio_stream-1` (C module + WS dialect)

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

---

## 6. Explicit non-goals

- Orchestrator does not speak SIP/RTP.  
- Do not change C module JSON schema without a coordinated module release.  
- Generic platform WS feeder is a **separate** edge package (`internal/edge/ws`); it does not replace FS dialect.
