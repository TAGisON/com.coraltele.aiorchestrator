# 11 — CI and CD

**Status:** **Locked** (owner go-ahead 2026-09-04 — “continue now”).  
**Layer:** L2 domain plan per [07_PLANNING_STANDARDS.md](./07_PLANNING_STANDARDS.md) §3.  
**Parent / architecture refs:** [07](./07_PLANNING_STANDARDS.md), [08](./08_PURGE_AND_SCHEMA_PHASES.md), [10](./10_CODING_PRINCIPLES.md), [P2.14](./phases/P2.14_migration_ci.md), [09](./09_EVIDENCE_AND_RECORDING.md).

**Current repo fact:** no `.github/workflows` present (inventory 2026-09-04). Go module `go 1.22`.

---

## Goal

Define the **minimum trustworthy CI/CD** for this programme so Implementers cannot merge broken builds, bad migrations, or secret leaks — without inventing a heavy multi-SKU platform pipeline.

---

## In scope

- PR / main continuous integration checks.  
- Migration apply-on-empty-Postgres rule (expands [P2.14](./phases/P2.14_migration_ci.md)).  
- Secrets scanning expectations.  
- Lab vs “promote” CD boundaries for V1.  
- Future L3 ids **CI.*** / **CD.*** for when workflows are implemented.  

## Out of scope

- Multi-region blue/green / canary productization.  
- Building FreeSWITCH / `mod_audio_stream` inside this repo’s CI (sibling edge repo).  
- Live Sarvam calls in CI (cost/flakes) — use fakes/unit tests.  
- Resurrecting old `.agent` phase YAML as CI (P1.9 deletes them).  
- Full `tests/agent` product-validation cloud farm in V1 CI (optional later job).  

## Open decisions

| ID | Question | Decision |
|---|---|---|
| **OD-11-1** | CI host | **SETTLED: GitHub Actions** on this GitHub repo (add `.github/workflows/` at L4). |
| **OD-11-2** | Required OS in CI | **SETTLED: `ubuntu-latest`** only for V1. Windows lab remains manual ([tools/lab](../tools/lab/)). |
| **OD-11-3** | Postgres in CI | **SETTLED:** service container Postgres for migration job; default unit tests may use memory store where already supported. |
| **OD-11-4** | Auto-deploy on merge to main | **SETTLED: No** for V1. CD = documented promote checklist + optional future workflow with explicit approval. |
| **OD-11-5** | golangci-lint required Day-1 | **SETTLED: No** for first CI wave — `gofmt` check + `go test` first; lint optional follow-up phase **CI.3**. |

---

## CI — required checks (V1)

### Job A — `go-build-test`

| Step | Command / rule |
|---|---|
| Checkout | actions/checkout |
| Setup Go | Go **1.22.x** matching `go.mod` |
| Format | Fail if `gofmt -l` reports files |
| Build | `go build ./...` |
| Test | `go test ./... -count=1` |

**Must pass** on every PR and on `main`.  
Aligns with most P1/P2/E phase `verification` blocks ([10](./10_CODING_PRINCIPLES.md) C10-P13).

### Job B — `migrate-empty`

| Step | Rule |
|---|---|
| Start Postgres | Service container (e.g. `postgres:16`) |
| Apply | Run store migrate path used by `store.Open` against empty DB — SoT `internal/store/migrations/` ([P2.14-R1](./phases/P2.14_migration_ci.md)) |
| Verify | Existing migrate tests and/or small CI helper exit 0 |
| Forbidden | Merge if empty apply fails |

**Trigger:** any PR that touches `internal/store/migrations/**` **or** always (prefer **always** once cheap — avoids missed dependency). Planning lock: **always run Job B**.

### Job C — `secrets-hygiene`

| Check | Rule |
|---|---|
| Block commit of `.agent/secrets.local.json` | Fail if file present in tree |
| Block obvious key patterns in diff (best-effort) | Optional Step Security / gitleaks — **CI.2** may add; V1 minimum = path deny + `go test` scenarios that already lock no-secrets-in-git if retained after P1.11 |

