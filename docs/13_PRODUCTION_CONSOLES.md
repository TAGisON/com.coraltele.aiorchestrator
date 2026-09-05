# 13 — Production consoles (Admin / Supervisor / User chat)

**Status:** **Locked** (owner “continue” 2026-09-05 — ODs settled as Proposed).  
**Layer:** L2 domain plan per [07_PLANNING_STANDARDS.md](./07_PLANNING_STANDARDS.md) §3.  
**Parent:** [07](./07_PLANNING_STANDARDS.md).  
**Architecture refs:** [01](./01_VISION_AND_SCOPE.md), [02](./02_CURRENT_STATE.md), [03](./03_BRAIN_AND_GRAPH.md), [04](./04_LIVE_TURN_MACHINE.md), [05](./05_MEDIA_AND_VENDORS.md), [06](./06_APPLICATION_FLOW.md) § Config + channels.  
**Prior rebuild mandate:** [08](./08_PURGE_AND_SCHEMA_PHASES.md) **OD-08-1** deleted old three consoles — this plan is the **rebuild**, not a restore of desk UIs.  
**Kernel complete:** Graph runtime G.0–G.7 Closed; evidence E.*; soak checklists L.0 / E.6. Consoles sit on that kernel.

---

## Goal

Ship three **production-ready** web consoles so operators can configure a legal conversation graph, supervisors can inspect evidence, and testers/callers can exercise the **same** pinned flow over **text chat** (no STT/TTS) before and alongside live DID calls.

---

## In scope

1. **Admin console** — full operational control of V1 configuration (see § Admin capability inventory): profiles, engines/credentials, flows (create / draft / update / publish / list / inspect versions), matrix, bindings, locale prompts inside `coral.flow.v1`, tenant settings needed for live answer, and linking profile/DID answer path to a published flow pin.  
2. **Supervisor console** — read-only session list, transcript timeline, audit allowlist browser, disposition, recording metadata, light counts (not full QM / executive dashboard).  
3. **User chat console** — caller enters details → creates session with flow pin → same graph + turn/tool machine as voice; STT/TTS ignored/stubbed; text I/O only.  
4. **Server catalogs** — enums / allowlists so UI dropdowns cannot invent illegal node types, edge kinds, tools, or clock modes.  
5. **API gaps** required for the above (e.g. bindings HTTP, catalog endpoint, chat/session mode) — planned as L3 before or with the UI that needs them.  
6. **Soak / dual-channel validation** checklists after consoles land.

## Out of scope

- Restoring purged `web/admin|user|supervisor` desk hybrid UIs or desk engine.  
- Caption / translator / meeting / Omni suite products ([01](./01_VISION_AND_SCOPE.md) out).  
- Call summarization, rich CRM push, sentiment, Agent Assist, QM scoring, WFO, executive AI dashboard ([01](./01_VISION_AND_SCOPE.md) Next/Later).  
- Free-typed illegal topology that bypasses publish validate.  
- Parallel L4 mega-waves (still OD-08-5 serial concentration).  
- Claiming lab soak **pass** without human sign-off on V.* / L.0.

---

## Settled decisions (OD-13)

| ID | Decision | Implication |
|---|---|---|
| **OD-13-1** | **A** — Three apps: `web/admin`, `web/supervisor`, `web/chat` + shared client under `web/shared/` | Clear blast radius; matches OD-08-1 rebuild paths |
| **OD-13-2** | **A** — Control Bearer / lab token (role claim later if needed) | SSO out of this wave |
| **OD-13-3** | **A** — `GET /v1/meta/catalog` single payload | U.1 implements; Admin dropdowns bind only to this |
| **OD-13-4** | **A** — `POST …/inject` + `GET …/events` (SSE) | Dedicated WS only if C.* proves SSE insufficient |
| **OD-13-5** | **C** — Structured builder primary + optional advanced JSON (still validate on publish) | Admin full create/update without illegal free-typed topology |
| **OD-13-6** | **A** — No new DDL for consoles V1 | Catalog from Go constants; existing `flow_*` / profile / evidence |
| **OD-13-7** | **`clock=chat`** — text channel; graph pin required like live; STT/TTS no-op / unused | Implement in **C.1**; catalog lists `live` \| `playback` \| `chat` |
| **OD-13-8** | Light aggregates only | Not 01 Later executive/QM dashboard |

