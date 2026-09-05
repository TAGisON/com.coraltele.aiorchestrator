# L3 — E.6 Evidence soak checklist

| Field | Value |
|---|---|
| **id** | `E.6` |
| **title** | Lab evidence soak checklist (no UI) |
| **status** | **Closed** — checklist authored |
| **parent_plan** | [09](../09_EVIDENCE_AND_RECORDING.md) E.6 |
| **depends_on** | E.1–E.5 Closed (`e6dc674`…`193c9dd`) |

## architecture_refs

- [09_EVIDENCE_AND_RECORDING.md](../09_EVIDENCE_AND_RECORDING.md) B1–B5, truth hierarchy, anti-patterns
- [E.1](./E.1_transcript_emitter.md) … [E.5](./E.5_disposition_tool_settle.md)
- [P2.3](./P2.3_transcript_events.md) / [P2.4](./P2.4_audit_events.md) / [P2.5](./P2.5_recording_metadata.md) / [P2.6](./P2.6_disposition.md)
- Lab UI: typically http://127.0.0.1:8011/ ([AGENTS.md](../../AGENTS.md))

## goal

Provide a single **written lab checklist** (no new UI) that an operator can walk after E.1–E.5 so transcript, audit, recording, and disposition evidence match docs/09 — with an owner sign-off block.

## in_scope

- This checklist document (procedures + pass criteria + known gaps + sign-off)
- Automated preflight commands (unit/integration already shipped; no new product code)
- `docs/phases/README.md` E.6 row

## out_scope

- New control UI / evidence dashboard
- Graph `edge_taken` / `tool_line` lab cases (deferred until graph runtime)
- Dedicated hangup-tool cases for `hangup_silence` / `hangup_abuse` (E.5 out_scope)
- CI.2 secrets hygiene / CD.*
- Fixing mono-mix vs stereo recorder (OD-09-1 vs current stereo file — note only)

## forbidden

- Shipping a UI “while here”
- Inventing new audit/transcript kinds in this phase
- Claiming full B1 compliance for `edge_taken` / `tool_line` without graph runtime

## exit_criteria

- [x] Checklist covers B1–B5 with observable pass/fail per scenario
- [x] Sign-off block present (owner fills when lab is run)
- [x] Known deferred gaps listed (honest, not hidden)
- [x] No product code required for phase pass

## verification

```text
# Preflight (CI-equivalent, no live Sarvam hang):
go test ./internal/store/... ./internal/runtime/observe/... ./internal/control/... ./internal/validation/... -count=1 -timeout 180s
```

## handoff

Evidence **E.*** wave complete for V1 runtime emitters.  
**Graph soak:** [L.0_graph_lab_soak.md](./L.0_graph_lab_soak.md) (after G.7).  
**Promote:** [CD.0](./CD.0_lab_promote.md).

---

# Lab soak checklist

**How to use:** run **Preflight**, then each **Scenario** in order (or mark N/A with reason). Tick **Pass** only when pass criteria are observed. Owner signs the block at the end after a real lab pass (or records deferred).

**Environment defaults:** orchestrator control http://127.0.0.1:8011/; Postgres (or Memory lab); recording enabled per profile/runtime config; fake or Sarvam gateways as available.

## 0 — Preflight (automated)

| # | Check | Command / action | Pass criteria | ☐ |
|---|---|---|---|---|
| 0.1 | Scoped tests green | `go test ./internal/store/... ./internal/runtime/observe/... ./internal/control/... ./internal/validation/... -count=1 -timeout 180s` | All packages `ok` | ☐ |
| 0.2 | No desk audit emitters | Search tree for `desk.` audit writes under `internal/` | No production emit of `desk.*` | ☐ |
| 0.3 | Tip SHAs | README E.1–E.5 Closed rows | Cite tip SHAs if filing a lab note | ☐ |

## 1 — B1 Transcript

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 1.1 | Happy turn | Live/lab session → inject or speak a clear user final → bot reply → stop | `GET …/transcript` has `user_final` and `bot_utterance` with monotonic `seq`; actionable true on path that moved the turn | ☐ |
| 1.2 | Transcript-only / suppress | Force short/echo-suspect or barge-forbidden path | Suppressed final still present as `user_final` with `actionable=false` and closed `actionable_reason` (e.g. `too_short`, `echo_suspect`, `barge_forbidden`) — **not** dropped | ☐ |
| 1.3 | No offline overwrite | If WAV re-STT available in lab | Product transcript unchanged by offline STT (truth hierarchy: orch stream wins) | ☐ |
| 1.4 | Graph evidence | See **[L.0](./L.0_graph_lab_soak.md)** | `edge_taken` / `tool_line` required on **graph** soak — not on profile-only E.6 path | ☐ / see L.0 |

