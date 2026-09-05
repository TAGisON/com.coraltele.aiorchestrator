# L3 — C.2 User chat console UI

| Field | Value |
|---|---|
| **id** | `C.2` |
| **title** | Chat UI — details → turns → bot/tool lines |
| **status** | **Closed** |
| **parent_plan** | [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) Wave C; OD-13-1/4/7 |
| **depends_on** | C.1 Closed (`c8bfe3f`); U.2 shells; A.5 pins (optional prefill) |

## architecture_refs

- [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) — User chat capability inventory
- [C.1_clock_chat.md](./C.1_clock_chat.md) — `clock=chat` + flow pin + no TTS
- OD-13-4 — `POST …/inject` + transcript (SSE optional refresh aid)
- `web/chat/`, `web/shared/api.js`, Control session/answer/inject/transcript/stop

## goal

Replace the Chat shell placeholder with a usable lab console: enter caller details, create a `clock=chat` session with a published flow pin, answer the graph, exchange text turns, and see bot / tool transcript lines.

## in_scope

- `web/chat/index.html` + `web/chat/chat.js` (or equivalent):
  - Token row (shared `OrchAPI`)
  - Start form: ANI / name (caller JSON), profile + flow dropdowns, flow version (default `latest`), clock fixed to `chat`
  - Optional prefill from `GET /v1/tenant/answer-pins` when profile matches
  - Actions: Start (create → answer), Send (`inject`), Stop, Refresh transcript
  - Turn list from `GET …/transcript` showing `role`, `text`, `event_kind` (bot / user / `tool_line` styled distinctly)
  - Session id + state probe
- Extend `OrchAPI` with session helpers used by Chat (`getSession`, `answerSession`, `injectText`, `stopSession`, `getTranscript`, `getDisposition` as needed)
- Minimal shared CSS for chat turn bubbles (reuse console tokens)
- Static serve tests updated for Chat UX strings + embed `chat/chat.js`
- This phase file + README / index handoff

## out_scope

- Evidence parity checklist polish (**C.3**)
- Chat soak checklist (**C.4**)
- Admin/Supervisor product work
- Dedicated WebSocket transport
- FreeSWITCH / live PCM path
- New Control routes (use existing APIs only)
- New DDL

## forbidden

- Bypassing graph (no free-typed bot replies)
- Admin write controls on Chat (no publish/engines)
- Desk-era UI revive
- Requiring STT/TTS for this channel

## exit_criteria

- [x] `/chat/` serves Start / Send / transcript UI (not U.2 placeholder only)
- [x] Create uses `clock=chat` + flow pin fields
- [x] Turns render from transcript including assistant and `tool_line` when present
- [x] `OrchAPI` exposes inject/answer/transcript helpers
- [x] `go test ./web/... ./internal/control/...` green; `go build ./cmd/aiorchestrator/`

## edge_cases

1. Missing flow pin → surface Control `flow_pin_required` error.
2. Empty inject text → disable Send or show client error.
3. Session completed/stopped → stop polling; show final state.
4. No answer pins → manual profile/flow select still works.

## verification

```text
go test ./web/... ./internal/control/... -count=1 -timeout 180s
go build ./cmd/aiorchestrator/
```

## rollback

Revert `web/chat/*` + `api.js` helpers; keep C.1 runtime.

## handoff

Next: **C.3** evidence parity on chat path.