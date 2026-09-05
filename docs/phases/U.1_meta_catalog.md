# L3 — U.1 Meta catalog API

| Field | Value |
|---|---|
| **id** | `U.1` |
| **title** | `GET /v1/meta/catalog` — server enums for Admin dropdowns |
| **status** | **Closed** (5d9713a) — `GET /v1/meta/catalog` |
| **parent_plan** | [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) OD-13-3, OD-13-6, OD-13-7 |
| **depends_on** | U.0 Closed; doc 13 Locked |

## architecture_refs

- [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) — catalog gate for Admin
- [03_BRAIN_AND_GRAPH.md](../03_BRAIN_AND_GRAPH.md) — closed node/edge/tool sets
- [P2.6_disposition.md](./P2.6_disposition.md) — `final` codes + `source`
- [P2.9_routing_matrix.md](./P2.9_routing_matrix.md) — matrix `action`
- [P2.10_bindings_redesign.md](./P2.10_bindings_redesign.md) — binding kind/status/mode
- `internal/flow/doc.go` — SchemaID, node/edge/tool constants
- `internal/store/models_flow.go` — binding kind/status
- `internal/control/disposition.go` — disposition allowlist

## goal

Expose one read-only JSON catalog so Admin (and later Chat) dropdowns only offer values the server already validates — no free-typed illegal topology.

## in_scope

- `GET /v1/meta/catalog` on Control mux (same auth as other `/v1/*`)
- Payload built from existing Go constants / allowlists (no new DB tables — OD-13-6)
- At minimum arrays/fields:
  - `schema_id` (single: `coral.flow.v1`)
  - `node_types`
  - `edge_kinds`
  - `tools` (`transfer`, `hangup`)
  - `matrix_actions` (`transfer` for V1 matrix rows)
  - `binding_kinds`, `binding_statuses`, `knowledge_modes` (`inline_faq`, `http_retrieve`)
  - `clocks` (`live`, `playback`, `chat`) — `chat` listed now; runtime wiring in **C.1**
  - `disposition_final` codes, `disposition_sources`
  - `transcript_event_kinds` (current emitter kinds including `edge_taken`, `tool_line`)
- Unit/control test: GET 200; each closed set non-empty; node_types include Entry…End; clocks include `chat`
- Package helper preferred under `internal/flow` or `internal/control` so validate + catalog stay aligned
- This phase file + phases README

## out_scope

- Admin / Supervisor / Chat UI (**U.2+**)
- Implementing `clock=chat` runtime (**C.1**)
- Bindings HTTP (**A.2**)
- Full JSON Schema file for coral.flow.v1
- Locales list as product i18n pack (optional soft list OK if already known; not required)
- New DDL

## forbidden

- Hardcoding a second divergent node/edge set that drifts from `internal/flow`
- Accepting POST/PUT on catalog
- Desk revive
- Secrets in catalog payload

## exit_criteria

- [x] `GET /v1/meta/catalog` returns 200 JSON
- [x] `node_types` / `edge_kinds` / `tools` match flow package closed sets
- [x] `clocks` includes `live`, `playback`, `chat`
- [x] `disposition_final` matches control allowlist
- [x] Test covers happy path
- [x] `go test` for touched packages passes

## edge_cases

1. Catalog must not invent `http_retrieve` runtime if Inform only does `inline_faq` today — listing mode for Admin config is OK; Inform behaviour stays G.6.
2. Adding a node type later = change flow constants + catalog helper together.
3. Unauthenticated access follows same Bearer rules as sibling `/v1` routes.

## verification

```text
go test ./internal/flow/... ./internal/control/... -count=1 -timeout 180s
```

## rollback

Remove route + catalog helper; revert this phase status.

## handoff

**Closed.** Next: **U.2** shared web client + three app shells (placeholder pages).
