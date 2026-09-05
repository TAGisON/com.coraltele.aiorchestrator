# L3 — CD.0 Lab promote checklist

| Field | Value |
|---|---|
| **id** | `CD.0` |
| **title** | Lab promote checklist (human CD) |
| **status** | **Closed** — PROMOTE.md authored |
| **parent_plan** | [11_CI_AND_CD.md](../11_CI_AND_CD.md) § CD V1 / phase CD.0 |
| **depends_on** | CI.0–CI.3 Closed; Doc 11 Locked |

## architecture_refs

- [11_CI_AND_CD.md](../11_CI_AND_CD.md) — CD model; OD-11-4 no auto-deploy; lab promote sketch
- [09_EVIDENCE_AND_RECORDING.md](../09_EVIDENCE_AND_RECORDING.md) — recording smoke after E.2
- [E.6_evidence_soak_checklist.md](./E.6_evidence_soak_checklist.md) — deeper evidence soak (optional after promote)
- Existing: `tools/lab/Init-LabDatabase.ps1`, `tools/lab/edge_smoke`, `conf/aiorchestrator.properties`

## goal

Publish a durable **lab promote** checklist so an operator can build, boot, and smoke the orchestrator on lab without reconstructing steps from chat.

## in_scope

- `tools/lab/PROMOTE.md` — preconditions, steps 1–11, non-goals, rollback, sign-off
- This phase file + `docs/phases/README.md` CD.0 row
- Cite CI Jobs A–D and post-E.2 recording check

## out_scope

- GitHub `workflow_dispatch` artifact build (**CD.1**)
- Auto-deploy / environment protection / prod path
- FreeSWITCH / mod build in this repo
- Running a full E.6 soak as a required promote gate (linked optional)

## forbidden

- Auto-deploy on merge
- Committing secrets into the checklist or repo
- Absorbing CD.1

## exit_criteria

- [x] `tools/lab/PROMOTE.md` exists with build → health → edge → recording steps
- [x] Sign-off block present
- [x] Explicit “no prod / no auto-deploy”
- [x] No product code required

## verification

```text
Test-Path tools/lab/PROMOTE.md
```

## handoff

Next: **CD.1** optional manual `workflow_dispatch` build artifact — **Closed** when workflow lands; see [CD.1](./CD.1_lab_build_workflow.md).
