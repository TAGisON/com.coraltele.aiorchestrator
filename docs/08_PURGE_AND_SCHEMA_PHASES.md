# 08 — P1 purge phases and P2 schema phases

**Status:** Open decisions **SETTLED** (2026-09-04). Catalog **Locked for product choices**; remaining work is L3 phase-file expansion per [07](./07_PLANNING_STANDARDS.md) — still **planning only** (no deletion or migrations until L4 go-ahead).  
**Parent:** [07_PLANNING_STANDARDS.md](./07_PLANNING_STANDARDS.md)  
**Architecture refs:** [01](./01_VISION_AND_SCOPE.md), [02](./02_CURRENT_STATE.md), [03](./03_BRAIN_AND_GRAPH.md), [04](./04_LIVE_TURN_MACHINE.md), [05](./05_MEDIA_AND_VENDORS.md), [06](./06_APPLICATION_FLOW.md)

---

## Goal

1. **P1** — Remove product/code leftovers that belong to the old hybrid desk + three-console world, so implementation cannot “repair” them.  
2. **P2** — Define data model in small phases with full detail and references, replacing desk-centric storage with `flow_*` graph/config/evidence storage aligned to 03–04.

---

## Settled decisions (OD-08)

| ID | Decision | Implication |
|---|---|---|
| **OD-08-1** | **Delete all three** UIs: admin, user, supervisor | P1.1–P1.3 all delete; no stub supervisor. Rebuild later under a new plan citing 01. |
| **OD-08-2** | **Delete** obsolete `.agent/phases` (and obsolete pipelines that only serve the old desk world) | P1.9 is hard delete — no archive folder that agents might still load. |
| **OD-08-3** | **New tables** named `flow_*` (and related new names) | Never name replacements `desk_*`. Final names locked per P2.7+ field tables. |
| **OD-08-4** | **Redesign knowledge later** — do **not** keep/rebind `kb_document` / `kb_chunk` as the V1 Inform store | P2.10 redesigns bindings from [03](./03_BRAIN_AND_GRAPH.md) (bindings ≠ nodes) + this programme’s planning chain. Old KB tables: **no new features** after P1; **DROP** only in P2.13 after the replacement entity is Locked. |
| **OD-08-5** | **Strict serial concentration** (see § Motto + Part D) | No bulk; no parallel L4; no mixing P1 and P2 deep work in one wave. |

---

## Motto — concentration, detail, no bulk (programme rule)

Taken from owner direction for this plan:

- Nothing lightly. Nothing in bulk.  
- One context at a time so concentration stays set.  
- Each phase fully detailed before the next opens.  
- No mixups: purge context ≠ schema context ≠ runtime-brain context.

**Operational rules derived from OD-08-5:**

| Rule | Meaning |
|---|---|
| Serial planning depth | Deep-detail / L3 authoring for **P1 completes and locks** before deep-detail for **P2** starts. Doc 08 may *list* both catalogs; it must not be treated as permission to flesh both at once. |
| Serial L4 later | Entire **P1.0–P1.12 Done** before any **P2 DDL / migrate**. No purge+migrate in one agent run. |
| One phase id | Agentic Implementer (future) runs **one** phase id per run — [07 §7](./07_PLANNING_STANDARDS.md). |
| No mega-steps | Forbidden: “delete all UIs + desk + drop tables” as one phase. |
| Paper ≠ execute | Writing field tables is planning; applying migrations is L4 after gate. |

---

## Part A — Keep inventory (P1.0 input)

### A.1 Keep (do not purge in P1)

| Path / area | Why (refs) |
|---|---|
| `internal/edge/**` | Dumb PCM + transfer/hangup verbs — [02](./02_CURRENT_STATE.md), [05](./05_MEDIA_AND_VENDORS.md), [06](./06_APPLICATION_FLOW.md) |
| `internal/gateway/sarvam*/**`, `internal/gateway/sarvam/**` | Vendor gateways — [05](./05_MEDIA_AND_VENDORS.md) |
| `internal/port/**` | Ports — keep; reshape later with graph runtime |
| `internal/runtime/**` | Session/composer/observe — **reshape later**, not blind-delete in P1 |
| `cmd/aiorchestrator/**` | Process entry |
| `docs/01`–`08` (+ later 09…) | Architecture + planning |
| Store foundations for session/audit/transcript/credentials | Reshape in P2; do not empty-delete without replacement plan |
| `kb_document` / `kb_chunk` **tables** (storage only) | Not deleted in P1. **Not** the V1 Inform design (OD-08-4). Frozen until P2.10 redesign + P2.13 DROP. |

