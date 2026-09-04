# Coral LLM Call Centre — documentation

**Branch purpose:** Greenfield architecture for an **LLM-based voice call centre** on Coral PABX / FreeSWITCH.  
**Status:** Architecture planning (brain + flow locked in prose). JSON schema deferred.

## Source of truth (read in order)

| # | Document | Contents |
|---|---|---|
| 1 | [01_VISION_AND_SCOPE.md](./01_VISION_AND_SCOPE.md) | What we are building; V1 ticks from the Next-Gen CC feature list; out of scope |
| 2 | [02_CURRENT_STATE.md](./02_CURRENT_STATE.md) | What already works vs what we discard architecturally |
| 3 | [03_BRAIN_AND_GRAPH.md](./03_BRAIN_AND_GRAPH.md) | Conversation graph: nodes, edges, tools, repair, bindings |
| 4 | [04_LIVE_TURN_MACHINE.md](./04_LIVE_TURN_MACHINE.md) | Speaking / listening / thinking / tool lock; actionable vs transcript-only STT |
| 5 | [05_MEDIA_AND_VENDORS.md](./05_MEDIA_AND_VENDORS.md) | STT / LLM / TTS roles; Sarvam capability baseline |
| 6 | [06_APPLICATION_FLOW.md](./06_APPLICATION_FLOW.md) | **Whole-application flowcharts** (start here for the big picture) |

## Hard engineering rules (unchanged)

- Orchestrator kernel is **Go** only (no Python/Java media kernel).
- Live PCM stays **in-memory** on the session path (no Kafka/Redis for audio).
- Vendors sit behind **ports / gateways**; FreeSWITCH `mod_audio_stream` is a **dumb PCM pipe**.
- Never commit secrets (e.g. `.agent/secrets.local.json`).

## Lab (runtime)

Default control UI: http://127.0.0.1:8011/ (when orch is running).  
Edge: FreeSWITCH + `mod_audio_stream` (sibling repo). Details of lab ops will be re-added when implementation restarts.
