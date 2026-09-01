# Live Talk CX + India Multilingual — Progressive Solution

**Status:** SOLUTION LOCK (design for implementation)  
**Date:** 1 September 2026  
**Parents:** `CONTACT_DESK_POC_SOLUTION.md`, `LANGUAGE_POLICY.md`, `PRODUCT_DECISIONS.md`, `docs/SOLUTION.md` §14–15, §24, `RUNTIME.md`, `EDGE_FS.md`  
**Trigger:** Lab RCA — welcome race, energy-false barge, overlap transcripts, intent/Hinglish gaps, EN+HI-only vs India-capable STT/LLM/TTS  

**Hard rule:** Do not implement beyond the phase that is open. Each phase has exit criteria. Product copy and desk content stay Configurator-owned; this file locks **runtime behaviour**.

---

## 0. Problem → outcome map

| RCA finding | Desired outcome |
|---|---|
| Silence ladder / barge race welcome | Welcome is the **first** bot speech after media Ready; silence armed only after |
| Energy VAD flushes TTS on noise | **Orch** commits barge only on **meaningful** speech; FS is dumb flush executor |
| Overlap / concurrent speak | One Speak/Turn owner; barge speech **persisted**; no duplicate `transcript_turn` seq |
| Hinglish / phrase miss → tech transfer | Intent ids stable; multilingual classify; safe clarify (not dump to tech) |
| Desk languages EN+HI only | Author **once** (primary locale); runtime **any allowed Indian language** via STT/LLM/TTS |

**Capability assumption (locked for design):** Tenant Listen / Think / Speak gateways used for Contact Desk are **India-multilingual**. The product does **not** invent a second MT kernel for V1 Talk; it uses gateway language detect + respond-in-language + optional prompt synthesis.

**Fail policy (settled — was ambiguous):**

| When | Behaviour |
|---|---|
| Publish / session create: pinned Listen or Speak cannot cover desk `runtime_languages` ∩ requested India default | **Fail closed** with clear validation error (do not start Talk pretending full India) |
| Mid-call: STT detects a language **outside** the session allowlist | **Fail soft** — keep `canonical_locale` Speak; soft offer; do not hard-hangup |
| Tenant shrinks allowlist to `{en-IN}` only | Allowed — runtime is that intersection; not a product bug |

---

## 1. Progressive delivery (optimized order)

Ship the highest blast-radius fixes first. Do not widen language surface until the media/turn machine is stable.

| Phase | Name | Goal | Depends on |
|---|---|---|---|
| **P0** | Establish + welcome gate + smart barge + turn exclusivity | Caller hears welcome; noise does not destroy TTS; transcripts ordered | — |
| **P1** | Multilingual dialog brain | One primary authored flow; reply + classify in caller language (India allowlist) | P0 |
| **P2** | CX depth | Voice-per-language map, richer barge semantics, optional MT interpret profiles | P1 |

**Optimization principle:** Prefer **state machine + ownership locks** over more timers. Prefer **STT/LLM evidence** over more energy knobs. Prefer **canonical intent ids** over per-language playbook forks.

---

## 2. Media / session phase machine (P0 — normative)

### 2.1 Phases

```text
Created
  → Attached          # control session exists; edge may not be up
  → Establishing      # feeder/sink WS up; hello exchanged; optional RTP settle
  → Ready             # sink inject path proven OR settle timer elapsed
  → Welcoming         # exclusive AnswerSession speak (welcome)
  → Conversing        # silence ladder armed; barge commit policy active
  → Draining / Terminal
```

| Phase | SilenceWatch | Candidate barge | Commit barge / flush | AnswerSession | User dialog turns |
|---|---|---|---|---|---|
| Establishing | **Off** | Off | Off | `409 not_ready` | Off |
| Ready | Off | Off | Off | **Allowed** | Off |
| Welcoming | Off | On (telemetry) | Only if `welcome_barge_allowed=true` | In progress | Queue finals (no Think until Conversing) |
| Conversing | **On** | On | On (policy) | Idempotent no-op if `welcome_completed` | On |

### 2.2 Ready criteria (closed)

**Always required:** edge `hello` accepted **and** sink attached.

**Plus at least one of:**

1. First uplink PCM frame observed on the session bus, **or**  
2. `rtp_settle_ms` elapsed since attach (default **500 ms**; ops range 400–800).

