# L3 — A.2 Bindings Control HTTP + Admin CRUD

| Field | Value |
|---|---|
| **id** | `A.2` |
| **title** | Bindings HTTP API + Admin binding CRUD |
| **status** | **Closed** — bindings HTTP + Admin CRUD |
| **parent_plan** | [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md); [P2.10](./P2.10_bindings_redesign.md) |
| **depends_on** | A.1 Closed (`411eefd`); G.1 store bindings; U.1 catalog |

## architecture_refs

- [P2.10_bindings_redesign.md](./P2.10_bindings_redesign.md) — `binding` entity; `inline_faq` / `http_retrieve`
- [03_BRAIN_AND_GRAPH.md](../03_BRAIN_AND_GRAPH.md) — bindings ≠ nodes
- [G.6_inform_binding.md](./G.6_inform_binding.md) — live lookup
- Store: `UpsertBinding` / `GetBinding` / `ListBindings`
- Catalog: `binding_kinds`, `binding_statuses`, `knowledge_modes`

## goal

Expose Control HTTP for knowledge/CRM bindings and an Admin screen that creates/updates them using **catalog enums only** for kind/status/mode — so Inform can reference real `binding_refs` later in the flow builder.

## in_scope

- Routes: `GET /v1/bindings`, `GET /v1/bindings/{id}`, `PUT /v1/bindings/{id}` (upsert)
- Validate `kind` ∈ knowledge|crm; `status` ∈ active|disabled; for `knowledge`, `config.mode` ∈ inline_faq|http_retrieve (default inline_faq if empty)
- Reject secrets-looking fields if easy (no `api_key` inside config) — optional soft check
- Admin page `web/admin/bindings.html` + hub link; kind/status/mode selects from catalog; inline_faq entry form
- Extend `web/shared/api.js`
- Control tests: upsert + list + get + reject bad kind
- This phase file + README

## out_scope

- Flow graph builder / binding_refs wiring in UI (**A.3–A.4**)
- Implementing `http_retrieve` runtime (still fail-closed per G.6)
- Delete binding endpoint (disable via status is enough for V1)
- Soft size-cap enforcement beyond reasonable body limit

## forbidden

- Free-typed kind/status/mode as primary UI controls
- Secrets in `config` (api_key field)
- Desk / kb_* revive
- Absorbing flow builder

## exit_criteria

- [x] PUT upsert + GET list/get work
- [x] Bad kind → 400/422
- [x] Admin bindings page served; uses catalog dropdowns
- [x] `go test ./internal/control/... ./web/...` green

## edge_cases

1. Empty config → store `{}`; knowledge mode treated as inline_faq at Inform time.
2. List filter `?kind=knowledge` optional.
3. Tenant from body or `X-Tenant-ID` / default like other tenant APIs.

## verification

```text
go test ./internal/control/... ./web/... -count=1 -timeout 180s
go build ./cmd/aiorchestrator/
```

## rollback

Remove bindings handlers + admin page; store APIs remain.

## handoff

**Closed.** Next: **A.3** flow list/create/draft Admin screens.
