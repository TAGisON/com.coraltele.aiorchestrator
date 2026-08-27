# com.coraltele.aiorchestrator

Coral Tele **Speech-and-Agent Platform** (working name: AI Orchestrator).

A server-side runtime that attaches audio or text **in**, runs a **named profile**, and attaches audio, text, or an action **out**. Call center, meeting summaries, policy copilots, captions, and interpretation are **profiles** of this product — not the product itself.

| | |
|---|---|
| **Artifact id** | `com.coraltele.aiorchestrator` |
| **Status** | Product locked. Architecture locked: Go, gateway routers, in-memory audio. |
| **Not this repo** | Telephony (`mod_audio_stream`), Coral ACD, TTS-Engine process, vendor clouds |

## Documents

| Doc | Role |
|---|---|
| [docs/SOLUTION.md](docs/SOLUTION.md) | **Architecture solution** for review (technical; product only summarized) |
| [docs/product/PRODUCT_DECISIONS.md](docs/product/PRODUCT_DECISIONS.md) | Locked **product** source of truth |
| [docs/architecture/INTEGRATION.md](docs/architecture/INTEGRATION.md) | Our profile/persona vs their KB/CRM; audit vs logs |
| [docs/architecture/CONTRACTS.md](docs/architecture/CONTRACTS.md) | Stable sockets vendors and feeders must fill |
| [docs/architecture/PORTS.md](docs/architecture/PORTS.md) | Frozen Go-shaped port interfaces |
| [docs/architecture/PLATFORM_FIRST.md](docs/architecture/PLATFORM_FIRST.md) | Coral complete before external vendors |
| [docs/architecture/CONTROL_API.md](docs/architecture/CONTROL_API.md) | Coral-facing HTTP/SSE API |
| [docs/architecture/RUNTIME.md](docs/architecture/RUNTIME.md) | Session, clocks, turn machine, Think path, failures |
| [docs/architecture/ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md) | Locked approach: Go, routers, in-memory audio |
| [docs/architecture/PROFILE_SCHEMA.md](docs/architecture/PROFILE_SCHEMA.md) | Profile JSON shape, audio rates 8–48 kHz, versioning |
| [docs/architecture/RULES_AND_SKILLS.md](docs/architecture/RULES_AND_SKILLS.md) | Rules engine, skills, playbooks, warm transfer |
| [docs/architecture/EDGE_FS.md](docs/architecture/EDGE_FS.md) | mod_audio_stream WSS auth and resampling |
| [docs/architecture/ANALYTICS_AND_POSTCALL.md](docs/architecture/ANALYTICS_AND_POSTCALL.md) | KPIs, disposition, captions delivery |
| [docs/architecture/OPERATIONS.md](docs/architecture/OPERATIONS.md) | Deploy, drain, limits, on-prem gateway packs |
| [docs/architecture/SERVICE.md](docs/architecture/SERVICE.md) | Tech stack, architecture style, what this service owns |
| [docs/architecture/TECH_CHOICES.md](docs/architecture/TECH_CHOICES.md) | Why this vs Go/Java/microservices/Kafka/etc. |
| [docs/learning/VENDOR_API_STUDY.md](docs/learning/VENDOR_API_STUDY.md) | What we learned from a working voice-AI API guide |
| [docs/architecture/BUILD.md](docs/architecture/BUILD.md) | What we use, components, latency, Coral integration |
| [docs/AGENT_PIPELINE.md](docs/AGENT_PIPELINE.md) | Agentic plan→code→review→summarize runner |
| [AGENTS.md](AGENTS.md) | Short agent entrypoint |
| [docs/verticals/README.md](docs/verticals/README.md) | Contact-center vertical lives in another repo |

## Rule

If a demo, vendor, or feeder disagrees with **product decisions**, change that file first. Architecture may not silently redefine the product.

## Agentic implementation pipeline

```powershell
cd C:\Users\user\Documents\GitHub\com.coraltele.aiorchestrator
.\tools\agent-runner\Install.ps1          # enter SMTP secrets locally (never commit)
.\tools\notify\Send-AgentMail.ps1 -Subject "pipeline test" -Body "ok"
.\tools\agent-runner\agent.ps1 start -From phase-a
.\tools\agent-runner\agent.ps1 next-prompt   # paste into a new Cursor agent chat
.\tools\agent-runner\agent.ps1 complete-role -Result pass
.\tools\agent-runner\agent.ps1 status
```

Full guide: [docs/AGENT_PIPELINE.md](docs/AGENT_PIPELINE.md).

## Initialize

```bash
git status
```

Language: **Go**. Vendor engines: **payment-gateway routers**. Live audio: **in-process memory only**. Details in `docs/architecture/ARCHITECTURE.md`.
