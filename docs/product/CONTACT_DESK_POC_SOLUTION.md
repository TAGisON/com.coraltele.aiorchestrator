# Contact Desk Platform — Complete Industrial Solution Architecture

**Status:** INDUSTRIAL ARCHITECTURE — agent-ready definition  
**Date:** 1 September 2026  
**Parents:** `PRODUCT_DECISIONS.md`, `CONTACT_AGENT.md`, `SYSTEM_ENGINES.md`, `LANGUAGE_POLICY.md`, Coral Telecom TFN script  
**Architecture parents:** `PORTS.md`, `RUNTIME.md`, `RULES_AND_SKILLS.md`, `PROFILE_SCHEMA.md`, `CONTROL_API.md`, `INTEGRATION.md`, `ANALYTICS_AND_POSTCALL.md`, `EDGE_FS.md`, `OPERATIONS.md`  
**Audience:** Product, architects, implementers, AI coding agents  

**How to use this file:** An implementer or agent must be able to build from **this document alone** for the Contact Desk vertical, without inventing entities, vocabularies, or flows. Platform kernel details remain in architecture parents; this file defines the **vertical completely**.

**Hard rule:** Build the **platform Contact Desk vertical** so Coral and future desks/CRMs fit without redesign. Do **not** patch `coral-cc` regex/ladder into this as the industrial desk.

---

## 0. Framing (locked)

| Statement | Lock |
|---|---|
| What we are building | **Industrial Contact Desk vertical** on the Speech-and-Agent Platform |
| What Coral TFN is | **First desk workload** (preset content), not a throwaway POC architecture |
| What we are not building | A second media kernel, a PBX/ACD, a CRM SoR, or disposable “demo-only” schemas |
| Desk vs Profile | **Desk = CC authoring/ops surface that publishes into `metadata.family: contact-agent` profile.** Runtime pins **`profile_version`** (+ optional `desk_id` / `desk_version` metadata). No parallel session brain. |
| Delivery | **Phased** (W0–W5). Compliance **seams** exist from W0; legal program depth phases in |
| Configurator UX | **GUI-only** for desk ops (forms, not JSON/regex/system prompts). Platform **API** remains for automation |
| Concurrency | `max_concurrent_sessions` in **tenant properties/ops**, not desk script GUI |

**Review history**

| Round | Lens | Outcome |
|---|---|---|
| 0–3 | Bot builder / ITSM / voice PII | Desk model, GUI, attributes, skills |
| 4 | Industrial + compliance-by-design | Not POC architecture; Coral = workload |
| 5 | All-rounder | Conditional approve; Desk≡profile, NFRs, thin W0 |
| 6 | Completeness vs agent-ready definition | Approved for build planning (§24) |
| **7** | **Final implementation-readiness sweep** | **Gaps closed (persistence, step schema, repair policy, thresholds, idempotency, schemas, authz, metrics, outbound boundary, tests) — see §25** |

---

## 1. Goal (what “done” means)

1. **Inbound** Coral TFN desk: Sales / Product / Tech / Complaint (script §1–22), multilingual India (EN+HI first), barge-in, silence ladder, short prompts, grounded KB, skills with **real contracts** (stubs OK until live connectors).  
2. **Outbound** desk **type** in GUI early; stub-runnable by W4.  
3. **GUI-only** day-to-day configuration for Desk Configurator.  
4. Full **audit** of decisions, attributes, skill results, PII reveals.  
5. LLM = dialog brain **inside** guarded states — never invents ticket id, email send, or transfer success.  
6. New CRM/ACD = **new skill connector**, same desk Actions GUI — **no desk redesign**.

Success = **script-complete CX + contracts + operable GUI + compliance seams**, not “mic works.”

---

## 2. Architecture laws (closed — do not violate)

1. **Go kernel; in-memory PCM; no Kafka/Redis for audio.**  
2. **Vendors behind ports/routers;** fakes before live vendor depth.  
3. **Rules > skills > grounding > free LLM.**  
4. **Acting requires skill + authority + audit.** Informing must not silently become acting.  
5. **Playbooks speak business actions; connectors speak HTTP/SOAP.** Desks never import Salesforce/Zendesk types.  
6. **One media WebSocket per live session.** Sticky to owning instance.  
7. **Tenant engines pin** Listen/Think/Speak for Contact Agent; no mid-session vendor hop.  
8. **Publish → immutable version;** live sessions keep pinned version.  
9. **Ticket ids only from skill results** — never LLM text.  
10. **Desk publishes Profile** — Desk is not a second runtime.  
11. **Compliance seams in schema from W0** — depth phases; model does not redesign later.  
12. **Configurator never edits** system prompts, regex, or raw profile JSON.

---

## 3. Layer stack

```text
┌─────────────────────────────────────────────────────────────┐
│  Consoles: Admin (Platform + Desk library) · Supervisor · User │
├─────────────────────────────────────────────────────────────┤
│  Control API (sessions, profiles/desks, engines, KB, erasure) │
├─────────────────────────────────────────────────────────────┤
│  Contact Desk vertical                                        │
│    Desk draft → Publish → profile_version + desk_version meta │
│    Guided path · attributes · disposition · CX policy         │
│    Skill binds · routing matrix · prompt library              │
├─────────────────────────────────────────────────────────────┤
│  Platform runtime (session actor + composer + think path)     │
│    Listen · Think · Speak · Knowledge · Skill routers         │
├─────────────────────────────────────────────────────────────┤
│  Edge (mod_audio_stream / lab WS) · Gateways · Skill connectors│
└─────────────────────────────────────────────────────────────┘
```

Ownership:

| Layer | Owner |
|---|---|
| Consoles / Desk GUI | This product |
| Session media + Think path | This product |
| Skill **contracts** + stubs | This product |
| Skill **live connectors** | This product (per CRM) or customer Dev |
| Identity / ACD / queue transfer execution | Coral |
| CRM / ticket SoR | Coral or customer system |
| SIP/RTP | FreeSWITCH + `mod_audio_stream` |

---

## 4. Entity catalog

Every durable or session object. **Do not collapse** these.

### 4.1 Classification legend

| Class | Meaning |
|---|---|
| `PLATFORM` | Speech-and-Agent Platform core |
| `VERTICAL` | Contact Desk vertical |
| `WORKLOAD` | Coral (or other) content on vertical |
| `EXTERNAL` | Coral/CRM/FS outside our SoR |
| `OPS` | Ops/properties, not Configurator script |

### 4.2 Entities

| Entity | Class | Definition | Lifetime | Pin / key |
|---|---|---|---|---|
| **Tenant** | PLATFORM | Org boundary | Durable | `tenant_id` |
| **Tenant engines** | PLATFORM | One Listen + Think + Speak id | Durable until PUT | `tenant_id` |
| **Credential** | PLATFORM | Secret per gateway | Durable | `tenant_id` + `gateway_id` |
| **Tenant properties** | OPS | Incl. `max_concurrent_sessions`, admission mode | Durable | `tenant_id` |
| **Retention policy** | VERTICAL | Days for transcript / attributes / audit | Durable | tenant default and/or desk override |
| **Skill definition** | PLATFORM/VERTICAL | Name, schema, authority, timeout, gateway | Catalog | `skill_name` |
| **Skill bind** | VERTICAL | Desk enables skill + connector config | Per desk version | desk + skill |
| **Knowledge pack** | PLATFORM | Documents / collection | Durable | `collection` |
| **Prompt asset** | VERTICAL | Locale text and/or WAV; barge flag | Versioned with desk | `prompt_id` + locale |
| **Intent** | VERTICAL | Named caller goal + example phrases | In desk version | `intent_id` |
| **Guided path** | VERTICAL | Ordered steps for an intent (or sub-tree) | In desk version | `path_id` |
| **Path step** | VERTICAL | Say / Ask / Confirm / Choice / Action / End | In path | `step_id` |
| **Routing matrix row** | VERTICAL | Intent → owner/queue → action mode | In desk version | intent |
| **CX policy** | VERTICAL | Barge-in, silence, listen-while-speak | In desk version | desk |
| **Desk** | VERTICAL | Authoring object: inbound\|outbound + purpose | Draft/published | `desk_id` |
| **Desk draft** | VERTICAL | Mutable config | Until publish | `desk_id` |
| **Desk version** | VERTICAL | Immutable snapshot metadata + content hash | Forever | `desk_id` + `desk_version` |
| **Profile** | PLATFORM | Runtime behaviour document | Durable | `profile_id` |
| **Profile version** | PLATFORM | Immutable publish of profile | Forever | `profile_id` + `profile_version` |
| **Session** | PLATFORM | One call/job | Created→Terminal | `session_id` |
| **gateway_binding** | PLATFORM | Pinned listen/think/speak | Session life | on session |
| **Attachment** | PLATFORM | Feeder and/or sink | ⊆ session | |
| **Contact attributes** | VERTICAL | Structured call state bag | Session; freeze on Terminal | JSONB |
| **Disposition** | VERTICAL/PLATFORM | Closed end code (+ AI suggestion vs Coral override) | Terminal/postcall | |
| **Evidence pack** | VERTICAL | session + versions + disposition + attribute keys | Postcall | |
| **Audit event** | PLATFORM | Turn / skill / PII reveal / publish | Durable per retention | |
| **Transcript turn** | PLATFORM | user\|assistant\|system text | Durable per retention | |
| **Connector** | VERTICAL | Implementation of a skill contract | Process registry | `gateway_id` |

