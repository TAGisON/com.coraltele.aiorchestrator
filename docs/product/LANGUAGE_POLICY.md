# Language policy (Contact Agent)

**Status:** LOCKED  
**Date:** 1 September 2026  
**Parents:** `CONTACT_AGENT.md`, `PRODUCT_DECISIONS.md`  
**Related:** `SYSTEM_ENGINES.md`, `docs/architecture/PROFILE_SCHEMA.md`, `docs/architecture/RUNTIME.md` §9, `docs/architecture/CONTROL_API.md`, **`LIVE_TALK_CX_AND_INDIA_LANGUAGE.md`** (runtime India multilingual + CX)

---

## 1. Intent

For Contact Agent Talk sessions:

1. **Auto-detect** caller language by default (Listen without a hard-coded primary).
2. **Lock** after the first confident detection — later ambient re-detect does not flip the session.
3. **Switch** only via explicit operator action (`PATCH …/profile-fields` with `language.primary`) — not by drifting STT guesses.
4. **Author once, speak many (India):** Desk **primary locale** is enough to publish a complete flow. Runtime may serve any **allowlisted Indian language** supported by pinned Listen/Think/Speak gateways (see `LIVE_TALK_CX_AND_INDIA_LANGUAGE.md` §6). Locale tabs are optional exact-wording overrides, not a requirement to duplicate the tree.

Playbook/rule `set_language` action: **N/A this phase** (`RULES_AND_SKILLS.md` has no such action). Operator PATCH is the explicit path.

---

## 2. Profile defaults (CC)

- `language.auto_detect: true` for Contact Agent presets / fixtures
- Do **not** force a primary language when auto-detect is on (omit or empty `primary`)
- `mid_call_switch: true` required for mid-session `language.primary` hot-swap (missing → treat as false → reject PATCH)
- `hot_swap_allowed` must include `language.primary` for the PATCH key to be accepted

---

## 3. Sarvam Listen mapping

| `LanguageHint` | Wire `language_code` / `language-code` |
|---|---|
| empty or case-insensitive `auto` | `unknown` (auto-detect; **not** `en-IN`) |
| concrete BCP-47 (e.g. `hi-IN`) | pass through unchanged |

Phase F lab notes that mention STT default `en-IN` remain historical for callers that pass an explicit hint. Contact Agent auto-detect overrides via empty/`auto` → `unknown`. TTS empty-language fallback may still use `DefaultSTTLanguage`; that path is separate.

---

## 4. Session language state

| Field | Meaning |
|---|---|
| `detected_language` | First confident Listen final BCP-47 (historical; not overwritten by ambient re-detect) |
| `active_language` | Language Think + Speak consume; Listen hint after lock |
| `canonical_locale` | Desk primary locale — prompt/path **lookup** key (authoring language) |

`detected_language` / `active_language` start empty. Set together on first confident lock (when tag ∈ runtime allowlist). `canonical_locale` is set at session create from the pinned desk/profile.

### Confident (locked definition)

A Listen **final** is confident when:

- `Language` is a non-empty BCP-47 tag, and not `unknown` (case-insensitive)
- If `Confidence` is present and **> 0** (vendor supplied `language_probability` or equivalent), require `Confidence >= 0.5`
- If `Confidence` is absent / `0`, non-empty BCP-47 alone locks

Empty `Language` never locks.

### After lock

- Later finals with a different `Language` do **not** update `detected_language` or `active_language`
- Subsequent Listen opens use `LanguageHint = active_language` for accuracy
- Ambient detects still must not re-lock

### Before lock

- Speak: omit / empty `SpeakRequest.Language` → gateway default behaviour
- Think: no forced language instruction line

### After lock (and after explicit PATCH)

- Speak: `SpeakRequest.Language = active_language`
- Think: thinkpath injects a system instruction to respond in `active_language` (no new `ThinkRequest` field; PORTS freeze)
- Prompt assets: resolve by `active_language` → `canonical_locale` → **locale synthesis** (render canonical meaning in `active_language`) → fail/escalate (`LIVE_TALK_CX_AND_INDIA_LANGUAGE.md` §6.3)

### Allowlist

- Runtime languages = desk `runtime_languages` ∩ engine capabilities (India default set in live-CX solution §6.4).
- Detect outside allowlist: do not lock to unsupported tag; keep canonical Speak; soft offer supported languages; do not hard-fail the call.

---

## 5. Explicit switch

`PATCH /v1/sessions/{id}/profile-fields` with `{ "language.primary": "<BCP-47>" }`:

1. Key must be in pinned profile `hot_swap_allowed`
2. Profile `language.mid_call_switch` must be true (else 422)
3. Sets session `active_language` to the new value (`detected_language` left as first historical detect)
4. Per RUNTIME §9: next Listen open uses the new hint; flush partials if a stream is open

Other PATCH keys: reject with 422 (this phase scopes to `language.primary` only among implemented handlers; still enforce `hot_swap_allowed`).

---

## 6. Out of scope

- coral-file / home-page language UI
- Full MT `one_way` / `two_way` interpret as the Contact Desk default path (platform §24 remains available for interpret **profiles**)
- Voice mapping by language (**cc-3** / live-CX **P2**)
- Playbook `set_language` rule action
- Requiring operators to author full guided paths in every Indian language (primary + optional tabs + synthesis is the model)
