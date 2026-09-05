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
8. `docs/07_PLANNING_STANDARDS.md` — planning layers + phase template (**planning only**)  
9. `docs/08_PURGE_AND_SCHEMA_PHASES.md` — P1 purge / P2 schema (**ODs settled**; planning only)  
10. `docs/09_EVIDENCE_AND_RECORDING.md` — transcript / audit / recording behaviour (**Draft**)  
11. `docs/10_CODING_PRINCIPLES.md` — coding rules + EC-* library (**Draft**)  
12. `docs/11_CI_AND_CD.md` — CI/CD plan (**Draft**)  
13. `docs/12_AGENTIC_L4_ROLES.md` — L4 roles + skills  
14. `docs/13_PRODUCTION_CONSOLES.md` — Admin / Supervisor / User chat (**Locked**)  
15. `docs/phases/` — L3 specs (kernel Closed; **U.0–U.2**, **A.1–A.2** Closed; A.3 next)  
16. `.cursor/skills/aiorchestrator-l4-{implementer,reviewer,summarizer}/` — L4 skills

Older product/architecture docs were removed on branch `docs/llm-callcentre-architecture`. Do not resurrect caption/translator/meeting platform scope into this programme. Do not restore purged desk consoles — rebuild only under doc 13.

**Implementation gate:** Console L4 requires a Closed/Ready L3 phase id under doc 13. Kernel L4 already authorized for prior waves. Plans must cite architecture refs; do not invent against 01–06 / 13.

## Hard rules

- Go kernel; no Python/Java media kernel  
- No Kafka/Redis for live PCM  
- Vendors behind ports/gateways; FS mod is dumb PCM pipe  
- Never commit `.agent/secrets.local.json`  
- Graph is law; LLM only allowlisted edges; tools arm→speak→exec  

## Lab

Orchestrator control UI typically http://127.0.0.1:8011/ when running.
