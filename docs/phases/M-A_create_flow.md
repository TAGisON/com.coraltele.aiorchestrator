# L3 — M-A Create `flow_*` registry

| Field | Value |
|---|---|
| **id** | `M-A` |
| **title** | CREATE `flow`, `flow_draft`, `flow_version` |
| **status** | **Closed** — DDL `010_flow_registry.sql` (owner signed 2026-09-05) |
| **parent_plan** | [P2.7](./P2.7_flow_publish_model.md) `migration_plan_note` step 1; [P2.14](./P2.14_migration_ci.md) M-A; [P2.13](./P2.13_drop_obsolete.md) sequence |
| **depends_on** | P2.0–P2.14 Closed; P1 Done; gate §8 Locked |

## architecture_refs

- [P2.7_flow_publish_model.md](./P2.7_flow_publish_model.md) — table field locks  
- [P2.14_migration_ci.md](./P2.14_migration_ci.md) — R1–R7  
- [P2.0_schema_principles.md](./P2.0_schema_principles.md) — P2.0-P10  
- [08](../08_PURGE_AND_SCHEMA_PHASES.md) OD-08-3  

## goal

Apply the first expand migration: create the `flow_*` registry tables so later waves can pin sessions and DROP `desk*` only after readers move.

## in_scope

- New SoT file `internal/store/migrations/010_flow_registry.sql` (next free N per P2.14-R1).  
- CREATE `flow`, `flow_draft`, `flow_version` + `flow_tenant_idx` exactly per P2.7 field tables.  
- Unit test asserting migration SQL contains the three tables.  
- Docs: this phase file + `docs/phases/README.md` M-A row.

## out_scope

- ALTER `session` flow pins (→ **M-C** / P2.7 step 2).  
- CREATE `binding` (**M-B**).  
- Recording / transcript ALTERs (**M-C** / **M-D**).  
- DROP `desk*` / `kb*` (**M-F**…).  
- Go store Repository APIs / runtime / UI.  
- Root `migrations/` copies (P2.14-R1 not a writer).  
- CI workflows (**CI.0**).  

## forbidden

- DROP any table.  
- CREATE `desk_*`.  
- Edit applied `001`–`009` history.  
- Bundle session pins or binding into this file.  
- Insert secrets / seed API keys.  

## exit_criteria

- [x] `010_flow_registry.sql` present under SoT path; columns match P2.7.  
- [x] `go test ./internal/store/... -count=1` passes (integration if `DATABASE_URL` set).  
- [x] No DROP / no session ALTER / no root writer.  

## verification

```text
go test ./internal/store/... -count=1
go build ./...
```

Manual lab (until Job B): empty Postgres + `DATABASE_URL` → Open/ApplyMigrations includes `010_flow_registry.sql`.

## rollback

L4: DROP the three new tables only if never written (expand/contract); prefer restore from snapshot if data exists.

## handoff

Next: **M-B** CREATE `binding` (P2.10), then M-C session pins (+ recording cols per P2.14 map — one concern per file).
