# 12 — Agentic L4 roles (Implementer / Reviewer / Summarizer)

**Status:** **Locked** (owner go-ahead 2026-09-04 — “continue now”). Skills active for L4.  
**Layer:** L2 + skill pointers per [07_PLANNING_STANDARDS.md](./07_PLANNING_STANDARDS.md) §7–§8.  
**Skills (project):**

| Role | Skill path |
|---|---|
| Implementer | [`.cursor/skills/aiorchestrator-l4-implementer/SKILL.md`](../.cursor/skills/aiorchestrator-l4-implementer/SKILL.md) |
| Reviewer | [`.cursor/skills/aiorchestrator-l4-reviewer/SKILL.md`](../.cursor/skills/aiorchestrator-l4-reviewer/SKILL.md) |
| Summarizer | [`.cursor/skills/aiorchestrator-l4-summarizer/SKILL.md`](../.cursor/skills/aiorchestrator-l4-summarizer/SKILL.md) |

---

## Goal

Define how agentic L4 runs against `docs/phases/*` so implementation cannot invent scope, mix phases, or skip the planning gate.

---

## In scope

- Role isolation and handoff order.  
- Work artifacts under `.agent/work/<phase-id>/`.  
- Gate rule before first code change.  
- How to invoke skills in chat.  

## Out of scope

- Replacing Coral global `coral-phase-*` skills for other repos.  
- Auto agent-runner YAML (may come later; L3 markdown is SoT here).  
- Starting P1.1 without owner Lock of 09–12.  

## Open decisions

| ID | Question | Decision |
|---|---|---|
| **OD-12-1** | Plan SoT for L4 | **SETTLED:** `docs/phases/<id>_*.md` (not old `.agent/phases/*.yaml`) |
| **OD-12-2** | Work artifact dir | **SETTLED:** `.agent/work/<phase-id>/` (`implementation.md`, `review.md`, `summary.md`, optional `blockers.md`) |
| **OD-12-3** | First L4 phase after gate | **SETTLED recommendation:** `P1.0` (sign-off only) then `P1.1` — or skip to `P1.1` if P1.0 already owner-signed |

---

## Pipeline

```text
Owner: Lock docs 09–12 + phase series as needed
  → User: “Implementer: phase P1.1”
  → Implementer skill (one id) → implementation.md
  → User: “Reviewer: phase P1.1”
  → Reviewer skill → review.md pass|fail|blocker
  → (fail → Implementer again)
  → User: “Summarizer: phase P1.1” [+ “commit” if desired]
  → summary.md [+ commit]
  → stop (next phase only on new user assign)
```

**Planner** remains human + planning chats amending `docs/` / `docs/phases/` — not these three skills.

---

## Role summary

| Role | May | Must not |
|---|---|---|
| **Implementer** | Code/delete/migrate inside one phase `in_scope` | Re-plan; absorb other phase ids; commit unless also Summarizer+asked |
| **Reviewer** | Score exit criteria; fail/blocker | Edit product code; approve without reading diff |
| **Summarizer** | Write summary; commit **only if asked** | Push; implement; start next phase |

---

## Gate checklist (copy from 07 §8)

Owner **Locked** 2026-09-04 (“continue now”):

- [x] [07](./07_PLANNING_STANDARDS.md) process  
- [x] [08](./08_PURGE_AND_SCHEMA_PHASES.md) ODs + P1/P2 catalog  
- [x] [09](./09_EVIDENCE_AND_RECORDING.md)  
- [x] [10](./10_CODING_PRINCIPLES.md)  
- [x] [11](./11_CI_AND_CD.md)  
- [x] **This doc (12) + three skills**  
- [x] P1 L3 files ack for first wave (P1.1–P1.4 verified wave)  

L4 authorized.

---

## Invocation examples

```text
Implementer: phase P1.1
Reviewer: phase P1.1
Summarizer: phase P1.1 (commit)
```

Verified wave (rare):

```text
Implementer: verified wave P1.5 + P1.6 (separate checklists in implementation.md)
```

---

## Anti-patterns

- Using deleted `.agent/phases/*.yaml` as current plan.  
- “Just delete everything desk-related in one PR.”  
- Reviewer fixing code.  
- Summarizer committing without Reviewer pass.  
- Starting graph runtime during a purge phase.  

---

## Handoff

**Planning catalog for purge/schema/evidence/coding/CI/roles is complete pending your Lock.**  

Next step is **owner sign-off**, not more L2 docs. After Lock, first L4 assign: Implementer on `P1.0` or `P1.1`.
