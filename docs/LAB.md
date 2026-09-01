# Consoles — Admin / Supervisor / User

Three separate UIs replace the old monolithic lab console.

| Console | URL | Purpose |
|---|---|---|
| Portal | http://127.0.0.1:8011/ (or `:8012` if 8011 is an older binary) | Entry + readiness banner |
| **Admin** | http://127.0.0.1:8011/admin/ | Engines, credentials, **Contact desks** (EN/HI editor), readiness |
| **Supervisor** | http://127.0.0.1:8011/supervisor/ | Session list, attributes, handoff, transcript, disposition |
| **User** | http://127.0.0.1:8011/user/ | **Text call** on a published desk (EN/HI) and live voice |

If port 8011 is already taken by an older process, start a current build with `HTTP_ADDR=:8012`. Install and smoke-test the Coral TFN desk with `.\tools\lab\Setup-CoralDesk.ps1 -Base http://127.0.0.1:8012`.

## FreeSWITCH edge (VoIP)

Telephony give/take lives in the **separate repo** [`mod_audio_stream-1`](https://github.com/TAGisON/mod_audio_stream-1) — clone as a **sibling** of this repo (not a submodule):

```text
GitHub/
  com.coraltele.aiorchestrator/   ← this repo (Go orchestrator)
  mod_audio_stream-1/             ← FreeSWITCH module + fs/ Lua dialplan
```

Deploy Lua + dialplan from the sibling clone:

```powershell
# From your machine (adjust sipserver path)
copy ..\mod_audio_stream-1\fs\ai_voice_bot.lua \\sipserver\...\scripts\
copy ..\mod_audio_stream-1\fs\ai_profiles.conf \\sipserver\...\scripts\
```

See `../mod_audio_stream-1/fs/README.md` (or `fs/README.md` inside that repo) for sipserver install, DID map (`101=coral-tfn`), and `ai_orch_url` if the orchestrator IP differs from `192.168.25.130:8011`.

Set `edge.base_url` in `conf/aiorchestrator.properties` to the **same host IP** FreeSWITCH uses for WSS (`ws://<ip>:8011/edge/fs`).

## Fresh install

```powershell
.\tools\lab\Init-LabDatabase.ps1 -Recreate
go run ./cmd/aiorchestrator
```

Boot applies **schema only**. Nothing is pre-seeded.

1. **Admin** → Save tenant engines (fakes offline, or Sarvam + credential).
2. **Admin → Contact desks** → **Install Coral TFN preset** → edit prompts in English/Hindi if needed → **Publish desk**.
3. Confirm readiness checklist is green (`GET /v1/platform/status` → `ready_for_sessions: true`).
4. **User → Text call** → pick `coral-tfn`, open in English or हिंदी, start a call. Voice call uses the published `coral-tfn` profile.
5. **Supervisor** → Open session / inspect attributes, handoff, disposition.

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
