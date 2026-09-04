# 01 — Vision and scope

## Product one-liner

An **LLM-based voice call centre agent** on Coral telephony: the caller talks naturally; a **configured conversation graph** decides the flow; STT/LLM/TTS are engines; **transfer** and **hangup** are locked tools.

We are **not** building caption products, live translators, meeting assistants, or a full Omni suite in V1.

## Origin of requirements

Sales/capability umbrella: *Next-Generation Coral PABX & AI Omni Contact Centre — AI Features & Capabilities*.

That document lists many AI features. This programme **filters** it to an LLM voicebot call centre. Other items are Next / Later.

## V1 — must be ticked (working call flow)

| Feature (from capability list) | V1 meaning |
|---|---|
| AI Voicebot / Virtual Agent | Bot greets, listens, replies, ends or transfers |
| Conversational IVR | Natural language over a **graph**, not DTMF-only |
| Customer Intent Classification | Speech → allowlisted intents / edges |
| AI-Powered Intelligent Call Routing | Intent → matrix → **transfer tool** |
| Speech-to-Text | Live STT for the bot |
| Automatic Call Transcription | Durable caller + bot + action timeline |
| Multilingual AI (Talk only) | Profile languages; mid-call switch on ask; TTS/STT follow active language — **not** caption/translator SKUs |
| Configuration | Admin/API defines graph, prompts, languages, voice, matrix, tools |
| Hangup / call end | Same tool-lock pattern as transfer |
| Thin FAQ / knowledge (optional per desk) | `Inform` node + knowledge **binding** |

**Also required for V1 engineering (not big brochure SKUs):**

- Tool lock: transfer / hangup (arm → final TTS → execute)
- Recording that **stops** with the call
- Audit of listen / speak / tool arm / tool exec
- Per-node repair for unclear / incomplete / out-of-context speech

## Explicitly not V1

| Capability list item | Timing |
|---|---|
| Call summarization | Next (after transcript is solid) |
| Rich CRM disposition push | Next (V1 may store disposition locally) |
| Sentiment / escalation detection | Next |
| AI Agent Assist / real-time guidance | After human transfer path is stable |
| AI Quality Management / call scoring | Later |
| Compliance monitoring | Later |
| Predictive analytics / WFO | Later |
| Executive AI dashboard | Later |
| AI Chatbot (web/digital) | Later |
| Voice biometrics / fraud | Later |
| Caption / one-way / two-way **translator products** | Out of this programme |

## Locked product principles

1. **Graph is law** — typed nodes; legal moves are declared edges (forward, back, retry, skip, global).
2. **LLM interprets inside the current node** — it does not invent topology or dial numbers.
3. **STT / LLM / TTS are pipes** — ears / understanding / mouth.
4. **Tools are irreversible when armed** — barge off → play closing line → execute once.
5. **Unclear / OOD / incomplete** — handled by **that node’s repair policy**, not by escaping the graph.
6. **External KB / CRM** — **bindings** referenced by nodes; optional per profile.
7. **Language-neutral graph** — one flow for all locales; prompts/TTS/STT follow `active_language`.
8. **Indian multilingual vendors** required for Talk; current baseline Sarvam (see media doc).

## What “done” means for V1 call flow

A live DID call can: welcome → (language if needed) → department / intent → FAQ if configured → **transfer** or **hangup**, with a complete transcript and a recording that ends with the leg — without false intents from bot echo or fragment turns.
