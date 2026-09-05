# L3 — E.0 Evidence inventory vs code

| Field | Value |
|---|---|
| **id** | `E.0` |
| **title** | Evidence inventory vs code (B1–B3 gap list) |
| **status** | **Closed** — gap list filed |
| **parent_plan** | [09_EVIDENCE_AND_RECORDING.md](../09_EVIDENCE_AND_RECORDING.md) § Phase breakdown E.0 |
| **depends_on** | Doc 09 Locked; P1 Done; P2.3–P2.6 Locked; M-D / M-Cr DDL Closed (`014` / `013`) |

## architecture_refs

- [09_EVIDENCE_AND_RECORDING.md](../09_EVIDENCE_AND_RECORDING.md) — B1–B5, OD-09-*
- [04_LIVE_TURN_MACHINE.md](../04_LIVE_TURN_MACHINE.md) — actionable vs transcript-only; Ending
- [P2.3_transcript_events.md](./P2.3_transcript_events.md)
- [P2.4_audit_events.md](./P2.4_audit_events.md)
- [P2.5_recording_metadata.md](./P2.5_recording_metadata.md)
- [P2.6_disposition.md](./P2.6_disposition.md)

## goal

Map live writers under `internal/runtime` / `internal/control` / `internal/store` to docs/09 B1–B3 and file an explicit gap list that later E.1–E.5 Implementers must close — without changing emitters in this phase.

## in_scope

- Read-only inventory of transcript, audit, recording, and disposition writers
- Gap list artifact: `.agent/work/E.0/gap_list.md` (B1 / B2 / B3 / B4-adjacent / cross-cutting)
- This phase file + `docs/phases/README.md` E.* row
- Cite concrete paths/symbols; no invented emitters

## out_scope

- Changing observe / composer / callops emitters (**E.1+**)
- Store API reshape for expanded transcript/recording columns (**E.1** / **E.2**)
- Orphan reaper implementation (**E.3**)
- CI workflows (**CI.0**)
- UI for evidence
- Graph/`flow_*` runtime load (separate future L3)

## forbidden

- Implementing E.1–E.5 “while here”
- Mixing schema DDL into this phase
- Claiming B1–B3 compliance without evidence in the gap list
- Reintroducing desk.* audit types

## exit_criteria

- [x] `.agent/work/E.0/gap_list.md` exists with B1, B2, B3 sections and ranked gaps
- [x] Each gap names current path/symbol or “missing”
- [x] `implementation.md` lists inventory commands/evidence
- [x] No product code changes required for pass (docs + artifacts only OK)

## edge_cases

1. Historical migrate_test / `008` CREATE strings are not writers — exclude from “emitted today”
2. Memory store mirrors must be noted as lab-only, not a second product path
3. Stereo on-disk recorder vs OD-09-1 mono — record as gap, do not “fix” in E.0
4. Postcall disposition suggestions are not live tool-settle finals — classify under B4-adjacent

## verification

```text
# gap list present and non-empty B1–B3 headings
Test-Path .agent/work/E.0/gap_list.md
# no unexpected Go edits for this phase (expect none)
git diff --stat -- internal/
```

## rollback

Delete `.agent/work/E.0/` artifacts and revert this phase file / README row.

## handoff

Next: **E.1** Transcript emitter (after gap list accepted); optionally interrupt for **CI.0**/**CI.1**.