**Persona** is a **subsection of profile**, not a separate entity.  
**Agent** = running instance of a published desk/profile on a session.

### 4.3 Desk ↔ Profile mapping (mandatory)

On **Publish**:

1. Validate publish checklist (§16).  
2. Compile GUI model → `profile.Document` with `metadata.family: contact-agent`.  
3. Create immutable `profile_version`.  
4. Record `desk_version` pointing at that profile version (same content).  
5. Runtime `POST /v1/sessions` pins **`profile_id` + `profile_version`**; stores `desk_id` / `desk_version` in session metadata and contact attributes.

| Desk GUI area | Profile / runtime target |
|---|---|
| Overview (name, direction, languages, voice, purpose) | `metadata`, `language`, `persona.voice*`, desk meta `direction`/`purpose` |
| Prompts | `response.clips` / prompt library refs / fallback clips |
| Intents + phrases | playbook intents + NLU examples (not regex UX) |
| Conversations (guided path) | `playbook.states` / path compiler output; `grounding.type=playbook` |
| Knowledge attach | `routers.knowledge` + collection binds |
| Actions / matrix | `skills.allowed` + definitions + matrix in playbook memory schema |
| Call behaviour | CX policy → composer flags (barge, silence, listen-while-speak) |
| System laws | Fixed rules pack (dev-owned) + `authority` |

### 4.4 Persistence model (Postgres — normative)

**Storage law:** desk draft and desk version are **single JSONB documents**; only fields we query or index become columns. Do not explode the GUI model into 15 relational tables.

| Table | Key | Columns (minimum) | Notes |
|---|---|---|---|
| `desk` | `id` | `tenant_id`, `name`, `direction`, `purpose`, `status`, `current_version`, `created_at`, `updated_at`, `created_by` | One row per desk |
| `desk_draft` | `desk_id` | `doc jsonb`, `schema_version`, `updated_by`, `updated_at` | Mutable authoring doc |
| `desk_version` | (`desk_id`,`version`) | `doc jsonb`, `content_hash`, `profile_id`, `profile_version`, `published_by`, `published_at`, `checklist jsonb` | Immutable |
| `attribute_catalog` | `key` | `label`, `value_type`, `pii_class`, `settable_by` | Drives masking + GUI labels |
| `session_attributes` | `session_id` | `attrs jsonb`, `frozen_at` | Freeze on Terminal |
| `session_disposition` | `session_id` | `suggestion`, `final_code`, `set_by`, `set_at`, `template_id` | Suggestion + ACW final |
| `skill_invocation` | `id` | `session_id`, `step_id`, `skill`, `idempotency_key`, `args_redacted jsonb`, `outcome`, `external_ref`, `latency_ms`, `at` | One row per attempt |
| `tenant_properties` | `tenant_id` | `max_concurrent_sessions`, `admission_mode`, `updated_at` | Ops, not desk GUI |
| `retention_policy` | (`tenant_id`,`desk_id` nullable) | `transcript_days`, `attributes_days`, `audit_days`, `recording_days`, `updated_at` | Desk row overrides tenant |
| `pii_access_audit` | `id` | `session_id`, `actor`, `keys[]`, `reason`, `at` | Every reveal |
| `erasure_request` | `id` | `tenant_id`, `session_id`, `requested_by`, `reason`, `state`, `executed_at` | `state`: `requested`\|`executed`\|`rejected` |
| `consent_record` | `id` | `tenant_id`, `ani`, `purpose`, `decision`, `source`, `at` | Outbound scrub evidence |

**Session table additions:** `desk_id`, `desk_version`, `purpose`, `direction`, `consent_ref` (nullable). Existing `profile_id`, `profile_version`, `gateway_binding`, state, timestamps stay platform-owned.

**Existing platform tables reused unchanged:** `tenant_engines`, credentials/settings, `transcript_turn`, audit/analytics event tables, KB tables.

### 4.5 Schema versioning and compatibility

| Rule | Statement |
|---|---|
| `schema_version` | Every desk doc carries it; compiler must accept current and **N-1** |
| Published versions | Immutable; never rewritten by a later compiler |
| Breaking desk schema change | New `schema_version` + migration of **drafts only**; published versions remain replayable as stored profile versions |
| Profile compile output | Must validate against `PROFILE_SCHEMA.md` before the version row is written; failure → `422 profile_invalid`, no partial publish |
| Content hash | `content_hash` over normalized doc; identical republish returns the existing version (no version churn) |

---

## 5. Function catalog

Every architectural function the vertical needs. Agents implement by **name + class**.

### 5.1 Function classes

| Class | Code | Who invokes |
|---|---|---|
| Control-plane API | `CTRL` | Consoles, Coral, automation |
| Desk authoring | `AUTH` | Desk Configurator GUI |
| Compile / publish | `PUB` | Publish pipeline |
| Session lifecycle | `SESS` | Control + runtime |
| Media / Talk | `MEDIA` | Composer / edge |
| Dialog / path | `DIAL` | Desk controller on Think path |
| NLU / LLM | `NLU` | Think gateway (guarded) |
| Skill execution | `SKILL` | Skill router |
| Connector I/O | `CONN` | Skill gateway implementation |
| Compliance | `COMP` | Policy + API + jobs |
| Postcall / ACW | `POST` | Worker + Supervisor |
| Ops / capacity | `OPS` | Properties + admission |
| Observability | `OBS` | Audit / analytics / SSE |

### 5.2 Complete function list

#### A. Tenant / platform setup (`CTRL` / `OPS`)

| ID | Function | Class | Behaviour |
|---|---|---|---|
| F-T01 | `PutTenantEngines` | CTRL | Set listen/think/speak ids; capability-check |
| F-T02 | `GetTenantEngines` | CTRL | 404 if never set |
| F-T03 | `PutGatewayCredential` | CTRL | Store secret; never return raw secret |
| F-T04 | `PutTenantProperties` | OPS | Incl. `max_concurrent_sessions`, `admission_mode` |
| F-T05 | `GetTenantProperties` | OPS | Read concurrency + retention defaults |
| F-T06 | `PutRetentionPolicy` | COMP | transcript_days, attributes_days, audit_days |
| F-T07 | `ListSkillCatalog` | CTRL | Tenant-visible skill definitions |
| F-T08 | `UpsertKnowledgeDocument` | CTRL | KB ingest |
| F-T09 | `ListKnowledgePacks` | CTRL | Collections |

#### B. Desk authoring (`AUTH`)

| ID | Function | Class | Behaviour |
|---|---|---|---|
| F-D01 | `CreateDesk` | AUTH | direction inbound\|outbound; purpose; name |
| F-D02 | `ListDesks` | AUTH | Tenant desks + status |
| F-D03 | `GetDeskDraft` | AUTH | Full editable model |
| F-D04 | `UpdateDeskOverview` | AUTH | name, languages, voice, purpose, direction |
| F-D05 | `UpsertPrompt` | AUTH | locale tabs; Text\|WAV; barge checkbox |
| F-D06 | `UpsertIntent` | AUTH | display name, example phrases, active |
| F-D07 | `UpsertGuidedPath` | AUTH | ordered steps for intent/sub-tree |
| F-D08 | `UpsertPathStep` | AUTH | Say/Ask/Confirm/Choice/Action/End |
| F-D09 | `AttachKnowledgePack` | AUTH | pack → intent/topic |
| F-D10 | `BindSkill` | AUTH | enable + connector config fields |
| F-D11 | `UpsertRoutingMatrix` | AUTH | intent → owner/queue → Transfer\|Ticket\|Both |
| F-D12 | `UpdateCXPolicy` | AUTH | barge, silence timeouts, listen-while-speak |
| F-D13 | `LoadWorkloadPreset` | AUTH | e.g. Coral inbound TFN into draft |
| F-D14 | `CloneDesk` | AUTH | Copy draft from another desk |
| F-D15 | `ExportDesk` | CTRL | Dev/CI JSON (Developer toggle) |
| F-D16 | `ImportDesk` | CTRL | Dev/CI only |

#### C. Publish (`PUB`)

| ID | Function | Class | Behaviour |
|---|---|---|---|
| F-P01 | `ValidatePublishChecklist` | PUB | Returns green/red items (§16) |
| F-P02 | `CompileDeskToProfile` | PUB | GUI → profile.Document |
| F-P03 | `PublishDesk` | PUB | Immutable desk_version + profile_version |
| F-P04 | `UnpublishDesk` | PUB | Stop new sessions; live keep pin |
| F-P05 | `GetDeskVersion` | PUB | Immutable snapshot |
| F-P06 | `ListDeskVersions` | PUB | History |

