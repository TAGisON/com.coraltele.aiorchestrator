# L3 — M-Cr Session recording lifecycle columns

| Field | Value |
|---|---|
| **id** | `M-Cr` |
| **title** | ALTER session ADD recording lifecycle columns |
| **status** | **Closed** — DDL `013_session_recording_lifecycle.sql` |
| **parent_plan** | [P2.5](./P2.5_recording_metadata.md) migration_plan_note; [P2.14](./P2.14_migration_ci.md) R2 split from M-C |
| **depends_on** | M-C Closed (`77a59ce`) |

## architecture_refs

- [P2.5_recording_metadata.md](./P2.5_recording_metadata.md)  
- [P2.14_migration_ci.md](./P2.14_migration_ci.md)  
- [P2.0_schema_principles.md](./P2.0_schema_principles.md) P2.0-P6 / P10  

## goal

Additive recording lifecycle stamps on `session` — no PCM; no transcript/DROP.

## in_scope

- `internal/store/migrations/013_session_recording_lifecycle.sql`  
- ADD: `recording_started_at`, `recording_stopped_at`, `recording_stop_reason`, `recording_bytes` per P2.5  
- Unit test  
- Docs: this file + README  

## out_scope

- Runtime stop-on-Ending (E.2)  
- Transcript expand (**M-D**)  
- Go Session field wiring  
- Root `migrations/`  

## forbidden

- BYTEA / PCM columns  
- DROP  
- Bundle transcript or flow pins  
- Edit `001`–`012` history  

## exit_criteria

- [x] `013` columns match P2.5  
- [x] `go test ./internal/store/...` passes  
- [x] No PCM / no DROP / no transcript  

## verification

```text
go test ./internal/store/... -count=1
go build ./...
```

## handoff

Next: **M-D** transcript_turn expand.
