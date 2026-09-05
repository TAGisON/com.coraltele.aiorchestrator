# Coral LLM Call Centre — documentation

**Branch purpose:** Greenfield architecture for an **LLM-based voice call centre** on Coral PABX / FreeSWITCH.  
**Status:** Architecture locked (01–06). Planning 07–12 Locked. **Doc 13 Locked**. Kernel G.* Closed; U.0–U.2 Closed; Admin A.1–A.6 Closed; **Chat C.1–C.4 Closed**; next S.1 Supervisor.

## Source of truth (read in order)

| # | Document | Contents |
|---|---|---|
| 1 | [01_VISION_AND_SCOPE.md](./01_VISION_AND_SCOPE.md) | What we are building; V1 ticks from the Next-Gen CC feature list; out of scope |
| 2 | [02_CURRENT_STATE.md](./02_CURRENT_STATE.md) | What already works vs what we discard architecturally |
| 3 | [03_BRAIN_AND_GRAPH.md](./03_BRAIN_AND_GRAPH.md) | Conversation graph: nodes, edges, tools, repair, bindings |
| 4 | [04_LIVE_TURN_MACHINE.md](./04_LIVE_TURN_MACHINE.md) | Speaking / listening / thinking / tool lock; actionable vs transcript-only STT |
| 5 | [05_MEDIA_AND_VENDORS.md](./05_MEDIA_AND_VENDORS.md) | STT / LLM / TTS roles; Sarvam capability baseline |
| 6 | [06_APPLICATION_FLOW.md](./06_APPLICATION_FLOW.md) | **Whole-application flowcharts** (start here for the big picture) |
| 7 | [07_PLANNING_STANDARDS.md](./07_PLANNING_STANDARDS.md) | Planning layers, phase template, reference discipline, agentic gate |
| 8 | [08_PURGE_AND_SCHEMA_PHASES.md](./08_PURGE_AND_SCHEMA_PHASES.md) | P1 purge + P2 schema catalog (**ODs settled**) |
| 9 | [09_EVIDENCE_AND_RECORDING.md](./09_EVIDENCE_AND_RECORDING.md) | Transcript / audit / recording behaviour |
| 10 | [10_CODING_PRINCIPLES.md](./10_CODING_PRINCIPLES.md) | Coding rules + EC-* edge-case library |
| 11 | [11_CI_AND_CD.md](./11_CI_AND_CD.md) | CI jobs + lab promote CD |
| 12 | [12_AGENTIC_L4_ROLES.md](./12_AGENTIC_L4_ROLES.md) | Implementer / Reviewer / Summarizer |
| 13 | [13_PRODUCTION_CONSOLES.md](./13_PRODUCTION_CONSOLES.md) | Admin / Supervisor / User chat (**Locked**) |
| — | [phases/](./phases/README.md) | L3 catalogs (P/M/E/CI/G/L + **U/A/C/S/V** consoles) |

## Hard engineering rules (unchanged)

- Orchestrator kernel is **Go** only (no Python/Java media kernel).
- Live PCM stays **in-memory** on the session path (no Kafka/Redis for audio).
- Vendors sit behind **ports / gateways**; FreeSWITCH `mod_audio_stream` is a **dumb PCM pipe**.
- Never commit secrets (e.g. `.agent/secrets.local.json`).

## Lab (runtime)

Default control UI: http://127.0.0.1:8011/ (when orch is running).  
Edge: FreeSWITCH + `mod_audio_stream` (sibling repo). Details of lab ops will be re-added when implementation restarts.
