# Control API — Coral-facing OpenAPI shape

**Status:** LOCKED (route + body shape; exact auth header still open)  
**Date:** 27 August 2026 (tenant engines 31 August 2026)  
**Parents:** `SOLUTION.md` §11, `PLATFORM_FIRST.md`, `PROFILE_SCHEMA.md`, `docs/product/SYSTEM_ENGINES.md`

This is the **HTTP surface Coral Java / admin / dialplan helpers** call. It is independent of Listen/Speak vendors. Publish as OpenAPI 3 when coding `internal/control`.

Auth: Coral estate token (middleware). Every request carries or resolves `tenant_id` + optional `coral_user_id`. Tenant scope for engines also accepts `X-Tenant-ID` (lab).

Base path: `/v1` (versioned so vendors never force a breaking API).

---

## 1. Routes

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/v1/health` | Process + PG; optional gateway summary |
| `GET` | `/v1/tenant/engines` | Active Listen/Think/Speak gateway ids for tenant |
| `PUT` | `/v1/tenant/engines` | Upsert tenant engines (validate registry ids) |
| `GET` | `/v1/tenant/config` | Bundle: engines + credentials (masked) + settings |
| `GET` | `/v1/tenant/credentials` | List gateway credentials (keys masked) |
| `GET` | `/v1/tenant/credentials/{gateway_id}` | One credential (masked) |
| `PUT` | `/v1/tenant/credentials/{gateway_id}` | Upsert API key (+ optional extra JSON) |
| `GET` | `/v1/tenant/settings` | List string settings |
| `GET` | `/v1/tenant/settings/{key}` | Get setting |
| `PUT` | `/v1/tenant/settings/{key}` | Upsert setting (`coral.base_url`, `control.auth_token`, …) |
| `POST` | `/v1/sessions` | Create session; pin profile + `gateway_binding`; optional edge token |
| `GET` | `/v1/sessions/{id}` | Status including `gateway_binding` |
| `POST` | `/v1/sessions/{id}/attachments` | Bind feeder/sink metadata (non-FS) |
| `POST` | `/v1/sessions/{id}/inject` | Text in (Speak / push) |
| `PATCH` | `/v1/sessions/{id}/profile-fields` | Hot-swap allowed fields only |
| `POST` | `/v1/sessions/{id}/stop` | Drain → terminal |
| `GET` | `/v1/sessions/{id}/events` | SSE event stream |
| `GET` | `/v1/sessions/{id}/transcript` | Ordered durable transcript turns |
| `GET` | `/v1/sessions/{id}/disposition` | AI disposition suggestion (postcall write) |
| `POST` | `/v1/jobs/playback` | Enqueue playback |
| `GET` | `/v1/jobs/{id}` | Job state |
| `GET` | `/v1/profiles` | List (tenant-scoped) |
| `POST` | `/v1/profiles` | Create draft |
| `POST` | `/v1/profiles/{id}/versions` | Publish immutable version |
| `GET` | `/v1/profiles/{id}/versions/{ver}` | Get pinned document |
| `POST` | `/v1/kb/documents` | Upload for ingest Knowledge |
| `GET` | `/v1/kb/documents/{id}` | Status / metadata |

WSS edge (not REST): `GET /edge/fs?token=…` — see `EDGE_FS.md`.

---

## 2. Core request/response shapes

### `GET /v1/tenant/engines`

Resolves tenant from auth or `X-Tenant-ID` (lab default tenant id `default`). Returns active gateway ids from DB (`tenant_engines`) only. **No boot/SQL/env seed** — if no row exists → `404` `not_found` with hint to `PUT /v1/tenant/engines`.

```json
{
  "tenant_id": "default",
  "listen": "sarvam-stt",
  "think": "sarvam-llm",
  "speak": "sarvam-tts",
  "source": "store"
}
```

`source` is always `store` when configured. Fresh install has no engines until an operator (or coral-file UI) PUTs them.

### `PUT /v1/tenant/engines`

```json
{
  "listen": "sarvam-stt",
  "think": "sarvam-llm",
  "speak": "sarvam-tts"
}
```

Validates each id against the process gateway registry (correct port). Unknown or wrong-port id → `422` + `bad_request` (or `profile_invalid` if treated as config invalid). Returns the stored binding with `source: "store"`.

### `GET /v1/tenant/config`

Single payload for admin / coral-file settings screens: engines + masked credentials + string settings.

```json
{
  "tenant_id": "default",
  "engines": { "tenant_id": "default", "listen": "sarvam-stt", "think": "sarvam-llm", "speak": "sarvam-tts", "source": "store" },
  "credentials": [
    { "tenant_id": "default", "gateway_id": "sarvam", "api_key_set": true, "api_key_preview": "****abcd", "updated_at": "…" }
  ],
  "settings": [
    { "tenant_id": "default", "key": "coral.base_url", "value": "https://…", "updated_at": "…" }
  ]
}
```

Raw API keys are never returned.

### `PUT /v1/tenant/credentials/{gateway_id}`

Upsert vendor secret used at runtime (e.g. `sarvam`, or per-adapter ids). Body:

```json
{ "api_key": "…", "extra": {} }
```

Response is masked (`api_key_set`, `api_key_preview`). Lab UI / coral-file call this instead of editing `.env`.

### `PUT /v1/tenant/settings/{key}`

Upsert string setting (`coral.base_url`, `control.auth_token`, …). Body: `{ "value": "…" }`.

### `POST /v1/sessions`

```json
{
  "profile_id": "contact-agent-inbound",
  "profile_version": "latest",
  "clock": "live",
  "caller": { "ani": "+9198…", "channel_id": "…" },
  "recording_ref": "fs://uuid/recording.wav",
  "metadata": {}
}
```

```json
{
  "session_id": "01H…",
  "profile_id": "contact-agent-inbound",
  "profile_version": 3,
  "clock": "live",
  "canonical_sample_rate_hz": 16000,
  "state": "Created",
  "gateway_binding": {
    "listen": "sarvam-stt",
    "think": "sarvam-llm",
    "speak": "sarvam-tts"
  },
  "edge_token": "eyJ…",
  "edge_wss_url": "wss://orch/edge/fs?token=…"
}
```

Session create resolves tenant engines → capability-checks each gateway → persists `gateway_binding` on the session row and passes it into runtime start.

### `GET /v1/sessions/{id}`

Returns `state` (`Created`|`Attached`|`Running`|`Draining`|`Completed`|`Cancelled`|`Failed`), pinned version, clock, `owner_instance`, `gateway_binding` (when set), `detected_language`, `active_language` (empty until lock/PATCH — `LANGUAGE_POLICY.md`), error if Failed.

### `POST /v1/sessions/{id}/inject`

```json
{ "text": "Your balance is …", "speak": true }
```

### `PATCH /v1/sessions/{id}/profile-fields`

Only keys in profile `hot_swap_allowed`. Contact Agent language switch (cc-2):

```json
{ "language.primary": "hi-IN" }
```

Requires `language.mid_call_switch: true`. Sets session `active_language`; next Listen hint uses the new value (`LANGUAGE_POLICY.md`, RUNTIME §9).

### `GET /v1/sessions/{id}/transcript`

Returns durable ordered turns for the session (cc-5). Appended on Talk turn complete (user then assistant rows share one `turn_id` per cycle; `seq` is monotonic per session). Live PCM is never stored here.

404 `not_found` if session missing. Empty `turns` array when session exists but no turns yet.

```json
{
  "session_id": "…",
  "turns": [
    {"seq": 1, "turn_id": "…", "role": "user", "text": "…", "created_at": "…"},
    {"seq": 2, "turn_id": "…", "role": "assistant", "text": "…", "created_at": "…"}
  ]
}
```

`role` is closed: `user` | `assistant` | `system`.

### `GET /v1/sessions/{id}/disposition`

Returns the AI disposition suggestion written by the postcall worker after session terminal (or playback complete). Tags: `resolved` | `unresolved` | `escalated`. Always upserts a row when postcall completes successfully (default `unresolved` when no `templates.disposition`).

404 `not_found` if session missing **or** no disposition row yet (postcall not done). Agent override (`final`) may stay null in V1 — Coral owns override UI.

```json
{
  "session_id": "…",
  "suggestion": "resolved",
  "template_id": "cc-disposition-v1",
  "source": "postcall_worker",
  "final": null,
  "updated_at": "…"
}
```

**Write path:** postcall worker `UpsertSessionDisposition` after Think/`templates.disposition` (or default), then audit `disposition.suggestion`. Optional skill `push_disposition` (else legacy `create_ticket` push) when allowed in profile.

### `POST /v1/sessions/{id}/stop`

```json
{ "reason": "operator" }
```

### `POST /v1/jobs/playback`

```json
{
  "profile_id": "meeting-mom",
  "profile_version": "latest",
  "file_uri": "s3://…/meeting.wav"
}
```

```json
{ "job_id": "01H…", "state": "Queued" }
```

### Error envelope (all failures)

```json
{
  "error": {
    "code": "profile_invalid",
    "message": "gateway id nextai-stt not registered",
    "details": {}
  }
}
```

Closed `error.code` set (extend only via this doc):  
`unauthorized`, `forbidden`, `not_found`, `conflict`, `profile_invalid`, `gateway_unavailable`, `rate_limited`, `bad_request`, `internal`.

---

## 3. SSE events (`GET /v1/sessions/{id}/events`)

`Content-Type: text/event-stream`. Each event:

```text
event: caption
data: {"session_id":"…","partial":false,"text":"…","language":"en-IN","ts_ms":0}

