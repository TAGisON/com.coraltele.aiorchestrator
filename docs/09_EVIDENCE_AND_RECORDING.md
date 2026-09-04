# 09 — Evidence: transcript, audit, recording

**Status:** **Locked** (owner go-ahead 2026-09-04 — “continue now”).  
**Layer:** L2 domain plan per [07_PLANNING_STANDARDS.md](./07_PLANNING_STANDARDS.md) §3.  
**Parent / architecture refs:**

| Doc | Role |
|---|---|
| [01_VISION_AND_SCOPE.md](./01_VISION_AND_SCOPE.md) | Transcription + recording stop are V1 |
| [02_CURRENT_STATE.md](./02_CURRENT_STATE.md) | Continuous events; recording leak lesson; WAV≠orch order |
| [03_BRAIN_AND_GRAPH.md](./03_BRAIN_AND_GRAPH.md) | Tools; graph edges as facts |
| [04_LIVE_TURN_MACHINE.md](./04_LIVE_TURN_MACHINE.md) | Actionable vs transcript-only; Ending; transcript intent |
| [05_MEDIA_AND_VENDORS.md](./05_MEDIA_AND_VENDORS.md) | PCM in-memory path only |
| [06_APPLICATION_FLOW.md](./06_APPLICATION_FLOW.md) | Finalize path |
| [08](./08_PURGE_AND_SCHEMA_PHASES.md) + [phases/P2.0–P2.6](./phases/README.md) | Storage locks this behaviour must not contradict |

---

## Goal

Define **observable evidence behaviour** for every live call: what is written to transcript and audit, when recording starts/stops, and how failures degrade — so runtime Implementers do not invent “turn-pair only” or leave recorders running.

---

## In scope

- Transcript event emission rules (aligned with [P2.3](./phases/P2.3_transcript_events.md)).  
- Audit emission rules (aligned with [P2.4](./phases/P2.4_audit_events.md)).  
- Recording lifecycle behaviour (aligned with [P2.5](./phases/P2.5_recording_metadata.md)).  
- Disposition write timing vs tools ([P2.6](./phases/P2.6_disposition.md)).  
- Truth hierarchy when sources disagree.  
- Degrade / missing / timeout rows.  
- Future L3 runtime phase ids **E.*** (evidence runtime — after P1 purge + relevant P2 DDL).  

## Out of scope

- Schema DDL (owned by P2.*).  
- Supervisor UI to view evidence (new UI later).  
- Post-call summary / QM / CRM push ([01](./01_VISION_AND_SCOPE.md) Next).  
- Offline forensic WAV re-STT as orch SoT.  
- Captions / translator SKUs.  
- Storing PCM in Postgres ([P2.0](./phases/P2.0_schema_principles.md) P2.0-P6).  

---

## Open decisions

| ID | Question | Decision |
|---|---|---|
| **OD-09-1** | Recording channel layout V1 | **SETTLED: mono mix** (caller+bot into one file keyed by `recording_ref`). Dual-channel = Later. |
| **OD-09-2** | Emit `bot_speak_start` / `bot_speak_end` in V1 | **SETTLED: optional / not required.** V1 minimum kinds per P2.3: `bot_utterance`, `user_final`, `edge_taken`, `tool_line`. |
| **OD-09-3** | Orch transcript vs offline WAV STT | **SETTLED:** orch event stream is **source of truth** for product transcript. WAV is forensic only; disagreement is expected under barge ([02](./02_CURRENT_STATE.md)). |
| **OD-09-4** | Transfer: stop orch recording when? | **SETTLED:** when orch leg reaches **Ending** after tool settle (same as hangup). Post-transfer human leg is **not** orch’s recording in V1. |

No remaining open decisions for locking this doc after owner sign-off.

---

## Truth hierarchy

| Rank | Source | Use |
|---|---|---|
| 1 | `transcript_*` events (`seq` order) | Product transcript / CX timeline |
| 2 | `audit_event` allowlisted types | Ops / tool lifecycle / recording lifecycle |
| 3 | `session_disposition.final` | Terminal business outcome |
| 4 | Session recording file via `recording_ref` | Audio evidence; not text SoT |
| 5 | Offline re-STT of WAV | Lab/debug only |

Never reverse (4) or (5) over (1) for “fixing” CX text in V1.

---

## Behaviour inventory

### B1 — Transcript (continuous events)

| Behaviour | Rule |
|---|---|
| Every user STT **final** | Persist `user_final` with `actionable` + `actionable_reason` when false ([04](./04_LIVE_TURN_MACHINE.md)) |
| Transcript-only finals | **Still persisted** — never dropped because they did not move the graph ([02](./02_CURRENT_STATE.md)) |
| Bot spoken text | Persist `bot_utterance` when TTS text is committed to speak (same text the turn machine intends) |
| Graph move | Persist `edge_taken` with `edge_id` / `node_id` when known |
| Tool closing line | Persist `tool_line` (may also be `bot_utterance` — prefer single `tool_line` kind when in ToolArmed closing) |
| Ordering | Monotonic `seq` per session; append-only ([P2.0-P4](./phases/P2.0_schema_principles.md)) |
| Corrections | New event only (e.g. redact note); no UPDATE of prior `text` |

