# L3 — T.0 Think edge pick inventory

| Field | Value |
|---|---|
| **id** | `T.0` |
| **title** | Think edge pick inventory + cage freeze |
| **status** | **Ready** |
| **parent_plan** | [14_THINK_EDGE_PICK.md](../14_THINK_EDGE_PICK.md) |
| **depends_on** | G.7 Closed; Doc 14 Locked; consoles V.1 Closed |

## architecture_refs

- [03_BRAIN_AND_GRAPH.md](../03_BRAIN_AND_GRAPH.md) — LLM only legal edges
- [04_LIVE_TURN_MACHINE.md](../04_LIVE_TURN_MACHINE.md) — Thinking I/O
- [10_CODING_PRINCIPLES.md](../10_CODING_PRINCIPLES.md) — EC-20–27
- [14_THINK_EDGE_PICK.md](../14_THINK_EDGE_PICK.md) — L2

## goal

Freeze the hard cage contract, name code seams (`runGraphTurn`, `matchChoice`, Think port), and confirm Sarvam-only desk pins vs fake registry — no product behaviour change yet.

## in_scope

- This file + phases README T.* table
- Written inventory of Talk/graph/Think call path
- Confirm degrade policy: Think fail → unclear → repair (no keyword fallback)
- Confirm fakes remain in registry; desk engines/profile = Sarvam

## out_scope

- Code changes to Talk or cursor
- Classifier implementation

## forbidden

- Starting T.1–T.3 implementation in this phase
- Deleting fake gateways

## exit_criteria

- [ ] Inventory + cage table committed in this file or Doc 14 (no contradiction)
- [ ] T.1–T.5 Ready with depends_on chain
- [ ] Owner accepts degrade + vendor policy

## edge_cases

- EC-20 illegal edge; EC-21 dial invent; EC-23 language→Tool — cited for later phases

## verification

```text
Test-Path docs/14_THINK_EDGE_PICK.md, docs/phases/T.0_think_edge_inventory.md
```

## rollback

Delete or mark Cancelled T.* rows if programme abandoned.

## handoff

Next: **T.1** cursor allowlist + TakeEdgeID.
