# L3 — G.3 Runtime core cursor (Entry / Speak / ListenChoice / End)

| Field | Value |
|---|---|
| **id** | `G.3` |
| **title** | Runtime core cursor (Entry/Speak/ListenChoice/End) |
| **status** | **Closed** — pinned sessions walk Entry→Speak→ListenChoice→End |
| **parent_plan** | [G.0](./G.0_graph_runtime_inventory.md); [03](../03_BRAIN_AND_GRAPH.md); [04](../04_LIVE_TURN_MACHINE.md) |
| **depends_on** | G.2 Closed (`5fbdb9a`) |

## architecture_refs

- [03_BRAIN_AND_GRAPH.md](../03_BRAIN_AND_GRAPH.md) — cursor on one node; legal edges only
- [04_LIVE_TURN_MACHINE.md](../04_LIVE_TURN_MACHINE.md) — Talk shell unchanged
- [P2.7](./P2.7_flow_publish_model.md) / [P2.8](./P2.8_prompts_locale.md)
- [G.0_gap_list.md](./G.0_gap_list.md) G-RT-1…4 (core subset)

## goal

When a live session is pinned to a published `flow_version`, load the graph and walk **Entry → Speak → ListenChoice → End** inside the existing Talk shell — without Tool ARM (G.4), repair policy (G.5), or requiring every session to pin a flow (G.7).

## in_scope

- Package `internal/runtime/graph`: cursor, prompt resolve, `next` auto-advance, ListenChoice option/intent match
- Optional `flow_id` / `flow_version` on `POST /v1/sessions`; persist pins; load doc onto actor
- `Talk.AnswerCall` / turn path uses cursor when present; else existing profile ladder
- End node → session end callback (disposition `hangup_completed` when graph completes)
- Unit + control tests for welcome → choice → End
- Docs: this file + README

## out_scope

- Tool transfer/hangup arm→speak→exec (**G.4**)
- Repair / ListenLanguage / max_retries (**G.5**)
- Inform + binding (**G.6**)
- Refuse create without flow pin (**G.7**)
- `edge_taken` evidence emitters (**G.7**)
- Dual desk brain

## forbidden

- LLM free dial strings
- Jumping without drawn edges
- Executing Tool nodes in this phase (fail closed / stop before ARM)

## exit_criteria

- [x] Cursor unit: Entry→Speak→ListenChoice→End
- [x] Session create with flow pin loads published doc
- [x] Answer + inject choice walks welcome → End on pinned session
- [x] Unpinned sessions keep profile AnswerCall / Path.Run
- [x] `go test` graph + control graph path

## verification

```text
go test ./internal/runtime/graph/... ./internal/control/... ./internal/runtime/composer/... -count=1 -timeout 180s
```

## handoff

Next: **G.4** Tool transfer/hangup arm→speak→exec + matrix.
