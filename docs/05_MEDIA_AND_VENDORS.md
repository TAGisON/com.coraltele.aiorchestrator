# 05 — Media pipe and vendors (STT / LLM / TTS)

## Roles

| Layer | Owns | Must not own |
|---|---|---|
| **Listen (STT)** | Speech → text (+ optional detected language) | Intent, transfer, hangup |
| **Think (LLM)** | Understand utterance **inside current node**; pick allowlisted edge or unclear | Graph topology, dial numbers |
| **Speak (TTS)** | Text → PCM for `active_language` + speaker | Dialogue decisions |
| **Graph + turn machine (ours)** | Cursor, barge, tool lock, repair, transcript policy | Vendor SDK details |
| **Edge (FreeSWITCH + mod)** | PCM up/down, execute transfer/hangup verbs | Product CX |

```text
Caller mic ──► STT ──► Turn machine + Graph (+ LLM)
                              │
                              ▼
Caller ear ◄── TTS ◄── reply text / tool closing line
```

## Vendor policy

- Live Talk requires **Indian multilingual** STT + TTS.
- Vendors are **gateways** behind ports (swappable).
- Current baseline: **Sarvam** (`sarvam-stt`, `sarvam-llm`, `sarvam-tts`).
- Profile language allowlist ⊆ TTS languages ∩ STT languages for bound gateways.
- Voice picker uses a **catalog we maintain** (id, gender, model). Sarvam has no required discover-voices API we depend on; speakers are request parameter `speaker`.

## Sarvam baseline (reference)

### STT — `saaras:v3`

- ~23 languages (22 Indic + English) + `unknown` auto-detect.
- Live path: streaming WebSocket; REST suitable for short offline clips.

### TTS — `bulbul:v3`

- **11 languages:** hi-IN, bn-IN, kn-IN, ml-IN, mr-IN, od-IN, pa-IN, ta-IN, te-IN, en-IN, gu-IN.
- Speakers (docs): male includes shubh (default), aditya, rahul, …; female includes ritu, priya, neha, ishita, …
- Official catalog: [Change the speaker voice](https://docs.sarvam.ai/api/api-guides-tutorials/text-to-speech/how-to/change-the-speaker-voice).
- **Talk allowlist for V1** follows TTS 11 unless another Speak gateway is added (STT-only languages are not live Talk locales).

### LLM

Chat completions behind Think gateway; constrained by graph allowlist.

## Edge

`mod_audio_stream` remains a **dumb pipe**: no product barge logic in the module; orch owns CX and when to arm transfer/hangup.