Session create HTTP 200 alone is **never** Ready. `POST …/answer` allowed only when phase ≥ `Ready`.

### 2.3 AnswerSession (`F-S04`) rewrite

| Rule | Lock |
|---|---|
| Prerequisite | Phase `Ready` → else `409 not_ready` + `Retry-After` hint (Lua polls/retries) |
| Ownership | Acquires **SpeakOwner** for welcome |
| Flags | Use `welcome_completed` (not legacy `answered` alone). Legacy `answered` may alias **only after** `welcome_completed` for compat |
| Mark complete | `welcome_completed=true` **only after** Speak mark **or** hard gateway failure after one retry |
| In flight | Concurrent `/answer` → `409 welcome_in_progress` (Lua must **not** double-post; poll until `welcome_completed` or timeout) |
| Success idempotent | `welcome_completed` → `200` empty body / no re-speak |
| Failure | Audit speak attempt; allow **one** retry; then enter Conversing with optional canned fail clip; arm silence |
| Next | `Welcoming` → `Conversing` → **then** `ArmSilenceWatch` |

**Anti-pattern (current bug class):** mark answered before speak finishes; start silence while still Listening during open TTS; energy barge flush of welcome.

### 2.4 Edge / Lua contract (single welcome trigger — settled)

**P0 lock — one trigger only:**

1. Create session → attach stream → hold with `silence_stream`.  
2. Wait until orch reports Ready (control HTTP poll — preferred; see §13.3).  
3. Lua **`POST /v1/sessions/{id}/answer`** exactly once to start welcome.  
4. Prefer orch returning after welcome **starts** (async) with `welcome_started`, Lua optionally polls `welcome_completed`; **acceptable P0**: blocking `/answer` until Speak mark with timeout ≥ welcome TTS budget (e.g. 20–30s), not full-call 45–60s if avoidable.  
5. Hangup → stop stream; orch Draining.

**Forbidden in P0:** orch **auto-welcome on Ready** while Lua also POSTs `/answer` (double-speak race). Optional later flag `auto_answer_on_ready` default **false** for Contact Desk.

FS `mod_audio_stream` remains: binary PCM + JSON control (`flush` only when orch sends it).

---

## 3. Smart barge — orch-owned (P0 — normative)

**Supersedes** `RUNTIME.md` §6 barge steps 1–5 for **Contact Desk / Talk live** when `barge_mode=commit_on_stt` (default). Platform RUNTIME text remains historical for generic Talk until patched; this file wins for desk.

### 3.1 Two-stage model

```text
Uplink PCM
  → Energy / local VAD          → CandidateBarge (signal only) while Speaking
                              → Endpointing aid while Listening/Capturing (unchanged)
  → Listen (partials / finals)  → Meaning evidence (Listen stream stays open during Speak)
  → BargePolicy.Decide          → Commit | Ignore | Hold
  → on Commit: Flush sink + cancel Speak/Think + Capturing
```

| Stage | Who | May flush TTS? |
|---|---|---|
| CandidateBarge | Local energy / Silero-class | **No** |
| CommitBarge | Orch policy | **Yes** (sends flush to edge) |

**Law:** FreeSWITCH / module never decides “meaningful speech.” They execute orch `flush`.

**Energy VAD still used for:** Listening → Capturing endpointing (speech start / silence end). **Energy must not** call `Sink.Flush` while Speaking.

### 3.2 Commit conditions (defaults — CX knobs)

Commit when **any** of:

1. Listen **final** with non-empty transcript after trim, length ≥ `min_barge_chars` (default **2**).  
2. Listen **partial** with `confidence ≥ barge_partial_confidence` (default **0.70**) **and** speech duration ≥ `min_barge_ms` (default **280 ms**) **and** listen-while-speak gate says “not echo.”  
3. Explicit DTMF barge (if profile enables).

Do **not** commit on energy alone while Speaking.

**P0 filler denylist:** not required (length gate only). Optional ops denylist deferred to P2.

**Gateway degradation:** if Listen gateway emits **no partials** while Speaking, Commit path = **finals only** (higher barge latency; still correct). Do not fall back to energy Commit.

### 3.3 Listen-while-speak / echo

| Mode | Behaviour |
|---|---|
| Gate On (default) | While Speaking: CandidateBarge telemetry only; uplink-only PCM to Listen; require Commit evidence; map desk `listen_while_speak` / `ListenWhileSpeak` (today unused in composer — **must wire**) |
| Gate Off | Still require Commit evidence; energy alone still does not flush |

