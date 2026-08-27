# Build notes (aligned to locked architecture)

See **`ARCHITECTURE.md`** for the approach. This file is only Coral fit and latency numbers we already measured.

**Kernel: Go.** `mod_audio_stream-1/Ai_code` is a **reference** for FS WS + 20 ms inject, not the codebase we extend.

**FS:** `uuid_audio_stream` URL → this Go service’s edge WebSocket; JSON schema unchanged. Auth: signed token in URL — see `EDGE_FS.md`.

**Audio rate:** profile-configurable **8000–48000 Hz** canonical; edges and gateways resample. Default contact-center: 16000 Hz; PSTN-heavy: 8000 Hz.

**Speak:** TTS-Engine is a **gateway** on the Speak router (gRPC, PCMU 8 kHz converted there). Next AI TTS is another gateway on the **same** router.

**Latency (live Talk):** p50 &lt; ~1.2 s; POC ~0.8–1.0 s. No broker on PCM. Local VAD. Stream STT/LLM/TTS. TTS-Engine first chunk ~50 ms Piper.

**Coral Java:** HTTP control + skill HTTP. Transfer stays FS/Coral. After-call: playback **job** in Postgres (file URI), not a live WS.

**Audio plane:** in-memory channels only. **Jobs/profiles/audit:** PostgreSQL.
