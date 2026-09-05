# L3 — U.0 Production consoles inventory

| Field | Value |
|---|---|
| **id** | `U.0` |
| **title** | Production consoles gap inventory + phase freeze |
| **status** | **Draft** — awaiting L2 OD settle / Close |
| **parent_plan** | [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) |
| **depends_on** | G.7 Closed; L.0 checklist authored; doc 13 Draft exists |

## architecture_refs

- [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) — Admin / Supervisor / Chat rebuild
- [01_VISION_AND_SCOPE.md](../01_VISION_AND_SCOPE.md) — Configuration + V1 ticks; Next/Later out
- [02_CURRENT_STATE.md](../02_CURRENT_STATE.md) — three consoles gone; placeholder `/`
- [03_BRAIN_AND_GRAPH.md](../03_BRAIN_AND_GRAPH.md) — closed node/edge/tool sets
- [06_APPLICATION_FLOW.md](../06_APPLICATION_FLOW.md) — Admin/API config box
- [08_PURGE_AND_SCHEMA_PHASES.md](../08_PURGE_AND_SCHEMA_PHASES.md) — OD-08-1 delete; rebuild under new plan
- Kernel: G.1–G.7, E.*, Control routes in `internal/control/server.go` + `ui.go`

## goal

Freeze an honest gap list between the production-console L2 and today’s Control/web surface so later U/A/C/S/V phases do not invent scope or resurrect desk UIs.

## in_scope

- This inventory document
- Phase id freeze for U/A/C/S/V (pointers only)
- Explicit Admin full-control gap table (create/update/publish flows and related config)
- No product code; no new web apps in this phase

## out_scope

- Implementing catalog, bindings HTTP, or any console UI
- Settling OD-13-* (owner on doc 13)
- DB drop/migrate
- Changing G.* runtime behaviour

## forbidden

- Claiming Admin/Supervisor/Chat UIs exist
- Porting old desk console paths as “inventory reuse”
- Expanding V1 into QM / summary / CRM push

## exit_criteria

- [x] Gap tables written for Admin / Supervisor / Chat / shared
- [x] Phase sketch matches doc 13
- [x] Admin full-control gaps named (including bindings HTTP + catalog)
- [ ] Owner acknowledges inventory (Close) and preferably settles doc 13 ODs

## edge_cases

1. Publish already validates `coral.flow.v1` — UI must still use catalogs so drafts are not systematically illegal.
2. Live create without flow pin → 422 — Admin A.5 must make pin obvious.
3. Bindings store works in tests without HTTP — Chat/Inform lab via API until A.2.
4. Placeholder `web/index.html` remains until U.2 replaces shell routing carefully.
5. Supervisor must not gain flow write by sharing Admin API client carelessly — separate apps (OD-13-1 A).

## verification

```text
Test-Path docs/13_PRODUCTION_CONSOLES.md
Test-Path docs/phases/U.0_consoles_inventory.md
# No code change required for U.0
```

## rollback

Delete or revert this file + README U.* rows if programme abandons consoles wave.

## handoff

After Close + doc 13 Locked (or owner unlock): author **U.1** catalog API L3, then L4 Implementer on U.1 only.

---

# Gap inventory (U.0)

## 1 — Web surface today

| Path / route | State |
|---|---|
| `web/index.html` | Placeholder only — states consoles removed |
| `web/admin/**`, `web/user/**`, `web/supervisor/**` | **Absent** (P1.1–P1.3) |
| Embedded three-console product | **Gone** |

## 2 — Admin (full control) — API vs UI

| Capability | API today | UI today | Follow-on phase |
|---|---|---|---|
| Create / list / get profiles | Yes | No | A.1 |
| Publish profile versions | Yes | No | A.1 |
| Tenant engines get/put | Yes | No | A.1 |
| Credentials get/put | Yes | No | A.1 |
| Settings / fallback | Yes | No | A.1 |
| Create / list / get flows | Yes (`/v1/flows*`) | No | A.3 |
| Get / put flow **draft** | Yes | No | A.3 |
| Publish flow version + get version | Yes (+ validate) | No | A.4 |
| Graph builder (nodes/edges/prompts/repair/matrix/binding_refs) | Document in draft JSON | No | A.4 |
| **Catalog enums for dropdowns** | Partial (validate constants only, not HTTP) | No | **U.1** |
| **Bindings CRUD HTTP** | Store only (no `/v1/bindings*`) | No | **A.2** |
| Gateways / platform status | Yes | No | A.5 |
| Profile/DID ↔ flow pin UX | Session create fields | No | A.5 |
| Admin end-to-end soak via UI | — | No | A.6 |

## 3 — Supervisor

| Capability | API today | UI today | Follow-on |
|---|---|---|---|
| List sessions | Yes | No | S.1 |
| Session detail | Yes | No | S.1 |
| Transcript | Yes | No | S.1 |
| Audit | Yes | No | S.2 |
| Disposition get | Yes | No | S.1 |
| Per-session analytics | Yes | No | S.1 / S.3 |
| Cross-session aggregates | Unclear / thin | No | S.3 if needed |
| Recording metadata visibility | Partial on session | No | S.1 |

## 4 — User chat

| Capability | API today | UI today | Follow-on |
|---|---|---|---|
| Create session + flow pin | Yes (live requires pin) | No | C.1 |
| Answer / inject / events / stop | Yes | No | C.1–C.2 |
| Explicit text-channel / no STT-TTS mode | **Not named** (OD-13-7) | No | C.1 |
| Chat transcript parity (`edge_taken`, `tool_line`) | Emitters exist on graph path | No | C.3 |
| Chat soak | — | No | C.4 |

## 5 — Shared foundation gaps

| Gap id | Gap | Phase |
|---|---|---|
| U-GAP-1 | No HTTP catalog for node/edge/tool/locale/clock/binding enums | U.1 |
| U-GAP-2 | No production web shells / shared API client | U.2 |
| U-GAP-3 | No bindings Control HTTP | A.2 |
| U-GAP-4 | No text-channel session semantics | C.1 |
| U-GAP-5 | Dual soak (chat + call) not checklisted under V.* | V.1 |

## 6 — Phase freeze (matches doc 13)

```text
U.0 (this) → U.1 catalog → U.2 shells
  → A.1 … A.6 Admin full config
  → C.1 … C.4 Chat
  → S.1 … S.4 Supervisor
  → V.1 … V.2 dual prove
```

## 7 — Explicit non-goals (inventory)

- Desk step editor, keyword NLU admin, old three-console CSS/JS
- Full Omni analytics / QM / WFO UIs
- Implementing anything in U.0
