package port

import (
	"fmt"
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
	Data       []byte
	SampleRate SampleRateHz
	Seq        uint64
	At         time.Time
}

// Capability is declared at gateway registration; routers filter by it.
type Capability struct {
	Streaming   bool
	Batch       bool
	Partials    bool
	Cancel      bool
	SSML        bool
	SampleRates []SampleRateHz // empty = gateway resamples any rate in 8–48 kHz
}

// Code is a closed set for APIs, audit, and analytics.
type Code string

const (
	CodeOK          Code = "ok"
	CodeCancelled   Code = "cancelled"
	CodeTimeout     Code = "timeout"
	CodeAuth        Code = "auth"
	CodeRateLimit   Code = "rate_limit"
	CodeBadAudio    Code = "bad_audio"
	CodeBadRequest  Code = "bad_request"
	CodeUnavailable Code = "unavailable"
	CodeNoHit       Code = "no_hit"
	CodeUnsupported Code = "unsupported"
	CodeInternal    Code = "internal"
)

// GatewayError is the only error type routers and composer should switch on.
type GatewayError struct {
	Code      Code
	Message   string
	Retryable bool
	Cause     error
}

func (e *GatewayError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *GatewayError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// AsGatewayError returns *GatewayError if err is or wraps one.
func AsGatewayError(err error) (*GatewayError, bool) {
	if err == nil {
		return nil, false
	}
	if ge, ok := err.(*GatewayError); ok {
		return ge, true
	}
	type unwrapper interface{ Unwrap() error }
	for {
		u, ok := err.(unwrapper)
		if !ok {
			return nil, false
		}
		err = u.Unwrap()
		if err == nil {
			return nil, false
		}
		if ge, ok := err.(*GatewayError); ok {
			return ge, true
		}
	}
}

// Health is returned by registry probes.
type Health struct {
	Healthy   bool
	LastOK    time.Time
	LastError string
	LatencyMS int64
}

// PortKind names a router rail.
type PortKind string

const (
	PortListen    PortKind = "listen"
	PortSpeak     PortKind = "speak"
	PortThink     PortKind = "think"
	PortTranslate PortKind = "translate"
	PortKnowledge PortKind = "knowledge"
	PortSkill     PortKind = "skill"
)
