# Ports — frozen Go-shaped interfaces

**Status:** LOCKED (shapes; package paths may match `internal/port`)  
**Date:** 27 August 2026  
**Parents:** `CONTRACTS.md`, `PLATFORM_FIRST.md`

This file is the **implementation contract** for gateways. English semantics stay in `CONTRACTS.md`. Wire formats for Next AI etc. stay **outside** these types — only gateway packages know them.

When coding starts, copy these types into Go. Changing a method signature after Phase A is a **breaking change**: update this file first, then all fakes and gateways.

---

## 1. Shared types

```go
package port

import (
    "context"
    "time"
)

// SessionID is the orchestrator id. Gateways must not invent a second primary id.
type SessionID string

// GatewayID is a registry key, e.g. "tts-engine", "fake-listen", "nextai-stt".
type GatewayID string

// SampleRateHz is session-canonical PCM rate (8000–48000).
type SampleRateHz int

// PCMFrame is one ~20 ms (or profile frame_ms) of mono s16le little-endian audio.
type PCMFrame struct {
    Data      []byte
    SampleRate SampleRateHz // must equal session canonical when on the bus
    Seq       uint64
    At        time.Time
}

// Capability is declared at gateway registration; routers filter by it.
type Capability struct {
    Streaming    bool
    Batch        bool
    Partials     bool
    Cancel       bool
    SSML         bool
    SampleRates  []SampleRateHz // empty = gateway resamples any rate in 8–48 kHz
}

// Code is a closed set for APIs, audit, and analytics.
type Code string

const (
    CodeOK              Code = "ok"
    CodeCancelled       Code = "cancelled"
    CodeTimeout         Code = "timeout"
    CodeAuth            Code = "auth"
    CodeRateLimit       Code = "rate_limit"
    CodeBadAudio        Code = "bad_audio"
    CodeBadRequest      Code = "bad_request"
    CodeUnavailable     Code = "unavailable"
    CodeNoHit           Code = "no_hit"       // Knowledge
    CodeUnsupported     Code = "unsupported"
    CodeInternal        Code = "internal"
)

// GatewayError is the only error type routers and composer should switch on.
type GatewayError struct {
    Code    Code
    Message string
    Retryable bool
    Cause   error
}

func (e *GatewayError) Error() string { /* ... */ }

// Health is returned by registry probes.
type Health struct {
    Healthy   bool
    LastOK    time.Time
    LastError string
    LatencyMS int64
}
```

---

## 2. Listen (STT)

```go
type ListenRequest struct {
    SessionID    SessionID
    SampleRate   SampleRateHz
    LanguageHint string // BCP-47 or empty
    Clock        string // "live" | "playback"
}

type ListenPartial struct {
    Text       string
    Confidence float32 // 0 if unknown
    Language   string
}

type ListenFinal struct {
    Text       string
    Confidence float32
    Language   string // detected if available
    StartMS    int64  // optional relative to utterance
    EndMS      int64
}

// ListenStream is opened for live (streaming) recognition.
type ListenStream interface {
    // WritePCM pushes canonical frames. May return GatewayError.
    WritePCM(ctx context.Context, frame PCMFrame) error
    // Partials is optional; close when stream ends.
    Partials() <-chan ListenPartial
    // Finals delivers utterance finals; close when stream ends.
    Finals() <-chan ListenFinal
    // Close ends recognition (feeder stop or session stop).
    Close(ctx context.Context) error
}

type Listen interface {
    ID() GatewayID
    Capabilities() Capability
    // OpenStream for live. Requires Capability.Streaming.
    OpenStream(ctx context.Context, req ListenRequest) (ListenStream, error)
    // RecognizeBatch for playback blobs. Requires Capability.Batch.
    RecognizeBatch(ctx context.Context, req ListenRequest, pcm []byte) (ListenFinal, error)
}
```

---

## 3. Speak (TTS)

```go
type SpeakRequest struct {
    SessionID  SessionID
    Text       string
    SSML       bool
    VoiceID    string // profile persona voice_id; gateway interprets
    SampleRate SampleRateHz // session canonical out
    Language   string
}

// SpeakStream synthesizes and emits PCM until mark or cancel.
type SpeakStream interface {
    // Frames emits canonical PCM. Closed when utterance done or cancelled.
    Frames() <-chan PCMFrame
    // Done is closed when synthesis finished successfully (mark semantics).
    Done() <-chan struct{}
    // Cancel stops delivery; leftover vendor audio must not be written after Cancel.
    Cancel(ctx context.Context) error
}

type Speak interface {
    ID() GatewayID
    Capabilities() Capability
    Speak(ctx context.Context, req SpeakRequest) (SpeakStream, error)
}
```

---

## 4. Think (LLM)

