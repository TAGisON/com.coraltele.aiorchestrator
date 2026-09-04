# 10 — Coding principles and edge-case library

**Status:** **Locked** (owner go-ahead 2026-09-04 — “continue now”).  
**Layer:** L2 domain plan per [07_PLANNING_STANDARDS.md](./07_PLANNING_STANDARDS.md) §3.  
**Parent / architecture refs:** [01](./01_VISION_AND_SCOPE.md)–[06](./06_APPLICATION_FLOW.md), [07](./07_PLANNING_STANDARDS.md), [08](./08_PURGE_AND_SCHEMA_PHASES.md), [09](./09_EVIDENCE_AND_RECORDING.md), [phases/](./phases/README.md).

---

## Goal

Give every future Implementer a **closed set of coding rules and named edge cases** so L4 work stays concentrated, cites locks, and does not reintroduce desk hybrid behaviour or silent invention.

---

## In scope

- Language / package / dependency rules.  
- Layering (edge, ports, gateways, graph, turn machine, control, store).  
- Concurrency and tool-lock coding rules.  
- Error / degrade coding rules (align [09](./09_EVIDENCE_AND_RECORDING.md) B5).  
- Secrets and logging.  
- Test expectations tied to phase exit criteria.  
- **Edge-case library** (IDs `EC-*`) that L3 phases must cite when relevant.  
- Future L3 ids **C.*** only if a principle needs a dedicated cleanup phase (optional).  

## Out of scope

- Style bikeshedding (gofmt/`go test` are enough unless repo adds golangci later in doc 11).  
- Implementing graph runtime (separate L3 after P1/P2).  
- Full JSON Schema for `coral.flow.v1` (still deferred; envelope in [P2.7](./phases/P2.7_flow_publish_model.md)).  
- CI wiring details ([11](./07_PLANNING_STANDARDS.md) §8).  

## Open decisions

| ID | Question | Decision |
|---|---|---|
| **OD-10-1** | Allow Python/Java in-repo for media kernel? | **SETTLED: No** — Go kernel only ([locks](../.cursor/rules/aiorchestrator-locks.mdc)). Lab scripts may be PowerShell/Python **outside** the live media path. |
| **OD-10-2** | Kafka/Redis for live PCM? | **SETTLED: No**. |
| **OD-10-3** | Default PR size | **SETTLED:** One L3 phase id per Implementer run / preferred one concern per PR ([07](./07_PLANNING_STANDARDS.md)). |

---

## Coding principles

### C10-P1 — Cite before you cut

- Every L4 change set must list `architecture_refs` from its L3 file.  
- If code needs behaviour not in 01–09 / phase L3: **stop** and amend planning docs first.  

### C10-P2 — One phase id

- Do not combine P1 purge + P2 migrate, or E.2 recording + graph walker, in one run.  
- If compile requires a wave (e.g. P1.5+P1.6): still **two checklists**, never one fuzzy “desk removal” commit message without both exits.  

### C10-P3 — Kernel and media path

- Orchestrator live path: **Go only**.  
- Live PCM: in-memory session path only; no broker.  
- FreeSWITCH `mod_audio_stream`: dumb PCM pipe — no product barge/FAQ in the module.  

### C10-P4 — Layer ownership

| Layer | May | Must not |
|---|---|---|
| `internal/edge/**` | PCM + transfer/hangup verbs | Graph decisions, dial invent |
| `internal/port/**` | Contracts | Vendor SDK imports |
| `internal/gateway/**` | Vendor adapters | Own CX / tool arm policy |
| Graph + turn machine (future packages) | Cursor, edges, repair, tool arm | Raw Sarvam protocol |
| `internal/control/**` | HTTP/session wiring | Reintroduce `internal/desk` |
| `internal/store/**` | Persistence | PCM BYTEA |

Vendors stay behind ports ([05](./05_MEDIA_AND_VENDORS.md)).

### C10-P5 — Graph is law

- LLM selects only allowlisted edges from current node ([03](./03_BRAIN_AND_GRAPH.md)).  
- No “helpful” FAQ/transfer outside edges.  
- Dial numbers only from published **matrix** at Tool ARM ([P2.9](./phases/P2.9_routing_matrix.md)).  

### C10-P6 — Tool lock coding

```text
ARM (freeze params) → barge off → Speak closing → EXECUTE once → settle → Ending
```

- Do not tear down edge WebSocket before hangup/transfer has a chance to settle ([04](./04_LIVE_TURN_MACHINE.md)).  
- Same machine for transfer and hangup.  
- Never execute tool twice for one arm.  

### C10-P7 — Turn machine coding

- Single-flight `Thinking` per session.  
- Respect actionable vs transcript-only ([04](./04_LIVE_TURN_MACHINE.md), [09](./09_EVIDENCE_AND_RECORDING.md)).  
- After Tool ARM: barge forbidden until settle/end.  

### C10-P8 — Evidence coding

- Append-only transcript/audit; no silent drop of transcript-only STT ([09](./09_EVIDENCE_AND_RECORDING.md)).  
- Recording **must** stop on Ending; WS death must not skip stop.  
- Emit only allowlisted audit types ([P2.4](./phases/P2.4_audit_events.md)).  
- Orch transcript is SoT over offline WAV STT (OD-09-3).  

### C10-P9 — Data and migrations

- Follow [P2.0](./phases/P2.0_schema_principles.md) and [P2.14](./phases/P2.14_migration_ci.md).  
- SoT migrations: `internal/store/migrations/`.  
- One concern per migration file; never recreate `desk_*`.  
- No secrets in SQL or git; credentials in `gateway_credentials` only.  

### C10-P10 — Purge discipline