Desk CX `barge_allowed=false` → never Commit (ignore STT barge too). `barge_allowed=true` + `barge_mode=commit_on_stt` = this section.

### 3.4 Welcome barge

| `welcome_barge_allowed` | Behaviour |
|---|---|
| `false` (default) | During Welcoming: buffer Commit candidates; **no flush**; after welcome mark, process queued user finals |
| `true` | Commit allowed during welcome; do **not** restart full welcome (existing NFR) |

### 3.5 Metrics (add)

| Metric | Meaning |
|---|---|
| `cd_barge_candidate_total` | Energy/partial candidates |
| `cd_barge_commit_total` | Actual flush commits |
| `cd_barge_suppress_echo_total` | Gate suppressed |
| `cd_welcome_first_audio_ms` | Ready → first welcome PCM |

Target: commit ≪ candidate on noisy lines; welcome first audio p95 within lab SLO.

---

## 4. Turn exclusivity + durable overlap transcripts (P0)

### 4.1 SpeakOwner / TurnOwner

Exactly one of: Welcoming speak · path Say/Ask speak · silence nudge speak · Thinking response speak.

| Event | Behaviour |
|---|---|
| New Speak while owner held | Reject or queue behind owner (silence **never** preempts Welcoming) |
| CommitBarge | Cancel owner; persist interrupted assistant text if any; open Capturing |
| Transcript seq | **Single allocator** per session (DB upsert / next-seq under session lock) |

### 4.2 Overlap transcript rules

| Case | Persist |
|---|---|
| User final during Speaking (Commit) | `assistant` row with `interrupted=true` + best-effort text (last full sentence spoken / buffered Speak text; empty allowed if none) + `user` row; then Think |
| User final during Welcoming (`welcome_barge_allowed=false`) | Queue; after welcome mark → `user` row + Think (do not flush welcome) |
| User Commit during Welcoming (`welcome_barge_allowed=true`) | Flush once; interrupted assistant + user; **do not** restart full welcome; enter Conversing / path |
| Candidate only (noise) | No user transcript; no flush |
| Concurrent Answer + silence + user | Impossible under SpeakOwner |

**Seq allocator:** `AppendTranscriptTurn` must serialize per `session_id` (row lock / advisory lock), not unlocked `MAX(seq)+1`.

### 4.3 Composer state (align `SOLUTION.md` §14)

Keep Listening → Capturing → Thinking → Speaking, but add **phase** (Welcoming vs Conversing) and **barge stage** (candidate vs commit). Playback unchanged (no barge).

---

## 5. Conversation flow / NLU (P0 content + P1 brain)

### 5.1 Stable contract

- **Intent ids** remain closed vocabulary (`sales_enquiry`, `product_information`, …).  
- Paths/skills key off **intent id**, never off surface language.  
- Example phrases on the desk are **hints**, not the only matcher.

### 5.2 P0 (Coral TFN workload — unblock)

1. Expand phrase bank: Hinglish + Hindi + English synonyms (e.g. प्रोडक्ट / उत्पाद / product).  
2. Normalize: casefold, strip punctuation, optional simple transliteration map for common Hinglish.  
3. Clarify policy: two clarifies → matrix **fallback intent** configured per desk (default Coral: `technical_support` **only if** matrix says so); prefer `unclear` path / soft re-ask before transfer.  
4. Do not accept bare English `product` alone below accept threshold without confirm (existing 0.40–0.60 band).

### 5.3 P1 (multilingual classify — preferred industrial path)

```text
STT final (any allowed language)
  → optional LanguageLock (first confident)
  → ClassifyIntent:
       A) phrase / normalize score (fast path)
       B) if below accept: Think LLM with enum=intent_ids + examples + user text
          → must return intent_id | unclear (schema-constrained)
  → AdvancePathStep in canonical path
  → Speak response in active_language
```

| Rule | Lock |
|---|---|
| LLM invents new intent | Forbidden |
| LLM picks skill args | Forbidden without path `arg_map` |
| Off-path question | KB answer if grounded → **return to same step** (existing §6.10) |
| Language of examples | Primary locale sufficient; LLM bridges other Indian languages |

---

## 6. India multilingual model (P1 — normative)

### 6.1 Authoring vs runtime

