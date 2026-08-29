# Product validation pipeline — operator guide

**Purpose:** Multi-agent QA for any repo that ships `tests/agent/MANIFEST.yaml` — invent scenarios, build fixtures, run harness, validate audit evidence, review, summarize.

**Companion:** Phase build stays on `docs/AGENT_PIPELINE.md` (`coral-phase`). This doc is `product-validation` only.

---

## Universal contract

| Path | Role |
|---|---|
| `tests/agent/MANIFEST.yaml` | Required. Missing → scenario-planner **blocker** `spec` |
| `tests/agent/FEATURES.md` | Human feature inventory — **what** must be tested |
| `tests/agent/features/catalog.yaml` | Machine index (ids, packages, status) |
| `tests/agent/scenarios/F-*.yaml` | One scenario per feature id |
| `tests/agent/fixtures/` | Profiles, audio, transcripts, … |
| Global skills `coral-validation-*` | Drive roles; same skills every repo |

This repo’s V1 wave covers **all must_test features** in FEATURES.md (ports → control → runtime → edges → audit → Sarvam). **No FreeSWITCH** until you provide a call-server endpoint (`F-edge-fs-live` deferred).

Quick harness (no agents):

```powershell
.\tools\agent-runner\Run-FeatureScenarios.ps1
```

---

## Roles

| Role | Skill | Artifact |
|---|---|---|
| scenario-planner | coral-validation-scenario-planner | scenarios.md |
| fixture-builder | coral-validation-fixture-builder | fixtures.md |
| test-runner | coral-validation-test-runner | run-log.md |
| audit-validator | coral-validation-audit-validator | audit-report.md |
| test-reviewer | coral-validation-test-reviewer | review.md |
| test-summarizer | coral-validation-test-summarizer | summary.md |

Fail loops (from `.agent/pipelines/product-validation.json`): reviewer → runner; auditor → runner; runner → fixture-builder; fixture-builder → planner; summarizer → runner.

---

## Start (one feature per round)

Same monitor as coding. Phases are feature ids (`F-ports-contract`, …) in `.agent/pipelines/product-validation.json`.

```powershell
cd C:\Users\user\Documents\GitHub\com.coraltele.aiorchestrator
$env:AGENT_NO_MAIL = "1"
.\tools\agent-runner\agent.ps1 start -Pipeline product-validation -From F-ports-contract
.\tools\agent-runner\agent.ps1 monitor-start
.\tools\agent-runner\agent.ps1 assign-role   # writes NEXT_PROMPT + DISPATCH only
```

Each round: scenario-planner → … → test-summarizer for **that feature only**, then auto-advance to the next `F-*` phase. Trail: `.agent/work/<feature-id>/`.

Parent agent / you run the prompt; monitor advances on `# agent-approval`.

```powershell
.\tools\agent-runner\agent.ps1 start -Pipeline coral-phase -From phase-a
```

---

## Adding a wave later (e.g. FS)

1. Human provides FS/call-server endpoint → record via `decide` or MANIFEST  
2. New phase YAML (e.g. `validation-fs`) + scenarios tagged `requires: [fs_edge]`  
3. Append phase id to `.agent/pipelines/product-validation.json` `phases`  
4. `start -Pipeline product-validation -From validation-fs`
