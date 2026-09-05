# L3 — C.3 Chat evidence parity

| Field | Value |
|---|---|
| **id** | `C.3` |
| **title** | Evidence parity on `clock=chat` path |
| **status** | **Closed** |
| **parent_plan** | [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) Wave C; [09](../09_EVIDENCE_AND_RECORDING.md) B1–B3 |
| **depends_on** | C.1–C.2 Closed; G.7 emitters; E.1–E.5 Closed |

## architecture_refs

- [09_EVIDENCE_AND_RECORDING.md](../09_EVIDENCE_AND_RECORDING.md) — transcript / audit / disposition truth hierarchy
- [G.7_evidence_cutover.md](./G.7_evidence_cutover.md) — `edge_taken` / `tool_line` / `graph.edge`
- [C.1_clock_chat.md](./C.1_clock_chat.md) — text channel; STT/TTS unused
- [U.0_consoles_inventory.md](./U.0_consoles_inventory.md) — chat transcript parity gap → C.3
- OD-13-7 — chat ≡ call brain; media differs only

## goal

Prove (and keep) that a pinned `clock=chat` session writes the same **product** transcript and graph audit evidence as live for the same published flow — without requiring STT/TTS or PCM recording success.

## in_scope

- Control test: chat + transfer flow → answer → inject intent → assert:
  - transcript has `edge_taken` and `tool_line` (and `bot_utterance` / `user_final` as applicable)
  - audit has `graph.edge` (and tool lifecycle allowlist events when tools run)
  - disposition final settles after tool (same as live)
- Document explicit **non-parity**: PCM/`recording.*` not required for chat channel (WAV SoT still forensic for live only)
- Chat UI: style/show `edge_taken` turns distinctly (alongside existing `tool_line`) so lab can see parity without Supervisor
- This phase file + README / index handoff
- `.agent/work/C.3/implementation.md`

## out_scope

- Chat soak checklist (**C.4**)
- Supervisor browsers (**S.***)
- Dual call+chat soak (**V.1**)
- Changing live recording lifecycle
- New DDL / new event kinds
- Requiring dual-channel WAV for chat

## forbidden

- Inventing a second chat-only transcript schema
- Dropping `edge_taken` / `tool_line` on chat to “simplify”
- Claiming recording parity for a text-only channel
- Desk revive

## exit_criteria

- [x] Chat control test proves `edge_taken` + `tool_line` + `graph.edge` on `clock=chat`
- [x] Disposition final present after transfer settle on chat (or documented equivalent End hangup path)
- [x] Chat UI marks `edge_taken` / `tool_line` in the turn list
- [x] Phase notes recording non-parity for chat
- [x] `go test ./web/... ./internal/control/...` green

## Recording non-parity (explicit)

Chat is text-only (`clock=chat`). Product SoT remains transcript + audit + disposition ([09](../09_EVIDENCE_AND_RECORDING.md) ranks 1–3). PCM / `recording.*` / WAV are **not** required for chat channel success; live call recording rules are unchanged.

## edge_cases

1. Chat speak short-circuit must still run Observer after speak (already C.1) — regression-guarded here.
2. Tool transfer still needs CallControl sink in lab tests (same as G.7 live).
3. Missing Speak gateway must not block chat evidence (C.1).

## verification

```text
go test ./web/... ./internal/control/... -count=1 -timeout 180s
go build ./cmd/aiorchestrator/
```

## rollback

Revert chat evidence test + UI edge styling; keep C.1/C.2.

## handoff

Next: **C.4** Chat soak checklist.
