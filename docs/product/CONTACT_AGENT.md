# Contact Agent (call-center vertical)

**Status:** LOCKED (product vertical for `contact-agent-cc` pipeline)  
**Date:** 31 August 2026  
**Parents:** `PRODUCT_DECISIONS.md`  
**Related:** `SYSTEM_ENGINES.md`, `LANGUAGE_POLICY.md`

---

## 1. What this vertical is

Inbound (and later outbound) **talking agent** on the Speech-and-Agent Platform:

- Intent + grounded answers (KB / skills)
- Warm handoff / escalate to human
- Transcript + disposition after the call
- Multi-profile per org (inbound, collections, after-hours, …) — same tenant engines

It is a **profile family** (`metadata.family: contact-agent`), not a separate product or media kernel.

---

## 2. Locked seams (phased)

| Concern | Lock | Phase |
|---|---|---|
| System / tenant engines | One Listen/Think/Speak per tenant; session `gateway_binding` pin; no mid-session vendor hop | cc-1 |
| Language | Auto-detect then lock; operator/user switch only | cc-2 → `LANGUAGE_POLICY.md` |
| Voice | Persona voice required for Talk/Speak; bound to Speak gateway | cc-3 |
| Response ladder | clip → template → LLM; vendor fail → clip + escalate | cc-4 |
| Transcript / disposition | Durable transcript APIs + disposition path | cc-5 |
| Lab validation | Lab surfaces for engines / language / ladder | cc-6 |

---

## 3. Session context (V1)

At create, a Contact Agent session carries at least:

- Pinned profile version (behaviour, persona, skills, grounding)
- `gateway_binding` from tenant engines (`SYSTEM_ENGINES.md`)
- Clock (`live` for Talk)
- Caller / metadata as supplied by Coral
- `detected_language` / `active_language` (empty until first confident Listen lock or operator PATCH — see `LANGUAGE_POLICY.md`)

Voice resolution lands in **cc-3**.

---

## 4. Multi-profile org

One Coral **tenant** may publish many Contact Agent profiles. All inherit the **same** tenant engine binding. Profiles differ by persona, rules, skills, KB collections, and ladder clips — not by STT/LLM/TTS vendor.

---

## 5. CRM / Coral boundary

- Coral owns identity, ACD/transfer, agent desktop disposition override.
- Orchestrator owns session runtime, transcript buffer, AI disposition **suggestion**, audit/analytics.
- Skills call **their** CRM/ticket HTTP APIs; we do not become a second CRM.

---

## 6. Profiles = behaviour only (V1)

For `family: contact-agent`:

- Engines inherit system/tenant (`SYSTEM_ENGINES.md`)
- `routers.listen|think|speak.providers` are optional / deprecated on the CC path
- Voice and language fields remain on the profile (see cc-2 / cc-3)
