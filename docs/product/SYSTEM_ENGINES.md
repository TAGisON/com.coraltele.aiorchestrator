# System / tenant engines

**Status:** LOCKED (Contact Agent vertical — V1)  
**Date:** 31 August 2026  
**Parents:** `PRODUCT_DECISIONS.md`, `CONTACT_AGENT.md`  
**Related:** `LANGUAGE_POLICY.md`, `docs/architecture/PROFILE_SCHEMA.md`, `docs/architecture/CONTROL_API.md`

---

## 1. Model

Each **tenant** has exactly **one active Listen**, **one active Think**, and **one active Speak** gateway id (STT / LLM / TTS).

| Slot | Port | Example lab ids |
|---|---|---|
| Listen | `listen` | `sarvam-stt` |
| Think | `think` | `sarvam-llm` |
| Speak | `speak` | `sarvam-tts` |

Profiles (**behaviour** only for Contact Agent V1) inherit these engines. They do **not** pick a vendor failover list for Talk.

---

## 2. Persistence

Durable row (or equivalent) keyed by `tenant_id`:

- `listen_id`, `think_id`, `speak_id`, `updated_at`

API: `GET` / `PUT /v1/tenant/engines` (see `CONTROL_API.md`).

---

## 3. Configuration (Control API only)

Migrations create the `tenant_engines` table (schema only). They **never** insert a default row.

Fresh install: `GET /v1/tenant/engines` → `404` until an operator runs `PUT /v1/tenant/engines` (lab Settings / Tenant Engines, or coral-file). No boot-properties or env seed of listen/think/speak ids.

Vendor API keys likewise: empty until `PUT /v1/tenant/credentials/{gateway_id}`.

---

## 4. Session pin (`gateway_binding`)

On `POST /v1/sessions`:

1. Resolve tenant engines from the store only (missing → `422` `tenant engines not configured`).
2. Capability-check each id against the process gateway registry (port kind + known id).
3. Persist snapshot on the session as `gateway_binding`: `{ "listen", "think", "speak" }`.
4. Pass the same binding into runtime start metadata.

`GET /v1/sessions/{id}` returns `gateway_binding`.

**V1 hard rule:** no mid-session vendor hop. Bound ids stay for the life of the session. Degrade later via clips / escalate (response ladder) — not by walking a provider list.

---

## 5. Contact Agent vs other families

| Family | Engine source (V1) |
|---|---|
| `contact-agent` | Tenant system engines; profile `routers.listen/think/speak.providers` optional / deprecated |
| Other families | May still use profile router provider lists until migrated |

If a Contact Agent profile still lists providers that **differ** from tenant engines, **tenant engines win**; log a warning.

---

## 6. Out of scope here

- Profile-level vendor override UI  
- Mid-session failover walk  
- Per-call engine override  
- Full coral-file admin panel (lab uses Control APIs above)
