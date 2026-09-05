# Lab promote checklist

**Programme:** LLM Call Centre orchestrator (`com.coraltele.aiorchestrator`)  
**SoT:** [docs/11_CI_AND_CD.md](../../docs/11_CI_AND_CD.md) CD V1 promote model  
**Phase:** [CD.0](../../docs/phases/CD.0_lab_promote.md)

V1 CD is **human-driven**. There is **no** auto-deploy to lab or prod on merge. Use this list after CI is green so promote does not depend on chat archaeology.

## Preconditions

| # | Check | Pass when |
|---|---|---|
| P1 | CI green on the tip you promote | Jobs **A** `go-build-test`, **B** `migrate-empty`, **C** `secrets-hygiene`, **D** `golangci` all green (or equivalent local commands below) |
| P1b | Optional artifact | Actions → **lab-build** → Run workflow (manual). Download `aiorchestrator-<sha>` (linux + windows). See [CD.1](../../docs/phases/CD.1_lab_build_workflow.md) | Artifact matches tip SHA in `TIP_SHA.txt` |
| P2 | Tip SHA known | Record commit SHA in the sign-off block |
| P3 | Secrets stay local | `.agent/secrets.local.json` exists on the lab host if needed; **never** committed ([CI.2](../../docs/phases/CI.2_secrets_hygiene.md)) |
| P4 | No prod push | This checklist ends at **lab**. Production needs a future OD. |

### Local CI-equivalent (optional before build)

```text
gofmt -l $(git ls-files '*.go')   # expect empty
go build ./...
go test ./... -count=1 -timeout 180s
go test ./internal/store/... -count=1 -timeout 120s   # with DATABASE_URL if Job B parity
go test ./internal/cihygiene/... -count=1
golangci-lint run ./...            # v1.64.8 + .golangci.yml
```

## Promote steps

| # | Step | Command / action | Pass criteria | ☐ |
|---|---|---|---|---|
| 1 | Fetch tip | `git fetch` + checkout the reviewed SHA / branch **or** download Actions artifact from **lab-build** (`workflow_dispatch`) | Working tree / binary matches intended tip | ☐ |
| 2 | Build binary | `go build -o aiorchestrator ./cmd/aiorchestrator` (Windows: `aiorchestrator.exe`) — skip if using lab-build artifact | Binary produced; no build errors | ☐ |
| 3 | Lab DB | `pwsh tools/lab/Init-LabDatabase.ps1` (or existing DB) | DB exists; schema applied on **boot** via migrations (not hand-edited SQL) | ☐ |
| 4 | Config | `conf/aiorchestrator.properties` — `database.url` set; lab may use `database.require=true` | Boot does not exit for missing DB when require=true | ☐ |
| 5 | Secrets | Copy `.agent/secrets.example.json` → `.agent/secrets.local.json` **or** configure tenant credentials via Control API / lab Settings | Sarvam (or fakes) available as intended for this promote | ☐ |
| 6 | Start | `./aiorchestrator` or `go run ./cmd/aiorchestrator` | Process stays up; logs show listen | ☐ |
| 7 | Health | `GET http://127.0.0.1:8011/v1/health` | HTTP 200 when engines/bindings healthy enough for lab (or known degraded policy) | ☐ |
| 8 | Lab UI | Open http://127.0.0.1:8011/lab/ | Page loads; **no** resurrected `/admin` SPA | ☐ |
| 9 | Edge uplink smoke | `go run ./tools/lab/edge_smoke` (profile `coral-tfn` or lab profile must exist) **or** manual WS uplink | Session create + edge WS connect succeeds | ☐ |
| 10 | Recording (E.2+) | Short live/lab call with recording on → stop/Ending | Session has `recording_stopped_at` set; file under `recording_ref` stable ([09](../../docs/09_EVIDENCE_AND_RECORDING.md), [E.6](../../docs/phases/E.6_evidence_soak_checklist.md)) | ☐ |
| 11 | Disposition (E.5+) | After clean stop or transfer path | `final` allowlisted; see E.6 B4 if doing full soak | ☐ |

## Explicit non-goals (do not block promote)

- Graph dialogue / `edge_taken` transcript kinds — may be offline until graph runtime.  
- Auto push to production.  
- Building FreeSWITCH / `mod_audio_stream` inside this repo’s CI (sibling edge repo).  
- Full `tests/agent` cloud farm as a merge gate.

## Rollback (lab)

1. Stop the process.  
2. Redeploy previous known-good binary / tip SHA.  
3. If schema forward-only broke lab: restore DB from backup or `Init-LabDatabase.ps1 -Recreate` (data loss) then boot older tip only if migrations still compatible — prefer forward fix.

## Sign-off

| Field | Value |
|---|---|
| Date | |
| Operator | |
| Tip SHA | |
| CI run URL / local preflight | |
| Steps passed | (list) |
| Deferred / N/A | (list + reason) |
| Notes | |
| Result | **lab promote OK** / **abort** |
