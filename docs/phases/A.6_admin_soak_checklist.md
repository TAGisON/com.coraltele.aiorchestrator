# L3 — A.6 Admin console soak checklist

| Field | Value |
|---|---|
| **id** | `A.6` |
| **title** | Admin V1 soak checklist (UI configure → publish → pin) |
| **status** | **Closed** — checklist authored |
| **parent_plan** | [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) A.6 |
| **depends_on** | A.1–A.5 Closed (`e1e6a78` tip of A.5); U.1–U.2 Closed |

## architecture_refs

- [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) — Admin full config
- [L.0_graph_lab_soak.md](./L.0_graph_lab_soak.md) — API/graph soak (complement; this file is **UI-first**)
- Lab: http://127.0.0.1:8011/admin/
- Fixture reference: [tools/lab/flows/coral_xfer_minimal.v1.json](../../tools/lab/flows/coral_xfer_minimal.v1.json)

## goal

Give operators one checklist to prove Admin can configure a desk end-to-end **via the production consoles** (engines → profile → binding → flow builder → publish → answer pin → lab session), without relying only on raw curl.

## in_scope

- This checklist + phases README
- Preflight `go test` / build pointers
- No new product code

## out_scope

- Chat / Supervisor product soaks (C.4 / S.4)
- Owner filling sign-off (human after real run)
- FreeSWITCH DID auto-answer wiring from `answer_pins` (future edge consumer)
- Claiming dual call+chat pass (V.1)

## forbidden

- Claiming **soak pass** without a completed sign-off row
- Using desk-era admin paths

## exit_criteria

- [x] Checklist covers Admin hub screens through pin + lab session
- [x] Sign-off block present
- [x] Points at builder + catalog dropdowns
- [x] No product code required for phase pass

## verification

```text
go test ./web/... ./internal/control/... -count=1 -timeout 180s
go build ./cmd/aiorchestrator/
Test-Path web/admin/pin.html, web/admin/flows-builder.html
```

## handoff

Admin **A.*** wave Closed for planning+implementation. Next programme: **C.1** text-channel session (`clock=chat`), or owner runs this soak + L.0 on lab.

---

# Admin soak checklist

**How to use:** Start orch on :8011 → open `/admin/` → tick only when observed. Owner signs after a real lab run.

**Defaults:** http://127.0.0.1:8011/; set Bearer in Admin token field if `AuthToken` is configured.

## 0 — Preflight

| # | Check | Action | Pass criteria | ☐ |
|---|---|---|---|---|
| 0.1 | Build/tests | `go test ./web/... ./internal/control/... -count=1 -timeout 180s` | All `ok` | ☐ |
| 0.2 | Hub | Open `/admin/` | Links to Engines, Profiles, Bindings, Flows, Builder, Live pin, Settings | ☐ |
| 0.3 | Catalog | Admin Engines or Builder loads; or `GET /v1/meta/catalog` | `node_types` / `clocks` present | ☐ |
| 0.4 | Platform | Live pin → Refresh platform status | Status JSON shown (ready or blockers listed) | ☐ |

## 1 — Tenant engines & credentials

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 1.1 | Engines | `/admin/engines.html` — select listen/think/speak from dropdowns → Save | Saved; reload shows same IDs | ☐ |
| 1.2 | Credential | Select gateway → paste API key → Save | List shows key set + masked preview (not full key) | ☐ |

## 2 — Profile

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 2.1 | Create | `/admin/profiles.html` — create profile id | Appears in list | ☐ |
| 2.2 | Publish | Provider dropdowns from gateways → Publish version | Version published (no 422) | ☐ |

## 3 — Binding (optional FAQ)

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 3.1 | Inline FAQ | `/admin/bindings.html` — kind/status/mode from catalog; add entry → Save | Binding listed; GET `/v1/bindings/{id}` OK | ☐ |
| 3.2 | N/A | Skip if desk has no Inform | Mark N/A | — |

## 4 — Flow create + builder + publish

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 4.1 | Create flow | `/admin/flows.html` — id, direction dropdown → Create | Listed | ☐ |
| 4.2 | Builder | `/admin/flows-builder.html?flow=…` — add Entry/Speak/ListenChoice/Tool/End from **type selects**; edges from **kind selects**; matrix disposition from catalog; prompts | Draft save succeeds | ☐ |
| 4.3 | Publish | Publish version | New version number; invalid graph shows `flow_invalid` details if broken | ☐ |
| 4.4 | Inspect | Load published version | Doc appears; can re-save as draft | ☐ |
| 4.5 | Optional | Paste [coral_xfer_minimal](../../tools/lab/flows/coral_xfer_minimal.v1.json) via Flows JSON editor → save → open Builder / publish | Equivalent valid graph | ☐ |

## 5 — Live pin + lab session

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 5.1 | Answer pin | `/admin/pin.html` — profile + flow dropdowns, version `latest`, optional DID → Save pins | Reload shows same row | ☐ |
| 5.2 | Lab session | Create session with pin; clock `playback` (or `live` if engines ready) | `session_id` returned; `flow_id`/`flow_version` set | ☐ |
| 5.3 | Live gate | Attempt live create **without** flow (API or cleared pin fields) | **422** `flow_pin_required` | ☐ |

## 6 — Cross-check (optional)

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 6.1 | Graph evidence | Continue with [L.0](./L.0_graph_lab_soak.md) answer/inject path | `edge_taken` / `tool_line` as in L.0 | ☐ |

## Sign-off

| Field | Value |
|---|---|
| Date | |
| Operator | |
| Tip SHA | |
| Lab host | |
| Result | **soak pass** / **soak fail** / **deferred** |
| Notes | |

_Phase A.6 closes when this checklist exists in-repo. Lab **soak pass** requires a completed sign-off after a real Admin UI run._
