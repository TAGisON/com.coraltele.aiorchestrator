# Agent product validation (universal)

## Split

- **This folder (app repo):** feature **contract** — what to test (FEATURES, catalog, scenarios, fixtures).
- **Evidence worktree:** run **results** —  
  `../com.coraltele.aiorchestrator-validation-evidence` (`INDEX.md` + `rounds/<F-id>/`).

Missing `MANIFEST.yaml` → scenario-planner **blocker** class `spec`.

## Layout (app)

```
tests/agent/
  MANIFEST.yaml
  FEATURES.md
  features/catalog.yaml
  scenarios/F-*.yaml
  fixtures/
  README.md
```

## Round flow

1. `agent.ps1 start -Pipeline product-validation -From <F-id>`  
2. Roles write under `.agent/work/<F-id>/`  
3. Summarizer pass → `Archive-FeatureRound.ps1` → evidence worktree commit  
4. Pipeline **pauses** → you run `agent.ps1 next-feature` when ready  

See `docs/VALIDATION_PIPELINE.md`.
