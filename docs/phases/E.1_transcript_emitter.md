# L3 — E.1 Transcript emitter (V1 kinds + actionable)

| Field | Value |
|---|---|
| **id** | `E.1` |
| **title** | Emit `user_final` / `bot_utterance` with actionable fields |
| **status** | **Closed** — user_final/bot_utterance emitters |
| **parent_plan** | [09](../09_EVIDENCE_AND_RECORDING.md) E.1; [E.0 gap list](../../.agent/work/E.0/gap_list.md) G-B1-1…4 |
| **depends_on** | E.0 Closed; M-D `014` Closed; P2.3 Locked |

## architecture_refs

- [09_EVIDENCE_AND_RECORDING.md](../09_EVIDENCE_AND_RECORDING.md) B1
- [P2.3_transcript_events.md](./P2.3_transcript_events.md)
- [04_LIVE_TURN_MACHINE.md](../04_LIVE_TURN_MACHINE.md)

## goal

Cut over store + observe emitters so new transcript rows use V1 `event_kind` values and structured `actionable` / `actionable_reason` instead of legacy turn-pair-only inserts.

## in_scope

- Expand `store.TranscriptTurn` + `AppendTranscriptTurn` / `ListTranscriptTurns` (PG + Memory) for `014` columns
- Observe: `OnTurnCompleted` / `AppendAssistantOnly` / user-final helper emit `user_final` + `bot_utterance` (never new `utterance`)
- Map suppress reasons to closed starter set (`echo_suspect`, `barge_forbidden`, `too_short`, …)
- Wire `runtime_adapter.auditListenDecision` to structured user_final (no `[reason]` text prefix)
- Unit tests for memory append + observe emit shape
- Docs: this file + README

## out_scope

- `edge_taken` / `tool_line` emitters (G-B1-5/6 — need graph/tools runtime)
- Recording lifecycle (**E.2**)
- Audit vocabulary cutover (**E.4**)
- Disposition (**E.5**)
- Transcript viewer UI

## forbidden

- Dropping transcript-only (non-actionable) user finals
- Writing new rows with `event_kind=utterance`
- PCM in payload
- Absorbing E.2–E.5

## exit_criteria

- [x] Model + INSERT/List cover event_kind, actionable, actionable_reason (and pass-through node/edge/language/payload defaults)
- [x] Observe turn path emits `user_final` + `bot_utterance`
- [x] Suppressed STT persists `user_final` with `actionable=false` + mapped reason
- [x] Scoped tests pass: `go test ./internal/store/... ./internal/runtime/observe/... ./internal/control/... -count=1 -timeout 120s`

## verification

```text
go test ./internal/store/... ./internal/runtime/observe/... ./internal/control/... -count=1 -timeout 120s
```

## rollback

Revert observe/store field wiring; columns remain in DDL.

## handoff

Next: **E.2** recording lifecycle stamps (or CI.2).
