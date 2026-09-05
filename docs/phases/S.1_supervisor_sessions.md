# L3 — S.1 Supervisor session list + detail

| Field | Value |
|---|---|
| **id** | `S.1` |
| **title** | Supervisor: session list + detail (transcript, disposition, recording meta) |
| **status** | **Closed** |
| **parent_plan** | [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) Wave S |
| **depends_on** | C.4 Closed (`e78d38f`); U.2 shells; existing `GET /v1/sessions*` |

## architecture_refs

- [13_PRODUCTION_CONSOLES.md](../13_PRODUCTION_CONSOLES.md) — Supervisor inventory (read-only)
- [09_EVIDENCE_AND_RECORDING.md](../09_EVIDENCE_AND_RECORDING.md) — transcript / disposition / recording ranks
- [E.2_recording_lifecycle.md](./E.2_recording_lifecycle.md) — recording stamps on session
- Control: `GET /v1/sessions`, `GET /v1/sessions/{id}`, transcript, disposition

## goal

Give supervisors a read-only console to browse recent sessions and open one for transcript timeline, disposition, and recording metadata — without Admin write controls or audit browser (S.2).

## in_scope

- Expose on session GET (and list where cheap): `flow_id` / `flow_version`; recording meta `recording_ref`, `recording_started_at`, `recording_stopped_at`, `recording_stop_reason`, `recording_bytes` when present
- `web/supervisor/index.html` + `supervisor.js`:
  - Token row; Refresh session list
  - Table/list: id, clock, state, profile, flow pin, recording_ref hint, created
  - Detail panel: session JSON summary, disposition, recording meta, transcript turns (reuse chat turn styling)
  - Read-only — no inject/stop/publish
- `OrchAPI.listSessions` (+ any missing helpers)
- Static/embed tests
- This phase file + README handoff

## out_scope

- Full audit browser (**S.2**)
- Cross-session aggregates (**S.3**)
- Supervisor soak (**S.4**)
- PATCH disposition / ops override
- New DDL
- Live SSE subscribe in Supervisor (poll/refresh is enough)

## forbidden

- Mutate flows, publish, arm tools, inject text
- Desk-era supervisor revive
- Secrets in UI dumps

## exit_criteria

- [x] `/supervisor/` lists sessions and opens detail with transcript + disposition
- [x] Recording meta fields appear on GET session when stamped (or empty/absent cleanly)
- [x] List includes flow pin fields when set
- [x] No write actions on Supervisor UI
- [x] `go test ./web/... ./internal/control/... ./internal/store/...` green (or store+control if store touched)

## edge_cases

1. Empty session list → friendly empty state.
2. Disposition 404 → show “none” not hard fail.
3. Chat sessions show transcript without requiring `recording_ref`.

## verification

```text
go test ./web/... ./internal/control/... ./internal/store/... -count=1 -timeout 180s
go build ./cmd/aiorchestrator/
```

## rollback

Revert supervisor UI + session JSON recording fields; store GetSession recording SELECT if added.

## handoff

Next: **S.2** Audit browser.
