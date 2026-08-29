# Agent product validation (universal)

Any compatible repo can be driven by global skills `coral-validation-*` if this tree exists.

## Layout

```
tests/agent/
  MANIFEST.yaml           # boot, harness, catalog pointers
  FEATURES.md             # human feature inventory (what to test)
  features/catalog.yaml   # machine index (ids, packages, status)
  scenarios/F-*.yaml      # one scenario per feature id
  fixtures/               # profiles, audio, …
  results/                # run outputs
  README.md
```

Missing `MANIFEST.yaml` or empty catalog → scenario-planner **blocker** class `spec`.

## Tester workflow

1. Read `FEATURES.md` + `features/catalog.yaml`  
2. Ensure every `must_test` feature has `scenarios/<id>.yaml`  
3. fixture-builder fills gaps under `fixtures/`  
4. test-runner executes scenarios (or `Run-FeatureScenarios.ps1`)  
5. audit-validator checks evidence kinds on E-* features  
6. test-reviewer challenges coverage vs catalog  
7. test-summarizer closes the wave  

## Scenario schema

See any `scenarios/F-*.yaml`. Required keys: `id`, `feature_id`, `title`, `status`, `what`, `how`, `steps`, `expect`.

Statuses: `must_test` | `optional_live` | `deferred` | `out_of_scope_v1`

## This product

Full inventory: [FEATURES.md](FEATURES.md). Operator guide: `docs/VALIDATION_PIPELINE.md`.