L3 may proceed; L4 console/API work allowed per Closed phase ids.

---

## Admin capability inventory (full control)

Admin is the configuration owner for V1. Every control below must be reachable from the Admin console once A.* phases Close — via UI backed by Control APIs (existing or gap-filled).

| Area | Admin can… | Primary APIs today | Gap? |
|---|---|---|---|
| **Profiles** | Create profile; publish versions; list; get version | `POST /v1/profiles`, `POST …/versions`, `GET /v1/profiles`, `GET …/versions/{ver}` | UI only |
| **Tenant engines** | Get/put listen·think·speak binding | `GET|PUT /v1/tenant/engines` | UI only |
| **Credentials** | Get/put gateway credentials | `GET|PUT /v1/tenant/credentials…` | UI only (never echo secrets in logs) |
| **Settings / fallback** | System settings; fallback WAVs | `/v1/tenant/settings*`, `/v1/tenant/fallback*` | UI only |
| **Flows — registry** | Create flow; list; get metadata | `POST|GET /v1/flows`, `GET /v1/flows/{id}` | UI only |
| **Flows — draft** | Read/write working draft | `GET|PUT /v1/flows/{id}/draft` | UI + builder |
| **Flows — publish** | Publish immutable version (validate `coral.flow.v1`) | `POST /v1/flows/{id}/versions`, `GET …/versions/{ver}` | UI; surface `flow_invalid` details |
| **Graph content** | Add/edit/remove nodes & edges; prompts/locales; repair; tools; matrix rows; binding_refs | Draft document fields | Builder must use **catalog enums only** |
| **Bindings** | Create/update/activate knowledge bindings for Inform | Store exists (G.1/G.6) | **HTTP `/v1/bindings*` missing** → U/A phase |
| **Live pin** | Ensure answer path uses published `flow_id`+`flow_version` | Session create requires pin for `clock=live` | Profile↔flow association UX (document in A.5) |
| **Gateways / platform** | Inspect gateway list / platform status | `GET /v1/gateways`, `GET /v1/platform/status` | UI only |
| **Catalog** | Load dropdown options | — | **`GET /v1/meta/catalog` (U.1)** |

**Admin must not:** invent node/edge/tool kinds; free-type transfer destinations outside matrix; edit published version blobs in place (new draft → new version); reintroduce desk engine.

---

## Supervisor capability inventory

| Area | Can… | APIs |
|---|---|---|
| Sessions | List / open session | `GET /v1/sessions`, `GET /v1/sessions/{id}` |
| Transcript | Timeline including graph kinds | `GET …/transcript` |
| Audit | Allowlisted events | `GET …/audit` |
| Disposition | View (patch only if product allows ops override — default read) | `GET …/disposition` |
| Analytics | Per-session + light aggregates | `GET …/analytics` (+ optional aggregate endpoint if missing) |
| Recording | Metadata / lifecycle stamps | Session fields / evidence APIs |

**Must not:** mutate flows, publish, or arm tools.

---

## User chat capability inventory

| Area | Can… | Notes |
|---|---|---|
| Start | Enter caller details; pick/receive profile + flow pin | Same create session contract as voice |
| Answer | Start graph (welcome) | `POST …/answer` |
| Talk | Send text turns | `inject` or chat-specific alias |
| See | Bot speak lines, tool closing lines, disposition | Events + transcript |
| End | Stop / natural End node | Same teardown |

**Must not:** bypass graph; call real STT/TTS for this channel; show Admin write controls.

---

## Locked product principles (consoles)

