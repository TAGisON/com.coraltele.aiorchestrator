# com.coraltele.aiorchestrator

Coral Tele **LLM Call Centre** orchestrator (voicebot on Coral PABX / FreeSWITCH).

Caller audio in → configured **conversation graph** + STT/LLM/TTS → audio / transfer / hangup out.  
Captions, translators, and meeting products are **out of this programme**.

| | |
|---|---|
| **Artifact id** | `com.coraltele.aiorchestrator` |
| **Status** | Architecture restart documented on branch `docs/llm-callcentre-architecture` |
| **Edge** | [`mod_audio_stream-1`](https://github.com/TAGisON/mod_audio_stream-1) — dumb PCM pipe |

## Documents (source of truth)

Start at **[docs/README.md](docs/README.md)**.

| Doc | Role |
|---|---|
| [docs/06_APPLICATION_FLOW.md](docs/06_APPLICATION_FLOW.md) | Whole-application flowcharts |
| [docs/01_VISION_AND_SCOPE.md](docs/01_VISION_AND_SCOPE.md) | V1 scope / feature ticks |
| [docs/02_CURRENT_STATE.md](docs/02_CURRENT_STATE.md) | What we keep vs rebuild |
| [docs/03_BRAIN_AND_GRAPH.md](docs/03_BRAIN_AND_GRAPH.md) | Graph brain (locked) |
| [docs/04_LIVE_TURN_MACHINE.md](docs/04_LIVE_TURN_MACHINE.md) | Live turn machine (locked) |
| [docs/05_MEDIA_AND_VENDORS.md](docs/05_MEDIA_AND_VENDORS.md) | STT / LLM / TTS |
| [AGENTS.md](AGENTS.md) | Short agent entrypoint |

## Hard rules

- Go kernel; no Python/Java media kernel  
- No Kafka/Redis for live PCM  
- Graph is law; tools arm → speak → execute  
- Never commit `.agent/secrets.local.json`  