#### D. Session lifecycle (`SESS` / `OPS`)

| ID | Function | Class | Behaviour |
|---|---|---|---|
| F-S01 | `AdmitSession` | OPS | If concurrent ≥ max → reject `429` or queue per `admission_mode` |
| F-S02 | `CreateSession` | SESS | Pin profile_version, gateway_binding, purpose, desk meta, ANI |
| F-S03 | `AttachEdge` | SESS | Feeder/sink WS; one duplex media WS |
| F-S04 | `AnswerSession` | SESS | Speak welcome (no user turn) |
| F-S05 | `InjectText` | SESS | Lab/simulator text turn |
| F-S06 | `PatchHotFields` | SESS | Only `hot_swap_allowed` (e.g. language) |
| F-S07 | `StopSession` | SESS | Draining → Terminal |
| F-S08 | `GetSession` | SESS | Incl. binding, desk meta, state |
| F-S09 | `StreamSessionEvents` | OBS | SSE: caption, turn, skill, state, error |

#### E. Media / Talk (`MEDIA`)

| ID | Function | Class | Behaviour |
|---|---|---|---|
| F-M01 | `IngestPCM` | MEDIA | Feeder frames → Listen |
| F-M02 | `ListenFinal` | MEDIA | STT final → dialog |
| F-M03 | `SpeakTextOrClip` | MEDIA | TTS or WAV prompt |
| F-M04 | `BargeIn` | MEDIA | Flush sink; cancel Speak/Think; do not restart full welcome |
| F-M05 | `ListenWhileSpeakGate` | MEDIA | Ignore speaker echo while TTS (CX On) |
| F-M06 | `SilenceWatch` | MEDIA | Nudge1 → Nudge2 → goodbye → disposition `abandoned_silence` |
| F-M07 | `LanguageLock` | MEDIA | First confident Listen; ambient ignore; PATCH to switch |
| F-M08 | `ResolveVoice` | MEDIA | persona.voice[speak_gateway] else voice_id |

#### F. Dialog / path (`DIAL` / `NLU`)

| ID | Function | Class | Behaviour |
|---|---|---|---|
| F-A01 | `DeskControllerTurn` | DIAL | Orchestrate CX → path → NLU → Speak/skill |
| F-A02 | `ClassifyIntent` | NLU | Example phrases + LLM extract inside playbook |
| F-A03 | `AdvancePathStep` | DIAL | Move FSM per step type |
| F-A04 | `FillSlot` | DIAL | Ask → attribute; allow mid-path correction via NLU |
| F-A05 | `ConfirmSummary` | DIAL | Speak summary; accept/correct |
| F-A06 | `BranchChoice` | DIAL | Transfer vs ticket vs continue |
| F-A07 | `ApplySystemLaws` | DIAL | Fixed rules: no invent ticket/email/transfer |
| F-A08 | `ResponseLadder` | DIAL | clip → template → llm (cc-4) |
| F-A09 | `GroundedAnswer` | NLU | Knowledge retrieve; miss → refuse/escalate if required |
| F-A10 | `UpdateContactAttributes` | DIAL | Write attribute bag; never invent ticket_id |
| F-A11 | `BuildHandoffPack` | DIAL | Attributes + summary + transcript excerpt |
| F-A12 | `SimulateTurn` | AUTH | Desk simulator without PSTN |

#### G. Skills (`SKILL` / `CONN`)

| ID | Function | Class | Behaviour |
|---|---|---|---|
| F-K01 | `ExecuteSkill` | SKILL | Validate allowlist, authority, confirm, schema, pre_skill |
| F-K02 | `ResolveCaller` | CONN | Lookup identity (stub/live) |
| F-K03 | `TransferToQueue` | CONN | POST handoff to Coral; drain session |
| F-K04 | `FindOpenComplaint` | CONN | Duplicate detect |
| F-K05 | `CreateServiceComplaint` | CONN | Create ticket; return real id or fail |
| F-K06 | `SendComplaintEmail` | CONN | May fail independently of ticket |
| F-K07 | `RegisterSalesEnquiry` | CONN | Callback / enquiry when Sales unavailable |
| F-K08 | `ScheduleCallback` | CONN | Outbound/callback stub |
| F-K09 | `SearchKnowledge` | CONN/SKILL | Explicit KB skill if used |
| F-K10 | `PushDisposition` | CONN | Postcall suggestion to Coral |
| F-K11 | `ScrubOutboundConsent` | CONN | DND/consent check before dial |

#### H. Compliance (`COMP`)

| ID | Function | Class | Behaviour |
|---|---|---|---|
| F-C01 | `SetSessionPurpose` | COMP | Required at create from desk |
| F-C02 | `ClassifyAttributePII` | COMP | Mask default; role reveal |
| F-C03 | `AuditPIIReveal` | COMP | Every unmask → audit row |
| F-C04 | `BuildEvidencePack` | COMP | Bind session evidence keys |
| F-C05 | `RequestErasure` | COMP | Delete-by-session API (contract W0) |
| F-C06 | `RunRetentionSweeper` | COMP | Delete past retention (live by W5) |
| F-C07 | `EnforceRecordingPolicy` | COMP | record yes/no + class (media later) |
| F-C08 | `CheckOutboundConsent` | COMP | Before outbound dial |

#### I. Postcall / ACW (`POST`)

| ID | Function | Class | Behaviour |
|---|---|---|---|
| F-O01 | `FreezeAttributes` | POST | Immutable after Terminal |
| F-O02 | `SuggestDisposition` | POST | AI suggestion from template |
| F-O03 | `SetDisposition` | POST | Closed code (system or Supervisor) |
| F-O04 | `UpsertSessionDisposition` | POST | Persist suggestion + final |
| F-O05 | `GetTranscript` | POST | Durable turns |
| F-O06 | `GetDisposition` | POST | API |
| F-O07 | `SupervisorACWCard` | POST | Attributes + disposition UI |

#### J. Observability (`OBS`)

| ID | Function | Class | Behaviour |
|---|---|---|---|
| F-V01 | `AuditTurn` | OBS | profile/version, gateways, tier, text policy |
| F-V02 | `AuditSkill` | OBS | name, args (allowed), outcome |
| F-V03 | `AuditPublish` | OBS | who published which version |
| F-V04 | `EmitAnalytics` | OBS | turn_completed dimensions incl. response_tier |

### 5.3 Critical function contracts (inputs / outputs / errors)

Every function below is **normative**. Others follow the same convention: validate → act → audit.

| Function | Input | Output | Errors | Idempotent? | Audit |
|---|---|---|---|---|---|
| `AdmitSession` | tenant_id | slot reserved \| rejected | `429 rate_limited` + `retry_after` | Yes (reservation id) | counter metric only |
| `CreateSession` | tenant_id, desk_id \| (profile_id+version), purpose, direction, caller/ANI, consent_ref? | session_id, profile pin, gateway_binding, desk meta | `422 profile_invalid` (engines/voice/desk unpublished), `429`, `404 not_found` | No | `session.created` |
| `PublishDesk` | desk_id, actor | desk_version, profile_version | `409 conflict` (checklist red), `422 profile_invalid` | Yes via `content_hash` | `desk.published` |
| `CompileDeskToProfile` | desk doc | profile.Document | `422 profile_invalid` + field path | Yes (pure) | none (part of publish) |
| `ExecuteSkill` | session_id, step_id, skill, args | result + branch (`ok\|fail\|duplicate\|timeout\|unavailable`) | port codes mapped to branch | **Only with idempotency key** (§9.1) | `skill.completed` |
| `UpdateContactAttributes` | session_id, key/value map | new attrs | `409 conflict` if frozen | Yes | `attributes.updated` |
| `RevealAttributes` | session_id, keys, actor, reason | unmasked values | `403 forbidden` | Yes | `pii.revealed` (mandatory) |
| `SetDisposition` | session_id, closed code, actor | stored | `422 bad_request` (code not in §6.6) | Yes | `disposition.set` |
| `RequestErasure` | session_id, requester, reason | request id + state | `404`, `409` (legal hold) | Yes | `erasure.requested` |
| `RunRetentionSweeper` | policy | deleted counts | — | Yes | `retention.swept` |
| `SimulateTurn` | desk draft or version, text turn | intent, slots, next step, would-call skill | `422` invalid draft | Yes | `simulator.turn` (no session audit) |
| `CheckOutboundConsent` | tenant, ANI, purpose | `allow` \| `deny` | `unavailable` → treat as **deny** | Yes | `consent.checked` |

**Fail-closed rules:** consent check unavailable → deny. PII reveal without reason → reject. Publish with red checklist → reject (no “force publish” in V1).

---

## 6. Closed vocabularies

Agents must not invent new enum values without a doc change.

### 6.1 Desk / session

