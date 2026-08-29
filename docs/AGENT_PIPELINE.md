# Coral agentic phase pipeline — operator guide

**Purpose:** Plan → Code → Review → Summarize in small phases, with status, PID-tracked workers, monitor, mail, and human control.

**Security:** Put SMTP credentials only in `.agent/secrets.local.json` (gitignored). Rotate any password that was pasted into chat. Use `Install.ps1` — never paste passwords into Cursor chat.

---

## One-time setup

```powershell
cd C:\Users\user\Documents\GitHub\com.coraltele.aiorchestrator
.\tools\agent-runner\Install.ps1
$env:AGENT_NO_MAIL = "0"   # or "1" to skip mail while testing
.\tools\notify\Send-AgentMail.ps1 -Subject "agent-pipeline test" -Body "Mail works."
```

---

## Recommended auto flow (quiet dispatch + monitor)

**Default:** no new Cursor windows, no Minimized PowerShell on the desktop. Monitor runs **Hidden**. `assign-role` writes `NEXT_PROMPT_<role>.txt` + `DISPATCH_<role>.json` only. You (or a parent agent Task) run that prompt; monitor advances when `# agent-approval` appears.

```powershell
$env:AGENT_NO_MAIL = "1"   # until secrets installed
.\tools\agent-runner\agent.ps1 start -From phase-c
.\tools\agent-runner\agent.ps1 monitor-start   # Hidden background poll

# Monitor every ~60s:
# - if role not yet dispatched → quiet file dispatch (once; no UI)
# - if # agent-approval present → complete-role → next role dispatch
# - never re-opens Cursor when a dispatch is already pending
```

Opt-in old behavior (opens Cursor — usually unwanted):

```yaml
# .agent/config.yaml
monitor:
  assign_opens_new_cursor_window: true
```

Or one-shot: `.\tools\agent-runner\Start-RoleWorker.ps1 -OpenCursor`

```powershell
.\tools\agent-runner\agent.ps1 status
.\tools\agent-runner\agent.ps1 assign-role      # rewrite prompt only (idempotent unless Force)
.\tools\agent-runner\agent.ps1 monitor-stop
.\tools\agent-runner\agent.ps1 stop | resume | restart-phase -Phase phase-c
.\tools\agent-runner\agent.ps1 decide -Id D1 -Answer "..."
.\tools\agent-runner\agent.ps1 continue
```

Manual fallback:

```powershell
.\tools\agent-runner\agent.ps1 next-prompt
# run prompt in an existing agent session / Task — do not need a new Cursor window
.\tools\agent-runner\agent.ps1 complete-role   # reads result from artifact only (no rubber-stamp)
```

---

## Role rules (commits)

| Role | Git duty |
|---|---|
| **Planner** | Read `git log` + prior phase summary; plan only the delta (`reviewed_prior_commits`) |
| **Coder** | Implement plan; no commit |
| **Reviewer** | Review vs plan; no commit |
| **Summarizer** | Re-run verify; **commit only if green**; record hash in summary |

---

## Status fields (monitor)

`.agent/status.json` when a role is dispatched (default quiet mode):

```json
"worker": {
  "pid": null,
  "session_id": "a1b2c3d4e5f6",
  "role": "coder",
  "phase": "phase-c",
  "prompt_file": "...",
  "started_at": "...",
  "status": "dispatched",
  "mode": "dispatch"
}
```

`mode: cursor` only when explicitly opted in (then `pid` is set). Monitor does **not** re-dispatch while status is `dispatched`/`assigned`. Clears `worker` after `complete-role`.

## This repo — how many phases?

**Build (`coral-phase`):** 6 total — `phase-a` … `phase-f` (PLATFORM_FIRST). Catalog complete on main.

**Validation (`product-validation`):** see `docs/VALIDATION_PIPELINE.md` — start with `validation-v1` (no FS yet).

```powershell
.\tools\agent-runner\agent.ps1 start -Pipeline product-validation -From validation-v1
```

---

## Roles (global skills) — coral-phase

| Role | Skill (`~/.cursor/skills/`) |
|---|---|
| Planner | `coral-phase-planner` |
| Coder | `coral-phase-coder` |
| Reviewer | `coral-phase-reviewer` |
| Summarizer | `coral-phase-summarizer` |

Validation roles: `coral-validation-*` (see VALIDATION_PIPELINE.md).

Global rules: `Documents/GitHub/.cursor/rules/coral-agent-*.mdc` (includes commits rule).

---

Phase YAML catalog: `.agent/phases/*.yaml` (build A–F + `validation-v1`).
Defs: `.agent/pipelines/coral-phase.json`, `product-validation.json`.
