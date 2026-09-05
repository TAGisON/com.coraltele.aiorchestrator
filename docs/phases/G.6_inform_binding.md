# L3 — G.6 Inform + knowledge binding

| Field | Value |
|---|---|
| **id** | `G.6` |
| **title** | Inform + knowledge binding |
| **status** | **Closed** — Inform + inline_faq live binding |
| **parent_plan** | [G.0](./G.0_graph_runtime_inventory.md); [P2.10](./P2.10_bindings_redesign.md); [03](../03_BRAIN_AND_GRAPH.md) |
| **depends_on** | G.5 Closed (`1d8fc87`) |

## architecture_refs

- [P2.10_bindings_redesign.md](./P2.10_bindings_redesign.md) — live binding lookup; `inline_faq` / `http_retrieve`
- [03_BRAIN_AND_GRAPH.md](../03_BRAIN_AND_GRAPH.md) — Inform + binding_ref
- [G.0_gap_list.md](./G.0_gap_list.md) G-RT-9
- Store: `UpsertBinding` / `GetBinding` (G.1)

## goal

When the cursor lands on an **Inform** node, resolve `binding_ref` to a live active `knowledge` binding, answer from `inline_faq` (thin FAQ), speak the answer, then follow `next`. Missing/disabled/miss → fail closed via drawn `repair` edge (no invented FAQ).

## in_scope

- Publish check: Inform `binding_ref` ∈ `binding_refs`
- Cursor Inform + `InformLookup`; session wires store-backed lookup
- `inline_faq` keyword match by locale (active → default)
- `http_retrieve` → fail closed to repair (no invent); optional later gateway wire
- Unit + control tests (Memory binding + graph walk)
- Docs: this file + README

## out_scope

- Vector KB / upload product
- CRM bindings runtime
- Evidence emitters / require flow pin (**G.7**)
- Binding admin UI; optional `/v1/bindings` HTTP (store upsert in tests OK)

## forbidden

- Wiring Inform to retired `kb_*` tables
- Inventing FAQ text on miss
- Secrets in binding.config

## exit_criteria

- [x] Inform + inline_faq answers and continues
- [x] Missing/disabled/miss → repair edge (or error if none)
- [x] Publish rejects Inform binding_ref not in binding_refs
- [x] `go test` graph + control + flow

## verification

```text
go test ./internal/flow/... ./internal/runtime/graph/... ./internal/control/... -count=1 -timeout 180s
```

## handoff

Next: **G.7** evidence + live cutover (require flow pin).
