# G.0 — Graph runtime inventory vs locks

**Date:** 2026-09-05  
**Phase:** G.0  
**Architecture SoT:** [docs/03_BRAIN_AND_GRAPH.md](../../docs/03_BRAIN_AND_GRAPH.md), [docs/04_LIVE_TURN_MACHINE.md](../../docs/04_LIVE_TURN_MACHINE.md), [docs/01_VISION_AND_SCOPE.md](../../docs/01_VISION_AND_SCOPE.md)  
**Schema locks:** P2.7–P2.10; DDL `010`/`011`/`012` (M-A/B/C)

Method: `rg` + read of `internal/store`, `internal/control`, `internal/runtime/{composer,thinkpath,session}`, migrations, phase docs.

---

## What exists today (keep)

| Area | Symbol / path | Notes |
|---|---|---|
| DDL `flow` / `flow_draft` / `flow_version` | `010_flow_registry.sql` | Tables present; migrate tests assert CREATE |
| DDL `binding` | `011_binding.sql` | Table present |
| DDL session pins | `012_session_flow_pin.sql` | Columns `flow_id`, `flow_version` on `session` |
| Live turn shell | `composer.Talk` + states | Listen/Think/Speak; barge; not graph-cursor |
| Think ladder | `thinkpath.Path.Run` | Profile playbook / clip / LLM / escalate skill — **not** node cursor |
| TransferIntent stub | `thinkpath.TransferIntent` | Comment: future graph/tools |
| Edge transfer/hangup verbs | `SessionRuntime.Transfer` / `FailCall` | Tool audit + disposition (E.4/E.5); FailCall ≠ hangup Tool |
| Coral-transfer | `gateway/coraltransfer` | Accepts `disposition_code`; dial from args/default |
| Evidence kinds ready | `EventKindEdgeTaken`, `AuditGraphEdge` | Constants exist; **no emitters** from graph |
| Media / STT / TTS / session control | control + edge + gateways | Pipe ready for brain |

---

## What is missing (ranked gaps)

### Config / store (blocks any live graph)

| ID | Gap | Current | Target | Owner phase |
|---|---|---|---|---|
| **G-CFG-1** | No Go models/repo for `flow*` | DDL only | CRUD draft + immutable publish + get version | **G.1** |
| **G-CFG-2** | No Go models/repo for `binding` | DDL only | CRUD/list knowledge bindings | **G.1** (minimal) / **G.6** Inform |
| **G-CFG-3** | Session Go ignores pin columns | `Session` has no `FlowID`/`FlowVersion`; INSERT/SELECT omit them | Wire fields Memory+PG | **G.1** |
| **G-CFG-4** | No control HTTP for flows | No `/v1/flows*` | Create/update draft, publish, get | **G.2** |
| **G-CFG-5** | No `coral.flow.v1` validator | — | Envelope + node/edge/matrix publish checks (P2.7–P2.9) | **G.2** |

### Runtime brain (blocks V1 call flow)

| ID | Gap | Current | Target | Owner phase |
|---|---|---|---|---|
| **G-RT-1** | No graph document loader on session start | Profile-only Talk | Load pinned `flow_version.doc` | **G.3** |
| **G-RT-2** | No cursor / node machine | `thinkpath` ladder | Cursor on one node; legal edges only ([03](../../docs/03_BRAIN_AND_GRAPH.md)) | **G.3** |
| **G-RT-3** | Entry → Speak welcome | Answer/welcome ad-hoc | Entry then Speak(prompt_ref) via P2.8 resolve | **G.3** |
| **G-RT-4** | ListenChoice → intent/option edges | LLM freeform / escalate skill | Allowlisted edges only; repair on no match | **G.3** + **G.5** |
| **G-RT-5** | Tool ARM transfer/hangup | Transfer API + FailCall | Matrix freeze → arm → closing Speak → exec once ([03]/[04](../../docs/04_LIVE_TURN_MACHINE.md)) | **G.4** |
| **G-RT-6** | Hangup Tool vs FailCall | FailCall → `system_failure` | Tool hangup → `hangup_*` finals (E.5 deferred) | **G.4** |
| **G-RT-7** | Per-node repair | Partial silence/barge only | `on_unclear` / retries / `on_exhausted` edges | **G.5** |
| **G-RT-8** | ListenLanguage / prompts locale | Preference + STT lock | Node + prompt_ref locale fail-closed (P2.8) | **G.5** |
| **G-RT-9** | Inform + knowledge binding | thinkpath Knowledge port optional | Inform node + `binding_ref` (P2.10); omit OK if no binding | **G.6** |
| **G-RT-10** | Session create without flow pin | Always profile pin | Refuse **new live** sessions without published flow (P2.7 transition end) | **G.3** or **G.7** cutover |

### Evidence (after walker)

| ID | Gap | Current | Target | Owner phase |
|---|---|---|---|---|
| **G-EV-1** | No `edge_taken` | Kind constant only | Emit on graph move | **G.7** |
| **G-EV-2** | No `tool_line` | — | Closing line kind | **G.7** |
| **G-EV-3** | No `graph.edge` audit | Allowlist const | Optional audit on take | **G.7** |

---

## Explicit non-gaps (do not rebuild)

- PCM pipe / mod_audio_stream dialect  
- Sarvam gateways behind ports  
- E.1–E.5 evidence writers (extend, don’t replace)  
- Desk / `x_desk` (purged)  
- Full JSON Schema file for flow doc (deferred per [03](../../docs/03_BRAIN_AND_GRAPH.md) / P2.7)  
- Admin SPA authoring UI (API-first V1 OK)  
- Automated desk→flow migrator (P2.7 Locked: none)

---

## Suggested G.* order

1. **G.1** — Store: flow/binding models + session pin fields (Memory + PG)  
2. **G.2** — Control: flow draft/publish API + envelope validator  
3. **G.3** — Runtime core: pin on create, load doc, Entry/Speak/ListenChoice/End (+ Decide)  
4. **G.4** — Tools: transfer/hangup arm→speak→exec + matrix `disposition_code`  
5. **G.5** — Repair + ListenLanguage + prompt locale resolve  
6. **G.6** — Inform + binding resolve (thin FAQ; skippable if no binding in lab graph)  
7. **G.7** — Evidence emitters + live cutover (refuse sessions without flow pin) + coral-xfer lab fixture  

Optional interrupt: author one **lab fixture** `coral.flow.v1` JSON under `tools/lab/` during G.2/G.3.

---

## V1 “done” mapping (docs/01)

| V1 tick | Satisfied after |
|---|---|
| Conversational IVR / graph | G.3+ |
| Intent → transfer | G.3 + G.4 |
| Hangup tool | G.4 |
| Multilingual talk | G.5 (+ existing STT/TTS) |
| Thin FAQ | G.6 |
| Transcript + recording | Already E.*; complete with G.7 kinds |
| Configuration | G.2 API (UI Later) |