event: session.state
data: {"session_id":"…","state":"Running"}

event: skill.completed
data: {"session_id":"…","name":"warm_transfer","ok":true}

event: error
data: {"session_id":"…","code":"timeout","message":"…"}
```

Event names (locked): `caption`, `session.state`, `skill.completed`, `turn.completed`, `error`.

---

## 4. Profile publish

`POST /v1/profiles/{id}/versions` body = profile document from `PROFILE_SCHEMA.md`.  
Server validates gateway ids against registry; returns `422` + `profile_invalid` if not.  
Talk/Speak profiles missing `persona.voice` / `persona.voice_id` also return `422` + `profile_invalid` (cc-3).

**Deferred (coral-file follow-up):** `GET /v1/gateways/{id}/voices` catalog for voice pickers — not in this control surface yet.

---

## 5. KB ingest

`POST /v1/kb/documents` multipart or `{ "uri": "…", "collection": "acme-faq-v2" }`.  
Returns document id + `indexing`|`ready`|`failed`. Retrieve stays on Knowledge port inside the process — not a public “search” API in Phase B unless needed.

---

## 6. Open items (do not block Phase B)

| Item | Constraint |
|---|---|
| Exact Coral auth header name | Middleware only |
| Whether OpenAPI lives in repo as `api/openapi.yaml` | Yes when coding starts |
| Pagination on list profiles | Add when needed; not Phase B |

---

## 7. Rule

Coral clients depend on **this** API. Vendor gateways must not require new control routes. If a vendor needs a new session field, add it here first with a version note.
