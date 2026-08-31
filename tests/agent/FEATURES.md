# Feature test catalog — Speech-and-Agent Platform

**Status:** source of truth for product validation  
**Wave:** `validation-v1` (no live FreeSWITCH until human provides endpoint)  
**Machine index:** `features/catalog.yaml`  
**Scenarios:** `scenarios/*.yaml` (one primary scenario id per feature id)

Testers (scenario-planner → … → test-summarizer) **must** cover every feature with `status: must_test` or `status: deferred` below. Do not invent product scope outside this catalog and locked docs.

---

## How to use

| Column | Meaning |
|---|---|
| **Feature id** | Stable id (`F-…`); scenario file `scenarios/<id>.yaml` |
| **What** | Observable behaviour to prove |
| **How** | Primary verification path (`go_test`, HTTP, skip) |
| **Evidence** | Audit / analytics / logs expected |
| **Status** | `must_test` \| `optional_live` \| `deferred` \| `out_of_scope_v1` |

---

## A — Ports, routers, fakes

| Id | Feature | What we need to test | How | Evidence | Status |
|---|---|---|---|---|---|
| `F-ports-contract` | Port contracts | Every Listen/Think/Speak/Translate/Knowledge/Skill fake passes contract tests; `GatewayError` / capability shapes hold | `go test ./internal/gateway/fake` (+ port contract pkgs) | none required | must_test |
| `F-router-registry` | Gateway registry + routers | Profile gateway ids resolve; wrong rail / unknown id rejected at selection or validate | `go test ./internal/router` `./internal/profile` | none | must_test |
| `F-fake-trio` | Fake Listen/Think/Speak | Talk/unit path can run entirely on fakes without vendors | covered by runtime + fake tests | none | must_test |

---

## B — Durable platform & Control API

| Id | Feature | What we need to test | How | Evidence | Status |
|---|---|---|---|---|---|
| `F-store-migrate` | Postgres migrations | profile, profile_version, session, audit_event, playback_job, postcall_job (and E tables) migrate clean | `go test ./internal/store` | schema present | must_test |
| `F-control-health` | `GET /v1/health` | Process health endpoint responds | control tests / HTTP | none | must_test |
| `F-control-profile-crud` | Profile create + version publish | Create draft; publish immutable version; get version | `go test ./internal/control` | none | must_test |
| `F-control-profile-422` | Profile validate | Unknown / wrong-rail gateway id → 422 at publish | control + profile tests | none | must_test |
| `F-control-session-lifecycle` | Session create / get / stop | Create pins profile version; get state; stop → terminal | control tests | session audit on terminal when wired | must_test |
| `F-control-inject` | Session inject | Text inject with/without speak flag accepted for session | control / phase tests | audit optional | must_test |
| `F-control-hot-swap` | Profile-fields PATCH | Only `hot_swap_allowed` keys accepted; money/PII rules not mid-swap | control tests if present else note gap | none | must_test |
| `F-control-sse-events` | Session SSE | `GET /v1/sessions/{id}/events` catalog / stream | phase_e / control | event names match ANALYTICS lock | must_test |
| `F-control-playback-job` | Playback job enqueue | `POST /v1/jobs/playback` + job get | control / phase_d | job row | must_test |
| `F-control-kb-upload` | KB document upload | `POST /v1/kb/documents` status path | control / ingest | none | must_test |
| `F-openapi` | OpenAPI published | `api/openapi.yaml` (or equivalent) matches Control routes used in tests | file + spot-check | none | must_test |

---

## C — Runtime kernel

| Id | Feature | What we need to test | How | Evidence | Status |
|---|---|---|---|---|---|
| `F-runtime-session` | Session actor | Lifecycle Created → … → terminal without FS | `go test ./internal/runtime/session` | none | must_test |
| `F-runtime-bus` | In-process bus | Events delivered in-process (no Kafka for PCM) | `go test ./internal/runtime/bus` | none | must_test |
| `F-runtime-clock` | Live + playback clocks | Clock selection per session/job | `go test ./internal/runtime/clock` | none | must_test |
| `F-runtime-vad` | Local VAD | VAD decisions unit-tested | `go test ./internal/runtime/vad` | none | must_test |
| `F-runtime-composer` | Talk composer + barge-in | Turn machine with fake Speak; barge-in path | `go test ./internal/runtime/composer` | barge_in analytics when E wired | must_test |
| `F-runtime-thinkpath` | Think path | Rules > skills > grounding > LLM order; knowledge/skill hooks with fakes | `go test ./internal/runtime/thinkpath` | audit skill/knowledge when applicable | must_test |

---

## D — Edges & first-party adapters

