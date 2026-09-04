# LLM Call Centre — agent notes

## Source of truth

Read in order under `docs/`:

1. `docs/README.md` — index  
2. `docs/01_VISION_AND_SCOPE.md` — V1 scope  
3. `docs/02_CURRENT_STATE.md` — keep vs rebuild  
4. `docs/03_BRAIN_AND_GRAPH.md` — graph brain (**locked**)  
5. `docs/04_LIVE_TURN_MACHINE.md` — live states (**locked**)  
6. `docs/05_MEDIA_AND_VENDORS.md` — STT/LLM/TTS  
7. `docs/06_APPLICATION_FLOW.md` — **whole-application flowcharts**

Older product/architecture docs were removed on branch `docs/llm-callcentre-architecture`. Do not resurrect caption/translator/meeting platform scope into this programme.

## Hard rules

- Go kernel; no Python/Java media kernel  
- No Kafka/Redis for live PCM  
- Vendors behind ports/gateways; FS mod is dumb PCM pipe  
- Never commit `.agent/secrets.local.json`  
- Graph is law; LLM only allowlisted edges; tools arm→speak→exec  

## Lab

Orchestrator control UI typically http://127.0.0.1:8011/ when running.
