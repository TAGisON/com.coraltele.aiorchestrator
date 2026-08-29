# Lab console — Postgres + POC UI

## Database (telemetry-style)

Shared local Postgres `127.0.0.1:5432`, **new DB name** `aiorchestrator` (same pattern as `telemetry`, `users`, `switch`).

```powershell
.\tools\lab\Init-LabDatabase.ps1
copy .env.example .env
# edit .env if needed — set SARVAM_API_KEY when testing real vendors
go run ./cmd/aiorchestrator
```

Open **http://127.0.0.1:8080/lab/**

Migrations apply automatically on boot when `DATABASE_URL` is set.

`REQUIRE_DATABASE=1` refuses to start on in-memory (recommended for lab).

## Logging

- Process logs: structured **JSON** on stdout (`LOG_FORMAT=json`, `LOG_LEVEL=info|debug|warn|error`)
- Every HTTP request: `method`, `path`, `status`, `duration_ms`
- Observe path: audit/analytics failures logged fail-open (never break media)

## Lab UI capabilities

| Tab | What you can do |
|---|---|
| Overview | Health, store backend (postgres/memory), Sarvam on/off |
| Gateways | All registered gateway ids by port |
| Profiles | Create, publish (fakes / Sarvam failover / captions presets), list from DB |
| Sessions | Create/stop, list from DB |
| Inspect | Session JSON + audit + analytics + SSE live tail |
| Playback | Enqueue playback job |
| API log | Browser-side request/response trail |

## Sarvam

Set `SARVAM_API_KEY` in `.env` / environment. Gateways register as `sarvam-stt`, `sarvam-llm`, `sarvam-tts`. Use the **sarvam + fake failover** profile preset in the UI. Real voice E2E WAV path is still a follow-up wave.
