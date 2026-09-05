# L3 — G.0 Graph runtime inventory

| Field | Value |
|---|---|
| **id** | `G.0` |
| **title** | Graph / `flow_*` runtime inventory + G.* breakdown |
| **status** | **Closed** — gap list filed |
| **parent_plan** | [03](../03_BRAIN_AND_GRAPH.md), [01](../01_VISION_AND_SCOPE.md), [P2.7](./P2.7_flow_publish_model.md) |
| **depends_on** | P2.7–P2.10 Locked; M-A/B/C DDL Closed; E.4–E.5 Closed; CD.1 Closed |

## architecture_refs

- [03_BRAIN_AND_GRAPH.md](../03_BRAIN_AND_GRAPH.md)
- [04_LIVE_TURN_MACHINE.md](../04_LIVE_TURN_MACHINE.md)
- [01_VISION_AND_SCOPE.md](../01_VISION_AND_SCOPE.md) — V1 done definition
- [P2.7](./P2.7_flow_publish_model.md) … [P2.10](./P2.10_bindings_redesign.md)
- [P2.9](./P2.9_routing_matrix.md) — matrix ARM
- Existing DDL: `010_flow_registry.sql`, `011_binding.sql`, `012_session_flow_pin.sql`

## goal

Map DDL + locks to live code and file an explicit gap list so later **G.1–G.7** Implementers can replace the missing dialogue brain without inventing topology or dual desk/flow brains.

## in_scope

- Read-only inventory of flow/binding/session-pin store, control APIs, composer/thinkpath vs graph cursor
- Gap list: [G.0_gap_list.md](./G.0_gap_list.md) (and local `.agent/work/G.0/gap_list.md`)
- Proposed **G.1–G.7** phase breakdown (this file + README)
- Docs only — no product code

## out_scope

- Implementing store/API/walker (**G.1+**)
- Admin UI
- JSON Schema file for flow doc
- Desk→flow migrator
- Absorbing E/CI/CD work

## forbidden

- Writing a second dialogue brain beside desk (desk is gone — do not resurrect)
- LLM-authored dial numbers
- Claiming V1 call-flow complete without walker + tools

## exit_criteria

- [x] [G.0_gap_list.md](./G.0_gap_list.md) exists with CFG / RT / EV gaps
- [x] G.1–G.7 sketched with depends-on intent
- [x] README G.* section started
- [x] No product code required for pass

## Proposed G.* (planning — implement one at a time)

| id | title | depends_on | exit sketch |
|---|---|---|---|
| **G.1** | Store flow + binding + session pin fields | G.0 | Memory+PG CRUD; Session.FlowID/Version round-trip |
| **G.2** | Flow control API + `coral.flow.v1` validate/publish | G.1 | Publish immutable version; reject bad envelope |
| **G.3** | Runtime core cursor (Entry/Speak/ListenChoice/End) | G.2 | Pinned live session walks welcome → choice → End |
| **G.4** | Tool transfer/hangup arm→speak→exec + matrix | G.3 | Matrix number frozen; E.5 dispositions on settle |
| **G.5** | Repair + ListenLanguage + prompt locale | G.4 | Exhausted repair follows drawn edge |
| **G.6** | Inform + knowledge binding | G.5 | FAQ path when binding present; fail closed if missing and required |
| **G.7** | Evidence + live cutover (require flow pin) | G.3+ | `edge_taken`/`tool_line`; refuse new live without pin |

## verification

```text
Test-Path docs/phases/G.0_gap_list.md
git diff --stat -- internal/
# expect no product code for this phase
```

## handoff

Next: **G.1** store APIs for `flow_*` / `binding` / session pins — say **continue**.