| Concern | Owner | Behaviour |
|---|---|---|
| **Primary locale** | Desk Overview | One language for Configurator copy (EN or HI or other tab) |
| **Locale tabs** | Optional | Extra prompt assets when operator cares about exact wording |
| **Runtime allowlist** | Desk + engine capabilities | Intersection of desk `runtime_languages` and Listen/Speak capability |
| **Default runtime** | Auto-detect | Per `LANGUAGE_POLICY` (updated): empty/`auto` until lock |

**Product statement (replaces “EN+HI only = product ceiling”):**  
Configuring a complete flow in **one primary language is enough**. The runtime must serve callers in **any allowlisted Indian language** using STT detect + LLM reply-in-language + TTS in `active_language`, without forcing the operator to duplicate the whole tree.

### 6.2 Session language fields

| Field | Meaning |
|---|---|
| `detected_language` | First confident Listen BCP-47 (historical) |
| `active_language` | Listen hint after lock; Think instruction; Speak language |
| `canonical_locale` | Desk primary — path/prompt **lookup** key |

### 6.3 Prompt resolution order (Speak)

1. Exact asset for `active_language` if tab exists.  
2. Else asset for `canonical_locale`.  
3. Else **synthesize**: Think/MT-lite instruction “render this meaning in `active_language`” from canonical text (audit as `response_tier=synthesized_locale`), then Speak.  
4. Else canned fail / escalate.

WAV clips: use only when locale matches; do not play wrong-language WAV; fall through to text path.

### 6.4 Allowlist (Contact Desk default)

Start from engine-reported India set. Document a **platform default allowlist** for Coral lab (ops may shrink):

`en-IN`, `hi-IN`, `bn-IN`, `ta-IN`, `te-IN`, `mr-IN`, `gu-IN`, `kn-IN`, `ml-IN`, `pa-IN`, `or-IN`, `as-IN`  
(Exact tags must match gateway docs; unknown detect → stay unlocked or map via gateway.)

If detect ∉ allowlist: keep speaking canonical; one soft line offering supported languages; do not hard-fail the call.

### 6.5 Relationship to `LANGUAGE_POLICY.md`

| Topic | Update |
|---|---|
| Auto-detect + lock + PATCH | Unchanged |
| “CC = EN/HI only” | **Superseded** for Contact Desk runtime by this §6 |
| Speak/Think after lock | Use `active_language` (unchanged) |
| Prompt tabs | Optional; synthesis fills gaps (new) |

---

## 7. CX policy fields (compile targets)

Extend desk CX / profile compile (names indicative):

| Field | Default | Phase |
|---|---|---|
| `rtp_settle_ms` | 500 | P0 |
| `silence_arm` | `after_welcome` | P0 |
| `welcome_barge_allowed` | false | P0 |
| `barge_mode` | `commit_on_stt` | P0 |
| `min_barge_ms` / `min_barge_chars` / `barge_partial_confidence` | 280 / 2 / 0.70 | P0 |
| `listen_while_speak` | true | already |
| `primary_locale` | desk | P1 |
| `runtime_languages` | india_default ∩ engines | P1 |
| `locale_synthesis` | true | P1 |
| `clarify_fallback` | desk matrix / `unclear` | P0 |

Silence timeouts (§6.12) unchanged **once armed**.

---

## 8. Function catalog deltas

| ID | Function | Phase | Behaviour |
|---|---|---|---|
| F-M09 | `EnterReady` | P0 | Settle / first media → Ready |
| F-M10 | `ArmSilenceWatch` | P0 | Only from Conversing |
| F-M11 | `EvaluateBarge` | P0 | Candidate → Commit/Ignore |
| F-M04 | `BargeIn` | P0 | **Only** after Commit |
| F-S04 | `AnswerSession` | P0 | Ready + SpeakOwner + complete mark |
| F-A02 | `ClassifyIntent` | P0/P1 | Phrases → LLM enum bridge |
| F-A13 | `ResolvePromptLocale` | P1 | Asset → synthesize → fail |
| F-M07 | `LanguageLock` | P1 | Allowlist-aware |

---

## 9. Phase exit criteria

### P0 — lab must pass

1. After stream +OK, first bot audio is **welcome** (not silence nudge), within settle + TTS budget.  
2. Noise / echo during welcome does **not** flush welcome when `welcome_barge_allowed=false`.  
3. Continuous inject+flush spray absent on quiet line.  
4. Meaningful barge mid-TTS: flush once; user + interrupted assistant transcript rows; bot continues path (no full welcome restart).  
5. No duplicate `transcript_turn` seq errors under overlap.  
6. Coral TFN: “मुझे प्रोडक्ट के बारे में जानना है” → `product_information` (not forced tech transfer).

