# L3 — M-D Expand `transcript_turn`

| Field | Value |
|---|---|
| **id** | `M-D` |
| **title** | ALTER `transcript_turn` expand + drop pair UNIQUE |
| **status** | **Closed** — DDL `014_transcript_turn_expand.sql` |
| **parent_plan** | [P2.3](./P2.3_transcript_events.md) migration_plan_note; [P2.14](./P2.14_migration_ci.md) M-D |
| **depends_on** | M-Cr Closed (`3b30daf`) |

## architecture_refs

- [P2.3_transcript_events.md](./P2.3_transcript_events.md)  
- [P2.14_migration_ci.md](./P2.14_migration_ci.md)  
- [P2.0_schema_principles.md](./P2.0_schema_principles.md) P2.0-P4 / P10  

## goal

One-concern expand: add event columns, backfill legacy rows, drop pair UNIQUE, allow NULL `turn_id`; keep `(session_id, seq)` unique.

## in_scope

- `internal/store/migrations/014_transcript_turn_expand.sql`  
- Unit test  
- Docs: this file + README  

## out_scope

- Runtime emitters (E.*)  
- Go `TranscriptTurn` struct reshape  
- DROP desk/kb  
- Root `migrations/`  

## forbidden

- PCM in `payload`  
- DROP unrelated tables  
- Edit `006` history  
- Bundle session/recording changes  

## exit_criteria

- [x] `014` implements P2.3 steps 1–5  
- [x] `go test ./internal/store/...` passes  
- [x] Pair UNIQUE dropped; seq UNIQUE kept  

## verification

```text
go test ./internal/store/... -count=1
go build ./...
```

## handoff

Next: **M-E** reader cutover (not SQL).
