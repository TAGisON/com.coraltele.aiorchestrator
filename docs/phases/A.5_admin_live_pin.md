# L3 — A.5 Admin live pin association + platform status

| Field | Value |
|---|---|
| **id** | `A.5` |
| **title** | Admin: profile/DID → flow pin + platform status |
| **status** | **Closed** — answer pins API + Admin live pin page |
| **parent_plan** | [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) A.5 |
| **depends_on** | A.4 Closed (`d5e2f9d`); G.7 live pin gate |

## architecture_refs

- [G.7_evidence_cutover.md](./G.7_evidence_cutover.md) — live requires `flow_id`/`flow_version`
- OD-13-6 — no new DDL; store pins in existing `system_settings`
- OD-13-7 — clocks include `chat` (create helper may offer it; runtime wiring still C.1)
- `/v1/platform/status`, `/v1/profiles`, `/v1/flows`, `/v1/sessions`

## goal

Give Admin a place to declare which published flow pins to which Talk profile (optional DID label), see platform readiness, and optionally create a lab session with that pin — without inventing a new DB table.

## in_scope

- Setting key `answer_pins` (JSON array) via Control:
  - `GET /v1/tenant/answer-pins`
  - `PUT /v1/tenant/answer-pins` body `{ "pins": [ { profile_id, flow_id, flow_version, did?, note? } ] }`
- Validate on PUT: profile exists; flow exists; `flow_version` is `latest` or positive int; reject unknown clocks if present
- Admin page `web/admin/pin.html`: list/edit pins (profile + flow dropdowns, version text/select, optional DID), platform status panel, lab “create session” helper (clock dropdown from catalog)
- Extend `OrchAPI`; hub + subnav
- Tests for answer-pins GET/PUT + static page
- This phase file + README

## out_scope

- FreeSWITCH/DID automatic answer wiring reading pins (document that edge may consume later; not implement FS changes here)
- Full `clock=chat` runtime (**C.1**)
- Admin soak checklist (**A.6**)
- New DDL tables

## forbidden

- Free-typed profile/flow ids as primary controls (dropdowns from list APIs)
- Secrets in pin records
- Desk revive

## exit_criteria

- [x] GET/PUT answer-pins work; bad flow → 422
- [x] Admin pin page served with profile/flow selects + platform status
- [x] Lab session create from pin helper works in control test or documented via existing create API
- [x] `go test ./web/... ./internal/control/...` green

## edge_cases

1. Empty pins array is valid.
2. Duplicate profile_id in PUT → 422 (one pin per profile).
3. Platform status blockers shown read-only.

## verification

```text
go test ./web/... ./internal/control/... -count=1 -timeout 180s
go build ./cmd/aiorchestrator/
```

## rollback

Remove answer-pins handlers + pin.html; setting rows may remain harmless.

## handoff

**Closed.** Next: **A.6** Admin soak checklist.