### P1 — multilingual

1. Desk authored in one primary locale only publishes green.  
2. Caller in another allowlisted language: lock language; intent path advances; bot replies in caller language.  
3. Missing locale tab → synthesized prompt audited; call continues.  
4. Ambient language drift after lock does not flip session (existing law).

### P2 — depth (later)

1. `persona.voice` map by language.  
2. Optional semantic barge (LLM/classifier on partial) with latency budget.  
3. Profile `language.behaviour` interpret modes remain platform §24 (not required for TFN desk).

---

## 10. Explicit non-goals (this solution)

- Replacing FreeSWITCH / moving PCM to Kafka/Redis.  
- Vendor SDKs inside composer.  
- Forcing operators to author full trees in every Indian language.  
- Energy-only barge as Commit.  
- Restarting welcome after every barge.  
- LLM-invented ticket ids / transfers.

---

## 11. Implementation order (when coding starts)

Follow §13 work packages in order **WP0 → WP5**. Do not open P1 language widen until P0 lab exit (§9) is green.

**Docs to patch when product accepts this file:**  
`LANGUAGE_POLICY.md` (runtime India — already cross-linked), `CONTACT_DESK_POC_SOLUTION.md` §1 / §13 language + CX cross-link, `docs/SOLUTION.md` §14 barge note, `EDGE_FS.md` + `mod_audio_stream-1/docs/WIRE_PROTOCOL.md` for Ready event / Lua wait (§13.3).

---

## 12. Acceptance for this design

Say **yes** to this progressive solution **and §15 settlements** if:

1. Welcome cannot lose to silence or noise flush.  
2. Orch owns meaningful barge; FS only executes.  
3. One turn owner; overlap transcripts durable.  
4. Intent ids stay language-agnostic; India STT/LLM/TTS carry surface language.  
5. Primary-locale authoring is enough; locale tabs optional; synthesis fills gaps.  
6. P0 before P1 — no multilingual widen on a broken talk machine.  
7. Repo split in §13 is accepted (edge stays dumb; brain stays in orch).  
8. Single welcome trigger = Lua `POST /answer` after Ready (`auto_answer_on_ready=false`).  
9. Parent-doc patches in §16.6 happen **with** WP0 (same delivery), not a gate before coding.  
10. §16 Listen-finals gating and synchronous `/answer` are accepted.

---

## 13. Two-repo ownership (normative)

```text
┌──────────────────────────────┐     duplex WSS      ┌──────────────────────────────┐
│  mod_audio_stream-1          │◄───────────────────►│  com.coraltele.aiorchestrator │
│  (FreeSWITCH edge)           │  PCM + control JSON │  (speech-and-agent kernel)    │
│                              │                     │                              │
│  • Capture uplink PCM        │                     │  • Session phase machine       │
│  • Inject downlink PCM       │                     │  • SpeakOwner / TurnOwner      │
│  • Execute orch flush/stop   │                     │  • BargePolicy (Commit only)   │
│  • hello / frame timing      │                     │  • Listen/Think/Speak routers  │
│  • Lua dialplan glue         │                     │  • Desk NLU / path / CX        │
│  • NO VAD / NO AI / NO barge │                     │  • Language lock + synthesis   │
│    decision                  │                     │  • Transcript + audit          │
└──────────────────────────────┘                     └──────────────────────────────┘
```

### 13.1 Law

| Concern | Owner repo | Forbidden in the other |
|---|---|---|
| Meaningful speech / barge Commit | **aiorchestrator** | Module must not invent barge policy or local “flush because energy” |
| Flush / inject buffer clear | Orch **decides**; module **executes** | Module must not flush TTS on its own heuristics |
| Welcome vs silence ordering | **aiorchestrator** phase + AnswerSession | Lua must not start AI dialog before Ready |
| STT / LLM / TTS / India languages | **aiorchestrator** (gateways + desk) | Module never sees text or language |
| SIP/RTP codec / FS media bug | **mod_audio_stream** (+ FS) | Orch does not own FreeSWITCH internals |
| Wire dialect (`hello`, binary, `flush`) | **Both** (contract) | Either side breaking `WIRE_PROTOCOL` / `EDGE_FS` without dual update |