### A.2 Purge targets (P1)

| Path / area | Why remove |
|---|---|
| `web/admin/**` | Old desk configurator UI — OD-08-1 |
| `web/user/**` | Old text-call UI — OD-08-1 |
| `web/supervisor/**` | Old supervisor — OD-08-1 (full delete) |
| `web/embed.go`, console routing | Three-console product surface |
| `internal/desk/**` | Old hybrid desk brain — [02](./02_CURRENT_STATE.md) discard |
| Desk control API + glue under `internal/control/` | Routes/handlers that only serve desk |
| `internal/gateway/deskskills/**` | Old desk skill runner |
| Desk-centric tests, lab publish scripts for coral-tfn/xfer install | Encourage repair of old path |
| Obsolete `.agent/phases/*` (and obsolete desk pipelines) | OD-08-2 — delete so agents cannot load old truth |

---

## Part B — P1 purge phases (detailed)

Each phase follows [07 §4](./07_PLANNING_STANDARDS.md). **Planning detail only.** Execute only after programme gate ([07 §8](./07_PLANNING_STANDARDS.md)).

### P1.0 — Purge inventory lock

- **Refs:** 02; this doc §A; OD-08-1…5  
- **Goal:** Freeze keep vs delete lists to exact paths (grep-backed inventory).  
- **In:** Path tables; control-route name list for P1.6; confirm OD text.  
- **Out:** No file deletion.  
- **Depends on:** Doc 07 Locked as process.  
- **Forbidden:** Starting P1.1 without path inventory attached.  
- **Exit:** §A tables + P1.6 file list Locked; ODs cited.  
- **Edge cases:** N/A.  
- **Verify:** Doc review sign-off on inventory appendix (added when L3 expands).

### P1.1 — Delete Admin UI

- **Refs:** 01 (config redesigned later); 02 discard UIs; OD-08-1  
- **Goal:** Remove `web/admin/**` so no desk editor remains.  
- **In:** `web/admin/**`; links from `web/index.html` / embed.  
- **Out:** New admin; graph editor.  
- **Depends on:** P1.0.  
- **Forbidden:** Porting old admin screens “temporarily.”  
- **Exit:** Paths absent; orch builds; no `/admin` static app.  
- **Edge cases:** Deep links to admin → 404/placeholder only.  
- **Verify:** `go build ./...`; `web/` has no `admin/`.

### P1.2 — Delete User UI

- **Refs:** 01 V1 is live voice; OD-08-1  
- **Goal:** Remove `web/user/**`.  
- **Depends on:** P1.0 (recommend after or with P1.1 — still **separate** phase id).  
- **Exit:** `web/user` absent; build OK.

### P1.3 — Delete Supervisor UI

- **Refs:** 01 — supervisor rebuild later under new plan; OD-08-1  
- **Goal:** Remove `web/supervisor/**` entirely.  
- **Depends on:** P1.0.  
- **Forbidden:** Leaving a “thin” session list.  
- **Exit:** Path absent; build OK.

### P1.4 — Remove console shell / embed

- **Refs:** 02; OD-08-1  
- **Goal:** Remove or replace `web/embed.go` + `web/index.html` so process does not ship old consoles.  
- **In:** Embed wiring in control server.  
- **Out:** New UI.  
- **Depends on:** P1.1–P1.3.  
- **Exit:** No embedded three-console product; health/API still up.  
- **Verify:** `go build`; `/` has no nav to admin/user/supervisor.

### P1.5 — Delete `internal/desk` package

