# 02 — Current state (what we have vs discard)

Honest inventory after live DID experiments (coral-xfer, Sarvam, FreeSWITCH).  
**Keep the media pipe. Rebuild the dialogue brain.**

## Keep (foundation)

| Piece | Status | Notes |
|---|---|---|
| FreeSWITCH ↔ orchestrator **PCM streaming** | Done | `mod_audio_stream` dumb pipe: uplink caller, downlink bot |
| Session create / answer / stop control surface | Exists | Shape may simplify; pattern stays |
| Edge **transfer** / **hangup** control verbs | Partially working | Semantics must become Tool-arm lock (architecture), not more patches |
| Sarvam STT / TTS / LLM **gateways** | Exists | Keep as replaceable vendors behind ports |
| Go orchestrator process + lab on :8011 | Exists | Continues as control plane host |
| Routing matrix idea (intent → number) | Exists | Becomes Tool param source |

## Discard as product architecture (rebuild)

These may still exist in code on `main`; they are **not** the target brain:

| Piece | Why wrong for the goal |
|---|---|
| Desk step-list engine as hybrid owner + keyword NLU + LLM assist | Three brains; races; FAQ from fragments |
| Turn-pair transcript as the only conversation record | Misses suppressed speech; order inversions under barge |
| Soft hangup/transfer without consistent arm→speak→exec | Dead air / late hangup after transfer |
| Language + FAQ + transfer mashed without per-node repair | Out-of-context utterances escape the step |
| Recording that continues after session end | Evidence and disk broken (observed multi‑tens of minutes after ~45s call) |
| Platform SKUs: captions / interpret / meeting pack as active goals | Out of this programme |

## Lessons from live calls (examples)

- Transfer to sales can succeed while **CX is wrong** (phantom FAQ turn from STT fragment after language switch).
- Hangup can be requested then aborted if WebSocket tears down before the edge verb settles.
- Offline WAV STT of the recording can disagree with orch transcript order when barge/overlap happens.
- Returning ANI language lock + mid-call “talk in English” must be a **graph edge**, not ad-hoc.

## Direction

```text
[KEEP]  Media pipe + gateway adapters + tool verbs to FS
[REPLACE] Dialogue ownership → Conversation Graph + Live Turn Machine
[ADD]     Continuous transcript/audit events; binding model for KB/CRM
[DEFER]   Post-call summary, CRM push, agent assist, QM
```

Implementation will follow the new docs on this branch; old product/architecture markdown under `docs/` was removed and replaced by this set.