1. **Graph is law** — UI only offers [03](./03_BRAIN_AND_GRAPH.md) closed node/edge/tool sets via catalog.  
2. **Publish validate is gate** — bad graphs never become runnable versions.  
3. **Tools** — matrix-sourced destinations; arm→speak→exec unchanged.  
4. **Chat ≡ call brain** — one runtime; channel differs only in media.  
5. **No desk resurrect** — new code under new `web/*` apps; cite this doc, not old desk screens.  
6. **Secrets** — credentials UI write-only display; never commit `.agent/secrets.local.json`.

---

## Anti-patterns

- Porting old admin/supervisor/user JS “temporarily.”  
- Free-text node `type` / edge `kind` fields without catalog.  
- Admin JSON paste that skips client-side enum checks (server validate still required, but UI must not encourage illegal drafts).  
- Chatbot that reimplements FAQ/transfer outside graph.  
- Supervisor write paths for config.  
- One L4 phase that builds all three consoles.  
- Dropping production data without an L3 migrate/rollback note (lab DB drop is owner-ops after Lock if needed).

---

## Phase breakdown (L3 catalog)

Serial waves. Detail each phase in `docs/phases/` before L4. Size: half-day–two days per phase.

### Wave U — Foundation

| Phase | Goal |
|---|---|
| **U.0** | Inventory gaps vs this L2; freeze phase sketch; no code |
| **U.1** | Server `GET /v1/meta/catalog` (enums from flow validate + tools + clocks + binding kinds) |
| **U.2** | Shared web client + three app shells + auth gate (placeholder pages OK) |

### Wave A — Admin (full configuration)

| Phase | Goal |
|---|---|
| **A.1** | Profiles + tenant engines/credentials/settings/fallback screens |
| **A.2** | Bindings Control HTTP + Admin binding CRUD |
| **A.3** | Flow list/create/get + draft save/load |
| **A.4** | Graph builder (nodes/edges/prompts/repair/matrix/binding_refs from catalog) + publish + version inspect |
| **A.5** | Profile / DID / live pin association UX + platform/gateway status |
| **A.6** | Admin soak checklist (configure → publish → pin via UI only) |

### Wave C — User chat

| Phase | Goal |
|---|---|
| **C.1** | Text-channel session mode (OD-13-7) + create/answer without STT/TTS |
| **C.2** | Chat UI (details → turns → bot lines / tool lines) |
| **C.3** | Evidence parity on chat path |
| **C.4** | Chat soak checklist |

### Wave S — Supervisor

| Phase | Goal |
|---|---|
| **S.1** | Session list + detail (transcript, disposition, recording meta) |
| **S.2** | Audit browser |
| **S.3** | Light aggregates |
| **S.4** | Supervisor soak checklist |

### Wave V — Dual-channel prove

| Phase | Goal |
|---|---|
| **V.1** | Same published flow: chat soak + call soak (owner DID) |
| **V.2** | Lab performance notes (optional; not WFO product) |

**Order:** U.0 → U.1 → U.2 → **A.*** (Admin complete) → **C.*** → **S.*** → **V.***  
(Admin before chat so chat tests a UI-published flow. Supervisor may swap after A.2 if owner prefers ops visibility earlier — default remains after Chat.)

---

## Database

- Prefer **OD-13-6 A** (no new DDL).  
- Owner already dropped lab DB: after this L2 **Locked**, run normal migrate chain on empty DB before L4 UI work if schema is missing.  
- Re-drop only if a Locked OD introduces incompatible DDL — name that in the migrate L3, never ad-hoc mid-UI.

---

## Handoff to L3

1. ~~Owner settles OD-13~~ **Done** (Locked).  
2. Close **U.0**; author/implement **U.1** catalog, then **U.2** shells — one id at a time ([12](./12_AGENTIC_L4_ROLES.md)).  
3. Each L3 cites this doc + 01–06; exit criteria observable.

---

## Handoff note (Locked)

Admin **A.1–A.6** Closed. **C.1–C.4** Closed. **S.1–S.4** Closed (Supervisor complete). Next: [phases/V.1](./phases/) dual chat+call prove, or human soak sign-off on [A.6](./phases/A.6_admin_soak_checklist.md) / [C.4](./phases/C.4_chat_soak_checklist.md) / [S.4](./phases/S.4_supervisor_soak_checklist.md).
