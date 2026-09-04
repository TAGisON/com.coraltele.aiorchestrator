# L3 phase files

**Parent:** [07_PLANNING_STANDARDS.md](../07_PLANNING_STANDARDS.md) §4  
**Domain plan:** [08_PURGE_AND_SCHEMA_PHASES.md](../08_PURGE_AND_SCHEMA_PHASES.md)

Gate **Locked** 2026-09-04. P1 L4 **Done**; P2.0–P2.14 **Closed**. DDL: M-A–M-E Closed; auto-loop → M-F.

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
| M-E | [M-E_reader_cutover.md](./M-E_reader_cutover.md) | **Closed** — kb/compliance Go readers removed |
| M-F…M-H | per [P2.13](./P2.13_drop_obsolete.md) | Not started (after M-E) |

## Still later

- E.* evidence runtime L3 expansion  
- CI.0–CI.2 workflow L3 when implementing Actions  
