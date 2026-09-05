# L3 — CD.1 Manual lab-build workflow

| Field | Value |
|---|---|
| **id** | `CD.1` |
| **title** | `workflow_dispatch` lab binary artifacts |
| **status** | **Closed** — lab-build workflow |
| **parent_plan** | [11_CI_AND_CD.md](../11_CI_AND_CD.md) § CD.1 |
| **depends_on** | CD.0 Closed (`9eb1aba`); CI.0 Closed |

## architecture_refs

- [11_CI_AND_CD.md](../11_CI_AND_CD.md) — OD-11-4 no auto-deploy; CD.1 sketch; artifacts table
- [CD.0_lab_promote.md](./CD.0_lab_promote.md) / [tools/lab/PROMOTE.md](../../tools/lab/PROMOTE.md)
- Existing CI: [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml) (merge gate — unchanged)

## goal

Add a **manual-only** GitHub Actions workflow that builds orchestrator binaries and uploads them as Actions artifacts for lab operators — without deploy or prod promotion.

## in_scope

- `.github/workflows/lab-build.yml` with `on: workflow_dispatch` only
- Cross-compile linux + windows amd64 into `dist/`; upload artifact (14-day retention)
- Record tip SHA in artifact
- Point PROMOTE.md at optional artifact download
- Docs: this file + README

## out_scope

- Trigger on push/PR/merge
- Deploy to lab/prod hosts
- Container image / GHCR publish
- Code signing / notarization
- FreeSWITCH / mod builds

## forbidden

- Auto-deploy
- Environment protection bypass to prod
- Embedding secrets in the workflow
- Absorbing unrelated CI changes into Job A–D behaviour

## exit_criteria

- [x] Workflow exists; **only** `workflow_dispatch`
- [x] Builds upload an artifact; no deploy job
- [x] PROMOTE.md mentions optional download
- [x] CI merge workflow unchanged in intent

## verification

```text
# YAML present; trigger is dispatch-only:
rg -n "workflow_dispatch|on:" .github/workflows/lab-build.yml
# No deploy/push steps:
rg -n "deploy|ghcr|docker push|environment:" .github/workflows/lab-build.yml
# expect no matches for deploy paths
```

## handoff

CI/CD V1 wave (**CI.0–CI.3**, **CD.0–CD.1**) complete for programme docs/11.  
Further work: graph/`flow_*` runtime L3, or owner-directed product phases.
