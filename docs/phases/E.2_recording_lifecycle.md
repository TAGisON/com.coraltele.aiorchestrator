# L3 — E.2 Recording lifecycle stamps

| Field | Value |
|---|---|
| **id** | `E.2` |
| **title** | Stamp recording start/stop on session + audit |
| **status** | **Closed** — start/stop stamps + recording.* audit |
| **parent_plan** | [09](../09_EVIDENCE_AND_RECORDING.md) E.2; [E.0](../../.agent/work/E.0/gap_list.md) G-B3-1/2 |
| **depends_on** | E.0 Closed; M-Cr `013` Closed; P2.5 Locked |

## architecture_refs

- [09_EVIDENCE_AND_RECORDING.md](../09_EVIDENCE_AND_RECORDING.md) B3
- [P2.5_recording_metadata.md](./P2.5_recording_metadata.md)
- [P2.4_audit_events.md](./P2.4_audit_events.md) `recording.started` / `recording.stopped`
- [04_LIVE_TURN_MACHINE.md](../04_LIVE_TURN_MACHINE.md) Ending → stop

## goal

Persist `recording_started_at` / `recording_stopped_at` / `recording_stop_reason` / `recording_bytes` when the recorder starts and stops, and emit matching audit events — without leaving stop dependent on client WS alone.

## in_scope

- Session fields + Repository APIs to mark start/stop (PG + Memory)
- Wire `callops.startRecorder` / `stopRecorder`
- Map stop reasons to P2.5 vocabulary
- Audit `recording.started` / `recording.stopped`
- Unit tests (memory)
- Docs: this file + README

## out_scope

- Orphan reaper (**E.3**)
- Mono mix rewrite (G-B3-4 / OD-09-1 — defer)
- Full session SELECT rewrite for every list path (stamp APIs sufficient)
- Tool/audit vocabulary cutover beyond recording.* (**E.4**)

## forbidden

- PCM in DB
- Skipping stop stamp when recorder was started
- Implementing E.3 reaper here

## exit_criteria

- [x] Start stamps ref + started_at; stop stamps stopped_at + reason (+ bytes when known)
- [x] Audits emitted on start/stop
- [x] Scoped tests pass

## verification

```text
go test ./internal/store/... ./internal/control/... ./internal/runtime/record/... -count=1 -timeout 120s
```

## handoff

Next: **E.3** orphan reaper.
