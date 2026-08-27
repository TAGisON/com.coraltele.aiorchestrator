# agent-approval

Append this block at the end of your role artifact (`plan.md`, `implementation.md`, `review.md`, or `summary.md`).

```yaml
# agent-approval
role: planner   # planner | coder | reviewer | summarizer
result: pass    # pass | fail | blocker
checklist:
  - id: example
    pass: true
blocker: null
# blocker example:
# blocker:
#   class: plan    # plan | human | env | spec
#   message: "PORTS.md missing Feeder open API"
#   return_to: planner
```
