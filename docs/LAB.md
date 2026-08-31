# Consoles — Admin / Supervisor / User

Three separate UIs replace the old monolithic lab console.

| Console | URL | Purpose |
|---|---|---|
| Portal | http://127.0.0.1:8011/ | Entry + readiness banner |
| **Admin** | http://127.0.0.1:8011/admin/ | Engines, credentials, settings, publish profiles, readiness |
| **Supervisor** | http://127.0.0.1:8011/supervisor/ | Session list, transcript, audit, analytics, disposition |
| **User** | http://127.0.0.1:8011/user/ | Start session, send Talk turns (text inject), stop |

## Fresh install

```powershell
.\tools\lab\Init-LabDatabase.ps1 -Recreate
go run ./cmd/aiorchestrator
```

Boot applies **schema only**. Nothing is pre-seeded.

1. **Admin** → Save tenant engines (fakes offline, or Sarvam + credential) → Create + publish a profile.
2. Confirm readiness checklist is green (`GET /v1/platform/status` → `ready_for_sessions: true`).
3. **User** → Start session → Send turns → Stop.
4. **Supervisor** → Open session → transcript / audit / analytics; disposition appears after postcall.

## Platform status (abnormal conditions)

`GET /v1/platform/status` returns:

- `status`: `ok` | `degraded` | `not_ready` | `unavailable`
- `blockers[]` — must clear before sessions (e.g. `tenant_engines_missing`, `no_profiles`, `database_unreachable`)
- `warnings[]` — startable but risky (e.g. `sarvam_credential_missing`, `store_memory_not_durable`)

UIs surface these as banners and checklists.

## Manual test matrix (edge cases)

| Scenario | Expected |
|---|---|
| Fresh DB, open User → Start | Blocked / Start fails with engines or profiles missing |
| Engines unset, `GET /v1/tenant/engines` | `404` `tenant engines not configured` |
| Engines set to unknown id | `422` on PUT |
| Sarvam engines, no credential | Status `degraded` + warning; turns may fail at vendor |
| No profiles | Status blocker `no_profiles`; User Start fails |
| Session create before engines | `422` with hint to PUT engines |
| Inject on missing/stopped session | Error banner on User |
| Supervisor before any session | Empty list message |
| Transcript mid-call with no turns yet | Empty transcript panel |
| Disposition before stop/postcall | `404` / “not written yet” note |
| Supervisor Force stop | Session terminal; audit `session.terminal` |
| DB down | Health / platform status unavailable; Admin banner |

## Recordings

Orchestrator stores optional `recording_ref` (external URI) only — not call PCM. Supervisor shows whether a ref was set. Live audio path remains FreeSWITCH edge WSS.

## Logging

Rolling JSON under `logs/aiorchestrator/` (see `conf/logging.xml` + properties).
