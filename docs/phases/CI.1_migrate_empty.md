# L3 — CI.1 Migrate-empty (Job B)

| Field | Value |
|---|---|
| **id** | `CI.1` |
| **title** | Job B `migrate-empty` with Postgres service |
| **status** | **Closed** — Job B migrate-empty |
| **parent_plan** | [11_CI_AND_CD.md](../11_CI_AND_CD.md) § Job B / phase CI.1 |
| **depends_on** | CI.0 Closed; P2.14-R4; SoT migrations through `017`

## architecture_refs

- [11_CI_AND_CD.md](../11_CI_AND_CD.md) — Job B always-run
- [P2.14_migration_ci.md](./P2.14_migration_ci.md) — R1/R4
- [10_CODING_PRINCIPLES.md](../10_CODING_PRINCIPLES.md)

## goal

Always apply embedded SoT migrations on empty Postgres in CI via existing `TestApplyMigrations_Integration` / store Open path.

## in_scope

- Extend `.github/workflows/ci.yml` with job `migrate-empty`
- Postgres 16 service container; `DATABASE_URL` set
- Run `go test ./internal/store/... -count=1` (includes integration when URL set)
- Docs: this file + README

## out_scope

- Job C secrets (**CI.2**)
- Changing migration SQL
- Root `migrations/` writer

## forbidden

- Live vendor calls
- Editing applied migration history
- Skipping Job B on non-migration PRs (always run)

## exit_criteria

- [x] Job B present; always runs on push/PR
- [x] Empty DB apply succeeds in CI design (integration test with DATABASE_URL)
- [x] Job A unchanged in intent

## verification

```text
# with Postgres + DATABASE_URL
go test ./internal/store/... -count=1
```

## rollback

Remove Job B from workflow; keep Job A.

## handoff

Next: **CI.2** secrets hygiene or **E.1** transcript emitter.
