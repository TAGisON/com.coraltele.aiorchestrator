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

## Recommended auto flow (independent sessions + monitor)

```powershell
# Terminal 1 — start pipeline at Phase B (or phase-a)
$env:AGENT_NO_MAIL = "1"   # until secrets installed
.\tools\agent-runner\agent.ps1 start -From phase-b
.\tools\agent-runner\agent.ps1 monitor-start

# Monitor every ~60s:
# - if no worker PID → Start-RoleWorker (new Cursor window + prompt)
# - if role wrote # agent-approval → complete-role and assign next role
```

Each role must run in the **Cursor window opened for that role** (true session independence). Do not reuse the planner window for coder/reviewer.

```powershell
.\tools\agent-runner\agent.ps1 status          # includes worker.pid when assigned
.\tools\agent-runner\agent.ps1 assign-role      # force re-assign current role
.\tools\agent-runner\agent.ps1 monitor-stop
.\tools\agent-runner\agent.ps1 stop | resume | restart-phase -Phase phase-a
.\tools\agent-runner\agent.ps1 decide -Id D1 -Answer "..."
.\tools\agent-runner\agent.ps1 continue
```

Manual fallback (no monitor):

```powershell
.\tools\agent-runner\agent.ps1 next-prompt
# paste into a NEW Cursor agent chat
.\tools\agent-runner\agent.ps1 complete-role -Result pass
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

`.agent/status.json` includes when a worker is assigned:

```json
"worker": {
  "pid": 12345,
  "session_id": "a1b2c3d4e5f6",
  "role": "coder",
  "phase": "phase-b",
  "prompt_file": "...",
  "started_at": "...",
  "status": "assigned"
}
```

When the role finishes, monitor clears `worker` after `complete-role`.

---

## Roles (global skills)

| Role | Skill (`~/.cursor/skills/`) |
|---|---|
| Planner | `coral-phase-planner` |
| Coder | `coral-phase-coder` |
| Reviewer | `coral-phase-reviewer` |
| Summarizer | `coral-phase-summarizer` |

Global rules: `Documents/GitHub/.cursor/rules/coral-agent-*.mdc` (includes commits rule).

---

## This repo phases

`.agent/phases/*.yaml` from `docs/architecture/PLATFORM_FIRST.md` (phase-a … phase-f).
