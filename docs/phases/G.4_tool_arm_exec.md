# L3 — G.4 Tool transfer/hangup arm→speak→exec + matrix

| Field | Value |
|---|---|
| **id** | `G.4` |
| **title** | Tool transfer/hangup arm→speak→exec + matrix |
| **status** | **Closed** — Tool ARM + matrix freeze + exec once |
| **parent_plan** | [G.0](./G.0_graph_runtime_inventory.md); [03](../03_BRAIN_AND_GRAPH.md); [P2.9](./P2.9_routing_matrix.md) |
| **depends_on** | G.3 Closed (`599d37b`) |

## architecture_refs

- [03_BRAIN_AND_GRAPH.md](../03_BRAIN_AND_GRAPH.md) — ARM → Speak closing → execute once
- [04_LIVE_TURN_MACHINE.md](../04_LIVE_TURN_MACHINE.md)
- [P2.9_routing_matrix.md](./P2.9_routing_matrix.md) — freeze number/owner/target/disposition
- [G.0_gap_list.md](./G.0_gap_list.md) G-RT-5, G-RT-6
- Existing: `SessionRuntime.Transfer`, `CallControl.Hangup`, E.5 dispositions

## goal

When the cursor lands on a **Tool** node, freeze params from the routing matrix (transfer) or hangup verb, speak the optional closing `prompt_ref`, then execute **once** via existing call-control — never LLM dial strings. Settle P2.6 finals (`transferred_*` / `hangup_completed`).

## in_scope

- Cursor: arm Tool (transfer via matrix intent; hangup without matrix)
- Talk: barge off for closing line; `OnToolArmed` after Speak
- Runtime: exec transfer / hangup once; disposition on settle; session end
- Fail closed if matrix row/number missing at ARM (no invent)
- Unit + control tests with stub CallControl
- Docs: this file + README

## out_scope

- Repair / ListenLanguage (**G.5**)
- Inform (**G.6**)
- `edge_taken` / `tool_line` evidence kinds (**G.7**)
- Replacing FailCall (pipeline failure stays `system_failure`)

## forbidden

- Free LLM/STT dial strings as SoT
- Executing Tool twice
- Using FailCall for intentional hangup Tool

## exit_criteria

- [x] Cursor arms transfer with frozen matrix number + disposition_code
- [x] Cursor arms hangup Tool
- [x] Talk speak → exec once; disposition settled
- [x] Missing matrix/number fail-closed (publish validate + ARM)
- [x] `go test` graph + control tool path

## verification

```text
go test ./internal/runtime/graph/... ./internal/control/... ./internal/runtime/composer/... -count=1 -timeout 180s
```

## handoff

Next: **G.5** repair + ListenLanguage + prompt locale.