| Field | Values |
|---|---|
| `direction` | `inbound` \| `outbound` |
| `desk.status` | `draft` \| `published` \| `unpublished` |
| `purpose` | `support` \| `sales` \| `survey` \| `collections` \| `other` |
| `admission_mode` | `reject` \| `queue` (V1 implement `reject`; `queue` reserved) |
| Session state | `Created` → `Attached` → `Running` → `Draining` → Terminal(`Completed`\|`Cancelled`\|`Failed`) |
| Clock | `live` (Talk desks) \| `playback` |
| Family | `contact-agent` |

### 6.2 Path step types

| Type | Meaning | Next |
|---|---|---|
| `Say` | Speak prompt/text; optional barge | Next step |
| `Ask` | Collect slot into attribute | Next / re-ask |
| `Confirm` | Speak summary; yes → next; no → correction | Branch |
| `Choice` | Enumerated options | Branch by option |
| `Action` | Run skill; branch success/fail/duplicate | Branch |
| `End` | Closing + optional “anything else?” | Terminal path |

### 6.3 Intent ids (Coral inbound preset)

`sales_enquiry` · `product_information` · `technical_support` · `service_complaint`  
(Plus runtime: `critical_outage`, `unclear` as system paths.)

### 6.4 Product catalog (Coral)

`ip_phone` · `media_gateway` · `call_server` · `call_center` · `cloud_box` · `vms` · `other`

### 6.5 Contact attributes (core bag)

| Key | Set by | PII class |
|---|---|---|
| `direction` | session create | none |
| `desk_id` / `desk_version` / `profile_version` | create | none |
| `purpose` | create | none |
| `language` / `active_language` | language lock | none |
| `ani` / `caller` | create / edge | confidential |
| `intent` | classify | none |
| `product` | Ask | none |
| `problem` | Ask | none |
| `impact` | Ask | none |
| `error_alarm` | Ask | none |
| `troubleshoot_notes` | Ask / NLU | internal |
| `customer_name` | Ask | confidential |
| `customer_email` | Ask | confidential |
| `customer_phone` | Ask | confidential |
| `ticket_id` | **skill only** | internal |
| `email_sent` | skill | none |
| `priority` | path / critical | none |
| `transfer_target` | matrix | none |
| `disposition` | terminal | none |
| `summary` | Confirm / Think | internal |
| `consent` (outbound) | create / scrub | confidential |

### 6.6 Disposition codes (ACW — closed)

`resolved_self_service` · `transferred_sales` · `transferred_tech` · `transferred_complaint_queue` · `ticket_created` · `ticket_failed_offered_transfer` · `callback_scheduled` · `duplicate_ticket_found` · `critical_escalated` · `abandoned_silence` · `abandoned_ws_drop` · `unresolved`

Platform postcall **suggestion** coarse set remains: `resolved` \| `unresolved` \| `escalated` (map from fine codes).

### 6.7 Authority / rules / ladder

- Authority: `inform` \| `decide` \| `act`  
- Rule phases: `pre_listen` \| `pre_think` \| `post_think` \| `pre_speak_first` \| `pre_skill`  
- Rule actions: `allow` \| `refuse` \| `escalate` \| `inject_text` \| `block_think` \| `strip_response`  
- Ladder tiers / analytics: `clip` \| `template` \| `llm` \| `refuse` \| `escalate`  
- Prompt media: `text_tts` \| `wav`  
- Tone (GUI): `professional` \| `warm`  
- Matrix action: `transfer` \| `ticket` \| `both`  
- Skill result branch: `ok` \| `fail` \| `duplicate` \| `timeout` \| `unavailable`

### 6.8 Error codes (control)

`unauthorized` · `forbidden` · `not_found` · `conflict` · `profile_invalid` · `gateway_unavailable` · `rate_limited` · `bad_request` · `internal`  
Port codes: `ok` · `cancelled` · `timeout` · `auth` · `rate_limit` · `bad_audio` · `bad_request` · `unavailable` · `no_hit` · `unsupported` · `internal`

### 6.9 Path step schema (normative)

Common fields on **every** step:

| Field | Type | Default | Meaning |
|---|---|---|---|
| `id` | string | — | Stable within path (audit key) |
| `type` | step type §6.2 | — | Behaviour |
| `label` | string | — | GUI display |
| `barge_allowed` | bool | `true` | Caller may interrupt |
| `max_retries` | int | `2` | Per-step repair attempts |
| `on_no_input` | `reprompt` \| `next` \| `route_fallback` \| `end` | `reprompt` | Silence at this step |
| `on_no_match` | `reprompt` \| `clarify` \| `route_fallback` \| `end` | `clarify` | Unparseable answer |
| `timeout_ms` | int | `8000` | Wait for caller answer |

Type-specific fields:

| Type | Fields |
|---|---|
| `Say` | `prompt_id` |
| `Ask` | `slot_key` (attribute), `prompt_id`, `reprompt_id?`, `validation` (`free_text`\|`email`\|`phone`\|`number`\|`choice`\|`product`), `required` (bool) |
| `Confirm` | `summary_template_id`, `on_yes` → step, `on_no` → step (correction) |
| `Choice` | `options[] { id, label, utterances[], next }`, `prompt_id` |
| `Action` | `skill`, `arg_map { arg ← attribute }`, `branches { ok, fail, duplicate, timeout, unavailable }`, `speak_on_wait?` |
| `End` | `closing_prompt_id`, `offer_anything_else` (bool), `disposition_hint` (code from §6.6) |

**Compiler law:** every non-`End` step must resolve to a reachable next step or a branch; unreachable or dangling steps → `422 profile_invalid` at publish.

### 6.10 Repair policy (no-input / no-match / correction)

| Situation | Behaviour |
|---|---|
| Silence at step | Nudge 1 → Nudge 2 → per `on_no_input`; global silence ladder ends call with `abandoned_silence` |
| Answer not parseable | Reprompt up to `max_retries` → then `on_no_match` |
| 3 consecutive failed steps (session-wide) | Route to matrix fallback (transfer) or `End` with `unresolved` |
| Caller corrects an earlier slot | `FillSlot` overwrites; re-run `Confirm` if the corrected slot is in the confirmed package |
| Caller answers two slots at once | Fill both; skip the satisfied `Ask` |
| Caller asks off-path question | Answer from KB if grounded, then **return to the same step** |
| Caller requests human at any step | Honor immediately → matrix transfer (system law) |

### 6.11 NLU decision thresholds (defaults, ops-tunable)

| Signal | Threshold | Action |
|---|---|---|
| Intent confidence ≥ `0.60` | accept | Enter intent path |
| `0.40` ≤ confidence < `0.60` | ambiguous | One confirm question (“Did you mean …?”) |
| confidence < `0.40` | reject | Clarify prompt (§16 fallback) |
| Two consecutive clarifies | fallback | Route per matrix (default: Tech/agent) or End |
| Slot extraction | schema-constrained | Values must satisfy `validation`; else no-match |

LLM never sets `intent` directly into a skill argument without passing the path's `arg_map`.

### 6.12 Product defaults (set now — ops may change; not blockers)

| Setting | Default |
|---|---|
| Silence nudge 1 | 6 s |
| Silence nudge 2 | +6 s |
| Hangup after nudge 2 | +8 s |
| Ask `timeout_ms` | 8000 |
| Ask `max_retries` | 2 |
| Barge-in | On |
| Listen-while-speak gate | On |
| Skill timeouts | `resolve_caller` 3000 · `find_open_complaint` 5000 · `create_service_complaint` 8000 · `send_complaint_email` 8000 · `transfer_to_queue` 5000 · `scrub_outbound_consent` 3000 |
| Retention defaults (until legal sets) | transcript 90 d · attributes 90 d · audit 365 d · recording 0 (off) |
| `max_concurrent_sessions` | 10 (lab) |

---

## 7. Roles and surfaces

| Role | Surface | May | Must not |
|---|---|---|---|
| Platform Admin | Admin → Platform setup | Engines, credentials, properties, Coral URL | Edit desk script prompts as “system prompts” |
| Desk Configurator | Admin → Desk library | All desk forms, publish, simulator | JSON, regex, LLM system prompt editing |
| Supervisor | Supervisor console | Transcript, attributes (reveal audited), disposition, force stop | Change published logic |
| Lab User / caller | User console / phone | Call | Config |
| Developer | API + Developer toggle | Export/import, new connector code | Be only path to change welcome text |

### 7.1 Admin GUI map

```text
Admin
├── Platform setup     Engines · Credentials · Properties · Coral base URL
├── Desk library
│   ├── Desks list → Create Inbound | Outbound
│   ├── Overview · Prompts · Intents · Conversations · Knowledge
│   ├── Actions (skills + routing matrix) · Call behaviour · Test · Publish
├── Knowledge packs    Shared upload
└── Prompt library     Optional shared prompts
```

Supervisor / User: published desks only.

---

## 8. Runtime flows

### 8.1 Publish flow

```text
Configurator edits draft
  → F-P01 ValidatePublishChecklist
  → F-P02 CompileDeskToProfile
  → F-P03 PublishDesk (profile_version + desk_version)
  → F-V03 AuditPublish
```

