# L3 — M-B Create `binding` registry

| Field | Value |
|---|---|
| **id** | `M-B` |
| **title** | CREATE `binding` |
| **status** | **Closed** — DDL `011_binding.sql` (owner signed 2026-09-05) |
| **parent_plan** | [P2.10](./P2.10_bindings_redesign.md) `migration_plan_note`; [P2.14](./P2.14_migration_ci.md) M-B; [P2.13](./P2.13_drop_obsolete.md) |
| **depends_on** | M-A Closed (`78f57f7`); P2.10 Locked; gate §8 Locked |

## architecture_refs

- [P2.10_bindings_redesign.md](./P2.10_bindings_redesign.md) — table + config modes  
- [P2.14_migration_ci.md](./P2.14_migration_ci.md) — R1–R7  
- [P2.0_schema_principles.md](./P2.0_schema_principles.md) — P2.0-P10 / P11  
- [08](../08_PURGE_AND_SCHEMA_PHASES.md) OD-08-4  

## goal

Apply the second expand migration: CREATE `binding` so Inform can leave `kb_*` without DROPping them in this file.

## in_scope

- New SoT file `internal/store/migrations/011_binding.sql`.  
- CREATE `binding` + index `(tenant_id, kind)` per P2.10.  
- Unit test asserting table present and no `kb_*` / DROP in this file.  
- Docs: this phase file + `docs/phases/README.md` M-B row.

## out_scope

- DROP `kb_*` (**M-G**).  
- Session pins / recording / transcript (**M-C** / **M-D**).  
- Go store APIs / Inform runtime / FAQ seed data.  
- Root `migrations/` writer.  
- CI.0 workflows.  

## forbidden

- DROP any table.  
- CREATE or ALTER `kb_*`.  
- Edit applied `001`–`010` history.  
- Secrets / API keys in SQL.  
- Bundle flow or session changes into this file.  

## exit_criteria

- [x] `011_binding.sql` present; columns match P2.10.  
- [x] `go test ./internal/store/... -count=1` passes.  
- [x] No DROP / no kb mutate / no root writer.  

## verification

```text
go test ./internal/store/... -count=1
go build ./...
```

## rollback

DROP `binding` only if never written; else snapshot restore.

## handoff

Next: **M-C** — session flow pins (and recording cols as **separate** file if one-concern requires split).