## 2 — B2 Audit

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 2.1 | Session lifecycle | Create session → stop cleanly | Audit includes `session.created`, `session.live`, `session.ending` (on HTTP stop), and terminal `session.completed` \| `cancelled` \| `failed` — **not** legacy `session.started` / `session.terminal` as new emits | ☐ |
| 2.2 | Turn state | Complete one talk turn | `turn.state` present (legacy `turn.completed` not required) | ☐ |
| 2.3 | Transfer tool chain | Arm transfer → settle | `tool.armed` → `tool.executing` → `tool.executed` (or `tool.failed`); payload has tool identity, no secrets/PCM | ☐ |
| 2.4 | Recording audits | Call with recording on | `recording.started` then `recording.stopped` with stop reason | ☐ |

## 3 — B3 Recording

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 3.1 | Start/stop stamps | Short recorded call → stop | Session has `recording_ref`, `recording_started_at`, `recording_stopped_at`, stop reason; file mtime stable after stop | ☐ |
| 3.2 | Ending without WS | Tear edge/WS before HTTP stop if lab can | Recording still stops / stamps (not reliant on client WS alone) | ☐ |
| 3.3 | Orphan reaper | Force terminal session with started_at set and stopped_at null (lab inject or kill mid-write) | Reaper stamps `orphan_reaper` + `recording.stopped` | ☐ |
| 3.4 | Layout note | Inspect file | V1 OD-09-1 target = mono mix; **current lab may still be stereo** — record observation; do not block soak on stereo alone | ☐ |

## 4 — B4 Disposition

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 4.1 | Transfer settle | Successful transfer | `session_disposition.final` is allowlisted `transferred_*`; `source=live_tool` | ☐ |
| 4.2 | Transfer / FailCall fail | Force transfer error or FailCall | `final=system_failure`, `source=live_tool` | ☐ |
| 4.3 | Clean stop, no tool | Create → talk optional → HTTP stop Completed | `final=out_of_scope`, `source=live_graph` if no prior final | ☐ |
| 4.4 | Caller drop | Edge gone / Cancelled path | `final=abandoned_caller` when no prior final | ☐ |
| 4.5 | Ops patch | `PATCH …/disposition` with allowlisted code | `source=ops_patch`; rejects non-allowlist | ☐ |
| 4.6 | Deferred hangup_* | Silence/abuse hangup tools | **N/A** until hangup tool path exists (FailCall remains `system_failure`) | — |

## 5 — B5 Degrade (spot-check)

| # | Scenario | Steps | Pass criteria | ☐ |
|---|---|---|---|---|
| 5.1 | Audit fail-open | (Optional) inject store fault if lab harness allows | Call continues; warn logged | ☐ |
| 5.2 | Recording start fail | Disable record path / disk | Call may continue; transcript/audit still written; `recording_ref` empty OK | ☐ |
| 5.3 | Disposition after stop fail | Recording stop error / orphan path | Disposition still present after Ending | ☐ |

## 6 — Anti-patterns (must not observe)

| # | Anti-pattern | ☐ Not seen |
|---|---|---|
| 6.1 | Dropping transcript-only STT finals | ☐ |
| 6.2 | Stopping recording only when HTTP stop runs while edge hangup already tore media (leak) | ☐ |
| 6.3 | Using offline WAV STT to overwrite orch transcript | ☐ |
| 6.4 | Emitting `desk.*` audit types | ☐ |
| 6.5 | Free-text or legacy `resolved`/`unresolved`/`escalated` as new `final` | ☐ |

## Known deferred gaps (do not treat as soak failures)

1. `edge_taken` / `tool_line` transcript kinds — need graph/tools runtime.  
2. `hangup_completed` / `hangup_silence` / `hangup_abuse` — need dedicated hangup tool (not FailCall).  
3. Stereo on-disk vs OD-09-1 mono mix — separate recording slice.  
4. Full matrix ARM freeze of `disposition_code` — coral-transfer accepts the arg; flow matrix runtime separate.

## Sign-off

| Field | Value |
|---|---|
| Lab date | |
| Operator | |
| Build / tip SHA | |
| Preflight 0.1 result | pass / fail |
| Scenarios passed | (list ids) |
| Scenarios N/A | (list ids + reason) |
| Failures / notes | |
| Owner signature | |
| Result | **soak pass** / **soak fail** / **deferred** |

_Phase E.6 closes when this checklist document exists in-repo. Lab **soak pass** requires a completed sign-off row above after a real run._