Mid-publish: **in-flight sessions keep old pin.**

### 8.2 Inbound call flow (happy path)

```text
1. Coral/FS or lab User requests session
2. F-S01 AdmitSession (capacity)
3. F-S02 CreateSession (pin profile_version, gateway_binding, purpose, attributes seed)
4. F-S03 AttachEdge (1 media WS)
5. F-S04 AnswerSession → F-M03 Welcome (short; locale)
6. Loop:
     F-M01 PCM → F-M02 ListenFinal
     F-M07 LanguageLock (once)
     F-M05 if speaking → gate echo
     F-A01 DeskControllerTurn:
       F-A02 intent | F-A03 path | F-A04 slots | F-A09 KB
       F-A08 ladder / F-A07 laws
       optional F-K01 skill → update attributes
       F-M03 Speak
     F-M04 on barge-in
     F-M06 on silence
7. End path / transfer / stop
8. Terminal → F-O01 freeze → F-O02/03 disposition → F-C04 evidence
9. F-O07 Supervisor ACW card available
```

### 8.3 Think path (platform order, desk-aware)

```text
memory → redact → playbook/path → Knowledge → rules pre
  → ladder (clip|template|llm) → Think if llm
  → rules post → Skill if act → Translate if any → Speak
```

Collision: **rules > skills > grounding > free LLM**.

### 8.4 Ticket lifecycle (Complaint)

```text
Confirm package (name/email/phone/product/problem)
  → F-K04 FindOpenComplaint
       ├─ duplicate → Choice (transfer | add info) → no second ticket
       └─ none → F-K05 CreateServiceComplaint
              ├─ ok → set ticket_id (skill only) → F-K06 SendComplaintEmail
              │         email fail ⇒ email_sent=false; ticket still valid
              └─ fail → offer Tech transfer; never invent id
```

### 8.5 Transfer policy

1. Prefer resolve in path.  
2. Unresolved → agent **or** ticket per matrix.  
3. Explicit “transfer me” → honor.  
4. Sales unavailable → `register_sales_enquiry` / callback.  
5. Handoff = F-A11 pack; Coral executes ACD transfer after skill OK.  
6. Session → Draining on successful transfer skill.

### 8.6 Outbound stub flow (W4)

```text
CreateSession(direction=outbound, consent fields)
  → F-C08 / F-K11 consent scrub
  → if denied: Terminal + disposition; no dial
  → else Answer/Speak campaign path (stub skills OK)
```

### 8.7 Degradation

| Failure | Behaviour |
|---|---|
| Engines missing | CreateSession `422` |
| Concurrent full | `429` (`admission_mode=reject`) |
| STT/LLM/TTS down | Canned clip + escalate/transfer; **no** mid-session vendor hop |
| Skill timeout/5xx | Fail branch → apology → transfer/callback |
| WS drop | Terminal + `abandoned_ws_drop` |
| Knowledge miss + required | Refuse or escalate (rules) |
| Grounding required + invent risk | Block free invent |

### 8.8 Session state machine (transitions)

| From | Event | To | Allowed operations in state |
|---|---|---|---|
| — | `CreateSession` (after admit) | `Created` | attach, stop |
| `Created` | edge attached / attachment bound | `Attached` | answer, inject, stop |
| `Attached` | `answer` or first frame | `Running` | inject, patch hot fields, reveal, stop |
| `Running` | transfer skill ok / `stop` / End step | `Draining` | read-only + finish |
| `Running` | WS drop | `Failed` (Terminal) | postcall only |
| `Draining` | flush complete | `Completed` (Terminal) | postcall only |
| any | operator cancel | `Cancelled` (Terminal) | postcall only |

On **Terminal** (any): `FreezeAttributes` → `SuggestDisposition` → `BuildEvidencePack` → release admission slot → retention clock starts.  
Supervisor may still set the **final** disposition after freeze; attributes stay immutable.

---

## 9. Skill contracts (stable I/O)

Configurator fills **business fields**; schemas are fixed.

| Skill | Authority | Input (min) | Output (min) | Notes |
|---|---|---|---|---|
| `resolve_caller` | inform | caller / customer_ref | customer_id, prefs? | Start-of-call optional |
| `transfer_to_queue` / `warm_transfer` | act | handoff pack fields | ok | Coral POST; drain |
| `find_open_complaint` | inform | caller, product? | open_ticket_id \| none | Duplicate branch |
| `create_service_complaint` | act | confirmed attrs | ticket_id \| error | **Never fake id** |
| `send_complaint_email` | act | ticket_id, email, summary | email_sent bool | Independent of create |
| `register_sales_enquiry` | act | contact + notes | enquiry_id \| stub | Sales unavailable |
| `schedule_callback` | act | phone, window | ok | Outbound/inbound |
| `search_knowledge` | inform | query | hits | Optional explicit |
| `push_disposition` | act | suggestion, session_id | ok | Postcall |
| `scrub_outbound_consent` | decide/act | ani, purpose | allow\|deny | Outbound |

**Stub mode:** same contract; returns deterministic ok/fail/duplicate for lab. GUI toggle Stub vs Live endpoint (Admin/bind).

**Warm transfer payload** (to Coral): `session_id`, `tenant_id`, `caller`, `intent`, `summary`, `transcript_excerpt`, `recording_ref`, `profile_id`/`version`, `escalation_reason`, plus attribute pack keys needed for screen-pop.

### 9.1 Idempotency and retry law

| Rule | Statement |
|---|---|
| Key | `idempotency_key = session_id + ":" + step_id + ":" + attempt` sent to every connector |
| Ledger | Write `skill_invocation` **before** the call; update outcome after |
| `act` skills | **No automatic retry** on `timeout`/`unavailable` — take the branch and let the path decide (transfer, callback, apology) |
| `inform` skills | May retry **once** on `timeout` |
| Duplicate protection | `find_open_complaint` before `create_service_complaint`; connectors should also honor the idempotency key |
| Unknown outcome | Treat as `timeout` branch; never assume success; never speak an id we did not receive |
| Replay | Same key + same args must not create a second external record |

### 9.2 Skill argument / result schemas (frozen)

```json
{
  "resolve_caller":            { "args": { "caller": "string?", "customer_ref": "string?" },
                                 "result": { "ok": "bool", "customer_id": "string?", "display_name": "string?", "preferred_language": "string?", "stub": "bool?" } },
  "transfer_to_queue":         { "args": { "target": "string", "intent": "string", "summary": "string", "transcript_excerpt": "string?", "attributes": "object" },
                                 "result": { "ok": "bool", "accepted_by": "string?", "error": "string?" } },
  "find_open_complaint":       { "args": { "caller": "string?", "customer_email": "string?", "product": "string?" },
                                 "result": { "ok": "bool", "found": "bool", "ticket_id": "string?", "status": "string?" } },
  "create_service_complaint":  { "args": { "customer_name": "string", "customer_phone": "string", "customer_email": "string?", "product": "string", "problem": "string", "impact": "string?", "error_alarm": "string?", "priority": "string?", "troubleshoot_notes": "string?" },
                                 "result": { "ok": "bool", "ticket_id": "string?", "error": "string?" } },
  "send_complaint_email":      { "args": { "ticket_id": "string", "to_email": "string", "summary": "string" },
                                 "result": { "ok": "bool", "email_sent": "bool", "error": "string?" } },
  "register_sales_enquiry":    { "args": { "customer_name": "string", "customer_phone": "string", "requirement": "string", "product": "string?" },
                                 "result": { "ok": "bool", "enquiry_id": "string?", "error": "string?" } },
  "schedule_callback":         { "args": { "customer_phone": "string", "window": "string", "reason": "string?" },
                                 "result": { "ok": "bool", "callback_id": "string?", "error": "string?" } },
  "search_knowledge":          { "args": { "query": "string", "collection": "string?" },
                                 "result": { "ok": "bool", "hits": [ { "text": "string", "source": "string", "score": "number" } ] } },
  "push_disposition":          { "args": { "session_id": "string", "suggestion": "string", "template_id": "string?", "transcript_excerpt": "string?", "recording_ref": "string?" },
                                 "result": { "ok": "bool" } },
  "scrub_outbound_consent":    { "args": { "ani": "string", "purpose": "string" },
                                 "result": { "ok": "bool", "decision": "allow|deny", "reason": "string?" } }
}
```

Rules: unknown args rejected; missing required arg → `bad_request` (no call); results outside this shape are a connector defect; `ticket_id` may be spoken **only** when `ok = true`.

---

## 10. Connector lifecycle (industrial adaptation)

| Stage | What |
|---|---|
| 1. Contract | Skill name + JSON Schema frozen in catalog |
| 2. Implement | Connector gateway behind Skill port |
| 3. Register | Process gateway registry |
| 4. Bind | Desk Actions GUI: enable + map Coral fields (queue, owner) |
| 5. Stub/Live | Toggle; secrets via credentials API |
| 6. Observe | Audit skill outcome; fail branches already in path |

