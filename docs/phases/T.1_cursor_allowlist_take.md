# L3 — T.1 Cursor allowlist + TakeEdgeID

| Field | Value |
|---|---|
| **id** | `T.1` |
| **title** | Cursor allowlist + TakeEdgeID |
| **status** | **Ready** |
| **parent_plan** | [14_THINK_EDGE_PICK.md](../14_THINK_EDGE_PICK.md) |
| **depends_on** | T.0 Closed |

## architecture_refs

- [03](../03_BRAIN_AND_GRAPH.md) — cursor; ListenLanguage; EC-23
- [G.3](./G.3_runtime_core_cursor.md) / [G.5](./G.5_repair_language.md)
- [10](../10_CODING_PRINCIPLES.md) EC-20, EC-23

## goal

Expose current-node listen allowlist (intent/option only) and `TakeEdgeID` that validates then advances — kernel owns repair edges; ListenLanguage locale + no Tool same turn preserved.

## in_scope

- `internal/runtime/graph/cursor.go` — AllowlistedListenEdges; TakeEdgeID
- Unit tests including ListenLanguage alias locale + EC-23
- Keep `HandleUtterance`/`matchChoice` for tests until T.3 removes Talk use

## out_scope

- Think calls; Talk wire-up (T.2/T.3)

## forbidden

- Letting LLM-visible allowlist include `kind=repair`
- Breaking existing cursor unit tests

## exit_criteria

- [ ] TakeEdgeID rejects foreign/illegal ids
- [ ] ListenLanguage TakeEdgeID sets locale; Tool same turn still forbidden
- [ ] `go test ./internal/runtime/graph/ -count=1`

## edge_cases

- Empty allowlist; duplicate labels; EC-23

## verification

```text
go test ./internal/runtime/graph/ -count=1 -timeout 60s
```

## rollback

Revert cursor API; Talk still on matchChoice.

## handoff

Next: **T.2** edgepick classifier.
