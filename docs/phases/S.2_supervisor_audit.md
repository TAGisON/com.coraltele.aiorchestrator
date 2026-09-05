# L3 — S.2 Supervisor audit browser

| Field | Value |
|---|---|
| **id** | `S.2` |
| **title** | Supervisor: allowlisted audit event browser |
| **status** | **Closed** |
| **parent_plan** | [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) Wave S |
| **depends_on** | S.1 Closed (`9ac1e73`); P2.4 / E.4 allowlist |

## architecture_refs

- [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) — Supervisor audit capability
- [P2.4_audit_events.md](./P2.4_audit_events.md) — V1 `event_type` allowlist
- [09_EVIDENCE_AND_RECORDING.md](../09_EVIDENCE_AND_RECORDING.md) — B2 audit
- `GET /v1/sessions/{id}/audit`; `GET /v1/meta/catalog`

## goal

Let supervisors browse a session’s allowlisted audit events (tool / graph / recording / session lifecycle) with a type filter — read-only, no secrets/PCM in payloads.

## in_scope

- Catalog field `audit_event_types` from store P2.4 allowlist constants
- Supervisor detail: Audit section — table of `created_at`, `event_type`, payload preview; filter select (All + catalog types)
- Load via existing `OrchAPI.getAudit` when opening session detail
- Static test strings; catalog test asserts non-empty `audit_event_types`
- This phase file + README handoff

## out_scope

- Cross-session aggregates (**S.3**)
- Supervisor soak (**S.4**)
- Server-side audit query language / pagination beyond existing list
- Emitting new audit types
- Write/delete audit rows

## forbidden

- Showing raw secrets or PCM in UI
- Mutating sessions from Supervisor
- Desk revive

## exit_criteria

- [x] Catalog includes `audit_event_types` (P2.4 set)
- [x] Supervisor detail shows audit events for selected session
- [x] Filter by `event_type` works client-side
- [x] `go test ./web/... ./internal/control/...` green

## edge_cases

1. Empty audit list → friendly empty state.
2. Unknown/legacy type in store still displayed (filter “All”); catalog lists allowlist only.
3. Large payload → truncate preview in UI (full JSON on expand or probe).

## verification

```text
go test ./web/... ./internal/control/... -count=1 -timeout 180s
go build ./cmd/aiorchestrator/
```

## rollback

Revert catalog field + audit panel; keep S.1.

## handoff

Next: **S.3** Light aggregates.
