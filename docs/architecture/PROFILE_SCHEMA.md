# Profile schema — architecture lock

**Status:** LOCKED (schema shape; field validation evolves with implementation)  
**Date:** 27 August 2026 (CC engines update 31 August 2026)  
**Parents:** `docs/product/PRODUCT_DECISIONS.md`, `SOLUTION.md`, `docs/product/SYSTEM_ENGINES.md`, `docs/product/CONTACT_AGENT.md`

A **profile** is a versioned configuration bundle. A **persona** is a subsection (voice, tone, instructions) — not a separate database entity. Product language “agent” means a **running instance** of a profile on a session, not another object type.

Profiles are stored in PostgreSQL (`profile` + immutable `profile_version` rows). Sessions **pin** one version at create.

### Contact Agent engines (V1)

For `metadata.family: contact-agent`, profiles are **behaviour only**: persona, rules, skills, grounding, language, voice. **Listen / Think / Speak gateway ids inherit tenant system engines** (`docs/product/SYSTEM_ENGINES.md`). Session create persists `gateway_binding`.

- `routers.listen|think|speak.providers` are **optional** on the CC path (deprecated for new CC profiles). If both profile lists and tenant engines are set and differ, **tenant engines win**; publish/session may warn.
- Knowledge / skill / translate routers remain profile-owned.
- **Voice required for Talk/Speak** (persona `voice_id` / voice) — enforced in cc-3; document now so schema readers expect it.

---

## 1. Top-level shape

```yaml
id: contact-agent-inbound
version: 3
tenant_id: acme-corp          # optional; null = platform template

metadata:
  display_name: Inbound contact agent
  family: contact-agent       # contact-agent | meeting | copilot | captions | interpret

audio:
  canonical_sample_rate_hz: 16000   # 8000–48000; see §2
  frame_ms: 20                      # fixed default; frame size derived from rate

clock:
  allowed: [live]                   # live | playback

modes:
  listen: true
  speak: true
  think: true
  talk: true                        # implies composer + VAD when live

language:
  behaviour: none                   # none | captions | one_way | two_way
  primary: en-IN                    # omit/empty when auto_detect (CC default)
  allowed: [en-IN, hi-IN]
  auto_detect: true                 # Contact Agent default true; no forced primary
  mid_call_switch: true             # required for PATCH language.primary

grounding:
  type: document_kb                 # none | document_kb | template | graph | playbook
  required: true                    # if true and no hit → refuse/escalate

brain:
  llm: true

memory:
  scope: session                    # none | session | customer
  customer_consent_required: false
  retention_days: 0

persona:
  name: Priya
  instructions: |
    You are a first-line support agent for Acme. Be concise. Never invent policy.
  # Preferred: map keyed by Speak gateway id (system-bound session gateway_binding.speak)
  voice:
    sarvam-tts: shubh
    fake-speak: lab-voice
  # Optional scalar alias when map miss/absent (also accept persona.voice as a string)
  # voice_id: shubh

routers:
  listen:
    providers: [nextai-stt, backup-stt]
  think:
    providers: [nextai-llm]
  speak:
    providers: [tts-engine, nextai-tts]
  knowledge:
    providers: [ingest-acme-faq, customer-kb-http]
  skill:
    default_timeout_ms: 5000
  translate:                          # only when language.behaviour ≠ none
    providers: [nextai-mt]

knowledge:
  ingest_collections: [acme-faq-v2]
  http_kb:
    gateway: customer-kb-http
    base_url_ref: vault:tenant/acme/kb_url

skills:
  allowed:
    - get_transaction_status
    - warm_transfer
    - create_ticket
  definitions:
    get_transaction_status:
      gateway: customer-crm-http
      authority: inform             # inform | decide | act
      confirm: false
    warm_transfer:
      gateway: coral-transfer
      authority: act
      confirm: false
    create_ticket:
      gateway: coral-crm
      authority: act
      confirm: true

rules:
  - id: disclosure-greeting
    phase: pre_speak_first
    action: inject_text
    text: "This call may be recorded and uses AI assistance."
  - id: no-invent-policy
    phase: pre_think
    when: { grounding_required: true, knowledge_hit: false }
    action: escalate
    skill: warm_transfer
  - id: block-card-numbers
    phase: pre_think
    when: { regex: '\d{12,19}' }
    action: refuse
    message: "Please do not share full card numbers on this line."

playbook:                             # when grounding.type = playbook
  entry: greet
  states:
    greet:
      on_intent: route_intent
      fallback: clarify
    route_intent:
      slots: [intent, account_id]
      on_complete: fulfill_or_escalate

templates:
  mom:
    id: meeting-summary-v1
  disposition:
    id: cc-disposition-v1

fallback:
  listen_down: { speak_canned: clip-apology-en, skill: warm_transfer }
  think_down: { speak_canned: clip-escalate-en, skill: warm_transfer }
  speak_down: { text_sink: true, skill: warm_transfer }

analytics:
  emit: [containment, handoff, no_grounding_hit, latency_hops]

hot_swap_allowed:                     # mid-session profile patches
  - language.primary
  - language.allowed
  - skills.allowed   # unlock after verify only via explicit control API
```

