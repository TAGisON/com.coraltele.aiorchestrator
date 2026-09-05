# L3 — T.5 Think edge pick soak checklist

| Field | Value |
|---|---|
| **id** | `T.5` |
| **title** | Think edge pick lab soak |
| **status** | **Ready** |
| **parent_plan** | [14_THINK_EDGE_PICK.md](../14_THINK_EDGE_PICK.md) |
| **depends_on** | T.4 Closed |

## architecture_refs

- [14](../14_THINK_EDGE_PICK.md)
- Lab: http://127.0.0.1:8011/chat/
- Fixture: [tools/lab/flows/coral_inbound_triage.v2.json](../../tools/lab/flows/coral_inbound_triage.v2.json)

## goal

Human/lab checklist proving Sarvam Think edge pick on the inbound triage desk (Hinglish, cage rejects, repair on Think fail) without claiming free-flow AI.

## in_scope

- This checklist + sign-off table
- Preflight: Sarvam credentials set; engines/profile Sarvam-only; flow pin latest

## out_scope

- Claiming chat transfer success (no telephony leg)
- Emotion / generative FAQ

## forbidden

- Claiming soak pass without sign-off row
- Re-enabling keyword fallback to “make soak pass”

## exit_criteria

- [ ] Checklist authored with Hinglish + illegal/jailbreak + Think-down rows
- [ ] Owner sign-off block present

## verification

```text
Test-Path docs/phases/T.5_think_edge_soak.md
go test ./internal/runtime/graph/... ./internal/runtime/composer/... ./internal/control/... -count=1 -timeout 180s
```

## Soak rows (author must keep)

| # | Scenario | Pass |
|---|---|---|
| 1 | Hinglish product ask → product/FAQ path without English-only token luck | ☐ |
| 2 | Model/illegal edge simulation → repair, no jump | ☐ |
| 3 | Credentials removed / Think fail → repair, call continues | ☐ |
| 4 | ListenLanguage then no Tool same turn | ☐ |
| 5 | Audit shows edgepick decision | ☐ |

## Sign-off

| Role | Name | Date | Result |
|---|---|---|---|
| Owner | | | Pass / Fail |

## rollback

N/A (docs). Product rollback = revert T.3.

## handoff

Programme T.* Closed after owner Pass; next optional emotion/FAQ generative under new L2.
