# L3 — G.1 Store flow + binding + session pins

| Field | Value |
|---|---|
| **id** | `G.1` |
| **title** | Store APIs for `flow_*`, `binding`, session pins |
| **status** | **Closed** — Memory+PG store APIs |
| **parent_plan** | [G.0](./G.0_graph_runtime_inventory.md); [P2.7](./P2.7_flow_publish_model.md); [P2.10](./P2.10_bindings_redesign.md) |
| **depends_on** | G.0 Closed (`fcfa5b2`); M-A/B/C DDL Closed |

## architecture_refs

- [P2.7_flow_publish_model.md](./P2.7_flow_publish_model.md)
- [P2.10_bindings_redesign.md](./P2.10_bindings_redesign.md)
- [G.0_gap_list.md](./G.0_gap_list.md) G-CFG-1…3
- DDL: `010_flow_registry.sql`, `011_binding.sql`, `012_session_flow_pin.sql`

## goal

Expose durable Go APIs (Memory + Postgres) for flow registry/draft/version, binding registry, and session `flow_id`/`flow_version` pins — without HTTP or graph walker.

## in_scope

- Models: `Flow`, `FlowDraft`, `FlowVersion`, `Binding`; `Session.FlowID` / `FlowVersion`
- Repository methods + Memory + Store implementations
- Create/Get session round-trip pins; list/update paths return pins when present
- Unit tests (Memory); no envelope validation (G.2)
- Docs: this file + README

## out_scope

- Control `/v1/flows*` (**G.2**)
- Publish validation of `coral.flow.v1`
- Graph runtime walker
- New DDL

## forbidden

- Desk table revive
- LLM dial strings
- Absorbing G.2 HTTP

## exit_criteria

- [x] Memory: create flow + draft + publish version + get version
- [x] Memory: binding upsert/get/list
- [x] Session create/get round-trips FlowID/FlowVersion
- [x] Repository interface extended; `go test ./internal/store/...`

## verification

```text
go test ./internal/store/... -count=1 -timeout 120s
```

## handoff

Next: **G.2** flow control API + validate/publish.
