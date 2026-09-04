# 03 — Brain and conversation graph

**Status: LOCKED (prose).** Schema JSON is deferred.

## Core idea

```text
Admin configures a graph (nodes + edges + bindings).
Runtime cursor sits on exactly one node.
LLM proposes only legal edges from that node (or unclear → repair).
STT/TTS convert speech ↔ text.
Tools (transfer / hangup) arm → speak → execute once.
```

## Graph pieces

| Piece | Meaning |
|---|---|
| Graph | Whole call flow for one published profile/desk |
| Node | Place in the call; **type** defines behaviour |
| Edge | Legal move (forward, back, retry, skip, global, tool_result, …) |
| Cursor | Current node |
| Slots | Collected facts (`active_language`, department, …) |
| Tool | Irreversible action with frozen params |
| Binding | External KB/CRM/FAQ dump config referenced by nodes |

## Node types (V1 closed set)

| Type | Job |
|---|---|
| `Entry` | Call start; optional ANI prefs → slots |
| `Speak` | Play one prompt; then `next` |
| `ListenChoice` | Map utterance → option/intent/skip edges |
| `ListenSlot` | Fill one validated slot |
| `ListenLanguage` | Set/switch `active_language` only (never Tool same turn) |
| `Decide` | Silent branch on slots |
| `Inform` | Grounded FAQ/KB answer via knowledge **binding** |
| `Tool` | Arm transfer/hangup → closing Speak → execute |
| `End` | Terminal disposition; teardown |

## Edges

Legal kinds: `next`, `option`, `intent`, `retry`, `back`, `skip`, `repair`, `tool_result`, `global`.

- **No edge ⇒ jump illegal**, even if the LLM “wants” it.
- Back / retry / skip are normal edges when admin draws them.
- Globals (e.g. `language_switch`, optional `transfer_sales`) are reusable edges; nodes opt in.

## Tool semantics

```text
Select Tool edge
  → ARM (params from matrix/slots — never free LLM dial strings)
  → barge OFF
  → Speak closing line (if configured)
  → EXECUTE once
  → settle (leg leaves or hangup)
  → Ending
```

Same machine for **transfer** and **hangup**.

## Repair (unclear / incomplete / out of context)

Each listen-like node owns:

| Policy | Typical behaviour |
|---|---|
| `on_unclear` / `on_no_match` | Retry same node (reprompt) |
| `on_no_input` | Silence nudge |
| `max_retries` | e.g. 3 |
| `on_exhausted` | Edge to Tool hangup, menu, or human — **drawn by admin** |

Incomplete fragments, unrelated asks, and disallowed jumps all land in **repair**, not in inventing FAQ/transfer.

## Bindings vs nodes

| Need | Mechanism |
|---|---|
| When to answer FAQ | `Inform` node + edge into it |
| FAQ dump / KB corpus / retrieve HTTP | Knowledge **binding** on profile |
| CRM lookup/write | Skill **binding**; optional; V1 may omit |
| Transfer number | Routing **matrix** → Tool params |

Missing binding ⇒ that desk simply has no such node/edge (or fails closed).

## Language

- One graph for all locales (no hi-graph vs en-graph).
- Prompts resolved by `active_language`.
- Mid-call switch only via `ListenLanguage` / global `language_switch`.
- Talk allowlist ⊆ vendor TTS ∩ STT capabilities.

## Minimal voicebot shape (coral-xfer class)

```text
Entry → Speak(welcome)
     → ListenLanguage? (if unlocked)
     → ListenChoice(department)
          → Tool(transfer *) 
          → Inform(FAQ) → listen anything else / back
          → repair exhaust → Tool(hangup)
     Globals: language_switch (from allowed listen nodes)
```