**Honest rule:** New CRM does **not** redesign desks **if** playbooks only call skill names. Each CRM still needs a **connector project** (secrets, maps, retries, idempotency).

---

## 11. Compliance-by-design

### 11.1 Seams mandatory in schema/API from W0

| Seam | Function | First delivery |
|---|---|---|
| `purpose` | F-C01 | Required field |
| `retention_policy` | F-T06 | Config row; sweeper can be stub |
| PII class on attributes | F-C02 | Mandatory |
| PII reveal audit | F-C03 | Mandatory |
| `evidence_pack` | F-C04 | Always on terminal |
| Erasure API | F-C05 | Contract W0; implement early |
| Outbound consent | F-C08 | Field W0; enforce W4 |
| Recording policy hook | F-C07 | Schema now; media later |
| Vendor DPA notes | credential metadata | Docs + ops |

### 11.2 Phased (extend seams — no model redesign)

Full audio redaction · formal DPA paperwork · contractual paging SLOs · multi-region residency.

---

## 12. Capacity and admission

| Property | Scope | Behaviour |
|---|---|---|
| `max_concurrent_sessions` | Tenant (ops) | Count Running+Attached+Draining |
| `admission_mode` | Tenant | V1: `reject` → HTTP `429`; `queue` reserved |
| Global per-instance cap | Ops | Per OPERATIONS.md |
| WS sticky | LB | Required for live media |

Later extensions (no desk redesign): vendor rate limits, cost ceilings, multi-node owner routing for control mutations.

### 12.1 Admission algorithm (normative)

```text
AdmitSession(tenant):
  active = count(sessions where tenant AND state in {Created, Attached, Running, Draining})
  if active >= max_concurrent_sessions:
      if admission_mode == "reject":  return 429 { error: rate_limited, retry_after: 5 }
      else:                           return 429 (queue reserved for a later wave)
  reserve slot atomically (DB counter row or transactional insert)
  return reservation
```

- Reservation is released on **Terminal** or on create failure (no leak on `422`).  
- Global per-instance cap from `OPERATIONS.md` still applies and is checked first.  
- Counting is per **tenant**, not per desk; a desk cannot raise its own limit.

---

## 13. Voice / CX NFR bar (industrial Talk)

| NFR | Requirement |
|---|---|
| Barge-in | Flush + cancel; acknowledge; continue path — **not** restart welcome |
| Listen-while-speak | Default On; ignore TTS echo as user speech |
| Silence | Configurable t1/t2/hangup; prompts from library |
| Language | Auto-detect then lock; EN/HI tabs; missing HI → EN + publish warning |
| Welcome length | Short; Configurator-owned |
| Ladder failover | clip/WAV before LLM; vendor down → canned + escalate |
| Corrections | Mid-path slot correction via NLU (name wrong after number) |
| Latency | Live Talk target per BUILD.md (p50 < ~1.2 s class); monitor p95 listen→first audio |
| Persona recall | Optional `resolve_caller` prefs at start (when skill on) |

### 13.1 Metrics and SLOs (named — implement with the runtime)

| Metric | Type | Target / alert |
|---|---|---|
| `cd_session_started_total{desk,direction}` | counter | — |
| `cd_turn_first_audio_ms` | histogram | p95 ≤ 1800 ms lab; p50 ≤ 1200 ms class |
| `cd_stt_final_latency_ms` | histogram | p95 ≤ 900 ms |
| `cd_think_latency_ms` | histogram | p95 ≤ 1500 ms |
| `cd_skill_latency_ms{skill}` | histogram | p95 ≤ skill timeout × 0.8 |
| `cd_skill_error_ratio{skill}` | ratio | alert > 5 % / 15 min |
| `cd_barge_in_total` / `cd_echo_suppressed_total` | counter | echo suppressed ≫ 0 means gate working |
| `cd_containment_ratio` | ratio | `resolved_self_service` ÷ completed |
| `cd_transfer_ratio{target}` | ratio | trend, not gate |
| `cd_abandoned_silence_ratio` | ratio | alert > 10 % |
| `cd_admission_rejected_total` | counter | capacity signal |
| `cd_publish_total{desk,result}` | counter | config change trail |

Every metric carries `tenant`, `desk_id`, `desk_version`. Audit rows remain the legal record; metrics are operational only.

---

## 14. Coral TFN workload (first preset)

**Not the architecture** — content loaded by `F-D13 LoadWorkloadPreset`.

### 14.1 Script map → GUI

| § | Content | Configured in |
|---|---|---|
| 1 | Welcome | Prompts |
| 2 | Intent ID + clarify | Intents + Clarify prompt |
| 3 | Sales | Path + matrix (Rahul Gupta / Sales) |
| 4 | Product + list | Path + Knowledge pack |
| 5 | Tech intake | Path + matrix (Arjun) |
| 6 | Complaint + product ID | Path + templates |
| 6.2–6.6 | Trees | Sub-paths: **W2 ships IP Phone + Generic**; Gateway/CC/Call Server/Cloud/VMS addable |
| 7 | Connect Tech | Action transfer |
| 8–14 | Register / ticket / email / fail / duplicate | Branches + skills |
| 15 | Critical | Priority transfer toggle/path |
| 16 | Fallback | Clarify |
| 17 | EN/HI | Overview + tabs |
| 18 | Barge-in | CX + prompt flags |
| 19 | Silence | CX + prompts |
| 20 | Close | Closing + End step |
| 21 | Matrix | Actions matrix |
| 22 | LLM laws | **System pack** (not Configurator textbox); tone dropdown only |

### 14.2 Transfer matrix (preset)

| Intent | Owner (label) | Default action |
|---|---|---|
| Sales Enquiry | Rahul Gupta | Transfer (else callback enquiry) |
| Product Information | Rahul Gupta | KB then optional transfer |
| Technical Support | Arjun Singh Topwal | Intake → summary → transfer |
| Service Complaint | Ritu / System | Troubleshoot → agent **or** ticket |
| Critical outage | Arjun | Priority transfer |

### 14.3 System laws (non-editable)

- Never invent ticket / email / transfer success.  
- Ask one question at a time when in Ask steps.  
- Prefer resolve before transfer unless explicit transfer request.  
- Ticket id spoken only from skill result.  
- Out of scope → clarify or transfer, not hallucinate Coral policy/pricing.

---

## 15. API surface (vertical additions)

Existing platform APIs remain (`CONTROL_API.md`). Vertical adds (names normative):

| Method | Path | Function |
|---|---|---|
| GET/POST | `/v1/desks` | List/Create |
| GET/PATCH | `/v1/desks/{id}` | Draft |
| PUT | `/v1/desks/{id}/prompts/{prompt_id}` | Upsert prompt |
| PUT | `/v1/desks/{id}/intents/{intent_id}` | Upsert intent |
| PUT | `/v1/desks/{id}/paths/{path_id}` | Guided path |
| PUT | `/v1/desks/{id}/matrix` | Routing matrix |
| PUT | `/v1/desks/{id}/cx` | CX policy |
| PUT | `/v1/desks/{id}/skills/{name}` | Bind |
| POST | `/v1/desks/{id}/simulate` | Simulator turn |
| POST | `/v1/desks/{id}/publish` | Publish |
| GET | `/v1/desks/{id}/versions` | History |
| GET/PUT | `/v1/tenant/properties` | Concurrency, admission |
| GET/PUT | `/v1/tenant/retention` | Retention policy |
| POST | `/v1/sessions/{id}/erasure` | Erasure request |
| GET | `/v1/sessions/{id}/attributes` | Contact attributes (mask default) |
| POST | `/v1/sessions/{id}/attributes/reveal` | Audited reveal |
| PATCH | `/v1/sessions/{id}/disposition` | Final ACW code |

Session create already pins profile; body may include `desk_id` resolving to latest published profile version.

### 15.1 Session create body (desk mode)

```json
{
  "desk_id": "coral-tfn-inbound",
  "purpose": "support",
  "direction": "inbound",
  "clock": "live",
  "caller": "+9111xxxxxxx",
  "metadata": { "coral_call_id": "…", "recording_ref": "…" },
  "consent_ref": null
}
```

Server resolves `desk_id` → latest **published** `profile_id` + `profile_version`, pins `gateway_binding`, seeds contact attributes (`direction`, `desk_id`, `desk_version`, `profile_version`, `purpose`, `ani`).  
Unpublished or unknown desk → `404 not_found`; desk published but engines missing → `422 profile_invalid`.

### 15.2 Authorization matrix

| Endpoint group | Platform Admin | Configurator | Supervisor | Coral estate token | Caller/User |
|---|---|---|---|---|---|
| `/v1/tenant/engines`, `/credentials`, `/properties` | RW | — | — | R | — |
| `/v1/tenant/retention` | RW | R | R | R | — |
| `/v1/desks/**` (draft, prompts, paths, matrix, cx, skills) | R | RW | R | R | — |
| `/v1/desks/{id}/publish` | R | **RW** | — | — | — |
| `/v1/desks/{id}/simulate` | RW | RW | R | — | — |
| `POST /v1/sessions`, `/answer`, `/stop` | RW | — | Stop only | RW | Create/stop own lab session |
| `/v1/sessions/{id}/transcript`, `/audit`, `/attributes` (masked) | R | — | R | R | — |
| `/v1/sessions/{id}/attributes/reveal` | — | — | **RW (audited)** | — | — |
| `PATCH /v1/sessions/{id}/disposition` | — | — | RW | RW | — |
| `/v1/sessions/{id}/erasure` | RW | — | Request only | RW | — |
| Export/Import desk | R | RW (Developer toggle) | — | RW | — |