### 13.2 Work by package (progressive)

| WP | Phase | aiorchestrator | mod_audio_stream-1 | Exit |
|---|---|---|---|---|
| **WP0** | P0 | Session phases `Establishing→Ready→Welcoming→Conversing`; `EnterReady`; silence arm only after welcome; rewrite `AnswerSession` (SpeakOwner, mark complete only after speak) | Lua: after stream `+OK`, **wait for Ready** (poll/event or settle+orch ack) before `POST /answer`; prefer non-blocking answer wait; keep silence hold only as media filler | First bot audio = welcome; no silence nudge first |
| **WP1** | P0 | `EvaluateBarge`: energy → Candidate only; Commit on STT evidence; energy must **not** call `Flush` while Speaking; welcome barge gated | **No policy change** — keep `flush` executor; optional: log flush source as orch-only (already) | No inject+flush spray on quiet/noise line |
| **WP2** | P0 | SpeakOwner + single transcript seq allocator; persist interrupted assistant + user on Commit; queue user finals during Welcoming | None (unless debug counters) | No duplicate seq; overlap rows present |
| **WP3** | P0 | Coral TFN phrases (Hinglish प्रोडक्ट etc.) + safer clarify/fallback | None | Product Hindi phrase → `product_information` |
| **WP4** | P1 | `runtime_languages` ∩ engines; LanguageLock allowlist; ClassifyIntent LLM enum bridge; ResolvePromptLocale synthesis; Think/Speak use `active_language` | None | One primary locale authored; other India languages work |
| **WP5** | P0/P1 | Metrics + `tests/agent` scenarios for welcome/barge/locale | Optional: wire conformance test for Ready event if added | Lab checklist green |

### 13.3 Edge wire / Lua deltas (minimal — prefer orch signal)

**P0 Ready probe (locked):** Control HTTP only — keep module WS unchanged.

1. Module sends `hello`; orch Establishing → Ready (settle / first uplink).  
2. Lua polls e.g. `GET /v1/sessions/{id}` until `media_phase=ready` (or dedicated `GET …/media-ready`) with timeout (~3–5s); on timeout treat as Ready only if stream `+OK` **and** settle slept (degraded lab path — audit warning).  
3. Lua `POST …/answer` once (§2.4).  
4. On `409 not_ready` → brief sleep + retry. On `409 welcome_in_progress` → poll status, do not re-POST speak.

**Module WS change:** **Not required for P0.** Optional later: orch→module `session_phase` for observability only — still **no** local barge.

**Do not add to module:** VAD, STT, language detect, “smart flush,” welcome audio files, intent logic.

### 13.4 Capability assumption (India multilingual)

| Slot | Expectation |
|---|---|
| Listen | Auto-detect + finals in allowlisted Indian languages (e.g. Sarvam-class) |
| Think | Respond / classify in `active_language`; schema-constrained intent enum |
| Speak | TTS voices for allowlisted tags (remap table per tenant engines) |

If a tenant pins a mono-language gateway, runtime allowlist **shrinks** to that intersection (do not pretend full India). If desk `runtime_languages` still demands wider coverage than pinned engines → **fail closed** at publish/create (§0).

### 13.5 Optimization summary (why this order)

1. **Talk machine first (WP0–WP2):** Multilingual on a racing welcome/barge path wastes vendor spend and confuses RCA.  
2. **Cheap content unblock (WP3):** Phrase bank before LLM classify — unblocks Coral TFN immediately.  
3. **Language widen (WP4):** Uses same intent ids + path; STT/LLM/TTS carry surface language — no per-language playbook forks.  
4. **Edge thin:** Almost all CX intelligence in orch → one place to tune; module upgrades stay wire/stability only.

---

## 14. Sequence (happy path after this solution)

```text
Caller INVITE
  → FS answer + Lua session create
  → uuid_audio_stream start + silence_stream hold
  → hello + uplink PCM
  → orch Establishing → Ready (settle / first media)
  → AnswerSession → Welcoming (SpeakOwner) → welcome TTS inject
  → welcome mark → Conversing → ArmSilenceWatch
  → user speaks meaningfully → CommitBarge (if mid-TTS) → flush once
  → transcript user (+ interrupted assistant) → ClassifyIntent → path
  → Speak in active_language → …
```

