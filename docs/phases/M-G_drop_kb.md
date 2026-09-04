# L3 — M-G DROP kb tables

| Field | Value |
|---|---|
| **id** | `M-G` |
| **title** | DROP `kb_chunk`, `kb_document` |
| **status** | **Closed** — DDL `016_drop_kb.sql` |
| **parent_plan** | [P2.13](./P2.13_drop_obsolete.md) M-G; [P2.14](./P2.14_migration_ci.md) |
| **depends_on** | M-E Closed (`24aefde`); M-F Closed (`cbdc974`); `binding` live (`011`) |

## architecture_refs

- [P2.13_drop_obsolete.md](./P2.13_drop_obsolete.md)  
- [P2.10_bindings_redesign.md](./P2.10_bindings_redesign.md)  
- [P2.14_migration_ci.md](./P2.14_migration_ci.md)  

## goal

Contract step: drop obsolete KB tables after binding replacement and zero Go kb readers (M-E).

## in_scope

- `internal/store/migrations/016_drop_kb.sql`  
- DROP order: `kb_chunk` → `kb_document`  
- Unit test  
- Docs: this file + README  

## out_scope

- DROP desk* (**M-F**, already Closed)  
- DROP skill/compliance (**M-H**)  
- Data migrate kb→binding  
- Root `migrations/`  
- Editing `002_phase_d_kb.sql` history  

## forbidden

- Edit `002` history  
- Bundle desk or compliance DROP  
- DROP while Go readers remain (cleared in M-E)  

## exit_criteria

- [x] `016` drops `kb_chunk` then `kb_document`  
- [x] Scoped store test passes  
- [x] No desk/compliance DROP in this file  

## verification

```text
go test ./internal/store/... -count=1
```

## handoff

Next: **M-H** DROP skill/compliance (after unused-confirm).