**Actionable reasons (closed starter set):** `echo_suspect`, `tool_locked`, `barge_forbidden`, `too_short`, `empty`, `thinking_busy`, `ending`, `legacy_import`.

### B2 — Audit

| Behaviour | Rule |
|---|---|
| Emit only allowlisted `event_type` | [P2.4](./phases/P2.4_audit_events.md) |
| Tool path | `tool.armed` → (`tool.executing`) → `tool.executed` \| `tool.failed` |
| Recording | `recording.started` / `recording.stopped` with reason |
| No `desk.*` | After P1.12 |
| Payload | No API keys, no PCM bytes |

Audit may omit mirroring every STT final if transcript table is SoT (P2.4 optional `stt.final`).

### B3 — Recording lifecycle

```text
session → live media up
  → start recorder ASAP after answer (or first downlink) 
  → set recording_ref + recording_started_at
  → audit recording.started
  → … call …
  → enter Ending (after tool settle or clean end)
  → STOP recorder (await close)
  → set recording_stopped_at + stop_reason
  → audit recording.stopped
  → session completed/failed
```

| Hard rule | Detail |
|---|---|
| Stop with Ending | Must not depend on client WebSocket staying up ([04](./04_LIVE_TURN_MACHINE.md), [02](./02_CURRENT_STATE.md) leak) |
| Stop before forgetting CallControl | Same ordering discipline as hangup arm |
| Orphan reaper | Sessions terminal with `started_at` set and `stopped_at` null → reaper stops file + stamps `orphan_reaper` |
| No PCM in DB | File/object only |

**Start policy (V1):** start on successful answer / media path ready — not lazily on first bot word only (misses caller speech). Exact hook named at runtime L3 (`E.2`).

### B4 — Disposition timing

| Event | Disposition |
|---|---|
| `tool.executed` transfer | Write `final` from matrix `disposition_code` / P2.6 map; `source=live_tool` |
| `tool.executed` hangup | Write matching `hangup_*` / `hangup_completed` |
| `tool.failed` | Prefer `system_failure` if leg unknown |
| Graph `End` without tool | `out_of_scope` or graph-configured code |
| Caller drop | `abandoned_caller` when detectable |

Disposition write is **after** successful tool settle when tool-driven; still attempt write on Ending even if transcript flush is best-effort.

### B5 — Degrade matrix

| Failure | Who degrades | What still succeeds |
|---|---|---|
| Transcript insert fails | Log + audit `session.failed` detail if terminal; **do not** invent silent drop of actionable path | Live call may continue; ops alert |
| Audit insert fails | Log; call continues | Transcript preferred |
| Recording start fails | Call may continue; `recording_ref` empty; audit note | Transcript/audit still required |
| Recording stop fails | Ending still completes session state; reaper owns leak | Disposition still written |
| Disk full | Fail start or stop per OS; mark `system_failure` if call cannot proceed safely | — |

---

## Anti-patterns

- Turn-pair transcript only (drop transcript-only STT).  
- Stopping recording only in HTTP `stop` handler while edge hangup tears WS first.  
- Using offline WAV STT to overwrite orch transcript.  
- Dual-writing desk.* audit types.  
- “Recording continues for QM after transfer” in V1 without a new OD.  
- Bulk “evidence week” mixing schema DDL + runtime emitter + UI.

---

## Phase breakdown (future runtime L3 — **E** series)

These are **not** P2 schema phases. Execute only after P1 Done and needed P2 expands applied (or columns exist). Each will get a full [07 §4](./07_PLANNING_STANDARDS.md) file under `docs/phases/` when this L2 is Locked.

| id | title | depends_on | goal | exit_criteria (sketch) |
|---|---|---|---|---|
| **E.0** | Evidence inventory vs code | Doc 09 Locked | Map `internal/runtime/record`, observe, transcript writers to B1–B3 | Gap list filed |
| **E.1** | Transcript emitter | E.0; P2.3 DDL | Emit V1 minimum kinds + actionable fields | Integration test: transcript-only final persisted |
| **E.2** | Recording start/stop on Ending | E.0; P2.5 DDL | Hard stop on Ending; WS teardown cannot skip | Lab: short call → file mtime stable; stopped_at set |
| **E.3** | Orphan reaper | E.2 | Reaper stamps `orphan_reaper` | Forced kill test |
| **E.4** | Audit tool + recording events | E.0; P2.4 | Allowlist emitters only | Grep emitters ⊆ allowlist |
| **E.5** | Disposition on tool settle | E.4; P2.6 | `final` codes written | Transfer/hangup cases |
| **E.6** | Evidence soak checklist | E.1–E.5 | Written lab checklist (no UI) | Checklist signed |

Edge cases per E.* expanded when L3 files are written (barge overlap, transfer settle, silence hangup, abuse hangup, answer-fail).

---

## Handoff to L3

When status → **Locked** (owner sign-off):

1. Create `docs/phases/E.0_…md` … `E.6_…md` from the table above (full §4 template).  
2. Do **not** start E.* L4 until Implementer skill + [07 §8](./07_PLANNING_STANDARDS.md) gate.  
3. Schema contradictions → amend P2.* / this doc in planning PR first.

**Next L2 wave:** Implementer (+ Reviewer) skill — last [07 §8](./07_PLANNING_STANDARDS.md) gate item.
