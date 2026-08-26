# com.coraltele.aiorchestrator

Coral Tele **Speech-and-Agent Platform** (working name: AI Orchestrator).

A server-side runtime that attaches audio or text **in**, runs a **named profile**, and attaches audio, text, or an action **out**. Call center, meeting summaries, policy copilots, captions, and interpretation are **profiles** of this product — not the product itself.

| | |
|---|---|
| **Artifact id** | `com.coraltele.aiorchestrator` |
| **Status** | Product locked; architecture approach locked for planning |
| **Not this repo** | Telephony (`mod_audio_stream`), Coral ACD, vendor clouds |

## Documents

| Doc | Role |
|---|---|
| [docs/product/PRODUCT_DECISIONS.md](docs/product/PRODUCT_DECISIONS.md) | **Locked** product source of truth |
| [docs/architecture/ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md) | How we build: runtime, contracts, translators, edges |
| [docs/architecture/CONTRACTS.md](docs/architecture/CONTRACTS.md) | Stable sockets vendors and feeders must fill |
| [docs/learning/VENDOR_API_STUDY.md](docs/learning/VENDOR_API_STUDY.md) | What we learned from a working voice-AI API guide |
| [docs/verticals/README.md](docs/verticals/README.md) | Contact-center vertical lives in another repo |

## Rule

If a demo, vendor, or feeder disagrees with **product decisions**, change that file first. Architecture may not silently redefine the product.

## Initialize

```bash
git status
```

Language and module layout are not chosen yet. This repository starts as the **decision and architecture home**.
