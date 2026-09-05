# L3 — E.3 Orphan recording reaper

| Field | Value |
|---|---|
| **id** | `E.3` |
| **title** | Stamp `orphan_reaper` on leaked recordings |
| **status** | **Closed** — orphan_reaper sweep |
| **parent_plan** | [09](../09_EVIDENCE_AND_RECORDING.md) E.3; P2.5 behavioural rule 3 |
| **depends_on** | E.2 Closed (`4c6418a`) |

## architecture_refs

- [09_EVIDENCE_AND_RECORDING.md](../09_EVIDENCE_AND_RECORDING.md) B3 orphan reaper
- [P2.5_recording_metadata.md](./P2.5_recording_metadata.md)
- [10_CODING_PRINCIPLES.md](../10_CODING_PRINCIPLES.md) EC-05 / EC-34

## goal

Periodically find terminal sessions with `recording_started_at` set and `recording_stopped_at` null, stamp stop with `orphan_reaper`, and emit `recording.stopped`.

## in_scope

- `ListOrphanRecordingSessions` (PG + Memory)
- Reaper sweep helper + boot ticker (alongside retention)
- Best-effort `recording_bytes` from file size when `recording_ref` exists
- Audit `recording.stopped` with reason `orphan_reaper`
- Unit tests
- Docs: this file + README

## out_scope

- Cross-process file-handle kill (stamp is enough after crash)
- Day-dir retention (already exists)
- Mono mix (G-B3-4)
- E.4 full audit allowlist cutover

## forbidden

- Touching live/non-terminal sessions
- Overwriting an existing stop reason
- PCM in DB

## exit_criteria

- [x] Orphans listed only when terminal + started + not stopped
- [x] Sweep stamps `orphan_reaper` (+ bytes when file present)
- [x] Boot starts periodic reaper
- [x] Scoped tests pass

## verification

```text
go test ./internal/store/... ./internal/runtime/record/... ./cmd/aiorchestrator/... -count=1 -timeout 120s
```

## handoff

Next: **E.4** audit allowlist emitters (or E.5 disposition).
