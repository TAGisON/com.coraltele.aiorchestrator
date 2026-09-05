# L3 — T.3 Talk wire graph turns to edgepick

| Field | Value |
|---|---|
| **id** | `T.3` |
| **title** | Talk runGraphTurn uses edgepick |
| **status** | **Ready** |
| **parent_plan** | [14_THINK_EDGE_PICK.md](../14_THINK_EDGE_PICK.md) |
| **depends_on** | T.2 Closed |

## architecture_refs

- [04](../04_LIVE_TURN_MACHINE.md) — single-flight Thinking; barge cancel
- [composer](../../internal/runtime/composer/composer.go) — `runGraphTurn`
- EC-10, EC-13, EC-14 (gates unchanged — classifier only when actionable listen)

## goal

On ListenChoice/ListenLanguage, `runGraphTurn` uses edgepick + TakeEdgeID (or repair). Spoken lines remain prompts/Inform/Tool only. No keyword path on production Talk.

## in_scope

- `internal/runtime/composer/composer.go` — runGraphTurn
- Resolve Think from session gateway binding (`sarvam-llm`)
- Think fail → repairLocked path
- Control/chat test with fake Think (Hinglish → fixed edge_id)

## out_scope

- Full audit schema (T.4 may add fields)
- Emotion / generative FAQ

## forbidden

- Calling matchChoice from runGraphTurn
- Speaking Think.Text to the caller as the answer

## exit_criteria

- [ ] Fake-Think inject test: Hinglish maps without English token overlap requirement
- [ ] Think error → repair, no panic
- [ ] `go test ./internal/runtime/composer/... ./internal/control/... -count=1`

## edge_cases

- Barge cancel mid-Think; second utterance while Thinking (unchanged gates)

## verification

```text
go test ./internal/runtime/composer/... ./internal/control/... ./internal/runtime/graph/... -count=1 -timeout 180s
```

## rollback

Restore matchChoice in runGraphTurn.

## handoff

Next: **T.4** evidence.
