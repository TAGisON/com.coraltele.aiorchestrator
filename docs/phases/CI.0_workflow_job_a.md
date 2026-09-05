# L3 — CI.0 Workflow skeleton (Job A)

| Field | Value |
|---|---|
| **id** | `CI.0` |
| **title** | GitHub Actions workflow with Job A `go-build-test` |
| **status** | **Closed** — Job A + refreshCfg MergeRefresh |
| **parent_plan** | [11_CI_AND_CD.md](../11_CI_AND_CD.md) § CI Job A / phase CI.0 |
| **depends_on** | Doc 11 Locked; [07](../07_PLANNING_STANDARDS.md) §8 gate; P2.14 Closed |

## architecture_refs

- [11_CI_AND_CD.md](../11_CI_AND_CD.md) — Job A, OD-11-1/2/5
- [10_CODING_PRINCIPLES.md](../10_CODING_PRINCIPLES.md) — verify defaults
- [P2.14_migration_ci.md](./P2.14_migration_ci.md) — Job B still deferred to CI.1

## goal

Add the first CI workflow so every PR/main run builds and unit-tests without live vendor calls.

## in_scope

- `.github/workflows/ci.yml` with job `go-build-test` (Job A)
- Steps: checkout, setup-go 1.22.x, `gofmt -l` fail-on-dirty, `go build ./...`, `go test ./... -count=1`
- Ensure CI env does not set `SARVAM_API_KEY` (live tests stay skipped)
- `gofmt -w` on tree if needed so Job A can pass
- Docs: this file + README CI.* row

## out_scope

- Job B Postgres migrate (**CI.1**)
- Job C secrets (**CI.2**)
- golangci-lint (**CI.3**)
- CD / deploy
- Live Sarvam / FreeSWITCH in CI

## forbidden

- Auto-deploy
- Calling live vendors in CI
- Editing migration SQL history
- Absorbing CI.1 Job B into this phase

## exit_criteria

- [x] `.github/workflows/ci.yml` exists with Job A steps per doc 11
- [x] Local: `gofmt -l` clean; `go build ./...`; `go test ./... -count=1` (no SARVAM key)
- [x] No Job B / Postgres service in this phase’s workflow yet

## verification

```text
gofmt -l $(git ls-files '*.go')
go build ./...
$env:SARVAM_API_KEY=''; go test ./... -count=1
```

## rollback

Delete `.github/workflows/ci.yml`; revert gofmt-only commits if undesired.

## handoff

Next: **CI.1** Job B migrate-empty.
