# L3 — A.4 Admin graph builder + publish + version inspect

| Field | Value |
|---|---|
| **id** | `A.4` |
| **title** | Admin: structured flow builder, publish, version inspect |
| **status** | **Closed** (d5e2f9d) — structured builder + publish + version inspect |
| **parent_plan** | [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) A.4; OD-13-5 |
| **depends_on** | A.3 Closed (`6c85e18`); U.1 catalog; G.2 validate/publish |

## architecture_refs

- [03_BRAIN_AND_GRAPH.md](../03_BRAIN_AND_GRAPH.md) — closed node/edge/tool sets
- [P2.7](./P2.7_flow_publish_model.md) / [P2.8](./P2.8_prompts_locale.md) / [P2.9](./P2.9_routing_matrix.md)
- OD-13-5 — structured builder primary; advanced JSON remains on A.3 flows page
- `GET /v1/meta/catalog`, `/v1/flows*`, `/v1/bindings`

## goal

Admin builds a legal `coral.flow.v1` draft via forms (catalog enums for node type, edge kind, tool, matrix action, disposition), publishes immutable versions, and inspects a published version — without free-typed illegal topology as the primary path.

## in_scope

- `web/admin/flows-builder.html` (+ optional `flow-builder.js`) linked from flows hub
- Load/save draft into structured UI: meta, nodes, edges, prompts, matrix, binding_refs
- Dropdowns from catalog (+ bindings list for refs; node ids for from/to/entry)
- Publish current draft (POST versions); surface `flow_invalid` details
- Inspect published version by version number
- Keep A.3 JSON editor as advanced escape hatch
- Smoke embed/static tests
- This phase file + README

## out_scope

- Visual canvas / drag-drop graph
- Profile↔DID pin (**A.5**)
- Changing server validate rules
- Auto-layout / simulation runner

## forbidden

- Primary free-text fields for node `type` / edge `kind` / tool / matrix `action`
- Skipping client enum checks while still relying only on server (both required)
- Desk revive

## exit_criteria

- [x] Builder page served; node type / edge kind are `<select>` from catalog
- [x] Can save draft and publish a valid minimal graph from the builder
- [x] Publish errors shown from server details
- [x] Can load a published version for inspect
- [x] `go test ./web/... ./internal/control/...` green

## edge_cases

1. Empty nodes → entry select empty until first node added.
2. Inform requires binding_ref ∈ binding_refs — UI should keep refs list editable.
3. Transfer Tool needs matrix_intent matching a matrix row — show both sections clearly.

## verification

```text
go test ./web/... ./internal/control/... -count=1 -timeout 180s
go build ./cmd/aiorchestrator/
```

## rollback

Remove builder page; keep A.3 flows + APIs.

## handoff

**Closed.** Next: **A.5** profile / DID / live pin association UX.