### 15.3 Error envelope

```json
{ "error": { "code": "profile_invalid", "message": "…", "details": { "field": "paths.complaint.steps[3].skill" } } }
```

Codes limited to §6.8. Every `4xx` names the offending field path; publish failures list **all** red checklist items, not the first.

---

## 16. Publish checklist (Configurator)

Must be green to publish (Coral inbound):

- [ ] Welcome EN set; HI set or explicit fallback accepted  
- [ ] Four primary intents active with ≥1 example phrase each  
- [ ] Each intent has a guided path with End or Action  
- [ ] Routing matrix filled for Sales / Tech / Complaint  
- [ ] Ticket skill enabled **or** “ticket disabled — transfer only” explicit  
- [ ] Silence prompts + timeouts set  
- [ ] If Product uses KB: ≥1 knowledge pack attached  
- [ ] Voice set for bound Speak gateway  
- [ ] Purpose set  
- [ ] Simulator smoke passed **or** Skip with reason (draft only)

---

## 17. Delivery waves

| Wave | Scope | Exit criteria |
|---|---|---|
| **W0** | Schema + Desk APIs + GUI shell + properties/admission + compliance **contracts** (purpose, retention row, erasure API stub, PII classes) | Create desk draft; checklist stub; no coral-cc path |
| **W1** | Coral inbound preset in GUI; Prompts/Intents/CX editable; short welcome; EN/HI; simulator v0 text | Change welcome in GUI → publish → next call speaks it |
| **W2** | Guided paths four intents; IP Phone + Generic trees; static skills; routing matrix GUI | Simulator completes Sales/Product/Tech/Complaint happy paths |
| **W3** | Ticket/email/duplicate/critical; handoff pack; ACW disposition; attribute freeze | Ticket id only from skill; Supervisor ACW card |
| **W4** | Outbound desk type + consent scrub stub | Outbound create respects consent deny |
| **W5** | Acceptance vs script; retention sweeper live; erasure path live | Configurator completes Coral desk with **zero JSON** |

**Thin W0 law:** contracts and shell first; do not boil full legal/redaction program before W1 Coral proof.

### 17.1 Test and evidence map (per wave)

Harness: `tests/agent/` scenarios + Go unit/integration tests; pipeline `product-validation` (`docs/VALIDATION_PIPELINE.md`). Evidence lands in the validation-evidence worktree.

| Wave | Unit / integration | Scenario (tests/agent) | Evidence artifact |
|---|---|---|---|
| W0 | Desk CRUD, compile validator, admission counter, retention/erasure contract stubs | `desk-create-publish-reject-red-checklist` | API transcript + 429 proof |
| W1 | Prompt/intent compile, welcome speak, language fallback | `welcome-change-gui-to-call`, `hi-missing-fallback-en` | Session audit showing new `desk_version` |
| W2 | Path FSM, slot fill, repair policy, matrix | `sales-happy`, `product-kb`, `tech-intake`, `complaint-ip-phone`, `no-match-repair` | Simulator + live-call transcripts |
| W3 | Ticket/email/duplicate/critical, handoff pack, freeze | `ticket-ok`, `ticket-fail-transfer`, `email-fail-ticket-ok`, `duplicate-no-second-ticket` | `skill_invocation` rows + ACW card |
| W4 | Outbound create, consent deny | `outbound-consent-deny`, `outbound-stub-path` | Consent record + disposition |
| W5 | Retention sweeper, erasure execute, full script pass | `erasure-deletes-session`, `retention-sweep`, `coral-script-e2e` | Deletion counts + acceptance sheet |

**Definition of done for any wave:** exit criteria met **and** its scenarios pass **and** no `coral-cc` regex path used in the run.

---

## 18. Acceptance criteria (industrial)

1. Configurator never opens JSON to run Coral inbound.  
2. Welcome change via GUI appears on next session only (version pin).  
3. Matrix owner change appears in handoff pack.  
4. Example phrase added → simulator classifies intent.  
5. Ticket failure never speaks invented id.  
6. Email fail leaves ticket_id set, `email_sent=false`.  
7. Duplicate path blocks second create.  
8. Barge-in does not restart full welcome.  
9. Concurrent cap returns `429` when full.  
10. PII masked in logs; Supervisor reveal audited.  
11. New connector can satisfy `create_service_complaint` without changing Complaint path GUI.  
12. Desk publish creates `contact-agent` profile version used by runtime.

---

## 19. Non-goals

- Replacing Coral ACD / SIP  
- Becoming CRM SoR  
- Configurator editing LLM system prompts  
- Free-form visual graph in first waves (guided paths only)  
- Patching coral-cc regex as industrial desk  
- Zero-effort “any CRM works” without a connector  
- Multi-region residency in W0–W5 (seam only)

### 19.1 Outbound boundary (who does what)

| Concern | Owner |
|---|---|
| Contact list / campaign / pacing / dialer | **Coral (or customer dialer)** — not this product |
| Placing the SIP call, answer detection | Coral + FreeSWITCH |
| Session creation for the answered call | Coral calls `POST /v1/sessions` with `direction: outbound` |
| Consent / DND scrub decision | **Ours** as a hook: `CheckOutboundConsent` / `scrub_outbound_consent` before the desk speaks (fail-closed) |
| Campaign script (what the agent says) | **Ours** — outbound desk paths in GUI |
| Retry of unanswered calls | Coral dialer |
| Disposition + attributes + evidence | **Ours** |

We do **not** build a dialer, list manager, or pacing engine in W0–W5. The outbound **desk type** exists so its GUI, consent field, and disposition path are designed in from the start.

---

## 20. Have / change / add (gap vs today)

| Have (platform) | Change | Add (vertical) |
|---|---|---|
| Talk media, engines GUI, language lock, ladder, transcript/disposition APIs, skill slot, FS edge | Stop Configurator-facing JSON profiles; stop regex-as-intent | Desk entity, guided paths, attributes, rich disposition, matrix, skill wizards, simulator, compliance seams wired, outbound type |

---

## 21. Locked product decisions (owner-settled)

| # | Decision |
|---|---|
| 1 | Inbound-first acceptance; outbound **type in GUI from W0**; stub runtime W4 |
| 2 | Complaint trees W2: **IP Phone + Generic** first; other packs addable |
| 3 | Configurator **inside Admin → Desk library** (same app) |
| 4 | `max_concurrent_sessions` in **properties/ops** |
| 5 | Desk **publishes** contact-agent profile |

### 21.1 Items owned outside engineering (do not block implementation)

Each has a **safe default already set**, so W0–W5 can start today.

| Item | Owner | Default in force until decided |
|---|---|---|
| Retention days (transcript / attributes / audit / recording) | Legal + Coral ops | §6.12 defaults (90 / 90 / 365 / recording off) |
| Live Coral endpoints (transfer, ticket, email, CRM) | Coral estate | **Stub connectors**; `coral.base_url` unset → stub mode |
| Routing owners (names / queue ids) | Coral CC manager | Preset labels from script §21, editable in matrix GUI |
| Vendor DPAs | Legal | Recorded as credential metadata note |
| Exact Hindi prompt copy | Coral ops | EN text with publish warning; runtime falls back to EN |
| Recording on/off + storage | Coral ops | Off (policy hook present) |
| Which complaint trees beyond IP Phone + Generic | Coral ops | Added later as path templates, no code change |

**Rule:** none of these change the model. Deciding them is data entry or a connector config, not a redesign.

---

## 22. Agent implementation notes

1. Prefer smallest wave that meets exit criteria.  
2. Never reintroduce coral-cc regex as the desk runtime.  
3. Compile path → playbook; do not hand-author conflicting FSMs.  
4. Secrets never in profile/desk JSON.  
5. Side-effect skills: no blind retry.  
6. If a required lock is missing → write blocker; do not invent HUMAN-CLASS product semantics.  
7. Cite this file + architecture parents in plans.

---

## 23. Source map

| Need | Doc |
|---|---|
| Platform product | `PRODUCT_DECISIONS.md` |
| CC vertical locks | `CONTACT_AGENT.md` |
| Engines | `SYSTEM_ENGINES.md` |
| Language | `LANGUAGE_POLICY.md` |
| Rules/skills/playbook | `RULES_AND_SKILLS.md` |
| Runtime | `RUNTIME.md` |
| Profile JSON | `PROFILE_SCHEMA.md` |
| Ports | `PORTS.md` |
| HTTP | `CONTROL_API.md` |
| CRM/KB/audit | `INTEGRATION.md` |
| Postcall | `ANALYTICS_AND_POSTCALL.md` |
| Edge | `EDGE_FS.md` |
| Ops limits | `OPERATIONS.md` |
| **This vertical** | **This file** |

