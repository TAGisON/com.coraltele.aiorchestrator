# 07 — Planning standards (agentic methodology)

**Status:** Active planning rules for branch `docs/llm-callcentre-architecture`.  
**Implementation:** Forbidden until the full planning set is complete and an **Implementer skill** is defined and approved.  
**Purpose:** Every later plan and every future agentic phase must be unambiguous — no silent invention, no repairing deleted code, no vague “make it work.”

---

## 1. Relationship to locked architecture

| Doc | Lock role |
|---|---|
| [01_VISION_AND_SCOPE.md](./01_VISION_AND_SCOPE.md) | V1 product ticks / out of scope |
| [02_CURRENT_STATE.md](./02_CURRENT_STATE.md) | Keep vs discard |
| [03_BRAIN_AND_GRAPH.md](./03_BRAIN_AND_GRAPH.md) | Graph brain |
| [04_LIVE_TURN_MACHINE.md](./04_LIVE_TURN_MACHINE.md) | Live turn machine |
| [05_MEDIA_AND_VENDORS.md](./05_MEDIA_AND_VENDORS.md) | STT / LLM / TTS |
| [06_APPLICATION_FLOW.md](./06_APPLICATION_FLOW.md) | Whole-application flow |
| **This file (07)** | How we write plans and phases |
| [08_PURGE_AND_SCHEMA_PHASES.md](./08_PURGE_AND_SCHEMA_PHASES.md) | P1 purge + P2 schema phase catalog |
| [09_EVIDENCE_AND_RECORDING.md](./09_EVIDENCE_AND_RECORDING.md) | Transcript / audit / recording behaviour |
| [10_CODING_PRINCIPLES.md](./10_CODING_PRINCIPLES.md) | Coding principles + edge-case library |
| [11_CI_AND_CD.md](./11_CI_AND_CD.md) | CI / CD |
| [12_AGENTIC_L4_ROLES.md](./12_AGENTIC_L4_ROLES.md) | L4 Implementer / Reviewer / Summarizer |

If a plan contradicts 01–06, **change the plan** or **explicitly amend 01–06** in a planning PR — never contradict quietly in implementation.

---

## 2. Layers (do not skip)

| Layer | Name | Allowed output | Forbidden |
|---|---|---|---|
| L0 | Architecture (done) | Docs 01–06 | Code |
| L1 | Planning standards | Doc 07 | Code |
| L2 | Domain plans | Doc per topic (e.g. 08, later 09…) | Code |
| L3 | Executable phases | Small phase specs with exit criteria | Code until Implementer skill + go-ahead |
| L4 | Implementation | Code via agentic Implementer | Planning invention |

**Rule:** No L4 without locked L3. No L3 without locked L2 for that area.

---

## 3. Mandatory template for every L2 domain plan

Every domain plan document must contain:

1. **Title + status** — Draft | Review | Locked  
2. **Parent references** — exact links to 01–06 (and sibling L2 docs) that this plan depends on  
3. **Goal** — one sentence  
4. **In scope / Out of scope**  
5. **Open decisions** — numbered; must be resolved before status = Locked  
6. **Detailed inventory or entity list** — paths, tables, APIs, or behaviours named explicitly  
7. **Phase breakdown** — each phase id, goal, in/out, depends-on, exit criteria, edge cases  
8. **Anti-patterns** — what implementers must not do (e.g. “do not repair `internal/desk`”)  
9. **Handoff to L3** — when this L2 is Locked, how phases become agent prompts  

---

## 4. Mandatory template for every L3 phase

Each phase (for humans or agentic AI) must include:

| Field | Requirement |
|---|---|
| `id` | Stable id e.g. `P1.5`, `P2.3` |
| `title` | Short name |
| `parent_plan` | Link to L2 doc section |
| `architecture_refs` | Bullet links to specific 01–06 sections |
| `goal` | One sentence |
| `in_scope` | Bullets — file paths / tables / behaviours |
| `out_scope` | Bullets — explicitly not this phase |
| `depends_on` | Prior phase ids that must be Done |
| `forbidden` | e.g. “do not reintroduce desk engine” |
| `exit_criteria` | Observable checks (compile, tests, file absent, migration applied) |
| `edge_cases` | 3–7 real scenarios this phase must not break *or* N/A if pure delete |
| `verification` | Exact commands / checks |
| `rollback` | How to undo if needed (esp. purge/migrate) |

**Size cap:** One phase ≈ half-day to two days for one Implementer agent. If it needs “and also…”, split.

---

## 5. Reference discipline (no confusion later)

- Prefer **absolute doc anchors**: `docs/03_BRAIN_AND_GRAPH.md` § Tool semantics.  
- Prefer **repo paths**: `web/admin/`, `internal/desk/`.  
- Prefer **table names** once P2 names them: e.g. `flow_graph_version` (illustrative until P2 locks names).  
- Never say “the old system” without naming the path.  
- Never say “update the UI” without saying which console (and after P1, old consoles are gone).  

---

## 6. Purge-first rule (P1)

From product intent ([02_CURRENT_STATE.md](./02_CURRENT_STATE.md)):

> Keep the media pipe. Rebuild the dialogue brain.

Therefore:

- **P1 deletes** wrong product surfaces and the old desk brain so later agents **cannot** “fix” them.  
- After a purge phase is Done, **reintroduction** of that area is only allowed via a **new** L3 phase that cites 03–06 — never by restoring deleted packages ad hoc.  

---

## 7. Agentic methodology (planning now; skills later)

| Role | When defined | Job |
|---|---|---|
| **Planner** | With L2/L3 docs | Writes/updates plans; does not code |
| **Implementer** | **After** planning complete | Executes **one** locked L3 phase only; skill file will define isolation |
| **Reviewer** | With Implementer skill | Checks exit criteria + architecture_refs |
| **Summarizer** | With Implementer skill | Evidence vs exit criteria; commit only if allowed |

**Implementer / Reviewer / Summarizer skills:** defined under `.cursor/skills/aiorchestrator-l4-*/` and [12](./12_AGENTIC_L4_ROLES.md).  
**Do not start agentic coding until owner Locks docs 09–12** (and acks first P1 wave).

---

## 8. Definition of “planning complete” (gate before any L4)

All of the following are **Locked** (owner go-ahead 2026-09-04):

- [x] Doc 07 (this file) — process  
- [x] Doc 08 P1 + P2 catalog + ODs settled  
- [x] L3 phase files P1.* + P2.* under `docs/phases/`  
- [x] [09](./09_EVIDENCE_AND_RECORDING.md)  
- [x] [10](./10_CODING_PRINCIPLES.md)  
- [x] [11](./11_CI_AND_CD.md)  
- [x] [12](./12_AGENTIC_L4_ROLES.md) + three L4 skills  

**L4 authorized.** First wave: P1.0 ack + verified wave P1.1–P1.4 (UI/shell).

---

## 9. Open decisions for doc 07

None for the methodology itself — **Locked as process**.  
Product decisions for purge/schema: **SETTLED** in [08](./08_PURGE_AND_SCHEMA_PHASES.md) (OD-08-1…5).
