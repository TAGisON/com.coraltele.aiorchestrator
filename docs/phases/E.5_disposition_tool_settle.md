# L3 — E.5 Disposition on tool settle

| Field | Value |
|---|---|
| **id** | `E.5` |
| **title** | Write P2.6 `final` on tool settle / Ending |
| **status** | **Closed** — live_tool + Ending fill-in |
| **parent_plan** | [09](../09_EVIDENCE_AND_RECORDING.md) E.5; [P2.6](./P2.6_disposition.md) |
| **depends_on** | E.4 Closed (`8fc4d67`); P2.6 Locked |

## architecture_refs

- [09_EVIDENCE_AND_RECORDING.md](../09_EVIDENCE_AND_RECORDING.md) B4
- [P2.6_disposition.md](./P2.6_disposition.md)
- [P2.9_routing_matrix.md](./P2.9_routing_matrix.md) — `disposition_code` on transfer ARM
- [04_LIVE_TURN_MACHINE.md](../04_LIVE_TURN_MACHINE.md) — tool executed → Ending
- [10_CODING_PRINCIPLES.md](../10_CODING_PRINCIPLES.md) EC-01 / evidence rules

## goal

Persist allowlisted `session_disposition.final` with Locked `source` after transfer/hangup settle, and fill gaps on Ending when no tool wrote a final.

## in_scope

- Map transfer settle → `transferred_*` (`source=live_tool`): prefer `disposition_code` / intent / reason; default `transferred_other`
- Map FailCall hangup settle → `system_failure` (`source=live_tool`); on `tool.failed` same
- Stop writing non-allowlist finals (`transferred`, `failed_*`, `source=system`)
- Pass optional `disposition_code` through coral-transfer → `TransferRequest`
- Ending fill-in when `final` empty: Cancelled → `abandoned_caller`; Failed → `system_failure`; Completed → `out_of_scope` (`source=live_graph`)
- Shared allowlist helper used by PATCH + writers
- Unit/control tests for transfer + FailCall + Ending fill-in
- Docs: this file + README

## out_scope

- Graph End node wiring beyond Ending fill-in heuristic
- Dedicated hangup tool for `hangup_silence` / `hangup_abuse` / `hangup_completed` (no V1 hangup tool path yet — FailCall stays `system_failure`)
- Full matrix runtime (P2.9 publish already Locked; ARM freeze when graph exists)
- CRM push / postcall suggestion rewrite
- CHECK constraint DDL

## forbidden

- Free-text or legacy `resolved`/`unresolved`/`escalated` as new `final`
- Inventing dial numbers
- Absorbing E.6 soak checklist

## exit_criteria

- [x] Transfer success writes allowlisted `transferred_*` + `live_tool`
- [x] FailCall / transfer fail writes `system_failure` + `live_tool`
- [x] Ending without prior final writes Cancelled/Failed/Completed fill-ins
- [x] Scoped tests pass

## verification

```text
go test ./internal/control/... ./internal/gateway/coraltransfer/... ./internal/store/... -count=1 -timeout 180s
```

## handoff

Next: **E.6** evidence soak checklist.