| Id | Feature | What we need to test | How | Evidence | Status |
|---|---|---|---|---|---|
| `F-edge-file` | File feeder (playback) | File edge feeds PCM/frames into runtime path in tests | `go test ./internal/edge/file` | none | must_test |
| `F-edge-token` | Edge token | Token mint/validate for edge attach | `go test ./internal/edge/token` | none | must_test |
| `F-edge-modaudiostream` | mod_audio_stream edge unit | Protocol/edge unit tests without live FS | `go test ./internal/edge/modaudiostream` | none | must_test |
| `F-edge-fs-live` | Live FreeSWITCH call | FS → session → inject round-trip on lab | requires human endpoint | audit + analytics full call | deferred |
| `F-gw-ingest` | Knowledge ingest-default | Index/retrieve path for uploaded docs | `go test ./internal/gateway/ingest` | none | must_test |
| `F-gw-coral-transfer` | Skill coral-transfer | Transfer skill executes (HTTP may stub) + audit hook | `go test ./internal/gateway/coraltransfer` | skill audit | must_test |
| `F-gw-coral-crm` | Skill coral-crm | CRM skill executes (may stub) | `go test ./internal/gateway/coralcrm` | skill audit | must_test |
| `F-gw-tts-engine` | TTS-Engine Speak | First-party Speak gateway | deferred in product | — | deferred |

---

## E — Audit, analytics, post-call

| Id | Feature | What we need to test | How | Evidence | Status |
|---|---|---|---|---|---|
| `F-audit-append` | Audit append-only | Session/turn terminal writes audit_event rows | store_e + control phase_e | audit kinds present | must_test |
| `F-analytics-events` | Analytics event catalog | session_started/completed/failed, turn, hop_latency, etc. as implemented | store_e + phase_e | analytics_event rows | must_test |
| `F-postcall-enqueue` | Postcall on terminal | postcall_job enqueued on session terminal / playback complete | store_e + phase_e | postcall_job row | must_test |
| `F-validation-v1` | Validation V1 umbrella harness | Tier A V1-A01..A08 Control+memory+audit+SSE+edge-gone+disposition | `go test ./internal/validation` | audit + analytics + disposition | must_test |

---

## F — External vendors (Sarvam)

| Id | Feature | What we need to test | How | Evidence | Status |
|---|---|---|---|---|---|
| `F-sarvam-stt` | sarvam-stt Listen | Contract/unit vs fake-listen shape | `go test ./internal/gateway/sarvamstt` | none | must_test |
| `F-sarvam-llm` | sarvam-llm Think | Contract/unit | `go test ./internal/gateway/sarvamllm` | none | must_test |
| `F-sarvam-tts` | sarvam-tts Speak | Contract/unit | `go test ./internal/gateway/sarvamtts` | none | must_test |
| `F-sarvam-failover` | Failover to fakes | Profile `[sarvam-*, fake-*]` fails over when Sarvam errors | `go test ./internal/gateway/sarvam` | none | must_test |
| `F-sarvam-live` | Live Sarvam E2E | Real API STT/LLM/TTS when key present | env-gated live tests | optional | optional_live |

---

## Architecture locks (negative tests)

| Id | Feature | What we need to test | How | Evidence | Status |
|---|---|---|---|---|---|
| `F-lock-no-pcm-broker` | No Kafka/Redis for live PCM | Bus/runtime stay in-memory for PCM | code + bus tests; reviewer check | none | must_test |
| `F-lock-no-vendor-in-composer` | Composer has no vendor SDK imports | `internal/runtime/composer` does not import sarvam/nextai | static review / go list | none | must_test |
| `F-lock-no-secrets-in-git` | Secrets never committed | `.agent/secrets.local.json` gitignored; fixtures have no keys | git check + fixtures scan | none | must_test |

---

## Job-family profiles (product vision — profile fixtures)

These are **profiles of the same platform**. V1 validates that profile documents + fakes can express them; full E2E may be deferred.

| Id | Family | What we need to test | Status |
|---|---|---|---|
| `F-job-contact-agent` | Contact Talk agent | Fixture profile listen+speak+think+skill transfer; unit/session path | must_test (fixture + unit) |
| `F-cc-lab-engines` | Contact Agent lab engines | Tenant engines + session `gateway_binding` + lab inject/transcript | must_test |
| `F-cc-behaviour-presets` | Contact Agent behaviour presets | ≥2 same-tenant CC fixtures (Sales/R&D/after-hours); clip/think_down unit | must_test |
| `F-job-captions` | Captions / Listen-only | Fixture listen-only; SSE/text path unit | must_test (fixture + unit) |
| `F-job-playback-mom` | Playback meeting pack | Playback job + think template fixture | must_test (fixture + unit) |
| `F-job-interpret` | Two-way interpret | Translate rail profile | deferred (MT depth) |
| `F-job-copilot` | Grounded copilot | Knowledge + rules fixture | must_test (fixture + unit) |

---

## Wave gates

**validation-v1 passes when:** every `must_test` feature has a scenario with result `pass` (or documented `skip` only if harness proves package missing — then **fail** for inventory gap), every `deferred` / `optional_live` is explicitly skipped with reason, audit-validator checks E features, test-reviewer confirms no silent drops from this catalog.

**Later wave (FS):** flip `F-edge-fs-live` to must_test when call-server URL is decided.