Noise during Welcoming with `welcome_barge_allowed=false`: Candidate only; **no flush**; welcome completes.

---

## 15. Design review settlements (pre-implementation)

Dual review vs RCA, `RUNTIME.md`, `CONTACT_DESK_POC_SOLUTION.md`, `LANGUAGE_POLICY.md`, and current code (`composer.AnswerCall`, `StartLiveTalk`/`silenceWatch`, energy `bargeIn`, Lua 400 ms answer). Settlements below are **normative**; body sections above were updated to match.

### 15.1 Pass 1 — defects found in earlier draft

| # | Defect | Settlement |
|---|---|---|
| D1 | Dual welcome triggers (Lua `/answer` **and** orch auto-welcome) | **Lua-only** trigger P0; `auto_answer_on_ready` default false |
| D2 | Fail-closed vs fail-soft contradiction on languages | Publish/create fail-closed; mid-call OOS detect fail-soft (§0) |
| D3 | Welcoming “Commit = Per CX” ambiguous vs no-flush default | Table: Commit only if `welcome_barge_allowed` |
| D4 | `answered` vs `welcome_completed` | Prefer `welcome_completed`; answered aliases after complete |
| D5 | Energy still needed for endpointing | Energy OK for Listening/Capturing; **never** Flush while Speaking |
| D6 | RUNTIME §6 still energy-barge | This file **supersedes** for desk `commit_on_stt` |
| D7 | Partials may be unavailable | Degrade to finals-only Commit; no energy fallback |
| D8 | Filler denylist undefined | P0 length-only; denylist P2 |
| D9 | Interrupted assistant text undefined | Best-effort buffer; empty + `interrupted=true` OK |
| D10 | Seq race | Per-session lock on append |
| D11 | CX `BargeIn`/`ListenWhileSpeak` unused in code | Must wire in WP1 |
| D12 | Parent docs still “EN+HI first” | Patch parents **before** WP4 code; P0 may ship with phrase fixes only |
| D13 | Ready “or” misread as optional hello | Always hello+sink; then uplink **or** settle |
| D14 | `/answer` 409 wait vs wait-body | `not_ready` retry; `in_progress` poll no re-speak |

### 15.2 Pass 2 — remaining risks (accepted, not blockers)

| Risk | Mitigation |
|---|---|
| STT-only barge feels “slow” vs energy | Accept for correctness; tune partials/min_barge_ms; P2 semantic barge optional |
| Blocking `/answer` still long if WaitMark hangs | Cap WaitMark; return after first downlink / mark; flush must cancel WaitMark |
| Locale synthesis quality | Audit `response_tier`; operator can add tabs for critical prompts |
| LLM classify cost/latency | Phrase fast path first; LLM only below accept |
| Doc/code drift | Exit criteria §9 + WP5 tests mandatory before “done” |

### 15.3 Pre-flight checklist

**Superseded by §16.6** (final). Product acceptance = §12 bullets + §16 sign-off table.

### 15.4 Verdict (Pass 3 — final)

**Design blockers: none.** All prior defects (§15.1) are settled in body text. Accepted risks (§15.2) are product trade-offs, not open decisions.

**Implementation gap (expected):** current tree still has energy `bargeIn`, early `answered`, `silenceWatch` on attach — WP0–WP2 close that deliberately; not a reason to reopen design.

**Go:** implement **WP0** after §16 pre-flight (parent-doc pointers + API fields — can land in same PR as WP0 code).

---

## 16. Final review — implementation contract (no blockers)

Pass 3 vs platform session model, control API, desk runtime, Coral preset, and edge Lua. **Normative for coders.**

### 16.1 Platform session vs media phase (orthogonal)

| Platform session (`CONTACT_DESK` §6.1) | Media sub-phase (this file) | When |
|---|---|---|
| `Created` | — | `POST /v1/sessions` |
| `Attached` | `Establishing` | Edge WS up; `hello` pending or in flight |
| `Running` | `Ready` | hello + sink; settle / first uplink |
| `Running` | `Welcoming` | `/answer` → welcome SpeakOwner |
| `Running` | `Conversing` | welcome mark → dialog + silence armed |
| `Draining` | `Draining` | hangup / stop |
| Terminal | — | frozen attributes, disposition |

Expose on **`GET /v1/sessions/{id}`** (and lab SSE if present):

