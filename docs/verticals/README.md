# Verticals

This platform is not a call center. Verticals **consume** it.

## Contact agent (AI Call Center)

Source of truth for that vertical (languages, two deploy profiles, golden scenarios):

`mod_audio_stream-1/docs/AI_Call_Center_Product_Decisions.md`

Telephony give/take (FreeSWITCH edge only): `mod_audio_stream-1` — **no STT, LLM, or TTS inside that module**.

When the vertical needs a platform capability this repo does not allow, **change** `docs/product/PRODUCT_DECISIONS.md` first.
