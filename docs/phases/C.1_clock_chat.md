# L3 — C.1 Text-channel session (`clock=chat`)

| Field | Value |
|---|---|
| **id** | `C.1` |
| **title** | `clock=chat` — text channel, flow pin, no STT/TTS |
| **status** | **Closed** (`c8bfe3f`) |
| **parent_plan** | [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) OD-13-7; Wave C |
| **depends_on** | A.6 Closed (`c53b441`); G.7 live pin gate; U.1 catalogs `chat` |

## architecture_refs

- [04_LIVE_TURN_MACHINE.md](../04_LIVE_TURN_MACHINE.md) — speak/listen; tools unchanged
- [03_BRAIN_AND_GRAPH.md](../03_BRAIN_AND_GRAPH.md) — graph is law
- OD-13-4 — inject + events for chat transport
- OD-13-7 — `clock=chat`; STT/TTS no-op
- `internal/runtime/clock`, `composer.Talk.speak`, `handleCreateSession`

## goal

Allow sessions with `clock=chat` that require the same published flow pin as live, walk the same graph via answer/inject, and **do not** call STT or TTS (text-only bot lines still land in transcript/memory).

## in_scope

- `clock.Chat` kind; `clock.New("chat")` → VAD off, playback-like pace
- Create session: `clock=chat` requires `flow_id`/`flow_version` (same gate as live); reject unknown clock strings
- `Talk.OnPCM` no-op for chat (like playback)
- `Talk.speak` / SpeakLine path: skip Speak gateway; still append assistant text + observe; transition Listening
- Control test: create chat+pin → answer → inject → transcript has bot text without needing Sarvam TTS
- Catalog already lists `chat` (U.1) — no change unless missing
- This phase file + README

## out_scope

- Chat UI bubbles (**C.2**)
- Evidence parity checklist polish (**C.3**)
- Skipping tenant engines entirely (Think may still be configured; Speak unused)
- FreeSWITCH edge for chat
- Changing tool ARM semantics

## forbidden

- Chat sessions inventing topology outside graph
- Live/chat without flow pin
- Desk revive

## exit_criteria

- [x] `clock=chat` without pin → 422 `flow_pin_required`
- [x] Chat session answer + inject works with flow pin
- [x] Speak path does not require Speak gateway success for chat (no TTS call)
- [x] `go test` for clock/composer/control green

## edge_cases

1. Unknown clock e.g. `foo` → 400.
2. Playback remains without pin (G.7).
3. Tool arm still runs (transfer/hangup) even on chat — lab may stub CallControl.

## verification

```text
go test ./internal/runtime/clock/... ./internal/runtime/composer/... ./internal/control/... -count=1 -timeout 180s
```

## rollback

Revert chat clock + speak short-circuit; catalog may still list `chat`.

## handoff

Next: **C.2** Chat UI (details → turns → bot/tool lines).
