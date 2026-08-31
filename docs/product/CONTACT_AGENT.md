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
- Resolved Speak `VoiceID` from persona (map by bound speak gateway, else scalar) — see §7

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

---

## 7. Voice resolution (cc-3)

**Publish:** Talk or Speak profiles must set `persona.voice` (gateway_id → speaker/voice ref map) and/or `persona.voice_id` (scalar). Missing both → `422` `profile_invalid`.

**Runtime (Composer Speak path):**

1. Speak gateway id = session `gateway_binding.speak` when pinned; else profile `routers.speak.providers` Select.
2. `SpeakRequest.VoiceID` = `persona.voice[speak_gateway_id]` if present, else `persona.voice_id` (string `persona.voice` is a scalar alias for `voice_id`).
3. `SpeakRequest.Language` = session `active_language` (cc-2). Gateways interpret `VoiceID` (e.g. Sarvam TTS → Bulbul `speaker`).

**Audit:** `turn.completed` payload includes `voice_id` alongside `gateways.speak` when resolved.

No profile-level Speak vendor override. Voices catalog API is out of scope (see `CONTROL_API.md`).
