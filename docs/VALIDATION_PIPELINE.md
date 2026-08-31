# Product validation pipeline — operator guide

**Purpose:** Multi-agent QA for feature contracts in `tests/agent/` — one feature per round, then pause. Run trails are archived outside this app repo.

**Companion:** Phase build stays on `docs/AGENT_PIPELINE.md` (`coral-phase`).

---

## Split: contract vs evidence

| Location | What lives here |
|---|---|
| **This app repo** `tests/agent/` | Feature contract only: FEATURES.md, catalog, scenarios, fixtures |
| **Evidence worktree** `../com.coraltele.aiorchestrator-validation-evidence` | Run trails: scenarios.md, run-log, audit, review, summary, INDEX |

Open dashboard: `com.coraltele.aiorchestrator-validation-evidence/INDEX.md`  
Branch: `validation-evidence` (orphan worktree).

---

## Universal contract (app)

| Path | Role |
|---|---|
| `tests/agent/MANIFEST.yaml` | Required |
| `tests/agent/FEATURES.md` | What must be tested |
| `tests/agent/features/catalog.yaml` | Machine index |
| `tests/agent/scenarios/F-*.yaml` | One scenario definition per feature |
| Global skills `coral-validation-*` | Drive roles |

---

## Roles (artifacts written under `.agent/work/<F-id>/`, then archived)

| Role | Skill | Artifact |
|---|---|---|
| scenario-planner | coral-validation-scenario-planner | scenarios.md |
| fixture-builder | coral-validation-fixture-builder | fixtures.md |
| test-runner | coral-validation-test-runner | run-log.md |
| audit-validator | coral-validation-audit-validator | audit-report.md |
| test-reviewer | coral-validation-test-reviewer | review.md |
| test-summarizer | coral-validation-test-summarizer | summary.md |

---

## Start one feature, then pause

```powershell
cd C:\Users\user\Documents\GitHub\com.coraltele.aiorchestrator
$env:AGENT_NO_MAIL = "1"
.\tools\agent-runner\agent.ps1 start -Pipeline product-validation -From F-ports-contract
.\tools\agent-runner\agent.ps1 monitor-start
.\tools\agent-runner\agent.ps1 assign-role
```

On summarizer **pass**: trail is copied into the evidence worktree and committed there; pipeline state becomes **`waiting_human`** (does not start the next feature).

```powershell
.\tools\agent-runner\agent.ps1 next-feature
# or: .\tools\agent-runner\agent.ps1 next-feature -From F-fake-trio
```

`pause_after_phase: true` is set in `.agent/pipelines/product-validation.json`.

---

## Validation V1 wave (Control + memory umbrella)

Single-phase pipeline for lab product+audit proof (no FS):

```powershell
.\tools\agent-runner\agent.ps1 start -Pipeline validation-v1 -From validation-v1
```

| Piece | Path |
|---|---|
| Phase | `.agent/phases/validation-v1.yaml` |
| Pipeline | `.agent/pipelines/validation-v1.json` |
| Scenarios | `.agent/work/validation-v1/scenarios.md` |
| Harness | `go test ./internal/validation` / `scripts/validation/Run-ValidationV1.ps1` |
| Feature id | `F-validation-v1` in `tests/agent/` |

Ops detail: `docs/architecture/OPERATIONS.md` §11.

---

## Adding FS later

1. Human provides call-server endpoint  
2. Flip `F-edge-fs-live` to must_test / add phase  
3. `next-feature -From F-edge-fs-live`
