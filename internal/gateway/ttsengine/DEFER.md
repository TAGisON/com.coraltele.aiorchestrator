# TTS-Engine Speak gateway — deferred

**Status:** Deferred (Phase D exit path b)  
**Date:** 27 August 2026

## Decision

Real gRPC Speak adapter `tts-engine` is **deferred** until a lab TTS-Engine address and proto/contract are available.

## What remains green

- Profile CI and Talk composer continue on **`fake-speak`**.
- Composer barge-in / cancel contract tests are unchanged and must stay green (`go test ./internal/gateway/fake/... ./internal/runtime/composer/...`).
- No composer or thinkpath changes for vendors.

## When to implement

Add `internal/gateway/ttsengine` implementing `port.Speak` with id `tts-engine`, register beside fakes, and run the same Speak cancel/frame contract suite behind `TTS_ENGINE_ADDR` or an integration build tag.

Do **not** invent Next AI Speak as a substitute.
