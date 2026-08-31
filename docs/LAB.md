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
| Overview | Health, store backend (postgres/memory), Sarvam on/off; **CC Quick start** |
| Tenant Engines | `GET`/`PUT /v1/tenant/engines` — single listen/think/speak ids (not failover lists) |
| Gateways | All registered gateway ids by port |
| Profiles | Create/publish; **Contact Agent** presets (Sales / R&D / after-hours) + **legacy** fakes/failover/captions |
| Sessions | Create/stop; show `gateway_binding`; inject text; PATCH language |
| Inspect | Session + audit + analytics + **transcript** + **disposition** + SSE; binding/languages summary |
| Playback | Enqueue playback job |
| API log | Browser-side request/response trail |

---

## Contact Agent CC path

Order for proving the vertical in lab (no FreeSWITCH required):

1. **Set tenant engines** — Tenant Engines tab → suggest fakes (`fake-listen` / `fake-think` / `fake-speak`) or Sarvam trio when `SARVAM_API_KEY` is set → Save PUT. Confirm `source` is `store` (or note env fallback in GET).
2. **Publish ≥2 behaviour presets** — Profiles → CC Sales + CC R&D (and/or after-hours). Each has `metadata.family: contact-agent`. **Do not** use legacy sarvam+fake failover for the CC demo.
3. **Create Talk session** — Sessions → profile_id `cc-sales` (or `cc-rnd`) → Create. Panel shows `gateway_binding` matching tenant engines.
4. **Multi-turn inject** — inject `hello`, then a second turn; Inspect → Transcript shows ordered user/assistant rows.
5. **Language lock / PATCH** — first confident Listen lock sets `active_language` (see LANGUAGE_POLICY); operator `PATCH language.primary` via Sessions helper when `mid_call_switch` + `hot_swap_allowed` allow.
6. **Ladder clip** — inject text matching a clip regex (e.g. `hello` → greeting clip); Analytics `turn_completed` dimensions include `response_tier=clip` (no Think).
7. **Inspect disposition** — stop session; Disposition panel loads `GET …/disposition` when postcall has written a suggestion.

### Engines vs profiles

| Layer | Owns |
|---|---|
| **Tenant engines** | STT / LLM / TTS gateway ids (`SYSTEM_ENGINES.md`) |
| **CC profiles** | Sales / R&D / after-hours behaviour — persona, voice map, language, ladder clips, skills |

Same tenant engines for every CC profile. Profiles differ by rules/persona/voice/clips — not by vendor failover lists.

### Realtime demo (system-bound)

Contact Agent realtime demo uses **session-pinned `gateway_binding`** from tenant engines. Comma failover (`sarvam-stt, fake-listen`, …) is **legacy / non-CC** only (labeled in the Profiles preset dropdown).

### Language lock

See `docs/product/LANGUAGE_POLICY.md`. Lab:

- Auto-detect → lock on first confident Listen final (or inject path when Actor bound).
- Ambient re-detect must not flip `active_language`.
- Operator switch: `PATCH /v1/sessions/{id}/profile-fields` with `{ "language.primary": "hi-IN" }` (Sessions helper).

### Voice / ladder

- Persona `voice` map (or `voice_id`) required for Talk/Speak publish.
- Ladder: clip → template → LLM; clip match skips Think.
- Pinned engines: Think total failure → `fallback.think_down` canned clip + escalate skill — **no** mid-session vendor hop (`CONTACT_AGENT.md` §8).

### Quota / rate-limit UX

| Signal | Expected UX |
|---|---|
| Think `rate_limit` / vendor total failure with `gateway_binding` pinned | `think_down` clip + escalate; API/audit shows no provider-list walk |
| Concurrent session create fairness | `429` when enforced (`OPERATIONS.md` §5) |

Lab proof without live Sarvam 429: `go test` path `TestComposer_ThinkDownClipEscalateNoVendorSwitch` (`./internal/runtime/composer`) and inject + Inspect after forcing Think failure in unit tests. Do not invent new vendor SDKs.

### Secrets

`SARVAM_API_KEY` and other credentials live in `.env` / environment only. Never commit `.env`, `.agent/secrets.local.json`, or API keys.

---

## Smoke checklist (no FreeSWITCH)

1. PUT tenant engines → fakes (or Sarvam single ids).
2. Create + publish `cc-sales` and `cc-rnd` (Contact Agent presets).
3. Create session on `cc-sales` → assert `gateway_binding` equals tenant engines.
4. Inject `hello` → GET transcript (≥1 user + assistant); analytics may show `response_tier=clip`.
5. Inject a second turn → transcript ordered; multi-turn context.
6. PATCH `language.primary` → GET session shows `active_language`.
7. Stop → Inspect audit / analytics / disposition.
8. Optional: repeat create on `cc-rnd` — same binding, different persona/clips.

---

## Scenario checklist (A–D)

| # | Scenario | How to prove |
|---|---|---|
| A | Multi-turn context | Lab: two+ injects → `GET …/transcript` ordered turns. Unit: `go test ./internal/control` (`TestInject_ClipTurnAndTranscript`) |
| B | Language lock | Lab: PATCH language helper + Inspect languages. Unit: `go test ./internal/control` language tests; docs `LANGUAGE_POLICY.md` |
| C | Quota / rate-limit UX | Documented above; unit: `go test ./internal/runtime/composer -run ThinkDown` (pinned `think_down`, no vendor hop) |
| D | Clip / ladder | Lab: inject greeting → analytics `response_tier=clip`. Unit: `go test ./internal/runtime/composer -run ClipPath` |

Thin agent scenarios: `tests/agent/scenarios/F-cc-lab-engines.yaml`, `F-cc-behaviour-presets.yaml` wrap the packages above.

---

## Sarvam (legacy / optional live)

Set `SARVAM_API_KEY` in `.env` / environment. Gateways register as `sarvam-stt`, `sarvam-llm`, `sarvam-tts`.

- **CC path:** put those **single** ids on Tenant Engines (Suggest Sarvam), then use CC behaviour presets.
- **Legacy non-CC:** Profiles preset **sarvam + fake failover** still exists for Phase F–style failover demos — not the Contact Agent default.

Real voice E2E WAV / FreeSWITCH path remains a follow-up wave.
