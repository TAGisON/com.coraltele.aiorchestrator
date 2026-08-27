# Control API — Coral-facing OpenAPI shape

**Status:** LOCKED (route + body shape; exact auth header still open)  
**Date:** 27 August 2026  
**Parents:** `SOLUTION.md` §11, `PLATFORM_FIRST.md`, `PROFILE_SCHEMA.md`

This is the **HTTP surface Coral Java / admin / dialplan helpers** call. It is independent of Listen/Speak vendors. Publish as OpenAPI 3 when coding `internal/control`.

Auth: Coral estate token (middleware). Every request carries or resolves `tenant_id` + optional `coral_user_id`.

Base path: `/v1` (versioned so vendors never force a breaking API).

---

## 1. Routes

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/v1/health` | Process + PG; optional gateway summary |
| `POST` | `/v1/sessions` | Create session; pin profile; optional edge token |
| `GET` | `/v1/sessions/{id}` | Status |
| `POST` | `/v1/sessions/{id}/attachments` | Bind feeder/sink metadata (non-FS) |
| `POST` | `/v1/sessions/{id}/inject` | Text in (Speak / push) |
| `PATCH` | `/v1/sessions/{id}/profile-fields` | Hot-swap allowed fields only |
| `POST` | `/v1/sessions/{id}/stop` | Drain → terminal |
| `GET` | `/v1/sessions/{id}/events` | SSE event stream |
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
  "edge_token": "eyJ…",
  "edge_wss_url": "wss://orch/edge/fs?token=…"
}
```

### `GET /v1/sessions/{id}`

Returns `state` (`Created`|`Attached`|`Running`|`Draining`|`Completed`|`Cancelled`|`Failed`), pinned version, clock, `owner_instance`, error if Failed.

### `POST /v1/sessions/{id}/inject`

```json
{ "text": "Your balance is …", "speak": true }
```

### `PATCH /v1/sessions/{id}/profile-fields`

Only keys in profile `hot_swap_allowed`:

```json
{ "language.primary": "hi-IN" }
```

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
