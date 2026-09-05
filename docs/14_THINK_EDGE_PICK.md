# 14 — Think edge pick (LLM allowlisted edges)

**Status: Locked** (planning) — 2026-09-05  
**Parent:** [03_BRAIN_AND_GRAPH.md](./03_BRAIN_AND_GRAPH.md), [04_LIVE_TURN_MACHINE.md](./04_LIVE_TURN_MACHINE.md), [05_MEDIA_AND_VENDORS.md](./05_MEDIA_AND_VENDORS.md), [10_CODING_PRINCIPLES.md](./10_CODING_PRINCIPLES.md) EC-20–27  
**Implements gap:** Docs already require “LLM proposes only legal edges”; live graph path still uses substring `matchChoice`.

## Goal

Replace listen-node **word-guess** with **Sarvam Think** selecting one **allowlisted** `edge_id` (or unclear/retry), without breaking the graph cage, tools, or turn machine.

## In scope

- Cursor allowlist + `TakeEdgeID` for listen nodes  
- Strict JSON Think classifier behind `port.Think`  
- Wire `Talk.runGraphTurn` to classifier  
- Audit of proposal / accept / reject / think_error  
- Tests + lab soak for Hinglish and cage breaks  
- Desk pins remain **Sarvam** engines; fakes stay in registry for CI only  

## Out of scope

- Free-flow LLM answers / generative FAQ  
- Emotion routing  
- Removing fake gateways from the binary  
- Chat telephony transfer (no leg — channel limit)  
- Full native TTS copy for every Indian locale  

## Open decisions

None — settled for V1 slice:

1. Think fail / timeout / illegal id → **unclear → node repair** (no keyword fallback).  
2. LLM cannot select `kind=repair` edges (kernel-owned).  
3. Fakes remain registered; desk profile/engines pin Sarvam only.

## Phase breakdown (L3)

| id | Goal | Depends |
|---|---|---|
| [T.0](./phases/T.0_think_edge_inventory.md) | Inventory + freeze cage contract | Doc 14 Locked |
| [T.1](./phases/T.1_cursor_allowlist_take.md) | Allowlist + TakeEdgeID + EC-23 | T.0 |
| [T.2](./phases/T.2_edgepick_classifier.md) | Think JSON classifier package | T.1 |
| [T.3](./phases/T.3_talk_wire_graph.md) | `runGraphTurn` uses classifier | T.2 |
| [T.4](./phases/T.4_edgepick_evidence.md) | Audit / turn evidence for picks | T.3 |
| [T.5](./phases/T.5_think_edge_soak.md) | Lab soak checklist + sign-off | T.4 |

## Anti-patterns

- Reintroducing `matchChoice` on production Talk path  
- Letting Think return caller-facing prose as the spoken answer  
- Passing matrix dial numbers into the Think prompt  
- Unregistering `fake-*` adapters to “force” real vendors  

## Handoff to L3 / L4

Implement **one** Ready/Locked L3 id at a time via `aiorchestrator-l4-implementer`. No L4 until T.0 Closed and the next phase status is Ready.