```go
type ChatMessage struct {
    Role    string // "system" | "user" | "assistant" | "tool"
    Content string
}

type SkillDescriptor struct {
    Name        string
    Description string
    InputSchema []byte // JSON Schema
}

type ThinkRequest struct {
    SessionID        SessionID
    Messages         []ChatMessage
    GroundingChunks  []string
    Skills           []SkillDescriptor
    Stream           bool
}

type SkillProposal struct {
    Name string
    Args []byte // JSON matching InputSchema
}

type ThinkResult struct {
    Text           string
    SkillProposal  *SkillProposal // nil if none
}

// ThinkStream for token streaming when Stream=true.
type ThinkStream interface {
    Tokens() <-chan string
    // Result is available after Tokens closes (or early if gateway buffers).
    Result(ctx context.Context) (ThinkResult, error)
    Cancel(ctx context.Context) error
}

type Think interface {
    ID() GatewayID
    Capabilities() Capability
    Complete(ctx context.Context, req ThinkRequest) (ThinkResult, error)
    CompleteStream(ctx context.Context, req ThinkRequest) (ThinkStream, error)
}
```

---

## 5. Translate (MT)

```go
type TranslateRequest struct {
    SessionID SessionID
    Text      string
    Source    string // BCP-47
    Target    string
}

type Translate interface {
    ID() GatewayID
    Capabilities() Capability
    Translate(ctx context.Context, req TranslateRequest) (string, error)
}
```

---

## 6. Knowledge

```go
type KnowledgeQuery struct {
    SessionID   SessionID
    Query       string
    Collections []string
    TopK        int
}

type KnowledgeSnippet struct {
    Text       string
    SourceURI  string
    Score      float32
    DocumentID string
}

type KnowledgeResult struct {
    Hit      bool
    Snippets []KnowledgeSnippet
}

type Knowledge interface {
    ID() GatewayID
    Capabilities() Capability
    Retrieve(ctx context.Context, q KnowledgeQuery) (KnowledgeResult, error)
}
```

`CodeNoHit` / `Hit=false` are both valid; composer treats `Hit=false` when `grounding.required`.

---

## 7. Skill

```go
type SkillRequest struct {
    SessionID SessionID
    Name      string
    Args      []byte // validated JSON
    TenantID  string
}

type SkillResult struct {
    OK     bool
    Output []byte // JSON; redacted before audit per policy
}

type Skill interface {
    ID() GatewayID
    Capabilities() Capability
    // Execute once. Side effects: router must not auto-retry on success ambiguity.
    Execute(ctx context.Context, req SkillRequest) (SkillResult, error)
}
```

Warm transfer (`coral-transfer`) implements `Skill`. Payload fields for Coral HTTP are in `RULES_AND_SKILLS.md`; this port stays generic.

---

## 8. Feeder and Sink (edges)

```go
type FeederMeta struct {
    PeerID       string // e.g. FS call uuid
    PeerRate     SampleRateHz
    SessionID    SessionID
}

type FeederEvent struct {
    Kind string // "dtmf" | "stop" | "error"
    Data string
}

type Feeder interface {
    ID() GatewayID
    // Frames after resample to session canonical.
    Frames() <-chan PCMFrame
    Events() <-chan FeederEvent
    Close(ctx context.Context) error
}

type Sink interface {
    ID() GatewayID
    WritePCM(ctx context.Context, frame PCMFrame) error // edge encodes to peer rate
    Flush(ctx context.Context) error                      // barge-in: drop unplayed
    // Mark signals playout finished for Talk composer when needed.
    WaitMark(ctx context.Context) error
    Close(ctx context.Context) error
}
```

FS edge implements Feeder+Sink behind one connection; types stay separate.

---

## 9. Registry

```go
type Factory func(cfg map[string]string) (any, error) // concrete port type

type Registration struct {
    ID           GatewayID
    Port         string // "listen"|"speak"|"think"|"translate"|"knowledge"|"skill"
    Capabilities Capability
    Factory      Factory
    Probe        func(ctx context.Context) Health
}

type Registry interface {
    Register(r Registration) error
    Get(id GatewayID) (Registration, bool)
    List(port string) []Registration
}
```

Routers take `Registry` + profile provider list + session clock; they never construct vendor clients.

---

## 10. Contract tests (required)

For each port, a shared test suite must assert:

| Case | Expectation |
|---|---|
| Happy path | Fake returns expected shape |
| Cancel | Speak/Listen/Think stop within timeout; no frames after Cancel |
| Timeout | Returns `CodeTimeout`, retryable as declared |
| Live without Streaming | Router refuses gateway |
| Knowledge miss | `Hit=false`, no panic |
| Skill side-effect | Second Execute is a new call; no auto-retry inside gateway |

Real gateways (TTS-Engine, later Next AI) run the **same** suite with build tags or integration env.

---

## 11. What this file deliberately excludes

- Next AI / Sarvam HTTP/WS payloads  
- TTS-Engine gRPC stubs (live in `internal/gateway/ttsengine`)  
- Control HTTP OpenAPI (separate; maps to session lifecycle, not these ports)  
- Profile YAML (see `PROFILE_SCHEMA.md`)

Gateways adapt **to** these ports. Ports do not adapt to vendors.