---

## 2. Audio sample rate (locked)

| Field | Rule |
|---|---|
| `audio.canonical_sample_rate_hz` | Integer **8000–48000** (inclusive). Common values: 8000, 11025, 16000, 22050, 24000, 32000, 44100, 48000. |
| Session | Pins rate from profile at create. Override only via control API if within profile `allowed_rates` (optional list; default = single rate). |
| Session bus | All in-process PCM at the session’s canonical rate. |
| Frame size | `frame_bytes = canonical_sample_rate_hz × 2 × (frame_ms / 1000)` (mono s16le). Default `frame_ms = 20`. |
| Edge / gateways | Resample to/from peer rate inside the edge or gateway. Declare supported rates in gateway capabilities. |
| Live validation | At session start: feeder declared rate, sink peer rate, and active Speak/Listen gateways must support resampling to/from canonical (or session create fails). |

PSTN paths often use **8 kHz**; wideband and meeting paths use **16 kHz** or higher. The profile chooses canonical rate; edges adapt — we do not force one global rate.

---

## 3. Versioning rules

| Event | Behaviour |
|---|---|
| Publish new version | Insert immutable `profile_version` row; bump version number. |
| Running session | Keeps pinned version until stop. |
| New session | Resolves `latest` or explicit version at create. |
| Invalid profile | Session create returns `422` with validation errors; never starts Running. |
| Hot swap | Only fields listed in `hot_swap_allowed`; applied via `PATCH /sessions/{id}/profile-fields`. |

---

## 4. Validation (minimum)

- At least one mode on.  
- If `talk` and `clock=live`: Listen + Speak required; Think required unless profile is Speak-only script.  
- If `grounding.required`: at least one Knowledge provider.  
- If `language.behaviour` ≠ `none`: Translate router providers required.  
- Each skill in `skills.allowed` must have a `definitions` entry.  
- Gateway ids in routers must exist in the process registry (or tenant gateway table).  
- Live clock: every selected Listen/Speak provider must advertise `streaming`.
- **`family: contact-agent`:** do **not** require profile-level `routers.listen|think|speak.providers` when tenant engines are configured (env or `tenant_engines` row). Warn if profile lists conflict with tenant engines.
- **Contact Agent language:** default `language.auto_detect: true`; do not force `primary` when auto-detect is on (`docs/product/LANGUAGE_POLICY.md`).
- **Talk / Speak voice:** if `modes.talk` or `modes.speak` is true, require non-empty `persona.voice_id` **or** non-empty `persona.voice` map (at least one gateway→speaker entry). Listen-only / captions-style profiles may omit persona voice. Publish returns `422` + `profile_invalid` when missing. Do **not** require a map key for every possible Speak gateway at publish (tenant Speak may be unknown). Runtime resolution: `persona.voice[gateway_binding.speak]` then scalar `voice_id` (see `CONTACT_AGENT.md`).

---

## 5. Relationship to Coral config

**Phase 1:** profiles live in orchestrator PostgreSQL; API CRUD + optional YAML import.  
**Later:** `com.coraltele.config` may **distribute** the same schema; orchestrator remains authoritative for **runtime** profile versions and session pins.

Admin GUI (in-category product) reads/writes this schema via control API — not a second profile format.
