# Speech-and-Agent Platform — agent notes

## Source of truth

1. `docs/product/PRODUCT_DECISIONS.md` — product  
2. `docs/SOLUTION.md` + `docs/architecture/*` — architecture  
3. `docs/architecture/PLATFORM_FIRST.md` — build order  
4. `docs/architecture/PORTS.md` — Go port freeze  
5. `.agent/phases/*.yaml` — executable phase exit criteria  

## Agentic pipelines

| Pipeline | Guide | Roles |
|---|---|---|
| `coral-phase` | `docs/AGENT_PIPELINE.md` | planner → coder → reviewer → summarizer |
| `product-validation` | `docs/VALIDATION_PIPELINE.md` | scenario-planner → fixture-builder → test-runner → audit-validator → test-reviewer → test-summarizer |

CLI: `tools/agent-runner/agent.ps1`  
Defs: `.agent/pipelines/*.json`  
Universal QA contract: `tests/agent/MANIFEST.yaml` + feature inventory `tests/agent/FEATURES.md`  
Handoff via `.agent/work/<phase>/` + `# agent-approval`, not agent-to-agent chat.

## Hard rules

- Go kernel; no Python/Java media kernel  
- No Kafka/Redis for PCM  
- No vendor SDKs in composer; fakes before Next AI  
- Never commit `.agent/secrets.local.json`  
