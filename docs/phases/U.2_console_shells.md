# L3 — U.2 Console shells + shared client

| Field | Value |
|---|---|
| **id** | `U.2` |
| **title** | Shared web client + Admin / Supervisor / Chat shells |
| **status** | **Closed** — shared client + three app shells |
| **parent_plan** | [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) OD-13-1, OD-13-2 |
| **depends_on** | U.1 Closed (`5d9713a`) |

## architecture_refs

- [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) — three apps + shared
- [08_PURGE_AND_SCHEMA_PHASES.md](../08_PURGE_AND_SCHEMA_PHASES.md) OD-08-1 — rebuild, do not restore desk UIs
- [U.1_meta_catalog.md](./U.1_meta_catalog.md) — shells may call catalog
- `web/embed.go`, `internal/control/ui.go`

## goal

Ship production shell surfaces and a shared API client so later A/C/S phases fill screens without redoing routing, embed, or auth bypass.

## in_scope

- `web/shared/api.js` — Bearer from `localStorage` key `orch_token` (or empty when lab has no AuthToken); `api.get/post/put/patch/delete`; `api.catalog()`
- `web/shared/styles.css` — shared shell chrome (not desk CSS revive)
- `web/admin/index.html`, `web/supervisor/index.html`, `web/chat/index.html` — placeholder production shells (title, role purpose, link home, optional catalog/status probe)
- Update `web/index.html` — hub links to `/admin/`, `/supervisor/`, `/chat/` (new rebuild, not old desk chooser)
- Update `web/embed.go` to embed all of the above
- `mountUIRoutes`: static serve `/admin/`, `/supervisor/`, `/chat/`, `/shared/` + keep `GET /`
- `authMiddlewareUIBypass` for those static prefixes (OD-13-2: API still Bearer when configured)
- Control or embed smoke test: shells return 200 HTML
- This phase file + phases README

## out_scope

- Admin config screens (A.1+)
- Chat session UX (C.*)
- Supervisor session browser (S.*)
- Implementing `clock=chat` (C.1)
- Bindings HTTP (A.2)
- SSO / role claims beyond token in shared client
- React/Vue build pipeline (vanilla HTML/JS for V1 shells)

## forbidden

- Copying purged desk admin/user/supervisor JS/CSS behaviour or desk editor
- Product config forms in this phase (shells + probe only)
- Requiring Bearer to load HTML/CSS/JS static files
- Putting secrets in static files

## exit_criteria

- [x] Embed includes admin, supervisor, chat, shared
- [x] `GET /admin/` (or index), `/supervisor/`, `/chat/`, `/shared/api.js` served
- [x] `GET /` links to the three new consoles
- [x] Shared client can call `/v1/meta/catalog` from Admin shell probe
- [x] `go test` / build for control + web green
- [x] No desk engine references in new web files

## edge_cases

1. Trailing slash: `/admin` vs `/admin/` — serve or redirect so hub links work.
2. `testServer` without uiFS must still pass existing tests; add one test with `web.UIFS`.
3. Deep links under `/admin/foo` 404 until A.* adds pages — OK.

## verification

```text
go test ./web/... ./internal/control/... -count=1 -timeout 180s
go build ./cmd/aiorchestrator/
```

## rollback

Revert web/* and ui.go static mounts to post-P1.4 placeholder-only.

## handoff

**Closed.** Next: **A.1** Admin profiles + tenant engines/credentials/settings screens.
