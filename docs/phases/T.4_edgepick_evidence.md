# L3 — T.4 Edgepick evidence

| Field | Value |
|---|---|
| **id** | `T.4` |
| **title** | Edgepick audit / turn evidence |
| **status** | **Ready** |
| **parent_plan** | [14_THINK_EDGE_PICK.md](../14_THINK_EDGE_PICK.md) |
| **depends_on** | T.3 Closed; E.4 Closed |

## architecture_refs

- [09](../09_EVIDENCE_AND_RECORDING.md) — audit behaviour
- [E.4](./E.4_audit_allowlist.md) — allowlisted event types
- EC-30 / EC-33

## goal

Record enough evidence to explain why a listen turn took an edge or repaired (proposal, accept/reject, think_error, latency) without logging secrets or dumping full vendor prompts with keys.

## in_scope

- Allowlisted audit event type(s) or turn.state payload fields for edgepick
- Catalog update if new type added
- Tests that emit on accept / reject / think_error

## out_scope

- Supervisor UI redesign (existing audit browser must show new type if added)

## forbidden

- Logging API keys or auth tokens
- Inventing audit types outside allowlist process

## exit_criteria

- [ ] Lab/control test shows edgepick decision in audit or turn.state
- [ ] No secret material in payloads

## edge_cases

- Think timeout still emits think_error + repair outcome

## verification

```text
go test ./internal/control/... -count=1 -timeout 180s -run Edgepick
```

## rollback

Stop emitting new fields/types; T.3 behaviour remains.

## handoff

Next: **T.5** soak.
