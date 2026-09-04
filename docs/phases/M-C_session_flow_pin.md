# L3 — M-C Session flow pins

| Field | Value |
|---|---|
| **id** | `M-C` |
| **title** | ALTER session ADD `flow_id` / `flow_version` |
| **status** | **Closed** — DDL `012_session_flow_pin.sql` |
| **parent_plan** | [P2.7](./P2.7_flow_publish_model.md) step 2; [P2.2](./P2.2_session.md); [P2.14](./P2.14_migration_ci.md) M-C (pins only — recording = **M-Cr**) |
| **depends_on** | M-B Closed (`3534682`); M-A Closed |

## architecture_refs

- [P2.7_flow_publish_model.md](./P2.7_flow_publish_model.md)  
- [P2.2_session.md](./P2.2_session.md)  
- [P2.14_migration_ci.md](./P2.14_migration_ci.md) R1–R7  
- [P2.0_schema_principles.md](./P2.0_schema_principles.md) P2.0-P10  

## goal

Additive session pins for published flow versions — expand only; no DROP; no recording cols in this file.

## in_scope

- `internal/store/migrations/012_session_flow_pin.sql`  
- `ALTER TABLE session ADD` nullable `flow_id` TEXT, `flow_version` INT  
- Unit test for this migration  
- Docs: this file + README row  

## out_scope

- Recording lifecycle cols (**M-Cr** / P2.5)  
- Transcript expand (**M-D**)  
- DROP desk/kb (**M-F**…)  
- Go Session struct / Repository writers  
- Root `migrations/`  

## forbidden

- DROP / desk CREATE  
- Bundling recording or transcript into this file  
- Edit `001`–`011` history  
- Secrets  

## exit_criteria

- [x] `012_session_flow_pin.sql` matches P2.7 pin columns  
- [x] `go test ./internal/store/... -count=1` passes  
- [x] No recording cols / no DROP in this file  

## verification

```text
go test ./internal/store/... -count=1
go build ./...
```

## handoff

Next phase after close: **M-Cr** recording lifecycle columns.
