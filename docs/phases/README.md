# L3 phase files

**Parent:** [07_PLANNING_STANDARDS.md](../07_PLANNING_STANDARDS.md) §4  
**Domain plan:** [08_PURGE_AND_SCHEMA_PHASES.md](../08_PURGE_AND_SCHEMA_PHASES.md)

Gate **Locked** 2026-09-04. P1 L4 **Done**; P2.0–P2.14 **Closed**. DDL: M-A–M-H Closed.

## P1 — Purge

| Phase | File | Status |
|---|---|---|
| P1.0 | [P1.0_purge_inventory.md](./P1.0_purge_inventory.md) | **Done** (owner signed) |
| P1.1–P1.4 | UI / shell | **Closed** (`573c94b`) |
| P1.5–P1.12 | desk → dead-code sweep | **Closed** (`2bc2dbc`…`d3f06d6`) |

Evidence: `.agent/work/P1.*` (local)

## P2 — Schema

| Phase | File | Status |
|---|---|---|
| P2.0 | [P2.0_schema_principles.md](./P2.0_schema_principles.md) | **Closed** (`1741e0e`) |
| P2.1 | [P2.1_credentials_engines.md](./P2.1_credentials_engines.md) | **Closed** (`64b2fa1`) |
| P2.2 | [P2.2_session.md](./P2.2_session.md) | **Closed** (`2a50cca`) |
| P2.3 | [P2.3_transcript_events.md](./P2.3_transcript_events.md) | **Closed** (`b154aa9`) |
| P2.4 | [P2.4_audit_events.md](./P2.4_audit_events.md) | **Closed** (`ce47941`) |
| P2.5 | [P2.5_recording_metadata.md](./P2.5_recording_metadata.md) | **Closed** (`7872487`) |
| P2.6 | [P2.6_disposition.md](./P2.6_disposition.md) | **Closed** (`cd81f37`) |
| P2.7 | [P2.7_flow_publish_model.md](./P2.7_flow_publish_model.md) | **Closed** (`5c6e376`) |
| P2.8 | [P2.8_prompts_locale.md](./P2.8_prompts_locale.md) | **Closed** (`d76f919`) |
| P2.9 | [P2.9_routing_matrix.md](./P2.9_routing_matrix.md) | **Closed** (`b5a1bd9`) |
| P2.10 | [P2.10_bindings_redesign.md](./P2.10_bindings_redesign.md) | **Closed** (`9381fac`) |
| P2.11 | [P2.11_caller_preference.md](./P2.11_caller_preference.md) | **Closed** (`7788d1e`) |
| P2.12 | [P2.12_slots_attributes.md](./P2.12_slots_attributes.md) | **Closed** (`8d2e1f9`) |
| P2.13 | [P2.13_drop_obsolete.md](./P2.13_drop_obsolete.md) | **Closed** (`1a59b3e`) |
| P2.14 | [P2.14_migration_ci.md](./P2.14_migration_ci.md) | **Closed** (`c1e8040`) |

## P2 DDL waves (expand/contract)

| Phase | File | Status |
|---|---|---|
| M-A | [M-A_create_flow.md](./M-A_create_flow.md) | **Closed** (`78f57f7`) — `010_flow_registry.sql` |
| M-B | [M-B_create_binding.md](./M-B_create_binding.md) | **Closed** (`3534682`) — `011_binding.sql` |
| M-C | [M-C_session_flow_pin.md](./M-C_session_flow_pin.md) | **Closed** (`77a59ce`) — `012_session_flow_pin.sql` |
| M-Cr | [M-Cr_session_recording.md](./M-Cr_session_recording.md) | **Closed** (`3b30daf`) — `013_session_recording_lifecycle.sql` |
| M-D | [M-D_transcript_expand.md](./M-D_transcript_expand.md) | **Closed** (`44c711b`) — `014_transcript_turn_expand.sql` |
| M-E | [M-E_reader_cutover.md](./M-E_reader_cutover.md) | **Closed** (`24aefde`) — kb/compliance Go readers removed |
| M-F | [M-F_drop_desk.md](./M-F_drop_desk.md) | **Closed** (`cbdc974`) — `015_drop_desk.sql` |
| M-G | [M-G_drop_kb.md](./M-G_drop_kb.md) | **Closed** (`89cab6c`) — `016_drop_kb.sql` |
| M-H | [M-H_drop_compliance.md](./M-H_drop_compliance.md) | **Closed** (`5862286`) — `017_drop_compliance.sql` |

## Evidence runtime (E.*)

| Phase | File | Status |
|---|---|---|
| E.0 | [E.0_evidence_inventory.md](./E.0_evidence_inventory.md) | **Closed** (`92643ca`) — gap list |
| E.1 | [E.1_transcript_emitter.md](./E.1_transcript_emitter.md) | **Closed** (e6dc674) |
| E.2 | [E.2_recording_lifecycle.md](./E.2_recording_lifecycle.md) | **Closed** (4c6418a) — start/stop stamps |
| E.3 | [E.3_orphan_reaper.md](./E.3_orphan_reaper.md) | **Closed** (1aafbce) — orphan_reaper |
| E.4 | [E.4_audit_allowlist.md](./E.4_audit_allowlist.md) | **Closed** (`8fc4d67`) — P2.4 allowlist emitters |
| E.5 | [E.5_disposition_tool_settle.md](./E.5_disposition_tool_settle.md) | **Closed** (193c9dd) — P2.6 final on tool settle / Ending |
| E.6 | [E.6_evidence_soak_checklist.md](./E.6_evidence_soak_checklist.md) | **Closed** (3471666) — lab soak checklist |