- **Refs:** 02 discard desk hybrid; 03 new brain  
- **Goal:** Remove entire old dialogue brain package.  
- **In:** `internal/desk/**` all Go sources/tests.  
- **Out:** New graph runtime (later programme phases — not P1).  
- **Depends on:** P1.1–P1.4 first (UI gone).  
- **Forbidden:** Keeping `preset_coral_xfer` as “temporary”; adapting `Engine.Turn` into new graph.  
- **Exit:** Package absent.  
- **Note:** Compile may fail until P1.6 — **allowed**; do not merge P1.5+P1.6 into one phase id. Same human PR wave only if **both** phase exit criteria are listed and checked separately.

### P1.6 — Delete desk control API and runtime glue

- **Refs:** 03, 04; 06 control box  
- **Goal:** Remove desk HTTP routes, desk controllers, install-desk handlers, sandbox tied to desk.  
- **In:** Named files under `internal/control/` (exact list from P1.0 inventory).  
- **Out:** Session create/answer/stop/media/transfer/hangup needed for pipe.  
- **Depends on:** P1.5.  
- **Forbidden:** Leaving dead imports to `internal/desk`.  
- **Exit:** `go build ./...` and `go test ./...` green **without** desk.  
- **Edge cases:** Live DID dialogue may be broken until graph runtime — **acceptable**; document “media pipe only / dialogue offline” as post-P1 state.

### P1.7 — Delete deskskills gateway

- **Refs:** 03 Tools vs old skills  
- **Goal:** Remove `internal/gateway/deskskills/**`.  
- **Depends on:** P1.6.  
- **Exit:** Package absent; build green.

### P1.8 — Stop desk persistence usage

- **Refs:** P2.7 will replace with `flow_*` (OD-08-3)  
- **Goal:** No runtime read/write of `desk` / `desk_draft` / `desk_version`.  
- **In:** Store methods and control callers.  
- **Out:** DROP TABLE (P2.13 only — not this phase).  
- **Depends on:** P1.6.  
- **Exit:** Grep shows no desk table access in Go (except historical migration files).

### P1.9 — Delete obsolete agent phases

- **Refs:** 07 §7; OD-08-2  
- **Goal:** Agents cannot load old platform-desk phase YAML as current truth.  
- **In:** `.agent/phases/*` obsolete set; obsolete `.agent/pipelines/*` that only drive old desk world (inventory in P1.0).  
- **Depends on:** P1.0.  
- **Forbidden:** Soft-archive that remains on the default agent path.  
- **Exit:** Listed paths **deleted**; `AGENTS.md` / docs index point only to docs 01–08 (+ later planning docs).

### P1.10 — Remove desk lab/publish scripts & fixtures

- **Refs:** 02  
- **Goal:** No publish/install Coral desk scripts that resurrect old path.  
- **Depends on:** P1.5–P1.8 preferred.  
- **Exit:** Scripts gone (not “commented obsolete”).

### P1.11 — Remove desk-centric tests

- **Refs:** 07 exit criteria  
- **Goal:** Delete tests that import `internal/desk` or desk API.  
- **Depends on:** P1.5–P1.8.  
- **Exit:** `go test ./...` green.

### P1.12 — Dead code sweep

- **Refs:** 02  
- **Goal:** Remove compile-dead helpers only used by desk.  
- **Depends on:** P1.11.  
- **Exit:** Grep clean for desk symbols; build/test green.  
- **Exit (P1 series):** Post-P1 state documented: media/edge/gateways remain; dialogue brain and three consoles gone.

### P1 anti-patterns

- Do not “adapt” `Engine.Turn` into the new graph.  
- Do not keep hidden admin/supervisor “for lab convenience.”  
- Do not leave stubs that re-enable desk.  
- Do not DROP `kb_*` or `desk_*` tables in P1 (persistence stop ≠ DROP; DROP is P2.13).

---

## Part C — P2 schema phases (detailed)

**Naming (OD-08-3):** All new graph/config tables use **new** names (`flow_*` and companions). Final names locked in each phase’s field table **before** any migration text.

**Planning gate for deep P2 work:** Begin L3 field-table authoring for P2 only after **P1 catalog + P1 L3 specs** are Locked (motto / OD-08-5). Listing below is the catalog, not permission to deep-detail in parallel with P1.

### P2.0 — Schema principles

