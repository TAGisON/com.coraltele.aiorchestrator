# L3 — V.1 Dual-channel prove (same published flow)

| Field | Value |
|---|---|
| **id** | `V.1` |
| **title** | Same published flow: chat soak + call soak |
| **status** | **Closed** — checklist authored |
| **parent_plan** | [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) Wave V; OD-13-7 |
| **depends_on** | S.4 Closed (`23d96f7`); C.4 / A.6 / L.0 checklists exist |

## architecture_refs

- [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) — Chat ≡ call brain; dual prove
- [C.4_chat_soak_checklist.md](./C.4_chat_soak_checklist.md) — chat UI path
- [L.0_graph_lab_soak.md](./L.0_graph_lab_soak.md) — live/API graph path (DID or lab)
- [A.6_admin_soak_checklist.md](./A.6_admin_soak_checklist.md) — publish + pin via Admin
- [S.4_supervisor_soak_checklist.md](./S.4_supervisor_soak_checklist.md) — inspect evidence
- Fixture: [tools/lab/flows/coral_xfer_minimal.v1.json](../../tools/lab/flows/coral_xfer_minimal.v1.json)

## goal

Prove **one** published `coral.flow.v1` pin works on **both** channels: text chat (`clock=chat` via `/chat/`) and voice/call (live or lab playback/DID per L.0) — same welcome → choice → transfer/hangup behaviour and evidence kinds — then inspect both in Supervisor.

## in_scope

- This dual-channel checklist + phases README / index handoff
- Cite existing soaks (do not duplicate full A.6/C.4/L.0 tables)
- Sign-off for dual **pass**
- No new product code

## out_scope

- Lab performance notes (**V.2**, optional)
- Owner filling sign-off (human after real runs)
- New features / QM / CRM
- Claiming production cutover without CD.0 promote

## forbidden

- Claiming dual **soak pass** without completed V.1 sign-off
- Using different unpublished graphs for “chat pass” vs “call pass”
- Desk revive

## exit_criteria

- [x] Checklist requires same `flow_id`+`flow_version` for chat and call
- [x] Points at C.4 + L.0 (or Admin lab live/playback) + S.4 inspect
- [x] Sign-off block for dual result
- [x] No product code required for phase pass

## verification

```text
Test-Path docs/phases/V.1_dual_channel_prove.md, tools/lab/flows/coral_xfer_minimal.v1.json
go test ./web/... ./internal/control/... -count=1 -timeout 180s
```

## handoff

Wave **V.1** Closed (checklist). Optional **V.2** lab performance notes. Programme consoles L3 catalog complete for V1 dual prove; owner runs soaks on lab.

---

# Dual-channel prove checklist

**How to use:** Publish **one** flow (Admin A.6 / fixture). Run chat path, then call path against the **same** pin. Inspect both sessions in Supervisor. Tick only when observed. Owner signs after real runs.

**Defaults:** http://127.0.0.1:8011/; Bearer if configured.

## 0 — Shared pin

| # | Check | Action | Pass criteria | ☐ |
|---|---|---|---|---|
| 0.1 | Publish | Admin Builder/Flows: publish fixture or equivalent graph | Record `flow_id` + `flow_version` (or `latest` resolved version) | ☐ |
| 0.2 | Pin | Answer pin maps Talk profile → that flow | Pin saved | ☐ |
| 0.3 | Same pin | Write pin ids here for both channels | Chat and call use **identical** flow pin | ☐ |

**Pinned flow:** `flow_id=` ________  `flow_version=` ________  `profile_id=` ________

## 1 — Chat channel (C.4 slice)

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 1.1 | Start | `/chat/` → Start with pin, `clock=chat` | Welcome/bot line | ☐ |
| 1.2 | Turn | Send allowed intent (e.g. sales / done) | Bot or tool line; graph progresses | ☐ |
| 1.3 | Evidence | Transcript shows expected kinds | `user_final` / `bot_utterance`; transfer path → `edge_taken` / `tool_line` | ☐ |
| 1.4 | Session id | Record chat `session_id` | ________ | ☐ |

## 2 — Call channel (L.0 / Admin lab)

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 2.1 | Create | Same pin; `clock=live` (DID) **or** lab `playback`/`live` via Admin pin helper / API | Session created with flow pin | ☐ |
| 2.2 | Answer | Answer + walk same choice/intent as chat | Welcome + transfer/hangup or End as designed | ☐ |
| 2.3 | Evidence | Transcript/audit | Same kinds as chat for the same graph walk | ☐ |
| 2.4 | Session id | Record call `session_id` | ________ | ☐ |

_If owner DID unavailable: mark call path **deferred** with playback+pin lab substitute and note in sign-off — dual **pass** still requires a live DID run later._

## 3 — Supervisor inspect (S.4 slice)

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 3.1 | Both listed | `/supervisor/` refresh | Both session ids visible | ☐ |
| 3.2 | Chat detail | Open chat session | Transcript + disposition (+ audit if tools) | ☐ |
| 3.3 | Call detail | Open call session | Transcript + disposition; recording meta if live recording on | ☐ |
| 3.4 | Summary | Light summary refresh | Counts reflect recent sessions | ☐ |

## Sign-off

| Field | Value |
|---|---|
| Date | |
| Operator | |
| Tip SHA | |
| Lab host | |
| Chat session | |
| Call session | |
| Call mode | live DID / playback substitute |
| Result | **dual pass** / **chat-only pass** / **soak fail** / **deferred** |
| Notes | |

_Phase V.1 closes when this checklist exists in-repo. Product **dual pass** requires a completed sign-off after real chat + call runs on the same pin._