## CI / CD (CI.*)

| Phase | File | Status |
|---|---|---|
| CI.0 | [CI.0_workflow_job_a.md](./CI.0_workflow_job_a.md) | **Closed** (`14b1965`) — Job A |
| CI.1 | [CI.1_migrate_empty.md](./CI.1_migrate_empty.md) | **Closed** (`14b1965`) — Job B |
| CI.2 | [CI.2_secrets_hygiene.md](./CI.2_secrets_hygiene.md) | **Closed** (36e0146) — Job C secrets-hygiene |
| CI.3 | [CI.3_golangci_lint.md](./CI.3_golangci_lint.md) | **Closed** (b286c10) — Job D golangci-lint (fail) |
| CD.0 | [CD.0_lab_promote.md](./CD.0_lab_promote.md) | **Closed** (9eb1aba) — lab PROMOTE.md |
| CD.1 | [CD.1_lab_build_workflow.md](./CD.1_lab_build_workflow.md) | **Closed** (00c09f6) — manual lab-build artifacts |

## Graph runtime (G.*)

| Phase | File | Status |
|---|---|---|
| G.0 | [G.0_graph_runtime_inventory.md](./G.0_graph_runtime_inventory.md) | **Closed** (fcfa5b2) — gap list + G.1–G.7 sketch |
| G.1 | [G.1_store_flow_binding.md](./G.1_store_flow_binding.md) | **Closed** (1c3140b) — flow/binding store + session pins |
| G.2 | [G.2_flow_control_api.md](./G.2_flow_control_api.md) | **Closed** (5fbdb9a) — `/v1/flows*` + coral.flow.v1 publish validate |
| G.3 | [G.3_runtime_core_cursor.md](./G.3_runtime_core_cursor.md) | **Closed** (599d37b) — Entry/Speak/ListenChoice/End cursor |
| G.4 | [G.4_tool_arm_exec.md](./G.4_tool_arm_exec.md) | **Closed** (6870910) — Tool ARM + matrix freeze + exec once |
| G.5 | [G.5_repair_language.md](./G.5_repair_language.md) | **Closed** (1d8fc87) — repair + ListenLanguage + locale |
| G.6 | [G.6_inform_binding.md](./G.6_inform_binding.md) | **Closed** (8a198e0) — Inform + inline_faq binding |
| G.7 | [G.7_evidence_cutover.md](./G.7_evidence_cutover.md) | **Closed** (ee2cbb3) — edge_taken/tool_line + live flow pin |

## Lab soak (L.*)

| Phase | File | Status |
|---|---|---|
| L.0 | [L.0_graph_lab_soak.md](./L.0_graph_lab_soak.md) | **Closed** (a84614b) — graph V1 soak checklist (+ fixture) |

## Production consoles (U.* / A.* / C.* / S.* / V.*)

**L2:** [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) (**Locked** — ODs settled 2026-09-05).  
Admin = full config (flows create/draft/publish, profiles, engines, bindings, matrix, pin). Chat = same graph without STT/TTS (`clock=chat`). Supervisor = evidence read-only.

| Phase | File | Status |
|---|---|---|
| U.0 | [U.0_consoles_inventory.md](./U.0_consoles_inventory.md) | **Closed** — gap inventory + phase freeze |
| U.1 | [U.1_meta_catalog.md](./U.1_meta_catalog.md) | **Closed** (5d9713a) — `GET /v1/meta/catalog` |
| U.2 | [U.2_console_shells.md](./U.2_console_shells.md) | **Closed** (60c46ab) — shared client + Admin/Supervisor/Chat shells |
| A.1 | [A.1_admin_tenant_config.md](./A.1_admin_tenant_config.md) | **Closed** (411eefd) — profiles / engines / credentials / settings |
| A.2 | [A.2_bindings_http_admin.md](./A.2_bindings_http_admin.md) | **Closed** (6b8b882) — bindings HTTP + Admin CRUD |
| A.3 | [A.3_admin_flows_draft.md](./A.3_admin_flows_draft.md) | **Closed** (6c85e18) — flow list/create/draft |
| A.4 | [A.4_admin_graph_builder.md](./A.4_admin_graph_builder.md) | **Closed** (d5e2f9d) — graph builder + publish + version inspect |
| A.5–A.6 | — | Next — live pin UX, Admin soak |
| C.1–C.4 | — | Planned — User chat channel |
| S.1–S.4 | — | Planned — Supervisor |
| V.1–V.2 | — | Planned — dual chat+call prove |

## Still later

- **A.5** profile / DID / live pin association, then A.6 Admin soak
- Owner **runs** L.0 / E.6 / later V.1 sign-off on lab (human)
- Full flow JSON Schema file ([03](../03_BRAIN_AND_GRAPH.md) deferred)
- Docs/01 Next/Later SKUs (summary, CRM push, QM, …)

