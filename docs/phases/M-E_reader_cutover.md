# L3 — M-E Reader cutover

| Field | Value |
|---|---|
| **id** | `M-E` |
| **title** | Switch/remove Go readers before DROP (desk / kb / compliance) |
| **status** | **Closed** — reader cutover (owner Run 2026-09-05) |
| **parent_plan** | [P2.13](./P2.13_drop_obsolete.md) M-E; [P2.10](./P2.10_bindings_redesign.md) |
| **depends_on** | M-D Closed (`44c711b`); `flow_*` + `binding` tables exist |

## architecture_refs

- [P2.13_drop_obsolete.md](./P2.13_drop_obsolete.md)  
- [P2.10_bindings_redesign.md](./P2.10_bindings_redesign.md)  
- OD-08-4  

## goal

Ensure no production Go path queries tables scheduled for M-F…M-H before DROP SQL lands.

## in_scope

- Remove KB HTTP + Repository methods + PG/Memory implementations  
- Ingest Knowledge → IndexLocal only (no `kb_*` SQL)  
- Remove unused skill/compliance Repository methods  
- Keep `KBChunk` type for lab IndexLocal only  
- Docs + tests  

## out_scope

- DROP SQL (M-F…M-H)  
- Full Inform binding runtime  
- Editing `002`/`008` history  

## exit_criteria

- [x] Desk SQL absent  
- [x] `kb_*` Go readers gone (except migrate history tests / comments)  
- [x] Compliance/skill store APIs gone  
- [x] `go build` / targeted tests pass  

## handoff

Next: Reviewer → … → **M-F** DROP desk*.
