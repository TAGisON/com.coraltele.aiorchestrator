# Language policy (Contact Agent)

**Status:** STUB LOCK (behaviour; implementation in cc-2)  
**Date:** 31 August 2026  
**Parents:** `CONTACT_AGENT.md`, `PRODUCT_DECISIONS.md`  
**Related:** `SYSTEM_ENGINES.md`, `docs/architecture/PROFILE_SCHEMA.md`

---

## 1. Intent (brainstorm → product)

For Contact Agent Talk sessions:

1. **Auto-detect** caller language by default (Listen without a hard-coded primary).
2. **Lock** after the first confident detection — later ambient re-detect does not flip the session.
3. **Switch** only via explicit user/operator action (control PATCH / playbook / rule) — not by drifting STT guesses.

Full runtime fields (`detected_language`, `active_language`), Sarvam empty/`unknown` mapping, and Think/Speak consumption land in **cc-2**. This file exists so cc-1 docs cross-link cleanly and PROFILE defaults stay aligned.

---

## 2. Profile defaults (CC)

- `language.auto_detect: true` preferred for Contact Agent presets
- Do not force a primary language in CC lab presets when auto-detect is on
- `mid_call_switch` remains for **explicit** switch paths only after lock

---

## 3. Out of scope until cc-2

- Persist language on session row  
- Sarvam STT language_code wiring  
- MT one_way / two_way interpret families  
