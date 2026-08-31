# Rules, skills, playbooks, and intents — architecture lock

**Status:** LOCKED  
**Date:** 27 August 2026  
**Parents:** `PROFILE_SCHEMA.md`, `SOLUTION.md`, product lock

Execution order on the **Think path** (pipeline): memory → redact → playbook (if any) → Knowledge retrieve → rules pre → Think gateway → rules post → Skill (if act) → Translate (if configured) → Speak/text out.

**Collision priority** (product lock — what wins when they disagree): **rules > skills > grounding > free LLM**. Rules can block Think, refuse, or force escalate before any model runs. Grounding blocks invention when `grounding.required` is true.

---

## 1. Rules engine

### Format

Declarative **JSON** rules in the profile (`rules[]`). No arbitrary code in profiles. Complex logic uses playbook states (§3), not embedded scripts.

Each rule:

| Field | Meaning |
|---|---|
| `id` | Stable name for audit |
| `phase` | When it runs (see table below) |
| `when` | Condition object (all keys must match) |
| `action` | `allow` \| `refuse` \| `escalate` \| `inject_text` \| `block_think` \| `strip_response` |
| `skill` | Optional skill to invoke on `escalate` |
| `message` | Spoken or logged text when refusing |

### Phases

| Phase | Runs |
|---|---|
| `pre_listen` | Before sending audio to Listen gateway (rare; e.g. mute) |
| `pre_think` | After STT final, before Knowledge/LLM |
| `post_think` | After LLM response, before Speak/skill act |
| `pre_speak_first` | Once per session before first Speak |
| `pre_skill` | Before Skill router executes |

### Condition vocabulary (closed)

| Key | Values |
|---|---|
| `grounding_required` | bool (from profile) |
| `knowledge_hit` | bool |
| `intent` | string (from playbook/slot) |
| `confidence_below` | float |
| `regex` | pattern on user text |
| `slot_missing` | slot name |
| `caller_request_human` | bool (keyword list in profile) |

First matching rule in profile order wins unless `action: allow` chains are documented for a profile family.

---

## 2. Skills

### Model

Skills are **named operations** with a stable contract. The LLM may **propose** a skill via structured output; the runtime **executes** only if:

1. Skill is in `profile.skills.allowed`  
2. `authority` permits the operation (inform / decide / act)  
3. `confirm: true` skills receive explicit confirmation (control API or DTMF/profile rule)  
4. `pre_skill` rules pass  
5. Skill router succeeds (no blind retry on side effects)

### Skill definition shape

```yaml
get_transaction_status:
  gateway: customer-crm-http
  authority: inform
  confirm: false
  input_schema:          # JSON Schema subset
    type: object
    properties:
      transaction_id: { type: string }
    required: [transaction_id]
  timeout_ms: 5000
```

### LLM integration

Think gateway receives **skill descriptors** (name, description, input schema). LLM returns optional structured block:

```json
{ "skill": "get_transaction_status", "args": { "transaction_id": "4412" } }
```

Runtime validates args against schema, runs Skill router, injects result into memory, then may Speak a summary. **Side-effecting skills** (`act`) never run without authority + audit row.

### Warm transfer (Coral skill gateway)

`warm_transfer` is a first-party **Skill** gateway (`coral-transfer`), not FS logic inside the composer.

**Payload to Coral** (HTTP POST to estate endpoint — exact path configured per deploy):

| Field | Content |
|---|---|
| `session_id` | Orchestrator session id |
| `tenant_id` | Tenant |
| `caller` | ANI / channel id |
| `intent` | Resolved intent + slots from playbook/memory |
| `summary` | Short AI summary (post-Think or template) |
| `transcript_excerpt` | Last N turns or full (policy) |
| `recording_ref` | FS/CDR recording URI if known |
| `profile_id` / `version` | Pinned profile |
| `escalation_reason` | rule id or low_confidence |

Coral agent desktop **screen-pop** consumes this event (existing CRM/ACD integration). Orchestrator session moves to **Draining** after successful handoff skill; FS transfer is triggered by Coral/FS, not by orchestrator SIP.

### resolve_customer (coral-crm stub)

Lookup / resolve caller identity against Coral CRM (stub only — no live CRM writeback in V1).

| Field (args) | Content |
|---|---|
| `caller` | ANI / channel id (optional if `customer_ref` set) |
| `customer_ref` | Opaque customer reference |

Stub result when BaseURL empty: `{ "ok": true, "stub": true, "customer_id": "…" }`.

### push_disposition (coral-crm stub)

Optional postcall skill to push AI disposition suggestion to Coral CRM/CDR. Prefer this over piggybacking `create_ticket` when both are allowed.

| Field (args) | Content |
|---|---|
| `session_id` | Orchestrator session id |
| `suggestion` | `resolved` \| `unresolved` \| `escalated` |
| `template_id` | Profile `templates.disposition.id` |
| `transcript_excerpt` | Optional last-N turns excerpt |
| `recording_ref` | Optional FS/CDR recording URI |

Postcall worker: if `push_disposition` ∈ `skills.allowed` + definitions → Execute; else fall back to existing optional `create_ticket` push. Full production Coral writeback remains out of scope until estate endpoint is configured.

---

## 3. Playbooks and intents

For **bounded** flows (on-prem talking IVR, contact-agent intents):

| Layer | Role |
|---|---|
| **Playbook** | Finite state machine in profile (`playbook.states`). Defines slots, transitions, and which step may call Think. |
| **Intent** | Named outcome + slots (e.g. `balance_inquiry`, `account_id`). Filled by STT text + optional lightweight classifier gateway or LLM extract step inside Think. |
| **Template** | Output shape for playback/post-call (MoM, disposition tags) — not conversational state. |
| **Graph** (later) | Knowledge gateway type for org/workflow grounding; playbook may reference graph nodes. |

**Hybrid (locked):**

- **Playbook-first profiles:** composer consults playbook state before free LLM; LLM used for slot fill and natural phrasing within allowed states.  
- **Copilot / open Talk profiles:** no playbook; Think + grounding + skills only.  
- **Contact-agent (cloud):** playbook for intent routing + LLM for dialogue within policy.

Intent and slots live in **session memory** (structured JSON), auditable per turn.

---

## 4. Grounding types (architecture mapping)

| Product type | Runtime |
|---|---|
| `document_kb` | Knowledge router → ingest / http_kb |
| `template` | Think path loads template by id; playback/post-call |
| `playbook` | State machine + optional KB per state |
| `graph` | Knowledge gateway `graph_kb` (later); same router |

---

## 5. Authority matrix

| Authority | LLM may suggest | Runtime executes | Example |
|---|---|---|---|
| `inform` | Yes | Read-only skill or spoken answer | FAQ, txn status |
| `decide` | Yes | Branch playbook; no external write | Route to queue bucket |
| `act` | Yes, with confirm if configured | Skill gateway write | Create ticket, transfer |

Rules can downgrade or block any authority level.