### Job D — `docs-agent-index` (optional light)

| Check | Rule |
|---|---|
| AGENTS.md / docs README still list 01–11 | Soft: manual review OK for V1; automate later |

**Not required Day-1.**

---

## What CI must **not** do

- Call live Sarvam / Coral / FreeSWITCH.  
- Require Admin/User/Supervisor UI (gone after P1).  
- Apply migrations by hand-editing history files.  
- Publish Docker images to production without human approval (OD-11-4).  
- Run deleted desk publish scripts.  

---

## CD — V1 promote model

```text
PR → CI Jobs A+B(+C) green → review → merge main
  → Lab promote (human): build binary / compose on lab host
  → Smoke: health + edge uplink (manual checklist)
  → No automatic prod push in V1
```

### Lab promote checklist (document for humans / Summarizer)

1. `go build -o aiorchestrator ./cmd/aiorchestrator`  
2. Migrations applied via boot (`database.require=true` in lab if using Postgres).  
3. Health endpoint OK.  
4. After P1: confirm no `/admin` SPA; dialogue may be offline until graph runtime.  
5. After E.2: short call → recording `stopped_at` set ([09](./09_EVIDENCE_AND_RECORDING.md)).  

### Production

- Out of V1 automation. Any prod path needs a **future OD** and approved CD workflow with environment protection.

---

## Branch / PR policy (planning)

| Rule | Detail |
|---|---|
| Implementation branch | Prefer phase-scoped branches; docs already on `docs/llm-callcentre-architecture` |
| Required status checks | Jobs A + B before merge to `main` (once workflows exist) |
| Phase PRs | One L3 phase id in title/body ([07](./07_PLANNING_STANDARDS.md), [10](./10_CODING_PRINCIPLES.md)) |
| Docs-only PRs | Job A still runs (cheap); Job B still runs |

---

## Artifacts

| Artifact | V1 |
|---|---|
| CI logs | GitHub Actions |
| Coverage | Optional later — not gate |
| Release binaries | Manual / tagged release Later |
| Container image | Optional Later — not required to start P1 L4 |

---

## Anti-patterns

- “CI = only my laptop.”  
- Skipping migrate job because “SQL looks fine.”  
- Flaky live-vendor jobs blocking merges.  
- Auto-deploy desk presets or old agent pipelines.  
- One workflow that builds FS + orch + GUI monorepo.  

---

## Phase breakdown (future L3)

Execute only after [07 §8](./07_PLANNING_STANDARDS.md) gate + Implementer skill. Full §4 files under `docs/phases/` when this doc is Locked.

| id | title | goal | exit_criteria (sketch) |
|---|---|---|---|
| **CI.0** | Workflow skeleton | Add `.github/workflows/ci.yml` with Job A | PR shows green build/test |
| **CI.1** | Migrate-empty job | Job B with Postgres service | PR touching migrations (and always) green |
| **CI.2** | Secrets hygiene | Job C path deny (+ optional gitleaks) | Planted secret file fails CI |
| **CI.3** | Optional lint | golangci-lint config | Fail on issues; may be warn-only first |
| **CD.0** | Lab promote doc | Checklist in docs or `tools/lab/PROMOTE.md` | Owner can promote without chat archaeology |
| **CD.1** | Optional manual workflow | `workflow_dispatch` build artifact | No auto prod |

---

## Relationship to agent harness

- `tests/agent/**` and `tools/agent-runner` remain **lab/validation** tools.  
- Not a merge gate in V1 CI unless a later OD promotes a thin subset.  
- After P1.9/P1.11, harness must not depend on deleted desk phases/pipelines.

---

## Handoff

When status → **Locked** (owner sign-off):

1. Implementer skill cites Jobs A/B as default verification for code phases.  
2. Create `docs/phases/CI.0_…` … as needed at L4 time.  
3. Next: owner Lock of docs 09–12; then L4.  
4. Do **not** implement CI.0 until gate complete.

**Not started:** workflow YAML, CD automation.