- **Refs:** 01 V1; 03 bindings; 04 transcript intent; 05 no PCM in DB  
- **Goal:** Rules for IDs, tenant_id, versioning, immutability of audit/transcript append, expand/contract migrations.  
- **Depends on:** P1 series Locked (planning); for L4: P1 Done.  
- **L3 file:** [phases/P2.0_schema_principles.md](./phases/P2.0_schema_principles.md) — **Planning Locked** 2026-09-04 (P2.0-P1…P12); no DDL.  
- **Exit:** Written principles + examples; no DDL yet — **met**.

### P2.1 — Credentials and tenant engine settings

- **Refs:** 05 gateways  
- **Existing:** `gateway_credentials`, `system_settings`, `tenant_engines`  
- **Goal:** Decide keep/reshape **field-by-field** (one entity focus).  
- **L3 file:** [phases/P2.1_credentials_engines.md](./phases/P2.1_credentials_engines.md) — **Planning Locked** 2026-09-04 (keep-as-is; migration **none**).  
- **Exit:** Field table Locked; migration plan note only — **met**.

### P2.2 — Session

- **Refs:** 06 lifecycle; 04 Ending  
- **Existing:** `session`  
- **Goal:** Canonical live CC session columns (flow version ref, languages, recording_ref, states).  
- **L3 file:** [phases/P2.2_session.md](./phases/P2.2_session.md) — **Planning Locked** 2026-09-04 (inventory vs `001`/`004`/`005` + Go `State*`; migration **none**; `flow_*` pins → P2.7).  
- **Exit:** Field table; deprecate unused columns listed explicitly — **met**.

### P2.3 — Transcript events

- **Refs:** 04 § Transcript; 01 Automatic Call Transcription  
- **Existing:** `transcript_turn`  
- **Goal:** Event-capable model (role, text, actionable flag, reason, tool/graph refs, seq, timestamps).  
- **L3 file:** [phases/P2.3_transcript_events.md](./phases/P2.3_transcript_events.md) — **Planning Locked** 2026-09-04 (keep name; expand cols + drop pair unique; DDL deferred).  
- **Exit:** Entity+fields Locked; mapping from old turns documented — **met**.

### P2.4 — Audit events

- **Refs:** 04 tool.armed etc.  
- **Existing:** `audit_event`  
- **Goal:** Standardize `event_type` vocabulary for V1.  
- **L3 file:** [phases/P2.4_audit_events.md](./phases/P2.4_audit_events.md) — **Planning Locked** 2026-09-04 (keep table; V1 allowlist + legacy inventory; DDL none).  
- **Exit:** Enum/list Locked — **met**.

### P2.5 — Recording metadata

- **Refs:** 02 recording leak lesson; 04 Ending  
- **Goal:** Fields + lifecycle rules (stop with leg).  
- **L3 file:** [phases/P2.5_recording_metadata.md](./phases/P2.5_recording_metadata.md) — **Planning Locked** 2026-09-04 (session expand cols; stop reasons; DDL deferred).  
- **Exit:** Rules Locked (runtime stop behaviour is a later runtime phase — cite this exit) — **met**.

### P2.6 — Disposition

- **Refs:** 01 V1 local disposition  
- **Existing:** `session_disposition`  
- **Goal:** Minimal V1 statuses aligned with tool outcomes (`transferred_*`, `out_of_scope`, …).  
- **Exit:** Value list Locked.

### P2.7 — Flow graph publish model (replaces desk_*)

- **Refs:** 03 entire; 06 config; OD-08-3  
- **Existing to replace:** `desk`, `desk_draft`, `desk_version`  
- **Goal:** Draft + immutable published versions of conversation graph; storage strategy for nodes/edges decided here.  
- **Exit:** Table names + columns Locked; **no** silent reuse of desk JSON shape without an explicit mapping note.  
- **Forbidden:** Naming new tables `desk_*`.

### P2.8 — Prompts / locale assets

- **Refs:** 03 language-neutral graph  
- **Goal:** How `prompt_ref` → per-locale text is stored and versioned with flow.  
- **Depends on:** P2.7 Locked.  
- **Exit:** Entity Locked.

### P2.9 — Routing matrix

- **Refs:** 03 Tool params; 01 routing  
- **Goal:** intent/target/owner/number storage per published flow version.  
- **Depends on:** P2.7 Locked.  
- **Exit:** Entity Locked.

