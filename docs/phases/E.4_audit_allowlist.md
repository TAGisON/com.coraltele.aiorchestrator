# L3 — E.4 Audit allowlist emitters

| Field | Value |
|---|---|
| **id** | `E.4` |
| **title** | Cut over audit emitters to P2.4 allowlist |
| **status** | **Closed** — allowlist emitters |
| **parent_plan** | [09](../09_EVIDENCE_AND_RECORDING.md) E.4; [P2.4](./P2.4_audit_events.md) |
| **depends_on** | E.0/E.2/E.3 Closed; P2.4 Locked |

## architecture_refs

- [P2.4_audit_events.md](./P2.4_audit_events.md)
- [09_EVIDENCE_AND_RECORDING.md](../09_EVIDENCE_AND_RECORDING.md) B2
- [03_BRAIN_AND_GRAPH.md](../03_BRAIN_AND_GRAPH.md) tools arm→speak→exec

## goal

Align production audit emitters with the Locked P2.4 V1 allowlist: session lifecycle, tool arm/exec chain, turn.state; keep recording.* from E.2/E.3; stop writing deprecated Phase-E type strings for those paths.

## in_scope

- `store` audit constants for allowlist types
- Observe: session.live / session.{completed,cancelled,failed} / turn.state / tool.executed (from skill)
- callops Transfer + FailCall hangup: tool.armed → tool.executing → tool.executed|tool.failed
- Map listen audit to optional `stt.final`; drop `speak.prompt` new emits
- Update DetectHandoffFromAudit + validation tests for new types
- Docs: this file + README

## out_scope

- graph.edge (no graph runtime yet)
- language.changed emitter polish
- Full historical row rewrite
- E.5 disposition finals
- Removing reader tolerance for legacy types

## forbidden

- LLM-invented event_type
- desk.* emits
- Absorbing E.5

## exit_criteria

- [x] New session/tool/turn emitters use allowlist strings
- [x] Transfer/hangup emit tool arm→exec chain
- [x] Scoped tests pass (store/observe/control/validation)

## verification

```text
go test ./internal/store/... ./internal/runtime/observe/... ./internal/control/... ./internal/validation/... -count=1 -timeout 180s
```

## handoff

Next: **E.5** disposition on tool settle.
