# L3 — T.2 Edgepick Think classifier

| Field | Value |
|---|---|
| **id** | `T.2` |
| **title** | Edgepick Think JSON classifier |
| **status** | **Ready** |
| **parent_plan** | [14_THINK_EDGE_PICK.md](../14_THINK_EDGE_PICK.md) |
| **depends_on** | T.1 Closed |

## architecture_refs

- [04](../04_LIVE_TURN_MACHINE.md) — Thinking rules
- [05](../05_MEDIA_AND_VENDORS.md) — Think behind port
- [14](../14_THINK_EDGE_PICK.md) — classifier contract
- [10](../10_CODING_PRINCIPLES.md) EC-20, EC-21

## goal

Add a small classifier that calls `port.Think.Complete` with a strict JSON contract and returns allowlisted `edge_id` or unclear/retry — no freestyle speech, no dial strings.

## in_scope

- New package e.g. `internal/runtime/graph/edgepick`
- Prompt: node, locale, retries, allowlist, user_text only
- Parse JSON; strip fences; ignore extra fields; reject illegal ids
- Timeout via context; unit tests with fake Think

## out_scope

- Wiring into Talk (T.3)
- Audit emitters (T.4)

## forbidden

- Putting matrix numbers or tool dial targets in the prompt
- Keyword matchChoice inside the classifier

## exit_criteria

- [ ] Fake Think happy path + bad JSON + illegal id + timeout covered
- [ ] `go test` for edgepick package green

## edge_cases

- Jailbreak prose; multi-intent ambiguity → unclear; empty allowlist skip Think

## verification

```text
go test ./internal/runtime/graph/... -count=1 -timeout 60s
```

## rollback

Delete edgepick package; T.1 remains.

## handoff

Next: **T.3** Talk wire-up.
