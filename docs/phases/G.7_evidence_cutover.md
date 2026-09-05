# L3 — G.7 Evidence + live flow-pin cutover

| Field | Value |
|---|---|
| **id** | `G.7` |
| **title** | Evidence emitters + require live flow pin |
| **status** | **Closed** — edge_taken/tool_line + live pin gate |
| **parent_plan** | [G.0](./G.0_graph_runtime_inventory.md); [09](../09_EVIDENCE_AND_RECORDING.md); [P2.7](./P2.7_flow_publish_model.md) |
| **depends_on** | G.6 Closed (`8a198e0`); G.3–G.6 Closed |

## architecture_refs

- [09_EVIDENCE_AND_RECORDING.md](../09_EVIDENCE_AND_RECORDING.md) — `edge_taken` / `tool_line`
- [P2.3](./P2.3_session_events.md) / [P2.4](./P2.4_audit_events.md) — kinds + `graph.edge`
- [P2.7](./P2.7_flow_publish_model.md) — named end of dual-pin window
- [G.0_gap_list.md](./G.0_gap_list.md) G-EV-1…3, G-RT-10

## goal

Emit graph evidence on cursor moves and Tool closing lines; refuse **new live** sessions without a published `flow_id`/`flow_version` pin (playback may still use profile-only).

## in_scope

- Observer: `edge_taken` transcript + optional `graph.edge` audit; `tool_line` for Tool closing Speak
- Talk graph path emits Taken edges / tool_line
- `POST /v1/sessions` with `clock=live` requires flow pin (422 if missing)
- Lab fixture JSON under `tools/lab/` (minimal coral-xfer shape)
- Tests + README
- Docs: this file

## out_scope

- Admin UI
- Dropping profile pin columns (later migrate)
- Changing playback/profile-ladder behaviour beyond clock exemption
- Full coral-xfer production content pack

## forbidden

- Inventing dial strings
- Silent dual brain for **live** (must pin flow)
- Emitting legacy `utterance` kind for new graph rows

## exit_criteria

- [x] Live create without flow → 422
- [x] Graph walk emits `edge_taken` (and `graph.edge` audit)
- [x] Tool closing Speak → `tool_line`
- [x] Playback create without flow still OK
- [x] Lab fixture file present
- [x] `go test` control + observe + graph

## verification

```text
go test ./internal/runtime/observe/... ./internal/runtime/graph/... ./internal/control/... -count=1 -timeout 180s
Test-Path tools/lab/flows/coral_xfer_minimal.v1.json
```

## handoff

Graph runtime wave **G.0–G.7** complete for V1 call-flow core. Next: lab soak / ops, not further G.*.
