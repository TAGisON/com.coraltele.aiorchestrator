# L3 — C.4 Chat console soak checklist

| Field | Value |
|---|---|
| **id** | `C.4` |
| **title** | Chat V1 soak checklist (UI details → turns → evidence) |
| **status** | **Closed** (`e78d38f`) — checklist authored |
| **parent_plan** | [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) C.4 |
| **depends_on** | C.1–C.3 Closed (`8ef4bc4` tip of C.3); A.5/A.6 for pin/publish; U.2 shells |

## architecture_refs

- [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) — User chat inventory; OD-13-7
- [C.1_clock_chat.md](./C.1_clock_chat.md) — `clock=chat`
- [C.2_chat_ui.md](./C.2_chat_ui.md) — `/chat/` Start / Send
- [C.3_chat_evidence.md](./C.3_chat_evidence.md) — transcript/audit parity; recording non-parity
- [A.6_admin_soak_checklist.md](./A.6_admin_soak_checklist.md) — Admin configure/publish/pin first
- Lab: http://127.0.0.1:8011/chat/
- Fixture: [tools/lab/flows/coral_xfer_minimal.v1.json](../../tools/lab/flows/coral_xfer_minimal.v1.json)

## goal

Give operators one checklist to prove the **User chat** console can exercise the same pinned graph as voice — via `/chat/` only — without STT/TTS, and that bot / edge / tool lines appear in the transcript.

## in_scope

- This checklist + phases README / index handoff
- Preflight `go test` / build / path pointers
- No new product code

## out_scope

- Owner filling sign-off (human after real run)
- Supervisor soak (**S.4**)
- Dual call+chat prove (**V.1**)
- Claiming WAV/recording parity for chat (C.3 non-parity)
- FreeSWITCH DID automation

## forbidden

- Claiming **soak pass** without a completed sign-off row
- Using desk-era user/chat paths
- Skipping flow pin on chat create

## exit_criteria

- [x] Checklist covers Chat Start → answer → Send → Stop + evidence kinds
- [x] Sign-off block present
- [x] Points at Admin pin/publish prerequisites
- [x] No product code required for phase pass

## verification

```text
go test ./web/... ./internal/control/... -count=1 -timeout 180s
go build ./cmd/aiorchestrator/
Test-Path web/chat/index.html, web/chat/chat.js, docs/phases/C.4_chat_soak_checklist.md
```

## handoff

Wave **C.*** Closed for planning+implementation. Next programme: **S.1** Supervisor session list/detail, or owner runs this soak (+ A.6 / L.0) on lab.

---

# Chat soak checklist

**How to use:** Prefer a flow already published + answer-pinned via [A.6](./A.6_admin_soak_checklist.md). Open `/chat/` → tick only when observed. Owner signs after a real lab run.

**Defaults:** http://127.0.0.1:8011/; set Bearer in Chat token field if `AuthToken` is configured. Clock is fixed to `chat`.

## 0 — Preflight

| # | Check | Action | Pass criteria | ☐ |
|---|---|---|---|---|
| 0.1 | Build/tests | `go test ./web/... ./internal/control/... -count=1 -timeout 180s` | All `ok` | ☐ |
| 0.2 | Chat shell | Open `/chat/` | Start chat form + token row; not U.2 placeholder-only | ☐ |
| 0.3 | Catalogs load | Reload catalogs (or page load) | Profile + flow dropdowns populated | ☐ |
| 0.4 | Pin gate | Attempt Start with flow cleared (or API create `clock=chat` without `flow_id`) | Error / **422** `flow_pin_required` | ☐ |

## 1 — Prerequisites (Admin)

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 1.1 | Published flow | Admin Builder/Flows: published version of transfer or End fixture (e.g. [coral_xfer_minimal](../../tools/lab/flows/coral_xfer_minimal.v1.json)) | Flow listed; version ≥ 1 | ☐ |
| 1.2 | Profile + engines | Talk profile published; tenant engines set (Think may still be used) | Profile selectable in Chat | ☐ |
| 1.3 | Answer pin (optional) | `/admin/pin.html` — profile → flow → Save | Chat Start prefills flow when profile selected | ☐ |

## 2 — Start chat (answer)

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 2.1 | Details | Enter optional ANI / name; select profile + flow; version `latest` | Clock shows `chat` (readonly) | ☐ |
| 2.2 | Start | Click **Start chat** | Session id shown; welcome/bot line appears in turns | ☐ |
| 2.3 | No TTS dependency | Observe Start succeeds without Sarvam Speak (lab fakes OK) | Bot text present; no Speak gateway failure blocking chat | ☐ |

## 3 — Turns

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 3.1 | Send | Type an allowed choice/intent → **Send** | User turn + bot (or tool) reply in transcript | ☐ |
| 3.2 | Evidence kinds | Refresh transcript if needed | See `user_final` / `bot_utterance`; on transfer path also `edge_taken` and `tool_line` | ☐ |
| 3.3 | Empty send | Clear message → Send | Client error or no inject; no crash | ☐ |

## 4 — End / disposition

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 4.1 | Natural end | Walk flow to End or Tool transfer settle | Session state completed/draining/stopped; disposition line if available | ☐ |
| 4.2 | Stop | Or click **Stop** mid-chat | Session stops; polling ends | ☐ |
| 4.3 | Recording | Confirm product success **without** requiring WAV | Chat soak does not fail for missing recording file | ☐ |

## 5 — Cross-check (optional)

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 5.1 | API evidence | `GET /v1/sessions/{id}/transcript` + `/audit` | Matches UI kinds (C.3) | ☐ |
| 5.2 | Same flow live | After chat pass, optional [L.0](./L.0_graph_lab_soak.md) on same pin | Dual prove deferred to **V.1** | ☐ |

## Sign-off

| Field | Value |
|---|---|
| Date | |
| Operator | |
| Tip SHA | |
| Lab host | |
| Result | **soak pass** / **soak fail** / **deferred** |
| Notes | |

_Phase C.4 closes when this checklist exists in-repo. Lab **soak pass** requires a completed sign-off after a real Chat UI run._