| Field | Type | Meaning |
|---|---|---|
| `media_phase` | string | `establishing` \| `ready` \| `welcoming` \| `conversing` \| `draining` |
| `welcome_completed` | bool | Welcome speak finished or hard-failed per §2.3 |
| `welcome_in_progress` | bool | Derived: `media_phase==welcoming` && !`welcome_completed` |

Legacy `answered` on `/answer` response: set **only when** `welcome_completed=true` (compat shim).

### 16.2 Listen / finals gating (closes welcome-race blocker)

| Phase | Listen stream | Uplink to Listen | Finals → desk |
|---|---|---|---|
| Establishing | May open (Ready detect + later barge) | Yes | **Drop/ignore for dialog** (no `DeskControllerTurn`) |
| Ready | Open | Yes | Ignore for dialog |
| Welcoming | Open | Yes (barge STT after WP1) | **Queue** non-empty finals FIFO; no Think/path |
| Conversing | Open | Yes | Dequeue queued finals **in order**, then live finals |

**Law:** `consumeListenFinals` must **not** call `OnListenFinal` / desk until `media_phase >= conversing`, except to append queued text internally.

### 16.3 WP0 `/answer` HTTP shape (single choice — no fork)

**P0 default (locked):** synchronous handler.

1. Lua `POST /v1/sessions/{id}/answer` when `media_phase=ready`.  
2. Orch → `Welcoming`; speaks welcome; blocks until Speak mark **or** hard fail after one retry (cap **30s**).  
3. Response `200`: `{ "welcome_completed": true, "answered": true }` (shim).  
4. Orch → `Conversing`; `ArmSilenceWatch`; drain welcome queue → desk turns.  
5. Errors: `409 not_ready` \| `409 welcome_in_progress` \| `504 welcome_timeout`.

Async `202 welcome_started` + poll: **optional later**; not required for P0 lab exit.

### 16.4 Desk dialog boundary (Coral TFN)

| Concern | Owner | Note |
|---|---|---|
| Media timing, barge Commit, welcome queue | This solution (orch) | WP0–WP2 |
| Intent/path/human-route shortcuts | Desk preset + `desk.Engine` | e.g. “transfer my call” → `transfer_sales_enquiry` — **Configurator paths**, not media layer |
| Phrase/clarify fixes | WP3 on `preset_coral` | Does not change §2–§4 |

Human-route before intake is **current Coral preset behaviour**; changing it is a **desk path edit**, not a live-talk blocker.

### 16.5 Parent-doc supersession (drift — not design blockers)

| Doc | Status | Action |
|---|---|---|
| `LIVE_TALK_CX_AND_INDIA_LANGUAGE.md` | **Wins** for desk Talk media + India runtime | — |
| `LANGUAGE_POLICY.md` | Already cross-linked | Verify §6.4 allowlist at WP4 |
| `RUNTIME.md` §6 energy barge | Superseded for desk | One-line pointer in WP0 doc patch |
| `CONTACT_DESK_POC_SOLUTION.md` §1 “EN+HI first” | Superseded by §6 here for **runtime**; EN+HI tabs remain valid **authoring** | One-line cross-link in WP0 doc patch |
| `SOLUTION.md` §14 | Add phase + barge stage note | WP0 doc patch |

Drift does **not** block WP0 code if implementers treat **this file** as normative for media behaviour.

### 16.6 Pre-flight checklist (updated)

- [ ] Product accepts §12 + §15 + §16  
- [ ] WP0 PR includes: `media_phase` fields + RUNTIME pointer (minimal)  
- [ ] Lab gateways: India-capable **or** honestly shrunk allowlist (WP4 only needs full India)  
- [ ] Implement WP0 → lab §9 items **1–3** → then WP1 → §9 **4–5** → WP2 → §9 **5** → WP3 → §9 **6**

### 16.7 Final sign-off

| Area | Blocker? | Status |
|---|---|---|
| Welcome vs silence ordering | No | Locked §2, §16.2 |
| Barge ownership | No | Locked §3 |
| Turn / transcript integrity | No | Locked §4 |
| India multilingual | No | P1 (WP4); EN+HI lab OK for P0 |
| Edge vs orch split | No | Locked §13 |
| API / Lua contract | No | Locked §16.1–16.3 |
| Coral TFN dialog | No | Preset + WP3; separate from media |
| Fail-closed vs soft language | No | Locked §0, §13.4 |

**Verdict: CLEARED FOR IMPLEMENTATION — start WP0.**
