# agent-approval

Append this block at the end of your role artifact (`plan.md`, `implementation.md`, `review.md`, or `summary.md`).

```yaml
# agent-approval
role: planner   # planner | coder | reviewer | summarizer
result: pass    # pass | fail | blocker
checklist:
  - id: reviewed_prior_commits   # planner: required
    pass: true
  - id: example
    pass: true
  - id: verify_rerun             # summarizer: required
    pass: true
  - id: committed_after_verify   # summarizer: required on pass
    pass: true
blocker: null
# blocker example:
# blocker:
#   class: plan    # plan | human | env | spec
#   message: "PORTS.md missing Feeder open API"
#   return_to: planner
```

Planner must include section **Already implemented (from git)** from `git log` / prior summaries.  
Summarizer must re-run verify and **git commit** only on green; put commit hash in summary.md.
