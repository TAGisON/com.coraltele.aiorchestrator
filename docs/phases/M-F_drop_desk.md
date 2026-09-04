# L3 — M-F DROP desk tables

| Field | Value |
|---|---|
| **id** | `M-F` |
| **title** | DROP `desk_version`, `desk_draft`, `desk` |
| **status** | **Closed** — DDL `015_drop_desk.sql` |
| **parent_plan** | [P2.13](./P2.13_drop_obsolete.md) M-F; [P2.14](./P2.14_migration_ci.md) |
| **depends_on** | M-E Closed (`24aefde`); `flow_*` live (`010`) |

## architecture_refs

- [P2.13_drop_obsolete.md](./P2.13_drop_obsolete.md)  
- [P2.7_flow_publish_model.md](./P2.7_flow_publish_model.md)  
- [P2.14_migration_ci.md](./P2.14_migration_ci.md)  

## goal

Contract step: drop obsolete desk registry after flow_* replacement and zero Go desk readers.

## in_scope

- `internal/store/migrations/015_drop_desk.sql`  
- DROP order: `desk_version` → `desk_draft` → `desk`  
- Unit test  
- Docs: this file + README  

## out_scope

- DROP kb* (**M-G**) / compliance (**M-H**)  
- Data migrate desk→flow  
- Root `migrations/`  

## forbidden

- Edit `008` history  
- Bundle kb DROP  
- DROP while Go readers remain (cleared in M-E)  

## exit_criteria

- [x] `015` drops three desk tables in safe order  
- [x] Scoped store test passes  
- [x] No kb DROP in this file  

## verification

```text
go test ./internal/store/... -count=1
```

## handoff

Next: **M-G** DROP kb*.