- After P1: do not restore `internal/desk`, three consoles, or deskskills “for lab convenience.”  
- New dialogue = new graph runtime phases citing [03](./03_BRAIN_AND_GRAPH.md)–[04](./04_LIVE_TURN_MACHINE.md).  

### C10-P11 — Bindings

- Inform uses [P2.10](./phases/P2.10_bindings_redesign.md) `binding` — never wire `kb_*` (OD-08-4).  
- Missing binding → fail closed / repair edge — do not invent FAQ text.  

### C10-P12 — Logging and PII

- Never log API keys, auth tokens, or raw PCM.  
- Mask `confidential` session attributes in APIs/logs ([P2.12](./phases/P2.12_slots_attributes.md)).  
- Prefer structured logs with `session_id`.  

### C10-P13 — Tests

- Phase exit criteria are the test contract.  
- Do not keep `t.Skip` stubs that still import deleted desk types.  
- Prefer focused tests for EC-* cases listed in the phase `edge_cases`.  

### C10-P14 — Dependencies

- Prefer stdlib + existing module deps.  
- New dependency requires note in phase summary (why + license OK).  
- No Python media service spun from orch.  

### C10-P15 — Commits (when Summarizer allowed)

- Human git identity only; no AI co-author trailers (workspace rule).  
- Message states phase id + why.  
- No commit of `.agent/secrets.local.json`.  

---

## Edge-case library

Cite as `EC-NN` in L3 `edge_cases` / tests.

### Media / edge

| ID | Scenario | Expected |
|---|---|---|
| **EC-01** | Caller hangs up during bot Speak | Ending; recording stop; disposition `abandoned_caller` or best-effort |
| **EC-02** | Transfer settle OK but orch WS already closing | Hangup/transfer verb still attempted before teardown; recording stop still runs |
| **EC-03** | Hangup armed then late “desk.end”-style second hangup | Second execute forbidden (single arm) |
| **EC-04** | Answer succeeds, recording start fails | Call may continue; empty `recording_ref`; no crash loop |
| **EC-05** | Recording stop fails on Ending | Session still terminals; reaper path ([09](./09_EVIDENCE_AND_RECORDING.md) E.3) |

### Turn / barge / STT

| ID | Scenario | Expected |
|---|---|---|
| **EC-10** | STT final while Speaking + barge forbidden | Transcript-only; no graph move |
| **EC-11** | STT fragment after language switch (echo/FAQ phantom) | Transcript-only or repair; **no** false Inform/transfer ([02](./02_CURRENT_STATE.md)) |
| **EC-12** | Barge commit with real utterance | Actionable after commit rules; prior bot audio policy per barge config |
| **EC-13** | Second utterance during Thinking | Not actionable (`thinking_busy`); may transcript-only |
| **EC-14** | STT final during ToolArmed / ToolExecuting / Ending | Transcript-only; no edge |

### Graph / tools / language

| ID | Scenario | Expected |
|---|---|---|
| **EC-20** | LLM proposes edge not on allowlist | Reject; repair / unclear — no jump |
| **EC-21** | LLM returns a phone number for transfer | Ignore; matrix number only |
| **EC-22** | Tool transfer with missing matrix row | Fail closed at publish or ARM; no dial |
| **EC-23** | ListenLanguage success then Tool same turn | **Forbidden** ([03](./03_BRAIN_AND_GRAPH.md)) |
| **EC-24** | Mid-call “talk in English” | Only via ListenLanguage / global `language_switch` edge |
| **EC-25** | Silence exhaust | Graph repair → configured hangup Tool — not ad-hoc |
| **EC-26** | Inform with disabled/missing binding | Fail closed / repair; no invented FAQ |
| **EC-27** | Weak greeting (“Hello”) vs ANI language lock | Must not wrongly pin language ([02](./02_CURRENT_STATE.md)) |

### Evidence / disposition

| ID | Scenario | Expected |
|---|---|---|
| **EC-30** | Transcript-only user final | Row persisted with `actionable=false` + reason |
| **EC-31** | Offline WAV STT order ≠ orch transcript | Orch wins for product SoT (OD-09-3) |
| **EC-32** | Transfer executed | Disposition `transferred_*`; recording stopped at orch Ending |
| **EC-33** | Audit emit unknown type | Forbidden in new code |
| **EC-34** | Process crash mid-call with recorder open | Reaper sets `orphan_reaper` |

### Purge / schema (implementer hygiene)

| ID | Scenario | Expected |
|---|---|---|
| **EC-40** | Tempted to keep `/admin` for lab | Forbidden (OD-08-1) |
| **EC-41** | Tempted to archive `.agent/phases` | Forbidden (OD-08-2) — delete |
| **EC-42** | Tempted to keep `kb_*` for Inform | Forbidden (OD-08-4) |
| **EC-43** | Mega-migration flow+drop+transcript | Forbidden ([P2.14](./phases/P2.14_migration_ci.md)) |

---

## Anti-patterns (summary)

- Repairing or wrapping `Engine.Turn` / `internal/desk`.  
- Product logic in `mod_audio_stream`.  
- Kafka/Redis PCM.  
- Dual brain (profile `x_desk` + flow) indefinitely.  
- Open disposition/audit strings from LLM.  
- Bulk unrelated refactors inside a phase PR.  

---

## Phase breakdown

No mandatory **C.*** L3 series. Principles are **constraints** on all L4 phases (P1/P2/E/G future).

Optional later: `C.0` lint/CI wiring when doc 11 exists.

---

## Handoff

When status → **Locked** (owner sign-off):

1. Implementer skill **must** require reading this file + citing EC-* in phase tests where listed.  
2. Reviewer checks forbidden list + EC coverage claimed in exit criteria.  
3. Next: owner Lock of 09–12; then L4 via Implementer skill (`docs/12`).

**Not started:** code, doc 11, skill files.
