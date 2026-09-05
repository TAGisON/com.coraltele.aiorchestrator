# L3 — S.4 Supervisor console soak checklist

| Field | Value |
|---|---|
| **id** | `S.4` |
| **title** | Supervisor V1 soak checklist (list → detail → audit → summary) |
| **status** | **Closed** — checklist authored |
| **parent_plan** | [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) S.4 |
| **depends_on** | S.1–S.3 Closed (`eea05cf` tip of S.3); C.4/A.6 for sessions to inspect |

## architecture_refs

- [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) — Supervisor inventory; OD-13-8
- [S.1_supervisor_sessions.md](./S.1_supervisor_sessions.md) — list + detail
- [S.2_supervisor_audit.md](./S.2_supervisor_audit.md) — audit browser
- [S.3_supervisor_aggregates.md](./S.3_supervisor_aggregates.md) — light summary
- [C.4_chat_soak_checklist.md](./C.4_chat_soak_checklist.md) — produce chat sessions to inspect
- Lab: http://127.0.0.1:8011/supervisor/

## goal

Give operators one checklist to prove the **Supervisor** console can browse recent sessions read-only — transcript, disposition, recording meta, allowlisted audit, and light aggregates — without Admin write controls.

## in_scope

- This checklist + phases README / index handoff
- Preflight `go test` / build / path pointers
- No new product code

## out_scope

- Owner filling sign-off (human after real run)
- Dual call+chat prove (**V.1**)
- Claiming QM / executive dashboard (OD-13-8)
- PATCH disposition / ops override

## forbidden

- Claiming **soak pass** without a completed sign-off row
- Using desk-era supervisor paths
- Mutating flows/publish from Supervisor during soak

## exit_criteria

- [x] Checklist covers list → detail (transcript/disposition/recording) → audit filter → light summary
- [x] Sign-off block present
- [x] Points at Chat/Admin prerequisites for having sessions
- [x] No product code required for phase pass

## verification

```text
go test ./web/... ./internal/control/... -count=1 -timeout 180s
go build ./cmd/aiorchestrator/
Test-Path web/supervisor/index.html, web/supervisor/supervisor.js, docs/phases/S.4_supervisor_soak_checklist.md
```

## handoff

Wave **S.*** Closed for planning+implementation. Next programme: **V.1** dual chat+call prove, or owner runs this soak (+ A.6 / C.4 / L.0) on lab.

---

# Supervisor soak checklist

**How to use:** Prefer at least one completed chat or live/playback session (via [C.4](./C.4_chat_soak_checklist.md) or [A.6](./A.6_admin_soak_checklist.md) lab create). Open `/supervisor/` → tick only when observed. Owner signs after a real lab run.

**Defaults:** http://127.0.0.1:8011/; set Bearer in Supervisor token field if `AuthToken` is configured. **Read-only** — no inject/stop/publish here.

## 0 — Preflight

| # | Check | Action | Pass criteria | ☐ |
|---|---|---|---|---|
| 0.1 | Build/tests | `go test ./web/... ./internal/control/... -count=1 -timeout 180s` | All `ok` | ☐ |
| 0.2 | Shell | Open `/supervisor/` | Sessions table + Light summary + token row | ☐ |
| 0.3 | No write UI | Scan page for publish / inject / engines | Absent (links to Admin/Chat only) | ☐ |
| 0.4 | Seed session | If list empty: run Chat Start or Admin lab session | ≥1 session appears after Refresh | ☐ |

## 1 — Session list

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 1.1 | Refresh | **Refresh list** | Rows show clock, state, profile, flow (when pinned), recording hint | ☐ |
| 1.2 | Open | Click a session id | Detail card opens with same id | ☐ |

## 2 — Session detail

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 2.1 | Summary | Read Summary probe | clock/state/profile/flow fields present | ☐ |
| 2.2 | Disposition | Read Disposition | final/suggestion/source or “(no disposition)” | ☐ |
| 2.3 | Recording | Read Recording meta | `recording_ref` / stamps shown or “(none)” / dashes | ☐ |
| 2.4 | Transcript | Scroll transcript turns | user/bot/edge/tool kinds visible when session had graph activity | ☐ |
| 2.5 | Reload | **Reload detail** | Data refreshes without error | ☐ |

## 3 — Audit browser

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 3.1 | Events | Open a session with tool/graph activity | Audit table has rows (`graph.edge` / `tool.*` / session lifecycle as applicable) | ☐ |
| 3.2 | Filter | Select an `event_type` from filter | Table shows only that type; All restores full list | ☐ |
| 3.3 | Catalog | Filter options include allowlisted types | Dropdown populated from catalog (not empty if catalog OK) | ☐ |

## 4 — Light aggregates

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 4.1 | Summary card | **Refresh summary** | JSON with `sessions_total`, `by_state`, `by_clock` | ☐ |
| 4.2 | Metrics | If sessions emitted analytics | `metrics` may include `contained` / `handoff` / `session_completed` | ☐ |
| 4.3 | Session analytics | Detail → Analytics (session) | Lines or “No analytics events.” | ☐ |
| 4.4 | Non-QM | Confirm copy/note | Explicit light / not QM dashboard | ☐ |

## 5 — Cross-check (optional)

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 5.1 | API | `GET /v1/analytics/summary` and `GET /v1/sessions/{id}/audit` | Match UI | ☐ |
| 5.2 | Dual path | Same flow via Chat then inspect here | Deferred dual **pass** to **V.1** | ☐ |

## Sign-off

| Field | Value |
|---|---|
| Date | |
| Operator | |
| Tip SHA | |
| Lab host | |
| Result | **soak pass** / **soak fail** / **deferred** |
| Notes | |

_Phase S.4 closes when this checklist exists in-repo. Lab **soak pass** requires a completed sign-off after a real Supervisor UI run._
