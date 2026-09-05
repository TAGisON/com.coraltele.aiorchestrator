# L3 — A.3 Admin flow list / create / draft

| Field | Value |
|---|---|
| **id** | `A.3` |
| **title** | Admin: flow registry + draft save/load |
| **status** | **Closed** — Admin flow list/create/draft |
| **parent_plan** | [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) Admin inventory A.3 |
| **depends_on** | A.2 Closed (`6b8b882`); G.2 `/v1/flows*` |

## architecture_refs

- [P2.7_flow_publish_model.md](./P2.7_flow_publish_model.md) — draft vs published version
- [G.2_flow_control_api.md](./G.2_flow_control_api.md) — Control routes
- OD-13-5 — structured builder in **A.4**; A.3 may use advanced JSON draft editor
- `internal/control/flows.go`

## goal

Admin can create flows, list them, load and save the mutable draft document — without the structured graph builder yet.

## in_scope

- `web/admin/flows.html` — create (id/name/tenant/direction select), list, load draft, save draft (JSON editor)
- Direction select: `inbound` \| `outbound` only (closed set)
- New draft scaffold uses `schema_id: coral.flow.v1` empty envelope (nodes/edges empty OK for draft)
- Extend `OrchAPI` flow helpers
- Hub + subnav links
- Smoke: page embedded and served
- This phase file + README

## out_scope

- Structured node/edge builder (**A.4**)
- Publish / version inspect UI (**A.4**) — may show `current_version` in list only
- Profile↔flow pin (**A.5**)
- Changing flow validate rules

## forbidden

- Free-typed direction outside inbound/outbound
- Claiming graph builder done
- Desk revive

## exit_criteria

- [x] Create flow via Admin → appears in list
- [x] Load/save draft via Admin (JSON body to PUT draft)
- [x] Direction is a dropdown
- [x] `go test ./web/... ./internal/control/...` green

## edge_cases

1. Incomplete draft JSON must still save (publish validates later).
2. Invalid JSON on save → client blocks before PUT.
3. Flow not found on draft load → show error.

## verification

```text
go test ./web/... ./internal/control/... -count=1 -timeout 180s
go build ./cmd/aiorchestrator/
```

## rollback

Remove `flows.html` and API helpers; flows HTTP remains.

## handoff

**Closed.** Next: **A.4** graph builder + publish + version inspect.
