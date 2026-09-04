# L3 — M-H DROP skill/compliance tables

| Field | Value |
|---|---|
| **id** | `M-H` |
| **title** | DROP `skill_invocation`, `pii_access_audit`, `erasure_request`, `consent_record` |
| **status** | **Closed** — DDL `017_drop_compliance.sql` |
| **parent_plan** | [P2.13](./P2.13_drop_obsolete.md) M-H; [P2.14](./P2.14_migration_ci.md) |
| **depends_on** | M-E Closed (`24aefde`); M-G Closed (`89cab6c`); unused-confirm |

## architecture_refs

- [P2.13_drop_obsolete.md](./P2.13_drop_obsolete.md)
- [P2.14_migration_ci.md](./P2.14_migration_ci.md)

## goal

Contract step: drop unused skill ledger and compliance tables after store API removal (M-E) and unused-confirm.

## in_scope

- `internal/store/migrations/017_drop_compliance.sql`
- DROP: `skill_invocation`, `pii_access_audit`, `erasure_request`, `consent_record`
- Unit test
- Docs: this file + README
- Record unused-confirm evidence

## out_scope

- DROP desk* / kb* (M-F / M-G, Closed)
- Editing `008_contact_desk.sql` history
- Root `migrations/`
- Resurrecting consent/erasure product features

## forbidden

- Edit `008` history
- Bundle desk or kb DROP
- DROP while Go readers remain

## unused_confirm (2026-09-05)

- [x] `rg` skill_invocation / AppendSkillInvocation → no production Go readers (only migrate_test banned list + `008` CREATE history)
- [x] `rg` erasure/consent/pii_access → no production Go readers (comment-only in memory_session_aux)
- [x] Control HTTP compliance routes gone (P1.6 / M-E)
- [x] Owner ack via DDL auto-loop continue through M-H

## exit_criteria

- [x] `017` drops four tables
- [x] Scoped store test passes
- [x] No desk/kb DROP in this file

## verification

```text
go test ./internal/store/... -count=1
```

## handoff

DDL contract wave M-A..M-H complete after commit.