### 23.1 Glossary (single meaning per word)

| Term | Meaning here |
|---|---|
| **Desk** | Authoring/ops object for one contact purpose; publishes a profile version |
| **Profile** | Runtime behaviour document the session pins |
| **Workload** | Content loaded onto a desk (e.g. Coral TFN preset) |
| **Path** | Ordered guided flow for one intent |
| **Step** | One Say/Ask/Confirm/Choice/Action/End node |
| **Slot** | A value the desk asks for; stored as a contact attribute |
| **Contact attribute** | Structured session fact (not transcript text) |
| **Disposition** | Closed end code for the contact |
| **ACW** | After-call work: disposition + attribute review by Supervisor |
| **Skill** | Named business action with a frozen contract |
| **Connector** | Implementation of a skill against a real system |
| **Stub mode** | Deterministic skill result for lab/demo, same contract |
| **Handoff pack** | Attributes + summary + excerpt sent on transfer |
| **Evidence pack** | Compliance bundle bound to a session |
| **Seam** | Schema/API hook that lets a capability land later without redesign |
| **Ladder** | clip → template → LLM response order |
| **Admission** | Capacity decision before a session is created |

---

## 24. Round 6 — Detailed review (after full definition)

### 24.1 Review method

Checked against: (a) R5 gaps closed?, (b) every entity/function/flow agent-executable?, (c) no dual-runtime?, (d) compliance seams without boil-ocean?, (e) Coral workload separable?, (f) blockers remaining?

### 24.2 Gap closure vs Round 5

| R5 must-lock | Status in this doc |
|---|---|
| Desk ≡ profile publish | **Closed** — §4.3, F-P02/F-P03, acceptance #12 |
| Talk NFR bar | **Closed** — §13 |
| Thin W0 | **Closed** — §17 thin W0 law |
| Connector lifecycle | **Closed** — §10 |
| Admission contract | **Closed** — §12 (`reject` V1; `queue` reserved) |

### 24.3 Completeness scorecard

| Area | Complete? | Notes |
|---|---|---|
| Entities | Yes | §4 |
| Function IDs | Yes | §5 (F-T / F-D / F-P / F-S / F-M / F-A / F-K / F-C / F-O / F-V) |
| Vocabularies | Yes | §6 |
| E2E flows | Yes | §8 |
| Skills I/O | Yes | §9 |
| GUI / roles | Yes | §7, §16 |
| APIs | Yes | §15 (path names normative; OpenAPI may refine) |
| Coral preset | Yes | §14 |
| Waves / AC | Yes | §17–18 |
| Compliance | Yes | §11 |
| Error matrix | Yes | §8.7 |
| Dual-model risk | Mitigated | Explicit compile to profile |
| Skill field-level schemas | Yes (closed in R7) | §9.2 |
| Persistence / storage | Yes (closed in R7) | §4.4–4.5 |
| Step schema, repair, thresholds | Yes (closed in R7) | §6.9–6.12 |
| Complaint tree question text | **Workload content** | Preset fixture at W1 carries full Coral script copy |
| Legal retention days | **Human** (default in force) | §21.1 |
| Live CRM endpoints | **Estate** (stub default) | §21.1 |

### 24.4 Architecture blockers?

| Question | Answer |
|---|---|
| Can an agent start W0 without inventing entities? | **Yes** |
| Is there an undefined dual runtime? | **No** — Desk compiles to profile |
| Is compliance “later redesign”? | **No** — seams in §11 |
| Is Coral the architecture? | **No** — workload §14 |
| Remaining blockers? | **None architectural.** Human: retention days, estate URLs. Impl: schemas, GUI, preset fixture. |

### 24.5 Verdict (Round 6)

| | |
|---|---|
| **Verdict** | **Approved as complete industrial Contact Desk solution architecture** for build planning. |
| **Meaning** | An agent can implement W0→W5 from this file + architecture parents **without inventing** entities, enums, or core flows. |
| **Not** | Disposable POC architecture; not “approved code.” |
| **Conditional only on** | Implementation must follow Desk→Profile compile (§4.3) and thin W0; must not ship coral-cc regex as the desk. |
| **Human items** | Retention day numbers; live Coral endpoint URLs; exact complaint tree copy in preset fixture. |

**One-line verdict:** Architecture is **definition-complete and industrially framed**; remaining work is **implementation and estate wiring**, not redesign.

---

## 25. Round 7 — Final implementation-readiness review

### 25.1 What this round looked for

Not framing (settled in R4/R5) and not breadth (settled in R6), but the things that **stop a coder or agent mid-file**: where is it stored, what exactly is a step, what happens on the third bad answer, what key stops a duplicate ticket, who may call this endpoint, what number proves it works, who dials outbound.

### 25.2 Gaps found in R6 and closed in this revision

| # | Gap at R6 | Closed by |
|---|---|---|
| 1 | No persistence model — agents would invent tables | §4.4 tables + JSONB storage law |
| 2 | No schema evolution rule for published desks | §4.5 (`schema_version`, N-1, immutability, content hash) |
| 3 | Function list had no I/O or error contracts | §5.3 contract table + fail-closed rules |
| 4 | Path steps named but not specified | §6.9 normative step fields + compiler law |
| 5 | No no-input / no-match / correction policy | §6.10 repair policy |
| 6 | “NLU classifies intent” with no thresholds | §6.11 confidence bands and fallback |
| 7 | Timeouts/silence left as “configurable” with no defaults | §6.12 product defaults |
| 8 | Session states listed, transitions not | §8.8 transition table + terminal sequence |
| 9 | “No blind retry” without an idempotency mechanism | §9.1 key, ledger, unknown-outcome rule |
| 10 | Skill schemas were prose, marked Partial | §9.2 frozen arg/result schemas |
| 11 | Admission described, not computed | §12.1 algorithm + slot release |
| 12 | NFRs without measurable metrics | §13.1 named metrics and targets |
| 13 | API list without roles or error envelope | §15.1–15.3 (incl. session create body) |
| 14 | No test/evidence mapping per wave | §17.1 scenario map + DoD |
| 15 | Outbound ownership ambiguous (did we build a dialer?) | §19.1 boundary table |
| 16 | Human items read as blockers | §21.1 register with safe defaults in force |
| 17 | Vocabulary drift risk across agents | §23.1 glossary |
| 18 | Broken internal reference (§16.4) | Corrected to §16 |

### 25.3 Consistency check against platform locks

| Lock | Status |
|---|---|
| Go kernel, in-memory PCM, no broker for audio | Respected (no new media path) |
| Tenant engines pin, no mid-session vendor hop | Respected (§8.7, §12) |
| rules > skills > grounding > free LLM | Respected (§8.3) |
| Desk ≡ `contact-agent` profile publish | Respected (§4.3, §4.5) |
| Coral owns identity/ACD/CRM SoR | Respected (§3, §19.1) |
| Secrets never in profile/desk docs | Respected (§4.4 credentials table, §22) |
| GUI-only for Configurator; API for platform | Respected (§7, §15.2) |

No contradiction found with `PRODUCT_DECISIONS.md`, `CONTACT_AGENT.md`, `SYSTEM_ENGINES.md`, `CONTROL_API.md`, `RULES_AND_SKILLS.md`, `OPERATIONS.md`.

### 25.4 Residual items (declared, none blocking)

| Item | Class | Why not a gap |
|---|---|---|
| OpenAPI YAML for `/v1/desks/**` | Implementation artifact | Routes, bodies, codes, and roles are fixed here; generating the spec is coding |
| Coral script text as preset fixture | Workload content | Loader function and mapping defined (§14, F-D13); the words are data |
| Live endpoint URLs, retention day numbers, owner names | External decision | Defaults in force (§21.1); config, not redesign |
| `admission_mode: queue` | Reserved | V1 is `reject`; adding a queue extends §12.1 without touching desks |
| Free visual flow designer | Deliberate non-goal | Guided paths ship first (§19) |

### 25.5 Final verdict

| | |
|---|---|
| **Verdict** | **Approved — implementation-ready, end to end.** |
| **Scope of approval** | An implementer or coding agent can execute **W0 → W5** from this file plus the architecture parents, without inventing entities, enums, storage, contracts, thresholds, or flows. |
| **Standing** | Industrial Contact Desk vertical architecture. Coral TFN is the first workload on it, not the architecture. |
| **Conditions carried into build** | (1) Desk compiles to a `contact-agent` profile version — never a second runtime. (2) Thin W0 (contracts + shell) before Coral W1 proof. (3) `coral-cc` regex path is never the desk runtime. (4) Fail-closed on consent, PII reveal, and red checklists. |
| **Open architectural gaps** | **None.** |

**One-line verdict:** The design is closed — remaining work is writing the code, entering the data, and wiring the estate.