### P2.10 — Bindings redesign (knowledge / future CRM)

- **Refs:** [03](./03_BRAIN_AND_GRAPH.md) bindings ≠ nodes; Inform node; OD-08-4; this doc motto  
- **Existing (to retire):** `kb_document`, `kb_chunk` — **not** adopted as V1 Inform store  
- **Goal:** Design the **new** binding mechanism how Inform (and later skills) call knowledge when present; field tables for the new entities; explicit non-goals (full CRM out of V1 unless stubbed).  
- **In:** Binding registry model; Inform reference rules; migrate-or-discard policy for any old KB rows (default: discard content with tables unless a later OD says otherwise).  
- **Out:** Shipping a production FAQ product in this phase; porting old chunk schema “as-is.”  
- **Depends on:** P2.7 Locked; architecture 03 bindings section unchanged or amended first if needed.  
- **Forbidden:** “Keep kb_* and wire Inform to them for speed.”  
- **Exit:** New entity(+fields) Locked; retirement note for `kb_*` ready for P2.13.

### P2.11 — Caller language preference

- **Refs:** 03 Entry / ListenLanguage  
- **Existing:** `caller_preference`  
- **Goal:** Keep/reshape for ANI → language.  
- **Exit:** Field table Locked.

### P2.12 — Slots / session attributes

- **Refs:** 03 slots  
- **Existing:** `session_attributes`  
- **Goal:** Minimal slot keys for V1 graph.  
- **Exit:** Key policy Locked.

### P2.13 — DROP obsolete tables

- **Depends on:** P2.7–P2.12 replacements **Locked** (and later L4 applied in order).  
- **Goal:** Explicit DROP list including at least: `desk`, `desk_draft`, `desk_version`, `kb_document`, `kb_chunk`, plus any unused skill ledger named in inventory.  
- **Exit:** DROP list Locked; expand/contract **order** written (one concern per migration step — no bulk DROP of unrelated families in one file without sequenced statements and rollback notes).

### P2.14 — Migration + CI discipline

- **Refs:** [11_CI_AND_CD.md](./11_CI_AND_CD.md)  
- **Goal:** Migration numbering, test DB apply, failure policy.  
- **Exit:** Rules Locked for Implementer; Job B (migrate-empty) expands empty-Postgres CI.

### P2 anti-patterns

- One mega-migration “fix everything.”  
- Storing PCM in Postgres.  
- LLM-writable dial numbers as source of truth (matrix only).  
- Designing UI tables before P2.7 flow model.  
- Reusing `kb_*` as the new Inform binding (violates OD-08-4).  
- Starting P2 DDL while P1 purge is incomplete (violates OD-08-5).

---

## Part D — Serial order (planning depth and later L4)

```text
OD-08 SETTLED (this revision)
 → Deep-detail / L3: P1.0 … P1.12 only (one phase at a time)
 → Lock P1 series
 → Deep-detail / L3: P2.0 only, then P2.1 … P2.6 one at a time
 → then P2.7 … P2.12 one at a time (flow_* + bindings redesign)
 → then P2.13 … P2.14
 → then Evidence plan doc 09, Coding principles 10, CI 11, L3 catalog close, Implementer skill
 → L4 gate ([07 §8](./07_PLANNING_STANDARDS.md)): execute P1.0, then P1.1, … never skip ahead
 → Only after P1.12 Done: L4 for P2 migrations in phase order
```

**Forbidden interleaves:** P2 field workshops while P1 L3 is still open; L4 migrate during L4 purge; combining P2.7+P2.10+P2.13 into one “schema week.”

---

## Part E — Handoff

- Product ODs for this doc: **SETTLED**.  
- L3 files live under [`docs/phases/`](./phases/README.md).  
- **Done (planning draft):** P1/P2 L3; docs 09–11; **[12 L4 roles](./12_AGENTIC_L4_ROLES.md)** + Implementer/Reviewer/Summarizer skills.  
- **Next:** **Owner Lock** of 09–12 (and P1 wave ack). Then: `Implementer: phase P1.1` (or P1.0). No L4 until Lock.  

**Not started:** code deletion, DDL, workflow YAML, E.*/CI.* L3 file expansion.
