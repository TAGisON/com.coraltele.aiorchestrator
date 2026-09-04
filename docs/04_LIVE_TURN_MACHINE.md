# 04 — Live turn machine

**Status: LOCKED (prose).**

Runs on every live call above the dumb PCM pipe. Decides when STT may move the graph, when barge is allowed, and when a Tool is irreversible.

## Goals

1. No false FAQ/transfer from bot echo or fragments while speaking (unless barge commits a real utterance).
2. Armed Tool always completes: closing line → execute.
3. Transcript records speech even when it does not drive the graph.
4. Unclear speech is repaired by the **current node**, not by escaping the graph.

## States

| State | Meaning |
|---|---|
| `Idle` | Not in live talk yet |
| `Speaking` | TTS playing / draining |
| `Listening` | Waiting for caller; finals may be actionable |
| `Thinking` | Resolving one utterance against the graph |
| `ToolArmed` | Transfer/hangup locked; barge off |
| `ToolExecuting` | Edge control verb in flight |
| `Ending` | Session + recording teardown |

```text
Idle → Speaking (welcome)
Speaking → Listening
Listening → Thinking
Thinking → Speaking | ToolArmed
ToolArmed → Speaking (closing) → ToolExecuting → Ending
```

## Actionable vs transcript-only STT

| Kind | Effect |
|---|---|
| Transcript-only | Audit/transcript with reason; **no** graph move |
| Actionable | May enter `Thinking` and select an edge |

### When actionable

| State | Actionable? |
|---|---|
| `Listening` | Yes, if filters pass |
| `Speaking` + barge allowed | Yes only after barge **commit** (real utterance, not echo) |
| `Speaking` + barge forbidden | No |
| `Thinking` | No (single-flight) |
| `ToolArmed` / `ToolExecuting` / `Ending` | No |

### Filters (V1 minimum)

Non-empty; min length; not likely TTS echo; not tool-locked.

## Barge

- Allowed only when the active Speak/listen prompt says so.
- After **Tool ARM**: barge forbidden until settle/end.
- Commit = actionable STT final (not energy alone as the decision).

## Tool lock sequence

```text
Graph selects Tool
  → ToolArmed + freeze params
  → barge_allowed = false
  → Speak closing prompt (STT transcript-only)
  → drain playout
  → ToolExecuting → transfer | hangup to edge
  → wait settle
  → Ending → stop recording, finalize session
```

Do not tear down the WebSocket before the hangup/transfer verb is armed and given a chance to settle.

## Thinking rules

1. One Thinking at a time per session.
2. Input: utterance + cursor + allowlisted edges + slots.
3. Output: legal `edge_id` | `retry` | `unclear`.
4. Validate edge exists.
5. `ListenLanguage` success updates language then leaves via edge — **never** Tool same turn.

## Silence / OOD

Driven as graph repair edges while in `Listening` (nudges, then exhaust → hangup Tool).  
Do not advance OOD ladders during `Speaking` / `ToolArmed` from “no reply”.

## Transcript (intent)

Continuous events, not only neat user/assistant pairs after a turn:

- bot speak start/end  
- user final (actionable true/false + reason)  
- graph edge taken  
- tool armed / executed / failed  

(Exact event names deferred; behaviour locked here.)
