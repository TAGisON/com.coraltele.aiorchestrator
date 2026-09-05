# L3 — G.2 Flow control API + coral.flow.v1 validate/publish

| Field | Value |
|---|---|
| **id** | `G.2` |
| **title** | Flow control API + `coral.flow.v1` validate/publish |
| **status** | **Closed** — Control `/v1/flows*` + publish validation |
| **parent_plan** | [G.0](./G.0_graph_runtime_inventory.md); [P2.7](./P2.7_flow_publish_model.md); [P2.8](./P2.8_prompts_locale.md); [P2.9](./P2.9_routing_matrix.md) |
| **depends_on** | G.1 Closed (`1c3140b`) |

## architecture_refs

- [P2.7_flow_publish_model.md](./P2.7_flow_publish_model.md) — envelope + publish rules
- [P2.8_prompts_locale.md](./P2.8_prompts_locale.md) — `default_locale` + prompt_ref
- [P2.9_routing_matrix.md](./P2.9_routing_matrix.md) — transfer Tool ↔ matrix
- [G.0_gap_list.md](./G.0_gap_list.md) G-CFG-4, G-CFG-5

## goal

Expose Control HTTP for flow create/draft/publish/get and reject publish of non-`coral.flow.v1` or structurally invalid documents — without a live graph walker.

## in_scope

- Package `internal/flow`: parse + publish validation (envelope, nodes/edges, prompts, matrix)
- Routes: create flow, get flow, put/get draft, publish version, get version, list flows
- Immutable `flow_version` via store `PublishFlowVersion`; content hash of published bytes
- HTTP 422 on validation failure (`flow_invalid`)
- Unit tests: good publish; reject bad `schema_id` / missing Entry / transfer without matrix number
- Docs: this file + README

## out_scope

- Graph runtime cursor (**G.3**)
- Binding entity existence checks beyond string refs (**G.6**)
- Admin SPA
- Full JSON Schema file
- Forcing session flow pins on create

## forbidden

- Desk revive
- LLM dial strings
- Absorbing walker / tool ARM (**G.3/G.4**)

## exit_criteria

- [x] POST create + PUT draft + POST versions publish succeeds for valid doc
- [x] Publish rejects bad envelope (422)
- [x] GET published version returns immutable doc
- [x] `go test` for `internal/flow` + `internal/control` flow handlers

## verification

```text
go test ./internal/flow/... ./internal/control/... -count=1 -timeout 180s
```

## handoff

Next: **G.3** runtime core cursor (Entry/Speak/ListenChoice/End).
