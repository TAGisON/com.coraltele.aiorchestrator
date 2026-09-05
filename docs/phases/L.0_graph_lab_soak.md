# L3 — L.0 Graph lab soak checklist

| Field | Value |
|---|---|
| **id** | `L.0` |
| **title** | Graph V1 lab soak checklist (API / DID) |
| **status** | **Closed** — checklist authored |
| **parent_plan** | [01](../01_VISION_AND_SCOPE.md) V1 done; [G.7](./G.7_evidence_cutover.md) |
| **depends_on** | G.0–G.7 Closed (`ee2cbb3` tip of G.7); E.6 Closed |

## architecture_refs

- [01_VISION_AND_SCOPE.md](../01_VISION_AND_SCOPE.md) — welcome → intent → FAQ optional → transfer/hangup
- [G.7_evidence_cutover.md](./G.7_evidence_cutover.md) — live flow pin; `edge_taken` / `tool_line`
- Fixture: [tools/lab/flows/coral_xfer_minimal.v1.json](../../tools/lab/flows/coral_xfer_minimal.v1.json)
- Lab UI: http://127.0.0.1:8011/
- Prior evidence soak: [E.6](./E.6_evidence_soak_checklist.md) (profile path); this file covers **graph** path

## goal

Give operators one written checklist to verify the **pinned-flow** V1 call path after G.7 — publish fixture → live session with pin → answer/choice/transfer or hangup → evidence kinds present.

## in_scope

- This checklist + README L.* section
- Point at existing fixture + Control `/v1/flows*` / `/v1/sessions*`
- Preflight test command (includes graph packages)
- No new product code

## out_scope

- Admin SPA authoring
- Full Sarvam DID automation in CI
- Filling the sign-off block (owner does that after a real lab run)
- Next/Later SKUs (summary, CRM, QM)

## forbidden

- Claiming soak **pass** without a completed sign-off row
- Live session without flow pin (G.7 gate)

## exit_criteria

- [x] Checklist covers publish → pin → welcome → transfer/hangup → evidence
- [x] Sign-off block present
- [x] Fixture path cited
- [x] No product code required for phase pass

## verification

```text
Test-Path tools/lab/flows/coral_xfer_minimal.v1.json
go test ./internal/runtime/graph/... ./internal/control/... ./internal/runtime/observe/... -count=1 -timeout 180s
```

## handoff

Programme L3 waves for V1 call-flow **planning+runtime** are Closed. Owner runs L.0 (and E.6 as needed) on lab; then promote via [CD.0](./CD.0_lab_promote.md).

---

# Graph lab soak checklist

**How to use:** Preflight → publish fixture → walk scenarios. Tick only when observed. Owner signs after a real lab run.

**Defaults:** Control http://127.0.0.1:8011/; tenant engines configured; recording on if checking B3.

## 0 — Preflight

| # | Check | Action | Pass criteria | ☐ |
|---|---|---|---|---|
| 0.1 | Graph tests | `go test ./internal/runtime/graph/... ./internal/control/... ./internal/runtime/observe/... -count=1 -timeout 180s` | All `ok` | ☐ |
| 0.2 | Fixture present | `tools/lab/flows/coral_xfer_minimal.v1.json` | File exists | ☐ |
| 0.3 | Tip | README G.7 Closed SHA | Record tip in sign-off | ☐ |
| 0.4 | Live pin gate | `POST /v1/sessions` with `clock=live` and **no** `flow_id` | **422** `flow_pin_required` | ☐ |

## 1 — Publish + pin

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 1.1 | Create flow | `POST /v1/flows` `{id, tenant_id, name}` | 201 | ☐ |
| 1.2 | Publish fixture | `POST /v1/flows/{id}/versions` body = coral_xfer_minimal JSON | 201; version ≥ 1 | ☐ |
| 1.3 | Live session | `POST /v1/sessions` with profile + `flow_id` + `flow_version` + `clock=live` | 201; response shows flow pins | ☐ |

## 2 — Welcome → transfer

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 2.1 | Answer | `POST …/answer` | Spoken welcome text; welcome completed | ☐ |
| 2.2 | Intent sales | Inject/speak matching **sales** (with telephony leg or stub CC in lab) | Closing transfer line; transfer settles; disposition `transferred_sales` (or allowlisted transfer final) | ☐ |
| 2.3 | Evidence | `GET …/transcript` (+ audit if available) | At least one `edge_taken`; Tool path has `tool_line`; audit may include `graph.edge` | ☐ |

## 3 — Hangup + repair

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 3.1 | Hangup path | New pinned session → answer → intent **bye** | Closing hangup line; disposition `hangup_completed` (not `system_failure`) | ☐ |
| 3.2 | Repair exhaust | New session → answer → unclear utterances until repair exhaust | Drawn repair → hangup Tool; no invented FAQ/transfer | ☐ |

## 4 — Optional Inform (if binding seeded)

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 4.1 | N/A default | Minimal fixture has empty `binding_refs` | Mark N/A unless lab published an Inform flow + active `inline_faq` binding | — |

## Sign-off

| Field | Value |
|---|---|
| Date | |
| Operator | |
| Tip SHA | |
| Lab host / DID | |
| Result | **soak pass** / **soak fail** / **deferred** |
| Notes | |

_Phase L.0 closes when this checklist exists in-repo. Lab **soak pass** requires a completed sign-off after a real run._
