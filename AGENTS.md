# Speech-and-Agent Platform — agent notes

## Source of truth

1. `docs/product/PRODUCT_DECISIONS.md` — product  
2. `docs/SOLUTION.md` + `docs/architecture/*` — architecture  
3. `docs/architecture/PLATFORM_FIRST.md` — build order  
4. `docs/architecture/PORTS.md` — Go port freeze  
5. `.agent/phases/*.yaml` — executable phase exit criteria  

## Agentic pipeline

Operator guide: `docs/AGENT_PIPELINE.md`  
CLI: `tools/agent-runner/agent.ps1`  

Roles (global skills): planner → coder → reviewer → summarizer.  
Handoff via `.agent/work/<phase>/` artifacts + `# agent-approval`, not agent-to-agent chat.

## Hard rules

- Go kernel; no Python/Java media kernel  
- No Kafka/Redis for PCM  
- No vendor SDKs in composer; fakes before Next AI  
- Never commit `.agent/secrets.local.json`  
