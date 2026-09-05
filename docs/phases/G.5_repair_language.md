# L3 — G.5 Repair + ListenLanguage + prompt locale

| Field | Value |
|---|---|
| **id** | `G.5` |
| **title** | Repair + ListenLanguage + prompt locale |
| **status** | **Closed** — repair + ListenLanguage + locale resolve |
| **parent_plan** | [G.0](./G.0_graph_runtime_inventory.md); [03](../03_BRAIN_AND_GRAPH.md); [P2.8](./P2.8_prompts_locale.md) |
| **depends_on** | G.4 Closed (`6870910`) |

## architecture_refs

- [03_BRAIN_AND_GRAPH.md](../03_BRAIN_AND_GRAPH.md) — per-node repair; ListenLanguage; EC-23
- [P2.8_prompts_locale.md](./P2.8_prompts_locale.md) — active → default_locale → fail closed
- [G.0_gap_list.md](./G.0_gap_list.md) G-RT-7, G-RT-8
- [10](../10_CODING_PRINCIPLES.md) EC-23 (ListenLanguage then Tool same turn forbidden)

## goal

On listen-like nodes, unclear utterances follow the node’s repair policy (reprompt, then drawn `repair` edge on exhaust). `ListenLanguage` sets `active_language` / cursor locale without arming a Tool in the same turn. Prompt resolve stays fail-closed per P2.8.

## in_scope

- Parse node `repair` (`max_retries`, `unclear_prompt_ref`); retry counter per node
- No-match → unclear Speak + stay; exhaust → sole `kind=repair` edge → advance (Tool/End/…)
- `ListenLanguage` utterance → option/intent locale; set cursor + Actor language; no Tool same turn
- Talk: apply locale switch; speak repair lines
- Unit + control tests
- Docs: this file + README

## out_scope

- Inform + binding (**G.6**)
- Silence `on_no_input` watchdog policy beyond existing silence arm
- Evidence emitters (**G.7**)
- Global `language_switch` edge catalogue (optional later)

## forbidden

- Inventing FAQ/transfer on unclear
- ListenLanguage success → Tool ARM same turn (EC-23)
- Silent invent of English when locale text missing

## exit_criteria

- [x] Unclear then exhaust follows repair edge
- [x] ListenLanguage sets locale; Tool same turn rejected
- [x] Prompt resolve uses active then default_locale
- [x] `go test` graph + control

## verification

```text
go test ./internal/runtime/graph/... ./internal/control/... ./internal/runtime/composer/... -count=1 -timeout 180s
```

## handoff

Next: **G.6** Inform + knowledge binding.
