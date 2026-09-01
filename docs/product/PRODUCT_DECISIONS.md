# Speech-and-Agent Platform — Locked Product Decisions

**Status:** LOCKED for planning (do not silently widen scope)  
**Date:** 25 August 2026  
**Home:** `com.coraltele.aiorchestrator`  
**Audience:** Product, delivery, sales packaging, future tickets  
**Contact-center vertical:** [`mod_audio_stream-1`](https://github.com/TAGisON/mod_audio_stream-1) repo (`docs/AI_Call_Center_Product_Decisions.md` when present)

This file is the product source of truth for **the platform**.  
It is **not** an architecture, vendor list, or integration SOW.  
Future feeders and skills plug into **slots named here**; they do not need to be specified now.

---

## 1. One-sentence product

A **configurable speech-and-agent runtime**: attach audio or text in, run a **named profile**, attach audio or text (or an action) out.

The profile decides what is on (listen, speak, think), whether the clock is **live or playback**, whether an **agent** may know / do / remember, and in which **language behavior**. Call center, meetings, policy copilots, captions, and interpretation are **profiles of this product**, not separate products.

---

## 2. What we are building / not building

**We are building:** a platform others consume. Operators (or APIs) define **profiles**. Something else supplies the stream and consumes the result.

**We are not building:** a call center, a video-meeting app, a mail server, or a CPaaS. Those may **integrate** later via feeder and skill slots.

**Working name:** Speech-and-Agent Platform.  
**Code / artifact:** `com.coraltele.aiorchestrator`.

---

## 3. Controllability (locked)

Work runs as a **named profile**, not a pile of raw toggles.

A profile always states:

1. **Pipe modes** — which of Listen / Speak / Talk / Think are on; live and/or playback.  
2. **Brain** — LLM off, or on (vendor is an implementation choice under modularity).  
3. **Language behavior** — none | captions | one-way interpret | two-way interpret.  
4. **Grounding** — none | document KB | templates | graph (org/workflow) | playbook.  
5. **Hands** — which skills are allowed; which need confirmation.  
6. **Law** — rules that cannot be overridden by the LLM.  
7. **Memory** — none | this session | this customer (consent + retention).  
8. **Learning** — analytics and suggested updates; no silent self-training.

Changeable between conversations without a code redeploy.  
Mid-conversation change is allowed only where safe (e.g. language switch, skill unlock after verification). Rewriting money/PII/escalation rules mid-session is **out**.

**Who edits profiles:** both a **non-technical admin** (presets + attach KB/skills/rules) and a **developer API**. V1 may ship API-first if the admin surface lags; the product still treats admin as in-category.

---

## 4. Modularity (locked)

Swappable without rewriting the product:

- STT, LLM, TTS (and later VAD / translation engines)  
- Feeders (where audio/text comes from)  
- Sinks (where audio/text/actions go)  
- Knowledge sources (files, graphs, templates)  
- Skills

**“Any vendor / any codec”** is the **direction**: published allowlists at the edge; canonical processing inside. It is not a V1 promise that every codec and every vendor works on the live path. Batch-only engines may run on **playback** only.

Fused speech-to-speech (single-vendor listen+think+speak) may exist as **one optional engine**. It is not the architecture of the product. **Default:** we call STT, LLM, and TTS as **separate services** through **routers** (payment-gateway style): the product says Speak; the **active** TTS gateway runs. TTS-Engine, Next AI TTS, and Sarvam are gateways on that rail — including first-party. Orchestration stays in this product.

Vendors (including Next AI) are **clients we consume**. We do not become their telephony socket, and we do not put our profile store inside a vendor.

**First-party engines are still vendors on the slot.** In-house TTS-Engine (Go) is a **Speak gateway** like Next AI TTS or Sarvam: same Speak contract, same gateway pattern, chosen by profile (`speak: tts-engine` vs `speak: nextai`). We own the process and the ops; we do **not** give it a private path around the orchestrator. Streaming vs file, PCM vs PCMU 8 kHz for PSTN, sample rate — those are **gateway capabilities**, not a second Speak API.

---

## 5. Pipe (locked)

Independent modes; LLM is not required for Listen or Speak.

| Mode | In | Out | Typical job |
|---|---|---|---|
| **Listen** | audio, live or playback | transcript / captions | captions, compliance tape, meeting tape |
| **Speak** | text | audio, live or playback | read-out, announcements, dubbed track |
| **Talk** | audio | audio | conversation or live interpretation |
| **Think** | transcript (live or playback) | text, structured result, and/or actions | copilot, MoM, QA, disposition |

Live and playback share the **mode**, not the **clock**. A profile that cannot say which clock it uses is incomplete.

---

## 6. Agent (locked)

Once there is text, the agent is a **bundle**, not “the LLM is on.”

- **Persona** — tone, language policy, refuse list  
- **Knowledge** — what may be treated as true  
- **Skills** — what may be *done* (lookup, mail, ticket, transfer, CRM), with audit  
- **Rules** — what is mandatory or forbidden  
- **Memory** — bounded, consensual  
- **Analytics** — so the bundle improves **on purpose**

**Priority when they collide:** rules > skills > grounding > free LLM.

**Authority (locked as a concept, not a V1 UI):** a profile is allowed to **inform**, **decide**, or **act**. Acting requires a skill + rule. Informing must not silently become acting.

**Grounding types (in-category, not one blob):**

- Document / FAQ corpus (RAG)  
- **Graph** (hierarchy, approval, workflow)  
- **Templates** (MoM shape, disposition shape, coaching rubric)  
- Playbooks (intents and paths)

If it is not in grounding and the profile is grounded, the product **refuses or escalates**. It does not invent policy, numbers, or org authority.

We **choose** what grounding is attached to an LLM call. Sending persona + profile + retrieved context in the request is our control. Storing our only KB solely inside a vendor is **out**.

**Where knowledge and CRM physically live** (see `docs/architecture/INTEGRATION.md`):

- **Ours:** profile, persona, agent, skill *contracts*, templates, session **audit**.  
- **Coral directory:** users, orgs, roles, existing user details — **we consume**, we do not replace. Sessions/profiles bind to `coral_user_id` / tenant; future extras key off that id.  
- **Theirs unless they opt in:** FAQ/KB/RAG corpora, CRM/RDBMS, txn/ticket truth.  
- **Knowledge router:** ingest/dump gateway **or** retrieve-API gateway **or** both.  
- **Skill router:** HTTP to their (or Coral) APIs for “status of my transaction.” Direct DB is last-resort, not default.  
- Greenfield tenants may use **only** our ingest store. Attach tenants **must** work without moving private data into us.

---

## 7. Job families (in-category — this is the product vision)

These must remain first-class. Not every family is V1 ship.

| Family | What the customer wants |
|---|---|
| **Contact agent** | Inbound talking agent: intent, KB/CRM, warm handoff, transcript, disposition (see CC vertical lock) |
| **Meeting pack** | After (or from) a session: summary / MoM / actions, optionally mailed or filed |
| **Grounded copilot** | Text or speech answers from policy + optional hierarchy graph |
| **Captions / one-way translate** | Live or playback voice → text (and optionally other-language text) |
| **Two-way interpret** | Each party hears the other in their language |

Same product. Different profiles. Do not collapse interpret into “chatbot,” or MoM into “call center.”

**Speaker labels (diarization)** are in-category for meeting and QA profiles. They are not required on a captions-only or IVR-like Talk profile.

---

## 8. How the product gets better (locked)

- **Session / customer memory** — facts, not a new personality every week.  
- **Operations loop** — analytics (gaps, failures, language quality, unwanted bias slices) → **versioned** updates to KB, rules, prompts, templates.  
- **Evaluation** — replay golden playbacks after a profile change.  
- **Not default:** automatic training on live conversations. Any such SKU is gated, reversible, and out of V1.

Wanted “bias” = brand and policy (persona + rules).  
Unwanted bias = **measured**, then the profile is corrected.

---

## 9. Extension slots (named now, implementations later)

Do not specify Zoom, Exotel, SMTP, Salesforce, etc. in this lock. Specify **slots**:

| Slot | Examples later (not commitments) |
|---|---|
| **Feeder** | file, websocket, meeting recording, telephony, browser, radio |
| **Sink** | file, websocket, captions overlay, second audio channel |
| **Knowledge** | ingest/dump, **their** search/KB HTTP API, graph import |
| **Skill** | **their** CRM/ticket/txn HTTP APIs, Coral CRM, mail, transfer |
| **Engine** | STT, LLM, TTS (and later VAD, MT) — one vendor gateway per engine |

A new integration is **in spec** when it fills a slot without changing the profile model. If it needs a new mode or a new collision rule, **update this file first**.

---

## 10. V1 vs later (realistic)

### V1 must (platform)

- Named profiles; Listen, Speak, Talk, Think; live and playback where the mode applies  
- LLM on/off; Listen/Speak work with brain off  
- Document KB + at least one **template** type (e.g. summary/MoM or disposition)  
- Rules; refuse/escalate if grounded and no hit  
- Session memory  
- One **skill slot** exercised (action with confirmation + audit) — which vendor/system is a project choice  
- Basic analytics: ran / failed / no-grounding-hit / handoff  
- Captions (Listen → text) live and playback  
- Modular STT / LLM / TTS behind the profile (swap without rewriting the job)  
- Feeder and sink as attachments, not hardcoded to one app  

### V1 should (if capacity)

- Non-technical admin for presets + attach KB/rules  
- One-way translation on captions and/or Speak  
- Playback Think for summary + one action skill (mail or equivalent)  
- PII redaction and disclosure flags on profiles that talk to people  
- Cost/latency counters per turn or per job  

### In-category, not V1-required (committed direction)

- Two-way live interpretation  
- Hierarchy/graph knowledge as a first-class grounding type  
- Full AI Call Center vertical (own lock; consumes this platform)  
- Diarization on meeting profiles  
- Chat as a feeder (same agent bundle)  
- On-prem engine pack  
- Supervisor listen/whisper on Talk  
- Suggested KB updates from analytics (human approve)  

### Out of V1 / not this product

- Owning the meeting or phone network  
- Silent self-learning from production audio  
- Unbounded multi-agent RPA  
- Voice cloning as a platform promise  
- Hybrid “edge voice + cloud brain” as a committed SKU  
- Every codec and every vendor on the live path  

---

## 11. Relationship to AI Call Center

Call Center is a **vertical profile family** on this platform (Talk + Think + playbook + CRM/transfer skills + CC analytics).

`mod_audio_stream-1/docs/AI_Call_Center_Product_Decisions.md` remains source of truth for **that vertical**.  
If the vertical needs a platform capability this file does not allow, **change this file first**.

`mod_audio_stream` is a **feeder/sink** only: give and take audio. OpenAI / Next AI / any engine lives behind this orchestrator’s contracts, not inside that module.

---

## 12. What this file is not

- Not a stack (OpenAI vs Next AI vs Sarvam vs in-house).  
- Not a list of codecs or protocols.  
- Not a committed SOW for a named customer.  
- Not permission to treat Call Center as the only SKU.

If a later design contradicts this file, **update this file first** — do not let a feeder, a vendor, or a demo redefine the product.
