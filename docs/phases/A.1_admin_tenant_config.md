# L3 — A.1 Admin profiles + tenant engines / credentials / settings

| Field | Value |
|---|---|
| **id** | `A.1` |
| **title** | Admin: profiles, engines, credentials, settings |
| **status** | **Closed** — Admin profiles / engines / credentials / settings |
| **parent_plan** | [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) Admin inventory |
| **depends_on** | U.2 Closed (`60c46ab`) |

## architecture_refs

- [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) — Admin full control; dropdowns from server
- [01_VISION_AND_SCOPE.md](../01_VISION_AND_SCOPE.md) — Configuration
- [05_MEDIA_AND_VENDORS.md](../05_MEDIA_AND_VENDORS.md) — engines behind ports
- Control: `/v1/profiles*`, `/v1/tenant/engines`, `/v1/tenant/credentials*`, `/v1/tenant/settings*`, `/v1/tenant/fallback*`, `/v1/gateways`
- U.1 catalog (clocks etc. not primary here; gateways drive engine dropdowns)

## goal

Give Admin working screens to create/list/publish profiles and manage tenant engines, gateway credentials, and system settings — using **selects populated from `/v1/gateways`** (and catalog where relevant), not free-typed illegal gateway IDs.

## in_scope

- `web/admin/` pages: hub update + `engines.html`, `profiles.html`, `settings.html` (and shared admin JS if needed)
- Extend `web/shared/api.js` helpers for these endpoints
- Form styles in `web/shared/styles.css` as needed
- Engines: GET/PUT listen·think·speak via `<select>` options from gateways by port
- Credentials: list (masked); PUT api_key for selected gateway_id from gateways list
- Settings: list + upsert key/value; fallback scenario list + optional WAV upload if store available
- Profiles: list; create id/display_name/tenant; publish version with modes + router providers from gateway dropdowns (build JSON client-side)
- Embed already covers `admin/*` — no Go route changes unless smoke test
- Smoke: embed/static test that new pages exist
- This phase file + README

## out_scope

- Flow graph builder (**A.3–A.4**)
- Bindings HTTP/UI (**A.2**)
- Chat / Supervisor product screens
- Full profile JSON Schema editor beyond structured publish form
- SSO

## forbidden

- Free-text gateway id fields as the primary engine UI (dropdowns required; advanced paste optional only if validate still server-side)
- Echoing full API keys in the UI after save (preview/mask only)
- Desk revive
- Flow publish in this phase

## exit_criteria

- [x] Admin hub links to engines, profiles, settings
- [x] Engines page saves via PUT with gateway selects
- [x] Credentials page can set api_key for a listed gateway
- [x] Profiles page can create + publish a minimal Talk profile using gateway selects
- [x] Settings page can list/upsert settings; fallback list visible or soft-unavailable
- [x] `go test ./web/... ./internal/control/...` green

## edge_cases

1. Gateways empty → selects empty; show clear error, do not invent IDs.
2. Engines 404 until first PUT — UI treats as empty form.
3. Fallback store unset → 503; show message, do not fail whole settings page.
4. Publish profile 422 — surface server error message in probe area.

## verification

```text
go test ./web/... ./internal/control/... -count=1 -timeout 180s
go build ./cmd/aiorchestrator/
```

## rollback

Remove new admin HTML/JS; restore U.2 hub-only admin index.

## handoff

**Closed.** Next: **A.2** bindings Control HTTP + Admin binding CRUD.
