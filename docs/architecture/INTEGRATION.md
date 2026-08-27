# Customer data, knowledge, CRM — and audit

**Status:** LOCKED direction.  
**Date:** 26 August 2026  
**Parents:** product lock, `ARCHITECTURE.md`

---

## 1. Split: what lives in *our* product vs what stays *theirs*

**We own (native, in this application):**

| Object | Why it is ours |
|---|---|
| **Profile** | Modes, clocks, active gateways, rules, language behaviour |
| **Persona / agent** | Voice of the product; versioned with the profile |
| **Skill definitions** | Name, input/output contract, confirmation, authority (inform vs act) |
| **Playbooks / templates** | MoM shape, disposition shape, refuse scripts |
| **Session + audit trail** | What *this* runtime did on a turn |

**They often already own (we must not demand a migration):**

| Object | Reality |
|---|---|
| **KB / FAQ / policy PDFs** | May sit in Confluence, SharePoint, a search box, or nowhere |
| **RAG corpus** | May already be indexed on their side |
| **CRM / RDBMS** (tickets, balances, txn status) | Source of truth is **their** app. A caller asking “where is my payment?” is **their** row, not ours |
| **Call recordings** | Often FreeSWITCH / their archive |

Two tenant shapes, **same product**:

1. **Greenfield** — they have no KB/CRM. They upload into us / we host a store.  
2. **Attach** — they have a working call center and systems. We **integrate**. We do not copy their private estate “so the bot can work.”

---

## 1b. Identity — Coral user management is the directory

**We do not grow a second user database.** Coral already has who the user is, tenant, roles, and details. This service **utilizes** that and can attach more behaviour later (which profile they may run, which skills, language) **on top of** that id — not instead of it.

| Person | Who they are | How this service sees them |
|---|---|---|
| **Operator** (admin, supervisor, agent) | Coral user | Auth/token from Coral; `coral_user_id` + tenant on control API and audit |
| **End customer / caller** | Often not a Coral login | Phone `from` / channel id → **Skill/CRM lookup** if we need “their” details. If they already have a Coral/customer record, we **reference that id**, we do not re-register them here |

**Ours:** profile, persona, session.  
**Coral’s:** users, orgs, roles, passwords, existing user attributes.  
**Bind:** session and profile assignment carry `tenant_id` / `coral_user_id` (and later any extra flags we store **keyed by that id**).

Future features (preferences, allowed gateways, which agent persona) hang off **the same Coral user**, via our tables as *extension*, never as a competing login system.

**Customer memory** (when profile `memory.scope = customer`): stored in orchestrator PG keyed by `tenant_id` + caller/customer id; facts only (not full transcript replay); requires consent flag on profile; TTL per `retention_days`. Session memory stays in-process until Terminal, then summary may merge into customer memory per policy.

Control plane: trust Coral auth (whatever the estate already uses). Orchestrator does not become IdP.

---

## 2. Knowledge (KB / FAQ / RAG) — same port, two gateways

Think never “has the internet.” It asks the **Knowledge router** (payment-gateway style): *retrieve grounding for this utterance.*

| Gateway | When | What we store |
|---|---|---|
| **Dump / ingest** | They agree to give us files or a bulk export | **A copy they chose to publish** into our store (chunked, versioned). Not their live CRM. |
| **Retrieve API** | They already have search/KB | We call **their** HTTP API **this turn**, get snippets, attach to the LLM call. Default: **do not keep the snippets** as our KB. |
| **Hybrid** | Dump for policy + API for live facts | Profile lists both; rules say which intent uses which |

If the profile is **grounded** and **no hit** from the configured knowledge gateways → refuse or escalate. We do not invent from the LLM’s memory of the world.

We still **never** put our only copy of knowledge only inside Next AI. If we ingest, it is **our** store or **their** API, then we **attach** chunks to the Think gateway for that call.

---

## 3. CRM / RDBMS / “status of my transaction”

This is a **Skill** (and sometimes Knowledge for read-only lookup), **not** a database we host.

```
User: “What’s the status of txn 4412?”
  → Think (our persona + rules)
  → Skill: get_transaction_status
  → Skill router → active CRM gateway (THEIR API)
  → We speak the result
```

- Credentials: **tenant-scoped** (their key, their base URL). We are a **client**.  
- **Default integration: HTTP APIs** (what they already expose to apps).  
- **Direct RDBMS** (JDBC to their production DB): **last-resort gateway**, read-only, allowlisted queries, not the product default. Schema coupling and blast radius are why APIs exist.  
- We **do not** become a replica of their CRM. Fetch **for the turn** (and whatever audit requires — see below).  
- Writes (create ticket, update status): **act** + confirmation + audit; same as a payment capture, not a silent LLM.

Coral CRM is **one** CRM gateway (first-party, like TTS-Engine on Speak). A customer’s Salesforce/custom core is **another** gateway on the **same** Skill/Knowledge routers.

---

## 4. What we persist vs what we only pass through

| | Persist in our PG (normal) | Pass through / ephemeral |
|---|---|---|
| Profile, persona, skill *defs* | Yes | — |
| Ingested KB they uploaded | Yes, versioned | — |
| Live API snippets | No, unless audit policy says keep a **redacted** copy | Default: memory for the turn only |
| CRM full records | **No** | Fetch, use, drop |
| Txn status answer | Audit: “skill X, id 4412, result=success, status=settled” — not their whole ledger | Raw JSON optional, retention/PII policy |

PII redaction **before** Think gateway and **before** audit payload if the profile says so.

---

## 5. Audit trail vs logging (both required, different jobs)

**Audit** — compliance / disputes / “why did the bot say that?”

- Append-only store (Postgres).  
- Correlated by **`session_id`** (and turn id).  
- Minimum on each turn: profile+version, clock, gateway ids (Listen/Think/Speak/Translate/Knowledge/Skill **ids**, not secrets), timestamps, STT text (policy), Think: hash or redacted prompt metadata + response (policy), Speak mark/cancel, skill name + args **as allowed** + outcome, barge-in, errors, `recording_ref` when known, intent/slots when playbook active.  
- Immutable: no UPDATE of history; corrections are new events.  
- Retention: tenant policy (days/years), export for their SIEM if they refuse to leave data with us.

**Logging** — operate the service

- Structured logs + **OTel traces** per hop (STT ms, retrieve ms, LLM ms, TTS ms, skill ms).  
- Debug detail, sampling, shorter retention.  
- **Not** the legal record. If it is not in audit, we cannot defend it.

Audio blobs: URI in audit, bytes in object store / their FS — same as playback jobs, not Kafka.

---

## 6b. Warm transfer and screen-pop (contact-center)

Warm transfer is a **Skill** gateway (`coral-transfer`), not FS logic in the composer.

Orchestrator POSTs to Coral (path configured per estate) with: `session_id`, `tenant_id`, `caller`, `intent`, `summary`, `transcript_excerpt`, `recording_ref`, `profile_id`, `version`, `escalation_reason`. Coral agent desktop screen-pop consumes this; FS queue transfer is triggered by Coral/ACD.

Full payload lock: `RULES_AND_SKILLS.md` §2. Post-call disposition and KPI export: `ANALYTICS_AND_POSTCALL.md`.

---

## 6. Product promise to a customer with an existing call center

“We do not take your CRM. We **call** it when the persona is allowed to. We can **host** FAQs you upload, or **query** the KB you already have. Every fetch and every spoken answer has an **audit row**. Engines (STT/LLM/TTS) stay behind routers; **your** systems stay behind Knowledge and Skill routers.”
